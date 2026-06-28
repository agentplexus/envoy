package openai

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/plexusone/omniagent/api/openai/auth"
	"github.com/plexusone/omniagent/api/openai/internal/ogen"
	"github.com/plexusone/omniagent/api/openai/operations"
	"github.com/plexusone/omniagent/api/openai/web"
)

// Server wraps the ogen-generated server with Chi router and Huma API.
type Server struct {
	handler    AgentHandler
	ogenSrv    *ogen.Server
	router     chi.Router
	humaAPI    huma.API
	config     Config
	usageStore *UsageStore
}

// Config configures the server.
type Config struct {
	// APIKeys is a list of valid API keys. If empty, no authentication is required.
	APIKeys []string

	// OpenAIPrefix is the URL prefix for OpenAI-compatible routes (default: "/openai/v1").
	OpenAIPrefix string

	// APIPrefix is the URL prefix for OmniAgent API routes (default: "/api").
	APIPrefix string

	// WebUI enables the embedded chat web UI at the root path.
	WebUI bool

	// PhoneNumber is the phone number to display in the web UI for voice calls.
	PhoneNumber string

	// Auth holds OAuth authentication configuration for the web UI.
	Auth *auth.Config

	// AAuth holds AAuth token validation configuration.
	// Deprecated: Use AgentAuth for unified ID-JAG and AAuth support.
	AAuth *auth.AAuthConfig

	// AgentAuth holds agent authentication configuration for both ID-JAG and AAuth.
	// This enables policy-based routing: ID-JAG for automatic auth, AAuth for sensitive actions.
	AgentAuth *auth.AgentAuthConfig

	// BaseURL is the public URL of the server (required for OAuth callbacks).
	BaseURL string

	// Logger is the logger for the server.
	Logger *slog.Logger
}

// Option configures the server.
type Option func(*Config)

// WithAPIKeys sets the valid API keys.
func WithAPIKeys(keys ...string) Option {
	return func(c *Config) {
		c.APIKeys = keys
	}
}

// WithOpenAIPrefix sets the URL prefix for OpenAI-compatible endpoints.
func WithOpenAIPrefix(prefix string) Option {
	return func(c *Config) {
		c.OpenAIPrefix = prefix
	}
}

// WithAPIPrefix sets the URL prefix for OmniAgent API endpoints.
func WithAPIPrefix(prefix string) Option {
	return func(c *Config) {
		c.APIPrefix = prefix
	}
}

// WithWebUI enables the embedded chat web UI.
func WithWebUI(enabled bool) Option {
	return func(c *Config) {
		c.WebUI = enabled
	}
}

// WithPhoneNumber sets the phone number to display in the web UI.
func WithPhoneNumber(phone string) Option {
	return func(c *Config) {
		c.PhoneNumber = phone
	}
}

// WithAuth sets the OAuth authentication configuration.
func WithAuth(cfg *auth.Config) Option {
	return func(c *Config) {
		c.Auth = cfg
	}
}

// WithAAuth sets the AAuth token validation configuration.
// Deprecated: Use WithAgentAuth for unified ID-JAG and AAuth support.
func WithAAuth(cfg *auth.AAuthConfig) Option {
	return func(c *Config) {
		c.AAuth = cfg
	}
}

// WithAgentAuth sets the agent authentication configuration.
// This enables both ID-JAG (automatic) and AAuth (human consent) token validation
// with policy-based routing based on action sensitivity.
func WithAgentAuth(cfg *auth.AgentAuthConfig) Option {
	return func(c *Config) {
		c.AgentAuth = cfg
	}
}

// WithBaseURL sets the public URL of the server for OAuth callbacks.
func WithBaseURL(url string) Option {
	return func(c *Config) {
		c.BaseURL = url
	}
}

