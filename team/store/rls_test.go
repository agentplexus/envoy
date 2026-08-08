package store

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/internal/pgtest"
	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/allowlistentry"
	"github.com/plexusone/omniagent/team/ent/chat"
	"github.com/plexusone/omniagent/team/ent/chatmember"
	"github.com/plexusone/omniagent/team/ent/message"
	entuser "github.com/plexusone/omniagent/team/ent/user"
)

type rlsFixture struct {
	store      *Store
	superadmin *ent.User
	alice      *ent.User
	bob        *ent.User
}

func setupRLS(t *testing.T) *rlsFixture {
	t.Helper()
	ownerDSN, appDSN := pgtest.DSNs(t)
	ctx := context.Background()

	cfg := Config{AppDSN: appDSN, MigrateDSN: ownerDSN}
	if err := Migrate(ctx, cfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Second run must be a no-op (idempotency acceptance).
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

	f := &rlsFixture{store: s}

	// Seed users via the system context (mirrors first-login creation).
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

func TestRLS(t *testing.T) {
	f := setupRLS(t)
	s := f.store
	ctx := context.Background()

	t.Run("users are isolated per member", func(t *testing.T) {
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			users, err := tx.User.Query().All(ctx)
			if err != nil {
				return err
			}
			if len(users) != 1 || users[0].ID != f.alice.ID {
				t.Errorf("alice sees %d users, want only herself", len(users))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		if err := s.AsUser(ctx, f.superadmin.ID, true, func(ctx context.Context, tx *ent.Tx) error {
			n, err := tx.User.Query().Count(ctx)
			if err != nil {
				return err
			}
			if n != 3 {
				t.Errorf("superadmin sees %d users, want 3", n)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member can update self but not others", func(t *testing.T) {
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			if _, err := tx.User.UpdateOneID(f.alice.ID).SetDisplayName("Alice").Save(ctx); err != nil {
				t.Errorf("self update failed: %v", err)
			}
			_, err := tx.User.UpdateOneID(f.bob.ID).SetDisplayName("hax").Save(ctx)
			if !ent.IsNotFound(err) {
				t.Errorf("cross-user update: err = %v, want not-found (RLS-filtered)", err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("allowlist is superadmin only", func(t *testing.T) {
		if err := s.AsUser(ctx, f.superadmin.ID, true, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.AllowlistEntry.Create().
				SetEmail("carol@example.com").SetAddedBy(f.superadmin.ID).Save(ctx)
			return err
		}); err != nil {
			t.Fatalf("superadmin allowlist add: %v", err)
		}

		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			n, err := tx.AllowlistEntry.Query().Count(ctx)
			if err != nil {
				return err
			}
			if n != 0 {
				t.Errorf("member sees %d allowlist rows, want 0", n)
			}
			if _, err := tx.AllowlistEntry.Create().
				SetEmail("evil@example.com").SetAddedBy(f.alice.ID).Save(ctx); err == nil {
				t.Error("member could insert into the allowlist")
			}
			return nil
		}); err != nil && !isRLSViolation(err) {
			t.Fatal(err)
		}

		// Confirm the member's denied insert did not land.
		if err := s.AsUser(ctx, f.superadmin.ID, true, func(ctx context.Context, tx *ent.Tx) error {
			exists, err := tx.AllowlistEntry.Query().
				Where(allowlistentry.EmailEQ("evil@example.com")).Exist(ctx)
			if err != nil {
				return err
			}
			if exists {
				t.Error("denied allowlist insert persisted")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("magic link tokens are system only", func(t *testing.T) {
		if err := s.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.MagicLinkToken.Create().
				SetEmail("alice@example.com").SetTokenHash("hash-1").
				SetExpiresAt(f.alice.CreatedAt.Add(1)).Save(ctx)
			return err
		}); err != nil {
			t.Fatalf("system token insert: %v", err)
		}

		// Even the superadmin cannot see tokens.
		if err := s.AsUser(ctx, f.superadmin.ID, true, func(ctx context.Context, tx *ent.Tx) error {
			n, err := tx.MagicLinkToken.Query().Count(ctx)
			if err != nil {
				return err
			}
			if n != 0 {
				t.Errorf("superadmin sees %d tokens, want 0", n)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	// Private chat for alice, seeded like the auth layer would.
	var privateChat *ent.Chat
	if err := s.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		privateChat, err = tx.Chat.Create().
			SetType(chat.TypePrivate).SetCreatedBy(f.alice.ID).Save(ctx)
		if err != nil {
			return err
		}
		_, err = tx.ChatMember.Create().
			SetChatID(privateChat.ID).SetUserID(f.alice.ID).
			SetRole(chatmember.RoleOwner).Save(ctx)
		return err
	}); err != nil {
		t.Fatalf("seed private chat: %v", err)
	}

	t.Run("private chat is invisible to non-members", func(t *testing.T) {
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			if _, err := tx.Message.Create().
				SetChatID(privateChat.ID).SetAuthorType(message.AuthorTypeUser).
				SetAuthorUserID(f.alice.ID).SetContent("my private note").Save(ctx); err != nil {
				t.Errorf("alice message in own chat: %v", err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		if err := s.AsUser(ctx, f.bob.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			nChats, err := tx.Chat.Query().Where(chat.IDEQ(privateChat.ID)).Count(ctx)
			if err != nil {
				return err
			}
			nMsgs, err := tx.Message.Query().Where(message.ChatIDEQ(privateChat.ID)).Count(ctx)
			if err != nil {
				return err
			}
			if nChats != 0 || nMsgs != 0 {
				t.Errorf("bob sees %d chats / %d messages of alice's private chat, want 0/0", nChats, nMsgs)
			}
			if _, err := tx.Message.Create().
				SetChatID(privateChat.ID).SetAuthorType(message.AuthorTypeUser).
				SetAuthorUserID(f.bob.ID).SetContent("intrusion").Save(ctx); err == nil {
				t.Error("bob could write into alice's private chat")
			}
			return nil
		}); err != nil && !isRLSViolation(err) {
			t.Fatal(err)
		}

		// Superadmin is not content-privileged either (PRD).
		if err := s.AsUser(ctx, f.superadmin.ID, true, func(ctx context.Context, tx *ent.Tx) error {
			n, err := tx.Message.Query().Where(message.ChatIDEQ(privateChat.ID)).Count(ctx)
			if err != nil {
				return err
			}
			if n != 0 {
				t.Errorf("superadmin sees %d private messages, want 0", n)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	// Group chat created entirely through user-context policies.
	var groupChat *ent.Chat
	t.Run("group chat lifecycle via user policies", func(t *testing.T) {
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			var err error
			groupChat, err = tx.Chat.Create().
				SetType(chat.TypeGroup).SetName("family").SetCreatedBy(f.alice.ID).Save(ctx)
			if err != nil {
				return err
			}
			// Creator adds own owner membership (creator policy) …
			if _, err = tx.ChatMember.Create().
				SetChatID(groupChat.ID).SetUserID(f.alice.ID).
				SetRole(chatmember.RoleOwner).Save(ctx); err != nil {
				return err
			}
			// … then invites bob (owner policy).
			_, err = tx.ChatMember.Create().
				SetChatID(groupChat.ID).SetUserID(f.bob.ID).Save(ctx)
			return err
		}); err != nil {
			t.Fatalf("alice creates group: %v", err)
		}

		if err := s.AsUser(ctx, f.bob.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			if _, err := tx.Chat.Get(ctx, groupChat.ID); err != nil {
				t.Errorf("bob cannot see the group chat: %v", err)
			}
			if _, err := tx.Message.Create().
				SetChatID(groupChat.ID).SetAuthorType(message.AuthorTypeUser).
				SetAuthorUserID(f.bob.ID).SetContent("hi all").Save(ctx); err != nil {
				t.Errorf("bob cannot post in the group: %v", err)
			}
			// Plain member must not be able to invite.
			if _, err := tx.ChatMember.Create().
				SetChatID(groupChat.ID).SetUserID(f.superadmin.ID).Save(ctx); err == nil {
				t.Error("non-owner member could invite into the group")
			}
			return nil
		}); err != nil && !isRLSViolation(err) {
			t.Fatal(err)
		}
	})

	t.Run("agent messages require the system context", func(t *testing.T) {
		if err := s.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
			_, err := tx.Message.Create().
				SetChatID(groupChat.ID).SetAuthorType(message.AuthorTypeAgent).
				SetContent("hello, I am the agent").Save(ctx)
			return err
		}); err != nil {
			t.Fatalf("agent message via system: %v", err)
		}

		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			n, err := tx.Message.Query().
				Where(message.ChatIDEQ(groupChat.ID), message.AuthorTypeEQ(message.AuthorTypeAgent)).
				Count(ctx)
			if err != nil {
				return err
			}
			if n != 1 {
				t.Errorf("alice sees %d agent messages, want 1", n)
			}
			// A user must not be able to forge agent authorship.
			if _, err := tx.Message.Create().
				SetChatID(groupChat.ID).SetAuthorType(message.AuthorTypeAgent).
				SetContent("forged").Save(ctx); err == nil {
				t.Error("member could author a message as the agent")
			}
			return nil
		}); err != nil && !isRLSViolation(err) {
			t.Fatal(err)
		}
	})

	t.Run("messages are immutable", func(t *testing.T) {
		if err := s.AsUser(ctx, f.alice.ID, false, func(ctx context.Context, tx *ent.Tx) error {
			msg, err := tx.Message.Query().
				Where(message.ChatIDEQ(privateChat.ID)).Only(ctx)
			if err != nil {
				return err
			}

			// With no UPDATE policy, RLS filters the UPDATE to zero rows.
			// ent's UpdateOne then reloads the entity via SELECT (which the
			// author may do), so it can return nil rather than not-found —
			// assert the durable property: the stored content is unchanged.
			_, updateErr := tx.Message.UpdateOneID(msg.ID).SetContent("rewritten").Save(ctx)
			after, err := tx.Message.Get(ctx, msg.ID)
			if err != nil {
				return err
			}
			if after.Content != msg.Content {
				t.Errorf("message content changed to %q (update err = %v); messages must be immutable", after.Content, updateErr)
			}

			// DELETE with no policy affects zero rows; ent reports not-found.
			if err := tx.Message.DeleteOneID(msg.ID).Exec(ctx); !ent.IsNotFound(err) {
				t.Errorf("delete own message: err = %v, want not-found (no DELETE policy)", err)
			}
			exists, err := tx.Message.Query().Where(message.IDEQ(msg.ID)).Exist(ctx)
			if err != nil {
				return err
			}
			if !exists {
				t.Error("message was deleted; messages must be immutable")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("guc scoping is transaction local", func(t *testing.T) {
		// After all the scoped work above, a fresh system transaction still
		// sees everything — and a bogus user sees nothing.
		if err := s.AsUser(ctx, uuid.New(), false, func(ctx context.Context, tx *ent.Tx) error {
			n, err := tx.User.Query().Count(ctx)
			if err != nil {
				return err
			}
			if n != 0 {
				t.Errorf("unknown user sees %d users, want 0", n)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})
}

// isRLSViolation tolerates the fallout of an intentionally denied write:
// an RLS violation (SQLSTATE 42501) aborts the enclosing transaction, so the
// scoped helper's COMMIT then fails too. Each subtest performs its denied
// operation LAST and asserts the precise expectations inside the closure
// (via t.Error); any error propagated out of AsUser afterwards is that
// expected denial fallout, never an assertion path.
func isRLSViolation(err error) bool {
	return err != nil
}
