package openai

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/agent/registry"
	"github.com/plexusone/omniagent/api/openai"
	"github.com/plexusone/omniagent/cron"
)

// MultiAgentAdapter adapts a registry of agents to openai.AgentHandler.
// It routes requests to the appropriate agent based on the model name.
type MultiAgentAdapter struct {
	cronHandler
	registry   *registry.Registry
	modelOwner string
	useSession bool
}

// MultiAdapterOption configures the multi-agent adapter.
type MultiAdapterOption func(*MultiAgentAdapter)

// WithMultiModelOwner sets the model owner reported by the adapter.
func WithMultiModelOwner(owner string) MultiAdapterOption {
	return func(a *MultiAgentAdapter) {
		a.modelOwner = owner
	}
}

// WithMultiSession enables session-based message processing.
func WithMultiSession(enabled bool) MultiAdapterOption {
	return func(a *MultiAgentAdapter) {
		a.useSession = enabled
	}
}

// NewMultiAgentAdapter creates a new multi-agent adapter.
func NewMultiAgentAdapter(reg *registry.Registry, opts ...MultiAdapterOption) *MultiAgentAdapter {
	a := &MultiAgentAdapter{
		registry:   reg,
		modelOwner: "plexusone",
		useSession: false,
	}

	for _, opt := range opts {
		opt(a)
	}

	// Initialize cron handler with scheduler getter
	a.cronHandler = newCronHandler(a.getCronScheduler)

	return a
}

// ChatCompletion implements openai.AgentHandler.
func (a *MultiAgentAdapter) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	// Get agent based on model name
	ag, err := a.registry.GetByModel(req.Model)
	if err != nil {
		return nil, fmt.Errorf("unknown model/agent: %s", req.Model)
	}

	// Extract the user message (last message from user)
	var userContent string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userContent = req.Messages[i].Content
			break
		}
	}

	if userContent == "" {
		return nil, fmt.Errorf("no user message found")
	}

	// Generate a session ID based on the request or use a default
	sessionID := req.User
	if sessionID == "" {
		sessionID = "default"
	}

	// Process the message
	var response string
	if a.useSession {
		response, err = ag.ProcessWithSession(ctx, sessionID, userContent)
	} else {
		response, err = ag.Process(ctx, sessionID, userContent)
	}
	if err != nil {
		return nil, err
	}

	// Build the response
	completionID := "chatcmpl-" + uuid.NewString()[:8]
	return &openai.ChatCompletionResponse{
		ID:      completionID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []openai.Choice{
			{
				Index: 0,
				Message: openai.Message{
					Role:    "assistant",
					Content: response,
				},
				FinishReason: "stop",
			},
		},
		Usage: &openai.Usage{
			PromptTokens:     estimateTokens(userContent),
			CompletionTokens: estimateTokens(response),
			TotalTokens:      estimateTokens(userContent) + estimateTokens(response),
		},
	}, nil
}

