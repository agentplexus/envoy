// Package operations provides Huma-compatible request/response types and operation registrations.
package operations

import (
	"context"
	"time"
)

// Handler defines the interface for backend operations.
// This combines AgentHandler, AgentManager, and MemoryHandler from the parent package.
type Handler interface {
	// Tools
	ListTools(ctx context.Context) ([]ToolInfo, error)

	// Cron jobs
	ListCronJobs(ctx context.Context) ([]CronJobInfo, error)
	GetCronJob(ctx context.Context, id string) (*CronJobInfo, error)
	CreateCronJob(ctx context.Context, req *CreateCronJobRequest) (*CronJobInfo, error)
	UpdateCronJob(ctx context.Context, id string, req *UpdateCronJobRequest) (*CronJobInfo, error)
	DeleteCronJob(ctx context.Context, id string) error
	TriggerCronJob(ctx context.Context, id string) (*CronJobResult, error)
	EnableCronJob(ctx context.Context, id string) error
	DisableCronJob(ctx context.Context, id string) error
}

// AgentManager defines agent management operations.
type AgentManager interface {
	ListAgents(ctx context.Context) ([]AgentInfo, error)
	GetAgent(ctx context.Context, id string) (*AgentInfo, error)
	CreateAgent(ctx context.Context, req *CreateAgentRequest) (*AgentInfo, error)
	UpdateAgent(ctx context.Context, id string, req *UpdateAgentRequest) (*AgentInfo, error)
	DeleteAgent(ctx context.Context, id string) error
	CloneAgent(ctx context.Context, id string, req *CloneAgentRequest) (*AgentInfo, error)
}

// MemoryHandler defines memory operations.
type MemoryHandler interface {
	ListMemories(ctx context.Context, collection string, limit int) ([]MemoryRecord, error)
	SearchMemories(ctx context.Context, collection, query string, limit int) ([]MemorySearchResult, error)
	StoreMemory(ctx context.Context, req *StoreMemoryRequest) (*MemoryRecord, error)
	DeleteMemory(ctx context.Context, collection, key string) error
	ListCollections(ctx context.Context) ([]MemoryCollection, error)
}

// UsageStore defines usage tracking operations.
type UsageStore interface {
	GetSummary(since, until time.Time) *UsageSummary
	GetTimeseries(since, until time.Time, interval string) *UsageTimeseries
	GetRecords(limit int) []UsageRecord
}

// ToolInfo represents information about an available tool.
type ToolInfo struct {
	Name        string         `json:"name" doc:"Tool name"`
	Description string         `json:"description" doc:"Tool description"`
	Parameters  map[string]any `json:"parameters,omitempty" doc:"JSON Schema for tool parameters"`
	Category    string         `json:"category,omitempty" doc:"Tool category"`
}

// CronJobInfo represents information about a scheduled job.
type CronJobInfo struct {
	ID          string           `json:"id" doc:"Job unique identifier"`
	Name        string           `json:"name" doc:"Job name"`
	Description string           `json:"description,omitempty" doc:"Job description"`
	Schedule    CronScheduleInfo `json:"schedule" doc:"Job schedule configuration"`
	Action      CronActionInfo   `json:"action" doc:"Job action configuration"`
	Status      string           `json:"status" doc:"Job status (active, disabled)"`
	CreatedAt   time.Time        `json:"created_at" doc:"Creation timestamp"`
	UpdatedAt   time.Time        `json:"updated_at" doc:"Last update timestamp"`
	LastRunAt   *time.Time       `json:"last_run_at,omitempty" doc:"Last run timestamp"`
	NextRunAt   *time.Time       `json:"next_run_at,omitempty" doc:"Next scheduled run timestamp"`
	RunCount    int64            `json:"run_count" doc:"Total number of runs"`
	LastError   string           `json:"last_error,omitempty" doc:"Last error message if any"`
}

