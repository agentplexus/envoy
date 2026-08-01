// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package agent

import (
	"context"
	"encoding/json"
	"testing"

	agentctx "github.com/plexusone/omniagent/context"
)

func TestWithTool(t *testing.T) {
	tool := NewBaseTool("test_tool", "Test", nil, nil)

	a := &Agent{
		tools: NewToolRegistry(),
	}

	opt := WithTool(tool)
	if err := opt(a); err != nil {
		t.Fatalf("WithTool failed: %v", err)
	}

	if _, ok := a.tools.Get("test_tool"); !ok {
		t.Error("tool not registered")
	}
}

func TestWithContextEngine(t *testing.T) {
	engine := agentctx.New(agentctx.Config{
		MaxMessages: 10,
	})

	a := &Agent{}

	opt := WithContextEngine(engine)
	if err := opt(a); err != nil {
		t.Fatalf("WithContextEngine failed: %v", err)
	}

	if a.contextEngine == nil {
		t.Error("context engine not set")
	}
}

func TestWithContextConfig(t *testing.T) {
	a := &Agent{}

	opt := WithContextConfig(agentctx.Config{
		MaxMessages: 25,
	})
	if err := opt(a); err != nil {
		t.Fatalf("WithContextConfig failed: %v", err)
	}

	if a.contextEngine == nil {
		t.Error("context engine not created")
	}
}

func TestWithMaxMessages(t *testing.T) {
	a := &Agent{}

	opt := WithMaxMessages(50)
	if err := opt(a); err != nil {
		t.Fatalf("WithMaxMessages failed: %v", err)
	}

	if a.contextEngine == nil {
		t.Error("context engine not created")
	}
}

func TestWithSkillDirs(t *testing.T) {
	a := &Agent{}

	opt := WithSkillDirs("./skills", "/opt/skills")
	if err := opt(a); err != nil {
		t.Fatalf("WithSkillDirs failed: %v", err)
	}

	if len(a.skillDirs) != 2 {
		t.Errorf("expected 2 skill dirs, got %d", len(a.skillDirs))
	}

	if a.skillDirs[0] != "./skills" {
		t.Errorf("expected './skills', got %q", a.skillDirs[0])
	}
}

func TestWithSkillIncludes(t *testing.T) {
	a := &Agent{}

	opt := WithSkillIncludes("github", "weather")
	if err := opt(a); err != nil {
		t.Fatalf("WithSkillIncludes failed: %v", err)
	}

	if len(a.skillIncludes) != 2 {
		t.Errorf("expected 2 includes, got %d", len(a.skillIncludes))
	}
}

func TestWithSkillExcludes(t *testing.T) {
	a := &Agent{}

	opt := WithSkillExcludes("slack", "trello")
	if err := opt(a); err != nil {
		t.Fatalf("WithSkillExcludes failed: %v", err)
	}

	if len(a.skillExcludes) != 2 {
		t.Errorf("expected 2 excludes, got %d", len(a.skillExcludes))
	}
}

func TestMultipleOptions(t *testing.T) {
	a := &Agent{
		tools: NewToolRegistry(),
	}

	tool1 := NewBaseTool("tool1", "Tool 1", nil, func(ctx context.Context, args json.RawMessage) (string, error) {
		return "result1", nil
	})
	tool2 := NewBaseTool("tool2", "Tool 2", nil, func(ctx context.Context, args json.RawMessage) (string, error) {
		return "result2", nil
	})

	opts := []Option{
		WithTool(tool1),
		WithTool(tool2),
		WithMaxMessages(100),
	}

	for _, opt := range opts {
		if err := opt(a); err != nil {
			t.Fatalf("option failed: %v", err)
		}
	}

	// Verify all options applied
	if _, ok := a.tools.Get("tool1"); !ok {
		t.Error("tool1 not registered")
	}
	if _, ok := a.tools.Get("tool2"); !ok {
		t.Error("tool2 not registered")
	}
	if a.contextEngine == nil {
		t.Error("context engine not set")
	}
}
