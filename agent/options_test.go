// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/plexusone/omnillm/provider"

	agentctx "github.com/plexusone/omniagent/context"
	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omniskill/skill"
)

// fakeSecretSkill is a compiled.Skill that records injected secrets, used to
// verify WithSecretEnv/SetSecretEnv injection.
type fakeSecretSkill struct {
	name    string
	secrets map[string]string
}

func (f *fakeSecretSkill) Name() string                     { return f.name }
func (f *fakeSecretSkill) Description() string              { return "fake" }
func (f *fakeSecretSkill) Tools() []skill.Tool              { return nil }
func (f *fakeSecretSkill) Init(context.Context) error       { return nil }
func (f *fakeSecretSkill) Close() error                     { return nil }
func (f *fakeSecretSkill) SetSecrets(env map[string]string) { f.secrets = env }

var _ compiled.SecretsAware = (*fakeSecretSkill)(nil)

func newTestAgent() *Agent {
	return &Agent{tools: NewToolRegistry(), logger: slog.Default()}
}

// TestWithSecretEnv_InjectsAfterRegister confirms secrets set after a skill is
// registered still reach it (SetSecretEnv pushes into existing skills).
func TestWithSecretEnv_InjectsAfterRegister(t *testing.T) {
	a := newTestAgent()
	fs := &fakeSecretSkill{name: "github"}
	if err := a.RegisterCompiledSkill(fs); err != nil {
		t.Fatalf("RegisterCompiledSkill: %v", err)
	}

	env := map[string]string{"GITHUB_TOKEN": "ghp"}
	if err := WithSecretEnv(env)(a); err != nil {
		t.Fatalf("WithSecretEnv: %v", err)
	}
	if fs.secrets["GITHUB_TOKEN"] != "ghp" {
		t.Errorf("secrets = %v, want GITHUB_TOKEN:ghp", fs.secrets)
	}
}

// TestWithSecretEnv_InjectsBeforeRegister confirms secrets set before a skill is
// registered reach it too (RegisterCompiledSkill applies a.secretEnv), so
// injection is order-independent.
func TestWithSecretEnv_InjectsBeforeRegister(t *testing.T) {
	a := newTestAgent()
	if err := WithSecretEnv(map[string]string{"GITHUB_TOKEN": "ghp"})(a); err != nil {
		t.Fatalf("WithSecretEnv: %v", err)
	}
	fs := &fakeSecretSkill{name: "github"}
	if err := a.RegisterCompiledSkill(fs); err != nil {
		t.Fatalf("RegisterCompiledSkill: %v", err)
	}
	if fs.secrets["GITHUB_TOKEN"] != "ghp" {
		t.Errorf("secrets = %v, want GITHUB_TOKEN:ghp", fs.secrets)
	}
}

func TestWithTool(t *testing.T) {
	tool := NewBaseTool("test_tool", "Test", nil, nil)

	a := &Agent{
		tools: NewToolRegistry(),
	}

	opt := WithTool(tool)
	if err := opt(a); err != nil {
		t.Fatalf("WithTool failed: %v", err)
	}

	if _, ok := a.tools.Get("test_tool"); !ok {
		t.Error("tool not registered")
	}
}

func TestWithContextEngine(t *testing.T) {
	engine := agentctx.New(agentctx.Config{
		MaxMessages: 10,
	})

	a := &Agent{}

	opt := WithContextEngine(engine)
	if err := opt(a); err != nil {
		t.Fatalf("WithContextEngine failed: %v", err)
	}

	if a.contextEngine == nil {
		t.Error("context engine not set")
	}
}

func TestWithContextConfig(t *testing.T) {
	a := &Agent{}

	opt := WithContextConfig(agentctx.Config{
		MaxMessages: 25,
	})
	if err := opt(a); err != nil {
		t.Fatalf("WithContextConfig failed: %v", err)
	}

	if a.contextEngine == nil {
		t.Error("context engine not created")
	}
}

func TestWithMaxMessages(t *testing.T) {
	a := &Agent{}

	opt := WithMaxMessages(50)
	if err := opt(a); err != nil {
		t.Fatalf("WithMaxMessages failed: %v", err)
	}

	if a.contextEngine == nil {
		t.Error("context engine not created")
	}
}

