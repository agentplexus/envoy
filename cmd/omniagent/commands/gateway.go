package commands

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/agent/registry"
	"github.com/plexusone/omniagent/gateway"
	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omniagent/voice"
	"github.com/plexusone/omniagent/web"
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

	// Setup context for graceful shutdown (needed early for skill init)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Team (multi-user) mode: validate config, run the PostgreSQL
	// migrations (schema + RLS policies), and build the auth/admin HTTP
	// surface. Registered with the gateway mux after it is created.
	var teamHTTP *gateway.TeamHTTP
	if cfg.Team.Enabled {
		th, cleanup, err := setupTeamMode(ctx, cfg, logger)
		if err != nil {
			return err
		}
		defer cleanup()
		teamHTTP = th
	}

	// Embedded web UI: capability-driven SPA + GET /api/capabilities,
	// registered whenever the web UI is enabled (personal opt-in, or
	// implied by team mode). Team-only routes stay gated on cfg.Team.Enabled
	// above, via teamHTTP.
	var webHTTP *gateway.WebHTTP
	if cfg.WebUIEnabled() {
		webHTTP = gateway.NewWebHTTP(gateway.WebHTTPConfig{
			Capabilities: cfg.Capabilities(),
			Assets:       web.Assets,
			Logger:       logger,
		})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

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

	// Session rollover option from config, shared by all agents.
	var rolloverOpt agent.Option
	if cfg.Sessions.Rollover.Enabled {
		policy := sessions.RolloverPolicy{
			IdleTimeout: cfg.Sessions.Rollover.IdleTimeout.Duration(),
			Daily:       cfg.Sessions.Rollover.Daily,
		}
		if tz := cfg.Sessions.Rollover.Timezone; tz != "" {
			loc, err := time.LoadLocation(tz)
			if err != nil {
				return fmt.Errorf("invalid sessions.rollover.timezone %q: %w", tz, err)
			}
			policy.Location = loc
		}
		rolloverOpt = agent.WithSessionRollover(policy)
	}

	// Create agent factory for registry
	agentFactory := func(regCfg *registry.AgentConfig) (*agent.Agent, error) {
		agentConfig := agent.Config{
			Provider:     regCfg.Provider,
			Model:        regCfg.Model,
			APIKey:       regCfg.APIKey,
			BaseURL:      regCfg.BaseURL,
			Temperature:  regCfg.Temperature,
			MaxTokens:    regCfg.MaxTokens,
			SystemPrompt: regCfg.SystemPrompt,
			Timezone:     regCfg.Timezone,
			Logger:       logger,
		}
		// Only set hook if non-nil to avoid interface{type, nil} gotcha
		if observabilityHook != nil {
			agentConfig.ObservabilityHook = observabilityHook
		}

		agentOpts := getAgentOptions()
		if rolloverOpt != nil {
			agentOpts = append(agentOpts, rolloverOpt)
		}

		ag, err := agent.New(agentConfig, agentOpts...)
		if err != nil {
			return nil, fmt.Errorf("create agent: %w", err)
		}

		// Initialize compiled skills if any were registered
		opts := getAgentOptions()
		if len(opts) > 0 {
			if err := ag.InitCompiledSkills(ctx); err != nil {
				ag.Close()
				return nil, fmt.Errorf("init compiled skills: %w", err)
			}
		}

		// Register search tool if available
		if searchTool, err := agent.NewSearchTool(); err == nil {
			ag.RegisterTool(searchTool)
			logger.Debug("search tool registered", "agent_id", regCfg.ID)
		}

		// Load skills if enabled and not using skill manager
		if cfg.Skills.Enabled && ag.SkillManager() == nil {
			searchPaths := cfg.Skills.Paths
			if len(searchPaths) == 0 {
				searchPaths = nil
			}
			if err := ag.LoadSkills(searchPaths); err != nil {
				logger.Warn("failed to load skills", "agent_id", regCfg.ID, "error", err)
			}
		}

		return ag, nil
	}

	// Create agent registry
	agentRegistry := registry.New(registry.RegistryConfig{
		Factory: agentFactory,
		Logger:  logger,
		Defaults: &registry.AgentConfig{
			Provider: cfg.Agent.Provider,
			Model:    cfg.Agent.Model,
			APIKey:   cfg.Agent.APIKey,
			BaseURL:  cfg.Agent.BaseURL,
			Timezone: cfg.Agent.Timezone,
		},
	})
	defer agentRegistry.Close()

	// Determine if we're in multi-agent or single-agent mode
	var agentInstance *agent.Agent
	if len(cfg.Agents) > 0 {
		// Multi-agent mode: load all configured agents
		if err := registerAgentsFromConfig(ctx, cfg, agentRegistry, logger); err != nil {
			return err
		}

		// Get default agent for gateway (first enabled agent)
		var err error
		agentInstance, err = agentRegistry.Default()
		if err != nil {
			return fmt.Errorf("no default agent available: %w", err)
		}
		logger.Info("multi-agent mode enabled", "count", agentRegistry.Count())
	} else if cfg.Agent.APIKey != "" {
		// Single-agent mode (backward compatible)
		regCfg := &registry.AgentConfig{
			ID:           "default",
			Name:         "OmniAgent",
			Provider:     cfg.Agent.Provider,
			Model:        cfg.Agent.Model,
			APIKey:       cfg.Agent.APIKey,
			BaseURL:      cfg.Agent.BaseURL,
			Temperature:  cfg.Agent.Temperature,
			MaxTokens:    cfg.Agent.MaxTokens,
			SystemPrompt: cfg.Agent.SystemPrompt,
			Timezone:     cfg.Agent.Timezone,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := agentRegistry.Create(ctx, regCfg); err != nil {
			return fmt.Errorf("create default agent: %w", err)
		}

		var err error
		agentInstance, err = agentRegistry.Get("default")
		if err != nil {
			return fmt.Errorf("get default agent: %w", err)
		}

		logger.Info("single-agent mode",
			"provider", cfg.Agent.Provider,
			"model", cfg.Agent.Model,
			"registered_options", len(getAgentOptions()))
	} else {
		logger.Warn("no API key configured, agent disabled (messages will be echoed)")
	}

	// Personal-mode chat: single implicit user, private chat with the
	// agent. Team mode gets its own chat surface later; skip here.
	var personalChatHTTP *gateway.PersonalChatHTTP
	if !cfg.Team.Enabled && cfg.WebUIEnabled() {
		pch, cleanup, err := setupPersonalChat(ctx, cfg, agentInstance, logger)
		if err != nil {
			return err
		}
		defer cleanup()
		personalChatHTTP = pch
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
		// Single-replica guard: Discord WebSocket connections are stateful and
		// only one instance should connect. Running multiple replicas will cause
		// duplicate message handling (double-answers).
		if replicas := os.Getenv("OMNIAGENT_REPLICAS"); replicas != "" && replicas != "1" {
			logger.Warn("Discord requires single-replica deployment; multiple replicas will cause double-answers",
				"replicas", replicas)
		}

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
			AccountSID:          cfg.Channels.TwilioSMS.AccountSID,
			AuthToken:           cfg.Channels.TwilioSMS.AuthToken,
			PhoneNumber:         cfg.Channels.TwilioSMS.PhoneNumber,
			MessagingServiceSid: cfg.Channels.TwilioSMS.MessagingServiceSid,
			Logger:              logger,
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

	// Team mode: mount the auth/admin API and gate WebSocket upgrades on
	// the session cookie.
	if teamHTTP != nil {
		gw.Handle("/api/", teamHTTP.Handler())
		gw.SetConnectAuthorizer(teamHTTP.ConnectAuthorizer())
		logger.Info("team mode: auth API mounted at /api/, WebSocket cookie-authenticated")
	}

	// Web UI: capabilities is an exact pattern so it keeps resolving even
	// when team mode also claims the "/api/" subtree above — ServeMux
	// prefers the longer, more specific pattern.
	if webHTTP != nil {
		gw.Handle("/api/capabilities", webHTTP.CapabilitiesHandler())
		gw.Handle("/", webHTTP.AssetsHandler())
		logger.Info("web UI enabled: SPA served at /, capabilities at /api/capabilities")
	}

	// Personal-mode chat API.
	if personalChatHTTP != nil {
		gw.Handle("/api/chat", personalChatHTTP.ChatHandler())
		gw.Handle("/api/chat/messages", personalChatHTTP.SendHandler())
		logger.Info("personal chat API mounted at /api/chat")
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
