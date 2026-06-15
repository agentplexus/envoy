package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.ngrok.com/ngrok"
	ngrokconfig "golang.ngrok.com/ngrok/config"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/voice"
	"github.com/plexusone/omnivoice-core/gateway"
)

var voiceCmd = &cobra.Command{
	Use:   "voice",
	Short: "Voice gateway commands for phone calls",
	Long: `Voice gateway commands for handling full-duplex phone calls via Twilio or Telnyx.

Start the voice gateway:
  omniagent voice serve

Show voice configuration:
  omniagent voice status`,
}

var voiceServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the voice gateway server",
	Long: `Start the voice gateway server to handle inbound and outbound phone calls.

The gateway supports Twilio and Telnyx for full-duplex bidirectional audio.

Required environment variables (per provider):
  Twilio:
    TWILIO_ACCOUNT_SID  - Twilio Account SID
    TWILIO_AUTH_TOKEN   - Twilio Auth Token

  Telnyx:
    TELNYX_API_KEY      - Telnyx API Key
    TELNYX_CONNECTION_ID - Telnyx Connection ID

Optional environment variables:
  NGROK_AUTHTOKEN     - For ngrok tunnel (required with --ngrok)
  VOICE_STT_PROVIDER  - STT provider (deepgram, openai, elevenlabs)
  VOICE_TTS_PROVIDER  - TTS provider (elevenlabs, openai, deepgram)
  DEEPGRAM_API_KEY    - For Deepgram STT
  ELEVENLABS_API_KEY  - For ElevenLabs TTS
  ANTHROPIC_API_KEY   - For Anthropic LLM
  OPENAI_API_KEY      - For OpenAI STT/TTS/LLM

Examples:
  # With Twilio
  omniagent voice serve --provider twilio --listen :8080 --public-url https://your-server.com

  # With Telnyx
  omniagent voice serve --provider telnyx --listen :8080 --public-url https://your-server.com

  # With ngrok tunnel (for local development)
  omniagent voice serve --listen :8080 --ngrok`,
	RunE: runVoiceServe,
}

var voiceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show voice gateway status",
	Long:  `Show the current voice gateway configuration and status.`,
	RunE:  runVoiceStatus,
}

var voiceCallCmd = &cobra.Command{
	Use:   "call <phone-number>",
	Short: "Make an outbound call",
	Long: `Make an outbound call to the specified phone number.

Example:
  omniagent voice call +1234567890`,
	Args: cobra.ExactArgs(1),
	RunE: runVoiceCall,
}

// Voice serve flags
var (
	voiceProvider     string
	voiceListenAddr   string
	voicePublicURL    string
	voicePhoneNumber  string
	voiceSTTProvider  string
	voiceTTSProvider  string
	voiceTTSVoiceID   string
	voiceLLMProvider  string
	voiceLLMModel     string
	voiceSystemPrompt string
	voiceGreeting     string
	voiceNgrok        bool
	voiceNgrokDomain  string
)

