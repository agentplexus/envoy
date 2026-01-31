package agent

import (
	"sync"
	"time"

	"github.com/agentplexus/omnillm/provider"
)

// Session represents a conversation session.
type Session struct {
	ID        string
	Messages  []provider.Message
	CreatedAt time.Time
	UpdatedAt time.Time
	Metadata  map[string]interface{}
	mu        sync.RWMutex
}

// SessionStore manages conversation sessions.
type SessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewSessionStore creates a new session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
	}
}

// Get retrieves a session by ID, creating one if it doesn't exist.
func (s *SessionStore) Get(id string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		session = &Session{
			ID:        id,
			Messages:  []provider.Message{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata:  make(map[string]interface{}),
		}
		s.sessions[id] = session
	}
	return session
}

// Delete removes a session.
func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// List returns all session IDs.
func (s *SessionStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	return ids
}

// AddMessage adds a message to the session.
func (sess *Session) AddMessage(role provider.Role, content string) {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.Messages = append(sess.Messages, provider.Message{
		Role:    role,
		Content: content,
	})
	sess.UpdatedAt = time.Now()
}

// GetMessages returns all messages in the session.
func (sess *Session) GetMessages() []provider.Message {
	sess.mu.RLock()
	defer sess.mu.RUnlock()

	// Return a copy to prevent external modification
	messages := make([]provider.Message, len(sess.Messages))
	copy(messages, sess.Messages)
	return messages
}

// SetMetadata sets a metadata value.
func (sess *Session) SetMetadata(key string, value interface{}) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.Metadata[key] = value
	sess.UpdatedAt = time.Now()
}

// GetMetadata gets a metadata value.
func (sess *Session) GetMetadata(key string) (interface{}, bool) {
	sess.mu.RLock()
	defer sess.mu.RUnlock()
	v, ok := sess.Metadata[key]
	return v, ok
}

// Clear removes all messages from the session.
func (sess *Session) Clear() {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.Messages = []provider.Message{}
	sess.UpdatedAt = time.Now()
}

// Trim keeps only the last n messages.
func (sess *Session) Trim(n int) {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if len(sess.Messages) > n {
		sess.Messages = sess.Messages[len(sess.Messages)-n:]
	}
	sess.UpdatedAt = time.Now()
}
