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

	// EventAgentEnd fires exactly once when an agent run reaches any
	// terminal state: normal completion, error, or abort (context
	// cancellation). Aborted runs still settle through this event.
	EventAgentEnd EventType = "agent.end"

	// EventSessionRollover fires when a session automatically rolls over
	// (idle timeout or daily boundary) — as opposed to a manual clear,
	// which emits no rollover. It carries the ENDED session's transcript
	// so hooks can persist it before the fresh session begins.
	EventSessionRollover EventType = "session.rollover"
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

// SessionRolloverEvent is the data for the session.rollover event: an
// automatic session boundary. Transcript is a snapshot of the ended
// conversation, taken before the session is reset, so handlers can persist
// it without racing the reset.
type SessionRolloverEvent struct {
	SessionID string `json:"session_id"`

	// Reason is the machine-readable rollover cause ("idle" or "daily").
	Reason string `json:"reason"`

	// Transcript holds the ended conversation as role/content pairs.
	Transcript []MessageEvent `json:"transcript"`

	// StartedAt is when the ended conversation began.
	StartedAt time.Time `json:"started_at"`

	// EndedAt is the ended conversation's last activity.
	EndedAt time.Time `json:"ended_at"`
}

// AgentEndEvent is the data for the agent.end lifecycle event.
// Classification: abort outranks error — a run cancelled by the caller
// reports Aborted=true with an empty Error, even when the cancellation
// surfaced as an error from a provider or tool call.
type AgentEndEvent struct {
	SessionID  string `json:"session_id,omitempty"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	Aborted    bool   `json:"aborted"`
	Response   string `json:"response,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}
