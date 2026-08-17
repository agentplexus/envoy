package gateway

import (
	"context"
	"testing"

	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omnistorage-core/kvs/backend/memory"
)

// sessionAwareAgent implements SessionAwareProcessor in addition to
// AgentProcessor, recording which method handleChat actually dispatched to.
type sessionAwareAgent struct {
	store             *sessions.Store
	lastSessionID     string
	processCalled     bool
	withSessionCalled bool
}

func (a *sessionAwareAgent) Process(_ context.Context, sessionID, _ string) (string, error) {
	a.processCalled = true
	a.lastSessionID = sessionID
	return "stateless", nil
}

func (a *sessionAwareAgent) ProcessWithSession(_ context.Context, sessionID, _ string) (string, error) {
	a.withSessionCalled = true
	a.lastSessionID = sessionID
	return "stateful", nil
}

func (a *sessionAwareAgent) SessionStore() *sessions.Store {
	return a.store
}

func TestHandleChat_UsesProcessWithSessionWhenStoreConfigured(t *testing.T) {
	store := sessions.NewStore(sessions.StoreConfig{Backend: memory.New()})
	agent := &sessionAwareAgent{store: store}

	gw, err := New(Config{Agent: agent})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := NewDefaultMessageHandler(gw)
	client := newClient(nil, gw)

	resp, err := handler.handleChat(context.Background(), client, &Message{
		ID:      "m1",
		Type:    MessageTypeChat,
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("handleChat: %v", err)
	}
	if !agent.withSessionCalled || agent.processCalled {
		t.Fatalf("expected ProcessWithSession only, got processCalled=%v withSessionCalled=%v",
			agent.processCalled, agent.withSessionCalled)
	}
	if agent.lastSessionID != client.ID {
		t.Errorf("session ID = %q, want client ID %q", agent.lastSessionID, client.ID)
	}
	if resp.Content != "stateful" {
		t.Errorf("response content = %q, want %q", resp.Content, "stateful")
	}
}

func TestHandleChat_FallsBackToProcessWhenNoStoreConfigured(t *testing.T) {
	// SessionAwareProcessor implemented, but SessionStore() returns nil —
	// e.g. WithCronScheduler without WithSessionsFromStorage.
	agent := &sessionAwareAgent{store: nil}

	gw, err := New(Config{Agent: agent})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := NewDefaultMessageHandler(gw)
	client := newClient(nil, gw)

	resp, err := handler.handleChat(context.Background(), client, &Message{
		ID:      "m1",
		Type:    MessageTypeChat,
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("handleChat: %v", err)
	}
	if !agent.processCalled || agent.withSessionCalled {
		t.Fatalf("expected Process only, got processCalled=%v withSessionCalled=%v",
			agent.processCalled, agent.withSessionCalled)
	}
	if resp.Content != "stateless" {
		t.Errorf("response content = %q, want %q", resp.Content, "stateless")
	}
}

func TestHandleChat_FallsBackToProcessWhenAgentNotSessionAware(t *testing.T) {
	// mockAgent (gateway_test.go) implements only AgentProcessor.
	agent := &mockAgent{response: "hi"}

	gw, err := New(Config{Agent: agent})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := NewDefaultMessageHandler(gw)
	client := newClient(nil, gw)

	resp, err := handler.handleChat(context.Background(), client, &Message{
		ID:      "m1",
		Type:    MessageTypeChat,
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("handleChat: %v", err)
	}
	if resp.Content != "hi" {
		t.Errorf("response content = %q, want %q", resp.Content, "hi")
	}
}
