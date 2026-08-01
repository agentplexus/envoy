// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/plexusone/omniagent/agent"
)

// mockTool is a simple test tool.
type mockTool struct {
	name   string
	result string
	err    error
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return "A test tool" }
func (t *mockTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}
func (t *mockTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return t.result, t.err
}

func TestNewToolsRPCHandler(t *testing.T) {
	handler := NewToolsRPCHandler(ToolsRPCConfig{})

	if handler.config.MaxRequestSize != 1<<20 {
		t.Errorf("Expected default MaxRequestSize 1MB, got %d", handler.config.MaxRequestSize)
	}

	if handler.config.Timeout != 30*1e9 {
		t.Errorf("Expected default Timeout 30s, got %v", handler.config.Timeout)
	}
}

func TestToolsRPCHandler_MethodNotAllowed(t *testing.T) {
	handler := NewToolsRPCHandler(ToolsRPCConfig{})

	req := httptest.NewRequest(http.MethodGet, "/tools/invoke", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestToolsRPCHandler_InvalidJSON(t *testing.T) {
	handler := NewToolsRPCHandler(ToolsRPCConfig{})

	req := httptest.NewRequest(http.MethodPost, "/tools/invoke", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestToolsRPCHandler_MissingToolName(t *testing.T) {
	handler := NewToolsRPCHandler(ToolsRPCConfig{})

	body := `{"arguments": {}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/invoke", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp ToolInvokeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error == "" {
		t.Error("Expected error message")
	}
}

func TestToolsRPCHandler_NoRegistry(t *testing.T) {
	handler := NewToolsRPCHandler(ToolsRPCConfig{})

	body := `{"tool": "test_tool", "arguments": {}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/invoke", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestToolsRPCHandler_ToolNotFound(t *testing.T) {
	registry := agent.NewToolRegistry()

	handler := NewToolsRPCHandler(ToolsRPCConfig{
		ToolRegistry: registry,
	})

	body := `{"tool": "nonexistent_tool", "arguments": {}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/invoke", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestToolsRPCHandler_SuccessfulInvocation(t *testing.T) {
	registry := agent.NewToolRegistry()
	registry.Register(&mockTool{
		name:   "test_tool",
		result: "success",
	})

	handler := NewToolsRPCHandler(ToolsRPCConfig{
		ToolRegistry: registry,
	})

	body := `{"tool": "test_tool", "arguments": {}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/invoke", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp ToolInvokeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Result != "success" {
		t.Errorf("Expected result 'success', got %q", resp.Result)
	}

	if resp.ToolName != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got %q", resp.ToolName)
	}

	if resp.DurationMs < 0 {
		t.Error("Duration should be non-negative")
	}
}

func TestToolsListHandler_MethodNotAllowed(t *testing.T) {
	handler := NewToolsListHandler(nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/tools/list", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestToolsListHandler_EmptyRegistry(t *testing.T) {
	handler := NewToolsListHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/tools/list", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp ToolsListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Tools == nil {
		t.Error("Expected empty tools array, got nil")
	}
}

func TestToolsListHandler_WithTools(t *testing.T) {
	registry := agent.NewToolRegistry()
	registry.Register(&mockTool{name: "tool1"})
	registry.Register(&mockTool{name: "tool2"})

	handler := NewToolsListHandler(registry, nil)

	req := httptest.NewRequest(http.MethodGet, "/tools/list", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp ToolsListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(resp.Tools))
	}
}

// errorMockTool returns an error when executed.
type errorMockTool struct {
	name string
	err  error
}

func (t *errorMockTool) Name() string        { return t.name }
func (t *errorMockTool) Description() string { return "A tool that errors" }
func (t *errorMockTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}
func (t *errorMockTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return "", t.err
}

func TestToolsRPCHandler_ToolExecutionError(t *testing.T) {
	registry := agent.NewToolRegistry()
	registry.Register(&errorMockTool{
		name: "error_tool",
		err:  context.DeadlineExceeded,
	})

	handler := NewToolsRPCHandler(ToolsRPCConfig{
		ToolRegistry: registry,
	})

	body := `{"tool": "error_tool", "arguments": {}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/invoke", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Tool execution errors return 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	var resp ToolInvokeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error == "" {
		t.Error("Expected error in response")
	}
}

func TestToolsRPCHandler_WithArguments(t *testing.T) {
	registry := agent.NewToolRegistry()
	registry.Register(&mockTool{
		name:   "test_tool",
		result: "done",
	})

	handler := NewToolsRPCHandler(ToolsRPCConfig{
		ToolRegistry: registry,
	})

	body := `{"tool": "test_tool", "arguments": {"key": "value", "number": 42}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/invoke", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestToolsRPCHandler_RequestTooLarge(t *testing.T) {
	handler := NewToolsRPCHandler(ToolsRPCConfig{
		MaxRequestSize: 10, // Very small limit
	})

	body := `{"tool": "test_tool", "arguments": {"data": "this is way too long for the limit"}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/invoke", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail due to size limit
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
