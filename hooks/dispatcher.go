package hooks

import (
	"context"
)

// Dispatcher provides a convenient interface for emitting events.
type Dispatcher struct {
	registry *Registry
}

// NewDispatcher creates a new dispatcher with the given registry.
func NewDispatcher(registry *Registry) *Dispatcher {
	return &Dispatcher{
		registry: registry,
	}
}

// Emit creates and dispatches an event synchronously.
// Returns any error from handler execution.
func (d *Dispatcher) Emit(ctx context.Context, eventType EventType, data any) error {
	if d.registry == nil {
		return nil
	}

	event := NewEvent(eventType, data)
	return d.registry.Dispatch(ctx, event)
}

// EmitWithSession creates and dispatches an event with a session ID.
func (d *Dispatcher) EmitWithSession(ctx context.Context, eventType EventType, sessionID string, data any) error {
	if d.registry == nil {
		return nil
	}

	event := NewEvent(eventType, data).WithSessionID(sessionID)
	return d.registry.Dispatch(ctx, event)
}

// EmitAsync creates and dispatches an event asynchronously.
// Errors are logged but not returned.
func (d *Dispatcher) EmitAsync(ctx context.Context, eventType EventType, data any) {
	if d.registry == nil {
		return
	}

	event := NewEvent(eventType, data)
	d.registry.DispatchAsync(ctx, event)
}

// EmitAsyncWithSession creates and dispatches an event with a session ID asynchronously.
func (d *Dispatcher) EmitAsyncWithSession(ctx context.Context, eventType EventType, sessionID string, data any) {
	if d.registry == nil {
		return
	}

	event := NewEvent(eventType, data).WithSessionID(sessionID)
	d.registry.DispatchAsync(ctx, event)
}
