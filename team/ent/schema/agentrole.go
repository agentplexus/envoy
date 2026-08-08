package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AgentRole is a user's per-agent role: owner (creator or assignee, full
// control incl. maintainers) or maintainer (added by an owner; everything
// except managing other maintainers). Per-agent roles are independent of
// chat membership — being in a group chat with an agent never implies a row
// here (PRD: "conversing with an agent never confers the right to configure
// it").
type AgentRole struct {
	ent.Schema
}

// Fields of the AgentRole.
func (AgentRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("agent_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Enum("role").Values("owner", "maintainer"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the AgentRole.
func (AgentRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("agent", Agent.Type).
			Ref("roles").
			Field("agent_id").
			Unique().
			Required(),
		edge.From("user", User.Type).
			Ref("agent_roles").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes of the AgentRole.
func (AgentRole) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent_id", "user_id").Unique(),
		index.Fields("user_id"),
	}
}
