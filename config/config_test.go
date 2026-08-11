package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	// Gateway defaults
	if cfg.Gateway.Address != "127.0.0.1:18789" {
		t.Errorf("Gateway.Address = %s, want 127.0.0.1:18789", cfg.Gateway.Address)
	}
	if cfg.Gateway.ReadTimeout != 30*time.Second {
		t.Errorf("Gateway.ReadTimeout = %v, want 30s", cfg.Gateway.ReadTimeout)
	}

	// Agent defaults
	if cfg.Agent.Provider != "anthropic" {
		t.Errorf("Agent.Provider = %s, want anthropic", cfg.Agent.Provider)
	}
	if cfg.Agent.Temperature != 0.7 {
		t.Errorf("Agent.Temperature = %f, want 0.7", cfg.Agent.Temperature)
	}

	// Channels disabled by default
	if cfg.Channels.Telegram.Enabled {
		t.Error("Telegram should be disabled by default")
	}
	if cfg.Channels.Discord.Enabled {
		t.Error("Discord should be disabled by default")
	}

	// Tools
	if !cfg.Tools.Browser.Enabled {
		t.Error("Browser tool should be enabled by default")
	}
	if cfg.Tools.Shell.Enabled {
		t.Error("Shell tool should be disabled by default")
	}
}

func TestLoadYAML(t *testing.T) {
	// Clear env vars that could override config values
	envVars := []string{
		"OMNIAGENT_AGENT_PROVIDER",
		"OMNIAGENT_AGENT_MODEL",
		"OMNIAGENT_GATEWAY_ADDRESS",
	}
	for _, v := range envVars {
		if orig := os.Getenv(v); orig != "" {
			os.Unsetenv(v)
			defer os.Setenv(v, orig)
		}
	}

	// Create temp config file
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
gateway:
  address: "0.0.0.0:9000"
agent:
  provider: openai
  model: gpt-4
channels:
  telegram:
    enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Gateway.Address != "0.0.0.0:9000" {
		t.Errorf("Gateway.Address = %s, want 0.0.0.0:9000", cfg.Gateway.Address)
	}
	if cfg.Agent.Provider != "openai" {
		t.Errorf("Agent.Provider = %s, want openai", cfg.Agent.Provider)
	}
	if cfg.Agent.Model != "gpt-4" {
		t.Errorf("Agent.Model = %s, want gpt-4", cfg.Agent.Model)
	}
	if !cfg.Channels.Telegram.Enabled {
		t.Error("Telegram should be enabled")
	}
}

func TestLoadYAML_TimezoneAndRollover(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
agent:
  provider: openai
  model: gpt-4
  timezone: America/Los_Angeles
sessions:
  rollover:
    enabled: true
    idle_timeout: 4h
    daily: true
    timezone: Europe/Berlin
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Agent.Timezone != "America/Los_Angeles" {
		t.Errorf("Agent.Timezone = %q, want America/Los_Angeles", cfg.Agent.Timezone)
	}
	if !cfg.Sessions.Rollover.Enabled {
		t.Error("Sessions.Rollover.Enabled should be true")
	}
	if cfg.Sessions.Rollover.IdleTimeout.Duration() != 4*time.Hour {
		t.Errorf("Sessions.Rollover.IdleTimeout = %v, want 4h", cfg.Sessions.Rollover.IdleTimeout)
	}
	if !cfg.Sessions.Rollover.Daily {
		t.Error("Sessions.Rollover.Daily should be true")
	}
	if cfg.Sessions.Rollover.Timezone != "Europe/Berlin" {
		t.Errorf("Sessions.Rollover.Timezone = %q, want Europe/Berlin", cfg.Sessions.Rollover.Timezone)
	}
}

