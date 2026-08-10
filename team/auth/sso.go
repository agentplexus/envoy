package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/identity"
	entuser "github.com/plexusone/omniagent/team/ent/user"
)

// ErrNotAllowed is returned when an SSO login resolves a verified email that
// is not allowlisted. No user or identity row is created — SSO never
// bypasses the allowlist.
var ErrNotAllowed = errors.New("email not allowed")

// ErrAccountDisabled is returned when an SSO login resolves to a disabled
// user.
var ErrAccountDisabled = errors.New("account is disabled")

// CompleteSSOLogin resolves a provider's verified identity to a user and
// opens a session, mirroring VerifyMagicLink's session-issuance shape.
// subject is the provider's stable id (OIDC sub / GitHub numeric account
// id); verifiedEmail must already be provider-verified — this trusts it.
//
// A returning identity (provider, subject already linked) logs into its
// user directly, with no allowlist re-check — they are already a member.
// A first-time identity is only linked after IsEmailAllowed passes; a
// non-allowlisted email is rejected before any user or identity row is
// created (ErrNotAllowed). Because linking resolves the user by email via
// team.Service.EnsureUser, a magic-link user who later signs in via SSO
// with the same email lands in their existing account.
func (s *Service) CompleteSSOLogin(ctx context.Context, provider identity.Provider, subject, verifiedEmail, userAgent, ip string) (sessionToken string, u *ent.User, err error) {
	u, err = s.findBySSOIdentity(ctx, provider, subject)
	if err != nil {
		return "", nil, err
	}
	if u == nil {
		allowed, aerr := s.team.IsEmailAllowed(ctx, verifiedEmail)
		if aerr != nil {
			return "", nil, fmt.Errorf("check allowlist: %w", aerr)
		}
		if !allowed {
			return "", nil, ErrNotAllowed
		}
		u, _, err = s.team.EnsureUser(ctx, verifiedEmail)
		if err != nil {
			return "", nil, fmt.Errorf("ensure user: %w", err)
		}
		if err := s.linkIdentity(ctx, u.ID, provider, subject, u.Email); err != nil {
			return "", nil, fmt.Errorf("link identity: %w", err)
		}
	}
	if u.Status == entuser.StatusDisabled {
		return "", nil, ErrAccountDisabled
	}

	sessionToken, err = s.createSession(ctx, u.ID, userAgent, ip)
	if err != nil {
		return "", nil, err
	}
	return sessionToken, u, nil
}

// findBySSOIdentity looks up the user linked to (provider, subject), or
// returns (nil, nil) when no such identity is linked yet.
func (s *Service) findBySSOIdentity(ctx context.Context, provider identity.Provider, subject string) (*ent.User, error) {
	var userID uuid.UUID
	found := false
	err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		id, qerr := tx.Identity.Query().
			Where(identity.ProviderEQ(provider), identity.ProviderSubjectEQ(subject)).
			Only(ctx)
		if ent.IsNotFound(qerr) {
			return nil
		}
		if qerr != nil {
			return qerr
		}
		userID = id.UserID
		found = true
		return nil
	})
	if err != nil || !found {
		return nil, err
	}

	var u *ent.User
	err = s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		var uerr error
		u, uerr = tx.User.Get(ctx, userID)
		return uerr
	})
	if ent.IsNotFound(err) {
		// The user row is gone but the identity row survived somehow; treat
		// as no linked identity rather than surfacing a confusing error.
		return nil, nil
	}
	return u, err
}

// linkIdentity idempotently inserts an Identity row: a concurrent duplicate
// insert on the unique (provider, provider_subject) index is swallowed,
// mirroring team.Service.AllowlistAdd's idempotent-add pattern.
func (s *Service) linkIdentity(ctx context.Context, userID uuid.UUID, provider identity.Provider, subject, verifiedEmail string) error {
	err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		_, cerr := tx.Identity.Create().
			SetUserID(userID).
			SetProvider(provider).
			SetProviderSubject(subject).
			SetVerifiedEmail(verifiedEmail).
			Save(ctx)
		return cerr
	})
	if ent.IsConstraintError(err) {
		return nil
	}
	return err
}
