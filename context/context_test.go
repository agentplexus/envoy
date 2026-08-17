package context

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/plexusone/omnillm/provider"
)

func TestEngine_ApplyMessageWindow(t *testing.T) {
	engine := New(Config{
		MaxMessages: 5,
	})

	// Create 10 messages
	messages := make([]provider.Message, 10)
	for i := 0; i < 10; i++ {
		messages[i] = provider.Message{
			Role:    provider.RoleUser,
			Content: string(rune('A' + i)),
		}
	}

	result, _ := engine.Apply(context.Background(), messages)

	if len(result) != 5 {
		t.Errorf("Apply() returned %d messages, want 5", len(result))
	}

	// Should have messages F, G, H, I, J (last 5)
	if result[0].Content != "F" {
		t.Errorf("first message = %q, want F", result[0].Content)
	}
	if result[4].Content != "J" {
		t.Errorf("last message = %q, want J", result[4].Content)
	}
}

func TestEngine_PreservesSystemMessage(t *testing.T) {
	engine := New(Config{
		MaxMessages: 3,
	})

	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: "System prompt"},
		{Role: provider.RoleUser, Content: "A"},
		{Role: provider.RoleAssistant, Content: "B"},
		{Role: provider.RoleUser, Content: "C"},
		{Role: provider.RoleAssistant, Content: "D"},
		{Role: provider.RoleUser, Content: "E"},
	}

	result, _ := engine.Apply(context.Background(), messages)

	if len(result) != 3 {
		t.Errorf("Apply() returned %d messages, want 3", len(result))
	}

	// Should have: System, D, E
	if result[0].Role != provider.RoleSystem {
		t.Error("first message should be system")
	}
	if result[0].Content != "System prompt" {
		t.Errorf("system message = %q, want 'System prompt'", result[0].Content)
	}
}

func TestEngine_NoLimitWhenUnderMax(t *testing.T) {
	engine := New(Config{
		MaxMessages: 10,
	})

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "A"},
		{Role: provider.RoleAssistant, Content: "B"},
		{Role: provider.RoleUser, Content: "C"},
	}

	result, _ := engine.Apply(context.Background(), messages)

	if len(result) != 3 {
		t.Errorf("Apply() returned %d messages, want 3", len(result))
	}
}

