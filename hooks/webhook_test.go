package hooks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebhookHookInterface(t *testing.T) {
	// Ensure WebhookHook implements Hook interface
	var _ Hook = (*WebhookHook)(nil)
}

func TestWebhookHookName(t *testing.T) {
	w := &WebhookHook{HookName: "test-webhook"}
	if w.Name() != "test-webhook" {
		t.Errorf("expected name %q, got %q", "test-webhook", w.Name())
	}
}

func TestWebhookHookEvents(t *testing.T) {
	events := []EventType{EventMessageReceived, EventMessageSent}
	w := &WebhookHook{HookEvents: events}

	got := w.Events()
	if len(got) != 2 {
		t.Errorf("expected 2 events, got %d", len(got))
	}
	if got[0] != EventMessageReceived || got[1] != EventMessageSent {
		t.Error("events not preserved")
	}
}

func TestWebhookHookInit(t *testing.T) {
	w := &WebhookHook{}

	err := w.Init(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check defaults
	if w.Method != http.MethodPost {
		t.Errorf("expected method POST, got %q", w.Method)
	}
	if w.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", w.Timeout)
	}
	if w.RetryDelay != time.Second {
		t.Errorf("expected retry delay 1s, got %v", w.RetryDelay)
	}
	if w.client == nil {
		t.Error("expected non-nil client")
	}
}

func TestWebhookHookHandle(t *testing.T) {
	var receivedBody []byte
	var receivedHeaders http.Header
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		receivedHeaders = r.Header
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := &WebhookHook{
		HookName:   "test",
		HookEvents: []EventType{EventMessageReceived},
		URL:        server.URL,
		Method:     http.MethodPost,
		Headers: map[string]string{
			"Authorization": "Bearer token123",
		},
		Timeout: 5 * time.Second,
	}

	if err := webhook.Init(context.Background()); err != nil {
		t.Fatalf("init error: %v", err)
	}

	event := NewEvent(EventMessageReceived, MessageEvent{
		Role:    "user",
		Content: "hello",
	})

	err := webhook.Handle(context.Background(), event)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify request
	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}

	// Verify headers
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Error("missing Content-Type header")
	}
	if receivedHeaders.Get("Authorization") != "Bearer token123" {
		t.Error("missing custom header")
	}

	// Verify body
	var parsed Event
	if err := json.Unmarshal(receivedBody, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if parsed.Type != EventMessageReceived {
		t.Errorf("expected type %q, got %q", EventMessageReceived, parsed.Type)
	}
}

func TestWebhookHookRetry(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := &WebhookHook{
		HookName:   "retry-test",
		HookEvents: []EventType{EventToolCalled},
		URL:        server.URL,
		RetryCount: 2,
		RetryDelay: 10 * time.Millisecond,
	}

	if err := webhook.Init(context.Background()); err != nil {
		t.Fatalf("init error: %v", err)
	}

	err := webhook.Handle(context.Background(), NewEvent(EventToolCalled, nil))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&requestCount) != 3 {
		t.Errorf("expected 3 requests (initial + 2 retries), got %d", requestCount)
	}
}

func TestWebhookHookRetryExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	webhook := &WebhookHook{
		HookName:   "fail-test",
		HookEvents: []EventType{EventToolCalled},
		URL:        server.URL,
		RetryCount: 1,
		RetryDelay: 10 * time.Millisecond,
	}

	if err := webhook.Init(context.Background()); err != nil {
		t.Fatalf("init error: %v", err)
	}

	err := webhook.Handle(context.Background(), NewEvent(EventToolCalled, nil))
	if err == nil {
		t.Error("expected error after retries exhausted")
	}
}

func TestWebhookHookContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := &WebhookHook{
		HookName:   "cancel-test",
		HookEvents: []EventType{EventToolCalled},
		URL:        server.URL,
		Timeout:    5 * time.Second,
	}

	if err := webhook.Init(context.Background()); err != nil {
		t.Fatalf("init error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := webhook.Handle(ctx, NewEvent(EventToolCalled, nil))
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestWebhookHookClose(t *testing.T) {
	webhook := &WebhookHook{}
	if err := webhook.Init(context.Background()); err != nil {
		t.Fatalf("init error: %v", err)
	}

	err := webhook.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
