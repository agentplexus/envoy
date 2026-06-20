// Package openai provides an OpenAI-compatible API server for OmniAgent.
package openai

import (
	"context"
	"time"
)

// AgentHandler defines the interface that agent implementations must satisfy.
// OmniAgent's agent.Agent will implement this interface via an adapter.
type AgentHandler interface {
	// ChatCompletion handles a chat completion request.
	// Returns the response text.
	ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)

	// ChatCompletionStream handles streaming chat completion.
	// Calls onDelta for each chunk, should return after all chunks are sent.
	ChatCompletionStream(ctx context.Context, req *ChatCompletionRequest,
		onDelta func(delta *ChatCompletionChunk) error) error

	// ListModels returns available models/agents.
	ListModels(ctx context.Context) ([]Model, error)

	// GetModel returns details for a specific model/agent.
	GetModel(ctx context.Context, modelID string) (*Model, error)

	// ListTools returns available tools registered with the agent.
	ListTools(ctx context.Context) ([]ToolInfo, error)

	// ListCronJobs returns all scheduled jobs.
	ListCronJobs(ctx context.Context) ([]CronJobInfo, error)

	// GetCronJob returns a specific job by ID.
	GetCronJob(ctx context.Context, id string) (*CronJobInfo, error)

	// CreateCronJob creates a new scheduled job.
	CreateCronJob(ctx context.Context, req *CreateCronJobRequest) (*CronJobInfo, error)

	// UpdateCronJob updates an existing job.
	UpdateCronJob(ctx context.Context, id string, req *UpdateCronJobRequest) (*CronJobInfo, error)

	// DeleteCronJob removes a job.
	DeleteCronJob(ctx context.Context, id string) error

	// TriggerCronJob runs a job immediately.
	TriggerCronJob(ctx context.Context, id string) (*CronJobResult, error)

	// EnableCronJob enables a disabled job.
	EnableCronJob(ctx context.Context, id string) error

	// DisableCronJob disables a job without deleting it.
	DisableCronJob(ctx context.Context, id string) error
}

// ToolInfo represents information about an available tool.
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Category    string                 `json:"category,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool/function call.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call within a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool represents a tool that can be called.
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents a function definition.
type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Strict      bool                   `json:"strict,omitempty"`
}

// ChatCompletionRequest represents a chat completion request.
type ChatCompletionRequest struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	Temperature      *float64  `json:"temperature,omitempty"`
	TopP             *float64  `json:"top_p,omitempty"`
	N                *int      `json:"n,omitempty"`
	Stream           bool      `json:"stream,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
	MaxTokens        *int      `json:"max_tokens,omitempty"`
	PresencePenalty  *float64  `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64  `json:"frequency_penalty,omitempty"`
	User             string    `json:"user,omitempty"`
	Tools            []Tool    `json:"tools,omitempty"`
	ToolChoice       any       `json:"tool_choice,omitempty"`
	Seed             *int      `json:"seed,omitempty"`
}

// ChatCompletionResponse represents a non-streaming chat completion response.
type ChatCompletionResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             *Usage   `json:"usage,omitempty"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"`
}

// Choice represents a completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
	Logprobs     any     `json:"logprobs,omitempty"`
}

// Usage represents token usage statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk represents a streaming chunk.
type ChatCompletionChunk struct {
	ID                string        `json:"id"`
	Object            string        `json:"object"`
	Created           int64         `json:"created"`
	Model             string        `json:"model"`
	Choices           []ChunkChoice `json:"choices"`
	Usage             *Usage        `json:"usage,omitempty"`
	SystemFingerprint string        `json:"system_fingerprint,omitempty"`
}

// ChunkChoice represents a choice in a streaming chunk.
type ChunkChoice struct {
	Index        int        `json:"index"`
	Delta        ChunkDelta `json:"delta"`
	FinishReason *string    `json:"finish_reason"`
	Logprobs     any        `json:"logprobs,omitempty"`
}

// ChunkDelta represents the delta content in a streaming chunk.
type ChunkDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Model represents an available model.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// Error represents an API error.
type Error struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param,omitempty"`
	Code    *string `json:"code,omitempty"`
}

// ErrorResponse wraps an error in the standard format.
type ErrorResponse struct {
	Error Error `json:"error"`
}

// CronJobInfo represents information about a scheduled job.
type CronJobInfo struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Schedule    CronScheduleInfo `json:"schedule"`
	Action      CronActionInfo   `json:"action"`
	Status      string           `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	LastRunAt   *time.Time       `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time       `json:"next_run_at,omitempty"`
	RunCount    int64            `json:"run_count"`
	LastError   string           `json:"last_error,omitempty"`
}

// CronScheduleInfo represents job schedule information.
type CronScheduleInfo struct {
	Cron     string `json:"cron,omitempty"`
	Once     string `json:"once,omitempty"` // RFC3339 timestamp
	Interval string `json:"interval,omitempty"`
}

// CronActionInfo represents job action information.
type CronActionInfo struct {
	Type           string            `json:"type"`
	SessionID      string            `json:"session_id,omitempty"`
	Message        string            `json:"message,omitempty"`
	WebhookURL     string            `json:"webhook_url,omitempty"`
	WebhookMethod  string            `json:"webhook_method,omitempty"`
	WebhookHeaders map[string]string `json:"webhook_headers,omitempty"`
	WebhookBody    string            `json:"webhook_body,omitempty"`
	ToolName       string            `json:"tool_name,omitempty"`
	ToolParams     map[string]any    `json:"tool_params,omitempty"`
}

