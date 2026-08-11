package config

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
)

// TeamConfig configures team (multi-user) mode: PostgreSQL-backed users,
// allowlist-closed magic-link auth, and private/group chats. Disabled by
// default — single-operator deployments are unaffected.
type TeamConfig struct {
	// Enabled turns team mode on.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Database is the PostgreSQL connection configuration.
	Database TeamDatabaseConfig `json:"database" yaml:"database"`

	// BaseURL is the externally visible origin (e.g. https://team.example.com),
	// used to build magic links and cookies.
	BaseURL string `json:"base_url" yaml:"base_url"`

	// SuperadminEmail bootstraps the superadmin on first login.
	SuperadminEmail string `json:"superadmin_email" yaml:"superadmin_email"`

	// SuperadminPassword, when set, seeds the superadmin's email+password
	// credential on startup (set-once: applied only if that account has no
	// password yet, so it never clobbers a later change). Lets an operator log
	// in without SMTP. Prefer supplying it via OMNIAGENT_TEAM_SUPERADMIN_PASSWORD
	// or a vault reference rather than a literal in a committed config file.
	SuperadminPassword string `json:"superadmin_password,omitempty" yaml:"superadmin_password,omitempty"` //nolint:gosec // G117: loaded from config/env, not a hardcoded credential

	// AgentHandle is the @-mention handle of the agent in group chats
	// (default "omniagent").
	AgentHandle string `json:"agent_handle,omitempty" yaml:"agent_handle,omitempty"`

	// SMTP configures magic-link email delivery.
	SMTP TeamSMTPConfig `json:"smtp" yaml:"smtp"`

	// Secrets configures the per-agent secret vault. When set, an @-mentioned
	// agent's runtime instance is built with its own agent-scoped secrets
	// injected into secrets-aware skills (per-agent MCP subprocess env). When
	// unset, agents run without injected secrets.
	Secrets TeamSecretsConfig `json:"secrets,omitempty" yaml:"secrets,omitempty"`

	// SSO configures optional OAuth/OIDC sign-in providers, additive to
	// magic-link email. Each provider is independent; a provider is
	// "configured" when both its client ID and secret are set. Redirect URIs
	// are not configurable — derived as
	// {base_url}/api/auth/{provider}/callback.
	SSO TeamSSOConfig `json:"sso,omitempty" yaml:"sso,omitempty"`
}

// TeamSSOConfig configures optional Google OIDC and GitHub OAuth sign-in.
type TeamSSOConfig struct {
	Google TeamOAuthProviderConfig `json:"google,omitempty" yaml:"google,omitempty"`
	GitHub TeamOAuthProviderConfig `json:"github,omitempty" yaml:"github,omitempty"`
}

// TeamOAuthProviderConfig holds one SSO provider's OAuth client credentials.
type TeamOAuthProviderConfig struct {
	ClientID     string `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty" yaml:"client_secret,omitempty"` //nolint:gosec // G117: loaded from config file
}

// configured reports whether both client ID and secret are set.
func (c TeamOAuthProviderConfig) configured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

// TeamSecretsConfig configures the OmniVault-backed team secret store. Secrets
// are namespaced per agent ("agents/<id>/<ENV_VAR>") so two agents load disjoint
// secrets with no cross-leak. Encryption-at-rest is not yet provided here (the
// mechanism ships first); "memory" suits tests and "file" a simple local store.
type TeamSecretsConfig struct {
	// Provider selects the OmniVault backing provider: "memory" or "file".
	// Empty disables secret injection.
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`

	// Dir is the storage directory for the "file" provider (required for it).
	Dir string `json:"dir,omitempty" yaml:"dir,omitempty"`
}

// teamSecretProviders is the set of supported backing providers.
var teamSecretProviders = map[string]bool{"memory": true, "file": true}

// TeamDatabaseConfig holds the two-role PostgreSQL connection strings.
type TeamDatabaseConfig struct {
	// AppDSN is the non-owner application role connection string.
	AppDSN string `json:"app_dsn" yaml:"app_dsn"`

	// MigrateDSN is the owner role used only for migrations-on-start.
	MigrateDSN string `json:"migrate_dsn" yaml:"migrate_dsn"`

	// AppRole is the application role name granted access by migrations
	// (default "omniagent_app").
	AppRole string `json:"app_role,omitempty" yaml:"app_role,omitempty"`
}

// TeamSMTPConfig configures outbound email for magic links.
type TeamSMTPConfig struct {
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"` //nolint:gosec // G117: loaded from config file
	From     string `json:"from" yaml:"from"`
}

// agentHandlePattern constrains the @-mention handle.
var agentHandlePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,31}$`)

// minSuperadminPasswordLen mirrors auth.MinPasswordLen; kept local so config
// validation has no dependency on the auth package.
const minSuperadminPasswordLen = 8

// Validate checks the team configuration. A disabled config is always valid.
func (c *TeamConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Database.AppDSN == "" {
		return fmt.Errorf("team.database.app_dsn is required when team mode is enabled")
	}
	if c.SuperadminEmail == "" {
		return fmt.Errorf("team.superadmin_email is required when team mode is enabled")
	}
	if _, err := mail.ParseAddress(c.SuperadminEmail); err != nil {
		return fmt.Errorf("team.superadmin_email %q is not a valid email address", c.SuperadminEmail)
	}
	if c.BaseURL != "" {
		u, err := url.Parse(c.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("team.base_url %q must be an http(s) origin", c.BaseURL)
		}
	}
	if c.AgentHandle != "" && !agentHandlePattern.MatchString(c.AgentHandle) {
		return fmt.Errorf("team.agent_handle %q must match %s", c.AgentHandle, agentHandlePattern)
	}
	if c.SuperadminPassword != "" && len(c.SuperadminPassword) < minSuperadminPasswordLen {
		return fmt.Errorf("team.superadmin_password must be at least %d characters", minSuperadminPasswordLen)
	}
	// SMTP becomes mandatory with magic-link auth (Phase 2); at the data
	// layer we only insist on coherence when partially configured.
	if (c.SMTP.Host != "") != (c.SMTP.From != "") {
		return fmt.Errorf("team.smtp requires both host and from when configured")
	}
	if c.SMTP.From != "" {
		if _, err := mail.ParseAddress(c.SMTP.From); err != nil {
			return fmt.Errorf("team.smtp.from %q is not a valid email address", c.SMTP.From)
		}
	}
	if c.Secrets.Provider != "" {
		if !teamSecretProviders[c.Secrets.Provider] {
			return fmt.Errorf("team.secrets.provider %q must be one of memory, file", c.Secrets.Provider)
		}
		if c.Secrets.Provider == "file" && c.Secrets.Dir == "" {
			return fmt.Errorf("team.secrets.dir is required when team.secrets.provider is file")
		}
	}
	if (c.SSO.Google.ClientID != "") != (c.SSO.Google.ClientSecret != "") {
		return fmt.Errorf("team.sso.google requires both client_id and client_secret when configured")
	}
	if (c.SSO.GitHub.ClientID != "") != (c.SSO.GitHub.ClientSecret != "") {
		return fmt.Errorf("team.sso.github requires both client_id and client_secret when configured")
	}
	if (c.SSO.Google.configured() || c.SSO.GitHub.configured()) && c.BaseURL == "" {
		return fmt.Errorf("team.base_url is required when an SSO provider is configured (used to build the redirect URI)")
	}
	return nil
}
