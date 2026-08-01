// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNewToolRegistry(t *testing.T) {
	r := NewToolRegistry()
	if r == nil {
		t.Fatal("NewToolRegistry returned nil")
	}

	if r.tools == nil {
		t.Error("tools map not initialized")
	}
}

func TestToolRegistry_Register(t *testing.T) {
	r := NewToolRegistry()
	tool := NewBaseTool("test_tool", "A test tool", nil, nil)

	r.Register(tool)

	if _, ok := r.Get("test_tool"); !ok {
		t.Error("tool not registered")
	}
}

func TestToolRegistry_Unregister(t *testing.T) {
	r := NewToolRegistry()
	tool := NewBaseTool("test_tool", "A test tool", nil, nil)

	r.Register(tool)
	r.Unregister("test_tool")

	if _, ok := r.Get("test_tool"); ok {
		t.Error("tool still registered after unregister")
	}
}

func TestToolRegistry_Get(t *testing.T) {
	r := NewToolRegistry()
	tool := NewBaseTool("test_tool", "A test tool", nil, nil)

	r.Register(tool)

	got, ok := r.Get("test_tool")
	if !ok {
		t.Fatal("tool not found")
	}

	if got.Name() != "test_tool" {
		t.Errorf("got tool name %q, want %q", got.Name(), "test_tool")
	}
}

func TestToolRegistry_Get_NotFound(t *testing.T) {
	r := NewToolRegistry()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected tool not found")
	}
}

func TestToolRegistry_List(t *testing.T) {
	r := NewToolRegistry()
	r.Register(NewBaseTool("tool1", "Tool 1", nil, nil))
	r.Register(NewBaseTool("tool2", "Tool 2", nil, nil))
	r.Register(NewBaseTool("tool3", "Tool 3", nil, nil))

	names := r.List()
	if len(names) != 3 {
		t.Errorf("expected 3 tools, got %d", len(names))
	}

	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	for _, expected := range []string{"tool1", "tool2", "tool3"} {
		if !nameMap[expected] {
			t.Errorf("expected tool %q in list", expected)
		}
	}
}

func TestToolRegistry_GetTools(t *testing.T) {
	r := NewToolRegistry()
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query",
			},
		},
	}

	r.Register(NewBaseTool("search", "Search the web", params, nil))

	tools := r.GetTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]
	if tool.Type != "function" {
		t.Errorf("expected type 'function', got %q", tool.Type)
	}

	if tool.Function.Name != "search" {
		t.Errorf("expected name 'search', got %q", tool.Function.Name)
	}

	if tool.Function.Description != "Search the web" {
		t.Errorf("expected description 'Search the web', got %q", tool.Function.Description)
	}
}

func TestToolRegistry_Execute(t *testing.T) {
	r := NewToolRegistry()

	handler := func(ctx context.Context, args json.RawMessage) (string, error) {
		var input struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			return "", err
		}
		return "Hello, " + input.Name, nil
	}

	r.Register(NewBaseTool("greet", "Greet someone", nil, handler))

	result, err := r.Execute(context.Background(), "greet", json.RawMessage(`{"name": "World"}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != "Hello, World" {
		t.Errorf("expected 'Hello, World', got %q", result)
	}
}

func TestToolRegistry_Execute_NotFound(t *testing.T) {
	r := NewToolRegistry()

	_, err := r.Execute(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}

	var notFoundErr *ToolNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("expected ToolNotFoundError, got %T", err)
	}

	if notFoundErr.Name != "nonexistent" {
		t.Errorf("expected tool name 'nonexistent', got %q", notFoundErr.Name)
	}
}

func TestToolNotFoundError_Error(t *testing.T) {
	err := &ToolNotFoundError{Name: "missing_tool"}
	expected := "tool not found: missing_tool"

	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestNewBaseTool(t *testing.T) {
	params := map[string]any{"type": "object"}
	handler := func(ctx context.Context, args json.RawMessage) (string, error) {
		return "result", nil
	}

	tool := NewBaseTool("my_tool", "My description", params, handler)

	if tool.Name() != "my_tool" {
		t.Errorf("expected name 'my_tool', got %q", tool.Name())
	}

	if tool.Description() != "My description" {
		t.Errorf("expected description 'My description', got %q", tool.Description())
	}

	if tool.Parameters()["type"] != "object" {
		t.Error("expected parameters to contain type: object")
	}
}

func TestBaseTool_Execute(t *testing.T) {
	handler := func(ctx context.Context, args json.RawMessage) (string, error) {
		return "executed", nil
	}

	tool := NewBaseTool("test", "Test tool", nil, handler)

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != "executed" {
		t.Errorf("expected 'executed', got %q", result)
	}
}

func TestBaseTool_Execute_WithError(t *testing.T) {
	expectedErr := errors.New("execution failed")
	handler := func(ctx context.Context, args json.RawMessage) (string, error) {
		return "", expectedErr
	}

	tool := NewBaseTool("failing_tool", "A tool that fails", nil, handler)

	_, err := tool.Execute(context.Background(), nil)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
