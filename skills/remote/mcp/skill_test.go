// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"testing"
)

// TestSetSecrets_MergePrecedence confirms injected secrets merge into the
// subprocess env and override statically configured Env, without mutating the
// caller's map or a shared base config.
func TestSetSecrets_MergePrecedence(t *testing.T) {
	base := Config{
		Name:    "github",
		Command: []string{"echo"},
		Env:     map[string]string{"KEEP": "1", "TOKEN": "static"},
	}

	s := NewSkill(base)
	injected := map[string]string{"TOKEN": "secret", "EXTRA": "e"}
	s.SetSecrets(injected)

	if got := s.config.Env["TOKEN"]; got != "secret" {
		t.Errorf("TOKEN = %q, want secret (injected overrides static)", got)
	}
	if got := s.config.Env["KEEP"]; got != "1" {
		t.Errorf("KEEP = %q, want 1 (static preserved)", got)
	}
	if got := s.config.Env["EXTRA"]; got != "e" {
		t.Errorf("EXTRA = %q, want e (injected added)", got)
	}

	// The caller's injected map must not be mutated.
	if len(injected) != 2 {
		t.Errorf("injected map mutated: %v", injected)
	}
	// A second skill from the same base config must be unaffected (base map not
	// mutated) — the basis for per-agent isolation over shared BaseOptions.
	s2 := NewSkill(base)
	if got := s2.config.Env["TOKEN"]; got != "static" {
		t.Errorf("second skill TOKEN = %q, want static (base config not mutated)", got)
	}
}

// TestSetSecrets_Empty is a no-op that leaves Env untouched.
func TestSetSecrets_Empty(t *testing.T) {
	s := NewSkill(Config{Name: "x", Command: []string{"echo"}, Env: map[string]string{"A": "1"}})
	s.SetSecrets(nil)
	if len(s.config.Env) != 1 || s.config.Env["A"] != "1" {
		t.Errorf("Env = %v, want {A:1}", s.config.Env)
	}
}

func TestNewSkill(t *testing.T) {
	skill := NewSkill(Config{
		Name:    "test",
		Command: []string{"echo", "hello"},
	})

	if skill == nil {
		t.Fatal("expected non-nil skill")
	}

	if skill.Name() != "test" {
		t.Fatalf("expected name 'test', got %q", skill.Name())
	}

	if skill.Description() != "MCP skill: test" {
		t.Fatalf("unexpected description: %q", skill.Description())
	}
}

func TestNewSkillWithDescription(t *testing.T) {
	skill := NewSkill(Config{
		Name:        "github",
		Description: "GitHub operations via MCP",
		Command:     []string{"npx", "-y", "@modelcontextprotocol/server-github"},
	})

	if skill.Description() != "GitHub operations via MCP" {
		t.Fatalf("unexpected description: %q", skill.Description())
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{
		Name:    "test",
		Command: []string{"echo"},
	}
	cfg.setDefaults()

	if cfg.Description == "" {
		t.Error("expected description to be set")
	}

	if cfg.ClientName != "omniagent" {
		t.Errorf("expected ClientName 'omniagent', got %q", cfg.ClientName)
	}

	if cfg.ClientVersion != "v1.0.0" {
		t.Errorf("expected ClientVersion 'v1.0.0', got %q", cfg.ClientVersion)
	}
}

func TestToolsBeforeInit(t *testing.T) {
	skill := NewSkill(Config{
		Name:    "test",
		Command: []string{"echo"},
	})

	tools := skill.Tools()
	if len(tools) != 0 {
		t.Fatalf("expected 0 tools before init, got %d", len(tools))
	}
}

func TestCloseBeforeInit(t *testing.T) {
	skill := NewSkill(Config{
		Name:    "test",
		Command: []string{"echo"},
	})

	// Close should not error even if never initialized
	if err := skill.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestLazyConnectInit(t *testing.T) {
	skill := NewSkill(Config{
		Name:        "test",
		Command:     []string{"echo"},
		LazyConnect: true,
	})

	// With lazy connect, Init should return immediately
	ctx := t.Context()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Session should not be created yet
	skill.mu.RLock()
	hasSession := skill.session != nil
	skill.mu.RUnlock()

	if hasSession {
		t.Error("expected no session with lazy connect")
	}
}

func TestConvertInputSchema(t *testing.T) {
	skill := NewSkill(Config{
		Name:    "test",
		Command: []string{"echo"},
	})

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
			"limit": map[string]any{
				"type":    "integer",
				"default": 10,
			},
		},
		"required": []any{"query"},
	}

	params := skill.convertInputSchema(schema)

	if params == nil {
		t.Fatal("expected non-nil params")
	}

	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}

	queryParam, ok := params["query"]
	if !ok {
		t.Fatal("expected 'query' param")
	}

	if queryParam.Type != "string" {
		t.Errorf("expected type 'string', got %q", queryParam.Type)
	}

	if !queryParam.Required {
		t.Error("expected 'query' to be required")
	}

	limitParam, ok := params["limit"]
	if !ok {
		t.Fatal("expected 'limit' param")
	}

	if limitParam.Required {
		t.Error("expected 'limit' to not be required")
	}

	// The default value should be 10 (could be int or float64 depending on source)
	switch v := limitParam.Default.(type) {
	case int:
		if v != 10 {
			t.Errorf("expected default 10, got %d", v)
		}
	case float64:
		if v != 10.0 {
			t.Errorf("expected default 10.0, got %f", v)
		}
	default:
		t.Errorf("expected numeric default, got %T", limitParam.Default)
	}
}

func TestConvertInputSchemaNil(t *testing.T) {
	skill := NewSkill(Config{
		Name:    "test",
		Command: []string{"echo"},
	})

	params := skill.convertInputSchema(nil)
	if params != nil {
		t.Errorf("expected nil params for nil schema, got %v", params)
	}
}

func TestConvertInputSchemaInvalid(t *testing.T) {
	skill := NewSkill(Config{
		Name:    "test",
		Command: []string{"echo"},
	})

	// Invalid schema type
	params := skill.convertInputSchema("not a map")
	if params != nil {
		t.Errorf("expected nil params for invalid schema, got %v", params)
	}
}