func init() {
	// Add voice subcommands
	voiceCmd.AddCommand(voiceServeCmd)
	voiceCmd.AddCommand(voiceStatusCmd)
	voiceCmd.AddCommand(voiceCallCmd)

	// Serve command flags
	voiceServeCmd.Flags().StringVar(&voiceProvider, "provider", "twilio", "Telephony provider (twilio, telnyx)")
	voiceServeCmd.Flags().StringVar(&voiceListenAddr, "listen", ":8080", "Address to listen on")
	voiceServeCmd.Flags().StringVar(&voicePublicURL, "public-url", "", "Public URL for webhooks (required)")
	voiceServeCmd.Flags().StringVar(&voicePhoneNumber, "phone", "", "Phone number for outbound calls")
	voiceServeCmd.Flags().StringVar(&voiceSTTProvider, "stt", "deepgram", "STT provider (deepgram, whisper, google)")
	voiceServeCmd.Flags().StringVar(&voiceTTSProvider, "tts", "elevenlabs", "TTS provider (elevenlabs, openai, google)")
	voiceServeCmd.Flags().StringVar(&voiceTTSVoiceID, "voice", "", "TTS voice ID")
	voiceServeCmd.Flags().StringVar(&voiceLLMProvider, "llm", "", "LLM provider (anthropic, openai) - auto-detected if not specified")
	voiceServeCmd.Flags().StringVar(&voiceLLMModel, "model", "", "LLM model (default: claude-sonnet-4-20250514)")
	voiceServeCmd.Flags().StringVar(&voiceSystemPrompt, "system-prompt", "", "Custom system prompt for the agent")
	voiceServeCmd.Flags().StringVar(&voiceGreeting, "greeting", "", "Initial greeting message when call connects")
	voiceServeCmd.Flags().BoolVar(&voiceNgrok, "ngrok", false, "Use ngrok tunnel for public URL (requires NGROK_AUTHTOKEN)")
	voiceServeCmd.Flags().StringVar(&voiceNgrokDomain, "ngrok-domain", "", "Custom ngrok domain (requires paid plan)")

	// Add to root
	rootCmd.AddCommand(voiceCmd)
}

