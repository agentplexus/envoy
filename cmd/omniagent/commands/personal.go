package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/config"
	"github.com/plexusone/omniagent/gateway"
	"github.com/plexusone/omniagent/team"
	"github.com/plexusone/omniagent/team/auth"
	"github.com/plexusone/omniagent/team/chats"
	"github.com/plexusone/omniagent/team/ent"
	entuser "github.com/plexusone/omniagent/team/ent/user"
	teamstore "github.com/plexusone/omniagent/team/store"
)

// personalLocalEmail/personalLocalUsername identify the single implicit
// user in personal mode (TRD §1a): no login, no allowlist, one account.
const (
	personalLocalEmail    = "local@omniagent.personal"
	personalLocalUsername = "me"
)

// setupPersonalMode opens the personal-mode store and wires the single
// implicit user's private chat with the agent (TRD §1a "Personal" profile;
// subset of RMI-110/111/113 — no group chats, no mention policy, since a
// private chat always responds), plus — when auth.enabled=true —
// single-account login (TRD §4 "Personal, single-account") reusing the same
// magic-link/cookie machinery team mode uses, restricted to one account.
// team.database.app_dsn is reused as "the store DSN" regardless of
// team.enabled — the store package infers postgres vs. sqlite from the DSN
// itself (PLAN.md Deployment Modes). authHTTP is nil when auth is off. The
// returned cleanup closes the store; call it on shutdown.
func setupPersonalMode(ctx context.Context, cfg *config.Config, agentInstance *agent.Agent, logger *slog.Logger) (chatHTTP *gateway.PersonalChatHTTP, authHTTP *gateway.TeamHTTP, cleanup func(), err error) {
	storeCfg := teamstore.Config{
		AppDSN: cfg.Team.Database.AppDSN,
		Logger: logger,
	}
	if err := teamstore.Migrate(ctx, storeCfg); err != nil {
		return nil, nil, nil, fmt.Errorf("personal store migration: %w", err)
	}
	st, err := teamstore.Open(ctx, storeCfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open personal store: %w", err)
	}
	cleanup = func() {
		if cerr := st.Close(); cerr != nil {
			logger.Error("close personal store", "error", cerr)
		}
	}

	var userID uuid.UUID
	if cfg.Auth.Enabled {
		if verr := cfg.Auth.Validate(); verr != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("auth config: %w", verr)
		}

		// SuperadminEmail doubles as the sole allowed email: team.Service
		// treats it as always-allowed and promotes it to superadmin on
		// first login (team/bootstrap.go) — no allowlist row is ever
		// created, and the admin allowlist endpoint isn't mounted (see
		// gateway.TeamHTTPConfig.Personal), so no second account can ever
		// be added.
		teamSvc, terr := team.NewService(st, team.Config{
			SuperadminEmail: cfg.Auth.OwnerEmail,
			Logger:          logger,
		})
		if terr != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("personal team service: %w", terr)
		}

		// Bootstrap the owner account eagerly so the personal chat handler
		// below has a concrete user ID at startup, same as the no-auth
		// path; logging in later resolves to this same user by email.
		owner, _, eerr := teamSvc.EnsureUser(ctx, cfg.Auth.OwnerEmail)
		if eerr != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("bootstrap owner user: %w", eerr)
		}
		userID = owner.ID

		mailer, merr := buildMailer(cfg.Auth.SMTP, "personal auth", logger)
		if merr != nil {
			cleanup()
			return nil, nil, nil, merr
		}

		baseURL := strings.TrimRight(cfg.Auth.BaseURL, "/")
		secure := strings.HasPrefix(strings.ToLower(baseURL), "https://")
		authSvc, aerr := auth.NewService(st, teamSvc, mailer, auth.Config{
			BaseURL: baseURL,
			AppName: "OmniAgent",
			Logger:  logger,
		})
		if aerr != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("personal auth service: %w", aerr)
		}

		authHTTP = gateway.NewTeamHTTP(authSvc, teamSvc, gateway.TeamHTTPConfig{
			CookieSecure: secure,
			SessionTTL:   auth.DefaultSessionTTL,
			BaseURL:      baseURL,
			Logger:       logger,
			Personal:     true,
		})
		logger.Info("personal single-account auth enabled", "owner_email", cfg.Auth.OwnerEmail, "cookie_secure", secure)
	} else {
		uid, berr := bootstrapLocalUser(ctx, st)
		if berr != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("bootstrap local user: %w", berr)
		}
		userID = uid
	}

	// Only assign Agent when agentInstance is genuinely non-nil: passing a
	// nil *agent.Agent through the AgentProcessor interface would produce a
	// non-nil interface wrapping a nil pointer, which chats.Service can't
	// detect with a plain nil check.
	chatCfg := chats.Config{Logger: logger}
	if agentInstance != nil {
		chatCfg.Agent = agentInstance
	}
	chatSvc, err := chats.NewService(st, chatCfg)
	if err != nil {
		cleanup()
		return nil, nil, nil, fmt.Errorf("chats service: %w", err)
	}

	logger.Info("personal chat mode enabled: chat store ready")
	chatHTTP = gateway.NewPersonalChatHTTP(gateway.PersonalChatHTTPConfig{
		Chats:  chatSvc,
		UserID: userID,
		Logger: logger,
	})
	return chatHTTP, authHTTP, cleanup, nil
}

// bootstrapLocalUser returns the single implicit personal-mode user,
// creating it on first run.
func bootstrapLocalUser(ctx context.Context, st *teamstore.Store) (uuid.UUID, error) {
	var id uuid.UUID
	err := st.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		existing, err := tx.User.Query().Where(entuser.UsernameEQ(personalLocalUsername)).Only(ctx)
		if err == nil {
			id = existing.ID
			return nil
		}
		if !ent.IsNotFound(err) {
			return err
		}
		u, err := tx.User.Create().
			SetEmail(personalLocalEmail).SetUsername(personalLocalUsername).
			SetRole(entuser.RoleSuperadmin).Save(ctx)
		if err != nil {
			return err
		}
		id = u.ID
		return nil
	})
	return id, err
}
