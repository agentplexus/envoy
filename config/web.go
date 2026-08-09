package config

// WebConfig controls the embedded browser UI (TRD §6), independent of team
// mode. Team mode always implies the web UI (it is the whole point of a
// hosted team deployment); personal mode opts in explicitly so the existing
// zero-dependency single-operator experience is unaffected by default.
type WebConfig struct {
	// Enabled serves the embedded SPA and GET /api/capabilities at the
	// gateway's HTTP address. Defaults to false in personal mode.
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// WebUIEnabled reports whether the embedded web UI (SPA + capabilities
// endpoint) should be served: explicit web.enabled, or implied by team mode.
func (c *Config) WebUIEnabled() bool {
	return c.Web.Enabled || c.Team.Enabled
}
