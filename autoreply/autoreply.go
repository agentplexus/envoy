// Package autoreply provides automatic response handling for messages.
//
// Auto-reply allows configuring rules to automatically respond to messages
// without invoking the LLM, useful for:
//   - FAQ responses
//   - Out-of-office messages
//   - Command shortcuts
//   - Rate limiting responses
package autoreply

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Rule defines an auto-reply rule.
type Rule struct {
	// ID is a unique identifier for the rule.
	ID string `json:"id" yaml:"id"`

	// Name is a human-readable name for the rule.
	Name string `json:"name" yaml:"name"`

	// Enabled controls whether the rule is active.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Priority determines rule evaluation order (lower = higher priority).
	Priority int `json:"priority" yaml:"priority"`

	// Conditions that must match for the rule to trigger.
	Conditions Conditions `json:"conditions" yaml:"conditions"`

	// Response to send when the rule matches.
	Response Response `json:"response" yaml:"response"`

	// RateLimit controls how often the rule can trigger.
	RateLimit *RateLimit `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty"`

	// compiled patterns (populated on load)
	compiledPatterns []*regexp.Regexp
}

// Conditions defines when a rule should trigger.
type Conditions struct {
	// Patterns are regex patterns to match against the message.
	// If multiple patterns are specified, any match triggers the rule.
	Patterns []string `json:"patterns,omitempty" yaml:"patterns,omitempty"`

	// Keywords are exact words to match (case-insensitive).
	Keywords []string `json:"keywords,omitempty" yaml:"keywords,omitempty"`

	// Channels limits the rule to specific channel types.
	Channels []string `json:"channels,omitempty" yaml:"channels,omitempty"`

	// Senders limits the rule to specific sender IDs.
	Senders []string `json:"senders,omitempty" yaml:"senders,omitempty"`

	// TimeRange limits the rule to specific times.
	TimeRange *TimeRange `json:"time_range,omitempty" yaml:"time_range,omitempty"`
}

// TimeRange defines a time window for rule activation.
type TimeRange struct {
	// Start time in HH:MM format (24-hour).
	Start string `json:"start" yaml:"start"`

	// End time in HH:MM format (24-hour).
	End string `json:"end" yaml:"end"`

	// Days of the week (0=Sunday, 6=Saturday).
	// Empty means all days.
	Days []int `json:"days,omitempty" yaml:"days,omitempty"`

	// Timezone for time calculations (default: UTC).
	Timezone string `json:"timezone,omitempty" yaml:"timezone,omitempty"`
}

// Response defines what to send when a rule matches.
type Response struct {
	// Text is the response message.
	Text string `json:"text" yaml:"text"`

	// Template is a Go template for dynamic responses.
	// Available variables: .Message, .Sender, .Channel, .Time
	Template string `json:"template,omitempty" yaml:"template,omitempty"`

	// StopProcessing prevents further rule evaluation if true.
	StopProcessing bool `json:"stop_processing" yaml:"stop_processing"`

	// PassThrough allows the message to continue to the LLM if true.
	PassThrough bool `json:"pass_through" yaml:"pass_through"`
}

// RateLimit controls how often a rule can trigger.
type RateLimit struct {
	// MaxCount is the maximum triggers allowed in the window.
	MaxCount int `json:"max_count" yaml:"max_count"`

	// Window is the time window for rate limiting.
	Window time.Duration `json:"window" yaml:"window"`

	// PerSender applies rate limiting per sender (default: global).
	PerSender bool `json:"per_sender" yaml:"per_sender"`
}

// Message represents an incoming message for auto-reply evaluation.
type Message struct {
	// Content is the message text.
	Content string

	// SenderID is the sender identifier.
	SenderID string

	// Channel is the channel type (telegram, discord, etc.).
	Channel string

	// Timestamp is when the message was received.
	Timestamp time.Time

	// Metadata contains additional message data.
	Metadata map[string]any
}

// Result represents the auto-reply evaluation result.
type Result struct {
	// Matched indicates if any rule matched.
	Matched bool

	// RuleID is the ID of the matched rule.
	RuleID string

	// Response is the response to send.
	Response string

	// StopProcessing indicates if processing should stop.
	StopProcessing bool

	// PassThrough indicates if the message should continue to LLM.
	PassThrough bool
}

// Handler evaluates messages against auto-reply rules.
type Handler struct {
	rules       []*Rule
	mu          sync.RWMutex
	rateLimiter *rateLimiter
}

// NewHandler creates a new auto-reply handler.
func NewHandler() *Handler {
	return &Handler{
		rateLimiter: newRateLimiter(),
	}
}

// AddRule adds a rule to the handler.
func (h *Handler) AddRule(rule *Rule) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Compile patterns
	for _, pattern := range rule.Conditions.Patterns {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return &RuleError{RuleID: rule.ID, Err: err}
		}
		rule.compiledPatterns = append(rule.compiledPatterns, re)
	}

	h.rules = append(h.rules, rule)
	h.sortRules()

	return nil
}

// RemoveRule removes a rule by ID.
func (h *Handler) RemoveRule(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, rule := range h.rules {
		if rule.ID == id {
			h.rules = append(h.rules[:i], h.rules[i+1:]...)
			return true
		}
	}
	return false
}

// GetRule returns a rule by ID.
func (h *Handler) GetRule(id string) (*Rule, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, rule := range h.rules {
		if rule.ID == id {
			return rule, true
		}
	}
	return nil, false
}

