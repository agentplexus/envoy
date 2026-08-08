// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omniobserve/agentops/middleware"
	"github.com/plexusone/omniobserve/observops"
)

// contextKey for storing IDs in context when full objects aren't available.
type contextKey string

const (
	taskIDContextKey     contextKey = "gateway.task_id"
	workflowIDContextKey contextKey = "gateway.workflow_id"
)

// ToolsRPCConfig configures the tools RPC handler.
type ToolsRPCConfig struct {
	// ToolRegistry is the registry of available tools.
	ToolRegistry *agent.ToolRegistry

	// Observability provides tracing and metrics.
	Observability *Observability

	// Logger is the logger for RPC events.
	Logger *slog.Logger

	// MaxRequestSize is the maximum request body size in bytes.
	// Default is 1MB.
	MaxRequestSize int64

	// Timeout is the maximum time for a tool invocation.
	// Default is 30 seconds.
	Timeout time.Duration
}

// ToolsRPCHandler handles SDK-facing tools.invoke RPC requests.
type ToolsRPCHandler struct {
	config ToolsRPCConfig
	logger *slog.Logger
}

// ToolInvokeRequest is the request format for tools.invoke RPC.
type ToolInvokeRequest struct {
	// Tool is the name of the tool to invoke.
	Tool string `json:"tool"`

	// Arguments are the tool arguments as JSON.
	Arguments json.RawMessage `json:"arguments"`

	// WorkflowID is the optional workflow ID for tracking.
	WorkflowID string `json:"workflow_id,omitempty"`

	// TaskID is the optional task ID for tracking.
	TaskID string `json:"task_id,omitempty"`

	// AgentID is the agent invoking the tool.
	AgentID string `json:"agent_id,omitempty"`

	// TraceID is the optional trace ID for correlation.
	TraceID string `json:"trace_id,omitempty"`

	// Metadata contains optional request metadata.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ToolInvokeResponse is the response format for tools.invoke RPC.
type ToolInvokeResponse struct {
	// Result is the tool execution result.
	Result string `json:"result,omitempty"`

	// Error is the error message if execution failed.
	Error string `json:"error,omitempty"`

	// ToolName is the name of the tool that was invoked.
	ToolName string `json:"tool_name"`

	// DurationMs is the execution duration in milliseconds.
	DurationMs int64 `json:"duration_ms"`

	// InvocationID is the unique ID for this invocation.
	InvocationID string `json:"invocation_id,omitempty"`

	// TraceID is the trace ID for correlation.
	TraceID string `json:"trace_id,omitempty"`
}

// NewToolsRPCHandler creates a new tools RPC handler.
func NewToolsRPCHandler(config ToolsRPCConfig) *ToolsRPCHandler {
	if config.MaxRequestSize == 0 {
		config.MaxRequestSize = 1 << 20 // 1MB
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &ToolsRPCHandler{
		config: config,
		logger: config.Logger,
	}
}

// ServeHTTP handles the HTTP request.
func (h *ToolsRPCHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request size
	r.Body = http.MaxBytesReader(w, r.Body, h.config.MaxRequestSize)

	// Parse request
	var req ToolInvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// Validate request
	if req.Tool == "" {
		h.errorResponse(w, http.StatusBadRequest, "tool name required")
		return
	}

	// Check tool exists
	if h.config.ToolRegistry == nil {
		h.errorResponse(w, http.StatusServiceUnavailable, "tool registry not configured")
		return
	}

	if _, ok := h.config.ToolRegistry.Get(req.Tool); !ok {
		h.errorResponse(w, http.StatusNotFound, fmt.Sprintf("tool not found: %s", req.Tool))
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), h.config.Timeout)
	defer cancel()

	// Set up context with agentops info if available
	if h.config.Observability != nil && h.config.Observability.Store() != nil {
		store := h.config.Observability.Store()
		ctx = middleware.WithStore(ctx, store)

		if req.AgentID != "" {
			ctx = middleware.WithAgent(ctx, middleware.AgentInfo{
				ID:   req.AgentID,
				Type: "sdk",
			})
		}
		if req.TaskID != "" {
			// Store task ID in context for tool invocation tracking
			ctx = context.WithValue(ctx, taskIDContextKey, req.TaskID)
		}
		if req.WorkflowID != "" {
			// Store workflow ID in context for tool invocation tracking
			ctx = context.WithValue(ctx, workflowIDContextKey, req.WorkflowID)
		}
	}

	// Start trace
	var tc *TraceContext
	if h.config.Observability != nil {
		tc = h.config.Observability.StartTrace(ctx, "tools.invoke",
			observops.Attribute("tool.name", req.Tool),
			observops.Attribute("agent.id", req.AgentID),
		)
		ctx = tc.ctx
	}

	// Execute tool with instrumentation
	start := time.Now()
	result, execErr := h.executeTool(ctx, req)
	duration := time.Since(start)

	// End trace
	if tc != nil {
		h.config.Observability.EndTrace(tc, execErr)
	}

	// Record metrics
	if h.config.Observability != nil {
		h.config.Observability.RecordToolInvocation(ctx, req.Tool, duration, execErr)
	}

	// Build response
	resp := ToolInvokeResponse{
		ToolName:   req.Tool,
		DurationMs: duration.Milliseconds(),
		TraceID:    req.TraceID,
	}

	if execErr != nil {
		resp.Error = execErr.Error()
		h.logger.Error("tool invocation failed",
			"tool", req.Tool,
			"error", execErr,
			"duration_ms", duration.Milliseconds(),
		)
	} else {
		resp.Result = result
		h.logger.Info("tool invocation completed",
			"tool", req.Tool,
			"duration_ms", duration.Milliseconds(),
		)
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if execErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

// executeTool executes the tool with agentops instrumentation.
func (h *ToolsRPCHandler) executeTool(ctx context.Context, req ToolInvokeRequest) (string, error) {
	// If agentops store is available, use ToolCall wrapper
	if h.config.Observability != nil && h.config.Observability.Store() != nil {
		return middleware.ToolCall(ctx, req.Tool, func() (string, error) {
			return h.config.ToolRegistry.Execute(ctx, req.Tool, req.Arguments)
		},
			middleware.WithToolType("sdk_rpc"),
			middleware.WithToolInput(map[string]any{
				"arguments":   string(req.Arguments),
				"workflow_id": req.WorkflowID,
				"task_id":     req.TaskID,
			}),
		)
	}

	// Direct execution without instrumentation
	return h.config.ToolRegistry.Execute(ctx, req.Tool, req.Arguments)
}

// errorResponse sends a JSON error response.
func (h *ToolsRPCHandler) errorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := ToolInvokeResponse{
		Error: message,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode error response", "error", err)
	}
}

// ToolsListHandler handles listing available tools.
type ToolsListHandler struct {
	registry *agent.ToolRegistry
	sessions *sessions.Store
	logger   *slog.Logger
}

// ToolInfo describes a tool for listing.
// Tools sourced from an MCP server expose their originating identity;
// non-MCP tools omit the MCP fields.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters,omitempty"`

	// Source is the tool's origin kind ("mcp", "skill"); empty for tools
	// registered directly.
	Source string `json:"source,omitempty"`

	// MCPServer is the originating MCP server name (MCP tools only).
	MCPServer string `json:"mcp_server,omitempty"`

	// MCPToolName is the tool's original name on its MCP server, before
	// any renaming applied at registration (MCP tools only).
	MCPToolName string `json:"mcp_tool_name,omitempty"`

	// DeniedBySession marks tools excluded for the requested session by
	// its tool overrides. Only populated on session-scoped listings
	// (?session_id=), where denied tools remain listed rather than hidden
	// so a read-only inventory stays complete.
	DeniedBySession bool `json:"denied_by_session,omitempty"`
}

// ToolsListResponse is the response format for tools.list RPC.
type ToolsListResponse struct {
	Tools []ToolInfo `json:"tools"`
}

// NewToolsListHandler creates a new tools list handler.
func NewToolsListHandler(registry *agent.ToolRegistry, logger *slog.Logger) *ToolsListHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ToolsListHandler{
		registry: registry,
		logger:   logger,
	}
}

