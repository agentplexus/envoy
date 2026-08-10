package chats

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

// echoAgent returns a fixed reply so tests can assert the exact turn
// without depending on a real LLM.
type echoAgent struct{ reply string }

func (a *echoAgent) Process(_ context.Context, _, content string) (string, error) {
	if a.reply != "" {
		return a.reply, nil
	}
	return "echo: " + content, nil
}

func setupChats(t *testing.T, agent AgentProcessor) (*Service, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "chats.db")
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

	var userID uuid.UUID
	if err := st.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		u, err := tx.User.Create().
			SetEmail("me@example.com").SetUsername("me").
			SetRole(entuser.RoleSuperadmin).Save(ctx)
		if err != nil {
			return err
		}
		userID = u.ID
		return nil
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	svc, err := NewService(st, Config{Agent: agent})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, userID
}

func TestPrivateChat_GetOrCreate(t *testing.T) {
	svc, userID := setupChats(t, nil)
	ctx := context.Background()

	c1, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	c2, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat (again): %v", err)
	}
	if c1.ID != c2.ID {
		t.Errorf("PrivateChat returned different chats: %s vs %s, want the same DM every time", c1.ID, c2.ID)
	}
}

func TestSend_PersistsAndRepliesViaAgent(t *testing.T) {
	svc, userID := setupChats(t, &echoAgent{reply: "hello back"})
	ctx := context.Background()

	c, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}

	userMsg, agentMsg, err := svc.Send(ctx, userID, c.ID, "hi there")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if userMsg.Content != "hi there" {
		t.Errorf("userMsg.Content = %q, want %q", userMsg.Content, "hi there")
	}
	if agentMsg.Content != "hello back" {
		t.Errorf("agentMsg.Content = %q, want %q", agentMsg.Content, "hello back")
	}
	if agentMsg.AuthorUserID != nil {
		t.Errorf("agentMsg.AuthorUserID = %v, want nil (agent-authored)", agentMsg.AuthorUserID)
	}

	msgs, err := svc.History(ctx, userID, c.ID, nil, 10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("History returned %d messages, want 2", len(msgs))
	}
	if msgs[0].ID != userMsg.ID || msgs[1].ID != agentMsg.ID {
		t.Error("History is not oldest-first (user then agent)")
	}
}

func TestSend_NoAgentConfiguredEchoes(t *testing.T) {
	svc, userID := setupChats(t, nil)
	ctx := context.Background()

	c, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	_, agentMsg, err := svc.Send(ctx, userID, c.ID, "ping")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if agentMsg.Content != "Message received: ping" {
		t.Errorf("agentMsg.Content = %q, want echo fallback", agentMsg.Content)
	}
}

func TestSend_EmptyMessageRejected(t *testing.T) {
	svc, userID := setupChats(t, nil)
	ctx := context.Background()
	c, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	if _, _, err := svc.Send(ctx, userID, c.ID, "   "); !errors.Is(err, ErrEmptyMessage) {
		t.Errorf("Send(blank) err = %v, want ErrEmptyMessage", err)
	}
}

func TestSend_NonMemberForbidden(t *testing.T) {
	svc, userID := setupChats(t, nil)
	ctx := context.Background()
	c, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}

	stranger := uuid.New()
	if _, _, err := svc.Send(ctx, stranger, c.ID, "intrusion"); !errors.Is(err, ErrForbidden) {
		t.Errorf("Send by non-member err = %v, want ErrForbidden", err)
	}
	if _, err := svc.History(ctx, stranger, c.ID, nil, 10); !errors.Is(err, ErrForbidden) {
		t.Errorf("History by non-member err = %v, want ErrForbidden", err)
	}
}

func TestHistoryBefore_ScrollBack(t *testing.T) {
	svc, userID := setupChats(t, &echoAgent{reply: "ok"})
	ctx := context.Background()
	c, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, _, err := svc.Send(ctx, userID, c.ID, "msg"); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	// 6 messages total. Newest page of 2, oldest-first.
	newest, err := svc.HistoryBefore(ctx, userID, c.ID, nil, 2)
	if err != nil {
		t.Fatalf("HistoryBefore(nil): %v", err)
	}
	if len(newest) != 2 {
		t.Fatalf("newest page = %d messages, want 2", len(newest))
	}
	if newest[0].ID.String() >= newest[1].ID.String() {
		t.Error("HistoryBefore is not oldest-first")
	}

	// The full forward history, for cross-checking positions.
	all, err := svc.History(ctx, userID, c.ID, nil, 100)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if newest[1].ID != all[5].ID || newest[0].ID != all[4].ID {
		t.Error("newest page is not the last two messages")
	}

	// Scroll back before the oldest loaded → the two preceding messages.
	older, err := svc.HistoryBefore(ctx, userID, c.ID, &newest[0].ID, 2)
	if err != nil {
		t.Fatalf("HistoryBefore(before): %v", err)
	}
	if len(older) != 2 || older[0].ID != all[2].ID || older[1].ID != all[3].ID {
		t.Errorf("older page = %v, want messages [2],[3]", older)
	}
}

