package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Message is one message in a chat, authored by a member or by the agent.
// Messages are immutable at v1: no update or delete policies exist.
type Message struct {
	ent.Schema
}

// Fields of the Message.
func (Message) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("chat_id", uuid.UUID{}),
		field.Enum("author_type").Values("user", "agent"),
		field.UUID("author_user_id", uuid.UUID{}).Optional().Nillable(),
		field.Text("content").NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the Message.
func (Message) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("chat", Chat.Type).
			Ref("messages").
			Field("chat_id").
			Unique().
			Required(),
		edge.From("author", User.Type).
			Ref("messages").
			Field("author_user_id").
			Unique(),
	}
}

// Indexes of the Message.
func (Message) Indexes() []ent.Index {
	return []ent.Index{
		// Keyset pagination: (chat_id, created_at, id).
		index.Fields("chat_id", "created_at"),
	}
}
