// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package policy provides channel conformance checks and policy enforcement.
package policy

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/plexusone/omniobserve/agentops"
	"github.com/plexusone/omniobserve/observops"
)

// ConformanceChecker validates messages against channel policies.
type ConformanceChecker struct {
	rules             []ConformanceRule
	observopsProvider observops.Provider
	agentopsStore     agentops.Store
	logger            *slog.Logger
	mu                sync.RWMutex

	// Metrics
	checksTotal  observops.Counter
	checksPassed observops.Counter
	checksFailed observops.Counter
	checkLatency observops.Histogram
	compliance   observops.Gauge
}

// ConformanceConfig configures the conformance checker.
type ConformanceConfig struct {
	// Rules are the conformance rules to apply.
	Rules []ConformanceRule

	// ObservopsProvider is the observops provider for metrics.
	ObservopsProvider observops.Provider

	// AgentopsStore is the agentops store for event recording.
	AgentopsStore agentops.Store

	// Logger is the logger for conformance events.
	Logger *slog.Logger
}

// ConformanceRule defines a single conformance check.
type ConformanceRule struct {
	// ID is the unique identifier for this rule.
	ID string

	// Name is the human-readable name for this rule.
	Name string

	// Description describes what this rule checks.
	Description string

	// Severity indicates the rule importance.
	Severity Severity

	// Channels is the list of channels this rule applies to.
	// Empty means all channels.
	Channels []string

	// Check is the function that performs the conformance check.
	Check ConformanceCheckFunc

	// Enabled controls whether this rule is active.
	Enabled bool
}

// ConformanceCheckFunc is a function that checks conformance.
// Returns nil if conformant, error if violation detected.
type ConformanceCheckFunc func(ctx context.Context, msg *Message) error

// Severity indicates the importance of a conformance rule.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Message represents a message to be checked for conformance.
type Message struct {
	// Channel is the channel the message is from/to.
	Channel string

	// Direction is "inbound" or "outbound".
	Direction string

	// Content is the message content.
	Content string

	// SenderID is the message sender identifier.
	SenderID string

	// Metadata contains additional message metadata.
	Metadata map[string]any

	// Timestamp is when the message was created.
	Timestamp time.Time
}

// CheckResult represents the result of a conformance check.
type CheckResult struct {
	// RuleID is the ID of the rule that was checked.
	RuleID string

	// RuleName is the name of the rule.
	RuleName string

	// Passed indicates if the check passed.
	Passed bool

	// Severity is the rule severity.
	Severity Severity

	// Message is the result message.
	Message string

	// Duration is how long the check took.
	Duration time.Duration

	// Timestamp is when the check was performed.
	Timestamp time.Time
}

// ConformanceReport is a collection of check results.
type ConformanceReport struct {
	// Channel is the channel that was checked.
	Channel string

	// MessageID is the message identifier.
	MessageID string

	// Results are the individual check results.
	Results []CheckResult

	// Passed indicates if all checks passed.
	Passed bool

	// Duration is the total check duration.
	Duration time.Duration

	// Timestamp is when the report was generated.
	Timestamp time.Time
}

