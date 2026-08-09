package config

import (
	"fmt"
	"net/mail"
	"net/url"
)

// AuthConfig gates browser-facing login, independent of team (multi-user)
// mode (TRD §1a/§4). It is the login-required axis for the web UI, distinct
// from the existing gateway.APIKeys on/off auth used for programmatic/WS
// clients.
type AuthConfig struct {
	// Enabled requires login for the web UI even in personal (single-user)
	// mode, reusing the cookie-session machinery minus the allowlist and
	// multi-user bootstrap. Defaults to false: personal/localhost use stays
	// no-auth. team.enabled=true implies this is true regardless of the
	// configured value — see Config.Capabilities.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// OwnerEmail is the sole account permitted to log in in personal
	// single-account mode (auth.enabled=true, team.enabled=false); it
	// becomes the superadmin on first login (TRD §4 "the sole account is
	// the configured owner"). Ignored when team.enabled=true — team mode's
	// superadmin_email governs instead.
	OwnerEmail string `json:"owner_email,omitempty" yaml:"owner_email,omitempty"`

	// BaseURL is the externally visible origin used to build the magic
	// link and select the session cookie's Secure attribute. Ignored when
	// team.enabled=true — team.base_url governs instead.
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`

	// SMTP delivers the magic-link email. Ignored when team.enabled=true.
	// When unset, links are logged instead of emailed (dev only).
	SMTP TeamSMTPConfig `json:"smtp,omitempty" yaml:"smtp,omitempty"`
}

// Validate checks the personal single-account auth configuration. Only
// meaningful when Enabled and team mode is off — team mode validates
// itself via TeamConfig.Validate and ignores these fields.
func (c *AuthConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.OwnerEmail == "" {
		return fmt.Errorf("auth.owner_email is required when auth.enabled=true and team mode is off")
	}
	if _, err := mail.ParseAddress(c.OwnerEmail); err != nil {
		return fmt.Errorf("auth.owner_email %q is not a valid email address", c.OwnerEmail)
	}
	if c.BaseURL != "" {
		u, err := url.Parse(c.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("auth.base_url %q must be an http(s) origin", c.BaseURL)
		}
	}
	if (c.SMTP.Host != "") != (c.SMTP.From != "") {
		return fmt.Errorf("auth.smtp requires both host and from when configured")
	}
	if c.SMTP.From != "" {
		if _, err := mail.ParseAddress(c.SMTP.From); err != nil {
			return fmt.Errorf("auth.smtp.from %q is not a valid email address", c.SMTP.From)
		}
	}
	return nil
}
