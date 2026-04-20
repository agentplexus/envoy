package commands

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/gateway"
	"github.com/plexusone/omniagent/voice"
	"github.com/plexusone/omnichat/provider"
	"github.com/plexusone/omnichat/providers/discord"
	"github.com/plexusone/omnichat/providers/telegram"
	"github.com/plexusone/omnichat/providers/twilio"
	"github.com/plexusone/omnichat/providers/whatsapp"
	"github.com/plexusone/omniobserve/integrations/omnillm"
	"github.com/plexusone/omniobserve/llmops"
	_ "github.com/plexusone/omniobserve/llmops/slog" // Register slog provider
)

var (
	gatewayAddress string
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Gateway management commands",
	Long:  "Commands for managing the omniagent WebSocket gateway.",
}

var gatewayRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the gateway server",
	Long: `Start the omniagent WebSocket gateway server.

The gateway serves as the control plane for all connected clients,
routing messages between channels and the AI agent.`,
	RunE: runGateway,
}

func init() {
	gatewayRunCmd.Flags().StringVar(&gatewayAddress, "address", "", "gateway listen address (default from config)")

	gatewayCmd.AddCommand(gatewayRunCmd)
}

func runGateway(cmd *cobra.Command, args []string) error {
	cfg := getConfig()
	logger := slog.Default()

	// Override from flag if provided
	address := cfg.Gateway.Address
	if gatewayAddress != "" {
		address = gatewayAddress
	}

	// Initialize observability if enabled
	var llmopsProvider llmops.Provider
	var observabilityHook *omnillm.Hook
	if cfg.Observability.Enabled {
		providerName := cfg.Observability.Provider
		if providerName == "" {
			providerName = "slog" // Default to slog for local logging
		}

		var err error
		llmopsProvider, err = llmops.Open(providerName,
			llmops.WithLogger(logger),
			llmops.WithAPIKey(cfg.Observability.APIKey),
			llmops.WithEndpoint(cfg.Observability.Endpoint),
			llmops.WithProjectName("omniagent"),
		)
		if err != nil {
			logger.Warn("failed to initialize observability", "provider", providerName, "error", err)
		} else {
			observabilityHook = omnillm.NewHook(llmopsProvider)
			defer llmopsProvider.Close()
			logger.Info("observability initialized", "provider", providerName)
		}
	}

	// Create agent if API key is configured
	var agentInstance *agent.Agent
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
		// Only set hook if non-nil to avoid interface{type, nil} gotcha
		if observabilityHook != nil {
			agentConfig.ObservabilityHook = observabilityHook
		}
		var err error
		agentInstance, err = agent.New(agentConfig)
		if err != nil {
			return fmt.Errorf("create agent: %w", err)
		}
		defer agentInstance.Close()
		logger.Info("agent initialized", "provider", cfg.Agent.Provider, "model", cfg.Agent.Model)

		// Register search tool if available
		if searchTool, err := agent.NewSearchTool(); err == nil {
			agentInstance.RegisterTool(searchTool)
			logger.Info("search tool registered")
		} else {
			logger.Debug("search tool not available", "error", err)
		}

		// Load skills if enabled
		if cfg.Skills.Enabled {
			searchPaths := cfg.Skills.Paths
			if len(searchPaths) == 0 {
				searchPaths = nil // Use defaults
			}
			if err := agentInstance.LoadSkills(searchPaths); err != nil {
				logger.Warn("failed to load skills", "error", err)
			}
		}
	} else {
		logger.Warn("no API key configured, agent disabled (messages will be echoed)")
	}

	// Initialize voice processor if enabled
	var voiceProcessor *voice.Processor
	if cfg.Voice.Enabled {
		var err error
		voiceProcessor, err = voice.New(voice.Config{
			Enabled:      true,
			ResponseMode: cfg.Voice.ResponseMode,
			STT: voice.STTConfig{
				Provider: cfg.Voice.STT.Provider,
				APIKey:   cfg.Voice.STT.APIKey,
				Model:    cfg.Voice.STT.Model,
				Language: cfg.Voice.STT.Language,
			},
			TTS: voice.TTSConfig{
				Provider: cfg.Voice.TTS.Provider,
				APIKey:   cfg.Voice.TTS.APIKey,
				Model:    cfg.Voice.TTS.Model,
				VoiceID:  cfg.Voice.TTS.VoiceID,
			},
		}, logger)
		if err != nil {
			return fmt.Errorf("create voice processor: %w", err)
		}
		defer voiceProcessor.Close()
		logger.Info("voice processor initialized",
			"stt_provider", cfg.Voice.STT.Provider,
			"tts_provider", cfg.Voice.TTS.Provider,
			"response_mode", cfg.Voice.ResponseMode)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Create message router and register channels
	router := provider.NewRouter(logger)
	webhookHandlers := make(map[string]http.Handler)

	// Register Telegram if configured
	if cfg.Channels.Telegram.Enabled {
		tg, err := telegram.New(telegram.Config{
			Token:  cfg.Channels.Telegram.Token,
			Logger: logger,
		})
		if err != nil {
			return fmt.Errorf("create telegram provider: %w", err)
		}
		router.Register(tg)
		logger.Info("telegram provider registered")
	}

	// Register Discord if configured
	if cfg.Channels.Discord.Enabled {
		dc, err := discord.New(discord.Config{
			Token:   cfg.Channels.Discord.Token,
			GuildID: cfg.Channels.Discord.GuildID,
			Logger:  logger,
		})
		if err != nil {
			return fmt.Errorf("create discord provider: %w", err)
		}
		router.Register(dc)
		logger.Info("discord provider registered")
	}

	// Register WhatsApp if configured
	if cfg.Channels.WhatsApp.Enabled {
		dbPath := cfg.Channels.WhatsApp.DBPath
		if dbPath == "" {
			dbPath = "whatsapp.db"
		}
		wa, err := whatsapp.New(whatsapp.Config{
			DBPath: dbPath,
			Logger: logger,
			QRCallback: func(qr string) {
				fmt.Println("\nScan this QR code with WhatsApp:")
				fmt.Println("(Settings -> Linked Devices -> Link a Device)")
				fmt.Println()
				qrterminal.GenerateWithConfig(qr, qrterminal.Config{
					Level:     qrterminal.L,
					Writer:    os.Stdout,
					BlackChar: qrterminal.WHITE,
					WhiteChar: qrterminal.BLACK,
					QuietZone: 1,
				})
				fmt.Println()
			},
		})
		if err != nil {
			return fmt.Errorf("create whatsapp provider: %w", err)
		}
		router.Register(wa)
		logger.Info("whatsapp provider registered")
	}

	// Register Twilio SMS if configured
	if cfg.Channels.TwilioSMS.Enabled {
		sms, err := twilio.New(twilio.Config{
			AccountSID:  cfg.Channels.TwilioSMS.AccountSID,
			AuthToken:   cfg.Channels.TwilioSMS.AuthToken,
			PhoneNumber: cfg.Channels.TwilioSMS.PhoneNumber,
			Logger:      logger,
		})
		if err != nil {
			return fmt.Errorf("create twilio sms provider: %w", err)
		}
		router.Register(sms)

		// Register webhook handler
		webhookPath := cfg.Channels.TwilioSMS.WebhookPath
		if webhookPath == "" {
			webhookPath = "/webhook/twilio/sms"
		}
		webhookHandlers[webhookPath] = sms.WebhookHandler()
		logger.Info("twilio sms provider registered", "webhook_path", webhookPath)
	}

	// Check if any channels are configured
	channels := router.ListProviders()
	if len(channels) == 0 {
		logger.Warn("no channels configured, running gateway only")
	} else {
		// Set up agent processing if available
		if agentInstance != nil {
			router.SetAgent(agentInstance)
			if voiceProcessor != nil {
				router.OnMessage(provider.All(), router.ProcessWithVoice(voiceProcessor))
				logger.Info("voice processing enabled for messages")
			} else {
				router.OnMessage(provider.All(), router.ProcessWithAgent())
			}
		}

		// Connect all channels
		if err := router.ConnectAll(ctx); err != nil {
			return fmt.Errorf("connect channels: %w", err)
		}
		defer func() {
			if err := router.DisconnectAll(context.Background()); err != nil {
				logger.Error("disconnect error", "error", err)
			}
		}()
		logger.Info("channels connected", "count", len(channels))
	}

	// Create and start gateway
	gw, err := gateway.New(gateway.Config{
		Address:         address,
		ReadTimeout:     cfg.Gateway.ReadTimeout,
		WriteTimeout:    cfg.Gateway.WriteTimeout,
		PingInterval:    cfg.Gateway.PingInterval,
		Agent:           agentInstance,
		Logger:          logger,
		WebhookHandlers: webhookHandlers,
	})
	if err != nil {
		return fmt.Errorf("create gateway: %w", err)
	}

	// Start gateway
	fmt.Printf("OmniAgent running on %s\n", address)
	fmt.Printf("Channels: %v\n", channels)
	fmt.Println("Press Ctrl+C to stop")

	if err := gw.Run(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("gateway error: %w", err)
	}

	fmt.Println("OmniAgent stopped")
	return nil
}
