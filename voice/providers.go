package voice

import (
	"context"
	"fmt"
	"os"

	omnivoice "github.com/plexusone/omnivoice-core"
	"github.com/plexusone/omnivoice-core/gateway"
	"github.com/plexusone/omnivoice-core/registry"

	// Provider packages - imported for:
	// 1. Side-effect registration via init() with omnivoice-core registry
	// 2. Type-safe With* option functions for provider-specific configuration
	googleRealtime "github.com/plexusone/omni-google/omnivoice/realtime"
	openaiRealtime "github.com/plexusone/omni-openai/omnivoice/realtime"
	telnyxGateway "github.com/plexusone/omni-telnyx/omnivoice/gateway"
	twilioGateway "github.com/plexusone/omni-twilio/omnivoice/gateway"
)

// createTwilioGateway creates a Twilio voice gateway using the registry pattern.
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

	// Build options using type-safe registry options
	opts := []registry.ProviderOption{
		// Twilio credentials
		registry.WithAccountSID(cfg.TwilioAccountSID),
		registry.WithAuthToken(cfg.TwilioAuthToken),
		registry.WithPhoneNumber(cfg.TwilioPhone),

		// Server configuration
		registry.WithListenAddr(cfg.ListenAddr),
		registry.WithPublicURL(cfg.PublicURL),

		// STT configuration
		registry.WithSTTProvider(cfg.Config.STT.Provider),
		registry.WithSTTAPIKey(cfg.Config.STT.APIKey),
		registry.WithSTTModel(cfg.Config.STT.Model),
		registry.WithSTTLanguage(cfg.Config.STT.Language),

		// TTS configuration
		registry.WithTTSProvider(cfg.Config.TTS.Provider),
		registry.WithTTSAPIKey(cfg.Config.TTS.APIKey),
		registry.WithTTSVoiceID(cfg.Config.TTS.VoiceID),
		registry.WithTTSModel(cfg.Config.TTS.Model),

		// LLM configuration
		registry.WithLLMProvider(cfg.LLMProvider),
		registry.WithLLMModel(cfg.LLMModel),
		registry.WithLLMSystemPrompt(cfg.LLMSystemPrompt),

		// Session configuration
		registry.WithGreeting(cfg.Greeting),
		registry.WithMaxSessionDuration(cfg.MaxSessionDuration),
		registry.WithInterruptionMode(cfg.InterruptionMode),
		registry.WithLogger(cfg.Logger),

		// Type-safe provider-specific options
		twilioGateway.WithTools(twilioTools),
		twilioGateway.WithToolHandlers(twilioHandlers),
	}

	// Add listener if provided
	if cfg.Listener != nil {
		opts = append(opts, registry.WithListener(cfg.Listener))
	}

	// Configure realtime mode if specified
	if cfg.Mode == "realtime" {
		opts = append(opts, registry.WithPipelineMode("realtime"))

		// Create realtime provider factory
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

		opts = append(opts,
			twilioGateway.WithRealtimeProviderFactory(factory),
			twilioGateway.WithRealtimeConfig(&gateway.RealtimeConfig{
				Provider:     cfg.RealtimeProvider,
				APIKey:       apiKey,
				Model:        cfg.RealtimeModel,
				Voice:        cfg.RealtimeVoice,
				Instructions: cfg.LLMSystemPrompt,
			}),
		)
	}

	// Get gateway via registry (returns registry.Gateway, need to unwrap)
	gw, err := omnivoice.GetGatewayProvider("twilio", opts...)
	if err != nil {
		return nil, err
	}

	// The registry returns a wrapper; extract the underlying gateway if possible
	if wrapper, ok := gw.(interface{ Gateway() gateway.Gateway }); ok {
		return wrapper.Gateway(), nil
	}

	// Fallback: wrap the registry.Gateway to satisfy gateway.Gateway interface
	return &registryGatewayAdapter{gw}, nil
}

