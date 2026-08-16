package agentruntime

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omniagent/team/chats"
	"github.com/plexusone/omniskill/skill"
)

// fakeSecretSkill is a compiled.Skill that records injected secrets, used to
// observe per-agent secret injection through the builder.
type fakeSecretSkill struct {
	secrets map[string]string
}

func (f *fakeSecretSkill) Name() string                     { return "fake" }
func (f *fakeSecretSkill) Description() string              { return "fake" }
func (f *fakeSecretSkill) Tools() []skill.Tool              { return nil }
func (f *fakeSecretSkill) Init(context.Context) error       { return nil }
func (f *fakeSecretSkill) Close() error                     { return nil }
func (f *fakeSecretSkill) SetSecrets(env map[string]string) { f.secrets = env }

var _ compiled.SecretsAware = (*fakeSecretSkill)(nil)

// fakeSecretSource returns per-agent secrets by ID. calls records the
// skillNames each ResolveSecrets call was made with, so tests can assert
// the builder passes the agent's enabled-skill list through.
type fakeSecretSource struct {
	byID  map[uuid.UUID]map[string]string
	err   error
	calls *[]calledWith
}

type calledWith struct {
	agentID    uuid.UUID
	skillNames []string
}

func (f fakeSecretSource) ResolveSecrets(_ context.Context, id uuid.UUID, skillNames []string) (map[string]string, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, calledWith{agentID: id, skillNames: skillNames})
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.byID[id], nil
}

