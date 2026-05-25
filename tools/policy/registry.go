// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omnillm/provider"
)

// contextKey is a custom type for context keys.
type contextKey string

const (
	// SenderIDKey is the context key for sender ID.
	SenderIDKey contextKey = "sender_id"
)

// WithSenderID adds a sender ID to the context.
func WithSenderID(ctx context.Context, senderID string) context.Context {
	return context.WithValue(ctx, SenderIDKey, senderID)
}

// GetSenderID retrieves the sender ID from context.
func GetSenderID(ctx context.Context) string {
	if v := ctx.Value(SenderIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// PolicyToolRegistry wraps a ToolRegistry with policy enforcement.
type PolicyToolRegistry struct {
	registry *agent.ToolRegistry
	manager  *Manager
}

// NewPolicyToolRegistry creates a policy-enforced tool registry.
func NewPolicyToolRegistry(registry *agent.ToolRegistry, manager *Manager) *PolicyToolRegistry {
	return &PolicyToolRegistry{
		registry: registry,
		manager:  manager,
	}
}

// Register adds a tool to the underlying registry.
func (r *PolicyToolRegistry) Register(tool agent.Tool) {
	r.registry.Register(tool)
}

// Unregister removes a tool from the underlying registry.
func (r *PolicyToolRegistry) Unregister(name string) {
	r.registry.Unregister(name)
}

// Get retrieves a tool by name.
func (r *PolicyToolRegistry) Get(name string) (agent.Tool, bool) {
	return r.registry.Get(name)
}

// List returns all registered tool names.
func (r *PolicyToolRegistry) List() []string {
	return r.registry.List()
}

// GetToolsForSender returns tool definitions filtered by sender policy.
func (r *PolicyToolRegistry) GetToolsForSender(senderID string) []provider.Tool {
	allTools := r.registry.GetTools()

	if r.manager == nil {
		return allTools
	}

	policy := r.manager.GetPolicy(senderID)
	if policy == nil {
		return allTools
	}

	var filtered []provider.Tool
	for _, tool := range allTools {
		name := tool.Function.Name

		// Check denied tools
		denied := false
		for _, d := range policy.DeniedTools {
			if d == name || d == "*" {
				denied = true
				break
			}
		}
		if denied {
			continue
		}

		// Check allowed tools
		if len(policy.AllowedTools) > 0 {
			allowed := false
			for _, a := range policy.AllowedTools {
				if a == name || a == "*" {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		filtered = append(filtered, tool)
	}

	return filtered
}

// Execute runs a tool with policy enforcement.
func (r *PolicyToolRegistry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	senderID := GetSenderID(ctx)

	// Check access policy
	if r.manager != nil && senderID != "" {
		if err := r.manager.CheckAccess(ctx, senderID, name); err != nil {
			return "", err
		}
	}

	// Track concurrent execution
	if r.manager != nil && senderID != "" {
		r.manager.StartExecution(senderID)
		defer r.manager.EndExecution(senderID)
	}

	// Execute tool
	result, err := r.registry.Execute(ctx, name, args)

	// Record usage for rate limiting
	if r.manager != nil && senderID != "" {
		r.manager.RecordUsage(senderID, name)
	}

	return result, err
}

// Manager returns the policy manager.
func (r *PolicyToolRegistry) Manager() *Manager {
	return r.manager
}

// Underlying returns the underlying tool registry.
func (r *PolicyToolRegistry) Underlying() *agent.ToolRegistry {
	return r.registry
}
