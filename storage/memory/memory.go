// Package memory provides an in-memory storage backend.
//
// This is the default storage backend, suitable for development
// and testing. Data is not persisted across restarts.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/plexusone/omniagent/storage"
)

// Verify interface compliance.
var _ storage.Storage = (*Storage)(nil)

// Storage implements storage.Storage with in-memory maps.
type Storage struct {
	mu      sync.RWMutex
	data    map[string]entry
	closed  bool
	closeCh chan struct{}
}

type entry struct {
	value     []byte
	expiresAt time.Time
}

// New creates a new in-memory storage.
func New() *Storage {
	s := &Storage{
		data:    make(map[string]entry),
		closeCh: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go s.cleanup()

	return s
}

// Get retrieves a value by key.
func (s *Storage) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, storage.ErrClosed
	}

	e, ok := s.data[key]
	if !ok {
		return nil, storage.ErrNotFound
	}

	// Check expiration
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		return nil, storage.ErrNotFound
	}

	// Return a copy to prevent mutation
	result := make([]byte, len(e.value))
	copy(result, e.value)
	return result, nil
}

// Set stores a value with an optional TTL.
func (s *Storage) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return storage.ErrClosed
	}

	// Copy value to prevent external mutation
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	e := entry{value: valueCopy}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}

	s.data[key] = e
	return nil
}

// Delete removes a key.
func (s *Storage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return storage.ErrClosed
	}

	delete(s.data, key)
	return nil
}

// Close releases storage resources.
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	close(s.closeCh)
	s.data = nil
	return nil
}

// cleanup periodically removes expired entries.
func (s *Storage) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.closeCh:
			return
		case <-ticker.C:
			s.removeExpired()
		}
	}
}

func (s *Storage) removeExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	now := time.Now()
	for key, e := range s.data {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			delete(s.data, key)
		}
	}
}

// Len returns the number of entries (for testing).
func (s *Storage) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}
