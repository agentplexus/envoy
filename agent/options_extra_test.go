package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"testing/fstest"
	"time"

	"github.com/plexusone/omnimemory/core"
	_ "github.com/plexusone/omnimemory/provider/kvs" // registers the KVS memory provider
	"github.com/plexusone/omnistorage-core/kvs/backend/memory"

	"github.com/plexusone/omniagent/agent/profiles"
	"github.com/plexusone/omniagent/agent/roles"
	"github.com/plexusone/omniagent/hooks"
	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omniagent/skills"
)

func TestWithMemory(t *testing.T) {
	client, err := core.NewClient(core.ClientConfig{
		Providers: []core.ProviderConfig{{
			Name:    core.ProviderNameKVS,
			Options: map[string]any{"store": memory.New()},
		}},
	})
	if err != nil {
		t.Fatalf("core.NewClient: %v", err)
	}
	defer client.Close()

	a := &Agent{}
	if err := WithMemory(client)(a); err != nil {
		t.Fatalf("WithMemory: %v", err)
	}
	if a.memory != client {
		t.Error("WithMemory did not set the memory client")
	}
}

func TestWithMemoryConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := &Agent{}
		err := WithMemoryConfig(core.ClientConfig{
			Providers: []core.ProviderConfig{{
				Name:    core.ProviderNameKVS,
				Options: map[string]any{"store": memory.New()},
			}},
		})(a)
		if err != nil {
			t.Fatalf("WithMemoryConfig: %v", err)
		}
		if a.memory == nil {
			t.Error("WithMemoryConfig did not create a memory client")
		}
		defer a.memory.Close()
	})

	t.Run("propagates client creation error", func(t *testing.T) {
		a := &Agent{}
		err := WithMemoryConfig(core.ClientConfig{})(a) // no providers configured
		if err == nil {
			t.Error("WithMemoryConfig should error with no providers configured")
		}
	})
}

func TestWithCronScheduler(t *testing.T) {
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
	if err := WithCronScheduler()(a); err != nil {
		t.Fatalf("WithCronScheduler: %v", err)
	}
	found := false
	for _, sk := range a.GetCompiledSkills() {
		if sk.Name() == "cron" {
			found = true
		}
	}
	if !found {
		t.Error("WithCronScheduler did not register the cron skill")
	}
}

