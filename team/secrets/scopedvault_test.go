package secrets

import (
	"context"
	"testing"

	"github.com/plexusone/omnivault/providers/memory"
	"github.com/plexusone/omnivault/vault"
)

// TestScopedVault_Isolation is the core tenancy guarantee: two scopes over one
// backing store cannot read each other's keys, and a prefix that is a substring
// of another namespace ("agents/A" vs "agents/AB") does not leak across the
// path boundary.
func TestScopedVault_Isolation(t *testing.T) {
	back := memory.New()
	a := Scoped(back, "agents/A")
	ab := Scoped(back, "agents/AB")

	ctx := context.Background()
	if err := a.Set(ctx, "TOKEN", &vault.Secret{Value: "aaa"}); err != nil {
		t.Fatalf("a.Set: %v", err)
	}
	if err := ab.Set(ctx, "TOKEN", &vault.Secret{Value: "bbb"}); err != nil {
		t.Fatalf("ab.Set: %v", err)
	}

	// Each scope reads only its own value.
	got, err := a.Get(ctx, "TOKEN")
	if err != nil {
		t.Fatalf("a.Get: %v", err)
	}
	if got.String() != "aaa" {
		t.Errorf("a TOKEN = %q, want aaa", got.String())
	}
	got, err = ab.Get(ctx, "TOKEN")
	if err != nil {
		t.Fatalf("ab.Get: %v", err)
	}
	if got.String() != "bbb" {
		t.Errorf("ab TOKEN = %q, want bbb", got.String())
	}

	// List on "agents/A" must not spill "agents/AB" keys.
	names, err := a.List(ctx, "")
	if err != nil {
		t.Fatalf("a.List: %v", err)
	}
	if len(names) != 1 || names[0] != "TOKEN" {
		t.Errorf("a.List = %v, want [TOKEN]", names)
	}
}

// TestScopedVault_KeysAreNamespaced confirms the backing store sees namespaced
// keys while the scope sees relative ones.
func TestScopedVault_KeysAreNamespaced(t *testing.T) {
	back := memory.New()
	a := Scoped(back, "agents/A")

	ctx := context.Background()
	if err := a.Set(ctx, "K", &vault.Secret{Value: "v"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Backing key is fully namespaced.
	if _, err := back.Get(ctx, "agents/A/K"); err != nil {
		t.Errorf("backing Get agents/A/K: %v", err)
	}
	// Relative key is not directly present in the backing store.
	if ok, _ := back.Exists(ctx, "K"); ok {
		t.Error("backing store unexpectedly has un-namespaced key K")
	}

	// Delete via the scope removes the namespaced key.
	if err := a.Delete(ctx, "K"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if ok, _ := a.Exists(ctx, "K"); ok {
		t.Error("K still exists after Delete")
	}
}

// TestScopedVault_Passthrough confirms an empty namespace is a passthrough.
func TestScopedVault_Passthrough(t *testing.T) {
	back := memory.New()
	p := Scoped(back, "")

	ctx := context.Background()
	if err := p.Set(ctx, "K", &vault.Secret{Value: "v"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := back.Get(ctx, "K"); err != nil {
		t.Errorf("backing Get K: %v", err)
	}
}

// TestScopedVault_TrimsSlashes confirms surrounding slashes on ns are ignored so
// "agents/A", "/agents/A" and "agents/A/" address the same namespace.
func TestScopedVault_TrimsSlashes(t *testing.T) {
	back := memory.New()
	ctx := context.Background()
	if err := Scoped(back, "agents/A").Set(ctx, "K", &vault.Secret{Value: "v"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := Scoped(back, "/agents/A/").Get(ctx, "K")
	if err != nil {
		t.Fatalf("Get with slashes: %v", err)
	}
	if got.String() != "v" {
		t.Errorf("value = %q, want v", got.String())
	}
}
