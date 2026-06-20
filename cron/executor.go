package cron

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/grokify/mogo/log/slogutil"
)

// AgentInterface defines the methods the executor needs from the agent.
// This interface avoids import cycles by not depending on the agent package.
type AgentInterface interface {
	// ProcessWithSession processes a message using persistent session history.
	ProcessWithSession(ctx context.Context, sessionID, content string) (string, error)

	// Tools returns the tool registry for executing tools.
	Tools() ToolExecutor
}

// ToolExecutor is the interface for executing tools.
type ToolExecutor interface {
	// Execute executes a tool by name with JSON arguments.
	Execute(ctx context.Context, name string, args json.RawMessage) (string, error)
}

// Executor implements ExecutionHandler to perform job actions.
type Executor struct {
	agent      AgentInterface
	httpClient *http.Client
}

// ExecutorConfig configures the executor.
type ExecutorConfig struct {
	// Agent is the agent instance for send_message and call_tool actions.
	// Can be nil if those action types won't be used.
	Agent AgentInterface

	// HTTPClient is the client for webhook calls.
	// If nil, a default client with 30s timeout is used.
	HTTPClient *http.Client
}

// NewExecutor creates a new executor.
func NewExecutor(config ExecutorConfig) *Executor {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &Executor{
		agent:      config.Agent,
		httpClient: client,
	}
}

// Execute implements ExecutionHandler by routing to the appropriate action handler.
func (e *Executor) Execute(ctx context.Context, job *Job) ExecutionResult {
	startedAt := time.Now()

	result := e.executeAction(ctx, job)

	result.StartedAt = startedAt
	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(startedAt)

	return result
}

// executeAction routes to the appropriate handler based on action type.
func (e *Executor) executeAction(ctx context.Context, job *Job) ExecutionResult {
	logger := slogutil.LoggerFromContext(ctx, slog.Default())
	logger.Info("executing job action",
		"job_id", job.ID,
		"job_name", job.Name,
		"action_type", job.Action.Type,
	)

	switch job.Action.Type {
	case ActionTypeSendMessage:
		return e.executeSendMessage(ctx, job)
	case ActionTypeCallWebhook:
		return e.executeCallWebhook(ctx, job)
	case ActionTypeCallTool:
		return e.executeCallTool(ctx, job)
	default:
		return ExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("unknown action type: %s", job.Action.Type),
		}
	}
}

// executeSendMessage sends a message to a session via the agent.
func (e *Executor) executeSendMessage(ctx context.Context, job *Job) ExecutionResult {
	if e.agent == nil {
		return ExecutionResult{
			Success: false,
			Error:   "agent not configured for send_message action",
		}
	}

	response, err := e.agent.ProcessWithSession(ctx, job.Action.SessionID, job.Action.Message)
	if err != nil {
		return ExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("send message failed: %v", err),
		}
	}

	return ExecutionResult{
		Success: true,
		Output: map[string]any{
			"session_id": job.Action.SessionID,
			"message":    job.Action.Message,
			"response":   response,
		},
	}
}

// executeCallWebhook makes an HTTP request to a webhook URL.
func (e *Executor) executeCallWebhook(ctx context.Context, job *Job) ExecutionResult {
	method := job.Action.WebhookMethod
	if method == "" {
		method = http.MethodPost
	}

	var body io.Reader
	if job.Action.WebhookBody != "" {
		body = bytes.NewBufferString(job.Action.WebhookBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, job.Action.WebhookURL, body)
	if err != nil {
		return ExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("create request failed: %v", err),
		}
	}

	// Set default content type for POST/PUT/PATCH
	if body != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add custom headers
	for key, value := range job.Action.WebhookHeaders {
		req.Header.Set(key, value)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return ExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("webhook request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // Limit to 1MB
	if err != nil {
		return ExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("read response failed: %v", err),
		}
	}

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("webhook returned status %d: %s", resp.StatusCode, string(respBody)),
			Output: map[string]any{
				"status_code": resp.StatusCode,
				"body":        string(respBody),
			},
		}
	}

	return ExecutionResult{
		Success: true,
		Output: map[string]any{
			"status_code": resp.StatusCode,
			"body":        string(respBody),
		},
	}
}

// executeCallTool invokes a registered tool.
func (e *Executor) executeCallTool(ctx context.Context, job *Job) ExecutionResult {
	if e.agent == nil {
		return ExecutionResult{
			Success: false,
			Error:   "agent not configured for call_tool action",
		}
	}

	// Marshal tool params to JSON
	paramsJSON, err := json.Marshal(job.Action.ToolParams)
	if err != nil {
		return ExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("marshal tool params failed: %v", err),
		}
	}

	result, err := e.agent.Tools().Execute(ctx, job.Action.ToolName, paramsJSON)
	if err != nil {
		return ExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("tool execution failed: %v", err),
		}
	}

	return ExecutionResult{
		Success: true,
		Output: map[string]any{
			"tool_name": job.Action.ToolName,
			"params":    job.Action.ToolParams,
			"result":    result,
		},
	}
}
