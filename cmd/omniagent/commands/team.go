package commands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/config"
	"github.com/plexusone/omniagent/gateway"
	"github.com/plexusone/omniagent/skills"
	"github.com/plexusone/omniagent/team"
	"github.com/plexusone/omniagent/team/agentruntime"
	"github.com/plexusone/omniagent/team/agents"
	"github.com/plexusone/omniagent/team/auth"
	"github.com/plexusone/omniagent/team/chats"
	"github.com/plexusone/omniagent/team/mail"
	"github.com/plexusone/omniagent/team/secrets"
	teamstore "github.com/plexusone/omniagent/team/store"
	"github.com/plexusone/omnivault/providers/file"
	"github.com/plexusone/omnivault/providers/memory"
	"github.com/plexusone/omnivault/vault"
)

// setupTeamMode migrates the team database and builds the auth/admin and chat
// HTTP handlers. The returned cleanup closes the per-agent runtime cache and the
// store; call it on shutdown.
//
// Agent-bound chats route their turns to a per-agent runtime instance built from
// the agent's persona + enabled skills (RMI-309) and its agent-scoped secrets
// (RMI-310, when a secret vault is configured), gated by the RMI-113 mention
// policy and scoped per chat (RMI-114). The runtime is wired only when an LLM API
// key is configured; without one, agent-bound chats stay silent and agent-less
// private DMs echo.
func setupTeamMode(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*gateway.TeamHTTP, *gateway.TeamChatHTTP, *gateway.AgentsHTTP, func(), error) {
	tc := cfg.Team
	if err := tc.Validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("team config: %w", err)
	}

	storeCfg := teamstore.Config{
		AppDSN:     tc.Database.AppDSN,
		MigrateDSN: tc.Database.MigrateDSN,
		AppRole:    tc.Database.AppRole,
		Logger:     logger,
	}
	if err := teamstore.Migrate(ctx, storeCfg); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("team database migration: %w", err)
	}

	st, err := teamstore.Open(ctx, storeCfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("open team database: %w", err)
	}
	// runtimeCache and secretVault are assigned below when a per-agent runtime is
	// wired; captured by reference so cleanup evicts/closes resident instances and
	// the backing secret vault first.
	var (
		runtimeCache *agentruntime.Cache
		secretVault  io.Closer
	)
	cleanup := func() {
		if runtimeCache != nil {
			if cerr := runtimeCache.Close(); cerr != nil {
				logger.Error("close agent runtime cache", "error", cerr)
			}
		}
		if secretVault != nil {
			if cerr := secretVault.Close(); cerr != nil {
				logger.Error("close team secret vault", "error", cerr)
			}
		}
		if cerr := st.Close(); cerr != nil {
			logger.Error("close team store", "error", cerr)
		}
	}

	teamSvc, err := team.NewService(st, team.Config{
		SuperadminEmail: tc.SuperadminEmail,
		AgentHandle:     tc.AgentHandle,
		Logger:          logger,
	})
	if err != nil {
		cleanup()
		return nil, nil, nil, nil, fmt.Errorf("team service: %w", err)
	}

	mailer, err := buildMailer(tc.SMTP, "team mode", logger)
	if err != nil {
		cleanup()
		return nil, nil, nil, nil, err
	}

	// Cookies are Secure over https; plain-HTTP dev drops the __Host- prefix.
	secure := strings.HasPrefix(strings.ToLower(tc.BaseURL), "https://")

	authSvc, err := auth.NewService(st, teamSvc, mailer, auth.Config{
		BaseURL: strings.TrimRight(tc.BaseURL, "/"),
		AppName: "OmniAgent",
		Logger:  logger,
	})
	if err != nil {
		cleanup()
		return nil, nil, nil, nil, fmt.Errorf("auth service: %w", err)
	}

	googleProvider, githubProvider, err := buildSSOProviders(ctx, tc.SSO, strings.TrimRight(tc.BaseURL, "/"), logger)
	if err != nil {
		cleanup()
		return nil, nil, nil, nil, err
	}

	teamHTTP := gateway.NewTeamHTTP(authSvc, teamSvc, gateway.TeamHTTPConfig{
		CookieSecure:   secure,
		SessionTTL:     auth.DefaultSessionTTL,
		BaseURL:        strings.TrimRight(tc.BaseURL, "/"),
		Logger:         logger,
		GoogleProvider: googleProvider,
		GitHubProvider: githubProvider,
	})

	// Agents registry (INIT-OMNIAGENT-005): gates agent-bound chat creation
	// (Can(CapCreateChat)) and, as the runtime's ConfigLoader, supplies each
	// agent's persona/model/skills in system context.
	agentsSvc, err := agents.NewService(st, agents.Config{
		AvailableSkills: cfg.Skills.Includes,
		Logger:          logger,
	})
	if err != nil {
		cleanup()
		return nil, nil, nil, nil, fmt.Errorf("agents service: %w", err)
	}

	// Team secret service (INIT-OMNIAGENT-004): a single OmniVault-backed store,
	// namespaced per agent. Built up front (independent of the LLM key) so the
	// owner/maintainer management API works even before an LLM is configured.
	// Nil when no provider is set — secrets are neither injected nor manageable.
	secretSvc, sv, serr := buildSecretService(tc.Secrets, logger)
	if serr != nil {
		cleanup()
		return nil, nil, nil, nil, fmt.Errorf("team secrets: %w", serr)
	}
	secretVault = sv

	// Per-agent runtime (RMI-OMNIAGENT-309): build each agent-bound chat's
	// instance lazily from its persona + enabled skills, held in a bounded LRU.
	// Wired only when an LLM API key is configured — without one a built agent
	// could not complete a turn, so leaving the runtime nil keeps agent-bound
	// chats silent (a wrong/failed reply is worse than none), exactly as before.
	var runtime chats.AgentRuntime
	if cfg.Agent.APIKey != "" {
		// Agent-scoped secrets (RMI-OMNIAGENT-310): when the secret service is
		// configured, each instance is built with its own agent's secrets injected
		// into secrets-aware skills (per-agent MCP subprocess env). A nil service
		// yields a nil source — no secrets injected, unchanged behavior.
		var secretSrc agentruntime.SecretSource
		if secretSvc != nil {
			secretSrc = agentSecretSource{svc: secretSvc}
		}

		builder := agentruntime.NewAgentBuilder(agentruntime.BuilderConfig{
			Defaults: agent.Config{
				Provider:     cfg.Agent.Provider,
				Model:        cfg.Agent.Model,
				APIKey:       cfg.Agent.APIKey,
				BaseURL:      cfg.Agent.BaseURL,
				Temperature:  cfg.Agent.Temperature,
				MaxTokens:    cfg.Agent.MaxTokens,
				SystemPrompt: cfg.Agent.SystemPrompt,
				Timezone:     cfg.Agent.Timezone,
			},
			BaseOptions: getAgentOptions(),
			Secrets:     secretSrc,
			Logger:      logger,
		})
		runtimeCache, err = agentruntime.New(agentruntime.Config{
			Loader:  agentConfigLoader{svc: agentsSvc},
			Builder: builder,
			Logger:  logger,
		})
		if err != nil {
			cleanup()
			return nil, nil, nil, nil, fmt.Errorf("agent runtime: %w", err)
		}
		runtime = runtimeCache
		logger.Info("team mode: per-agent runtime enabled",
			"provider", cfg.Agent.Provider, "model", cfg.Agent.Model,
			"secrets", secretSrc != nil)
	} else {
		logger.Warn("team mode: no agent API key configured — agent-bound chats will not respond")
	}

	// MemoryTenant scopes each chat turn's memory to TenantID=team,
	// SubjectID="chat:<id>" (RMI-OMNIAGENT-114): a chat's memories are isolated
	// to that chat.
	chatSvc, err := chats.NewService(st, chats.Config{
		Agents:       agentsSvc,
		Runtime:      runtime,
		MemoryTenant: chats.MemoryTenantTeam,
		Logger:       logger,
	})
	if err != nil {
		cleanup()
		return nil, nil, nil, nil, fmt.Errorf("chats service: %w", err)
	}
	teamChatHTTP := gateway.NewTeamChatHTTP(gateway.TeamChatHTTPConfig{
		Chats:  chatSvc,
		Logger: logger,
	})

	// Agents management + discovery surface (INIT-OMNIAGENT-005 Phase 5): the
	// owner/maintainer config area, the catalog, and superadmin curation, all
	// over the same agentsSvc gate. When a secret vault is configured, it also
	// serves the agent Secrets management surface (INIT-OMNIAGENT-004): an
	// owner/maintainer sets values that the agent's skills receive, and the
	// declared-secrets catalog comes from the loaded skills' SKILL.md metadata.
	agentsHTTPCfg := gateway.AgentsHTTPConfig{
		Agents: agentsSvc,
		Logger: logger,
	}
	if secretSvc != nil {
		agentsHTTPCfg.Secrets = secretSvc
		agentsHTTPCfg.SkillSecretDecls = skillSecretDeclLookup(cfg.Skills, logger)
		if runtimeCache != nil {
			agentsHTTPCfg.InvalidateAgent = runtimeCache.Invalidate
		}
	}
	agentsHTTP := gateway.NewAgentsHTTP(agentsHTTPCfg)

	logger.Info("team mode enabled: database migrated (schema + RLS)",
		"superadmin_email", tc.SuperadminEmail, "cookie_secure", secure)
	return teamHTTP, teamChatHTTP, agentsHTTP, cleanup, nil
}