func TestSimpleTokenCounter(t *testing.T) {
	counter := &SimpleTokenCounter{CharsPerToken: 4}

	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"Hi", 1},          // 2 chars -> 1 token
		{"Hello", 2},       // 5 chars -> 2 tokens
		{"Hello World", 3}, // 11 chars -> 3 tokens
	}

	for _, tc := range tests {
		got := counter.CountText(tc.text)
		if got != tc.want {
			t.Errorf("CountText(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

func TestSimpleTokenCounter_Message(t *testing.T) {
	counter := &SimpleTokenCounter{CharsPerToken: 4}

	msg := provider.Message{
		Role:    provider.RoleUser,
		Content: "Hello World", // ~3 tokens
	}

	tokens := counter.Count(msg)

	// 3 tokens for content + 4 overhead = 7
	if tokens != 7 {
		t.Errorf("Count() = %d, want 7", tokens)
	}
}

func TestEngine_TokenWindow(t *testing.T) {
	engine := New(Config{
		MaxTokens:     100,
		ReserveTokens: 20,
		TokenCounter:  &SimpleTokenCounter{CharsPerToken: 1}, // 1:1 for easy testing
	})

	// Create messages that total ~200 tokens
	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: "System"},        // 6 + 4 = 10 tokens
		{Role: provider.RoleUser, Content: "AAAAAAAAAA"},      // 10 + 4 = 14
		{Role: provider.RoleAssistant, Content: "BBBBBBBBBB"}, // 10 + 4 = 14
		{Role: provider.RoleUser, Content: "CCCCCCCCCC"},      // 10 + 4 = 14
		{Role: provider.RoleAssistant, Content: "DDDDDDDDDD"}, // 10 + 4 = 14
		{Role: provider.RoleUser, Content: "EEEEEEEEEE"},      // 10 + 4 = 14
	}

	result, _ := engine.Apply(context.Background(), messages)

	// Budget is 100 - 20 = 80 tokens
	// System (10) + some messages should fit

	totalTokens := engine.EstimateTokens(result)
	if totalTokens > 80 {
		t.Errorf("result tokens = %d, should be <= 80", totalTokens)
	}

	// Should still have system message
	if result[0].Role != provider.RoleSystem {
		t.Error("should preserve system message")
	}
}

func TestTokenBudget(t *testing.T) {
	budget := TokenBudget{
		Total:    1000,
		Reserved: 200,
		Used:     0,
	}

	if budget.Available() != 800 {
		t.Errorf("Available() = %d, want 800", budget.Available())
	}

	budget.Consume(300)
	if budget.Available() != 500 {
		t.Errorf("Available() = %d, want 500", budget.Available())
	}

	if budget.OverBudget() {
		t.Error("should not be over budget")
	}

	budget.Consume(600)
	if !budget.OverBudget() {
		t.Error("should be over budget")
	}

	budget.Reset()
	if budget.Used != 0 {
		t.Errorf("Used after reset = %d, want 0", budget.Used)
	}
}

func TestWindow_ApplyRecent(t *testing.T) {
	window := NewWindow(WindowConfig{
		Strategy:    WindowStrategyRecent,
		MaxMessages: 4,
	})

	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: "System"},
		{Role: provider.RoleUser, Content: "A"},
		{Role: provider.RoleAssistant, Content: "B"},
		{Role: provider.RoleUser, Content: "C"},
		{Role: provider.RoleAssistant, Content: "D"},
		{Role: provider.RoleUser, Content: "E"},
		{Role: provider.RoleAssistant, Content: "F"},
	}

	result, err := window.Apply(context.Background(), messages)
	if err != nil {
		t.Errorf("Apply() unexpected error: %v", err)
	}

	if len(result) != 4 {
		t.Errorf("Apply() returned %d messages, want 4", len(result))
	}

	// Should have: System, D, E, F
	if result[0].Content != "System" {
		t.Errorf("first message = %q, want System", result[0].Content)
	}
	if result[3].Content != "F" {
		t.Errorf("last message = %q, want F", result[3].Content)
	}
}

func TestExtractPairs(t *testing.T) {
	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: "System"},
		{Role: provider.RoleUser, Content: "Q1"},
		{Role: provider.RoleAssistant, Content: "A1"},
		{Role: provider.RoleUser, Content: "Q2"},
		{Role: provider.RoleAssistant, Content: "A2"},
		{Role: provider.RoleUser, Content: "Q3"}, // No response yet
	}

	pairs := ExtractPairs(messages)

	if len(pairs) != 2 {
		t.Errorf("ExtractPairs() returned %d pairs, want 2", len(pairs))
	}

	if pairs[0].User.Content != "Q1" || pairs[0].Assistant.Content != "A1" {
		t.Error("first pair mismatch")
	}

	if pairs[1].User.Content != "Q2" || pairs[1].Assistant.Content != "A2" {
		t.Error("second pair mismatch")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxMessages != 100 {
		t.Errorf("MaxMessages = %d, want 100", cfg.MaxMessages)
	}
	if cfg.MaxTokens != 0 {
		t.Errorf("MaxTokens = %d, want 0", cfg.MaxTokens)
	}
	if cfg.ReserveTokens != 4096 {
		t.Errorf("ReserveTokens = %d, want 4096", cfg.ReserveTokens)
	}
	if cfg.CompactionEnabled {
		t.Error("CompactionEnabled should be false by default")
	}
	if cfg.CompactionThreshold != 50 {
		t.Errorf("CompactionThreshold = %d, want 50", cfg.CompactionThreshold)
	}
	if cfg.TokenCounter == nil {
		t.Error("TokenCounter should not be nil")
	}
}