// WithLogger sets the logger for the server.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// New creates a new OpenAI-compatible API server.
func New(handler AgentHandler, opts ...Option) (*Server, error) {
	cfg := Config{
		OpenAIPrefix: "/openai/v1",
		APIPrefix:    "/api",
		BaseURL:      "http://localhost:8080",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		handler:    handler,
		config:     cfg,
		usageStore: NewUsageStore(10000),
	}

	// 1. Create Chi router with middleware
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// 2. Setup OAuth authentication if enabled
	var sessions *auth.SessionManager
	if cfg.Auth != nil && cfg.Auth.Enabled {
		if err := cfg.Auth.Validate(); err != nil {
			return nil, err
		}

		sessions = auth.NewSessionManager(cfg.Auth)
		providers := auth.NewProviders(cfg.Auth, cfg.BaseURL)
		acl := auth.NewACL(cfg.Auth)

		// Enable development mode if using localhost
		if cfg.BaseURL == "http://localhost:8080" || cfg.BaseURL[:16] == "http://localhost" || cfg.BaseURL[:17] == "http://127.0.0.1:" {
			sessions.SetDevelopmentMode(true)
		}

		authHandlers, err := auth.NewHandlers(auth.HandlersConfig{
			Config:    cfg.Auth,
			Sessions:  sessions,
			Providers: providers,
			ACL:       acl,
			Assets:    web.Assets,
			Logger:    logger,
		})
		if err != nil {
			return nil, err
		}

		// Apply auth middleware
		authMiddleware := auth.NewMiddleware(sessions, cfg.Auth)
		r.Use(authMiddleware.RequireAuth)

		// Register auth routes
		r.Get("/login", authHandlers.HandleLogin)
		r.Get("/logout", authHandlers.HandleLogout)
		r.Get("/auth/github", authHandlers.HandleOAuthStart(auth.ProviderGitHub))
		r.Get("/auth/github/callback", authHandlers.HandleOAuthCallback(auth.ProviderGitHub))
		r.Get("/auth/google", authHandlers.HandleOAuthStart(auth.ProviderGoogle))
		r.Get("/auth/google/callback", authHandlers.HandleOAuthCallback(auth.ProviderGoogle))

		logger.Info("OAuth authentication enabled",
			"github", cfg.Auth.HasGitHub(),
			"google", cfg.Auth.HasGoogle(),
			"acl_emails", len(cfg.Auth.AllowedEmails),
			"acl_domains", len(cfg.Auth.AllowedDomains))
	}

	s.router = r

	// 2. Create ogen server for OpenAI-compatible endpoints
	ogenHandler := &ogenServerHandler{agent: handler}
	secHandler := &securityHandler{
		apiKeys: cfg.APIKeys,
	}

	// Initialize AgentAuth verifier (unified ID-JAG + AAuth) if configured
	if cfg.AgentAuth != nil && cfg.AgentAuth.Enabled {
		secHandler.agentAuthVerifier = auth.NewAgentAuthVerifier(cfg.AgentAuth)
		logger.Info("Agent authentication enabled",
			"idjag", cfg.AgentAuth.IDJAGEnabled,
			"aauth", cfg.AgentAuth.AAuthEnabled,
			"sensitive_actions", cfg.AgentAuth.SensitiveActions)
	}

	// Initialize legacy AAuth verifier if configured (deprecated, prefer AgentAuth)
	if cfg.AAuth != nil && cfg.AAuth.Enabled {
		secHandler.aauthVerifier = auth.NewAAuthVerifier(*cfg.AAuth)
		logger.Info("AAuth token validation enabled (deprecated, use AgentAuth)",
			"issuer", cfg.AAuth.IssuerURL,
			"audience", cfg.AAuth.Audience)
	}

	ogenSrv, err := ogen.NewServer(ogenHandler, secHandler)
	if err != nil {
		return nil, err
	}
	s.ogenSrv = ogenSrv

	// 3. Create streaming handler for chat completions
	streamingHandler := NewStreamingHandler(handler, ogenSrv)
	streamingHandler.SetUsageStore(s.usageStore)

	// 4. Mount ogen routes for OpenAI-compatible endpoints
	// These bypass Huma for streaming and ogen compatibility
	r.Post(cfg.OpenAIPrefix+"/chat/completions", streamingHandler.ServeHTTP)
	r.Get(cfg.OpenAIPrefix+"/models", s.handleOgenModels)
	r.Get(cfg.OpenAIPrefix+"/models/{model}", s.handleOgenModel)

	// 5. Create Huma API for custom endpoints
	humaConfig := huma.DefaultConfig("OmniAgent API", "1.0.0")
	humaConfig.Info.Description = "OpenAI-compatible API with OmniAgent extensions for agents, tools, cron jobs, usage analytics, and semantic memory."
	humaConfig.DocsPath = ""    // Disable default docs, we use Scalar
	humaConfig.SchemasPath = "" // Disable separate schema endpoint
	humaConfig.Servers = nil    // Let client infer server from request
	api := humachi.New(r, humaConfig)
	s.humaAPI = api

	// 6. Register Huma operations
	// Create adapter to wrap AgentHandler for operations
	opsHandler := &operationsHandlerAdapter{agent: handler}
	operations.RegisterHealthOperation(api)
	operations.RegisterToolOperations(api, opsHandler)
	operations.RegisterCronOperations(api, opsHandler)

	// Register agent operations if handler implements AgentManager
	if manager, ok := handler.(AgentManager); ok {
		opsManager := &operationsManagerAdapter{manager: manager}
		operations.RegisterAgentOperations(api, opsManager)
	}

	// Register memory operations if handler implements MemoryHandler
	if memHandler, ok := handler.(MemoryHandler); ok {
		opsMem := &operationsMemoryAdapter{handler: memHandler}
		operations.RegisterMemoryOperations(api, opsMem)
	}

	// Register usage operations with the usage store
	opsUsage := &operationsUsageAdapter{store: s.usageStore}
	operations.RegisterUsageOperations(api, opsUsage)

	// 7. Serve OpenAPI spec under /api/
	r.Get(cfg.APIPrefix+"/openapi.json", s.handleOpenAPIJSON)
	r.Get(cfg.APIPrefix+"/openapi.yaml", s.handleOpenAPIYAML)

	// 8. Serve API documentation with Scalar
	r.Get("/docs", s.handleScalarDocs)
	r.Get("/docs/*", s.handleScalarDocs)

	// 9. Serve embedded web UI if enabled
	if cfg.WebUI {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path != "/" {
				http.NotFound(w, req)
				return
			}
			content, err := fs.ReadFile(web.Assets, "index.html")
			if err != nil {
				http.Error(w, "Web UI not available", http.StatusInternalServerError)
				return
			}

			// Inject phone number if configured
			if cfg.PhoneNumber != "" {
				injection := []byte(`<script>window.OMNIAGENT_PHONE="` + cfg.PhoneNumber + `";</script>`)
				content = bytes.Replace(content, []byte("</head>"), append(injection, []byte("</head>")...), 1)
			}

			// Inject user info if authenticated
			if sessions != nil {
				if user := sessions.GetUser(req); user != nil {
					userJSON := fmt.Sprintf(`<script>window.OMNIAGENT_USER={"email":%q,"name":%q,"picture":%q};</script>`,
						user.Email, user.Name, user.Picture)
					content = bytes.Replace(content, []byte("</head>"), append([]byte(userJSON), []byte("</head>")...), 1)
				}
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			//nolint:gosec // G705: user data from trusted OAuth provider, %q format escapes strings
			_, _ = w.Write(content)
		})
	}

	return s, nil
}

