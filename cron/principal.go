package cron

import (
	"context"
	"strings"
)

// sessionPrincipalPrefix namespaces principals derived from agent sessions.
const sessionPrincipalPrefix = "session:"

// principalContextKey is the context key for the authorizing principal.
type principalContextKey struct{}

// ContextWithPrincipal returns a context carrying the authorizing principal
// for work performed on behalf of a caller (e.g. "session:<id>"). Job
// creation reads it to stamp Job.OwnerPrincipal — principals travel via
// context, never via caller-supplied tool parameters, so they cannot be
// spoofed through the cron tool surface.
func ContextWithPrincipal(ctx context.Context, principal string) context.Context {
	if principal == "" {
		return ctx
	}
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext returns the authorizing principal carried by the
// context, or empty when none is set.
func PrincipalFromContext(ctx context.Context) string {
	principal, _ := ctx.Value(principalContextKey{}).(string)
	return principal
}

// SessionPrincipal builds the principal string for an agent session.
func SessionPrincipal(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	return sessionPrincipalPrefix + sessionID
}

// SessionIDFromPrincipal extracts the session ID from a session principal.
// Returns false for principals of any other form.
func SessionIDFromPrincipal(principal string) (string, bool) {
	id, ok := strings.CutPrefix(principal, sessionPrincipalPrefix)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}
