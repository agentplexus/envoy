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

// newMessageID returns a time-ordered (UUIDv7) ID so keyset pagination by
// (created_at, id) stays correctly ordered even when two messages land in
// the same clock tick — created_at's storage precision varies by dialect.
func newMessageID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return id
}

// Fields of the Message.
func (Message) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newMessageID),
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
