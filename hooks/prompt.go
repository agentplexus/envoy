package hooks

import "context"

// PromptTurn describes an upcoming model turn for synchronous pre-turn hooks.
type PromptTurn struct {
	// SessionID is the session the run belongs to (empty when the agent
	// runs without a session).
	SessionID string `json:"session_id,omitempty"`

	// Content is the user message that started the run.
	Content string `json:"content"`

	// Iteration is the zero-based tool-call loop iteration within the run.
	Iteration int `json:"iteration"`

	// Tools is the set of tool names currently available to the turn.
	Tools []string `json:"tools"`
}

// ToolsAllowFunc is a synchronous pre-turn hook that can narrow the tools
// submitted to the model for a single turn. Unlike event hooks — which are
// asynchronous, fire-and-forget observers — its return value is consumed by
// the agent:
//
//   - nil leaves the tool set unchanged
//   - an empty slice removes all optional tools for this turn
//   - a list of names narrows the set to its intersection with Tools
//
// The hook runs on every loop iteration of a run, so narrowing applies
// per-turn and a later turn can widen again. It must not block: it runs on
// the request path before each model call.
type ToolsAllowFunc func(ctx context.Context, turn PromptTurn) []string
