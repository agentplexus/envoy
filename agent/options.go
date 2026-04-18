package agent

import (
	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omniagent/storage"
)

// Option configures the agent.
type Option func(*Agent) error

// WithCompiledSkill registers a compiled skill with the agent.
// Multiple skills can be registered by calling this option multiple times.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithCompiledSkill(investSkill),
//	    agent.WithCompiledSkill(weatherSkill),
//	)
func WithCompiledSkill(skill compiled.Skill) Option {
	return func(a *Agent) error {
		return a.RegisterCompiledSkill(skill)
	}
}

// WithStorage sets the storage backend for the agent.
// Storage is automatically injected into any storage-aware compiled skills.
//
// Example:
//
//	sqliteStorage, _ := sqlite.New(sqlite.Config{Path: "data.db"})
//	agent, err := agent.New(config,
//	    agent.WithStorage(sqliteStorage),
//	    agent.WithCompiledSkill(investSkill),
//	)
func WithStorage(s storage.Storage) Option {
	return func(a *Agent) error {
		a.SetStorage(s)
		return nil
	}
}

// WithTool registers a single tool with the agent.
//
// Example:
//
//	agent, err := agent.New(config,
//	    agent.WithTool(myTool),
//	)
func WithTool(tool Tool) Option {
	return func(a *Agent) error {
		a.RegisterTool(tool)
		return nil
	}
}
