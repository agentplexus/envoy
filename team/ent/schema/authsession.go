package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// AuthSession is a server-side login session backing the browser cookie.
// Only the SHA-256 hash of the cookie token is stored.
type AuthSession struct {
	ent.Schema
}

// Fields of the AuthSession.
func (AuthSession) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("user_id", uuid.UUID{}),
		field.String("token_hash").Unique().NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("last_seen_at").Default(time.Now),
		field.Time("expires_at"),
		field.String("user_agent").Optional(),
		field.String("created_ip").Optional(),
	}
}

// Edges of the AuthSession.
func (AuthSession) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("auth_sessions").
			Field("user_id").
			Unique().
			Required(),
	}
}
