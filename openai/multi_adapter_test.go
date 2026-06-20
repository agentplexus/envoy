package openai

import (
	"context"
	"testing"
	"time"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/agent/registry"
	"github.com/plexusone/omniagent/api/openai"
)

// mockAgentFactory creates a minimal agent for testing.
func mockAgentFactory(cfg *registry.AgentConfig) (*agent.Agent, error) {
	return agent.New(agent.Config{
		Provider:     "anthropic",
		Model:        cfg.Model,
		APIKey:       "test-key",
		SystemPrompt: cfg.SystemPrompt,
	})
}

// setupTestRegistry creates a registry with test agents.
func setupTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()

	reg := registry.New(registry.RegistryConfig{
		Factory: mockAgentFactory,
		Defaults: &registry.AgentConfig{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-20250514",
			APIKey:   "test-key",
		},
	})

	// Create test agents
	ctx := context.Background()

	if err := reg.Create(ctx, &registry.AgentConfig{
		ID:          "default",
		Name:        "Default Agent",
		Description: "The default assistant",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
		Temperature: 0.7,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}); err != nil {
		t.Fatalf("failed to create default agent: %v", err)
	}

	if err := reg.Create(ctx, &registry.AgentConfig{
		ID:           "coder",
		Name:         "Code Expert",
		Description:  "Expert programmer",
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-20250514",
		SystemPrompt: "You are an expert programmer.",
		Temperature:  0.5,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}); err != nil {
		t.Fatalf("failed to create coder agent: %v", err)
	}

	return reg
}

func TestNewMultiAgentAdapter(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	t.Run("default options", func(t *testing.T) {
		adapter := NewMultiAgentAdapter(reg)

		if adapter.registry != reg {
			t.Error("registry not set")
		}
		if adapter.modelOwner != "plexusone" {
			t.Errorf("modelOwner = %s, want plexusone", adapter.modelOwner)
		}
		if adapter.useSession != false {
			t.Error("useSession should be false by default")
		}
	})

	t.Run("with options", func(t *testing.T) {
		adapter := NewMultiAgentAdapter(reg,
			WithMultiModelOwner("custom-owner"),
			WithMultiSession(true),
		)

		if adapter.modelOwner != "custom-owner" {
			t.Errorf("modelOwner = %s, want custom-owner", adapter.modelOwner)
		}
		if adapter.useSession != true {
			t.Error("useSession should be true")
		}
	})
}

func TestMultiAgentAdapter_ListModels(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg, WithMultiModelOwner("test-owner"))
	ctx := context.Background()

	models, err := adapter.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("got %d models, want 2", len(models))
	}

	// Check that both agents are returned as models
	modelIDs := make(map[string]bool)
	for _, m := range models {
		modelIDs[m.ID] = true
		if m.Object != "model" {
			t.Errorf("model %s has object = %s, want model", m.ID, m.Object)
		}
		if m.OwnedBy != "test-owner" {
			t.Errorf("model %s has owned_by = %s, want test-owner", m.ID, m.OwnedBy)
		}
	}

	if !modelIDs["default"] {
		t.Error("default model not found")
	}
	if !modelIDs["coder"] {
		t.Error("coder model not found")
	}
}

func TestMultiAgentAdapter_GetModel(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	t.Run("existing model", func(t *testing.T) {
		model, err := adapter.GetModel(ctx, "coder")
		if err != nil {
			t.Fatalf("GetModel failed: %v", err)
		}

		if model.ID != "coder" {
			t.Errorf("ID = %s, want coder", model.ID)
		}
		if model.Object != "model" {
			t.Errorf("Object = %s, want model", model.Object)
		}
	})

	t.Run("non-existing model", func(t *testing.T) {
		_, err := adapter.GetModel(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existing model")
		}
	})
}

func TestMultiAgentAdapter_ListAgents(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	agents, err := adapter.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("got %d agents, want 2", len(agents))
	}

	// Check agent details
	agentMap := make(map[string]openai.AgentInfo)
	for _, a := range agents {
		agentMap[a.ID] = a
	}

	if defaultAgent, ok := agentMap["default"]; !ok {
		t.Error("default agent not found")
	} else {
		if defaultAgent.Name != "Default Agent" {
			t.Errorf("default agent name = %s, want Default Agent", defaultAgent.Name)
		}
		if defaultAgent.Temperature != 0.7 {
			t.Errorf("default agent temperature = %f, want 0.7", defaultAgent.Temperature)
		}
	}

	if coderAgent, ok := agentMap["coder"]; !ok {
		t.Error("coder agent not found")
	} else {
		if coderAgent.SystemPrompt != "You are an expert programmer." {
			t.Errorf("coder agent system_prompt = %s", coderAgent.SystemPrompt)
		}
	}
}

