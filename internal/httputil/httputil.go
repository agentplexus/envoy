// Package httputil provides HTTP utilities for safe request/response handling.
package httputil

import (
	"fmt"
	"io"
)

// Default limits for response body reads.
const (
	// MaxErrorBodySize is the maximum size for error response bodies (64KB).
	MaxErrorBodySize int64 = 64 * 1024

	// MaxJSONBodySize is the maximum size for JSON response bodies (1MB).
	MaxJSONBodySize int64 = 1024 * 1024

	// MaxFileBodySize is the maximum size for file downloads (100MB).
	MaxFileBodySize int64 = 100 * 1024 * 1024
)

// ReadLimited reads up to limit bytes from r.
// Returns an error if the content exceeds the limit.
func ReadLimited(r io.Reader, limit int64) ([]byte, error) {
	lr := io.LimitReader(r, limit+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response body exceeds %d bytes", limit)
	}
	return data, nil
}

// ReadErrorBody reads an error response body with a safe limit.
// Useful for reading error messages from failed HTTP responses.
func ReadErrorBody(r io.Reader) ([]byte, error) {
	return ReadLimited(r, MaxErrorBodySize)
}

// ReadJSONBody reads a JSON response body with a safe limit.
func ReadJSONBody(r io.Reader) ([]byte, error) {
	return ReadLimited(r, MaxJSONBodySize)
}

// LimitReader wraps io.LimitReader for consistency.
func LimitReader(r io.Reader, n int64) io.Reader {
	return io.LimitReader(r, n)
}
