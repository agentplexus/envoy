package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// mockAgent is a simple agent for testing.
type mockAgent struct {
	response string
	err      error
}

func (m *mockAgent) Process(ctx context.Context, sessionID, content string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.response != "" {
		return m.response, nil
	}
	return "Echo: " + content, nil
}

func TestGatewayWebSocket(t *testing.T) {
	// Create gateway with mock agent
	gw, err := New(Config{
		Address: "127.0.0.1:0",
		Agent:   &mockAgent{response: "Hello from agent!"},
	})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.handleWebSocket)
	mux.HandleFunc("/health", gw.handleHealth)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect WebSocket client
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Wait for client registration
	time.Sleep(50 * time.Millisecond)

	// Verify client connected
	if gw.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", gw.ClientCount())
	}

	t.Run("ping-pong", func(t *testing.T) {
		// Send ping
		ping := &Message{
			ID:   "ping-1",
			Type: MessageTypePing,
		}
		if err := conn.WriteJSON(ping); err != nil {
			t.Fatalf("Failed to send ping: %v", err)
		}

		// Read pong
		var pong Message
		if err := conn.ReadJSON(&pong); err != nil {
			t.Fatalf("Failed to read pong: %v", err)
		}

		if pong.Type != MessageTypePong {
			t.Errorf("Expected pong, got %s", pong.Type)
		}
		if pong.ID != "ping-1" {
			t.Errorf("Expected ID ping-1, got %s", pong.ID)
		}
	})

	t.Run("chat-with-agent", func(t *testing.T) {
		// Send chat message
		chat := &Message{
			ID:      "chat-1",
			Type:    MessageTypeChat,
			Content: "Hello, agent!",
		}
		if err := conn.WriteJSON(chat); err != nil {
			t.Fatalf("Failed to send chat: %v", err)
		}

		// Read response
		var resp Message
		if err := conn.ReadJSON(&resp); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if resp.Type != MessageTypeResponse {
			t.Errorf("Expected response, got %s", resp.Type)
		}
		if resp.Content != "Hello from agent!" {
			t.Errorf("Expected 'Hello from agent!', got %s", resp.Content)
		}
	})

	t.Run("auth", func(t *testing.T) {
		// Send auth
		auth := &Message{
			ID:   "auth-1",
			Type: MessageTypeAuth,
			Data: map[string]interface{}{"token": "test-token"},
		}
		if err := conn.WriteJSON(auth); err != nil {
			t.Fatalf("Failed to send auth: %v", err)
		}

		// Read response
		var resp Message
		if err := conn.ReadJSON(&resp); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if resp.Type != MessageTypeResponse {
			t.Errorf("Expected response, got %s", resp.Type)
		}
		if resp.Data["authenticated"] != true {
			t.Error("Expected authenticated: true")
		}
	})

	t.Run("subscribe", func(t *testing.T) {
		// Send subscribe
		sub := &Message{
			ID:      "sub-1",
			Type:    MessageTypeSubscribe,
			Channel: "test-channel",
		}
		if err := conn.WriteJSON(sub); err != nil {
			t.Fatalf("Failed to send subscribe: %v", err)
		}

		// Read response
		var resp Message
		if err := conn.ReadJSON(&resp); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if resp.Type != MessageTypeResponse {
			t.Errorf("Expected response, got %s", resp.Type)
		}
		if resp.Data["subscribed"] != true {
			t.Error("Expected subscribed: true")
		}
	})
}

func TestGatewayHealth(t *testing.T) {
	gw, err := New(Config{Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", gw.handleHealth)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Check health endpoint
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to get health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health: %v", err)
	}

	if health["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", health["status"])
	}
}

func TestGatewayNoAgent(t *testing.T) {
	// Create gateway without agent (echo mode)
	gw, err := New(Config{Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.handleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Connect
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	// Send chat message
	chat := &Message{
		ID:      "chat-1",
		Type:    MessageTypeChat,
		Content: "Hello!",
	}
	if err := conn.WriteJSON(chat); err != nil {
		t.Fatalf("Failed to send chat: %v", err)
	}

	// Read response (should be echo)
	var resp Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Type != MessageTypeResponse {
		t.Errorf("Expected response, got %s", resp.Type)
	}
	if resp.Content != "Message received: Hello!" {
		t.Errorf("Expected echo response, got %s", resp.Content)
	}
}

func TestGatewayBroadcast(t *testing.T) {
	gw, err := New(Config{Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.handleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect two clients
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect client 1: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect client 2: %v", err)
	}
	defer conn2.Close()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	if gw.ClientCount() != 2 {
		t.Errorf("Expected 2 clients, got %d", gw.ClientCount())
	}

	// Broadcast a message
	broadcastMsg := NewEventMessage("test_event", "broadcast", map[string]interface{}{"data": "test"})
	gw.Broadcast(broadcastMsg)

	// Both clients should receive it
	for i, conn := range []*websocket.Conn{conn1, conn2} {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			t.Errorf("Client %d failed to read broadcast: %v", i+1, err)
			continue
		}
		if msg.Type != MessageTypeEvent {
			t.Errorf("Client %d: expected event, got %s", i+1, msg.Type)
		}
	}
}