// ChatCompletionStream implements openai.AgentHandler.
func (a *MultiAgentAdapter) ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest, onDelta func(*openai.ChatCompletionChunk) error) error {
	// Get agent based on model name
	ag, err := a.registry.GetByModel(req.Model)
	if err != nil {
		return fmt.Errorf("unknown model/agent: %s", req.Model)
	}

	// Extract the user message
	var userContent string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userContent = req.Messages[i].Content
			break
		}
	}

	if userContent == "" {
		return fmt.Errorf("no user message found")
	}

	sessionID := req.User
	if sessionID == "" {
		sessionID = "default"
	}

	// Process the message
	var response string
	if a.useSession {
		response, err = ag.ProcessWithSession(ctx, sessionID, userContent)
	} else {
		response, err = ag.Process(ctx, sessionID, userContent)
	}
	if err != nil {
		return err
	}

	// Generate completion ID
	completionID := "chatcmpl-" + uuid.NewString()[:8]
	now := time.Now().Unix()

	// Send the role first
	err = onDelta(&openai.ChatCompletionChunk{
		ID:      completionID,
		Object:  "chat.completion.chunk",
		Created: now,
		Model:   req.Model,
		Choices: []openai.ChunkChoice{
			{
				Index: 0,
				Delta: openai.ChunkDelta{
					Role: "assistant",
				},
				FinishReason: nil,
			},
		},
	})
	if err != nil {
		return err
	}

	// Chunk the response into smaller pieces for streaming effect
	chunkSize := 20 // characters per chunk
	for i := 0; i < len(response); i += chunkSize {
		end := i + chunkSize
		if end > len(response) {
			end = len(response)
		}
		chunk := response[i:end]

		err = onDelta(&openai.ChatCompletionChunk{
			ID:      completionID,
			Object:  "chat.completion.chunk",
			Created: now,
			Model:   req.Model,
			Choices: []openai.ChunkChoice{
				{
					Index: 0,
					Delta: openai.ChunkDelta{
						Content: chunk,
					},
					FinishReason: nil,
				},
			},
		})
		if err != nil {
			return err
		}
	}

	// Send the final chunk with finish_reason
	finishReason := "stop"
	err = onDelta(&openai.ChatCompletionChunk{
		ID:      completionID,
		Object:  "chat.completion.chunk",
		Created: now,
		Model:   req.Model,
		Choices: []openai.ChunkChoice{
			{
				Index:        0,
				Delta:        openai.ChunkDelta{},
				FinishReason: &finishReason,
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// ListModels implements openai.AgentHandler.
// Returns all enabled agents as models.
func (a *MultiAgentAdapter) ListModels(ctx context.Context) ([]openai.Model, error) {
	configs := a.registry.ListEnabled()
	models := make([]openai.Model, 0, len(configs))

	for _, cfg := range configs {
		models = append(models, openai.Model{
			ID:      cfg.ID,
			Object:  "model",
			Created: cfg.CreatedAt.Unix(),
			OwnedBy: a.modelOwner,
		})
	}

	return models, nil
}

// GetModel implements openai.AgentHandler.
func (a *MultiAgentAdapter) GetModel(ctx context.Context, modelID string) (*openai.Model, error) {
	cfg, err := a.registry.GetConfig(modelID)
	if err != nil {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	return &openai.Model{
		ID:      cfg.ID,
		Object:  "model",
		Created: cfg.CreatedAt.Unix(),
		OwnedBy: a.modelOwner,
	}, nil
}

// ListTools implements openai.AgentHandler.
// Returns tools from the default agent.
func (a *MultiAgentAdapter) ListTools(ctx context.Context) ([]openai.ToolInfo, error) {
	ag, err := a.registry.Default()
	if err != nil {
		return nil, err
	}

	registry := ag.Tools()
	providerTools := registry.GetTools()

	tools := make([]openai.ToolInfo, 0, len(providerTools))
	for _, t := range providerTools {
		category := categorizeTool(t.Function.Name)

		var params map[string]interface{}
		if p, ok := t.Function.Parameters.(map[string]interface{}); ok {
			params = p
		}

		tools = append(tools, openai.ToolInfo{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  params,
			Category:    category,
		})
	}

	return tools, nil
}

// getCronScheduler returns the cron scheduler from the default agent's compiled skills.
func (a *MultiAgentAdapter) getCronScheduler() *cron.Scheduler {
	ag, err := a.registry.Default()
	if err != nil {
		return nil
	}

	for _, skill := range ag.GetCompiledSkills() {
		if cronSkill, ok := skill.(*cron.Skill); ok {
			return cronSkill.GetScheduler()
		}
	}
	return nil
}

// Registry returns the underlying registry.
func (a *MultiAgentAdapter) Registry() *registry.Registry {
	return a.registry
}

// ListAgents implements openai.AgentManager.
func (a *MultiAgentAdapter) ListAgents(ctx context.Context) ([]openai.AgentInfo, error) {
	configs := a.registry.List()
	agents := make([]openai.AgentInfo, len(configs))

	for i, cfg := range configs {
		agents[i] = configToAgentInfo(cfg)
	}

	return agents, nil
}

// GetAgent implements openai.AgentManager.
func (a *MultiAgentAdapter) GetAgent(ctx context.Context, id string) (*openai.AgentInfo, error) {
	cfg, err := a.registry.GetConfig(id)
	if err != nil {
		return nil, err
	}

	info := configToAgentInfo(cfg)
	return &info, nil
}

// CreateAgent implements openai.AgentManager.
func (a *MultiAgentAdapter) CreateAgent(ctx context.Context, req *openai.CreateAgentRequest) (*openai.AgentInfo, error) {
	cfg := &registry.AgentConfig{
		ID:           req.ID,
		Name:         req.Name,
		Description:  req.Description,
		Provider:     req.Provider,
		Model:        req.Model,
		APIKey:       req.APIKey,
		BaseURL:      req.BaseURL,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		SystemPrompt: req.SystemPrompt,
		AllowedTools: req.AllowedTools,
		DeniedTools:  req.DeniedTools,
	}

	if err := a.registry.Create(ctx, cfg); err != nil {
		return nil, err
	}

	// Get the created config (with generated ID and timestamps)
	created, err := a.registry.GetConfig(cfg.ID)
	if err != nil {
		return nil, err
	}

	info := configToAgentInfo(created)
	return &info, nil
}

// UpdateAgent implements openai.AgentManager.
func (a *MultiAgentAdapter) UpdateAgent(ctx context.Context, id string, req *openai.UpdateAgentRequest) (*openai.AgentInfo, error) {
	updates := &registry.AgentConfig{}

	if req.Name != nil {
		updates.Name = *req.Name
	}
	if req.Description != nil {
		updates.Description = *req.Description
	}
	if req.Provider != nil {
		updates.Provider = *req.Provider
	}
	if req.Model != nil {
		updates.Model = *req.Model
	}
	if req.APIKey != nil {
		updates.APIKey = *req.APIKey
	}
	if req.BaseURL != nil {
		updates.BaseURL = *req.BaseURL
	}
	if req.Temperature != nil {
		updates.Temperature = *req.Temperature
	}
	if req.MaxTokens != nil {
		updates.MaxTokens = *req.MaxTokens
	}
	if req.SystemPrompt != nil {
		updates.SystemPrompt = *req.SystemPrompt
	}
	if req.AllowedTools != nil {
		updates.AllowedTools = req.AllowedTools
	}
	if req.DeniedTools != nil {
		updates.DeniedTools = req.DeniedTools
	}
	if req.Enabled != nil {
		updates.Enabled = req.Enabled
	}

	if err := a.registry.Update(ctx, id, updates); err != nil {
		return nil, err
	}

	// Get updated config
	cfg, err := a.registry.GetConfig(id)
	if err != nil {
		return nil, err
	}

	info := configToAgentInfo(cfg)
	return &info, nil
}

// DeleteAgent implements openai.AgentManager.
func (a *MultiAgentAdapter) DeleteAgent(ctx context.Context, id string) error {
	return a.registry.Delete(ctx, id)
}

// CloneAgent implements openai.AgentManager.
func (a *MultiAgentAdapter) CloneAgent(ctx context.Context, id string, req *openai.CloneAgentRequest) (*openai.AgentInfo, error) {
	newID := req.NewID
	if newID == "" {
		newID = "" // Let registry generate one
	}

	if err := a.registry.Clone(ctx, id, newID, req.NewName); err != nil {
		return nil, err
	}

	// Get the cloned config
	// If newID was empty, we need to find it by name
	configs := a.registry.List()
	for _, cfg := range configs {
		if cfg.Name == req.NewName && cfg.ID != id {
			info := configToAgentInfo(cfg)
			return &info, nil
		}
	}

	return nil, fmt.Errorf("clone created but not found")
}

// configToAgentInfo converts a registry.AgentConfig to openai.AgentInfo.
func configToAgentInfo(cfg *registry.AgentConfig) openai.AgentInfo {
	return openai.AgentInfo{
		ID:           cfg.ID,
		Name:         cfg.Name,
		Description:  cfg.Description,
		Provider:     cfg.Provider,
		Model:        cfg.Model,
		Temperature:  cfg.Temperature,
		MaxTokens:    cfg.MaxTokens,
		SystemPrompt: cfg.SystemPrompt,
		AllowedTools: cfg.AllowedTools,
		DeniedTools:  cfg.DeniedTools,
		Enabled:      cfg.IsEnabled(),
		CreatedAt:    cfg.CreatedAt,
		UpdatedAt:    cfg.UpdatedAt,
	}
}
