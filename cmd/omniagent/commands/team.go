package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/plexusone/omniagent/config"
	"github.com/plexusone/omniagent/gateway"
	"github.com/plexusone/omniagent/team"
	"github.com/plexusone/omniagent/team/auth"
	"github.com/plexusone/omniagent/team/chats"
	"github.com/plexusone/omniagent/team/mail"
	teamstore "github.com/plexusone/omniagent/team/store"
)

// setupTeamMode migrates the team database and builds the auth/admin and chat
// HTTP handlers. The returned cleanup closes the store; call it on shutdown.
//
// The chat service is created without an agent: group messages are persisted
// and fanned out with no agent turn, and private DMs echo. Binding a chat to
// its agent's runtime (persona + skills + secrets) is RMI-113/RMI-309.
func setupTeamMode(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*gateway.TeamHTTP, *gateway.TeamChatHTTP, func(), error) {
	tc := cfg.Team
	if err := tc.Validate(); err != nil {
		return nil, nil, nil, fmt.Errorf("team config: %w", err)
	}

	storeCfg := teamstore.Config{
		AppDSN:     tc.Database.AppDSN,
		MigrateDSN: tc.Database.MigrateDSN,
		AppRole:    tc.Database.AppRole,
		Logger:     logger,
	}
	if err := teamstore.Migrate(ctx, storeCfg); err != nil {
		return nil, nil, nil, fmt.Errorf("team database migration: %w", err)
	}

	st, err := teamstore.Open(ctx, storeCfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open team database: %w", err)
	}
	cleanup := func() {
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
		return nil, nil, nil, fmt.Errorf("team service: %w", err)
	}

	mailer, err := buildMailer(tc.SMTP, "team mode", logger)
	if err != nil {
		cleanup()
		return nil, nil, nil, err
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
		return nil, nil, nil, fmt.Errorf("auth service: %w", err)
	}

	teamHTTP := gateway.NewTeamHTTP(authSvc, teamSvc, gateway.TeamHTTPConfig{
		CookieSecure: secure,
		SessionTTL:   auth.DefaultSessionTTL,
		BaseURL:      strings.TrimRight(tc.BaseURL, "/"),
		Logger:       logger,
	})

	// MemoryTenant scopes each chat turn's memory to TenantID=team,
	// SubjectID="chat:<id>" (RMI-OMNIAGENT-114): a chat's memories are isolated
	// to that chat. Takes effect once a per-agent runtime (RMI-309) actually
	// runs turns; until then chats stay silent and no memory is written.
	chatSvc, err := chats.NewService(st, chats.Config{
		MemoryTenant: chats.MemoryTenantTeam,
		Logger:       logger,
	})
	if err != nil {
		cleanup()
		return nil, nil, nil, fmt.Errorf("chats service: %w", err)
	}
	teamChatHTTP := gateway.NewTeamChatHTTP(gateway.TeamChatHTTPConfig{
		Chats:  chatSvc,
		Logger: logger,
	})

	logger.Info("team mode enabled: database migrated (schema + RLS)",
		"superadmin_email", tc.SuperadminEmail, "cookie_secure", secure)
	return teamHTTP, teamChatHTTP, cleanup, nil
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
