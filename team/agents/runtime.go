package agents

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/agentskill"
)

// RuntimeConfig is an agent's resolved runtime configuration — the persona,
// model/provider, and enabled-skill subset a per-agent runtime instance
// (RMI-OMNIAGENT-309) is built from. It carries no secrets; secret binding is
// RMI-OMNIAGENT-310.
type RuntimeConfig struct {
	ID       uuid.UUID
	Slug     string
	Name     string
	Persona  string
	Model    string
	Provider string
	Skills   []string
}

// LoadRuntimeConfig loads an agent's runtime configuration by ID in system
// context. The per-agent runtime is a system principal, not a user, so this
// deliberately bypasses RLS visibility (a private agent still runs its own
// chats' turns). Returns ErrNotFound when the agent does not exist.
func (s *Service) LoadRuntimeConfig(ctx context.Context, agentID uuid.UUID) (RuntimeConfig, error) {
	var rc RuntimeConfig
	err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		a, err := tx.Agent.Get(ctx, agentID)
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		rows, err := tx.AgentSkill.Query().Where(agentskill.AgentIDEQ(agentID)).All(ctx)
		if err != nil {
			return err
		}
		skills := make([]string, len(rows))
		for i, r := range rows {
			skills[i] = r.Skill
		}
		sort.Strings(skills)
		rc = RuntimeConfig{
			ID:       a.ID,
			Slug:     a.Slug,
			Name:     a.Name,
			Persona:  a.Persona,
			Model:    a.Model,
			Provider: a.Provider,
			Skills:   skills,
		}
		return nil
	})
	if err != nil {
		return RuntimeConfig{}, err
	}
	return rc, nil
}

// AgentSlugByID returns just an agent's slug, in system context — the cheap read
// a group turn makes for @-mention matching, without loading the full runtime
// configuration. Returns ErrNotFound when the agent does not exist.
func (s *Service) AgentSlugByID(ctx context.Context, agentID uuid.UUID) (string, error) {
	var slug string
	err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		a, err := tx.Agent.Get(ctx, agentID)
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		slug = a.Slug
		return nil
	})
	if err != nil {
		return "", err
	}
	return slug, nil
}
