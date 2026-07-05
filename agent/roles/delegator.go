// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package roles

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/plexusone/omniskill/role"
)

// Delegator manages sub-agent delegation based on role rules.
//
// The delegator evaluates delegation rules defined in a role's RoleSpec
// and determines whether tasks can be delegated to other roles.
type Delegator struct {
	config *role.DelegationConfig
	rules  []role.DelegationRule
	budget *delegationBudgetTracker
}

// NewDelegator creates a new Delegator with the given configuration.
func NewDelegator(config *role.DelegationConfig) *Delegator {
	if config == nil {
		return &Delegator{
			config: &role.DelegationConfig{Enabled: false},
		}
	}

	d := &Delegator{
		config: config,
		rules:  config.Rules,
	}

	if config.Budget != nil {
		d.budget = newDelegationBudgetTracker(config.Budget)
	}

	return d
}

// ErrDelegationDisabled is returned when delegation is not enabled.
var ErrDelegationDisabled = errors.New("delegation is not enabled for this role")

// ErrNoDelegationRule is returned when no delegation rule matches.
var ErrNoDelegationRule = errors.New("no delegation rule matches the task")

// ErrBudgetExceeded is returned when the delegation budget is exceeded.
var ErrBudgetExceeded = errors.New("delegation budget exceeded")

// CanDelegate determines if a task type can be delegated.
// Returns true and the list of target roles if delegation is possible.
func (d *Delegator) CanDelegate(ctx context.Context, taskType string) (bool, []string) {
	if !d.config.Enabled {
		return false, nil
	}

	for _, rule := range d.rules {
		if d.matchesTaskPattern(rule.TaskPatterns, taskType) {
			return true, rule.TargetRoles
		}
	}

	return false, nil
}

// SelectTargetRole finds the best role for a given task type.
// Returns the first matching target role based on rule priority.
func (d *Delegator) SelectTargetRole(ctx context.Context, taskType string) (string, error) {
	if !d.config.Enabled {
		return "", ErrDelegationDisabled
	}

	// Sort rules by priority
	rules := make([]role.DelegationRule, len(d.rules))
	copy(rules, d.rules)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	for _, rule := range rules {
		if d.matchesTaskPattern(rule.TaskPatterns, taskType) {
			if len(rule.TargetRoles) > 0 {
				return rule.TargetRoles[0], nil
			}
		}
	}

	return "", ErrNoDelegationRule
}

// Delegate prepares a delegation request for a task.
// Returns a DelegationRequest that can be used to track the delegation.
func (d *Delegator) Delegate(ctx context.Context, taskType, taskID string, input map[string]any) (*DelegationRequest, error) {
	if !d.config.Enabled {
		return nil, ErrDelegationDisabled
	}

	targetRole, err := d.SelectTargetRole(ctx, taskType)
	if err != nil {
		return nil, err
	}

	// Check budget
	if d.budget != nil && !d.budget.canDelegate() {
		return nil, ErrBudgetExceeded
	}

	// Find the matching rule for timeout
	var timeout time.Duration
	for _, rule := range d.rules {
		if d.matchesTaskPattern(rule.TaskPatterns, taskType) {
			if rule.Timeout != "" {
				timeout, _ = time.ParseDuration(rule.Timeout)
			}
			break
		}
	}
	if timeout == 0 && d.config.DefaultTimeout != "" {
		timeout, _ = time.ParseDuration(d.config.DefaultTimeout)
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	req := &DelegationRequest{
		TaskID:     taskID,
		TaskType:   taskType,
		TargetRole: targetRole,
		Input:      input,
		Timeout:    timeout,
		CreatedAt:  time.Now(),
		Status:     role.DelegationStatusPending,
	}

	if d.budget != nil {
		d.budget.recordDelegation()
	}

	return req, nil
}

// IsAutonomous checks if a task type can be delegated autonomously.
func (d *Delegator) IsAutonomous(taskType string) bool {
	for _, rule := range d.rules {
		if d.matchesTaskPattern(rule.TaskPatterns, taskType) {
			return rule.Autonomous
		}
	}
	return false
}

// Complete marks a delegation as completed, freeing budget resources.
func (d *Delegator) Complete(req *DelegationRequest) {
	if d.budget != nil {
		d.budget.CompleteDelegation()
	}
	if req != nil {
		req.Status = role.DelegationStatusCompleted
	}
}

// matchesTaskPattern checks if a task type matches any of the patterns.
func (d *Delegator) matchesTaskPattern(patterns []string, taskType string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, taskType)
		if err == nil && matched {
			return true
		}
		// Also try exact match
		if pattern == taskType {
			return true
		}
	}
	return false
}

// DelegationRequest represents a request to delegate work to another role.
type DelegationRequest struct {
	TaskID     string
	TaskType   string
	TargetRole string
	Input      map[string]any
	Timeout    time.Duration
	CreatedAt  time.Time
	Status     role.DelegationStatus
	Result     *role.DelegationResult
}

// delegationBudgetTracker tracks delegation resource usage.
type delegationBudgetTracker struct {
	mu         sync.Mutex
	budget     *role.DelegationBudget
	concurrent int
	dailyCount int
	lastReset  time.Time
}

func newDelegationBudgetTracker(budget *role.DelegationBudget) *delegationBudgetTracker {
	return &delegationBudgetTracker{
		budget:    budget,
		lastReset: time.Now(),
	}
}

func (t *delegationBudgetTracker) canDelegate() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.resetIfNeeded()

	if t.budget.MaxConcurrent > 0 && t.concurrent >= t.budget.MaxConcurrent {
		return false
	}
	if t.budget.MaxDaily > 0 && t.dailyCount >= t.budget.MaxDaily {
		return false
	}
	return true
}

func (t *delegationBudgetTracker) recordDelegation() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.resetIfNeeded()
	t.concurrent++
	t.dailyCount++
}

// CompleteDelegation marks a delegation as complete, reducing the concurrent count.
func (t *delegationBudgetTracker) CompleteDelegation() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.concurrent > 0 {
		t.concurrent--
	}
}

func (t *delegationBudgetTracker) resetIfNeeded() {
	now := time.Now()
	if now.Sub(t.lastReset) >= 24*time.Hour {
		t.dailyCount = 0
		t.lastReset = now
	}
}
