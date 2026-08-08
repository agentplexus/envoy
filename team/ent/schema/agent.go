package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Agent is a first-class, named configuration: a persona/model bound to a
// subset of the deployment's available skills and its own agent-scoped
// secrets (INIT-OMNIAGENT-004). Owners and maintainers (AgentRole) configure
// it; conversing with it (via Chat) never grants configuration rights.
type Agent struct {
	ent.Schema
}

// Fields of the Agent.
func (Agent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("slug").
			SchemaType(map[string]string{dialect.Postgres: "citext"}).
			Unique().
			NotEmpty(),
		field.String("name").NotEmpty(),
		field.String("description").Optional(),
		field.Text("persona").Optional(),
		field.String("model").Optional(),
		field.String("provider").Optional(),
		// Visibility is the owner/maintainer-controlled discoverability
		// switch (TRD section 4): "listed" agents are startable by any
		// allowlisted user, "private" only by editors and invitees.
		field.Enum("visibility").Values("private", "listed").Default("private"),
		// Featured is superadmin-only curation, independent of visibility
		// (TRD open question 1: maintainers do not control this).
		field.Bool("featured").Default(false),
		field.UUID("created_by", uuid.UUID{}),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Agent.
func (Agent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("creator", User.Type).
			Ref("agents_created").
			Field("created_by").
			Unique().
			Required(),
		edge.To("skills", AgentSkill.Type),
		edge.To("roles", AgentRole.Type),
	}
}
