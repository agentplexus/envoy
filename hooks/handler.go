package hooks

import "context"

// HandlerFunc is the signature for quick hook handlers.
// It receives the context and event, and returns an error if handling fails.
type HandlerFunc func(ctx context.Context, event Event) error

// HandlerConfig wraps a handler with configuration.
type HandlerConfig struct {
	// Name is an optional identifier for the handler (used in logging).
	Name string

	// Events lists the event types this handler responds to.
	Events []EventType

	// Handler is the function to call when an event occurs.
	Handler HandlerFunc
}
