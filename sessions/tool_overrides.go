package sessions

import "slices"

// ToolOverrides holds per-session tool scoping. The agent applies these when
// building the tool set for a session's turns; the shared tool registry is
// never mutated, so concurrent sessions with different overrides get
// independent tool sets.
type ToolOverrides struct {
	// Tools maps individual tool names to enabled (true) or disabled
	// (false). Tools absent from the map keep their default availability.
	// This covers skill-provided tools and built-ins (e.g. web search) —
	// they are all addressed by registered tool name.
	Tools map[string]bool `json:"tools,omitempty"`

	// MCPServers maps MCP server names to enabled/disabled. Disabling a
	// server removes all of its tools for this session.
	MCPServers map[string]bool `json:"mcp_servers,omitempty"`

	// MCPToolsDeny lists denied tool names per MCP server, using the
	// tool's original name on that server.
	MCPToolsDeny map[string][]string `json:"mcp_tools_deny,omitempty"`
}

// IsZero reports whether no overrides are set (including a nil receiver).
func (o *ToolOverrides) IsZero() bool {
	return o == nil || (len(o.Tools) == 0 && len(o.MCPServers) == 0 && len(o.MCPToolsDeny) == 0)
}

// Denies reports whether the overrides deny the given tool. The source
// fields describe the tool's provenance: source is the origin kind (e.g.
// "mcp"), sourceName the originating server/skill, and sourceTool the
// tool's original name at its source. Non-MCP tools pass empty provenance.
func (o *ToolOverrides) Denies(name, source, sourceName, sourceTool string) bool {
	if o.IsZero() {
		return false
	}
	if enabled, ok := o.Tools[name]; ok && !enabled {
		return true
	}
	if source == "mcp" {
		if enabled, ok := o.MCPServers[sourceName]; ok && !enabled {
			return true
		}
		if slices.Contains(o.MCPToolsDeny[sourceName], sourceTool) {
			return true
		}
	}
	return false
}
