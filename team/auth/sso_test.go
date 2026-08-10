package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/identity"
	entuser "github.com/plexusone/omniagent/team/ent/user"
)

func TestCompleteSSOLogin_NewIdentityNewUser(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.team.AllowlistAdd(ctx, f.rootAct, "member@example.com", ""); err != nil {
		t.Fatalf("AllowlistAdd: %v", err)
	}

	sessionToken, u, err := f.svc.CompleteSSOLogin(ctx, identity.ProviderGoogle, "google-sub-1", "member@example.com", "ua", "1.2.3.4")
	if err != nil {
		t.Fatalf("CompleteSSOLogin: %v", err)
	}
	if sessionToken == "" {
		t.Error("empty session token")
	}
	if u.Email != "member@example.com" {
		t.Errorf("user email = %q, want member@example.com", u.Email)
	}
}

func TestCompleteSSOLogin_LandsInExistingMagicLinkAccount(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.team.AllowlistAdd(ctx, f.rootAct, "member@example.com", ""); err != nil {
		t.Fatalf("AllowlistAdd: %v", err)
	}
	// First login is via magic link.
	if err := f.svc.RequestMagicLink(ctx, "member@example.com", "1.2.3.4"); err != nil {
		t.Fatalf("RequestMagicLink: %v", err)
	}
	_, magicUser, err := f.svc.VerifyMagicLink(ctx, f.linkToken(t))
	if err != nil {
		t.Fatalf("VerifyMagicLink: %v", err)
	}

	// Later, the same person signs in via Google with the same verified
	// email — must land in the same account.
	_, ssoUser, err := f.svc.CompleteSSOLogin(ctx, identity.ProviderGoogle, "google-sub-1", "member@example.com", "", "")
	if err != nil {
		t.Fatalf("CompleteSSOLogin: %v", err)
	}
	if ssoUser.ID != magicUser.ID {
		t.Errorf("SSO login user ID = %v, want %v (same account as magic-link)", ssoUser.ID, magicUser.ID)
	}
}

func TestCompleteSSOLogin_NonAllowlistedRejectedNoRowsCreated(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	before := countUsers(t, f)

	_, _, err := f.svc.CompleteSSOLogin(ctx, identity.ProviderGoogle, "google-sub-2", "stranger@example.com", "", "")
	if !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("err = %v, want ErrNotAllowed", err)
	}

	after := countUsers(t, f)
	if after != before {
		t.Errorf("user count changed from %d to %d; non-allowlisted SSO must create nothing", before, after)
	}

	u, ferr := f.svc.findBySSOIdentity(ctx, identity.ProviderGoogle, "google-sub-2")
	if ferr != nil {
		t.Fatalf("findBySSOIdentity: %v", ferr)
	}
	if u != nil {
		t.Error("identity row was created for a rejected non-allowlisted login")
	}
}

func TestCompleteSSOLogin_DisabledUserRejected(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.team.AllowlistAdd(ctx, f.rootAct, "member@example.com", ""); err != nil {
		t.Fatalf("AllowlistAdd: %v", err)
	}
	_, u, err := f.svc.CompleteSSOLogin(ctx, identity.ProviderGoogle, "google-sub-3", "member@example.com", "", "")
	if err != nil {
		t.Fatalf("CompleteSSOLogin (first login): %v", err)
	}
	if err := f.team.SetUserStatus(ctx, f.rootAct, u.ID, entuser.StatusDisabled); err != nil {
		t.Fatalf("SetUserStatus: %v", err)
	}

	_, _, err = f.svc.CompleteSSOLogin(ctx, identity.ProviderGoogle, "google-sub-3", "member@example.com", "", "")
	if !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("err = %v, want ErrAccountDisabled", err)
	}
}

func TestCompleteSSOLogin_IdempotentReLogin(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.team.AllowlistAdd(ctx, f.rootAct, "member@example.com", ""); err != nil {
		t.Fatalf("AllowlistAdd: %v", err)
	}
	_, u1, err := f.svc.CompleteSSOLogin(ctx, identity.ProviderGoogle, "google-sub-4", "member@example.com", "", "")
	if err != nil {
		t.Fatalf("CompleteSSOLogin (1st): %v", err)
	}
	_, u2, err := f.svc.CompleteSSOLogin(ctx, identity.ProviderGoogle, "google-sub-4", "member@example.com", "", "")
	if err != nil {
		t.Fatalf("CompleteSSOLogin (2nd): %v", err)
	}
	if u1.ID != u2.ID {
		t.Errorf("re-login resolved to a different user: %v vs %v", u1.ID, u2.ID)
	}

	var count int
	err = f.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		var qerr error
		count, qerr = tx.Identity.Query().
			Where(identity.ProviderEQ(identity.ProviderGoogle), identity.ProviderSubjectEQ("google-sub-4")).
			Count(ctx)
		return qerr
	})
	if err != nil {
		t.Fatalf("count identities: %v", err)
	}
	if count != 1 {
		t.Errorf("identity row count = %d, want exactly 1 after two logins", count)
	}
}

func TestCompleteSSOLogin_TwoProvidersOneAccount(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.team.AllowlistAdd(ctx, f.rootAct, "member@example.com", ""); err != nil {
		t.Fatalf("AllowlistAdd: %v", err)
	}
	_, uGoogle, err := f.svc.CompleteSSOLogin(ctx, identity.ProviderGoogle, "google-sub-5", "member@example.com", "", "")
	if err != nil {
		t.Fatalf("CompleteSSOLogin (google): %v", err)
	}
	_, uGithub, err := f.svc.CompleteSSOLogin(ctx, identity.ProviderGithub, "12345", "member@example.com", "", "")
	if err != nil {
		t.Fatalf("CompleteSSOLogin (github): %v", err)
	}
	if uGoogle.ID != uGithub.ID {
		t.Fatalf("google and github logins resolved to different users: %v vs %v", uGoogle.ID, uGithub.ID)
	}

	var count int
	err = f.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		var qerr error
		count, qerr = tx.Identity.Query().Where(identity.UserIDEQ(uGoogle.ID)).Count(ctx)
		return qerr
	})
	if err != nil {
		t.Fatalf("count identities: %v", err)
	}
	if count != 2 {
		t.Errorf("identity row count for the user = %d, want 2 (google + github)", count)
	}
}

func TestVerifyMagicLink_BackfillsIdentity(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.team.AllowlistAdd(ctx, f.rootAct, "member@example.com", ""); err != nil {
		t.Fatalf("AllowlistAdd: %v", err)
	}
	if err := f.svc.RequestMagicLink(ctx, "member@example.com", "1.2.3.4"); err != nil {
		t.Fatalf("RequestMagicLink: %v", err)
	}
	_, u, err := f.svc.VerifyMagicLink(ctx, f.linkToken(t))
	if err != nil {
		t.Fatalf("VerifyMagicLink: %v", err)
	}

	var count int
	err = f.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		var qerr error
		count, qerr = tx.Identity.Query().
			Where(identity.UserIDEQ(u.ID), identity.ProviderEQ(identity.ProviderMagicLink)).
			Count(ctx)
		return qerr
	})
	if err != nil {
		t.Fatalf("count identities: %v", err)
	}
	if count != 1 {
		t.Errorf("magic_link identity row count = %d, want 1", count)
	}
}

func countUsers(t *testing.T, f *fixture) int {
	t.Helper()
	users, err := f.team.ListUsers(context.Background(), f.rootAct)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	return len(users)
}
