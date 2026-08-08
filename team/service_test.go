package team

import (
	"context"
	"errors"
	"testing"

	"github.com/plexusone/omniagent/internal/pgtest"
	"github.com/plexusone/omniagent/team/ent"
	entuser "github.com/plexusone/omniagent/team/ent/user"
	"github.com/plexusone/omniagent/team/store"
)

// The suite needs the same two-role PostgreSQL as team/store; see
// deploy/team/dev/docker-compose.dev.yaml.
func setupService(t *testing.T, superadminEmail string) *Service {
	t.Helper()
	ownerDSN, appDSN := pgtest.DSNs(t)
	ctx := context.Background()

	cfg := store.Config{AppDSN: appDSN, MigrateDSN: ownerDSN}
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

	svc, err := NewService(st, Config{SuperadminEmail: superadminEmail})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestBootstrapAndService(t *testing.T) {
	svc := setupService(t, "root@example.com")
	ctx := context.Background()

	// --- Superadmin bootstrap on first login ---------------------------
	root, created, err := svc.EnsureUser(ctx, "Root@Example.com") // case-insensitive
	if err != nil || !created {
		t.Fatalf("EnsureUser(root): created=%v err=%v", created, err)
	}
	if root.Role != entuser.RoleSuperadmin {
		t.Fatalf("bootstrap role = %v, want superadmin", root.Role)
	}
	rootActor := Actor{UserID: root.ID, Superadmin: true}

	// Second login: same user, not recreated.
	again, created, err := svc.EnsureUser(ctx, "root@example.com")
	if err != nil || created || again.ID != root.ID {
		t.Fatalf("EnsureUser(root, again): id=%v created=%v err=%v", again.ID, created, err)
	}

	// --- Username derivation ------------------------------------------
	alice, created, err := svc.EnsureUser(ctx, "Alice.Smith+home@example.com")
	if err != nil || !created {
		t.Fatalf("EnsureUser(alice): %v", err)
	}
	if alice.Username != "alice-smith-home" {
		t.Errorf("derived username = %q, want alice-smith-home", alice.Username)
	}
	if alice.Role != entuser.RoleMember {
		t.Errorf("alice role = %v, want member", alice.Role)
	}
	aliceActor := Actor{UserID: alice.ID}

	// Collision on the same local part gets a numeric suffix.
	alice2, _, err := svc.EnsureUser(ctx, "alice.smith+home@other.example")
	if err != nil {
		t.Fatalf("EnsureUser(alice2): %v", err)
	}
	if alice2.Username != "alice-smith-home-2" {
		t.Errorf("collision username = %q, want alice-smith-home-2", alice2.Username)
	}

	// --- Member authorization boundaries ------------------------------
	if _, err := svc.ListUsers(ctx, aliceActor); !errors.Is(err, ErrForbidden) {
		t.Errorf("member ListUsers err = %v, want forbidden", err)
	}
	if _, err := svc.AllowlistAdd(ctx, aliceActor, "friend@example.com", ""); !errors.Is(err, ErrForbidden) {
		t.Errorf("member AllowlistAdd err = %v, want forbidden", err)
	}
	if err := svc.SetUserStatus(ctx, aliceActor, root.ID, entuser.StatusDisabled); !errors.Is(err, ErrForbidden) {
		t.Errorf("member SetUserStatus err = %v, want forbidden", err)
	}
	if err := svc.RenameUser(ctx, aliceActor, root.ID, "stolen"); !errors.Is(err, ErrForbidden) {
		t.Errorf("member rename of other err = %v, want forbidden", err)
	}

	// --- Rename (US-3: including the superadmin themselves) ------------
	if err := svc.RenameUser(ctx, rootActor, root.ID, "Captain"); err != nil {
		t.Errorf("superadmin self-rename: %v", err)
	}
	self, err := svc.GetSelf(ctx, rootActor)
	if err != nil || self.Username != "captain" {
		t.Errorf("renamed username = %q err=%v, want captain (lowercased)", self.Username, err)
	}
	if err := svc.RenameUser(ctx, aliceActor, alice.ID, "alice"); err != nil {
		t.Errorf("member self-rename: %v", err)
	}
	if err := svc.RenameUser(ctx, aliceActor, alice.ID, "no spaces!"); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("invalid username err = %v, want ErrInvalidUsername", err)
	}
	// Uniqueness: taking an existing username fails.
	if err := svc.RenameUser(ctx, aliceActor, alice.ID, "captain"); err == nil {
		t.Error("rename onto an existing username succeeded")
	}

	// --- Allowlist -----------------------------------------------------
	if _, err := svc.AllowlistAdd(ctx, rootActor, "Friend@Example.com", "school friend"); err != nil {
		t.Fatalf("AllowlistAdd: %v", err)
	}
	// Idempotent re-add.
	if _, err := svc.AllowlistAdd(ctx, rootActor, "friend@example.com", ""); err != nil {
		t.Errorf("idempotent AllowlistAdd: %v", err)
	}
	entries, err := svc.AllowlistList(ctx, rootActor)
	if err != nil || len(entries) != 1 {
		t.Errorf("AllowlistList: %d entries err=%v, want 1", len(entries), err)
	}

	if ok, err := svc.IsEmailAllowed(ctx, "friend@example.com"); err != nil || !ok {
		t.Errorf("allowlisted email: ok=%v err=%v, want true", ok, err)
	}
	if ok, err := svc.IsEmailAllowed(ctx, "root@example.com"); err != nil || !ok {
		t.Errorf("superadmin email: ok=%v err=%v, want true", ok, err)
	}
	if ok, err := svc.IsEmailAllowed(ctx, "stranger@example.com"); err != nil || ok {
		t.Errorf("stranger email: ok=%v err=%v, want false", ok, err)
	}

	if err := svc.AllowlistRemove(ctx, rootActor, "friend@example.com"); err != nil {
		t.Fatalf("AllowlistRemove: %v", err)
	}
	if ok, _ := svc.IsEmailAllowed(ctx, "friend@example.com"); ok {
		t.Error("removed email still allowed")
	}

	// --- Status management --------------------------------------------
	if err := svc.SetUserStatus(ctx, rootActor, root.ID, entuser.StatusDisabled); !errors.Is(err, ErrForbidden) {
		t.Errorf("superadmin self-disable err = %v, want forbidden (lockout guard)", err)
	}
	if err := svc.SetUserStatus(ctx, rootActor, alice.ID, entuser.StatusDisabled); err != nil {
		t.Errorf("disable member: %v", err)
	}

	// --- Superadmin overview -------------------------------------------
	users, err := svc.ListUsers(ctx, rootActor)
	if err != nil || len(users) != 3 {
		t.Errorf("ListUsers: %d users err=%v, want 3", len(users), err)
	}
}

