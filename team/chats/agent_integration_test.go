package chats

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/store"
)

// fakeGate is a stub AgentGate: it records the last call and returns a fixed
// verdict, so the chats service's RMI-308 gating can be tested without the real
// agents registry.
type fakeGate struct {
	allow     bool
	err       error
	lastAgent uuid.UUID
	lastUser  uuid.UUID
	calls     int
}

func (g *fakeGate) AuthorizeStartChat(_ context.Context, userID uuid.UUID, _ bool, agentID uuid.UUID) (bool, error) {
	g.calls++
	g.lastUser = userID
	g.lastAgent = agentID
	return g.allow, g.err
}

// setupGated builds a chats service wired to the given gate, plus the store
// (so agent rows can be seeded for the chats.agent_id FK) and a seeded user.
func setupGated(t *testing.T, gate AgentGate) (*Service, *store.Store, uuid.UUID) {
	t.Helper()
	st, userID := openTestStore(t)
	svc, err := NewService(st, Config{Agents: gate})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, st, userID
}

// seedAgent inserts an agent row (owned by createdBy) so a chat may reference
// it — the stub gate authorizes creation, but the agent_id FK still requires a
// real row.
func seedAgent(t *testing.T, st *store.Store, createdBy uuid.UUID, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := st.AsSystem(context.Background(), func(ctx context.Context, tx *ent.Tx) error {
		a, err := tx.Agent.Create().SetSlug(slug).SetName(slug).SetCreatedBy(createdBy).Save(ctx)
		if err != nil {
			return err
		}
		id = a.ID
		return nil
	}); err != nil {
		t.Fatalf("seedAgent: %v", err)
	}
	return id
}

func TestStartAgentDM_NoRegistry(t *testing.T) {
	svc, userID := setupChats(t, nil) // no Agents gate
	if _, err := svc.StartAgentDM(context.Background(), Actor{UserID: userID}, uuid.New()); !errors.Is(err, ErrNoAgentRegistry) {
		t.Errorf("err = %v, want ErrNoAgentRegistry", err)
	}
}

func TestStartAgentDM_Forbidden(t *testing.T) {
	gate := &fakeGate{allow: false}
	svc, _, userID := setupGated(t, gate)
	if _, err := svc.StartAgentDM(context.Background(), Actor{UserID: userID}, uuid.New()); !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
	if gate.calls != 1 {
		t.Errorf("gate calls = %d, want 1", gate.calls)
	}
}

func TestStartAgentDM_GetOrCreatePerAgent(t *testing.T) {
	gate := &fakeGate{allow: true}
	svc, st, userID := setupGated(t, gate)
	ctx := context.Background()
	actor := Actor{UserID: userID}
	agentA := seedAgent(t, st, userID, "agent-a")
	agentB := seedAgent(t, st, userID, "agent-b")

	c1, err := svc.StartAgentDM(ctx, actor, agentA)
	if err != nil {
		t.Fatalf("StartAgentDM(A): %v", err)
	}
	if c1.AgentID == nil || *c1.AgentID != agentA {
		t.Fatalf("chat agent_id = %v, want %v", c1.AgentID, agentA)
	}
	// Idempotent: same agent returns the same chat.
	c1b, err := svc.StartAgentDM(ctx, actor, agentA)
	if err != nil {
		t.Fatalf("StartAgentDM(A) again: %v", err)
	}
	if c1b.ID != c1.ID {
		t.Errorf("second DM id = %v, want %v (one per user per agent)", c1b.ID, c1.ID)
	}
	// A different agent yields a distinct DM.
	c2, err := svc.StartAgentDM(ctx, actor, agentB)
	if err != nil {
		t.Fatalf("StartAgentDM(B): %v", err)
	}
	if c2.ID == c1.ID {
		t.Error("different agents shared a DM")
	}
	if gate.lastAgent != agentB || gate.lastUser != userID {
		t.Errorf("gate saw (%v,%v), want (%v,%v)", gate.lastUser, gate.lastAgent, userID, agentB)
	}
}

func TestCreateGroupWithAgent(t *testing.T) {
	ctx := context.Background()

	t.Run("no registry", func(t *testing.T) {
		svc, userID := setupChats(t, nil)
		if _, err := svc.CreateGroupWithAgent(ctx, Actor{UserID: userID}, "g", uuid.New()); !errors.Is(err, ErrNoAgentRegistry) {
			t.Errorf("err = %v, want ErrNoAgentRegistry", err)
		}
	})

	t.Run("forbidden", func(t *testing.T) {
		svc, _, userID := setupGated(t, &fakeGate{allow: false})
		if _, err := svc.CreateGroupWithAgent(ctx, Actor{UserID: userID}, "g", uuid.New()); !errors.Is(err, ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("empty name rejected before gate", func(t *testing.T) {
		gate := &fakeGate{allow: true}
		svc, _, userID := setupGated(t, gate)
		if _, err := svc.CreateGroupWithAgent(ctx, Actor{UserID: userID}, "  ", uuid.New()); !errors.Is(err, ErrEmptyName) {
			t.Errorf("err = %v, want ErrEmptyName", err)
		}
	})

	t.Run("created with agent binding", func(t *testing.T) {
		gate := &fakeGate{allow: true}
		svc, st, userID := setupGated(t, gate)
		agentID := seedAgent(t, st, userID, "group-agent")
		c, err := svc.CreateGroupWithAgent(ctx, Actor{UserID: userID}, "Team Room", agentID)
		if err != nil {
			t.Fatalf("CreateGroupWithAgent: %v", err)
		}
		if c.AgentID == nil || *c.AgentID != agentID {
			t.Errorf("chat agent_id = %v, want %v", c.AgentID, agentID)
		}
		if c.Name != "Team Room" {
			t.Errorf("name = %q", c.Name)
		}
	})
}
