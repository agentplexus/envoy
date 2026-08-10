package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Chat is a conversation: each member's private chat with the agent, or a
// group chat with multiple members where the agent replies on @-mention.
type Chat struct {
	ent.Schema
}

// Fields of the Chat.
func (Chat) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.Enum("type").Values("private", "group"),
		field.String("name").Optional(),
		field.UUID("created_by", uuid.UUID{}),
		// AgentID binds the chat to the agent it converses with
		// (INIT-OMNIAGENT-005). Optional/nillable: personal-mode DMs and
		// pre-agent chats carry no agent and use the deployment default.
		field.UUID("agent_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the Chat.
func (Chat) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("creator", User.Type).
			Ref("chats_created").
			Field("created_by").
			Unique().
			Required(),
		edge.To("members", ChatMember.Type),
		edge.To("messages", Message.Type),
		// The agent this chat converses with (INIT-OMNIAGENT-005). Optional:
		// personal-mode DMs carry no agent binding.
		edge.To("agent", Agent.Type).
			Field("agent_id").
			Unique(),
	}
}

// Indexes of the Chat.
func (Chat) Indexes() []ent.Index {
	return []ent.Index{
		// One private chat per user per agent. Personal-mode DMs (agent_id
		// NULL) are deduped at the service layer instead — Postgres/SQLite
		// treat NULLs as distinct, so the index does not constrain them.
		index.Fields("created_by", "agent_id").
			Annotations(entsql.IndexWhere("type = 'private'")).
			Unique(),
	}
}
