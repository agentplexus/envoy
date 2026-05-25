// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package policy provides per-sender tool access control.
//
// The policy package enables fine-grained control over which tools
// are available to specific senders (users, channels, etc.). This
// supports use cases like:
//   - Restricting dangerous tools to admin users
//   - Enabling different tool sets for different channels
//   - Rate limiting tool usage per sender
package policy

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Policy defines tool access control for a sender.
type Policy struct {
	// SenderID is the sender this policy applies to.
	// Can be a user ID, channel ID, or wildcard "*" for default.
	SenderID string

	// AllowedTools lists tools the sender can use.
	// Empty means all tools allowed (unless DeniedTools is set).
	AllowedTools []string

	// DeniedTools lists tools the sender cannot use.
	// Takes precedence over AllowedTools.
	DeniedTools []string

	// RateLimits sets per-tool rate limits.
	// Key is tool name, value is max calls per minute.
	RateLimits map[string]int

	// MaxConcurrent limits concurrent tool executions.
	// 0 means unlimited.
	MaxConcurrent int

	// Metadata contains additional policy data.
	Metadata map[string]any
}

// Manager manages per-sender tool policies.
type Manager struct {
	policies      map[string]*Policy // senderID -> policy
	defaultPolicy *Policy
	usage         map[string]*usageTracker // senderID:toolName -> tracker
	mu            sync.RWMutex
}

// usageTracker tracks tool usage for rate limiting.
type usageTracker struct {
	calls  []time.Time
	active int
	mu     sync.Mutex
}

// NewManager creates a new policy manager.
func NewManager() *Manager {
	return &Manager{
		policies: make(map[string]*Policy),
		usage:    make(map[string]*usageTracker),
	}
}

// SetDefaultPolicy sets the default policy for senders without specific policies.
func (m *Manager) SetDefaultPolicy(p *Policy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultPolicy = p
}

// SetPolicy sets the policy for a specific sender.
func (m *Manager) SetPolicy(senderID string, p *Policy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p.SenderID = senderID
	m.policies[senderID] = p
}

// GetPolicy returns the policy for a sender.
// Returns the default policy if no specific policy exists.
func (m *Manager) GetPolicy(senderID string) *Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if p, ok := m.policies[senderID]; ok {
		return p
	}
	return m.defaultPolicy
}

// RemovePolicy removes a sender's policy.
func (m *Manager) RemovePolicy(senderID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.policies, senderID)
}

// CheckAccess checks if a sender can use a tool.
func (m *Manager) CheckAccess(ctx context.Context, senderID, toolName string) error {
	policy := m.GetPolicy(senderID)
	if policy == nil {
		return nil // No policy means allowed
	}

	// Check denied tools first
	for _, denied := range policy.DeniedTools {
		if denied == toolName || denied == "*" {
			return &AccessDeniedError{
				SenderID: senderID,
				ToolName: toolName,
				Reason:   "tool is in denied list",
			}
		}
	}

	// Check allowed tools
	if len(policy.AllowedTools) > 0 {
		allowed := false
		for _, a := range policy.AllowedTools {
			if a == toolName || a == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			return &AccessDeniedError{
				SenderID: senderID,
				ToolName: toolName,
				Reason:   "tool is not in allowed list",
			}
		}
	}

	// Check rate limits
	if limit, ok := policy.RateLimits[toolName]; ok {
		if err := m.checkRateLimit(senderID, toolName, limit); err != nil {
			return err
		}
	}

	// Check concurrent execution limit
	if policy.MaxConcurrent > 0 {
		if err := m.checkConcurrent(senderID, policy.MaxConcurrent); err != nil {
			return err
		}
	}

	return nil
}

