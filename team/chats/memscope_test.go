package chats

import (
	"context"
	"testing"

	"github.com/plexusone/omniagent/memscope"
)

// capturingAgent records the memory scope and session ID it is invoked with, so
// tests can assert the chats service stamps the per-chat memory scope onto the
// turn context (RMI-OMNIAGENT-114).
type capturingAgent struct {
	scope   memscope.Scope
	scoped  bool
	session string
}

func (a *capturingAgent) Process(ctx context.Context, sessionID, _ string) (string, error) {
	a.scope, a.scoped = memscope.FromContext(ctx)
	a.session = sessionID
	return "ok", nil
}

// TestRunTurn_StampsChatMemoryScope verifies that in team mode (MemoryTenant
// set) a chat turn runs with TenantID=team, SubjectID="chat:<id>", matching the
// session key — so a chat's memories are isolated to that chat.
func TestRunTurn_StampsChatMemoryScope(t *testing.T) {
	st, userID := openTestStore(t)
	cap := &capturingAgent{}
	svc, err := NewService(st, Config{Agent: cap, MemoryTenant: MemoryTenantTeam})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()

	c, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	if _, err := svc.GenerateReply(ctx, c.ID, "hi"); err != nil {
		t.Fatalf("GenerateReply: %v", err)
	}

	if !cap.scoped {
		t.Fatal("no memory scope stamped on the turn context")
	}
	want := SessionID(c.ID)
	if cap.scope.TenantID != MemoryTenantTeam {
		t.Errorf("scope tenant = %q, want %q", cap.scope.TenantID, MemoryTenantTeam)
	}
	if cap.scope.SubjectID != want {
		t.Errorf("scope subject = %q, want %q", cap.scope.SubjectID, want)
	}
	if cap.session != want {
		t.Errorf("session id = %q, want %q (subject must mirror the session key)", cap.session, want)
	}
}

// TestRunTurn_NoScopeWhenMemoryTenantUnset verifies personal mode (no
// MemoryTenant) leaves memory scoping to the agent's own configuration —
// nothing is stamped, so behavior is unchanged.
func TestRunTurn_NoScopeWhenMemoryTenantUnset(t *testing.T) {
	st, userID := openTestStore(t)
	cap := &capturingAgent{}
	svc, err := NewService(st, Config{Agent: cap})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()

	c, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	if _, err := svc.GenerateReply(ctx, c.ID, "hi"); err != nil {
		t.Fatalf("GenerateReply: %v", err)
	}

	if cap.scoped {
		t.Errorf("memory scope stamped in personal mode: %+v", cap.scope)
	}
}

// TestAgentTurn_StampsChatMemoryScope verifies the mention-gated group turn also
// runs under the per-chat memory scope when a runtime is wired.
func TestAgentTurn_StampsChatMemoryScope(t *testing.T) {
	st, userID := openTestStore(t)
	cap := &capturingAgent{}
	rt := &fakeRuntime{slug: "helper", proc: cap}
	svc, err := NewService(st, Config{Agents: &fakeGate{allow: true}, Runtime: rt, MemoryTenant: MemoryTenantTeam})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()

	agentID := seedAgent(t, st, userID, "helper")
	c, err := svc.CreateGroupWithAgent(ctx, Actor{UserID: userID}, "Room", agentID)
	if err != nil {
		t.Fatalf("CreateGroupWithAgent: %v", err)
	}
	_, responded, err := svc.AgentTurn(ctx, c, "hey @helper")
	if err != nil {
		t.Fatalf("AgentTurn: %v", err)
	}
	if !responded {
		t.Fatal("mentioned agent did not respond")
	}
	if !cap.scoped || cap.scope.TenantID != MemoryTenantTeam || cap.scope.SubjectID != SessionID(c.ID) {
		t.Errorf("scope = %+v (scoped=%v), want {team, %s}", cap.scope, cap.scoped, SessionID(c.ID))
	}
}
