package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// HostFunctions provides sandboxed implementations of host capabilities.
type HostFunctions struct {
	config Config
}

// NewHostFunctions creates host functions with the given configuration.
func NewHostFunctions(config Config) *HostFunctions {
	return &HostFunctions{config: config}
}

// FSRead reads a file if the fs_read capability is granted.
func (h *HostFunctions) FSRead(ctx context.Context, path string) ([]byte, error) {
	if !h.config.HasCapability(CapFSRead) {
		return nil, NewCapabilityError(CapFSRead, "fs_read")
	}

	// Validate path
	absPath, err := h.validatePath(path)
	if err != nil {
		return nil, err
	}

	// Read file with size limit
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	if h.config.MaxOutputBytes > 0 && len(data) > h.config.MaxOutputBytes {
		return data[:h.config.MaxOutputBytes], nil
	}

	return data, nil
}

// FSWrite writes a file if the fs_write capability is granted.
func (h *HostFunctions) FSWrite(ctx context.Context, path string, data []byte) error {
	if !h.config.HasCapability(CapFSWrite) {
		return NewCapabilityError(CapFSWrite, "fs_write")
	}

	// Validate path
	absPath, err := h.validatePath(path)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	return os.WriteFile(absPath, data, 0600) //nolint:gosec // G306: User-configurable file permissions
}

// HTTPFetch makes an HTTP request if the net_http capability is granted.
func (h *HostFunctions) HTTPFetch(ctx context.Context, method, url string, body []byte, headers map[string]string) ([]byte, int, error) {
	if !h.config.HasCapability(CapNetHTTP) {
		return nil, 0, NewCapabilityError(CapNetHTTP, "http_fetch")
	}

	// Validate host if restrictions are configured
	if err := h.validateHost(url); err != nil {
		return nil, 0, err
	}

	// Create request
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	// Add headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Make request with timeout
	client := &http.Client{
		Timeout: h.config.Timeout,
	}

	resp, err := client.Do(req) //nolint:gosec // G704: URL is validated by validateHost
	if err != nil {
		return nil, 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	// Read response with limit
	limitReader := io.LimitReader(resp.Body, int64(h.config.MaxOutputBytes))
	respBody, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// ExecRun executes a command if the exec_run capability is granted.
func (h *HostFunctions) ExecRun(ctx context.Context, command string, args []string) ([]byte, []byte, int, error) {
	if !h.config.HasCapability(CapExecRun) {
		return nil, nil, 0, NewCapabilityError(CapExecRun, "exec_run")
	}

	// Validate command if restrictions are configured
	if err := h.validateCommand(command); err != nil {
		return nil, nil, 0, err
	}

	// Create command with timeout
	cmd := exec.CommandContext(ctx, command, args...)

	// Set process group so we can kill all child processes on timeout
	setProcessGroup(cmd)

	// Set working directory if configured
	if h.config.WorkingDir != "" {
		cmd.Dir = h.config.WorkingDir
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, max: h.config.MaxOutputBytes}
	cmd.Stderr = &limitedWriter{w: &stderr, max: h.config.MaxOutputBytes}

	// Run with timeout
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			// Kill entire process tree on timeout
			killProcessGroup(cmd)
			return stdout.Bytes(), stderr.Bytes(), -1, NewTimeoutError(h.config.Timeout)
		} else {
			return stdout.Bytes(), stderr.Bytes(), -1, fmt.Errorf("exec: %w", err)
		}
	}

	return stdout.Bytes(), stderr.Bytes(), exitCode, nil
}

// validatePath ensures the path is within allowed directories.
func (h *HostFunctions) validatePath(path string) (string, error) {
	// Clean and make absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Resolve symlinks to prevent traversal attacks
	// For non-existent files, resolve the parent directory
	resolvedPath := absPath
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		resolvedPath = resolved
	} else if os.IsNotExist(err) {
		// File doesn't exist yet, resolve parent directory
		parentPath := filepath.Dir(absPath)
		if resolvedParent, parentErr := filepath.EvalSymlinks(parentPath); parentErr == nil {
			resolvedPath = filepath.Join(resolvedParent, filepath.Base(absPath))
		}
	}

	// Check against allowed paths
	allowedPaths := h.config.AllowedPaths
	if len(allowedPaths) == 0 && h.config.WorkingDir != "" {
		allowedPaths = []string{h.config.WorkingDir}
	}

	if len(allowedPaths) > 0 {
		allowed := false
		for _, allowedPath := range allowedPaths {
			// Resolve symlinks in allowed path too
			allowedAbs, _ := filepath.Abs(allowedPath)
			if resolvedAllowed, err := filepath.EvalSymlinks(allowedAbs); err == nil {
				allowedAbs = resolvedAllowed
			}

			// Check if path starts with allowed path
			if strings.HasPrefix(resolvedPath, allowedAbs+string(filepath.Separator)) || resolvedPath == allowedAbs {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", &ExecutionError{
				Kind:    "capability",
				Message: fmt.Sprintf("path %q is outside allowed directories", path),
			}
		}
	}

	return resolvedPath, nil
}

// validateHost ensures the URL host is in the allowed list.
func (h *HostFunctions) validateHost(url string) error {
	if len(h.config.AllowedHosts) == 0 {
		return nil // All hosts allowed
	}

	// Extract host from URL
	for _, allowed := range h.config.AllowedHosts {
		if strings.Contains(url, allowed) {
			return nil
		}
	}

	return &ExecutionError{
		Kind:    "capability",
		Message: fmt.Sprintf("host not in allowed list for URL: %s", url),
	}
}

// validateCommand ensures the command is in the allowed list.
func (h *HostFunctions) validateCommand(command string) error {
	if len(h.config.AllowedCommands) == 0 {
		return &ExecutionError{
			Kind:    "capability",
			Message: "no commands are allowed (AllowedCommands is empty)",
		}
	}

	// Get base name of command
	baseName := filepath.Base(command)

	for _, allowed := range h.config.AllowedCommands {
		if baseName == allowed || command == allowed {
			return nil
		}
	}

	return &ExecutionError{
		Kind:    "capability",
		Message: fmt.Sprintf("command %q is not in allowed list", command),
	}
}

// limitedWriter limits the amount of data written.
type limitedWriter struct {
	w       io.Writer
	max     int
	written int
}

func (w *limitedWriter) Write(p []byte) (n int, err error) {
	if w.max > 0 && w.written >= w.max {
		return len(p), nil // Silently discard
	}

	remaining := w.max - w.written
	if w.max > 0 && len(p) > remaining {
		p = p[:remaining]
	}

	n, err = w.w.Write(p)
	w.written += n
	return n, err
}

// ExecuteCommand is a high-level helper for simple command execution.
func (h *HostFunctions) ExecuteCommand(ctx context.Context, command string, args []string, timeout time.Duration) (*Result, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	start := time.Now()
	stdout, stderr, exitCode, err := h.ExecRun(ctx, command, args)

	return &Result{
		Output:   stdout,
		Error:    stderr,
		ExitCode: exitCode,
		Duration: time.Since(start),
	}, err
}
