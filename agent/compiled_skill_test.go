package agent

import (
	"context"
	"log/slog"
	"testing"

	"github.com/plexusone/omniskill/skill"
)

// secretRequiringSkill is a compiled.Skill that declares required secrets,
// used to test RegisterCompiledSkill's gating (RMI-OMNIAGENT-210).
type secretRequiringSkill struct {
	name     string
	required []string
}

func (s *secretRequiringSkill) Name() string                   { return s.name }
func (s *secretRequiringSkill) Description() string            { return "test skill" }
func (s *secretRequiringSkill) Tools() []skill.Tool            { return []skill.Tool{fakeTool()} }
func (s *secretRequiringSkill) Init(ctx context.Context) error { return nil }
func (s *secretRequiringSkill) Close() error                   { return nil }
func (s *secretRequiringSkill) RequiredSecrets() []string      { return s.required }

// fakeTool returns a minimal skill.Tool for tool-count assertions.
func fakeTool() skill.Tool {
	return skill.NewTool("fake_tool", "fake", nil,
		func(ctx context.Context, params map[string]any) (any, error) { return "ok", nil })
}

func TestRegisterCompiledSkill_SkipsOnUnmetRequiredSecret(t *testing.T) {
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
	sk := &secretRequiringSkill{name: "github", required: []string{"GITHUB_TOKEN"}}

	if err := a.RegisterCompiledSkill(sk); err != nil {
		t.Fatalf("RegisterCompiledSkill: %v", err)
	}

	if len(a.tools.Describe()) != 0 {
		t.Errorf("tools registered = %d, want 0 (skill should be skipped)", len(a.tools.Describe()))
	}
	if len(a.compiledSkills) != 0 {
		t.Errorf("compiledSkills = %d, want 0 (skipped skill must not be tracked for Init)", len(a.compiledSkills))
	}
}

func TestRegisterCompiledSkill_RegistersWhenRequiredSecretPresent(t *testing.T) {
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default(), secretEnv: map[string]string{"GITHUB_TOKEN": "x"}}
	sk := &secretRequiringSkill{name: "github", required: []string{"GITHUB_TOKEN"}}

	if err := a.RegisterCompiledSkill(sk); err != nil {
		t.Fatalf("RegisterCompiledSkill: %v", err)
	}

	if len(a.tools.Describe()) != 1 {
		t.Errorf("tools registered = %d, want 1", len(a.tools.Describe()))
	}
	if len(a.compiledSkills) != 1 {
		t.Errorf("compiledSkills = %d, want 1", len(a.compiledSkills))
	}
}

func TestRegisterCompiledSkill_NoRequirerUnaffected(t *testing.T) {
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
	sk := &mcpLikeSkill{name: "plain", tools: []skill.Tool{fakeTool()}}

	if err := a.RegisterCompiledSkill(sk); err != nil {
		t.Fatalf("RegisterCompiledSkill: %v", err)
	}

	if len(a.tools.Describe()) != 1 {
		t.Errorf("tools registered = %d, want 1 (skill without SecretRequirer must register normally)", len(a.tools.Describe()))
	}
}
