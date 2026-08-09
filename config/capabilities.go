package config

// Capabilities describes which browser-facing surfaces are active for the
// current deployment mode. The web UI reads this once (GET /api/capabilities)
// and renders accordingly — there is no separate personal/team build
// (TRD §1a/§6).
type Capabilities struct {
	// MultiUser is true in team mode: more than one account, RLS isolation,
	// group chats, and admin all become meaningful.
	MultiUser bool `json:"multiUser"`
	// AuthRequired is true when the web UI must show a login screen: either
	// team mode (which implies auth) or personal mode with auth.enabled set.
	AuthRequired bool `json:"authRequired"`
	// GroupChats, Admin, and Catalog are team-only surfaces: a single
	// implicit user has no one to group-chat with, administer, or browse an
	// agent catalog for.
	GroupChats bool `json:"groupChats"`
	Admin      bool `json:"admin"`
	Catalog    bool `json:"catalog"`
}

// Capabilities derives the active capability set from the team and auth
// config axes (TRD §1a). team.enabled implies auth.enabled regardless of the
// configured Auth.Enabled value.
func (c *Config) Capabilities() Capabilities {
	multiUser := c.Team.Enabled
	return Capabilities{
		MultiUser:    multiUser,
		AuthRequired: multiUser || c.Auth.Enabled,
		GroupChats:   multiUser,
		Admin:        multiUser,
		Catalog:      multiUser,
	}
}
