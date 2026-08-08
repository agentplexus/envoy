package team

import (
	"context"
	"fmt"
	"strings"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/allowlistentry"
	entuser "github.com/plexusone/omniagent/team/ent/user"
)

// IsEmailAllowed reports whether an email may log in: it is allowlisted or
// is the configured superadmin email. Runs in the system context — it is
// consulted by the auth layer before any user exists.
func (s *Service) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return false, err
	}
	if s.cfg.SuperadminEmail != "" && strings.EqualFold(email, s.cfg.SuperadminEmail) {
		return true, nil
	}
	var allowed bool
	err = s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		var qerr error
		allowed, qerr = tx.AllowlistEntry.Query().
			Where(allowlistentry.EmailEQ(email)).Exist(ctx)
		return qerr
	})
	return allowed, err
}

// EnsureUser returns the user for a verified email, creating it on first
// login. Superadmin bootstrap happens here:
//
//   - A newly created user whose email matches the configured superadmin
//     email gets the superadmin role.
//   - An existing member with that email is promoted only when no
//     superadmin exists yet (config set after their first login).
//   - Changing the configured email later never demotes an existing
//     superadmin — demotion is an explicit administrative act, not a
//     config side effect.
//
// The caller (auth layer) must have verified both the email and its
// allowlist status; EnsureUser does not re-check the allowlist.
func (s *Service) EnsureUser(ctx context.Context, email string) (u *ent.User, created bool, err error) {
	email, err = normalizeEmail(email)
	if err != nil {
		return nil, false, err
	}
	isBootstrapEmail := s.cfg.SuperadminEmail != "" && strings.EqualFold(email, s.cfg.SuperadminEmail)

	err = s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		existing, qerr := tx.User.Query().Where(entuser.EmailEQ(email)).Only(ctx)
		switch {
		case qerr == nil:
			u = existing
			if isBootstrapEmail && existing.Role != entuser.RoleSuperadmin {
				n, cerr := tx.User.Query().
					Where(entuser.RoleEQ(entuser.RoleSuperadmin)).Count(ctx)
				if cerr != nil {
					return cerr
				}
				if n == 0 {
					u, cerr = tx.User.UpdateOneID(existing.ID).
						SetRole(entuser.RoleSuperadmin).Save(ctx)
					if cerr != nil {
						return cerr
					}
					s.logger.Info("promoted existing user to superadmin (bootstrap)",
						"user_id", u.ID, "email", email)
				}
			}
			return nil

		case ent.IsNotFound(qerr):
			username, uerr := s.uniqueUsername(ctx, tx, email)
			if uerr != nil {
				return uerr
			}
			role := entuser.RoleMember
			if isBootstrapEmail {
				role = entuser.RoleSuperadmin
			}
			u, uerr = tx.User.Create().
				SetEmail(email).
				SetUsername(username).
				SetRole(role).
				Save(ctx)
			if uerr != nil {
				return uerr
			}
			created = true
			s.logger.Info("created user on first login",
				"user_id", u.ID, "username", username, "role", role)
			return nil

		default:
			return qerr
		}
	})
	if err != nil {
		return nil, false, err
	}
	return u, created, nil
}

// uniqueUsername derives a username from the email local part, sanitized to
// the allowed form and uniquified with a numeric suffix on collision.
func (s *Service) uniqueUsername(ctx context.Context, tx *ent.Tx, email string) (string, error) {
	base := sanitizeUsername(strings.SplitN(email, "@", 2)[0])

	candidate := base
	for i := 2; ; i++ {
		exists, err := tx.User.Query().
			Where(entuser.UsernameEQ(candidate)).Exist(ctx)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
		if i > 1000 {
			return "", fmt.Errorf("could not derive a unique username for %q", email)
		}
	}
}

// sanitizeUsername lowercases and strips the local part down to the allowed
// username alphabet, guaranteeing a valid non-empty result.
func sanitizeUsername(local string) string {
	local = strings.ToLower(local)
	var b strings.Builder
	for _, r := range local {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == '.', r == '+':
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-_")
	if len(name) < 3 {
		name = "user-" + name
		name = strings.TrimSuffix(name, "-")
	}
	if len(name) > 32 {
		name = name[:32]
	}
	return name
}
