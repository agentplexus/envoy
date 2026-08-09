package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/chats"
	"github.com/plexusone/omniagent/team/ent"
	entuser "github.com/plexusone/omniagent/team/ent/user"
	"github.com/plexusone/omniagent/team/store"
)

func setupPersonalChatHTTP(t *testing.T) *PersonalChatHTTP {
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
	return NewPersonalChatHTTP(PersonalChatHTTPConfig{Chats: chatSvc, UserID: userID})
}

func TestPersonalChatHTTP_ChatAndSend(t *testing.T) {
	h := setupPersonalChatHTTP(t)

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
	chatID := got.ID

	// Send a message — no agent configured, so it echoes.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/messages", strings.NewReader(`{"content":"hi"}`))
	h.SendHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/chat/messages status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var sendResp sendChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sendResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sendResp.User.Content != "hi" {
		t.Errorf("User.Content = %q, want %q", sendResp.User.Content, "hi")
	}
	if sendResp.Agent == nil || sendResp.Agent.Content != "Message received: hi" {
		t.Errorf("Agent reply = %+v, want echo fallback", sendResp.Agent)
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

func TestPersonalChatHTTP_SendEmptyRejected(t *testing.T) {
	h := setupPersonalChatHTTP(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/messages", strings.NewReader(`{"content":"  "}`))
	h.SendHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestPersonalChatHTTP_MethodNotAllowed(t *testing.T) {
	h := setupPersonalChatHTTP(t)

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
}
