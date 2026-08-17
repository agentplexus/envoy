package agent

import (
	"testing"

	"github.com/plexusone/omnillm/provider"
)

// TestLegacySessionStore covers the deprecated in-memory SessionStore/Session
// types in session.go (kept for backward compatibility; new code should use
// sessions.Store instead).
func TestLegacySessionStore(t *testing.T) {
	store := NewSessionStore()

	t.Run("Get creates a session on first access", func(t *testing.T) {
		s := store.Get("s1")
		if s == nil || s.ID != "s1" {
			t.Fatalf("Get() = %+v, want a session with ID s1", s)
		}
		if len(s.GetMessages()) != 0 {
			t.Errorf("new session has %d messages, want 0", len(s.GetMessages()))
		}
	})

	t.Run("Get returns the same session on repeat access", func(t *testing.T) {
		first := store.Get("s2")
		first.AddMessage(provider.RoleUser, "hello")
		second := store.Get("s2")
		if len(second.GetMessages()) != 1 {
			t.Errorf("Get() did not return the same session instance, got %d messages", len(second.GetMessages()))
		}
	})

	t.Run("List returns all created session IDs", func(t *testing.T) {
		fresh := NewSessionStore()
		fresh.Get("a")
		fresh.Get("b")
		ids := fresh.List()
		if len(ids) != 2 {
			t.Fatalf("List() = %v, want 2 ids", ids)
		}
		set := map[string]bool{}
		for _, id := range ids {
			set[id] = true
		}
		if !set["a"] || !set["b"] {
			t.Errorf("List() = %v, want [a b]", ids)
		}
	})

	t.Run("Delete removes a session", func(t *testing.T) {
		fresh := NewSessionStore()
		fresh.Get("gone")
		fresh.Delete("gone")
		if len(fresh.List()) != 0 {
			t.Errorf("List() after Delete = %v, want empty", fresh.List())
		}
		// Getting it again creates a fresh session rather than erroring.
		recreated := fresh.Get("gone")
		if len(recreated.GetMessages()) != 0 {
			t.Errorf("recreated session has messages: %v", recreated.GetMessages())
		}
	})
}

func TestLegacySession_MessagesAndMetadata(t *testing.T) {
	s := NewSessionStore().Get("s")

	s.AddMessage(provider.RoleUser, "hi")
	s.AddMessage(provider.RoleAssistant, "hello")
	msgs := s.GetMessages()
	if len(msgs) != 2 || msgs[0].Content != "hi" || msgs[1].Content != "hello" {
		t.Fatalf("GetMessages() = %+v", msgs)
	}

	// GetMessages returns a defensive copy.
	msgs[0].Content = "mutated"
	if s.GetMessages()[0].Content != "hi" {
		t.Error("GetMessages() leaked the internal slice — external mutation affected session state")
	}

	s.SetMetadata("key", "value")
	got, ok := s.GetMetadata("key")
	if !ok || got != "value" {
		t.Errorf("GetMetadata() = %v, %v, want value, true", got, ok)
	}
	if _, ok := s.GetMetadata("missing"); ok {
		t.Error("GetMetadata() for an unset key should report ok=false")
	}

	s.Clear()
	if len(s.GetMessages()) != 0 {
		t.Errorf("Clear() left %d messages", len(s.GetMessages()))
	}
}

func TestLegacySession_Trim(t *testing.T) {
	s := NewSessionStore().Get("s")
	for i := 0; i < 5; i++ {
		s.AddMessage(provider.RoleUser, "m")
	}
	s.Trim(3)
	if len(s.GetMessages()) != 3 {
		t.Errorf("Trim(3) left %d messages, want 3", len(s.GetMessages()))
	}

	// Trimming to more than the current length is a no-op.
	s.Trim(10)
	if len(s.GetMessages()) != 3 {
		t.Errorf("Trim(10) on a 3-message session changed length to %d", len(s.GetMessages()))
	}
}
