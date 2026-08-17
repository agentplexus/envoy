package config

import "time"

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		Gateway: GatewayConfig{
			Address:      "127.0.0.1:18789",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			PingInterval: 30 * time.Second,
		},
		Agent: AgentConfig{
			Provider:     "anthropic",
			Model:        "claude-sonnet-4-20250514",
			Temperature:  0.7,
			MaxTokens:    4096,
			SystemPrompt: "You are OmniAgent, a helpful AI assistant. You represent the user across communication channels, responding on their behalf with care and precision.\n\nYou have access to the following tools:\n- web_search: Search the web for current information, news, weather, or any real-time data.\n\nIMPORTANT: When users ask about current events, news, weather, prices, or anything that requires up-to-date information, you MUST use the web_search tool. Do not say you cannot search - use your tools.",
		},
		Storage: StorageConfig{
			Type: "sqlite",
			Path: DefaultStoragePath(),
		},
		Sessions: SessionsConfig{
			Enabled: true,
			// TTL left zero: the session store falls back to
			// sessions.DefaultSessionTTL (7 days) when unset.
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{
				Enabled: false,
			},
			Discord: DiscordConfig{
				Enabled: false,
			},
			WhatsApp: WhatsAppConfig{
				Enabled: false,
				DBPath:  "whatsapp.db",
			},
		},
		Tools: ToolsConfig{
			Browser: BrowserToolConfig{
				Enabled:  true,
				Headless: true,
			},
			Shell: ShellToolConfig{
				Enabled: false, // Disabled by default for security
			},
		},
		Skills: SkillsConfig{
			Enabled:     true,
			MaxInjected: 20,
		},
		Voice: VoiceConfig{
			Enabled:      false,
			ResponseMode: "auto",
			STT: STTConfig{
				Provider: "deepgram",
				Model:    "nova-2",
			},
			TTS: TTSConfig{
				Provider: "deepgram",
				Model:    "aura-asteria-en",
				VoiceID:  "aura-asteria-en",
			},
		},
		Observability: ObservabilityConfig{
			Enabled: false,
		},
	}
}
