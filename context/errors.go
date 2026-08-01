package context

import "errors"

// ErrCompactionFailed indicates that context summarization failed.
// This is a sentinel error that can be checked with errors.Is().
var ErrCompactionFailed = errors.New("compaction failed")

// CompactionError wraps a compaction failure with additional context.
type CompactionError struct {
	// Reason describes why compaction failed.
	Reason string
	// Cause is the underlying error, if any.
	Cause error
}

// Error implements the error interface.
func (e *CompactionError) Error() string {
	if e.Cause != nil {
		return "compaction failed: " + e.Reason + ": " + e.Cause.Error()
	}
	return "compaction failed: " + e.Reason
}

// Unwrap returns the underlying cause for errors.Is/As compatibility.
func (e *CompactionError) Unwrap() error {
	return e.Cause
}

// Is reports whether this error matches the target.
func (e *CompactionError) Is(target error) bool {
	return target == ErrCompactionFailed
}

// NewCompactionError creates a CompactionError with the given reason and cause.
func NewCompactionError(reason string, cause error) *CompactionError {
	return &CompactionError{
		Reason: reason,
		Cause:  cause,
	}
}
