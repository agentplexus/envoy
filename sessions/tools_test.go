package sessions

import (
	"context"
	"testing"

	"github.com/plexusone/omnillm/provider"
	"github.com/plexusone/omnistorage/kvs/memory"
)

func TestSkill_Metadata(t *testing.T) {
	skill := NewSkill()

	if skill.Name() != "sessions" {
		t.Errorf("Name() = %q, want 'sessions'", skill.Name())
	}

	if skill.Description() == "" {
		t.Error("Description() should not be empty")
	}

	tools := skill.Tools()
	if len(tools) != 7 {
		t.Errorf("Tools() returned %d tools, want 7", len(tools))
	}

	// Verify tool names
	expectedTools := []string{
		"sessions_list",
		"sessions_history",
		"sessions_get",
		"sessions_clear",
		"sessions_delete",
		"sessions_metadata_set",
		"sessions_metadata_get",
	}

	for i, expected := range expectedTools {
		if tools[i].Name != expected {
			t.Errorf("tool[%d].Name = %q, want %q", i, tools[i].Name, expected)
		}
	}
}

func TestSkill_Init_NoStorage(t *testing.T) {
	skill := NewSkill()

	err := skill.Init(context.Background())
	if err == nil {
		t.Error("Init() should error when storage not set")
	}
}

func TestSkill_Init_WithStorage(t *testing.T) {
	backend := memory.New()
	defer backend.Close()

	skill := NewSkill()
	skill.SetStorage(backend)

	err := skill.Init(context.Background())
	if err != nil {
		t.Errorf("Init() error = %v", err)
	}
}