func TestEngine_AvailableTokens(t *testing.T) {
	engine := New(Config{
		MaxTokens:     1000,
		ReserveTokens: 200,
		TokenCounter:  &SimpleTokenCounter{CharsPerToken: 1},
	})

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Hello"},    // 5 + 4 = 9 tokens
		{Role: provider.RoleAssistant, Content: "Hi!"}, // 3 + 4 = 7 tokens
	}

	available := engine.AvailableTokens(messages)
	// Budget = 1000 - 200 = 800
	// Used = 16
	// Available = 800 - 16 = 784
	if available != 784 {
		t.Errorf("AvailableTokens() = %d, want 784", available)
	}
}

func TestEngine_AvailableTokens_Unlimited(t *testing.T) {
	engine := New(Config{
		MaxTokens: 0, // Unlimited
	})

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Hello"},
	}

	available := engine.AvailableTokens(messages)
	if available != -1 {
		t.Errorf("AvailableTokens() = %d, want -1 for unlimited", available)
	}
}

func TestEngine_AvailableTokens_Negative(t *testing.T) {
	engine := New(Config{
		MaxTokens:     100,
		ReserveTokens: 50,
		TokenCounter:  &SimpleTokenCounter{CharsPerToken: 1},
	})

	// Create messages that exceed the budget
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: string(make([]byte, 200))}, // 200+ tokens
	}

	available := engine.AvailableTokens(messages)
	if available != 0 {
		t.Errorf("AvailableTokens() = %d, want 0 when over budget", available)
	}
}

func TestEngine_ApplyTokenWindow_ZeroBudget(t *testing.T) {
	engine := New(Config{
		MaxTokens:     100,
		ReserveTokens: 100, // Budget = 0
		TokenCounter:  &SimpleTokenCounter{CharsPerToken: 1},
	})

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Hello"},
	}

	result, _ := engine.Apply(context.Background(), messages)

	// With zero budget, should return original messages (no trimming)
	if len(result) != 1 {
		t.Errorf("Apply() returned %d messages, want 1", len(result))
	}
}

func TestEngine_ApplyTokenWindow_OverBudget(t *testing.T) {
	engine := New(Config{
		MaxTokens:     50,
		ReserveTokens: 10,
		TokenCounter:  &SimpleTokenCounter{CharsPerToken: 1},
	})

	// Create many messages that exceed the budget
	messages := make([]provider.Message, 10)
	for i := 0; i < 10; i++ {
		messages[i] = provider.Message{
			Role:    provider.RoleUser,
			Content: "AAAAAAAAAA", // 10 chars + 4 overhead = 14 tokens each
		}
	}

	result, _ := engine.Apply(context.Background(), messages)

	// Should have fewer messages due to token limit
	if len(result) >= len(messages) {
		t.Errorf("Apply() should trim messages, got %d, original %d", len(result), len(messages))
	}

	// Verify token count is within budget
	totalTokens := engine.EstimateTokens(result)
	budget := 50 - 10
	if totalTokens > budget {
		t.Errorf("result tokens = %d, should be <= %d", totalTokens, budget)
	}
}

func TestEngine_ApplyEmptyMessages(t *testing.T) {
	engine := New(Config{
		MaxMessages: 10,
	})

	var messages []provider.Message
	result, _ := engine.Apply(context.Background(), messages)

	if len(result) != 0 {
		t.Errorf("Apply() on empty should return empty, got %d", len(result))
	}
}