// CronScheduleInfo represents job schedule information.
type CronScheduleInfo struct {
	Cron     string `json:"cron,omitempty" doc:"Cron expression (e.g., '0 * * * *')"`
	Once     string `json:"once,omitempty" doc:"One-time schedule (RFC3339 timestamp)"`
	Interval string `json:"interval,omitempty" doc:"Interval duration (e.g., '1h', '30m')"`
}

// CronActionInfo represents job action information.
type CronActionInfo struct {
	Type           string            `json:"type" doc:"Action type (send_message, call_webhook, call_tool)"`
	SessionID      string            `json:"session_id,omitempty" doc:"Session ID for send_message"`
	Message        string            `json:"message,omitempty" doc:"Message content for send_message"`
	WebhookURL     string            `json:"webhook_url,omitempty" doc:"Webhook URL for call_webhook"`
	WebhookMethod  string            `json:"webhook_method,omitempty" doc:"HTTP method for webhook (GET, POST, etc.)"`
	WebhookHeaders map[string]string `json:"webhook_headers,omitempty" doc:"HTTP headers for webhook"`
	WebhookBody    string            `json:"webhook_body,omitempty" doc:"Request body for webhook"`
	ToolName       string            `json:"tool_name,omitempty" doc:"Tool name for call_tool"`
	ToolParams     map[string]any    `json:"tool_params,omitempty" doc:"Tool parameters for call_tool"`
}

// CreateCronJobRequest represents a request to create a scheduled job.
type CreateCronJobRequest struct {
	Name        string           `json:"name" doc:"Job name" required:"true"`
	Description string           `json:"description,omitempty" doc:"Job description"`
	Schedule    CronScheduleInfo `json:"schedule" doc:"Job schedule configuration" required:"true"`
	Action      CronActionInfo   `json:"action" doc:"Job action configuration" required:"true"`
}

// UpdateCronJobRequest represents a request to update a scheduled job.
type UpdateCronJobRequest struct {
	Name        *string           `json:"name,omitempty" doc:"Job name"`
	Description *string           `json:"description,omitempty" doc:"Job description"`
	Schedule    *CronScheduleInfo `json:"schedule,omitempty" doc:"Job schedule configuration"`
	Action      *CronActionInfo   `json:"action,omitempty" doc:"Job action configuration"`
}

// CronJobResult represents the result of triggering a job.
type CronJobResult struct {
	Success   bool   `json:"success" doc:"Whether the job succeeded"`
	Output    any    `json:"output,omitempty" doc:"Job output"`
	Error     string `json:"error,omitempty" doc:"Error message if failed"`
	Duration  string `json:"duration" doc:"Execution duration"`
	StartedAt string `json:"started_at" doc:"Start timestamp (RFC3339)"`
}

// AgentInfo represents information about a configured agent.
type AgentInfo struct {
	ID           string    `json:"id" doc:"Agent unique identifier"`
	Name         string    `json:"name" doc:"Agent display name"`
	Description  string    `json:"description,omitempty" doc:"Agent description"`
	Provider     string    `json:"provider" doc:"LLM provider (anthropic, openai, etc.)"`
	Model        string    `json:"model" doc:"Model identifier"`
	Temperature  float64   `json:"temperature,omitempty" doc:"Sampling temperature (0.0-2.0)"`
	MaxTokens    int       `json:"max_tokens,omitempty" doc:"Maximum tokens to generate"`
	SystemPrompt string    `json:"system_prompt,omitempty" doc:"System prompt for the agent"`
	AllowedTools []string  `json:"allowed_tools,omitempty" doc:"List of allowed tool names"`
	DeniedTools  []string  `json:"denied_tools,omitempty" doc:"List of denied tool names"`
	Enabled      bool      `json:"enabled" doc:"Whether agent is enabled"`
	CreatedAt    time.Time `json:"created_at" doc:"Creation timestamp"`
	UpdatedAt    time.Time `json:"updated_at" doc:"Last update timestamp"`
}

