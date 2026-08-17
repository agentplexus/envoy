// Package config provides configuration types and loading for omniagent.
package config

import "time"

// Config is the root configuration for omniagent.
type Config struct {
	Gateway       GatewayConfig       `json:"gateway" yaml:"gateway"`
	Agent         AgentConfig         `json:"agent" yaml:"agent"`
	Agents        []AgentConfig       `json:"agents,omitempty" yaml:"agents,omitempty"` // Multi-agent configs
	Storage       StorageConfig       `json:"storage" yaml:"storage"`
	Sessions      SessionsConfig      `json:"sessions" yaml:"sessions"`
	Auth          AuthConfig          `json:"auth" yaml:"auth"`
	Web           WebConfig           `json:"web" yaml:"web"`
	Team          TeamConfig          `json:"team" yaml:"team"`
	Channels      ChannelsConfig      `json:"channels" yaml:"channels"`
	Tools         ToolsConfig         `json:"tools" yaml:"tools"`
	Skills        SkillsConfig        `json:"skills" yaml:"skills"`
	Memory        MemoryConfig        `json:"memory" yaml:"memory"`
	Voice         VoiceConfig         `json:"voice" yaml:"voice"`
	Image         ImageConfig         `json:"image" yaml:"image"`
	Observability ObservabilityConfig `json:"observability" yaml:"observability"`
	Tokens        TokenConfig         `json:"tokens" yaml:"tokens"`
	// Secrets holds global secret bindings available to all skills —
	// env-var name -> plain value or vault URI (op://, bw://, file://,
	// env://) — resolved in place by ResolveCredentials. Per-skill
	// overrides live in Skills.Config[name].Secrets and take precedence
	// (RMI-OMNIAGENT-201/202).
	Secrets map[string]string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
}

// MemoryConfig configures the omnimemory integration.
type MemoryConfig struct {
	Enabled   bool           `json:"enabled" yaml:"enabled"`
	Provider  string         `json:"provider" yaml:"provider"`     // memory, postgres, kvs, mem0, twilio
	DSN       string         `json:"dsn" yaml:"dsn"`               // Database connection string (postgres)
	APIKey    string         `json:"api_key" yaml:"api_key"`       // API key (mem0, twilio) //nolint:gosec // G117: APIKey loaded from config file
	Endpoint  string         `json:"endpoint" yaml:"endpoint"`     // API endpoint (mem0, twilio)
	TenantID  string         `json:"tenant_id" yaml:"tenant_id"`   // Default tenant for this agent
	AgentID   string         `json:"agent_id" yaml:"agent_id"`     // Agent identifier
	Options   map[string]any `json:"options" yaml:"options"`       // Provider-specific options
	Embedder  EmbedderConfig `json:"embedder" yaml:"embedder"`     // Optional embedder configuration
	RecallMax int            `json:"recall_max" yaml:"recall_max"` // Max memories to recall per request
}

// EmbedderConfig configures the embedding model for semantic memory.
type EmbedderConfig struct {
	Provider string `json:"provider" yaml:"provider"` // openai, bedrock, etc.
	Model    string `json:"model" yaml:"model"`       // text-embedding-3-small, etc.
	APIKey   string `json:"api_key" yaml:"api_key"`   //nolint:gosec // G117: APIKey loaded from config file
}

