package config

import "testing"

func TestConfig_WebUIEnabled(t *testing.T) {
	tests := []struct {
		name string
		web  bool
		team bool
		want bool
	}{
		{name: "default off", want: false},
		{name: "explicit web.enabled", web: true, want: true},
		{name: "implied by team mode", team: true, want: true},
		{name: "both on", web: true, team: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Web: WebConfig{Enabled: tt.web}, Team: TeamConfig{Enabled: tt.team}}
			if got := cfg.WebUIEnabled(); got != tt.want {
				t.Errorf("WebUIEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
