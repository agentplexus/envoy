// Package context provides context management for conversations.
//
// The context engine handles:
//   - Sliding window to keep conversations within token limits
//   - History compaction to summarize older messages
//   - Token counting for context budget management
package context

import (
	"context"

	"github.com/plexusone/omnillm/provider"
)

// Engine manages conversation context for LLM requests.
type Engine struct {
	config Config
}

// Config configures the context engine.
type Config struct {
	// MaxMessages is the maximum number of messages to include.
	// Zero means no limit.
	MaxMessages int

	// MaxTokens is the maximum token budget for context.
	// Zero means no limit.
	MaxTokens int

	// ReserveTokens is the number of tokens to reserve for the response.
	// This is subtracted from MaxTokens when calculating available context.
	ReserveTokens int

	// CompactionEnabled enables automatic history compaction.
	CompactionEnabled bool

	// CompactionThreshold is the message count that triggers compaction.
	// When messages exceed this, older messages are summarized.
	CompactionThreshold int

	// TokenCounter estimates token count for messages.
	// If nil, a simple character-based estimator is used.
	TokenCounter TokenCounter

	// Summarizer condenses older messages when compaction triggers
	// (RMI-OMNIAGENT-027). Required for CompactionEnabled to have any
	// effect beyond a no-op — see EnableCompaction.
	Summarizer SummarizeFunc
}

// DefaultConfig returns a default configuration.
func DefaultConfig() Config {
	return Config{
		MaxMessages:         100,
		MaxTokens:           0, // No token limit by default
		ReserveTokens:       4096,
		CompactionEnabled:   false,
		CompactionThreshold: 50,
		TokenCounter:        &SimpleTokenCounter{},
	}
}

// New creates a new context engine.
func New(config Config) *Engine {
	if config.TokenCounter == nil {
		config.TokenCounter = &SimpleTokenCounter{}
	}
	return &Engine{config: config}
}

// Apply applies context management to a list of messages.
// It returns a potentially reduced set of messages that fits within
// limits, and any error encountered compacting them (RMI-OMNIAGENT-027).
// The returned messages are always usable even when err is non-nil —
// compaction failures fall back to plain windowing, never break the
// caller's turn.
func (e *Engine) Apply(ctx context.Context, messages []provider.Message) ([]provider.Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	result := messages
	var compactionErr error

	// Compact (summarize) older messages first, if enabled and over
	// threshold, by delegating to a Window configured for
	// WindowStrategySummarize — reusing its split/fallback logic rather
	// than duplicating it here.
	if e.config.CompactionEnabled && e.config.CompactionThreshold > 0 && len(result) > e.config.CompactionThreshold {
		w := NewWindow(WindowConfig{
			Strategy:     WindowStrategySummarize,
			MaxMessages:  e.config.CompactionThreshold,
			TokenCounter: e.config.TokenCounter,
			Summarizer:   e.config.Summarizer,
		})
		compacted, err := w.Apply(ctx, result)
		compactionErr = err
		result = compacted
	}

	// Apply message limit
	if e.config.MaxMessages > 0 && len(result) > e.config.MaxMessages {
		result = e.applyMessageWindow(result)
	}

	// Apply token limit
	if e.config.MaxTokens > 0 {
		result = e.applyTokenWindow(result)
	}

	return result, compactionErr
}

// EnableCompaction turns on LLM-based conversation summarization
// (RMI-OMNIAGENT-027): once the message count exceeds threshold, older
// messages are condensed via fn instead of being dropped outright.
func (e *Engine) EnableCompaction(threshold int, fn SummarizeFunc) {
	e.config.CompactionEnabled = true
	e.config.CompactionThreshold = threshold
	e.config.Summarizer = fn
}

// applyMessageWindow keeps only the most recent messages within the limit.
// It preserves the system message if present.
func (e *Engine) applyMessageWindow(messages []provider.Message) []provider.Message {
	if len(messages) <= e.config.MaxMessages {
		return messages
	}

	// Check if first message is system message
	hasSystem := len(messages) > 0 && messages[0].Role == provider.RoleSystem

	if hasSystem {
		// Keep system + last (MaxMessages-1) messages
		keepCount := e.config.MaxMessages - 1
		if keepCount < 1 {
			keepCount = 1
		}

		startIdx := len(messages) - keepCount
		if startIdx < 1 {
			startIdx = 1
		}

		result := make([]provider.Message, 0, keepCount+1)
		result = append(result, messages[0]) // System message
		result = append(result, messages[startIdx:]...)
		return result
	}

	// No system message, just keep last MaxMessages
	startIdx := len(messages) - e.config.MaxMessages
	return messages[startIdx:]
}

// applyTokenWindow keeps messages within the token budget.
// It preserves the system message and removes oldest messages first.
func (e *Engine) applyTokenWindow(messages []provider.Message) []provider.Message {
	budget := e.config.MaxTokens - e.config.ReserveTokens
	if budget <= 0 {
		return messages
	}

	// Check if first message is system message
	hasSystem := len(messages) > 0 && messages[0].Role == provider.RoleSystem

	// Calculate total tokens
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += e.config.TokenCounter.Count(msg)
	}

	if totalTokens <= budget {
		return messages
	}

	// Need to trim - work backwards from most recent
	result := make([]provider.Message, 0, len(messages))
	currentTokens := 0

	// If we have a system message, account for it first
	systemTokens := 0
	if hasSystem {
		systemTokens = e.config.TokenCounter.Count(messages[0])
		currentTokens = systemTokens
	}

	// Add messages from the end until we hit the budget
	for i := len(messages) - 1; i >= 0; i-- {
		if hasSystem && i == 0 {
			continue // Skip system message, we'll add it at the start
		}

		msgTokens := e.config.TokenCounter.Count(messages[i])
		if currentTokens+msgTokens > budget {
			break
		}

		result = append([]provider.Message{messages[i]}, result...)
		currentTokens += msgTokens
	}

	// Add system message at the start
	if hasSystem {
		result = append([]provider.Message{messages[0]}, result...)
	}

	return result
}

// EstimateTokens returns the estimated token count for a message list.
func (e *Engine) EstimateTokens(messages []provider.Message) int {
	total := 0
	for _, msg := range messages {
		total += e.config.TokenCounter.Count(msg)
	}
	return total
}

// AvailableTokens returns the remaining token budget after accounting for messages.
func (e *Engine) AvailableTokens(messages []provider.Message) int {
	if e.config.MaxTokens == 0 {
		return -1 // Unlimited
	}

	used := e.EstimateTokens(messages)
	available := e.config.MaxTokens - e.config.ReserveTokens - used
	if available < 0 {
		return 0
	}
	return available
}
