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

func TestHandler_TemplateRendering(t *testing.T) {
	h := NewHandler()

	tests := []struct {
		name     string
		template string
		msg      *Message
		want     string
	}{
		{
			name:     "simple variable",
			template: "Hello, {{.Sender}}!",
			msg: &Message{
				Content:   "hi",
				SenderID:  "user123",
				Channel:   "discord",
				Timestamp: time.Now(),
			},
			want: "Hello, user123!",
		},
		{
			name:     "multiple variables",
			template: "Message from {{.Sender}} on {{.Channel}}: {{.Message}}",
			msg: &Message{
				Content:   "test message",
				SenderID:  "alice",
				Channel:   "telegram",
				Timestamp: time.Now(),
			},
			want: "Message from alice on telegram: test message",
		},
		{
			name:     "time formatting",
			template: "Received at {{.Time.Format \"15:04\"}}",
			msg: &Message{
				Content:   "hi",
				SenderID:  "user",
				Channel:   "slack",
				Timestamp: time.Date(2026, 7, 29, 14, 30, 0, 0, time.UTC),
			},
			want: "Received at 14:30",
		},
		{
			name:     "invalid template falls back to raw",
			template: "Hello, {{.InvalidField}",
			msg: &Message{
				Content:  "hi",
				SenderID: "user",
			},
			want: "Hello, {{.InvalidField}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rule := &Rule{
				ID:      "template-rule",
				Enabled: true,
				Conditions: Conditions{
					Keywords: []string{"hi", "test"},
				},
				Response: Response{
					Template: tc.template,
					Text:     "fallback text",
				},
			}

			// Reset handler for each test
			h = NewHandler()
			if err := h.AddRule(rule); err != nil {
				t.Fatalf("AddRule failed: %v", err)
			}

			result := h.Evaluate(context.Background(), tc.msg)

			if !result.Matched {
				t.Fatal("rule should match")
			}

			if result.Response != tc.want {
				t.Errorf("Response = %q, want %q", result.Response, tc.want)
			}
		})
	}
}

func TestHandler_TemplateWithMetadata(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "metadata-rule",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"help"},
		},
		Response: Response{
			Template: "Your ticket ID is {{index .Metadata \"ticket_id\"}}",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	msg := &Message{
		Content:   "I need help",
		SenderID:  "user",
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"ticket_id": "TICKET-123",
		},
	}

	result := h.Evaluate(context.Background(), msg)

	if !result.Matched {
		t.Fatal("rule should match")
	}

	want := "Your ticket ID is TICKET-123"
	if result.Response != want {
		t.Errorf("Response = %q, want %q", result.Response, want)
	}
}

func TestHandler_GetRule(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "test-rule",
		Name:    "Test Rule",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"test"},
		},
		Response: Response{
			Text: "Found!",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// Get existing rule
	found, ok := h.GetRule("test-rule")
	if !ok {
		t.Fatal("expected to find rule")
	}
	if found.Name != "Test Rule" {
		t.Errorf("Name = %q, want 'Test Rule'", found.Name)
	}

	// Get non-existent rule
	_, ok = h.GetRule("non-existent")
	if ok {
		t.Error("expected not to find non-existent rule")
	}
}

func TestHandler_SenderFilter(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "sender-filter",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"hello"},
			Senders:  []string{"allowed-user"},
		},
		Response: Response{
			Text: "Hello allowed user!",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// Allowed sender
	msg1 := &Message{
		Content:   "hello",
		SenderID:  "allowed-user",
		Timestamp: time.Now(),
	}
	result1 := h.Evaluate(context.Background(), msg1)
	if !result1.Matched {
		t.Error("should match for allowed sender")
	}

	// Not allowed sender
	msg2 := &Message{
		Content:   "hello",
		SenderID:  "other-user",
		Timestamp: time.Now(),
	}
	result2 := h.Evaluate(context.Background(), msg2)
	if result2.Matched {
		t.Error("should not match for non-allowed sender")
	}
}

