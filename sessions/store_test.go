package sessions

import (
	"context"
	"testing"
	"time"

	"github.com/plexusone/omnillm/provider"
	"github.com/plexusone/omnistorage-core/kvs/backend/memory"
)

func TestStore_GetAndSave(t *testing.T) {
	ctx := context.Background()
	backend := memory.New()
	defer backend.Close()

	store := NewStore(StoreConfig{
		Backend: backend,
		TTL:     time.Hour,
	})

	// Get creates a new session
	session, err := store.Get(ctx, "test-session")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if session.ID != "test-session" {
		t.Errorf("session.ID = %q, want %q", session.ID, "test-session")
	}

	// Add some messages
	session.AddMessage(provider.RoleUser, "Hello")
	session.AddMessage(provider.RoleAssistant, "Hi there!")

	// Save the session
	if err := store.Save(ctx, session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Clear cache and reload
	store.ClearCache()

	// Get the session again
	loaded, err := store.Get(ctx, "test-session")
	if err != nil {
		t.Fatalf("Get() after save error = %v", err)
	}

	if len(loaded.Messages) != 2 {
		t.Errorf("loaded session has %d messages, want 2", len(loaded.Messages))
	}

	if loaded.Messages[0].Content != "Hello" {
		t.Errorf("first message = %q, want %q", loaded.Messages[0].Content, "Hello")
	}
}

func TestStore_GetIfExists(t *testing.T) {
	ctx := context.Background()
	backend := memory.New()
	defer backend.Close()

	store := NewStore(StoreConfig{
		Backend: backend,
		TTL:     time.Hour,
	})

	// GetIfExists should return error for non-existent session
	_, err := store.GetIfExists(ctx, "nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("GetIfExists() error = %v, want ErrSessionNotFound", err)
	}

	// Create a session
	session, err := store.Get(ctx, "test-session")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := store.Save(ctx, session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Clear cache
	store.ClearCache()

	// Now GetIfExists should work
	loaded, err := store.GetIfExists(ctx, "test-session")
	if err != nil {
		t.Fatalf("GetIfExists() error = %v", err)
	}

	if loaded.ID != "test-session" {
		t.Errorf("session.ID = %q, want %q", loaded.ID, "test-session")
	}
}

func TestStore_Delete(t *testing.T) {
	ctx := context.Background()
	backend := memory.New()
	defer backend.Close()

	store := NewStore(StoreConfig{
		Backend: backend,
		TTL:     time.Hour,
	})

	// Create and save a session
	session, err := store.Get(ctx, "to-delete")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := store.Save(ctx, session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Delete the session
	if err := store.Delete(ctx, "to-delete"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Session should no longer exist
	_, err = store.GetIfExists(ctx, "to-delete")
	if err != ErrSessionNotFound {
		t.Errorf("GetIfExists() after delete error = %v, want ErrSessionNotFound", err)
	}
}

func TestStore_List(t *testing.T) {
	ctx := context.Background()
	backend := memory.New()
	defer backend.Close()

	store := NewStore(StoreConfig{
		Backend: backend,
		TTL:     time.Hour,
	})

	// Create and save multiple sessions
	for _, id := range []string{"session1", "session2", "session3"} {
		session, err := store.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get(%s) error = %v", id, err)
		}
		if err := store.Save(ctx, session); err != nil {
			t.Fatalf("Save(%s) error = %v", id, err)
		}
	}

	// List all sessions
	ids, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("List() returned %d sessions, want 3", len(ids))
	}

	// Check that all expected IDs are present
	found := make(map[string]bool)
	for _, id := range ids {
		found[id] = true
	}

	for _, expected := range []string{"session1", "session2", "session3"} {
		if !found[expected] {
			t.Errorf("List() missing session %q", expected)
		}
	}
}

func TestSession_AddMessage(t *testing.T) {
	session := NewSession("test")

	session.AddMessage(provider.RoleUser, "Hello")
	session.AddMessage(provider.RoleAssistant, "Hi!")

	messages := session.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("GetMessages() returned %d messages, want 2", len(messages))
	}

	if messages[0].Role != provider.RoleUser || messages[0].Content != "Hello" {
		t.Errorf("first message = %+v, want user: Hello", messages[0])
	}

	if messages[1].Role != provider.RoleAssistant || messages[1].Content != "Hi!" {
		t.Errorf("second message = %+v, want assistant: Hi!", messages[1])
	}
}

func TestSession_Trim(t *testing.T) {
	session := NewSession("test")

	// Add 5 messages
	for i := 1; i <= 5; i++ {
		session.AddMessage(provider.RoleUser, string(rune('A'+i-1)))
	}

	// Trim to last 3
	session.Trim(3)

	messages := session.GetMessages()
	if len(messages) != 3 {
		t.Fatalf("after Trim(3), got %d messages, want 3", len(messages))
	}

	// Should have C, D, E
	if messages[0].Content != "C" {
		t.Errorf("first message after trim = %q, want C", messages[0].Content)
	}
}

func TestSession_Clear(t *testing.T) {
	session := NewSession("test")
	session.AddMessage(provider.RoleUser, "Hello")
	session.AddMessage(provider.RoleAssistant, "Hi!")

	session.Clear()

	messages := session.GetMessages()
	if len(messages) != 0 {
		t.Errorf("after Clear(), got %d messages, want 0", len(messages))
	}
}

func TestSession_Metadata(t *testing.T) {
	session := NewSession("test")

	session.SetMetadata("key1", "value1")
	session.SetMetadata("key2", 42)

	v1, ok := session.GetMetadata("key1")
	if !ok || v1 != "value1" {
		t.Errorf("GetMetadata(key1) = %v, %v, want value1, true", v1, ok)
	}

	v2, ok := session.GetMetadata("key2")
	if !ok || v2 != 42 {
		t.Errorf("GetMetadata(key2) = %v, %v, want 42, true", v2, ok)
	}

	_, ok = session.GetMetadata("nonexistent")
	if ok {
		t.Error("GetMetadata(nonexistent) should return false")
	}
}

func TestSession_AddMessageWithToolCalls(t *testing.T) {
	session := NewSession("test")

	toolCalls := []provider.ToolCall{
		{
			ID:   "call_123",
			Type: "function",
			Function: provider.ToolFunction{
				Name:      "get_weather",
				Arguments: `{"location": "NYC"}`,
			},
		},
	}

	session.AddMessageWithToolCalls(provider.RoleAssistant, "Let me check the weather", toolCalls)

	messages := session.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("GetMessages() returned %d messages, want 1", len(messages))
	}

	msg := messages[0]
	if msg.Role != provider.RoleAssistant {
		t.Errorf("message role = %q, want assistant", msg.Role)
	}
	if msg.Content != "Let me check the weather" {
		t.Errorf("message content = %q, want 'Let me check the weather'", msg.Content)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("message has %d tool calls, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("tool call name = %q, want 'get_weather'", msg.ToolCalls[0].Function.Name)
	}
}

func TestSession_AddToolResult(t *testing.T) {
	session := NewSession("test")

	session.AddToolResult("call_123", `{"temperature": 72, "unit": "F"}`)

	messages := session.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("GetMessages() returned %d messages, want 1", len(messages))
	}

	msg := messages[0]
	if msg.Role != provider.RoleTool {
		t.Errorf("message role = %q, want tool", msg.Role)
	}
	if msg.ToolCallID == nil || *msg.ToolCallID != "call_123" {
		t.Errorf("tool call ID = %v, want 'call_123'", msg.ToolCallID)
	}
	if msg.Content != `{"temperature": 72, "unit": "F"}` {
		t.Errorf("message content = %q, want JSON result", msg.Content)
	}
}

