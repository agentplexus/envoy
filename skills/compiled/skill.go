package compiled

import (
	"context"

	"github.com/plexusone/omniskill/skill"
	"github.com/plexusone/omnistorage-core/kvs"
)

// Skill is the interface that compiled skills implement.
// This is compatible with skill.Skill from omniskill.
type Skill interface {
	// Name returns the skill identifier (e.g., "invest", "weather").
	Name() string

	// Description returns a human-readable description of the skill.
	Description() string

	// Tools returns the tools provided by this skill.
	Tools() []skill.Tool

	// Init initializes the skill. Called once when the agent starts.
	Init(ctx context.Context) error

	// Close releases resources. Called when the agent shuts down.
	Close() error
}

// StorageAware is an optional interface for skills that need storage.
// If a skill implements this interface, the agent will inject the storage
// backend after creation and before Init() is called.
type StorageAware interface {
	SetStorage(s kvs.Store)
}

// AgentAware is an optional interface for skills that need agent access.
// If a skill implements this interface, the agent will inject itself
// after creation and before Init() is called.
// The agent is passed as interface{} to avoid import cycles.
type AgentAware interface {
	SetAgent(a interface{})
}

// SecretsAware is an optional interface for skills that accept injected secrets
// (e.g. an MCP server's subprocess environment). If a skill implements this
// interface, the agent injects the resolved secret env after creation and
// before Init() is called. Secrets are keyed by environment-variable name.
//
// This is the seam per-agent runtime instances use to bind agent-scoped secrets
// (RMI-OMNIAGENT-310): each instance is built with its own agent's secrets, so
// two agents' skills receive disjoint environments.
type SecretsAware interface {
	SetSecrets(env map[string]string)
}

// SecretRequirer is an optional interface for skills that declare which
// secrets (env-var names) they cannot function without. If a skill
// implements this interface, the agent checks the names against the
// resolved secret env after SecretsAware injection and before Init() is
// called — a missing required secret excludes the skill (with a logged
// reason) instead of the skill failing however its underlying provider
// reacts to an incomplete environment (RMI-OMNIAGENT-210). Mirrors
// skills.Skill.UnmetRequiredSecrets' gating for markdown-declared skills,
// for skills registered via agent.Option instead of SKILL.md discovery.
type SecretRequirer interface {
	RequiredSecrets() []string
}

// SkillInfo provides metadata about a skill for introspection.
type SkillInfo struct {
	Name        string
	Description string
	ToolCount   int
	Tools       []ToolInfo
}

// Info returns metadata about the skill.
func Info(s Skill) SkillInfo {
	tools := s.Tools()
	toolInfos := make([]ToolInfo, len(tools))
	for i, t := range tools {
		toolInfos[i] = ToolInfo{
			Name:        t.Name(),
			Description: t.Description(),
		}
	}

	return SkillInfo{
		Name:        s.Name(),
		Description: s.Description(),
		ToolCount:   len(tools),
		Tools:       toolInfos,
	}
}

// ToolInfo provides metadata about a tool.
type ToolInfo struct {
	Name        string
	Description string
}
