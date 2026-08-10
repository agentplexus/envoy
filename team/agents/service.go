// Package agents is the virtual-agents service layer (INIT-OMNIAGENT-005): a
// persisted agent = a persona/model bound to an enabled subset of the
// deployment's skills, its per-agent owner/maintainer roles, an authorization
// matrix (Can), and a private/listed + featured registry. An agent is the
// anchor that chats (INIT-003) and agent-scoped secrets (INIT-004) attach to;
// conversing with an agent never grants the right to configure it — that is a
// per-agent role, independent of chat membership.
//
// The service is the primary authorization gate; PostgreSQL row-level security
// (team/store) is the defense-in-depth backstop. On the single-user SQLite
// path there are no policies, so the service-layer checks stand alone.
package agents

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/agent"
	"github.com/plexusone/omniagent/team/ent/agentrole"
	"github.com/plexusone/omniagent/team/store"
)

// Sentinel errors returned by the service layer.
var (
	// ErrForbidden is returned when the actor lacks the required per-agent
	// capability (e.g. a non-editor trying to configure an agent).
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound is returned when the agent does not exist (or is invisible
	// to the actor — indistinguishable by design under RLS).
	ErrNotFound = errors.New("not found")
	// ErrInvalidSlug is returned for slugs outside the allowed form.
	ErrInvalidSlug = errors.New("invalid slug: use 3-32 characters of a-z, 0-9, '-' or '_', starting with a letter or digit")
	// ErrEmptyName is returned when an agent is created without a name.
	ErrEmptyName = errors.New("agent name is empty")
	// ErrSlugTaken is returned when the requested slug already exists.
	ErrSlugTaken = errors.New("agent slug is already taken")
)

// MaxNameBytes / MaxDescriptionBytes / MaxPersonaBytes cap the free-text
// fields so a single agent row stays bounded.
const (
	MaxNameBytes        = 200
	MaxDescriptionBytes = 2000
	MaxPersonaBytes     = 32 << 10
)

// slugPattern constrains an agent slug (stored as citext, so case-insensitive
// on postgres). Same shape as usernames: 3-32 chars, starts alphanumeric.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,31}$`)

// Actor identifies the authenticated caller. Superadmin flows into the store
// so the owner/superadmin administration rules resolve correctly under RLS,
// and into the service's own capability checks.
type Actor struct {
	UserID     uuid.UUID
	Superadmin bool
}

// Config configures the agents service.
type Config struct {
	// Logger defaults to slog.Default().
	Logger *slog.Logger

	// AvailableSkills is the deployment's registered skill catalog (the
	// compiled/MCP/markdown skill names). SetAgentSkills accepts only skills
	// in this catalog. Empty means the deployment registered none, so no
	// skill may be enabled (RMI-302, TRD §5).
	AvailableSkills []string

	// BlockedSkills is the operator deny-list (config agents.blocked_skills):
	// skills removed from the catalog that can never be enabled, bounding the
	// blast radius of user-created agents (e.g. a shell/exec skill).
	BlockedSkills []string
}

// Service exposes agent operations over the team store.
type Service struct {
	store  *store.Store
	logger *slog.Logger

	// allowed maps a lowercased skill name to its canonical catalog name
	// (available minus blocked). blocked is the lowercased deny-list, kept so
	// SetAgentSkills can report a blocked skill distinctly from an unknown one.
	allowed     map[string]string
	allowedList []string // canonical names, sorted, for AvailableSkills
	blocked     map[string]bool
}

// NewService creates the agents service. The available-skills catalog is
// resolved once here (available minus blocked); an empty catalog is valid and
// simply means no skill may be enabled.
func NewService(st *store.Store, cfg Config) (*Service, error) {
	if st == nil {
		return nil, fmt.Errorf("agents: store is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	blocked := make(map[string]bool, len(cfg.BlockedSkills))
	for _, b := range cfg.BlockedSkills {
		if key := strings.ToLower(strings.TrimSpace(b)); key != "" {
			blocked[key] = true
		}
	}
	allowed := make(map[string]string)
	for _, a := range cfg.AvailableSkills {
		canon := strings.TrimSpace(a)
		key := strings.ToLower(canon)
		if key == "" || blocked[key] {
			continue
		}
		allowed[key] = canon
	}
	allowedList := make([]string, 0, len(allowed))
	for _, canon := range allowed {
		allowedList = append(allowedList, canon)
	}
	sort.Strings(allowedList)

	return &Service{
		store:       st,
		logger:      cfg.Logger,
		allowed:     allowed,
		allowedList: allowedList,
		blocked:     blocked,
	}, nil
}

// CreateSpec is the input to CreateAgent. Slug and Name are required; the rest
// are optional descriptive/runtime fields. Visibility starts private and
// featured starts false — both are changed later via the registry methods.
type CreateSpec struct {
	Slug        string
	Name        string
	Description string
	Persona     string
	Model       string
	Provider    string
}

// CreateAgent creates an agent and inserts the creator's owner role in the
// same transaction (the two-step bootstrap the RLS insert policy authorizes
// via team_is_agent_creator). Permissive: any allowlisted user may create an
// agent and becomes its owner. Slugs are globally unique.
func (s *Service) CreateAgent(ctx context.Context, actor Actor, spec CreateSpec) (*ent.Agent, error) {
	spec.Slug = strings.ToLower(strings.TrimSpace(spec.Slug))
	spec.Name = strings.TrimSpace(spec.Name)
	if !slugPattern.MatchString(spec.Slug) {
		return nil, ErrInvalidSlug
	}
	if spec.Name == "" {
		return nil, ErrEmptyName
	}
	if err := validateText(spec.Name, MaxNameBytes, "name"); err != nil {
		return nil, err
	}
	if err := validateText(spec.Description, MaxDescriptionBytes, "description"); err != nil {
		return nil, err
	}
	if err := validateText(spec.Persona, MaxPersonaBytes, "persona"); err != nil {
		return nil, err
	}

	var a *ent.Agent
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		a, err = tx.Agent.Create().
			SetSlug(spec.Slug).
			SetName(spec.Name).
			SetDescription(spec.Description).
			SetPersona(spec.Persona).
			SetModel(spec.Model).
			SetProvider(spec.Provider).
			SetCreatedBy(actor.UserID).
			Save(ctx)
		if err != nil {
			return err
		}
		_, err = tx.AgentRole.Create().
			SetAgentID(a.ID).SetUserID(actor.UserID).SetRole(agentrole.RoleOwner).Save(ctx)
		return err
	})
	if ent.IsConstraintError(err) {
		return nil, ErrSlugTaken
	}
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}
	return a, nil
}

