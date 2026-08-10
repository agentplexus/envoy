package agentruntime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/team/chats"
)

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

	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// AgentBuilder is the production Builder: it constructs a real *agent.Agent from
// an agent's persona + enabled skills. The persona becomes the system prompt;
// the enabled-skill subset is applied via WithSkillIncludes over the shared
// BaseOptions skill source; model/provider fall back to the deployment defaults.
//
// Agent-scoped secret binding — injecting the agent's secrets and per-agent MCP
// subprocess env, and proving two agents load disjoint skills/secrets with no
// cross-leak — is RMI-OMNIAGENT-310, layered on this builder next.
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
func (b *AgentBuilder) Build(_ context.Context, cfg AgentConfig) (chats.AgentProcessor, error) {
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

	opts := make([]agent.Option, 0, len(b.cfg.BaseOptions)+1)
	opts = append(opts, b.cfg.BaseOptions...)
	if len(cfg.Skills) > 0 {
		opts = append(opts, agent.WithSkillIncludes(cfg.Skills...))
	}

	ag, err := agent.New(ac, opts...)
	if err != nil {
		return nil, fmt.Errorf("build agent %q: %w", cfg.Slug, err)
	}
	return ag, nil
}
