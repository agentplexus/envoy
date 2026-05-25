// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package profiles

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestProgressDetailMode_String(t *testing.T) {
	tests := []struct {
		mode     ProgressDetailMode
		expected string
	}{
		{ProgressModeQuiet, "quiet"},
		{ProgressModeMinimal, "minimal"},
		{ProgressModeNormal, "normal"},
		{ProgressModeVerbose, "verbose"},
		{ProgressModeDebug, "debug"},
		{ProgressDetailMode(99), "unknown(99)"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if got := tc.mode.String(); got != tc.expected {
				t.Errorf("String() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestParseProgressMode(t *testing.T) {
	tests := []struct {
		input    string
		expected ProgressDetailMode
		wantErr  bool
	}{
		{"quiet", ProgressModeQuiet, false},
		{"silent", ProgressModeQuiet, false},
		{"minimal", ProgressModeMinimal, false},
		{"normal", ProgressModeNormal, false},
		{"default", ProgressModeNormal, false},
		{"verbose", ProgressModeVerbose, false},
		{"full", ProgressModeVerbose, false},
		{"debug", ProgressModeDebug, false},
		{"trace", ProgressModeDebug, false},
		{"QUIET", ProgressModeQuiet, false}, // case insensitive
		{"invalid", ProgressModeNormal, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseProgressMode(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseProgressMode(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
				return
			}
			if !tc.wantErr && got != tc.expected {
				t.Errorf("ParseProgressMode(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestToolProgress_Duration(t *testing.T) {
	p := &ToolProgress{
		StartTime: time.Now().Add(-5 * time.Second),
	}

	// Ongoing execution
	dur := p.Duration()
	if dur < 5*time.Second {
		t.Errorf("Duration should be >= 5s, got %v", dur)
	}

	// Completed execution
	p.EndTime = p.StartTime.Add(3 * time.Second)
	dur = p.Duration()
	if dur != 3*time.Second {
		t.Errorf("Duration should be 3s, got %v", dur)
	}
}

func TestProgressReporter_StartComplete(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewProgressReporter(ProgressModeMinimal, &buf)

	ctx := context.Background()
	params := map[string]any{"query": "test"}

	// Start
	id := reporter.Start(ctx, "search", params)
	if id == "" {
		t.Error("Start should return an ID")
	}

	active := reporter.Active()
	if len(active) != 1 {
		t.Errorf("Expected 1 active, got %d", len(active))
	}

	// Complete
	reporter.Complete(id, "found 5 results", nil)

	active = reporter.Active()
	if len(active) != 0 {
		t.Errorf("Expected 0 active after complete, got %d", len(active))
	}

	// Check output
	output := buf.String()
	if !strings.Contains(output, "search") {
		t.Error("Output should contain tool name")
	}
}

func TestProgressReporter_CompleteWithError(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewProgressReporter(ProgressModeNormal, &buf)

	ctx := context.Background()
	id := reporter.Start(ctx, "shell", nil)
	reporter.Complete(id, "", errors.New("command failed"))

	output := buf.String()
	if !strings.Contains(output, "command failed") {
		t.Error("Output should contain error message")
	}
}

func TestProgressReporter_Update(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewProgressReporter(ProgressModeVerbose, &buf)

	ctx := context.Background()
	id := reporter.Start(ctx, "download", nil)

	reporter.Update(id, 50, "Downloading...")

	active := reporter.Active()
	if len(active) != 1 {
		t.Fatal("Expected 1 active")
	}
	if active[0].Progress != 50 {
		t.Errorf("Expected progress 50, got %d", active[0].Progress)
	}

	reporter.Complete(id, "done", nil)
}

func TestProgressReporter_Cancel(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewProgressReporter(ProgressModeMinimal, &buf)

	ctx := context.Background()
	id := reporter.Start(ctx, "long-task", nil)
	reporter.Cancel(id)

	active := reporter.Active()
	if len(active) != 0 {
		t.Errorf("Expected 0 active after cancel, got %d", len(active))
	}

	output := buf.String()
	if !strings.Contains(output, "cancelled") {
		t.Error("Output should indicate cancellation")
	}
}

func TestProgressReporter_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewProgressReporter(ProgressModeQuiet, &buf)

	ctx := context.Background()
	id := reporter.Start(ctx, "test", nil)
	reporter.Complete(id, "result", nil)

	if buf.Len() != 0 {
		t.Errorf("Quiet mode should produce no output, got: %s", buf.String())
	}
}

func TestProgressReporter_Callbacks(t *testing.T) {
	reporter := NewProgressReporter(ProgressModeQuiet, nil)

	var events []string
	reporter.OnProgress(func(p *ToolProgress) {
		events = append(events, string(p.Status))
	})

	ctx := context.Background()
	id := reporter.Start(ctx, "test", nil)
	reporter.Update(id, 50, "half")
	reporter.Complete(id, "", nil)

	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d: %v", len(events), events)
	}

	expected := []string{"running", "running", "completed"}
	for i, e := range expected {
		if events[i] != e {
			t.Errorf("Event %d: expected %q, got %q", i, e, events[i])
		}
	}
}

func TestProgressReporter_SetMode(t *testing.T) {
	reporter := NewProgressReporter(ProgressModeQuiet, nil)

	if reporter.Mode() != ProgressModeQuiet {
		t.Error("Initial mode should be quiet")
	}

	reporter.SetMode(ProgressModeVerbose)

	if reporter.Mode() != ProgressModeVerbose {
		t.Error("Mode should be verbose after SetMode")
	}
}

func TestProgressReporter_RedactParams(t *testing.T) {
	reporter := NewProgressReporter(ProgressModeQuiet, nil)

	params := map[string]any{
		"query":    "test",
		"password": "secret123",
		"api_key":  "abc123",
		"data":     "public",
	}

	redacted := reporter.redactParams(params)

	if redacted["query"] != "test" {
		t.Error("Non-sensitive param should not be redacted")
	}
	if redacted["password"] != "[REDACTED]" {
		t.Error("password should be redacted")
	}
	if redacted["api_key"] != "[REDACTED]" {
		t.Error("api_key should be redacted")
	}
}

func TestProgressReporter_TruncateResult(t *testing.T) {
	reporter := NewProgressReporter(ProgressModeQuiet, nil)

	// Short result
	short := "short result"
	if reporter.truncateResult(short) != short {
		t.Error("Short result should not be truncated")
	}

	// Long result
	long := strings.Repeat("x", 2000)
	truncated := reporter.truncateResult(long)
	if len(truncated) >= len(long) {
		t.Error("Long result should be truncated")
	}
	if !strings.Contains(truncated, "more bytes") {
		t.Error("Truncated result should indicate remaining bytes")
	}
}

func TestProgressConfig_GetModeForTool(t *testing.T) {
	config := &ProgressConfig{
		Mode: ProgressModeNormal,
		ToolModes: map[string]ProgressDetailMode{
			"shell":   ProgressModeDebug,
			"browser": ProgressModeQuiet,
		},
	}

	// Tool-specific mode
	if config.GetModeForTool("shell") != ProgressModeDebug {
		t.Error("shell should use debug mode")
	}

	// Default mode
	if config.GetModeForTool("search") != ProgressModeNormal {
		t.Error("search should use default normal mode")
	}
}

func TestProgressReporter_FormatModes(t *testing.T) {
	progress := &ToolProgress{
		ToolName:  "test-tool",
		Status:    ToolProgressCompleted,
		StartTime: time.Now().Add(-2 * time.Second),
		EndTime:   time.Now(),
		Parameters: map[string]any{
			"query": "hello",
		},
		Result: "success",
	}

	// Test minimal format
	var buf bytes.Buffer
	reporter := NewProgressReporter(ProgressModeMinimal, &buf)
	reporter.report(progress)
	if !strings.Contains(buf.String(), "✓") {
		t.Error("Minimal format should include checkmark for completed")
	}

	// Test normal format
	buf.Reset()
	reporter.SetMode(ProgressModeNormal)
	reporter.report(progress)
	if !strings.Contains(buf.String(), "test-tool") {
		t.Error("Normal format should include tool name")
	}

	// Test verbose format
	buf.Reset()
	reporter.SetMode(ProgressModeVerbose)
	reporter.report(progress)
	output := buf.String()
	if !strings.Contains(output, "Completed") {
		t.Error("Verbose format should include Completed")
	}

	// Test debug format
	buf.Reset()
	reporter.SetMode(ProgressModeDebug)
	reporter.report(progress)
	output = buf.String()
	if !strings.Contains(output, "Started:") {
		t.Error("Debug format should include Started timestamp")
	}
}
