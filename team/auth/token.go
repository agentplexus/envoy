package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// tokenBytes is the entropy of magic-link and session tokens (256 bits).
const tokenBytes = 32

// newToken returns a URL-safe random token and its SHA-256 hash (hex).
// Only the hash is ever stored; the raw token lives in the link/cookie.
func newToken() (raw, hash string, err error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

// hashToken hashes a raw token for storage and lookup. Lookups are by the
// unique hash column, so a match is exact; there is no separate secret to
// compare in variable time.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
