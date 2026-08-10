package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/agentrole"
	entuser "github.com/plexusone/omniagent/team/ent/user"
)

// Role-management sentinel errors (RMI-304).
var (
	// ErrUserNotFound is returned when a maintainer username resolves to no user.
	ErrUserNotFound = errors.New("user not found")
	// ErrLastOwner is returned when the sole owner tries to leave an agent that
	// still has other role holders (which would orphan its configuration).
	ErrLastOwner = errors.New("cannot leave: you are the agent's only owner")
	// ErrCannotRemoveOwner is returned when RemoveMaintainer targets an owner;
	// owners leave via LeaveAgent (or are reassigned by a superadmin).
	ErrCannotRemoveOwner = errors.New("cannot remove an owner")
)

// RoleView pairs a per-agent role with the holder's username, for display.
// Usernames are resolved via system context because RLS hides other users'
// rows from a non-superadmin; among an agent's editors, revealing co-editors'
// usernames is expected.
type RoleView struct {
	UserID    uuid.UUID `json:"userId"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// AddMaintainer grants the maintainer role to a user (by username) on an agent.
// Owner or superadmin only (CapManageMaintainers). Idempotent: if the user
// already holds a role on the agent, that role is returned unchanged (an
// existing owner is never demoted to maintainer).
func (s *Service) AddMaintainer(ctx context.Context, actor Actor, agentID uuid.UUID, username string) (*ent.AgentRole, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return nil, ErrUserNotFound
	}
	if err := s.requireOwner(ctx, actor, agentID); err != nil {
		return nil, err
	}

	// Resolve the target via system context: RLS hides other users from a
	// non-superadmin owner. Only the resolved user ID crosses back.
	var targetID uuid.UUID
	if err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		u, err := tx.User.Query().Where(entuser.UsernameEQ(username)).Only(ctx)
		if err != nil {
			return err
		}
		targetID = u.ID
		return nil
	}); err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("resolve maintainer: %w", err)
	}

	var role *ent.AgentRole
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		existing, err := tx.AgentRole.Query().
			Where(agentrole.AgentIDEQ(agentID), agentrole.UserIDEQ(targetID)).Only(ctx)
		if err == nil {
			role = existing // already owner or maintainer; leave as-is
			return nil
		}
		if !ent.IsNotFound(err) {
			return err
		}
		role, err = tx.AgentRole.Create().
			SetAgentID(agentID).SetUserID(targetID).SetRole(agentrole.RoleMaintainer).Save(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("add maintainer: %w", err)
	}
	return role, nil
}

// RemoveMaintainer removes another user's role from an agent. Owner or
// superadmin only. Owners cannot be removed this way (ErrCannotRemoveOwner);
// use LeaveAgent to remove yourself.
func (s *Service) RemoveMaintainer(ctx context.Context, actor Actor, agentID, userID uuid.UUID) error {
	if userID == actor.UserID {
		return fmt.Errorf("use LeaveAgent to remove yourself: %w", ErrForbidden)
	}
	if err := s.requireOwner(ctx, actor, agentID); err != nil {
		return err
	}
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		target, err := tx.AgentRole.Query().
			Where(agentrole.AgentIDEQ(agentID), agentrole.UserIDEQ(userID)).Only(ctx)
		if err != nil {
			return err
		}
		if target.Role == agentrole.RoleOwner {
			return ErrCannotRemoveOwner
		}
		_, err = tx.AgentRole.Delete().
			Where(agentrole.AgentIDEQ(agentID), agentrole.UserIDEQ(userID)).Exec(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return ErrForbidden
	}
	return err
}

// LeaveAgent removes the actor's own role from an agent (self-leave). The sole
// owner cannot leave while other role holders remain (it would orphan the
// agent's configuration); a superadmin reassigns ownership in that case
// (TRD §9). An agent with no remaining roles is permitted (orphaned, superadmin-
// reassignable) — matching the chat sole-owner semantics.
func (s *Service) LeaveAgent(ctx context.Context, actor Actor, agentID uuid.UUID) error {
	err := s.store.AsUser(ctx, actor.UserID, false, func(ctx context.Context, tx *ent.Tx) error {
		self, err := tx.AgentRole.Query().
			Where(agentrole.AgentIDEQ(agentID), agentrole.UserIDEQ(actor.UserID)).Only(ctx)
		if err != nil {
			return err // NotFound → not a role holder → ErrForbidden below
		}
		if self.Role == agentrole.RoleOwner {
			otherOwner, err := tx.AgentRole.Query().
				Where(agentrole.AgentIDEQ(agentID), agentrole.RoleEQ(agentrole.RoleOwner), agentrole.UserIDNEQ(actor.UserID)).Exist(ctx)
			if err != nil {
				return err
			}
			if !otherOwner {
				otherRole, err := tx.AgentRole.Query().
					Where(agentrole.AgentIDEQ(agentID), agentrole.UserIDNEQ(actor.UserID)).Exist(ctx)
				if err != nil {
					return err
				}
				if otherRole {
					return ErrLastOwner
				}
			}
		}
		_, err = tx.AgentRole.Delete().
			Where(agentrole.AgentIDEQ(agentID), agentrole.UserIDEQ(actor.UserID)).Exec(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return ErrForbidden
	}
	return err
}

// Roles lists an agent's role holders (owners and maintainers) with usernames,
// oldest first. The actor must be an editor or superadmin.
func (s *Service) Roles(ctx context.Context, actor Actor, agentID uuid.UUID) ([]RoleView, error) {
	if err := s.requireEditor(ctx, actor, agentID); err != nil {
		return nil, err
	}
	var roles []*ent.AgentRole
	if err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		roles, err = tx.AgentRole.Query().
			Where(agentrole.AgentIDEQ(agentID)).
			Order(ent.Asc(agentrole.FieldCreatedAt)).All(ctx)
		return err
	}); err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, len(roles))
	for i, r := range roles {
		ids[i] = r.UserID
	}
	names := make(map[uuid.UUID]string, len(ids))
	if len(ids) > 0 {
		if err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
			users, err := tx.User.Query().Where(entuser.IDIn(ids...)).All(ctx)
			if err != nil {
				return err
			}
			for _, u := range users {
				names[u.ID] = u.Username
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("resolve role usernames: %w", err)
		}
	}
	out := make([]RoleView, len(roles))
	for i, r := range roles {
		out[i] = RoleView{UserID: r.UserID, Username: names[r.UserID], Role: r.Role.String(), CreatedAt: r.CreatedAt}
	}
	return out, nil
}
