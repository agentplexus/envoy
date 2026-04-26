// Package agent provides the AI agent runtime for omniagent.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	agentctx "github.com/plexusone/omniagent/context"
	"github.com/plexusone/omniagent/hooks"
	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omniagent/skills"
	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omnillm"
	"github.com/plexusone/omnillm/provider"
	"github.com/plexusone/omnistorage"
)

// Agent is the AI agent that processes messages.
type Agent struct {
	client         *omnillm.ChatClient
	tools          *ToolRegistry
	skills         []*skills.Skill   // Markdown skills (SKILL.md)
	compiledSkills []compiled.Skill  // Compiled Go skills
	storage        omnistorage.Store // Key-value storage backend
	sessions       *sessions.Store   // Persistent session storage
	contextEngine  *agentctx.Engine  // Context management engine
	hooks          *hooks.Registry   // Event hook registry
	dispatcher     *hooks.Dispatcher
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

	hookRegistry := hooks.NewRegistry()
	a := &Agent{
		client:     client,
		tools:      NewToolRegistry(),
		hooks:      hookRegistry,
		dispatcher: hooks.NewDispatcher(hookRegistry),
		config:     config,
		logger:     config.Logger,
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
// This is a stateless call that doesn't use session history.
// Use ProcessWithSession for conversation continuity.
func (a *Agent) Process(ctx context.Context, sessionID, content string) (string, error) {
	return a.processInternal(ctx, nil, content)
}

// ProcessWithSession processes a message using persistent session history.
// Conversation history is automatically loaded and saved.
func (a *Agent) ProcessWithSession(ctx context.Context, sessionID, content string) (string, error) {
	if a.sessions == nil {
		return "", fmt.Errorf("session store not configured: use WithSessionStore option")
	}

	// Load or create session
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("load session: %w", err)
	}

	// Check if this is a new session (no messages yet)
	isNewSession := len(session.GetMessages()) == 0

	// Emit session created event for new sessions
	if isNewSession {
		a.dispatcher.EmitAsync(ctx, hooks.EventSessionCreated, hooks.SessionEvent{
			SessionID: sessionID,
			Action:    "created",
		})
	}

	// Add user message to session
	session.AddMessage(provider.RoleUser, content)

	// Process with session history
	response, err := a.processInternal(ctx, session, content)
	if err != nil {
		// Still save the user message even on error
		if saveErr := a.sessions.Save(ctx, session); saveErr != nil {
			a.logger.Error("failed to save session after error", "error", saveErr)
		}
		return "", err
	}

	// Add assistant response to session
	session.AddMessage(provider.RoleAssistant, response)

	// Save session
	if err := a.sessions.Save(ctx, session); err != nil {
		a.logger.Error("failed to save session", "error", err)
		// Don't fail the request, just log the error
	}

	// Emit session updated event
	a.dispatcher.EmitAsync(ctx, hooks.EventSessionUpdated, hooks.SessionEvent{
		SessionID: sessionID,
		Action:    "updated",
	})

	return response, nil
}

// processInternal is the core message processing logic.
func (a *Agent) processInternal(ctx context.Context, session *sessions.Session, content string) (string, error) {
	a.logger.Info("processing message", "model", a.config.Model, "provider", a.config.Provider)

	// Emit message received event
	a.dispatcher.EmitAsync(ctx, hooks.EventMessageReceived, hooks.MessageEvent{
		Role:    "user",
		Content: content,
	})

	// Build messages array
	var messages []provider.Message

	// Add system prompt with injected skills
	systemPrompt := a.buildSystemPrompt()
	if systemPrompt != "" {
		a.logger.Info("using system prompt", "length", len(systemPrompt), "skills", len(a.skills))
		messages = append(messages, provider.Message{
			Role:    provider.RoleSystem,
			Content: systemPrompt,
		})
	}

	// Add conversation history from session (excluding the just-added user message)
	if session != nil {
		history := session.GetMessages()
		// Exclude the last message (the current user input we just added)
		if len(history) > 1 {
			messages = append(messages, history[:len(history)-1]...)
		}
		a.logger.Info("loaded session history", "session_id", session.ID, "messages", len(history)-1)
	}

	// Add current user message
	messages = append(messages, provider.Message{
		Role:    provider.RoleUser,
		Content: content,
	})

	// Apply context management (windowing, token limits)
	if a.contextEngine != nil {
		beforeCount := len(messages)
		messages = a.contextEngine.Apply(messages)
		if len(messages) < beforeCount {
			a.logger.Info("applied context windowing",
				"before", beforeCount,
				"after", len(messages),
				"tokens", a.contextEngine.EstimateTokens(messages))
		}
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
			// Emit message sent event
			a.dispatcher.EmitAsync(ctx, hooks.EventMessageSent, hooks.MessageEvent{
				Role:    "assistant",
				Content: choice.Message.Content,
			})
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

			// Parse tool arguments for event
			var toolParams map[string]any
			_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &toolParams)

			// Emit tool called event
			a.dispatcher.EmitAsync(ctx, hooks.EventToolCalled, hooks.ToolEvent{
				Name:   toolCall.Function.Name,
				Params: toolParams,
			})

			result, err := a.tools.Execute(ctx, toolCall.Function.Name, []byte(toolCall.Function.Arguments))
			var errStr string
			if err != nil {
				a.logger.Error("tool execution failed", "name", toolCall.Function.Name, "error", err)
				errStr = err.Error()
				result = fmt.Sprintf("Error: %v", err)
			}

			// Emit tool completed event
			a.dispatcher.EmitAsync(ctx, hooks.EventToolCompleted, hooks.ToolEvent{
				Name:   toolCall.Function.Name,
				Params: toolParams,
				Result: result,
				Error:  errStr,
			})

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
// Deprecated: Use ProcessWithSession instead.
func (a *Agent) ProcessWithMemory(ctx context.Context, sessionID, content string) (string, error) {
	return a.ProcessWithSession(ctx, sessionID, content)
}

// GetSession retrieves a session by ID.
// Returns nil if sessions are not configured or session doesn't exist.
func (a *Agent) GetSession(ctx context.Context, sessionID string) (*sessions.Session, error) {
	if a.sessions == nil {
		return nil, fmt.Errorf("session store not configured")
	}
	return a.sessions.GetIfExists(ctx, sessionID)
}

// ListSessions returns all session IDs.
func (a *Agent) ListSessions(ctx context.Context) ([]string, error) {
	if a.sessions == nil {
		return nil, fmt.Errorf("session store not configured")
	}
	return a.sessions.List(ctx)
}

// DeleteSession removes a session.
func (a *Agent) DeleteSession(ctx context.Context, sessionID string) error {
	if a.sessions == nil {
		return fmt.Errorf("session store not configured")
	}
	return a.sessions.Delete(ctx, sessionID)
}

// ClearSession clears the conversation history for a session.
func (a *Agent) ClearSession(ctx context.Context, sessionID string) error {
	if a.sessions == nil {
		return fmt.Errorf("session store not configured")
	}

	session, err := a.sessions.GetIfExists(ctx, sessionID)
	if err != nil {
		return err
	}

	session.Clear()
	return a.sessions.Save(ctx, session)
}

// SessionStore returns the session store, or nil if not configured.
func (a *Agent) SessionStore() *sessions.Store {
	return a.sessions
}

// ContextEngine returns the context engine, or nil if not configured.
func (a *Agent) ContextEngine() *agentctx.Engine {
	return a.contextEngine
}

// SetContextEngine sets the context engine.
func (a *Agent) SetContextEngine(engine *agentctx.Engine) {
	a.contextEngine = engine
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

	// Close hooks
	if a.hooks != nil {
		if err := a.hooks.Close(); err != nil {
			a.logger.Error("failed to close hooks", "error", err)
		}
	}

	// Close storage
	if a.storage != nil {
		if err := a.storage.Close(); err != nil {
			a.logger.Error("failed to close storage", "error", err)
		}
	}

	return a.client.Close()
}

// HookRegistry returns the hook registry.
func (a *Agent) HookRegistry() *hooks.Registry {
	return a.hooks
}

// Dispatcher returns the event dispatcher.
func (a *Agent) Dispatcher() *hooks.Dispatcher {
	return a.dispatcher
}

// InitHooks initializes all registered hooks.
func (a *Agent) InitHooks(ctx context.Context) error {
	if a.hooks == nil {
		return nil
	}
	return a.hooks.Init(ctx)
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
