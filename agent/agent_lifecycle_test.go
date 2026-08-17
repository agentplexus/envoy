package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/plexusone/omnimemory/core"
	"github.com/plexusone/omniskill/role"
	"github.com/plexusone/omniskill/skill"

	"github.com/plexusone/omniagent/agent/profiles"
	"github.com/plexusone/omniagent/agent/roles"
	"github.com/plexusone/omniagent/hooks"
	"github.com/plexusone/omniagent/skills"
)

func TestAgent_HookRegistryAndDispatcher(t *testing.T) {
	registry := hooks.NewRegistry()
	dispatcher := hooks.NewDispatcher(registry)
	a := &Agent{hooks: registry, dispatcher: dispatcher}

	if a.HookRegistry() != registry {
		t.Error("HookRegistry() did not return the configured registry")
	}
	if a.Dispatcher() != dispatcher {
		t.Error("Dispatcher() did not return the configured dispatcher")
	}
}

func TestAgent_InitHooks(t *testing.T) {
	t.Run("nil registry is a no-op", func(t *testing.T) {
		a := &Agent{}
		if err := a.InitHooks(context.Background()); err != nil {
			t.Errorf("InitHooks() with nil registry = %v, want nil", err)
		}
	})

	t.Run("delegates to the registry", func(t *testing.T) {
		registry := hooks.NewRegistry()
		a := &Agent{hooks: registry}
		if err := a.InitHooks(context.Background()); err != nil {
			t.Errorf("InitHooks() = %v, want nil", err)
		}
	})
}

func TestAgent_SkillManagerGetter(t *testing.T) {
	a := &Agent{}
	if a.SkillManager() != nil {
		t.Error("SkillManager() should be nil by default")
	}
	mgr := skills.NewManager(skills.ManagerConfig{})
	a.skillManager = mgr
	if a.SkillManager() != mgr {
		t.Error("SkillManager() did not return the configured manager")
	}
}

func TestBuildSystemPromptWithMemories_InjectsMemories(t *testing.T) {
	a := &Agent{config: Config{SystemPrompt: "base prompt"}}
	memories := []*core.Memory{
		{Type: core.MemoryTypeObservation, Content: "remembered fact one"},
		{Type: core.MemoryTypeObservation, Content: "remembered fact two"},
	}

	got := a.buildSystemPromptWithMemories(memories)

	for _, want := range []string{
		"base prompt",
		"## Relevant Memories",
		"remembered fact one",
		"remembered fact two",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q, got %q", want, got)
		}
	}
}

func TestInjectMemoriesIntoPrompt_EmptyIsNoop(t *testing.T) {
	a := &Agent{}
	if got := a.injectMemoriesIntoPrompt("unchanged", nil); got != "unchanged" {
		t.Errorf("injectMemoriesIntoPrompt(nil) = %q, want unchanged", got)
	}
}

func TestAgent_ProfileGetSet(t *testing.T) {
	a := &Agent{}
	if a.Profile() != nil {
		t.Error("Profile() should be nil by default")
	}
	profile := &profiles.BootstrapProfile{Name: "test-profile"}
	a.SetProfile(profile)
	if a.Profile() != profile {
		t.Error("SetProfile did not take effect")
	}
}

func TestAgent_ProfileRegistryGetter(t *testing.T) {
	a := &Agent{}
	if a.ProfileRegistry() != nil {
		t.Error("ProfileRegistry() should be nil by default")
	}
	reg := profiles.NewProfileRegistry()
	a.profileRegistry = reg
	if a.ProfileRegistry() != reg {
		t.Error("ProfileRegistry() did not return the configured registry")
	}
}

func TestAgent_ActivateProfile(t *testing.T) {
	t.Run("no registry configured errors", func(t *testing.T) {
		a := &Agent{}
		if err := a.ActivateProfile(context.Background(), "anything"); err == nil {
			t.Error("ActivateProfile should error without a profile registry")
		}
	})

	t.Run("unknown profile errors", func(t *testing.T) {
		reg := profiles.NewProfileRegistry()
		a := &Agent{profileRegistry: reg, logger: slog.Default()}
		if err := a.ActivateProfile(context.Background(), "missing"); err == nil {
			t.Error("ActivateProfile should error for an unregistered profile")
		}
	})

	t.Run("activates a known profile", func(t *testing.T) {
		reg := profiles.NewProfileRegistry()
		if err := reg.Register(&profiles.BootstrapProfile{Name: "code-assistant"}); err != nil {
			t.Fatalf("Register: %v", err)
		}
		a := &Agent{profileRegistry: reg, logger: slog.Default()}

		if err := a.ActivateProfile(context.Background(), "code-assistant"); err != nil {
			t.Fatalf("ActivateProfile: %v", err)
		}
		if a.Profile() == nil || a.Profile().Name != "code-assistant" {
			t.Errorf("Profile() = %+v, want code-assistant activated", a.Profile())
		}
	})

	t.Run("lean mode clones and applies to the activated profile", func(t *testing.T) {
		reg := profiles.NewProfileRegistry()
		original := &profiles.BootstrapProfile{Name: "lean-target", MaxContextMessages: 100}
		if err := reg.Register(original); err != nil {
			t.Fatalf("Register: %v", err)
		}
		a := &Agent{
			profileRegistry: reg,
			logger:          slog.Default(),
			leanMode:        profiles.NewLeanMode(profiles.LeanLevelAggressive),
		}
		if err := a.ActivateProfile(context.Background(), "lean-target"); err != nil {
			t.Fatalf("ActivateProfile: %v", err)
		}
		if a.Profile() == original {
			t.Error("lean mode must clone the profile, not activate the registry's shared instance")
		}
		if a.Profile().Name != "lean-target" {
			t.Errorf("Profile().Name = %q, want lean-target", a.Profile().Name)
		}
	})
}

