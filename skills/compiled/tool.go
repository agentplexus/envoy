// Package compiled provides interfaces for compiled Go skills.
//
// This package re-exports types from github.com/plexusone/omniskill/skill
// for backwards compatibility, adding only omniagent-specific extensions
// like StorageAware.
package compiled

import (
	"github.com/plexusone/omniskill/skill"
)

// Tool is an alias for skill.FuncTool.
// Use skill.NewTool() to create new tools.
type Tool = skill.FuncTool

// ToolHandler is an alias for skill.ToolFunc.
type ToolHandler = skill.ToolFunc

// Parameter is an alias for skill.Parameter.
type Parameter = skill.Parameter

// NewTool creates a new tool. This is a convenience wrapper around skill.NewTool.
func NewTool(name, description string, params map[string]Parameter, handler ToolHandler) *Tool {
	return skill.NewTool(name, description, params, handler)
}

// ParametersToJSONSchema converts parameters to JSON Schema format.
// This is a convenience wrapper around skill.ParametersToJSONSchema.
func ParametersToJSONSchema(params map[string]Parameter) map[string]any {
	return skill.ParametersToJSONSchema(params)
}
