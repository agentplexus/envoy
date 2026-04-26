// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package mcp provides a compiled.Skill implementation that wraps an MCP server.
//
// This allows external MCP servers to be used as agent skills. The skill
// connects to the MCP server, discovers its tools, and exposes them through
// the compiled.Skill interface.
//
// # Example
//
//	skill := mcp.NewSkill(mcp.Config{
//		Name:    "github",
//		Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"},
//		Env: map[string]string{
//			"GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN"),
//		},
//	})
//
//	agent, err := agent.New(config,
//		agent.WithCompiledSkill(skill),
//	)
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"

	mcpclient "github.com/plexusone/mcpkit/client"
	"github.com/plexusone/omniagent/skills/compiled"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Skill wraps an MCP server as a compiled.Skill.
//
// The skill spawns the MCP server as a subprocess and communicates with it
// via stdio. Tools discovered from the server are exposed through the
// compiled.Skill interface.
type Skill struct {
	config  Config
	client  *mcpclient.Client
	session *mcpclient.Session
	tools   []compiled.Tool
	mu      sync.RWMutex

	// For lazy connect
	connectOnce sync.Once
	connectErr  error
}

// NewSkill creates a new MCP skill with the given configuration.
//
// The skill is not connected until Init() is called (or first tool call
// if LazyConnect is true).
func NewSkill(cfg Config) *Skill {
	cfg.setDefaults()

	return &Skill{
		config: cfg,
	}
}

// Name returns the skill identifier.
func (s *Skill) Name() string {
	return s.config.Name
}

// Description returns a human-readable description of the skill.
func (s *Skill) Description() string {
	return s.config.Description
}

// Tools returns the tools provided by this skill.
//
// If LazyConnect is enabled and the skill hasn't connected yet,
// this returns an empty slice.
func (s *Skill) Tools() []compiled.Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.tools
}

// Init initializes the skill by connecting to the MCP server.
//
// If LazyConnect is false (default), this connects immediately and
// discovers all available tools.
//
// If LazyConnect is true, this returns immediately and connection
// happens on first tool call.
func (s *Skill) Init(ctx context.Context) error {
	if s.config.LazyConnect {
		return nil
	}

	return s.connect(ctx)
}

// Close releases resources and terminates the MCP server.
func (s *Skill) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil {
		err := s.session.Close()
		s.session = nil
		s.client = nil
		s.tools = nil
		return err
	}

	return nil
}

// connect establishes the connection to the MCP server.
func (s *Skill) connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil {
		return nil // Already connected
	}

	// Create client
	s.client = mcpclient.New(s.config.ClientName, s.config.ClientVersion, nil)

	// Create command
	if len(s.config.Command) == 0 {
		return errors.New("mcp: empty command")
	}

	// G204: Command comes from trusted configuration, not user input.
	cmd := exec.CommandContext(ctx, s.config.Command[0], s.config.Command[1:]...) //nolint:gosec

	// Set environment
	if len(s.config.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range s.config.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	// Connect to server
	session, err := s.client.ConnectCommand(ctx, cmd, nil)
	if err != nil {
		return fmt.Errorf("mcp: connect failed: %w", err)
	}

	s.session = session

	// Discover tools
	if err := s.discoverTools(ctx); err != nil {
		_ = session.Close()
		s.session = nil
		return fmt.Errorf("mcp: tool discovery failed: %w", err)
	}

	return nil
}

// ensureConnected ensures the skill is connected, using lazy connect if needed.
func (s *Skill) ensureConnected(ctx context.Context) error {
	s.mu.RLock()
	connected := s.session != nil
	s.mu.RUnlock()

	if connected {
		return nil
	}

	s.connectOnce.Do(func() {
		s.connectErr = s.connect(ctx)
	})

	return s.connectErr
}

// discoverTools fetches tools from the MCP server and converts them.
// Must be called with s.mu held.
func (s *Skill) discoverTools(ctx context.Context) error {
	mcpTools, err := s.session.ListTools(ctx)
	if err != nil {
		return err
	}

	s.tools = make([]compiled.Tool, 0, len(mcpTools))

	for _, mcpTool := range mcpTools {
		tool := s.convertTool(mcpTool)
		s.tools = append(s.tools, tool)
	}

	return nil
}

// convertTool converts an MCP tool to a compiled.Tool.
func (s *Skill) convertTool(mcpTool *mcp.Tool) compiled.Tool {
	params := s.convertInputSchema(mcpTool.InputSchema)

	return compiled.Tool{
		Name:        mcpTool.Name,
		Description: mcpTool.Description,
		Parameters:  params,
		Handler:     s.makeToolHandler(mcpTool.Name),
	}
}

// convertInputSchema converts an MCP tool's input schema to compiled.Parameter map.
func (s *Skill) convertInputSchema(schema any) map[string]compiled.Parameter {
	if schema == nil {
		return nil
	}

	// The schema is typically a map[string]any from JSON unmarshaling
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return nil
	}

	properties, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		return nil
	}

	required := make(map[string]bool)
	if reqSlice, ok := schemaMap["required"].([]any); ok {
		for _, r := range reqSlice {
			if name, ok := r.(string); ok {
				required[name] = true
			}
		}
	}

	params := make(map[string]compiled.Parameter)
	for name, propVal := range properties {
		prop, ok := propVal.(map[string]any)
		if !ok {
			continue
		}

		param := compiled.Parameter{
			Required: required[name],
		}

		if t, ok := prop["type"].(string); ok {
			param.Type = t
		}
		if d, ok := prop["description"].(string); ok {
			param.Description = d
		}
		if def, ok := prop["default"]; ok {
			param.Default = def
		}
		if enumVals, ok := prop["enum"].([]any); ok {
			param.Enum = enumVals
		}

		params[name] = param
	}

	return params
}

// makeToolHandler creates a handler that proxies tool calls to the MCP server.
func (s *Skill) makeToolHandler(toolName string) compiled.ToolHandler {
	return func(ctx context.Context, params map[string]any) (any, error) {
		// Ensure connected (for lazy connect)
		if err := s.ensureConnected(ctx); err != nil {
			return nil, err
		}

		s.mu.RLock()
		session := s.session
		s.mu.RUnlock()

		if session == nil {
			return nil, errors.New("mcp: not connected")
		}

		// Call the MCP tool
		result, err := session.CallTool(ctx, toolName, params)
		if err != nil {
			return nil, fmt.Errorf("mcp: tool call failed: %w", err)
		}

		// Handle error responses
		if result.IsError {
			if len(result.Content) > 0 {
				if text, ok := result.Content[0].(*mcp.TextContent); ok {
					return nil, errors.New(text.Text)
				}
			}
			return nil, errors.New("mcp: tool returned error")
		}

		// Return structured content if available
		if result.StructuredContent != nil {
			// StructuredContent could be json.RawMessage or already unmarshaled
			switch v := result.StructuredContent.(type) {
			case json.RawMessage:
				var structured any
				if err := json.Unmarshal(v, &structured); err == nil {
					return structured, nil
				}
			default:
				// Already a Go type, return as-is
				return v, nil
			}
		}

		// Fall back to text content
		if len(result.Content) > 0 {
			if text, ok := result.Content[0].(*mcp.TextContent); ok {
				return text.Text, nil
			}
		}

		return nil, nil
	}
}

// Verify Skill implements compiled.Skill at compile time.
var _ compiled.Skill = (*Skill)(nil)
