package registry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/agent"
)

// Registry manages multiple agent instances.
type Registry struct {
	agents    map[string]*agent.Agent
	configs   map[string]*AgentConfig
	store     *Store
	defaults  *AgentConfig
	factoryFn AgentFactory
	logger    *slog.Logger
	mu        sync.RWMutex
}

// AgentFactory creates an agent from a config.
// This allows customization of agent creation (e.g., adding tools, skills).
type AgentFactory func(cfg *AgentConfig) (*agent.Agent, error)

// RegistryConfig configures the registry.
type RegistryConfig struct {
	// Store is the persistent storage for agent configs.
	Store *Store

	// Defaults provides fallback values for new agents.
	Defaults *AgentConfig

	// Factory creates agents from configs. If nil, uses DefaultAgentFactory.
	Factory AgentFactory

	// Logger for registry operations.
	Logger *slog.Logger
}

// New creates a new agent registry.
func New(cfg RegistryConfig) *Registry {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	factory := cfg.Factory
	if factory == nil {
		factory = DefaultAgentFactory
	}

	return &Registry{
		agents:    make(map[string]*agent.Agent),
		configs:   make(map[string]*AgentConfig),
		store:     cfg.Store,
		defaults:  cfg.Defaults,
		factoryFn: factory,
		logger:    logger,
	}
}

// DefaultAgentFactory creates an agent using the standard agent.New function.
func DefaultAgentFactory(cfg *AgentConfig) (*agent.Agent, error) {
	return agent.New(agent.Config{
		Provider:     cfg.Provider,
		Model:        cfg.Model,
		APIKey:       cfg.APIKey,
		BaseURL:      cfg.BaseURL,
		Temperature:  cfg.Temperature,
		MaxTokens:    cfg.MaxTokens,
		SystemPrompt: cfg.SystemPrompt,
	})
}

// Get retrieves an agent by ID.
// Returns ErrAgentNotFound if the agent doesn't exist.
func (r *Registry) Get(id string) (*agent.Agent, error) {
	r.mu.RLock()
	ag, ok := r.agents[id]
	r.mu.RUnlock()

	if !ok {
		return nil, ErrAgentNotFound
	}
	return ag, nil
}

// GetConfig retrieves an agent's config by ID.
func (r *Registry) GetConfig(id string) (*AgentConfig, error) {
	r.mu.RLock()
	cfg, ok := r.configs[id]
	r.mu.RUnlock()

	if !ok {
		return nil, ErrAgentNotFound
	}
	return cfg.Clone(), nil
}

// GetByModel retrieves an agent by model name.
// Supports exact ID match, case-insensitive name match, and "omniagent" default.
func (r *Registry) GetByModel(modelName string) (*agent.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Empty or "omniagent" returns default
	if modelName == "" || strings.EqualFold(modelName, "omniagent") {
		return r.defaultLocked()
	}

	// Exact ID match
	if ag, ok := r.agents[modelName]; ok {
		return ag, nil
	}

	// Case-insensitive name match
	lowerName := strings.ToLower(modelName)
	for id, cfg := range r.configs {
		if strings.ToLower(cfg.Name) == lowerName || strings.ToLower(cfg.ID) == lowerName {
			return r.agents[id], nil
		}
	}

	return nil, fmt.Errorf("agent not found: %s", modelName)
}

// defaultLocked returns the default agent (must hold r.mu).
func (r *Registry) defaultLocked() (*agent.Agent, error) {
	// Return "default" agent if exists
	if ag, ok := r.agents["default"]; ok {
		return ag, nil
	}

	// Return first enabled agent
	for id, cfg := range r.configs {
		if cfg.IsEnabled() {
			return r.agents[id], nil
		}
	}

	return nil, fmt.Errorf("no agents available")
}

// Default returns the default agent.
func (r *Registry) Default() (*agent.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultLocked()
}

// List returns all agent configs.
func (r *Registry) List() []*AgentConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make([]*AgentConfig, 0, len(r.configs))
	for _, cfg := range r.configs {
		configs = append(configs, cfg.Clone())
	}
	return configs
}

// ListEnabled returns configs for enabled agents only.
func (r *Registry) ListEnabled() []*AgentConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make([]*AgentConfig, 0, len(r.configs))
	for _, cfg := range r.configs {
		if cfg.IsEnabled() {
			configs = append(configs, cfg.Clone())
		}
	}
	return configs
}

