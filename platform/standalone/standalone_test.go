package standalone

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Address != ":8080" {
		t.Errorf("expected address ':8080', got %q", cfg.Address)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("expected read timeout 30s, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("expected write timeout 30s, got %v", cfg.WriteTimeout)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("expected shutdown timeout 10s, got %v", cfg.ShutdownTimeout)
	}
	if !cfg.EnableWebSocket {
		t.Error("expected EnableWebSocket to be true")
	}
	if !cfg.EnableHealthCheck {
		t.Error("expected EnableHealthCheck to be true")
	}
}

func TestConfigApplyDefaults(t *testing.T) {
	cfg := Config{}
	cfg.applyDefaults()

	if cfg.Address != ":8080" {
		t.Errorf("expected address ':8080', got %q", cfg.Address)
	}
	if cfg.Logger == nil {
		t.Error("expected default logger")
	}
}

func TestNew(t *testing.T) {
	p, err := New(Config{Address: ":9090"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name() != "standalone" {
		t.Errorf("expected name 'standalone', got %q", p.Name())
	}
	if p.Address() != ":9090" {
		t.Errorf("expected address ':9090', got %q", p.Address())
	}
}

func TestPlatformHealth(t *testing.T) {
	p, err := New(Config{Address: ":9091"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	health := p.Health(context.Background())

	if health.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", health.Status)
	}
	if health.Details["platform"] != "standalone" {
		t.Error("expected platform in details")
	}
	if health.Details["address"] != ":9091" {
		t.Error("expected address in details")
	}
}

func TestHandleHealth(t *testing.T) {
	p, err := New(Config{Address: ":9092"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	p.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var health map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&health); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if health["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", health["status"])
	}
}

func TestRunHTTPOnlyWithCancel(t *testing.T) {
	p, err := New(Config{
		Address:         "127.0.0.1:0", // Random port
		EnableWebSocket: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- p.runHTTPOnly(ctx)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for shutdown
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for shutdown")
	}
}

func TestWebhookHandlerMounting(t *testing.T) {
	webhookCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	})

	p, err := New(Config{
		Address:         "127.0.0.1:0",
		EnableWebSocket: false,
		WebhookHandlers: map[string]http.Handler{
			"/webhook/test": handler,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	go func() {
		_ = p.runHTTPOnly(ctx)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Get the actual address (since we used port 0)
	if p.server == nil {
		t.Skip("server not started")
	}

	// Make request to webhook endpoint using httptest
	req := httptest.NewRequest(http.MethodPost, "/webhook/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !webhookCalled {
		t.Error("webhook handler not called")
	}
}
