package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validTeamConfig() TeamConfig {
	return TeamConfig{
		Enabled: true,
		Database: TeamDatabaseConfig{ //nolint:gosec // G101: dummy test DSNs, not credentials
			AppDSN:     "postgres://omniagent_app:pw@db:5432/omniagent_team",
			MigrateDSN: "postgres://owner:pw@db:5432/omniagent_team",
		},
		BaseURL:         "https://team.example.com",
		SuperadminEmail: "root@example.com",
		AgentHandle:     "omniagent",
		SMTP: TeamSMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
			From: "agent@example.com",
		},
	}
}

func TestTeamConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*TeamConfig)
		wantErr string // empty = valid
	}{
		{name: "disabled is always valid", mutate: func(c *TeamConfig) { *c = TeamConfig{} }},
		{name: "full config valid", mutate: func(c *TeamConfig) {}},
		{name: "missing app dsn", mutate: func(c *TeamConfig) { c.Database.AppDSN = "" }, wantErr: "app_dsn"},
		{name: "missing superadmin email", mutate: func(c *TeamConfig) { c.SuperadminEmail = "" }, wantErr: "superadmin_email"},
		{name: "bad superadmin email", mutate: func(c *TeamConfig) { c.SuperadminEmail = "not-an-email" }, wantErr: "superadmin_email"},
		{name: "bad base url scheme", mutate: func(c *TeamConfig) { c.BaseURL = "ftp://x" }, wantErr: "base_url"},
		{name: "empty base url ok at data layer", mutate: func(c *TeamConfig) { c.BaseURL = "" }},
		{name: "bad agent handle", mutate: func(c *TeamConfig) { c.AgentHandle = "Not Valid!" }, wantErr: "agent_handle"},
		{name: "smtp host without from", mutate: func(c *TeamConfig) { c.SMTP.From = "" }, wantErr: "smtp"},
		{name: "bad smtp from", mutate: func(c *TeamConfig) { c.SMTP.From = "nope" }, wantErr: "smtp.from"},
		{name: "no smtp at all ok at data layer", mutate: func(c *TeamConfig) { c.SMTP = TeamSMTPConfig{} }},
		{name: "secrets memory provider ok", mutate: func(c *TeamConfig) { c.Secrets = TeamSecretsConfig{Provider: "memory"} }},
		{name: "secrets file provider needs dir", mutate: func(c *TeamConfig) { c.Secrets = TeamSecretsConfig{Provider: "file"} }, wantErr: "secrets.dir"},
		{name: "secrets file provider with dir ok", mutate: func(c *TeamConfig) { c.Secrets = TeamSecretsConfig{Provider: "file", Dir: "/var/secrets"} }},
		{name: "secrets unknown provider", mutate: func(c *TeamConfig) { c.Secrets = TeamSecretsConfig{Provider: "vault"} }, wantErr: "secrets.provider"},
		{name: "secrets empty provider ok", mutate: func(c *TeamConfig) { c.Secrets = TeamSecretsConfig{} }},
		{name: "sso google configured ok", mutate: func(c *TeamConfig) {
			c.SSO.Google = TeamOAuthProviderConfig{ClientID: "id", ClientSecret: "secret"}
		}},
		{name: "sso google missing secret", mutate: func(c *TeamConfig) {
			c.SSO.Google = TeamOAuthProviderConfig{ClientID: "id"}
		}, wantErr: "team.sso.google"},
		{name: "sso google missing id", mutate: func(c *TeamConfig) {
			c.SSO.Google = TeamOAuthProviderConfig{ClientSecret: "secret"}
		}, wantErr: "team.sso.google"},
		{name: "sso github configured ok", mutate: func(c *TeamConfig) {
			c.SSO.GitHub = TeamOAuthProviderConfig{ClientID: "id", ClientSecret: "secret"}
		}},
		{name: "sso github missing secret", mutate: func(c *TeamConfig) {
			c.SSO.GitHub = TeamOAuthProviderConfig{ClientID: "id"}
		}, wantErr: "team.sso.github"},
		{name: "sso configured requires base_url", mutate: func(c *TeamConfig) {
			c.BaseURL = ""
			c.SSO.Google = TeamOAuthProviderConfig{ClientID: "id", ClientSecret: "secret"}
		}, wantErr: "team.base_url is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validTeamConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadYAML_Team(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
team:
  enabled: true` + /* dummy test DSNs, not credentials */ `
  base_url: https://team.example.com
  superadmin_email: root@example.com
  agent_handle: casey
  database:
    app_dsn: postgres://omniagent_app:pw@db:5432/omniagent_team
    migrate_dsn: postgres://owner:pw@db:5432/omniagent_team
    app_role: omniagent_app
  smtp:
    host: smtp.example.com
    port: 587
    from: agent@example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Team.Enabled {
		t.Error("Team.Enabled should be true")
	}
	if cfg.Team.Database.AppDSN == "" || cfg.Team.Database.MigrateDSN == "" {
		t.Errorf("database DSNs did not load: %+v", cfg.Team.Database)
	}
	if cfg.Team.AgentHandle != "casey" {
		t.Errorf("AgentHandle = %q, want casey", cfg.Team.AgentHandle)
	}
	if cfg.Team.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %d, want 587", cfg.Team.SMTP.Port)
	}
	if err := cfg.Team.Validate(); err != nil {
		t.Errorf("loaded config should validate: %v", err)
	}
}
