// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package profiles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ProgressDetailMode controls how much detail is shown during tool execution.
type ProgressDetailMode int

const (
	// ProgressModeQuiet shows no progress output.
	ProgressModeQuiet ProgressDetailMode = iota

	// ProgressModeMinimal shows only tool names and completion status.
	ProgressModeMinimal

	// ProgressModeNormal shows tool names, parameters, and results summary.
	ProgressModeNormal

	// ProgressModeVerbose shows full details including parameters and results.
	ProgressModeVerbose

	// ProgressModeDebug shows all details plus timing and internal state.
	ProgressModeDebug
)

// String returns the string representation of the progress mode.
func (m ProgressDetailMode) String() string {
	switch m {
	case ProgressModeQuiet:
		return "quiet"
	case ProgressModeMinimal:
		return "minimal"
	case ProgressModeNormal:
		return "normal"
	case ProgressModeVerbose:
		return "verbose"
	case ProgressModeDebug:
		return "debug"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

// ParseProgressMode parses a string into a ProgressDetailMode.
func ParseProgressMode(s string) (ProgressDetailMode, error) {
	switch strings.ToLower(s) {
	case "quiet", "silent", "none":
		return ProgressModeQuiet, nil
	case "minimal", "min":
		return ProgressModeMinimal, nil
	case "normal", "default":
		return ProgressModeNormal, nil
	case "verbose", "full":
		return ProgressModeVerbose, nil
	case "debug", "trace":
		return ProgressModeDebug, nil
	default:
		return ProgressModeNormal, fmt.Errorf("unknown progress mode: %q", s)
	}
}

// ToolProgress represents progress information for a tool execution.
type ToolProgress struct {
	// ToolName is the name of the tool being executed.
	ToolName string

	// Status is the current execution status.
	Status ToolProgressStatus

	// StartTime is when the tool started executing.
	StartTime time.Time

	// EndTime is when the tool finished executing.
	EndTime time.Time

	// Parameters are the tool input parameters (may be redacted).
	Parameters map[string]any

	// Result is the tool output (may be truncated).
	Result string

	// Error is any error that occurred.
	Error string

	// BytesProcessed is the number of bytes processed (for streaming tools).
	BytesProcessed int64

	// Progress is the completion percentage (0-100) if known.
	Progress int

	// Message is a human-readable status message.
	Message string
}

// ToolProgressStatus represents the status of a tool execution.
type ToolProgressStatus string

const (
	ToolProgressPending   ToolProgressStatus = "pending"
	ToolProgressRunning   ToolProgressStatus = "running"
	ToolProgressCompleted ToolProgressStatus = "completed"
	ToolProgressFailed    ToolProgressStatus = "failed"
	ToolProgressCancelled ToolProgressStatus = "cancelled"
)

// Duration returns the execution duration.
func (p *ToolProgress) Duration() time.Duration {
	if p.EndTime.IsZero() {
		return time.Since(p.StartTime)
	}
	return p.EndTime.Sub(p.StartTime)
}

// ProgressReporter handles reporting tool execution progress.
type ProgressReporter struct {
	mode      ProgressDetailMode
	output    io.Writer
	logger    *slog.Logger
	mu        sync.Mutex
	active    map[string]*ToolProgress
	callbacks []ProgressCallback
}

// ProgressCallback is called when tool progress changes.
type ProgressCallback func(progress *ToolProgress)

// NewProgressReporter creates a new progress reporter.
func NewProgressReporter(mode ProgressDetailMode, output io.Writer) *ProgressReporter {
	return &ProgressReporter{
		mode:   mode,
		output: output,
		logger: slog.Default(),
		active: make(map[string]*ToolProgress),
	}
}

// SetMode changes the progress detail mode.
func (r *ProgressReporter) SetMode(mode ProgressDetailMode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mode = mode
}

// Mode returns the current progress detail mode.
func (r *ProgressReporter) Mode() ProgressDetailMode {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mode
}

// SetLogger sets the logger for the reporter.
func (r *ProgressReporter) SetLogger(logger *slog.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = logger
}

// OnProgress registers a callback for progress updates.
func (r *ProgressReporter) OnProgress(cb ProgressCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks = append(r.callbacks, cb)
}

// Start reports that a tool has started executing.
func (r *ProgressReporter) Start(ctx context.Context, toolName string, params map[string]any) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate unique ID for this execution
	id := fmt.Sprintf("%s-%d", toolName, time.Now().UnixNano())

	progress := &ToolProgress{
		ToolName:   toolName,
		Status:     ToolProgressRunning,
		StartTime:  time.Now(),
		Parameters: r.redactParams(params),
		Message:    "Starting execution",
	}

	r.active[id] = progress
	r.report(progress)
	r.notifyCallbacks(progress)

	return id
}

// Update reports progress on an ongoing tool execution.
func (r *ProgressReporter) Update(id string, percentComplete int, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	progress, ok := r.active[id]
	if !ok {
		return
	}

	progress.Progress = percentComplete
	progress.Message = message
	r.report(progress)
	r.notifyCallbacks(progress)
}

// Complete reports that a tool has finished executing.
func (r *ProgressReporter) Complete(id string, result string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	progress, ok := r.active[id]
	if !ok {
		return
	}

	progress.EndTime = time.Now()
	progress.Result = r.truncateResult(result)
	progress.Progress = 100

	if err != nil {
		progress.Status = ToolProgressFailed
		progress.Error = err.Error()
		progress.Message = "Execution failed"
	} else {
		progress.Status = ToolProgressCompleted
		progress.Message = "Execution completed"
	}

	r.report(progress)
	r.notifyCallbacks(progress)

	delete(r.active, id)
}

// Cancel reports that a tool execution was cancelled.
func (r *ProgressReporter) Cancel(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	progress, ok := r.active[id]
	if !ok {
		return
	}

	progress.EndTime = time.Now()
	progress.Status = ToolProgressCancelled
	progress.Message = "Execution cancelled"

	r.report(progress)
	r.notifyCallbacks(progress)

	delete(r.active, id)
}

// Active returns the currently active tool executions.
func (r *ProgressReporter) Active() []*ToolProgress {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]*ToolProgress, 0, len(r.active))
	for _, p := range r.active {
		result = append(result, p)
	}
	return result
}