func TestEngine_Compaction_TriggersOverThreshold(t *testing.T) {
	engine := New(Config{
		CompactionEnabled:   true,
		CompactionThreshold: 10,
		Summarizer: func(_ context.Context, messages []provider.Message) (string, error) {
			return fmt.Sprintf("summary of %d messages", len(messages)), nil
		},
	})

	messages := summarizeMessagesForTest(14) // System + 14 = 15, over threshold
	result, err := engine.Apply(context.Background(), messages)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}

	// Compaction ran (System + summary + 5 kept = 7), and since
	// MaxMessages/MaxTokens are unset here, nothing further trims it.
	if len(result) != 7 {
		t.Fatalf("Apply() returned %d messages, want 7 (compacted)", len(result))
	}
	if !strings.Contains(result[1].Content, "summary of 9 messages") {
		t.Errorf("summary message = %+v, want it to reference the summarizer's output", result[1])
	}
}

func TestEngine_Compaction_DisabledByDefault(t *testing.T) {
	called := false
	engine := New(Config{
		Summarizer: func(context.Context, []provider.Message) (string, error) {
			called = true
			return "x", nil
		},
	})
	// CompactionEnabled defaults to false — Summarizer alone must not
	// trigger compaction.

	messages := summarizeMessagesForTest(14)
	if _, err := engine.Apply(context.Background(), messages); err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if called {
		t.Error("summarizer should not be called when CompactionEnabled is false")
	}
}

func TestEngine_Compaction_UnderThresholdNoOp(t *testing.T) {
	called := false
	engine := New(Config{})
	engine.EnableCompaction(20, func(context.Context, []provider.Message) (string, error) {
		called = true
		return "x", nil
	})

	messages := summarizeMessagesForTest(5) // well under threshold
	result, err := engine.Apply(context.Background(), messages)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if called {
		t.Error("summarizer should not be called under threshold")
	}
	if len(result) != len(messages) {
		t.Errorf("Apply() returned %d messages, want %d (unchanged)", len(result), len(messages))
	}
}

func TestEngine_Compaction_ComposesWithMessageWindow(t *testing.T) {
	engine := New(Config{MaxMessages: 5})
	engine.EnableCompaction(10, func(_ context.Context, messages []provider.Message) (string, error) {
		return "summary", nil
	})

	messages := summarizeMessagesForTest(14) // System + 14 = 15
	result, err := engine.Apply(context.Background(), messages)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	// Compaction first reduces 15 -> 7 (System + summary + 5 kept), then
	// MaxMessages=5 windowing trims that further.
	if len(result) != 5 {
		t.Fatalf("Apply() returned %d messages, want 5 (windowed after compaction)", len(result))
	}
	if result[0].Role != provider.RoleSystem {
		t.Error("should preserve system message through both steps")
	}
}

func TestEngine_Compaction_ErrorStillReturnsUsableMessages(t *testing.T) {
	engine := New(Config{})
	engine.EnableCompaction(10, func(context.Context, []provider.Message) (string, error) {
		return "", errors.New("llm down")
	})

	messages := summarizeMessagesForTest(14)
	result, err := engine.Apply(context.Background(), messages)

	if !errors.Is(err, ErrCompactionFailed) {
		t.Errorf("Apply() error = %v, want a CompactionError", err)
	}
	if len(result) == 0 {
		t.Error("Apply() should still return usable messages on compaction failure")
	}
}

func TestEngine_EnableCompaction(t *testing.T) {
	engine := New(DefaultConfig())
	if engine.config.CompactionEnabled {
		t.Fatal("CompactionEnabled should start false")
	}

	fn := func(context.Context, []provider.Message) (string, error) { return "", nil }
	engine.EnableCompaction(42, fn)

	if !engine.config.CompactionEnabled {
		t.Error("EnableCompaction should set CompactionEnabled")
	}
	if engine.config.CompactionThreshold != 42 {
		t.Errorf("CompactionThreshold = %d, want 42", engine.config.CompactionThreshold)
	}
	if engine.config.Summarizer == nil {
		t.Error("EnableCompaction should set Summarizer")
	}
}

func TestEngine_NilTokenCounter(t *testing.T) {
	engine := New(Config{
		MaxTokens:    100,
		TokenCounter: nil, // Should default to SimpleTokenCounter
	})

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Hello"},
	}

	// Should not panic
	tokens := engine.EstimateTokens(messages)
	if tokens <= 0 {
		t.Error("EstimateTokens should return positive value")
	}
}

