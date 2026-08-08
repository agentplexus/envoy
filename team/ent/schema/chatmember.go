package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ChatMember is a user's membership in a chat. Uniqueness of
// (chat_id, user_id) enforces one membership row per user per chat.
type ChatMember struct {
	ent.Schema
}

// Fields of the ChatMember.
func (ChatMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("chat_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Enum("role").Values("owner", "member").Default("member"),
		field.Time("joined_at").Default(time.Now).Immutable(),
	}
}

// Edges of the ChatMember.
func (ChatMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("chat", Chat.Type).
			Ref("members").
			Field("chat_id").
			Unique().
			Required(),
		edge.From("user", User.Type).
			Ref("memberships").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes of the ChatMember.
func (ChatMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("chat_id", "user_id").Unique(),
		index.Fields("user_id"),
	}
}
