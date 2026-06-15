package voice

import (
	"context"
	"fmt"
	"os"

	"github.com/plexusone/omnivoice-core/gateway"

	googleRealtime "github.com/plexusone/omni-google/omnivoice/realtime"
	openaiRealtime "github.com/plexusone/omni-openai/omnivoice/realtime"
	telnyxGateway "github.com/plexusone/omni-telnyx/omnivoice/gateway"
	twilioGateway "github.com/plexusone/omni-twilio/omnivoice/gateway"
)

// createTwilioGateway creates a Twilio voice gateway.
func createTwilioGateway(cfg GatewayConfig) (gateway.Gateway, error) {
	// Convert tools to Twilio gateway format
	var twilioTools []twilioGateway.ToolDefinition
	for _, t := range cfg.Tools {
		twilioTools = append(twilioTools, twilioGateway.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}

	// Convert tool handlers
	var twilioHandlers map[string]twilioGateway.ToolHandler
	if cfg.ToolHandlers != nil {
		twilioHandlers = make(map[string]twilioGateway.ToolHandler)
		for name, handler := range cfg.ToolHandlers {
			h := handler // capture for closure
			twilioHandlers[name] = func(ctx context.Context, args map[string]any) (string, error) {
				return h(ctx, args)
			}
		}
	}

	twilioCfg := twilioGateway.Config{
		AccountSID:         cfg.TwilioAccountSID,
		AuthToken:          cfg.TwilioAuthToken,
		PhoneNumber:        cfg.TwilioPhone,
		ListenAddr:         cfg.ListenAddr,
		PublicURL:          cfg.PublicURL,
		Listener:           cfg.Listener,
		STTProvider:        cfg.Config.STT.Provider,
		STTAPIKey:          cfg.Config.STT.APIKey,
		STTModel:           cfg.Config.STT.Model,
		STTLanguage:        cfg.Config.STT.Language,
		TTSProvider:        cfg.Config.TTS.Provider,
		TTSAPIKey:          cfg.Config.TTS.APIKey,
		TTSVoiceID:         cfg.Config.TTS.VoiceID,
		TTSModel:           cfg.Config.TTS.Model,
		LLMProvider:        cfg.LLMProvider,
		LLMModel:           cfg.LLMModel,
		LLMSystemPrompt:    cfg.LLMSystemPrompt,
		Tools:              twilioTools,
		ToolHandlers:       twilioHandlers,
		Greeting:           cfg.Greeting,
		MaxSessionDuration: cfg.MaxSessionDuration,
		InterruptionMode:   cfg.InterruptionMode,
		Logger:             cfg.Logger,
	}

	// Configure realtime mode if specified
	if cfg.Mode == "realtime" {
		twilioCfg.Mode = gateway.PipelineModeRealtime

		// Create realtime provider factory and config
		var factory gateway.RealtimeProviderFactory
		switch cfg.RealtimeProvider {
		case "openai":
			factory = openaiRealtime.NewFactory()
		case "gemini":
			factory = googleRealtime.NewFactory()
		default:
			return nil, fmt.Errorf("unsupported realtime provider: %s (supported: openai, gemini)", cfg.RealtimeProvider)
		}

		// Resolve API key from environment if not provided
		apiKey := cfg.RealtimeAPIKey
		if apiKey == "" {
			switch cfg.RealtimeProvider {
			case "openai":
				apiKey = os.Getenv("OPENAI_API_KEY")
			case "gemini":
				apiKey = os.Getenv("GEMINI_API_KEY")
				if apiKey == "" {
					apiKey = os.Getenv("GOOGLE_API_KEY")
				}
			}
		}

		twilioCfg.RealtimeProvider = factory
		twilioCfg.RealtimeConfig = &gateway.RealtimeConfig{
			Provider:     cfg.RealtimeProvider,
			APIKey:       apiKey,
			Model:        cfg.RealtimeModel,
			Voice:        cfg.RealtimeVoice,
			Instructions: cfg.LLMSystemPrompt,
		}
	}

	return twilioGateway.New(twilioCfg)
}

// createTelnyxGateway creates a Telnyx voice gateway.
func createTelnyxGateway(cfg GatewayConfig) (gateway.Gateway, error) {
	return telnyxGateway.New(telnyxGateway.Config{
		APIKey:             cfg.TelnyxAPIKey,
		PhoneNumber:        cfg.TelnyxPhone,
		ConnectionID:       cfg.TelnyxConnectionID,
		ListenAddr:         cfg.ListenAddr,
		PublicURL:          cfg.PublicURL,
		STTProvider:        cfg.Config.STT.Provider,
		STTAPIKey:          cfg.Config.STT.APIKey,
		STTModel:           cfg.Config.STT.Model,
		STTLanguage:        cfg.Config.STT.Language,
		TTSProvider:        cfg.Config.TTS.Provider,
		TTSAPIKey:          cfg.Config.TTS.APIKey,
		TTSVoiceID:         cfg.Config.TTS.VoiceID,
		TTSModel:           cfg.Config.TTS.Model,
		LLMProvider:        cfg.LLMProvider,
		LLMModel:           cfg.LLMModel,
		LLMSystemPrompt:    cfg.LLMSystemPrompt,
		MaxSessionDuration: cfg.MaxSessionDuration,
		InterruptionMode:   cfg.InterruptionMode,
		Logger:             cfg.Logger,
	})
}