func TestModelTokenCounter(t *testing.T) {
	// Test with fallback
	fallback := &SimpleTokenCounter{CharsPerToken: 4}
	counter := &ModelTokenCounter{
		Model:    "gpt-4",
		Fallback: fallback,
	}

	msg := provider.Message{
		Role:    provider.RoleUser,
		Content: "Hello World",
	}

	tokens := counter.Count(msg)
	expected := fallback.Count(msg)
	if tokens != expected {
		t.Errorf("Count() = %d, want %d (fallback)", tokens, expected)
	}

	textTokens := counter.CountText("Hello World")
	expectedText := fallback.CountText("Hello World")
	if textTokens != expectedText {
		t.Errorf("CountText() = %d, want %d (fallback)", textTokens, expectedText)
	}
}

func TestModelTokenCounter_NoFallback(t *testing.T) {
	counter := &ModelTokenCounter{
		Model:    "gpt-4",
		Fallback: nil,
	}

	msg := provider.Message{
		Role:    provider.RoleUser,
		Content: "Hello World",
	}

	// Should use default SimpleTokenCounter
	tokens := counter.Count(msg)
	if tokens <= 0 {
		t.Error("Count() should return positive value")
	}

	textTokens := counter.CountText("Hello World")
	if textTokens <= 0 {
		t.Error("CountText() should return positive value")
	}
}

func TestSimpleTokenCounter_WithToolCalls(t *testing.T) {
	counter := &SimpleTokenCounter{CharsPerToken: 4}

	msg := provider.Message{
		Role:    provider.RoleAssistant,
		Content: "Let me check",
		ToolCalls: []provider.ToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: provider.ToolFunction{
					Name:      "get_weather",
					Arguments: `{"location": "NYC"}`,
				},
			},
		},
	}

	tokens := counter.Count(msg)

	// Content: 12 chars -> 3 tokens
	// Overhead: 4 tokens
	// Tool name: 11 chars -> 3 tokens
	// Tool args: 18 chars -> 5 tokens
	// Tool overhead: 4 tokens
	// Total: 3 + 4 + 3 + 5 + 4 = 19
	if tokens != 19 {
		t.Errorf("Count() with tool calls = %d, want 19", tokens)
	}
}

// summarizeMessagesForTest builds System + N numbered user/assistant
// messages, for tests exercising applySummarize's split logic (which
// floors recentCount at 5, so small message sets never trigger real
// summarization — see TestWindow_ApplySummarize_NotEnoughOlderMessages).
func summarizeMessagesForTest(n int) []provider.Message {
	messages := []provider.Message{{Role: provider.RoleSystem, Content: "System"}}
	for i := 0; i < n; i++ {
		messages = append(messages, provider.Message{
			Role:    provider.RoleUser,
			Content: fmt.Sprintf("msg-%d", i),
		})
	}
	return messages
}

func TestWindow_ApplySummarize_Success(t *testing.T) {
	var gotMessages []provider.Message
	window := NewWindow(WindowConfig{
		Strategy:    WindowStrategySummarize,
		MaxMessages: 10,
		Summarizer: func(_ context.Context, messages []provider.Message) (string, error) {
			gotMessages = messages
			return "the gist of it", nil
		},
	})

	messages := summarizeMessagesForTest(14) // System + 14 = 15 total
	result, err := window.Apply(context.Background(), messages)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}

	// recentCount = max(10/2, 5) = 5; startIdx = 1 (system); splitIdx =
	// 15 - 5 = 10. Summarized: messages[1:10] (9 messages). Kept
	// verbatim: messages[10:15] (5 messages).
	if len(gotMessages) != 9 {
		t.Fatalf("summarizer called with %d messages, want 9", len(gotMessages))
	}
	if gotMessages[0].Content != "msg-0" || gotMessages[8].Content != "msg-8" {
		t.Errorf("summarizer received wrong slice: first=%q last=%q", gotMessages[0].Content, gotMessages[8].Content)
	}

	// result = [system, summary, msg-9..msg-13] = 7 messages
	if len(result) != 7 {
		t.Fatalf("Apply() returned %d messages, want 7", len(result))
	}
	if result[0].Role != provider.RoleSystem || result[0].Content != "System" {
		t.Errorf("first message = %+v, want the original system message", result[0])
	}
	if result[1].Role != provider.RoleSystem || !strings.Contains(result[1].Content, "the gist of it") {
		t.Errorf("summary message = %+v, want a system message containing the summary", result[1])
	}
	if result[2].Content != "msg-9" || result[6].Content != "msg-13" {
		t.Errorf("kept messages = %q..%q, want msg-9..msg-13", result[2].Content, result[6].Content)
	}
}

