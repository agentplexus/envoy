// Package registry provides multi-agent management for OmniAgent.
package registry

import (
	"time"
)

// AgentConfig defines configuration for a single agent.
type AgentConfig struct {
	// ID is the unique identifier for the agent (used as model name).
	ID string `json:"id" yaml:"id"`

	// Name is the human-readable display name.
	Name string `json:"name" yaml:"name"`

	// Description provides additional context about the agent's purpose.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Provider is the LLM provider (e.g., "anthropic", "openai").
	Provider string `json:"provider" yaml:"provider"`

	// Model is the LLM model to use (e.g., "claude-sonnet-4-20250514").
	Model string `json:"model" yaml:"model"`

	// APIKey is the API key for the provider (optional, can use env var).
	APIKey string `json:"api_key,omitempty" yaml:"api_key,omitempty"` //nolint:gosec // G101: API key intentionally stored

	// BaseURL is an optional custom base URL for the provider.
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`

	// Temperature controls randomness (0.0-2.0).
	Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`

	// MaxTokens limits the response length.
	MaxTokens int `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`

	// SystemPrompt is the system message for the agent.
	SystemPrompt string `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`

	// AllowedTools limits which tools this agent can use (empty = all).
	AllowedTools []string `json:"allowed_tools,omitempty" yaml:"allowed_tools,omitempty"`

	// DeniedTools excludes specific tools from this agent.
	DeniedTools []string `json:"denied_tools,omitempty" yaml:"denied_tools,omitempty"`

	// Enabled determines if the agent is active. Defaults to true.
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// CreatedAt is when the agent was created.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`

	// UpdatedAt is when the agent was last modified.
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// IsEnabled returns whether the agent is enabled.
// Defaults to true if Enabled is nil.
func (c *AgentConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// Clone creates a deep copy of the config.
func (c *AgentConfig) Clone() *AgentConfig {
	if c == nil {
		return nil
	}

	clone := *c

	// Deep copy slices
	if c.AllowedTools != nil {
		clone.AllowedTools = make([]string, len(c.AllowedTools))
		copy(clone.AllowedTools, c.AllowedTools)
	}
	if c.DeniedTools != nil {
		clone.DeniedTools = make([]string, len(c.DeniedTools))
		copy(clone.DeniedTools, c.DeniedTools)
	}

	// Deep copy Enabled pointer
	if c.Enabled != nil {
		enabled := *c.Enabled
		clone.Enabled = &enabled
	}

	return &clone
}

// Merge applies non-zero values from other to this config.
func (c *AgentConfig) Merge(other *AgentConfig) {
	if other == nil {
		return
	}

	if other.Name != "" {
		c.Name = other.Name
	}
	if other.Description != "" {
		c.Description = other.Description
	}
	if other.Provider != "" {
		c.Provider = other.Provider
	}
	if other.Model != "" {
		c.Model = other.Model
	}
	if other.APIKey != "" {
		c.APIKey = other.APIKey
	}
	if other.BaseURL != "" {
		c.BaseURL = other.BaseURL
	}
	if other.Temperature != 0 {
		c.Temperature = other.Temperature
	}
	if other.MaxTokens != 0 {
		c.MaxTokens = other.MaxTokens
	}
	if other.SystemPrompt != "" {
		c.SystemPrompt = other.SystemPrompt
	}
	if other.AllowedTools != nil {
		c.AllowedTools = make([]string, len(other.AllowedTools))
		copy(c.AllowedTools, other.AllowedTools)
	}
	if other.DeniedTools != nil {
		c.DeniedTools = make([]string, len(other.DeniedTools))
		copy(c.DeniedTools, other.DeniedTools)
	}
	if other.Enabled != nil {
		enabled := *other.Enabled
		c.Enabled = &enabled
	}
}
