package agent

import (
	agentctx "github.com/plexusone/omniagent/context"
	"github.com/plexusone/omniagent/cron"
	"github.com/plexusone/omniagent/hooks"
	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omniagent/skills/compiled"
	mcpskill "github.com/plexusone/omniagent/skills/remote/mcp"
	"github.com/plexusone/omnistorage"
)

// Option configures the agent.
type Option func(*Agent) error

// WithCompiledSkill registers a compiled skill with the agent.
// Multiple skills can be registered by calling this option multiple times.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithCompiledSkill(investSkill),
//	    agent.WithCompiledSkill(weatherSkill),
//	)
func WithCompiledSkill(skill compiled.Skill) Option {
	return func(a *Agent) error {
		return a.RegisterCompiledSkill(skill)
	}
}

// WithStorage sets the storage backend for the agent.
// Storage is automatically injected into any storage-aware compiled skills.
//
// Example:
//
//	sqliteStore, _ := sqlite.New(sqlite.Config{Path: "data.db"})
//	agent, err := agent.New(config,
//	    agent.WithStorage(sqliteStore),
//	    agent.WithCompiledSkill(investSkill),
//	)
func WithStorage(s omnistorage.Store) Option {
	return func(a *Agent) error {
		a.SetStorage(s)
		return nil
	}
}

// WithTool registers a single tool with the agent.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithTool(myTool),
//	)
func WithTool(tool Tool) Option {
	return func(a *Agent) error {
		a.RegisterTool(tool)
		return nil
	}
}

// WithSessionStore sets the session store for conversation persistence.
// This enables ProcessWithSession to maintain conversation history.
//
// Example:
//
//	sqliteStore, _ := sqlite.New(sqlite.Config{Path: "data.db"})
//	sessionStore := sessions.NewStore(sessions.StoreConfig{Backend: sqliteStore})
//	agent, err := agent.New(config,
//	    agent.WithSessionStore(sessionStore),
//	)
func WithSessionStore(store *sessions.Store) Option {
	return func(a *Agent) error {
		a.sessions = store
		return nil
	}
}

// WithSessionsFromStorage creates a session store from the given KVS backend.
// This is a convenience option that combines WithStorage and WithSessionStore.
//
// Example:
//
//	sqliteStore, _ := sqlite.New(sqlite.Config{Path: "data.db"})
//	agent, err := agent.New(config,
//	    agent.WithSessionsFromStorage(sqliteStore),
//	)
func WithSessionsFromStorage(backend omnistorage.Store) Option {
	return func(a *Agent) error {
		a.SetStorage(backend)
		a.sessions = sessions.NewStore(sessions.StoreConfig{
			Backend: backend,
			TTL:     sessions.DefaultSessionTTL,
		})
		return nil
	}
}

// WithContextEngine sets the context engine for conversation management.
// This enables automatic windowing and token limit enforcement.
//
// Example:
//
//	engine := context.New(context.Config{
//	    MaxMessages: 50,
//	    MaxTokens:   8000,
//	})
//	agent, err := agent.New(config,
//	    agent.WithContextEngine(engine),
//	)
func WithContextEngine(engine *agentctx.Engine) Option {
	return func(a *Agent) error {
		a.contextEngine = engine
		return nil
	}
}

// WithContextConfig creates a context engine with the given configuration.
// This is a convenience option for simple context configuration.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithContextConfig(context.Config{
//	        MaxMessages: 50,
//	        MaxTokens:   8000,
//	    }),
//	)
func WithContextConfig(cfg agentctx.Config) Option {
	return func(a *Agent) error {
		a.contextEngine = agentctx.New(cfg)
		return nil
	}
}

// WithMaxMessages sets a simple message limit for context.
// This creates a context engine with only a message limit.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithMaxMessages(50),
//	)
func WithMaxMessages(max int) Option {
	return func(a *Agent) error {
		a.contextEngine = agentctx.New(agentctx.Config{
			MaxMessages: max,
		})
		return nil
	}
}

