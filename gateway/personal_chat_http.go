package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/chats"
	"github.com/plexusone/omniagent/team/ent"
)

// defaultChatPageSize is the message page size for the initial chat load and
// each scroll-back fetch (RMI-117). maxChatPageSize caps a client-supplied
// limit so limit+1 (the has-more probe) stays within the service's own cap.
const (
	defaultChatPageSize = 50
	maxChatPageSize     = 100
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

	// broadcast delivers an agent reply to the user's live WebSocket
	// connection(s) once the (asynchronous) agent turn completes. Nil until
	// SetBroadcaster wires it to the gateway; when nil the reply is still
	// persisted and shows up on the next history load.
	broadcast func(*Message)
}

// NewPersonalChatHTTP builds the personal chat handler set.
func NewPersonalChatHTTP(cfg PersonalChatHTTPConfig) *PersonalChatHTTP {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &PersonalChatHTTP{chats: cfg.Chats, userID: cfg.UserID, logger: cfg.Logger}
}

// SetBroadcaster wires live agent-reply delivery to a broadcast sink (the
// gateway's Broadcast). Called after the gateway is constructed since the
// chat handler is built first. Personal mode is single-user, so broadcasting
// to every connected client is equivalent to targeting the one user.
func (h *PersonalChatHTTP) SetBroadcaster(b func(*Message)) { h.broadcast = b }

type messageView struct {
	ID         uuid.UUID `json:"id"`
	AuthorType string    `json:"authorType"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
}

type chatView struct {
	ID       uuid.UUID     `json:"id"`
	Messages []messageView `json:"messages"`
	// HasMore reports whether older messages exist before Messages[0]
	// (drives the scroll-back fetch).
	HasMore bool `json:"hasMore"`
}

type historyView struct {
	Messages []messageView `json:"messages"`
	HasMore  bool          `json:"hasMore"`
}

type sendChatRequest struct {
	Content string `json:"content"`
}

// sendChatResponse acknowledges the persisted user message. The agent reply
// is delivered asynchronously over the WebSocket (see SendHandler), not in
// this body.
type sendChatResponse struct {
	User messageView `json:"user"`
}

func toMessageView(m *ent.Message) messageView {
	return messageView{ID: m.ID, AuthorType: string(m.AuthorType), Content: m.Content, CreatedAt: m.CreatedAt}
}

// chatMessageEvent builds the WS event carrying an agent reply to live
// clients.
func chatMessageEvent(m messageView) *Message {
	return NewEventMessage("chat.message", "", map[string]interface{}{
		"id":         m.ID,
		"authorType": m.AuthorType,
		"content":    m.Content,
		"createdAt":  m.CreatedAt,
	})
}

// page fetches up to limit messages older than before (nil = newest page),
// oldest-first, plus a has-more flag. It over-fetches by one to detect more.
func (h *PersonalChatHTTP) page(ctx context.Context, chatID uuid.UUID, before *uuid.UUID, limit int) ([]messageView, bool, error) {
	msgs, err := h.chats.HistoryBefore(ctx, h.userID, chatID, before, limit+1)
	if err != nil {
		return nil, false, err
	}
	hasMore := false
	if len(msgs) > limit {
		hasMore = true
		msgs = msgs[len(msgs)-limit:] // keep the newest `limit`; drop the older probe
	}
	views := make([]messageView, len(msgs))
	for i, m := range msgs {
		views[i] = toMessageView(m)
	}
	return views, hasMore, nil
}

// ChatHandler serves GET /api/chat: the caller's private chat and its newest
// page of history (oldest-first), with hasMore driving scroll-back via
// HistoryHandler.
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
		views, hasMore, err := h.page(ctx, c.ID, nil, defaultChatPageSize)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, chatView{ID: c.ID, Messages: views, HasMore: hasMore})
	})
}

// HistoryHandler serves GET /api/chat/history?before=<id>&limit=<n>: a page
// of messages older than the cursor for scroll-back (keyset pagination).
func (h *PersonalChatHTTP) HistoryHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var before *uuid.UUID
		if s := r.URL.Query().Get("before"); s != "" {
			id, perr := uuid.Parse(s)
			if perr != nil {
				writeError(w, http.StatusBadRequest, "invalid before cursor")
				return
			}
			before = &id
		}
		limit := defaultChatPageSize
		if s := r.URL.Query().Get("limit"); s != "" {
			if n, perr := strconv.Atoi(s); perr == nil && n > 0 {
				limit = n
			}
		}
		if limit > maxChatPageSize {
			limit = maxChatPageSize
		}

		ctx := r.Context()
		c, err := h.chats.PrivateChat(ctx, h.userID)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		views, hasMore, err := h.page(ctx, c.ID, before, limit)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, historyView{Messages: views, HasMore: hasMore})
	})
}

// SendHandler serves POST /api/chat/messages: it persists the user's message
// and returns it immediately (202). The agent turn runs asynchronously and
// its reply is delivered over the WebSocket (chat.message event) — a slow
// LLM turn must not block the HTTP response.
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
		userMsg, err := h.chats.PostUserMessage(ctx, h.userID, c.ID, req.Content)
		if err != nil {
			if errors.Is(err, chats.ErrEmptyMessage) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			h.writeServiceError(w, err)
			return
		}

		// Run the agent turn out-of-band and deliver the reply over WS. The
		// context is detached from the request (WithoutCancel) so the slow
		// agent turn is not cancelled when this handler returns 202.
		go h.generateReply(context.WithoutCancel(ctx), c.ID, userMsg.Content)

		writeJSON(w, http.StatusAccepted, sendChatResponse{User: toMessageView(userMsg)})
	})
}

// generateReply runs the agent turn for a just-persisted user message and
// broadcasts the reply (or an error event) to live WS clients. It runs in
// its own goroutine on a detached context (see SendHandler) so it survives
// the HTTP request that triggered it.
func (h *PersonalChatHTTP) generateReply(ctx context.Context, chatID uuid.UUID, content string) {
	agentMsg, err := h.chats.GenerateReply(ctx, chatID, content)
	if err != nil {
		h.logger.Error("personal chat agent turn failed", "error", err)
		if h.broadcast != nil {
			h.broadcast(NewErrorMessage("", "agent reply failed"))
		}
		return
	}
	if h.broadcast != nil {
		h.broadcast(chatMessageEvent(toMessageView(agentMsg)))
	}
}

func (h *PersonalChatHTTP) writeServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, chats.ErrForbidden) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	h.logger.Error("personal chat request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
