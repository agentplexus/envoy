package context

import (
	"errors"
	"fmt"
	"testing"
)

func TestCompactionError_Error(t *testing.T) {
	tests := []struct {
		name   string
		err    *CompactionError
		wantIn string
	}{
		{
			name:   "with reason only",
			err:    NewCompactionError("summarizer not configured", nil),
			wantIn: "summarizer not configured",
		},
		{
			name:   "with cause",
			err:    NewCompactionError("LLM call failed", fmt.Errorf("timeout")),
			wantIn: "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			if msg == "" {
				t.Error("Error() returned empty string")
			}
			if len(tt.wantIn) > 0 && !containsString(msg, tt.wantIn) {
				t.Errorf("Error() = %q, want to contain %q", msg, tt.wantIn)
			}
		})
	}
}

func TestCompactionError_Is(t *testing.T) {
	err := NewCompactionError("test", nil)

	if !errors.Is(err, ErrCompactionFailed) {
		t.Error("CompactionError should match ErrCompactionFailed")
	}

	if errors.Is(err, fmt.Errorf("other")) {
		t.Error("CompactionError should not match arbitrary errors")
	}
}

func TestCompactionError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("underlying cause")
	err := NewCompactionError("test", cause)

	if !errors.Is(err, cause) {
		t.Error("CompactionError should unwrap to cause")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
