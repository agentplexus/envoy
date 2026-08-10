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

// TeamChatHTTP serves the multi-user (team-mode) chat surface: private DMs and
// group chats with membership fan-out (RMI-110/111/112). It is the team
// counterpart to PersonalChatHTTP; each request is scoped to the authenticated
// principal (set on the context by TeamHTTP.RequireAuth, which must wrap this
// handler) and enforced again by row-level security in the store.
//
// The agent turn follows the RMI-113 mention policy (chats.Service.AgentTurn):
// private chats always respond; group chats respond only when the message
// @-mentions the bound agent's slug. The reply runs on the chat's bound agent
// runtime and is persisted then broadcast. Per-agent runtime construction
// (persona + skills + secrets) is supplied by INIT-005 RMI-309; until a runtime
// is wired, agent-bound chats stay silent and agent-less DMs use the fallback.
type TeamChatHTTP struct {
	chats  *chats.Service
	logger *slog.Logger
	mux    *http.ServeMux

	// broadcast fans a message out to the given users' live sockets
	// (gateway.BroadcastToUsers). Nil until SetBroadcaster wires it; when nil
	// messages are still persisted and appear on the next history load.
	broadcast func(userIDs []string, msg *Message)
}

// TeamChatHTTPConfig configures the team chat handler.
type TeamChatHTTPConfig struct {
	Chats  *chats.Service
	Logger *slog.Logger
}

// NewTeamChatHTTP builds the team chat handler set.
func NewTeamChatHTTP(cfg TeamChatHTTPConfig) *TeamChatHTTP {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	h := &TeamChatHTTP{chats: cfg.Chats, logger: cfg.Logger}
	h.routes()
	return h
}

// SetBroadcaster wires membership-scoped live delivery (the gateway's
// BroadcastToUsers). Called after the gateway is constructed.
func (h *TeamChatHTTP) SetBroadcaster(b func(userIDs []string, msg *Message)) { h.broadcast = b }

// Handler returns the routed handler. Mount it at "/api/chats" and
// "/api/chats/", wrapped in TeamHTTP.RequireAuth so every route has an
// authenticated principal on its context.
func (h *TeamChatHTTP) Handler() http.Handler { return h.mux }

func (h *TeamChatHTTP) routes() {
	h.mux = http.NewServeMux()
	// The literal "dm" segment is more specific than "{id}", so it wins for
	// GET /api/chats/dm even though {id} would otherwise match.
	h.mux.HandleFunc("GET /api/chats", h.handleList)
	h.mux.HandleFunc("POST /api/chats", h.handleCreateGroup)
	h.mux.HandleFunc("GET /api/chats/dm", h.handleDM)
	h.mux.HandleFunc("GET /api/chats/{id}", h.handleChat)
	h.mux.HandleFunc("GET /api/chats/{id}/messages", h.handleHistory)
	h.mux.HandleFunc("POST /api/chats/{id}/messages", h.handleSend)
	h.mux.HandleFunc("GET /api/chats/{id}/members", h.handleMembers)
	h.mux.HandleFunc("POST /api/chats/{id}/members", h.handleInvite)
	h.mux.HandleFunc("DELETE /api/chats/{id}/members/{userID}", h.handleRemoveMember)
	h.mux.HandleFunc("POST /api/chats/{id}/leave", h.handleLeave)
}

// ---- Views ---------------------------------------------------------------

type chatSummary struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type chatDetail struct {
	ID       uuid.UUID     `json:"id"`
	Type     string        `json:"type"`
	Name     string        `json:"name,omitempty"`
	Messages []messageView `json:"messages"`
	HasMore  bool          `json:"hasMore"`
}

// ---- Handlers ------------------------------------------------------------

func (h *TeamChatHTTP) handleList(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	cs, err := h.chats.ListChats(r.Context(), actor)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	out := make([]chatSummary, len(cs))
	for i, c := range cs {
		out[i] = chatSummary{ID: c.ID, Type: c.Type.String(), Name: c.Name, CreatedAt: c.CreatedAt}
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": out})
}

type createGroupRequest struct {
	Name string `json:"name"`
}

func (h *TeamChatHTTP) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	var req createGroupRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	c, err := h.chats.CreateGroup(r.Context(), actor, req.Name)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, chatSummary{ID: c.ID, Type: c.Type.String(), Name: c.Name, CreatedAt: c.CreatedAt})
}

