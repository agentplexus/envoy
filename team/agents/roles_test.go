package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestAddMaintainer_OwnerOnly(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "team-bot", Name: "Team"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Owner adds bob as maintainer.
	if _, err := f.svc.AddMaintainer(ctx, f.actor(f.alice), a.ID, "bob"); err != nil {
		t.Fatalf("AddMaintainer: %v", err)
	}

	// Bob (maintainer) can now configure the agent...
	name := "renamed"
	if _, err := f.svc.UpdateAgent(ctx, f.actor(f.bob), a.ID, UpdateSpec{Name: &name}); err != nil {
		t.Errorf("maintainer update failed: %v", err)
	}
	// ...but cannot add another maintainer (owner-only).
	if _, err := f.svc.AddMaintainer(ctx, f.actor(f.bob), a.ID, "root"); !errors.Is(err, ErrForbidden) {
		t.Errorf("maintainer AddMaintainer err = %v, want ErrForbidden", err)
	}

	roles, err := f.svc.Roles(ctx, f.actor(f.alice), a.ID)
	if err != nil {
		t.Fatalf("Roles: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("roles = %d, want 2", len(roles))
	}
}

func TestAddMaintainer_Idempotent_NoDemote(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "idem-bot", Name: "Idem"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	// Adding the owner "as maintainer" must not demote them.
	role, err := f.svc.AddMaintainer(ctx, f.actor(f.alice), a.ID, "alice")
	if err != nil {
		t.Fatalf("AddMaintainer(owner): %v", err)
	}
	if role.Role.String() != "owner" {
		t.Errorf("owner role changed to %q", role.Role)
	}

	// Adding bob twice is idempotent.
	if _, err := f.svc.AddMaintainer(ctx, f.actor(f.alice), a.ID, "bob"); err != nil {
		t.Fatalf("add bob: %v", err)
	}
	if _, err := f.svc.AddMaintainer(ctx, f.actor(f.alice), a.ID, "bob"); err != nil {
		t.Fatalf("re-add bob: %v", err)
	}
	roles, err := f.svc.Roles(ctx, f.actor(f.alice), a.ID)
	if err != nil {
		t.Fatalf("Roles: %v", err)
	}
	if len(roles) != 2 {
		t.Errorf("roles = %d, want 2 (alice owner + bob maintainer)", len(roles))
	}
}

func TestAddMaintainer_UserNotFound(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "nf-bot", Name: "NF"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if _, err := f.svc.AddMaintainer(ctx, f.actor(f.alice), a.ID, "nobody"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}

func TestRemoveMaintainer(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "rm-bot", Name: "RM"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	m, err := f.svc.AddMaintainer(ctx, f.actor(f.alice), a.ID, "bob")
	if err != nil {
		t.Fatalf("AddMaintainer: %v", err)
	}

	// A maintainer cannot remove another; only owner/superadmin.
	if err := f.svc.RemoveMaintainer(ctx, f.actor(f.bob), a.ID, f.superadmin); !errors.Is(err, ErrForbidden) {
		t.Errorf("maintainer remove err = %v, want ErrForbidden", err)
	}
	// Cannot remove yourself via RemoveMaintainer.
	if err := f.svc.RemoveMaintainer(ctx, f.actor(f.alice), a.ID, f.alice); !errors.Is(err, ErrForbidden) {
		t.Errorf("self-remove err = %v, want ErrForbidden", err)
	}
	// Owner cannot be removed by RemoveMaintainer.
	if err := f.svc.RemoveMaintainer(ctx, f.actor(f.bob), a.ID, f.alice); err == nil {
		t.Error("expected error removing owner")
	}
	// Owner removes the maintainer.
	if err := f.svc.RemoveMaintainer(ctx, f.actor(f.alice), a.ID, m.UserID); err != nil {
		t.Fatalf("owner remove: %v", err)
	}
	roles, err := f.svc.Roles(ctx, f.actor(f.alice), a.ID)
	if err != nil {
		t.Fatalf("Roles: %v", err)
	}
	if len(roles) != 1 {
		t.Errorf("roles = %d, want 1", len(roles))
	}
}

func TestLeaveAgent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	t.Run("maintainer self-leave", func(t *testing.T) {
		a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "leave1", Name: "L1"})
		if err != nil {
			t.Fatalf("CreateAgent: %v", err)
		}
		if _, err := f.svc.AddMaintainer(ctx, f.actor(f.alice), a.ID, "bob"); err != nil {
			t.Fatalf("AddMaintainer: %v", err)
		}
		if err := f.svc.LeaveAgent(ctx, f.actor(f.bob), a.ID); err != nil {
			t.Fatalf("bob leave: %v", err)
		}
	})

	t.Run("sole owner with other members cannot leave", func(t *testing.T) {
		a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "leave2", Name: "L2"})
		if err != nil {
			t.Fatalf("CreateAgent: %v", err)
		}
		if _, err := f.svc.AddMaintainer(ctx, f.actor(f.alice), a.ID, "bob"); err != nil {
			t.Fatalf("AddMaintainer: %v", err)
		}
		if err := f.svc.LeaveAgent(ctx, f.actor(f.alice), a.ID); !errors.Is(err, ErrLastOwner) {
			t.Errorf("sole owner leave err = %v, want ErrLastOwner", err)
		}
	})

	t.Run("sole owner alone may leave (orphan)", func(t *testing.T) {
		a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "leave3", Name: "L3"})
		if err != nil {
			t.Fatalf("CreateAgent: %v", err)
		}
		if err := f.svc.LeaveAgent(ctx, f.actor(f.alice), a.ID); err != nil {
			t.Errorf("sole owner alone leave: %v", err)
		}
	})

	t.Run("non-member leave is forbidden", func(t *testing.T) {
		a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "leave4", Name: "L4"})
		if err != nil {
			t.Fatalf("CreateAgent: %v", err)
		}
		if err := f.svc.LeaveAgent(ctx, f.actor(f.bob), a.ID); !errors.Is(err, ErrForbidden) {
			t.Errorf("non-member leave err = %v, want ErrForbidden", err)
		}
	})
}

func TestRoles_EditorOnly(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "roles-bot", Name: "Roles"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if _, err := f.svc.Roles(ctx, f.actor(f.bob), a.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("non-editor Roles err = %v, want ErrForbidden", err)
	}
	// Superadmin may administer any agent's roles.
	if _, err := f.svc.AddMaintainer(ctx, f.superAdminActor(), a.ID, "bob"); err != nil {
		t.Errorf("superadmin AddMaintainer: %v", err)
	}
}

func TestRemoveMaintainer_NotFound(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "rmnf-bot", Name: "RMNF"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	// bob has no role → removing him returns ErrForbidden (not-found mapped).
	if err := f.svc.RemoveMaintainer(ctx, f.actor(f.alice), a.ID, uuid.New()); !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}