func TestLoadJSON(t *testing.T) {
	// Clear env vars that could override config values
	envVars := []string{
		"OMNIAGENT_AGENT_PROVIDER",
		"OMNIAGENT_AGENT_MODEL",
		"OMNIAGENT_GATEWAY_ADDRESS",
	}
	for _, v := range envVars {
		if orig := os.Getenv(v); orig != "" {
			os.Unsetenv(v)
			defer os.Setenv(v, orig)
		}
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	content := `{
  "gateway": {"address": "localhost:8080"},
  "agent": {"provider": "gemini"}
}`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Gateway.Address != "localhost:8080" {
		t.Errorf("Gateway.Address = %s, want localhost:8080", cfg.Gateway.Address)
	}
	if cfg.Agent.Provider != "gemini" {
		t.Errorf("Agent.Provider = %s, want gemini", cfg.Agent.Provider)
	}
}

func TestLoadEnv(t *testing.T) {
	// Set env vars
	os.Setenv("OMNIAGENT_GATEWAY_ADDRESS", "192.168.1.1:5000")
	os.Setenv("OMNIAGENT_AGENT_PROVIDER", "xai")
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	defer func() {
		os.Unsetenv("OMNIAGENT_GATEWAY_ADDRESS")
		os.Unsetenv("OMNIAGENT_AGENT_PROVIDER")
		os.Unsetenv("TELEGRAM_BOT_TOKEN")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Gateway.Address != "192.168.1.1:5000" {
		t.Errorf("Gateway.Address = %s, want 192.168.1.1:5000", cfg.Gateway.Address)
	}
	if cfg.Agent.Provider != "xai" {
		t.Errorf("Agent.Provider = %s, want xai", cfg.Agent.Provider)
	}
	if cfg.Channels.Telegram.Token != "test-token" {
		t.Errorf("Telegram.Token = %s, want test-token", cfg.Channels.Telegram.Token)
	}
	if !cfg.Channels.Telegram.Enabled {
		t.Error("Telegram should be auto-enabled when token is set")
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoadEnv_Discord(t *testing.T) {
	os.Setenv("DISCORD_BOT_TOKEN", "discord-test-token")
	defer os.Unsetenv("DISCORD_BOT_TOKEN")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Channels.Discord.Token != "discord-test-token" {
		t.Errorf("Discord.Token = %s, want discord-test-token", cfg.Channels.Discord.Token)
	}
	if !cfg.Channels.Discord.Enabled {
		t.Error("Discord should be auto-enabled when token is set")
	}
}

func TestLoadEnv_Twilio(t *testing.T) {
	os.Setenv("TWILIO_ACCOUNT_SID", "AC123456")
	os.Setenv("TWILIO_AUTH_TOKEN", "auth-secret")
	os.Setenv("TWILIO_PHONE_NUMBER", "+15551234567")
	os.Setenv("TWILIO_WEBHOOK_PATH", "/custom/webhook")
	defer func() {
		os.Unsetenv("TWILIO_ACCOUNT_SID")
		os.Unsetenv("TWILIO_AUTH_TOKEN")
		os.Unsetenv("TWILIO_PHONE_NUMBER")
		os.Unsetenv("TWILIO_WEBHOOK_PATH")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Channels.TwilioSMS.AccountSID != "AC123456" {
		t.Errorf("TwilioSMS.AccountSID = %s, want AC123456", cfg.Channels.TwilioSMS.AccountSID)
	}
	if cfg.Channels.TwilioSMS.AuthToken != "auth-secret" {
		t.Errorf("TwilioSMS.AuthToken = %s, want auth-secret", cfg.Channels.TwilioSMS.AuthToken)
	}
	if cfg.Channels.TwilioSMS.PhoneNumber != "+15551234567" {
		t.Errorf("TwilioSMS.PhoneNumber = %s, want +15551234567", cfg.Channels.TwilioSMS.PhoneNumber)
	}
	if cfg.Channels.TwilioSMS.WebhookPath != "/custom/webhook" {
		t.Errorf("TwilioSMS.WebhookPath = %s, want /custom/webhook", cfg.Channels.TwilioSMS.WebhookPath)
	}
	if !cfg.Channels.TwilioSMS.Enabled {
		t.Error("TwilioSMS should be auto-enabled when account SID is set")
	}
}

func TestLoadEnv_WhatsApp(t *testing.T) {
	os.Setenv("WHATSAPP_ENABLED", "true")
	os.Setenv("WHATSAPP_DB_PATH", "/custom/wa.db")
	defer func() {
		os.Unsetenv("WHATSAPP_ENABLED")
		os.Unsetenv("WHATSAPP_DB_PATH")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.Channels.WhatsApp.Enabled {
		t.Error("WhatsApp should be enabled")
	}
	if cfg.Channels.WhatsApp.DBPath != "/custom/wa.db" {
		t.Errorf("WhatsApp.DBPath = %s, want /custom/wa.db", cfg.Channels.WhatsApp.DBPath)
	}
}

func TestLoadEnv_Voice(t *testing.T) {
	os.Setenv("OMNIAGENT_VOICE_ENABLED", "true")
	os.Setenv("OMNIAGENT_VOICE_RESPONSE_MODE", "always")
	os.Setenv("OMNIAGENT_VOICE_STT_API_KEY", "stt-key")
	os.Setenv("OMNIAGENT_VOICE_STT_MODEL", "nova-2")
	os.Setenv("OMNIAGENT_VOICE_TTS_API_KEY", "tts-key")
	os.Setenv("OMNIAGENT_VOICE_TTS_MODEL", "aura")
	os.Setenv("OMNIAGENT_VOICE_TTS_VOICE_ID", "asteria")
	defer func() {
		os.Unsetenv("OMNIAGENT_VOICE_ENABLED")
		os.Unsetenv("OMNIAGENT_VOICE_RESPONSE_MODE")
		os.Unsetenv("OMNIAGENT_VOICE_STT_API_KEY")
		os.Unsetenv("OMNIAGENT_VOICE_STT_MODEL")
		os.Unsetenv("OMNIAGENT_VOICE_TTS_API_KEY")
		os.Unsetenv("OMNIAGENT_VOICE_TTS_MODEL")
		os.Unsetenv("OMNIAGENT_VOICE_TTS_VOICE_ID")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.Voice.Enabled {
		t.Error("Voice should be enabled")
	}
	if cfg.Voice.ResponseMode != "always" {
		t.Errorf("Voice.ResponseMode = %s, want always", cfg.Voice.ResponseMode)
	}
	if cfg.Voice.STT.APIKey != "stt-key" {
		t.Errorf("Voice.STT.APIKey = %s, want stt-key", cfg.Voice.STT.APIKey)
	}
	if cfg.Voice.STT.Model != "nova-2" {
		t.Errorf("Voice.STT.Model = %s, want nova-2", cfg.Voice.STT.Model)
	}
	if cfg.Voice.TTS.APIKey != "tts-key" {
		t.Errorf("Voice.TTS.APIKey = %s, want tts-key", cfg.Voice.TTS.APIKey)
	}
	if cfg.Voice.TTS.Model != "aura" {
		t.Errorf("Voice.TTS.Model = %s, want aura", cfg.Voice.TTS.Model)
	}
	if cfg.Voice.TTS.VoiceID != "asteria" {
		t.Errorf("Voice.TTS.VoiceID = %s, want asteria", cfg.Voice.TTS.VoiceID)
	}
}

func TestLoadEnv_VoiceFallbackToDeepgram(t *testing.T) {
	// Test fallback to DEEPGRAM_API_KEY when specific keys not set
	os.Setenv("DEEPGRAM_API_KEY", "deepgram-shared-key")
	defer os.Unsetenv("DEEPGRAM_API_KEY")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Voice.STT.APIKey != "deepgram-shared-key" {
		t.Errorf("Voice.STT.APIKey = %s, want deepgram-shared-key", cfg.Voice.STT.APIKey)
	}
	if cfg.Voice.TTS.APIKey != "deepgram-shared-key" {
		t.Errorf("Voice.TTS.APIKey = %s, want deepgram-shared-key", cfg.Voice.TTS.APIKey)
	}
}

func TestLoadEnv_Observability(t *testing.T) {
	os.Setenv("OMNIAGENT_OBSERVABILITY_PROVIDER", "langfuse")
	os.Setenv("OMNIAGENT_OBSERVABILITY_ENDPOINT", "https://langfuse.example.com")
	os.Setenv("OMNIAGENT_OBSERVABILITY_API_KEY", "obs-api-key")
	defer func() {
		os.Unsetenv("OMNIAGENT_OBSERVABILITY_PROVIDER")
		os.Unsetenv("OMNIAGENT_OBSERVABILITY_ENDPOINT")
		os.Unsetenv("OMNIAGENT_OBSERVABILITY_API_KEY")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.Observability.Enabled {
		t.Error("Observability should be auto-enabled when provider is set")
	}
	if cfg.Observability.Provider != "langfuse" {
		t.Errorf("Observability.Provider = %s, want langfuse", cfg.Observability.Provider)
	}
	if cfg.Observability.Endpoint != "https://langfuse.example.com" {
		t.Errorf("Observability.Endpoint = %s, want https://langfuse.example.com", cfg.Observability.Endpoint)
	}
	if cfg.Observability.APIKey != "obs-api-key" {
		t.Errorf("Observability.APIKey = %s, want obs-api-key", cfg.Observability.APIKey)
	}
}

func TestLoadEnv_Team(t *testing.T) {
	os.Setenv("OMNIAGENT_TEAM_ENABLED", "true")
	os.Setenv("OMNIAGENT_TEAM_DATABASE_APP_DSN", "postgres://app@db/team")
	os.Setenv("OMNIAGENT_TEAM_DATABASE_MIGRATE_DSN", "postgres://owner@db/team")
	os.Setenv("OMNIAGENT_TEAM_DATABASE_APP_ROLE", "custom_app_role")
	os.Setenv("OMNIAGENT_TEAM_BASE_URL", "https://team.example.com")
	os.Setenv("OMNIAGENT_TEAM_SUPERADMIN_EMAIL", "root@example.com")
	os.Setenv("OMNIAGENT_TEAM_SUPERADMIN_PASSWORD", "boot-passphrase")
	os.Setenv("OMNIAGENT_TEAM_AGENT_HANDLE", "helper")
	os.Setenv("OMNIAGENT_TEAM_SMTP_HOST", "smtp.example.com")
	os.Setenv("OMNIAGENT_TEAM_SMTP_PORT", "587")
	os.Setenv("OMNIAGENT_TEAM_SMTP_USERNAME", "smtp-user")
	os.Setenv("OMNIAGENT_TEAM_SMTP_PASSWORD", "smtp-pass")
	os.Setenv("OMNIAGENT_TEAM_SMTP_FROM", "agent@example.com")
	defer func() {
		os.Unsetenv("OMNIAGENT_TEAM_ENABLED")
		os.Unsetenv("OMNIAGENT_TEAM_DATABASE_APP_DSN")
		os.Unsetenv("OMNIAGENT_TEAM_DATABASE_MIGRATE_DSN")
		os.Unsetenv("OMNIAGENT_TEAM_DATABASE_APP_ROLE")
		os.Unsetenv("OMNIAGENT_TEAM_BASE_URL")
		os.Unsetenv("OMNIAGENT_TEAM_SUPERADMIN_EMAIL")
		os.Unsetenv("OMNIAGENT_TEAM_SUPERADMIN_PASSWORD")
		os.Unsetenv("OMNIAGENT_TEAM_AGENT_HANDLE")
		os.Unsetenv("OMNIAGENT_TEAM_SMTP_HOST")
		os.Unsetenv("OMNIAGENT_TEAM_SMTP_PORT")
		os.Unsetenv("OMNIAGENT_TEAM_SMTP_USERNAME")
		os.Unsetenv("OMNIAGENT_TEAM_SMTP_PASSWORD")
		os.Unsetenv("OMNIAGENT_TEAM_SMTP_FROM")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.Team.Enabled {
		t.Error("Team.Enabled = false, want true")
	}
	if cfg.Team.Database.AppDSN != "postgres://app@db/team" {
		t.Errorf("Team.Database.AppDSN = %s, want postgres://app@db/team", cfg.Team.Database.AppDSN)
	}
	if cfg.Team.Database.MigrateDSN != "postgres://owner@db/team" {
		t.Errorf("Team.Database.MigrateDSN = %s, want postgres://owner@db/team", cfg.Team.Database.MigrateDSN)
	}
	if cfg.Team.Database.AppRole != "custom_app_role" {
		t.Errorf("Team.Database.AppRole = %s, want custom_app_role", cfg.Team.Database.AppRole)
	}
	if cfg.Team.BaseURL != "https://team.example.com" {
		t.Errorf("Team.BaseURL = %s, want https://team.example.com", cfg.Team.BaseURL)
	}
	if cfg.Team.SuperadminPassword != "boot-passphrase" {
		t.Errorf("Team.SuperadminPassword = %s, want boot-passphrase", cfg.Team.SuperadminPassword)
	}
	if cfg.Team.SuperadminEmail != "root@example.com" {
		t.Errorf("Team.SuperadminEmail = %s, want root@example.com", cfg.Team.SuperadminEmail)
	}
	if cfg.Team.AgentHandle != "helper" {
		t.Errorf("Team.AgentHandle = %s, want helper", cfg.Team.AgentHandle)
	}
	if cfg.Team.SMTP.Host != "smtp.example.com" {
		t.Errorf("Team.SMTP.Host = %s, want smtp.example.com", cfg.Team.SMTP.Host)
	}
	if cfg.Team.SMTP.Port != 587 {
		t.Errorf("Team.SMTP.Port = %d, want 587", cfg.Team.SMTP.Port)
	}
	if cfg.Team.SMTP.Username != "smtp-user" {
		t.Errorf("Team.SMTP.Username = %s, want smtp-user", cfg.Team.SMTP.Username)
	}
	if cfg.Team.SMTP.Password != "smtp-pass" {
		t.Errorf("Team.SMTP.Password = %s, want smtp-pass", cfg.Team.SMTP.Password)
	}
	if cfg.Team.SMTP.From != "agent@example.com" {
		t.Errorf("Team.SMTP.From = %s, want agent@example.com", cfg.Team.SMTP.From)
	}
}

func TestLoadEnv_TeamSSO(t *testing.T) {
	os.Setenv("OMNIAGENT_TEAM_SSO_GOOGLE_CLIENT_ID", "google-id")
	os.Setenv("OMNIAGENT_TEAM_SSO_GOOGLE_CLIENT_SECRET", "google-secret")
	os.Setenv("OMNIAGENT_TEAM_SSO_GITHUB_CLIENT_ID", "github-id")
	os.Setenv("OMNIAGENT_TEAM_SSO_GITHUB_CLIENT_SECRET", "github-secret")
	defer func() {
		os.Unsetenv("OMNIAGENT_TEAM_SSO_GOOGLE_CLIENT_ID")
		os.Unsetenv("OMNIAGENT_TEAM_SSO_GOOGLE_CLIENT_SECRET")
		os.Unsetenv("OMNIAGENT_TEAM_SSO_GITHUB_CLIENT_ID")
		os.Unsetenv("OMNIAGENT_TEAM_SSO_GITHUB_CLIENT_SECRET")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Team.SSO.Google.ClientID != "google-id" {
		t.Errorf("Team.SSO.Google.ClientID = %s, want google-id", cfg.Team.SSO.Google.ClientID)
	}
	if cfg.Team.SSO.Google.ClientSecret != "google-secret" {
		t.Errorf("Team.SSO.Google.ClientSecret = %s, want google-secret", cfg.Team.SSO.Google.ClientSecret)
	}
	if cfg.Team.SSO.GitHub.ClientID != "github-id" {
		t.Errorf("Team.SSO.GitHub.ClientID = %s, want github-id", cfg.Team.SSO.GitHub.ClientID)
	}
	if cfg.Team.SSO.GitHub.ClientSecret != "github-secret" {
		t.Errorf("Team.SSO.GitHub.ClientSecret = %s, want github-secret", cfg.Team.SSO.GitHub.ClientSecret)
	}
}

func TestLoadEnv_AgentProviderSpecificKeys(t *testing.T) {
	tests := []struct {
		provider string
		envVar   string
		envValue string
	}{
		{"anthropic", "ANTHROPIC_API_KEY", "anthropic-key"},
		{"openai", "OPENAI_API_KEY", "openai-key"},
		{"gemini", "GEMINI_API_KEY", "gemini-key"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			os.Setenv("OMNIAGENT_AGENT_PROVIDER", tt.provider)
			os.Setenv(tt.envVar, tt.envValue)
			defer func() {
				os.Unsetenv("OMNIAGENT_AGENT_PROVIDER")
				os.Unsetenv(tt.envVar)
			}()

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}

			if cfg.Agent.APIKey != tt.envValue {
				t.Errorf("Agent.APIKey = %s, want %s", cfg.Agent.APIKey, tt.envValue)
			}
		})
	}
}

func TestLoadWithContext_InvalidVaultCredential(t *testing.T) {
	// Create temp config with invalid vault URI
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
agent:
  api_key: "invalid://scheme/secret"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("Load should fail for invalid vault URI in credentials")
	}
}

func TestExpandEnvVars(t *testing.T) {
	os.Setenv("TEST_EXPAND_VAR", "expanded-value")
	defer os.Unsetenv("TEST_EXPAND_VAR")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Dollar brace", "${TEST_EXPAND_VAR}", "expanded-value"},
		{"Dollar prefix", "$TEST_EXPAND_VAR", "expanded-value"},
		{"No var", "plain-string", "plain-string"},
		{"Mixed", "prefix-${TEST_EXPAND_VAR}-suffix", "prefix-expanded-value-suffix"},
		{"Nonexistent", "${NONEXISTENT_VAR_XYZ}", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandEnvVars(tt.input)
			if result != tt.expected {
				t.Errorf("ExpandEnvVars(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
