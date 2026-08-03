package agent

import "testing"

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
