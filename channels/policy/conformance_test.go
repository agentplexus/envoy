// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package policy

import (
	"context"
	"testing"
	"time"
)

func TestNewConformanceChecker(t *testing.T) {
	checker, err := NewConformanceChecker(ConformanceConfig{})
	if err != nil {
		t.Fatalf("NewConformanceChecker failed: %v", err)
	}

	if checker == nil {
		t.Fatal("NewConformanceChecker returned nil")
	}
}

func TestConformanceChecker_Check(t *testing.T) {
	checker, err := NewConformanceChecker(ConformanceConfig{
		Rules: []ConformanceRule{
			NoEmptyContentRule,
			ValidSenderRule,
		},
	})
	if err != nil {
		t.Fatalf("NewConformanceChecker failed: %v", err)
	}

	ctx := context.Background()

	// Valid message
	msg := &Message{
		Channel:   "slack",
		Direction: "inbound",
		Content:   "Hello, world!",
		SenderID:  "user-123",
		Timestamp: time.Now(),
	}

	report, err := checker.Check(ctx, msg)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if !report.Passed {
		t.Errorf("Expected check to pass, got results: %+v", report.Results)
	}

	if len(report.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(report.Results))
	}

	// Invalid message - empty content
	msg.Content = ""
	report, err = checker.Check(ctx, msg)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if report.Passed {
		t.Error("Expected check to fail for empty content")
	}

	// Invalid message - no sender
	msg.Content = "Hello"
	msg.SenderID = ""
	report, err = checker.Check(ctx, msg)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if report.Passed {
		t.Error("Expected check to fail for missing sender")
	}
}

func TestConformanceChecker_ChannelFiltering(t *testing.T) {
	slackOnlyRule := ConformanceRule{
		ID:       "slack_only",
		Name:     "Slack Only Rule",
		Channels: []string{"slack"},
		Enabled:  true,
		Check: func(_ context.Context, _ *Message) error {
			return nil
		},
	}

	allChannelsRule := ConformanceRule{
		ID:       "all_channels",
		Name:     "All Channels Rule",
		Channels: []string{}, // Empty = all channels
		Enabled:  true,
		Check: func(_ context.Context, _ *Message) error {
			return nil
		},
	}

	checker, err := NewConformanceChecker(ConformanceConfig{
		Rules: []ConformanceRule{slackOnlyRule, allChannelsRule},
	})
	if err != nil {
		t.Fatalf("NewConformanceChecker failed: %v", err)
	}

	ctx := context.Background()

	// Slack message should match both rules
	msg := &Message{
		Channel:   "slack",
		Content:   "test",
		SenderID:  "user",
		Timestamp: time.Now(),
	}

	report, err := checker.Check(ctx, msg)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if len(report.Results) != 2 {
		t.Errorf("Expected 2 results for slack, got %d", len(report.Results))
	}

	// Telegram message should only match allChannelsRule
	msg.Channel = "telegram"
	report, err = checker.Check(ctx, msg)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if len(report.Results) != 1 {
		t.Errorf("Expected 1 result for telegram, got %d", len(report.Results))
	}
}

func TestConformanceChecker_AddRemoveRule(t *testing.T) {
	checker, err := NewConformanceChecker(ConformanceConfig{})
	if err != nil {
		t.Fatalf("NewConformanceChecker failed: %v", err)
	}

	// Add rule
	checker.AddRule(ConformanceRule{
		ID:      "test_rule",
		Name:    "Test Rule",
		Enabled: true,
		Check: func(_ context.Context, _ *Message) error {
			return nil
		},
	})

	rules := checker.ListRules()
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(rules))
	}

	// Remove rule
	checker.RemoveRule("test_rule")

	rules = checker.ListRules()
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules, got %d", len(rules))
	}
}

