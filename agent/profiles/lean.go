// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package profiles

import (
	"fmt"
)

// LeanMode defines resource optimization settings for local models.
// This reduces memory usage and improves performance for constrained environments.
type LeanMode struct {
	// Enabled activates lean mode optimizations.
	Enabled bool

	// Level controls the aggressiveness of optimizations.
	// Higher levels reduce more resources but may impact quality.
	Level LeanLevel

	// MaxContextTokens limits the context window size.
	// Lower values reduce memory usage.
	MaxContextTokens int

	// MaxResponseTokens limits the response length.
	MaxResponseTokens int

	// DisableTools disables all tool usage when true.
	DisableTools bool

	// ToolAllowlist limits which tools are available.
	// Empty means all tools (unless DisableTools is true).
	ToolAllowlist []string

	// DisableHistory prevents storing conversation history.
	DisableHistory bool

	// CompactPrompts removes whitespace and shortens prompts.
	CompactPrompts bool

	// SkipSystemPrompt omits the system prompt to save tokens.
	SkipSystemPrompt bool

	// BatchSize controls how many messages to process at once.
	// Lower values reduce peak memory usage.
	BatchSize int

	// StreamingDisabled prevents streaming responses.
	// Useful when streaming adds overhead.
	StreamingDisabled bool

	// CacheDisabled prevents caching of responses.
	CacheDisabled bool
}

// LeanLevel represents the aggressiveness of lean mode optimizations.
type LeanLevel int

const (
	// LeanLevelOff disables lean mode.
	LeanLevelOff LeanLevel = iota

	// LeanLevelLight applies minimal optimizations.
	// - Reduces context to 2048 tokens
	// - Limits response to 512 tokens
	LeanLevelLight

	// LeanLevelModerate applies balanced optimizations.
	// - Reduces context to 1024 tokens
	// - Limits response to 256 tokens
	// - Compacts prompts
	LeanLevelModerate

	// LeanLevelAggressive applies maximum optimizations.
	// - Reduces context to 512 tokens
	// - Limits response to 128 tokens
	// - Disables history
	// - Skips system prompt
	LeanLevelAggressive
)

// String returns the string representation of the lean level.
func (l LeanLevel) String() string {
	switch l {
	case LeanLevelOff:
		return "off"
	case LeanLevelLight:
		return "light"
	case LeanLevelModerate:
		return "moderate"
	case LeanLevelAggressive:
		return "aggressive"
	default:
		return fmt.Sprintf("unknown(%d)", l)
	}
}

// ParseLeanLevel parses a string into a LeanLevel.
func ParseLeanLevel(s string) (LeanLevel, error) {
	switch s {
	case "off", "none", "disabled":
		return LeanLevelOff, nil
	case "light", "low", "minimal":
		return LeanLevelLight, nil
	case "moderate", "medium", "balanced":
		return LeanLevelModerate, nil
	case "aggressive", "high", "maximum":
		return LeanLevelAggressive, nil
	default:
		return LeanLevelOff, fmt.Errorf("unknown lean level: %q", s)
	}
}

// NewLeanMode creates a LeanMode with default settings for the given level.
func NewLeanMode(level LeanLevel) *LeanMode {
	m := &LeanMode{
		Enabled: level != LeanLevelOff,
		Level:   level,
	}

	switch level {
	case LeanLevelLight:
		m.MaxContextTokens = 2048
		m.MaxResponseTokens = 512
	case LeanLevelModerate:
		m.MaxContextTokens = 1024
		m.MaxResponseTokens = 256
		m.CompactPrompts = true
	case LeanLevelAggressive:
		m.MaxContextTokens = 512
		m.MaxResponseTokens = 128
		m.CompactPrompts = true
		m.DisableHistory = true
		m.SkipSystemPrompt = true
		m.StreamingDisabled = true
	}

	return m
}

// Apply applies lean mode settings to a bootstrap profile.
func (m *LeanMode) Apply(profile *BootstrapProfile) {
	if !m.Enabled || m.Level == LeanLevelOff {
		return
	}

	// Apply context limits
	if m.MaxContextTokens > 0 && (profile.MaxContextTokens == 0 || m.MaxContextTokens < profile.MaxContextTokens) {
		profile.MaxContextTokens = m.MaxContextTokens
	}

	// Apply response limits
	if m.MaxResponseTokens > 0 {
		profile.MaxTokens = &m.MaxResponseTokens
	}

	// Apply tool restrictions
	if m.DisableTools {
		profile.DeniedTools = []string{"*"}
	} else if len(m.ToolAllowlist) > 0 {
		profile.AllowedTools = m.ToolAllowlist
	}

	// Apply system prompt modifications
	if m.SkipSystemPrompt {
		profile.SystemPrompt = ""
		profile.SystemPromptPrefix = ""
		profile.SystemPromptSuffix = ""
	}
}

// EstimateMemorySavings returns an estimated percentage of memory savings.
func (m *LeanMode) EstimateMemorySavings() float64 {
	if !m.Enabled {
		return 0
	}

	switch m.Level {
	case LeanLevelLight:
		return 0.2 // ~20% savings
	case LeanLevelModerate:
		return 0.4 // ~40% savings
	case LeanLevelAggressive:
		return 0.6 // ~60% savings
	default:
		return 0
	}
}

// LeanModeConfig configures lean mode behavior.
type LeanModeConfig struct {
	// Level is the lean mode level.
	Level LeanLevel

	// CustomSettings override the level defaults.
	CustomSettings *LeanMode

	// ModelSpecific maps model names to lean mode settings.
	// Allows different settings per model.
	ModelSpecific map[string]*LeanMode
}

// GetForModel returns lean mode settings for a specific model.
func (c *LeanModeConfig) GetForModel(model string) *LeanMode {
	// Check model-specific settings first
	if c.ModelSpecific != nil {
		if m, ok := c.ModelSpecific[model]; ok {
			return m
		}
	}

	// Use custom settings if provided
	if c.CustomSettings != nil {
		return c.CustomSettings
	}

	// Fall back to level defaults
	return NewLeanMode(c.Level)
}

// Predefined lean mode configurations.
var (
	// LeanModeDisabled is a convenience for disabling lean mode.
	LeanModeDisabled = &LeanMode{Enabled: false, Level: LeanLevelOff}

	// LeanModeForOllama is optimized for Ollama local models.
	LeanModeForOllama = &LeanMode{
		Enabled:           true,
		Level:             LeanLevelModerate,
		MaxContextTokens:  2048,
		MaxResponseTokens: 512,
		CompactPrompts:    true,
		BatchSize:         4,
	}

	// LeanModeForLMStudio is optimized for LM Studio local models.
	LeanModeForLMStudio = &LeanMode{
		Enabled:           true,
		Level:             LeanLevelLight,
		MaxContextTokens:  4096,
		MaxResponseTokens: 1024,
	}

	// LeanModeForLlamaCpp is optimized for llama.cpp direct usage.
	LeanModeForLlamaCpp = &LeanMode{
		Enabled:           true,
		Level:             LeanLevelAggressive,
		MaxContextTokens:  512,
		MaxResponseTokens: 256,
		CompactPrompts:    true,
		DisableHistory:    true,
		StreamingDisabled: true,
		CacheDisabled:     true,
	}
)