// RecordUsage records a tool usage for rate limiting.
func (m *Manager) RecordUsage(senderID, toolName string) {
	key := senderID + ":" + toolName
	m.mu.Lock()
	tracker, ok := m.usage[key]
	if !ok {
		tracker = &usageTracker{}
		m.usage[key] = tracker
	}
	m.mu.Unlock()

	tracker.mu.Lock()
	tracker.calls = append(tracker.calls, time.Now())
	tracker.mu.Unlock()
}

// StartExecution marks the start of a tool execution.
func (m *Manager) StartExecution(senderID string) {
	key := senderID + ":_concurrent"
	m.mu.Lock()
	tracker, ok := m.usage[key]
	if !ok {
		tracker = &usageTracker{}
		m.usage[key] = tracker
	}
	m.mu.Unlock()

	tracker.mu.Lock()
	tracker.active++
	tracker.mu.Unlock()
}

// EndExecution marks the end of a tool execution.
func (m *Manager) EndExecution(senderID string) {
	key := senderID + ":_concurrent"
	m.mu.RLock()
	tracker, ok := m.usage[key]
	m.mu.RUnlock()

	if ok {
		tracker.mu.Lock()
		if tracker.active > 0 {
			tracker.active--
		}
		tracker.mu.Unlock()
	}
}

// checkRateLimit checks if the sender has exceeded the rate limit.
func (m *Manager) checkRateLimit(senderID, toolName string, limit int) error {
	key := senderID + ":" + toolName
	m.mu.RLock()
	tracker, ok := m.usage[key]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// Remove calls older than 1 minute
	cutoff := time.Now().Add(-time.Minute)
	var recentCalls []time.Time
	for _, t := range tracker.calls {
		if t.After(cutoff) {
			recentCalls = append(recentCalls, t)
		}
	}
	tracker.calls = recentCalls

	if len(recentCalls) >= limit {
		return &RateLimitError{
			SenderID: senderID,
			ToolName: toolName,
			Limit:    limit,
			Window:   time.Minute,
		}
	}

	return nil
}

// checkConcurrent checks if the sender has too many concurrent executions.
func (m *Manager) checkConcurrent(senderID string, limit int) error {
	key := senderID + ":_concurrent"
	m.mu.RLock()
	tracker, ok := m.usage[key]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	tracker.mu.Lock()
	active := tracker.active
	tracker.mu.Unlock()

	if active >= limit {
		return &ConcurrencyError{
			SenderID: senderID,
			Limit:    limit,
			Active:   active,
		}
	}

	return nil
}

// CleanupUsage removes stale usage data.
func (m *Manager) CleanupUsage() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-time.Hour)
	for key, tracker := range m.usage {
		tracker.mu.Lock()
		var recentCalls []time.Time
		for _, t := range tracker.calls {
			if t.After(cutoff) {
				recentCalls = append(recentCalls, t)
			}
		}
		tracker.calls = recentCalls
		tracker.mu.Unlock()

		// Remove empty trackers
		if len(tracker.calls) == 0 && tracker.active == 0 {
			delete(m.usage, key)
		}
	}
}

// AccessDeniedError is returned when a sender cannot access a tool.
type AccessDeniedError struct {
	SenderID string
	ToolName string
	Reason   string
}

func (e *AccessDeniedError) Error() string {
	return fmt.Sprintf("access denied for sender %s to tool %s: %s",
		e.SenderID, e.ToolName, e.Reason)
}

// RateLimitError is returned when a sender exceeds the rate limit.
type RateLimitError struct {
	SenderID string
	ToolName string
	Limit    int
	Window   time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded for sender %s on tool %s: %d calls per %s",
		e.SenderID, e.ToolName, e.Limit, e.Window)
}

// ConcurrencyError is returned when a sender exceeds concurrent execution limit.
type ConcurrencyError struct {
	SenderID string
	Limit    int
	Active   int
}

func (e *ConcurrencyError) Error() string {
	return fmt.Sprintf("concurrency limit exceeded for sender %s: %d active (limit: %d)",
		e.SenderID, e.Active, e.Limit)
}