// handleOgenModels handles GET /openai/v1/models through ogen
func (s *Server) handleOgenModels(w http.ResponseWriter, r *http.Request) {
	s.ogenSrv.ServeHTTP(w, r)
}

// handleOgenModel handles GET /openai/v1/models/{model} through ogen
func (s *Server) handleOgenModel(w http.ResponseWriter, r *http.Request) {
	s.ogenSrv.ServeHTTP(w, r)
}

// HumaAPI returns the Huma API for external access to the OpenAPI spec.
func (s *Server) HumaAPI() huma.API {
	return s.humaAPI
}

// Handler returns the http.Handler for embedding in other servers.
func (s *Server) Handler() http.Handler {
	return s.router
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server with appropriate timeouts.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // Allow longer for streaming responses
		IdleTimeout:  120 * time.Second,
	}
	return srv.ListenAndServe()
}

// UsageStore returns the server's usage store for external recording.
func (s *Server) UsageStore() *UsageStore {
	return s.usageStore
}

// GetMergedSpec returns the merged OpenAPI specification.
// This combines the ogen spec (OpenAI-compatible endpoints) with
// the Huma-generated spec (OmniAgent extension endpoints).
func (s *Server) GetMergedSpec() (any, error) {
	return s.getMergedSpec()
}

// securityHandler implements ogen.SecurityHandler.
type securityHandler struct {
	apiKeys            []string
	aauthVerifier      *auth.AAuthVerifier      // Deprecated: kept for backward compatibility
	agentAuthVerifier  *auth.AgentAuthVerifier  // New unified verifier
}