func TestMultiAgentAdapter_GetAgent(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	t.Run("existing agent", func(t *testing.T) {
		agent, err := adapter.GetAgent(ctx, "coder")
		if err != nil {
			t.Fatalf("GetAgent failed: %v", err)
		}

		if agent.ID != "coder" {
			t.Errorf("ID = %s, want coder", agent.ID)
		}
		if agent.Name != "Code Expert" {
			t.Errorf("Name = %s, want Code Expert", agent.Name)
		}
		if agent.Description != "Expert programmer" {
			t.Errorf("Description = %s, want Expert programmer", agent.Description)
		}
	})

	t.Run("non-existing agent", func(t *testing.T) {
		_, err := adapter.GetAgent(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existing agent")
		}
	})
}

func TestMultiAgentAdapter_CreateAgent(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	req := &openai.CreateAgentRequest{
		ID:           "writer",
		Name:         "Writing Assistant",
		Description:  "Helps with writing tasks",
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-20250514",
		Temperature:  0.9,
		SystemPrompt: "You are a creative writer.",
	}

	agent, err := adapter.CreateAgent(ctx, req)
	if err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	if agent.ID != "writer" {
		t.Errorf("ID = %s, want writer", agent.ID)
	}
	if agent.Name != "Writing Assistant" {
		t.Errorf("Name = %s, want Writing Assistant", agent.Name)
	}
	if agent.Temperature != 0.9 {
		t.Errorf("Temperature = %f, want 0.9", agent.Temperature)
	}
	if !agent.Enabled {
		t.Error("Enabled should be true by default")
	}

	// Verify agent was persisted
	retrieved, err := adapter.GetAgent(ctx, "writer")
	if err != nil {
		t.Fatalf("failed to retrieve created agent: %v", err)
	}
	if retrieved.Name != "Writing Assistant" {
		t.Errorf("retrieved name = %s, want Writing Assistant", retrieved.Name)
	}
}

func TestMultiAgentAdapter_CreateAgent_AutoID(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	req := &openai.CreateAgentRequest{
		// No ID specified - should be auto-generated
		Name:     "Auto ID Agent",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
	}

	agent, err := adapter.CreateAgent(ctx, req)
	if err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	if agent.ID == "" {
		t.Error("ID should be auto-generated")
	}
	if agent.Name != "Auto ID Agent" {
		t.Errorf("Name = %s, want Auto ID Agent", agent.Name)
	}
}

func TestMultiAgentAdapter_UpdateAgent(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	// Update the coder agent
	newName := "Senior Code Expert"
	newTemp := 0.3
	req := &openai.UpdateAgentRequest{
		Name:        &newName,
		Temperature: &newTemp,
	}

	agent, err := adapter.UpdateAgent(ctx, "coder", req)
	if err != nil {
		t.Fatalf("UpdateAgent failed: %v", err)
	}

	if agent.Name != "Senior Code Expert" {
		t.Errorf("Name = %s, want Senior Code Expert", agent.Name)
	}
	if agent.Temperature != 0.3 {
		t.Errorf("Temperature = %f, want 0.3", agent.Temperature)
	}
	// Original fields should be preserved
	if agent.SystemPrompt != "You are an expert programmer." {
		t.Errorf("SystemPrompt changed unexpectedly: %s", agent.SystemPrompt)
	}
}

func TestMultiAgentAdapter_UpdateAgent_NotFound(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	newName := "Updated"
	req := &openai.UpdateAgentRequest{
		Name: &newName,
	}

	_, err := adapter.UpdateAgent(ctx, "nonexistent", req)
	if err == nil {
		t.Error("expected error for non-existing agent")
	}
}

func TestMultiAgentAdapter_DeleteAgent(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	// Delete the coder agent
	err := adapter.DeleteAgent(ctx, "coder")
	if err != nil {
		t.Fatalf("DeleteAgent failed: %v", err)
	}

	// Verify it's gone
	_, err = adapter.GetAgent(ctx, "coder")
	if err == nil {
		t.Error("agent should be deleted")
	}

	// Verify other agents still exist
	agents, err := adapter.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("got %d agents, want 1", len(agents))
	}
}

func TestMultiAgentAdapter_DeleteAgent_NotFound(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	err := adapter.DeleteAgent(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existing agent")
	}
}