func TestSkill_Close(t *testing.T) {
	skill := NewSkill()

	err := skill.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func setupSkillWithData(t *testing.T) (*Skill, func()) {
	t.Helper()

	backend := memory.New()
	skill := NewSkill()
	skill.SetStorage(backend)

	ctx := context.Background()

	// Create test sessions with data
	session1, _ := skill.store.Get(ctx, "session-1")
	session1.AddMessage(provider.RoleUser, "Hello")
	session1.AddMessage(provider.RoleAssistant, "Hi there!")
	session1.SetMetadata("user_name", "Alice")
	if err := skill.store.Save(ctx, session1); err != nil {
		t.Fatalf("Save(session1) error = %v", err)
	}

	session2, _ := skill.store.Get(ctx, "session-2")
	session2.AddMessage(provider.RoleUser, "How are you?")
	if err := skill.store.Save(ctx, session2); err != nil {
		t.Fatalf("Save(session2) error = %v", err)
	}

	cleanup := func() {
		backend.Close()
	}

	return skill, cleanup
}

func TestSkill_HandleList(t *testing.T) {
	skill, cleanup := setupSkillWithData(t)
	defer cleanup()

	ctx := context.Background()
	result, err := skill.handleList(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("handleList() error = %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is not map[string]any")
	}

	total, ok := resultMap["total"].(int)
	if !ok || total != 2 {
		t.Errorf("total = %v, want 2", resultMap["total"])
	}

	// Sessions key should exist
	sessions, ok := resultMap["sessions"]
	if !ok {
		t.Fatalf("sessions key not found in result")
	}

	// Verify it's a slice with the expected number of elements
	// The sessions slice is an anonymous struct type, so we can't type assert directly
	// We rely on the total count which matches the slice length
	if sessions == nil {
		t.Error("sessions should not be nil")
	}
}

func TestSkill_HandleHistory(t *testing.T) {
	skill, cleanup := setupSkillWithData(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("valid session", func(t *testing.T) {
		result, err := skill.handleHistory(ctx, map[string]any{
			"session_id": "session-1",
		})
		if err != nil {
			t.Fatalf("handleHistory() error = %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["session_id"] != "session-1" {
			t.Errorf("session_id = %v, want session-1", resultMap["session_id"])
		}
		if resultMap["total"] != 2 {
			t.Errorf("total = %v, want 2", resultMap["total"])
		}
	})

	t.Run("with limit", func(t *testing.T) {
		result, err := skill.handleHistory(ctx, map[string]any{
			"session_id": "session-1",
			"limit":      float64(1),
		})
		if err != nil {
			t.Fatalf("handleHistory() error = %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["returned"] != 1 {
			t.Errorf("returned = %v, want 1", resultMap["returned"])
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		_, err := skill.handleHistory(ctx, map[string]any{})
		if err == nil {
			t.Error("handleHistory() should error without session_id")
		}
	})

	t.Run("nonexistent session", func(t *testing.T) {
		_, err := skill.handleHistory(ctx, map[string]any{
			"session_id": "nonexistent",
		})
		if err == nil {
			t.Error("handleHistory() should error for nonexistent session")
		}
	})
}

func TestSkill_HandleGet(t *testing.T) {
	skill, cleanup := setupSkillWithData(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("valid session", func(t *testing.T) {
		result, err := skill.handleGet(ctx, map[string]any{
			"session_id": "session-1",
		})
		if err != nil {
			t.Fatalf("handleGet() error = %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["id"] != "session-1" {
			t.Errorf("id = %v, want session-1", resultMap["id"])
		}
		if resultMap["message_count"] != 2 {
			t.Errorf("message_count = %v, want 2", resultMap["message_count"])
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		_, err := skill.handleGet(ctx, map[string]any{})
		if err == nil {
			t.Error("handleGet() should error without session_id")
		}
	})
}

func TestSkill_HandleClear(t *testing.T) {
	skill, cleanup := setupSkillWithData(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("clear session", func(t *testing.T) {
		result, err := skill.handleClear(ctx, map[string]any{
			"session_id": "session-1",
		})
		if err != nil {
			t.Fatalf("handleClear() error = %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["messages_cleared"] != 2 {
			t.Errorf("messages_cleared = %v, want 2", resultMap["messages_cleared"])
		}
		if resultMap["status"] != "cleared" {
			t.Errorf("status = %v, want cleared", resultMap["status"])
		}

		// Verify session is cleared
		session, _ := skill.store.GetIfExists(ctx, "session-1")
		if len(session.Messages) != 0 {
			t.Errorf("session still has %d messages after clear", len(session.Messages))
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		_, err := skill.handleClear(ctx, map[string]any{})
		if err == nil {
			t.Error("handleClear() should error without session_id")
		}
	})
}

func TestSkill_HandleDelete(t *testing.T) {
	skill, cleanup := setupSkillWithData(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("delete session", func(t *testing.T) {
		result, err := skill.handleDelete(ctx, map[string]any{
			"session_id": "session-1",
		})
		if err != nil {
			t.Fatalf("handleDelete() error = %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["status"] != "deleted" {
			t.Errorf("status = %v, want deleted", resultMap["status"])
		}

		// Verify session is deleted
		_, err = skill.store.GetIfExists(ctx, "session-1")
		if err != ErrSessionNotFound {
			t.Errorf("GetIfExists() should return ErrSessionNotFound after delete")
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		_, err := skill.handleDelete(ctx, map[string]any{})
		if err == nil {
			t.Error("handleDelete() should error without session_id")
		}
	})
}

func TestSkill_HandleMetadataSet(t *testing.T) {
	skill, cleanup := setupSkillWithData(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("set string value", func(t *testing.T) {
		result, err := skill.handleMetadataSet(ctx, map[string]any{
			"session_id": "session-1",
			"key":        "theme",
			"value":      "dark",
		})
		if err != nil {
			t.Fatalf("handleMetadataSet() error = %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["status"] != "set" {
			t.Errorf("status = %v, want set", resultMap["status"])
		}

		// Verify metadata was set
		session, _ := skill.store.GetIfExists(ctx, "session-1")
		v, ok := session.GetMetadata("theme")
		if !ok || v != "dark" {
			t.Errorf("metadata theme = %v, %v, want dark, true", v, ok)
		}
	})

	t.Run("set JSON value", func(t *testing.T) {
		result, err := skill.handleMetadataSet(ctx, map[string]any{
			"session_id": "session-1",
			"key":        "prefs",
			"value":      `{"lang": "en", "tz": "UTC"}`,
		})
		if err != nil {
			t.Fatalf("handleMetadataSet() error = %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["status"] != "set" {
			t.Errorf("status = %v, want set", resultMap["status"])
		}

		// Verify metadata was set as parsed JSON
		session, _ := skill.store.GetIfExists(ctx, "session-1")
		v, ok := session.GetMetadata("prefs")
		if !ok {
			t.Fatal("prefs metadata not found")
		}
		vMap, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("prefs is not map[string]any, got %T", v)
		}
		if vMap["lang"] != "en" {
			t.Errorf("prefs.lang = %v, want en", vMap["lang"])
		}
	})

	t.Run("missing params", func(t *testing.T) {
		_, err := skill.handleMetadataSet(ctx, map[string]any{})
		if err == nil {
			t.Error("should error without session_id")
		}

		_, err = skill.handleMetadataSet(ctx, map[string]any{
			"session_id": "session-1",
		})
		if err == nil {
			t.Error("should error without key")
		}

		_, err = skill.handleMetadataSet(ctx, map[string]any{
			"session_id": "session-1",
			"key":        "foo",
		})
		if err == nil {
			t.Error("should error without value")
		}
	})
}

func TestSkill_HandleMetadataGet(t *testing.T) {
	skill, cleanup := setupSkillWithData(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get existing key", func(t *testing.T) {
		result, err := skill.handleMetadataGet(ctx, map[string]any{
			"session_id": "session-1",
			"key":        "user_name",
		})
		if err != nil {
			t.Fatalf("handleMetadataGet() error = %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["exists"] != true {
			t.Errorf("exists = %v, want true", resultMap["exists"])
		}
		if resultMap["value"] != "Alice" {
			t.Errorf("value = %v, want Alice", resultMap["value"])
		}
	})

	t.Run("get nonexistent key", func(t *testing.T) {
		result, err := skill.handleMetadataGet(ctx, map[string]any{
			"session_id": "session-1",
			"key":        "nonexistent",
		})
		if err != nil {
			t.Fatalf("handleMetadataGet() error = %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["exists"] != false {
			t.Errorf("exists = %v, want false", resultMap["exists"])
		}
	})

	t.Run("missing params", func(t *testing.T) {
		_, err := skill.handleMetadataGet(ctx, map[string]any{})
		if err == nil {
			t.Error("should error without session_id")
		}

		_, err = skill.handleMetadataGet(ctx, map[string]any{
			"session_id": "session-1",
		})
		if err == nil {
			t.Error("should error without key")
		}
	})

	t.Run("nonexistent session", func(t *testing.T) {
		_, err := skill.handleMetadataGet(ctx, map[string]any{
			"session_id": "nonexistent",
			"key":        "foo",
		})
		if err == nil {
			t.Error("should error for nonexistent session")
		}
	})
}
