package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/chat"
	"github.com/plexusone/omniagent/team/ent/chatmember"
	"github.com/plexusone/omniagent/team/ent/message"
	entuser "github.com/plexusone/omniagent/team/ent/user"
)

// TestSQLiteRoundTrip exercises the personal-mode path with no external
// database: dialect inference from a plain file path, schema creation, and
// AsSystem/AsUser as pass-throughs (no RLS, single implicit user).
func TestSQLiteRoundTrip(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "personal.db")
	cfg := Config{AppDSN: dsn}

	if dialectFromDSN(dsn) != "sqlite3" {
		t.Fatalf("dialectFromDSN(%q) = %q, want sqlite3", dsn, dialectFromDSN(dsn))
	}

	if err := Migrate(ctx, cfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Idempotency: rerunning schema creation must not fail.
	if err := Migrate(ctx, cfg); err != nil {
		t.Fatalf("Migrate (rerun): %v", err)
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

	var owner *ent.User
	if err := s.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		owner, err = tx.User.Create().
			SetEmail("me@example.com").SetUsername("me").
			SetRole(entuser.RoleSuperadmin).Save(ctx)
		return err
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// AsUser is a pass-through in personal mode: the single user sees
	// everything, with no RLS GUCs to set.
	if err := s.AsUser(ctx, owner.ID, false, func(ctx context.Context, tx *ent.Tx) error {
		n, err := tx.User.Query().Count(ctx)
		if err != nil {
			return err
		}
		if n != 1 {
			t.Errorf("user count = %d, want 1", n)
		}

		privateChat, err := tx.Chat.Create().
			SetType(chat.TypePrivate).SetCreatedBy(owner.ID).Save(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.ChatMember.Create().
			SetChatID(privateChat.ID).SetUserID(owner.ID).
			SetRole(chatmember.RoleOwner).Save(ctx); err != nil {
			return err
		}
		if _, err := tx.Message.Create().
			SetChatID(privateChat.ID).SetAuthorType(message.AuthorTypeUser).
			SetAuthorUserID(owner.ID).SetContent("hello, personal mode").Save(ctx); err != nil {
			return err
		}

		nMsgs, err := tx.Message.Query().Where(message.ChatIDEQ(privateChat.ID)).Count(ctx)
		if err != nil {
			return err
		}
		if nMsgs != 1 {
			t.Errorf("message count = %d, want 1", nMsgs)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
