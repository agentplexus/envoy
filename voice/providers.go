package voice

import (
	"context"
	"fmt"
	"os"

	"github.com/plexusone/omnivoice"
	"github.com/plexusone/omnivoice-core/gateway"
	"github.com/plexusone/omnivoice-core/registry"

	// Import providers/all for side-effect registration
	_ "github.com/plexusone/omnivoice/providers/all"
)

// convertTools converts gateway tools to omnivoice format.
func convertTools(tools []ToolDefinition) []omnivoice.ToolDefinition {
	result := make([]omnivoice.ToolDefinition, len(tools))
	for i, t := range tools {
		result[i] = omnivoice.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}
	}
	return result
}

// convertHandlers converts tool handlers to omnivoice format.
func convertHandlers(handlers map[string]ToolHandler) map[string]omnivoice.ToolHandler {
	if handlers == nil {
		return nil
	}
	result := make(map[string]omnivoice.ToolHandler, len(handlers))
	for name, handler := range handlers {
		h := handler // capture for closure
		result[name] = func(ctx context.Context, args map[string]any) (string, error) {
			return h(ctx, args)
		}
	}
	return result
}

// buildCommonGatewayOptions builds provider options common to all gateways.
func buildCommonGatewayOptions(cfg GatewayConfig) []registry.ProviderOption {
	return []registry.ProviderOption{
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
	}
}

// extractGateway extracts the gateway.Gateway from a registry.Gateway.
func extractGateway(gw registry.Gateway) gateway.Gateway {
	// The registry returns a wrapper; extract the underlying gateway if possible
	if wrapper, ok := gw.(interface{ Gateway() gateway.Gateway }); ok {
		return wrapper.Gateway()
	}
	// Fallback: wrap the registry.Gateway to satisfy gateway.Gateway interface
	return &registryGatewayAdapter{gw}
}

// createTwilioGateway creates a Twilio voice gateway using the registry pattern.
func createTwilioGateway(cfg GatewayConfig) (gateway.Gateway, error) {
	tools := convertTools(cfg.Tools)
	handlers := convertHandlers(cfg.ToolHandlers)

	// Build options: Twilio credentials + common options + provider-specific
	opts := []registry.ProviderOption{
		registry.WithAccountSID(cfg.TwilioAccountSID),
		registry.WithAuthToken(cfg.TwilioAuthToken),
		registry.WithPhoneNumber(cfg.TwilioPhone),
	}
	opts = append(opts, buildCommonGatewayOptions(cfg)...)
	opts = append(opts,
		omnivoice.WithGatewayTools("twilio", tools),
		omnivoice.WithGatewayToolHandlers("twilio", handlers),
	)

	// Add listener if provided
	if cfg.Listener != nil {
		opts = append(opts, registry.WithListener(cfg.Listener))
	}

	// Configure realtime mode if specified
	if cfg.Mode == "realtime" {
		realtimeOpts, err := buildRealtimeOptions(cfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, realtimeOpts...)
	}

	// Get gateway via registry
	gw, err := omnivoice.GetGatewayProvider("twilio", opts...)
	if err != nil {
		return nil, err
	}
	return extractGateway(gw), nil
}

// createTelnyxGateway creates a Telnyx voice gateway using the registry pattern.
func createTelnyxGateway(cfg GatewayConfig) (gateway.Gateway, error) {
	tools := convertTools(cfg.Tools)
	handlers := convertHandlers(cfg.ToolHandlers)

	// Build options: Telnyx credentials + common options + provider-specific
	opts := []registry.ProviderOption{
		registry.WithAPIKey(cfg.TelnyxAPIKey),
		registry.WithPhoneNumber(cfg.TelnyxPhone),
		registry.WithConnectionID(cfg.TelnyxConnectionID),
	}
	opts = append(opts, buildCommonGatewayOptions(cfg)...)
	opts = append(opts,
		omnivoice.WithGatewayTools("telnyx", tools),
		omnivoice.WithGatewayToolHandlers("telnyx", handlers),
	)

	// Add listener if provided
	if cfg.Listener != nil {
		opts = append(opts, registry.WithListener(cfg.Listener))
	}

	// Get gateway via registry
	gw, err := omnivoice.GetGatewayProvider("telnyx", opts...)
	if err != nil {
		return nil, err
	}
	return extractGateway(gw), nil
}

// buildRealtimeOptions builds provider options for realtime mode.
func buildRealtimeOptions(cfg GatewayConfig) ([]registry.ProviderOption, error) {
	factory, err := omnivoice.GetRealtimeFactory(cfg.RealtimeProvider)
	if err != nil {
		return nil, fmt.Errorf("get realtime factory: %w", err)
	}

	// Resolve API key from environment if not provided
	apiKey := cfg.RealtimeAPIKey
	if apiKey == "" {
		apiKey = resolveRealtimeAPIKey(cfg.RealtimeProvider)
	}

	return []registry.ProviderOption{
		registry.WithPipelineMode("realtime"),
		omnivoice.WithRealtimeFactory(factory),
		omnivoice.WithGatewayRealtimeConfig(&gateway.RealtimeConfig{
			Provider:     cfg.RealtimeProvider,
			APIKey:       apiKey,
			Model:        cfg.RealtimeModel,
			Voice:        cfg.RealtimeVoice,
			Instructions: cfg.LLMSystemPrompt,
		}),
	}, nil
}

// resolveRealtimeAPIKey resolves the API key from environment variables.
func resolveRealtimeAPIKey(provider string) string {
	switch provider {
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "gemini":
		if key := os.Getenv("GEMINI_API_KEY"); key != "" {
			return key
		}
		return os.Getenv("GOOGLE_API_KEY")
	default:
		return ""
	}
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
