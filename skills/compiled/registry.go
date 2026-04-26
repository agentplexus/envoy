package compiled

import (
	"context"
	"fmt"
	"sync"

	"github.com/plexusone/omnistorage"
)

// Registry manages compiled skills.
type Registry struct {
	mu     sync.RWMutex
	skills map[string]Skill
}

// NewRegistry creates a new skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]Skill),
	}
}

// Register adds a skill to the registry.
func (r *Registry) Register(s Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := s.Name()
	if _, exists := r.skills[name]; exists {
		return fmt.Errorf("skill %q already registered", name)
	}

	r.skills[name] = s
	return nil
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.skills[name]
	return s, ok
}

// All returns all registered skills.
func (r *Registry) All() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		skills = append(skills, s)
	}
	return skills
}

// AllTools returns all tools from all registered skills.
func (r *Registry) AllTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []Tool
	for _, s := range r.skills {
		tools = append(tools, s.Tools()...)
	}
	return tools
}

// FindTool finds a tool by name across all skills.
func (r *Registry) FindTool(name string) (*Tool, Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, s := range r.skills {
		for _, t := range s.Tools() {
			if t.Name == name {
				return &t, s, true
			}
		}
	}
	return nil, nil, false
}

// InitAll initializes all registered skills.
func (r *Registry) InitAll(ctx context.Context) error {
	r.mu.RLock()
	skills := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		skills = append(skills, s)
	}
	r.mu.RUnlock()

	for _, s := range skills {
		if err := s.Init(ctx); err != nil {
			return fmt.Errorf("init skill %q: %w", s.Name(), err)
		}
	}
	return nil
}

// CloseAll closes all registered skills.
func (r *Registry) CloseAll() error {
	r.mu.RLock()
	skills := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		skills = append(skills, s)
	}
	r.mu.RUnlock()

	var lastErr error
	for _, s := range skills {
		if err := s.Close(); err != nil {
			lastErr = fmt.Errorf("close skill %q: %w", s.Name(), err)
		}
	}
	return lastErr
}

// InjectStorage injects storage into all storage-aware skills.
func (r *Registry) InjectStorage(storage omnistorage.Store) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, s := range r.skills {
		if sa, ok := s.(StorageAware); ok {
			sa.SetStorage(storage)
		}
	}
}

// Count returns the number of registered skills.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.skills)
}

// ToolCount returns the total number of tools across all skills.
func (r *Registry) ToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, s := range r.skills {
		count += len(s.Tools())
	}
	return count
}
