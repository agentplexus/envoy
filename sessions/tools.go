package sessions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omnistorage-core/kvs"
)

// Skill implements the compiled.Skill interface for session management.
type Skill struct {
	store *Store
}

// NewSkill creates a new session management skill.
// The store must be set before Init is called.
func NewSkill() *Skill {
	return &Skill{}
}

// Name returns the skill identifier.
func (s *Skill) Name() string {
	return "sessions"
}

// Description returns a human-readable description.
func (s *Skill) Description() string {
	return "Manage conversation sessions, view history, and interact with other sessions"
}

// Tools returns the tools provided by this skill.
func (s *Skill) Tools() []compiled.Tool {
	return []compiled.Tool{
		{
			Name:        "sessions_list",
			Description: "List all active conversation sessions",
			Parameters:  map[string]compiled.Parameter{},
			Handler:     s.handleList,
		},
		{
			Name:        "sessions_history",
			Description: "Get the conversation history for a session",
			Parameters: map[string]compiled.Parameter{
				"session_id": {
					Type:        "string",
					Description: "The ID of the session to get history for",
					Required:    true,
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of messages to return (default: 50)",
					Required:    false,
				},
			},
			Handler: s.handleHistory,
		},
		{
			Name:        "sessions_get",
			Description: "Get details about a specific session",
			Parameters: map[string]compiled.Parameter{
				"session_id": {
					Type:        "string",
					Description: "The ID of the session to get",
					Required:    true,
				},
			},
			Handler: s.handleGet,
		},
		{
			Name:        "sessions_clear",
			Description: "Clear the conversation history for a session",
			Parameters: map[string]compiled.Parameter{
				"session_id": {
					Type:        "string",
					Description: "The ID of the session to clear",
					Required:    true,
				},
			},
			Handler: s.handleClear,
		},
		{
			Name:        "sessions_delete",
			Description: "Delete a session entirely",
			Parameters: map[string]compiled.Parameter{
				"session_id": {
					Type:        "string",
					Description: "The ID of the session to delete",
					Required:    true,
				},
			},
			Handler: s.handleDelete,
		},
		{
			Name:        "sessions_metadata_set",
			Description: "Set metadata on a session",
			Parameters: map[string]compiled.Parameter{
				"session_id": {
					Type:        "string",
					Description: "The ID of the session",
					Required:    true,
				},
				"key": {
					Type:        "string",
					Description: "The metadata key",
					Required:    true,
				},
				"value": {
					Type:        "string",
					Description: "The metadata value (JSON encoded for complex values)",
					Required:    true,
				},
			},
			Handler: s.handleMetadataSet,
		},
		{
			Name:        "sessions_metadata_get",
			Description: "Get metadata from a session",
			Parameters: map[string]compiled.Parameter{
				"session_id": {
					Type:        "string",
					Description: "The ID of the session",
					Required:    true,
				},
				"key": {
					Type:        "string",
					Description: "The metadata key",
					Required:    true,
				},
			},
			Handler: s.handleMetadataGet,
		},
	}
}

// Init initializes the skill.
func (s *Skill) Init(ctx context.Context) error {
	if s.store == nil {
		return fmt.Errorf("session store not set: call SetStorage before Init")
	}
	return nil
}

// Close releases resources.
func (s *Skill) Close() error {
	return nil
}

// SetStorage implements compiled.StorageAware.
func (s *Skill) SetStorage(backend kvs.Store) {
	s.store = NewStore(StoreConfig{
		Backend: backend,
		TTL:     DefaultSessionTTL,
	})
}

// handleList handles the sessions_list tool call.
func (s *Skill) handleList(ctx context.Context, params map[string]any) (any, error) {
	ids, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	type sessionInfo struct {
		ID        string `json:"id"`
		Messages  int    `json:"message_count"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	result := make([]sessionInfo, 0, len(ids))
	for _, id := range ids {
		session, err := s.store.GetIfExists(ctx, id)
		if err != nil {
			continue // Skip sessions that can't be loaded
		}
		result = append(result, sessionInfo{
			ID:        session.ID,
			Messages:  len(session.Messages),
			CreatedAt: session.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: session.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return map[string]any{
		"sessions": result,
		"total":    len(result),
	}, nil
}

// handleHistory handles the sessions_history tool call.
func (s *Skill) handleHistory(ctx context.Context, params map[string]any) (any, error) {
	sessionID, ok := params["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	limit := 50
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	session, err := s.store.GetIfExists(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	messages := session.GetMessages()
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	type messageInfo struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	result := make([]messageInfo, len(messages))
	for i, msg := range messages {
		result[i] = messageInfo{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	return map[string]any{
		"session_id": sessionID,
		"messages":   result,
		"total":      len(session.Messages),
		"returned":   len(result),
	}, nil
}

// handleGet handles the sessions_get tool call.
func (s *Skill) handleGet(ctx context.Context, params map[string]any) (any, error) {
	sessionID, ok := params["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	session, err := s.store.GetIfExists(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	return map[string]any{
		"id":            session.ID,
		"agent_id":      session.AgentID,
		"message_count": len(session.Messages),
		"created_at":    session.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at":    session.UpdatedAt.Format("2006-01-02 15:04:05"),
		"metadata":      session.Metadata,
	}, nil
}

// handleClear handles the sessions_clear tool call.
func (s *Skill) handleClear(ctx context.Context, params map[string]any) (any, error) {
	sessionID, ok := params["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	session, err := s.store.GetIfExists(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	previousCount := len(session.Messages)
	session.Clear()

	if err := s.store.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	return map[string]any{
		"session_id":       sessionID,
		"messages_cleared": previousCount,
		"status":           "cleared",
	}, nil
}

// handleDelete handles the sessions_delete tool call.
func (s *Skill) handleDelete(ctx context.Context, params map[string]any) (any, error) {
	sessionID, ok := params["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	if err := s.store.Delete(ctx, sessionID); err != nil {
		return nil, fmt.Errorf("delete session: %w", err)
	}

	return map[string]any{
		"session_id": sessionID,
		"status":     "deleted",
	}, nil
}

// handleMetadataSet handles the sessions_metadata_set tool call.
func (s *Skill) handleMetadataSet(ctx context.Context, params map[string]any) (any, error) {
	sessionID, ok := params["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	key, ok := params["key"].(string)
	if !ok || key == "" {
		return nil, fmt.Errorf("key is required")
	}

	valueStr, ok := params["value"].(string)
	if !ok {
		return nil, fmt.Errorf("value is required")
	}

	// Try to parse as JSON, otherwise use as string
	var value any
	if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
		value = valueStr
	}

	session, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	session.SetMetadata(key, value)

	if err := s.store.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	return map[string]any{
		"session_id": sessionID,
		"key":        key,
		"status":     "set",
	}, nil
}

// handleMetadataGet handles the sessions_metadata_get tool call.
func (s *Skill) handleMetadataGet(ctx context.Context, params map[string]any) (any, error) {
	sessionID, ok := params["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	key, ok := params["key"].(string)
	if !ok || key == "" {
		return nil, fmt.Errorf("key is required")
	}

	session, err := s.store.GetIfExists(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	value, exists := session.GetMetadata(key)
	if !exists {
		return map[string]any{
			"session_id": sessionID,
			"key":        key,
			"exists":     false,
		}, nil
	}

	return map[string]any{
		"session_id": sessionID,
		"key":        key,
		"value":      value,
		"exists":     true,
	}, nil
}

// Ensure Skill implements the required interfaces.
var (
	_ compiled.Skill        = (*Skill)(nil)
	_ compiled.StorageAware = (*Skill)(nil)
)