// aauthClaimsKey is the context key for AAuth claims (legacy).
type aauthClaimsKey struct{}

// agentAuthClaimsKey is the context key for AgentAuth claims.
type agentAuthClaimsKey struct{}

// GetAAuthClaims retrieves AAuth claims from the context.
// Deprecated: Use GetAgentAuthClaims for unified claim access.
func GetAAuthClaims(ctx context.Context) *auth.AAuthClaims {
	claims, _ := ctx.Value(aauthClaimsKey{}).(*auth.AAuthClaims)
	return claims
}

// GetAgentAuthClaims retrieves agent authentication claims from the context.
// Works for both ID-JAG and AAuth tokens.
func GetAgentAuthClaims(ctx context.Context) *auth.AgentAuthClaims {
	claims, _ := ctx.Value(agentAuthClaimsKey{}).(*auth.AgentAuthClaims)
	return claims
}

// HandleBearerAuth validates the bearer token.
// It supports API keys, ID-JAG tokens, and AAuth JWT tokens.
func (h *securityHandler) HandleBearerAuth(ctx context.Context, _ ogen.OperationName, t ogen.BearerAuth) (context.Context, error) {
	// Try AgentAuth (unified ID-JAG + AAuth) verification first if configured
	if h.agentAuthVerifier != nil && h.agentAuthVerifier.IsAgentToken(t.Token) {
		claims, err := h.agentAuthVerifier.Verify(ctx, t.Token)
		if err == nil {
			// Store claims in context for downstream handlers
			return context.WithValue(ctx, agentAuthClaimsKey{}, claims), nil
		}
		// If AgentAuth verification fails, fall through to legacy check
	}

	// Try legacy AAuth token validation if configured (deprecated)
	if h.aauthVerifier != nil && h.aauthVerifier.IsAAuthToken(t.Token) {
		claims, err := h.aauthVerifier.Verify(ctx, t.Token)
		if err == nil {
			// Store claims in context for downstream handlers
			return context.WithValue(ctx, aauthClaimsKey{}, claims), nil
		}
		// If AAuth verification fails, fall through to API key check
	}

	// If no API keys are configured and no verifiers, allow all requests
	if len(h.apiKeys) == 0 && h.aauthVerifier == nil && h.agentAuthVerifier == nil {
		return ctx, nil
	}

	// Check if the token matches any configured API key
	for _, key := range h.apiKeys {
		if t.Token == key {
			return ctx, nil
		}
	}

	return ctx, ErrUnauthorized
}

// ErrUnauthorized is returned when authentication fails.
var ErrUnauthorized = &unauthorizedError{}

type unauthorizedError struct{}

func (e *unauthorizedError) Error() string {
	return "unauthorized"
}

// Adapters to convert between package types and operations types

// operationsHandlerAdapter adapts AgentHandler to operations.Handler
type operationsHandlerAdapter struct {
	agent AgentHandler
}

func (a *operationsHandlerAdapter) ListTools(ctx context.Context) ([]operations.ToolInfo, error) {
	tools, err := a.agent.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]operations.ToolInfo, len(tools))
	for i, t := range tools {
		result[i] = operations.ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
			Category:    t.Category,
		}
	}
	return result, nil
}

func (a *operationsHandlerAdapter) ListCronJobs(ctx context.Context) ([]operations.CronJobInfo, error) {
	jobs, err := a.agent.ListCronJobs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]operations.CronJobInfo, len(jobs))
	for i, j := range jobs {
		result[i] = convertCronJobInfo(j)
	}
	return result, nil
}

func (a *operationsHandlerAdapter) GetCronJob(ctx context.Context, id string) (*operations.CronJobInfo, error) {
	job, err := a.agent.GetCronJob(ctx, id)
	if err != nil {
		return nil, err
	}
	result := convertCronJobInfo(*job)
	return &result, nil
}

