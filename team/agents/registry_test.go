package agents

import (
	"context"
	"errors"
	"testing"
)

func TestSetVisibility(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "vis-bot", Name: "Vis"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if a.Visibility.String() != "private" {
		t.Fatalf("default visibility = %q, want private", a.Visibility)
	}

	// Invalid value rejected.
	if _, err := f.svc.SetVisibility(ctx, f.actor(f.alice), a.ID, "public"); !errors.Is(err, ErrInvalidVisibility) {
		t.Errorf("err = %v, want ErrInvalidVisibility", err)
	}

	// Non-editor forbidden.
	if _, err := f.svc.SetVisibility(ctx, f.actor(f.bob), a.ID, "listed"); !errors.Is(err, ErrForbidden) {
		t.Errorf("bob set visibility err = %v, want ErrForbidden", err)
	}

	// Owner sets listed.
	got, err := f.svc.SetVisibility(ctx, f.actor(f.alice), a.ID, "listed")
	if err != nil {
		t.Fatalf("owner set visibility: %v", err)
	}
	if got.Visibility.String() != "listed" {
		t.Errorf("visibility = %q, want listed", got.Visibility)
	}

	// Maintainer may also set visibility.
	if _, err := f.svc.AddMaintainer(ctx, f.actor(f.alice), a.ID, "bob"); err != nil {
		t.Fatalf("AddMaintainer: %v", err)
	}
	if _, err := f.svc.SetVisibility(ctx, f.actor(f.bob), a.ID, "private"); err != nil {
		t.Errorf("maintainer set visibility: %v", err)
	}
}

func TestSetFeatured_SuperadminOnly(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a, err := f.svc.CreateAgent(ctx, f.actor(f.alice), CreateSpec{Slug: "feat-bot", Name: "Feat"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Owner cannot feature their own agent.
	if _, err := f.svc.SetFeatured(ctx, f.actor(f.alice), a.ID, true); !errors.Is(err, ErrForbidden) {
		t.Errorf("owner SetFeatured err = %v, want ErrForbidden", err)
	}

	// Superadmin can.
	got, err := f.svc.SetFeatured(ctx, f.superAdminActor(), a.ID, true)
	if err != nil {
		t.Fatalf("superadmin SetFeatured: %v", err)
	}
	if !got.Featured {
		t.Error("featured not set")
	}
	if _, err := f.svc.SetFeatured(ctx, f.superAdminActor(), a.ID, false); err != nil {
		t.Fatalf("unset featured: %v", err)
	}
}
