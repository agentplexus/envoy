// Package agent provides the AI agent runtime for omniagent.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/plexusone/omnillm"
	"github.com/plexusone/omnillm/provider"
	"github.com/plexusone/omnimemory/core"
	"github.com/plexusone/omnistorage-core/kvs"

	"github.com/plexusone/omniagent/agent/profiles"
	"github.com/plexusone/omniagent/agent/roles"
	agentctx "github.com/plexusone/omniagent/context"
	"github.com/plexusone/omniagent/cron"
	"github.com/plexusone/omniagent/hooks"
	"github.com/plexusone/omniagent/memscope"
	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omniagent/skills"
	"github.com/plexusone/omniagent/skills/compiled"
)

// Agent is the AI agent that processes messages.
type Agent struct {
	client         *omnillm.ChatClient
	tools          *ToolRegistry
	skills         []*skills.Skill   // Markdown skills (SKILL.md)
	skillManager   *skills.Manager   // Skill manager (if using packs)
	skillPacks     []fs.FS           // Embedded skill packs
	skillDirs      []string          // Skill directories
	skillIncludes  []string          // Include only these skills
	skillExcludes  []string          // Exclude these skills
	compiledSkills []compiled.Skill  // Compiled Go skills
	storage        kvs.Store         // Key-value storage backend
	secretEnv      map[string]string // Injected secrets (env-var name -> value) for secrets-aware skills
	sessions       *sessions.Store   // Persistent session storage
	memory         *core.Client      // OmniMemory client for semantic memory
	contextEngine  *agentctx.Engine  // Context management engine
	hooks          *hooks.Registry   // Event hook registry
	dispatcher     *hooks.Dispatcher
	config         Config
	logger         *slog.Logger
	location       *time.Location // Resolved timezone for temporal context
	mu             sync.RWMutex

	// Profile-related fields
	profile          *profiles.BootstrapProfile // Active bootstrap profile
	profileRegistry  *profiles.ProfileRegistry  // Profile registry for dynamic selection
	leanMode         *profiles.LeanMode         // Lean mode configuration
	progressReporter *profiles.ProgressReporter // Tool progress reporter

	// Role-related fields
	roleManager *roles.Manager // Role manager for persona-based behavior

	// toolsAllowHooks are synchronous pre-turn hooks that can narrow the
	// tool set submitted to the model for a single turn.
	toolsAllowHooks []hooks.ToolsAllowFunc

	// rolloverPolicy configures automatic session rollover (nil disables).
	rolloverPolicy *sessions.RolloverPolicy
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
	Timezone          string // IANA timezone for temporal context (empty = UTC)
	Logger            *slog.Logger
	ObservabilityHook omnillm.ObservabilityHook

	// Memory configuration
	TenantID string // Tenant ID for multi-tenancy (memory scope)
	AgentID  string // Agent ID for memory attribution
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

	// Resolve the timezone for temporal context once at construction so an
	// invalid configuration fails fast rather than silently degrading.
	location := time.UTC
	if config.Timezone != "" {
		location, err = time.LoadLocation(config.Timezone)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("invalid timezone %q: %w", config.Timezone, err)
		}
	}

	hookRegistry := hooks.NewRegistry()
	a := &Agent{
		client:     client,
		tools:      NewToolRegistry(),
		hooks:      hookRegistry,
		dispatcher: hooks.NewDispatcher(hookRegistry),
		config:     config,
		logger:     config.Logger,
		location:   location,
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(a); err != nil {
			// Close client on error
			client.Close()
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	// Initialize skill manager if packs or dirs are configured
	if err := a.initSkillManager(); err != nil {
		client.Close()
		return nil, fmt.Errorf("init skill manager: %w", err)
	}

	// Persist rolled-over sessions to memory. Registered unconditionally;
	// the handler no-ops when memory is not configured, and the event only
	// fires when a rollover policy is set.
	hookRegistry.RegisterHandler(hooks.EventSessionRollover, "session-memory", a.saveRolloverMemory)

	return a, nil
}

// initSkillManager initializes the skill manager from configured options.
func (a *Agent) initSkillManager() error {
	// Skip if a custom manager was provided
	if a.skillManager != nil {
		a.skills = a.skillManager.Available()
		a.logger.Info("using custom skill manager",
			"total", a.skillManager.Count(),
			"available", len(a.skills))
		return nil
	}

	// Skip if no packs configured (use legacy LoadSkills behavior)
	if len(a.skillPacks) == 0 && len(a.skillDirs) == 0 &&
		len(a.skillIncludes) == 0 && len(a.skillExcludes) == 0 {
		return nil
	}

	// Create and load manager
	a.skillManager = skills.NewManager(skills.ManagerConfig{
		Packs:    a.skillPacks,
		Dirs:     a.skillDirs,
		Includes: a.skillIncludes,
		Excludes: a.skillExcludes,
	})

	if err := a.skillManager.Load(); err != nil {
		return err
	}

	a.skills = a.skillManager.Available()
	a.logger.Info("skills loaded from manager",
		"total", a.skillManager.Count(),
		"available", len(a.skills))
	return nil
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

	// Apply automatic rollover before this turn: an idle or day-boundary
	// session ends here (its context is persisted via the rollover event)
	// and the turn continues on a fresh conversation.
	a.maybeRolloverSession(ctx, session)

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
func (a *Agent) processInternal(ctx context.Context, session *sessions.Session, content string) (response string, err error) {
	// Settle every run — normal completion, error, or abort — through the
	// agent.end lifecycle event. The deferred emit guarantees the event
	// fires exactly once on all terminal paths.
	runStart := time.Now()
	defer func() {
		a.emitAgentEnd(ctx, session, response, err, time.Since(runStart))
	}()

	model := a.modelForSession(session)
	a.logger.Info("processing message", "model", model, "provider", a.config.Provider)

	// Emit message received event
	a.dispatcher.EmitAsync(ctx, hooks.EventMessageReceived, hooks.MessageEvent{
		Role:    "user",
		Content: content,
	})

	// Recall relevant memories if configured
	var memories []*core.Memory
	if a.memory != nil {
		sessionID := ""
		if session != nil {
			sessionID = session.ID
		}
		// A caller may stamp a memory scope on the context (e.g. a chat turn
		// scopes to TenantID=team, SubjectID="chat:<id>" — RMI-OMNIAGENT-114).
		// When unset, fall back to the agent's tenant and the session subject.
		tenantID, subjectID := memscope.Resolve(ctx, a.config.TenantID, sessionID)
		resp, err := a.memory.Recall(ctx, &core.RecallRequest{
			Context: core.Context{
				TenantID:  tenantID,
				SubjectID: subjectID,
				AgentID:   a.config.AgentID,
				SessionID: sessionID,
			},
			Query:      content,
			MaxResults: 5,
		})
		if err != nil {
			a.logger.Warn("failed to recall memories", "error", err)
		} else if len(resp.Memories) > 0 {
			memories = resp.Memories
			a.logger.Info("recalled memories", "count", len(memories))
		}
	}

	// Build messages array
	var messages []provider.Message

	// Add system prompt with injected skills and memories
	systemPrompt := a.buildSystemPromptWithMemories(memories)
	if systemPrompt != "" {
		a.logger.Info("using system prompt", "length", len(systemPrompt), "skills", len(a.skills), "memories", len(memories))
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

	// Add tools if available, scoped by the session's tool overrides.
	tools := a.filterToolsForSession(session, a.tools.GetTools())
	a.logger.Info("tools available for request", "count", len(tools))
	for _, t := range tools {
		paramsJSON, _ := json.Marshal(t.Function.Parameters)
		a.logger.Info("tool in request", "name", t.Function.Name, "type", t.Type, "params", string(paramsJSON))
	}

	turnSessionID := ""
	if session != nil {
		turnSessionID = session.ID
	}

	// Assistant text emitted on tool-call turns, preserved so it is not
	// lost from the final output when a tool call follows it.
	var textSegments []string

	// Process with potential tool calls (max 5 iterations to prevent infinite loops)
	for i := 0; i < 5; i++ {
		req := &provider.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
		}

		if a.config.Temperature > 0 {
			req.Temperature = &a.config.Temperature
		}
		if a.config.MaxTokens > 0 {
			req.MaxTokens = &a.config.MaxTokens
		}

		// Pre-turn hooks may narrow the tool set for this iteration only;
		// the registry itself is never mutated, and the full set is offered
		// again on the next iteration so a later turn can widen.
		turnTools := a.narrowToolsForTurn(ctx, turnSessionID, content, i, tools)
		if len(turnTools) > 0 {
			req.Tools = turnTools
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
			// Join any assistant text preserved from earlier tool-call
			// turns with the final response. Single-turn output without
			// preserved segments is returned unchanged.
			final := choice.Message.Content
			if len(textSegments) > 0 {
				final = joinAssistantSegments(append(textSegments, choice.Message.Content))
			}
			// Emit message sent event
			a.dispatcher.EmitAsync(ctx, hooks.EventMessageSent, hooks.MessageEvent{
				Role:    "assistant",
				Content: final,
			})
			// No tool calls, return the response
			return final, nil
		}

		// Execute tool calls
		a.logger.Info("executing tool calls", "count", len(choice.Message.ToolCalls))

		// Preserve assistant text emitted alongside the tool calls so it
		// survives into the final output instead of being discarded.
		if strings.TrimSpace(choice.Message.Content) != "" {
			textSegments = append(textSegments, choice.Message.Content)
		}

		// Add assistant message with tool calls to conversation, keeping
		// its Content so session history stays faithful to the model output.
		messages = append(messages, provider.Message{
			Role:      provider.RoleAssistant,
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		})

		// Tool executions carry the session's authorizing principal so
		// tools that create durable work (e.g. cron jobs) can stamp it.
		execCtx := ctx
		if turnSessionID != "" {
			execCtx = cron.ContextWithPrincipal(ctx, cron.SessionPrincipal(turnSessionID))
		}

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

			result, err := a.tools.Execute(execCtx, toolCall.Function.Name, []byte(toolCall.Function.Arguments))
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

// maybeRolloverSession applies the configured rollover policy to a loaded
// session. When the policy triggers, the ended conversation is snapshotted
// and emitted synchronously as a session.rollover event (so persistence
// hooks complete before the fresh conversation begins), then the session's
// messages are cleared in place under the same session ID. Manual clears
// never pass through here and emit no rollover event.
func (a *Agent) maybeRolloverSession(ctx context.Context, session *sessions.Session) {
	if a.rolloverPolicy == nil || session == nil {
		return
	}

	now := time.Now()
	reason, ok := a.rolloverPolicy.ShouldRollover(session, now, a.timezone())
	if !ok {
		return
	}

	// Snapshot the ended conversation before resetting it.
	messages := session.GetMessages()
	transcript := make([]hooks.MessageEvent, 0, len(messages))
	for _, m := range messages {
		transcript = append(transcript, hooks.MessageEvent{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	event := hooks.SessionRolloverEvent{
		SessionID:  session.ID,
		Reason:     string(reason),
		Transcript: transcript,
		StartedAt:  session.CreatedAt,
		EndedAt:    session.UpdatedAt,
	}

	// Synchronous emit: persistence handlers (e.g. the session-memory hook)
	// finish before the turn proceeds. A handler failure is logged, never
	// fails the turn — losing a memory write must not block the user.
	if a.dispatcher != nil {
		if err := a.dispatcher.EmitWithSession(ctx, hooks.EventSessionRollover, session.ID, event); err != nil {
			a.logger.Error("session rollover hook failed", "session_id", session.ID, "error", err)
		}
	}

	// Reset the conversation in place: same session ID, fresh messages.
	// Session configuration (metadata, tool overrides) is retained.
	session.Messages = []provider.Message{}
	session.UpdatedAt = now

	if a.sessions != nil {
		if err := a.sessions.Save(ctx, session); err != nil {
			a.logger.Error("failed to save rolled-over session", "session_id", session.ID, "error", err)
		}
	}

	a.logger.Info("session rolled over",
		"session_id", session.ID, "reason", reason, "messages_persisted", len(transcript))
}

// saveRolloverMemory persists a rolled-over session's conversation to
// semantic memory. No-op when memory is not configured.
func (a *Agent) saveRolloverMemory(ctx context.Context, e hooks.Event) error {
	if a.memory == nil {
		return nil
	}
	data, ok := e.Data.(hooks.SessionRolloverEvent)
	if !ok {
		return nil
	}

	content := formatRolloverMemory(data, a.timezone())
	// Persist under the same scope the turn recalled from: a stamped memory
	// scope (e.g. a chat's TenantID=team, SubjectID="chat:<id>") overrides the
	// agent tenant and the session subject, so a chat's rolled-over context is
	// stored to that chat's memory (RMI-OMNIAGENT-114). The rollover event is
	// emitted synchronously on the turn context, so the scope is still present.
	tenantID, subjectID := memscope.Resolve(ctx, a.config.TenantID, data.SessionID)
	_, err := a.memory.Add(ctx, &core.AddRequest{
		Context: core.Context{
			TenantID:  tenantID,
			SubjectID: subjectID,
			AgentID:   a.config.AgentID,
			SessionID: data.SessionID,
		},
		Type:    core.MemoryTypeObservation,
		Content: content,
		Metadata: map[string]any{
			"source":     "session_rollover",
			"reason":     data.Reason,
			"session_id": data.SessionID,
		},
	})
	if err != nil {
		return fmt.Errorf("save rollover memory: %w", err)
	}
	return nil
}

// Bounds for rollover memory records, keeping single writes from growing
// with unbounded conversation length.
const (
	rolloverMemoryMaxMessages = 20
	rolloverMemoryMaxMsgChars = 500
)

// formatRolloverMemory renders a rolled-over conversation as one memory
// record. The boundary date is formatted in the user's timezone, and the
// transcript is capped to its most recent messages.
func formatRolloverMemory(data hooks.SessionRolloverEvent, loc *time.Location) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Session %s ended (%s) on %s.\n",
		data.SessionID, data.Reason, data.EndedAt.In(loc).Format("2006-01-02"))

	transcript := data.Transcript
	if len(transcript) > rolloverMemoryMaxMessages {
		fmt.Fprintf(&sb, "Conversation (last %d of %d messages):\n",
			rolloverMemoryMaxMessages, len(transcript))
		transcript = transcript[len(transcript)-rolloverMemoryMaxMessages:]
	} else {
		sb.WriteString("Conversation:\n")
	}

	for _, m := range transcript {
		content := m.Content
		if len(content) > rolloverMemoryMaxMsgChars {
			// Cut on a rune boundary so multi-byte characters survive.
			cut := rolloverMemoryMaxMsgChars
			for cut > 0 && !utf8.RuneStart(content[cut]) {
				cut--
			}
			content = content[:cut] + "…"
		}
		fmt.Fprintf(&sb, "%s: %s\n", m.Role, content)
	}

	return sb.String()
}

// filterToolsForSession applies the session's tool overrides to the tool
// set: individually disabled tools, disabled MCP servers, and per-server
// denied MCP tools are removed for this session only. The shared registry
// is never mutated, so concurrent sessions with different overrides get
// independent tool sets.
func (a *Agent) filterToolsForSession(session *sessions.Session, tools []provider.Tool) []provider.Tool {
	if session == nil || session.ToolOverrides.IsZero() || len(tools) == 0 {
		return tools
	}
	ov := session.ToolOverrides

	// MCP-scoped overrides need tool provenance from the registry.
	byName := make(map[string]ToolDescriptor)
	if len(ov.MCPServers) > 0 || len(ov.MCPToolsDeny) > 0 {
		for _, d := range a.tools.Describe() {
			byName[d.Name] = d
		}
	}

	filtered := make([]provider.Tool, 0, len(tools))
	for _, t := range tools {
		d := byName[t.Function.Name]
		if ov.Denies(t.Function.Name, d.Source, d.SourceName, d.SourceTool) {
			continue
		}
		filtered = append(filtered, t)
	}

	if len(filtered) != len(tools) && a.logger != nil {
		a.logger.Info("session tool overrides narrowed tools",
			"session_id", session.ID, "before", len(tools), "after", len(filtered))
	}
	return filtered
}

// modelForSession resolves the model for a session's turns: the session's
// override when set, otherwise the agent default (which SetSessionModel can
// update when a sticky change is requested).
func (a *Agent) modelForSession(session *sessions.Session) string {
	if session != nil && session.Model != "" {
		return session.Model
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config.Model
}

// SetSessionModel sets (or clears, with an empty model) a session's model
// override. When sticky is true, the model also becomes the agent's default
// so new sessions inherit it — best-effort and in-process only: the change
// is not persisted to the config file (configuration is load-only), so it
// lasts until restart. Changes take effect on the session's next turn.
func (a *Agent) SetSessionModel(ctx context.Context, sessionID, model string, sticky bool) error {
	if a.sessions == nil {
		return fmt.Errorf("session store not configured")
	}

	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	session.Model = model
	session.UpdatedAt = time.Now()
	if err := a.sessions.Save(ctx, session); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	if sticky && model != "" {
		a.mu.Lock()
		a.config.Model = model
		a.mu.Unlock()
		a.logger.Info("agent default model updated (sticky, in-process only — not persisted to config)",
			"model", model, "session_id", sessionID)
	}

	return nil
}

// ResolvePrincipal verifies an authorizing principal at execution time.
// Session principals ("session:<id>") resolve against the agent's session
// store: the principal is valid only while its session still exists.
// Everything else — unknown principal forms, no session store, expired or
// deleted sessions — resolves false, keeping scheduled work fail-closed.
// Authority therefore follows the creating session's lifetime: a job that
// outlives its session is denied rather than run with orphaned authority.
func (a *Agent) ResolvePrincipal(ctx context.Context, principal string) bool {
	sessionID, ok := cron.SessionIDFromPrincipal(principal)
	if !ok {
		return false
	}
	if a.sessions == nil {
		return false
	}
	if _, err := a.sessions.GetIfExists(ctx, sessionID); err != nil {
		return false
	}
	return true
}

// SetSessionToolOverrides persists per-session tool overrides. The session
// is created if it does not exist; passing nil clears the overrides.
// Changes take effect on the session's next turn.
func (a *Agent) SetSessionToolOverrides(ctx context.Context, sessionID string, overrides *sessions.ToolOverrides) error {
	if a.sessions == nil {
		return fmt.Errorf("session store not configured")
	}

	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	session.ToolOverrides = overrides
	session.UpdatedAt = time.Now()

	if err := a.sessions.Save(ctx, session); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

// narrowToolsForTurn applies the registered pre-turn hooks to the tool set
// for one loop iteration. Hooks compose by intersection: each hook sees the
// names surviving the previous one. A nil return leaves the set unchanged;
// an empty return removes all optional tools for this turn.
func (a *Agent) narrowToolsForTurn(ctx context.Context, sessionID, content string, iteration int, tools []provider.Tool) []provider.Tool {
	if len(a.toolsAllowHooks) == 0 || len(tools) == 0 {
		return tools
	}

	current := tools
	for _, hook := range a.toolsAllowHooks {
		names := make([]string, len(current))
		for i, t := range current {
			names[i] = t.Function.Name
		}
		allow := hook(ctx, hooks.PromptTurn{
			SessionID: sessionID,
			Content:   content,
			Iteration: iteration,
			Tools:     names,
		})
		if allow == nil {
			continue
		}
		current = filterToolsByAllow(current, allow)
		if len(current) == 0 {
			break
		}
	}

	if len(current) != len(tools) && a.logger != nil {
		a.logger.Info("pre-turn hook narrowed tools",
			"iteration", iteration, "before", len(tools), "after", len(current))
	}
	return current
}

// filterToolsByAllow returns the intersection of tools with the allow set,
// preserving the original order. An empty allow set yields no tools.
func filterToolsByAllow(tools []provider.Tool, allow []string) []provider.Tool {
	allowed := make(map[string]struct{}, len(allow))
	for _, name := range allow {
		allowed[name] = struct{}{}
	}

	filtered := make([]provider.Tool, 0, len(tools))
	for _, t := range tools {
		if _, ok := allowed[t.Function.Name]; ok {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// emitAgentEnd emits the agent.end lifecycle event for a terminal run state.
// Delivery uses a non-cancellable context so an aborted run still settles
// through the event instead of having its emission suppressed by the abort.
func (a *Agent) emitAgentEnd(ctx context.Context, session *sessions.Session, response string, runErr error, duration time.Duration) {
	if a.dispatcher == nil {
		return
	}

	success, errMsg, aborted := classifyAgentEnd(ctx.Err(), runErr)

	sessionID := ""
	if session != nil {
		sessionID = session.ID
	}

	a.dispatcher.EmitAsyncWithSession(context.WithoutCancel(ctx), hooks.EventAgentEnd, sessionID, hooks.AgentEndEvent{
		SessionID:  sessionID,
		Success:    success,
		Error:      errMsg,
		Aborted:    aborted,
		Response:   response,
		DurationMs: duration.Milliseconds(),
	})
}

// classifyAgentEnd derives the terminal classification for an agent run.
// Abort outranks error: when a run fails because the caller cancelled it,
// the event reports aborted with an empty error — a cancellation (or a
// timeout surfaced while aborting) is not misreported as a failure cause.
// A run that returned a result is a success even if the context was
// cancelled just after completion.
func classifyAgentEnd(ctxErr, runErr error) (success bool, errMsg string, aborted bool) {
	if runErr == nil {
		return true, "", false
	}
	if errors.Is(ctxErr, context.Canceled) || errors.Is(runErr, context.Canceled) {
		return false, "", true
	}
	return false, runErr.Error(), false
}

// joinAssistantSegments joins assistant text segments emitted across
// tool-call turns with a single blank line, normalizing newlines at each
// boundary so segments that already end or begin with newlines do not
// double-space. Blank segments are skipped.
func joinAssistantSegments(segments []string) string {
	result := ""
	for _, seg := range segments {
		if strings.TrimSpace(seg) == "" {
			continue
		}
		if result == "" {
			result = seg
			continue
		}
		result = strings.TrimRight(result, "\n") + "\n\n" + strings.TrimLeft(seg, "\n")
	}
	return result
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

// Memory returns the omnimemory client, or nil if not configured.
func (a *Agent) Memory() *core.Client {
	return a.memory
}

// SetMemory sets the omnimemory client.
func (a *Agent) SetMemory(client *core.Client) {
	a.memory = client
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

// Tools returns the tool registry for accessing registered tools.
func (a *Agent) Tools() *ToolRegistry {
	return a.tools
}

// Close closes the agent and releases resources.
func (a *Agent) Close() error {
	// Close role manager first
	if a.roleManager != nil {
		if err := a.roleManager.Close(); err != nil {
			a.logger.Error("failed to close role manager", "error", err)
		}
	}

	// Close compiled skills
	if err := a.CloseCompiledSkills(); err != nil {
		a.logger.Error("failed to close compiled skills", "error", err)
	}

	// Close hooks
	if a.hooks != nil {
		if err := a.hooks.Close(); err != nil {
			a.logger.Error("failed to close hooks", "error", err)
		}
	}

	// Close memory client
	if a.memory != nil {
		if err := a.memory.Close(); err != nil {
			a.logger.Error("failed to close memory client", "error", err)
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

// SkillManager returns the skill manager, or nil if not configured.
func (a *Agent) SkillManager() *skills.Manager {
	return a.skillManager
}

// buildSystemPromptWithMemories builds the system prompt with skills and recalled memories.
func (a *Agent) buildSystemPromptWithMemories(memories []*core.Memory) string {
	basePrompt := a.config.SystemPrompt

	// Prepend role system prompt if configured
	if a.roleManager != nil {
		rolePrompt, err := a.roleManager.SystemPrompt(context.Background())
		if err == nil && rolePrompt != "" {
			if basePrompt != "" {
				basePrompt = rolePrompt + "\n\n" + basePrompt
			} else {
				basePrompt = rolePrompt
			}
		}
	}

	// Apply profile modifications if active
	if a.profile != nil {
		basePrompt = a.profile.BuildSystemPrompt(basePrompt)
	}

	// Inject memories if available
	if len(memories) > 0 {
		basePrompt = a.injectMemoriesIntoPrompt(basePrompt, memories)
	}

	if len(a.skills) > 0 {
		basePrompt = skills.InjectIntoPrompt(basePrompt, a.skills, skills.DefaultInjectConfig())
	}

	// Append temporal context last (volatile suffix): it is recomputed on
	// every prompt build so long-running sessions never reason with a stale
	// date, and keeping it after the stable content means it will not
	// invalidate a cached prefix if prompt caching is added later.
	temporal := temporalContext(time.Now(), a.timezone())
	if basePrompt == "" {
		return temporal
	}
	return basePrompt + "\n\n" + temporal
}

// timezone returns the agent's resolved timezone, defaulting to UTC when the
// agent was constructed without one (e.g. zero-value Agent in tests).
func (a *Agent) timezone() *time.Location {
	if a.location == nil {
		return time.UTC
	}
	return a.location
}

// temporalContext renders the date/timezone block appended to the system
// prompt. It deliberately emits a coarse date stamp rather than a clock time,
// which would be wrong minutes after the prompt was built.
func temporalContext(now time.Time, loc *time.Location) string {
	local := now.In(loc)
	return fmt.Sprintf("## Temporal Context\n\nCurrent date: %s (%s)\nTimezone: %s\n\nThe date above is refreshed on every turn; prefer it over any dates mentioned earlier in the conversation.",
		local.Format("2006-01-02"), local.Weekday(), loc.String())
}

// injectMemoriesIntoPrompt adds recalled memories to the system prompt.
func (a *Agent) injectMemoriesIntoPrompt(prompt string, memories []*core.Memory) string {
	if len(memories) == 0 {
		return prompt
	}

	var sb strings.Builder
	sb.WriteString(prompt)
	sb.WriteString("\n\n## Relevant Memories\n\n")
	sb.WriteString("The following memories have been recalled based on the current context:\n\n")

	for i, m := range memories {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, m.Type, m.Content))
	}

	return sb.String()
}

// Profile returns the active bootstrap profile, or nil if not set.
func (a *Agent) Profile() *profiles.BootstrapProfile {
	return a.profile
}

// SetProfile sets the active bootstrap profile.
func (a *Agent) SetProfile(profile *profiles.BootstrapProfile) {
	a.profile = profile
}

// ProfileRegistry returns the profile registry, or nil if not configured.
func (a *Agent) ProfileRegistry() *profiles.ProfileRegistry {
	return a.profileRegistry
}

// ActivateProfile activates a profile by name from the registry.
// Returns an error if the profile is not found.
func (a *Agent) ActivateProfile(ctx context.Context, name string) error {
	if a.profileRegistry == nil {
		return fmt.Errorf("profile registry not configured")
	}

	profile, ok := a.profileRegistry.Get(name)
	if !ok {
		return fmt.Errorf("profile not found: %s", name)
	}

	// Apply lean mode if configured
	if a.leanMode != nil && a.leanMode.Enabled {
		profile = profile.Clone()
		a.leanMode.Apply(profile)
	}

	a.profile = profile
	a.logger.Info("profile activated", "name", name)
	return nil
}

// LeanMode returns the lean mode configuration, or nil if not set.
func (a *Agent) LeanMode() *profiles.LeanMode {
	return a.leanMode
}

// SetLeanMode sets the lean mode configuration.
func (a *Agent) SetLeanMode(mode *profiles.LeanMode) {
	a.leanMode = mode
}

// ProgressReporter returns the progress reporter, or nil if not configured.
func (a *Agent) ProgressReporter() *profiles.ProgressReporter {
	return a.progressReporter
}

// SetProgressReporter sets the progress reporter.
func (a *Agent) SetProgressReporter(reporter *profiles.ProgressReporter) {
	a.progressReporter = reporter
}

// RoleManager returns the role manager, or nil if not configured.
func (a *Agent) RoleManager() *roles.Manager {
	return a.roleManager
}

// SetRoleManager sets the role manager.
func (a *Agent) SetRoleManager(mgr *roles.Manager) {
	a.roleManager = mgr
}

// InitRole initializes the role with its skills.
// This should be called after agent creation if using WithRole.
func (a *Agent) InitRole(ctx context.Context) error {
	if a.roleManager == nil {
		return nil
	}
	return a.roleManager.Init(ctx)
}