// GetAgent returns an agent the actor may see (editor, listed, or superadmin —
// enforced by RLS on postgres). Returns ErrNotFound when it does not exist or
// is invisible to the actor.
func (s *Service) GetAgent(ctx context.Context, actor Actor, agentID uuid.UUID) (*ent.Agent, error) {
	var a *ent.Agent
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		a, err = tx.Agent.Get(ctx, agentID)
		return err
	})
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ListMyAgents returns the agents the actor owns or maintains, newest first.
func (s *Service) ListMyAgents(ctx context.Context, actor Actor) ([]*ent.Agent, error) {
	var out []*ent.Agent
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		out, err = tx.Agent.Query().
			Where(agent.HasRolesWith(agentrole.UserIDEQ(actor.UserID))).
			Order(ent.Desc(agent.FieldCreatedAt)).All(ctx)
		return err
	})
	return out, err
}

// UpdateSpec patches an agent's descriptive/runtime fields. A nil pointer
// leaves the field unchanged. Visibility and featured are not here — they are
// registry operations (SetVisibility / SetFeatured) with their own authz.
type UpdateSpec struct {
	Name        *string
	Description *string
	Persona     *string
	Model       *string
	Provider    *string
}

// UpdateAgent applies an UpdateSpec. Requires the actor to be an editor
// (owner/maintainer) or superadmin (CapConfigure).
func (s *Service) UpdateAgent(ctx context.Context, actor Actor, agentID uuid.UUID, spec UpdateSpec) (*ent.Agent, error) {
	if err := s.requireEditor(ctx, actor, agentID); err != nil {
		return nil, err
	}
	if spec.Name != nil {
		*spec.Name = strings.TrimSpace(*spec.Name)
		if *spec.Name == "" {
			return nil, ErrEmptyName
		}
		if err := validateText(*spec.Name, MaxNameBytes, "name"); err != nil {
			return nil, err
		}
	}
	if spec.Description != nil {
		if err := validateText(*spec.Description, MaxDescriptionBytes, "description"); err != nil {
			return nil, err
		}
	}
	if spec.Persona != nil {
		if err := validateText(*spec.Persona, MaxPersonaBytes, "persona"); err != nil {
			return nil, err
		}
	}

	var a *ent.Agent
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		upd := tx.Agent.UpdateOneID(agentID)
		if spec.Name != nil {
			upd.SetName(*spec.Name)
		}
		if spec.Description != nil {
			upd.SetDescription(*spec.Description)
		}
		if spec.Persona != nil {
			upd.SetPersona(*spec.Persona)
		}
		if spec.Model != nil {
			upd.SetModel(*spec.Model)
		}
		if spec.Provider != nil {
			upd.SetProvider(*spec.Provider)
		}
		var err error
		a, err = upd.Save(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	return a, nil
}

// DeleteAgent removes an agent (and, by cascade, its skills and roles).
// Requires the actor to be an owner or superadmin.
func (s *Service) DeleteAgent(ctx context.Context, actor Actor, agentID uuid.UUID) error {
	if err := s.requireOwner(ctx, actor, agentID); err != nil {
		return err
	}
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		return tx.Agent.DeleteOneID(agentID).Exec(ctx)
	})
	if ent.IsNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	return nil
}

