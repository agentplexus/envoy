// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package profiles provides agent-specific initialization profiles.
package profiles

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/plexusone/omnillm-core/provider"
)

// BootstrapProfile defines agent-specific initialization configuration.
// Each profile can customize system prompts, tools, context limits,
// and other agent behaviors.
type BootstrapProfile struct {
	// Name is the unique identifier for this profile.
	Name string

	// Description provides a human-readable description of the profile.
	Description string

	// SystemPrompt overrides the agent's default system prompt.
	// If empty, the agent's configured system prompt is used.
	SystemPrompt string

	// SystemPromptPrefix is prepended to the system prompt.
	SystemPromptPrefix string

	// SystemPromptSuffix is appended to the system prompt.
	SystemPromptSuffix string

	// AllowedTools limits which tools are available to the agent.
	// If empty, all registered tools are available.
	AllowedTools []string

	// DeniedTools prevents specific tools from being used.
	// Takes precedence over AllowedTools.
	DeniedTools []string

	// MaxContextMessages limits the conversation history length.
	// Zero means use the agent's default.
	MaxContextMessages int

	// MaxContextTokens limits the total token count in context.
	// Zero means use the agent's default.
	MaxContextTokens int

	// Temperature overrides the LLM temperature setting.
	// Nil means use the agent's default.
	Temperature *float64

	// MaxTokens overrides the LLM max tokens setting.
	// Nil means use the agent's default.
	MaxTokens *int

	// Model overrides the LLM model.
	// Empty means use the agent's default.
	Model string

	// ToolPolicies defines per-tool configuration.
	ToolPolicies map[string]ToolPolicy

	// OnInit is called when the profile is activated.
	OnInit func(ctx context.Context) error

	// OnClose is called when the profile is deactivated.
	OnClose func(ctx context.Context) error

	// Metadata holds arbitrary profile-specific data.
	Metadata map[string]any
}

// ToolPolicy defines per-tool configuration within a profile.
type ToolPolicy struct {
	// Enabled controls whether the tool is available.
	Enabled bool

	// RateLimit is the maximum calls per minute. Zero means unlimited.
	RateLimit int

	// Timeout overrides the tool's default timeout in seconds.
	// Zero means use the tool's default.
	Timeout int

	// RequiresConfirmation indicates the user must confirm before execution.
	RequiresConfirmation bool
}

// ProfileRegistry manages bootstrap profiles.
type ProfileRegistry struct {
	profiles map[string]*BootstrapProfile
	mu       sync.RWMutex
	logger   *slog.Logger
}

// NewProfileRegistry creates a new profile registry.
func NewProfileRegistry() *ProfileRegistry {
	return &ProfileRegistry{
		profiles: make(map[string]*BootstrapProfile),
		logger:   slog.Default(),
	}
}

// SetLogger sets the logger for the registry.
func (r *ProfileRegistry) SetLogger(logger *slog.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = logger
}

// Register adds a profile to the registry.
func (r *ProfileRegistry) Register(profile *BootstrapProfile) error {
	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.profiles[profile.Name]; exists {
		return fmt.Errorf("profile %q already registered", profile.Name)
	}

	r.profiles[profile.Name] = profile
	r.logger.Info("profile registered", "name", profile.Name)
	return nil
}

// Unregister removes a profile from the registry.
func (r *ProfileRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.profiles, name)
}

// Get retrieves a profile by name.
func (r *ProfileRegistry) Get(name string) (*BootstrapProfile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.profiles[name]
	return p, ok
}

// List returns all registered profile names.
func (r *ProfileRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.profiles))
	for name := range r.profiles {
		names = append(names, name)
	}
	return names
}

// ProfileSession represents an active profile session.
type ProfileSession struct {
	Profile *BootstrapProfile
	Active  bool
}

// Activate initializes the profile session.
func (s *ProfileSession) Activate(ctx context.Context) error {
	if s.Active {
		return nil
	}

	if s.Profile.OnInit != nil {
		if err := s.Profile.OnInit(ctx); err != nil {
			return fmt.Errorf("profile init: %w", err)
		}
	}

	s.Active = true
	return nil
}

// Deactivate closes the profile session.
func (s *ProfileSession) Deactivate(ctx context.Context) error {
	if !s.Active {
		return nil
	}

	if s.Profile.OnClose != nil {
		if err := s.Profile.OnClose(ctx); err != nil {
			return fmt.Errorf("profile close: %w", err)
		}
	}

	s.Active = false
	return nil
}

