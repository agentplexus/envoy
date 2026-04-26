package standalone

import (
	"log/slog"
	"net/http"
	"time"
)

// Config configures the standalone platform.
type Config struct {
	// Address is the HTTP server listen address (default: ":8080").
	Address string

	// ReadTimeout is the HTTP read timeout (default: 30s).
	ReadTimeout time.Duration

	// WriteTimeout is the HTTP write timeout (default: 30s).
	WriteTimeout time.Duration

	// ShutdownTimeout is the graceful shutdown timeout (default: 10s).
	ShutdownTimeout time.Duration

	// Logger for platform logging.
	Logger *slog.Logger

	// WebhookHandlers maps paths to HTTP handlers for incoming webhooks.
	// Example: {"/webhook/twilio/sms": twilioHandler}
	WebhookHandlers map[string]http.Handler

	// EnableWebSocket enables the WebSocket control plane (default: true).
	EnableWebSocket bool

	// EnableHealthCheck enables the /health endpoint (default: true).
	EnableHealthCheck bool

	// TLS configures TLS for the HTTP server.
	TLS *TLSConfig
}

// TLSConfig configures TLS for the HTTP server.
type TLSConfig struct {
	CertFile string
	KeyFile  string
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Address:           ":8080",
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		EnableWebSocket:   true,
		EnableHealthCheck: true,
	}
}

// applyDefaults fills in default values for unset fields.
func (c *Config) applyDefaults() {
	if c.Address == "" {
		c.Address = ":8080"
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 30 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 30 * time.Second
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = 10 * time.Second
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
}
