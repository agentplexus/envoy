package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/grokify/mogo/log/slogutil"
	"github.com/plexusone/omnistorage-core/kvs"
	"golang.org/x/sync/semaphore"
)

const (
	// maxConcurrentDispatches limits concurrent async hook dispatches to prevent
	// resource exhaustion. Excess dispatches are dropped with a warning.
	maxConcurrentDispatches = 100
)

// Registry manages hook registrations and event dispatch.
type Registry struct {
	handlers  map[EventType][]registeredHandler
	hooks     []Hook
	storage   kvs.Store
	mu        sync.RWMutex
	asyncSem  *semaphore.Weighted // Limits concurrent async dispatches
	asyncOnce sync.Once           // Ensures semaphore is initialized once
}

// registeredHandler wraps a handler with its name.
type registeredHandler struct {
	name    string
	handler HandlerFunc
}

// NewRegistry creates a new hook registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[EventType][]registeredHandler),
	}
}

// RegisterHandler registers a quick handler for an event type.
// The name is optional and used for logging.
func (r *Registry) RegisterHandler(event EventType, name string, handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[event] = append(r.handlers[event], registeredHandler{
		name:    name,
		handler: handler,
	})
}

// RegisterHook registers a compiled hook.
// The hook will be initialized when Init() is called on the registry.
func (r *Registry) RegisterHook(hook Hook) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Inject storage if the hook needs it
	if sa, ok := hook.(StorageAware); ok && r.storage != nil {
		sa.SetStorage(r.storage)
	}

	r.hooks = append(r.hooks, hook)
	return nil
}

// RegisterWebhook registers a webhook-based hook.
func (r *Registry) RegisterWebhook(webhook *WebhookHook) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Webhooks are treated as compiled hooks
	r.hooks = append(r.hooks, webhook)
}

// SetStorage sets the storage backend for storage-aware hooks.
func (r *Registry) SetStorage(s kvs.Store) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.storage = s

	// Inject storage into existing hooks that need it
	for _, hook := range r.hooks {
		if sa, ok := hook.(StorageAware); ok {
			sa.SetStorage(s)
		}
	}
}

// Init initializes all registered hooks.
func (r *Registry) Init(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	logger := slogutil.LoggerFromContext(ctx, slog.Default())

	for _, hook := range r.hooks {
		if err := hook.Init(ctx); err != nil {
			return fmt.Errorf("init hook %s: %w", hook.Name(), err)
		}
		logger.Debug("initialized hook", "name", hook.Name(), "events", hook.Events())
	}

	return nil
}

// Close releases resources for all registered hooks.
func (r *Registry) Close() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []error
	for _, hook := range r.hooks {
		if err := hook.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close hook %s: %w", hook.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close hooks: %d errors", len(errs))
	}
	return nil
}

// Dispatch dispatches an event to all registered handlers synchronously.
// Returns the first error encountered, but continues dispatching to remaining handlers.
func (r *Registry) Dispatch(ctx context.Context, event Event) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	logger := slogutil.LoggerFromContext(ctx, slog.Default())
	var firstErr error

	// Dispatch to quick handlers
	handlers := r.handlers[event.Type]
	for _, h := range handlers {
		if err := h.handler(ctx, event); err != nil {
			logger.Error("handler error",
				"event", event.Type,
				"handler", h.name,
				"error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	// Dispatch to compiled hooks
	for _, hook := range r.hooks {
		if !r.hookHandlesEvent(hook, event.Type) {
			continue
		}

		if err := hook.Handle(ctx, event); err != nil {
			logger.Error("hook error",
				"event", event.Type,
				"hook", hook.Name(),
				"error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// DispatchAsync dispatches an event asynchronously (fire-and-forget).
// Concurrent async dispatches are bounded to prevent resource exhaustion.
// If the concurrency limit is reached, the dispatch is dropped with a warning.
// Errors are logged but not returned.
func (r *Registry) DispatchAsync(ctx context.Context, event Event) {
	// Initialize semaphore lazily (once)
	r.asyncOnce.Do(func() {
		r.asyncSem = semaphore.NewWeighted(maxConcurrentDispatches)
	})

	logger := slogutil.LoggerFromContext(ctx, slog.Default())

	// Try to acquire a slot without blocking
	if !r.asyncSem.TryAcquire(1) {
		logger.Warn("async dispatch dropped: concurrency limit reached",
			"event", event.Type,
			"limit", maxConcurrentDispatches)
		return
	}

	//nolint:gosec // G118: intentionally using background context for async dispatch to outlive request
	go func() {
		defer r.asyncSem.Release(1)

		// Create a background context since the original may be cancelled
		bgCtx := context.Background()

		// Copy logger from original context if available
		if logger != slog.Default() {
			bgCtx = slogutil.ContextWithLogger(bgCtx, logger)
		}

		_ = r.Dispatch(bgCtx, event)
	}()
}

// hookHandlesEvent checks if a hook handles a specific event type.
func (r *Registry) hookHandlesEvent(hook Hook, eventType EventType) bool {
	for _, e := range hook.Events() {
		if e == eventType {
			return true
		}
	}
	return false
}

// HandlerCount returns the number of handlers registered for an event type.
func (r *Registry) HandlerCount(event EventType) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := len(r.handlers[event])

	// Count compiled hooks that handle this event
	for _, hook := range r.hooks {
		if r.hookHandlesEvent(hook, event) {
			count++
		}
	}

	return count
}

// HookCount returns the total number of compiled hooks registered.
func (r *Registry) HookCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hooks)
}
