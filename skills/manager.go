package skills

import (
	"io/fs"
	"sync"
)

// ManagerConfig configures the skill manager.
type ManagerConfig struct {
	// Packs are embedded skill packs to load.
	Packs []fs.FS

	// Dirs are filesystem directories to search for skills.
	// Directory skills override embedded skills with the same name.
	Dirs []string

	// Includes limits loaded skills to only these names.
	// If empty, all skills are included.
	Includes []string

	// Excludes prevents these skills from being loaded.
	// Applied after includes.
	Excludes []string
}

// Manager handles loading and managing skills from multiple sources.
type Manager struct {
	config  ManagerConfig
	skills  []*Skill
	byName  map[string]*Skill
	mu      sync.RWMutex
	loaded  bool
}

// NewManager creates a new skill manager with the given configuration.
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		config: cfg,
		byName: make(map[string]*Skill),
	}
}

// Load discovers and loads skills from all configured sources.
// Skills from directories override embedded skills with the same name.
// Includes/excludes filters are applied after loading.
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	seen := make(map[string]bool)
	var allSkills []*Skill

	// Load from directories first (they take priority)
	dirs := m.config.Dirs
	if len(dirs) == 0 {
		dirs = DefaultSearchPaths()
	}

	dirSkills, err := Discover(dirs)
	if err != nil {
		return err
	}

	for _, skill := range dirSkills {
		seen[skill.Name] = true
		allSkills = append(allSkills, skill)
	}

	// Load from embedded packs (skip if already loaded from directory)
	for _, pack := range m.config.Packs {
		packSkills, err := DiscoverFromFS(pack, seen)
		if err != nil {
			// Log but don't fail - packs are optional
			continue
		}
		allSkills = append(allSkills, packSkills...)
	}

	// Apply includes filter
	if len(m.config.Includes) > 0 {
		includeSet := make(map[string]bool)
		for _, name := range m.config.Includes {
			includeSet[name] = true
		}

		var filtered []*Skill
		for _, skill := range allSkills {
			if includeSet[skill.Name] {
				filtered = append(filtered, skill)
			}
		}
		allSkills = filtered
	}

	// Apply excludes filter
	if len(m.config.Excludes) > 0 {
		excludeSet := make(map[string]bool)
		for _, name := range m.config.Excludes {
			excludeSet[name] = true
		}

		var filtered []*Skill
		for _, skill := range allSkills {
			if !excludeSet[skill.Name] {
				filtered = append(filtered, skill)
			}
		}
		allSkills = filtered
	}

	// Build index
	m.skills = allSkills
	m.byName = make(map[string]*Skill)
	for _, skill := range allSkills {
		m.byName[skill.Name] = skill
	}
	m.loaded = true

	return nil
}

// All returns all loaded skills.
func (m *Manager) All() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.skills
}

// Available returns skills that have all requirements met.
func (m *Manager) Available() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var available []*Skill
	for _, skill := range m.skills {
		if skill.IsAvailable() {
			available = append(available, skill)
		}
	}
	return available
}

// Get returns a skill by name, or nil if not found.
func (m *Manager) Get(name string) *Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byName[name]
}

// Count returns the total number of loaded skills.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.skills)
}

// AvailableCount returns the number of available skills.
func (m *Manager) AvailableCount() int {
	return len(m.Available())
}
