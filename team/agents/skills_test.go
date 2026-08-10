package agents

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	entuser "github.com/plexusone/omniagent/team/ent/user"
	"github.com/plexusone/omniagent/team/store"
)

// setupWithCatalog is like setup but injects an available-skills catalog and
// deny-list so the RMI-302 subset validation can be exercised.
func setupWithCatalog(t *testing.T, available, blocked []string) *fixture {
	t.Helper()
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "agents.db")
	cfg := store.Config{AppDSN: dsn}
	if err := store.Migrate(ctx, cfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st, err := store.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	f := &fixture{}
	if err := st.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		root, err := tx.User.Create().
			SetEmail("root@example.com").SetUsername("root").
			SetRole(entuser.RoleSuperadmin).Save(ctx)
		if err != nil {
			return err
		}
		alice, err := tx.User.Create().SetEmail("alice@example.com").SetUsername("alice").Save(ctx)
		if err != nil {
			return err
		}
		bob, err := tx.User.Create().SetEmail("bob@example.com").SetUsername("bob").Save(ctx)
		if err != nil {
			return err
		}
		f.superadmin, f.alice, f.bob = root.ID, alice.ID, bob.ID
		return nil
	}); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	svc, err := NewService(st, Config{AvailableSkills: available, BlockedSkills: blocked})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	f.svc = svc
	return f
}

func TestAvailableSkills_ExcludesBlocked(t *testing.T) {
	f := setupWithCatalog(t, []string{"web_search", "shell", "calendar"}, []string{"shell"})
	got := f.svc.AvailableSkills()
	want := []string{"calendar", "web_search"} // sorted, shell removed
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AvailableSkills = %v, want %v", got, want)
	}
}

func TestSetAgentSkills_SubsetEnforced(t *testing.T) {
	f := setupWithCatalog(t, []string{"web_search", "calendar"}, []string{"shell"})
	ctx := context.Background()

	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "skill-bot", Name: "Skill"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Valid subset (case-insensitive), with a duplicate collapsed.
	if err := f.svc.SetAgentSkills(ctx, f.actor(f.alice), a.ID, []string{"WEB_SEARCH", "calendar", "web_search"}); err != nil {
		t.Fatalf("SetAgentSkills: %v", err)
	}
	got, err := f.svc.AgentSkills(ctx, f.actor(f.alice), a.ID)
	if err != nil {
		t.Fatalf("AgentSkills: %v", err)
	}
	want := []string{"calendar", "web_search"} // canonical casing, sorted, deduped
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AgentSkills = %v, want %v", got, want)
	}

	// Unknown skill is rejected and leaves the set unchanged.
	if err := f.svc.SetAgentSkills(ctx, f.actor(f.alice), a.ID, []string{"web_search", "does_not_exist"}); !errors.Is(err, ErrUnknownSkill) {
		t.Errorf("unknown skill err = %v, want ErrUnknownSkill", err)
	}
	// Blocked skill is rejected distinctly.
	if err := f.svc.SetAgentSkills(ctx, f.actor(f.alice), a.ID, []string{"shell"}); !errors.Is(err, ErrBlockedSkill) {
		t.Errorf("blocked skill err = %v, want ErrBlockedSkill", err)
	}
	// The valid set from before is intact (rejection was atomic).
	got, err = f.svc.AgentSkills(ctx, f.actor(f.alice), a.ID)
	if err != nil {
		t.Fatalf("AgentSkills: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("after rejected sets, AgentSkills = %v, want %v", got, want)
	}
}

func TestSetAgentSkills_EditorOnly(t *testing.T) {
	f := setupWithCatalog(t, []string{"web_search"}, nil)
	ctx := context.Background()
	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "guard-bot", Name: "Guard"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if err := f.svc.SetAgentSkills(ctx, f.actor(f.bob), a.ID, []string{"web_search"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("bob set skills err = %v, want ErrForbidden", err)
	}
}

func TestSetAgentSkills_Replaces(t *testing.T) {
	f := setupWithCatalog(t, []string{"a", "bbb", "ccc"}, nil)
	ctx := context.Background()
	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "replace-bot", Name: "Replace"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if err := f.svc.SetAgentSkills(ctx, f.actor(f.alice), a.ID, []string{"a", "bbb"}); err != nil {
		t.Fatalf("first set: %v", err)
	}
	// Replacing with a smaller set removes the others (full replace, not merge).
	if err := f.svc.SetAgentSkills(ctx, f.actor(f.alice), a.ID, []string{"ccc"}); err != nil {
		t.Fatalf("second set: %v", err)
	}
	got, err := f.svc.AgentSkills(ctx, f.actor(f.alice), a.ID)
	if err != nil {
		t.Fatalf("AgentSkills: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"ccc"}) {
		t.Errorf("AgentSkills = %v, want [ccc]", got)
	}
}

func TestAgentSkills_NotFound(t *testing.T) {
	f := setupWithCatalog(t, []string{"a"}, nil)
	ctx := context.Background()
	if _, err := f.svc.AgentSkills(ctx, f.actor(f.alice), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