// Create adds a new agent to the registry.
func (r *Registry) Create(ctx context.Context, cfg *AgentConfig) error {
	// Generate ID if not provided
	if cfg.ID == "" {
		cfg.ID = uuid.NewString()[:8]
	}

	// Check for duplicate
	r.mu.RLock()
	_, exists := r.configs[cfg.ID]
	r.mu.RUnlock()
	if exists {
		return ErrAgentExists
	}

	// Apply defaults
	if r.defaults != nil {
		if cfg.Provider == "" {
			cfg.Provider = r.defaults.Provider
		}
		if cfg.Model == "" {
			cfg.Model = r.defaults.Model
		}
		if cfg.APIKey == "" {
			cfg.APIKey = r.defaults.APIKey
		}
		if cfg.Timezone == "" {
			cfg.Timezone = r.defaults.Timezone
		}
	}

	// Set timestamps
	now := time.Now()
	cfg.CreatedAt = now
	cfg.UpdatedAt = now

	// Create agent instance
	ag, err := r.factoryFn(cfg)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// Store config
	if r.store != nil {
		if err := r.store.Save(ctx, cfg); err != nil {
			ag.Close()
			return fmt.Errorf("save agent config: %w", err)
		}
	}

	// Add to registry
	r.mu.Lock()
	r.agents[cfg.ID] = ag
	r.configs[cfg.ID] = cfg.Clone()
	r.mu.Unlock()

	r.logger.Info("agent created", "id", cfg.ID, "name", cfg.Name)
	return nil
}

// Update modifies an existing agent's config.
// Note: This does not recreate the agent, only updates the stored config.
// Call Reload to apply changes to the running agent.
func (r *Registry) Update(ctx context.Context, id string, updates *AgentConfig) error {
	r.mu.Lock()
	cfg, ok := r.configs[id]
	if !ok {
		r.mu.Unlock()
		return ErrAgentNotFound
	}

	// Apply updates
	cfg.Merge(updates)
	cfg.UpdatedAt = time.Now()

	// Clone for storage
	storedCfg := cfg.Clone()
	r.mu.Unlock()

	// Persist changes
	if r.store != nil {
		if err := r.store.Save(ctx, storedCfg); err != nil {
			return fmt.Errorf("save agent config: %w", err)
		}
	}

	r.logger.Info("agent updated", "id", id)
	return nil
}

// Delete removes an agent from the registry.
func (r *Registry) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	ag, ok := r.agents[id]
	if !ok {
		r.mu.Unlock()
		return ErrAgentNotFound
	}

	delete(r.agents, id)
	delete(r.configs, id)
	r.mu.Unlock()

	// Close the agent
	if err := ag.Close(); err != nil {
		r.logger.Error("failed to close agent", "id", id, "error", err)
	}

	// Remove from store
	if r.store != nil {
		if err := r.store.Delete(ctx, id); err != nil {
			return fmt.Errorf("delete agent config: %w", err)
		}
	}

	r.logger.Info("agent deleted", "id", id)
	return nil
}

// Clone duplicates an existing agent with a new ID and name.
func (r *Registry) Clone(ctx context.Context, srcID, newID, newName string) error {
	r.mu.RLock()
	srcCfg, ok := r.configs[srcID]
	if !ok {
		r.mu.RUnlock()
		return ErrAgentNotFound
	}
	clonedCfg := srcCfg.Clone()
	r.mu.RUnlock()

	// Set new identity
	clonedCfg.ID = newID
	clonedCfg.Name = newName
	clonedCfg.CreatedAt = time.Time{} // Will be set by Create
	clonedCfg.UpdatedAt = time.Time{}

	return r.Create(ctx, clonedCfg)
}

// Reload recreates an agent from its current config.
// Use this to apply config changes to a running agent.
func (r *Registry) Reload(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, ok := r.configs[id]
	if !ok {
		return ErrAgentNotFound
	}

	oldAgent := r.agents[id]

	// Create new agent
	newAgent, err := r.factoryFn(cfg)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// Replace agent
	r.agents[id] = newAgent

	// Close old agent
	if oldAgent != nil {
		if err := oldAgent.Close(); err != nil {
			r.logger.Error("failed to close old agent", "id", id, "error", err)
		}
	}

	r.logger.Info("agent reloaded", "id", id)
	return nil
}

// Load loads all agents from the store.
func (r *Registry) Load(ctx context.Context) error {
	if r.store == nil {
		return nil
	}

	configs, err := r.store.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}

	for _, cfg := range configs {
		ag, err := r.factoryFn(cfg)
		if err != nil {
			r.logger.Error("failed to create agent", "id", cfg.ID, "error", err)
			continue
		}

		r.mu.Lock()
		r.agents[cfg.ID] = ag
		r.configs[cfg.ID] = cfg
		r.mu.Unlock()

		r.logger.Info("agent loaded", "id", cfg.ID, "name", cfg.Name)
	}

	return nil
}

// Count returns the number of agents.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// Close closes all agents.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for id, ag := range r.agents {
		if err := ag.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close agent %s: %w", id, err))
		}
	}

	r.agents = make(map[string]*agent.Agent)
	r.configs = make(map[string]*AgentConfig)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing agents: %v", errs)
	}
	return nil
}
