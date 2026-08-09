package config

import "testing"

func TestAuthConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     AuthConfig
		wantErr bool
	}{
		{name: "disabled skips validation", cfg: AuthConfig{Enabled: false}},
		{
			name:    "enabled without owner email",
			cfg:     AuthConfig{Enabled: true},
			wantErr: true,
		},
		{
			name:    "enabled with invalid owner email",
			cfg:     AuthConfig{Enabled: true, OwnerEmail: "not-an-email"},
			wantErr: true,
		},
		{
			name: "enabled with owner email only",
			cfg:  AuthConfig{Enabled: true, OwnerEmail: "owner@example.com"},
		},
		{
			name: "valid base URL",
			cfg:  AuthConfig{Enabled: true, OwnerEmail: "owner@example.com", BaseURL: "https://agent.example.com"},
		},
		{
			name:    "invalid base URL scheme",
			cfg:     AuthConfig{Enabled: true, OwnerEmail: "owner@example.com", BaseURL: "ftp://agent.example.com"},
			wantErr: true,
		},
		{
			name:    "smtp host without from",
			cfg:     AuthConfig{Enabled: true, OwnerEmail: "owner@example.com", SMTP: TeamSMTPConfig{Host: "smtp.example.com"}},
			wantErr: true,
		},
		{
			name:    "smtp from is not a valid email",
			cfg:     AuthConfig{Enabled: true, OwnerEmail: "owner@example.com", SMTP: TeamSMTPConfig{Host: "smtp.example.com", From: "nope"}},
			wantErr: true,
		},
		{
			name: "valid smtp",
			cfg: AuthConfig{
				Enabled:    true,
				OwnerEmail: "owner@example.com",
				SMTP:       TeamSMTPConfig{Host: "smtp.example.com", From: "agent@example.com"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
