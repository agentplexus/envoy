package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omniskill/skill"
	"github.com/plexusone/omnistorage-core/kvs"
)

// compiledToolWrapper wraps a skill.Tool to implement agent.Tool.
type compiledToolWrapper struct {
	tool       skill.Tool
	skillName  string
	sourceType string // "skill" by default; "mcp" for MCP-backed skills
}

func (w *compiledToolWrapper) Name() string {
	return w.tool.Name()
}

// ToolSource implements ToolIdentity.
func (w *compiledToolWrapper) ToolSource() string {
	return w.sourceType
}

// ToolSourceName implements ToolIdentity.
func (w *compiledToolWrapper) ToolSourceName() string {
	return w.skillName
}

// ToolSourceToolName implements ToolIdentity.
func (w *compiledToolWrapper) ToolSourceToolName() string {
	return w.tool.Name()
}

func (w *compiledToolWrapper) Description() string {
	return w.tool.Description()
}

func (w *compiledToolWrapper) Parameters() map[string]interface{} {
	return skill.ParametersToJSONSchema(w.tool.Parameters())
}

func (w *compiledToolWrapper) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	// Parse arguments
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	// Execute the tool
	result, err := w.tool.Call(ctx, params)
	if err != nil {
		return "", err
	}

	// Convert result to string
	switch v := result.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		// Marshal as JSON
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Sprintf("%v", result), nil
		}
		return string(data), nil
	}
}

// RegisterCompiledSkill registers a compiled skill with the agent.
// This converts the skill's tools to agent tools and registers them.
func (a *Agent) RegisterCompiledSkill(skill compiled.Skill) error {
	// Check if skill implements StorageAware and inject storage
	if sa, ok := skill.(compiled.StorageAware); ok && a.storage != nil {
		sa.SetStorage(a.storage)
	}

	// Check if skill implements AgentAware and inject agent
	if aa, ok := skill.(compiled.AgentAware); ok {
		aa.SetAgent(a)
	}

	// Check if skill implements SecretsAware and inject secrets
	if sa, ok := skill.(compiled.SecretsAware); ok && len(a.secretEnv) > 0 {
		sa.SetSecrets(a.secretEnv)
	}

	// Determine the source kind: skills backed by an external protocol
	// (e.g. MCP) declare it via an optional SourceType method.
	sourceType := "skill"
	if st, ok := skill.(interface{ SourceType() string }); ok {
		sourceType = st.SourceType()
	}

	// Register all tools from the skill
	for _, tool := range skill.Tools() {
		wrapper := &compiledToolWrapper{
			tool:       tool,
			skillName:  skill.Name(),
			sourceType: sourceType,
		}
		a.tools.Register(wrapper)
	}

	// Track the skill for lifecycle management
	a.mu.Lock()
	a.compiledSkills = append(a.compiledSkills, skill)
	a.mu.Unlock()

	a.logger.Info("registered compiled skill",
		"name", skill.Name(),
		"tools", len(skill.Tools()),
	)

	return nil
}

// InitCompiledSkills initializes all registered compiled skills.
func (a *Agent) InitCompiledSkills(ctx context.Context) error {
	a.mu.RLock()
	skills := a.compiledSkills
	a.mu.RUnlock()

	for _, skill := range skills {
		if err := skill.Init(ctx); err != nil {
			return fmt.Errorf("init skill %q: %w", skill.Name(), err)
		}
		a.logger.Info("initialized compiled skill", "name", skill.Name())
	}

	return nil
}

// CloseCompiledSkills closes all registered compiled skills.
func (a *Agent) CloseCompiledSkills() error {
	a.mu.RLock()
	skills := a.compiledSkills
	a.mu.RUnlock()

	var lastErr error
	for _, skill := range skills {
		if err := skill.Close(); err != nil {
			lastErr = fmt.Errorf("close skill %q: %w", skill.Name(), err)
			a.logger.Error("failed to close compiled skill", "name", skill.Name(), "error", err)
		}
	}

	return lastErr
}

// SetStorage sets the storage backend for the agent.
// This also injects storage into any storage-aware compiled skills.
func (a *Agent) SetStorage(s kvs.Store) {
	a.mu.Lock()
	a.storage = s
	skills := a.compiledSkills
	a.mu.Unlock()

	// Inject storage and agent into existing compiled skills
	for _, skill := range skills {
		if sa, ok := skill.(compiled.StorageAware); ok {
			sa.SetStorage(s)
		}
		if aa, ok := skill.(compiled.AgentAware); ok {
			aa.SetAgent(a)
		}
	}
}

// SetSecretEnv sets the injected secret environment for the agent and pushes it
// into any already-registered secrets-aware compiled skills. Skills registered
// afterward pick it up in RegisterCompiledSkill, so injection is order-
// independent (as with SetStorage). Secrets are keyed by env-var name.
func (a *Agent) SetSecretEnv(env map[string]string) {
	a.mu.Lock()
	a.secretEnv = env
	skills := a.compiledSkills
	a.mu.Unlock()

	if len(env) == 0 {
		return
	}
	for _, skill := range skills {
		if sa, ok := skill.(compiled.SecretsAware); ok {
			sa.SetSecrets(env)
		}
	}
}

// GetCompiledSkills returns all registered compiled skills.
func (a *Agent) GetCompiledSkills() []compiled.Skill {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]compiled.Skill, len(a.compiledSkills))
	copy(result, a.compiledSkills)
	return result
}
