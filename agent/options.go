package agent

import (
	"fmt"
	"io"
	"io/fs"

	"github.com/plexusone/omnimemory/core"
	"github.com/plexusone/omniskill/role"
	"github.com/plexusone/omnistorage-core/kvs"

	"github.com/plexusone/omniagent/agent/profiles"
	"github.com/plexusone/omniagent/agent/roles"
	agentctx "github.com/plexusone/omniagent/context"
	"github.com/plexusone/omniagent/cron"
	"github.com/plexusone/omniagent/hooks"
	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omniagent/skills"
	"github.com/plexusone/omniagent/skills/compiled"
	mcpskill "github.com/plexusone/omniagent/skills/remote/mcp"
	openapiskill "github.com/plexusone/omniagent/skills/remote/openapi"
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
func WithStorage(s kvs.Store) Option {
	return func(a *Agent) error {
		a.SetStorage(s)
		return nil
	}
}

// WithSecretEnv injects secrets (keyed by environment-variable name) into the
// agent's secrets-aware compiled skills — notably MCP servers, whose subprocess
// environment receives them. Injection is order-independent: skills registered
// before or after this option all receive the secrets before Init().
//
// Per-agent runtime instances (RMI-OMNIAGENT-310) use this to bind agent-scoped
// secrets, so two agents' MCP subprocesses run with disjoint environments.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithMCPSkill(mcpskill.Config{Name: "github", Command: cmd}),
//	    agent.WithSecretEnv(map[string]string{"GITHUB_TOKEN": token}),
//	)
func WithSecretEnv(env map[string]string) Option {
	return func(a *Agent) error {
		a.SetSecretEnv(env)
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
func WithSessionsFromStorage(backend kvs.Store) Option {
	return func(a *Agent) error {
		a.SetStorage(backend)
		a.sessions = sessions.NewStore(sessions.StoreConfig{
			Backend: backend,
			TTL:     sessions.DefaultSessionTTL,
		})
		return nil
	}
}

// WithMemory sets the omnimemory client for semantic memory operations.
// This enables memory_store, memory_search, memory_recall, and other tools.
//
// Example:
//
//	client, _ := core.NewClient(core.ClientConfig{
//	    Providers: []core.ProviderConfig{
//	        {Name: core.ProviderNameMemory},
//	    },
//	})
//	agent, err := agent.New(config,
//	    agent.WithMemory(client),
//	)
func WithMemory(client *core.Client) Option {
	return func(a *Agent) error {
		a.memory = client
		return nil
	}
}

// WithMemoryConfig creates an omnimemory client with the given configuration.
// This is a convenience option that creates the client from configuration.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithMemoryConfig(core.ClientConfig{
//	        Providers: []core.ProviderConfig{
//	            {Name: core.ProviderNamePostgres, DSN: os.Getenv("DATABASE_URL")},
//	        },
//	    }),
//	)
func WithMemoryConfig(cfg core.ClientConfig) Option {
	return func(a *Agent) error {
		client, err := core.NewClient(cfg)
		if err != nil {
			return err
		}
		a.memory = client
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

// WithOpenAPISkill registers an OpenAPI spec as a compiled skill.
// This parses the OpenAPI specification and exposes operations as tools.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithOpenAPISkill(openapiskill.Config{
//	        Name:    "petstore",
//	        SpecURL: "https://petstore3.swagger.io/api/v3/openapi.json",
//	        Auth: openapiskill.AuthConfig{
//	            Type:   openapiskill.AuthAPIKey,
//	            APIKey: os.Getenv("PETSTORE_API_KEY"),
//	        },
//	    }),
//	)
func WithOpenAPISkill(cfg openapiskill.Config) Option {
	return func(a *Agent) error {
		return a.RegisterCompiledSkill(openapiskill.NewSkill(cfg))
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

// WithSessionRollover enables automatic session rollover. When a session
// exceeds the policy's idle timeout or crosses a calendar-day boundary, its
// conversation ends: a session.rollover event carrying the ended transcript
// fires (the built-in session-memory hook persists it to semantic memory
// when memory is configured), and the turn continues on a fresh
// conversation under the same session ID. Manual session clears are
// unaffected and emit no rollover.
//
// The day boundary resolves in policy.Location when set, otherwise the
// agent's configured Timezone (default UTC).
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithSessionRollover(sessions.RolloverPolicy{
//	        IdleTimeout: 4 * time.Hour,
//	        Daily:       true,
//	    }),
//	)
func WithSessionRollover(policy sessions.RolloverPolicy) Option {
	return func(a *Agent) error {
		if policy.IdleTimeout <= 0 && !policy.Daily {
			return fmt.Errorf("session rollover policy must set IdleTimeout or Daily")
		}
		a.rolloverPolicy = &policy
		return nil
	}
}

// WithToolsAllowHook registers a synchronous pre-turn hook that can narrow
// the tools submitted to the model for a single turn. The hook runs before
// every model call: returning nil leaves the tool set unchanged, an empty
// slice removes all optional tools for that turn, and a list of names
// narrows the set to its intersection with the available tools. The tool
// registry itself is never mutated.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithToolsAllowHook(func(ctx context.Context, turn hooks.PromptTurn) []string {
//	        if turn.Iteration > 2 {
//	            return []string{} // stop offering tools after 3 iterations
//	        }
//	        return nil
//	    }),
//	)
func WithToolsAllowHook(fn hooks.ToolsAllowFunc) Option {
	return func(a *Agent) error {
		if fn == nil {
			return fmt.Errorf("tools allow hook must not be nil")
		}
		a.toolsAllowHooks = append(a.toolsAllowHooks, fn)
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

// WithSkillPack registers an embedded skill pack with the agent.
// Skills from packs are loaded after directory skills, so directory
// skills with the same name will override pack skills.
//
// Example:
//
//	import skills "github.com/plexusone/omniagent-skills"
//
//	agent, err := agent.New(config,
//	    agent.WithSkillPack(skills.Default().FS()),
//	)
func WithSkillPack(pack fs.FS) Option {
	return func(a *Agent) error {
		a.skillPacks = append(a.skillPacks, pack)
		return nil
	}
}

// WithSkillDirs sets the directories to search for skills.
// Directory skills override embedded skills with the same name.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithSkillDirs("./skills", "~/.omniagent/skills"),
//	)
func WithSkillDirs(dirs ...string) Option {
	return func(a *Agent) error {
		a.skillDirs = dirs
		return nil
	}
}

// WithSkillIncludes limits loaded skills to only those with matching names.
// If not set, all discovered skills are included.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithSkillPack(skills.Default().FS()),
//	    agent.WithSkillIncludes("github", "weather"),
//	)
func WithSkillIncludes(names ...string) Option {
	return func(a *Agent) error {
		a.skillIncludes = names
		return nil
	}
}

// WithSkillExcludes prevents skills with matching names from being loaded.
// Applied after includes.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithSkillPack(skills.Default().FS()),
//	    agent.WithSkillExcludes("slack", "trello"),
//	)
func WithSkillExcludes(names ...string) Option {
	return func(a *Agent) error {
		a.skillExcludes = names
		return nil
	}
}

// WithSkillManager sets a custom skill manager for the agent.
// Use this for advanced skill loading scenarios.
//
// Example:
//
//	mgr := skills.NewManager(skills.ManagerConfig{
//	    Packs:    []fs.FS{skills.Default().FS()},
//	    Dirs:     []string{"./custom-skills"},
//	    Includes: []string{"github"},
//	})
//	mgr.Load()
//
//	agent, err := agent.New(config,
//	    agent.WithSkillManager(mgr),
//	)
func WithSkillManager(mgr *skills.Manager) Option {
	return func(a *Agent) error {
		a.skillManager = mgr
		return nil
	}
}

// WithBootstrapProfile sets the bootstrap profile for agent initialization.
// Profiles customize system prompts, tools, and context limits per-agent.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithBootstrapProfile(profiles.CodeAssistantProfile),
//	)
func WithBootstrapProfile(profile *profiles.BootstrapProfile) Option {
	return func(a *Agent) error {
		a.profile = profile
		return nil
	}
}

// WithProfileRegistry sets a profile registry for dynamic profile selection.
// This allows switching profiles at runtime based on context.
//
// Example:
//
//	registry := profiles.NewProfileRegistry()
//	registry.Register(profiles.CodeAssistantProfile)
//	registry.Register(profiles.RestrictedProfile)
//
//	agent, err := agent.New(config,
//	    agent.WithProfileRegistry(registry),
//	)
func WithProfileRegistry(registry *profiles.ProfileRegistry) Option {
	return func(a *Agent) error {
		a.profileRegistry = registry
		return nil
	}
}

// WithLeanMode enables lean mode for resource optimization.
// This is especially useful for local models with limited resources.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithLeanMode(profiles.NewLeanMode(profiles.LeanLevelModerate)),
//	)
func WithLeanMode(mode *profiles.LeanMode) Option {
	return func(a *Agent) error {
		a.leanMode = mode
		return nil
	}
}

// WithLeanLevel enables lean mode at the specified level.
// This is a convenience function for common lean mode configurations.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithLeanLevel(profiles.LeanLevelLight),
//	)
func WithLeanLevel(level profiles.LeanLevel) Option {
	return func(a *Agent) error {
		a.leanMode = profiles.NewLeanMode(level)
		return nil
	}
}

// WithProgressReporter sets the progress reporter for tool execution.
// This controls how tool execution progress is displayed.
//
// Example:
//
//	reporter := profiles.NewProgressReporter(profiles.ProgressModeVerbose, os.Stderr)
//
//	agent, err := agent.New(config,
//	    agent.WithProgressReporter(reporter),
//	)
func WithProgressReporter(reporter *profiles.ProgressReporter) Option {
	return func(a *Agent) error {
		a.progressReporter = reporter
		return nil
	}
}

// WithProgressMode sets the progress detail mode for tool execution.
// This is a convenience function that creates a progress reporter with the given mode.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithProgressMode(profiles.ProgressModeVerbose, os.Stderr),
//	)
func WithProgressMode(mode profiles.ProgressDetailMode, output io.Writer) Option {
	return func(a *Agent) error {
		a.progressReporter = profiles.NewProgressReporter(mode, output)
		return nil
	}
}

// WithRole registers a role with the agent.
// A role is a high-level agent persona that combines skills, workflows,
// and system prompts into a cohesive behavior.
//
// The role's required skills must be provided as compiled skills.
// The role's system prompt will be prepended to the agent's system prompt.
//
// Example:
//
//	pmRole := meetingpm.New(meetingpm.Config{
//	    DefaultConfluenceSpace: "TEAM",
//	})
//	meetingSkill := meeting.NewSkill(...)
//	googleSkill := google.NewSkill(...)
//
//	agent, err := agent.New(config,
//	    agent.WithRole(pmRole, meetingSkill, googleSkill),
//	)
func WithRole(r role.Role, skills ...compiled.Skill) Option {
	return func(a *Agent) error {
		// Create role manager with skills
		mgr, err := roles.NewManager(r, skills...)
		if err != nil {
			return err
		}

		a.roleManager = mgr

		// Also register the skills as compiled skills
		for _, s := range skills {
			if err := a.RegisterCompiledSkill(s); err != nil {
				return err
			}
		}

		return nil
	}
}

// WithRoleManager sets a pre-configured role manager.
// Use this for advanced role configuration scenarios.
//
// Example:
//
//	mgr, _ := roles.NewManager(pmRole, meetingSkill, googleSkill)
//	mgr.Init(ctx)
//
//	agent, err := agent.New(config,
//	    agent.WithRoleManager(mgr),
//	)
func WithRoleManager(mgr *roles.Manager) Option {
	return func(a *Agent) error {
		a.roleManager = mgr
		return nil
	}
}
