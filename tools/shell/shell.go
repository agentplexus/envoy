// Package shell provides shell execution tools for omniagent.
package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/plexusone/omniagent/agent"
)

// Tool provides shell command execution capabilities.
type Tool struct {
	workingDir string
	allowlist  []string
	logger     *slog.Logger
}

// Config configures the shell tool.
type Config struct {
	WorkingDir string
	Allowlist  []string
	Logger     *slog.Logger
}

// New creates a new shell tool.
func New(config Config) (*Tool, error) {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &Tool{
		workingDir: config.WorkingDir,
		allowlist:  config.Allowlist,
		logger:     config.Logger,
	}, nil
}

// Name returns the tool name.
func (t *Tool) Name() string {
	return "shell"
}

// Description returns the tool description.
func (t *Tool) Description() string {
	return "Execute shell commands on the system. Use with caution."
}

// Parameters returns the JSON schema for tool parameters.
func (t *Tool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default: 60, max: 300)",
			},
		},
		"required": []string{"command"},
	}
}

// Execute runs the shell command.
func (t *Tool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse parameters: %w", err)
	}

	if params.Command == "" {
		return "", fmt.Errorf("command required")
	}

	// Check allowlist if configured
	if len(t.allowlist) > 0 && !t.isAllowed(params.Command) {
		return "", fmt.Errorf("command not in allowlist")
	}

	// Set timeout
	if params.Timeout == 0 {
		params.Timeout = 60
	}
	if params.Timeout > 300 {
		params.Timeout = 300
	}

	timeout := time.Duration(params.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	t.logger.Info("executing shell command",
		"command", params.Command,
		"timeout", timeout,
		"working_dir", t.workingDir)

	// Create command
	// #nosec G204 - Command execution is intentional; allowlist restricts commands when configured
	cmd := exec.CommandContext(ctx, "sh", "-c", params.Command)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()

	// Build result
	result := strings.Builder{}
	if stdout.Len() > 0 {
		result.WriteString("stdout:\n")
		result.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString("stderr:\n")
		result.WriteString(stderr.String())
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result.String(), fmt.Errorf("command timed out after %v", timeout)
		}
		return result.String(), fmt.Errorf("command failed: %w", err)
	}

	if result.Len() == 0 {
		return "(no output)", nil
	}

	return result.String(), nil
}

// isAllowed checks if a command is in the allowlist.
// Comparisons are case-insensitive to handle mixed-case tool names.
func (t *Tool) isAllowed(command string) bool {
	// Extract the base command (first word)
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}
	baseCmd := strings.ToLower(parts[0])

	for _, allowed := range t.allowlist {
		allowedLower := strings.ToLower(allowed)
		// Support prefix matching with *
		if prefix, found := strings.CutSuffix(allowedLower, "*"); found {
			if strings.HasPrefix(baseCmd, prefix) {
				return true
			}
		} else if baseCmd == allowedLower {
			return true
		}
	}
	return false
}

// Ensure Tool implements agent.Tool interface.
var _ agent.Tool = (*Tool)(nil)
