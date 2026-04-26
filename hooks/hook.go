package hooks

import (
	"context"

	"github.com/plexusone/omnistorage"
)

// Hook is the interface for compiled hooks.
// Compiled hooks are complex, reusable event handlers that can maintain
// state and require initialization/cleanup.
//
// # Example Implementation
//
//	type AuditHook struct {
//	    logger *slog.Logger
//	}
//
//	func (h *AuditHook) Name() string { return "audit" }
//
//	func (h *AuditHook) Events() []EventType {
//	    return []EventType{EventMessageReceived, EventMessageSent}
//	}
//
//	func (h *AuditHook) Handle(ctx context.Context, event Event) error {
//	    h.logger.Info("event", "type", event.Type, "data", event.Data)
//	    return nil
//	}
//
//	func (h *AuditHook) Init(ctx context.Context) error { return nil }
//	func (h *AuditHook) Close() error { return nil }
type Hook interface {
	// Name returns the hook identifier.
	Name() string

	// Events returns the event types this hook handles.
	Events() []EventType

	// Handle processes an event.
	Handle(ctx context.Context, event Event) error

	// Init initializes the hook. Called once at startup.
	Init(ctx context.Context) error

	// Close releases resources. Called on shutdown.
	Close() error
}

// StorageAware is an optional interface for hooks that need persistence.
// If a hook implements this interface, the registry will inject the storage
// backend after registration and before Init() is called.
type StorageAware interface {
	SetStorage(s omnistorage.Store)
}
