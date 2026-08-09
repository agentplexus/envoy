package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/config"
	"github.com/plexusone/omniagent/gateway"
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

// setupPersonalChat opens the personal-mode chat store and bootstraps the
// single implicit local user's private chat with the agent (TRD §1a
// "Personal" profile; subset of RMI-110/111/113 — no group chats, no
// mention policy, since a private chat always responds). team.database.app_dsn
// is reused as "the chat store DSN" regardless of team.enabled — the store
// package infers postgres vs. sqlite from the DSN itself (PLAN.md
// Deployment Modes). The returned cleanup closes the store; call it on
// shutdown.
func setupPersonalChat(ctx context.Context, cfg *config.Config, agentInstance *agent.Agent, logger *slog.Logger) (*gateway.PersonalChatHTTP, func(), error) {
	storeCfg := teamstore.Config{
		AppDSN: cfg.Team.Database.AppDSN,
		Logger: logger,
	}
	if err := teamstore.Migrate(ctx, storeCfg); err != nil {
		return nil, nil, fmt.Errorf("personal chat store migration: %w", err)
	}
	st, err := teamstore.Open(ctx, storeCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open personal chat store: %w", err)
	}
	cleanup := func() {
		if cerr := st.Close(); cerr != nil {
			logger.Error("close personal chat store", "error", cerr)
		}
	}

	userID, err := bootstrapLocalUser(ctx, st)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("bootstrap local user: %w", err)
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
		return nil, nil, fmt.Errorf("chats service: %w", err)
	}

	logger.Info("personal chat mode enabled: chat store ready", "local_user", personalLocalUsername)
	return gateway.NewPersonalChatHTTP(gateway.PersonalChatHTTPConfig{
		Chats:  chatSvc,
		UserID: userID,
		Logger: logger,
	}), cleanup, nil
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
