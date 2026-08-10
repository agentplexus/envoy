// Package memscope carries a memory scope — the tenant and subject a turn's
// memory operations are attributed to — through a context.Context.
//
// It exists so a caller that owns the scoping decision (e.g. the team chats
// service, which knows a turn runs on behalf of a specific chat) can stamp the
// scope onto the turn context, and the consumers of that scope (the agent's
// automatic recall/rollover memory and the memory skill's tools) can honor it
// without either side depending on the other. This decouples "where a memory
// belongs" from "who runs the turn": the agent needs no knowledge of chats, and
// chats needs no knowledge of the agent's memory internals.
//
// A chat turn is scoped TenantID=<team tenant>, SubjectID="chat:<id>" so a
// chat's memories are isolated to that chat (RMI-OMNIAGENT-114). When no scope
// is set (personal CLI, non-chat turns), consumers fall back to their own
// configuration, so behavior is unchanged.
package memscope

import "context"

// Scope identifies the tenant and subject a turn's memory operations belong to.
// A zero-valued field means "unset" — a consumer keeps its own default for that
// field, so a scope may override the tenant, the subject, or both.
type Scope struct {
	// TenantID is the memory tenant (e.g. "team"). Empty leaves the
	// consumer's configured tenant in place.
	TenantID string
	// SubjectID is who/what the memory is about (e.g. "chat:<id>"). Empty
	// leaves the consumer's default subject in place.
	SubjectID string
}

// scopeContextKey is the context key for the memory scope.
type scopeContextKey struct{}

// NewContext returns a context carrying scope. An entirely empty scope is a
// no-op — it returns ctx unchanged so consumers still resolve their defaults.
func NewContext(ctx context.Context, scope Scope) context.Context {
	if scope.TenantID == "" && scope.SubjectID == "" {
		return ctx
	}
	return context.WithValue(ctx, scopeContextKey{}, scope)
}

// FromContext returns the memory scope carried by ctx and whether one was set.
func FromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(Scope)
	return scope, ok
}

// Resolve applies any scope carried by ctx on top of the given tenant/subject
// defaults, returning the effective tenant and subject. A scope field overrides
// its default only when non-empty, so an unset field preserves the default.
func Resolve(ctx context.Context, tenantID, subjectID string) (string, string) {
	scope, ok := FromContext(ctx)
	if !ok {
		return tenantID, subjectID
	}
	if scope.TenantID != "" {
		tenantID = scope.TenantID
	}
	if scope.SubjectID != "" {
		subjectID = scope.SubjectID
	}
	return tenantID, subjectID
}
