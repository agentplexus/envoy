// Package config provides configuration types and loading for omniagent.
package config

import "time"

// Config is the root configuration for omniagent.
type Config struct {
	Gateway       GatewayConfig       `json:"gateway" yaml:"gateway"`
	Agent         AgentConfig         `json:"agent" yaml:"agent"`
	Channels      ChannelsConfig      `json:"channels" yaml:"channels"`
	Tools         ToolsConfig         `json:"tools" yaml:"tools"`
	Skills        SkillsConfig        `json:"skills" yaml:"skills"`
	Voice         VoiceConfig         `json:"voice" yaml:"voice"`
	Observability ObservabilityConfig `json:"observability" yaml:"observability"`
}

// GatewayConfig configures the WebSocket gateway.
type GatewayConfig struct {
	Address      string        `json:"address" yaml:"address"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	PingInterval time.Duration `json:"ping_interval" yaml:"ping_interval"`
}

// AgentConfig configures the AI agent.
type AgentConfig struct {
	Provider     string  `json:"provider" yaml:"provider"`
	Model        string  `json:"model" yaml:"model"`
	APIKey       string  `json:"api_key" yaml:"api_key"` //nolint:gosec // G117: APIKey loaded from config file
	BaseURL      string  `json:"base_url" yaml:"base_url"`
	Temperature  float64 `json:"temperature" yaml:"temperature"`
	MaxTokens    int     `json:"max_tokens" yaml:"max_tokens"`
	SystemPrompt string  `json:"system_prompt" yaml:"system_prompt"`
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
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	AccountSID  string `json:"account_sid" yaml:"account_sid"`
	AuthToken   string `json:"auth_token" yaml:"auth_token"` //nolint:gosec // G101: Auth token loaded from config file
	PhoneNumber string `json:"phone_number" yaml:"phone_number"`
	WebhookPath string `json:"webhook_path" yaml:"webhook_path"` // Default: /webhook/twilio/sms
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
	Packs       []string `json:"packs" yaml:"packs"`             // Skill pack names (e.g., "omniagent-skills")
	Paths       []string `json:"paths" yaml:"paths"`             // Directories to search (alias: dirs)
	Includes    []string `json:"includes" yaml:"includes"`       // Only load these skills
	Excludes    []string `json:"excludes" yaml:"excludes"`       // Skip these skills
	Disabled    []string `json:"disabled" yaml:"disabled"`       // Deprecated: use excludes
	MaxInjected int      `json:"max_injected" yaml:"max_injected"`
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