func TestHistoryBefore_NonMemberForbidden(t *testing.T) {
	svc, userID := setupChats(t, nil)
	ctx := context.Background()
	c, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	if _, err := svc.HistoryBefore(ctx, uuid.New(), c.ID, nil, 10); !errors.Is(err, ErrForbidden) {
		t.Errorf("HistoryBefore by non-member err = %v, want ErrForbidden", err)
	}
}

// createUser inserts an additional user (system context) and returns its ID,
// so group tests can exercise multi-member membership.
func createUser(t *testing.T, svc *Service, username string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	if err := svc.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		u, err := tx.User.Create().
			SetEmail(username + "@example.com").SetUsername(username).
			SetRole(entuser.RoleMember).Save(ctx)
		if err != nil {
			return err
		}
		id = u.ID
		return nil
	}); err != nil {
		t.Fatalf("createUser %s: %v", username, err)
	}
	return id
}

func TestCreateGroup_OwnerMembership(t *testing.T) {
	svc, ownerID := setupChats(t, nil)
	ctx := context.Background()
	owner := Actor{UserID: ownerID}

	c, err := svc.CreateGroup(ctx, owner, "  Family  ")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if c.Name != "Family" {
		t.Errorf("group name = %q, want trimmed %q", c.Name, "Family")
	}
	if c.Type.String() != "group" {
		t.Errorf("group type = %q, want group", c.Type)
	}

	members, err := svc.Members(ctx, owner, c.ID)
	if err != nil {
		t.Fatalf("Members: %v", err)
	}
	if len(members) != 1 || members[0].UserID != ownerID || members[0].Role.String() != "owner" {
		t.Fatalf("creator membership = %+v, want single owner", members)
	}

	if _, err := svc.CreateGroup(ctx, owner, "   "); !errors.Is(err, ErrEmptyName) {
		t.Errorf("CreateGroup(blank) err = %v, want ErrEmptyName", err)
	}
}