// WithCronScheduler registers the cron skill for scheduled job execution.
// This requires storage to be configured (use WithStorage or WithSessionsFromStorage).
//
// Example:
//
//	sqliteStore, _ := sqlite.New(sqlite.Config{Path: "data.db"})
//	agent, err := agent.New(config,
//	    agent.WithSessionsFromStorage(sqliteStore),
//	    agent.WithCronScheduler(),
//	)
func WithCronScheduler() Option {
	return func(a *Agent) error {
		return a.RegisterCompiledSkill(cron.NewSkill())
	}
}

// WithMCPSkill registers an MCP server as a compiled skill.
// This spawns the MCP server as a subprocess and exposes its tools to the agent.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithMCPSkill(mcpskill.Config{
//	        Name:    "github",
//	        Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"},
//	        Env: map[string]string{
//	            "GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN"),
//	        },
//	    }),
//	)
func WithMCPSkill(cfg mcpskill.Config) Option {
	return func(a *Agent) error {
		return a.RegisterCompiledSkill(mcpskill.NewSkill(cfg))
	}
}

// WithHook registers a quick handler for an event type.
// This is the simplest way to handle events.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithHook(hooks.EventMessageReceived, func(ctx context.Context, e hooks.Event) error {
//	        msg := e.Data.(hooks.MessageEvent)
//	        log.Printf("Received: %s", msg.Content)
//	        return nil
//	    }),
//	)
func WithHook(event hooks.EventType, handler hooks.HandlerFunc) Option {
	return func(a *Agent) error {
		a.hooks.RegisterHandler(event, "", handler)
		return nil
	}
}

// WithNamedHook registers a named handler for an event type.
// The name is used in logging to identify the handler.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithNamedHook(hooks.EventMessageReceived, "audit-log", func(ctx context.Context, e hooks.Event) error {
//	        msg := e.Data.(hooks.MessageEvent)
//	        log.Printf("[AUDIT] Received: %s", msg.Content)
//	        return nil
//	    }),
//	)
func WithNamedHook(event hooks.EventType, name string, handler hooks.HandlerFunc) Option {
	return func(a *Agent) error {
		a.hooks.RegisterHandler(event, name, handler)
		return nil
	}
}

// WithCompiledHook registers a compiled hook for handling events.
// Compiled hooks implement the hooks.Hook interface and support
// initialization, cleanup, and multi-event handling.
//
// Example:
//
//	type AuditHook struct {
//	    logger *slog.Logger
//	}
//
//	func (h *AuditHook) Name() string { return "audit" }
//	func (h *AuditHook) Events() []hooks.EventType {
//	    return []hooks.EventType{hooks.EventMessageReceived, hooks.EventMessageSent}
//	}
//	func (h *AuditHook) Handle(ctx context.Context, event hooks.Event) error {
//	    h.logger.Info("event", "type", event.Type, "data", event.Data)
//	    return nil
//	}
//	func (h *AuditHook) Init(ctx context.Context) error { return nil }
//	func (h *AuditHook) Close() error { return nil }
//
//	agent, err := agent.New(config,
//	    agent.WithCompiledHook(&AuditHook{logger: slog.Default()}),
//	)
func WithCompiledHook(hook hooks.Hook) Option {
	return func(a *Agent) error {
		return a.hooks.RegisterHook(hook)
	}
}

// WithWebhookHook registers a webhook-based hook that sends events
// to an HTTP endpoint.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithWebhookHook(&hooks.WebhookHook{
//	        HookName:   "slack-notify",
//	        HookEvents: []hooks.EventType{hooks.EventMessageSent},
//	        URL:        "https://hooks.slack.com/services/xxx",
//	        Method:     "POST",
//	        Timeout:    5 * time.Second,
//	    }),
//	)
func WithWebhookHook(webhook *hooks.WebhookHook) Option {
	return func(a *Agent) error {
		a.hooks.RegisterWebhook(webhook)
		return nil
	}
}
