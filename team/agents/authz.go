package agents

import (
	"context"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent/agent"
	"github.com/plexusone/omniagent/team/ent/agentrole"
)

// Capability enumerates the per-agent authorization checks (INIT-005 TRD §3).
// Can(actor, agent, capability) is the single service-layer gate the HTTP and
// chat layers consult; RLS is the defense-in-depth backstop.
type Capability int

const (
	// CapChat is the right to converse with the agent (in a chat the actor
	// belongs to). Governed here at the agent level; per-chat membership is
	// enforced separately by the chats service.
	CapChat Capability = iota
	// CapCreateChat is the right to start a new chat with the agent.
	CapCreateChat
	// CapConfigure is the right to change the agent's skills, persona, model,
	// and secrets.
	CapConfigure
	// CapManageMaintainers is the right to add/remove maintainers (owner only).
	CapManageMaintainers
	// CapManageRegistry is the right to change the agent's visibility
	// (private/listed). Featured is superadmin-only curation, checked
	// separately in SetFeatured — not covered by this capability.
	CapManageRegistry
	// CapAdminister is superadmin deployment administration.
	CapAdminister
)

// String renders a Capability for logging.
func (c Capability) String() string {
	switch c {
	case CapChat:
		return "chat"
	case CapCreateChat:
		return "create_chat"
	case CapConfigure:
		return "configure"
	case CapManageMaintainers:
		return "manage_maintainers"
	case CapManageRegistry:
		return "manage_registry"
	case CapAdminister:
		return "administer"
	default:
		return "unknown"
	}
}

// Can reports whether the actor holds a capability on an agent, per the TRD §3
// matrix:
//
//   - owner       → all of Configure, ManageMaintainers, ManageRegistry,
//     CreateChat, Chat.
//   - maintainer  → Configure, ManageRegistry, CreateChat, Chat — not
//     ManageMaintainers.
//   - superadmin  → Administer, plus everything except an agent's secret
//     values (secret reads are an INIT-004 concern, not modeled here).
//   - any user    → CreateChat/Chat iff the agent's visibility is "listed"
//     (private agents are startable only by editors/superadmin; an invitee
//     converses via chat membership, enforced by the chats service).
//
// Can returns false (not an error) for a non-existent or invisible agent, so
// callers may treat "cannot" and "not found" uniformly where appropriate.
// The underlying facts are read via system context so the decision itself is
// not filtered by RLS.
func (s *Service) Can(ctx context.Context, actor Actor, agentID uuid.UUID, capability Capability) (bool, error) {
	vis, role, hasRole, exists, err := s.lookup(ctx, actor.UserID, agentID)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	isOwner := hasRole && role == agentrole.RoleOwner
	isEditor := hasRole // owner or maintainer

	switch capability {
	case CapAdminister:
		return actor.Superadmin, nil
	case CapManageMaintainers:
		return isOwner || actor.Superadmin, nil
	case CapConfigure, CapManageRegistry:
		return isEditor || actor.Superadmin, nil
	case CapCreateChat, CapChat:
		return isEditor || actor.Superadmin || vis == agent.VisibilityListed, nil
	default:
		return false, nil
	}
}
