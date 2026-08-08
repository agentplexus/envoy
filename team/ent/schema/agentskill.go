package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// AgentSkill is one skill enabled on an agent. The enabled set must be a
// subset of the deployment's available-skills catalog (TRD section 5); this
// table only records the membership.
type AgentSkill struct {
	ent.Schema
}

// Fields of the AgentSkill.
func (AgentSkill) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("agent_id", uuid.UUID{}),
		field.String("skill").
			SchemaType(map[string]string{dialect.Postgres: "citext"}).
			NotEmpty(),
	}
}

// Edges of the AgentSkill.
func (AgentSkill) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("agent", Agent.Type).
			Ref("skills").
			Field("agent_id").
			Unique().
			Required(),
	}
}

// Indexes of the AgentSkill.
func (AgentSkill) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent_id", "skill").Unique(),
	}
}
