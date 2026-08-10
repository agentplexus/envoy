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

	"github.com/plexusone/omniagent/team/auth"
	"github.com/plexusone/omniagent/team/chats"
	"github.com/plexusone/omniagent/team/ent"
	entuser "github.com/plexusone/omniagent/team/ent/user"
	"github.com/plexusone/omniagent/team/store"
)

// fanoutCall records one membership-scoped broadcast.
type fanoutCall struct {
	userIDs []string
	msg     *Message
}

// setupTeamChatHTTP builds a team chat handler over a temp SQLite store, seeds
// alice (superadmin), bob and carol (members), and wires a buffered fan-out
// sink so tests can observe membership-scoped delivery.
func setupTeamChatHTTP(t *testing.T) (*TeamChatHTTP, map[string]uuid.UUID, <-chan fanoutCall) {
	t.Helper()
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "teamchat.db")
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

	ids := map[string]uuid.UUID{}
	seed := func(username string, role entuser.Role) {
		if err := st.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
			u, err := tx.User.Create().
				SetEmail(username + "@example.com").SetUsername(username).SetRole(role).Save(ctx)
			if err != nil {
				return err
			}
			ids[username] = u.ID
			return nil
		}); err != nil {
			t.Fatalf("seed %s: %v", username, err)
		}
	}
	seed("alice", entuser.RoleSuperadmin)
	seed("bob", entuser.RoleMember)
	seed("carol", entuser.RoleMember)

	chatSvc, err := chats.NewService(st, chats.Config{})
	if err != nil {
		t.Fatalf("chats.NewService: %v", err)
	}
	h := NewTeamChatHTTP(TeamChatHTTPConfig{Chats: chatSvc})
	calls := make(chan fanoutCall, 16)
	h.SetBroadcaster(func(userIDs []string, msg *Message) { calls <- fanoutCall{userIDs, msg} })
	return h, ids, calls
}

// asUser returns an *http.Request carrying the authenticated principal in
// context (as TeamHTTP.RequireAuth would) plus the CSRF header on mutations.
func asUser(method, target, body string, id uuid.UUID, super bool) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	if method != http.MethodGet {
		r.Header.Set(csrfHeader, "1")
	}
	p := &auth.Principal{UserID: id, Superadmin: super}
	return r.WithContext(context.WithValue(r.Context(), principalCtxKey{}, p))
}

func createGroup(t *testing.T, h *TeamChatHTTP, ownerID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodPost, "/api/chats", `{"name":"`+name+`"}`, ownerID, true))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create group %q status = %d, body = %s", name, rec.Code, rec.Body.String())
	}
	var got chatSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode group: %v", err)
	}
	return got.ID
}

func TestTeamChat_CreateListDetail(t *testing.T) {
	h, ids, _ := setupTeamChatHTTP(t)
	alice := ids["alice"]

	gid := createGroup(t, h, alice, "Family")

	// List shows the group.
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodGet, "/api/chats", "", alice, true))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var list struct {
		Chats []chatSummary `json:"chats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Chats) != 1 || list.Chats[0].ID != gid || list.Chats[0].Type != "group" {
		t.Fatalf("list = %+v, want single group %s", list.Chats, gid)
	}

	// Detail.
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodGet, "/api/chats/"+gid.String(), "", alice, true))
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d", rec.Code)
	}
	var detail chatDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.ID != gid || detail.Name != "Family" || len(detail.Messages) != 0 || detail.HasMore {
		t.Fatalf("detail = %+v, want empty Family group", detail)
	}
}

func TestTeamChat_InviteAndMembers(t *testing.T) {
	h, ids, calls := setupTeamChatHTTP(t)
	alice, bob := ids["alice"], ids["bob"]
	gid := createGroup(t, h, alice, "Trip")

	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodPost, "/api/chats/"+gid.String()+"/members", `{"username":"bob"}`, alice, true))
	if rec.Code != http.StatusOK {
		t.Fatalf("invite status = %d, body = %s", rec.Code, rec.Body.String())
	}
	// member.added is fanned out to the current members.
	select {
	case c := <-calls:
		if c.msg.Content != "chat.member.added" {
			t.Errorf("fanout event = %q, want chat.member.added", c.msg.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("no member.added fanout")
	}

	// Members lists both with usernames.
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodGet, "/api/chats/"+gid.String()+"/members", "", alice, true))
	var got struct {
		Members []chats.MemberView `json:"members"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode members: %v", err)
	}
	if len(got.Members) != 2 {
		t.Fatalf("members = %d, want 2", len(got.Members))
	}
	byName := map[string]chats.MemberView{}
	for _, m := range got.Members {
		byName[m.Username] = m
	}
	if byName["alice"].Role != "owner" || byName["bob"].Role != "member" || byName["bob"].UserID != bob {
		t.Fatalf("members = %+v, want alice owner + bob member", got.Members)
	}
}

