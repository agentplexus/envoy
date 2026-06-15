// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package profiles

import (
	"context"
	"sync"
	"time"
)

// CommentaryMode controls the verbosity of inter-tool commentary.
type CommentaryMode int

const (
	// CommentaryNone disables all commentary.
	CommentaryNone CommentaryMode = iota

	// CommentaryBrief shows only key decisions and transitions.
	CommentaryBrief

	// CommentaryVerbose shows thinking, reasoning, and detailed summaries.
	CommentaryVerbose
)

// String returns the string representation of the commentary mode.
func (m CommentaryMode) String() string {
	switch m {
	case CommentaryNone:
		return "none"
	case CommentaryBrief:
		return "brief"
	case CommentaryVerbose:
		return "verbose"
	default:
		return "none"
	}
}

// ParseCommentaryMode parses a string into a CommentaryMode.
func ParseCommentaryMode(s string) CommentaryMode {
	switch s {
	case "none", "":
		return CommentaryNone
	case "brief":
		return CommentaryBrief
	case "verbose", "full":
		return CommentaryVerbose
	default:
		return CommentaryNone
	}
}

// CommentaryType identifies the type of commentary.
type CommentaryType string

const (
	// CommentaryThinking represents internal reasoning/thinking.
	CommentaryThinking CommentaryType = "thinking"

	// CommentaryProgress represents progress updates.
	CommentaryProgress CommentaryType = "progress"

	// CommentaryToolSummary represents a summary of tool execution.
	CommentaryToolSummary CommentaryType = "tool_summary"

	// CommentaryDecision represents a decision point.
	CommentaryDecision CommentaryType = "decision"

	// CommentaryTransition represents a transition between steps.
	CommentaryTransition CommentaryType = "transition"
)

// Commentary represents a single commentary entry.
type Commentary struct {
	// ID is a unique identifier for this commentary.
	ID string `json:"id"`

	// Type identifies the type of commentary.
	Type CommentaryType `json:"type"`

	// Content is the commentary text.
	Content string `json:"content"`

	// ToolName is the associated tool name (for tool-related commentary).
	ToolName string `json:"tool_name,omitempty"`

	// Timestamp is when the commentary was created.
	Timestamp time.Time `json:"timestamp"`

	// Metadata contains additional key-value data.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// CommentaryEmitter manages emission of commentary during agent execution.
type CommentaryEmitter struct {
	mu       sync.Mutex
	mode     CommentaryMode
	buffer   []*Commentary
	dispatch func(*Commentary)
	counter  int64
}

// CommentaryCallback is called when commentary is emitted.
type CommentaryCallback func(*Commentary)

// NewCommentaryEmitter creates a new commentary emitter.
func NewCommentaryEmitter(mode CommentaryMode) *CommentaryEmitter {
	return &CommentaryEmitter{
		mode:   mode,
		buffer: make([]*Commentary, 0),
	}
}

// SetMode changes the commentary mode.
func (e *CommentaryEmitter) SetMode(mode CommentaryMode) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mode = mode
}

// Mode returns the current commentary mode.
func (e *CommentaryEmitter) Mode() CommentaryMode {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.mode
}

// SetDispatch sets the callback for immediate commentary dispatch.
func (e *CommentaryEmitter) SetDispatch(dispatch CommentaryCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dispatch = dispatch
}

// EmitThinking emits a thinking/reasoning commentary.
func (e *CommentaryEmitter) EmitThinking(ctx context.Context, content string) {
	if e.mode < CommentaryVerbose {
		return
	}
	e.emit(ctx, CommentaryThinking, content, "", nil)
}

// EmitProgress emits a progress commentary.
func (e *CommentaryEmitter) EmitProgress(ctx context.Context, content string) {
	if e.mode < CommentaryBrief {
		return
	}
	e.emit(ctx, CommentaryProgress, content, "", nil)
}

// EmitToolSummary emits a summary of tool execution.
func (e *CommentaryEmitter) EmitToolSummary(ctx context.Context, toolName string, result any) {
	if e.mode < CommentaryBrief {
		return
	}

	content := summarizeToolResult(toolName, result)
	e.emit(ctx, CommentaryToolSummary, content, toolName, nil)
}

// EmitDecision emits a decision point commentary.
func (e *CommentaryEmitter) EmitDecision(ctx context.Context, decision string, options []string) {
	if e.mode < CommentaryBrief {
		return
	}

	metadata := map[string]any{
		"options": options,
	}
	e.emit(ctx, CommentaryDecision, decision, "", metadata)
}

// EmitTransition emits a step transition commentary.
func (e *CommentaryEmitter) EmitTransition(ctx context.Context, from, to string) {
	if e.mode < CommentaryVerbose {
		return
	}

	content := "Transitioning from " + from + " to " + to
	e.emit(ctx, CommentaryTransition, content, "", map[string]any{
		"from": from,
		"to":   to,
	})
}

// emit creates and dispatches a commentary entry.
func (e *CommentaryEmitter) emit(_ context.Context, ctype CommentaryType, content, toolName string, metadata map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.counter++
	c := &Commentary{
		ID:        generateCommentaryID(e.counter),
		Type:      ctype,
		Content:   content,
		ToolName:  toolName,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	e.buffer = append(e.buffer, c)

	// Dispatch immediately if callback is set
	if e.dispatch != nil {
		e.dispatch(c)
	}
}

// Buffer returns all buffered commentary.
func (e *CommentaryEmitter) Buffer() []*Commentary {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]*Commentary, len(e.buffer))
	copy(result, e.buffer)
	return result
}

// Flush clears the commentary buffer and returns its contents.
func (e *CommentaryEmitter) Flush() []*Commentary {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := e.buffer
	e.buffer = make([]*Commentary, 0)
	return result
}

// Clear empties the buffer without returning.
func (e *CommentaryEmitter) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buffer = make([]*Commentary, 0)
}

// generateCommentaryID creates a unique ID for commentary.
func generateCommentaryID(counter int64) string {
	return time.Now().Format("20060102150405") + "-" + string(rune('a'+counter%26))
}

// summarizeToolResult creates a human-readable summary of a tool result.
func summarizeToolResult(toolName string, result any) string {
	switch v := result.(type) {
	case string:
		if len(v) > 100 {
			return toolName + ": " + v[:100] + "..."
		}
		return toolName + ": " + v
	case []byte:
		return toolName + ": received " + formatBytes(int64(len(v)))
	case map[string]any:
		return toolName + ": returned object with " + formatCount(len(v), "field")
	case []any:
		return toolName + ": returned " + formatCount(len(v), "item")
	case nil:
		return toolName + ": completed"
	default:
		return toolName + ": completed with result"
	}
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return formatFloat(float64(bytes)/GB) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/MB) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/KB) + " KB"
	default:
		return formatInt(bytes) + " bytes"
	}
}

// formatCount formats a count with a pluralized noun.
func formatCount(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return formatInt(int64(count)) + " " + noun + "s"
}

// formatFloat formats a float with 1 decimal place.
func formatFloat(f float64) string {
	return formatInt(int64(f*10)/10) + "." + formatInt(int64(f*10)%10)
}

// formatInt formats an integer.
func formatInt(i int64) string {
	return string(rune('0' + i%10))
}
