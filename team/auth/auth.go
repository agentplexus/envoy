// Package auth implements passwordless, allowlist-closed authentication for
// team mode: magic-link issuance/verification and server-side cookie
// sessions. It is a library — no HTTP types cross its boundary; the gateway
// adapts it to endpoints.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team"
	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/authsession"
	"github.com/plexusone/omniagent/team/ent/magiclinktoken"
	entuser "github.com/plexusone/omniagent/team/ent/user"
	"github.com/plexusone/omniagent/team/mail"
	"github.com/plexusone/omniagent/team/store"
)

// Defaults for token and session lifetimes.
const (
	DefaultTokenTTL   = 15 * time.Minute
	DefaultSessionTTL = 30 * 24 * time.Hour
	// sessionRenewWindow: only slide the session expiry when at least this
	// much time has passed since last_seen, to avoid a write per request.
	sessionRenewWindow = time.Hour
)

// ErrInvalidToken is returned for any magic-link failure (unknown, expired,
// or already consumed) — deliberately indistinguishable.
var ErrInvalidToken = errors.New("invalid or expired token")

// ErrInvalidSession is returned for an unknown or expired session.
var ErrInvalidSession = errors.New("invalid or expired session")

// Principal is the authenticated identity resolved from a session.
type Principal struct {
	UserID     uuid.UUID
	Username   string
	Email      string
	Superadmin bool
	Disabled   bool
}

// Config configures the auth service.
type Config struct {
	// BaseURL is the externally visible origin; magic links are built as
	// BaseURL + "/api/auth/verify?token=...".
	BaseURL string
	// AppName is shown in the login email.
	AppName string
	// TokenTTL is the magic-link lifetime (default 15m).
	TokenTTL time.Duration
	// SessionTTL is the cookie session lifetime (default 30d).
	SessionTTL time.Duration
	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

func (c *Config) setDefaults() {
	if c.TokenTTL <= 0 {
		c.TokenTTL = DefaultTokenTTL
	}
	if c.SessionTTL <= 0 {
		c.SessionTTL = DefaultSessionTTL
	}
	if c.AppName == "" {
		c.AppName = "OmniAgent"
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
}

// Service performs authentication over the team store.
type Service struct {
	store  *store.Store
	team   *team.Service
	mailer mail.Mailer
	cfg    Config
	logger *slog.Logger
}

// NewService creates the auth service.
func NewService(st *store.Store, teamSvc *team.Service, mailer mail.Mailer, cfg Config) (*Service, error) {
	if st == nil || teamSvc == nil || mailer == nil {
		return nil, fmt.Errorf("auth: store, team service, and mailer are required")
	}
	cfg.setDefaults()
	return &Service{store: st, team: teamSvc, mailer: mailer, cfg: cfg, logger: cfg.Logger}, nil
}

// RequestMagicLink issues and emails a login link when the email is allowed.
//
// The response is uniform by contract: this returns nil for both allowed and
// non-allowed emails (doing nothing in the latter case), so callers cannot
// enumerate the allowlist. Only malformed input yields an error. Internal
// failures (token store, mail delivery) are logged, not surfaced — the
// operator sees them, the requester does not.
func (s *Service) RequestMagicLink(ctx context.Context, email, clientIP string) error {
	allowed, err := s.team.IsEmailAllowed(ctx, email)
	if err != nil {
		if errors.Is(err, team.ErrInvalidEmail) {
			return team.ErrInvalidEmail
		}
		s.logger.Error("magic link: allowlist check failed", "error", err)
		return nil // uniform: do not leak infrastructure state
	}
	if !allowed {
		s.logger.Info("magic link requested for non-allowlisted email (ignored)")
		return nil
	}

	raw, hash, err := newToken()
	if err != nil {
		s.logger.Error("magic link: token generation failed", "error", err)
		return nil
	}
	expires := time.Now().Add(s.cfg.TokenTTL)

	if err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		_, cerr := tx.MagicLinkToken.Create().
			SetEmail(email).
			SetTokenHash(hash).
			SetExpiresAt(expires).
			SetCreatedIP(clientIP).
			Save(ctx)
		return cerr
	}); err != nil {
		s.logger.Error("magic link: store token failed", "error", err)
		return nil
	}

	link := fmt.Sprintf("%s/api/auth/verify?token=%s", s.cfg.BaseURL, raw)
	msg := mail.MagicLinkMessage(email, mail.MagicLinkData{
		AppName: s.cfg.AppName,
		Link:    link,
		TTL:     s.cfg.TokenTTL,
	})
	if err := s.mailer.Send(ctx, msg); err != nil {
		s.logger.Error("magic link: send failed", "error", err)
		return nil
	}
	s.logger.Info("magic link sent", "expires_at", expires)
	return nil
}

