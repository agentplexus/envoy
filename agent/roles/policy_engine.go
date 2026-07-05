// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package roles

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/plexusone/omniskill/role"
)

// PolicyEngine enforces policy rules at runtime.
//
// The engine evaluates policies defined in a role's RoleSpec and
// determines whether specific operations should be allowed, warned,
// or blocked.
type PolicyEngine struct {
	policies []role.Policy
}

// NewPolicyEngine creates a new PolicyEngine with the given policies.
func NewPolicyEngine(policies []role.Policy) *PolicyEngine {
	return &PolicyEngine{
		policies: policies,
	}
}

// ErrPolicyDenied is returned when a policy blocks an operation.
var ErrPolicyDenied = errors.New("operation denied by policy")

// CheckToolAccess determines if a tool can be used.
// Returns nil if allowed, ErrPolicyDenied if blocked.
func (e *PolicyEngine) CheckToolAccess(ctx context.Context, toolName string) error {
	for _, policy := range e.policies {
		if !policy.Enabled {
			continue
		}
		if policy.Type != role.PolicyTypeToolAccess {
			continue
		}

		for _, rule := range policy.Rules {
			if !e.matchesTarget(rule.Target, "tool", toolName) {
				continue
			}

			switch rule.Action {
			case role.PolicyActionDeny:
				return e.handleViolation(policy, rule, "tool", toolName)
			case role.PolicyActionAllow:
				return nil
			}
		}
	}

	// Default allow if no policy matches
	return nil
}

// CheckDataAccess determines if a data type can be accessed.
// Returns nil if allowed, ErrPolicyDenied if blocked.
func (e *PolicyEngine) CheckDataAccess(ctx context.Context, dataType string) error {
	for _, policy := range e.policies {
		if !policy.Enabled {
			continue
		}
		if policy.Type != role.PolicyTypeDataAccess {
			continue
		}

		for _, rule := range policy.Rules {
			if !e.matchesTarget(rule.Target, "data", dataType) {
				continue
			}

			switch rule.Action {
			case role.PolicyActionDeny:
				return e.handleViolation(policy, rule, "data", dataType)
			case role.PolicyActionAllow:
				return nil
			}
		}
	}

	return nil
}

// CheckRateLimit determines if an operation is within rate limits.
// Returns nil if allowed, ErrPolicyDenied if rate limit exceeded.
func (e *PolicyEngine) CheckRateLimit(ctx context.Context, operation string) error {
	// Rate limiting requires stateful tracking.
	// This is a placeholder that always allows operations.
	// A full implementation would track operation counts over time.
	return nil
}

// RequiresConfirmation determines if an operation needs user confirmation.
func (e *PolicyEngine) RequiresConfirmation(ctx context.Context, operation string) bool {
	for _, policy := range e.policies {
		if !policy.Enabled {
			continue
		}
		if policy.Type != role.PolicyTypeConfirmation {
			continue
		}

		for _, rule := range policy.Rules {
			if e.matchesTarget(rule.Target, "operation", operation) {
				return true
			}
		}
	}

	return false
}

// matchesTarget checks if a target pattern matches the given type and name.
func (e *PolicyEngine) matchesTarget(target role.PolicyTarget, targetType, name string) bool {
	if target.Type != targetType {
		return false
	}

	// Support glob patterns
	matched, err := filepath.Match(target.Pattern, name)
	if err != nil {
		// Invalid pattern, try exact match
		return target.Pattern == name
	}
	return matched
}

// handleViolation processes a policy violation based on enforcement mode.
func (e *PolicyEngine) handleViolation(policy role.Policy, rule role.PolicyRule, targetType, name string) error {
	switch policy.Enforcement.Mode {
	case role.EnforcementModeBlock:
		msg := policy.Enforcement.Message
		if msg == "" {
			msg = rule.Reason
		}
		if msg == "" {
			msg = "access denied"
		}
		return &PolicyViolation{
			PolicyID:   policy.ID,
			PolicyName: policy.Name,
			RuleID:     rule.ID,
			TargetType: targetType,
			TargetName: name,
			Message:    msg,
		}
	case role.EnforcementModeWarn:
		// Log warning but allow
		return nil
	case role.EnforcementModeAudit:
		// Log for audit but allow
		return nil
	default:
		return ErrPolicyDenied
	}
}

// PolicyViolation provides details about a policy violation.
type PolicyViolation struct {
	PolicyID   string
	PolicyName string
	RuleID     string
	TargetType string
	TargetName string
	Message    string
}

// Error implements the error interface.
func (v *PolicyViolation) Error() string {
	var b strings.Builder
	b.WriteString("policy violation: ")
	b.WriteString(v.PolicyName)
	if v.Message != "" {
		b.WriteString(": ")
		b.WriteString(v.Message)
	}
	return b.String()
}

// Unwrap returns the underlying error for errors.Is compatibility.
func (v *PolicyViolation) Unwrap() error {
	return ErrPolicyDenied
}
