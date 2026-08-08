package agent

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/plexusone/omnillm/provider"
)

// Tool represents an agent tool that can be invoked.
type Tool interface {
	// Name returns the tool name.
	Name() string
	// Description returns a description of what the tool does.
	Description() string
	// Parameters returns the JSON schema for the tool parameters.
	Parameters() map[string]interface{}
	// Execute runs the tool with the given arguments.
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolRegistry manages available tools.
type ToolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewToolRegistry creates a new tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Unregister removes a tool from the registry.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tool names.
func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// GetTools returns tool definitions for the LLM.
func (r *ToolRegistry) GetTools() []provider.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]provider.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, provider.Tool{
			Type: "function",
			Function: provider.ToolSpec{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		})
	}
	return tools
}

// ToolIdentity is an optional interface a Tool can implement to expose its
// provenance — where the tool came from (e.g. which MCP server).
type ToolIdentity interface {
	// ToolSource returns the origin kind, e.g. "mcp" or "skill".
	ToolSource() string
	// ToolSourceName returns the originating container: the MCP server or
	// skill name the tool belongs to.
	ToolSourceName() string
	// ToolSourceToolName returns the tool's original name at its source,
	// before any renaming or namespacing applied at registration.
	ToolSourceToolName() string
}

// ToolDescriptor describes a registered tool, including provenance for
// tools that implement ToolIdentity.
type ToolDescriptor struct {
	Name        string
	Description string
	Parameters  map[string]interface{}

	// Source is the origin kind ("mcp", "skill", …); empty for tools
	// registered directly without provenance.
	Source string
	// SourceName is the originating MCP server / skill name.
	SourceName string
	// SourceTool is the tool's original name at its source.
	SourceTool string
}

// Describe returns descriptors for all registered tools, exposing MCP and
// skill identity where available. Unlike GetTools (which builds provider
// definitions for the LLM), this is the inventory surface for listings.
func (r *ToolRegistry) Describe() []ToolDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	descriptors := make([]ToolDescriptor, 0, len(r.tools))
	for _, tool := range r.tools {
		d := ToolDescriptor{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		}
		if id, ok := tool.(ToolIdentity); ok {
			d.Source = id.ToolSource()
			d.SourceName = id.ToolSourceName()
			d.SourceTool = id.ToolSourceToolName()
		}
		descriptors = append(descriptors, d)
	}
	return descriptors
}

// Execute runs a tool by name with the given arguments.
func (r *ToolRegistry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	tool, ok := r.Get(name)
	if !ok {
		return "", &ToolNotFoundError{Name: name}
	}
	return tool.Execute(ctx, args)
}

// ToolNotFoundError is returned when a tool is not found.
type ToolNotFoundError struct {
	Name string
}

func (e *ToolNotFoundError) Error() string {
	return "tool not found: " + e.Name
}

// BaseTool provides a base implementation for tools.
type BaseTool struct {
	name        string
	description string
	parameters  map[string]interface{}
	handler     func(ctx context.Context, args json.RawMessage) (string, error)
}

// NewBaseTool creates a new base tool.
func NewBaseTool(name, description string, parameters map[string]interface{}, handler func(ctx context.Context, args json.RawMessage) (string, error)) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
		parameters:  parameters,
		handler:     handler,
	}
}

func (t *BaseTool) Name() string                       { return t.name }
func (t *BaseTool) Description() string                { return t.description }
func (t *BaseTool) Parameters() map[string]interface{} { return t.parameters }

func (t *BaseTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return t.handler(ctx, args)
}