func TestAgent_LeanModeGetSet(t *testing.T) {
	a := &Agent{}
	if a.LeanMode() != nil {
		t.Error("LeanMode() should be nil by default")
	}
	mode := profiles.NewLeanMode(profiles.LeanLevelModerate)
	a.SetLeanMode(mode)
	if a.LeanMode() != mode {
		t.Error("SetLeanMode did not take effect")
	}
}

func TestAgent_ProgressReporterGetSet(t *testing.T) {
	a := &Agent{}
	if a.ProgressReporter() != nil {
		t.Error("ProgressReporter() should be nil by default")
	}
	reporter := profiles.NewProgressReporter(profiles.ProgressModeNormal, io.Discard)
	a.SetProgressReporter(reporter)
	if a.ProgressReporter() != reporter {
		t.Error("SetProgressReporter did not take effect")
	}
}

func TestAgent_RoleManagerGetSet(t *testing.T) {
	a := &Agent{}
	if a.RoleManager() != nil {
		t.Error("RoleManager() should be nil by default")
	}
	mgr, err := roles.NewManager(&fakeRole{name: "r"})
	if err != nil {
		t.Fatalf("roles.NewManager: %v", err)
	}
	a.SetRoleManager(mgr)
	if a.RoleManager() != mgr {
		t.Error("SetRoleManager did not take effect")
	}
}

func TestAgent_InitRole(t *testing.T) {
	t.Run("nil manager is a no-op", func(t *testing.T) {
		a := &Agent{}
		if err := a.InitRole(context.Background()); err != nil {
			t.Errorf("InitRole() with no role manager = %v, want nil", err)
		}
	})

	t.Run("delegates to the role manager", func(t *testing.T) {
		r := &fakeRole{name: "r"}
		mgr, err := roles.NewManager(r)
		if err != nil {
			t.Fatalf("roles.NewManager: %v", err)
		}
		a := &Agent{roleManager: mgr}
		if err := a.InitRole(context.Background()); err != nil {
			t.Fatalf("InitRole: %v", err)
		}
		if !r.initCalled {
			t.Error("InitRole did not call the underlying role's Init")
		}
	})

	t.Run("propagates the role's init error", func(t *testing.T) {
		boom := errors.New("role init boom")
		r := &fakeRole{name: "r", initErr: boom}
		mgr, err := roles.NewManager(r)
		if err != nil {
			t.Fatalf("roles.NewManager: %v", err)
		}
		a := &Agent{roleManager: mgr}
		if err := a.InitRole(context.Background()); !errors.Is(err, boom) {
			t.Errorf("InitRole() error = %v, want wrapping %v", err, boom)
		}
	})
}

// TestBuildSystemPromptWithMemories_RolePrompt verifies a configured role's
// system prompt is prepended ahead of the agent's base prompt.
func TestBuildSystemPromptWithMemories_RolePrompt(t *testing.T) {
	r := &fakeRole{name: "r", systemPrompt: "role persona"}
	mgr, err := roles.NewManager(r)
	if err != nil {
		t.Fatalf("roles.NewManager: %v", err)
	}
	a := &Agent{roleManager: mgr, config: Config{SystemPrompt: "base prompt"}}

	got := a.buildSystemPromptWithMemories(nil)
	if !strings.Contains(got, "role persona") || !strings.Contains(got, "base prompt") {
		t.Errorf("prompt = %q, want both role persona and base prompt", got)
	}
	if strings.Index(got, "role persona") > strings.Index(got, "base prompt") {
		t.Errorf("role prompt must precede the base prompt, got %q", got)
	}
}

// fakeRole is a minimal role.Role implementation for exercising
// roles.Manager without any real role/skill wiring.
type fakeRole struct {
	name            string
	systemPrompt    string
	systemPromptErr error
	requiredSkills  []string
	initErr         error
	closeErr        error
	initCalled      bool
}

func (r *fakeRole) Name() string         { return r.name }
func (r *fakeRole) Description() string  { return "fake role" }
func (r *fakeRole) Version() string      { return "0.0.0" }
func (r *fakeRole) Spec() *role.RoleSpec { return nil }
func (r *fakeRole) SystemPrompt(context.Context) (string, error) {
	return r.systemPrompt, r.systemPromptErr
}
func (r *fakeRole) RequiredSkills() []string { return r.requiredSkills }
func (r *fakeRole) Init(_ context.Context, _ map[string]skill.Skill) error {
	r.initCalled = true
	return r.initErr
}
func (r *fakeRole) Close() error               { return r.closeErr }
func (r *fakeRole) Workflows() []role.Workflow { return nil }

var _ role.Role = (*fakeRole)(nil)
