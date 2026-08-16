package agentruntime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/team/chats"
)

// SecretSource resolves an agent's secrets, by ID, into the environment map its
// runtime instance injects (env-var name → value). It is the seam through which
// agent-scoped secrets (RMI-OMNIAGENT-310) reach the builder, keeping the
// builder independent of the secret store and unit-testable with a fake. A nil
// SecretSource means no secrets are injected (prior behavior). Resolution runs
// in system context — the runtime is a system principal, not a user.
//
// skillNames is the agent's own enabled-skill list (AgentConfig.Skills) — an
// implementation that layers in skill-scoped fallback bindings (RMI-OMNIAGENT-208)
// must restrict them to these names, never the deployment's full skill set, or
// it leaks a binding into an agent that doesn't have that skill enabled.
type SecretSource interface {
	ResolveSecrets(ctx context.Context, agentID uuid.UUID, skillNames []string) (map[string]string, error)
}

// BuilderConfig configures the production AgentBuilder.
type BuilderConfig struct {
	// Defaults supplies the deployment-wide LLM configuration (provider, model,
	// API key, base URL, timezone, temperature/token limits). An agent's own
	// Model/Provider override the defaults when set; everything else is inherited.
	Defaults agent.Config

	// BaseOptions are agent.Options shared by every built instance — the
	// deployment's skill source (skill pack/manager or dirs), tools, session
	// store, and rollover policy. The builder layers each agent's enabled-skill
	// subset (WithSkillIncludes) on top, so BaseOptions supplies where skills
	// come from and the agent's config selects which are active.
	BaseOptions []agent.Option

	// Secrets resolves each agent's agent-scoped secrets, injected into the
	// instance's secrets-aware skills (notably per-agent MCP subprocess env) via
	// agent.WithSecretEnv. Optional; nil means no secrets are injected. Because
	// each instance is built with only its own agent's secrets, two agents load
	// disjoint secrets with no cross-leak (RMI-OMNIAGENT-310).
	Secrets SecretSource

	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// AgentBuilder is the production Builder: it constructs a real *agent.Agent from
// an agent's persona + enabled skills + agent-scoped secrets. The persona
// becomes the system prompt; the enabled-skill subset is applied via
// WithSkillIncludes over the shared BaseOptions skill source; model/provider
// fall back to the deployment defaults; and, when a SecretSource is configured,
// the agent's secrets are resolved and injected via WithSecretEnv (per-agent MCP
// subprocess env and other secrets-aware skills), so two agents load disjoint
// secrets with no cross-leak (RMI-OMNIAGENT-310).
type AgentBuilder struct {
	cfg BuilderConfig
}

// AgentBuilder is a Builder.
var _ Builder = (*AgentBuilder)(nil)

// NewAgentBuilder creates the production builder.
func NewAgentBuilder(cfg BuilderConfig) *AgentBuilder {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &AgentBuilder{cfg: cfg}
}

// Build constructs the agent instance for cfg. It satisfies Builder.
func (b *AgentBuilder) Build(ctx context.Context, cfg AgentConfig) (chats.AgentProcessor, error) {
	ac := b.cfg.Defaults
	ac.Logger = b.cfg.Logger
	if cfg.Provider != "" {
		ac.Provider = cfg.Provider
	}
	if cfg.Model != "" {
		ac.Model = cfg.Model
	}
	// The persona is the agent's system prompt; an agent with no persona
	// inherits the deployment default (leave ac.SystemPrompt untouched).
	if cfg.Persona != "" {
		ac.SystemPrompt = cfg.Persona
	}
	// Attribute this instance's automatic memory to the agent. Per-chat memory
	// subject/tenant is stamped on the turn context by the chats service
	// (memscope, RMI-114) and takes precedence at recall/rollover time.
	ac.AgentID = cfg.ID.String()

	opts := make([]agent.Option, 0, len(b.cfg.BaseOptions)+2)
	opts = append(opts, b.cfg.BaseOptions...)
	if len(cfg.Skills) > 0 {
		opts = append(opts, agent.WithSkillIncludes(cfg.Skills...))
	}

	// Bind this agent's secrets to its own instance. Because each instance
	// receives only its own agent's secrets, two agents' skills (and MCP
	// subprocess environments) are disjoint (RMI-OMNIAGENT-310).
	if b.cfg.Secrets != nil {
		env, err := b.cfg.Secrets.ResolveSecrets(ctx, cfg.ID, cfg.Skills)
		if err != nil {
			return nil, fmt.Errorf("resolve secrets for agent %q: %w", cfg.Slug, err)
		}
		if len(env) > 0 {
			opts = append(opts, agent.WithSecretEnv(env))
		}
	}

	ag, err := agent.New(ac, opts...)
	if err != nil {
		return nil, fmt.Errorf("build agent %q: %w", cfg.Slug, err)
	}
	return ag, nil
}