func TestSession_MetadataNilMap(t *testing.T) {
	// Test GetMetadata with nil map
	session := &Session{
		ID:       "test",
		Metadata: nil,
	}

	_, ok := session.GetMetadata("key")
	if ok {
		t.Error("GetMetadata on nil map should return false")
	}

	// SetMetadata should initialize the map
	session.SetMetadata("key", "value")
	v, ok := session.GetMetadata("key")
	if !ok || v != "value" {
		t.Errorf("GetMetadata after set = %v, %v, want 'value', true", v, ok)
	}
}

func TestStore_Touch(t *testing.T) {
	ctx := context.Background()
	backend := memory.New()
	defer backend.Close()

	store := NewStore(StoreConfig{
		Backend: backend,
		TTL:     time.Hour,
	})

	// Create and save a session
	session, err := store.Get(ctx, "touch-test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	session.AddMessage(provider.RoleUser, "Hello")
	if err := store.Save(ctx, session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Touch the session - should not error
	if err := store.Touch(ctx, "touch-test"); err != nil {
		t.Fatalf("Touch() error = %v", err)
	}

	// Session should still be accessible
	store.ClearCache()
	touched, err := store.GetIfExists(ctx, "touch-test")
	if err != nil {
		t.Fatalf("GetIfExists() after Touch error = %v", err)
	}

	// Content should be preserved
	if len(touched.Messages) != 1 {
		t.Errorf("session has %d messages, want 1", len(touched.Messages))
	}
}

func TestStore_Touch_NonExistent(t *testing.T) {
	ctx := context.Background()
	backend := memory.New()
	defer backend.Close()

	store := NewStore(StoreConfig{
		Backend: backend,
		TTL:     time.Hour,
	})

	// Touch non-existent session should error
	err := store.Touch(ctx, "nonexistent")
	if err == nil {
		t.Error("Touch() should error for non-existent session")
	}
}

func TestStore_Close(t *testing.T) {
	backend := memory.New()

	store := NewStore(StoreConfig{
		Backend: backend,
		TTL:     time.Hour,
	})

	// Close should not error
	err := store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestStore_DefaultTTL(t *testing.T) {
	backend := memory.New()
	defer backend.Close()

	// Create store without TTL, should use default
	store := NewStore(StoreConfig{
		Backend: backend,
	})

	if store.ttl != DefaultSessionTTL {
		t.Errorf("store TTL = %v, want %v", store.ttl, DefaultSessionTTL)
	}
}

func TestStore_CacheHit(t *testing.T) {
	ctx := context.Background()
	backend := memory.New()
	defer backend.Close()

	store := NewStore(StoreConfig{
		Backend: backend,
		TTL:     time.Hour,
	})

	// Get creates session and caches it
	session1, err := store.Get(ctx, "cache-test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	session1.AddMessage(provider.RoleUser, "First message")

	// Get again should return cached version (same pointer)
	session2, err := store.Get(ctx, "cache-test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Should have the message added to session1
	if len(session2.Messages) != 1 {
		t.Errorf("cached session should have 1 message, got %d", len(session2.Messages))
	}
}
