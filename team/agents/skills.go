package agents

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/agent"
	"github.com/plexusone/omniagent/team/ent/agentskill"
)

// Skill-related sentinel errors (RMI-302).
var (
	// ErrUnknownSkill is returned when an enabled skill is not in the
	// deployment's available-skills catalog.
	ErrUnknownSkill = errors.New("unknown skill: not in the deployment's available-skills catalog")
	// ErrBlockedSkill is returned when an enabled skill is on the operator
	// deny-list (agents.blocked_skills) and can never be enabled.
	ErrBlockedSkill = errors.New("blocked skill: withheld by the operator deny-list")
)

// AvailableSkills returns the deployment's available-skills catalog (registered
// skills minus the operator deny-list), sorted by canonical name. This is the
// set an agent's enabled skills may be drawn from.
func (s *Service) AvailableSkills() []string {
	out := make([]string, len(s.allowedList))
	copy(out, s.allowedList)
	return out
}

// SetAgentSkills replaces an agent's enabled-skill set. Every skill must be in
// the available-skills catalog (AvailableSkills); an unknown skill yields
// ErrUnknownSkill and a deny-listed one ErrBlockedSkill, so nothing is
// persisted unless the whole set validates. Requires the actor to be an editor
// (CapConfigure). Skills are matched case-insensitively (citext) and stored in
// their canonical catalog casing; duplicates are collapsed.
func (s *Service) SetAgentSkills(ctx context.Context, actor Actor, agentID uuid.UUID, skills []string) error {
	if err := s.requireEditor(ctx, actor, agentID); err != nil {
		return err
	}

	canon := make([]string, 0, len(skills))
	seen := make(map[string]bool, len(skills))
	for _, sk := range skills {
		key := strings.ToLower(strings.TrimSpace(sk))
		if key == "" {
			continue
		}
		c, ok := s.allowed[key]
		if !ok {
			if s.blocked[key] {
				return fmt.Errorf("%q: %w", strings.TrimSpace(sk), ErrBlockedSkill)
			}
			return fmt.Errorf("%q: %w", strings.TrimSpace(sk), ErrUnknownSkill)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		canon = append(canon, c)
	}

	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		if _, err := tx.AgentSkill.Delete().Where(agentskill.AgentIDEQ(agentID)).Exec(ctx); err != nil {
			return err
		}
		for _, c := range canon {
			if _, err := tx.AgentSkill.Create().SetAgentID(agentID).SetSkill(c).Save(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("set agent skills: %w", err)
	}
	return nil
}

// AgentSkills returns an agent's enabled skills, sorted. Readable by anyone who
// can see the agent (editor, listed, or superadmin — enforced by RLS); returns
// ErrNotFound when the agent is absent or invisible to the actor.
func (s *Service) AgentSkills(ctx context.Context, actor Actor, agentID uuid.UUID) ([]string, error) {
	var skills []string
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		visible, err := tx.Agent.Query().Where(agent.IDEQ(agentID)).Exist(ctx)
		if err != nil {
			return err
		}
		if !visible {
			return ErrNotFound
		}
		rows, err := tx.AgentSkill.Query().Where(agentskill.AgentIDEQ(agentID)).All(ctx)
		if err != nil {
			return err
		}
		skills = make([]string, len(rows))
		for i, r := range rows {
			skills[i] = r.Skill
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(skills)
	return skills, nil
}