// FilterTools returns tools filtered by the profile's allow/deny lists.
func (p *BootstrapProfile) FilterTools(tools []provider.Tool) []provider.Tool {
	if len(p.AllowedTools) == 0 && len(p.DeniedTools) == 0 {
		return tools
	}

	// Build lookup sets
	denied := make(map[string]bool, len(p.DeniedTools))
	for _, name := range p.DeniedTools {
		denied[name] = true
	}

	allowed := make(map[string]bool, len(p.AllowedTools))
	hasAllowWildcard := false
	for _, name := range p.AllowedTools {
		if name == "*" {
			hasAllowWildcard = true
		}
		allowed[name] = true
	}

	var filtered []provider.Tool
	for _, tool := range tools {
		name := tool.Function.Name

		// Check denied first (takes precedence)
		if denied[name] || denied["*"] {
			continue
		}

		// Check allowed
		if len(p.AllowedTools) > 0 {
			if !allowed[name] && !hasAllowWildcard {
				continue
			}
		}

		filtered = append(filtered, tool)
	}

	return filtered
}

// BuildSystemPrompt constructs the final system prompt with prefix/suffix.
func (p *BootstrapProfile) BuildSystemPrompt(basePrompt string) string {
	prompt := basePrompt

	// Override with profile's system prompt if set
	if p.SystemPrompt != "" {
		prompt = p.SystemPrompt
	}

	// Apply prefix
	if p.SystemPromptPrefix != "" {
		prompt = p.SystemPromptPrefix + "\n\n" + prompt
	}

	// Apply suffix
	if p.SystemPromptSuffix != "" {
		prompt = prompt + "\n\n" + p.SystemPromptSuffix
	}

	return prompt
}

// GetToolPolicy returns the policy for a specific tool.
func (p *BootstrapProfile) GetToolPolicy(toolName string) (ToolPolicy, bool) {
	if p.ToolPolicies == nil {
		return ToolPolicy{}, false
	}
	policy, ok := p.ToolPolicies[toolName]
	return policy, ok
}

// Clone creates a deep copy of the profile.
func (p *BootstrapProfile) Clone() *BootstrapProfile {
	clone := &BootstrapProfile{
		Name:               p.Name,
		Description:        p.Description,
		SystemPrompt:       p.SystemPrompt,
		SystemPromptPrefix: p.SystemPromptPrefix,
		SystemPromptSuffix: p.SystemPromptSuffix,
		MaxContextMessages: p.MaxContextMessages,
		MaxContextTokens:   p.MaxContextTokens,
		Model:              p.Model,
		OnInit:             p.OnInit,
		OnClose:            p.OnClose,
	}

	if p.Temperature != nil {
		t := *p.Temperature
		clone.Temperature = &t
	}
	if p.MaxTokens != nil {
		t := *p.MaxTokens
		clone.MaxTokens = &t
	}

	if len(p.AllowedTools) > 0 {
		clone.AllowedTools = make([]string, len(p.AllowedTools))
		copy(clone.AllowedTools, p.AllowedTools)
	}

	if len(p.DeniedTools) > 0 {
		clone.DeniedTools = make([]string, len(p.DeniedTools))
		copy(clone.DeniedTools, p.DeniedTools)
	}

	if len(p.ToolPolicies) > 0 {
		clone.ToolPolicies = make(map[string]ToolPolicy, len(p.ToolPolicies))
		for k, v := range p.ToolPolicies {
			clone.ToolPolicies[k] = v
		}
	}

	if len(p.Metadata) > 0 {
		clone.Metadata = make(map[string]any, len(p.Metadata))
		for k, v := range p.Metadata {
			clone.Metadata[k] = v
		}
	}

	return clone
}

// Predefined profiles for common use cases.
var (
	// DefaultProfile is the default agent profile with no restrictions.
	DefaultProfile = &BootstrapProfile{
		Name:        "default",
		Description: "Default profile with no restrictions",
	}

	// RestrictedProfile limits tool access and context size.
	RestrictedProfile = &BootstrapProfile{
		Name:               "restricted",
		Description:        "Restricted profile with limited tools and context",
		DeniedTools:        []string{"shell", "browser", "file_write"},
		MaxContextMessages: 20,
		MaxContextTokens:   4000,
	}

	// ReadOnlyProfile only allows read operations.
	ReadOnlyProfile = &BootstrapProfile{
		Name:        "readonly",
		Description: "Read-only profile that prevents modifications",
		AllowedTools: []string{
			"search", "read", "glob", "grep", "web_fetch",
		},
	}

	// CodeAssistantProfile is optimized for code assistance.
	CodeAssistantProfile = &BootstrapProfile{
		Name:        "code_assistant",
		Description: "Profile optimized for code assistance tasks",
		SystemPromptSuffix: `You are a code assistant. Focus on:
- Writing clean, maintainable code
- Following best practices and conventions
- Explaining code changes clearly
- Suggesting tests when appropriate`,
		AllowedTools: []string{
			"read", "write", "edit", "glob", "grep", "shell",
		},
	}
)
