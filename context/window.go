package context

import (
	"context"

	"github.com/plexusone/omnillm/provider"
)

// SummarizeFunc condenses a batch of messages into a single summary
// string, used by WindowStrategySummarize (RMI-OMNIAGENT-027). Injected
// rather than called directly by this package so context stays free of
// any LLM-client dependency — agent.Agent supplies the real
// implementation, reusing its own client.
type SummarizeFunc func(ctx context.Context, messages []provider.Message) (string, error)

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
	strategy   WindowStrategy
	maxSize    int
	counter    TokenCounter
	summarizer SummarizeFunc
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

	// Summarizer is called for WindowStrategySummarize. If nil, that
	// strategy falls back to WindowStrategyRecent and returns a
	// CompactionError explaining why.
	Summarizer SummarizeFunc
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
		strategy:   config.Strategy,
		maxSize:    maxSize,
		counter:    counter,
		summarizer: config.Summarizer,
	}
}

// Apply applies the windowing strategy to messages.
// Returns the windowed messages and any error encountered.
// For WindowStrategySummarize, a CompactionError is returned if summarization fails.
func (w *Window) Apply(ctx context.Context, messages []provider.Message) ([]provider.Message, error) {
	if len(messages) <= w.maxSize {
		return messages, nil
	}

	switch w.strategy {
	case WindowStrategySummarize:
		return w.applySummarize(ctx, messages)
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

// applySummarize condenses the older portion of messages into a single
// summary message via the configured Summarizer, keeping the most recent
// messages verbatim. Falls back to applyRecent and returns a
// CompactionError if no Summarizer is configured or the call fails —
// callers always get a usable message list back, even on error.
func (w *Window) applySummarize(ctx context.Context, messages []provider.Message) ([]provider.Message, error) {
	if w.summarizer == nil {
		return w.applyRecent(messages), NewCompactionError("no summarizer configured", nil)
	}

	hasSystem := len(messages) > 0 && messages[0].Role == provider.RoleSystem

	recentCount := w.maxSize / 2
	if recentCount < 5 {
		recentCount = 5
	}

	startIdx := 0
	if hasSystem {
		startIdx = 1
	}

	splitIdx := len(messages) - recentCount
	if splitIdx <= startIdx {
		// Not enough older messages to be worth summarizing.
		return w.applyRecent(messages), nil
	}

	summary, err := w.summarizer(ctx, messages[startIdx:splitIdx])
	if err != nil {
		return w.applyRecent(messages), NewCompactionError("summarization failed", err)
	}

	result := make([]provider.Message, 0, len(messages)-splitIdx+startIdx+1)
	if hasSystem {
		result = append(result, messages[0])
	}
	result = append(result, provider.Message{
		Role:    provider.RoleSystem,
		Content: "Summary of earlier conversation:\n" + summary,
	})
	result = append(result, messages[splitIdx:]...)
	return result, nil
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
