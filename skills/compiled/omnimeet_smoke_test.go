//go:build smoke

package compiled

import (
	"context"
	"testing"

	"github.com/plexusone/omnimeet-core/skill"
)

// TestOmniMeetSkillWiring validates that the OmniMeet MeetingSkill
// can be created, registered, and has the expected tools.
// This is a smoke test - it does NOT test actual meeting functionality.
// Full integration tests are in omni-livekit/omnimeet/skill_integration_test.go
func TestOmniMeetSkillWiring(t *testing.T) {
	// Create skill with nil provider (smoke test only - no real calls)
	meetingSkill := skill.New(nil, skill.Config{
		DefaultAgentName:   "Smoke Test Agent",
		DefaultMeetingName: "Smoke Test Meeting",
	})

	// Verify skill metadata
	if meetingSkill.Name() != "meeting" {
		t.Errorf("skill name = %q, want %q", meetingSkill.Name(), "meeting")
	}

	if meetingSkill.Description() == "" {
		t.Error("skill description is empty")
	}

	// Verify expected tools are present
	tools := meetingSkill.Tools()
	expectedTools := map[string]bool{
		"create_meeting":     false,
		"get_meeting":        false,
		"list_meetings":      false,
		"end_meeting":        false,
		"join_meeting":       false,
		"leave_meeting":      false,
		"get_join_link":      false,
		"list_participants":  false,
		"speak_in_meeting":   false,
	}

	for _, tool := range tools {
		if _, expected := expectedTools[tool.Name()]; expected {
			expectedTools[tool.Name()] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not found", name)
		}
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("tool count = %d, want %d", len(tools), len(expectedTools))
	}
}

// TestOmniMeetSkillRegistry validates that the MeetingSkill
// can be registered with the compiled skill registry.
func TestOmniMeetSkillRegistry(t *testing.T) {
	registry := NewRegistry()

	meetingSkill := skill.New(nil, skill.Config{})

	// Register should succeed
	if err := registry.Register(meetingSkill); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Get should return the skill
	got, ok := registry.Get("meeting")
	if !ok {
		t.Fatal("Get() should find registered skill")
	}
	if got.Name() != "meeting" {
		t.Errorf("Get() name = %q, want %q", got.Name(), "meeting")
	}

	// All tools should be accessible via FindTool
	toolsToFind := []string{
		"create_meeting",
		"get_meeting",
		"end_meeting",
		"get_join_link",
	}

	for _, toolName := range toolsToFind {
		tool, sk, ok := registry.FindTool(toolName)
		if !ok {
			t.Errorf("FindTool(%q) should find tool", toolName)
			continue
		}
		if tool.Name() != toolName {
			t.Errorf("FindTool(%q) tool name = %q", toolName, tool.Name())
		}
		if sk.Name() != "meeting" {
			t.Errorf("FindTool(%q) skill name = %q, want %q", toolName, sk.Name(), "meeting")
		}
	}
}

// TestOmniMeetSkillLifecycle validates Init/Close work correctly.
func TestOmniMeetSkillLifecycle(t *testing.T) {
	meetingSkill := skill.New(nil, skill.Config{})

	ctx := context.Background()

	// Init should succeed with nil provider
	if err := meetingSkill.Init(ctx); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Close should succeed
	if err := meetingSkill.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
