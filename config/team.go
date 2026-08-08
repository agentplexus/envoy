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

	// AgentHandle is the @-mention handle of the agent in group chats
	// (default "omniagent").
	AgentHandle string `json:"agent_handle,omitempty" yaml:"agent_handle,omitempty"`

	// SMTP configures magic-link email delivery.
	SMTP TeamSMTPConfig `json:"smtp" yaml:"smtp"`
}

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
	return nil
}
