package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/chats"
	"github.com/plexusone/omniagent/team/ent"
	entuser "github.com/plexusone/omniagent/team/ent/user"
	"github.com/plexusone/omniagent/team/store"
)

// setupPersonalChatHTTP builds a personal chat handler over a temp SQLite
// store and wires a buffered broadcaster so tests can observe the async
// agent reply that SendHandler delivers over the WebSocket.
func setupPersonalChatHTTP(t *testing.T) (*PersonalChatHTTP, <-chan *Message) {
	t.Helper()
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "personal.db")
	storeCfg := store.Config{AppDSN: dsn}
	if err := store.Migrate(ctx, storeCfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st, err := store.Open(ctx, storeCfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	var userID uuid.UUID
	if err := st.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		u, err := tx.User.Create().
			SetEmail("local@omniagent.personal").SetUsername("me").
			SetRole(entuser.RoleSuperadmin).Save(ctx)
		if err != nil {
			return err
		}
		userID = u.ID
		return nil
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	chatSvc, err := chats.NewService(st, chats.Config{})
	if err != nil {
		t.Fatalf("chats.NewService: %v", err)
	}
	h := NewPersonalChatHTTP(PersonalChatHTTPConfig{Chats: chatSvc, UserID: userID})
	bcast := make(chan *Message, 8)
	h.SetBroadcaster(func(m *Message) { bcast <- m })
	return h, bcast
}

func TestPersonalChatHTTP_ChatAndSend(t *testing.T) {
	h, bcast := setupPersonalChatHTTP(t)

	// Fresh chat, no messages yet.
	rec := httptest.NewRecorder()
	h.ChatHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/chat", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/chat status = %d, want 200", rec.Code)
	}
	var got chatView
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Messages) != 0 {
		t.Errorf("fresh chat has %d messages, want 0", len(got.Messages))
	}
	if got.HasMore {
		t.Errorf("fresh chat HasMore = true, want false")
	}
	chatID := got.ID

	// Send a message: 202 with only the user message; the reply is async.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/messages", strings.NewReader(`{"content":"hi"}`))
	h.SendHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/chat/messages status = %d, want 202, body = %s", rec.Code, rec.Body.String())
	}
	var sendResp sendChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sendResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sendResp.User.Content != "hi" {
		t.Errorf("User.Content = %q, want %q", sendResp.User.Content, "hi")
	}

	// The agent reply is broadcast over the WS (no agent configured → echo).
	select {
	case m := <-bcast:
		if m.Type != MessageTypeEvent || m.Content != "chat.message" {
			t.Fatalf("broadcast = %+v, want chat.message event", m)
		}
		if c, _ := m.Data["content"].(string); c != "Message received: hi" {
			t.Errorf("broadcast content = %q, want echo fallback", c)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for agent reply broadcast")
	}

	// Chat now shows both messages, same chat ID.
	rec = httptest.NewRecorder()
	h.ChatHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/chat", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != chatID {
		t.Errorf("chat ID changed across requests: %s vs %s", got.ID, chatID)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("chat has %d messages, want 2", len(got.Messages))
	}
}

func TestPersonalChatHTTP_HistoryScrollBack(t *testing.T) {
	h, _ := setupPersonalChatHTTP(t)
	ctx := context.Background()

	c, err := h.chats.PrivateChat(ctx, h.userID)
	if err != nil {
		t.Fatalf("PrivateChat: %v", err)
	}
	// 3 sends → 6 messages (user + echo agent each), time-ordered IDs.
	for i := 0; i < 3; i++ {
		if _, _, err := h.chats.Send(ctx, h.userID, c.ID, "m"); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}

	// Newest page of 2, with more available.
	rec := httptest.NewRecorder()
	h.HistoryHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/chat/history?limit=2", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("history status = %d, want 200", rec.Code)
	}
	var p1 historyView
	if err := json.Unmarshal(rec.Body.Bytes(), &p1); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(p1.Messages) != 2 || !p1.HasMore {
		t.Fatalf("page 1 = %d msgs hasMore=%v, want 2 true", len(p1.Messages), p1.HasMore)
	}

	// Scroll back before the oldest of page 1 → the next 2 older messages.
	before := p1.Messages[0].ID
	rec = httptest.NewRecorder()
	h.HistoryHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/chat/history?limit=2&before="+before.String(), nil))
	var p2 historyView
	if err := json.Unmarshal(rec.Body.Bytes(), &p2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(p2.Messages) != 2 || !p2.HasMore {
		t.Fatalf("page 2 = %d msgs hasMore=%v, want 2 true", len(p2.Messages), p2.HasMore)
	}
	// Scroll-back must exclude the cursor message itself (strictly older).
	if p2.Messages[0].ID == before || p2.Messages[1].ID == before {
		t.Error("scroll-back returned the cursor message itself")
	}

	// Final page: the oldest 2, no more.
	before = p2.Messages[0].ID
	rec = httptest.NewRecorder()
	h.HistoryHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/chat/history?limit=2&before="+before.String(), nil))
	var p3 historyView
	if err := json.Unmarshal(rec.Body.Bytes(), &p3); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(p3.Messages) != 2 || p3.HasMore {
		t.Fatalf("page 3 = %d msgs hasMore=%v, want 2 false", len(p3.Messages), p3.HasMore)
	}
}

func TestPersonalChatHTTP_SendEmptyRejected(t *testing.T) {
	h, _ := setupPersonalChatHTTP(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/messages", strings.NewReader(`{"content":"  "}`))
	h.SendHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestPersonalChatHTTP_MethodNotAllowed(t *testing.T) {
	h, _ := setupPersonalChatHTTP(t)

	rec := httptest.NewRecorder()
	h.ChatHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/chat", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/chat status = %d, want 405", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.SendHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/chat/messages", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/chat/messages status = %d, want 405", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.HistoryHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/chat/history", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/chat/history status = %d, want 405", rec.Code)
	}
}