// GatewayConfig configures the WebSocket gateway.
type GatewayConfig struct {
	Address      string        `json:"address" yaml:"address"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	PingInterval time.Duration `json:"ping_interval" yaml:"ping_interval"`
}

// SessionsConfig configures session behavior.
type SessionsConfig struct {
	// Enabled turns on persistent session history for the gateway's
	// Discord/Telegram/WebSocket message path (RMI-OMNIAGENT-007). Defaults
	// to true; set false to keep today's stateless-per-message behavior
	// even when Storage is configured.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// TTL is how long an idle session is kept before expiring. Zero means
	// the session store's own default (sessions.DefaultSessionTTL, 7 days).
	TTL Duration `json:"ttl" yaml:"ttl"`

	// Rollover configures automatic session rollover.
	Rollover RolloverConfig `json:"rollover" yaml:"rollover"`
}

// StorageConfig configures the persistent backend used for session history
// and (single-agent) cron job state (RMI-OMNIAGENT-007).
type StorageConfig struct {
	// Type selects the backend: "sqlite" (default), "redis", or "memory".
	Type string `json:"type" yaml:"type"`

	// Path is the SQLite database file path. Used only when Type is
	// "sqlite" (or empty). Defaults to DefaultStoragePath().
	Path string `json:"path" yaml:"path"`

	// Redis configures the Redis backend. Required when Type is "redis".
	Redis RedisConfig `json:"redis" yaml:"redis"`
}

// RedisConfig configures a Redis-backed storage connection.
type RedisConfig struct {
	// URL is the Redis connection URL, e.g. "redis://localhost:6379".
	URL string `json:"url" yaml:"url"`
}

// RolloverConfig configures automatic session rollover: when triggered, a
// session's conversation ends (persisted to memory when memory is enabled)
// and continues fresh under the same session ID.
type RolloverConfig struct {
	// Enabled turns automatic rollover on. At least one of IdleTimeout or
	// Daily must be set when enabled.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// IdleTimeout rolls a session over when it has been inactive longer
	// than this duration (e.g. "4h"). Zero disables idle rollover.
	IdleTimeout Duration `json:"idle_timeout" yaml:"idle_timeout"`

	// Daily rolls a session over when a calendar-day boundary is crossed.
	Daily bool `json:"daily" yaml:"daily"`

	// Timezone resolves the day boundary (IANA name). Empty falls back to
	// the agent's timezone, then UTC.
	Timezone string `json:"timezone" yaml:"timezone"`
}

// AgentConfig configures the AI agent.
type AgentConfig struct {
	ID           string   `json:"id,omitempty" yaml:"id,omitempty"`
	Name         string   `json:"name,omitempty" yaml:"name,omitempty"`
	Description  string   `json:"description,omitempty" yaml:"description,omitempty"`
	Provider     string   `json:"provider" yaml:"provider"`
	Model        string   `json:"model" yaml:"model"`
	APIKey       string   `json:"api_key" yaml:"api_key"` //nolint:gosec // G117: APIKey loaded from config file
	BaseURL      string   `json:"base_url" yaml:"base_url"`
	Temperature  float64  `json:"temperature" yaml:"temperature"`
	MaxTokens    int      `json:"max_tokens" yaml:"max_tokens"`
	SystemPrompt string   `json:"system_prompt" yaml:"system_prompt"`
	Timezone     string   `json:"timezone,omitempty" yaml:"timezone,omitempty"` // IANA timezone for temporal context (empty = UTC)
	AllowedTools []string `json:"allowed_tools,omitempty" yaml:"allowed_tools,omitempty"`
	DeniedTools  []string `json:"denied_tools,omitempty" yaml:"denied_tools,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// IsEnabled returns whether the agent is enabled.
// Defaults to true if Enabled is nil.
func (c *AgentConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// ChannelsConfig configures messaging channels.
type ChannelsConfig struct {
	Telegram  TelegramConfig  `json:"telegram" yaml:"telegram"`
	Discord   DiscordConfig   `json:"discord" yaml:"discord"`
	WhatsApp  WhatsAppConfig  `json:"whatsapp" yaml:"whatsapp"`
	TwilioSMS TwilioSMSConfig `json:"twilio_sms" yaml:"twilio_sms"`
}

// WhatsAppConfig configures the WhatsApp channel.
type WhatsAppConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	DBPath  string `json:"db_path" yaml:"db_path"`
}

// TelegramConfig configures the Telegram channel.
type TelegramConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Token   string `json:"token" yaml:"token"`
}

// DiscordConfig configures the Discord channel.
type DiscordConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Token   string `json:"token" yaml:"token"`
	GuildID string `json:"guild_id" yaml:"guild_id"`
}