func TestWindow_ApplySummarize_SummarizerError(t *testing.T) {
	window := NewWindow(WindowConfig{
		Strategy:    WindowStrategySummarize,
		MaxMessages: 10,
		Summarizer: func(context.Context, []provider.Message) (string, error) {
			return "", errors.New("llm unavailable")
		},
	})

	messages := summarizeMessagesForTest(14)
	result, err := window.Apply(context.Background(), messages)

	if !errors.Is(err, ErrCompactionFailed) {
		t.Errorf("Apply() error = %v, want a CompactionError", err)
	}
	var ce *CompactionError
	if !errors.As(err, &ce) || ce.Cause == nil || ce.Cause.Error() != "llm unavailable" {
		t.Errorf("Apply() error = %v, want CompactionError wrapping the summarizer's error", err)
	}
	// Falls back to plain recency windowing — still fully usable.
	if len(result) != 10 {
		t.Errorf("Apply() returned %d messages, want 10 (recency fallback)", len(result))
	}
}

func TestWindow_ApplySummarize_NoSummarizerConfigured(t *testing.T) {
	window := NewWindow(WindowConfig{
		Strategy:    WindowStrategySummarize,
		MaxMessages: 3,
	})

	messages := summarizeMessagesForTest(5) // System + 5 = 6 total
	result, err := window.Apply(context.Background(), messages)

	if !errors.Is(err, ErrCompactionFailed) {
		t.Errorf("Apply() error = %v, want a CompactionError", err)
	}
	if len(result) != 3 {
		t.Errorf("Apply() returned %d messages, want 3 (recency fallback)", len(result))
	}
}

func TestWindow_ApplySummarize_NotEnoughOlderMessages(t *testing.T) {
	// 6 messages total; applySummarize's recentCount floors at 5, so
	// there's only 1 "older" message — not worth summarizing. No error:
	// this is a legitimate no-op, not a failure.
	called := false
	window := NewWindow(WindowConfig{
		Strategy:    WindowStrategySummarize,
		MaxMessages: 3,
		Summarizer: func(context.Context, []provider.Message) (string, error) {
			called = true
			return "should not be called", nil
		},
	})

	messages := summarizeMessagesForTest(5) // System + 5 = 6 total
	result, err := window.Apply(context.Background(), messages)

	if err != nil {
		t.Errorf("Apply() error = %v, want nil", err)
	}
	if called {
		t.Error("summarizer should not have been called")
	}
	if len(result) != 3 {
		t.Errorf("Apply() returned %d messages, want 3 (recency fallback)", len(result))
	}
}