// CreateCronJobRequest represents a request to create a scheduled job.
type CreateCronJobRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Schedule    CronScheduleInfo `json:"schedule"`
	Action      CronActionInfo   `json:"action"`
}

// UpdateCronJobRequest represents a request to update a scheduled job.
type UpdateCronJobRequest struct {
	Name        *string           `json:"name,omitempty"`
	Description *string           `json:"description,omitempty"`
	Schedule    *CronScheduleInfo `json:"schedule,omitempty"`
	Action      *CronActionInfo   `json:"action,omitempty"`
}

// CronJobResult represents the result of triggering a job.
type CronJobResult struct {
	Success   bool   `json:"success"`
	Output    any    `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	Duration  string `json:"duration"`
	StartedAt string `json:"started_at"`
}

// AgentManager is an optional interface for managing multiple agents.
// Adapters that support multi-agent can implement this interface.
type AgentManager interface {
	// ListAgents returns all configured agents.
	ListAgents(ctx context.Context) ([]AgentInfo, error)

	// GetAgent returns details for a specific agent.
	GetAgent(ctx context.Context, id string) (*AgentInfo, error)

	// CreateAgent creates a new agent configuration.
	CreateAgent(ctx context.Context, req *CreateAgentRequest) (*AgentInfo, error)

	// UpdateAgent updates an existing agent configuration.
	UpdateAgent(ctx context.Context, id string, req *UpdateAgentRequest) (*AgentInfo, error)

	// DeleteAgent removes an agent.
	DeleteAgent(ctx context.Context, id string) error

	// CloneAgent duplicates an existing agent.
	CloneAgent(ctx context.Context, id string, req *CloneAgentRequest) (*AgentInfo, error)
}

// AgentInfo represents information about a configured agent.
type AgentInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Temperature  float64   `json:"temperature,omitempty"`
	MaxTokens    int       `json:"max_tokens,omitempty"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	AllowedTools []string  `json:"allowed_tools,omitempty"`
	DeniedTools  []string  `json:"denied_tools,omitempty"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateAgentRequest represents a request to create an agent.
type CreateAgentRequest struct {
	ID           string   `json:"id,omitempty"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Provider     string   `json:"provider,omitempty"`
	Model        string   `json:"model,omitempty"`
	APIKey       string   `json:"api_key,omitempty"` //nolint:gosec // G101: API key in request
	BaseURL      string   `json:"base_url,omitempty"`
	Temperature  float64  `json:"temperature,omitempty"`
	MaxTokens    int      `json:"max_tokens,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	DeniedTools  []string `json:"denied_tools,omitempty"`
}

// UpdateAgentRequest represents a request to update an agent.
type UpdateAgentRequest struct {
	Name         *string  `json:"name,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Provider     *string  `json:"provider,omitempty"`
	Model        *string  `json:"model,omitempty"`
	APIKey       *string  `json:"api_key,omitempty"` //nolint:gosec // G101: API key in request
	BaseURL      *string  `json:"base_url,omitempty"`
	Temperature  *float64 `json:"temperature,omitempty"`
	MaxTokens    *int     `json:"max_tokens,omitempty"`
	SystemPrompt *string  `json:"system_prompt,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	DeniedTools  []string `json:"denied_tools,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty"`
}

// CloneAgentRequest represents a request to clone an agent.
type CloneAgentRequest struct {
	NewID   string `json:"new_id,omitempty"`
	NewName string `json:"new_name"`
}

// UsageHandler is an optional interface for usage tracking.
// Adapters that support usage tracking can implement this interface.
type UsageHandler interface {
	// GetUsageSummary returns aggregated usage statistics.
	GetUsageSummary(ctx context.Context, since, until time.Time) (*UsageSummary, error)

	// GetUsageTimeseries returns time-bucketed usage data.
	GetUsageTimeseries(ctx context.Context, since, until time.Time, interval string) (*UsageTimeseries, error)

	// GetUsageRecords returns recent usage records.
	GetUsageRecords(ctx context.Context, limit int) ([]UsageRecord, error)

	// RecordUsage records a usage event.
	RecordUsage(record UsageRecord)
}

// MemoryHandler is an optional interface for semantic memory operations.
// Adapters that support memory can implement this interface.
type MemoryHandler interface {
	// ListMemories returns memories from a collection.
	ListMemories(ctx context.Context, collection string, limit int) ([]MemoryRecord, error)

	// SearchMemories performs semantic search on memories.
	SearchMemories(ctx context.Context, collection, query string, limit int) ([]MemorySearchResult, error)

	// StoreMemory stores a new memory.
	StoreMemory(ctx context.Context, req *StoreMemoryRequest) (*MemoryRecord, error)

	// DeleteMemory deletes a memory by key.
	DeleteMemory(ctx context.Context, collection, key string) error

	// ListCollections returns all memory collections.
	ListCollections(ctx context.Context) ([]MemoryCollection, error)
}

// MemoryRecord represents a stored memory.
type MemoryRecord struct {
	Key        string            `json:"key"`
	Content    string            `json:"content"`
	Collection string            `json:"collection"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

// MemorySearchResult represents a memory search result with score.
type MemorySearchResult struct {
	MemoryRecord
	Score float64 `json:"score"`
}

// MemoryCollection represents a memory collection.
type MemoryCollection struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Count       int    `json:"count"`
}

// StoreMemoryRequest represents a request to store a memory.
type StoreMemoryRequest struct {
	Content    string            `json:"content"`
	Key        string            `json:"key,omitempty"`
	Collection string            `json:"collection,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}
