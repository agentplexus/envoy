package store

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/internal/pgtest"
	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/agent"
	"github.com/plexusone/omniagent/team/ent/agentrole"
	"github.com/plexusone/omniagent/team/ent/agentskill"
	entuser "github.com/plexusone/omniagent/team/ent/user"
)

// setupAgentFixture reuses setupRLS's user seeding (superadmin/alice/bob) so
// this suite can run independently of the chat-focused rls_test.go.
func setupAgentFixture(t *testing.T) *rlsFixture {
	t.Helper()
	ownerDSN, appDSN := pgtest.DSNs(t)
	ctx := context.Background()

	cfg := Config{AppDSN: appDSN, MigrateDSN: ownerDSN}
	if err := Migrate(ctx, cfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	f := &rlsFixture{store: s}
	if err := s.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		f.superadmin, err = tx.User.Create().
			SetEmail("root@example.com").SetUsername("root").
			SetRole(entuser.RoleSuperadmin).Save(ctx)
		if err != nil {
			return err
		}
		f.alice, err = tx.User.Create().
			SetEmail("alice@example.com").SetUsername("alice").Save(ctx)
		if err != nil {
			return err
		}
		f.bob, err = tx.User.Create().
			SetEmail("bob@example.com").SetUsername("bob").Save(ctx)
		return err
	}); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	return f
}

// createAgentAsUser exercises the two-step bootstrap: an authenticated user
// creates the agent row, then inserts their own owner role, mirroring the
// insert-bootstrap pattern used for chats/chat_members.
func createAgentAsUser(t *testing.T, s *Store, userID uuid.UUID, slug string) *ent.Agent {
	t.Helper()
	ctx := context.Background()
	var a *ent.Agent
	if err := s.AsUser(ctx, userID, false, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		a, err = tx.Agent.Create().
			SetSlug(slug).SetName(slug).SetCreatedBy(userID).Save(ctx)
		if err != nil {
			return err
		}
		_, err = tx.AgentRole.Create().
			SetAgentID(a.ID).SetUserID(userID).SetRole(agentrole.RoleOwner).Save(ctx)
		return err
	}); err != nil {
		t.Fatalf("createAgentAsUser: %v", err)
	}
	return a
}