func runVoiceServe(cmd *cobra.Command, args []string) error {
	// Validate flags
	if !voiceNgrok && voicePublicURL == "" {
		return fmt.Errorf("--public-url is required for webhooks (or use --ngrok)")
	}

	// Auto-detect LLM provider if not specified
	if voiceLLMProvider == "" {
		switch {
		case os.Getenv("ANTHROPIC_API_KEY") != "":
			voiceLLMProvider = "anthropic"
		case os.Getenv("OPENAI_API_KEY") != "":
			voiceLLMProvider = "openai"
		default:
			return fmt.Errorf("no LLM provider configured: set ANTHROPIC_API_KEY or OPENAI_API_KEY, or use --llm flag")
		}
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Handle shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down voice gateway...")
		cancel()
	}()

	// Set up ngrok tunnel if requested
	var ngrokListener net.Listener
	publicURL := voicePublicURL

	if voiceNgrok {
		var err error
		ngrokListener, publicURL, err = createNgrokTunnel(ctx, logger)
		if err != nil {
			return fmt.Errorf("create ngrok tunnel: %w", err)
		}
		defer ngrokListener.Close()
	}

	// Get system prompt
	systemPrompt := voiceSystemPrompt
	if systemPrompt == "" {
		systemPrompt = voice.DefaultVoiceSystemPrompt
	}

	// Get greeting (flag takes precedence over env var)
	greeting := voiceGreeting
	if greeting == "" {
		greeting = os.Getenv("OMNIAGENT_VOICE_GREETING")
	}

	// Get STT provider (env var overrides default, flag overrides env var)
	sttProvider := voiceSTTProvider
	if sttProvider == "deepgram" { // default value
		if envSTT := os.Getenv("VOICE_STT_PROVIDER"); envSTT != "" {
			sttProvider = envSTT
		}
	}

	// Get TTS provider (env var overrides default, flag overrides env var)
	ttsProvider := voiceTTSProvider
	if ttsProvider == "elevenlabs" { // default value
		if envTTS := os.Getenv("VOICE_TTS_PROVIDER"); envTTS != "" {
			ttsProvider = envTTS
		}
	}

	// Create agent to get tools from registered skills
	cfg := getConfig()
	var tools []voice.ToolDefinition
	var toolHandlers map[string]voice.ToolHandler
	if cfg.Agent.APIKey != "" {
		agentConfig := agent.Config{
			Provider:     cfg.Agent.Provider,
			Model:        cfg.Agent.Model,
			APIKey:       cfg.Agent.APIKey,
			BaseURL:      cfg.Agent.BaseURL,
			Temperature:  cfg.Agent.Temperature,
			MaxTokens:    cfg.Agent.MaxTokens,
			SystemPrompt: cfg.Agent.SystemPrompt,
			Logger:       logger,
		}
		agentInstance, err := agent.New(agentConfig, getAgentOptions()...)
		if err != nil {
			logger.Warn("failed to create agent for tools", "error", err)
		} else {
			defer agentInstance.Close()

			// Initialize compiled skills
			if err := agentInstance.InitCompiledSkills(ctx); err != nil {
				logger.Warn("failed to init compiled skills", "error", err)
			}

			// Register search tool (omniserp) - same as gateway.go
			if searchTool, err := agent.NewSearchTool(); err == nil {
				agentInstance.RegisterTool(searchTool)
				logger.Info("search tool registered for voice")
			} else {
				logger.Debug("search tool not available", "error", err)
			}

			// Get tools from agent registry
			agentTools := agentInstance.Tools().GetTools()
			if len(agentTools) > 0 {
				toolHandlers = make(map[string]voice.ToolHandler)

				for _, t := range agentTools {
					// Safely convert parameters to map[string]any
					var params map[string]any
					switch p := t.Function.Parameters.(type) {
					case map[string]any:
						params = p
					default:
						// If type is unexpected, marshal/unmarshal to convert
						if t.Function.Parameters != nil {
							data, _ := json.Marshal(t.Function.Parameters)
							_ = json.Unmarshal(data, &params)
						}
					}

					tools = append(tools, voice.ToolDefinition{
						Name:        t.Function.Name,
						Description: t.Function.Description,
						Parameters:  params,
					})

					// Create handler that calls the agent's tool executor
					// Capture toolName in local variable for the closure
					toolName := t.Function.Name
					toolHandlers[toolName] = func(ctx context.Context, args map[string]any) (string, error) {
						argsJSON, _ := json.Marshal(args)
						return agentInstance.Tools().Execute(ctx, toolName, argsJSON)
					}
				}

				if len(tools) > 0 {
					logger.Info("voice tools loaded", "count", len(tools), "names", toolNames(tools))
				}
			}
		}
	}

	// Create gateway config
	gwConfig := voice.GatewayConfig{
		Provider:           voiceProvider,
		TwilioAccountSID:   os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:    os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioPhone:        voicePhoneNumber,
		TelnyxAPIKey:       os.Getenv("TELNYX_API_KEY"),
		TelnyxPhone:        voicePhoneNumber,
		TelnyxConnectionID: os.Getenv("TELNYX_CONNECTION_ID"),
		PublicURL:          publicURL,
		ListenAddr:         voiceListenAddr,
		Listener:           ngrokListener,
		Config: voice.Config{
			Enabled:      true,
			ResponseMode: "always",
			STT: voice.STTConfig{
				Provider: sttProvider,
			},
			TTS: voice.TTSConfig{
				Provider: ttsProvider,
				VoiceID:  voiceTTSVoiceID,
			},
		},
		LLMProvider:        voiceLLMProvider,
		LLMModel:           voiceLLMModel,
		LLMSystemPrompt:    systemPrompt,
		Tools:              tools,
		ToolHandlers:       toolHandlers,
		Greeting:           greeting,
		MaxSessionDuration: 30 * time.Minute,
		InterruptionMode:   "immediate",
		Logger:             logger,
	}

	// Create gateway
	gw, err := voice.NewGateway(gwConfig)
	if err != nil {
		return fmt.Errorf("create voice gateway: %w", err)
	}

	// Set up call handler
	gw.OnCall(func(info *gateway.CallInfo) error {
		logger.Info("incoming call",
			"from", info.From,
			"to", info.To,
			"call_id", info.CallID)
		return nil // Accept all calls
	})

	// Print startup info
	fmt.Println()
	fmt.Println("Voice Gateway Starting")
	fmt.Println("======================")
	fmt.Printf("Provider:   %s\n", voiceProvider)
	fmt.Printf("Listen:     %s\n", voiceListenAddr)
	fmt.Printf("Public URL: %s\n", publicURL)
	if voiceNgrok {
		fmt.Println("Tunnel:     ngrok")
	}
	fmt.Printf("STT:        %s\n", sttProvider)
	fmt.Printf("TTS:        %s\n", ttsProvider)
	fmt.Printf("LLM:        %s\n", voiceLLMProvider)
	fmt.Println()
	fmt.Printf("Configure %s webhooks:\n", voiceProvider)
	fmt.Printf("  Voice URL: %s/voice/inbound\n", publicURL)
	fmt.Printf("  Status:    %s/voice/status\n", publicURL)
	fmt.Println()

	// Start gateway
	return gw.Start(ctx)
}

