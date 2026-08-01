package context

import (
	"github.com/plexusone/omnillm/provider"
)

// WindowStrategy defines how messages are windowed.
type WindowStrategy int

const (
	// WindowStrategyRecent keeps only the most recent messages.
	WindowStrategyRecent WindowStrategy = iota

	// WindowStrategySummarize summarizes older messages before windowing.
	WindowStrategySummarize

	// WindowStrategyImportant keeps important messages (user questions, key responses).
	WindowStrategyImportant
)

// Window manages a sliding window of messages.
type Window struct {
	strategy WindowStrategy
	maxSize  int
	counter  TokenCounter
}

// WindowConfig configures a window.
type WindowConfig struct {
	// Strategy is the windowing strategy.
	Strategy WindowStrategy

	// MaxMessages is the maximum messages to keep.
	MaxMessages int

	// MaxTokens is the maximum tokens to keep.
	MaxTokens int

	// TokenCounter for token estimation.
	TokenCounter TokenCounter
}

// NewWindow creates a new message window.
func NewWindow(config WindowConfig) *Window {
	counter := config.TokenCounter
	if counter == nil {
		counter = &SimpleTokenCounter{}
	}

	maxSize := config.MaxMessages
	if maxSize <= 0 {
		maxSize = 100
	}

	return &Window{
		strategy: config.Strategy,
		maxSize:  maxSize,
		counter:  counter,
	}
}

// Apply applies the windowing strategy to messages.
// Returns the windowed messages and any error encountered.
// For WindowStrategySummarize, a CompactionError is returned if summarization fails.
func (w *Window) Apply(messages []provider.Message) ([]provider.Message, error) {
	if len(messages) <= w.maxSize {
		return messages, nil
	}

	switch w.strategy {
	case WindowStrategySummarize:
		return w.applySummarize(messages)
	case WindowStrategyImportant:
		return w.applyImportant(messages), nil
	default:
		return w.applyRecent(messages), nil
	}
}

// applyRecent keeps only the most recent messages.
func (w *Window) applyRecent(messages []provider.Message) []provider.Message {
	// Check for system message
	hasSystem := len(messages) > 0 && messages[0].Role == provider.RoleSystem

	if hasSystem {
		keepCount := w.maxSize - 1
		if keepCount < 1 {
			keepCount = 1
		}

		startIdx := len(messages) - keepCount
		if startIdx < 1 {
			startIdx = 1
		}

		result := make([]provider.Message, 0, keepCount+1)
		result = append(result, messages[0])
		result = append(result, messages[startIdx:]...)
		return result
	}

	startIdx := len(messages) - w.maxSize
	if startIdx < 0 {
		startIdx = 0
	}
	return messages[startIdx:]
}

// applySummarize attempts to summarize older messages.
// Returns CompactionError if summarization fails (e.g., no summarizer configured).
func (w *Window) applySummarize(messages []provider.Message) ([]provider.Message, error) {
	// TODO: Implement summarization with LLM call
	// For now, return a typed error instead of silently falling back
	return w.applyRecent(messages), NewCompactionError(
		"summarization not implemented",
		nil,
	)
}

// applyImportant keeps messages marked as important.
func (w *Window) applyImportant(messages []provider.Message) []provider.Message {
	// Check for system message
	hasSystem := len(messages) > 0 && messages[0].Role == provider.RoleSystem

	// Identify important messages:
	// - System message
	// - User messages (questions)
	// - Assistant messages with tool calls
	// - Recent messages

	important := make([]provider.Message, 0)
	recent := make([]provider.Message, 0)

	recentCount := w.maxSize / 2
	if recentCount < 5 {
		recentCount = 5
	}

	for i, msg := range messages {
		// Always keep system message
		if hasSystem && i == 0 {
			important = append(important, msg)
			continue
		}

		// Keep recent messages
		if i >= len(messages)-recentCount {
			recent = append(recent, msg)
			continue
		}

		// Keep user messages (questions are important context)
		if msg.Role == provider.RoleUser {
			important = append(important, msg)
			continue
		}

		// Keep assistant messages with tool calls
		if msg.Role == provider.RoleAssistant && len(msg.ToolCalls) > 0 {
			important = append(important, msg)
		}
	}

	// Combine important + recent, respecting max size
	result := append(important, recent...)

	// If still over limit, trim from important (not recent)
	if len(result) > w.maxSize {
		excess := len(result) - w.maxSize
		// Remove from important, keeping system if present
		startTrim := 0
		if hasSystem {
			startTrim = 1
		}

		if excess < len(important)-startTrim {
			important = append(important[:startTrim], important[startTrim+excess:]...)
			result = append(important, recent...)
		} else {
			// Too many excess, just use recent strategy
			return w.applyRecent(messages)
		}
	}

	return result
}

// MessagePair represents a user-assistant exchange.
type MessagePair struct {
	User      provider.Message
	Assistant provider.Message
}

// ExtractPairs extracts user-assistant message pairs from a conversation.
func ExtractPairs(messages []provider.Message) []MessagePair {
	pairs := make([]MessagePair, 0)
	var currentUser *provider.Message

	for i := range messages {
		msg := &messages[i]

		if msg.Role == provider.RoleUser {
			currentUser = msg
		} else if msg.Role == provider.RoleAssistant && currentUser != nil {
			pairs = append(pairs, MessagePair{
				User:      *currentUser,
				Assistant: *msg,
			})
			currentUser = nil
		}
	}

	return pairs
}
