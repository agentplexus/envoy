package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/plexusone/omnistorage-core/kvs"
)

const (
	// agentKeyPrefix is the prefix for agent config keys in the KVS store.
	agentKeyPrefix = "agent:config:"
)

// ErrAgentNotFound is returned when an agent is not found.
var ErrAgentNotFound = errors.New("agent not found")

// ErrAgentExists is returned when trying to create an agent that already exists.
var ErrAgentExists = errors.New("agent already exists")

// Store manages persistent agent configuration storage.
type Store struct {
	backend kvs.Store
	cache   map[string]*AgentConfig
	mu      sync.RWMutex
}

// StoreConfig configures the agent store.
type StoreConfig struct {
	// Backend is the KVS storage backend.
	Backend kvs.Store
}

// NewStore creates a new agent config store.
func NewStore(config StoreConfig) *Store {
	return &Store{
		backend: config.Backend,
		cache:   make(map[string]*AgentConfig),
	}
}

// Get retrieves an agent config by ID.
// Returns ErrAgentNotFound if the agent doesn't exist.
func (s *Store) Get(ctx context.Context, id string) (*AgentConfig, error) {
	// Check cache first
	s.mu.RLock()
	if cfg, ok := s.cache[id]; ok {
		s.mu.RUnlock()
		return cfg.Clone(), nil
	}
	s.mu.RUnlock()

	// Load from backend
	key := agentKeyPrefix + id
	data, err := s.backend.Get(ctx, key)
	if err != nil {
		if errors.Is(err, kvs.ErrNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("get agent: %w", err)
	}

	var cfg AgentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal agent: %w", err)
	}

	// Update cache
	s.mu.Lock()
	s.cache[id] = &cfg
	s.mu.Unlock()

	return cfg.Clone(), nil
}

// Save persists an agent config to storage.
func (s *Store) Save(ctx context.Context, cfg *AgentConfig) error {
	if cfg.ID == "" {
		return fmt.Errorf("agent ID is required")
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal agent: %w", err)
	}

	key := agentKeyPrefix + cfg.ID
	// Agent configs don't expire, use 0 TTL
	if err := s.backend.Set(ctx, key, data, 0); err != nil {
		return fmt.Errorf("set agent: %w", err)
	}

	// Update cache
	s.mu.Lock()
	s.cache[cfg.ID] = cfg.Clone()
	s.mu.Unlock()

	return nil
}

// Delete removes an agent config.
func (s *Store) Delete(ctx context.Context, id string) error {
	key := agentKeyPrefix + id
	if err := s.backend.Delete(ctx, key); err != nil {
		if !errors.Is(err, kvs.ErrNotFound) {
			return fmt.Errorf("delete agent: %w", err)
		}
	}

	// Remove from cache
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()

	return nil
}

// List returns all agent configs.
// This requires the backend to implement kvs.ListableStore.
func (s *Store) List(ctx context.Context) ([]*AgentConfig, error) {
	listable, ok := s.backend.(kvs.ListableStore)
	if !ok {
		// Fall back to cached configs if backend doesn't support listing
		s.mu.RLock()
		configs := make([]*AgentConfig, 0, len(s.cache))
		for _, cfg := range s.cache {
			configs = append(configs, cfg.Clone())
		}
		s.mu.RUnlock()
		return configs, nil
	}

	keys, err := listable.List(ctx, agentKeyPrefix)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}

	// Load all configs
	configs := make([]*AgentConfig, 0, len(keys))
	prefixLen := len(agentKeyPrefix)
	for _, key := range keys {
		var id string
		if len(key) > prefixLen {
			id = key[prefixLen:]
		} else {
			id = key
		}

		cfg, err := s.Get(ctx, id)
		if err != nil {
			if errors.Is(err, ErrAgentNotFound) {
				continue // Skip deleted agents
			}
			return nil, err
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}

// ListEnabled returns all enabled agent configs.
func (s *Store) ListEnabled(ctx context.Context) ([]*AgentConfig, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	enabled := make([]*AgentConfig, 0, len(all))
	for _, cfg := range all {
		if cfg.IsEnabled() {
			enabled = append(enabled, cfg)
		}
	}
	return enabled, nil
}

// Exists checks if an agent with the given ID exists.
func (s *Store) Exists(ctx context.Context, id string) (bool, error) {
	_, err := s.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ClearCache removes all cached configs.
// This does not delete configs from the backend.
func (s *Store) ClearCache() {
	s.mu.Lock()
	s.cache = make(map[string]*AgentConfig)
	s.mu.Unlock()
}

// Count returns the number of agent configs.
func (s *Store) Count(ctx context.Context) (int, error) {
	configs, err := s.List(ctx)
	if err != nil {
		return 0, err
	}
	return len(configs), nil
}

// Close closes the underlying backend.
func (s *Store) Close() error {
	return s.backend.Close()
}
