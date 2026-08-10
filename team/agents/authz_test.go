package agents

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/agent"
)

// setVisibility flips an agent's visibility directly via the store (RMI-306's
// SetVisibility is a later item); the authz matrix only reads the field.
func (f *fixture) setVisibility(t *testing.T, agentID uuid.UUID, v agent.Visibility) {
	t.Helper()
	if err := f.st.AsSystem(context.Background(), func(ctx context.Context, tx *ent.Tx) error {
		_, err := tx.Agent.UpdateOneID(agentID).SetVisibility(v).Save(ctx)
		return err
	}); err != nil {
		t.Fatalf("setVisibility: %v", err)
	}
}

func TestCan_Matrix(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "matrix-bot", Name: "Matrix"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if _, err := f.svc.AddMaintainer(ctx, f.actor(f.alice), a.ID, "bob"); err != nil {
		t.Fatalf("AddMaintainer: %v", err)
	}

	owner := f.actor(f.alice)
	maintainer := f.actor(f.bob)
	stranger := Actor{UserID: uuid.New()} // no role
	super := f.superAdminActor()

	type row struct {
		name  string
		actor Actor
		cap   Capability
		want  bool
	}
	rows := []row{
		// Owner: everything.
		{"owner configure", owner, CapConfigure, true},
		{"owner manage maintainers", owner, CapManageMaintainers, true},
		{"owner manage registry", owner, CapManageRegistry, true},
		{"owner create chat", owner, CapCreateChat, true},
		{"owner administer", owner, CapAdminister, false},

		// Maintainer: all but ManageMaintainers and Administer.
		{"maintainer configure", maintainer, CapConfigure, true},
		{"maintainer manage registry", maintainer, CapManageRegistry, true},
		{"maintainer create chat", maintainer, CapCreateChat, true},
		{"maintainer manage maintainers", maintainer, CapManageMaintainers, false},
		{"maintainer administer", maintainer, CapAdminister, false},

		// Stranger: nothing on a private agent.
		{"stranger configure", stranger, CapConfigure, false},
		{"stranger create chat (private)", stranger, CapCreateChat, false},
		{"stranger chat (private)", stranger, CapChat, false},

		// Superadmin: administer + everything except (secrets, not modeled).
		{"superadmin administer", super, CapAdminister, true},
		{"superadmin configure", super, CapConfigure, true},
		{"superadmin manage maintainers", super, CapManageMaintainers, true},
		{"superadmin create chat", super, CapCreateChat, true},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			got, err := f.svc.Can(ctx, r.actor, a.ID, r.cap)
			if err != nil {
				t.Fatalf("Can: %v", err)
			}
			if got != r.want {
				t.Errorf("Can(%s) = %v, want %v", r.cap, got, r.want)
			}
		})
	}

	// Once listed, any user may start a chat / converse, but still not configure.
	f.setVisibility(t, a.ID, agent.VisibilityListed)
	for _, r := range []row{
		{"stranger create chat (listed)", stranger, CapCreateChat, true},
		{"stranger chat (listed)", stranger, CapChat, true},
		{"stranger configure (listed)", stranger, CapConfigure, false},
		{"stranger manage registry (listed)", stranger, CapManageRegistry, false},
	} {
		t.Run(r.name, func(t *testing.T) {
			got, err := f.svc.Can(ctx, r.actor, a.ID, r.cap)
			if err != nil {
				t.Fatalf("Can: %v", err)
			}
			if got != r.want {
				t.Errorf("Can(%s) = %v, want %v", r.cap, got, r.want)
			}
		})
	}
}

func TestCan_NonexistentAgent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	ok, err := f.svc.Can(ctx, f.superAdminActor(), uuid.New(), CapConfigure)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if ok {
		t.Error("Can on a nonexistent agent = true, want false")
	}
}
