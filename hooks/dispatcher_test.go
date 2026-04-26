package hooks

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewDispatcher(t *testing.T) {
	r := NewRegistry()
	d := NewDispatcher(r)

	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}
	if d.registry != r {
		t.Error("registry not set")
	}
}

func TestDispatcherEmit(t *testing.T) {
	r := NewRegistry()
	d := NewDispatcher(r)

	var receivedEvent Event
	r.RegisterHandler(EventMessageReceived, "test", func(ctx context.Context, event Event) error {
		receivedEvent = event
		return nil
	})

	data := MessageEvent{Role: "user", Content: "hello"}
	err := d.Emit(context.Background(), EventMessageReceived, data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if receivedEvent.Type != EventMessageReceived {
		t.Errorf("expected type %q, got %q", EventMessageReceived, receivedEvent.Type)
	}

	msg, ok := receivedEvent.Data.(MessageEvent)
	if !ok {
		t.Fatal("data not MessageEvent")
	}
	if msg.Content != "hello" {
		t.Error("data not preserved")
	}
}

func TestDispatcherEmitWithSession(t *testing.T) {
	r := NewRegistry()
	d := NewDispatcher(r)

	var receivedEvent Event
	r.RegisterHandler(EventSessionCreated, "test", func(ctx context.Context, event Event) error {
		receivedEvent = event
		return nil
	})

	err := d.EmitWithSession(context.Background(), EventSessionCreated, "sess-123", SessionEvent{
		SessionID: "sess-123",
		Action:    "created",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if receivedEvent.SessionID != "sess-123" {
		t.Errorf("expected session ID %q, got %q", "sess-123", receivedEvent.SessionID)
	}
}

func TestDispatcherEmitError(t *testing.T) {
	r := NewRegistry()
	d := NewDispatcher(r)

	expectedErr := errors.New("handler error")
	r.RegisterHandler(EventToolCalled, "error-handler", func(ctx context.Context, event Event) error {
		return expectedErr
	})

	err := d.Emit(context.Background(), EventToolCalled, nil)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestDispatcherEmitAsync(t *testing.T) {
	r := NewRegistry()
	d := NewDispatcher(r)

	var called int32
	r.RegisterHandler(EventMessageSent, "async-test", func(ctx context.Context, event Event) error {
		atomic.StoreInt32(&called, 1)
		return nil
	})

	d.EmitAsync(context.Background(), EventMessageSent, nil)

	// Wait for async execution
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&called) != 1 {
		t.Error("async handler not called")
	}
}

func TestDispatcherEmitAsyncWithSession(t *testing.T) {
	r := NewRegistry()
	d := NewDispatcher(r)

	var receivedSessionID string
	var called int32
	r.RegisterHandler(EventSessionUpdated, "async-session-test", func(ctx context.Context, event Event) error {
		receivedSessionID = event.SessionID
		atomic.StoreInt32(&called, 1)
		return nil
	})

	d.EmitAsyncWithSession(context.Background(), EventSessionUpdated, "async-sess", nil)

	// Wait for async execution
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&called) != 1 {
		t.Error("async handler not called")
	}
	if receivedSessionID != "async-sess" {
		t.Errorf("expected session ID %q, got %q", "async-sess", receivedSessionID)
	}
}

func TestDispatcherNilRegistry(t *testing.T) {
	d := NewDispatcher(nil)

	// Should not panic with nil registry
	err := d.Emit(context.Background(), EventMessageReceived, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = d.EmitWithSession(context.Background(), EventMessageReceived, "sess", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Async should also not panic
	d.EmitAsync(context.Background(), EventMessageReceived, nil)
	d.EmitAsyncWithSession(context.Background(), EventMessageReceived, "sess", nil)
}

func TestDispatcherEventTimestamp(t *testing.T) {
	r := NewRegistry()
	d := NewDispatcher(r)

	var receivedEvent Event
	r.RegisterHandler(EventToolCompleted, "timestamp-test", func(ctx context.Context, event Event) error {
		receivedEvent = event
		return nil
	})

	before := time.Now()
	_ = d.Emit(context.Background(), EventToolCompleted, nil)
	after := time.Now()

	if receivedEvent.Timestamp.Before(before) || receivedEvent.Timestamp.After(after) {
		t.Error("timestamp not in expected range")
	}
}
