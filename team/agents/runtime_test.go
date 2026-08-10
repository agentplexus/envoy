package agents

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
)

func TestLoadRuntimeConfig(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{
		Slug:     "helper",
		Name:     "Helper Bot",
		Persona:  "You are a concise helper.",
		Model:    "gpt-4o-mini",
		Provider: "openai",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Seed enabled skills directly (bypasses the catalog, which the fixture
	// leaves empty) so the loader's skill read is exercised.
	if err := f.st.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		for _, sk := range []string{"weather", "github"} {
			if _, err := tx.AgentSkill.Create().SetAgentID(a.ID).SetSkill(sk).Save(ctx); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed skills: %v", err)
	}

	rc, err := f.svc.LoadRuntimeConfig(ctx, a.ID)
	if err != nil {
		t.Fatalf("LoadRuntimeConfig: %v", err)
	}
	if rc.ID != a.ID || rc.Slug != "helper" || rc.Name != "Helper Bot" {
		t.Errorf("identity = %+v, want id=%v slug=helper name=Helper Bot", rc, a.ID)
	}
	if rc.Persona != "You are a concise helper." || rc.Model != "gpt-4o-mini" || rc.Provider != "openai" {
		t.Errorf("config = %+v, want persona/model/provider set", rc)
	}
	// Skills are returned sorted.
	if want := []string{"github", "weather"}; !reflect.DeepEqual(rc.Skills, want) {
		t.Errorf("skills = %v, want %v (sorted)", rc.Skills, want)
	}
}

func TestLoadRuntimeConfig_NotFound(t *testing.T) {
	f := setup(t)
	if _, err := f.svc.LoadRuntimeConfig(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadRuntimeConfig on missing agent = %v, want ErrNotFound", err)
	}
}

// TestLoadRuntimeConfig_SystemContext confirms the loader reads in system
// context: a private agent (not listed) still loads, since the runtime is a
// system principal, not a user subject to visibility.
func TestLoadRuntimeConfig_SystemContext(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// Bob creates a private agent; alice cannot see it, but the runtime can.
	a, err := f.svc.CreateAgent(ctx, f.actor(f.bob), CreateSpec{Slug: "bobs-bot", Name: "Bob's Bot"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	rc, err := f.svc.LoadRuntimeConfig(ctx, a.ID)
	if err != nil {
		t.Fatalf("LoadRuntimeConfig (private agent): %v", err)
	}
	if rc.Slug != "bobs-bot" {
		t.Errorf("slug = %q, want bobs-bot", rc.Slug)
	}
}

func TestAgentSlugByID(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "sluggo", Name: "Sluggo"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	slug, err := f.svc.AgentSlugByID(ctx, a.ID)
	if err != nil {
		t.Fatalf("AgentSlugByID: %v", err)
	}
	if slug != "sluggo" {
		t.Errorf("slug = %q, want sluggo", slug)
	}

	if _, err := f.svc.AgentSlugByID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AgentSlugByID on missing agent = %v, want ErrNotFound", err)
	}
}
