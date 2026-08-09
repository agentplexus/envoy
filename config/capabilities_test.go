package config

import "testing"

func TestConfig_Capabilities(t *testing.T) {
	tests := []struct {
		name         string
		team         bool
		auth         bool
		wantMultiSet bool
		want         Capabilities
	}{
		{
			name: "personal, no auth (default)",
			want: Capabilities{},
		},
		{
			name: "personal, single-account auth",
			auth: true,
			want: Capabilities{AuthRequired: true},
		},
		{
			name: "team implies auth even if Auth.Enabled is left false",
			team: true,
			want: Capabilities{
				MultiUser:    true,
				AuthRequired: true,
				GroupChats:   true,
				Admin:        true,
				Catalog:      true,
			},
		},
		{
			name: "team with explicit auth is unchanged",
			team: true,
			auth: true,
			want: Capabilities{
				MultiUser:    true,
				AuthRequired: true,
				GroupChats:   true,
				Admin:        true,
				Catalog:      true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Team: TeamConfig{Enabled: tt.team},
				Auth: AuthConfig{Enabled: tt.auth},
			}
			got := cfg.Capabilities()
			if got != tt.want {
				t.Errorf("Capabilities() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
