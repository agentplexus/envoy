package agent

import (
	"strings"
	"testing"
	"time"
)

func TestJoinAssistantSegments(t *testing.T) {
	tests := []struct {
		name     string
		segments []string
		want     string
	}{
		{
			name:     "empty",
			segments: nil,
			want:     "",
		},
		{
			name:     "single segment unchanged",
			segments: []string{"The answer is 42."},
			want:     "The answer is 42.",
		},
		{
			name:     "pre-tool text joined with final response",
			segments: []string{"Let me check X.", "The answer is Y."},
			want:     "Let me check X.\n\nThe answer is Y.",
		},
		{
			name:     "three segments across tool turns",
			segments: []string{"First.", "Second.", "Third."},
			want:     "First.\n\nSecond.\n\nThird.",
		},
		{
			name:     "no double spacing when segments already end with newlines",
			segments: []string{"Let me check X.\n\n", "The answer is Y."},
			want:     "Let me check X.\n\nThe answer is Y.",
		},
		{
			name:     "no double spacing when segments begin with newlines",
			segments: []string{"Let me check X.", "\n\nThe answer is Y."},
			want:     "Let me check X.\n\nThe answer is Y.",
		},
		{
			name:     "blank segments skipped",
			segments: []string{"Let me check X.", "   ", "", "The answer is Y."},
			want:     "Let me check X.\n\nThe answer is Y.",
		},
		{
			name:     "internal newlines preserved",
			segments: []string{"Line one.\nLine two.", "Final."},
			want:     "Line one.\nLine two.\n\nFinal.",
		},
		{
			name:     "leading whitespace of first segment preserved",
			segments: []string{"  indented start", "end"},
			want:     "  indented start\n\nend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinAssistantSegments(tt.segments); got != tt.want {
				t.Errorf("joinAssistantSegments(%q) = %q, want %q", tt.segments, got, tt.want)
			}
		})
	}
}

func TestTemporalContext(t *testing.T) {
	// 2026-08-03 01:30 UTC is 2026-08-02 18:30 in Los Angeles — the date
	// must resolve in the configured zone, not the host zone.
	instant := time.Date(2026, 8, 3, 1, 30, 0, 0, time.UTC)

	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	tests := []struct {
		name     string
		loc      *time.Location
		wantDate string
		wantDay  string
		wantZone string
	}{
		{name: "utc", loc: time.UTC, wantDate: "2026-08-03", wantDay: "Monday", wantZone: "UTC"},
		{name: "timezone shifts date", loc: la, wantDate: "2026-08-02", wantDay: "Sunday", wantZone: "America/Los_Angeles"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := temporalContext(instant, tt.loc)
			for _, want := range []string{"## Temporal Context", "Current date: " + tt.wantDate + " (" + tt.wantDay + ")", "Timezone: " + tt.wantZone} {
				if !strings.Contains(got, want) {
					t.Errorf("temporalContext() = %q, missing %q", got, want)
				}
			}
			// Coarse date only — no clock time.
			if strings.Contains(got, ":30") {
				t.Errorf("temporalContext() = %q, must not contain a clock time", got)
			}
		})
	}
}

func TestBuildSystemPrompt_TemporalContext(t *testing.T) {
	a := &Agent{config: Config{SystemPrompt: "You are a helpful assistant."}}

	got := a.buildSystemPromptWithMemories(nil)

	if !strings.HasPrefix(got, "You are a helpful assistant.") {
		t.Errorf("prompt should start with the configured base prompt, got %q", got)
	}
	if !strings.Contains(got, "## Temporal Context") {
		t.Errorf("prompt should contain the temporal context block, got %q", got)
	}
	// The temporal block must be the volatile suffix — after the base prompt.
	if strings.Index(got, "## Temporal Context") < strings.Index(got, "You are a helpful assistant.") {
		t.Errorf("temporal context must come after the stable base prompt, got %q", got)
	}
	// Nil location (zero-value Agent) falls back to UTC.
	if !strings.Contains(got, "Timezone: UTC") {
		t.Errorf("zero-value agent should default to UTC, got %q", got)
	}

	// Empty base prompt still yields date grounding.
	empty := &Agent{}
	if got := empty.buildSystemPromptWithMemories(nil); !strings.HasPrefix(got, "## Temporal Context") {
		t.Errorf("empty base prompt should return the temporal block alone, got %q", got)
	}
}
