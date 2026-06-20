package registry

import (
	"context"
	"testing"
	"time"

	"github.com/plexusone/omniagent/agent"
)

// mockAgent creates a minimal agent for testing.
func mockAgentFactory(cfg *AgentConfig) (*agent.Agent, error) {
	// For testing, we'll create a real agent with a test config
	// In a real test, you might mock the omnillm client
	return agent.New(agent.Config{
		Provider:     "anthropic",
		Model:        cfg.Model,
		APIKey:       "test-key", // Will fail on actual calls but works for struct creation
		SystemPrompt: cfg.SystemPrompt,
	})
}

func TestAgentConfig_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  *bool
		expected bool
	}{
		{
			name:     "nil defaults to true",
			enabled:  nil,
			expected: true,
		},
		{
			name:     "explicit true",
			enabled:  boolPtr(true),
			expected: true,
		},
		{
			name:     "explicit false",
			enabled:  boolPtr(false),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AgentConfig{Enabled: tt.enabled}
			if got := cfg.IsEnabled(); got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAgentConfig_Clone(t *testing.T) {
	original := &AgentConfig{
		ID:           "test-id",
		Name:         "Test Agent",
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-20250514",
		AllowedTools: []string{"tool1", "tool2"},
		DeniedTools:  []string{"tool3"},
		Enabled:      boolPtr(true),
		CreatedAt:    time.Now(),
	}

	clone := original.Clone()

	// Verify values are copied
	if clone.ID != original.ID {
		t.Errorf("ID not copied: got %s, want %s", clone.ID, original.ID)
	}

	// Verify slices are deep copied
	clone.AllowedTools[0] = "modified"
	if original.AllowedTools[0] == "modified" {
		t.Error("AllowedTools was not deep copied")
	}

	// Verify Enabled is deep copied
	*clone.Enabled = false
	if !*original.Enabled {
		t.Error("Enabled was not deep copied")
	}
}

func TestAgentConfig_Merge(t *testing.T) {
	base := &AgentConfig{
		ID:       "base-id",
		Name:     "Base Agent",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
	}

	updates := &AgentConfig{
		Name:        "Updated Agent",
		Temperature: 0.7,
	}

	base.Merge(updates)

	if base.Name != "Updated Agent" {
		t.Errorf("Name not merged: got %s, want Updated Agent", base.Name)
	}
	if base.Temperature != 0.7 {
		t.Errorf("Temperature not merged: got %f, want 0.7", base.Temperature)
	}
	if base.ID != "base-id" {
		t.Errorf("ID should not change: got %s, want base-id", base.ID)
	}
}

func TestRegistry_Create(t *testing.T) {
	reg := New(RegistryConfig{
		Factory: mockAgentFactory,
	})
	defer reg.Close()

	cfg := &AgentConfig{
		ID:       "test-agent",
		Name:     "Test Agent",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
	}

	err := reg.Create(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify agent exists
	_, err = reg.Get("test-agent")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	// Verify config
	gotCfg, err := reg.GetConfig("test-agent")
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if gotCfg.Name != "Test Agent" {
		t.Errorf("Name mismatch: got %s, want Test Agent", gotCfg.Name)
	}
}

func TestRegistry_CreateDuplicate(t *testing.T) {
	reg := New(RegistryConfig{
		Factory: mockAgentFactory,
	})
	defer reg.Close()

	cfg := &AgentConfig{
		ID:       "dup-agent",
		Name:     "First Agent",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
	}

	if err := reg.Create(context.Background(), cfg); err != nil {
		t.Fatalf("First Create failed: %v", err)
	}

	// Try to create duplicate
	cfg.Name = "Second Agent"
	err := reg.Create(context.Background(), cfg)
	if err != ErrAgentExists {
		t.Errorf("Expected ErrAgentExists, got: %v", err)
	}
}

func TestRegistry_GetByModel(t *testing.T) {
	reg := New(RegistryConfig{
		Factory: mockAgentFactory,
	})
	defer reg.Close()

	// Create test agents
	if err := reg.Create(context.Background(), &AgentConfig{
		ID:       "default",
		Name:     "Default Agent",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
	}); err != nil {
		t.Fatalf("Create default failed: %v", err)
	}

	if err := reg.Create(context.Background(), &AgentConfig{
		ID:       "coder",
		Name:     "Code Expert",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
	}); err != nil {
		t.Fatalf("Create coder failed: %v", err)
	}

	tests := []struct {
		name      string
		modelName string
		wantID    string
		wantErr   bool
	}{
		{
			name:      "exact ID match",
			modelName: "coder",
			wantID:    "coder",
		},
		{
			name:      "omniagent returns default",
			modelName: "omniagent",
			wantID:    "default",
		},
		{
			name:      "empty returns default",
			modelName: "",
			wantID:    "default",
		},
		{
			name:      "case insensitive name",
			modelName: "CODE EXPERT",
			wantID:    "coder",
		},
		{
			name:      "not found",
			modelName: "nonexistent",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ag, err := reg.GetByModel(tt.modelName)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetByModel failed: %v", err)
			}
			if ag == nil {
				t.Error("Expected agent, got nil")
			}
		})
	}
}

func TestRegistry_Clone(t *testing.T) {
	reg := New(RegistryConfig{
		Factory: mockAgentFactory,
	})
	defer reg.Close()

	// Create source agent
	if err := reg.Create(context.Background(), &AgentConfig{
		ID:           "source",
		Name:         "Source Agent",
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-20250514",
		SystemPrompt: "You are helpful.",
		Temperature:  0.7,
	}); err != nil {
		t.Fatalf("Create source failed: %v", err)
	}

	// Clone it
	err := reg.Clone(context.Background(), "source", "cloned", "Cloned Agent")
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	// Verify clone exists
	clonedCfg, err := reg.GetConfig("cloned")
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	if clonedCfg.Name != "Cloned Agent" {
		t.Errorf("Name mismatch: got %s, want Cloned Agent", clonedCfg.Name)
	}
	if clonedCfg.SystemPrompt != "You are helpful." {
		t.Errorf("SystemPrompt not copied: got %s", clonedCfg.SystemPrompt)
	}
	if clonedCfg.Temperature != 0.7 {
		t.Errorf("Temperature not copied: got %f", clonedCfg.Temperature)
	}
}

func TestRegistry_Delete(t *testing.T) {
	reg := New(RegistryConfig{
		Factory: mockAgentFactory,
	})
	defer reg.Close()

	// Create agent
	if err := reg.Create(context.Background(), &AgentConfig{
		ID:       "to-delete",
		Name:     "Delete Me",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete it
	if err := reg.Delete(context.Background(), "to-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err := reg.Get("to-delete")
	if err != ErrAgentNotFound {
		t.Errorf("Expected ErrAgentNotFound, got: %v", err)
	}
}

func TestRegistry_List(t *testing.T) {
	reg := New(RegistryConfig{
		Factory: mockAgentFactory,
	})
	defer reg.Close()

	// Create multiple agents
	for i := 0; i < 3; i++ {
		enabled := i != 1 // Disable agent 1
		if err := reg.Create(context.Background(), &AgentConfig{
			ID:       string(rune('a' + i)),
			Name:     "Agent " + string(rune('A'+i)),
			Provider: "anthropic",
			Model:    "claude-sonnet-4-20250514",
			Enabled:  &enabled,
		}); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// List all
	all := reg.List()
	if len(all) != 3 {
		t.Errorf("List() returned %d agents, want 3", len(all))
	}

	// List enabled only
	enabled := reg.ListEnabled()
	if len(enabled) != 2 {
		t.Errorf("ListEnabled() returned %d agents, want 2", len(enabled))
	}
}

func boolPtr(b bool) *bool {
	return &b
}
