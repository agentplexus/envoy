package autoreply

import (
	"context"
	"testing"
	"time"
)

func TestHandler_AddRule(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "test-rule",
		Name:    "Test Rule",
		Enabled: true,
		Conditions: Conditions{
			Patterns: []string{"hello.*world"},
		},
		Response: Response{
			Text: "Hello!",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	rules := h.ListRules()
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestHandler_AddRule_InvalidPattern(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "bad-rule",
		Enabled: true,
		Conditions: Conditions{
			Patterns: []string{"[invalid"},
		},
	}

	err := h.AddRule(rule)
	if err == nil {
		t.Error("expected error for invalid pattern")
	}
}

func TestHandler_RemoveRule(t *testing.T) {
	h := NewHandler()

	rule := &Rule{ID: "test-rule", Enabled: true}
	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	if !h.RemoveRule("test-rule") {
		t.Error("RemoveRule should return true for existing rule")
	}

	if h.RemoveRule("nonexistent") {
		t.Error("RemoveRule should return false for nonexistent rule")
	}

	if len(h.ListRules()) != 0 {
		t.Error("expected 0 rules after removal")
	}
}

func TestHandler_Evaluate_PatternMatch(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "greeting",
		Enabled: true,
		Conditions: Conditions{
			Patterns: []string{"^hello", "^hi"},
		},
		Response: Response{
			Text:           "Hello there!",
			StopProcessing: true,
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	tests := []struct {
		name    string
		content string
		matched bool
	}{
		{"matches hello", "hello world", true},
		{"matches hi", "hi there", true},
		{"no match", "goodbye", false},
		{"case insensitive", "HELLO", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				Content:   tt.content,
				Timestamp: time.Now(),
			}

			result := h.Evaluate(context.Background(), msg)

			if result.Matched != tt.matched {
				t.Errorf("Matched = %v, want %v", result.Matched, tt.matched)
			}

			if tt.matched && result.Response != "Hello there!" {
				t.Errorf("Response = %q, want %q", result.Response, "Hello there!")
			}
		})
	}
}

func TestHandler_Evaluate_KeywordMatch(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "help",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"help", "support"},
		},
		Response: Response{
			Text: "How can I help you?",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	tests := []struct {
		name    string
		content string
		matched bool
	}{
		{"matches help", "I need help", true},
		{"matches support", "contact support please", true},
		{"no match", "thanks", false},
		{"case insensitive", "HELP ME", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				Content:   tt.content,
				Timestamp: time.Now(),
			}

			result := h.Evaluate(context.Background(), msg)

			if result.Matched != tt.matched {
				t.Errorf("Matched = %v, want %v", result.Matched, tt.matched)
			}
		})
	}
}

func TestHandler_Evaluate_ChannelFilter(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "telegram-only",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"hello"},
			Channels: []string{"telegram"},
		},
		Response: Response{
			Text: "Telegram greeting!",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	tests := []struct {
		name    string
		channel string
		matched bool
	}{
		{"telegram matches", "telegram", true},
		{"discord no match", "discord", false},
		{"case insensitive", "Telegram", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				Content:   "hello",
				Channel:   tt.channel,
				Timestamp: time.Now(),
			}

			result := h.Evaluate(context.Background(), msg)

			if result.Matched != tt.matched {
				t.Errorf("Matched = %v, want %v", result.Matched, tt.matched)
			}
		})
	}
}

func TestHandler_Evaluate_DisabledRule(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "disabled",
		Enabled: false,
		Conditions: Conditions{
			Keywords: []string{"test"},
		},
		Response: Response{
			Text: "This should not match",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	msg := &Message{
		Content:   "test message",
		Timestamp: time.Now(),
	}

	result := h.Evaluate(context.Background(), msg)

	if result.Matched {
		t.Error("disabled rule should not match")
	}
}

func TestHandler_Evaluate_Priority(t *testing.T) {
	h := NewHandler()

	// Add rules in reverse priority order
	rule2 := &Rule{
		ID:       "low-priority",
		Enabled:  true,
		Priority: 10,
		Conditions: Conditions{
			Keywords: []string{"test"},
		},
		Response: Response{
			Text: "Low priority response",
		},
	}

	rule1 := &Rule{
		ID:       "high-priority",
		Enabled:  true,
		Priority: 1,
		Conditions: Conditions{
			Keywords: []string{"test"},
		},
		Response: Response{
			Text: "High priority response",
		},
	}

	if err := h.AddRule(rule2); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}
	if err := h.AddRule(rule1); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	msg := &Message{
		Content:   "test",
		Timestamp: time.Now(),
	}

	result := h.Evaluate(context.Background(), msg)

	if !result.Matched {
		t.Fatal("expected match")
	}

	if result.RuleID != "high-priority" {
		t.Errorf("RuleID = %q, want %q", result.RuleID, "high-priority")
	}

	if result.Response != "High priority response" {
		t.Errorf("Response = %q, want %q", result.Response, "High priority response")
	}
}

func TestHandler_RateLimit(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "rate-limited",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"ping"},
		},
		Response: Response{
			Text: "pong",
		},
		RateLimit: &RateLimit{
			MaxCount: 2,
			Window:   time.Second,
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	msg := &Message{
		Content:   "ping",
		Timestamp: time.Now(),
	}

	// First two should match
	for i := 0; i < 2; i++ {
		result := h.Evaluate(context.Background(), msg)
		if !result.Matched {
			t.Errorf("request %d should match", i+1)
		}
	}

	// Third should be rate limited
	result := h.Evaluate(context.Background(), msg)
	if result.Matched {
		t.Error("third request should be rate limited")
	}
}

func TestHandler_TimeRange(t *testing.T) {
	h := NewHandler()

	// Create a time range that is definitely active now
	now := time.Now().UTC()
	startTime := now.Add(-time.Hour).Format("15:04")
	endTime := now.Add(time.Hour).Format("15:04")

	rule := &Rule{
		ID:      "time-based",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"hello"},
			TimeRange: &TimeRange{
				Start:    startTime,
				End:      endTime,
				Timezone: "UTC",
			},
		},
		Response: Response{
			Text: "Hello!",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	msg := &Message{
		Content:   "hello",
		Timestamp: now,
	}

	result := h.Evaluate(context.Background(), msg)
	if !result.Matched {
		t.Error("should match within time range")
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter()

	// Should allow up to maxCount
	for i := 0; i < 3; i++ {
		if !rl.allow("test", 3, time.Second) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// Should deny after maxCount
	if rl.allow("test", 3, time.Second) {
		t.Error("request 4 should be denied")
	}

	// Different key should be independent
	if !rl.allow("other", 3, time.Second) {
		t.Error("different key should be allowed")
	}
}
