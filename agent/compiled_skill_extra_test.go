package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/plexusone/omnistorage-core/kvs"

	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omniskill/skill"
)

// TestCompiledToolWrapper_Execute exercises the tool-execution path used for
// every compiled-skill tool call: argument parsing, delegating to the
// underlying skill.Tool, and result-type coercion (string, []byte, and
// arbitrary values that must be JSON-marshaled).
func TestCompiledToolWrapper_Execute(t *testing.T) {
	tests := []struct {
		name    string
		tool    skill.Tool
		args    string
		want    string
		wantErr string
	}{
		{
			name: "string result passes through",
			tool: skill.NewTool("t", "d", nil, func(ctx context.Context, params map[string]any) (any, error) {
				return "plain string", nil
			}),
			args: `{}`,
			want: "plain string",
		},
		{
			name: "byte slice result converted to string",
			tool: skill.NewTool("t", "d", nil, func(ctx context.Context, params map[string]any) (any, error) {
				return []byte("byte result"), nil
			}),
			args: `{}`,
			want: "byte result",
		},
		{
			name: "struct result marshaled as JSON",
			tool: skill.NewTool("t", "d", nil, func(ctx context.Context, params map[string]any) (any, error) {
				return map[string]any{"ok": true}, nil
			}),
			args: `{}`,
			want: `{"ok":true}`,
		},
		{
			name: "params forwarded to the tool call",
			tool: skill.NewTool("t", "d", nil, func(ctx context.Context, params map[string]any) (any, error) {
				return params["name"], nil
			}),
			args: `{"name":"world"}`,
			want: "world",
		},
		{
			name: "tool call error propagates",
			tool: skill.NewTool("t", "d", nil, func(ctx context.Context, params map[string]any) (any, error) {
				return nil, errors.New("tool failed")
			}),
			args:    `{}`,
			wantErr: "tool failed",
		},
		{
			name:    "invalid argument JSON errors before calling the tool",
			tool:    skill.NewTool("t", "d", nil, func(ctx context.Context, params map[string]any) (any, error) { return "unreached", nil }),
			args:    `not-json`,
			wantErr: "parse arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &compiledToolWrapper{tool: tt.tool, skillName: "s", sourceType: "skill"}
			got, err := w.Execute(context.Background(), json.RawMessage(tt.args))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Execute() error = nil, want containing %q", tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			if got != tt.want {
				t.Errorf("Execute() = %q, want %q", got, tt.want)
			}
		})
	}
}

// initCloseSkill is a compiled.Skill whose Init/Close can be scripted to
// fail, for testing InitCompiledSkills/CloseCompiledSkills error handling.
type initCloseSkill struct {
	name     string
	initErr  error
	closeErr error
}

func (s *initCloseSkill) Name() string                   { return s.name }
func (s *initCloseSkill) Description() string            { return "d" }
func (s *initCloseSkill) Tools() []skill.Tool            { return nil }
func (s *initCloseSkill) Init(ctx context.Context) error { return s.initErr }
func (s *initCloseSkill) Close() error                   { return s.closeErr }

var _ compiled.Skill = (*initCloseSkill)(nil)

func TestInitCompiledSkills(t *testing.T) {
	t.Run("all succeed", func(t *testing.T) {
		a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
		skA := &initCloseSkill{name: "a"}
		skB := &initCloseSkill{name: "b"}
		if err := a.RegisterCompiledSkill(skA); err != nil {
			t.Fatalf("RegisterCompiledSkill a: %v", err)
		}
		if err := a.RegisterCompiledSkill(skB); err != nil {
			t.Fatalf("RegisterCompiledSkill b: %v", err)
		}
		if err := a.InitCompiledSkills(context.Background()); err != nil {
			t.Fatalf("InitCompiledSkills: %v", err)
		}
	})

	t.Run("stops and returns wrapped error on first failure", func(t *testing.T) {
		a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
		boom := errors.New("init boom")
		sk := &initCloseSkill{name: "failing", initErr: boom}
		if err := a.RegisterCompiledSkill(sk); err != nil {
			t.Fatalf("RegisterCompiledSkill: %v", err)
		}
		err := a.InitCompiledSkills(context.Background())
		if err == nil || !errors.Is(err, boom) {
			t.Fatalf("InitCompiledSkills() error = %v, want wrapping %v", err, boom)
		}
	})
}