func (a *operationsHandlerAdapter) CreateCronJob(ctx context.Context, req *operations.CreateCronJobRequest) (*operations.CronJobInfo, error) {
	input := &CreateCronJobRequest{
		Name:        req.Name,
		Description: req.Description,
		Schedule: CronScheduleInfo{
			Cron:     req.Schedule.Cron,
			Once:     req.Schedule.Once,
			Interval: req.Schedule.Interval,
		},
		Action: CronActionInfo{
			Type:           req.Action.Type,
			SessionID:      req.Action.SessionID,
			Message:        req.Action.Message,
			WebhookURL:     req.Action.WebhookURL,
			WebhookMethod:  req.Action.WebhookMethod,
			WebhookHeaders: req.Action.WebhookHeaders,
			WebhookBody:    req.Action.WebhookBody,
			ToolName:       req.Action.ToolName,
			ToolParams:     req.Action.ToolParams,
		},
	}
	job, err := a.agent.CreateCronJob(ctx, input)
	if err != nil {
		return nil, err
	}
	result := convertCronJobInfo(*job)
	return &result, nil
}

func (a *operationsHandlerAdapter) UpdateCronJob(ctx context.Context, id string, req *operations.UpdateCronJobRequest) (*operations.CronJobInfo, error) {
	input := &UpdateCronJobRequest{
		Name:        req.Name,
		Description: req.Description,
	}
	if req.Schedule != nil {
		input.Schedule = &CronScheduleInfo{
			Cron:     req.Schedule.Cron,
			Once:     req.Schedule.Once,
			Interval: req.Schedule.Interval,
		}
	}
	if req.Action != nil {
		input.Action = &CronActionInfo{
			Type:           req.Action.Type,
			SessionID:      req.Action.SessionID,
			Message:        req.Action.Message,
			WebhookURL:     req.Action.WebhookURL,
			WebhookMethod:  req.Action.WebhookMethod,
			WebhookHeaders: req.Action.WebhookHeaders,
			WebhookBody:    req.Action.WebhookBody,
			ToolName:       req.Action.ToolName,
			ToolParams:     req.Action.ToolParams,
		}
	}
	job, err := a.agent.UpdateCronJob(ctx, id, input)
	if err != nil {
		return nil, err
	}
	result := convertCronJobInfo(*job)
	return &result, nil
}

func (a *operationsHandlerAdapter) DeleteCronJob(ctx context.Context, id string) error {
	return a.agent.DeleteCronJob(ctx, id)
}

func (a *operationsHandlerAdapter) TriggerCronJob(ctx context.Context, id string) (*operations.CronJobResult, error) {
	result, err := a.agent.TriggerCronJob(ctx, id)
	if err != nil {
		return nil, err
	}
	return &operations.CronJobResult{
		Success:   result.Success,
		Output:    result.Output,
		Error:     result.Error,
		Duration:  result.Duration,
		StartedAt: result.StartedAt,
	}, nil
}

func (a *operationsHandlerAdapter) EnableCronJob(ctx context.Context, id string) error {
	return a.agent.EnableCronJob(ctx, id)
}

func (a *operationsHandlerAdapter) DisableCronJob(ctx context.Context, id string) error {
	return a.agent.DisableCronJob(ctx, id)
}

func convertCronJobInfo(j CronJobInfo) operations.CronJobInfo {
	return operations.CronJobInfo{
		ID:          j.ID,
		Name:        j.Name,
		Description: j.Description,
		Schedule: operations.CronScheduleInfo{
			Cron:     j.Schedule.Cron,
			Once:     j.Schedule.Once,
			Interval: j.Schedule.Interval,
		},
		Action: operations.CronActionInfo{
			Type:           j.Action.Type,
			SessionID:      j.Action.SessionID,
			Message:        j.Action.Message,
			WebhookURL:     j.Action.WebhookURL,
			WebhookMethod:  j.Action.WebhookMethod,
			WebhookHeaders: j.Action.WebhookHeaders,
			WebhookBody:    j.Action.WebhookBody,
			ToolName:       j.Action.ToolName,
			ToolParams:     j.Action.ToolParams,
		},
		Status:    j.Status,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
		LastRunAt: j.LastRunAt,
		NextRunAt: j.NextRunAt,
		RunCount:  j.RunCount,
		LastError: j.LastError,
	}
}

// operationsManagerAdapter adapts AgentManager to operations.AgentManager
type operationsManagerAdapter struct {
	manager AgentManager
}