func TestWindow_ApplyImportant(t *testing.T) {
	window := NewWindow(WindowConfig{
		Strategy:    WindowStrategyImportant,
		MaxMessages: 6,
	})

	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: "System"},
		{Role: provider.RoleUser, Content: "Q1"},
		{Role: provider.RoleAssistant, Content: "A1"},
		{Role: provider.RoleUser, Content: "Q2"},
		{Role: provider.RoleAssistant, Content: "A2", ToolCalls: []provider.ToolCall{
			{ID: "1", Function: provider.ToolFunction{Name: "test"}},
		}},
		{Role: provider.RoleUser, Content: "Q3"},
		{Role: provider.RoleAssistant, Content: "A3"},
		{Role: provider.RoleUser, Content: "Q4"},
		{Role: provider.RoleAssistant, Content: "A4"},
		{Role: provider.RoleUser, Content: "Q5"},
	}

	result, err := window.Apply(context.Background(), messages)
	if err != nil {
		t.Errorf("Apply() unexpected error: %v", err)
	}

	if len(result) > 6 {
		t.Errorf("Apply() returned %d messages, want <= 6", len(result))
	}

	// Should preserve system message
	if result[0].Role != provider.RoleSystem {
		t.Error("should preserve system message")
	}
}

func TestWindow_ApplyImportant_SmallWindow(t *testing.T) {
	window := NewWindow(WindowConfig{
		Strategy:    WindowStrategyImportant,
		MaxMessages: 4,
	})

	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: "System"},
		{Role: provider.RoleUser, Content: "Q1"},
		{Role: provider.RoleAssistant, Content: "A1"},
		{Role: provider.RoleUser, Content: "Q2"},
		{Role: provider.RoleAssistant, Content: "A2"},
		{Role: provider.RoleUser, Content: "Q3"},
		{Role: provider.RoleAssistant, Content: "A3"},
		{Role: provider.RoleUser, Content: "Q4"},
		{Role: provider.RoleAssistant, Content: "A4"},
		{Role: provider.RoleUser, Content: "Q5"},
		{Role: provider.RoleAssistant, Content: "A5"},
		{Role: provider.RoleUser, Content: "Q6"},
	}

	result, err := window.Apply(context.Background(), messages)
	if err != nil {
		t.Errorf("Apply() unexpected error: %v", err)
	}

	if len(result) > 4 {
		t.Errorf("Apply() returned %d messages, want <= 4", len(result))
	}

	// Should still have system message
	if result[0].Role != provider.RoleSystem {
		t.Error("should preserve system message")
	}
}

func TestWindow_NewWindow_Defaults(t *testing.T) {
	window := NewWindow(WindowConfig{
		MaxMessages: 0, // Should default to 100
	})

	if window.maxSize != 100 {
		t.Errorf("maxSize = %d, want 100", window.maxSize)
	}

	if window.counter == nil {
		t.Error("counter should not be nil")
	}
}

func TestWindow_NoTrimNeeded(t *testing.T) {
	window := NewWindow(WindowConfig{
		Strategy:    WindowStrategyRecent,
		MaxMessages: 10,
	})

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "A"},
		{Role: provider.RoleAssistant, Content: "B"},
	}

	result, err := window.Apply(context.Background(), messages)
	if err != nil {
		t.Errorf("Apply() unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Apply() returned %d messages, want 2", len(result))
	}
}

func TestEngine_MessageWindowEdgeCases(t *testing.T) {
	t.Run("maxMessages=1 with system", func(t *testing.T) {
		engine := New(Config{
			MaxMessages: 1,
		})

		messages := []provider.Message{
			{Role: provider.RoleSystem, Content: "System"},
			{Role: provider.RoleUser, Content: "A"},
			{Role: provider.RoleAssistant, Content: "B"},
		}

		result, _ := engine.Apply(context.Background(), messages)

		// Should keep system + at least 1 recent message
		if len(result) < 1 {
			t.Error("should keep at least 1 message")
		}
		if result[0].Role != provider.RoleSystem {
			t.Error("should preserve system message")
		}
	})

	t.Run("no system message", func(t *testing.T) {
		engine := New(Config{
			MaxMessages: 2,
		})

		messages := []provider.Message{
			{Role: provider.RoleUser, Content: "A"},
			{Role: provider.RoleAssistant, Content: "B"},
			{Role: provider.RoleUser, Content: "C"},
			{Role: provider.RoleAssistant, Content: "D"},
		}

		result, _ := engine.Apply(context.Background(), messages)

		if len(result) != 2 {
			t.Errorf("Apply() returned %d messages, want 2", len(result))
		}

		// Should have C, D (last 2)
		if result[0].Content != "C" {
			t.Errorf("first message = %q, want C", result[0].Content)
		}
	})
}