func TestConformanceChecker_EnableDisableRule(t *testing.T) {
	checker, err := NewConformanceChecker(ConformanceConfig{
		Rules: []ConformanceRule{
			{
				ID:      "test_rule",
				Name:    "Test Rule",
				Enabled: true,
				Check: func(_ context.Context, _ *Message) error {
					return nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewConformanceChecker failed: %v", err)
	}

	ctx := context.Background()
	msg := &Message{
		Channel:   "test",
		Content:   "test",
		SenderID:  "user",
		Timestamp: time.Now(),
	}

	// Should have 1 result when enabled
	report, _ := checker.Check(ctx, msg)
	if len(report.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(report.Results))
	}

	// Disable rule
	checker.DisableRule("test_rule")

	// Should have 0 results when disabled
	report, _ = checker.Check(ctx, msg)
	if len(report.Results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(report.Results))
	}

	// Enable rule
	checker.EnableRule("test_rule")

	// Should have 1 result again
	report, _ = checker.Check(ctx, msg)
	if len(report.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(report.Results))
	}
}

func TestMaxLengthRule(t *testing.T) {
	rule := MaxLengthRule(10)
	ctx := context.Background()

	// Short message - should pass
	msg := &Message{Content: "short"}
	if err := rule.Check(ctx, msg); err != nil {
		t.Errorf("Expected short message to pass: %v", err)
	}

	// Long message - should fail
	msg.Content = "this is a very long message that exceeds the limit"
	if err := rule.Check(ctx, msg); err == nil {
		t.Error("Expected long message to fail")
	}
}

func TestContentPatternRule(t *testing.T) {
	// Must match email pattern
	emailRule := ContentPatternRule("email", "Email Required", `\w+@\w+\.\w+`, SeverityError, true)
	ctx := context.Background()

	// With email - should pass
	msg := &Message{Content: "Contact me at test@example.com"}
	if err := emailRule.Check(ctx, msg); err != nil {
		t.Errorf("Expected message with email to pass: %v", err)
	}

	// Without email - should fail
	msg.Content = "No email here"
	if err := emailRule.Check(ctx, msg); err == nil {
		t.Error("Expected message without email to fail")
	}

	// Must NOT match profanity pattern
	noProfanityRule := ContentPatternRule("no_profanity", "No Profanity", `(?i)badword`, SeverityError, false)

	// Clean message - should pass
	msg.Content = "This is a clean message"
	if err := noProfanityRule.Check(ctx, msg); err != nil {
		t.Errorf("Expected clean message to pass: %v", err)
	}

	// Message with profanity - should fail
	msg.Content = "This has BADWORD in it"
	if err := noProfanityRule.Check(ctx, msg); err == nil {
		t.Error("Expected message with profanity to fail")
	}
}

func TestInMemoryRateLimitTracker(t *testing.T) {
	tracker := NewInMemoryRateLimitTracker(time.Minute)

	// Initial count should be 0
	if count := tracker.GetCount("user1"); count != 0 {
		t.Errorf("Expected initial count 0, got %d", count)
	}

	// Increment
	tracker.Increment("user1")
	if count := tracker.GetCount("user1"); count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	// Increment again
	tracker.Increment("user1")
	if count := tracker.GetCount("user1"); count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}

	// Different user
	tracker.Increment("user2")
	if count := tracker.GetCount("user2"); count != 1 {
		t.Errorf("Expected user2 count 1, got %d", count)
	}
}

func TestRateLimitRule(t *testing.T) {
	tracker := NewInMemoryRateLimitTracker(time.Minute)
	rule := RateLimitRule(2, tracker)
	ctx := context.Background()

	msg := &Message{SenderID: "user1"}

	// First message - should pass
	if err := rule.Check(ctx, msg); err != nil {
		t.Errorf("First message should pass: %v", err)
	}

	// Second message - should pass
	if err := rule.Check(ctx, msg); err != nil {
		t.Errorf("Second message should pass: %v", err)
	}

	// Third message - should fail (rate limited)
	if err := rule.Check(ctx, msg); err == nil {
		t.Error("Third message should fail due to rate limit")
	}
}

func TestSeverityToAgentops(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityInfo, "info"},
		{SeverityWarning, "warn"},
		{SeverityError, "error"},
		{SeverityCritical, "error"},
		{Severity("unknown"), "info"},
	}

	for _, tc := range tests {
		result := severityToAgentops(tc.severity)
		if result != tc.expected {
			t.Errorf("severityToAgentops(%s) = %s, want %s", tc.severity, result, tc.expected)
		}
	}
}
