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

func TestGatewayOriginCheck(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		want           bool
	}{
		{
			name:           "no allowed origins (allow all)",
			allowedOrigins: nil,
			requestOrigin:  "https://example.com",
			want:           true,
		},
		{
			name:           "wildcard allows all",
			allowedOrigins: []string{"*"},
			requestOrigin:  "https://anything.com",
			want:           true,
		},
		{
			name:           "exact match",
			allowedOrigins: []string{"https://example.com"},
			requestOrigin:  "https://example.com",
			want:           true,
		},
		{
			name:           "case insensitive match",
			allowedOrigins: []string{"https://EXAMPLE.com"},
			requestOrigin:  "https://example.com",
			want:           true,
		},
		{
			name:           "no match",
			allowedOrigins: []string{"https://example.com"},
			requestOrigin:  "https://other.com",
			want:           false,
		},
		{
			name:           "wildcard subdomain match",
			allowedOrigins: []string{"https://*.example.com"},
			requestOrigin:  "https://app.example.com",
			want:           true,
		},
		{
			name:           "wildcard subdomain - base domain match",
			allowedOrigins: []string{"https://*.example.com"},
			requestOrigin:  "https://example.com",
			want:           true,
		},
		{
			name:           "wildcard subdomain - nested subdomain",
			allowedOrigins: []string{"https://*.example.com"},
			requestOrigin:  "https://deep.app.example.com",
			want:           true,
		},
		{
			name:           "no origin header (same-origin)",
			allowedOrigins: []string{"https://example.com"},
			requestOrigin:  "",
			want:           true,
		},
		{
			name:           "multiple allowed origins",
			allowedOrigins: []string{"https://a.com", "https://b.com", "https://c.com"},
			requestOrigin:  "https://b.com",
			want:           true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gw, err := New(Config{
				Address:        "127.0.0.1:0",
				AllowedOrigins: tc.allowedOrigins,
			})
			if err != nil {
				t.Fatalf("Failed to create gateway: %v", err)
			}

			req := httptest.NewRequest("GET", "/ws", nil)
			if tc.requestOrigin != "" {
				req.Header.Set("Origin", tc.requestOrigin)
			}

			got := gw.checkOrigin(req)
			if got != tc.want {
				t.Errorf("checkOrigin() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGatewayGetClient(t *testing.T) {
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

	// Connect a client
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	// Get client ID by checking client count
	if gw.ClientCount() != 1 {
		t.Fatalf("Expected 1 client, got %d", gw.ClientCount())
	}

	// Test GetClient with non-existent ID
	client := gw.GetClient("non-existent-id")
	if client != nil {
		t.Error("Expected nil for non-existent client")
	}
}

func TestGatewayOnMessage(t *testing.T) {
	customHandlerCalled := false

	gw, err := New(Config{Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Set custom message handler
	gw.OnMessage(func(ctx context.Context, client *Client, msg *Message) (*Message, error) {
		customHandlerCalled = true
		return &Message{
			ID:        msg.ID,
			Type:      MessageTypeResponse,
			Content:   "custom response",
			Timestamp: time.Now(),
		}, nil
	})

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.handleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Send message
	chat := &Message{ID: "test-1", Type: MessageTypeChat, Content: "Hello"}
	if err := conn.WriteJSON(chat); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	var resp Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if !customHandlerCalled {
		t.Error("Custom handler was not called")
	}

	if resp.Content != "custom response" {
		t.Errorf("Expected 'custom response', got %q", resp.Content)
	}
}

func TestGatewayUnknownMessageType(t *testing.T) {
	gw, err := New(Config{Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.handleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Send message with unknown type
	msg := &Message{ID: "test-1", Type: MessageType("unknown")}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	var resp Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if resp.Type != MessageTypeError {
		t.Errorf("Expected error type, got %s", resp.Type)
	}
}

func TestGatewaySubscribeNoChannel(t *testing.T) {
	gw, err := New(Config{Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.handleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Send subscribe without channel
	msg := &Message{ID: "test-1", Type: MessageTypeSubscribe, Channel: ""}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	var resp Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if resp.Type != MessageTypeError {
		t.Errorf("Expected error type for missing channel, got %s", resp.Type)
	}
}

func TestClientMetadata(t *testing.T) {
	gw, err := New(Config{Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.handleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Auth sets metadata
	auth := &Message{ID: "auth-1", Type: MessageTypeAuth}
	if err := conn.WriteJSON(auth); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	var resp Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	// Subscribe adds to subscriptions metadata
	sub := &Message{ID: "sub-1", Type: MessageTypeSubscribe, Channel: "ch1"}
	if err := conn.WriteJSON(sub); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	// Subscribe to another channel
	sub2 := &Message{ID: "sub-2", Type: MessageTypeSubscribe, Channel: "ch2"}
	if err := conn.WriteJSON(sub2); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if resp.Data["subscribed"] != true {
		t.Error("Expected subscribed: true")
	}
}

func TestMessageHelpers(t *testing.T) {
	t.Run("NewChatResponse", func(t *testing.T) {
		msg := NewChatResponse("id-1", "Hello")
		if msg.ID != "id-1" {
			t.Errorf("ID = %q, want 'id-1'", msg.ID)
		}
		if msg.Type != MessageTypeResponse {
			t.Errorf("Type = %s, want response", msg.Type)
		}
		if msg.Content != "Hello" {
			t.Errorf("Content = %q, want 'Hello'", msg.Content)
		}
	})

	t.Run("NewErrorMessage", func(t *testing.T) {
		msg := NewErrorMessage("id-2", "something went wrong")
		if msg.ID != "id-2" {
			t.Errorf("ID = %q, want 'id-2'", msg.ID)
		}
		if msg.Type != MessageTypeError {
			t.Errorf("Type = %s, want error", msg.Type)
		}
		if msg.Error != "something went wrong" {
			t.Errorf("Error = %q, want 'something went wrong'", msg.Error)
		}
	})

	t.Run("NewEventMessage", func(t *testing.T) {
		data := map[string]interface{}{"key": "value"}
		msg := NewEventMessage("user_joined", "lobby", data)
		if msg.Type != MessageTypeEvent {
			t.Errorf("Type = %s, want event", msg.Type)
		}
		if msg.Channel != "lobby" {
			t.Errorf("Channel = %q, want 'lobby'", msg.Channel)
		}
		if msg.Content != "user_joined" {
			t.Errorf("Content = %q, want 'user_joined'", msg.Content)
		}
		if msg.Data["key"] != "value" {
			t.Errorf("Data[key] = %v, want 'value'", msg.Data["key"])
		}
	})
}

func TestGatewayAuthWithToken(t *testing.T) {
	// Test auth with token field (compatibility)
	gw, err := New(Config{
		Address:     "127.0.0.1:0",
		RequireAuth: true,
		APIKeys:     []string{"my-token"},
	})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.handleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Auth with token field instead of api_key
	auth := &Message{
		ID:   "auth-1",
		Type: MessageTypeAuth,
		Data: map[string]interface{}{"token": "my-token"},
	}
	if err := conn.WriteJSON(auth); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	var resp Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if resp.Type != MessageTypeResponse {
		t.Errorf("Expected response, got %s", resp.Type)
	}
	if resp.Data["authenticated"] != true {
		t.Error("Expected authenticated: true")
	}
}

func TestGatewayAuthNoKey(t *testing.T) {
	gw, err := New(Config{
		Address:     "127.0.0.1:0",
		RequireAuth: true,
		APIKeys:     []string{"valid-key"},
	})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.handleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Auth without any key
	auth := &Message{
		ID:   "auth-1",
		Type: MessageTypeAuth,
		Data: map[string]interface{}{},
	}
	if err := conn.WriteJSON(auth); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	var resp Message
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if resp.Type != MessageTypeError {
		t.Errorf("Expected error, got %s", resp.Type)
	}
	if resp.Data["error"] != "authentication_required" {
		t.Errorf("Expected authentication_required, got %v", resp.Data["error"])
	}
}

func TestGatewayRun(t *testing.T) {
	gw, err := New(Config{
		Address: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- gw.Run(ctx)
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel to trigger shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Errorf("Run returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Run did not return after cancel")
	}
}

func TestClientSendBufferFull(t *testing.T) {
	gw, err := New(Config{Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gw.handleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect but don't read - this will cause send buffer to fill
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Get the client from the gateway (indirectly via broadcast)
	// Send many messages to fill the buffer (buffer size is 256)
	for i := 0; i < 300; i++ {
		gw.Broadcast(&Message{
			ID:      "flood-" + string(rune('0'+i%10)),
			Type:    MessageTypeEvent,
			Content: "flood message",
		})
	}

	// This should not block or panic - messages beyond buffer are dropped
	time.Sleep(50 * time.Millisecond)
}

func TestGatewayWithWebhookHandlers(t *testing.T) {
	webhookCalled := false
	webhookHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	})

	gw, err := New(Config{
		Address: "127.0.0.1:0",
		WebhookHandlers: map[string]http.Handler{
			"/webhook/test": webhookHandler,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- gw.Run(ctx)
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// The webhook handler is mounted but we can't easily test it without knowing the port
	// At least we verify it doesn't error during setup
	_ = webhookCalled

	cancel()

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Error("Run did not return after cancel")
	}
}

func TestGatewayAuthentication(t *testing.T) {
	t.Run("no auth required", func(t *testing.T) {
		gw, err := New(Config{
			Address:     "127.0.0.1:0",
			RequireAuth: false,
		})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/ws", gw.handleWebSocket)
		server := httptest.NewServer(mux)
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		time.Sleep(50 * time.Millisecond)

		// Send chat without auth - should work
		chat := &Message{ID: "chat-1", Type: MessageTypeChat, Content: "Hello"}
		if err := conn.WriteJSON(chat); err != nil {
			t.Fatalf("Failed to send chat: %v", err)
		}

		var resp Message
		if err := conn.ReadJSON(&resp); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if resp.Type != MessageTypeResponse {
			t.Errorf("Expected response, got %s", resp.Type)
		}
	})

	t.Run("auth required - valid key", func(t *testing.T) {
		gw, err := New(Config{
			Address:     "127.0.0.1:0",
			RequireAuth: true,
			APIKeys:     []string{"valid-key-123"},
		})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/ws", gw.handleWebSocket)
		server := httptest.NewServer(mux)
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		time.Sleep(50 * time.Millisecond)

		// Authenticate with valid key
		auth := &Message{
			ID:   "auth-1",
			Type: MessageTypeAuth,
			Data: map[string]interface{}{"api_key": "valid-key-123"},
		}
		if err := conn.WriteJSON(auth); err != nil {
			t.Fatalf("Failed to send auth: %v", err)
		}

		var authResp Message
		if err := conn.ReadJSON(&authResp); err != nil {
			t.Fatalf("Failed to read auth response: %v", err)
		}

		if authResp.Type != MessageTypeResponse {
			t.Errorf("Expected response, got %s", authResp.Type)
		}
		if authResp.Data["authenticated"] != true {
			t.Error("Expected authenticated: true")
		}

		// Now chat should work
		chat := &Message{ID: "chat-1", Type: MessageTypeChat, Content: "Hello"}
		if err := conn.WriteJSON(chat); err != nil {
			t.Fatalf("Failed to send chat: %v", err)
		}

		var chatResp Message
		if err := conn.ReadJSON(&chatResp); err != nil {
			t.Fatalf("Failed to read chat response: %v", err)
		}

		if chatResp.Type != MessageTypeResponse {
			t.Errorf("Expected response, got %s", chatResp.Type)
		}
	})

	t.Run("auth required - invalid key", func(t *testing.T) {
		gw, err := New(Config{
			Address:     "127.0.0.1:0",
			RequireAuth: true,
			APIKeys:     []string{"valid-key-123"},
		})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/ws", gw.handleWebSocket)
		server := httptest.NewServer(mux)
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		time.Sleep(50 * time.Millisecond)

		// Try to authenticate with invalid key
		auth := &Message{
			ID:   "auth-1",
			Type: MessageTypeAuth,
			Data: map[string]interface{}{"api_key": "wrong-key"},
		}
		if err := conn.WriteJSON(auth); err != nil {
			t.Fatalf("Failed to send auth: %v", err)
		}

		var authResp Message
		if err := conn.ReadJSON(&authResp); err != nil {
			t.Fatalf("Failed to read auth response: %v", err)
		}

		if authResp.Type != MessageTypeError {
			t.Errorf("Expected error, got %s", authResp.Type)
		}
		if authResp.Data["error"] != "invalid_credentials" {
			t.Errorf("Expected invalid_credentials error, got %v", authResp.Data["error"])
		}
	})

	t.Run("auth required - chat without auth", func(t *testing.T) {
		gw, err := New(Config{
			Address:     "127.0.0.1:0",
			RequireAuth: true,
			APIKeys:     []string{"valid-key-123"},
		})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/ws", gw.handleWebSocket)
		server := httptest.NewServer(mux)
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		time.Sleep(50 * time.Millisecond)

		// Try to chat without authenticating
		chat := &Message{ID: "chat-1", Type: MessageTypeChat, Content: "Hello"}
		if err := conn.WriteJSON(chat); err != nil {
			t.Fatalf("Failed to send chat: %v", err)
		}

		var resp Message
		if err := conn.ReadJSON(&resp); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if resp.Type != MessageTypeError {
			t.Errorf("Expected error, got %s", resp.Type)
		}
		if resp.Data["error"] != "authentication_required" {
			t.Errorf("Expected authentication_required error, got %v", resp.Data["error"])
		}
	})
}