func TestWithCompaction(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{textResponse("condensed")}}
	a := processTestAgent(t, fp)

	opt := WithCompaction(3)
	if err := opt(a); err != nil {
		t.Fatalf("WithCompaction failed: %v", err)
	}
	if a.contextEngine == nil {
		t.Fatal("context engine not created")
	}

	messages := summarizeMessagesForTest(14) // well over threshold=3
	result, err := a.contextEngine.Apply(context.Background(), messages)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if fp.callCount() != 1 {
		t.Fatalf("summarizer LLM call count = %d, want 1 (compaction should have triggered)", fp.callCount())
	}
	if len(result) >= len(messages) {
		t.Errorf("Apply() returned %d messages, want fewer than the original %d", len(result), len(messages))
	}
}

func TestWithCompaction_PreservesExistingMaxMessagesWhenCalledAfter(t *testing.T) {
	// WithMaxMessages/WithContextConfig/WithContextEngine replace the
	// context engine wholesale, so WithCompaction must be applied after
	// them to compose rather than being wiped out — verify the reverse
	// composes correctly (WithCompaction called after WithMaxMessages).
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{textResponse("condensed")}}
	a := processTestAgent(t, fp)

	for _, opt := range []Option{WithMaxMessages(5), WithCompaction(10)} {
		if err := opt(a); err != nil {
			t.Fatalf("option failed: %v", err)
		}
	}

	messages := summarizeMessagesForTest(14)
	result, err := a.contextEngine.Apply(context.Background(), messages)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if fp.callCount() != 1 {
		t.Errorf("summarizer LLM call count = %d, want 1", fp.callCount())
	}
	// MaxMessages=5 still applies after compaction.
	if len(result) != 5 {
		t.Errorf("Apply() returned %d messages, want 5 (MaxMessages preserved)", len(result))
	}
}

func TestWithSkillDirs(t *testing.T) {
	a := &Agent{}

	opt := WithSkillDirs("./skills", "/opt/skills")
	if err := opt(a); err != nil {
		t.Fatalf("WithSkillDirs failed: %v", err)
	}

	if len(a.skillDirs) != 2 {
		t.Errorf("expected 2 skill dirs, got %d", len(a.skillDirs))
	}

	if a.skillDirs[0] != "./skills" {
		t.Errorf("expected './skills', got %q", a.skillDirs[0])
	}
}

func TestWithSkillIncludes(t *testing.T) {
	a := &Agent{}

	opt := WithSkillIncludes("github", "weather")
	if err := opt(a); err != nil {
		t.Fatalf("WithSkillIncludes failed: %v", err)
	}

	if len(a.skillIncludes) != 2 {
		t.Errorf("expected 2 includes, got %d", len(a.skillIncludes))
	}
}

func TestWithSkillExcludes(t *testing.T) {
	a := &Agent{}

	opt := WithSkillExcludes("slack", "trello")
	if err := opt(a); err != nil {
		t.Fatalf("WithSkillExcludes failed: %v", err)
	}

	if len(a.skillExcludes) != 2 {
		t.Errorf("expected 2 excludes, got %d", len(a.skillExcludes))
	}
}

func TestMultipleOptions(t *testing.T) {
	a := &Agent{
		tools: NewToolRegistry(),
	}

	tool1 := NewBaseTool("tool1", "Tool 1", nil, func(ctx context.Context, args json.RawMessage) (string, error) {
		return "result1", nil
	})
	tool2 := NewBaseTool("tool2", "Tool 2", nil, func(ctx context.Context, args json.RawMessage) (string, error) {
		return "result2", nil
	})

	opts := []Option{
		WithTool(tool1),
		WithTool(tool2),
		WithMaxMessages(100),
	}

	for _, opt := range opts {
		if err := opt(a); err != nil {
			t.Fatalf("option failed: %v", err)
		}
	}

	// Verify all options applied
	if _, ok := a.tools.Get("tool1"); !ok {
		t.Error("tool1 not registered")
	}
	if _, ok := a.tools.Get("tool2"); !ok {
		t.Error("tool2 not registered")
	}
	if a.contextEngine == nil {
		t.Error("context engine not set")
	}
}
