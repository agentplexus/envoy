package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads configuration from a file and environment variables.
// Environment variables override file values.
// Vault-backed credentials (op://, bw://, file://, env://) are resolved automatically.
func Load(path string) (*Config, error) {
	return LoadWithContext(context.Background(), path)
}

// LoadWithContext reads configuration with a context for vault operations.
func LoadWithContext(ctx context.Context, path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		if err := loadFile(path, &cfg); err != nil {
			return nil, fmt.Errorf("load config file: %w", err)
		}
	}

	loadEnv(&cfg)

	// Resolve vault-backed credentials
	if err := cfg.ResolveCredentials(ctx); err != nil {
		return nil, fmt.Errorf("resolve credentials: %w", err)
	}

	return &cfg, nil
}

// loadFile reads configuration from a YAML or JSON file.
func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, cfg)
	case ".json":
		return json.Unmarshal(data, cfg)
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return json.Unmarshal(data, cfg)
		}
		return nil
	}
}

// loadEnv loads configuration from environment variables.
func loadEnv(cfg *Config) {
	// Gateway
	if v := os.Getenv("OMNIAGENT_GATEWAY_ADDRESS"); v != "" {
		cfg.Gateway.Address = v
	}

	// Agent
	if v := os.Getenv("OMNIAGENT_AGENT_PROVIDER"); v != "" {
		cfg.Agent.Provider = v
	}
	if v := os.Getenv("OMNIAGENT_AGENT_MODEL"); v != "" {
		cfg.Agent.Model = v
	}
	if v := os.Getenv("OMNIAGENT_AGENT_API_KEY"); v != "" {
		cfg.Agent.APIKey = v
	}
	if v := os.Getenv("OMNIAGENT_AGENT_SYSTEM_PROMPT"); v != "" {
		cfg.Agent.SystemPrompt = v
	}
	if v := os.Getenv("OMNIAGENT_AGENT_BASE_URL"); v != "" {
		cfg.Agent.BaseURL = v
	}
	// Also check provider-specific env vars
	if cfg.Agent.APIKey == "" {
		switch cfg.Agent.Provider {
		case "anthropic":
			cfg.Agent.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		case "openai":
			cfg.Agent.APIKey = os.Getenv("OPENAI_API_KEY")
		case "gemini":
			cfg.Agent.APIKey = os.Getenv("GEMINI_API_KEY")
		case "ollama":
			// Ollama doesn't require an API key - it's a local provider
		}
	}

	// Storage
	if v := os.Getenv("OMNIAGENT_STORAGE_TYPE"); v != "" {
		cfg.Storage.Type = v
	}
	if v := os.Getenv("OMNIAGENT_STORAGE_PATH"); v != "" {
		cfg.Storage.Path = v
	}
	if v := os.Getenv("OMNIAGENT_STORAGE_REDIS_URL"); v != "" {
		cfg.Storage.Redis.URL = v
	}

	// Sessions
	if v := os.Getenv("OMNIAGENT_SESSIONS_ENABLED"); v != "" {
		cfg.Sessions.Enabled = v == "true"
	}
	if v := os.Getenv("OMNIAGENT_SESSIONS_TTL"); v != "" {
		// Invalid values are ignored, same as other loadEnv fields that
		// don't currently surface a load-time error (e.g. SMTP port above).
		_ = cfg.Sessions.TTL.set(v)
	}

	// Telegram
	if v := os.Getenv("TELEGRAM_BOT_TOKEN"); v != "" {
		cfg.Channels.Telegram.Token = v
		cfg.Channels.Telegram.Enabled = true
	}

	// Discord
	if v := os.Getenv("DISCORD_BOT_TOKEN"); v != "" {
		cfg.Channels.Discord.Token = v
		cfg.Channels.Discord.Enabled = true
	}

	// WhatsApp
	if os.Getenv("WHATSAPP_ENABLED") == "true" {
		cfg.Channels.WhatsApp.Enabled = true
	}
	if v := os.Getenv("WHATSAPP_DB_PATH"); v != "" {
		cfg.Channels.WhatsApp.DBPath = v
	}

	// Twilio SMS
	if v := os.Getenv("TWILIO_ACCOUNT_SID"); v != "" {
		cfg.Channels.TwilioSMS.AccountSID = v
		cfg.Channels.TwilioSMS.Enabled = true
	}
	if v := os.Getenv("TWILIO_AUTH_TOKEN"); v != "" {
		cfg.Channels.TwilioSMS.AuthToken = v
	}
	if v := os.Getenv("TWILIO_PHONE_NUMBER"); v != "" {
		cfg.Channels.TwilioSMS.PhoneNumber = v
	}
	if v := os.Getenv("TWILIO_MESSAGING_SERVICE_SID"); v != "" {
		cfg.Channels.TwilioSMS.MessagingServiceSid = v
	}
	if v := os.Getenv("TWILIO_WEBHOOK_PATH"); v != "" {
		cfg.Channels.TwilioSMS.WebhookPath = v
	}

	// Voice
	if os.Getenv("OMNIAGENT_VOICE_ENABLED") == "true" {
		cfg.Voice.Enabled = true
	}
	if v := os.Getenv("OMNIAGENT_VOICE_RESPONSE_MODE"); v != "" {
		cfg.Voice.ResponseMode = v
	}
	// STT - check specific env var first, then fallback to DEEPGRAM_API_KEY
	if v := os.Getenv("OMNIAGENT_VOICE_STT_API_KEY"); v != "" {
		cfg.Voice.STT.APIKey = v
	} else if v := os.Getenv("DEEPGRAM_API_KEY"); v != "" {
		cfg.Voice.STT.APIKey = v
	}
	if v := os.Getenv("OMNIAGENT_VOICE_STT_MODEL"); v != "" {
		cfg.Voice.STT.Model = v
	}
	// TTS - check specific env var first, then fallback to DEEPGRAM_API_KEY
	if v := os.Getenv("OMNIAGENT_VOICE_TTS_API_KEY"); v != "" {
		cfg.Voice.TTS.APIKey = v
	} else if v := os.Getenv("DEEPGRAM_API_KEY"); v != "" {
		cfg.Voice.TTS.APIKey = v
	}
	if v := os.Getenv("OMNIAGENT_VOICE_TTS_MODEL"); v != "" {
		cfg.Voice.TTS.Model = v
	}
	if v := os.Getenv("OMNIAGENT_VOICE_TTS_VOICE_ID"); v != "" {
		cfg.Voice.TTS.VoiceID = v
	}

	// Team (multi-user) mode
	if os.Getenv("OMNIAGENT_TEAM_ENABLED") == "true" {
		cfg.Team.Enabled = true
	}
	if v := os.Getenv("OMNIAGENT_TEAM_DATABASE_APP_DSN"); v != "" {
		cfg.Team.Database.AppDSN = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_DATABASE_MIGRATE_DSN"); v != "" {
		cfg.Team.Database.MigrateDSN = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_DATABASE_APP_ROLE"); v != "" {
		cfg.Team.Database.AppRole = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_BASE_URL"); v != "" {
		cfg.Team.BaseURL = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SUPERADMIN_EMAIL"); v != "" {
		cfg.Team.SuperadminEmail = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SUPERADMIN_PASSWORD"); v != "" {
		cfg.Team.SuperadminPassword = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_AGENT_HANDLE"); v != "" {
		cfg.Team.AgentHandle = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SMTP_HOST"); v != "" {
		cfg.Team.SMTP.Host = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SMTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Team.SMTP.Port = port
		}
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SMTP_USERNAME"); v != "" {
		cfg.Team.SMTP.Username = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SMTP_PASSWORD"); v != "" {
		cfg.Team.SMTP.Password = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SMTP_FROM"); v != "" {
		cfg.Team.SMTP.From = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SSO_GOOGLE_CLIENT_ID"); v != "" {
		cfg.Team.SSO.Google.ClientID = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SSO_GOOGLE_CLIENT_SECRET"); v != "" {
		cfg.Team.SSO.Google.ClientSecret = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SSO_GITHUB_CLIENT_ID"); v != "" {
		cfg.Team.SSO.GitHub.ClientID = v
	}
	if v := os.Getenv("OMNIAGENT_TEAM_SSO_GITHUB_CLIENT_SECRET"); v != "" {
		cfg.Team.SSO.GitHub.ClientSecret = v
	}

	// Observability
	if v := os.Getenv("OMNIAGENT_OBSERVABILITY_PROVIDER"); v != "" {
		cfg.Observability.Provider = v
		cfg.Observability.Enabled = true
	}
	if v := os.Getenv("OMNIAGENT_OBSERVABILITY_ENDPOINT"); v != "" {
		cfg.Observability.Endpoint = v
	}
	if v := os.Getenv("OMNIAGENT_OBSERVABILITY_API_KEY"); v != "" {
		cfg.Observability.APIKey = v
	}
}

// ExpandEnvVars expands environment variables in string values.
// Supports ${VAR} and $VAR syntax.
func ExpandEnvVars(s string) string {
	return os.ExpandEnv(s)
}