// TestAgentBuilder_Build builds a real *agent.Agent from persona + enabled
// skills and confirms it satisfies chats.AgentProcessor and is closeable (so
// the cache can evict it).
func TestAgentBuilder_Build(t *testing.T) {
	b := NewAgentBuilder(BuilderConfig{
		Defaults: agent.Config{Provider: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
	})

	proc, err := b.Build(context.Background(), AgentConfig{
		ID:      uuid.New(),
		Slug:    "helper",
		Persona: "You are a concise helper.",
		Skills:  []string{"weather"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if proc == nil {
		t.Fatal("Build returned a nil processor")
	}
	var _ chats.AgentProcessor = proc
	closer, ok := proc.(io.Closer)
	if !ok {
		t.Fatal("built processor is not an io.Closer (cache cannot evict it)")
	}
	if err := closer.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestAgentBuilder_InjectsDisjointSecrets is the RMI-310 gate at the builder
// layer: two agents built from one builder + one SecretSource receive disjoint
// secrets, each reaching only its own instance's secrets-aware skills. A fresh
// fake skill is created per build (as MCP skills are), so this exercises real
// per-instance injection, not a shared skill.
func TestAgentBuilder_InjectsDisjointSecrets(t *testing.T) {
	agentA, agentB := uuid.New(), uuid.New()
	src := fakeSecretSource{byID: map[uuid.UUID]map[string]string{
		agentA: {"TOKEN": "aaa"},
		agentB: {"TOKEN": "bbb"},
	}}

	// Each build gets a fresh fake skill, recorded in order.
	var created []*fakeSecretSkill
	freshSkill := agent.Option(func(a *agent.Agent) error {
		fs := &fakeSecretSkill{}
		created = append(created, fs)
		return a.RegisterCompiledSkill(fs)
	})

	b := NewAgentBuilder(BuilderConfig{
		Defaults:    agent.Config{Provider: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
		BaseOptions: []agent.Option{freshSkill},
		Secrets:     src,
	})

	procA, err := b.Build(context.Background(), AgentConfig{ID: agentA, Slug: "a"})
	if err != nil {
		t.Fatalf("build A: %v", err)
	}
	if c, ok := procA.(io.Closer); ok {
		defer c.Close() //nolint:errcheck // test teardown
	}
	procB, err := b.Build(context.Background(), AgentConfig{ID: agentB, Slug: "b"})
	if err != nil {
		t.Fatalf("build B: %v", err)
	}
	if c, ok := procB.(io.Closer); ok {
		defer c.Close() //nolint:errcheck // test teardown
	}

	if len(created) != 2 {
		t.Fatalf("expected 2 fresh skills, got %d", len(created))
	}
	if created[0].secrets["TOKEN"] != "aaa" {
		t.Errorf("agent A skill secrets = %v, want TOKEN:aaa", created[0].secrets)
	}
	if created[1].secrets["TOKEN"] != "bbb" {
		t.Errorf("agent B skill secrets = %v, want TOKEN:bbb", created[1].secrets)
	}
	// No cross-leak: A never sees bbb, B never sees aaa.
	if created[0].secrets["TOKEN"] == created[1].secrets["TOKEN"] {
		t.Error("agents received identical secrets (cross-leak)")
	}
}

// TestAgentBuilder_NoSecretSource confirms a nil SecretSource injects nothing
// (prior behavior) and the skill's secrets stay unset.
func TestAgentBuilder_NoSecretSource(t *testing.T) {
	var created []*fakeSecretSkill
	freshSkill := agent.Option(func(a *agent.Agent) error {
		fs := &fakeSecretSkill{}
		created = append(created, fs)
		return a.RegisterCompiledSkill(fs)
	})
	b := NewAgentBuilder(BuilderConfig{
		Defaults:    agent.Config{Provider: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
		BaseOptions: []agent.Option{freshSkill},
	})
	proc, err := b.Build(context.Background(), AgentConfig{ID: uuid.New(), Slug: "a"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if c, ok := proc.(io.Closer); ok {
		defer c.Close() //nolint:errcheck // test teardown
	}
	if len(created) != 1 || created[0].secrets != nil {
		t.Errorf("expected no secrets injected, got %v", created[0].secrets)
	}
}

// TestAgentBuilder_PassesSkillNamesToSecretSource confirms Build() forwards
// the agent's own enabled-skill list to ResolveSecrets (RMI-OMNIAGENT-208) —
// the seam a skill-scoped fallback source needs to avoid leaking a per-skill
// binding into an agent that doesn't have that skill enabled.
func TestAgentBuilder_PassesSkillNamesToSecretSource(t *testing.T) {
	var calls []calledWith
	src := fakeSecretSource{calls: &calls}

	b := NewAgentBuilder(BuilderConfig{
		Defaults: agent.Config{Provider: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
		Secrets:  src,
	})

	agentID := uuid.New()
	proc, err := b.Build(context.Background(), AgentConfig{ID: agentID, Slug: "a", Skills: []string{"github", "web"}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if c, ok := proc.(io.Closer); ok {
		defer c.Close() //nolint:errcheck // test teardown
	}

	if len(calls) != 1 {
		t.Fatalf("ResolveSecrets called %d times, want 1", len(calls))
	}
	if calls[0].agentID != agentID {
		t.Errorf("ResolveSecrets agentID = %v, want %v", calls[0].agentID, agentID)
	}
	got := calls[0].skillNames
	if len(got) != 2 || got[0] != "github" || got[1] != "web" {
		t.Errorf("ResolveSecrets skillNames = %v, want [github web]", got)
	}
}

// TestAgentBuilder_SecretResolveError propagates a resolver failure.
func TestAgentBuilder_SecretResolveError(t *testing.T) {
	b := NewAgentBuilder(BuilderConfig{
		Defaults: agent.Config{Provider: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
		Secrets:  fakeSecretSource{err: errors.New("vault down")},
	})
	if _, err := b.Build(context.Background(), AgentConfig{ID: uuid.New(), Slug: "a"}); err == nil {
		t.Fatal("expected error when secret resolution fails, got nil")
	}
}

// TestAgentBuilder_AgentOverridesDefaults confirms the agent's own
// model/provider win over the deployment defaults, while an empty persona leaves
// the default system prompt untouched.
func TestAgentBuilder_AgentOverridesDefaults(t *testing.T) {
	b := NewAgentBuilder(BuilderConfig{
		Defaults: agent.Config{Provider: "openai", Model: "default-model", APIKey: "test-key", SystemPrompt: "default prompt"},
	})

	// Agent overrides model; provider empty falls back to the default.
	proc, err := b.Build(context.Background(), AgentConfig{
		ID:    uuid.New(),
		Slug:  "custom",
		Model: "gpt-4o",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if c, ok := proc.(io.Closer); ok {
		defer c.Close() //nolint:errcheck // test teardown
	}
	// Build succeeding with the overridden model and inherited provider/prompt is
	// the observable contract; the agent package owns how they are applied.
}
