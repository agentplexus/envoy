// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/plexusone/omniagent/api/openai/operations"
	"github.com/plexusone/omnistorage-core/kvs"
)

const (
	conversationKeyPrefix = "conversation:"
)

// ConversationStore manages conversation persistence.
type ConversationStore struct {
	backend kvs.Store
	cache   map[string]map[string]operations.Conversation // userID -> convID -> conversation
	mu      sync.RWMutex
	ttl     time.Duration
}

// ConversationStoreConfig configures the conversation store.
type ConversationStoreConfig struct {
	// Backend is the KVS storage backend. If nil, uses in-memory only.
	Backend kvs.Store
	// TTL is the conversation time-to-live. Zero means 30 days default.
	TTL time.Duration
}

// NewConversationStore creates a new conversation store.
func NewConversationStore(config ConversationStoreConfig) *ConversationStore {
	ttl := config.TTL
	if ttl == 0 {
		ttl = 30 * 24 * time.Hour // 30 days default
	}

	return &ConversationStore{
		backend: config.Backend,
		cache:   make(map[string]map[string]operations.Conversation),
		ttl:     ttl,
	}
}

// makeKey creates a storage key for a conversation.
func (s *ConversationStore) makeKey(userID, convID string) string {
	return conversationKeyPrefix + userID + ":" + convID
}

// ListConversations returns all conversations for a user.
func (s *ConversationStore) ListConversations(ctx context.Context, userID string) ([]operations.Conversation, error) {
	s.mu.RLock()
	userConvs, ok := s.cache[userID]
	s.mu.RUnlock()

	if ok {
		result := make([]operations.Conversation, 0, len(userConvs))
		for _, conv := range userConvs {
			result = append(result, conv)
		}
		return result, nil
	}

	// Try to load from backend
	if s.backend != nil {
		if err := s.loadUserConversations(ctx, userID); err != nil {
			return nil, err
		}

		s.mu.RLock()
		userConvs = s.cache[userID]
		s.mu.RUnlock()

		if userConvs != nil {
			result := make([]operations.Conversation, 0, len(userConvs))
			for _, conv := range userConvs {
				result = append(result, conv)
			}
			return result, nil
		}
	}

	return []operations.Conversation{}, nil
}

// GetConversation returns a single conversation.
func (s *ConversationStore) GetConversation(ctx context.Context, userID, conversationID string) (*operations.Conversation, error) {
	s.mu.RLock()
	userConvs, ok := s.cache[userID]
	if ok {
		if conv, found := userConvs[conversationID]; found {
			s.mu.RUnlock()
			return &conv, nil
		}
	}
	s.mu.RUnlock()

	// Try to load from backend
	if s.backend != nil {
		key := s.makeKey(userID, conversationID)
		data, err := s.backend.Get(ctx, key)
		if err != nil {
			if errors.Is(err, kvs.ErrNotFound) {
				return nil, errors.New("conversation not found")
			}
			return nil, fmt.Errorf("get conversation: %w", err)
		}

		var conv operations.Conversation
		if err := json.Unmarshal(data, &conv); err != nil {
			return nil, fmt.Errorf("unmarshal conversation: %w", err)
		}

		// Update cache
		s.mu.Lock()
		if s.cache[userID] == nil {
			s.cache[userID] = make(map[string]operations.Conversation)
		}
		s.cache[userID][conversationID] = conv
		s.mu.Unlock()

		return &conv, nil
	}

	return nil, errors.New("conversation not found")
}

// SaveConversation creates or updates a conversation.
func (s *ConversationStore) SaveConversation(ctx context.Context, userID string, conv operations.Conversation) error {
	// Update cache
	s.mu.Lock()
	if s.cache[userID] == nil {
		s.cache[userID] = make(map[string]operations.Conversation)
	}
	s.cache[userID][conv.ID] = conv
	s.mu.Unlock()

	// Persist to backend
	if s.backend != nil {
		data, err := json.Marshal(conv)
		if err != nil {
			return fmt.Errorf("marshal conversation: %w", err)
		}

		key := s.makeKey(userID, conv.ID)
		if err := s.backend.Set(ctx, key, data, s.ttl); err != nil {
			return fmt.Errorf("save conversation: %w", err)
		}
	}

	return nil
}

// DeleteConversation removes a conversation.
func (s *ConversationStore) DeleteConversation(ctx context.Context, userID, conversationID string) error {
	// Remove from cache
	s.mu.Lock()
	if userConvs, ok := s.cache[userID]; ok {
		delete(userConvs, conversationID)
	}
	s.mu.Unlock()

	// Remove from backend
	if s.backend != nil {
		key := s.makeKey(userID, conversationID)
		if err := s.backend.Delete(ctx, key); err != nil {
			if !errors.Is(err, kvs.ErrNotFound) {
				return fmt.Errorf("delete conversation: %w", err)
			}
		}
	}

	return nil
}

// SyncConversations syncs multiple conversations at once.
func (s *ConversationStore) SyncConversations(ctx context.Context, userID string, convs []operations.Conversation) error {
	for _, conv := range convs {
		if err := s.SaveConversation(ctx, userID, conv); err != nil {
			return err
		}
	}
	return nil
}

// loadUserConversations loads all conversations for a user from the backend.
func (s *ConversationStore) loadUserConversations(ctx context.Context, userID string) error {
	if s.backend == nil {
		return nil
	}

	listable, ok := s.backend.(kvs.ListableStore)
	if !ok {
		return nil // Backend doesn't support listing
	}

	prefix := conversationKeyPrefix + userID + ":"
	keys, err := listable.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("list conversations: %w", err)
	}

	s.mu.Lock()
	if s.cache[userID] == nil {
		s.cache[userID] = make(map[string]operations.Conversation)
	}
	s.mu.Unlock()

	for _, key := range keys {
		data, err := s.backend.Get(ctx, key)
		if err != nil {
			continue // Skip failed reads
		}

		var conv operations.Conversation
		if err := json.Unmarshal(data, &conv); err != nil {
			continue // Skip malformed data
		}

		s.mu.Lock()
		s.cache[userID][conv.ID] = conv
		s.mu.Unlock()
	}

	return nil
}

// Close closes the underlying backend.
func (s *ConversationStore) Close() error {
	if s.backend != nil {
		return s.backend.Close()
	}
	return nil
}
