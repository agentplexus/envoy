// Package sessions provides persistent session management for the agent.
package sessions

import (
	"encoding/json"
	"time"

	"github.com/plexusone/omnillm/provider"
)

// Session represents a conversation session.
type Session struct {
	ID        string             `json:"id"`
	AgentID   string             `json:"agent_id,omitempty"`
	Messages  []provider.Message `json:"messages"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
	Metadata  map[string]any     `json:"metadata,omitempty"`

	// ToolOverrides scopes the tool set for this session's turns.
	// Nil means no overrides. Changes take effect on the next turn.
	ToolOverrides *ToolOverrides `json:"tool_overrides,omitempty"`

	// Model overrides the agent's default model for this session's turns.
	// Empty uses the agent default. Changes take effect on the next turn.
	Model string `json:"model,omitempty"`
}

// NewSession creates a new session with the given ID.
func NewSession(id string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		Messages:  []provider.Message{},
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]any),
	}
}

// AddMessage adds a message to the session.
func (s *Session) AddMessage(role provider.Role, content string) {
	s.Messages = append(s.Messages, provider.Message{
		Role:    role,
		Content: content,
	})
	s.UpdatedAt = time.Now()
}

// AddMessageWithToolCalls adds a message with tool calls to the session.
func (s *Session) AddMessageWithToolCalls(role provider.Role, content string, toolCalls []provider.ToolCall) {
	s.Messages = append(s.Messages, provider.Message{
		Role:      role,
		Content:   content,
		ToolCalls: toolCalls,
	})
	s.UpdatedAt = time.Now()
}

// AddToolResult adds a tool result message to the session.
func (s *Session) AddToolResult(toolCallID, content string) {
	s.Messages = append(s.Messages, provider.Message{
		Role:       provider.RoleTool,
		Content:    content,
		ToolCallID: &toolCallID,
	})
	s.UpdatedAt = time.Now()
}

// GetMessages returns all messages in the session.
func (s *Session) GetMessages() []provider.Message {
	messages := make([]provider.Message, len(s.Messages))
	copy(messages, s.Messages)
	return messages
}

// SetMetadata sets a metadata value.
func (s *Session) SetMetadata(key string, value any) {
	if s.Metadata == nil {
		s.Metadata = make(map[string]any)
	}
	s.Metadata[key] = value
	s.UpdatedAt = time.Now()
}

// GetMetadata gets a metadata value.
func (s *Session) GetMetadata(key string) (any, bool) {
	if s.Metadata == nil {
		return nil, false
	}
	v, ok := s.Metadata[key]
	return v, ok
}

// Clear removes all messages from the session.
func (s *Session) Clear() {
	s.Messages = []provider.Message{}
	s.UpdatedAt = time.Now()
}

// Trim keeps only the last n messages.
func (s *Session) Trim(n int) {
	if len(s.Messages) > n {
		s.Messages = s.Messages[len(s.Messages)-n:]
	}
	s.UpdatedAt = time.Now()
}

// MarshalJSON serializes the session to JSON.
func (s *Session) MarshalJSON() ([]byte, error) {
	type Alias Session
	return json.Marshal((*Alias)(s))
}

// UnmarshalJSON deserializes the session from JSON.
func (s *Session) UnmarshalJSON(data []byte) error {
	type Alias Session
	aux := (*Alias)(s)
	return json.Unmarshal(data, aux)
}
