package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockAgent implements AgentInterface for testing.
type mockAgent struct {
	processResponse string
	processError    error
	tools           *mockToolExecutor
}

func (m *mockAgent) ProcessWithSession(ctx context.Context, sessionID, content string) (string, error) {
	if m.processError != nil {
		return "", m.processError
	}
	return m.processResponse, nil
}

func (m *mockAgent) Tools() ToolExecutor {
	return m.tools
}

// mockToolExecutor implements ToolExecutor for testing.
type mockToolExecutor struct {
	executeResult string
	executeError  error
}

func (m *mockToolExecutor) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	if m.executeError != nil {
		return "", m.executeError
	}
	return m.executeResult, nil
}

func TestExecutor_Execute_SendMessage(t *testing.T) {
	agent := &mockAgent{
		processResponse: "Hello from the agent!",
	}
	executor := NewExecutor(ExecutorConfig{Agent: agent})

	job := &Job{
		ID:   "test-job",
		Name: "Test Send Message",
		Action: Action{
			Type:      ActionTypeSendMessage,
			SessionID: "test-session",
			Message:   "Hello, world!",
		},
	}

	result := executor.Execute(context.Background(), job)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}

	output, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatal("expected map output")
	}

	if output["response"] != "Hello from the agent!" {
		t.Errorf("unexpected response: %v", output["response"])
	}

	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestExecutor_Execute_SendMessage_NoAgent(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{})

	job := &Job{
		ID:   "test-job",
		Name: "Test Send Message",
		Action: Action{
			Type:      ActionTypeSendMessage,
			SessionID: "test-session",
			Message:   "Hello, world!",
		},
	}

	result := executor.Execute(context.Background(), job)

	if result.Success {
		t.Error("expected failure when agent not configured")
	}

	if result.Error != "agent not configured for send_message action" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestExecutor_Execute_SendMessage_Error(t *testing.T) {
	agent := &mockAgent{
		processError: fmt.Errorf("session not found"),
	}
	executor := NewExecutor(ExecutorConfig{Agent: agent})

	job := &Job{
		ID:   "test-job",
		Name: "Test Send Message",
		Action: Action{
			Type:      ActionTypeSendMessage,
			SessionID: "invalid-session",
			Message:   "Hello!",
		},
	}

	result := executor.Execute(context.Background(), job)

	if result.Success {
		t.Error("expected failure")
	}

	if result.Error != "send message failed: session not found" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestExecutor_Execute_CallWebhook(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Error("missing custom header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	executor := NewExecutor(ExecutorConfig{})

	job := &Job{
		ID:   "test-job",
		Name: "Test Webhook",
		Action: Action{
			Type:       ActionTypeCallWebhook,
			WebhookURL: server.URL,
			WebhookHeaders: map[string]string{
				"X-Custom-Header": "custom-value",
			},
			WebhookBody: `{"test":"data"}`,
		},
	}

	result := executor.Execute(context.Background(), job)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}

	output, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatal("expected map output")
	}

	if output["status_code"] != 200 {
		t.Errorf("unexpected status code: %v", output["status_code"])
	}
}

func TestExecutor_Execute_CallWebhook_Error(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	executor := NewExecutor(ExecutorConfig{})

	job := &Job{
		ID:   "test-job",
		Name: "Test Webhook Error",
		Action: Action{
			Type:       ActionTypeCallWebhook,
			WebhookURL: server.URL,
		},
	}

	result := executor.Execute(context.Background(), job)

	if result.Success {
		t.Error("expected failure for 500 response")
	}

	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestExecutor_Execute_CallWebhook_DefaultMethod(t *testing.T) {
	// Create test server that validates method
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected default POST method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewExecutor(ExecutorConfig{})

	job := &Job{
		ID:   "test-job",
		Name: "Test Default Method",
		Action: Action{
			Type:       ActionTypeCallWebhook,
			WebhookURL: server.URL,
			// WebhookMethod not set - should default to POST
		},
	}

	result := executor.Execute(context.Background(), job)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
}

func TestExecutor_Execute_CallTool(t *testing.T) {
	agent := &mockAgent{
		tools: &mockToolExecutor{
			executeResult: `{"result":"tool output"}`,
		},
	}
	executor := NewExecutor(ExecutorConfig{Agent: agent})

	job := &Job{
		ID:   "test-job",
		Name: "Test Call Tool",
		Action: Action{
			Type:     ActionTypeCallTool,
			ToolName: "test_tool",
			ToolParams: map[string]any{
				"param1": "value1",
			},
		},
	}

	result := executor.Execute(context.Background(), job)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}

	output, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatal("expected map output")
	}

	if output["tool_name"] != "test_tool" {
		t.Errorf("unexpected tool name: %v", output["tool_name"])
	}
}

func TestExecutor_Execute_CallTool_NoAgent(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{})

	job := &Job{
		ID:   "test-job",
		Name: "Test Call Tool",
		Action: Action{
			Type:     ActionTypeCallTool,
			ToolName: "test_tool",
		},
	}

	result := executor.Execute(context.Background(), job)

	if result.Success {
		t.Error("expected failure when agent not configured")
	}

	if result.Error != "agent not configured for call_tool action" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestExecutor_Execute_CallTool_Error(t *testing.T) {
	agent := &mockAgent{
		tools: &mockToolExecutor{
			executeError: fmt.Errorf("tool not found"),
		},
	}
	executor := NewExecutor(ExecutorConfig{Agent: agent})

	job := &Job{
		ID:   "test-job",
		Name: "Test Call Tool",
		Action: Action{
			Type:     ActionTypeCallTool,
			ToolName: "nonexistent_tool",
		},
	}

	result := executor.Execute(context.Background(), job)

	if result.Success {
		t.Error("expected failure")
	}

	if result.Error != "tool execution failed: tool not found" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestExecutor_Execute_UnknownAction(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{})

	job := &Job{
		ID:   "test-job",
		Name: "Test Unknown Action",
		Action: Action{
			Type: ActionType("unknown_action"),
		},
	}

	result := executor.Execute(context.Background(), job)

	if result.Success {
		t.Error("expected failure for unknown action")
	}

	if result.Error != "unknown action type: unknown_action" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestExecutor_Execute_TimingFields(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{})

	job := &Job{
		ID:   "test-job",
		Name: "Test Timing",
		Action: Action{
			Type: ActionType("unknown"), // Will fail quickly
		},
	}

	before := time.Now()
	result := executor.Execute(context.Background(), job)
	after := time.Now()

	if result.StartedAt.Before(before) || result.StartedAt.After(after) {
		t.Error("StartedAt not in expected range")
	}

	if result.FinishedAt.Before(result.StartedAt) {
		t.Error("FinishedAt before StartedAt")
	}

	if result.Duration < 0 {
		t.Error("expected non-negative duration")
	}
}