func (a *operationsManagerAdapter) ListAgents(ctx context.Context) ([]operations.AgentInfo, error) {
	agents, err := a.manager.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]operations.AgentInfo, len(agents))
	for i, ag := range agents {
		result[i] = convertAgentInfo(ag)
	}
	return result, nil
}

func (a *operationsManagerAdapter) GetAgent(ctx context.Context, id string) (*operations.AgentInfo, error) {
	agent, err := a.manager.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	result := convertAgentInfo(*agent)
	return &result, nil
}

func (a *operationsManagerAdapter) CreateAgent(ctx context.Context, req *operations.CreateAgentRequest) (*operations.AgentInfo, error) {
	input := &CreateAgentRequest{
		ID:           req.ID,
		Name:         req.Name,
		Description:  req.Description,
		Provider:     req.Provider,
		Model:        req.Model,
		APIKey:       req.APIKey,
		BaseURL:      req.BaseURL,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		SystemPrompt: req.SystemPrompt,
		AllowedTools: req.AllowedTools,
		DeniedTools:  req.DeniedTools,
	}
	agent, err := a.manager.CreateAgent(ctx, input)
	if err != nil {
		return nil, err
	}
	result := convertAgentInfo(*agent)
	return &result, nil
}

func (a *operationsManagerAdapter) UpdateAgent(ctx context.Context, id string, req *operations.UpdateAgentRequest) (*operations.AgentInfo, error) {
	input := &UpdateAgentRequest{
		Name:         req.Name,
		Description:  req.Description,
		Provider:     req.Provider,
		Model:        req.Model,
		APIKey:       req.APIKey,
		BaseURL:      req.BaseURL,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		SystemPrompt: req.SystemPrompt,
		AllowedTools: req.AllowedTools,
		DeniedTools:  req.DeniedTools,
		Enabled:      req.Enabled,
	}
	agent, err := a.manager.UpdateAgent(ctx, id, input)
	if err != nil {
		return nil, err
	}
	result := convertAgentInfo(*agent)
	return &result, nil
}

func (a *operationsManagerAdapter) DeleteAgent(ctx context.Context, id string) error {
	return a.manager.DeleteAgent(ctx, id)
}

func (a *operationsManagerAdapter) CloneAgent(ctx context.Context, id string, req *operations.CloneAgentRequest) (*operations.AgentInfo, error) {
	input := &CloneAgentRequest{
		NewID:   req.NewID,
		NewName: req.NewName,
	}
	agent, err := a.manager.CloneAgent(ctx, id, input)
	if err != nil {
		return nil, err
	}
	result := convertAgentInfo(*agent)
	return &result, nil
}

func convertAgentInfo(a AgentInfo) operations.AgentInfo {
	return operations.AgentInfo{
		ID:           a.ID,
		Name:         a.Name,
		Description:  a.Description,
		Provider:     a.Provider,
		Model:        a.Model,
		Temperature:  a.Temperature,
		MaxTokens:    a.MaxTokens,
		SystemPrompt: a.SystemPrompt,
		AllowedTools: a.AllowedTools,
		DeniedTools:  a.DeniedTools,
		Enabled:      a.Enabled,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

// operationsMemoryAdapter adapts MemoryHandler to operations.MemoryHandler
type operationsMemoryAdapter struct {
	handler MemoryHandler
}

func (a *operationsMemoryAdapter) ListMemories(ctx context.Context, collection string, limit int) ([]operations.MemoryRecord, error) {
	memories, err := a.handler.ListMemories(ctx, collection, limit)
	if err != nil {
		return nil, err
	}
	result := make([]operations.MemoryRecord, len(memories))
	for i, m := range memories {
		result[i] = operations.MemoryRecord{
			Key:        m.Key,
			Content:    m.Content,
			Collection: m.Collection,
			Metadata:   m.Metadata,
			CreatedAt:  m.CreatedAt,
		}
	}
	return result, nil
}

func (a *operationsMemoryAdapter) SearchMemories(ctx context.Context, collection, query string, limit int) ([]operations.MemorySearchResult, error) {
	results, err := a.handler.SearchMemories(ctx, collection, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]operations.MemorySearchResult, len(results))
	for i, r := range results {
		out[i] = operations.MemorySearchResult{
			MemoryRecord: operations.MemoryRecord{
				Key:        r.Key,
				Content:    r.Content,
				Collection: r.Collection,
				Metadata:   r.Metadata,
				CreatedAt:  r.CreatedAt,
			},
			Score: r.Score,
		}
	}
	return out, nil
}

func (a *operationsMemoryAdapter) StoreMemory(ctx context.Context, req *operations.StoreMemoryRequest) (*operations.MemoryRecord, error) {
	input := &StoreMemoryRequest{
		Content:    req.Content,
		Key:        req.Key,
		Collection: req.Collection,
		Metadata:   req.Metadata,
	}
	memory, err := a.handler.StoreMemory(ctx, input)
	if err != nil {
		return nil, err
	}
	return &operations.MemoryRecord{
		Key:        memory.Key,
		Content:    memory.Content,
		Collection: memory.Collection,
		Metadata:   memory.Metadata,
		CreatedAt:  memory.CreatedAt,
	}, nil
}

func (a *operationsMemoryAdapter) DeleteMemory(ctx context.Context, collection, key string) error {
	return a.handler.DeleteMemory(ctx, collection, key)
}

func (a *operationsMemoryAdapter) ListCollections(ctx context.Context) ([]operations.MemoryCollection, error) {
	collections, err := a.handler.ListCollections(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]operations.MemoryCollection, len(collections))
	for i, c := range collections {
		result[i] = operations.MemoryCollection{
			Name:        c.Name,
			Description: c.Description,
			Count:       c.Count,
		}
	}
	return result, nil
}