// report outputs progress based on the current mode.
func (r *ProgressReporter) report(p *ToolProgress) {
	if r.mode == ProgressModeQuiet || r.output == nil {
		return
	}

	var msg string

	switch r.mode {
	case ProgressModeMinimal:
		msg = r.formatMinimal(p)
	case ProgressModeNormal:
		msg = r.formatNormal(p)
	case ProgressModeVerbose:
		msg = r.formatVerbose(p)
	case ProgressModeDebug:
		msg = r.formatDebug(p)
	}

	if msg != "" {
		fmt.Fprintln(r.output, msg)
	}

	// Also log at appropriate level
	if r.mode >= ProgressModeDebug {
		r.logger.Debug("tool progress",
			"tool", p.ToolName,
			"status", p.Status,
			"duration", p.Duration(),
		)
	}
}

func (r *ProgressReporter) formatMinimal(p *ToolProgress) string {
	switch p.Status {
	case ToolProgressRunning:
		return fmt.Sprintf("→ %s...", p.ToolName)
	case ToolProgressCompleted:
		return fmt.Sprintf("✓ %s (%.2fs)", p.ToolName, p.Duration().Seconds())
	case ToolProgressFailed:
		return fmt.Sprintf("✗ %s (error)", p.ToolName)
	case ToolProgressCancelled:
		return fmt.Sprintf("⊘ %s (cancelled)", p.ToolName)
	default:
		return ""
	}
}

func (r *ProgressReporter) formatNormal(p *ToolProgress) string {
	switch p.Status {
	case ToolProgressRunning:
		params := r.formatParamsSummary(p.Parameters)
		if params != "" {
			return fmt.Sprintf("→ %s: %s", p.ToolName, params)
		}
		return fmt.Sprintf("→ %s", p.ToolName)
	case ToolProgressCompleted:
		result := r.summarizeResult(p.Result)
		if result != "" {
			return fmt.Sprintf("✓ %s: %s (%.2fs)", p.ToolName, result, p.Duration().Seconds())
		}
		return fmt.Sprintf("✓ %s (%.2fs)", p.ToolName, p.Duration().Seconds())
	case ToolProgressFailed:
		return fmt.Sprintf("✗ %s: %s", p.ToolName, p.Error)
	case ToolProgressCancelled:
		return fmt.Sprintf("⊘ %s: cancelled", p.ToolName)
	default:
		return ""
	}
}

