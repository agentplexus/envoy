package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/agent/registry"
	"github.com/plexusone/omniagent/api/openai"
	"github.com/plexusone/omniagent/api/openai/auth"
	openaiAdapter "github.com/plexusone/omniagent/openai"
	"github.com/plexusone/omniobserve/integrations/omnillm"
	"github.com/plexusone/omniobserve/llmops"
)

var (
	openaiAddress    string
	openaiAPIKeys    []string
	openaiModelID    string
	openaiUseSession bool
	openaiWebUI      bool
)

var openaiCmd = &cobra.Command{
	Use:   "openai",
	Short: "OpenAI-compatible API server commands",
	Long:  "Commands for managing the OpenAI-compatible API server.",
}

var openaiServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the OpenAI-compatible API server",
	Long: `Start an OpenAI-compatible API server for OmniAgent.

This enables web frontends like LibreChat, Open WebUI, and other
OpenAI-compatible clients to connect to OmniAgent.

Example:
  omniagent openai serve --address :8080
  omniagent openai serve --api-key sk-my-key
  omniagent openai serve --model-id my-agent
  omniagent openai serve --web-ui  # Enable built-in chat UI at http://localhost:8080/`,
	RunE: runOpenAIServer,
}

var (
	openaiSpecOutput string
	openaiSpecFormat string
)

var openaiSpecCmd = &cobra.Command{
	Use:   "spec",
	Short: "Generate OpenAPI specification",
	Long: `Generate a static OpenAPI specification file.

This generates the merged OpenAPI spec combining OpenAI-compatible endpoints
and OmniAgent extension endpoints (tools, agents, cron, usage, memory).

Examples:
  omniagent openai spec                           # Output JSON to stdout
  omniagent openai spec -o docs/api/openapi.json  # Write JSON to file
  omniagent openai spec -o docs/api/openapi.yaml --format yaml  # Write YAML
  omniagent openai spec -o docs/api/openapi.json -o docs/api/openapi.yaml  # Both formats`,
	RunE: runOpenAISpec,
}

func init() {
	openaiServeCmd.Flags().StringVar(&openaiAddress, "address", ":8080", "listen address")
	openaiServeCmd.Flags().StringArrayVar(&openaiAPIKeys, "api-key", nil, "API key(s) for authentication (can be specified multiple times)")
	openaiServeCmd.Flags().StringVar(&openaiModelID, "model-id", "omniagent", "model ID to report")
	openaiServeCmd.Flags().BoolVar(&openaiUseSession, "session", false, "enable session-based message processing")
	openaiServeCmd.Flags().BoolVar(&openaiWebUI, "web-ui", false, "enable embedded chat web UI at root path")

	openaiSpecCmd.Flags().StringVarP(&openaiSpecOutput, "output", "o", "", "output file path (default: stdout)")
	openaiSpecCmd.Flags().StringVar(&openaiSpecFormat, "format", "", "output format: json or yaml (default: infer from filename, or json)")

	openaiCmd.AddCommand(openaiServeCmd)
	openaiCmd.AddCommand(openaiSpecCmd)
}