func TestBootstrap_PromotionAndNoDemotion(t *testing.T) {
	// A member logs in before any superadmin config points at them.
	svc := setupService(t, "later-admin@example.com")
	ctx := context.Background()

	// Create a plain member first (different email).
	member, _, err := svc.EnsureUser(ctx, "member@example.com")
	if err != nil || member.Role != entuser.RoleMember {
		t.Fatalf("member setup: role=%v err=%v", member.Role, err)
	}

	// The configured email's first login: created directly as superadmin.
	admin, created, err := svc.EnsureUser(ctx, "later-admin@example.com")
	if err != nil || !created || admin.Role != entuser.RoleSuperadmin {
		t.Fatalf("admin bootstrap: created=%v role=%v err=%v", created, admin.Role, err)
	}

	// Simulate a config change to a different address: existing superadmin
	// must NOT be demoted, and the new email joins as a plain member
	// (promotion only applies when no superadmin exists).
	svc2, err := NewService(svc.store, Config{SuperadminEmail: "member@example.com"})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	m2, _, err := svc2.EnsureUser(ctx, "member@example.com")
	if err != nil {
		t.Fatalf("EnsureUser under new config: %v", err)
	}
	if m2.Role != entuser.RoleMember {
		t.Errorf("member was promoted despite an existing superadmin (role=%v)", m2.Role)
	}
	adminAfter, _, err := svc2.EnsureUser(ctx, "later-admin@example.com")
	if err != nil || adminAfter.Role != entuser.RoleSuperadmin {
		t.Errorf("existing superadmin demoted by config change: role=%v err=%v", adminAfter.Role, err)
	}
}

func TestBootstrap_PromoteWhenNoSuperadminExists(t *testing.T) {
	// Member exists, then config is set to their email: promote on login.
	svc := setupService(t, "") // no superadmin configured yet
	ctx := context.Background()

	member, _, err := svc.EnsureUser(ctx, "founder@example.com")
	if err != nil || member.Role != entuser.RoleMember {
		t.Fatalf("member setup: role=%v err=%v", member.Role, err)
	}

	svc2, err := NewService(svc.store, Config{SuperadminEmail: "founder@example.com"})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	promoted, created, err := svc2.EnsureUser(ctx, "founder@example.com")
	if err != nil || created {
		t.Fatalf("promotion login: created=%v err=%v", created, err)
	}
	if promoted.Role != entuser.RoleSuperadmin {
		t.Errorf("role = %v, want superadmin (promoted: config set after first login)", promoted.Role)
	}
}

// Compile-time guard: the ent user status/role enums the service relies on.
var (
	_ = entuser.StatusActive
	_ = ent.IsNotFound
)
