package gateway

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/chats"
	"github.com/plexusone/omniagent/team/ent"
)

// PersonalChatHTTPConfig configures the personal-mode chat handler: a single
// implicit user's DM with the agent (TRD §1a "Personal" profile; §5 "private
// chat always responds"). Team mode's chat/membership/group HTTP surface
// (RMI-110-114) is a separate, not-yet-built endpoint set behind auth.
type PersonalChatHTTPConfig struct {
	Chats  *chats.Service
	UserID uuid.UUID
	Logger *slog.Logger
}

// PersonalChatHTTP serves the personal-mode chat endpoints.
type PersonalChatHTTP struct {
	chats  *chats.Service
	userID uuid.UUID
	logger *slog.Logger
}

// NewPersonalChatHTTP builds the personal chat handler set.
func NewPersonalChatHTTP(cfg PersonalChatHTTPConfig) *PersonalChatHTTP {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &PersonalChatHTTP{chats: cfg.Chats, userID: cfg.UserID, logger: cfg.Logger}
}

type messageView struct {
	ID         uuid.UUID `json:"id"`
	AuthorType string    `json:"authorType"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
}

type chatView struct {
	ID       uuid.UUID     `json:"id"`
	Messages []messageView `json:"messages"`
}

type sendChatRequest struct {
	Content string `json:"content"`
}

type sendChatResponse struct {
	User  messageView  `json:"user"`
	Agent *messageView `json:"agent,omitempty"`
}

func toMessageView(m *ent.Message) messageView {
	return messageView{ID: m.ID, AuthorType: string(m.AuthorType), Content: m.Content, CreatedAt: m.CreatedAt}
}

// ChatHandler serves GET /api/chat: the caller's private chat and its full
// history (personal mode has no keyset pagination surface at v1 — one
// user's message volume is small).
func (h *PersonalChatHTTP) ChatHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ctx := r.Context()
		c, err := h.chats.PrivateChat(ctx, h.userID)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		msgs, err := h.chats.History(ctx, h.userID, c.ID, nil, 200)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		views := make([]messageView, len(msgs))
		for i, m := range msgs {
			views[i] = toMessageView(m)
		}
		writeJSON(w, http.StatusOK, chatView{ID: c.ID, Messages: views})
	})
}

// SendHandler serves POST /api/chat/messages: persists the message, runs
// the agent's turn, and returns both.
func (h *PersonalChatHTTP) SendHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req sendChatRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, chats.MaxMessageBytes+1024)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		ctx := r.Context()
		c, err := h.chats.PrivateChat(ctx, h.userID)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		userMsg, agentMsg, err := h.chats.Send(ctx, h.userID, c.ID, req.Content)
		if err != nil {
			if errors.Is(err, chats.ErrEmptyMessage) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			h.writeServiceError(w, err)
			return
		}

		resp := sendChatResponse{User: toMessageView(userMsg)}
		if agentMsg != nil {
			v := toMessageView(agentMsg)
			resp.Agent = &v
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

func (h *PersonalChatHTTP) writeServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, chats.ErrForbidden) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	h.logger.Error("personal chat request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
