package agents

import (
	"context"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/agent"
	"github.com/plexusone/omniagent/team/ent/agentrole"
)

// CatalogEntry is one discoverable agent as seen by a particular caller.
type CatalogEntry struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"`
	Featured    bool      `json:"featured"`
	// CanStart reports whether this caller may start a chat with the agent
	// (Can(CapCreateChat)).
	CanStart bool `json:"canStart"`
}

// Catalog is the discovery view: superadmin-curated Featured agents and
// owner-opted Listed agents, each scoped to what the caller may see.
type Catalog struct {
	Featured []CatalogEntry `json:"featured"`
	Listed   []CatalogEntry `json:"listed"`
}

// Catalog returns the agents discoverable by the actor, in two sections
// (TRD §4): Featured (superadmin curation) and Listed (owner-opted
// visibility=listed, excluding those already in Featured). Both are computed
// in the actor's scope, so RLS filters out agents the caller cannot see — a
// featured-but-private agent surfaces only to its editors/superadmin, never
// leaking a private agent to the whole deployment.
func (s *Service) Catalog(ctx context.Context, actor Actor) (*Catalog, error) {
	var featured, listed []*ent.Agent
	editor := make(map[uuid.UUID]bool)
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		featured, err = tx.Agent.Query().
			Where(agent.Featured(true)).
			Order(ent.Asc(agent.FieldName)).All(ctx)
		if err != nil {
			return err
		}
		listed, err = tx.Agent.Query().
			Where(agent.VisibilityEQ(agent.VisibilityListed), agent.Featured(false)).
			Order(ent.Asc(agent.FieldName)).All(ctx)
		if err != nil {
			return err
		}
		// The actor's own editor roles, so CanStart is correct for
		// private-but-visible (e.g. featured-private) entries without an extra
		// query per row.
		roles, err := tx.AgentRole.Query().Where(agentrole.UserIDEQ(actor.UserID)).All(ctx)
		if err != nil {
			return err
		}
		for _, r := range roles {
			editor[r.AgentID] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	entry := func(a *ent.Agent) CatalogEntry {
		canStart := actor.Superadmin || a.Visibility == agent.VisibilityListed || editor[a.ID]
		return CatalogEntry{
			ID:          a.ID,
			Slug:        a.Slug,
			Name:        a.Name,
			Description: a.Description,
			Visibility:  a.Visibility.String(),
			Featured:    a.Featured,
			CanStart:    canStart,
		}
	}

	out := &Catalog{
		Featured: make([]CatalogEntry, len(featured)),
		Listed:   make([]CatalogEntry, len(listed)),
	}
	for i, a := range featured {
		out.Featured[i] = entry(a)
	}
	for i, a := range listed {
		out.Listed[i] = entry(a)
	}
	return out, nil
}

// CanStartChat reports whether the actor may start a chat with the agent — the
// start-chat authorization the chats service consults (RMI-308). It is
// Can(CapCreateChat): listed → any allowlisted user; private → owner,
// maintainer, or superadmin.
func (s *Service) CanStartChat(ctx context.Context, actor Actor, agentID uuid.UUID) (bool, error) {
	return s.Can(ctx, actor, agentID, CapCreateChat)
}

// AuthorizeStartChat is the primitive-signature adapter that satisfies the
// chats package's AgentGate interface, so the chats service can gate
// agent-bound chat creation on Can(CapCreateChat) without importing the agents
// package (the composition root wires *agents.Service in as the gate).
func (s *Service) AuthorizeStartChat(ctx context.Context, userID uuid.UUID, superadmin bool, agentID uuid.UUID) (bool, error) {
	return s.CanStartChat(ctx, Actor{UserID: userID, Superadmin: superadmin}, agentID)
}