func runOpenAIServer(cmd *cobra.Command, args []string) error {
	cfg := getConfig()
	logger := slog.Default()

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Initialize observability if enabled
	var llmopsProvider llmops.Provider
	var observabilityHook *omnillm.Hook
	if cfg.Observability.Enabled {
		providerName := cfg.Observability.Provider
		if providerName == "" {
			providerName = "slog"
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

	// Require API key configuration
	if cfg.Agent.APIKey == "" && len(cfg.Agents) == 0 {
		return fmt.Errorf("agent API key not configured (set agent.api_key in config)")
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
			Logger:       logger,
		}
		if observabilityHook != nil {
			agentConfig.ObservabilityHook = observabilityHook
		}

		ag, err := agent.New(agentConfig, getAgentOptions()...)
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

		// Register chart tool for rendering charts in the web UI
		chartTool := agent.NewChartTool()
		ag.RegisterTool(chartTool)
		logger.Debug("chart tool registered", "agent_id", regCfg.ID)

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
		},
	})
	defer agentRegistry.Close()

	// Determine if we're in multi-agent or single-agent mode
	var adapter openai.AgentHandler
	if len(cfg.Agents) > 0 {
		// Multi-agent mode: load all configured agents
		if err := registerAgentsFromConfig(ctx, cfg, agentRegistry, logger); err != nil {
			return err
		}

		// Use MultiAgentAdapter
		adapter = openaiAdapter.NewMultiAgentAdapter(agentRegistry,
			openaiAdapter.WithMultiModelOwner("plexusone"),
			openaiAdapter.WithMultiSession(openaiUseSession),
		)
		logger.Info("multi-agent mode enabled", "count", agentRegistry.Count())
	} else {
		// Single-agent mode (backward compatible)
		regCfg := &registry.AgentConfig{
			ID:           openaiModelID,
			Name:         "OmniAgent",
			Provider:     cfg.Agent.Provider,
			Model:        cfg.Agent.Model,
			APIKey:       cfg.Agent.APIKey,
			BaseURL:      cfg.Agent.BaseURL,
			Temperature:  cfg.Agent.Temperature,
			MaxTokens:    cfg.Agent.MaxTokens,
			SystemPrompt: cfg.Agent.SystemPrompt,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := agentRegistry.Create(ctx, regCfg); err != nil {
			return fmt.Errorf("create default agent: %w", err)
		}

		agentInstance, err := agentRegistry.Get(openaiModelID)
		if err != nil {
			return fmt.Errorf("get default agent: %w", err)
		}

		logger.Info("single-agent mode",
			"provider", cfg.Agent.Provider,
			"model", cfg.Agent.Model)

		// Use single AgentAdapter for backward compatibility
		adapter = openaiAdapter.NewAgentAdapter(agentInstance,
			openaiAdapter.WithModelID(openaiModelID),
			openaiAdapter.WithModelOwner("plexusone"),
			openaiAdapter.WithSession(openaiUseSession),
		)
	}

	// Create server (uses default prefixes: /openai/v1 for OpenAI-compat, /api for custom)
	serverOpts := []openai.Option{
		openai.WithWebUI(openaiWebUI),
		openai.WithLogger(logger),
	}
	if len(openaiAPIKeys) > 0 {
		serverOpts = append(serverOpts, openai.WithAPIKeys(openaiAPIKeys...))
		logger.Info("API key authentication enabled", "keys", len(openaiAPIKeys))
	} else {
		logger.Warn("no API keys configured, server is open")
	}

	// Add phone number if configured (for web UI voice calling)
	if phoneNumber := os.Getenv("TWILIO_PHONE_NUMBER"); phoneNumber != "" {
		serverOpts = append(serverOpts, openai.WithPhoneNumber(phoneNumber))
		logger.Info("phone number configured for web UI", "phone", phoneNumber)
	}

	// Load and configure OAuth authentication
	authConfig := auth.LoadFromEnv()
	if authConfig.Enabled {
		if err := authConfig.Validate(); err != nil {
			return fmt.Errorf("auth config validation: %w", err)
		}

		serverOpts = append(serverOpts, openai.WithAuth(authConfig))

		// Set base URL for OAuth callbacks
		baseURL := os.Getenv("AUTH_BASE_URL")
		if baseURL == "" {
			// Default to http://localhost with the configured address
			port := openaiAddress
			if strings.HasPrefix(port, ":") {
				baseURL = "http://localhost" + port
			} else {
				baseURL = "http://" + port
			}
		}
		serverOpts = append(serverOpts, openai.WithBaseURL(baseURL))

		logger.Info("OAuth authentication enabled",
			"base_url", baseURL,
			"github", authConfig.HasGitHub(),
			"google", authConfig.HasGoogle())
	}

	srv, err := openai.New(adapter, serverOpts...)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	logger.Info("starting OpenAI-compatible API server",
		"address", openaiAddress,
		"model_id", openaiModelID,
		"session", openaiUseSession,
		"web_ui", openaiWebUI)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(openaiAddress)
	}()

	// Wait for shutdown or error
	select {
	case <-ctx.Done():
		logger.Info("server stopped")
		return nil
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}
}