func TestMultiAgentAdapter_CloneAgent(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	req := &openai.CloneAgentRequest{
		NewID:   "coder-v2",
		NewName: "Code Expert V2",
	}

	cloned, err := adapter.CloneAgent(ctx, "coder", req)
	if err != nil {
		t.Fatalf("CloneAgent failed: %v", err)
	}

	if cloned.ID != "coder-v2" {
		t.Errorf("ID = %s, want coder-v2", cloned.ID)
	}
	if cloned.Name != "Code Expert V2" {
		t.Errorf("Name = %s, want Code Expert V2", cloned.Name)
	}
	// Should preserve other fields from original
	if cloned.SystemPrompt != "You are an expert programmer." {
		t.Errorf("SystemPrompt not copied: %s", cloned.SystemPrompt)
	}
	if cloned.Temperature != 0.5 {
		t.Errorf("Temperature not copied: %f", cloned.Temperature)
	}

	// Original should still exist
	original, err := adapter.GetAgent(ctx, "coder")
	if err != nil {
		t.Fatalf("original agent should still exist: %v", err)
	}
	if original.Name != "Code Expert" {
		t.Error("original agent name should be unchanged")
	}

	// Verify we now have 3 agents
	agents, err := adapter.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	if len(agents) != 3 {
		t.Errorf("got %d agents, want 3", len(agents))
	}
}

func TestMultiAgentAdapter_CloneAgent_NotFound(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	req := &openai.CloneAgentRequest{
		NewName: "Clone",
	}

	_, err := adapter.CloneAgent(ctx, "nonexistent", req)
	if err == nil {
		t.Error("expected error for non-existing agent")
	}
}

func TestMultiAgentAdapter_Registry(t *testing.T) {
	reg := setupTestRegistry(t)
	defer reg.Close()

	adapter := NewMultiAgentAdapter(reg)

	if adapter.Registry() != reg {
		t.Error("Registry() should return the underlying registry")
	}
}

func TestConfigToAgentInfo(t *testing.T) {
	now := time.Now()
	enabled := true

	cfg := &registry.AgentConfig{
		ID:           "test-id",
		Name:         "Test Agent",
		Description:  "Test description",
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-20250514",
		Temperature:  0.8,
		MaxTokens:    4096,
		SystemPrompt: "You are helpful.",
		AllowedTools: []string{"read", "write"},
		DeniedTools:  []string{"exec"},
		Enabled:      &enabled,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	info := configToAgentInfo(cfg)

	if info.ID != cfg.ID {
		t.Errorf("ID = %s, want %s", info.ID, cfg.ID)
	}
	if info.Name != cfg.Name {
		t.Errorf("Name = %s, want %s", info.Name, cfg.Name)
	}
	if info.Description != cfg.Description {
		t.Errorf("Description = %s, want %s", info.Description, cfg.Description)
	}
	if info.Provider != cfg.Provider {
		t.Errorf("Provider = %s, want %s", info.Provider, cfg.Provider)
	}
	if info.Model != cfg.Model {
		t.Errorf("Model = %s, want %s", info.Model, cfg.Model)
	}
	if info.Temperature != cfg.Temperature {
		t.Errorf("Temperature = %f, want %f", info.Temperature, cfg.Temperature)
	}
	if info.MaxTokens != cfg.MaxTokens {
		t.Errorf("MaxTokens = %d, want %d", info.MaxTokens, cfg.MaxTokens)
	}
	if info.SystemPrompt != cfg.SystemPrompt {
		t.Errorf("SystemPrompt = %s, want %s", info.SystemPrompt, cfg.SystemPrompt)
	}
	if len(info.AllowedTools) != 2 {
		t.Errorf("AllowedTools length = %d, want 2", len(info.AllowedTools))
	}
	if len(info.DeniedTools) != 1 {
		t.Errorf("DeniedTools length = %d, want 1", len(info.DeniedTools))
	}
	if !info.Enabled {
		t.Error("Enabled should be true")
	}
	if !info.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", info.CreatedAt, now)
	}
	if !info.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", info.UpdatedAt, now)
	}
}

func TestConfigToAgentInfo_DefaultEnabled(t *testing.T) {
	cfg := &registry.AgentConfig{
		ID:   "test",
		Name: "Test",
		// Enabled is nil - should default to true
	}

	info := configToAgentInfo(cfg)

	if !info.Enabled {
		t.Error("Enabled should default to true when nil")
	}
}
