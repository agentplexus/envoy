// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package profiles

import (
	"context"
	"testing"

	"github.com/plexusone/omnillm-core/provider"
)

func TestProfileRegistry_Register(t *testing.T) {
	r := NewProfileRegistry()

	profile := &BootstrapProfile{
		Name:        "test",
		Description: "Test profile",
	}

	err := r.Register(profile)
	if err != nil {
		t.Errorf("Register failed: %v", err)
	}

	// Duplicate registration should fail
	err = r.Register(profile)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}

	// Empty name should fail
	err = r.Register(&BootstrapProfile{})
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

func TestProfileRegistry_Get(t *testing.T) {
	r := NewProfileRegistry()
	if err := r.Register(&BootstrapProfile{Name: "test"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	p, ok := r.Get("test")
	if !ok {
		t.Error("Expected to find profile")
	}
	if p.Name != "test" {
		t.Errorf("Expected name 'test', got %q", p.Name)
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent profile")
	}
}

func TestProfileRegistry_List(t *testing.T) {
	r := NewProfileRegistry()
	if err := r.Register(&BootstrapProfile{Name: "alpha"}); err != nil {
		t.Fatalf("Register alpha failed: %v", err)
	}
	if err := r.Register(&BootstrapProfile{Name: "beta"}); err != nil {
		t.Fatalf("Register beta failed: %v", err)
	}

	names := r.List()
	if len(names) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(names))
	}
}

func TestProfileRegistry_Unregister(t *testing.T) {
	r := NewProfileRegistry()
	if err := r.Register(&BootstrapProfile{Name: "test"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	r.Unregister("test")

	_, ok := r.Get("test")
	if ok {
		t.Error("Expected profile to be unregistered")
	}
}

func TestBootstrapProfile_FilterTools(t *testing.T) {
	tools := []provider.Tool{
		{Function: provider.ToolSpec{Name: "shell"}},
		{Function: provider.ToolSpec{Name: "read"}},
		{Function: provider.ToolSpec{Name: "write"}},
		{Function: provider.ToolSpec{Name: "browser"}},
	}

	// Test deny list
	p := &BootstrapProfile{
		DeniedTools: []string{"shell", "browser"},
	}
	filtered := p.FilterTools(tools)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(filtered))
	}
	for _, tool := range filtered {
		if tool.Function.Name == "shell" || tool.Function.Name == "browser" {
			t.Errorf("Tool %s should be denied", tool.Function.Name)
		}
	}

	// Test allow list
	p = &BootstrapProfile{
		AllowedTools: []string{"read", "write"},
	}
	filtered = p.FilterTools(tools)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(filtered))
	}

	// Test wildcard allow with specific deny
	p = &BootstrapProfile{
		AllowedTools: []string{"*"},
		DeniedTools:  []string{"shell"},
	}
	filtered = p.FilterTools(tools)
	if len(filtered) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(filtered))
	}
}