// operationsUsageAdapter adapts UsageStore to operations.UsageStore
type operationsUsageAdapter struct {
	store *UsageStore
}

func (a *operationsUsageAdapter) GetSummary(since, until time.Time) *operations.UsageSummary {
	s := a.store.GetSummary(since, until)
	byModel := make(map[string]*operations.ModelUsage)
	for k, v := range s.ByModel {
		byModel[k] = &operations.ModelUsage{
			Model:            v.Model,
			Requests:         v.Requests,
			PromptTokens:     v.PromptTokens,
			CompletionTokens: v.CompletionTokens,
			TotalTokens:      v.TotalTokens,
			Cost:             v.Cost,
		}
	}
	byAgent := make(map[string]*operations.AgentUsage)
	for k, v := range s.ByAgent {
		byAgent[k] = &operations.AgentUsage{
			AgentID:          v.AgentID,
			Requests:         v.Requests,
			PromptTokens:     v.PromptTokens,
			CompletionTokens: v.CompletionTokens,
			TotalTokens:      v.TotalTokens,
			Cost:             v.Cost,
		}
	}
	return &operations.UsageSummary{
		TotalRequests:     s.TotalRequests,
		TotalPromptTokens: s.TotalPromptTokens,
		TotalCompTokens:   s.TotalCompTokens,
		TotalTokens:       s.TotalTokens,
		TotalCost:         s.TotalCost,
		AvgLatency:        s.AvgLatency,
		ByModel:           byModel,
		ByAgent:           byAgent,
		PeriodStart:       s.PeriodStart,
		PeriodEnd:         s.PeriodEnd,
	}
}

func (a *operationsUsageAdapter) GetTimeseries(since, until time.Time, interval string) *operations.UsageTimeseries {
	ts := a.store.GetTimeseries(since, until, interval)
	buckets := make([]operations.UsageBucket, len(ts.Buckets))
	for i, b := range ts.Buckets {
		buckets[i] = operations.UsageBucket{
			Timestamp:        b.Timestamp,
			Requests:         b.Requests,
			PromptTokens:     b.PromptTokens,
			CompletionTokens: b.CompletionTokens,
			TotalTokens:      b.TotalTokens,
			Cost:             b.Cost,
		}
	}
	return &operations.UsageTimeseries{
		Interval: ts.Interval,
		Buckets:  buckets,
	}
}

func (a *operationsUsageAdapter) GetRecords(limit int) []operations.UsageRecord {
	records := a.store.GetRecords(limit)
	result := make([]operations.UsageRecord, len(records))
	for i, r := range records {
		result[i] = operations.UsageRecord{
			ID:               r.ID,
			Timestamp:        r.Timestamp,
			Model:            r.Model,
			AgentID:          r.AgentID,
			SessionID:        r.SessionID,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			TotalTokens:      r.TotalTokens,
			Cost:             r.Cost,
			Latency:          r.Latency,
		}
	}
	return result
}