// VerifyMagicLink consumes a token and returns a new session's raw token
// (to be set as the cookie) plus the resolved user. Any failure returns
// ErrInvalidToken. A disabled user is rejected.
func (s *Service) VerifyMagicLink(ctx context.Context, rawToken string) (sessionToken string, u *ent.User, err error) {
	if rawToken == "" {
		return "", nil, ErrInvalidToken
	}
	hash := hashToken(rawToken)

	var email string
	err = s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		tok, qerr := tx.MagicLinkToken.Query().
			Where(magiclinktoken.TokenHashEQ(hash)).Only(ctx)
		if ent.IsNotFound(qerr) {
			return ErrInvalidToken
		}
		if qerr != nil {
			return qerr
		}
		if tok.ConsumedAt != nil || time.Now().After(tok.ExpiresAt) {
			return ErrInvalidToken
		}
		// Consume atomically: the update is guarded on consumed_at being
		// null so a concurrent second use affects zero rows.
		n, uerr := tx.MagicLinkToken.Update().
			Where(
				magiclinktoken.IDEQ(tok.ID),
				magiclinktoken.ConsumedAtIsNil(),
			).
			SetConsumedAt(time.Now()).
			Save(ctx)
		if uerr != nil {
			return uerr
		}
		if n == 0 {
			return ErrInvalidToken // lost the race
		}
		email = tok.Email
		return nil
	})
	if err != nil {
		return "", nil, err
	}

	// First-login user creation / superadmin bootstrap.
	u, _, err = s.team.EnsureUser(ctx, email)
	if err != nil {
		return "", nil, fmt.Errorf("ensure user: %w", err)
	}
	if u.Status == entuser.StatusDisabled {
		return "", nil, ErrInvalidToken
	}

	sessionToken, err = s.createSession(ctx, u.ID, "", "")
	if err != nil {
		return "", nil, err
	}
	return sessionToken, u, nil
}

// createSession mints a server-side session and returns its raw token.
func (s *Service) createSession(ctx context.Context, userID uuid.UUID, userAgent, ip string) (string, error) {
	raw, hash, err := newToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(s.cfg.SessionTTL)
	if err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		_, cerr := tx.AuthSession.Create().
			SetUserID(userID).
			SetTokenHash(hash).
			SetExpiresAt(expires).
			SetUserAgent(userAgent).
			SetCreatedIP(ip).
			Save(ctx)
		return cerr
	}); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return raw, nil
}

// Authenticate resolves a session cookie token to a Principal, sliding the
// expiry when it is stale. Returns ErrInvalidSession for unknown/expired
// sessions or disabled users.
func (s *Service) Authenticate(ctx context.Context, rawSessionToken string) (*Principal, error) {
	if rawSessionToken == "" {
		return nil, ErrInvalidSession
	}
	hash := hashToken(rawSessionToken)

	var p *Principal
	err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		sess, qerr := tx.AuthSession.Query().
			Where(authsession.TokenHashEQ(hash)).Only(ctx)
		if ent.IsNotFound(qerr) {
			return ErrInvalidSession
		}
		if qerr != nil {
			return qerr
		}
		if time.Now().After(sess.ExpiresAt) {
			// Opportunistically delete the expired row.
			_ = tx.AuthSession.DeleteOneID(sess.ID).Exec(ctx) //nolint:errcheck // best-effort cleanup
			return ErrInvalidSession
		}

		usr, uerr := tx.User.Get(ctx, sess.UserID)
		if uerr != nil {
			return ErrInvalidSession
		}
		if usr.Status == entuser.StatusDisabled {
			return ErrInvalidSession
		}

		// Slide the expiry, but only when stale enough to matter.
		if time.Since(sess.LastSeenAt) > sessionRenewWindow {
			if _, rerr := tx.AuthSession.UpdateOneID(sess.ID).
				SetLastSeenAt(time.Now()).
				SetExpiresAt(time.Now().Add(s.cfg.SessionTTL)).
				Save(ctx); rerr != nil {
				return rerr
			}
		}

		p = &Principal{
			UserID:     usr.ID,
			Username:   usr.Username,
			Email:      usr.Email,
			Superadmin: usr.Role == entuser.RoleSuperadmin,
			Disabled:   usr.Status == entuser.StatusDisabled,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Logout revokes the session identified by its raw cookie token. Unknown
// tokens are a no-op (idempotent logout).
func (s *Service) Logout(ctx context.Context, rawSessionToken string) error {
	if rawSessionToken == "" {
		return nil
	}
	hash := hashToken(rawSessionToken)
	return s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		_, derr := tx.AuthSession.Delete().
			Where(authsession.TokenHashEQ(hash)).Exec(ctx)
		return derr
	})
}

// Actor converts a Principal into a team.Actor for service calls.
func (p *Principal) Actor() team.Actor {
	return team.Actor{UserID: p.UserID, Superadmin: p.Superadmin}
}
