package agent

import (
	"context"
	"testing"

	agentctx "github.com/plexusone/omniagent/context"
	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omnistorage-core/kvs/backend/memory"
)

func TestAgent_SessionStore(t *testing.T) {
	backend := memory.New()
	defer backend.Close()

	sessionStore := sessions.NewStore(sessions.StoreConfig{
		Backend: backend,
	})

	// Test that WithSessionStore sets the session store
	a := &Agent{}
	opt := WithSessionStore(sessionStore)
	if err := opt(a); err != nil {
		t.Fatalf("WithSessionStore() error = %v", err)
	}

	if a.sessions != sessionStore {
		t.Error("WithSessionStore() did not set session store")
	}

	if a.SessionStore() != sessionStore {
		t.Error("SessionStore() did not return correct store")
	}
}

func TestAgent_WithSessionsFromStorage(t *testing.T) {
	backend := memory.New()
	defer backend.Close()

	a := &Agent{}
	opt := WithSessionsFromStorage(backend)
	if err := opt(a); err != nil {
		t.Fatalf("WithSessionsFromStorage() error = %v", err)
	}

	if a.sessions == nil {
		t.Error("WithSessionsFromStorage() did not create session store")
	}

	if a.storage != backend {
		t.Error("WithSessionsFromStorage() did not set storage backend")
	}
}

func TestAgent_SessionMethods_NoStore(t *testing.T) {
	ctx := context.Background()
	a := &Agent{}

	// All session methods should return error when store is not configured
	_, err := a.GetSession(ctx, "test")
	if err == nil {
		t.Error("GetSession() should return error when store not configured")
	}

	_, err = a.ListSessions(ctx)
	if err == nil {
		t.Error("ListSessions() should return error when store not configured")
	}

	err = a.DeleteSession(ctx, "test")
	if err == nil {
		t.Error("DeleteSession() should return error when store not configured")
	}

	err = a.ClearSession(ctx, "test")
	if err == nil {
		t.Error("ClearSession() should return error when store not configured")
	}
}

func TestAgent_SessionMethods_WithStore(t *testing.T) {
	ctx := context.Background()
	backend := memory.New()
	defer backend.Close()

	sessionStore := sessions.NewStore(sessions.StoreConfig{
		Backend: backend,
	})

	a := &Agent{
		sessions: sessionStore,
	}

	// Create a session via the store
	session, err := sessionStore.Get(ctx, "test-session")
	if err != nil {
		t.Fatalf("sessions.Get() error = %v", err)
	}
	session.AddMessage("user", "Hello")
	if err := sessionStore.Save(ctx, session); err != nil {
		t.Fatalf("sessions.Save() error = %v", err)
	}

	// Test GetSession
	got, err := a.GetSession(ctx, "test-session")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got.ID != "test-session" {
		t.Errorf("GetSession() ID = %q, want %q", got.ID, "test-session")
	}

	// Test ListSessions
	ids, err := a.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != "test-session" {
		t.Errorf("ListSessions() = %v, want [test-session]", ids)
	}

	// Test ClearSession
	if err := a.ClearSession(ctx, "test-session"); err != nil {
		t.Fatalf("ClearSession() error = %v", err)
	}

	cleared, err := a.GetSession(ctx, "test-session")
	if err != nil {
		t.Fatalf("GetSession() after clear error = %v", err)
	}
	if len(cleared.GetMessages()) != 0 {
		t.Errorf("ClearSession() did not clear messages, got %d", len(cleared.GetMessages()))
	}

	// Test DeleteSession
	if err := a.DeleteSession(ctx, "test-session"); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	_, err = a.GetSession(ctx, "test-session")
	if err == nil {
		t.Error("GetSession() should return error after delete")
	}
}

func TestAgent_WithContextEngine(t *testing.T) {
	engine := agentctx.New(agentctx.Config{
		MaxMessages: 50,
		MaxTokens:   8000,
	})

	a := &Agent{}
	opt := WithContextEngine(engine)
	if err := opt(a); err != nil {
		t.Fatalf("WithContextEngine() error = %v", err)
	}

	if a.contextEngine != engine {
		t.Error("WithContextEngine() did not set context engine")
	}

	if a.ContextEngine() != engine {
		t.Error("ContextEngine() did not return correct engine")
	}
}

func TestAgent_WithContextConfig(t *testing.T) {
	a := &Agent{}
	opt := WithContextConfig(agentctx.Config{
		MaxMessages: 100,
	})
	if err := opt(a); err != nil {
		t.Fatalf("WithContextConfig() error = %v", err)
	}

	if a.contextEngine == nil {
		t.Error("WithContextConfig() did not create context engine")
	}
}

func TestAgent_WithMaxMessages(t *testing.T) {
	a := &Agent{}
	opt := WithMaxMessages(25)
	if err := opt(a); err != nil {
		t.Fatalf("WithMaxMessages() error = %v", err)
	}

	if a.contextEngine == nil {
		t.Error("WithMaxMessages() did not create context engine")
	}
}

func TestAgent_SetContextEngine(t *testing.T) {
	a := &Agent{}

	engine := agentctx.New(agentctx.DefaultConfig())
	a.SetContextEngine(engine)

	if a.contextEngine != engine {
		t.Error("SetContextEngine() did not set engine")
	}
}
