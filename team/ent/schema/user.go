// Package schema defines the Ent schemas for the team (multi-user) system
// of record. PostgreSQL-only: citext columns and row-level security policies
// (applied by team/store migrations) assume Postgres.
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// User holds team members and the superadmin.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("email").
			SchemaType(map[string]string{dialect.Postgres: "citext"}).
			Unique().
			NotEmpty(),
		field.String("username").
			SchemaType(map[string]string{dialect.Postgres: "citext"}).
			Unique().
			NotEmpty(),
		field.String("display_name").Optional(),
		// password_hash is the argon2id PHC-encoded credential for optional
		// email+password login (additive to magic-link/SSO). Nullable: unset
		// means the account has no password. Sensitive so ent omits it from
		// String()/logging.
		field.String("password_hash").Optional().Sensitive(),
		field.Enum("role").Values("superadmin", "member").Default("member"),
		field.Enum("status").Values("active", "disabled").Default("active"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("identities", Identity.Type),
		edge.To("auth_sessions", AuthSession.Type),
		edge.To("allowlist_added", AllowlistEntry.Type),
		edge.To("chats_created", Chat.Type),
		edge.To("memberships", ChatMember.Type),
		edge.To("messages", Message.Type),
		edge.To("agents_created", Agent.Type),
		edge.To("agent_roles", AgentRole.Type),
	}
}
