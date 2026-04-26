package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/plexusone/omnistorage-core/kvs"
)

const (
	// sessionKeyPrefix is the prefix for session keys in the KVS store.
	sessionKeyPrefix = "session:"

	// DefaultSessionTTL is the default TTL for sessions (7 days).
	DefaultSessionTTL = 7 * 24 * time.Hour
)

// ErrSessionNotFound is returned when a session is not found.
var ErrSessionNotFound = errors.New("session not found")

// Store manages persistent session storage.
type Store struct {
	backend kvs.Store
	ttl     time.Duration
	cache   map[string]*Session
	mu      sync.RWMutex
}

// StoreConfig configures the session store.
type StoreConfig struct {
	// Backend is the KVS storage backend.
	Backend kvs.Store

	// TTL is the session time-to-live. Zero means no expiration.
	TTL time.Duration
}

// NewStore creates a new session store.
func NewStore(config StoreConfig) *Store {
	ttl := config.TTL
	if ttl == 0 {
		ttl = DefaultSessionTTL
	}

	return &Store{
		backend: config.Backend,
		ttl:     ttl,
		cache:   make(map[string]*Session),
	}
}

// Get retrieves a session by ID.
// If the session doesn't exist, it creates a new one.
func (s *Store) Get(ctx context.Context, id string) (*Session, error) {
	// Check cache first
	s.mu.RLock()
	if session, ok := s.cache[id]; ok {
		s.mu.RUnlock()
		return session, nil
	}
	s.mu.RUnlock()

	// Load from backend
	key := sessionKeyPrefix + id
	data, err := s.backend.Get(ctx, key)
	if err != nil {
		if errors.Is(err, kvs.ErrNotFound) {
			// Create new session
			session := NewSession(id)
			if err := s.Save(ctx, session); err != nil {
				return nil, fmt.Errorf("save new session: %w", err)
			}
			return session, nil
		}
		return nil, fmt.Errorf("get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	// Update cache
	s.mu.Lock()
	s.cache[id] = &session
	s.mu.Unlock()

	return &session, nil
}

// GetIfExists retrieves a session by ID only if it exists.
// Returns ErrSessionNotFound if the session doesn't exist.
func (s *Store) GetIfExists(ctx context.Context, id string) (*Session, error) {
	// Check cache first
	s.mu.RLock()
	if session, ok := s.cache[id]; ok {
		s.mu.RUnlock()
		return session, nil
	}
	s.mu.RUnlock()

	// Load from backend
	key := sessionKeyPrefix + id
	data, err := s.backend.Get(ctx, key)
	if err != nil {
		if errors.Is(err, kvs.ErrNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	// Update cache
	s.mu.Lock()
	s.cache[id] = &session
	s.mu.Unlock()

	return &session, nil
}

// Save persists a session to storage.
func (s *Store) Save(ctx context.Context, session *Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	key := sessionKeyPrefix + session.ID
	if err := s.backend.Set(ctx, key, data, s.ttl); err != nil {
		return fmt.Errorf("set session: %w", err)
	}

	// Update cache
	s.mu.Lock()
	s.cache[session.ID] = session
	s.mu.Unlock()

	return nil
}

// Delete removes a session.
func (s *Store) Delete(ctx context.Context, id string) error {
	key := sessionKeyPrefix + id
	if err := s.backend.Delete(ctx, key); err != nil {
		if !errors.Is(err, kvs.ErrNotFound) {
			return fmt.Errorf("delete session: %w", err)
		}
	}

	// Remove from cache
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()

	return nil
}

// List returns all session IDs.
// This requires the backend to implement kvs.ListableStore.
func (s *Store) List(ctx context.Context) ([]string, error) {
	listable, ok := s.backend.(kvs.ListableStore)
	if !ok {
		// Fall back to cached sessions if backend doesn't support listing
		s.mu.RLock()
		ids := make([]string, 0, len(s.cache))
		for id := range s.cache {
			ids = append(ids, id)
		}
		s.mu.RUnlock()
		return ids, nil
	}

	keys, err := listable.List(ctx, sessionKeyPrefix)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	// Strip prefix from keys
	ids := make([]string, len(keys))
	prefixLen := len(sessionKeyPrefix)
	for i, key := range keys {
		if len(key) > prefixLen {
			ids[i] = key[prefixLen:]
		} else {
			ids[i] = key
		}
	}

	return ids, nil
}

// Touch updates the session's TTL without modifying its content.
func (s *Store) Touch(ctx context.Context, id string) error {
	session, err := s.GetIfExists(ctx, id)
	if err != nil {
		return err
	}
	return s.Save(ctx, session)
}

// Clear removes all cached sessions.
// This does not delete sessions from the backend.
func (s *Store) ClearCache() {
	s.mu.Lock()
	s.cache = make(map[string]*Session)
	s.mu.Unlock()
}

// Close closes the underlying backend.
func (s *Store) Close() error {
	return s.backend.Close()
}