func TestBootstrapProfile_BuildSystemPrompt(t *testing.T) {
	base := "You are an assistant."

	// Override
	p := &BootstrapProfile{
		SystemPrompt: "You are a code assistant.",
	}
	result := p.BuildSystemPrompt(base)
	if result != "You are a code assistant." {
		t.Errorf("Expected override, got %q", result)
	}

	// Prefix only
	p = &BootstrapProfile{
		SystemPromptPrefix: "IMPORTANT:",
	}
	result = p.BuildSystemPrompt(base)
	if result != "IMPORTANT:\n\nYou are an assistant." {
		t.Errorf("Expected prefixed, got %q", result)
	}

	// Suffix only
	p = &BootstrapProfile{
		SystemPromptSuffix: "Be concise.",
	}
	result = p.BuildSystemPrompt(base)
	if result != "You are an assistant.\n\nBe concise." {
		t.Errorf("Expected suffixed, got %q", result)
	}

	// Both prefix and suffix
	p = &BootstrapProfile{
		SystemPromptPrefix: "IMPORTANT:",
		SystemPromptSuffix: "Be concise.",
	}
	result = p.BuildSystemPrompt(base)
	expected := "IMPORTANT:\n\nYou are an assistant.\n\nBe concise."
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestBootstrapProfile_GetToolPolicy(t *testing.T) {
	p := &BootstrapProfile{
		ToolPolicies: map[string]ToolPolicy{
			"shell": {
				Enabled:              true,
				RateLimit:            10,
				RequiresConfirmation: true,
			},
		},
	}

	policy, ok := p.GetToolPolicy("shell")
	if !ok {
		t.Error("Expected to find policy")
	}
	if policy.RateLimit != 10 {
		t.Errorf("Expected rate limit 10, got %d", policy.RateLimit)
	}

	_, ok = p.GetToolPolicy("read")
	if ok {
		t.Error("Expected not to find policy")
	}
}

func TestBootstrapProfile_Clone(t *testing.T) {
	temp := 0.7
	tokens := 1000
	original := &BootstrapProfile{
		Name:               "original",
		Description:        "Original profile",
		SystemPrompt:       "System prompt",
		AllowedTools:       []string{"read", "write"},
		DeniedTools:        []string{"shell"},
		MaxContextMessages: 50,
		Temperature:        &temp,
		MaxTokens:          &tokens,
		ToolPolicies: map[string]ToolPolicy{
			"shell": {RateLimit: 5},
		},
		Metadata: map[string]any{
			"key": "value",
		},
	}

	clone := original.Clone()

	// Verify values copied
	if clone.Name != original.Name {
		t.Errorf("Name not cloned")
	}
	if clone.MaxContextMessages != original.MaxContextMessages {
		t.Errorf("MaxContextMessages not cloned")
	}
	if *clone.Temperature != *original.Temperature {
		t.Errorf("Temperature not cloned")
	}

	// Verify deep copy (modifications don't affect original)
	clone.AllowedTools[0] = "modified"
	if original.AllowedTools[0] == "modified" {
		t.Error("Clone shares AllowedTools slice")
	}

	clone.ToolPolicies["shell"] = ToolPolicy{RateLimit: 999}
	if original.ToolPolicies["shell"].RateLimit == 999 {
		t.Error("Clone shares ToolPolicies map")
	}
}

func TestProfileSession_ActivateDeactivate(t *testing.T) {
	initCalled := false
	closeCalled := false

	profile := &BootstrapProfile{
		Name: "test",
		OnInit: func(ctx context.Context) error {
			initCalled = true
			return nil
		},
		OnClose: func(ctx context.Context) error {
			closeCalled = true
			return nil
		},
	}

	session := &ProfileSession{Profile: profile}
	ctx := context.Background()

	// Activate
	err := session.Activate(ctx)
	if err != nil {
		t.Errorf("Activate failed: %v", err)
	}
	if !initCalled {
		t.Error("OnInit not called")
	}
	if !session.Active {
		t.Error("Session not marked active")
	}

	// Double activate should be no-op
	initCalled = false
	if err := session.Activate(ctx); err != nil {
		t.Errorf("Double activate failed: %v", err)
	}
	if initCalled {
		t.Error("OnInit called again on double activate")
	}

	// Deactivate
	err = session.Deactivate(ctx)
	if err != nil {
		t.Errorf("Deactivate failed: %v", err)
	}
	if !closeCalled {
		t.Error("OnClose not called")
	}
	if session.Active {
		t.Error("Session still marked active")
	}

	// Double deactivate should be no-op
	closeCalled = false
	if err := session.Deactivate(ctx); err != nil {
		t.Errorf("Double deactivate failed: %v", err)
	}
	if closeCalled {
		t.Error("OnClose called again on double deactivate")
	}
}

func TestPredefinedProfiles(t *testing.T) {
	// Verify predefined profiles exist and have valid names
	profiles := []*BootstrapProfile{
		DefaultProfile,
		RestrictedProfile,
		ReadOnlyProfile,
		CodeAssistantProfile,
	}

	for _, p := range profiles {
		if p.Name == "" {
			t.Error("Predefined profile has empty name")
		}
		if p.Description == "" {
			t.Errorf("Profile %q has empty description", p.Name)
		}
	}

	// Verify specific profile configurations
	if len(RestrictedProfile.DeniedTools) == 0 {
		t.Error("RestrictedProfile should have denied tools")
	}

	if len(ReadOnlyProfile.AllowedTools) == 0 {
		t.Error("ReadOnlyProfile should have allowed tools")
	}
}