// ListRules returns all rules.
func (h *Handler) ListRules() []*Rule {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*Rule, len(h.rules))
	copy(result, h.rules)
	return result
}

// Evaluate checks a message against all rules and returns a result.
func (h *Handler) Evaluate(ctx context.Context, msg *Message) *Result {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, rule := range h.rules {
		if !rule.Enabled {
			continue
		}

		if !h.matchesConditions(rule, msg) {
			continue
		}

		if !h.checkRateLimit(rule, msg) {
			continue
		}

		// Rule matched
		response := h.buildResponse(rule, msg)

		return &Result{
			Matched:        true,
			RuleID:         rule.ID,
			Response:       response,
			StopProcessing: rule.Response.StopProcessing,
			PassThrough:    rule.Response.PassThrough,
		}
	}

	return &Result{Matched: false}
}

// matchesConditions checks if a message matches a rule's conditions.
func (h *Handler) matchesConditions(rule *Rule, msg *Message) bool {
	// Check channel filter
	if len(rule.Conditions.Channels) > 0 {
		found := false
		for _, ch := range rule.Conditions.Channels {
			if strings.EqualFold(ch, msg.Channel) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check sender filter
	if len(rule.Conditions.Senders) > 0 {
		found := false
		for _, s := range rule.Conditions.Senders {
			if s == msg.SenderID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check time range
	if rule.Conditions.TimeRange != nil {
		if !h.inTimeRange(rule.Conditions.TimeRange, msg.Timestamp) {
			return false
		}
	}

	// Check patterns (any match)
	patternMatched := len(rule.compiledPatterns) == 0
	for _, re := range rule.compiledPatterns {
		if re.MatchString(msg.Content) {
			patternMatched = true
			break
		}
	}

	// Check keywords (any match)
	keywordMatched := len(rule.Conditions.Keywords) == 0
	contentLower := strings.ToLower(msg.Content)
	for _, kw := range rule.Conditions.Keywords {
		if strings.Contains(contentLower, strings.ToLower(kw)) {
			keywordMatched = true
			break
		}
	}

	// Both pattern and keyword conditions must pass (if specified)
	if len(rule.compiledPatterns) > 0 && len(rule.Conditions.Keywords) > 0 {
		return patternMatched && keywordMatched
	}

	return patternMatched && keywordMatched
}

// inTimeRange checks if a timestamp falls within a time range.
func (h *Handler) inTimeRange(tr *TimeRange, t time.Time) bool {
	loc := time.UTC
	if tr.Timezone != "" {
		if l, err := time.LoadLocation(tr.Timezone); err == nil {
			loc = l
		}
	}

	t = t.In(loc)

	// Check day of week
	if len(tr.Days) > 0 {
		dayMatched := false
		weekday := int(t.Weekday())
		for _, d := range tr.Days {
			if d == weekday {
				dayMatched = true
				break
			}
		}
		if !dayMatched {
			return false
		}
	}

	// Parse start and end times
	start, err := time.Parse("15:04", tr.Start)
	if err != nil {
		return false
	}
	end, err := time.Parse("15:04", tr.End)
	if err != nil {
		return false
	}

	// Create time values for today
	now := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, loc)
	startTime := time.Date(t.Year(), t.Month(), t.Day(), start.Hour(), start.Minute(), 0, 0, loc)
	endTime := time.Date(t.Year(), t.Month(), t.Day(), end.Hour(), end.Minute(), 0, 0, loc)

	// Handle overnight ranges (e.g., 22:00 - 06:00)
	if endTime.Before(startTime) {
		return now.After(startTime) || now.Before(endTime)
	}

	return now.After(startTime) && now.Before(endTime)
}

// checkRateLimit checks if a rule is within rate limits.
func (h *Handler) checkRateLimit(rule *Rule, msg *Message) bool {
	if rule.RateLimit == nil {
		return true
	}

	key := rule.ID
	if rule.RateLimit.PerSender {
		key = rule.ID + ":" + msg.SenderID
	}

	return h.rateLimiter.allow(key, rule.RateLimit.MaxCount, rule.RateLimit.Window)
}

// buildResponse builds the response text for a matched rule.
//
//nolint:unparam // msg is reserved for template rendering support
func (h *Handler) buildResponse(rule *Rule, msg *Message) string {
	if rule.Response.Template != "" {
		// TODO: Implement template rendering
		return rule.Response.Text
	}
	return rule.Response.Text
}

// sortRules sorts rules by priority (lower = higher priority).
func (h *Handler) sortRules() {
	for i := 0; i < len(h.rules)-1; i++ {
		for j := i + 1; j < len(h.rules); j++ {
			if h.rules[j].Priority < h.rules[i].Priority {
				h.rules[i], h.rules[j] = h.rules[j], h.rules[i]
			}
		}
	}
}

// RuleError represents an error with a specific rule.
type RuleError struct {
	RuleID string
	Err    error
}

func (e *RuleError) Error() string {
	return "rule " + e.RuleID + ": " + e.Err.Error()
}

func (e *RuleError) Unwrap() error {
	return e.Err
}

// rateLimiter implements a simple sliding window rate limiter.
type rateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		windows: make(map[string][]time.Time),
	}
}

func (r *rateLimiter) allow(key string, maxCount int, window time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	// Remove expired entries
	times := r.windows[key]
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= maxCount {
		r.windows[key] = valid
		return false
	}

	r.windows[key] = append(valid, now)
	return true
}
