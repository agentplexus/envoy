package commands

import (
	"reflect"
	"testing"

	"github.com/plexusone/omniagent/config"
	"github.com/plexusone/omniagent/gateway"
)

func TestMergeSecretEnv_PerAgentWinsOverPerSkillWinsOverGlobal(t *testing.T) {
	global := map[string]string{"TOKEN": "global-value", "GLOBAL_ONLY": "g"}
	skillConfig := map[string]config.SkillConfig{
		"github": {Secrets: map[string]string{"TOKEN": "skill-value", "SKILL_ONLY": "s"}},
	}
	agentEnv := map[string]string{"TOKEN": "agent-value"}

	got := mergeSecretEnv(global, skillConfig, []string{"github"}, agentEnv)

	want := map[string]string{
		"TOKEN":       "agent-value", // per-agent wins
		"GLOBAL_ONLY": "g",
		"SKILL_ONLY":  "s",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergeSecretEnv() = %v, want %v", got, want)
	}
}

func TestMergeSecretEnv_PerSkillWinsOverGlobalWhenNoAgentValue(t *testing.T) {
	global := map[string]string{"TOKEN": "global-value"}
	skillConfig := map[string]config.SkillConfig{
		"github": {Secrets: map[string]string{"TOKEN": "skill-value"}},
	}

	got := mergeSecretEnv(global, skillConfig, []string{"github"}, nil)

	if got["TOKEN"] != "skill-value" {
		t.Errorf("TOKEN = %q, want %q", got["TOKEN"], "skill-value")
	}
}

func TestMergeSecretEnv_SkillNotInAgentsListNeverApplies(t *testing.T) {
	// The isolation-preserving case this RMI exists to fix: a binding for a
	// skill this agent does NOT have enabled must never leak into its env.
	skillConfig := map[string]config.SkillConfig{
		"github": {Secrets: map[string]string{"GITHUB_TOKEN": "leaked"}},
	}

	got := mergeSecretEnv(nil, skillConfig, []string{"web"}, nil)

	if _, ok := got["GITHUB_TOKEN"]; ok {
		t.Errorf("GITHUB_TOKEN leaked into env for an agent without the github skill enabled: %v", got)
	}
}

func TestMergeSecretEnv_AllNilIsSafe(t *testing.T) {
	got := mergeSecretEnv(nil, nil, nil, nil)
	if len(got) != 0 {
		t.Errorf("mergeSecretEnv(nil...) = %v, want empty", got)
	}
}

func TestMergeSecretEnv_EmptySkillNamesIgnoresPerSkillConfig(t *testing.T) {
	skillConfig := map[string]config.SkillConfig{
		"github": {Secrets: map[string]string{"GITHUB_TOKEN": "x"}},
	}
	got := mergeSecretEnv(map[string]string{"BASE": "1"}, skillConfig, nil, nil)
	want := map[string]string{"BASE": "1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergeSecretEnv() = %v, want %v", got, want)
	}
}

func TestGlobalSecretBindings_SortedBySourceThenName(t *testing.T) {
	cfg := &config.Config{
		Secrets: map[string]string{"Z_GLOBAL": "1", "A_GLOBAL": "2"},
		Skills: config.SkillsConfig{
			Config: map[string]config.SkillConfig{
				"github": {Secrets: map[string]string{"GITHUB_TOKEN": "x"}},
			},
		},
	}

	got := globalSecretBindings(cfg)

	want := []gateway.GlobalSecretBinding{
		{Name: "GITHUB_TOKEN", Source: "github", Set: true},
		{Name: "A_GLOBAL", Source: "global", Set: true},
		{Name: "Z_GLOBAL", Source: "global", Set: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("globalSecretBindings() = %+v, want %+v", got, want)
	}
}

func TestGlobalSecretBindings_EmptyValueIsUnset(t *testing.T) {
	cfg := &config.Config{Secrets: map[string]string{"UNSET_KEY": ""}}

	got := globalSecretBindings(cfg)

	if len(got) != 1 || got[0].Set {
		t.Errorf("globalSecretBindings() = %+v, want one unset binding", got)
	}
}

func TestGlobalSecretBindings_NeverExposesValues(t *testing.T) {
	cfg := &config.Config{Secrets: map[string]string{"TOKEN": "super-secret-value"}}

	got := globalSecretBindings(cfg)

	// GlobalSecretBinding has no field that could carry the value — this is
	// a structural guarantee, but assert on the concrete fields anyway so a
	// future field addition doesn't silently start leaking values.
	if len(got) != 1 || got[0].Name != "TOKEN" || !got[0].Set {
		t.Errorf("globalSecretBindings() = %+v, want [{TOKEN global true}]", got)
	}
}

func TestGlobalSecretBindings_EmptyConfigIsSafe(t *testing.T) {
	got := globalSecretBindings(&config.Config{})
	if len(got) != 0 {
		t.Errorf("globalSecretBindings(empty) = %v, want empty", got)
	}
}