// createNgrokTunnel creates an ngrok tunnel and returns the listener and public URL.
func createNgrokTunnel(ctx context.Context, logger *slog.Logger) (net.Listener, string, error) {
	authtoken := os.Getenv("NGROK_AUTHTOKEN")
	if authtoken == "" {
		return nil, "", fmt.Errorf("NGROK_AUTHTOKEN environment variable is required for --ngrok")
	}

	logger.Info("creating ngrok tunnel...")

	// Configure ngrok options
	ngrokOpts := []ngrok.ConnectOption{
		ngrok.WithAuthtoken(authtoken),
	}

	// Configure the HTTP endpoint
	httpConfig := ngrokconfig.HTTPEndpoint()
	if voiceNgrokDomain != "" {
		httpConfig = ngrokconfig.HTTPEndpoint(ngrokconfig.WithDomain(voiceNgrokDomain))
	}

	// Create the tunnel
	listener, err := ngrok.Listen(ctx, httpConfig, ngrokOpts...)
	if err != nil {
		return nil, "", fmt.Errorf("ngrok listen: %w", err)
	}

	// ngrok listener address is just hostname, need to add https scheme
	publicURL := "https://" + listener.Addr().String()

	logger.Info("ngrok tunnel established", "url", publicURL)

	return listener, publicURL, nil
}

func runVoiceStatus(cmd *cobra.Command, args []string) error {
	// Check environment
	twilioAccountSID := os.Getenv("TWILIO_ACCOUNT_SID")
	twilioAuthToken := os.Getenv("TWILIO_AUTH_TOKEN")
	telnyxAPIKey := os.Getenv("TELNYX_API_KEY")
	telnyxConnectionID := os.Getenv("TELNYX_CONNECTION_ID")
	ngrokToken := os.Getenv("NGROK_AUTHTOKEN")

	fmt.Println("Voice Gateway Configuration")
	fmt.Println("===========================")
	fmt.Println()

	// Twilio
	fmt.Println("Twilio:")
	if twilioAccountSID != "" {
		fmt.Printf("  Account SID:  %s...%s\n", twilioAccountSID[:4], twilioAccountSID[len(twilioAccountSID)-4:])
	} else {
		fmt.Println("  Account SID:  Not configured (set TWILIO_ACCOUNT_SID)")
	}
	if twilioAuthToken != "" {
		fmt.Println("  Auth Token:   Configured")
	} else {
		fmt.Println("  Auth Token:   Not configured (set TWILIO_AUTH_TOKEN)")
	}
	fmt.Println()

	// Telnyx
	fmt.Println("Telnyx:")
	if telnyxAPIKey != "" {
		fmt.Println("  API Key:      Configured")
	} else {
		fmt.Println("  API Key:      Not configured (set TELNYX_API_KEY)")
	}
	if telnyxConnectionID != "" {
		fmt.Printf("  Connection:   %s\n", telnyxConnectionID)
	} else {
		fmt.Println("  Connection:   Not configured (set TELNYX_CONNECTION_ID)")
	}
	fmt.Println()

	// Ngrok
	fmt.Println("Ngrok:")
	if ngrokToken != "" {
		fmt.Println("  Auth Token:   Configured (use --ngrok flag)")
	} else {
		fmt.Println("  Auth Token:   Not configured (set NGROK_AUTHTOKEN)")
	}
	fmt.Println()

	// STT Providers
	fmt.Println("STT Providers:")
	if os.Getenv("DEEPGRAM_API_KEY") != "" {
		fmt.Println("  Deepgram:     Configured")
	} else {
		fmt.Println("  Deepgram:     Not configured (set DEEPGRAM_API_KEY)")
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		fmt.Println("  OpenAI:       Configured (Whisper)")
	} else {
		fmt.Println("  OpenAI:       Not configured (set OPENAI_API_KEY)")
	}
	fmt.Println()

	// TTS Providers
	fmt.Println("TTS Providers:")
	if os.Getenv("ELEVENLABS_API_KEY") != "" {
		fmt.Println("  ElevenLabs:   Configured")
	} else {
		fmt.Println("  ElevenLabs:   Not configured (set ELEVENLABS_API_KEY)")
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		fmt.Println("  OpenAI:       Configured")
	}
	fmt.Println()

	// LLM Providers
	fmt.Println("LLM Providers:")
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		fmt.Println("  Anthropic:    Configured (Claude)")
	} else {
		fmt.Println("  Anthropic:    Not configured (set ANTHROPIC_API_KEY)")
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		fmt.Println("  OpenAI:       Configured (GPT-4o)")
	}

	return nil
}