// agentConfigLoader adapts *agents.Service to agentruntime.ConfigLoader,
// mapping the agents-owned RuntimeConfig to the runtime's AgentConfig. The
// adapter lives at the composition root so neither package depends on the other.
type agentConfigLoader struct {
	svc *agents.Service
}

func (l agentConfigLoader) AgentSlug(ctx context.Context, agentID uuid.UUID) (string, error) {
	return l.svc.AgentSlugByID(ctx, agentID)
}

func (l agentConfigLoader) LoadConfig(ctx context.Context, agentID uuid.UUID) (agentruntime.AgentConfig, error) {
	rc, err := l.svc.LoadRuntimeConfig(ctx, agentID)
	if err != nil {
		return agentruntime.AgentConfig{}, err
	}
	return agentruntime.AgentConfig{
		ID:       rc.ID,
		Slug:     rc.Slug,
		Name:     rc.Name,
		Persona:  rc.Persona,
		Model:    rc.Model,
		Provider: rc.Provider,
		Skills:   rc.Skills,
	}, nil
}

// agentSecretSource adapts *secrets.Service to agentruntime.SecretSource,
// resolving an agent's agent-scoped secrets into the env map its instance
// injects. The adapter lives at the composition root so neither package depends
// on the other.
type agentSecretSource struct {
	svc *secrets.Service
}

