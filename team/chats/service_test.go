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
