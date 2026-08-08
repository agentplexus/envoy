package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// MagicLinkToken is a single-use, expiring login token. Only the SHA-256
// hash is stored; the raw token exists solely in the emailed link. Row-level
// security restricts this table to the system (auth-layer) context.
type MagicLinkToken struct {
	ent.Schema
}

// Fields of the MagicLinkToken.
func (MagicLinkToken) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("email").
			SchemaType(map[string]string{dialect.Postgres: "citext"}).
			NotEmpty(),
		field.String("token_hash").Unique().NotEmpty(),
		field.Time("expires_at"),
		field.Time("consumed_at").Optional().Nillable(),
		field.String("created_ip").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}