func runVoiceCall(cmd *cobra.Command, args []string) error {
	phoneNumber := args[0]

	if voicePublicURL == "" {
		return fmt.Errorf("--public-url is required")
	}

	// Auto-detect LLM provider if not specified
	if voiceLLMProvider == "" {
		switch {
		case os.Getenv("ANTHROPIC_API_KEY") != "":
			voiceLLMProvider = "anthropic"
		case os.Getenv("OPENAI_API_KEY") != "":
			voiceLLMProvider = "openai"
		default:
			return fmt.Errorf("no LLM provider configured: set ANTHROPIC_API_KEY or OPENAI_API_KEY, or use --llm flag")
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create minimal gateway config for outbound call
	gwConfig := voice.GatewayConfig{
		Provider:           voiceProvider,
		TwilioAccountSID:   os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:    os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioPhone:        voicePhoneNumber,
		TelnyxAPIKey:       os.Getenv("TELNYX_API_KEY"),
		TelnyxPhone:        voicePhoneNumber,
		TelnyxConnectionID: os.Getenv("TELNYX_CONNECTION_ID"),
		PublicURL:          voicePublicURL,
		ListenAddr:         voiceListenAddr,
		Config: voice.Config{
			Enabled: true,
			STT:     voice.STTConfig{Provider: voiceSTTProvider},
			TTS:     voice.TTSConfig{Provider: voiceTTSProvider},
		},
		LLMProvider:     voiceLLMProvider,
		LLMModel:        voiceLLMModel,
		LLMSystemPrompt: voice.DefaultVoiceSystemPrompt,
		Logger:          logger,
	}

	gw, err := voice.NewGateway(gwConfig)
	if err != nil {
		return fmt.Errorf("create voice gateway: %w", err)
	}

	ctx := context.Background()
	session, err := gw.MakeCall(ctx, phoneNumber)
	if err != nil {
		return fmt.Errorf("make call: %w", err)
	}

	fmt.Printf("Call initiated: %s\n", session.ID())
	fmt.Printf("To: %s\n", phoneNumber)
	fmt.Println()
	fmt.Println("Waiting for call to connect...")

	// Wait for session events
	for event := range session.Events() {
		switch event.Type {
		case gateway.EventSessionStarted:
			fmt.Println("Call connected!")
		case gateway.EventUserTranscript:
			fmt.Printf("User: %v\n", event.Data)
		case gateway.EventAgentTranscript:
			fmt.Printf("Agent: %v\n", event.Data)
		case gateway.EventSessionEnded:
			fmt.Println("Call ended.")
			return nil
		case gateway.EventError:
			fmt.Printf("Error: %v\n", event.Error)
		}
	}

	return nil
}

// toolNames returns the names of the tools for logging.
func toolNames(tools []voice.ToolDefinition) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}
