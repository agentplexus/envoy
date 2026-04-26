// Package hooks provides event-driven extensibility for omniagent.
//
// The hooks package supports three registration methods:
//
// 1. Quick handlers - WithHook(event, func) for simple function handlers
// 2. Compiled hooks - WithCompiledHook(hook) for complex, reusable hooks
// 3. Webhook handlers - YAML config for HTTP-based external integrations
//
// # Example Usage
//
//	agent, _ := agent.New(config,
//	    agent.WithHook(hooks.EventMessageReceived, func(ctx context.Context, e hooks.Event) error {
//	        msg := e.Data.(hooks.MessageEvent)
//	        log.Printf("Received: %s", msg.Content)
//	        return nil
//	    }),
//	)
package hooks

import (
	"time"
)

// EventType identifies the type of event.
type EventType string

// Event types supported by the hooks system.
const (
	// EventMessageReceived fires when a user message is received.
	EventMessageReceived EventType = "message.received"

	// EventMessageSent fires when an assistant response is sent.
	EventMessageSent EventType = "message.sent"

	// EventToolCalled fires before a tool is executed.
	EventToolCalled EventType = "tool.called"

	// EventToolCompleted fires after a tool execution completes.
	EventToolCompleted EventType = "tool.completed"

	// EventSessionCreated fires when a new session is created.
	EventSessionCreated EventType = "session.created"

	// EventSessionUpdated fires when a session is updated.
	EventSessionUpdated EventType = "session.updated"

	// EventJobExecuted fires after a cron job runs.
	EventJobExecuted EventType = "job.executed"
)

// Event represents an event in the hooks system.
type Event struct {
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id,omitempty"`
	Data      any       `json:"data"`
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType EventType, data any) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// WithSessionID returns a copy of the event with the session ID set.
func (e Event) WithSessionID(sessionID string) Event {
	e.SessionID = sessionID
	return e
}

// MessageEvent is the data for message events.
type MessageEvent struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolEvent is the data for tool events.
type ToolEvent struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params,omitempty"`
	Result string         `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

// SessionEvent is the data for session events.
type SessionEvent struct {
	SessionID string `json:"session_id"`
	Action    string `json:"action"` // created, updated
}

// JobEvent is the data for job events.
type JobEvent struct {
	JobID   string `json:"job_id"`
	JobName string `json:"job_name"`
	Success bool   `json:"success"`
}