func TestHandler_PatternAndKeyword(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "combo-rule",
		Enabled: true,
		Conditions: Conditions{
			Patterns: []string{"urgent"},
			Keywords: []string{"help"},
		},
		Response: Response{
			Text: "Urgent help needed!",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// Both pattern and keyword present
	msg1 := &Message{
		Content:   "This is urgent, I need help!",
		Timestamp: time.Now(),
	}
	result1 := h.Evaluate(context.Background(), msg1)
	if !result1.Matched {
		t.Error("should match when both pattern and keyword present")
	}

	// Only pattern
	msg2 := &Message{
		Content:   "This is urgent!",
		Timestamp: time.Now(),
	}
	result2 := h.Evaluate(context.Background(), msg2)
	if result2.Matched {
		t.Error("should not match when only pattern present")
	}

	// Only keyword
	msg3 := &Message{
		Content:   "I need help",
		Timestamp: time.Now(),
	}
	result3 := h.Evaluate(context.Background(), msg3)
	if result3.Matched {
		t.Error("should not match when only keyword present")
	}
}

func TestHandler_TimeRange_OvernightRange(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "overnight-rule",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"hello"},
			TimeRange: &TimeRange{
				Start:    "22:00",
				End:      "06:00", // Overnight range
				Timezone: "UTC",
			},
		},
		Response: Response{
			Text: "Good night!",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// 23:00 should match (after start)
	msg1 := &Message{
		Content:   "hello",
		Timestamp: time.Date(2026, 7, 29, 23, 0, 0, 0, time.UTC),
	}
	result1 := h.Evaluate(context.Background(), msg1)
	if !result1.Matched {
		t.Error("should match at 23:00")
	}

	// 03:00 should match (before end)
	msg2 := &Message{
		Content:   "hello",
		Timestamp: time.Date(2026, 7, 29, 3, 0, 0, 0, time.UTC),
	}
	result2 := h.Evaluate(context.Background(), msg2)
	if !result2.Matched {
		t.Error("should match at 03:00")
	}

	// 12:00 should not match
	msg3 := &Message{
		Content:   "hello",
		Timestamp: time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
	}
	result3 := h.Evaluate(context.Background(), msg3)
	if result3.Matched {
		t.Error("should not match at 12:00")
	}
}

func TestHandler_TimeRange_InvalidFormat(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "invalid-time-rule",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"hello"},
			TimeRange: &TimeRange{
				Start: "invalid",
				End:   "06:00",
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
		Timestamp: time.Now(),
	}
	result := h.Evaluate(context.Background(), msg)
	// Should not match due to invalid time format
	if result.Matched {
		t.Error("should not match with invalid time format")
	}
}

func TestHandler_RateLimit_PerSender(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "rate-limited",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"hello"},
		},
		Response: Response{
			Text: "Hello!",
		},
		RateLimit: &RateLimit{
			MaxCount:  2,
			Window:    time.Minute,
			PerSender: true,
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// User 1: First two should match
	for i := 0; i < 2; i++ {
		msg := &Message{
			Content:   "hello",
			SenderID:  "user1",
			Timestamp: time.Now(),
		}
		result := h.Evaluate(context.Background(), msg)
		if !result.Matched {
			t.Errorf("user1 request %d should match", i+1)
		}
	}

	// User 1: Third should not match
	msg1 := &Message{
		Content:   "hello",
		SenderID:  "user1",
		Timestamp: time.Now(),
	}
	result1 := h.Evaluate(context.Background(), msg1)
	if result1.Matched {
		t.Error("user1 third request should not match")
	}

	// User 2: Should still be able to match (independent rate limit)
	msg2 := &Message{
		Content:   "hello",
		SenderID:  "user2",
		Timestamp: time.Now(),
	}
	result2 := h.Evaluate(context.Background(), msg2)
	if !result2.Matched {
		t.Error("user2 should still be able to match")
	}
}

func TestRuleError(t *testing.T) {
	err := &RuleError{
		RuleID: "test-rule",
		Err:    context.DeadlineExceeded,
	}

	if err.Error() != "rule test-rule: context deadline exceeded" {
		t.Errorf("Error() = %q, unexpected format", err.Error())
	}

	if err.Unwrap() != context.DeadlineExceeded {
		t.Error("Unwrap() should return original error")
	}
}

func TestHandler_StopProcessing(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:       "stop-rule",
		Enabled:  true,
		Priority: 1,
		Conditions: Conditions{
			Keywords: []string{"stop"},
		},
		Response: Response{
			Text:           "Stopped!",
			StopProcessing: true,
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	msg := &Message{
		Content:   "please stop",
		Timestamp: time.Now(),
	}
	result := h.Evaluate(context.Background(), msg)

	if !result.StopProcessing {
		t.Error("StopProcessing should be true")
	}
}

func TestHandler_PassThrough(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "passthrough-rule",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"faq"},
		},
		Response: Response{
			Text:        "Here's an FAQ response",
			PassThrough: true,
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	msg := &Message{
		Content:   "faq question",
		Timestamp: time.Now(),
	}
	result := h.Evaluate(context.Background(), msg)

	if !result.PassThrough {
		t.Error("PassThrough should be true")
	}
}

func TestHandler_TemplateExecutionError(t *testing.T) {
	h := NewHandler()

	rule := &Rule{
		ID:      "bad-template",
		Enabled: true,
		Conditions: Conditions{
			Keywords: []string{"test"},
		},
		Response: Response{
			Template: "{{.NonExistent.Deep.Path}}",
		},
	}

	if err := h.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	msg := &Message{
		Content:   "test",
		Timestamp: time.Now(),
	}
	result := h.Evaluate(context.Background(), msg)

	// Should fall back to raw template on execution error
	if result.Response != "{{.NonExistent.Deep.Path}}" {
		t.Errorf("Response = %q, expected fallback to raw template", result.Response)
	}
}
