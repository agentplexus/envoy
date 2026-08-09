package config

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
}