func (r *ProgressReporter) formatVerbose(p *ToolProgress) string {
	var sb strings.Builder

	switch p.Status {
	case ToolProgressRunning:
		sb.WriteString(fmt.Sprintf("→ Executing: %s\n", p.ToolName))
		if len(p.Parameters) > 0 {
			paramsJSON, _ := json.MarshalIndent(p.Parameters, "  ", "  ")
			sb.WriteString(fmt.Sprintf("  Parameters: %s\n", string(paramsJSON)))
		}
	case ToolProgressCompleted:
		sb.WriteString(fmt.Sprintf("✓ Completed: %s (%.2fs)\n", p.ToolName, p.Duration().Seconds()))
		if p.Result != "" {
			sb.WriteString(fmt.Sprintf("  Result: %s\n", p.Result))
		}
	case ToolProgressFailed:
		sb.WriteString(fmt.Sprintf("✗ Failed: %s (%.2fs)\n", p.ToolName, p.Duration().Seconds()))
		sb.WriteString(fmt.Sprintf("  Error: %s\n", p.Error))
	case ToolProgressCancelled:
		sb.WriteString(fmt.Sprintf("⊘ Cancelled: %s (%.2fs)\n", p.ToolName, p.Duration().Seconds()))
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

func (r *ProgressReporter) formatDebug(p *ToolProgress) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("[%s] Tool: %s\n", p.Status, p.ToolName))
	sb.WriteString(fmt.Sprintf("  Started: %s\n", p.StartTime.Format(time.RFC3339Nano)))

	if !p.EndTime.IsZero() {
		sb.WriteString(fmt.Sprintf("  Ended: %s\n", p.EndTime.Format(time.RFC3339Nano)))
		sb.WriteString(fmt.Sprintf("  Duration: %s\n", p.Duration()))
	}

	if len(p.Parameters) > 0 {
		paramsJSON, _ := json.MarshalIndent(p.Parameters, "  ", "  ")
		sb.WriteString(fmt.Sprintf("  Parameters: %s\n", string(paramsJSON)))
	}

	if p.Progress > 0 && p.Progress < 100 {
		sb.WriteString(fmt.Sprintf("  Progress: %d%%\n", p.Progress))
	}

	if p.BytesProcessed > 0 {
		sb.WriteString(fmt.Sprintf("  Bytes Processed: %d\n", p.BytesProcessed))
	}

	if p.Result != "" {
		sb.WriteString(fmt.Sprintf("  Result: %s\n", p.Result))
	}

	if p.Error != "" {
		sb.WriteString(fmt.Sprintf("  Error: %s\n", p.Error))
	}

	sb.WriteString(fmt.Sprintf("  Message: %s\n", p.Message))

	return strings.TrimSuffix(sb.String(), "\n")
}

func (r *ProgressReporter) formatParamsSummary(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}

	var parts []string
	for k, v := range params {
		switch val := v.(type) {
		case string:
			if len(val) > 30 {
				val = val[:30] + "..."
			}
			parts = append(parts, fmt.Sprintf("%s=%q", k, val))
		default:
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		if len(parts) >= 3 {
			parts = append(parts, "...")
			break
		}
	}
	return strings.Join(parts, ", ")
}

func (r *ProgressReporter) summarizeResult(result string) string {
	if result == "" {
		return ""
	}

	// Count lines
	lines := strings.Count(result, "\n") + 1
	if lines > 1 {
		return fmt.Sprintf("(%d lines)", lines)
	}

	// Truncate long single-line results
	if len(result) > 50 {
		return result[:50] + "..."
	}

	return result
}

func (r *ProgressReporter) redactParams(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}

	// List of parameter names that should be redacted
	sensitiveKeys := map[string]bool{
		"password": true,
		"secret":   true,
		"token":    true,
		"api_key":  true,
		"apiKey":   true,
		"auth":     true,
	}

	redacted := make(map[string]any, len(params))
	for k, v := range params {
		if sensitiveKeys[strings.ToLower(k)] {
			redacted[k] = "[REDACTED]"
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

func (r *ProgressReporter) truncateResult(result string) string {
	const maxLen = 1000
	if len(result) <= maxLen {
		return result
	}
	return result[:maxLen] + fmt.Sprintf("... (%d more bytes)", len(result)-maxLen)
}

func (r *ProgressReporter) notifyCallbacks(progress *ToolProgress) {
	for _, cb := range r.callbacks {
		cb(progress)
	}
}

// ProgressConfig configures progress reporting behavior.
type ProgressConfig struct {
	// Mode is the detail level for progress output.
	Mode ProgressDetailMode

	// Output is where progress is written (default: os.Stderr).
	Output io.Writer

	// ToolModes allows per-tool mode overrides.
	ToolModes map[string]ProgressDetailMode

	// Callbacks receive progress updates.
	Callbacks []ProgressCallback
}

// GetModeForTool returns the progress mode for a specific tool.
func (c *ProgressConfig) GetModeForTool(toolName string) ProgressDetailMode {
	if c.ToolModes != nil {
		if mode, ok := c.ToolModes[toolName]; ok {
			return mode
		}
	}
	return c.Mode
}
