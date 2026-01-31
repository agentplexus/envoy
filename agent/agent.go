// Package agent provides the AI agent runtime for envoy.
package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/agentplexus/omnillm"
	"github.com/agentplexus/omnillm/provider"
)

// Agent is the AI agent that processes messages.
type Agent struct {
	client *omnillm.ChatClient
	tools  *ToolRegistry
	config Config
	logger *slog.Logger
}

// Config configures the agent.
type Config struct {
	Provider     string
	Model        string
	APIKey       string
	BaseURL      string
	Temperature  float64
	MaxTokens    int
	SystemPrompt string
	Logger       *slog.Logger
}

// New creates a new agent.
func New(config Config) (*Agent, error) {
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
		Providers: []omnillm.ProviderConfig{providerConfig},
		Logger:    config.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	return &Agent{
		client: client,
		tools:  NewToolRegistry(),
		config: config,
		logger: config.Logger,
	}, nil
}

// Process processes a message and returns a response.
func (a *Agent) Process(ctx context.Context, sessionID, content string) (string, error) {
	messages := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: content,
		},
	}

	// Add system prompt if configured
	if a.config.SystemPrompt != "" {
		messages = append([]provider.Message{
			{
				Role:    provider.RoleSystem,
				Content: a.config.SystemPrompt,
			},
		}, messages...)
	}

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

	// Add tools if available
	tools := a.tools.GetTools()
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

	return resp.Choices[0].Message.Content, nil
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
	return a.client.Close()
}
