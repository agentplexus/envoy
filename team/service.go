// Package team is the library-first service layer for the multi-user
// ("team/family") system of record: users, roles, the allowlist, and —
// in later phases — chats and authentication.
//
// Authorization is enforced twice by design: the service checks the acting
// user before issuing queries (clear errors), and row-level security in
// PostgreSQL backstops every query regardless (defense in depth). Never
// infer authorization from ent errors alone: an RLS-filtered UPDATE can
// surface as a not-found rather than a denial.
package team

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/allowlistentry"
	entuser "github.com/plexusone/omniagent/team/ent/user"
	"github.com/plexusone/omniagent/team/store"
)

// Sentinel errors returned by the service layer.
var (
	// ErrForbidden is returned when the actor lacks permission.
	ErrForbidden = errors.New("forbidden")

	// ErrInvalidUsername is returned for usernames outside the allowed form.
	ErrInvalidUsername = errors.New("invalid username: use 3-32 characters of a-z, 0-9, '-' or '_', starting with a letter or digit")

	// ErrInvalidEmail is returned for unparseable email addresses.
	ErrInvalidEmail = errors.New("invalid email address")

	// ErrNotFound is returned when the referenced entity does not exist
	// (or is invisible to the actor — indistinguishable by design).
	ErrNotFound = errors.New("not found")
)

