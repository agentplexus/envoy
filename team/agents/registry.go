package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/agent"
)

// ErrInvalidVisibility is returned for a visibility value other than
// "private" or "listed".
var ErrInvalidVisibility = errors.New(`invalid visibility: use "private" or "listed"`)

// parseVisibility maps a string to the ent enum, rejecting anything else.
func parseVisibility(v string) (agent.Visibility, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "private":
		return agent.VisibilityPrivate, nil
	case "listed":
		return agent.VisibilityListed, nil
	default:
		return "", ErrInvalidVisibility
	}
}

// SetVisibility changes an agent's discoverability (private/listed). This is
// CapManageRegistry — an owner or maintainer (or superadmin) may set it. It is
// distinct from featured, which is superadmin-only curation.
func (s *Service) SetVisibility(ctx context.Context, actor Actor, agentID uuid.UUID, visibility string) (*ent.Agent, error) {
	vv, err := parseVisibility(visibility)
	if err != nil {
		return nil, err
	}
	// CapManageRegistry(visibility) resolves to editor-or-superadmin, i.e.
	// requireEditor (see the Can matrix); this also maps existence to
	// ErrNotFound and non-editors to ErrForbidden.
	if err := s.requireEditor(ctx, actor, agentID); err != nil {
		return nil, err
	}
	var a *ent.Agent
	err = s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		a, err = tx.Agent.UpdateOneID(agentID).SetVisibility(vv).Save(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("set visibility: %w", err)
	}
	return a, nil
}

// SetFeatured toggles the featured flag. Superadmin-only curation (TRD §9 Q1):
// owners/maintainers control visibility, never featured. A non-superadmin gets
// ErrForbidden; a missing agent gets ErrNotFound.
func (s *Service) SetFeatured(ctx context.Context, actor Actor, agentID uuid.UUID, featured bool) (*ent.Agent, error) {
	if !actor.Superadmin {
		return nil, ErrForbidden
	}
	var a *ent.Agent
	err := s.store.AsUser(ctx, actor.UserID, true, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		a, err = tx.Agent.UpdateOneID(agentID).SetFeatured(featured).Save(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("set featured: %w", err)
	}
	return a, nil
}