// TwilioSMSConfig configures the Twilio SMS channel.
type TwilioSMSConfig struct {
	Enabled             bool   `json:"enabled" yaml:"enabled"`
	AccountSID          string `json:"account_sid" yaml:"account_sid"`
	AuthToken           string `json:"auth_token" yaml:"auth_token"` //nolint:gosec // G101: Auth token loaded from config file
	PhoneNumber         string `json:"phone_number" yaml:"phone_number"`
	MessagingServiceSid string `json:"messaging_service_sid" yaml:"messaging_service_sid"` // For RCS
	WebhookPath         string `json:"webhook_path" yaml:"webhook_path"`                   // Default: /webhook/twilio/sms
}

// ToolsConfig configures available tools.
type ToolsConfig struct {
	Browser BrowserToolConfig `json:"browser" yaml:"browser"`
	Shell   ShellToolConfig   `json:"shell" yaml:"shell"`
}

// BrowserToolConfig configures the browser automation tool.
type BrowserToolConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Headless bool   `json:"headless" yaml:"headless"`
	UserData string `json:"user_data" yaml:"user_data"`
}

// ShellToolConfig configures the shell execution tool.
type ShellToolConfig struct {
	Enabled    bool     `json:"enabled" yaml:"enabled"`
	WorkingDir string   `json:"working_dir" yaml:"working_dir"`
	Allowlist  []string `json:"allowlist" yaml:"allowlist"`
}

// SkillsConfig configures skill loading.
type SkillsConfig struct {
	Enabled     bool     `json:"enabled" yaml:"enabled"`
	Packs       []string `json:"packs" yaml:"packs"`       // Skill pack names (e.g., "omniagent-skills")
	Paths       []string `json:"paths" yaml:"paths"`       // Directories to search (alias: dirs)
	Includes    []string `json:"includes" yaml:"includes"` // Only load these skills
	Excludes    []string `json:"excludes" yaml:"excludes"` // Skip these skills
	Disabled    []string `json:"disabled" yaml:"disabled"` // Deprecated: use excludes
	MaxInjected int      `json:"max_injected" yaml:"max_injected"`
	// Config holds per-skill settings keyed by skill name, mirroring
	// Tokens.Services' map-by-name shape (RMI-OMNIAGENT-202).
	Config map[string]SkillConfig `json:"config,omitempty" yaml:"config,omitempty"`
}

// SkillConfig is one skill's config.yaml overrides.
type SkillConfig struct {
	// Secrets binds this skill's declared secret env-var names to plain
	// values or vault URIs, resolved in place by ResolveCredentials.
	// Takes precedence over the same key in the root Config.Secrets.
	Secrets map[string]string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
}

// VoiceConfig configures voice processing.
type VoiceConfig struct {
	Enabled      bool      `json:"enabled" yaml:"enabled"`
	ResponseMode string    `json:"response_mode" yaml:"response_mode"`
	STT          STTConfig `json:"stt" yaml:"stt"`
	TTS          TTSConfig `json:"tts" yaml:"tts"`
}

// STTConfig configures speech-to-text.
type STTConfig struct {
	Provider string `json:"provider" yaml:"provider"`
	APIKey   string `json:"api_key" yaml:"api_key"` //nolint:gosec // G117: APIKey loaded from config file
	Model    string `json:"model" yaml:"model"`
	Language string `json:"language" yaml:"language"`
}

// TTSConfig configures text-to-speech.
type TTSConfig struct {
	Provider string `json:"provider" yaml:"provider"`
	APIKey   string `json:"api_key" yaml:"api_key"` //nolint:gosec // G117: APIKey loaded from config file
	Model    string `json:"model" yaml:"model"`
	VoiceID  string `json:"voice_id" yaml:"voice_id"`
}

// ObservabilityConfig configures observability features.
type ObservabilityConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Provider string `json:"provider" yaml:"provider"`
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	APIKey   string `json:"api_key" yaml:"api_key"` //nolint:gosec // G117: APIKey loaded from config file
}

// ImageConfig configures image generation via OmniImage.
type ImageConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Provider string `json:"provider" yaml:"provider"` // openai, fal
	Model    string `json:"model" yaml:"model"`       // Default model (e.g., gpt-image-2, fal-ai/flux-pro)
	APIKey   string `json:"api_key" yaml:"api_key"`   //nolint:gosec // G117: APIKey loaded from config file
	BaseURL  string `json:"base_url" yaml:"base_url"` // Optional custom base URL
}