// usernamePattern constrains usernames (stored as citext, so effectively
// case-insensitive; the service lowercases before storing).
var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,31}$`)

// Actor identifies the authenticated caller of a service operation.
type Actor struct {
	UserID     uuid.UUID
	Superadmin bool
}

// Config configures the team service.
type Config struct {
	// SuperadminEmail bootstraps the superadmin: the first login by this
	// email creates (or, if no superadmin exists yet, promotes) the
	// superadmin user. Later config changes never demote an existing one.
	SuperadminEmail string

	// AgentHandle is the @-mention handle of the agent (default "omniagent").
	AgentHandle string

	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// Service exposes team operations over the RLS-scoped store.
type Service struct {
	store  *store.Store
	cfg    Config
	logger *slog.Logger
}

// NewService creates the team service.
func NewService(st *store.Store, cfg Config) (*Service, error) {
	if st == nil {
		return nil, fmt.Errorf("team: store is required")
	}
	if cfg.SuperadminEmail != "" {
		if _, err := mail.ParseAddress(cfg.SuperadminEmail); err != nil {
			return nil, fmt.Errorf("team: superadmin email: %w", ErrInvalidEmail)
		}
	}
	if cfg.AgentHandle == "" {
		cfg.AgentHandle = "omniagent"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Service{store: st, cfg: cfg, logger: cfg.Logger}, nil
}

// AgentHandle returns the agent's @-mention handle.
func (s *Service) AgentHandle() string {
	return s.cfg.AgentHandle
}

// GetSelf returns the actor's own user record.
func (s *Service) GetSelf(ctx context.Context, actor Actor) (*ent.User, error) {
	var u *ent.User
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		u, err = tx.User.Get(ctx, actor.UserID)
		return err
	})
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	return u, err
}

// ListUsers returns all users. Superadmin only.
func (s *Service) ListUsers(ctx context.Context, actor Actor) ([]*ent.User, error) {
	if !actor.Superadmin {
		return nil, fmt.Errorf("list users: %w", ErrForbidden)
	}
	var users []*ent.User
	err := s.store.AsUser(ctx, actor.UserID, true, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		users, err = tx.User.Query().Order(ent.Asc(entuser.FieldCreatedAt)).All(ctx)
		return err
	})
	return users, err
}

// RenameUser changes a user's username. Members may rename themselves;
// the superadmin may rename anyone (US-3 covers renaming themselves).
func (s *Service) RenameUser(ctx context.Context, actor Actor, userID uuid.UUID, username string) error {
	if userID != actor.UserID && !actor.Superadmin {
		return fmt.Errorf("rename user: %w", ErrForbidden)
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return ErrInvalidUsername
	}
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		_, err := tx.User.UpdateOneID(userID).SetUsername(username).Save(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return fmt.Errorf("rename user: %w", ErrNotFound)
	}
	return err
}

// SetDisplayName changes a user's display name (self, or superadmin).
func (s *Service) SetDisplayName(ctx context.Context, actor Actor, userID uuid.UUID, name string) error {
	if userID != actor.UserID && !actor.Superadmin {
		return fmt.Errorf("set display name: %w", ErrForbidden)
	}
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		_, err := tx.User.UpdateOneID(userID).SetDisplayName(strings.TrimSpace(name)).Save(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return fmt.Errorf("set display name: %w", ErrNotFound)
	}
	return err
}

// SetUserStatus enables or disables a user. Superadmin only; the superadmin
// cannot disable themselves (lockout guard).
func (s *Service) SetUserStatus(ctx context.Context, actor Actor, userID uuid.UUID, status entuser.Status) error {
	if !actor.Superadmin {
		return fmt.Errorf("set user status: %w", ErrForbidden)
	}
	if userID == actor.UserID && status == entuser.StatusDisabled {
		return fmt.Errorf("set user status: superadmin cannot disable themselves: %w", ErrForbidden)
	}
	err := s.store.AsUser(ctx, actor.UserID, true, func(ctx context.Context, tx *ent.Tx) error {
		_, err := tx.User.UpdateOneID(userID).SetStatus(status).Save(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return fmt.Errorf("set user status: %w", ErrNotFound)
	}
	return err
}

// AllowlistAdd approves an email for login. Superadmin only.
func (s *Service) AllowlistAdd(ctx context.Context, actor Actor, email, note string) (*ent.AllowlistEntry, error) {
	if !actor.Superadmin {
		return nil, fmt.Errorf("allowlist add: %w", ErrForbidden)
	}
	email, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}
	var entry *ent.AllowlistEntry
	err = s.store.AsUser(ctx, actor.UserID, true, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		entry, err = tx.AllowlistEntry.Create().
			SetEmail(email).
			SetAddedBy(actor.UserID).
			SetNote(note).
			Save(ctx)
		return err
	})
	if ent.IsConstraintError(err) {
		// Already allowlisted: return the existing entry (idempotent add).
		var existing *ent.AllowlistEntry
		gerr := s.store.AsUser(ctx, actor.UserID, true, func(ctx context.Context, tx *ent.Tx) error {
			var qerr error
			existing, qerr = tx.AllowlistEntry.Query().
				Where(allowlistentry.EmailEQ(email)).Only(ctx)
			return qerr
		})
		if gerr != nil {
			return nil, gerr
		}
		return existing, nil
	}
	return entry, err
}

// AllowlistRemove revokes an email's approval. Superadmin only. Removing an
// email does not delete an existing user; disable the user for that.
func (s *Service) AllowlistRemove(ctx context.Context, actor Actor, email string) error {
	if !actor.Superadmin {
		return fmt.Errorf("allowlist remove: %w", ErrForbidden)
	}
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	return s.store.AsUser(ctx, actor.UserID, true, func(ctx context.Context, tx *ent.Tx) error {
		_, err := tx.AllowlistEntry.Delete().
			Where(allowlistentry.EmailEQ(email)).Exec(ctx)
		return err
	})
}

// AllowlistList returns all allowlisted emails. Superadmin only.
func (s *Service) AllowlistList(ctx context.Context, actor Actor) ([]*ent.AllowlistEntry, error) {
	if !actor.Superadmin {
		return nil, fmt.Errorf("allowlist list: %w", ErrForbidden)
	}
	var entries []*ent.AllowlistEntry
	err := s.store.AsUser(ctx, actor.UserID, true, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		entries, err = tx.AllowlistEntry.Query().
			Order(ent.Asc(allowlistentry.FieldCreatedAt)).All(ctx)
		return err
	})
	return entries, err
}

// normalizeEmail validates and lowercases an email address.
func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}
	return email, nil
}
