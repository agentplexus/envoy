package voice

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/plexusone/omnillm"
	"github.com/plexusone/omnivoice-core/gateway"
)

// GatewayConfig configures the voice gateway integration.
type GatewayConfig struct {
	// Provider selects the telephony provider ("twilio" or "telnyx").
	Provider string

	// Twilio configuration
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioPhone      string

	// Telnyx configuration
	TelnyxAPIKey       string
	TelnyxPhone        string
	TelnyxConnectionID string

	// Server configuration
	PublicURL  string
	ListenAddr string
	Listener   net.Listener // Optional external listener (e.g., ngrok)

	// Pipeline mode: "text" (STT→LLM→TTS, ~500-1000ms) or "realtime" (voice-to-voice, ~100-200ms)
	// Default: "text"
	Mode string

	// Voice processing (used when Mode is "text")
	Config Config

	// LLM configuration (used when Mode is "text")
	LLMProvider     string
	LLMModel        string
	LLMSystemPrompt string

	// Realtime configuration (used when Mode is "realtime")
	// RealtimeProvider: "openai" or "gemini"
	RealtimeProvider string
	RealtimeAPIKey   string
	RealtimeModel    string // e.g., "gpt-4o-realtime-preview-2024-12-17", "gemini-2.0-flash-live"
	RealtimeVoice    string // e.g., "alloy", "Puck"

	// Tools available to the voice LLM
	Tools        []ToolDefinition
	ToolHandlers map[string]ToolHandler

	// Greeting is the initial message spoken when a call connects.
	Greeting string

	// Session limits
	MaxSessionDuration time.Duration
	InterruptionMode   string

	// Logger
	Logger *slog.Logger
}

// Gateway wraps a voice gateway provider with omniagent integration.
type Gateway struct {
	config   GatewayConfig
	provider gateway.Gateway
	llm      *omnillm.ChatClient
	llmModel string
	logger   *slog.Logger

	// Session callbacks
	onSessionStart func(session gateway.Session)
	onSessionEnd   func(session gateway.Session)
}

