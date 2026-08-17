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

	"github.com/plexusone/omniagent/sessions"
)

// AgentProcessor processes messages through an AI agent.
type AgentProcessor interface {
	Process(ctx context.Context, sessionID, content string) (string, error)
}

// SessionToolConfigurator applies per-session tool overrides. Agents that
// support session-scoped tool scoping implement it in addition to
// AgentProcessor; the gateway feature is unavailable otherwise.
type SessionToolConfigurator interface {
	SetSessionToolOverrides(ctx context.Context, sessionID string, overrides *sessions.ToolOverrides) error
}

// SessionModelConfigurator applies per-session model selection. Agents that
// support it implement this in addition to AgentProcessor; the gateway
// feature is unavailable otherwise.
type SessionModelConfigurator interface {
	SetSessionModel(ctx context.Context, sessionID, model string, sticky bool) error
}

// SessionAwareProcessor is implemented by agents that maintain durable
// per-session conversation history (RMI-OMNIAGENT-007). Detected via type
// assertion so an agent without a configured session store keeps today's
// stateless AgentProcessor.Process behavior — mirrors the SessionTool/
// SessionModelConfigurator optional-capability pattern above.
type SessionAwareProcessor interface {
	ProcessWithSession(ctx context.Context, sessionID, content string) (string, error)
	SessionStore() *sessions.Store
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
	authLimiter *authFailureLimiter
	metrics     *Metrics

	// extraMounts are additional HTTP handlers registered by embedders
	// (e.g. team-mode auth/admin routes), applied to the mux in Run.
	extraMounts map[string]http.Handler

	// connectAuthorizer, when set, gates WebSocket upgrades: it runs before
	// the upgrade and, on success, returns the authenticated user ID bound
	// to the client. Used by team mode for cookie-authenticated sockets.
	connectAuthorizer ConnectAuthorizer

	// Handlers
	onMessage MessageHandler
}

// ConnectAuthorizer authorizes a WebSocket upgrade request. It returns the
// authenticated user ID and true to allow the upgrade, or false to reject it
// (the caller responds 401 before upgrading).
type ConnectAuthorizer func(r *http.Request) (userID string, ok bool)

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
		config:      config,
		clients:     make(map[string]*Client),
		logger:      config.Logger,
		agent:       config.Agent,
		authLimiter: newAuthFailureLimiter(),
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

// Handle registers an additional HTTP handler at pattern, applied to the
// server mux when Run starts. Patterns follow http.ServeMux rules; a
// trailing slash (e.g. "/api/") mounts a subtree.
func (g *Gateway) Handle(pattern string, handler http.Handler) {
	if g.extraMounts == nil {
		g.extraMounts = make(map[string]http.Handler)
	}
	g.extraMounts[pattern] = handler
}

// SetConnectAuthorizer installs a WebSocket upgrade authorizer (team mode).
func (g *Gateway) SetConnectAuthorizer(a ConnectAuthorizer) {
	g.connectAuthorizer = a
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

	// Mount embedder handlers (e.g. team-mode auth/admin routes).
	for pattern, handler := range g.extraMounts {
		g.logger.Info("mounting handler", "pattern", pattern)
		mux.Handle(pattern, handler)
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
	// Team mode gates the upgrade on a valid session cookie before
	// upgrading, and binds the authenticated user to the client.
	var userID string
	if g.connectAuthorizer != nil {
		var ok bool
		userID, ok = g.connectAuthorizer(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	conn, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	client := newClient(conn, g)
	if userID != "" {
		client.SetMetadata("user_id", userID)
		client.SetMetadata("authenticated", true)
	}
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

// BroadcastToUsers sends a message only to connected clients whose
// authenticated user_id is in userIDs — the membership-scoped fan-out for
// chat rooms (RMI-112). A chat's message is delivered to exactly its members'
// sockets and no others, so there is no cross-chat leakage: a client whose
// user is not a member never receives it. Clients with no bound user_id
// (unauthenticated) are never matched.
func (g *Gateway) BroadcastToUsers(userIDs []string, msg *Message) {
	if len(userIDs) == 0 {
		return
	}
	recipients := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		if id != "" {
			recipients[id] = struct{}{}
		}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, client := range g.clients {
		uid, ok := client.GetMetadata("user_id")
		if !ok {
			continue
		}
		s, _ := uid.(string)
		if _, member := recipients[s]; member {
			client.Send(msg)
		}
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
