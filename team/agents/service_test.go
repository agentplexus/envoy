package agents

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	entuser "github.com/plexusone/omniagent/team/ent/user"
	"github.com/plexusone/omniagent/team/store"
)

// fixture holds a sqlite-backed service and three seeded users. On the SQLite
// path there is no RLS, so these tests exercise the service-layer
// authorization checks directly (the RLS backstop is covered separately by
// team/store/agents_rls_test.go against PostgreSQL).
type fixture struct {
	svc        *Service
	st         *store.Store
	superadmin uuid.UUID
	alice      uuid.UUID
	bob        uuid.UUID
}

func setup(t *testing.T) *fixture {
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

	svc, err := NewService(st, Config{})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	f.svc = svc
	f.st = st
	return f
}

func (f *fixture) actor(id uuid.UUID) Actor { return Actor{UserID: id} }

func (f *fixture) superAdminActor() Actor {
	return Actor{UserID: f.superadmin, Superadmin: true}
}

func TestCreateAgent_BootstrapsOwner(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "alices-bot", Name: "Alice's Bot"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if a.Slug != "alices-bot" || a.Name != "Alice's Bot" {
		t.Errorf("unexpected agent: %+v", a)
	}
	if a.CreatedBy != f.alice {
		t.Errorf("created_by = %v, want alice", a.CreatedBy)
	}

	// Alice is the owner and it shows up in her list.
	mine, err := f.svc.ListMyAgents(ctx, f.actor(f.alice))
	if err != nil {
		t.Fatalf("ListMyAgents: %v", err)
	}
	if len(mine) != 1 || mine[0].ID != a.ID {
		t.Errorf("ListMyAgents = %+v, want [%v]", mine, a.ID)
	}
}

func TestCreateAgent_Validation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	cases := []struct {
		name string
		spec CreateSpec
		want error
	}{
		{"empty slug", CreateSpec{Slug: "", Name: "X"}, ErrInvalidSlug},
		{"too short slug", CreateSpec{Slug: "ab", Name: "X"}, ErrInvalidSlug},
		{"bad char slug", CreateSpec{Slug: "Bad Slug!", Name: "X"}, ErrInvalidSlug},
		{"empty name", CreateSpec{Slug: "valid-slug", Name: "  "}, ErrEmptyName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.svc.CreateAgent(ctx, f.actor(f.alice), tc.spec); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCreateAgent_SlugTaken(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "dup", Name: "First"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	// Same slug (even different case) from another user is rejected.
	if _, err := f.svc.CreateAgent(ctx, f.actor(f.bob), CreateSpec{Slug: "DUP", Name: "Second"}); !errors.Is(err, ErrSlugTaken) {
		t.Errorf("err = %v, want ErrSlugTaken", err)
	}
}

func TestUpdateAgent_EditorOnly(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "cfg-bot", Name: "Cfg"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	desc := "owner-set description"
	if _, err := f.svc.UpdateAgent(ctx, f.actor(f.alice), a.ID, UpdateSpec{Description: &desc}); err != nil {
		t.Fatalf("owner update: %v", err)
	}

	// Bob is not an editor.
	hax := "hax"
	if _, err := f.svc.UpdateAgent(ctx, f.actor(f.bob), a.ID, UpdateSpec{Description: &hax}); !errors.Is(err, ErrForbidden) {
		t.Errorf("bob update err = %v, want ErrForbidden", err)
	}

	// Superadmin may configure any agent.
	admin := "admin-set"
	if _, err := f.svc.UpdateAgent(ctx, f.superAdminActor(), a.ID, UpdateSpec{Description: &admin}); err != nil {
		t.Errorf("superadmin update: %v", err)
	}

	got, err := f.svc.GetAgent(ctx, f.actor(f.alice), a.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.Description != admin {
		t.Errorf("description = %q, want %q", got.Description, admin)
	}
}

func TestUpdateAgent_NotFound(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	name := "x"
	if _, err := f.svc.UpdateAgent(ctx, f.actor(f.alice), uuid.New(), UpdateSpec{Name: &name}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteAgent_OwnerOnly(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "del-bot", Name: "Del"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Non-owner cannot delete.
	if err := f.svc.DeleteAgent(ctx, f.actor(f.bob), a.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("bob delete err = %v, want ErrForbidden", err)
	}

	// Owner can.
	if err := f.svc.DeleteAgent(ctx, f.actor(f.alice), a.ID); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if _, err := f.svc.GetAgent(ctx, f.actor(f.alice), a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete GetAgent err = %v, want ErrNotFound", err)
	}
}
