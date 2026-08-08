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
	executeCalls  int
}

func (m *mockToolExecutor) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	m.executeCalls++
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

func principalJob(actionType ActionType, principal string) *Job {
	job := &Job{
		ID:             "authority-job",
		Name:           "Authority Test",
		OwnerPrincipal: principal,
	}
	switch actionType {
	case ActionTypeCallTool:
		job.Action = Action{Type: ActionTypeCallTool, ToolName: "exec"}
	case ActionTypeSendMessage:
		job.Action = Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "hi"}
	case ActionTypeCallWebhook:
		job.Action = Action{Type: ActionTypeCallWebhook}
	}
	return job
}

func TestExecutor_Authority_RemovedPrincipalDeniesAllTools(t *testing.T) {
	tools := &mockToolExecutor{executeResult: "ran"}
	agent := &mockAgent{tools: tools}
	executor := NewExecutor(ExecutorConfig{
		Agent: agent,
		PrincipalResolver: func(ctx context.Context, principal string) bool {
			return false // the account was removed
		},
	})

	result := executor.Execute(context.Background(), principalJob(ActionTypeCallTool, "removed-account"))

	if result.Success {
		t.Fatal("expected fail-closed denial for removed principal")
	}
	if tools.executeCalls != 0 {
		t.Fatalf("tool was invoked %d times despite denial", tools.executeCalls)
	}
	if result.Error == "" {
		t.Error("denial must carry an explicit reason")
	}
}

func TestExecutor_Authority_NoResolverFailsClosed(t *testing.T) {
	tools := &mockToolExecutor{executeResult: "ran"}
	agent := &mockAgent{tools: tools}
	executor := NewExecutor(ExecutorConfig{Agent: agent}) // no resolver

	result := executor.Execute(context.Background(), principalJob(ActionTypeCallTool, "some-account"))

	if result.Success {
		t.Fatal("a principal-bearing job must be denied when no resolver can verify it")
	}
	if tools.executeCalls != 0 {
		t.Fatalf("tool was invoked %d times despite denial", tools.executeCalls)
	}
}

func TestExecutor_Authority_ConfiguredPrincipalExecutes(t *testing.T) {
	tools := &mockToolExecutor{executeResult: "ran"}
	agent := &mockAgent{tools: tools}
	executor := NewExecutor(ExecutorConfig{
		Agent: agent,
		PrincipalResolver: func(ctx context.Context, principal string) bool {
			return principal == "alive-account"
		},
	})

	result := executor.Execute(context.Background(), principalJob(ActionTypeCallTool, "alive-account"))

	if !result.Success {
		t.Fatalf("configured principal must execute normally, got error: %s", result.Error)
	}
	if tools.executeCalls != 1 {
		t.Fatalf("expected exactly 1 tool invocation, got %d", tools.executeCalls)
	}
}

func TestExecutor_Authority_LegacyJobWithoutPrincipalExecutes(t *testing.T) {
	tools := &mockToolExecutor{executeResult: "ran"}
	agent := &mockAgent{tools: tools}
	executor := NewExecutor(ExecutorConfig{
		Agent: agent,
		PrincipalResolver: func(ctx context.Context, principal string) bool {
			return false // resolver would deny anything it is asked about
		},
	})

	result := executor.Execute(context.Background(), principalJob(ActionTypeCallTool, ""))

	if !result.Success {
		t.Fatalf("legacy job without principal must keep working, got error: %s", result.Error)
	}
	if tools.executeCalls != 1 {
		t.Fatalf("expected exactly 1 tool invocation, got %d", tools.executeCalls)
	}
}

func TestExecutor_Authority_SendMessageDenied(t *testing.T) {
	agent := &mockAgent{processResponse: "should never be returned"}
	executor := NewExecutor(ExecutorConfig{
		Agent: agent,
		PrincipalResolver: func(ctx context.Context, principal string) bool {
			return false
		},
	})

	result := executor.Execute(context.Background(), principalJob(ActionTypeSendMessage, "removed-account"))

	if result.Success {
		t.Fatal("send_message under a removed principal must be denied")
	}
}