// NewGateway creates a new voice gateway integrated with omniagent.
func NewGateway(cfg GatewayConfig) (*Gateway, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Load defaults from environment
	if cfg.TwilioAccountSID == "" {
		cfg.TwilioAccountSID = os.Getenv("TWILIO_ACCOUNT_SID")
	}
	if cfg.TwilioAuthToken == "" {
		cfg.TwilioAuthToken = os.Getenv("TWILIO_AUTH_TOKEN")
	}
	if cfg.TelnyxAPIKey == "" {
		cfg.TelnyxAPIKey = os.Getenv("TELNYX_API_KEY")
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.MaxSessionDuration == 0 {
		cfg.MaxSessionDuration = 30 * time.Minute
	}
	if cfg.InterruptionMode == "" {
		cfg.InterruptionMode = "immediate"
	}
	if cfg.Provider == "" {
		cfg.Provider = "twilio" // Default to Twilio
	}

	// Create the underlying gateway provider
	var provider gateway.Gateway
	var err error

	switch cfg.Provider {
	case "twilio":
		provider, err = createTwilioGateway(cfg)
	case "telnyx":
		provider, err = createTelnyxGateway(cfg)
	default:
		return nil, fmt.Errorf("unsupported voice provider: %s (supported: twilio, telnyx)", cfg.Provider)
	}
	if err != nil {
		return nil, fmt.Errorf("create %s gateway: %w", cfg.Provider, err)
	}

	// Determine API key for the LLM provider
	var llmAPIKey string
	switch cfg.LLMProvider {
	case "openai":
		llmAPIKey = os.Getenv("OPENAI_API_KEY")
	case "anthropic":
		llmAPIKey = os.Getenv("ANTHROPIC_API_KEY")
	case "gemini":
		llmAPIKey = os.Getenv("GEMINI_API_KEY")
	}

	// Create LLM client for omniagent integration
	llm, err := omnillm.NewClient(omnillm.ClientConfig{
		Providers: []omnillm.ProviderConfig{
			{
				Provider: omnillm.ProviderName(cfg.LLMProvider),
				APIKey:   llmAPIKey,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	// Default model if not specified
	llmModel := cfg.LLMModel
	if llmModel == "" {
		switch cfg.LLMProvider {
		case "anthropic":
			llmModel = "claude-sonnet-4-20250514"
		case "openai":
			llmModel = "gpt-4o"
		default:
			llmModel = "claude-sonnet-4-20250514"
		}
	}

	return &Gateway{
		config:   cfg,
		provider: provider,
		llm:      llm,
		llmModel: llmModel,
		logger:   cfg.Logger,
	}, nil
}

// OnCall sets the handler for incoming calls.
func (g *Gateway) OnCall(handler gateway.CallHandler) {
	g.provider.OnCall(handler)
}

// OnSessionStart sets the callback for when a session starts.
func (g *Gateway) OnSessionStart(handler func(session gateway.Session)) {
	g.onSessionStart = handler
}

// OnSessionEnd sets the callback for when a session ends.
func (g *Gateway) OnSessionEnd(handler func(session gateway.Session)) {
	g.onSessionEnd = handler
}

// Start starts the voice gateway.
func (g *Gateway) Start(ctx context.Context) error {
	g.logger.Info("starting omniagent voice gateway",
		"provider", g.provider.Name(),
		"listen_addr", g.config.ListenAddr,
		"public_url", g.config.PublicURL)

	return g.provider.Start(ctx)
}

// Stop stops the voice gateway.
func (g *Gateway) Stop() error {
	return g.provider.Stop()
}

// MakeCall initiates an outbound call.
func (g *Gateway) MakeCall(ctx context.Context, to string) (gateway.Session, error) {
	return g.provider.MakeCall(ctx, to)
}

// GetSession retrieves an active session.
func (g *Gateway) GetSession(callID string) (gateway.Session, bool) {
	return g.provider.GetSession(callID)
}

// ListSessions returns all active sessions.
func (g *Gateway) ListSessions() []gateway.Session {
	return g.provider.ListSessions()
}

// ProcessWithAgent processes a conversation turn using the omniagent LLM.
func (g *Gateway) ProcessWithAgent(ctx context.Context, session gateway.Session, userText string) (string, error) {
	// Build messages from transcript
	transcript := session.Transcript()
	messages := make([]omnillm.Message, 0, len(transcript)+2)

	// Add system prompt
	if g.config.LLMSystemPrompt != "" {
		messages = append(messages, omnillm.Message{
			Role:    omnillm.RoleSystem,
			Content: g.config.LLMSystemPrompt,
		})
	}

	// Add history
	for _, turn := range transcript {
		role := omnillm.RoleUser
		if turn.Role == "agent" {
			role = omnillm.RoleAssistant
		}
		messages = append(messages, omnillm.Message{
			Role:    role,
			Content: turn.Text,
		})
	}

	// Add current user message
	messages = append(messages, omnillm.Message{
		Role:    omnillm.RoleUser,
		Content: userText,
	})

	// Generate response
	resp, err := g.llm.CreateChatCompletion(ctx, &omnillm.ChatCompletionRequest{
		Model:    g.llmModel,
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("LLM completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return resp.Choices[0].Message.Content, nil
}

// VoiceAgentLLMProvider implements the gateway.LLMProvider interface using omniagent.
type VoiceAgentLLMProvider struct {
	gateway *Gateway
}

// NewVoiceAgentLLMProvider creates a new LLM provider that uses omniagent.
func NewVoiceAgentLLMProvider(gw *Gateway) *VoiceAgentLLMProvider {
	return &VoiceAgentLLMProvider{gateway: gw}
}

// Generate produces a response using the omniagent LLM.
func (p *VoiceAgentLLMProvider) Generate(ctx context.Context, input string, history []gateway.Turn) (string, []gateway.ToolCall, error) {
	// Build messages
	messages := make([]omnillm.Message, 0, len(history)+2)

	// Add system prompt
	if p.gateway.config.LLMSystemPrompt != "" {
		messages = append(messages, omnillm.Message{
			Role:    omnillm.RoleSystem,
			Content: p.gateway.config.LLMSystemPrompt,
		})
	}

	// Add history
	for _, turn := range history {
		role := omnillm.RoleUser
		if turn.Role == "agent" {
			role = omnillm.RoleAssistant
		}
		messages = append(messages, omnillm.Message{
			Role:    role,
			Content: turn.Text,
		})
	}

	// Add current user message
	messages = append(messages, omnillm.Message{
		Role:    omnillm.RoleUser,
		Content: input,
	})

	// Generate response
	resp, err := p.gateway.llm.CreateChatCompletion(ctx, &omnillm.ChatCompletionRequest{
		Model:    p.gateway.llmModel,
		Messages: messages,
	})
	if err != nil {
		return "", nil, fmt.Errorf("LLM completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", nil, fmt.Errorf("no response from LLM")
	}

	response := resp.Choices[0].Message.Content

	// Parse tool calls from response (if any)
	var toolCalls []gateway.ToolCall
	// Tool call parsing would be implemented here based on the LLM's output format

	return response, toolCalls, nil
}

// ToolDefinition defines a tool available to the voice LLM.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolHandler processes a tool call.
type ToolHandler func(ctx context.Context, args map[string]any) (string, error)

// DefaultVoiceSystemPrompt is the default system prompt for voice conversations.
const DefaultVoiceSystemPrompt = `You are a helpful voice assistant powered by OmniAgent. You are having a phone conversation with a user.

Guidelines:
- Keep responses concise and conversational - aim for 1-3 sentences
- Use natural spoken language, not written language
- Avoid using markdown, bullet points, or formatting
- If you need to list things, say them naturally (e.g., "first... second... and third...")
- Ask clarifying questions when the user's intent is unclear
- Be friendly and professional
- If you don't know something, say so honestly

Remember: This is a voice conversation over the phone, so speak naturally and be mindful of the audio-only format.`
