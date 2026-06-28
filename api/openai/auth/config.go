// Package auth provides OAuth 2.0 authentication for the OmniAgent web UI.
package auth

import (
	"os"
	"strings"
)

// Config holds authentication configuration.
type Config struct {
	// Enabled controls whether authentication is required for the web UI.
	Enabled bool

	// SessionSecret is the secret key for signing session cookies.
	// Must be at least 32 bytes when auth is enabled.
	SessionSecret string

	// CookieDomain optionally restricts cookies to a specific domain.
	CookieDomain string

	// GitHub holds GitHub OAuth configuration.
	GitHub OAuthProviderConfig

	// Google holds Google OAuth configuration.
	Google OAuthProviderConfig

	// AllowedEmails is a list of exact email addresses allowed to login.
	AllowedEmails []string

	// AllowedDomains is a list of email domains allowed to login (e.g., "@company.com").
	AllowedDomains []string
}

// OAuthProviderConfig holds OAuth configuration for a provider.
type OAuthProviderConfig struct {
	// ClientID is the OAuth client ID.
	ClientID string

	// ClientSecret is the OAuth client secret.
	ClientSecret string
}

// HasGitHub returns true if GitHub OAuth is configured.
func (c *Config) HasGitHub() bool {
	return c.GitHub.ClientID != "" && c.GitHub.ClientSecret != ""
}

// HasGoogle returns true if Google OAuth is configured.
func (c *Config) HasGoogle() bool {
	return c.Google.ClientID != "" && c.Google.ClientSecret != ""
}

// HasProviders returns true if at least one OAuth provider is configured.
func (c *Config) HasProviders() bool {
	return c.HasGitHub() || c.HasGoogle()
}

// HasACL returns true if email or domain ACL is configured.
func (c *Config) HasACL() bool {
	return len(c.AllowedEmails) > 0 || len(c.AllowedDomains) > 0
}

// LoadFromEnv loads authentication configuration from environment variables.
//
// Environment variables:
//   - AUTH_ENABLED: Enable auth (default: false)
//   - AUTH_SESSION_SECRET: Secret for cookie signing (required when enabled)
//   - AUTH_COOKIE_DOMAIN: Optional cookie domain
//   - AUTH_GITHUB_CLIENT_ID: GitHub OAuth client ID
//   - AUTH_GITHUB_CLIENT_SECRET: GitHub OAuth client secret
//   - AUTH_GOOGLE_CLIENT_ID: Google OAuth client ID
//   - AUTH_GOOGLE_CLIENT_SECRET: Google OAuth client secret
//   - AUTH_ALLOWED_EMAILS: Comma-separated allowed emails
//   - AUTH_ALLOWED_DOMAINS: Comma-separated allowed domains (e.g., "@company.com")
func LoadFromEnv() *Config {
	cfg := &Config{
		Enabled:       envBool("AUTH_ENABLED"),
		SessionSecret: os.Getenv("AUTH_SESSION_SECRET"),
		CookieDomain:  os.Getenv("AUTH_COOKIE_DOMAIN"),
		GitHub: OAuthProviderConfig{
			ClientID:     os.Getenv("AUTH_GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("AUTH_GITHUB_CLIENT_SECRET"),
		},
		Google: OAuthProviderConfig{
			ClientID:     os.Getenv("AUTH_GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("AUTH_GOOGLE_CLIENT_SECRET"),
		},
		AllowedEmails:  parseCSV(os.Getenv("AUTH_ALLOWED_EMAILS")),
		AllowedDomains: parseCSV(os.Getenv("AUTH_ALLOWED_DOMAINS")),
	}

	return cfg
}

// LoadAAuthFromEnv loads AAuth configuration from environment variables.
//
// Environment variables:
//   - AUTH_AAUTH_ENABLED: Enable AAuth token validation (default: false)
//   - AUTH_AAUTH_ISSUER: AAuth issuer URL (PeopleServer URL)
//   - AUTH_AAUTH_AUDIENCE: Expected audience claim (this service's URL)
//   - AUTH_AAUTH_JWKS_URL: Optional custom JWKS URL (defaults to {issuer}/.well-known/jwks.json)
func LoadAAuthFromEnv() *AAuthConfig {
	return &AAuthConfig{
		Enabled:   envBool("AUTH_AAUTH_ENABLED"),
		IssuerURL: os.Getenv("AUTH_AAUTH_ISSUER"),
		Audience:  os.Getenv("AUTH_AAUTH_AUDIENCE"),
		JWKSURL:   os.Getenv("AUTH_AAUTH_JWKS_URL"),
	}
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.SessionSecret == "" {
		return ErrMissingSessionSecret
	}

	if len(c.SessionSecret) < 32 {
		return ErrSessionSecretTooShort
	}

	if !c.HasProviders() {
		return ErrNoProviders
	}

	return nil
}

// envBool returns true if the environment variable is set to a truthy value.
func envBool(key string) bool {
	v := strings.ToLower(os.Getenv(key))
	return v == "true" || v == "1" || v == "yes"
}

// parseCSV splits a comma-separated string into a slice, trimming whitespace.
func parseCSV(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
