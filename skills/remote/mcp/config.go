// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

// Config configures an MCP skill.
type Config struct {
	// Name is the skill identifier (e.g., "github").
	// This must be unique among all skills registered with an agent.
	Name string

	// Description is a human-readable description of the skill.
	// If empty, defaults to "MCP skill: <name>".
	Description string

	// Command to spawn the MCP server.
	// Example: []string{"npx", "-y", "@modelcontextprotocol/server-github"}
	Command []string

	// Env sets environment variables for the command.
	// These are merged with the current environment.
	// Example: map[string]string{"GITHUB_TOKEN": "xxx"}
	Env map[string]string

	// RequiredEnv names the environment variables this MCP server cannot
	// start without. If any is absent from Env after SetSecrets injection,
	// the skill is excluded at registration (RMI-OMNIAGENT-210) instead of
	// failing however the subprocess reacts to an incomplete environment.
	RequiredEnv []string

	// LazyConnect defers connection until first tool call (default: false).
	// When true, Init() returns immediately and connection happens on first use.
	// When false, Init() connects and discovers tools synchronously.
	LazyConnect bool

	// ClientName identifies this client to the MCP server.
	// If empty, defaults to "omniagent".
	ClientName string

	// ClientVersion identifies this client version to the MCP server.
	// If empty, defaults to "v1.0.0".
	ClientVersion string
}

// setDefaults applies default values to the config.
func (c *Config) setDefaults() {
	if c.Description == "" {
		c.Description = "MCP skill: " + c.Name
	}
	if c.ClientName == "" {
		c.ClientName = "omniagent"
	}
	if c.ClientVersion == "" {
		c.ClientVersion = "v1.0.0"
	}
}
