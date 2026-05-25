// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package profiles

import (
	"testing"
)

func TestLeanLevel_String(t *testing.T) {
	tests := []struct {
		level    LeanLevel
		expected string
	}{
		{LeanLevelOff, "off"},
		{LeanLevelLight, "light"},
		{LeanLevelModerate, "moderate"},
		{LeanLevelAggressive, "aggressive"},
		{LeanLevel(99), "unknown(99)"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if got := tc.level.String(); got != tc.expected {
				t.Errorf("String() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestParseLeanLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LeanLevel
		wantErr  bool
	}{
		{"off", LeanLevelOff, false},
		{"none", LeanLevelOff, false},
		{"light", LeanLevelLight, false},
		{"low", LeanLevelLight, false},
		{"moderate", LeanLevelModerate, false},
		{"medium", LeanLevelModerate, false},
		{"aggressive", LeanLevelAggressive, false},
		{"high", LeanLevelAggressive, false},
		{"invalid", LeanLevelOff, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseLeanLevel(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseLeanLevel(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
				return
			}
			if !tc.wantErr && got != tc.expected {
				t.Errorf("ParseLeanLevel(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestNewLeanMode(t *testing.T) {
	// Test off level
	m := NewLeanMode(LeanLevelOff)
	if m.Enabled {
		t.Error("LeanLevelOff should not be enabled")
	}

	// Test light level
	m = NewLeanMode(LeanLevelLight)
	if !m.Enabled {
		t.Error("LeanLevelLight should be enabled")
	}
	if m.MaxContextTokens != 2048 {
		t.Errorf("Expected MaxContextTokens 2048, got %d", m.MaxContextTokens)
	}
	if m.MaxResponseTokens != 512 {
		t.Errorf("Expected MaxResponseTokens 512, got %d", m.MaxResponseTokens)
	}

	// Test moderate level
	m = NewLeanMode(LeanLevelModerate)
	if m.MaxContextTokens != 1024 {
		t.Errorf("Expected MaxContextTokens 1024, got %d", m.MaxContextTokens)
	}
	if !m.CompactPrompts {
		t.Error("Moderate level should enable compact prompts")
	}

	// Test aggressive level
	m = NewLeanMode(LeanLevelAggressive)
	if m.MaxContextTokens != 512 {
		t.Errorf("Expected MaxContextTokens 512, got %d", m.MaxContextTokens)
	}
	if !m.DisableHistory {
		t.Error("Aggressive level should disable history")
	}
	if !m.SkipSystemPrompt {
		t.Error("Aggressive level should skip system prompt")
	}
}

func TestLeanMode_Apply(t *testing.T) {
	// Test with disabled lean mode
	profile := &BootstrapProfile{Name: "test"}
	m := NewLeanMode(LeanLevelOff)
	m.Apply(profile)
	if profile.MaxContextTokens != 0 {
		t.Error("Disabled lean mode should not modify profile")
	}

	// Test with light mode
	profile = &BootstrapProfile{Name: "test"}
	m = NewLeanMode(LeanLevelLight)
	m.Apply(profile)
	if profile.MaxContextTokens != 2048 {
		t.Errorf("Expected MaxContextTokens 2048, got %d", profile.MaxContextTokens)
	}

	// Test DisableTools
	profile = &BootstrapProfile{Name: "test"}
	m = &LeanMode{Enabled: true, Level: LeanLevelLight, DisableTools: true}
	m.Apply(profile)
	if len(profile.DeniedTools) != 1 || profile.DeniedTools[0] != "*" {
		t.Error("DisableTools should set DeniedTools to [*]")
	}

	// Test ToolAllowlist
	profile = &BootstrapProfile{Name: "test"}
	m = &LeanMode{Enabled: true, Level: LeanLevelLight, ToolAllowlist: []string{"read", "write"}}
	m.Apply(profile)
	if len(profile.AllowedTools) != 2 {
		t.Errorf("Expected 2 allowed tools, got %d", len(profile.AllowedTools))
	}

	// Test SkipSystemPrompt
	profile = &BootstrapProfile{
		Name:               "test",
		SystemPrompt:       "Original",
		SystemPromptPrefix: "Prefix",
		SystemPromptSuffix: "Suffix",
	}
	m = &LeanMode{Enabled: true, Level: LeanLevelAggressive, SkipSystemPrompt: true}
	m.Apply(profile)
	if profile.SystemPrompt != "" {
		t.Error("SkipSystemPrompt should clear SystemPrompt")
	}
	if profile.SystemPromptPrefix != "" {
		t.Error("SkipSystemPrompt should clear SystemPromptPrefix")
	}
	if profile.SystemPromptSuffix != "" {
		t.Error("SkipSystemPrompt should clear SystemPromptSuffix")
	}
}

func TestLeanMode_EstimateMemorySavings(t *testing.T) {
	tests := []struct {
		level    LeanLevel
		expected float64
	}{
		{LeanLevelOff, 0},
		{LeanLevelLight, 0.2},
		{LeanLevelModerate, 0.4},
		{LeanLevelAggressive, 0.6},
	}

	for _, tc := range tests {
		t.Run(tc.level.String(), func(t *testing.T) {
			m := NewLeanMode(tc.level)
			if got := m.EstimateMemorySavings(); got != tc.expected {
				t.Errorf("EstimateMemorySavings() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestLeanModeConfig_GetForModel(t *testing.T) {
	config := &LeanModeConfig{
		Level: LeanLevelLight,
		ModelSpecific: map[string]*LeanMode{
			"llama-7b": {Enabled: true, Level: LeanLevelAggressive, MaxContextTokens: 256},
		},
	}

	// Model-specific settings
	m := config.GetForModel("llama-7b")
	if m.MaxContextTokens != 256 {
		t.Errorf("Expected model-specific MaxContextTokens 256, got %d", m.MaxContextTokens)
	}

	// Default level
	m = config.GetForModel("gpt-4")
	if m.Level != LeanLevelLight {
		t.Errorf("Expected default level light, got %v", m.Level)
	}
}

func TestPredefinedLeanModes(t *testing.T) {
	// Verify disabled mode
	if LeanModeDisabled.Enabled {
		t.Error("LeanModeDisabled should not be enabled")
	}

	// Verify Ollama mode
	if !LeanModeForOllama.Enabled {
		t.Error("LeanModeForOllama should be enabled")
	}
	if LeanModeForOllama.MaxContextTokens != 2048 {
		t.Errorf("LeanModeForOllama MaxContextTokens = %d, want 2048", LeanModeForOllama.MaxContextTokens)
	}

	// Verify LM Studio mode
	if LeanModeForLMStudio.Level != LeanLevelLight {
		t.Errorf("LeanModeForLMStudio Level = %v, want light", LeanModeForLMStudio.Level)
	}

	// Verify llama.cpp mode
	if LeanModeForLlamaCpp.Level != LeanLevelAggressive {
		t.Errorf("LeanModeForLlamaCpp Level = %v, want aggressive", LeanModeForLlamaCpp.Level)
	}
	if !LeanModeForLlamaCpp.DisableHistory {
		t.Error("LeanModeForLlamaCpp should disable history")
	}
}