func (s agentSecretSource) ResolveSecrets(ctx context.Context, agentID uuid.UUID) (map[string]string, error) {
	return s.svc.ResolveAgentSecrets(ctx, agentID)
}

// buildSecretService constructs the team secret service from config, returning
// it alongside the backing vault to close on shutdown. Returns (nil, nil, nil)
// when no secret provider is configured, so no secrets are injected and the
// secrets management surface stays unregistered (unchanged behavior). Tenant
// isolation (per-agent namespaces) is applied by secrets.Service, so the raw
// deployment vault is passed through. The same service backs both the runtime's
// SecretSource (read/inject) and the owner/maintainer management API (write).
func buildSecretService(cfg config.TeamSecretsConfig, logger *slog.Logger) (*secrets.Service, io.Closer, error) {
	if cfg.Provider == "" {
		return nil, nil, nil
	}

	var v vault.Vault
	switch cfg.Provider {
	case "memory":
		v = memory.New()
	case "file":
		fv, err := file.New(file.Config{Directory: cfg.Dir})
		if err != nil {
			return nil, nil, fmt.Errorf("open file secret vault %q: %w", cfg.Dir, err)
		}
		v = fv
	default:
		return nil, nil, fmt.Errorf("unsupported team secrets provider %q", cfg.Provider)
	}

	svc, err := secrets.NewService(secrets.Config{Vault: v, Logger: logger})
	if err != nil {
		if cerr := v.Close(); cerr != nil {
			logger.Error("close secret vault after service init failure", "error", cerr)
		}
		return nil, nil, err
	}
	return svc, v, nil
}