// CreateAgentRequest represents a request to create an agent.
type CreateAgentRequest struct {
	ID           string   `json:"id,omitempty" doc:"Agent ID (auto-generated if empty)"`
	Name         string   `json:"name" doc:"Agent display name" required:"true"`
	Description  string   `json:"description,omitempty" doc:"Agent description"`
	Provider     string   `json:"provider,omitempty" doc:"LLM provider"`
	Model        string   `json:"model,omitempty" doc:"Model identifier"`
	APIKey       string   `json:"api_key,omitempty" doc:"API key for the provider"` //nolint:gosec // G101: API key in request
	BaseURL      string   `json:"base_url,omitempty" doc:"Base URL for the provider"`
	Temperature  float64  `json:"temperature,omitempty" doc:"Sampling temperature"`
	MaxTokens    int      `json:"max_tokens,omitempty" doc:"Maximum tokens to generate"`
	SystemPrompt string   `json:"system_prompt,omitempty" doc:"System prompt"`
	AllowedTools []string `json:"allowed_tools,omitempty" doc:"Allowed tool names"`
	DeniedTools  []string `json:"denied_tools,omitempty" doc:"Denied tool names"`
}

// UpdateAgentRequest represents a request to update an agent.
type UpdateAgentRequest struct {
	Name         *string  `json:"name,omitempty" doc:"Agent display name"`
	Description  *string  `json:"description,omitempty" doc:"Agent description"`
	Provider     *string  `json:"provider,omitempty" doc:"LLM provider"`
	Model        *string  `json:"model,omitempty" doc:"Model identifier"`
	APIKey       *string  `json:"api_key,omitempty" doc:"API key for the provider"` //nolint:gosec // G101: API key in request
	BaseURL      *string  `json:"base_url,omitempty" doc:"Base URL for the provider"`
	Temperature  *float64 `json:"temperature,omitempty" doc:"Sampling temperature"`
	MaxTokens    *int     `json:"max_tokens,omitempty" doc:"Maximum tokens to generate"`
	SystemPrompt *string  `json:"system_prompt,omitempty" doc:"System prompt"`
	AllowedTools []string `json:"allowed_tools,omitempty" doc:"Allowed tool names"`
	DeniedTools  []string `json:"denied_tools,omitempty" doc:"Denied tool names"`
	Enabled      *bool    `json:"enabled,omitempty" doc:"Whether agent is enabled"`
}

// CloneAgentRequest represents a request to clone an agent.
type CloneAgentRequest struct {
	NewID   string `json:"new_id,omitempty" doc:"New agent ID (auto-generated if empty)"`
	NewName string `json:"new_name" doc:"New agent name" required:"true"`
}

// MemoryRecord represents a stored memory.
type MemoryRecord struct {
	Key        string            `json:"key" doc:"Memory unique key"`
	Content    string            `json:"content" doc:"Memory content"`
	Collection string            `json:"collection" doc:"Collection name"`
	Metadata   map[string]string `json:"metadata,omitempty" doc:"Custom metadata"`
	CreatedAt  time.Time         `json:"created_at" doc:"Creation timestamp"`
}

// MemorySearchResult represents a memory search result with score.
type MemorySearchResult struct {
	MemoryRecord
	Score float64 `json:"score" doc:"Relevance score (0.0-1.0)"`
}

// MemoryCollection represents a memory collection.
type MemoryCollection struct {
	Name        string `json:"name" doc:"Collection name"`
	Description string `json:"description,omitempty" doc:"Collection description"`
	Count       int    `json:"count" doc:"Number of memories in collection"`
}

// StoreMemoryRequest represents a request to store a memory.
type StoreMemoryRequest struct {
	Content    string            `json:"content" doc:"Memory content" required:"true"`
	Key        string            `json:"key,omitempty" doc:"Memory key (auto-generated if empty)"`
	Collection string            `json:"collection,omitempty" doc:"Collection name (defaults to 'default')"`
	Metadata   map[string]string `json:"metadata,omitempty" doc:"Custom metadata"`
}

