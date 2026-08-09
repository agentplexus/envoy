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
	"github.com/plexusone/omniagent/team/mail"
	teamstore "github.com/plexusone/omniagent/team/store"
)

// setupTeamMode migrates the team database and builds the auth/admin HTTP
// handler. The returned cleanup closes the store; call it on shutdown.
func setupTeamMode(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*gateway.TeamHTTP, func(), error) {
	tc := cfg.Team
	if err := tc.Validate(); err != nil {
		return nil, nil, fmt.Errorf("team config: %w", err)
	}

	storeCfg := teamstore.Config{
		AppDSN:     tc.Database.AppDSN,
		MigrateDSN: tc.Database.MigrateDSN,
		AppRole:    tc.Database.AppRole,
		Logger:     logger,
	}
	if err := teamstore.Migrate(ctx, storeCfg); err != nil {
		return nil, nil, fmt.Errorf("team database migration: %w", err)
	}

	st, err := teamstore.Open(ctx, storeCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open team database: %w", err)
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
		return nil, nil, fmt.Errorf("team service: %w", err)
	}

	mailer, err := buildMailer(tc.SMTP, "team mode", logger)
	if err != nil {
		cleanup()
		return nil, nil, err
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
		return nil, nil, fmt.Errorf("auth service: %w", err)
	}

	teamHTTP := gateway.NewTeamHTTP(authSvc, teamSvc, gateway.TeamHTTPConfig{
		CookieSecure: secure,
		SessionTTL:   auth.DefaultSessionTTL,
		BaseURL:      strings.TrimRight(tc.BaseURL, "/"),
		Logger:       logger,
	})

	logger.Info("team mode enabled: database migrated (schema + RLS)",
		"superadmin_email", tc.SuperadminEmail, "cookie_secure", secure)
	return teamHTTP, cleanup, nil
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