func TestAgentsRLS(t *testing.T) {
	f := setupAgentFixture(t)
	s := f.store
	ctx := context.Background()

	t.Run("creator bootstraps as owner via self-insert", func(t *testing.T) {
		a := createAgentAsUser(t, s, f.alice.ID, "alices-bot")

		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			role, err := tx.AgentRole.Query().
				Where(agentrole.AgentIDEQ(a.ID), agentrole.UserIDEQ(f.alice.ID)).Only(ctx)
			if err != nil {
				return err
			}
			if role.Role != agentrole.RoleOwner {
				t.Errorf("creator role = %q, want owner", role.Role)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("private agent invisible to non-editors", func(t *testing.T) {
		a := createAgentAsUser(t, s, f.alice.ID, "private-bot")

		if err := s.AsUser(ctx, f.bob.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			exists, err := tx.Agent.Query().Where(agent.IDEQ(a.ID)).Exist(ctx)
			if err != nil {
				return err
			}
			if exists {
				t.Error("bob can see alice's private agent")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		// Superadmin CAN see it (administrative visibility) but per PRD/TRD
		// this is row visibility, not secret access — a separate concern.
		if err := s.AsUser(ctx, f.superadmin.ID, true, func(ctx context.Context, tx *ent.Tx) error {
			exists, err := tx.Agent.Query().Where(agent.IDEQ(a.ID)).Exist(ctx)
			if err != nil {
				return err
			}
			if !exists {
				t.Error("superadmin cannot see the private agent (expected administrative visibility)")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("listed agent visible to any user", func(t *testing.T) {
		a := createAgentAsUser(t, s, f.alice.ID, "listed-bot")
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.Agent.UpdateOneID(a.ID).SetVisibility(agent.VisibilityListed).Save(ctx)
			return err
		}); err != nil {
			t.Fatalf("set listed: %v", err)
		}

		if err := s.AsUser(ctx, f.bob.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			exists, err := tx.Agent.Query().Where(agent.IDEQ(a.ID)).Exist(ctx)
			if err != nil {
				return err
			}
			if !exists {
				t.Error("bob cannot see alice's listed agent")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("non-editor cannot update or add skills", func(t *testing.T) {
		a := createAgentAsUser(t, s, f.alice.ID, "guarded-bot")

		if err := s.AsUser(ctx, f.bob.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			if _, err := tx.Agent.UpdateOneID(a.ID).SetDescription("hax").Save(ctx); err == nil {
				// If ent doesn't error (RLS-filtered update reload quirk),
				// assert the durable state below did not change.
				t.Log("update did not error; verifying durable state instead")
			}
			if _, err := tx.AgentSkill.Create().SetAgentID(a.ID).SetSkill("shell").Save(ctx); err == nil {
				t.Error("bob could add a skill to alice's agent")
			}
			return nil
		}); err != nil && !isRLSViolation(err) {
			t.Fatal(err)
		}

		// Durable-state check (per ENGINEERING.md: never trust ent update
		// errors alone under RLS).
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			got, err := tx.Agent.Get(ctx, a.ID)
			if err != nil {
				return err
			}
			if got.Description == "hax" {
				t.Error("bob's denied update persisted")
			}
			n, err := tx.AgentSkill.Query().Where(agentskill.AgentIDEQ(a.ID)).Count(ctx)
			if err != nil {
				return err
			}
			if n != 0 {
				t.Error("bob's denied skill insert persisted")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("editor can update and add skills", func(t *testing.T) {
		a := createAgentAsUser(t, s, f.alice.ID, "editable-bot")

		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			if _, err := tx.Agent.UpdateOneID(a.ID).SetDescription("updated").Save(ctx); err != nil {
				t.Errorf("owner update failed: %v", err)
			}
			if _, err := tx.AgentSkill.Create().SetAgentID(a.ID).SetSkill("web_search").Save(ctx); err != nil {
				t.Errorf("owner add skill failed: %v", err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("owner adds maintainer; maintainer cannot add another maintainer", func(t *testing.T) {
		a := createAgentAsUser(t, s, f.alice.ID, "team-bot")

		// Owner (alice) adds bob as maintainer.
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.AgentRole.Create().
				SetAgentID(a.ID).SetUserID(f.bob.ID).SetRole(agentrole.RoleMaintainer).Save(ctx)
			return err
		}); err != nil {
			t.Fatalf("owner add maintainer: %v", err)
		}

		// Maintainer (bob) can now edit the agent...
		if err := s.AsUser(ctx, f.bob.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			if _, err := tx.Agent.UpdateOneID(a.ID).SetDescription("maintained").Save(ctx); err != nil {
				t.Errorf("maintainer update failed: %v", err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		// ...but cannot add a third maintainer (owner-only capability).
		if err := s.AsUser(ctx, f.bob.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			if _, err := tx.AgentRole.Create().
				SetAgentID(a.ID).SetUserID(f.superadmin.ID).SetRole(agentrole.RoleMaintainer).Save(ctx); err == nil {
				t.Error("maintainer could add another maintainer")
			}
			return nil
		}); err != nil && !isRLSViolation(err) {
			t.Fatal(err)
		}

		// Durable check: no second maintainer row was created by bob.
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			n, err := tx.AgentRole.Query().Where(agentrole.AgentIDEQ(a.ID)).Count(ctx)
			if err != nil {
				return err
			}
			if n != 2 { // alice (owner) + bob (maintainer)
				t.Errorf("agent_roles count = %d, want 2", n)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("maintainer can self-leave", func(t *testing.T) {
		a := createAgentAsUser(t, s, f.alice.ID, "leave-bot")
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.AgentRole.Create().
				SetAgentID(a.ID).SetUserID(f.bob.ID).SetRole(agentrole.RoleMaintainer).Save(ctx)
			return err
		}); err != nil {
			t.Fatalf("add maintainer: %v", err)
		}

		if err := s.AsUser(ctx, f.bob.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.AgentRole.Delete().
				Where(agentrole.AgentIDEQ(a.ID), agentrole.UserIDEQ(f.bob.ID)).Exec(ctx)
			return err
		}); err != nil {
			t.Fatalf("self-leave: %v", err)
		}

		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			exists, err := tx.AgentRole.Query().
				Where(agentrole.AgentIDEQ(a.ID), agentrole.UserIDEQ(f.bob.ID)).Exist(ctx)
			if err != nil {
				return err
			}
			if exists {
				t.Error("bob's role row still exists after self-leave")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("only owner can remove another agent's maintainer", func(t *testing.T) {
		a := createAgentAsUser(t, s, f.alice.ID, "remove-bot")
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.AgentRole.Create().
				SetAgentID(a.ID).SetUserID(f.bob.ID).SetRole(agentrole.RoleMaintainer).Save(ctx)
			return err
		}); err != nil {
			t.Fatalf("add maintainer: %v", err)
		}

		// Owner removes the maintainer.
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.AgentRole.Delete().
				Where(agentrole.AgentIDEQ(a.ID), agentrole.UserIDEQ(f.bob.ID)).Exec(ctx)
			return err
		}); err != nil {
			t.Fatalf("owner remove maintainer: %v", err)
		}

		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			exists, err := tx.AgentRole.Query().
				Where(agentrole.AgentIDEQ(a.ID), agentrole.UserIDEQ(f.bob.ID)).Exist(ctx)
			if err != nil {
				return err
			}
			if exists {
				t.Error("maintainer role still exists after owner removed it")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("only owner can delete the agent, not a maintainer", func(t *testing.T) {
		a := createAgentAsUser(t, s, f.alice.ID, "delete-bot")
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.AgentRole.Create().
				SetAgentID(a.ID).SetUserID(f.bob.ID).SetRole(agentrole.RoleMaintainer).Save(ctx)
			return err
		}); err != nil {
			t.Fatalf("add maintainer: %v", err)
		}

		if err := s.AsUser(ctx, f.bob.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			if err := tx.Agent.DeleteOneID(a.ID).Exec(ctx); !ent.IsNotFound(err) {
				t.Errorf("maintainer delete: err = %v, want not-found (denied)", err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			return tx.Agent.DeleteOneID(a.ID).Exec(ctx)
		}); err != nil {
			t.Errorf("owner delete failed: %v", err)
		}
	})

	t.Run("superadmin can update but agent-editor rules still apply to others", func(t *testing.T) {
		a := createAgentAsUser(t, s, f.alice.ID, "admin-bot")

		if err := s.AsUser(ctx, f.superadmin.ID, true, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.Agent.UpdateOneID(a.ID).SetFeatured(true).Save(ctx)
			return err
		}); err != nil {
			t.Errorf("superadmin update (featured) failed: %v", err)
		}

		if err := s.AsUser(ctx, f.bob.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			got, err := tx.Agent.Get(ctx, a.ID)
			// bob is not an editor and the agent is private, so he should
			// not even be able to read it (visibility unchanged).
			if err == nil {
				t.Errorf("bob unexpectedly read the private agent: %+v", got)
			}
			return nil
		}); err != nil && !ent.IsNotFound(err) {
			t.Fatal(err)
		}
	})
}

func mustParseUUID(t *testing.T, s string) (out [16]byte) {
	t.Helper()
	u, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", s, err)
	}
	return u
}
