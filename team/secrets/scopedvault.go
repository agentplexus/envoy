// Package secrets provides the team-mode secret store: an OmniVault-backed,
// multi-tenant vault whose per-agent namespaces let a runtime instance
// (RMI-OMNIAGENT-310) load only its own agent's secrets. OmniVault itself is a
// flat, path-keyed store with no native tenancy; ScopedVault adds that tenancy
// by path-namespacing, and Service turns an agent's namespace into the env map
// its instance injects into MCP subprocesses and secrets-aware skills.
//
// ScopedVault is a generic decorator intended to be upstreamed into omnivault;
// it lives here for now (RMI-310) to avoid a cross-repo release cycle.
package secrets

import (
	"context"
	"strings"

	"github.com/plexusone/omnivault/vault"
)

// ScopedVault confines a vault.Vault to a namespace by prefixing every path with
// "<namespace>/". It is how OmniVault — a flat, path-keyed store with no native
// tenancy — is made multi-tenant: a caller handed Scoped(v, "agents/A") cannot
// express a path outside "agents/A/", so two tenants over one backing store
// share no reachable keys. Isolation is structural, not policy-enforced.
type ScopedVault struct {
	inner vault.Vault
	ns    string // normalized, no surrounding slashes; "" means passthrough
}

// ScopedVault is a vault.Vault.
var _ vault.Vault = (*ScopedVault)(nil)

// Scoped returns inner confined to namespace ns. Surrounding slashes on ns are
// normalized away; an empty ns yields a passthrough (equivalent to inner).
func Scoped(inner vault.Vault, ns string) *ScopedVault {
	return &ScopedVault{inner: inner, ns: strings.Trim(ns, "/")}
}

// prefix returns the backing-key prefix for this namespace ("agents/A/"), or ""
// for a passthrough scope. The trailing slash is essential: it makes List match
// on a path boundary so "agents/A" never captures "agents/AB".
func (s *ScopedVault) prefix() string {
	if s.ns == "" {
		return ""
	}
	return s.ns + "/"
}

// key maps a namespace-relative path to its backing key.
func (s *ScopedVault) key(path string) string {
	return s.prefix() + path
}

// Get retrieves a secret at the namespace-relative path.
func (s *ScopedVault) Get(ctx context.Context, path string) (*vault.Secret, error) {
	return s.inner.Get(ctx, s.key(path))
}

// Set stores a secret at the namespace-relative path.
func (s *ScopedVault) Set(ctx context.Context, path string, secret *vault.Secret) error {
	return s.inner.Set(ctx, s.key(path), secret)
}

// Delete removes a secret at the namespace-relative path.
func (s *ScopedVault) Delete(ctx context.Context, path string) error {
	return s.inner.Delete(ctx, s.key(path))
}

// Exists reports whether a secret exists at the namespace-relative path.
func (s *ScopedVault) Exists(ctx context.Context, path string) (bool, error) {
	return s.inner.Exists(ctx, s.key(path))
}

// List returns the namespace-relative paths matching prefix. Backing keys
// outside the namespace are never returned — both because the query is scoped to
// "<ns>/<prefix>" and because any out-of-namespace key is defensively filtered.
func (s *ScopedVault) List(ctx context.Context, prefix string) ([]string, error) {
	full, err := s.inner.List(ctx, s.prefix()+prefix)
	if err != nil {
		return nil, err
	}
	nsPrefix := s.prefix()
	out := make([]string, 0, len(full))
	for _, k := range full {
		if nsPrefix == "" {
			out = append(out, k)
			continue
		}
		if !strings.HasPrefix(k, nsPrefix) {
			continue // backing store returned an out-of-namespace key
		}
		out = append(out, strings.TrimPrefix(k, nsPrefix))
	}
	return out, nil
}

// Name delegates to the backing provider.
func (s *ScopedVault) Name() string { return s.inner.Name() }

// Capabilities delegates to the backing provider.
func (s *ScopedVault) Capabilities() vault.Capabilities { return s.inner.Capabilities() }

// Close is a no-op: the backing vault is shared across scopes and owned by
// whoever constructed it.
func (s *ScopedVault) Close() error { return nil }