// runOpenAISpec generates the OpenAPI specification.
func runOpenAISpec(_ *cobra.Command, _ []string) error {
	// Create a minimal mock handler for spec generation
	handler := &specMockHandler{}

	// Create server with mock handler (uses default prefixes)
	srv, err := openai.New(handler)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	// Get the merged OpenAPI spec
	spec, err := srv.GetMergedSpec()
	if err != nil {
		return fmt.Errorf("get merged spec: %w", err)
	}

	// Determine output format
	format := openaiSpecFormat
	if format == "" && openaiSpecOutput != "" {
		ext := strings.ToLower(filepath.Ext(openaiSpecOutput))
		switch ext {
		case ".yaml", ".yml":
			format = "yaml"
		default:
			format = "json"
		}
	}
	if format == "" {
		format = "json"
	}

	// Encode the spec
	var output []byte
	switch format {
	case "yaml", "yml":
		output, err = yaml.Marshal(spec)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
	default:
		output, err = json.MarshalIndent(spec, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		output = append(output, '\n')
	}

	// Write output
	if openaiSpecOutput == "" {
		// Write to stdout
		_, err = os.Stdout.Write(output)
		return err
	}

	// Create parent directories if needed
	if dir := filepath.Dir(openaiSpecOutput); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
	}

	// Write to file
	if err := os.WriteFile(openaiSpecOutput, output, 0o600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("OpenAPI spec written to %s\n", openaiSpecOutput)
	return nil
}

// specMockHandler is a minimal handler for generating the OpenAPI spec.
// It implements all required interfaces to enable full spec generation.
type specMockHandler struct{}

func (h *specMockHandler) ChatCompletion(_ context.Context, _ *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *specMockHandler) ChatCompletionStream(_ context.Context, _ *openai.ChatCompletionRequest, _ func(*openai.ChatCompletionChunk) error) error {
	return fmt.Errorf("not implemented")
}

func (h *specMockHandler) ListModels(_ context.Context) ([]openai.Model, error) {
	return []openai.Model{{ID: "omniagent", Object: "model", Created: time.Now().Unix(), OwnedBy: "omniagent"}}, nil
}

func (h *specMockHandler) GetModel(_ context.Context, modelID string) (*openai.Model, error) {
	return &openai.Model{ID: modelID, Object: "model", Created: time.Now().Unix(), OwnedBy: "omniagent"}, nil
}

func (h *specMockHandler) ListTools(_ context.Context) ([]openai.ToolInfo, error) {
	return []openai.ToolInfo{}, nil
}

func (h *specMockHandler) ListCronJobs(_ context.Context) ([]openai.CronJobInfo, error) {
	return []openai.CronJobInfo{}, nil
}

func (h *specMockHandler) GetCronJob(_ context.Context, _ string) (*openai.CronJobInfo, error) {
	return nil, fmt.Errorf("not found")
}

func (h *specMockHandler) CreateCronJob(_ context.Context, _ *openai.CreateCronJobRequest) (*openai.CronJobInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *specMockHandler) UpdateCronJob(_ context.Context, _ string, _ *openai.UpdateCronJobRequest) (*openai.CronJobInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *specMockHandler) DeleteCronJob(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}

func (h *specMockHandler) TriggerCronJob(_ context.Context, _ string) (*openai.CronJobResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *specMockHandler) EnableCronJob(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}

func (h *specMockHandler) DisableCronJob(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}

// AgentManager interface implementation
func (h *specMockHandler) ListAgents(_ context.Context) ([]openai.AgentInfo, error) {
	return []openai.AgentInfo{}, nil
}

func (h *specMockHandler) GetAgent(_ context.Context, _ string) (*openai.AgentInfo, error) {
	return nil, fmt.Errorf("not found")
}

func (h *specMockHandler) CreateAgent(_ context.Context, _ *openai.CreateAgentRequest) (*openai.AgentInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *specMockHandler) UpdateAgent(_ context.Context, _ string, _ *openai.UpdateAgentRequest) (*openai.AgentInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *specMockHandler) DeleteAgent(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}

func (h *specMockHandler) CloneAgent(_ context.Context, _ string, _ *openai.CloneAgentRequest) (*openai.AgentInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// MemoryHandler interface implementation
func (h *specMockHandler) ListMemories(_ context.Context, _ string, _ int) ([]openai.MemoryRecord, error) {
	return []openai.MemoryRecord{}, nil
}

func (h *specMockHandler) SearchMemories(_ context.Context, _, _ string, _ int) ([]openai.MemorySearchResult, error) {
	return []openai.MemorySearchResult{}, nil
}

func (h *specMockHandler) StoreMemory(_ context.Context, _ *openai.StoreMemoryRequest) (*openai.MemoryRecord, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *specMockHandler) DeleteMemory(_ context.Context, _, _ string) error {
	return fmt.Errorf("not implemented")
}

func (h *specMockHandler) ListCollections(_ context.Context) ([]openai.MemoryCollection, error) {
	return []openai.MemoryCollection{}, nil
}