func TestInvite_AddsConversantAndFansOut(t *testing.T) {
	svc, ownerID := setupChats(t, nil)
	ctx := context.Background()
	owner := Actor{UserID: ownerID}
	bobID := createUser(t, svc, "bob")

	c, err := svc.CreateGroup(ctx, owner, "Trip")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	m, err := svc.Invite(ctx, owner, c.ID, "BOB") // case-insensitive
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if m.UserID != bobID || m.Role.String() != "member" {
		t.Errorf("invited membership = %+v, want bob as member", m)
	}

	// Idempotent re-invite.
	if _, err := svc.Invite(ctx, owner, c.ID, "bob"); err != nil {
		t.Fatalf("re-Invite: %v", err)
	}

	ids, err := svc.MemberUserIDs(ctx, owner, c.ID)
	if err != nil {
		t.Fatalf("MemberUserIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("member count = %d, want 2 (owner + bob)", len(ids))
	}

	// Unknown username.
	if _, err := svc.Invite(ctx, owner, c.ID, "nobody"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("Invite(unknown) err = %v, want ErrUserNotFound", err)
	}
}

func TestInvite_NonOwnerForbidden(t *testing.T) {
	svc, ownerID := setupChats(t, nil)
	ctx := context.Background()
	owner := Actor{UserID: ownerID}
	bobID := createUser(t, svc, "bob")
	carolID := createUser(t, svc, "carol")

	c, err := svc.CreateGroup(ctx, owner, "Trip")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if _, err := svc.Invite(ctx, owner, c.ID, "bob"); err != nil {
		t.Fatalf("Invite bob: %v", err)
	}

	// bob is a plain member, not an owner → cannot invite carol.
	_ = carolID
	if _, err := svc.Invite(ctx, Actor{UserID: bobID}, c.ID, "carol"); !errors.Is(err, ErrForbidden) {
		t.Errorf("non-owner Invite err = %v, want ErrForbidden", err)
	}
	// A complete stranger cannot invite either.
	if _, err := svc.Invite(ctx, Actor{UserID: uuid.New()}, c.ID, "carol"); !errors.Is(err, ErrForbidden) {
		t.Errorf("stranger Invite err = %v, want ErrForbidden", err)
	}
}

func TestInvite_PrivateChatRejected(t *testing.T) {
	svc, ownerID := setupChats(t, nil)
	ctx := context.Background()
	owner := Actor{UserID: ownerID}
	createUser(t, svc, "bob")

	dm, err := svc.PrivateChat(ctx, ownerID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	if _, err := svc.Invite(ctx, owner, dm.ID, "bob"); !errors.Is(err, ErrNotGroup) {
		t.Errorf("Invite into private DM err = %v, want ErrNotGroup", err)
	}
}

func TestLeaveAndRemove(t *testing.T) {
	svc, ownerID := setupChats(t, nil)
	ctx := context.Background()
	owner := Actor{UserID: ownerID}
	bobID := createUser(t, svc, "bob")

	c, err := svc.CreateGroup(ctx, owner, "Trip")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if _, err := svc.Invite(ctx, owner, c.ID, "bob"); err != nil {
		t.Fatalf("Invite: %v", err)
	}

	// Sole owner cannot leave while bob remains.
	if err := svc.Leave(ctx, owner, c.ID); !errors.Is(err, ErrLastOwner) {
		t.Errorf("owner Leave err = %v, want ErrLastOwner", err)
	}

	// Owner cannot remove themselves via RemoveMember.
	if err := svc.RemoveMember(ctx, owner, c.ID, ownerID); !errors.Is(err, ErrForbidden) {
		t.Errorf("RemoveMember(self) err = %v, want ErrForbidden", err)
	}

	// A plain member cannot remove anyone.
	if err := svc.RemoveMember(ctx, Actor{UserID: bobID}, c.ID, ownerID); !errors.Is(err, ErrForbidden) {
		t.Errorf("member RemoveMember err = %v, want ErrForbidden", err)
	}

	// Owner removes bob.
	if err := svc.RemoveMember(ctx, owner, c.ID, bobID); err != nil {
		t.Fatalf("RemoveMember bob: %v", err)
	}
	ids, err := svc.MemberUserIDs(ctx, owner, c.ID)
	if err != nil {
		t.Fatalf("MemberUserIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != ownerID {
		t.Fatalf("after removal members = %v, want [owner]", ids)
	}

	// Now the sole owner (last member) may leave.
	if err := svc.Leave(ctx, owner, c.ID); err != nil {
		t.Errorf("last-member owner Leave err = %v, want nil", err)
	}

	// bob (removed) can no longer see the chat.
	if _, err := svc.GetChat(ctx, Actor{UserID: bobID}, c.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("removed member GetChat err = %v, want ErrForbidden", err)
	}
}

func TestListChats_MemberScoped(t *testing.T) {
	svc, aliceID := setupChats(t, nil)
	ctx := context.Background()
	alice := Actor{UserID: aliceID}
	bobID := createUser(t, svc, "bob")
	bob := Actor{UserID: bobID}

	// Alice owns a private DM and a group; bob is in neither yet.
	if _, err := svc.PrivateChat(ctx, aliceID); err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	g, err := svc.CreateGroup(ctx, alice, "Trip")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	aliceChats, err := svc.ListChats(ctx, alice)
	if err != nil {
		t.Fatalf("ListChats(alice): %v", err)
	}
	if len(aliceChats) != 2 {
		t.Fatalf("alice sees %d chats, want 2", len(aliceChats))
	}

	bobChats, err := svc.ListChats(ctx, bob)
	if err != nil {
		t.Fatalf("ListChats(bob): %v", err)
	}
	if len(bobChats) != 0 {
		t.Fatalf("bob sees %d chats, want 0 before invite", len(bobChats))
	}

	if _, err := svc.Invite(ctx, alice, g.ID, "bob"); err != nil {
		t.Fatalf("Invite bob: %v", err)
	}
	bobChats, err = svc.ListChats(ctx, bob)
	if err != nil {
		t.Fatalf("ListChats(bob) after invite: %v", err)
	}
	if len(bobChats) != 1 || bobChats[0].ID != g.ID {
		t.Fatalf("bob sees %v after invite, want [group]", bobChats)
	}
}

func TestHistory_KeysetPagination(t *testing.T) {
	svc, userID := setupChats(t, &echoAgent{reply: "ok"})
	ctx := context.Background()
	c, err := svc.PrivateChat(ctx, userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, _, err := svc.Send(ctx, userID, c.ID, "msg"); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	// 3 sends = 6 messages (user+agent each).
	all, err := svc.History(ctx, userID, c.ID, nil, 100)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(all) != 6 {
		t.Fatalf("History returned %d messages, want 6", len(all))
	}

	cursor := all[1].ID
	rest, err := svc.History(ctx, userID, c.ID, &cursor, 100)
	if err != nil {
		t.Fatalf("History (paginated): %v", err)
	}
	if len(rest) != 4 {
		t.Fatalf("History after cursor returned %d messages, want 4", len(rest))
	}
	if rest[0].ID != all[2].ID {
		t.Error("History after cursor did not resume at the next message")
	}
}
