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
	}
}

// Indexes of the Chat.
func (Chat) Indexes() []ent.Index {
	return []ent.Index{
		// One private chat per user.
		index.Fields("created_by").
			Annotations(entsql.IndexWhere("type = 'private'")).
			Unique(),
	}
}
