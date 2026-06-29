package operations

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// ToolUsageHandler provides tool usage statistics.
type ToolUsageHandler interface {
	GetToolUsageSummary(ctx context.Context, since, until time.Time) (*ToolUsageSummary, error)
	GetToolStats(ctx context.Context, toolName string, since, until time.Time) (*ToolUsageStats, error)
	RecordToolUsage(ctx context.Context, record *ToolUsageRecord) error
}

// ToolUsageRecord represents a single tool invocation.
type ToolUsageRecord struct {
	ToolName  string    `json:"tool_name"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id,omitempty"`
	Latency   int64     `json:"latency_ms,omitempty"`
	Success   bool      `json:"success"`
}

// ToolUsageStats represents aggregated statistics for a tool.
type ToolUsageStats struct {
	ToolName    string    `json:"tool_name" doc:"Name of the tool"`
	CallCount   int64     `json:"call_count" doc:"Total number of calls"`
	LastUsed    time.Time `json:"last_used" doc:"Timestamp of last use"`
	AvgLatency  float64   `json:"avg_latency_ms,omitempty" doc:"Average latency in milliseconds"`
	SuccessRate float64   `json:"success_rate" doc:"Success rate (0-1)"`
}

// ToolUsageSummary provides overall tool usage statistics.
type ToolUsageSummary struct {
	TotalCalls int64                      `json:"total_calls" doc:"Total tool calls"`
	ByTool     map[string]*ToolUsageStats `json:"by_tool" doc:"Statistics per tool"`
	TopTools   []*ToolUsageStats          `json:"top_tools" doc:"Top tools by usage"`
}

// GetToolUsageInput is the input for getting tool usage.
type GetToolUsageInput struct {
	Since string `query:"since" doc:"Start time (RFC3339 or relative: 1h, 24h, 7d, 30d)"`
	Until string `query:"until" doc:"End time (RFC3339, defaults to now)"`
}

// GetToolUsageOutput is the output for tool usage.
type GetToolUsageOutput struct {
	Body ToolUsageSummary
}

// GetToolStatsInput is the input for getting stats for a specific tool.
type GetToolStatsInput struct {
	ToolName string `path:"name" doc:"Tool name"`
	Since    string `query:"since" doc:"Start time (RFC3339 or relative: 1h, 24h, 7d, 30d)"`
	Until    string `query:"until" doc:"End time (RFC3339, defaults to now)"`
}

// GetToolStatsOutput is the output for tool stats.
type GetToolStatsOutput struct {
	Body ToolUsageStats
}

// RegisterToolUsageOperations registers tool usage endpoints.
func RegisterToolUsageOperations(api huma.API, handler ToolUsageHandler, prefix string) {
	// GET /api/v1/tools/usage - Get overall tool usage summary
	huma.Register(api, huma.Operation{
		OperationID:   "getToolUsage",
		Method:        http.MethodGet,
		Path:          prefix + "/tools/usage",
		Summary:       "Get tool usage summary",
		Description:   "Returns aggregated tool usage statistics.",
		Tags:          []string{"Tools"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *GetToolUsageInput) (*GetToolUsageOutput, error) {
		since, until := parseTimeRange(input.Since, input.Until)
		summary, err := handler.GetToolUsageSummary(ctx, since, until)
		if err != nil {
			return nil, err
		}
		return &GetToolUsageOutput{Body: *summary}, nil
	})

	// GET /api/v1/tools/{name}/stats - Get stats for a specific tool
	huma.Register(api, huma.Operation{
		OperationID:   "getToolStats",
		Method:        http.MethodGet,
		Path:          prefix + "/tools/{name}/stats",
		Summary:       "Get tool statistics",
		Description:   "Returns usage statistics for a specific tool.",
		Tags:          []string{"Tools"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *GetToolStatsInput) (*GetToolStatsOutput, error) {
		since, until := parseTimeRange(input.Since, input.Until)
		stats, err := handler.GetToolStats(ctx, input.ToolName, since, until)
		if err != nil {
			return nil, err
		}
		return &GetToolStatsOutput{Body: *stats}, nil
	})
}

// parseTimeRange is defined in usage.go