// UsageRecord represents a single usage event.
type UsageRecord struct {
	ID               string    `json:"id" doc:"Record ID"`
	Timestamp        time.Time `json:"timestamp" doc:"Request timestamp"`
	Model            string    `json:"model" doc:"Model used"`
	AgentID          string    `json:"agent_id,omitempty" doc:"Agent ID if applicable"`
	SessionID        string    `json:"session_id,omitempty" doc:"Session ID"`
	PromptTokens     int       `json:"prompt_tokens" doc:"Input tokens"`
	CompletionTokens int       `json:"completion_tokens" doc:"Output tokens"`
	TotalTokens      int       `json:"total_tokens" doc:"Total tokens"`
	Cost             float64   `json:"cost" doc:"Estimated cost in USD"`
	Latency          int64     `json:"latency_ms" doc:"Response latency in milliseconds"`
}

// UsageSummary provides aggregated usage statistics.
type UsageSummary struct {
	TotalRequests     int64                  `json:"total_requests" doc:"Total number of requests"`
	TotalPromptTokens int64                  `json:"total_prompt_tokens" doc:"Total input tokens"`
	TotalCompTokens   int64                  `json:"total_completion_tokens" doc:"Total output tokens"`
	TotalTokens       int64                  `json:"total_tokens" doc:"Total tokens"`
	TotalCost         float64                `json:"total_cost" doc:"Total estimated cost in USD"`
	AvgLatency        float64                `json:"avg_latency_ms" doc:"Average response latency"`
	ByModel           map[string]*ModelUsage `json:"by_model" doc:"Usage breakdown by model"`
	ByAgent           map[string]*AgentUsage `json:"by_agent" doc:"Usage breakdown by agent"`
	PeriodStart       time.Time              `json:"period_start" doc:"Period start timestamp"`
	PeriodEnd         time.Time              `json:"period_end" doc:"Period end timestamp"`
}

// ModelUsage tracks usage per model.
type ModelUsage struct {
	Model            string  `json:"model" doc:"Model identifier"`
	Requests         int64   `json:"requests" doc:"Number of requests"`
	PromptTokens     int64   `json:"prompt_tokens" doc:"Input tokens"`
	CompletionTokens int64   `json:"completion_tokens" doc:"Output tokens"`
	TotalTokens      int64   `json:"total_tokens" doc:"Total tokens"`
	Cost             float64 `json:"cost" doc:"Estimated cost in USD"`
}

// AgentUsage tracks usage per agent.
type AgentUsage struct {
	AgentID          string  `json:"agent_id" doc:"Agent identifier"`
	Requests         int64   `json:"requests" doc:"Number of requests"`
	PromptTokens     int64   `json:"prompt_tokens" doc:"Input tokens"`
	CompletionTokens int64   `json:"completion_tokens" doc:"Output tokens"`
	TotalTokens      int64   `json:"total_tokens" doc:"Total tokens"`
	Cost             float64 `json:"cost" doc:"Estimated cost in USD"`
}

// UsageTimeseries represents time-bucketed usage data.
type UsageTimeseries struct {
	Interval string        `json:"interval" doc:"Bucket interval (hour, day)"`
	Buckets  []UsageBucket `json:"buckets" doc:"Time-bucketed data"`
}

// UsageBucket represents usage for a single time bucket.
type UsageBucket struct {
	Timestamp        time.Time `json:"timestamp" doc:"Bucket start timestamp"`
	Requests         int64     `json:"requests" doc:"Number of requests"`
	PromptTokens     int64     `json:"prompt_tokens" doc:"Input tokens"`
	CompletionTokens int64     `json:"completion_tokens" doc:"Output tokens"`
	TotalTokens      int64     `json:"total_tokens" doc:"Total tokens"`
	Cost             float64   `json:"cost" doc:"Estimated cost in USD"`
}