func TestTokenBudget_AvailableNegative(t *testing.T) {
	budget := TokenBudget{
		Total:    100,
		Reserved: 150, // More than total
		Used:     0,
	}

	if budget.Available() != 0 {
		t.Errorf("Available() = %d, want 0 when reserved > total", budget.Available())
	}
}

func TestNewModelTokenCounter_OpenAI(t *testing.T) {
	counter := NewModelTokenCounter("gpt-4")

	// Should have tiktoken encoding loaded
	if counter.encoding == nil {
		t.Skip("tiktoken encoding not available")
	}

	// Test accurate token counting
	text := "Hello, world!"
	tokens := counter.CountText(text)

	// GPT-4 should tokenize "Hello, world!" into roughly 4 tokens
	if tokens < 1 || tokens > 10 {
		t.Errorf("CountText(%q) = %d, expected 1-10 tokens", text, tokens)
	}
}

func TestNewModelTokenCounter_Anthropic(t *testing.T) {
	counter := NewModelTokenCounter("claude-3-opus")

	// Anthropic uses ~3.5 chars per token
	text := "Hello, world!"
	tokens := counter.CountText(text)

	// 13 chars / 3.5 ≈ 4 tokens
	if tokens < 2 || tokens > 8 {
		t.Errorf("CountText(%q) = %d, expected 2-8 tokens for Anthropic", text, tokens)
	}
}

func TestNewModelTokenCounter_Unknown(t *testing.T) {
	counter := NewModelTokenCounter("unknown-model")

	// Should use fallback (SimpleTokenCounter, ~4 chars per token)
	text := "Hello, world!"
	tokens := counter.CountText(text)

	// 13 chars / 4 ≈ 4 tokens
	if tokens < 2 || tokens > 6 {
		t.Errorf("CountText(%q) = %d, expected 2-6 tokens for unknown model", text, tokens)
	}
}

func TestIsOpenAIModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"gpt-4", true},
		{"gpt-4o", true},
		{"gpt-4-turbo", true},
		{"gpt-3.5-turbo", true},
		{"o1", true},
		{"o1-preview", true},
		{"o3", true},
		{"text-davinci-003", true},
		{"davinci", true},
		{"claude-3-opus", false},
		{"llama-3", false},
		{"mistral-7b", false},
	}

	for _, tc := range tests {
		got := isOpenAIModel(tc.model)
		if got != tc.want {
			t.Errorf("isOpenAIModel(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

func TestIsAnthropicModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"claude-3-opus", true},
		{"claude-3-sonnet", true},
		{"claude-2", true},
		{"Claude-3-haiku", true},
		{"gpt-4", false},
		{"llama-3", false},
	}

	for _, tc := range tests {
		got := isAnthropicModel(tc.model)
		if got != tc.want {
			t.Errorf("isAnthropicModel(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

func TestModelTokenCounter_EmptyText(t *testing.T) {
	counter := NewModelTokenCounter("gpt-4")

	tokens := counter.CountText("")
	if tokens != 0 {
		t.Errorf("CountText(\"\") = %d, want 0", tokens)
	}
}

func TestModelTokenCounter_Message(t *testing.T) {
	counter := NewModelTokenCounter("gpt-4")

	msg := provider.Message{
		Role:    provider.RoleUser,
		Content: "Hello",
		ToolCalls: []provider.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: provider.ToolFunction{
					Name:      "test",
					Arguments: "{}",
				},
			},
		},
	}

	tokens := counter.Count(msg)

	// Should include content + overhead + tool call tokens
	if tokens < 5 {
		t.Errorf("Count() = %d, expected at least 5 tokens for message with tool call", tokens)
	}
}
