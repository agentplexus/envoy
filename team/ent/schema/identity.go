package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Identity links a login provider identity to a user. A user may hold
// several (magic link plus SSO providers).
type Identity struct {
	ent.Schema
}

// Fields of the Identity.
func (Identity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("user_id", uuid.UUID{}),
		field.Enum("provider").Values("magic_link", "google", "github"),
		// ProviderSubject is the provider's stable subject identifier
		// (email for magic_link, sub for OIDC, account id for GitHub).
		field.String("provider_subject").NotEmpty(),
		field.String("verified_email").NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the Identity.
func (Identity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("identities").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes of the Identity.
func (Identity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider", "provider_subject").Unique(),
	}
}
