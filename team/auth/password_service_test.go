package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/plexusone/omniagent/team"
	entuser "github.com/plexusone/omniagent/team/ent/user"
)

// makeMember allowlists and creates an active member, returning its actor.
func makeMember(t *testing.T, f *fixture, email string) team.Actor {
	t.Helper()
	ctx := context.Background()
	if _, err := f.team.AllowlistAdd(ctx, f.rootAct, email, ""); err != nil {
		t.Fatalf("AllowlistAdd: %v", err)
	}
	u, _, err := f.team.EnsureUser(ctx, email)
	if err != nil {
		t.Fatalf("EnsureUser: %v", err)
	}
	return team.Actor{UserID: u.ID, Superadmin: false}
}

func TestLoginWithPassword(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	member := makeMember(t, f, "member@example.com")

	// Superadmin sets the member's password.
	if err := f.svc.SetPassword(ctx, f.rootAct, member.UserID, "", "s3cret-passphrase"); err != nil {
		t.Fatalf("SetPassword (superadmin): %v", err)
	}

	// Correct credentials mint a session.
	tok, u, err := f.svc.LoginWithPassword(ctx, "member@example.com", "s3cret-passphrase", "ua", "1.2.3.4")
	if err != nil {
		t.Fatalf("LoginWithPassword: %v", err)
	}
	if tok == "" || u.ID != member.UserID {
		t.Fatalf("login returned tok=%q user=%v", tok, u)
	}

	// Wrong password and unknown email both return the same uniform error.
	if _, _, err := f.svc.LoginWithPassword(ctx, "member@example.com", "wrong", "", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password err = %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := f.svc.LoginWithPassword(ctx, "nobody@example.com", "whatever", "", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown email err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginWithPassword_NoPasswordSet(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	makeMember(t, f, "member@example.com") // no password set

	if _, _, err := f.svc.LoginWithPassword(ctx, "member@example.com", "anything-here", "", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("no-password login err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginWithPassword_Disabled(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	member := makeMember(t, f, "member@example.com")
	if err := f.svc.SetPassword(ctx, f.rootAct, member.UserID, "", "s3cret-passphrase"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if err := f.team.SetUserStatus(ctx, f.rootAct, member.UserID, entuser.StatusDisabled); err != nil {
		t.Fatalf("SetUserStatus: %v", err)
	}
	if _, _, err := f.svc.LoginWithPassword(ctx, "member@example.com", "s3cret-passphrase", "", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("disabled login err = %v, want ErrInvalidCredentials", err)
	}
}

func TestSetPassword_SelfService(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	member := makeMember(t, f, "member@example.com")

	// First-time self-set needs no current password.
	if err := f.svc.SetPassword(ctx, member, member.UserID, "", "first-passphrase"); err != nil {
		t.Fatalf("first self-set: %v", err)
	}
	// Changing it now requires the correct current password.
	if err := f.svc.SetPassword(ctx, member, member.UserID, "wrong", "second-passphrase"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("change with wrong current err = %v, want ErrInvalidCredentials", err)
	}
	if err := f.svc.SetPassword(ctx, member, member.UserID, "first-passphrase", "second-passphrase"); err != nil {
		t.Fatalf("change with correct current: %v", err)
	}
	if _, _, err := f.svc.LoginWithPassword(ctx, "member@example.com", "second-passphrase", "", ""); err != nil {
		t.Errorf("login after change: %v", err)
	}
}

func TestSetPassword_Forbidden(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a := makeMember(t, f, "a@example.com")
	b := makeMember(t, f, "b@example.com")

	// A member cannot set another member's password.
	if err := f.svc.SetPassword(ctx, a, b.UserID, "", "passphrase-xyz"); !errors.Is(err, team.ErrForbidden) {
		t.Errorf("cross-member set err = %v, want ErrForbidden", err)
	}
}

func TestSetPassword_Weak(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	member := makeMember(t, f, "member@example.com")
	if err := f.svc.SetPassword(ctx, member, member.UserID, "", "short"); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("weak password err = %v, want ErrWeakPassword", err)
	}
}
