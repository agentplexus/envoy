// Package standalone provides a standalone HTTP server platform for omniagent.
//
// The standalone platform runs a local HTTP server with:
//   - WebSocket control plane for real-time client connections
//   - Webhook endpoints for receiving messages from external services
//   - Health check endpoint for monitoring
//   - Graceful shutdown handling
//
// # Example Usage
//
//	p, _ := standalone.New(standalone.Config{
//	    Address: ":8080",
//	    WebhookHandlers: map[string]http.Handler{
//	        "/webhook/twilio/sms": twilioHandler,
//	    },
//	})
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
//	defer cancel()
//
//	if err := p.Run(ctx, myAgent); err != nil {
//	    log.Fatal(err)
//	}
package standalone

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/gateway"
	"github.com/plexusone/omniagent/platform"
)

// Platform is a standalone HTTP server platform.
type Platform struct {
	config  Config
	gateway *gateway.Gateway
	server  *http.Server
	logger  *slog.Logger
	agent   *agent.Agent
	mu      sync.RWMutex
}

// New creates a new standalone platform.
func New(config Config) (*Platform, error) {
	config.applyDefaults()

	return &Platform{
		config: config,
		logger: config.Logger,
	}, nil
}

// Name implements platform.Platform.
func (p *Platform) Name() string {
	return "standalone"
}

// Run implements platform.Platform.
// It starts the HTTP server and blocks until context is cancelled.
func (p *Platform) Run(ctx context.Context, a *agent.Agent) error {
	p.mu.Lock()
	p.agent = a
	p.mu.Unlock()

	// Create gateway for WebSocket support
	if p.config.EnableWebSocket {
		gw, err := gateway.New(gateway.Config{
			Address:         p.config.Address,
			ReadTimeout:     p.config.ReadTimeout,
			WriteTimeout:    p.config.WriteTimeout,
			Agent:           a,
			Logger:          p.logger,
			WebhookHandlers: p.config.WebhookHandlers,
		})
		if err != nil {
			return fmt.Errorf("create gateway: %w", err)
		}
		p.gateway = gw

		p.logger.Info("standalone platform starting",
			"address", p.config.Address,
			"websocket", true,
			"webhooks", len(p.config.WebhookHandlers))

		return gw.Run(ctx)
	}

	// HTTP-only mode (no WebSocket)
	return p.runHTTPOnly(ctx)
}

// runHTTPOnly runs the platform in HTTP-only mode without WebSocket support.
func (p *Platform) runHTTPOnly(ctx context.Context) error {
	mux := http.NewServeMux()

	// Mount webhook handlers
	for path, handler := range p.config.WebhookHandlers {
		p.logger.Info("mounting webhook handler", "path", path)
		mux.Handle(path, handler)
	}

	// Health check endpoint
	if p.config.EnableHealthCheck {
		mux.HandleFunc("/health", p.handleHealth)
	}

	p.server = &http.Server{
		Addr:         p.config.Address,
		Handler:      mux,
		ReadTimeout:  p.config.ReadTimeout,
		WriteTimeout: p.config.WriteTimeout,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		p.logger.Info("standalone platform starting",
			"address", p.config.Address,
			"websocket", false,
			"webhooks", len(p.config.WebhookHandlers))

		var err error
		if p.config.TLS != nil {
			err = p.server.ListenAndServeTLS(p.config.TLS.CertFile, p.config.TLS.KeyFile)
		} else {
			err = p.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		p.logger.Info("standalone platform shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), p.config.ShutdownTimeout)
		defer cancel()
		return p.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// handleHealth handles health check requests.
func (p *Platform) handleHealth(w http.ResponseWriter, _ *http.Request) {
	health := p.Health(context.Background())

	w.Header().Set("Content-Type", "application/json")
	if health.Status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(health)
}

// Health implements platform.HealthChecker.
func (p *Platform) Health(_ context.Context) platform.Health {
	details := map[string]any{
		"platform": "standalone",
		"address":  p.config.Address,
	}

	if p.gateway != nil {
		details["websocket_clients"] = p.gateway.ClientCount()
	}

	return platform.Health{
		Status:  "healthy",
		Details: details,
	}
}

// Address returns the configured listen address.
func (p *Platform) Address() string {
	return p.config.Address
}

// Gateway returns the underlying gateway, if WebSocket is enabled.
func (p *Platform) Gateway() *gateway.Gateway {
	return p.gateway
}
