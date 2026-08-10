package secrets

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omnivault/providers/memory"
)

func TestNewService_Validation(t *testing.T) {
	if _, err := NewService(Config{}); err == nil {
		t.Fatal("NewService with nil vault: want error, got nil")
	}
	if _, err := NewService(Config{Vault: memory.New()}); err != nil {
		t.Fatalf("NewService: %v", err)
	}
}

// TestResolveAgentSecrets confirms an agent's namespace resolves to its env map.
func TestResolveAgentSecrets(t *testing.T) {
	svc, err := NewService(Config{Vault: memory.New()})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()
	agentID := uuid.New()

	if err := svc.SetAgentSecret(ctx, agentID, "GITHUB_TOKEN", "ghp_xxx"); err != nil {
		t.Fatalf("SetAgentSecret: %v", err)
	}
	if err := svc.SetAgentSecret(ctx, agentID, "WEATHER_KEY", "wk_yyy"); err != nil {
		t.Fatalf("SetAgentSecret: %v", err)
	}

	env, err := svc.ResolveAgentSecrets(ctx, agentID)
	if err != nil {
		t.Fatalf("ResolveAgentSecrets: %v", err)
	}
	if len(env) != 2 || env["GITHUB_TOKEN"] != "ghp_xxx" || env["WEATHER_KEY"] != "wk_yyy" {
		t.Errorf("env = %v, want {GITHUB_TOKEN:ghp_xxx, WEATHER_KEY:wk_yyy}", env)
	}
}

// TestListAgentSecretNames confirms the write-only listing returns names only
// (INIT-OMNIAGENT-004) — the value is never part of the listing.
func TestListAgentSecretNames(t *testing.T) {
	svc, err := NewService(Config{Vault: memory.New()})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()
	agentID := uuid.New()

	if got, err := svc.ListAgentSecretNames(ctx, agentID); err != nil || len(got) != 0 {
		t.Fatalf("empty list = %v, %v; want [] nil", got, err)
	}

	if err := svc.SetAgentSecret(ctx, agentID, "GITHUB_TOKEN", "ghp_xxx"); err != nil {
		t.Fatalf("SetAgentSecret: %v", err)
	}
	if err := svc.SetAgentSecret(ctx, agentID, "WEATHER_KEY", "wk_yyy"); err != nil {
		t.Fatalf("SetAgentSecret: %v", err)
	}

	names, err := svc.ListAgentSecretNames(ctx, agentID)
	if err != nil {
		t.Fatalf("ListAgentSecretNames: %v", err)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	if len(names) != 2 || !got["GITHUB_TOKEN"] || !got["WEATHER_KEY"] {
		t.Errorf("names = %v, want [GITHUB_TOKEN WEATHER_KEY]", names)
	}

	if err := svc.DeleteAgentSecret(ctx, agentID, "GITHUB_TOKEN"); err != nil {
		t.Fatalf("DeleteAgentSecret: %v", err)
	}
	names, _ = svc.ListAgentSecretNames(ctx, agentID)
	if len(names) != 1 || names[0] != "WEATHER_KEY" {
		t.Errorf("names after delete = %v, want [WEATHER_KEY]", names)
	}
}

// TestResolveAgentSecrets_Empty confirms an agent with no secrets yields a
// non-nil empty map (so injection is a clean no-op).
func TestResolveAgentSecrets_Empty(t *testing.T) {
	svc, err := NewService(Config{Vault: memory.New()})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	env, err := svc.ResolveAgentSecrets(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ResolveAgentSecrets: %v", err)
	}
	if env == nil || len(env) != 0 {
		t.Errorf("env = %v, want empty non-nil map", env)
	}
}

// TestResolveAgentSecrets_Disjoint is the RMI-310 gate at the service layer: two
// agents over one backing store resolve disjoint secrets with no cross-leak.
func TestResolveAgentSecrets_Disjoint(t *testing.T) {
	svc, err := NewService(Config{Vault: memory.New()})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	if err := svc.SetAgentSecret(ctx, a, "TOKEN", "aaa"); err != nil {
		t.Fatalf("set a: %v", err)
	}
	if err := svc.SetAgentSecret(ctx, b, "TOKEN", "bbb"); err != nil {
		t.Fatalf("set b: %v", err)
	}

	envA, err := svc.ResolveAgentSecrets(ctx, a)
	if err != nil {
		t.Fatalf("resolve a: %v", err)
	}
	envB, err := svc.ResolveAgentSecrets(ctx, b)
	if err != nil {
		t.Fatalf("resolve b: %v", err)
	}
	if envA["TOKEN"] != "aaa" || len(envA) != 1 {
		t.Errorf("agent A env = %v, want {TOKEN:aaa}", envA)
	}
	if envB["TOKEN"] != "bbb" || len(envB) != 1 {
		t.Errorf("agent B env = %v, want {TOKEN:bbb}", envB)
	}
}
