// Package agent provides the AI agent runtime for omniagent.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/plexusone/omnillm"
	"github.com/plexusone/omnillm/provider"

	"github.com/plexusone/omniagent/skills"
	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omniagent/storage"
)

// Agent is the AI agent that processes messages.
type Agent struct {
	client         *omnillm.ChatClient
	tools          *ToolRegistry
	skills         []*skills.Skill        // Markdown skills (SKILL.md)
	compiledSkills []compiled.Skill       // Compiled Go skills
	storage        storage.Storage        // Storage backend
	config         Config
	logger         *slog.Logger
	mu             sync.RWMutex
}

// Config configures the agent.
type Config struct {
	Provider          string
	Model             string
	APIKey            string //nolint:gosec // G117: APIKey is intentionally stored for provider authentication
	BaseURL           string
	Temperature       float64
	MaxTokens         int
	SystemPrompt      string
	Logger            *slog.Logger
	ObservabilityHook omnillm.ObservabilityHook
}

// New creates a new agent with optional configuration.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithStorage(sqliteStorage),
//	    agent.WithCompiledSkill(investSkill),
//	)
func New(config Config, opts ...Option) (*Agent, error) {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	// Build provider configuration
	providerConfig := omnillm.ProviderConfig{
		Provider: omnillm.ProviderName(config.Provider),
		APIKey:   config.APIKey,
	}
	if config.BaseURL != "" {
		providerConfig.BaseURL = config.BaseURL
	}

	// Create omnillm client
	client, err := omnillm.NewClient(omnillm.ClientConfig{
		Providers:         []omnillm.ProviderConfig{providerConfig},
		Logger:            config.Logger,
		ObservabilityHook: config.ObservabilityHook,
	})
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	a := &Agent{
		client: client,
		tools:  NewToolRegistry(),
		config: config,
		logger: config.Logger,
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(a); err != nil {
			// Close client on error
			client.Close()
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return a, nil
}

// Process processes a message and returns a response.
func (a *Agent) Process(ctx context.Context, sessionID, content string) (string, error) {
	a.logger.Info("processing message", "model", a.config.Model, "provider", a.config.Provider)
	messages := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: content,
		},
	}

	// Add system prompt with injected skills
	systemPrompt := a.buildSystemPrompt()
	if systemPrompt != "" {
		a.logger.Info("using system prompt", "length", len(systemPrompt), "skills", len(a.skills))
		messages = append([]provider.Message{
			{
				Role:    provider.RoleSystem,
				Content: systemPrompt,
			},
		}, messages...)
	}

	// Add tools if available
	tools := a.tools.GetTools()
	a.logger.Info("tools available for request", "count", len(tools))
	for _, t := range tools {
		paramsJSON, _ := json.Marshal(t.Function.Parameters)
		a.logger.Info("tool in request", "name", t.Function.Name, "type", t.Type, "params", string(paramsJSON))
	}

	// Process with potential tool calls (max 5 iterations to prevent infinite loops)
	for i := 0; i < 5; i++ {
		req := &provider.ChatCompletionRequest{
			Model:    a.config.Model,
			Messages: messages,
		}

		if a.config.Temperature > 0 {
			req.Temperature = &a.config.Temperature
		}
		if a.config.MaxTokens > 0 {
			req.MaxTokens = &a.config.MaxTokens
		}

		if len(tools) > 0 {
			req.Tools = tools
		}

		resp, err := a.client.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", fmt.Errorf("chat completion: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no response choices")
		}

		choice := resp.Choices[0]

		a.logger.Info("LLM response",
			"content_length", len(choice.Message.Content),
			"tool_calls", len(choice.Message.ToolCalls),
			"finish_reason", choice.FinishReason)

		// Check if the model wants to call tools
		if len(choice.Message.ToolCalls) == 0 {
			// No tool calls, return the response
			return choice.Message.Content, nil
		}

		// Execute tool calls
		a.logger.Info("executing tool calls", "count", len(choice.Message.ToolCalls))

		// Add assistant message with tool calls to conversation
		messages = append(messages, provider.Message{
			Role:      provider.RoleAssistant,
			ToolCalls: choice.Message.ToolCalls,
		})

		// Execute each tool and add results
		for _, toolCall := range choice.Message.ToolCalls {
			a.logger.Info("calling tool", "name", toolCall.Function.Name)

			result, err := a.tools.Execute(ctx, toolCall.Function.Name, []byte(toolCall.Function.Arguments))
			if err != nil {
				a.logger.Error("tool execution failed", "name", toolCall.Function.Name, "error", err)
				result = fmt.Sprintf("Error: %v", err)
			}

			// Add tool result to conversation
			toolCallID := toolCall.ID
			messages = append(messages, provider.Message{
				Role:       provider.RoleTool,
				Content:    result,
				ToolCallID: &toolCallID,
			})
		}
	}

	return "", fmt.Errorf("exceeded maximum tool call iterations")
}

// ProcessWithMemory processes a message using conversation memory.
func (a *Agent) ProcessWithMemory(ctx context.Context, sessionID, content string) (string, error) {
	// TODO: Implement memory-aware processing using omnillm memory features
	return a.Process(ctx, sessionID, content)
}

// RegisterTool registers a tool with the agent.
func (a *Agent) RegisterTool(tool Tool) {
	a.tools.Register(tool)
}

// Close closes the agent and releases resources.
func (a *Agent) Close() error {
	// Close compiled skills first
	if err := a.CloseCompiledSkills(); err != nil {
		a.logger.Error("failed to close compiled skills", "error", err)
	}

	// Close storage
	if a.storage != nil {
		if err := a.storage.Close(); err != nil {
			a.logger.Error("failed to close storage", "error", err)
		}
	}

	return a.client.Close()
}

// LoadSkills loads skills from the given directories.
func (a *Agent) LoadSkills(dirs []string) error {
	if len(dirs) == 0 {
		dirs = skills.DefaultSearchPaths()
	}

	discovered, err := skills.Discover(dirs)
	if err != nil {
		return fmt.Errorf("discovering skills: %w", err)
	}

	// Filter to available skills only
	var available []*skills.Skill
	for _, skill := range discovered {
		errs := skill.CheckRequirements()
		if len(errs) == 0 {
			available = append(available, skill)
			a.logger.Info("skill loaded", "name", skill.Name, "path", skill.Path)
		} else {
			a.logger.Warn("skill unavailable", "name", skill.Name, "errors", len(errs))
		}
	}

	a.skills = available
	a.logger.Info("skills loaded", "total", len(discovered), "available", len(available))
	return nil
}

// GetSkills returns the loaded skills.
func (a *Agent) GetSkills() []*skills.Skill {
	return a.skills
}

// buildSystemPrompt builds the system prompt with injected skills.
func (a *Agent) buildSystemPrompt() string {
	if len(a.skills) == 0 {
		return a.config.SystemPrompt
	}

	return skills.InjectIntoPrompt(a.config.SystemPrompt, a.skills, skills.DefaultInjectConfig())
}