// WithSessions enables session-scoped listings (?session_id=) by giving the
// handler access to session tool overrides. Returns the handler for chaining.
func (h *ToolsListHandler) WithSessions(store *sessions.Store) *ToolsListHandler {
	h.sessions = store
	return h
}

// ServeHTTP handles the HTTP request.
func (h *ToolsListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"}); err != nil {
			h.logger.Error("failed to encode error response", "error", err)
		}
		return
	}

	// Session-scoped listing: resolve the session's tool overrides so
	// denied tools can be flagged (not hidden — the inventory stays
	// complete for read-only callers).
	var overrides *sessions.ToolOverrides
	if sessionID := r.URL.Query().Get("session_id"); sessionID != "" && h.sessions != nil {
		session, err := h.sessions.GetIfExists(r.Context(), sessionID)
		switch {
		case err == nil:
			overrides = session.ToolOverrides
		case errors.Is(err, sessions.ErrSessionNotFound):
			// Unknown session: list without overrides.
		default:
			h.logger.Error("failed to load session for tools listing", "session_id", sessionID, "error", err)
		}
	}

	tools := []ToolInfo{}
	if h.registry != nil {
		for _, d := range h.registry.Describe() {
			info := ToolInfo{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  d.Parameters,
				Source:      d.Source,
			}
			if d.Source == "mcp" {
				info.MCPServer = d.SourceName
				info.MCPToolName = d.SourceTool
			}
			if overrides.Denies(d.Name, d.Source, d.SourceName, d.SourceTool) {
				info.DeniedBySession = true
			}
			tools = append(tools, info)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ToolsListResponse{Tools: tools}); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}
