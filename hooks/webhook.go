package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/plexusone/omniagent/internal/httputil"
)

// WebhookHook sends events to an HTTP endpoint.
type WebhookHook struct {
	// HookName is the identifier for this webhook.
	HookName string `json:"name" yaml:"name"`

	// HookEvents lists the event types this webhook receives.
	HookEvents []EventType `json:"events" yaml:"events"`

	// URL is the endpoint to send events to.
	URL string `json:"url" yaml:"url"`

	// Method is the HTTP method (POST or PUT). Defaults to POST.
	Method string `json:"method" yaml:"method"`

	// Headers are additional HTTP headers to send.
	Headers map[string]string `json:"headers" yaml:"headers"`

	// Timeout is the request timeout. Defaults to 10 seconds.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// RetryCount is the number of retries on failure. Defaults to 0.
	RetryCount int `json:"retry_count" yaml:"retry_count"`

	// RetryDelay is the delay between retries. Defaults to 1 second.
	RetryDelay time.Duration `json:"retry_delay" yaml:"retry_delay"`

	client *http.Client
}

// Name implements Hook.
func (w *WebhookHook) Name() string {
	return w.HookName
}

// Events implements Hook.
func (w *WebhookHook) Events() []EventType {
	return w.HookEvents
}

// Init implements Hook.
func (w *WebhookHook) Init(_ context.Context) error {
	// Set defaults
	if w.Method == "" {
		w.Method = http.MethodPost
	}
	if w.Timeout == 0 {
		w.Timeout = 10 * time.Second
	}
	if w.RetryDelay == 0 {
		w.RetryDelay = time.Second
	}

	w.client = &http.Client{
		Timeout: w.Timeout,
	}

	return nil
}

// Close implements Hook.
func (w *WebhookHook) Close() error {
	if w.client != nil {
		w.client.CloseIdleConnections()
	}
	return nil
}

// Handle implements Hook.
func (w *WebhookHook) Handle(ctx context.Context, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	var lastErr error
	attempts := w.RetryCount + 1

	for i := 0; i < attempts; i++ {
		if i > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(w.RetryDelay):
			}
		}

		if err := w.sendRequest(ctx, body); err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	return fmt.Errorf("webhook failed after %d attempts: %w", attempts, lastErr)
}

// sendRequest sends the HTTP request.
func (w *WebhookHook) sendRequest(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, w.Method, w.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Drain body to allow connection reuse (bounded to prevent OOM)
	_, _ = io.Copy(io.Discard, httputil.LimitReader(resp.Body, httputil.MaxJSONBodySize))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
