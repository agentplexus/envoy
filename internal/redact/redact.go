// Package redact masks resolved secret values out of log output.
//
// Skill secrets (RMI-OMNIAGENT-200/201/202) are resolved once from a vault
// and then threaded into MCP subprocess environments, OpenAPI auth headers,
// and compiled-skill env maps (RMI-OMNIAGENT-203/209/210). None of those
// paths intentionally log the resolved value, but nothing previously stopped
// a future debug line — or a naive %v/%+v dump of a skill's internal config —
// from leaking one. Register call sites register each resolved value here;
// NewHandler wraps a slog.Handler so any log record containing a registered
// value has it masked before it reaches the underlying handler.
package redact

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// minSecretLen is the shortest value Register will track. Short values (e.g.
// "1" or "ok") are common as ordinary log content; redacting them would mask
// unrelated log output rather than protect a secret.
const minSecretLen = 4

const mask = "[REDACTED]"

var (
	mu     sync.RWMutex
	values = map[string]struct{}{}
)

// Register adds resolved secret values to the redaction set. Empty values and
// values shorter than minSecretLen are ignored. Safe for concurrent use.
func Register(vals ...string) {
	mu.Lock()
	defer mu.Unlock()
	for _, v := range vals {
		if len(v) < minSecretLen {
			continue
		}
		values[v] = struct{}{}
	}
}

// Reset clears the registry. Test-only — production call sites only ever add
// values as secrets are resolved.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	values = map[string]struct{}{}
}

// String replaces every occurrence of a registered value in s with a mask.
func String(s string) string {
	mu.RLock()
	defer mu.RUnlock()
	for v := range values {
		if strings.Contains(s, v) {
			s = strings.ReplaceAll(s, v, mask)
		}
	}
	return s
}

// Handler wraps an slog.Handler, masking registered secret values out of the
// message and every attribute (including nested groups) of each record.
type Handler struct {
	next slog.Handler
}

// NewHandler wraps next so records it handles have registered secret values
// masked first.
func NewHandler(next slog.Handler) slog.Handler {
	return &Handler{next: next}
}

// Enabled implements slog.Handler.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle implements slog.Handler.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, String(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(redactAttr(a))
		return true
	})
	return h.next.Handle(ctx, nr)
}

// WithAttrs implements slog.Handler.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redactAttr(a)
	}
	return &Handler{next: h.next.WithAttrs(out)}
}

// WithGroup implements slog.Handler.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{next: h.next.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	a.Value = redactValue(a.Value)
	return a
}

func redactValue(v slog.Value) slog.Value {
	switch v.Kind() {
	case slog.KindString:
		return slog.StringValue(String(v.String()))
	case slog.KindGroup:
		g := v.Group()
		out := make([]slog.Attr, len(g))
		for i, a := range g {
			out[i] = redactAttr(a)
		}
		return slog.GroupValue(out...)
	case slog.KindAny:
		// A %v/%+v dump of a struct holding a secret (e.g. a skill's config)
		// or an error whose text embeds one are the realistic accidental-leak
		// paths — stringify and mask those, leave other Any values untouched.
		switch av := v.Any().(type) {
		case error:
			return slog.StringValue(String(av.Error()))
		case interface{ String() string }:
			return slog.StringValue(String(av.String()))
		}
		return v
	default:
		return v
	}
}