func TestTeamChat_SendGroupFansOutNoAgentReply(t *testing.T) {
	h, ids, calls := setupTeamChatHTTP(t)
	alice, bob := ids["alice"], ids["bob"]
	gid := createGroup(t, h, alice, "Trip")

	// Invite bob (drains one member.added fanout).
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodPost, "/api/chats/"+gid.String()+"/members", `{"username":"bob"}`, alice, true))
	if rec.Code != http.StatusOK {
		t.Fatalf("invite status = %d", rec.Code)
	}
	<-calls

	// Alice sends a message.
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodPost, "/api/chats/"+gid.String()+"/messages", `{"content":"hello team"}`, alice, true))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("send status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var sent sendChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sent); err != nil {
		t.Fatalf("decode send: %v", err)
	}
	if sent.User.Content != "hello team" {
		t.Errorf("echoed user content = %q", sent.User.Content)
	}

	// Exactly one fanout: the user message to both members. No agent reply in
	// a group (RMI-113 territory).
	select {
	case c := <-calls:
		if c.msg.Content != "chat.message" {
			t.Fatalf("fanout event = %q, want chat.message", c.msg.Content)
		}
		if got, _ := c.msg.Data["content"].(string); got != "hello team" {
			t.Errorf("fanout content = %q", got)
		}
		set := map[string]bool{}
		for _, u := range c.userIDs {
			set[u] = true
		}
		if !set[alice.String()] || !set[bob.String()] || len(c.userIDs) != 2 {
			t.Errorf("fanout recipients = %v, want alice+bob", c.userIDs)
		}
	case <-time.After(time.Second):
		t.Fatal("no message fanout")
	}
	select {
	case c := <-calls:
		t.Fatalf("unexpected second fanout in a group: %q", c.msg.Content)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestTeamChat_PrivateDMGetsAgentReply(t *testing.T) {
	h, ids, calls := setupTeamChatHTTP(t)
	alice := ids["alice"]

	// Get-or-create the DM.
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodGet, "/api/chats/dm", "", alice, true))
	if rec.Code != http.StatusOK {
		t.Fatalf("dm status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var dm chatDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &dm); err != nil {
		t.Fatalf("decode dm: %v", err)
	}
	if dm.Type != "private" {
		t.Fatalf("dm type = %q, want private", dm.Type)
	}

	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodPost, "/api/chats/"+dm.ID.String()+"/messages", `{"content":"hi"}`, alice, true))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("send status = %d", rec.Code)
	}

	// First fanout: the user message.
	got := drain(t, calls, "user message")
	if c, _ := got.Data["content"].(string); c != "hi" {
		t.Errorf("user fanout content = %q", c)
	}
	// Second fanout: the async agent reply (echo, no agent configured).
	got = drain(t, calls, "agent reply")
	if c, _ := got.Data["authorType"].(string); c != "agent" {
		t.Errorf("reply authorType = %q, want agent", c)
	}
	if c, _ := got.Data["content"].(string); c != "Message received: hi" {
		t.Errorf("reply content = %q, want echo", c)
	}
}

func TestTeamChat_NonMemberForbidden(t *testing.T) {
	h, ids, _ := setupTeamChatHTTP(t)
	alice, bob := ids["alice"], ids["bob"]
	gid := createGroup(t, h, alice, "Private")

	// bob is not a member.
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodGet, "/api/chats/"+gid.String(), "", bob, false))
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-member detail status = %d, want 403", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodPost, "/api/chats/"+gid.String()+"/messages", `{"content":"intrusion"}`, bob, false))
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-member send status = %d, want 403", rec.Code)
	}
	// bob cannot invite into a group he isn't in.
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, asUser(http.MethodPost, "/api/chats/"+gid.String()+"/members", `{"username":"carol"}`, bob, false))
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-member invite status = %d, want 403", rec.Code)
	}
}

func TestTeamChat_CSRFRequiredOnMutations(t *testing.T) {
	h, ids, _ := setupTeamChatHTTP(t)
	alice := ids["alice"]

	// POST without the CSRF header (asUser sets it, so build by hand).
	r := httptest.NewRequest(http.MethodPost, "/api/chats", strings.NewReader(`{"name":"x"}`))
	p := &auth.Principal{UserID: alice, Superadmin: true}
	r = r.WithContext(context.WithValue(r.Context(), principalCtxKey{}, p))
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Errorf("create without CSRF status = %d, want 403", rec.Code)
	}
}

func TestTeamChat_Unauthenticated(t *testing.T) {
	h, _, _ := setupTeamChatHTTP(t)
	// No principal in context.
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/chats", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated list status = %d, want 401", rec.Code)
	}
}

// drain waits for one fan-out and returns its message, failing on timeout.
func drain(t *testing.T, calls <-chan fanoutCall, what string) *Message {
	t.Helper()
	select {
	case c := <-calls:
		return c.msg
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s fanout", what)
		return nil
	}
}
