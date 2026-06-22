package auth

import (
	"os"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	// Save original env vars
	origVars := map[string]string{
		"AUTH_ENABLED":              os.Getenv("AUTH_ENABLED"),
		"AUTH_SESSION_SECRET":       os.Getenv("AUTH_SESSION_SECRET"),
		"AUTH_COOKIE_DOMAIN":        os.Getenv("AUTH_COOKIE_DOMAIN"),
		"AUTH_GITHUB_CLIENT_ID":     os.Getenv("AUTH_GITHUB_CLIENT_ID"),
		"AUTH_GITHUB_CLIENT_SECRET": os.Getenv("AUTH_GITHUB_CLIENT_SECRET"),
		"AUTH_GOOGLE_CLIENT_ID":     os.Getenv("AUTH_GOOGLE_CLIENT_ID"),
		"AUTH_GOOGLE_CLIENT_SECRET": os.Getenv("AUTH_GOOGLE_CLIENT_SECRET"),
		"AUTH_ALLOWED_EMAILS":       os.Getenv("AUTH_ALLOWED_EMAILS"),
		"AUTH_ALLOWED_DOMAINS":      os.Getenv("AUTH_ALLOWED_DOMAINS"),
	}

	// Restore env vars after test
	defer func() {
		for k, v := range origVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set test env vars
	os.Setenv("AUTH_ENABLED", "true")
	os.Setenv("AUTH_SESSION_SECRET", "test-secret-that-is-at-least-32-bytes-long")
	os.Setenv("AUTH_COOKIE_DOMAIN", ".example.com")
	os.Setenv("AUTH_GITHUB_CLIENT_ID", "github-client-id")
	os.Setenv("AUTH_GITHUB_CLIENT_SECRET", "github-client-secret")
	os.Setenv("AUTH_GOOGLE_CLIENT_ID", "google-client-id")
	os.Setenv("AUTH_GOOGLE_CLIENT_SECRET", "google-client-secret")
	os.Setenv("AUTH_ALLOWED_EMAILS", "user1@example.com, user2@example.com")
	os.Setenv("AUTH_ALLOWED_DOMAINS", "@company.com, @corp.com")

	cfg := LoadFromEnv()

	if !cfg.Enabled {
		t.Error("expected Enabled to be true")
	}
	if cfg.SessionSecret != "test-secret-that-is-at-least-32-bytes-long" {
		t.Errorf("unexpected SessionSecret: %s", cfg.SessionSecret)
	}
	if cfg.CookieDomain != ".example.com" {
		t.Errorf("unexpected CookieDomain: %s", cfg.CookieDomain)
	}
	if cfg.GitHub.ClientID != "github-client-id" {
		t.Errorf("unexpected GitHub.ClientID: %s", cfg.GitHub.ClientID)
	}
	if cfg.GitHub.ClientSecret != "github-client-secret" {
		t.Errorf("unexpected GitHub.ClientSecret: %s", cfg.GitHub.ClientSecret)
	}
	if cfg.Google.ClientID != "google-client-id" {
		t.Errorf("unexpected Google.ClientID: %s", cfg.Google.ClientID)
	}
	if cfg.Google.ClientSecret != "google-client-secret" {
		t.Errorf("unexpected Google.ClientSecret: %s", cfg.Google.ClientSecret)
	}
	if len(cfg.AllowedEmails) != 2 {
		t.Errorf("expected 2 allowed emails, got %d", len(cfg.AllowedEmails))
	}
	if len(cfg.AllowedDomains) != 2 {
		t.Errorf("expected 2 allowed domains, got %d", len(cfg.AllowedDomains))
	}
}

func TestConfig_HasProviders(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		hasGitHub bool
		hasGoogle bool
		hasAny    bool
	}{
		{
			name:      "no providers",
			cfg:       &Config{},
			hasGitHub: false,
			hasGoogle: false,
			hasAny:    false,
		},
		{
			name: "github only",
			cfg: &Config{
				GitHub: OAuthProviderConfig{
					ClientID:     "id",
					ClientSecret: "secret",
				},
			},
			hasGitHub: true,
			hasGoogle: false,
			hasAny:    true,
		},
		{
			name: "google only",
			cfg: &Config{
				Google: OAuthProviderConfig{
					ClientID:     "id",
					ClientSecret: "secret",
				},
			},
			hasGitHub: false,
			hasGoogle: true,
			hasAny:    true,
		},
		{
			name: "both providers",
			cfg: &Config{
				GitHub: OAuthProviderConfig{
					ClientID:     "id",
					ClientSecret: "secret",
				},
				Google: OAuthProviderConfig{
					ClientID:     "id",
					ClientSecret: "secret",
				},
			},
			hasGitHub: true,
			hasGoogle: true,
			hasAny:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.HasGitHub(); got != tt.hasGitHub {
				t.Errorf("HasGitHub() = %v, want %v", got, tt.hasGitHub)
			}
			if got := tt.cfg.HasGoogle(); got != tt.hasGoogle {
				t.Errorf("HasGoogle() = %v, want %v", got, tt.hasGoogle)
			}
			if got := tt.cfg.HasProviders(); got != tt.hasAny {
				t.Errorf("HasProviders() = %v, want %v", got, tt.hasAny)
			}
		})
	}
}

func TestConfig_HasACL(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		hasACL bool
	}{
		{
			name:   "no ACL",
			cfg:    &Config{},
			hasACL: false,
		},
		{
			name: "emails only",
			cfg: &Config{
				AllowedEmails: []string{"user@example.com"},
			},
			hasACL: true,
		},
		{
			name: "domains only",
			cfg: &Config{
				AllowedDomains: []string{"@company.com"},
			},
			hasACL: true,
		},
		{
			name: "both",
			cfg: &Config{
				AllowedEmails:  []string{"user@example.com"},
				AllowedDomains: []string{"@company.com"},
			},
			hasACL: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.HasACL(); got != tt.hasACL {
				t.Errorf("HasACL() = %v, want %v", got, tt.hasACL)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr error
	}{
		{
			name:    "disabled - no validation",
			cfg:     &Config{Enabled: false},
			wantErr: nil,
		},
		{
			name:    "enabled - missing secret",
			cfg:     &Config{Enabled: true},
			wantErr: ErrMissingSessionSecret,
		},
		{
			name: "enabled - secret too short",
			cfg: &Config{
				Enabled:       true,
				SessionSecret: "short",
			},
			wantErr: ErrSessionSecretTooShort,
		},
		{
			name: "enabled - no providers",
			cfg: &Config{
				Enabled:       true,
				SessionSecret: "this-is-a-secret-that-is-32-bytes-long",
			},
			wantErr: ErrNoProviders,
		},
		{
			name: "valid config",
			cfg: &Config{
				Enabled:       true,
				SessionSecret: "this-is-a-secret-that-is-32-bytes-long",
				GitHub: OAuthProviderConfig{
					ClientID:     "id",
					ClientSecret: "secret",
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"", false},
		{"other", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			os.Setenv("TEST_BOOL", tt.value)
			defer os.Unsetenv("TEST_BOOL")

			if got := envBool("TEST_BOOL"); got != tt.want {
				t.Errorf("envBool(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
		{",a,b,", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseCSV(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseCSV(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseCSV(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
