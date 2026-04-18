package compiled

import (
	"context"
)

// Tool represents a callable function that the LLM can invoke.
type Tool struct {
	// Name is the tool identifier (e.g., "analyze_stock").
	Name string

	// Description explains what the tool does. This is shown to the LLM.
	Description string

	// Parameters defines the tool's input parameters using JSON Schema.
	Parameters map[string]Parameter

	// Handler is the function that executes the tool.
	Handler ToolHandler
}

// ToolHandler is the function signature for tool implementations.
type ToolHandler func(ctx context.Context, params map[string]any) (any, error)

// Parameter defines a tool parameter.
type Parameter struct {
	// Type is the JSON Schema type: "string", "number", "integer", "boolean", "array", "object".
	Type string `json:"type"`

	// Description explains the parameter. Shown to the LLM.
	Description string `json:"description,omitempty"`

	// Required indicates if this parameter must be provided.
	Required bool `json:"-"`

	// Default is the default value if not provided.
	Default any `json:"default,omitempty"`

	// Enum restricts values to a specific set.
	Enum []any `json:"enum,omitempty"`

	// Items defines the schema for array elements (when Type is "array").
	Items *Parameter `json:"items,omitempty"`

	// Properties defines object properties (when Type is "object").
	Properties map[string]Parameter `json:"properties,omitempty"`
}

// ToJSONSchema converts a tool's parameters to JSON Schema format
// compatible with OpenAI/Anthropic function calling.
func (t *Tool) ToJSONSchema() map[string]any {
	properties := make(map[string]any)
	required := []string{}

	for name, param := range t.Parameters {
		properties[name] = paramToSchema(param)
		if param.Required {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

func paramToSchema(p Parameter) map[string]any {
	schema := map[string]any{
		"type": p.Type,
	}

	if p.Description != "" {
		schema["description"] = p.Description
	}

	if p.Default != nil {
		schema["default"] = p.Default
	}

	if len(p.Enum) > 0 {
		schema["enum"] = p.Enum
	}

	if p.Items != nil {
		schema["items"] = paramToSchema(*p.Items)
	}

	if len(p.Properties) > 0 {
		props := make(map[string]any)
		req := []string{}
		for name, prop := range p.Properties {
			props[name] = paramToSchema(prop)
			if prop.Required {
				req = append(req, name)
			}
		}
		schema["properties"] = props
		if len(req) > 0 {
			schema["required"] = req
		}
	}

	return schema
}
