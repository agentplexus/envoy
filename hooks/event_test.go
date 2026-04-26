package hooks

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEvent(t *testing.T) {
	data := MessageEvent{
		Role:    "user",
		Content: "hello",
	}

	before := time.Now()
	event := NewEvent(EventMessageReceived, data)
	after := time.Now()

	if event.Type != EventMessageReceived {
		t.Errorf("expected type %q, got %q", EventMessageReceived, event.Type)
	}

	if event.Timestamp.Before(before) || event.Timestamp.After(after) {
		t.Error("timestamp not in expected range")
	}

	if event.SessionID != "" {
		t.Error("expected empty session ID")
	}

	msg, ok := event.Data.(MessageEvent)
	if !ok {
		t.Fatal("data not MessageEvent")
	}
	if msg.Role != "user" || msg.Content != "hello" {
		t.Error("data not preserved correctly")
	}
}

func TestEventWithSessionID(t *testing.T) {
	event := NewEvent(EventToolCalled, nil)
	sessionEvent := event.WithSessionID("session-123")

	// Original should be unchanged
	if event.SessionID != "" {
		t.Error("original event modified")
	}

	// New event should have session ID
	if sessionEvent.SessionID != "session-123" {
		t.Errorf("expected session ID %q, got %q", "session-123", sessionEvent.SessionID)
	}

	// Other fields should match
	if sessionEvent.Type != event.Type {
		t.Error("type not preserved")
	}
	if sessionEvent.Timestamp != event.Timestamp {
		t.Error("timestamp not preserved")
	}
}

func TestEventJSONSerialization(t *testing.T) {
	event := Event{
		Type:      EventMessageSent,
		Timestamp: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		SessionID: "sess-1",
		Data: MessageEvent{
			Role:    "assistant",
			Content: "Hello!",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["type"] != "message.sent" {
		t.Errorf("expected type %q, got %q", "message.sent", parsed["type"])
	}
	if parsed["session_id"] != "sess-1" {
		t.Errorf("expected session_id %q, got %q", "sess-1", parsed["session_id"])
	}
}

func TestMessageEvent(t *testing.T) {
	msg := MessageEvent{
		Role:    "user",
		Content: "test message",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed MessageEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed.Role != msg.Role || parsed.Content != msg.Content {
		t.Error("round-trip failed")
	}
}

func TestToolEvent(t *testing.T) {
	tool := ToolEvent{
		Name:   "search",
		Params: map[string]any{"query": "test"},
		Result: "found 5 results",
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed ToolEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed.Name != tool.Name || parsed.Result != tool.Result {
		t.Error("round-trip failed")
	}
	if parsed.Params["query"] != "test" {
		t.Error("params not preserved")
	}
}

func TestSessionEvent(t *testing.T) {
	sess := SessionEvent{
		SessionID: "session-abc",
		Action:    "created",
	}

	data, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed SessionEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed.SessionID != sess.SessionID || parsed.Action != sess.Action {
		t.Error("round-trip failed")
	}
}

func TestJobEvent(t *testing.T) {
	job := JobEvent{
		JobID:   "job-123",
		JobName: "cleanup",
		Success: true,
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed JobEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed.JobID != job.JobID || parsed.JobName != job.JobName || parsed.Success != job.Success {
		t.Error("round-trip failed")
	}
}