// skillSecretDeclLookup returns a best-effort lookup from a skill name to the
// secrets it declares in its SKILL.md, driving the agent Secrets catalog UI.
// It loads directory skills once at startup; a load failure degrades to "no
// declarations" rather than failing the gateway (the set/inject path works by
// env-var name regardless). Embedded skill packs are not covered by the
// catalog at v1.
func skillSecretDeclLookup(cfg config.SkillsConfig, logger *slog.Logger) func(string) []gateway.SecretDecl {
	dirs := cfg.Paths
	if len(dirs) == 0 {
		dirs = skills.DefaultSearchPaths()
	}
	mgr := skills.NewManager(skills.ManagerConfig{
		Dirs:     dirs,
		Includes: cfg.Includes,
		Excludes: cfg.Excludes,
	})
	if err := mgr.Load(); err != nil {
		logger.Warn("skill secrets: could not load skills for the declaration catalog", "error", err)
		return func(string) []gateway.SecretDecl { return nil }
	}
	return func(name string) []gateway.SecretDecl {
		sk := mgr.Get(name)
		if sk == nil {
			return nil
		}
		decls := sk.DeclaredSecrets()
		out := make([]gateway.SecretDecl, len(decls))
		for i, d := range decls {
			out[i] = gateway.SecretDecl{
				Name:        d.Name,
				Description: d.Description,
				Env:         d.EnvVar(),
				Required:    d.Required,
			}
		}
		return out
	}
}

// googleDiscoveryTimeout bounds NewGoogleProvider's OIDC discovery call so a
// hung DNS/TLS attempt cannot hang gateway startup indefinitely.
const googleDiscoveryTimeout = 15 * time.Second

// buildSSOProviders constructs the configured Google/GitHub SSO providers,
// returning gateway.SSOProvider values (nil when a provider is not
// configured, so its routes are not registered). Google performs OIDC
// discovery — a real network call — at construction: a failure here is
// fatal, matching every other external-dependency error in setupTeamMode
// (an operator who configured Google SSO intends it to work, not silently
// never offer the button).
func buildSSOProviders(ctx context.Context, cfg config.TeamSSOConfig, baseURL string, logger *slog.Logger) (google, github gateway.SSOProvider, err error) {
	if cfg.Google.ClientID != "" && cfg.Google.ClientSecret != "" {
		discoverCtx, cancel := context.WithTimeout(ctx, googleDiscoveryTimeout)
		defer cancel()
		gp, gerr := auth.NewGoogleProvider(discoverCtx, cfg.Google.ClientID, cfg.Google.ClientSecret, baseURL+"/api/auth/google/callback")
		if gerr != nil {
			return nil, nil, fmt.Errorf("google sso: %w", gerr)
		}
		google = gp
		logger.Info("team mode: Google SSO enabled")
	}
	if cfg.GitHub.ClientID != "" && cfg.GitHub.ClientSecret != "" {
		github = auth.NewGitHubProvider(cfg.GitHub.ClientID, cfg.GitHub.ClientSecret, baseURL+"/api/auth/github/callback")
		logger.Info("team mode: GitHub SSO enabled")
	}
	return google, github, nil
}

// buildMailer returns a real SMTP mailer when configured, otherwise a log
// mailer so the magic-link flow works out of the box (the link is logged
// instead of emailed — dev only). label identifies the caller in log lines.
func buildMailer(smtp config.TeamSMTPConfig, label string, logger *slog.Logger) (mail.Mailer, error) {
	if smtp.Host == "" {
		logger.Warn(label + ": no SMTP configured — magic links will be LOGGED, not emailed (dev only)")
		return mail.NewLogMailer(logger), nil
	}
	mailer, err := mail.NewSMTPMailer(mail.SMTPConfig{
		Host:     smtp.Host,
		Port:     smtp.Port,
		Username: smtp.Username,
		Password: smtp.Password,
		From:     smtp.From,
	})
	if err != nil {
		return nil, fmt.Errorf("smtp mailer: %w", err)
	}
	logger.Info(label+": SMTP mailer configured", "host", smtp.Host)
	return mailer, nil
}
