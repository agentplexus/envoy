package gateway

import (
	"testing"
	"time"
)

func TestNewChatResponse(t *testing.T) {
	msg := NewChatResponse("123", "Hello, world!")

	if msg.ID != "123" {
		t.Errorf("ID = %s, want 123", msg.ID)
	}
	if msg.Type != MessageTypeResponse {
		t.Errorf("Type = %s, want %s", msg.Type, MessageTypeResponse)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("Content = %s, want Hello, world!", msg.Content)
	}
	if msg.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestNewErrorMessage(t *testing.T) {
	msg := NewErrorMessage("456", "something went wrong")

	if msg.ID != "456" {
		t.Errorf("ID = %s, want 456", msg.ID)
	}
	if msg.Type != MessageTypeError {
		t.Errorf("Type = %s, want %s", msg.Type, MessageTypeError)
	}
	if msg.Error != "something went wrong" {
		t.Errorf("Error = %s, want something went wrong", msg.Error)
	}
}

func TestNewEventMessage(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	msg := NewEventMessage("user_joined", "general", data)

	if msg.Type != MessageTypeEvent {
		t.Errorf("Type = %s, want %s", msg.Type, MessageTypeEvent)
	}
	if msg.Channel != "general" {
		t.Errorf("Channel = %s, want general", msg.Channel)
	}
	if msg.Content != "user_joined" {
		t.Errorf("Content = %s, want user_joined", msg.Content)
	}
	if msg.Data["key"] != "value" {
		t.Errorf("Data[key] = %v, want value", msg.Data["key"])
	}
}

func TestMessageTypes(t *testing.T) {
	// Verify message type constants
	types := []MessageType{
		MessageTypeChat,
		MessageTypePing,
		MessageTypeAuth,
		MessageTypeSubscribe,
		MessageTypeResponse,
		MessageTypePong,
		MessageTypeError,
		MessageTypeEvent,
	}

	seen := make(map[MessageType]bool)
	for _, mt := range types {
		if seen[mt] {
			t.Errorf("Duplicate message type: %s", mt)
		}
		seen[mt] = true
		if mt == "" {
			t.Error("Message type should not be empty")
		}
	}
}

func TestMessageTimestamp(t *testing.T) {
	before := time.Now()
	msg := NewChatResponse("test", "content")
	after := time.Now()

	if msg.Timestamp.Before(before) || msg.Timestamp.After(after) {
		t.Errorf("Timestamp %v should be between %v and %v", msg.Timestamp, before, after)
	}
}