// handleDM get-or-creates the caller's private DM with the agent and returns
// its newest page (mirrors personal mode's GET /api/chat).
func (h *TeamChatHTTP) handleDM(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	c, err := h.chats.PrivateChat(r.Context(), actor.UserID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	views, hasMore, err := h.page(r.Context(), actor.UserID, c.ID, nil, defaultChatPageSize)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, chatDetail{ID: c.ID, Type: c.Type.String(), Name: c.Name, Messages: views, HasMore: hasMore})
}

func (h *TeamChatHTTP) handleChat(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	chatID, ok := h.chatID(w, r)
	if !ok {
		return
	}
	c, err := h.chats.GetChat(r.Context(), actor, chatID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	views, hasMore, err := h.page(r.Context(), actor.UserID, chatID, nil, defaultChatPageSize)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, chatDetail{ID: c.ID, Type: c.Type.String(), Name: c.Name, Messages: views, HasMore: hasMore})
}

func (h *TeamChatHTTP) handleHistory(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	chatID, ok := h.chatID(w, r)
	if !ok {
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
	views, hasMore, err := h.page(r.Context(), actor.UserID, chatID, before, limit)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, historyView{Messages: views, HasMore: hasMore})
}

func (h *TeamChatHTTP) handleSend(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	chatID, ok := h.chatID(w, r)
	if !ok {
		return
	}
	var req sendChatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, chats.MaxMessageBytes+1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	// GetChat both membership-checks the actor and gives us the chat (its type
	// and agent binding) that AgentTurn needs to apply the mention policy.
	c, err := h.chats.GetChat(ctx, actor, chatID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	userMsg, err := h.chats.PostUserMessage(ctx, actor.UserID, chatID, req.Content)
	if err != nil {
		if errors.Is(err, chats.ErrEmptyMessage) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeServiceError(w, err)
		return
	}

	// Fan the user's message out to every member's live sockets.
	recipients := h.recipients(ctx, actor, chatID)
	h.fanout(recipients, teamChatMessageEvent(chatID, toMessageView(userMsg)))

	// Run the agent turn out-of-band on a detached context: private chats
	// always respond; group chats respond only when the message @-mentions the
	// bound agent (RMI-113). AgentTurn decides and stays silent otherwise.
	go h.generateReply(context.WithoutCancel(ctx), c, userMsg.Content, recipients)

	writeJSON(w, http.StatusAccepted, sendChatResponse{User: toMessageView(userMsg)})
}

func (h *TeamChatHTTP) handleMembers(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	chatID, ok := h.chatID(w, r)
	if !ok {
		return
	}
	members, err := h.chats.MembersDetailed(r.Context(), actor, chatID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

type inviteRequest struct {
	Username string `json:"username"`
}

func (h *TeamChatHTTP) handleInvite(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	chatID, ok := h.chatID(w, r)
	if !ok {
		return
	}
	var req inviteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	m, err := h.chats.Invite(r.Context(), actor, chatID, req.Username)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	// Notify existing members (incl. the new one if already connected).
	h.fanout(h.recipients(r.Context(), actor, chatID), NewEventMessage("chat.member.added", chatID.String(), map[string]any{
		"chatId": chatID,
		"userId": m.UserID,
		"role":   m.Role.String(),
	}))
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "userId": m.UserID, "role": m.Role.String()})
}

func (h *TeamChatHTTP) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	chatID, ok := h.chatID(w, r)
	if !ok {
		return
	}
	memberID, perr := uuid.Parse(r.PathValue("userID"))
	if perr != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	// Snapshot recipients before removal so the removed member is notified too.
	recipients := h.recipients(r.Context(), actor, chatID)
	if err := h.chats.RemoveMember(r.Context(), actor, chatID, memberID); err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.fanout(recipients, NewEventMessage("chat.member.removed", chatID.String(), map[string]any{
		"chatId": chatID,
		"userId": memberID,
	}))
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *TeamChatHTTP) handleLeave(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	chatID, ok := h.chatID(w, r)
	if !ok {
		return
	}
	recipients := h.recipients(r.Context(), actor, chatID)
	if err := h.chats.Leave(r.Context(), actor, chatID); err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.fanout(recipients, NewEventMessage("chat.member.removed", chatID.String(), map[string]any{
		"chatId": chatID,
		"userId": actor.UserID,
	}))
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// ---- Internals -----------------------------------------------------------

// page fetches up to limit messages older than before (nil = newest page),
// oldest-first, plus a has-more flag (over-fetches by one to detect more).
func (h *TeamChatHTTP) page(ctx context.Context, userID, chatID uuid.UUID, before *uuid.UUID, limit int) ([]messageView, bool, error) {
	msgs, err := h.chats.HistoryBefore(ctx, userID, chatID, before, limit+1)
	if err != nil {
		return nil, false, err
	}
	hasMore := false
	if len(msgs) > limit {
		hasMore = true
		msgs = msgs[len(msgs)-limit:]
	}
	views := make([]messageView, len(msgs))
	for i, m := range msgs {
		views[i] = toMessageView(m)
	}
	return views, hasMore, nil
}

// generateReply runs the agent turn for a chat and fans the reply out to the
// chat's members. The mention policy lives in AgentTurn: when the agent stays
// silent (a group message with no @-mention, or an agent-bound chat with no
// runtime wired) it reports responded=false and nothing is broadcast. Runs in
// its own goroutine on a detached context (see handleSend).
func (h *TeamChatHTTP) generateReply(ctx context.Context, c *ent.Chat, content string, recipients []string) {
	agentMsg, responded, err := h.chats.AgentTurn(ctx, c, content)
	if err != nil {
		h.logger.Error("team chat agent turn failed", "error", err, "chat", c.ID)
		h.fanout(recipients, NewErrorMessage("", "agent reply failed"))
		return
	}
	if !responded {
		return
	}
	h.fanout(recipients, teamChatMessageEvent(c.ID, toMessageView(agentMsg)))
}

// recipients returns the string user IDs of a chat's members, for fan-out.
// Failure is logged and yields no recipients — delivery is best-effort; the
// message is already persisted and will show up on the next history load.
func (h *TeamChatHTTP) recipients(ctx context.Context, actor chats.Actor, chatID uuid.UUID) []string {
	ids, err := h.chats.MemberUserIDs(ctx, actor, chatID)
	if err != nil {
		h.logger.Error("resolve chat recipients", "error", err, "chat", chatID)
		return nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func (h *TeamChatHTTP) fanout(recipients []string, msg *Message) {
	if h.broadcast != nil && len(recipients) > 0 {
		h.broadcast(recipients, msg)
	}
}

// actor resolves the authenticated principal (set by RequireAuth) to a chats
// Actor; responds 401 if absent.
func (h *TeamChatHTTP) actor(w http.ResponseWriter, r *http.Request) (chats.Actor, bool) {
	p := principalFrom(r.Context())
	if p == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return chats.Actor{}, false
	}
	return chats.Actor{UserID: p.UserID, Superadmin: p.Superadmin}, true
}

// actorCSRF is actor plus the custom-header CSRF check required on mutations.
func (h *TeamChatHTTP) actorCSRF(w http.ResponseWriter, r *http.Request) (chats.Actor, bool) {
	if !hasCSRF(r) {
		writeError(w, http.StatusForbidden, "missing CSRF header")
		return chats.Actor{}, false
	}
	return h.actor(w, r)
}

func (h *TeamChatHTTP) chatID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chat id")
		return uuid.UUID{}, false
	}
	return id, true
}

func (h *TeamChatHTTP) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, chats.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, chats.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, chats.ErrNotGroup):
		writeError(w, http.StatusBadRequest, "chat is not a group")
	case errors.Is(err, chats.ErrEmptyName):
		writeError(w, http.StatusBadRequest, "group name is required")
	case errors.Is(err, chats.ErrEmptyMessage):
		writeError(w, http.StatusBadRequest, "message content is empty")
	case errors.Is(err, chats.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, chats.ErrLastOwner):
		writeError(w, http.StatusConflict, "you are the group's only owner")
	default:
		h.logger.Error("team chat request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

// teamChatMessageEvent builds the WS event carrying a chat message to live
// members. The channel and payload carry the chat ID so a client can route it
// to the right room.
func teamChatMessageEvent(chatID uuid.UUID, m messageView) *Message {
	return NewEventMessage("chat.message", chatID.String(), map[string]interface{}{
		"chatId":       chatID,
		"id":           m.ID,
		"authorType":   m.AuthorType,
		"authorUserId": m.AuthorUserID,
		"content":      m.Content,
		"createdAt":    m.CreatedAt,
	})
}
