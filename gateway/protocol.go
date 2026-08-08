package gateway

import "time"

// MessageType represents the type of gateway message.
type MessageType string

const (
	// Client -> Gateway
	MessageTypeChat         MessageType = "chat"
	MessageTypePing         MessageType = "ping"
	MessageTypeAuth         MessageType = "auth"
	MessageTypeSubscribe    MessageType = "subscribe"
	MessageTypeSessionTools MessageType = "session_tools"
	MessageTypeSessionModel MessageType = "session_model"

	// Gateway -> Client
	MessageTypeResponse MessageType = "response"
	MessageTypePong     MessageType = "pong"
	MessageTypeError    MessageType = "error"
	MessageTypeEvent    MessageType = "event"
)

// Message is the base message structure for gateway communication.
type Message struct {
	ID        string                 `json:"id,omitempty"`
	Type      MessageType            `json:"type"`
	Channel   string                 `json:"channel,omitempty"`
	Content   string                 `json:"content,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp,omitempty"`
}

// ChatMessage represents a chat message.
type ChatMessage struct {
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content"`
	Channel   string `json:"channel,omitempty"`
	ReplyTo   string `json:"reply_to,omitempty"`
}

// AuthMessage represents an authentication message.
type AuthMessage struct {
	Token    string `json:"token,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
}

// EventMessage represents an event notification.
type EventMessage struct {
	Event   string                 `json:"event"`
	Channel string                 `json:"channel,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// NewChatResponse creates a chat response message.
func NewChatResponse(id, content string) *Message {
	return &Message{
		ID:        id,
		Type:      MessageTypeResponse,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewErrorMessage creates an error message.
func NewErrorMessage(id, errMsg string) *Message {
	return &Message{
		ID:        id,
		Type:      MessageTypeError,
		Error:     errMsg,
		Timestamp: time.Now(),
	}
}

// NewEventMessage creates an event message.
func NewEventMessage(event, channel string, data map[string]interface{}) *Message {
	return &Message{
		Type:      MessageTypeEvent,
		Channel:   channel,
		Content:   event,
		Data:      data,
		Timestamp: time.Now(),
	}
}