// createTelnyxGateway creates a Telnyx voice gateway using the registry pattern.
func createTelnyxGateway(cfg GatewayConfig) (gateway.Gateway, error) {
	// Convert tools to Telnyx gateway format
	var telnyxTools []telnyxGateway.ToolDefinition
	for _, t := range cfg.Tools {
		telnyxTools = append(telnyxTools, telnyxGateway.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}

	// Convert tool handlers
	var telnyxHandlers map[string]telnyxGateway.ToolHandler
	if cfg.ToolHandlers != nil {
		telnyxHandlers = make(map[string]telnyxGateway.ToolHandler)
		for name, handler := range cfg.ToolHandlers {
			h := handler // capture for closure
			telnyxHandlers[name] = func(ctx context.Context, args map[string]any) (string, error) {
				return h(ctx, args)
			}
		}
	}

	// Build options using type-safe registry options
	opts := []registry.ProviderOption{
		// Telnyx credentials
		registry.WithAPIKey(cfg.TelnyxAPIKey),
		registry.WithPhoneNumber(cfg.TelnyxPhone),
		registry.WithConnectionID(cfg.TelnyxConnectionID),

		// Server configuration
		registry.WithListenAddr(cfg.ListenAddr),
		registry.WithPublicURL(cfg.PublicURL),

		// STT configuration
		registry.WithSTTProvider(cfg.Config.STT.Provider),
		registry.WithSTTAPIKey(cfg.Config.STT.APIKey),
		registry.WithSTTModel(cfg.Config.STT.Model),
		registry.WithSTTLanguage(cfg.Config.STT.Language),

		// TTS configuration
		registry.WithTTSProvider(cfg.Config.TTS.Provider),
		registry.WithTTSAPIKey(cfg.Config.TTS.APIKey),
		registry.WithTTSVoiceID(cfg.Config.TTS.VoiceID),
		registry.WithTTSModel(cfg.Config.TTS.Model),

		// LLM configuration
		registry.WithLLMProvider(cfg.LLMProvider),
		registry.WithLLMModel(cfg.LLMModel),
		registry.WithLLMSystemPrompt(cfg.LLMSystemPrompt),

		// Session configuration
		registry.WithGreeting(cfg.Greeting),
		registry.WithMaxSessionDuration(cfg.MaxSessionDuration),
		registry.WithInterruptionMode(cfg.InterruptionMode),
		registry.WithLogger(cfg.Logger),

		// Type-safe provider-specific options
		telnyxGateway.WithTools(telnyxTools),
		telnyxGateway.WithToolHandlers(telnyxHandlers),
	}

	// Add listener if provided
	if cfg.Listener != nil {
		opts = append(opts, registry.WithListener(cfg.Listener))
	}

	// Get gateway via registry
	gw, err := omnivoice.GetGatewayProvider("telnyx", opts...)
	if err != nil {
		return nil, err
	}

	// The registry returns a wrapper; extract the underlying gateway if possible
	if wrapper, ok := gw.(interface{ Gateway() gateway.Gateway }); ok {
		return wrapper.Gateway(), nil
	}

	// Fallback: wrap the registry.Gateway to satisfy gateway.Gateway interface
	return &registryGatewayAdapter{gw}, nil
}

// registryGatewayAdapter adapts registry.Gateway to gateway.Gateway interface.
type registryGatewayAdapter struct {
	gw registry.Gateway
}

func (a *registryGatewayAdapter) Name() gateway.ProviderName {
	return gateway.ProviderName(a.gw.Name())
}

func (a *registryGatewayAdapter) Start(ctx context.Context) error {
	return a.gw.Start(ctx)
}

func (a *registryGatewayAdapter) Stop() error {
	return a.gw.Stop()
}

func (a *registryGatewayAdapter) OnCall(handler gateway.CallHandler) {
	// Not supported via adapter - requires concrete type
}

func (a *registryGatewayAdapter) MakeCall(ctx context.Context, to string) (gateway.Session, error) {
	return nil, fmt.Errorf("MakeCall not supported via registry adapter")
}

func (a *registryGatewayAdapter) GetSession(id string) (gateway.Session, bool) {
	return nil, false
}

func (a *registryGatewayAdapter) ListSessions() []gateway.Session {
	return nil
}
