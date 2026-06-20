// Package openai provides an OpenAI-compatible API adapter for OmniAgent.
package openai

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/api/openai"
	"github.com/plexusone/omniagent/cron"
)

// AgentAdapter adapts agent.Agent to openai.AgentHandler.
type AgentAdapter struct {
	cronHandler
	agent      *agent.Agent
	modelID    string
	modelOwner string
	useSession bool
}

// AdapterOption configures the adapter.
type AdapterOption func(*AgentAdapter)

// WithModelID sets the model ID reported by the adapter.
func WithModelID(id string) AdapterOption {
	return func(a *AgentAdapter) {
		a.modelID = id
	}
}

// WithModelOwner sets the model owner reported by the adapter.
func WithModelOwner(owner string) AdapterOption {
	return func(a *AgentAdapter) {
		a.modelOwner = owner
	}
}

// WithSession enables session-based message processing.
func WithSession(enabled bool) AdapterOption {
	return func(a *AgentAdapter) {
		a.useSession = enabled
	}
}

// NewAgentAdapter creates a new adapter for the given agent.
func NewAgentAdapter(ag *agent.Agent, opts ...AdapterOption) *AgentAdapter {
	a := &AgentAdapter{
		agent:      ag,
		modelID:    "omniagent",
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
func (a *AgentAdapter) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
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
	var err error
	if a.useSession {
		response, err = a.agent.ProcessWithSession(ctx, sessionID, userContent)
	} else {
		response, err = a.agent.Process(ctx, sessionID, userContent)
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
func (a *AgentAdapter) ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest, onDelta func(*openai.ChatCompletionChunk) error) error {
	// For now, simulate streaming by getting the full response and chunking it
	// In the future, this could use omnillm's streaming capabilities

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
	var err error
	if a.useSession {
		response, err = a.agent.ProcessWithSession(ctx, sessionID, userContent)
	} else {
		response, err = a.agent.Process(ctx, sessionID, userContent)
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
func (a *AgentAdapter) ListModels(ctx context.Context) ([]openai.Model, error) {
	return []openai.Model{
		{
			ID:      a.modelID,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: a.modelOwner,
		},
	}, nil
}

// GetModel implements openai.AgentHandler.
func (a *AgentAdapter) GetModel(ctx context.Context, modelID string) (*openai.Model, error) {
	if modelID != a.modelID {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	return &openai.Model{
		ID:      a.modelID,
		Object:  "model",
		Created: time.Now().Unix(),
		OwnedBy: a.modelOwner,
	}, nil
}

// ListTools implements openai.AgentHandler.
func (a *AgentAdapter) ListTools(ctx context.Context) ([]openai.ToolInfo, error) {
	registry := a.agent.Tools()
	providerTools := registry.GetTools()

	tools := make([]openai.ToolInfo, 0, len(providerTools))
	for _, t := range providerTools {
		// Categorize tools based on name patterns
		category := categorizeTool(t.Function.Name)

		// Convert parameters to map[string]interface{}
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

// categorizeTool assigns a category based on tool name patterns.
func categorizeTool(name string) string {
	switch {
	case name == "web_search" || name == "search":
		return "search"
	case name == "render_chart" || name == "chart":
		return "visualization"
	case name == "read_file" || name == "write_file" || name == "list_files":
		return "filesystem"
	case name == "execute" || name == "run_command" || name == "bash":
		return "execution"
	default:
		return "general"
	}
}

// estimateTokens provides a rough estimate of token count.
// Uses the common approximation of ~4 characters per token.
func estimateTokens(text string) int {
	return (len(text) + 3) / 4
}

// getCronScheduler returns the cron scheduler from compiled skills.
func (a *AgentAdapter) getCronScheduler() *cron.Scheduler {
	for _, skill := range a.agent.GetCompiledSkills() {
		if cronSkill, ok := skill.(*cron.Skill); ok {
			return cronSkill.GetScheduler()
		}
	}
	return nil
}
