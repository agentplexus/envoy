package cron

import (
	"context"
	"testing"
)

func TestPrincipalContext_RoundTrip(t *testing.T) {
	ctx := context.Background()

	if got := PrincipalFromContext(ctx); got != "" {
		t.Errorf("empty context principal = %q, want empty", got)
	}

	ctx = ContextWithPrincipal(ctx, "session:abc")
	if got := PrincipalFromContext(ctx); got != "session:abc" {
		t.Errorf("principal = %q, want session:abc", got)
	}

	// Empty principal is a no-op.
	if got := PrincipalFromContext(ContextWithPrincipal(context.Background(), "")); got != "" {
		t.Errorf("empty principal stored as %q", got)
	}
}

func TestSessionPrincipal(t *testing.T) {
	if got := SessionPrincipal("abc"); got != "session:abc" {
		t.Errorf("SessionPrincipal = %q", got)
	}
	if got := SessionPrincipal(""); got != "" {
		t.Errorf("SessionPrincipal(\"\") = %q, want empty", got)
	}

	id, ok := SessionIDFromPrincipal("session:abc")
	if !ok || id != "abc" {
		t.Errorf("SessionIDFromPrincipal = %q, %v", id, ok)
	}
	if _, ok := SessionIDFromPrincipal("account:abc"); ok {
		t.Error("non-session principal must not parse")
	}
	if _, ok := SessionIDFromPrincipal("session:"); ok {
		t.Error("empty session id must not parse")
	}
	if _, ok := SessionIDFromPrincipal(""); ok {
		t.Error("empty principal must not parse")
	}
}

func TestHandleCreate_StampsOwnerPrincipal(t *testing.T) {
	skill := NewSkill()
	skill.SetStorage(newListableMockStore())
	if err := skill.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer skill.Close()

	params := map[string]any{
		"name":          "stamped job",
		"schedule_cron": "0 0 9 * * *",
		"action_type":   "call_tool",
		"tool_name":     "web_search",
	}

	// Creation under a session principal stamps the job.
	ctx := ContextWithPrincipal(context.Background(), SessionPrincipal("sess-1"))
	result, err := skill.handleCreate(ctx, params)
	if err != nil {
		t.Fatalf("handleCreate: %v", err)
	}
	id := result.(map[string]any)["id"].(string)

	job, err := skill.GetScheduler().GetJob(context.Background(), id)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.OwnerPrincipal != "session:sess-1" {
		t.Errorf("OwnerPrincipal = %q, want session:sess-1", job.OwnerPrincipal)
	}

	// Creation without a principal leaves the job unstamped (legacy).
	params["name"] = "unstamped job"
	result, err = skill.handleCreate(context.Background(), params)
	if err != nil {
		t.Fatalf("handleCreate: %v", err)
	}
	id = result.(map[string]any)["id"].(string)
	job, err = skill.GetScheduler().GetJob(context.Background(), id)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.OwnerPrincipal != "" {
		t.Errorf("OwnerPrincipal = %q, want empty", job.OwnerPrincipal)
	}

	// A caller-supplied owner_principal param must NOT be honored.
	params["name"] = "spoof attempt"
	params["owner_principal"] = "session:admin"
	result, err = skill.handleCreate(context.Background(), params)
	if err != nil {
		t.Fatalf("handleCreate: %v", err)
	}
	id = result.(map[string]any)["id"].(string)
	job, err = skill.GetScheduler().GetJob(context.Background(), id)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.OwnerPrincipal != "" {
		t.Errorf("caller-supplied principal was honored: %q", job.OwnerPrincipal)
	}
}
