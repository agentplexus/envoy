package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"github.com/plexusone/omniagent/team"
	"github.com/plexusone/omniagent/team/ent"
	entuser "github.com/plexusone/omniagent/team/ent/user"
)

// ErrWeakPassword is returned when a password fails the minimum-length policy.
var ErrWeakPassword = errors.New("password too short")

// ErrInvalidCredentials is returned for any password-login failure — unknown
// email, no password set, disabled account, or wrong password — deliberately
// indistinguishable so the endpoint cannot be used to enumerate accounts.
var ErrInvalidCredentials = errors.New("invalid email or password")

// MinPasswordLen is the minimum accepted password length.
const MinPasswordLen = 8

// argon2id parameters (OWASP-aligned defaults). Encoded into every hash (PHC
// format) so stored credentials remain verifiable if these are tuned later.
const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// hashPassword returns an argon2id PHC-encoded hash
// ($argon2id$v=19$m=...,t=...,p=...$salt$hash) for a plaintext password.
// It enforces MinPasswordLen.
func hashPassword(plain string) (string, error) {
	if len(plain) < MinPasswordLen {
		return "", ErrWeakPassword
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// verifyPassword reports whether plain matches an argon2id PHC-encoded hash.
// The comparison is constant-time. A malformed encoding returns false.
func verifyPassword(encoded, plain string) bool {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 || len(want) > 1024 {
		return false
	}
	//nolint:gosec // G115: len(want) is bounded to [1,1024] just above.
	got := argon2.IDKey([]byte(plain), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// LoginWithPassword verifies an email+password credential and, on success,
// opens a session (mirroring VerifyMagicLink's shape). Every failure —
// unknown email, no password set, disabled account, or a wrong password —
// returns ErrInvalidCredentials so callers cannot enumerate accounts. Login
// is inherently allowlist-gated: a password only exists on an already-created
// (therefore allowlisted) user.
func (s *Service) LoginWithPassword(ctx context.Context, email, plaintext, userAgent, ip string) (sessionToken string, u *ent.User, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || plaintext == "" {
		return "", nil, ErrInvalidCredentials
	}

	err = s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		found, qerr := tx.User.Query().Where(entuser.EmailEQ(email)).Only(ctx)
		if ent.IsNotFound(qerr) {
			return ErrInvalidCredentials
		}
		if qerr != nil {
			return qerr
		}
		u = found
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	if u.PasswordHash == "" || u.Status == entuser.StatusDisabled || !verifyPassword(u.PasswordHash, plaintext) {
		return "", nil, ErrInvalidCredentials
	}

	sessionToken, err = s.createSession(ctx, u.ID, userAgent, ip)
	if err != nil {
		return "", nil, err
	}
	return sessionToken, u, nil
}

// SetPassword sets or replaces a user's password. Authorization mirrors the
// team user mutations: a user may set their own (userID == actor.UserID), or a
// superadmin may set anyone's. When a user changes their own password and one
// is already set, the current password must be supplied and verified; a
// first-time self-set (no existing password, e.g. after magic-link) and any
// superadmin set skip that check. newPlain must meet MinPasswordLen.
func (s *Service) SetPassword(ctx context.Context, actor team.Actor, userID uuid.UUID, current, newPlain string) error {
	if userID != actor.UserID && !actor.Superadmin {
		return fmt.Errorf("set password: %w", team.ErrForbidden)
	}
	hash, err := hashPassword(newPlain)
	if err != nil {
		return err
	}

	return s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		target, qerr := tx.User.Get(ctx, userID)
		if ent.IsNotFound(qerr) {
			return team.ErrNotFound
		}
		if qerr != nil {
			return qerr
		}
		// Self-change with an existing password requires the current one.
		if userID == actor.UserID && !actor.Superadmin && target.PasswordHash != "" {
			if current == "" || !verifyPassword(target.PasswordHash, current) {
				return ErrInvalidCredentials
			}
		}
		_, uerr := tx.User.UpdateOneID(userID).SetPasswordHash(hash).Save(ctx)
		return uerr
	})
}