func TestWithHookAndWithNamedHook(t *testing.T) {
	registry := hooks.NewRegistry()
	a := &Agent{hooks: registry}

	var gotUnnamed, gotNamed bool
	if err := WithHook(hooks.EventMessageReceived, func(context.Context, hooks.Event) error {
		gotUnnamed = true
		return nil
	})(a); err != nil {
		t.Fatalf("WithHook: %v", err)
	}
	if err := WithNamedHook(hooks.EventMessageReceived, "audit", func(context.Context, hooks.Event) error {
		gotNamed = true
		return nil
	})(a); err != nil {
		t.Fatalf("WithNamedHook: %v", err)
	}

	dispatcher := hooks.NewDispatcher(registry)
	if err := dispatcher.Emit(context.Background(), hooks.EventMessageReceived, hooks.MessageEvent{}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if !gotUnnamed || !gotNamed {
		t.Errorf("handlers not invoked: unnamed=%v named=%v", gotUnnamed, gotNamed)
	}
}

func TestWithSessionRollover(t *testing.T) {
	t.Run("valid policy sets the rollover policy", func(t *testing.T) {
		a := &Agent{}
		err := WithSessionRollover(sessions.RolloverPolicy{IdleTimeout: time.Hour})(a)
		if err != nil {
			t.Fatalf("WithSessionRollover: %v", err)
		}
		if a.rolloverPolicy == nil || a.rolloverPolicy.IdleTimeout != time.Hour {
			t.Errorf("rolloverPolicy = %+v", a.rolloverPolicy)
		}
	})

	t.Run("daily-only policy is valid", func(t *testing.T) {
		a := &Agent{}
		if err := WithSessionRollover(sessions.RolloverPolicy{Daily: true})(a); err != nil {
			t.Fatalf("WithSessionRollover: %v", err)
		}
	})

	t.Run("empty policy errors", func(t *testing.T) {
		a := &Agent{}
		if err := WithSessionRollover(sessions.RolloverPolicy{})(a); err == nil {
			t.Error("WithSessionRollover should error when neither IdleTimeout nor Daily is set")
		}
	})
}

func TestWithToolsAllowHook(t *testing.T) {
	t.Run("nil hook errors", func(t *testing.T) {
		a := &Agent{}
		if err := WithToolsAllowHook(nil)(a); err == nil {
			t.Error("WithToolsAllowHook(nil) should error")
		}
	})

	t.Run("valid hook is appended", func(t *testing.T) {
		a := &Agent{}
		fn := func(context.Context, hooks.PromptTurn) []string { return nil }
		if err := WithToolsAllowHook(fn)(a); err != nil {
			t.Fatalf("WithToolsAllowHook: %v", err)
		}
		if len(a.toolsAllowHooks) != 1 {
			t.Errorf("toolsAllowHooks = %d, want 1", len(a.toolsAllowHooks))
		}
	})
}

func TestWithCompiledHook(t *testing.T) {
	registry := hooks.NewRegistry()
	a := &Agent{hooks: registry}

	h := &fakeCompiledHook{name: "audit", events: []hooks.EventType{hooks.EventMessageReceived}}
	if err := WithCompiledHook(h)(a); err != nil {
		t.Fatalf("WithCompiledHook: %v", err)
	}

	dispatcher := hooks.NewDispatcher(registry)
	if err := dispatcher.Emit(context.Background(), hooks.EventMessageReceived, hooks.MessageEvent{}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if !h.handled {
		t.Error("compiled hook was not invoked")
	}
}

// fakeCompiledHook is a minimal hooks.Hook implementation for
// WithCompiledHook / RegisterHook tests.
type fakeCompiledHook struct {
	name    string
	events  []hooks.EventType
	handled bool
}

func (h *fakeCompiledHook) Name() string              { return h.name }
func (h *fakeCompiledHook) Events() []hooks.EventType { return h.events }
func (h *fakeCompiledHook) Handle(context.Context, hooks.Event) error {
	h.handled = true
	return nil
}
func (h *fakeCompiledHook) Init(context.Context) error { return nil }
func (h *fakeCompiledHook) Close() error               { return nil }

func TestWithWebhookHook(t *testing.T) {
	registry := hooks.NewRegistry()
	a := &Agent{hooks: registry}

	wh := &hooks.WebhookHook{
		HookName:   "notify",
		HookEvents: []hooks.EventType{hooks.EventMessageSent},
		URL:        "http://127.0.0.1:0/unreachable",
		Method:     "POST",
	}
	if err := WithWebhookHook(wh)(a); err != nil {
		t.Fatalf("WithWebhookHook: %v", err)
	}
	// Registration itself must not error even though the endpoint is
	// unreachable — delivery failures happen async at emit time.
}

func TestWithSkillPack(t *testing.T) {
	a := &Agent{}
	pack := fstest.MapFS{"SKILL.md": &fstest.MapFile{Data: []byte("---\nname: x\n---\n")}}
	if err := WithSkillPack(pack)(a); err != nil {
		t.Fatalf("WithSkillPack: %v", err)
	}
	if len(a.skillPacks) != 1 {
		t.Errorf("skillPacks = %d, want 1", len(a.skillPacks))
	}
}

func TestWithSkillManager(t *testing.T) {
	a := &Agent{}
	mgr := skills.NewManager(skills.ManagerConfig{})
	if err := WithSkillManager(mgr)(a); err != nil {
		t.Fatalf("WithSkillManager: %v", err)
	}
	if a.skillManager != mgr {
		t.Error("WithSkillManager did not set the manager")
	}
}

func TestWithBootstrapProfile(t *testing.T) {
	a := &Agent{}
	profile := &profiles.BootstrapProfile{Name: "p"}
	if err := WithBootstrapProfile(profile)(a); err != nil {
		t.Fatalf("WithBootstrapProfile: %v", err)
	}
	if a.profile != profile {
		t.Error("WithBootstrapProfile did not set the profile")
	}
}

func TestWithProfileRegistry(t *testing.T) {
	a := &Agent{}
	reg := profiles.NewProfileRegistry()
	if err := WithProfileRegistry(reg)(a); err != nil {
		t.Fatalf("WithProfileRegistry: %v", err)
	}
	if a.profileRegistry != reg {
		t.Error("WithProfileRegistry did not set the registry")
	}
}

func TestWithLeanModeAndWithLeanLevel(t *testing.T) {
	a := &Agent{}
	mode := profiles.NewLeanMode(profiles.LeanLevelLight)
	if err := WithLeanMode(mode)(a); err != nil {
		t.Fatalf("WithLeanMode: %v", err)
	}
	if a.leanMode != mode {
		t.Error("WithLeanMode did not set lean mode")
	}

	a2 := &Agent{}
	if err := WithLeanLevel(profiles.LeanLevelAggressive)(a2); err != nil {
		t.Fatalf("WithLeanLevel: %v", err)
	}
	if a2.leanMode == nil || a2.leanMode.Level != profiles.LeanLevelAggressive {
		t.Errorf("leanMode = %+v, want LeanLevelAggressive", a2.leanMode)
	}
}

func TestWithProgressReporterAndWithProgressMode(t *testing.T) {
	a := &Agent{}
	reporter := profiles.NewProgressReporter(profiles.ProgressModeVerbose, io.Discard)
	if err := WithProgressReporter(reporter)(a); err != nil {
		t.Fatalf("WithProgressReporter: %v", err)
	}
	if a.progressReporter != reporter {
		t.Error("WithProgressReporter did not set the reporter")
	}

	a2 := &Agent{}
	if err := WithProgressMode(profiles.ProgressModeMinimal, io.Discard)(a2); err != nil {
		t.Fatalf("WithProgressMode: %v", err)
	}
	if a2.progressReporter == nil || a2.progressReporter.Mode() != profiles.ProgressModeMinimal {
		t.Errorf("progressReporter = %+v, want mode Minimal", a2.progressReporter)
	}
}

func TestWithRole(t *testing.T) {
	t.Run("success wires the role manager and registers skills", func(t *testing.T) {
		a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
		sk := &mcpLikeSkill{name: "required-skill"}
		r := &fakeRole{name: "r", requiredSkills: []string{"required-skill"}}

		if err := WithRole(r, sk)(a); err != nil {
			t.Fatalf("WithRole: %v", err)
		}
		if a.roleManager == nil {
			t.Fatal("WithRole did not set a role manager")
		}
		if len(a.GetCompiledSkills()) != 1 {
			t.Errorf("compiled skills = %d, want 1 (role's skill registered)", len(a.GetCompiledSkills()))
		}
	})

	t.Run("missing required skill errors", func(t *testing.T) {
		a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
		r := &fakeRole{name: "r", requiredSkills: []string{"never-provided"}}
		if err := WithRole(r)(a); err == nil {
			t.Error("WithRole should error when a required skill is missing")
		}
	})
}

func TestWithRoleManager(t *testing.T) {
	a := &Agent{}
	mgr, err := roles.NewManager(&fakeRole{name: "r"})
	if err != nil {
		t.Fatalf("roles.NewManager: %v", err)
	}
	if err := WithRoleManager(mgr)(a); err != nil {
		t.Fatalf("WithRoleManager: %v", err)
	}
	if a.roleManager != mgr {
		t.Error("WithRoleManager did not set the manager")
	}
}

func TestWithStorage(t *testing.T) {
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
	backend := newMemoryBackend(t)
	if err := WithStorage(backend)(a); err != nil {
		t.Fatalf("WithStorage: %v", err)
	}
	if a.storage != backend {
		t.Error("WithStorage did not set the storage backend")
	}
}

func TestWithCompiledSkill(t *testing.T) {
	a := &Agent{tools: NewToolRegistry(), logger: slog.Default()}
	sk := &mcpLikeSkill{name: "s"}
	if err := WithCompiledSkill(sk)(a); err != nil {
		t.Fatalf("WithCompiledSkill: %v", err)
	}
	if len(a.GetCompiledSkills()) != 1 {
		t.Errorf("compiled skills = %d, want 1", len(a.GetCompiledSkills()))
	}
}

// TestInitSkillManager_CustomManager exercises initSkillManager's
// custom-manager branch (New() skips its own manager construction when one
// was supplied via WithSkillManager) including the secret-gating filter.
func TestInitSkillManager_CustomManager(t *testing.T) {
	a := &Agent{logger: slog.Default()}
	// An empty ManagerConfig.Dirs falls back to skills.DefaultSearchPaths(),
	// which includes ~/.omniagent/skills — pointing at an empty temp dir
	// keeps this test from picking up a developer's real installed skills.
	mgr := skills.NewManager(skills.ManagerConfig{Dirs: []string{t.TempDir()}})
	if err := mgr.Load(); err != nil {
		t.Fatalf("mgr.Load: %v", err)
	}
	a.skillManager = mgr

	if err := a.initSkillManager(); err != nil {
		t.Fatalf("initSkillManager: %v", err)
	}
	// An empty manager has nothing available; the point is the branch ran
	// without touching a.skillPacks/a.skillDirs at all.
	if len(a.GetSkills()) != 0 {
		t.Errorf("GetSkills() = %v, want empty for an empty manager", a.GetSkills())
	}
}

// TestInitSkillManager_NoConfigIsNoop covers the early-return branch when
// neither a manager nor packs/dirs/includes/excludes were configured.
func TestInitSkillManager_NoConfigIsNoop(t *testing.T) {
	a := &Agent{logger: slog.Default()}
	if err := a.initSkillManager(); err != nil {
		t.Fatalf("initSkillManager: %v", err)
	}
	if a.skillManager != nil {
		t.Error("initSkillManager should not create a manager with no configuration")
	}
}
