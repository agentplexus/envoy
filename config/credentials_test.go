package config

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestIsUnknownVaultURI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Invalid scheme", "invalid://path", true},
		{"HTTP URL", "https://example.com", true},
		{"FTP URL", "ftp://server/path", true},
		{"Keeper URI (no provider registered)", "keeper://record/field", true},
		{"1Password URI", "op://MyVault/item", false},
		{"Bitwarden URI", "bw://org-id/secret", false},
		{"Env URI", "env://VAR_NAME", false},
		{"Plain string", "just-a-string", false},
		{"Empty string", "", false},
		{"Partial scheme", "op:/something", false},
		{"No scheme", "no-scheme-here", false},
		{"Colon only", ":", false},
		{"Double colon only", "::", false},
		{"Starts with colon", "://path", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUnknownVaultURI(tt.input)
			if result != tt.expected {
				t.Errorf("isUnknownVaultURI(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsVaultURI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"1Password URI", "op://MyVault/item/field", true},
		{"Bitwarden URI", "bw://org-id/secret", true},
		{"Keeper URI (no provider registered)", "keeper://record/field", false},
		{"File URI", "file:///path/to/secret", true},
		{"Env URI", "env://VAR_NAME", true},
		{"Plain string", "sk-ant-api03-xxx", false},
		{"Empty string", "", false},
		{"HTTP URL", "https://example.com", false},
		{"Partial scheme", "op:/", false},
		{"No path", "op://", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isVaultURI(tt.input)
			if result != tt.expected {
				t.Errorf("isVaultURI(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetScheme(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"1Password", "op://MyVault/item", "op"},
		{"Bitwarden", "bw://org-id/secret", "bw"},
		{"Keeper", "keeper://record", "keeper"},
		{"File", "file:///path/to/secret", "file"},
		{"Env", "env://VAR_NAME", "env"},
		{"No scheme", "plain-string", ""},
		{"Partial", "op:/something", ""},
		{"Empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getScheme(tt.input)
			if result != tt.expected {
				t.Errorf("getScheme(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"1Password full", "op://MyVault/item/field", "MyVault/item/field"},
		{"Bitwarden", "bw://org-id/secret", "org-id/secret"},
		{"Keeper", "keeper://record", "record"},
		{"File", "file:///path/to/secret", "/path/to/secret"},
		{"Env", "env://VAR_NAME", "VAR_NAME"},
		{"No scheme", "plain-string", "plain-string"},
		{"Empty path", "op://", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPath(tt.input)
			if result != tt.expected {
				t.Errorf("getPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

//nolint:gosec // G101: Test fixtures with fake credentials
func TestResolveCredentials_NoVaultURIs(t *testing.T) {
	// Test that plain string values are not modified
	cfg := &Config{
		Agent: AgentConfig{
			APIKey: "sk-ant-api03-plainkey",
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{
				Token: "123456:ABC-DEF",
			},
			Discord: DiscordConfig{
				Token: "discord-bot-token",
			},
		},
	}

	ctx := context.Background()
	err := cfg.ResolveCredentials(ctx)
	if err != nil {
		t.Errorf("ResolveCredentials() error = %v, want nil", err)
	}

	// Values should remain unchanged
	if cfg.Agent.APIKey != "sk-ant-api03-plainkey" {
		t.Errorf("Agent.APIKey = %q, want %q", cfg.Agent.APIKey, "sk-ant-api03-plainkey")
	}
	if cfg.Channels.Telegram.Token != "123456:ABC-DEF" {
		t.Errorf("Channels.Telegram.Token = %q, want %q", cfg.Channels.Telegram.Token, "123456:ABC-DEF")
	}
	if cfg.Channels.Discord.Token != "discord-bot-token" {
		t.Errorf("Channels.Discord.Token = %q, want %q", cfg.Channels.Discord.Token, "discord-bot-token")
	}
}

func TestResolveCredentials_EmptyValues(t *testing.T) {
	// Test that empty values are handled correctly
	cfg := &Config{
		Agent: AgentConfig{
			APIKey: "",
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{
				Token: "",
			},
		},
	}

	ctx := context.Background()
	err := cfg.ResolveCredentials(ctx)
	if err != nil {
		t.Errorf("ResolveCredentials() error = %v, want nil", err)
	}

	// Empty values should remain empty
	if cfg.Agent.APIKey != "" {
		t.Errorf("Agent.APIKey = %q, want empty", cfg.Agent.APIKey)
	}
}

//nolint:gosec // G101: Test fixtures with fake credentials and vault URIs
func TestResolveCredentials_EnvVault(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_AGENT_API_KEY", "test-anthropic-key")
	os.Setenv("TEST_TELEGRAM_TOKEN", "123456:ABC-telegram")
	os.Setenv("TEST_DISCORD_TOKEN", "discord-bot-token-xyz")
	os.Setenv("TEST_TWILIO_SID", "AC1234567890")
	os.Setenv("TEST_TWILIO_AUTH", "twilio-auth-secret")
	os.Setenv("TEST_STT_KEY", "deepgram-stt-key")
	os.Setenv("TEST_TTS_KEY", "deepgram-tts-key")
	os.Setenv("TEST_OBS_KEY", "observability-key")
	defer func() {
		os.Unsetenv("TEST_AGENT_API_KEY")
		os.Unsetenv("TEST_TELEGRAM_TOKEN")
		os.Unsetenv("TEST_DISCORD_TOKEN")
		os.Unsetenv("TEST_TWILIO_SID")
		os.Unsetenv("TEST_TWILIO_AUTH")
		os.Unsetenv("TEST_STT_KEY")
		os.Unsetenv("TEST_TTS_KEY")
		os.Unsetenv("TEST_OBS_KEY")
	}()

	cfg := &Config{
		Agent: AgentConfig{
			APIKey: "env://TEST_AGENT_API_KEY",
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{
				Token: "env://TEST_TELEGRAM_TOKEN",
			},
			Discord: DiscordConfig{
				Token: "env://TEST_DISCORD_TOKEN",
			},
			TwilioSMS: TwilioSMSConfig{
				AccountSID: "env://TEST_TWILIO_SID",
				AuthToken:  "env://TEST_TWILIO_AUTH",
			},
		},
		Voice: VoiceConfig{
			STT: STTConfig{
				APIKey: "env://TEST_STT_KEY",
			},
			TTS: TTSConfig{
				APIKey: "env://TEST_TTS_KEY",
			},
		},
		Observability: ObservabilityConfig{
			APIKey: "env://TEST_OBS_KEY",
		},
	}

	ctx := context.Background()
	err := cfg.ResolveCredentials(ctx)
	if err != nil {
		t.Fatalf("ResolveCredentials() error = %v", err)
	}

	// Verify all values were resolved
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"Agent.APIKey", cfg.Agent.APIKey, "test-anthropic-key"},
		{"Channels.Telegram.Token", cfg.Channels.Telegram.Token, "123456:ABC-telegram"},
		{"Channels.Discord.Token", cfg.Channels.Discord.Token, "discord-bot-token-xyz"},
		{"Channels.TwilioSMS.AccountSID", cfg.Channels.TwilioSMS.AccountSID, "AC1234567890"},
		{"Channels.TwilioSMS.AuthToken", cfg.Channels.TwilioSMS.AuthToken, "twilio-auth-secret"},
		{"Voice.STT.APIKey", cfg.Voice.STT.APIKey, "deepgram-stt-key"},
		{"Voice.TTS.APIKey", cfg.Voice.TTS.APIKey, "deepgram-tts-key"},
		{"Observability.APIKey", cfg.Observability.APIKey, "observability-key"},
	}

	for _, tt := range tests {
		if tt.got != tt.expected {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
		}
	}
}

//nolint:gosec // G101: Test fixtures with vault URIs and fake credentials
func TestResolveCredentials_MixedVaultAndPlain(t *testing.T) {
	// Test mixing vault URIs and plain strings
	os.Setenv("TEST_MIXED_KEY", "vault-resolved-key")
	defer os.Unsetenv("TEST_MIXED_KEY")

	cfg := &Config{
		Agent: AgentConfig{
			APIKey: "env://TEST_MIXED_KEY", // Vault URI
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{
				Token: "plain-telegram-token", // Plain string
			},
			Discord: DiscordConfig{
				Token: "env://TEST_MIXED_KEY", // Vault URI (same var)
			},
		},
	}

	ctx := context.Background()
	err := cfg.ResolveCredentials(ctx)
	if err != nil {
		t.Fatalf("ResolveCredentials() error = %v", err)
	}

	if cfg.Agent.APIKey != "vault-resolved-key" {
		t.Errorf("Agent.APIKey = %q, want %q", cfg.Agent.APIKey, "vault-resolved-key")
	}
	if cfg.Channels.Telegram.Token != "plain-telegram-token" {
		t.Errorf("Telegram.Token = %q, want %q", cfg.Channels.Telegram.Token, "plain-telegram-token")
	}
	if cfg.Channels.Discord.Token != "vault-resolved-key" {
		t.Errorf("Discord.Token = %q, want %q", cfg.Channels.Discord.Token, "vault-resolved-key")
	}
}

//nolint:gosec // G101: Test fixture with intentionally invalid vault URI
func TestResolveCredentials_InvalidVaultURI(t *testing.T) {
	// Test that invalid vault URIs return errors
	cfg := &Config{
		Agent: AgentConfig{
			APIKey: "invalid://not-a-real-scheme/secret",
		},
	}

	ctx := context.Background()
	err := cfg.ResolveCredentials(ctx)
	if err == nil {
		t.Error("ResolveCredentials() should return error for invalid vault URI")
	}
}

//nolint:gosec // G101: Test fixture with vault URI reference
func TestResolveCredentials_EnvVarNotSet(t *testing.T) {
	// Test that missing env var returns error
	cfg := &Config{
		Agent: AgentConfig{
			APIKey: "env://NONEXISTENT_VAR_12345",
		},
	}

	ctx := context.Background()
	err := cfg.ResolveCredentials(ctx)
	if err == nil {
		t.Error("ResolveCredentials() should return error for missing env var")
	}
}

func TestCredentialResolver_CachesVaults(t *testing.T) {
	// Test that the resolver caches vaults by scheme
	resolver := newCredentialResolver()
	defer resolver.close()

	// Get vault twice for same scheme
	v1, err := resolver.getVault("env")
	if err != nil {
		t.Fatalf("getVault(env) first call error = %v", err)
	}

	v2, err := resolver.getVault("env")
	if err != nil {
		t.Fatalf("getVault(env) second call error = %v", err)
	}

	// Should be the same instance (pointer equality)
	if v1 != v2 {
		t.Error("getVault should return cached vault instance")
	}
}

//nolint:gosec // G101: Test fixtures with fake credentials and vault URIs
func TestResolveCredentials_GlobalSecrets(t *testing.T) {
	os.Setenv("TEST_GLOBAL_SECRET", "resolved-github-token")
	defer os.Unsetenv("TEST_GLOBAL_SECRET")

	cfg := &Config{
		Secrets: map[string]string{
			"GITHUB_TOKEN": "env://TEST_GLOBAL_SECRET",
			"PLAIN_VALUE":  "already-a-value",
		},
	}

	if err := cfg.ResolveCredentials(context.Background()); err != nil {
		t.Fatalf("ResolveCredentials() error = %v", err)
	}
	if got := cfg.Secrets["GITHUB_TOKEN"]; got != "resolved-github-token" {
		t.Errorf("Secrets[GITHUB_TOKEN] = %q, want %q", got, "resolved-github-token")
	}
	if got := cfg.Secrets["PLAIN_VALUE"]; got != "already-a-value" {
		t.Errorf("Secrets[PLAIN_VALUE] = %q, want unchanged", got)
	}
}

//nolint:gosec // G101: Test fixtures with fake credentials and vault URIs
func TestResolveCredentials_SkillSecrets(t *testing.T) {
	os.Setenv("TEST_SKILL_SECRET", "resolved-skill-token")
	defer os.Unsetenv("TEST_SKILL_SECRET")

	cfg := &Config{
		Skills: SkillsConfig{
			Config: map[string]SkillConfig{
				"github": {Secrets: map[string]string{
					"GITHUB_TOKEN": "env://TEST_SKILL_SECRET",
				}},
			},
		},
	}

	if err := cfg.ResolveCredentials(context.Background()); err != nil {
		t.Fatalf("ResolveCredentials() error = %v", err)
	}
	if got := cfg.Skills.Config["github"].Secrets["GITHUB_TOKEN"]; got != "resolved-skill-token" {
		t.Errorf("Skills.Config[github].Secrets[GITHUB_TOKEN] = %q, want %q", got, "resolved-skill-token")
	}
}

//nolint:gosec // G101: Test fixture with intentionally unsupported vault scheme
func TestResolveCredentials_SecretsUnknownScheme(t *testing.T) {
	cfg := &Config{
		Secrets: map[string]string{"GITHUB_TOKEN": "keeper://record/field"},
	}
	err := cfg.ResolveCredentials(context.Background())
	if err == nil {
		t.Fatal("ResolveCredentials() should return error for keeper:// (no provider registered)")
	}
	if !strings.Contains(err.Error(), "secrets.GITHUB_TOKEN") {
		t.Errorf("error = %v, want it to name secrets.GITHUB_TOKEN", err)
	}
}

//nolint:gosec // G101: Test fixture with intentionally missing env var
func TestResolveCredentials_SkillSecretsErrorPath(t *testing.T) {
	cfg := &Config{
		Skills: SkillsConfig{
			Config: map[string]SkillConfig{
				"github": {Secrets: map[string]string{
					"GITHUB_TOKEN": "env://NONEXISTENT_SKILL_SECRET_VAR_12345",
				}},
			},
		},
	}
	err := cfg.ResolveCredentials(context.Background())
	if err == nil {
		t.Fatal("ResolveCredentials() should return error for missing env var")
	}
	if !strings.Contains(err.Error(), "skills.config.github.secrets.GITHUB_TOKEN") {
		t.Errorf("error = %v, want it to name skills.config.github.secrets.GITHUB_TOKEN", err)
	}
}

func TestCredentialResolver_DifferentSchemes(t *testing.T) {
	// Test that different schemes get different vaults
	resolver := newCredentialResolver()
	defer resolver.close()

	vEnv, err := resolver.getVault("env")
	if err != nil {
		t.Fatalf("getVault(env) error = %v", err)
	}

	vMemory, err := resolver.getVault("memory")
	if err != nil {
		t.Fatalf("getVault(memory) error = %v", err)
	}

	// Should be different instances
	if vEnv == vMemory {
		t.Error("Different schemes should return different vault instances")
	}
}