func TestCloseCompiledSkills(t *testing.T) {
	t.Run("all succeed", func(t *testing.T) {
		a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
		if err := a.RegisterCompiledSkill(&initCloseSkill{name: "a"}); err != nil {
			t.Fatalf("RegisterCompiledSkill: %v", err)
		}
		if err := a.CloseCompiledSkills(); err != nil {
			t.Fatalf("CloseCompiledSkills: %v", err)
		}
	})

	t.Run("aggregates: continues closing remaining skills and returns the last error", func(t *testing.T) {
		a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
		boom1 := errors.New("close boom 1")
		boom2 := errors.New("close boom 2")
		if err := a.RegisterCompiledSkill(&initCloseSkill{name: "a", closeErr: boom1}); err != nil {
			t.Fatalf("RegisterCompiledSkill a: %v", err)
		}
		if err := a.RegisterCompiledSkill(&initCloseSkill{name: "b", closeErr: boom2}); err != nil {
			t.Fatalf("RegisterCompiledSkill b: %v", err)
		}
		err := a.CloseCompiledSkills()
		if err == nil || !errors.Is(err, boom2) {
			t.Fatalf("CloseCompiledSkills() error = %v, want wrapping the last skill's error (%v)", err, boom2)
		}
	})
}

func TestGetCompiledSkills(t *testing.T) {
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
	if got := a.GetCompiledSkills(); len(got) != 0 {
		t.Fatalf("GetCompiledSkills() = %v, want empty before registration", got)
	}

	sk := &initCloseSkill{name: "a"}
	if err := a.RegisterCompiledSkill(sk); err != nil {
		t.Fatalf("RegisterCompiledSkill: %v", err)
	}

	got := a.GetCompiledSkills()
	if len(got) != 1 || got[0].Name() != "a" {
		t.Fatalf("GetCompiledSkills() = %v, want [a]", got)
	}

	// Returned slice is a copy: mutating it must not affect the agent's
	// internal tracking.
	got[0] = &initCloseSkill{name: "mutated"}
	if a.GetCompiledSkills()[0].Name() != "a" {
		t.Error("GetCompiledSkills() leaked internal slice — mutation affected agent state")
	}
}

// storageAwareSkill records injected storage/agent for SetStorage tests.
type storageAwareSkill struct {
	initCloseSkill
	storage kvs.Store
	agent   any
}

func (s *storageAwareSkill) SetStorage(store kvs.Store) { s.storage = store }
func (s *storageAwareSkill) SetAgent(agent any)         { s.agent = agent }

var (
	_ compiled.StorageAware = (*storageAwareSkill)(nil)
	_ compiled.AgentAware   = (*storageAwareSkill)(nil)
)

// TestSetStorage_InjectsIntoRegisteredSkills verifies SetStorage pushes the
// backend (and the agent itself) into already-registered storage/agent-aware
// compiled skills, matching SetSecretEnv's order-independent injection
// pattern.
func TestSetStorage_InjectsIntoRegisteredSkills(t *testing.T) {
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
	sk := &storageAwareSkill{initCloseSkill: initCloseSkill{name: "storage-user"}}
	if err := a.RegisterCompiledSkill(sk); err != nil {
		t.Fatalf("RegisterCompiledSkill: %v", err)
	}

	backend := newMemoryBackend(t)
	a.SetStorage(backend)

	if sk.storage != backend {
		t.Error("SetStorage did not inject the backend into the storage-aware skill")
	}
	if sk.agent != a {
		t.Error("SetStorage did not inject the agent into the agent-aware skill")
	}
}

func TestSetSecretEnv_EmptyEnvIsNoop(t *testing.T) {
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
	fs := &fakeSecretSkill{name: "s"}
	if err := a.RegisterCompiledSkill(fs); err != nil {
		t.Fatalf("RegisterCompiledSkill: %v", err)
	}
	a.SetSecretEnv(nil)
	if fs.secrets != nil {
		t.Errorf("SetSecretEnv(nil) pushed secrets = %v, want untouched", fs.secrets)
	}
	if a.secretEnv != nil {
		t.Errorf("a.secretEnv = %v, want nil", a.secretEnv)
	}
}
