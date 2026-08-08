package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// AllowlistEntry is an email the superadmin has approved for login.
// Signup is closed: no allowlist entry (and not the configured superadmin
// email) means no magic link is ever issued.
type AllowlistEntry struct {
	ent.Schema
}

// Annotations of the AllowlistEntry.
func (AllowlistEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "allowlist"},
	}
}

// Fields of the AllowlistEntry.
func (AllowlistEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("email").
			SchemaType(map[string]string{dialect.Postgres: "citext"}).
			Unique().
			NotEmpty(),
		field.UUID("added_by", uuid.UUID{}),
		field.String("note").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the AllowlistEntry.
func (AllowlistEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("added_by_user", User.Type).
			Ref("allowlist_added").
			Field("added_by").
			Unique().
			Required(),
	}
}