// NewConformanceChecker creates a new conformance checker.
func NewConformanceChecker(config ConformanceConfig) (*ConformanceChecker, error) {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	c := &ConformanceChecker{
		rules:             config.Rules,
		observopsProvider: config.ObservopsProvider,
		agentopsStore:     config.AgentopsStore,
		logger:            config.Logger,
	}

	// Initialize metrics if provider available
	if config.ObservopsProvider != nil {
		if err := c.initMetrics(); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// initMetrics initializes all metrics instruments.
func (c *ConformanceChecker) initMetrics() error {
	meter := c.observopsProvider.Meter()

	var err error

	c.checksTotal, err = meter.Counter("conformance.checks.total",
		observops.WithDescription("Total conformance checks performed"),
		observops.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	c.checksPassed, err = meter.Counter("conformance.checks.passed",
		observops.WithDescription("Conformance checks that passed"),
		observops.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	c.checksFailed, err = meter.Counter("conformance.checks.failed",
		observops.WithDescription("Conformance checks that failed"),
		observops.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	c.checkLatency, err = meter.Histogram("conformance.checks.latency",
		observops.WithDescription("Conformance check latency"),
		observops.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	c.compliance, err = meter.Gauge("conformance.compliance.ratio",
		observops.WithDescription("Current compliance ratio (0-1)"),
		observops.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	return nil
}

// Check performs all applicable conformance checks on a message.
func (c *ConformanceChecker) Check(ctx context.Context, msg *Message) (*ConformanceReport, error) {
	start := time.Now()

	c.mu.RLock()
	rules := c.applicableRules(msg.Channel)
	c.mu.RUnlock()

	report := &ConformanceReport{
		Channel:   msg.Channel,
		Results:   make([]CheckResult, 0, len(rules)),
		Passed:    true,
		Timestamp: time.Now(),
	}

	for _, rule := range rules {
		result := c.checkRule(ctx, rule, msg)
		report.Results = append(report.Results, result)

		if !result.Passed {
			report.Passed = false
		}

		// Record metrics
		c.recordCheckMetrics(ctx, result)

		// Record event
		c.recordCheckEvent(ctx, msg, result)
	}

	report.Duration = time.Since(start)

	// Update compliance gauge
	c.updateComplianceGauge(ctx, report)

	return report, nil
}

// checkRule performs a single rule check.
func (c *ConformanceChecker) checkRule(ctx context.Context, rule ConformanceRule, msg *Message) CheckResult {
	start := time.Now()

	result := CheckResult{
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Severity:  rule.Severity,
		Timestamp: time.Now(),
	}

	err := rule.Check(ctx, msg)
	result.Duration = time.Since(start)

	if err != nil {
		result.Passed = false
		result.Message = err.Error()
	} else {
		result.Passed = true
		result.Message = "check passed"
	}

	return result
}

// applicableRules returns rules that apply to the given channel.
func (c *ConformanceChecker) applicableRules(channel string) []ConformanceRule {
	var applicable []ConformanceRule

	for _, rule := range c.rules {
		if !rule.Enabled {
			continue
		}

		// Empty channels means all channels
		if len(rule.Channels) == 0 {
			applicable = append(applicable, rule)
			continue
		}

		// Check if channel matches
		for _, ch := range rule.Channels {
			if ch == channel || ch == "*" {
				applicable = append(applicable, rule)
				break
			}
		}
	}

	return applicable
}

// recordCheckMetrics records metrics for a check result.
func (c *ConformanceChecker) recordCheckMetrics(ctx context.Context, result CheckResult) {
	attrs := observops.WithAttributes(
		observops.Attribute("rule.id", result.RuleID),
		observops.Attribute("severity", string(result.Severity)),
	)

	if c.checksTotal != nil {
		c.checksTotal.Add(ctx, 1, attrs)
	}

	if result.Passed {
		if c.checksPassed != nil {
			c.checksPassed.Add(ctx, 1, attrs)
		}
	} else {
		if c.checksFailed != nil {
			c.checksFailed.Add(ctx, 1, attrs)
		}
	}

	if c.checkLatency != nil {
		c.checkLatency.Record(ctx, float64(result.Duration.Milliseconds()), attrs)
	}
}

// recordCheckEvent records an agentops event for a check result.
func (c *ConformanceChecker) recordCheckEvent(ctx context.Context, msg *Message, result CheckResult) {
	if c.agentopsStore == nil {
		return
	}

	eventType := "conformance_check"
	if !result.Passed {
		eventType = "conformance_violation"
	}

	_, err := c.agentopsStore.EmitEvent(ctx, eventType,
		agentops.WithEventCategory("policy"),
		agentops.WithEventSeverity(severityToAgentops(result.Severity)),
		agentops.WithEventData(map[string]any{
			"rule_id":   result.RuleID,
			"rule_name": result.RuleName,
			"channel":   msg.Channel,
			"direction": msg.Direction,
			"sender_id": msg.SenderID,
			"passed":    result.Passed,
			"message":   result.Message,
		}),
	)
	if err != nil {
		c.logger.Error("failed to record conformance event",
			"error", err,
			"rule_id", result.RuleID,
		)
	}
}

// updateComplianceGauge updates the compliance ratio gauge.
func (c *ConformanceChecker) updateComplianceGauge(ctx context.Context, report *ConformanceReport) {
	if c.compliance == nil || len(report.Results) == 0 {
		return
	}

	passed := 0
	for _, r := range report.Results {
		if r.Passed {
			passed++
		}
	}

	ratio := float64(passed) / float64(len(report.Results))
	c.compliance.Record(ctx, ratio,
		observops.WithAttributes(observops.Attribute("channel", report.Channel)),
	)
}

// AddRule adds a new conformance rule.
func (c *ConformanceChecker) AddRule(rule ConformanceRule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules = append(c.rules, rule)
}

// RemoveRule removes a rule by ID.
func (c *ConformanceChecker) RemoveRule(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, rule := range c.rules {
		if rule.ID == id {
			c.rules = append(c.rules[:i], c.rules[i+1:]...)
			return
		}
	}
}

// EnableRule enables a rule by ID.
func (c *ConformanceChecker) EnableRule(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.rules {
		if c.rules[i].ID == id {
			c.rules[i].Enabled = true
			return
		}
	}
}

// DisableRule disables a rule by ID.
func (c *ConformanceChecker) DisableRule(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.rules {
		if c.rules[i].ID == id {
			c.rules[i].Enabled = false
			return
		}
	}
}

// ListRules returns all registered rules.
func (c *ConformanceChecker) ListRules() []ConformanceRule {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rules := make([]ConformanceRule, len(c.rules))
	copy(rules, c.rules)
	return rules
}

func severityToAgentops(s Severity) string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warn"
	case SeverityError, SeverityCritical:
		return "error"
	default:
		return "info"
	}
}

// Predefined conformance rules.
var (
	// MaxLengthRule checks message content length.
	MaxLengthRule = func(maxLength int) ConformanceRule {
		return ConformanceRule{
			ID:          "max_length",
			Name:        "Maximum Message Length",
			Description: fmt.Sprintf("Messages must not exceed %d characters", maxLength),
			Severity:    SeverityWarning,
			Enabled:     true,
			Check: func(_ context.Context, msg *Message) error {
				if len(msg.Content) > maxLength {
					return fmt.Errorf("message length %d exceeds maximum %d", len(msg.Content), maxLength)
				}
				return nil
			},
		}
	}

	// NoEmptyContentRule checks that messages have content.
	NoEmptyContentRule = ConformanceRule{
		ID:          "no_empty_content",
		Name:        "No Empty Content",
		Description: "Messages must have non-empty content",
		Severity:    SeverityError,
		Enabled:     true,
		Check: func(_ context.Context, msg *Message) error {
			if msg.Content == "" {
				return fmt.Errorf("message content is empty")
			}
			return nil
		},
	}

	// ValidSenderRule checks that messages have a sender.
	ValidSenderRule = ConformanceRule{
		ID:          "valid_sender",
		Name:        "Valid Sender",
		Description: "Messages must have a valid sender ID",
		Severity:    SeverityError,
		Enabled:     true,
		Check: func(_ context.Context, msg *Message) error {
			if msg.SenderID == "" {
				return fmt.Errorf("sender ID is required")
			}
			return nil
		},
	}

	// ContentPatternRule checks message content against a regex pattern.
	ContentPatternRule = func(id, name string, pattern string, severity Severity, mustMatch bool) ConformanceRule {
		re := regexp.MustCompile(pattern)
		return ConformanceRule{
			ID:          id,
			Name:        name,
			Description: fmt.Sprintf("Content must %smatch pattern: %s", map[bool]string{true: "", false: "not "}[mustMatch], pattern),
			Severity:    severity,
			Enabled:     true,
			Check: func(_ context.Context, msg *Message) error {
				matches := re.MatchString(msg.Content)
				if mustMatch && !matches {
					return fmt.Errorf("content does not match required pattern")
				}
				if !mustMatch && matches {
					return fmt.Errorf("content matches prohibited pattern")
				}
				return nil
			},
		}
	}

	// RateLimitRule checks if sender is within rate limits.
	RateLimitRule = func(maxPerMinute int, tracker RateLimitTracker) ConformanceRule {
		return ConformanceRule{
			ID:          "rate_limit",
			Name:        "Rate Limit",
			Description: fmt.Sprintf("Senders limited to %d messages per minute", maxPerMinute),
			Severity:    SeverityWarning,
			Enabled:     true,
			Check: func(ctx context.Context, msg *Message) error {
				count := tracker.GetCount(msg.SenderID)
				if count >= maxPerMinute {
					return fmt.Errorf("rate limit exceeded: %d/%d messages per minute", count, maxPerMinute)
				}
				tracker.Increment(msg.SenderID)
				return nil
			},
		}
	}
)

// RateLimitTracker tracks message rates per sender.
type RateLimitTracker interface {
	GetCount(senderID string) int
	Increment(senderID string)
}

// InMemoryRateLimitTracker is a simple in-memory rate limit tracker.
type InMemoryRateLimitTracker struct {
	counts map[string]*rateLimitEntry
	window time.Duration
	mu     sync.Mutex
}

type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// NewInMemoryRateLimitTracker creates a new in-memory rate limit tracker.
func NewInMemoryRateLimitTracker(window time.Duration) *InMemoryRateLimitTracker {
	return &InMemoryRateLimitTracker{
		counts: make(map[string]*rateLimitEntry),
		window: window,
	}
}

// GetCount returns the current count for a sender.
func (t *InMemoryRateLimitTracker) GetCount(senderID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.counts[senderID]
	if !ok {
		return 0
	}

	// Check if window has expired
	if time.Now().After(entry.resetTime) {
		delete(t.counts, senderID)
		return 0
	}

	return entry.count
}

// Increment increments the count for a sender.
func (t *InMemoryRateLimitTracker) Increment(senderID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.counts[senderID]
	if !ok || time.Now().After(entry.resetTime) {
		t.counts[senderID] = &rateLimitEntry{
			count:     1,
			resetTime: time.Now().Add(t.window),
		}
		return
	}

	entry.count++
}
