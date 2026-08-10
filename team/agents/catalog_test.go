package agents

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent/agent"
)

func TestCatalog_Sections(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// Alice creates three agents: a listed one, a featured one, and a private one.
	listed, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "listed-a", Name: "Listed A"})
	if err != nil {
		t.Fatalf("create listed: %v", err)
	}
	if _, err := f.svc.SetVisibility(ctx, f.actor(f.alice), listed.ID, "listed"); err != nil {
		t.Fatalf("set listed: %v", err)
	}

	feat, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "feat-a", Name: "Feat A"})
	if err != nil {
		t.Fatalf("create feat: %v", err)
	}
	if _, err := f.svc.SetVisibility(ctx, f.actor(f.alice), feat.ID, "listed"); err != nil {
		t.Fatalf("set feat listed: %v", err)
	}
	if _, err := f.svc.SetFeatured(ctx, f.superAdminActor(), feat.ID, true); err != nil {
		t.Fatalf("feature: %v", err)
	}

	if _, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "priv-a", Name: "Priv A"}); err != nil {
		t.Fatalf("create private: %v", err)
	}

	cat, err := f.svc.Catalog(ctx, f.actor(f.alice))
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(cat.Featured) != 1 || cat.Featured[0].ID != feat.ID {
		t.Errorf("Featured = %+v, want [feat]", cat.Featured)
	}
	if len(cat.Listed) != 1 || cat.Listed[0].ID != listed.ID {
		t.Errorf("Listed = %+v, want [listed] (featured excluded)", cat.Listed)
	}
	// Alice is the owner, so she may start either.
	if !cat.Featured[0].CanStart || !cat.Listed[0].CanStart {
		t.Error("owner CanStart should be true for both")
	}
}

func TestCanStartChat(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "start-bot", Name: "Start"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	stranger := Actor{UserID: uuid.New()}

	// Private: only editors/superadmin may start.
	ok, err := f.svc.CanStartChat(ctx, stranger, a.ID)
	if err != nil {
		t.Fatalf("CanStartChat: %v", err)
	}
	if ok {
		t.Error("stranger CanStartChat on private = true, want false")
	}
	if ok, _ := f.svc.CanStartChat(ctx, f.actor(f.alice), a.ID); !ok {
		t.Error("owner CanStartChat = false, want true")
	}

	// Listed: any user may start.
	f.setVisibility(t, a.ID, agent.VisibilityListed)
	if ok, _ := f.svc.CanStartChat(ctx, stranger, a.ID); !ok {
		t.Error("stranger CanStartChat on listed = false, want true")
	}
}
