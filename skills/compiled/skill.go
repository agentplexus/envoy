// Package compiled provides interfaces for compiled Go skills.
//
// Compiled skills are Go packages that implement the Skill interface,
// providing callable tools that the agent can use. Unlike markdown skills
// (SKILL.md files) which inject instructions into the system prompt,
// compiled skills register actual functions that the LLM can invoke.
//
// # Example Usage
//
//	// Import your skill package
//	import "example.com/myskill"
//
//	skill, _ := myskill.New(myskill.Config{...})
//	agent, _ := agent.New(config,
//	    agent.WithCompiledSkill(skill),
//	)
package compiled

import (
	"context"

	"github.com/plexusone/omnistorage-core/kvs"
)

// Skill is the interface that compiled skills implement.
type Skill interface {
	// Name returns the skill identifier (e.g., "invest", "weather").
	Name() string

	// Description returns a human-readable description of the skill.
	Description() string

	// Tools returns the tools provided by this skill.
	Tools() []Tool

	// Init initializes the skill. Called once when the agent starts.
	Init(ctx context.Context) error

	// Close releases resources. Called when the agent shuts down.
	Close() error
}

// StorageAware is an optional interface for skills that need storage.
// If a skill implements this interface, the agent will inject the storage
// backend after creation and before Init() is called.
type StorageAware interface {
	SetStorage(s kvs.Store)
}

// SkillInfo provides metadata about a skill for introspection.
type SkillInfo struct {
	Name        string
	Description string
	ToolCount   int
	Tools       []ToolInfo
}

// Info returns metadata about the skill.
func Info(s Skill) SkillInfo {
	tools := s.Tools()
	toolInfos := make([]ToolInfo, len(tools))
	for i, t := range tools {
		toolInfos[i] = ToolInfo{
			Name:        t.Name,
			Description: t.Description,
		}
	}

	return SkillInfo{
		Name:        s.Name(),
		Description: s.Description(),
		ToolCount:   len(tools),
		Tools:       toolInfos,
	}
}

// ToolInfo provides metadata about a tool.
type ToolInfo struct {
	Name        string
	Description string
}
