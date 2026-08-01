// Package gateway provides the WebSocket control plane for omniagent.
package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AgentProcessor processes messages through an AI agent.
type AgentProcessor interface {
	Process(ctx context.Context, sessionID, content string) (string, error)
}

// Config configures the gateway server.
type Config struct {
	Address         string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PingInterval    time.Duration
	Logger          *slog.Logger
	Agent           AgentProcessor
	WebhookHandlers map[string]http.Handler // Path -> Handler for webhook endpoints
	AllowedOrigins  []string                // Allowed origins for WebSocket connections (empty allows all)
	APIKeys         []string                // Valid API keys for authentication (empty disables auth)
	RequireAuth     bool                    // If true, clients must authenticate before sending messages
	RateLimit       *RateLimitConfig        // Per-sender rate limiting config (nil disables)
	EnableMetrics   bool                    // If true, expose /metrics endpoint for Prometheus
}

// Gateway is the WebSocket control plane server.
type Gateway struct {
	config      Config
	upgrader    websocket.Upgrader
	clients     map[string]*Client
	mu          sync.RWMutex
	logger      *slog.Logger
	agent       AgentProcessor
	rateLimiter *RateLimiter
	metrics     *Metrics

	// Handlers
	onMessage MessageHandler
}

// MessageHandler handles incoming messages from clients.
type MessageHandler func(ctx context.Context, client *Client, msg *Message) (*Message, error)

// New creates a new Gateway.
func New(config Config) (*Gateway, error) {
	if config.Address == "" {
		config.Address = "127.0.0.1:18789"
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 30 * time.Second
	}
	if config.PingInterval == 0 {
		config.PingInterval = 30 * time.Second
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	gw := &Gateway{
		config:  config,
		clients: make(map[string]*Client),
		logger:  config.Logger,
		agent:   config.Agent,
	}

	// Initialize rate limiter if configured
	if config.RateLimit != nil {
		gw.rateLimiter = NewRateLimiter(*config.RateLimit)
	}

	// Initialize metrics if enabled
	if config.EnableMetrics {
		gw.metrics = NewMetrics("omniagent")
	}

	// Configure WebSocket upgrader with origin checking
	gw.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     gw.checkOrigin,
	}

	// Set up default message handler
	defaultHandler := NewDefaultMessageHandler(gw)
	gw.onMessage = defaultHandler.Handle

	return gw, nil
}

// OnMessage sets the message handler.
func (g *Gateway) OnMessage(handler MessageHandler) {
	g.onMessage = handler
}

// Run starts the gateway server.
func (g *Gateway) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", g.handleWebSocket)
	mux.HandleFunc("/health", g.handleHealth)

	// Mount metrics endpoint if enabled
	if g.metrics != nil {
		mux.Handle("/metrics", g.metrics.Handler())
		g.logger.Info("metrics endpoint enabled", "path", "/metrics")
	}

	// Mount webhook handlers
	for path, handler := range g.config.WebhookHandlers {
		g.logger.Info("mounting webhook handler", "path", path)
		mux.Handle(path, handler)
	}

	server := &http.Server{
		Addr:         g.config.Address,
		Handler:      mux,
		ReadTimeout:  g.config.ReadTimeout,
		WriteTimeout: g.config.WriteTimeout,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		g.logger.Info("gateway starting", "address", g.config.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		g.logger.Info("gateway shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// handleWebSocket handles WebSocket upgrade requests.
func (g *Gateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	client := newClient(conn, g)
	g.registerClient(client)

	go client.readPump()
	go client.writePump()
}

// handleHealth handles health check requests.
func (g *Gateway) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := struct {
		Status  string `json:"status"`
		Clients int    `json:"clients"`
	}{
		Status:  "ok",
		Clients: g.ClientCount(),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// registerClient registers a new client.
func (g *Gateway) registerClient(client *Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clients[client.ID] = client
	g.logger.Info("client connected", "id", client.ID)
}

// unregisterClient removes a client.
func (g *Gateway) unregisterClient(client *Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.clients[client.ID]; ok {
		delete(g.clients, client.ID)
		g.logger.Info("client disconnected", "id", client.ID)
	}
}

// ClientCount returns the number of connected clients.
func (g *Gateway) ClientCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.clients)
}

// Broadcast sends a message to all connected clients.
func (g *Gateway) Broadcast(msg *Message) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, client := range g.clients {
		client.Send(msg)
	}
}

// GetClient returns a client by ID.
func (g *Gateway) GetClient(id string) *Client {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.clients[id]
}

// checkOrigin validates the WebSocket upgrade request origin.
// If no allowed origins are configured, all origins are allowed.
// Otherwise, the request origin must match one of the allowed origins.
func (g *Gateway) checkOrigin(r *http.Request) bool {
	// If no allowed origins configured, allow all (development mode)
	if len(g.config.AllowedOrigins) == 0 {
		return true
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		// No origin header - allow same-origin requests (no Origin header means same-origin)
		return true
	}

	// Parse the origin
	originURL, err := url.Parse(origin)
	if err != nil {
		g.logger.Warn("invalid origin header", "origin", origin, "error", err)
		return false
	}

	// Check against allowed origins
	for _, allowed := range g.config.AllowedOrigins {
		// Support wildcard matching
		if allowed == "*" {
			return true
		}

		// Exact match
		if strings.EqualFold(origin, allowed) {
			return true
		}

		// Parse allowed origin for comparison
		allowedURL, err := url.Parse(allowed)
		if err != nil {
			continue
		}

		// Match scheme and host (port included in host)
		if strings.EqualFold(originURL.Scheme, allowedURL.Scheme) &&
			strings.EqualFold(originURL.Host, allowedURL.Host) {
			return true
		}

		// Support wildcard subdomain matching (e.g., "https://*.example.com")
		if strings.HasPrefix(allowedURL.Host, "*.") {
			baseDomain := strings.TrimPrefix(allowedURL.Host, "*.")
			if strings.EqualFold(originURL.Scheme, allowedURL.Scheme) &&
				(strings.EqualFold(originURL.Host, baseDomain) ||
					strings.HasSuffix(strings.ToLower(originURL.Host), "."+strings.ToLower(baseDomain))) {
				return true
			}
		}
	}

	g.logger.Warn("origin not allowed", "origin", origin, "allowed", g.config.AllowedOrigins)
	return false
}
