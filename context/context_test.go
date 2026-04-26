package context

import (
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

	result := engine.Apply(messages)

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

	result := engine.Apply(messages)

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

	result := engine.Apply(messages)

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

	result := engine.Apply(messages)

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

	result := window.Apply(messages)

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

	result := engine.Apply(messages)

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

	result := engine.Apply(messages)

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
	result := engine.Apply(messages)

	if len(result) != 0 {
		t.Errorf("Apply() on empty should return empty, got %d", len(result))
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

func TestWindow_ApplySummarize(t *testing.T) {
	window := NewWindow(WindowConfig{
		Strategy:    WindowStrategySummarize,
		MaxMessages: 3,
	})

	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: "System"},
		{Role: provider.RoleUser, Content: "A"},
		{Role: provider.RoleAssistant, Content: "B"},
		{Role: provider.RoleUser, Content: "C"},
		{Role: provider.RoleAssistant, Content: "D"},
		{Role: provider.RoleUser, Content: "E"},
	}

	result := window.Apply(messages)

	// Summarize falls back to recent for now
	if len(result) != 3 {
		t.Errorf("Apply() returned %d messages, want 3", len(result))
	}

	// Should have System, D, E (most recent)
	if result[0].Content != "System" {
		t.Errorf("first message = %q, want System", result[0].Content)
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

	result := window.Apply(messages)

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

	result := window.Apply(messages)

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

	result := window.Apply(messages)

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

		result := engine.Apply(messages)

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

		result := engine.Apply(messages)

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
