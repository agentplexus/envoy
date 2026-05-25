// Package config provides configuration types and loading for omniagent.
package config

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/plexusone/omnivault"
	"github.com/plexusone/omnivault/vault"
)

// vaultSchemes are the URI schemes that indicate a vault-backed credential.
var vaultSchemes = []string{"op", "bw", "keeper", "file", "env"}

// credentialResolver caches vault providers by scheme for credential resolution.
type credentialResolver struct {
	mu     sync.RWMutex
	vaults map[string]vault.Vault
}

// newCredentialResolver creates a new credential resolver.
func newCredentialResolver() *credentialResolver {
	return &credentialResolver{
		vaults: make(map[string]vault.Vault),
	}
}

// getVault returns a cached vault for the scheme, or creates one.
func (r *credentialResolver) getVault(scheme string) (vault.Vault, error) {
	r.mu.RLock()
	v, ok := r.vaults[scheme]
	r.mu.RUnlock()
	if ok {
		return v, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if v, ok := r.vaults[scheme]; ok {
		return v, nil
	}

	// Create vault with just the scheme (no path)
	v, err := omnivault.VaultFromURI(scheme + "://")
	if err != nil {
		return nil, err
	}
	r.vaults[scheme] = v
	return v, nil
}

// close closes all cached vaults.
func (r *credentialResolver) close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for _, v := range r.vaults {
		if err := v.Close(); err != nil {
			lastErr = err
		}
	}
	r.vaults = make(map[string]vault.Vault)
	return lastErr
}

// isVaultURI returns true if the value is a vault URI.
func isVaultURI(s string) bool {
	for _, scheme := range vaultSchemes {
		if strings.HasPrefix(s, scheme+"://") {
			return true
		}
	}
	return false
}

// isUnknownVaultURI returns true if the value looks like a vault URI
// but uses an unknown scheme (e.g., "invalid://path").
func isUnknownVaultURI(s string) bool {
	// Check if it has a URI scheme pattern
	idx := strings.Index(s, "://")
	if idx == -1 || idx == 0 {
		return false
	}
	// It has a scheme, but is it a known one?
	return !isVaultURI(s)
}

// getScheme extracts the scheme from a URI.
func getScheme(uri string) string {
	idx := strings.Index(uri, "://")
	if idx == -1 {
		return ""
	}
	return uri[:idx]
}

// getPath extracts the path from a URI (everything after scheme://).
func getPath(uri string) string {
	idx := strings.Index(uri, "://")
	if idx == -1 {
		return uri
	}
	return uri[idx+3:]
}

// ResolveCredentials resolves all vault-backed credentials in the config.
// Values starting with op://, bw://, keeper://, file://, or env:// are
// resolved using omnivault. Plain string values are left unchanged.
func (c *Config) ResolveCredentials(ctx context.Context) error {
	resolver := newCredentialResolver()
	defer resolver.close()

	// Collect all credential fields that need resolution
	fields := []struct {
		name  string
		value *string
	}{
		{"agent.api_key", &c.Agent.APIKey},
		{"channels.telegram.token", &c.Channels.Telegram.Token},
		{"channels.discord.token", &c.Channels.Discord.Token},
		{"channels.twilio_sms.account_sid", &c.Channels.TwilioSMS.AccountSID},
		{"channels.twilio_sms.auth_token", &c.Channels.TwilioSMS.AuthToken},
		{"voice.stt.api_key", &c.Voice.STT.APIKey},
		{"voice.tts.api_key", &c.Voice.TTS.APIKey},
		{"observability.api_key", &c.Observability.APIKey},
	}

	for _, f := range fields {
		if *f.value == "" {
			continue
		}

		// Check for unknown vault URI schemes (potential typos)
		if isUnknownVaultURI(*f.value) {
			return fmt.Errorf("resolve %s: unknown vault URI scheme in %q", f.name, *f.value)
		}

		if !isVaultURI(*f.value) {
			continue
		}

		resolved, err := resolveCredential(ctx, resolver, *f.value)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", f.name, err)
		}
		*f.value = resolved
	}

	return nil
}

// resolveCredential resolves a single vault URI to its secret value.
func resolveCredential(ctx context.Context, resolver *credentialResolver, uri string) (string, error) {
	scheme := getScheme(uri)
	if scheme == "" {
		return "", fmt.Errorf("invalid vault URI: %s", uri)
	}

	v, err := resolver.getVault(scheme)
	if err != nil {
		return "", fmt.Errorf("open vault: %w", err)
	}

	// Get the path after scheme://
	// Provider's Get() handles the full path including vault/item/field parsing
	path := getPath(uri)
	if path == "" {
		return "", fmt.Errorf("no path in vault URI: %s", uri)
	}

	secret, err := v.Get(ctx, path)
	if err != nil {
		return "", fmt.Errorf("get secret: %w", err)
	}

	return secret.Value, nil
}