// ---- Authorization helpers ----------------------------------------------

// lookup reads, via system context so the decision does not depend on RLS
// visibility, the actor's role on an agent (if any) and whether the agent
// exists. It is the shared substrate for requireEditor and requireOwner.
func (s *Service) lookup(ctx context.Context, actorID, agentID uuid.UUID) (role agentrole.Role, hasRole, exists bool, err error) {
	err = s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		ok, e := tx.Agent.Query().Where(agent.IDEQ(agentID)).Exist(ctx)
		if e != nil {
			return e
		}
		if !ok {
			return nil
		}
		exists = true

		r, e := tx.AgentRole.Query().
			Where(agentrole.AgentIDEQ(agentID), agentrole.UserIDEQ(actorID)).Only(ctx)
		if e == nil {
			hasRole = true
			role = r.Role
			return nil
		}
		if !ent.IsNotFound(e) {
			return e
		}
		return nil
	})
	return
}

// requireEditor authorizes a configure-level operation: owner, maintainer, or
// superadmin. ErrNotFound when the agent is absent; ErrForbidden otherwise.
func (s *Service) requireEditor(ctx context.Context, actor Actor, agentID uuid.UUID) error {
	_, hasRole, exists, err := s.lookup(ctx, actor.UserID, agentID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	if actor.Superadmin || hasRole { // owner and maintainer are both editors
		return nil
	}
	return ErrForbidden
}

// requireOwner authorizes an owner-only operation (delete, manage
// maintainers): owner or superadmin.
func (s *Service) requireOwner(ctx context.Context, actor Actor, agentID uuid.UUID) error {
	role, hasRole, exists, err := s.lookup(ctx, actor.UserID, agentID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	if actor.Superadmin || (hasRole && role == agentrole.RoleOwner) {
		return nil
	}
	return ErrForbidden
}

// validateText enforces a byte cap on a free-text field.
func validateText(s string, max int, field string) error {
	if len(s) > max {
		return fmt.Errorf("%s exceeds %d bytes", field, max)
	}
	return nil
}
