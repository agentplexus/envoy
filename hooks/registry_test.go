package hooks

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/plexusone/omnistorage-core/kvs"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.handlers == nil {
		t.Error("expected non-nil handlers map")
	}
}

func TestRegistryRegisterHandler(t *testing.T) {
	r := NewRegistry()

	var called bool
	handler := func(ctx context.Context, event Event) error {
		called = true
		return nil
	}

	r.RegisterHandler(EventMessageReceived, "test-handler", handler)

	if r.HandlerCount(EventMessageReceived) != 1 {
		t.Errorf("expected 1 handler, got %d", r.HandlerCount(EventMessageReceived))
	}

	// Dispatch event
	err := r.Dispatch(context.Background(), NewEvent(EventMessageReceived, nil))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !called {
		t.Error("handler not called")
	}
}

func TestRegistryDispatchMultipleHandlers(t *testing.T) {
	r := NewRegistry()

	var count int32

	for i := 0; i < 3; i++ {
		r.RegisterHandler(EventToolCalled, "", func(ctx context.Context, event Event) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
	}

	err := r.Dispatch(context.Background(), NewEvent(EventToolCalled, nil))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 handlers called, got %d", count)
	}
}

func TestRegistryDispatchHandlerError(t *testing.T) {
	r := NewRegistry()

	expectedErr := errors.New("test error")

	r.RegisterHandler(EventMessageSent, "error-handler", func(ctx context.Context, event Event) error {
		return expectedErr
	})

	err := r.Dispatch(context.Background(), NewEvent(EventMessageSent, nil))
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestRegistryDispatchContinuesOnError(t *testing.T) {
	r := NewRegistry()

	var secondCalled bool

	r.RegisterHandler(EventMessageSent, "error-handler", func(ctx context.Context, event Event) error {
		return errors.New("first error")
	})
	r.RegisterHandler(EventMessageSent, "success-handler", func(ctx context.Context, event Event) error {
		secondCalled = true
		return nil
	})

	_ = r.Dispatch(context.Background(), NewEvent(EventMessageSent, nil))

	if !secondCalled {
		t.Error("second handler not called after first handler error")
	}
}

// mockHook is a test implementation of Hook.
type mockHook struct {
	name        string
	events      []EventType
	initCalled  bool
	handleCalls int
	closeCalled bool
	initErr     error
	handleErr   error
	closeErr    error
}

func (h *mockHook) Name() string {
	return h.name
}

func (h *mockHook) Events() []EventType {
	return h.events
}

func (h *mockHook) Init(ctx context.Context) error {
	h.initCalled = true
	return h.initErr
}

func (h *mockHook) Handle(ctx context.Context, event Event) error {
	h.handleCalls++
	return h.handleErr
}

func (h *mockHook) Close() error {
	h.closeCalled = true
	return h.closeErr
}

func TestRegistryRegisterHook(t *testing.T) {
	r := NewRegistry()

	hook := &mockHook{
		name:   "test-hook",
		events: []EventType{EventMessageReceived, EventMessageSent},
	}

	err := r.RegisterHook(hook)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if r.HookCount() != 1 {
		t.Errorf("expected 1 hook, got %d", r.HookCount())
	}

	// Check handler count for events the hook handles
	if r.HandlerCount(EventMessageReceived) != 1 {
		t.Errorf("expected 1 handler for EventMessageReceived, got %d", r.HandlerCount(EventMessageReceived))
	}
	if r.HandlerCount(EventMessageSent) != 1 {
		t.Errorf("expected 1 handler for EventMessageSent, got %d", r.HandlerCount(EventMessageSent))
	}
	// Hook doesn't handle this event
	if r.HandlerCount(EventToolCalled) != 0 {
		t.Errorf("expected 0 handlers for EventToolCalled, got %d", r.HandlerCount(EventToolCalled))
	}
}

func TestRegistryInit(t *testing.T) {
	r := NewRegistry()

	hook := &mockHook{
		name:   "test-hook",
		events: []EventType{EventMessageReceived},
	}
	_ = r.RegisterHook(hook)

	err := r.Init(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !hook.initCalled {
		t.Error("hook Init not called")
	}
}

func TestRegistryInitError(t *testing.T) {
	r := NewRegistry()

	hook := &mockHook{
		name:    "error-hook",
		events:  []EventType{EventMessageReceived},
		initErr: errors.New("init failed"),
	}
	_ = r.RegisterHook(hook)

	err := r.Init(context.Background())
	if err == nil {
		t.Error("expected error")
	}
}

func TestRegistryClose(t *testing.T) {
	r := NewRegistry()

	hook := &mockHook{
		name:   "test-hook",
		events: []EventType{EventMessageReceived},
	}
	_ = r.RegisterHook(hook)

	err := r.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !hook.closeCalled {
		t.Error("hook Close not called")
	}
}

func TestRegistryDispatchToHook(t *testing.T) {
	r := NewRegistry()

	hook := &mockHook{
		name:   "test-hook",
		events: []EventType{EventToolCalled},
	}
	_ = r.RegisterHook(hook)
	_ = r.Init(context.Background())

	// Dispatch event that hook handles
	_ = r.Dispatch(context.Background(), NewEvent(EventToolCalled, nil))

	if hook.handleCalls != 1 {
		t.Errorf("expected 1 handle call, got %d", hook.handleCalls)
	}

	// Dispatch event that hook doesn't handle
	_ = r.Dispatch(context.Background(), NewEvent(EventMessageReceived, nil))

	if hook.handleCalls != 1 {
		t.Errorf("expected still 1 handle call, got %d", hook.handleCalls)
	}
}

func TestRegistryDispatchAsync(t *testing.T) {
	r := NewRegistry()

	var called int32

	r.RegisterHandler(EventSessionCreated, "async-handler", func(ctx context.Context, event Event) error {
		atomic.StoreInt32(&called, 1)
		return nil
	})

	r.DispatchAsync(context.Background(), NewEvent(EventSessionCreated, nil))

	// Wait for async dispatch
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&called) != 1 {
		t.Error("async handler not called")
	}
}

func TestRegistryDispatchNoHandlers(t *testing.T) {
	r := NewRegistry()

	// Should not error when no handlers registered
	err := r.Dispatch(context.Background(), NewEvent(EventJobExecuted, nil))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegistryRegisterWebhook(t *testing.T) {
	r := NewRegistry()

	webhook := &WebhookHook{
		HookName:   "test-webhook",
		HookEvents: []EventType{EventMessageReceived},
		URL:        "https://example.com/hook",
	}

	r.RegisterWebhook(webhook)

	if r.HookCount() != 1 {
		t.Errorf("expected 1 hook, got %d", r.HookCount())
	}

	if r.HandlerCount(EventMessageReceived) != 1 {
		t.Errorf("expected 1 handler for EventMessageReceived, got %d", r.HandlerCount(EventMessageReceived))
	}
}

// mockStorageAwareHook implements both Hook and StorageAware.
type mockStorageAwareHook struct {
	mockHook
	storage kvs.Store
}

func (h *mockStorageAwareHook) SetStorage(s kvs.Store) {
	h.storage = s
}

// mockStore is a minimal kvs.Store implementation for testing.
type mockStore struct{}

func (m *mockStore) Get(_ context.Context, _ string) ([]byte, error)                  { return nil, nil }
func (m *mockStore) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error { return nil }
func (m *mockStore) Delete(_ context.Context, _ string) error                         { return nil }
func (m *mockStore) Close() error                                                     { return nil }

func TestRegistrySetStorage(t *testing.T) {
	r := NewRegistry()

	hook := &mockStorageAwareHook{
		mockHook: mockHook{
			name:   "storage-aware-hook",
			events: []EventType{EventMessageReceived},
		},
	}

	_ = r.RegisterHook(hook)

	// Set storage after registration
	mockStorage := &mockStore{}
	r.SetStorage(mockStorage)

	if hook.storage != mockStorage {
		t.Errorf("expected storage to be injected, got %v", hook.storage)
	}
}

func TestRegistryRegisterHookWithStorageInjection(t *testing.T) {
	r := NewRegistry()

	// Set storage first
	mockStorage := &mockStore{}
	r.SetStorage(mockStorage)

	hook := &mockStorageAwareHook{
		mockHook: mockHook{
			name:   "storage-aware-hook",
			events: []EventType{EventMessageReceived},
		},
	}

	_ = r.RegisterHook(hook)

	// Storage should be injected during registration
	if hook.storage != mockStorage {
		t.Errorf("expected storage to be injected on registration, got %v", hook.storage)
	}
}

func TestRegistryCloseError(t *testing.T) {
	r := NewRegistry()

	hook := &mockHook{
		name:     "error-hook",
		events:   []EventType{EventMessageReceived},
		closeErr: errors.New("close failed"),
	}
	_ = r.RegisterHook(hook)

	err := r.Close()
	if err == nil {
		t.Error("expected error from Close")
	}

	if !hook.closeCalled {
		t.Error("hook Close not called")
	}
}

func TestRegistryDispatchHookError(t *testing.T) {
	r := NewRegistry()

	expectedErr := errors.New("hook handle error")
	hook := &mockHook{
		name:      "error-hook",
		events:    []EventType{EventToolCompleted},
		handleErr: expectedErr,
	}
	_ = r.RegisterHook(hook)

	err := r.Dispatch(context.Background(), NewEvent(EventToolCompleted, nil))
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestRegistryDispatchContinuesAfterHookError(t *testing.T) {
	r := NewRegistry()

	hook1 := &mockHook{
		name:      "error-hook",
		events:    []EventType{EventSessionUpdated},
		handleErr: errors.New("hook1 error"),
	}
	hook2 := &mockHook{
		name:   "success-hook",
		events: []EventType{EventSessionUpdated},
	}

	_ = r.RegisterHook(hook1)
	_ = r.RegisterHook(hook2)

	_ = r.Dispatch(context.Background(), NewEvent(EventSessionUpdated, nil))

	if hook2.handleCalls != 1 {
		t.Error("second hook not called after first hook error")
	}
}

func TestRegistryInitNoHooks(t *testing.T) {
	r := NewRegistry()

	// Should not error when no hooks registered
	err := r.Init(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegistryCloseNoHooks(t *testing.T) {
	r := NewRegistry()

	// Should not error when no hooks registered
	err := r.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
