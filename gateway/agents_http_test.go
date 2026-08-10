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

	"github.com/plexusone/omniagent/team/agents"
	"github.com/plexusone/omniagent/team/auth"
	"github.com/plexusone/omniagent/team/chats"
	"github.com/plexusone/omniagent/team/ent"
	entuser "github.com/plexusone/omniagent/team/ent/user"
	"github.com/plexusone/omniagent/team/store"
)

// setupAgentsHTTP builds an agents handler and an agent-aware team chat handler
// over one temp SQLite store, sharing the agents service as the chat's
// AgentGate so agent-bound chat starts can be exercised. Seeds alice
// (superadmin), bob and carol (members). The catalog's available-skills are
// {web-search, calculator}.
func setupAgentsHTTP(t *testing.T) (*AgentsHTTP, *TeamChatHTTP, map[string]uuid.UUID) {
	ah, ch, ids, _ := setupAgentsHTTPFull(t, nil)
	return ah, ch, ids
}

// setupAgentsHTTPFull is setupAgentsHTTP with a hook to tweak the
// AgentsHTTPConfig (e.g. wire the secrets surface) and access to the agents
// service. The base config always has Agents set.
func setupAgentsHTTPFull(t *testing.T, tweak func(*AgentsHTTPConfig)) (*AgentsHTTP, *TeamChatHTTP, map[string]uuid.UUID, *agents.Service) {
	t.Helper()
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "agents.db")
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

	agentsSvc, err := agents.NewService(st, agents.Config{
		AvailableSkills: []string{"web-search", "calculator"},
	})
	if err != nil {
		t.Fatalf("agents.NewService: %v", err)
	}
	cfg := AgentsHTTPConfig{Agents: agentsSvc}
	if tweak != nil {
		tweak(&cfg)
	}
	ah := NewAgentsHTTP(cfg)

	chatSvc, err := chats.NewService(st, chats.Config{Agents: agentsSvc})
	if err != nil {
		t.Fatalf("chats.NewService: %v", err)
	}
	ch := NewTeamChatHTTP(TeamChatHTTPConfig{Chats: chatSvc})

	return ah, ch, ids, agentsSvc
}

// do issues a request against a handler as the given user and returns the
// recorder.
func do(h http.Handler, method, target, body string, id uuid.UUID, super bool) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, asUser(method, target, body, id, super))
	return rec
}

// createAgent posts a new agent as owner and returns its ID.
func createAgent(t *testing.T, h *AgentsHTTP, owner uuid.UUID, slug string) uuid.UUID {
	t.Helper()
	rec := do(h.Handler(), http.MethodPost, "/api/agents",
		`{"slug":"`+slug+`","name":"`+slug+` bot","persona":"helpful"}`, owner, false)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create agent %q: status = %d, body = %s", slug, rec.Code, rec.Body.String())
	}
	var v agentView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode agent: %v", err)
	}
	return v.ID
}

func TestAgents_CreateListGet(t *testing.T) {
	h, _, ids := setupAgentsHTTP(t)
	bob := ids["bob"]

	id := createAgent(t, h, bob, "helper")

	// Creator sees it in their list.
	rec := do(h.Handler(), http.MethodGet, "/api/agents", "", bob, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var list struct {
		Agents []agentView `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Agents) != 1 || list.Agents[0].ID != id {
		t.Fatalf("list = %+v, want single agent %s", list.Agents, id)
	}

	// Detail carries caps: the creator is an owner, so all editor caps hold and
	// ManageMaintainers is true; Administer is false (bob is not superadmin).
	rec = do(h.Handler(), http.MethodGet, "/api/agents/"+id.String(), "", bob, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var d agentDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if d.Visibility != "private" {
		t.Errorf("visibility = %q, want private", d.Visibility)
	}
	if !d.Caps.Configure || !d.Caps.ManageMaintainers || !d.Caps.ManageRegistry {
		t.Errorf("owner caps = %+v, want all editor caps true", d.Caps)
	}
	if d.Caps.Administer {
		t.Error("bob is not superadmin; Administer should be false")
	}
	if len(d.AvailableSkills) != 2 {
		t.Errorf("availableSkills = %v, want 2", d.AvailableSkills)
	}
}

func TestAgents_CreateValidation(t *testing.T) {
	h, _, ids := setupAgentsHTTP(t)
	bob := ids["bob"]

	// Bad slug.
	rec := do(h.Handler(), http.MethodPost, "/api/agents", `{"slug":"No!","name":"x"}`, bob, false)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad slug status = %d, want 400", rec.Code)
	}
	// Duplicate slug → 409.
	createAgent(t, h, bob, "dup")
	rec = do(h.Handler(), http.MethodPost, "/api/agents", `{"slug":"dup","name":"other"}`, bob, false)
	if rec.Code != http.StatusConflict {
		t.Fatalf("dup slug status = %d, want 409", rec.Code)
	}
}

func TestAgents_StrangerCannotConfigure(t *testing.T) {
	// The SQLite test store has no row-level security, so read-visibility of a
	// private agent (GetAgent) is not enforced here — that is proven against
	// Postgres RLS in team/store. What the service layer enforces on its own,
	// and therefore what this HTTP surface must honor everywhere, is the
	// capability gate: a non-editor holds no editor capabilities and every
	// mutation is refused.
	h, _, ids := setupAgentsHTTP(t)
	bob, carol := ids["bob"], ids["carol"]

	id := createAgent(t, h, bob, "secret")

	// Carol holds no editor capabilities on the private agent.
	rec := do(h.Handler(), http.MethodGet, "/api/agents/"+id.String(), "", carol, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}
	var d agentDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if d.Caps.Configure || d.Caps.ManageMaintainers || d.Caps.ManageRegistry || d.Caps.Administer {
		t.Errorf("stranger caps = %+v, want all false", d.Caps)
	}

	// And every mutation is refused (requireEditor stands without RLS).
	rec = do(h.Handler(), http.MethodPatch, "/api/agents/"+id.String(), `{"name":"hijack"}`, carol, false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("stranger update status = %d, want 403", rec.Code)
	}
	rec = do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/skills", `{"skills":[]}`, carol, false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("stranger set-skills status = %d, want 403", rec.Code)
	}
}

func TestAgents_Skills(t *testing.T) {
	h, _, ids := setupAgentsHTTP(t)
	bob := ids["bob"]
	id := createAgent(t, h, bob, "skilled")

	// Set a valid subset.
	rec := do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/skills",
		`{"skills":["web-search"]}`, bob, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("set skills status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Skills []string `json:"skills"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode skills: %v", err)
	}
	if len(got.Skills) != 1 || got.Skills[0] != "web-search" {
		t.Fatalf("skills = %v, want [web-search]", got.Skills)
	}

	// Unknown skill → 400, nothing persisted.
	rec = do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/skills",
		`{"skills":["nope"]}`, bob, false)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown skill status = %d, want 400", rec.Code)
	}
}

func TestAgents_Maintainers(t *testing.T) {
	h, _, ids := setupAgentsHTTP(t)
	bob, carol := ids["bob"], ids["carol"]
	id := createAgent(t, h, bob, "team")

	// Owner adds carol as a maintainer.
	rec := do(h.Handler(), http.MethodPost, "/api/agents/"+id.String()+"/maintainers",
		`{"username":"carol"}`, bob, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("add maintainer status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Carol (maintainer) can configure but NOT manage maintainers.
	rec = do(h.Handler(), http.MethodGet, "/api/agents/"+id.String(), "", carol, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("maintainer detail status = %d", rec.Code)
	}
	var d agentDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !d.Caps.Configure || !d.Caps.ManageRegistry {
		t.Errorf("maintainer caps = %+v, want Configure+ManageRegistry", d.Caps)
	}
	if d.Caps.ManageMaintainers {
		t.Error("maintainer must not have ManageMaintainers")
	}

	// Carol cannot add another maintainer.
	rec = do(h.Handler(), http.MethodPost, "/api/agents/"+id.String()+"/maintainers",
		`{"username":"alice"}`, carol, false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("maintainer add-maintainer status = %d, want 403", rec.Code)
	}

	// Owner lists roles: two holders.
	rec = do(h.Handler(), http.MethodGet, "/api/agents/"+id.String()+"/roles", "", bob, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("roles status = %d", rec.Code)
	}
	var roles struct {
		Roles []agents.RoleView `json:"roles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &roles); err != nil {
		t.Fatalf("decode roles: %v", err)
	}
	if len(roles.Roles) != 2 {
		t.Fatalf("roles = %+v, want 2", roles.Roles)
	}

	// Owner removes carol.
	rec = do(h.Handler(), http.MethodDelete, "/api/agents/"+id.String()+"/maintainers/"+carol.String(), "", bob, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("remove maintainer status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAgents_VisibilityAndCatalog(t *testing.T) {
	h, _, ids := setupAgentsHTTP(t)
	bob, carol := ids["bob"], ids["carol"]
	id := createAgent(t, h, bob, "listme")

	// Private: not in carol's catalog.
	if got := catalogFor(t, h, carol); len(got.Featured)+len(got.Listed) != 0 {
		t.Fatalf("private agent leaked into catalog: %+v", got)
	}

	// Owner lists it.
	rec := do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/visibility",
		`{"visibility":"listed"}`, bob, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("set visibility status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Now carol sees it in Listed and may start a chat.
	got := catalogFor(t, h, carol)
	if len(got.Listed) != 1 || got.Listed[0].ID != id {
		t.Fatalf("catalog Listed = %+v, want agent %s", got.Listed, id)
	}
	if !got.Listed[0].CanStart {
		t.Error("listed agent should be startable by any user")
	}

	// A stranger cannot set visibility.
	rec = do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/visibility",
		`{"visibility":"private"}`, carol, false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("stranger set-visibility status = %d, want 403", rec.Code)
	}
	// Invalid visibility value → 400.
	rec = do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/visibility",
		`{"visibility":"public"}`, bob, false)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid visibility status = %d, want 400", rec.Code)
	}
}

func TestAgents_FeaturedSuperadminOnly(t *testing.T) {
	h, _, ids := setupAgentsHTTP(t)
	alice, bob := ids["alice"], ids["bob"]
	id := createAgent(t, h, bob, "star")

	// Owner (non-superadmin) cannot feature.
	rec := do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/featured", `{"featured":true}`, bob, false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("owner feature status = %d, want 403", rec.Code)
	}

	// Superadmin can.
	rec = do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/featured", `{"featured":true}`, alice, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("superadmin feature status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// It surfaces in the superadmin's Featured section.
	got := catalogFor(t, h, alice, true)
	if len(got.Featured) != 1 || got.Featured[0].ID != id || !got.Featured[0].Featured {
		t.Fatalf("catalog Featured = %+v, want featured agent %s", got.Featured, id)
	}
}

func TestAgents_StartAgentDMFromCatalog(t *testing.T) {
	h, ch, ids := setupAgentsHTTP(t)
	bob, carol := ids["bob"], ids["carol"]
	id := createAgent(t, h, bob, "chatme")

	// List it so carol may start a chat.
	if rec := do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/visibility",
		`{"visibility":"listed"}`, bob, false); rec.Code != http.StatusOK {
		t.Fatalf("list agent: %d", rec.Code)
	}

	// Carol starts an agent-bound DM through the chat surface.
	rec := do(ch.Handler(), http.MethodPost, "/api/chats/agents/"+id.String()+"/dm", "", carol, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("start agent dm status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var d chatDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode chat: %v", err)
	}
	if d.Type != "private" {
		t.Errorf("chat type = %q, want private", d.Type)
	}

	// Idempotent: a second start returns the same chat.
	rec = do(ch.Handler(), http.MethodPost, "/api/chats/agents/"+id.String()+"/dm", "", carol, false)
	var d2 chatDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &d2); err != nil {
		t.Fatalf("decode chat 2: %v", err)
	}
	if d2.ID != d.ID {
		t.Errorf("second DM id = %s, want same as %s", d2.ID, d.ID)
	}
}

func TestAgents_StartAgentDMForbiddenPrivate(t *testing.T) {
	h, ch, ids := setupAgentsHTTP(t)
	bob, carol := ids["bob"], ids["carol"]
	id := createAgent(t, h, bob, "hidden") // stays private

	// Carol is not an editor of a private agent → cannot start.
	rec := do(ch.Handler(), http.MethodPost, "/api/chats/agents/"+id.String()+"/dm", "", carol, false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("start private-agent dm status = %d, want 403", rec.Code)
	}
}

func TestAgents_CreateGroupWithAgent(t *testing.T) {
	h, ch, ids := setupAgentsHTTP(t)
	bob := ids["bob"]
	id := createAgent(t, h, bob, "groupbot")

	// Owner (editor) can start a group even while private.
	rec := do(ch.Handler(), http.MethodPost, "/api/chats/agents/"+id.String()+"/group",
		`{"name":"Ops Room"}`, bob, false)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create agent group status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var c chatSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode group: %v", err)
	}
	if c.Type != "group" || c.Name != "Ops Room" {
		t.Fatalf("group = %+v, want named group", c)
	}
}

func TestAgents_DeleteOwnerOnly(t *testing.T) {
	h, _, ids := setupAgentsHTTP(t)
	bob, carol := ids["bob"], ids["carol"]
	id := createAgent(t, h, bob, "trash")

	// Add carol as maintainer; a maintainer cannot delete.
	if rec := do(h.Handler(), http.MethodPost, "/api/agents/"+id.String()+"/maintainers",
		`{"username":"carol"}`, bob, false); rec.Code != http.StatusOK {
		t.Fatalf("add maintainer: %d", rec.Code)
	}
	rec := do(h.Handler(), http.MethodDelete, "/api/agents/"+id.String(), "", carol, false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("maintainer delete status = %d, want 403", rec.Code)
	}

	// Owner deletes.
	rec = do(h.Handler(), http.MethodDelete, "/api/agents/"+id.String(), "", bob, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner delete status = %d, body = %s", rec.Code, rec.Body.String())
	}
	// Gone.
	rec = do(h.Handler(), http.MethodGet, "/api/agents/"+id.String(), "", bob, false)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete status = %d, want 404", rec.Code)
	}
}

func TestAgents_CSRFRequiredOnMutations(t *testing.T) {
	h, _, ids := setupAgentsHTTP(t)
	bob := ids["bob"]

	// POST without the CSRF header (asUser adds it only for non-GET; build the
	// request by hand to omit it) but with the authenticated principal.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agents", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalCtxKey{}, &auth.Principal{UserID: bob}))
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d, want 403", rec.Code)
	}
}

// catalogFor fetches the catalog as the given user (optionally superadmin).
func catalogFor(t *testing.T, h *AgentsHTTP, id uuid.UUID, super ...bool) agents.Catalog {
	t.Helper()
	s := len(super) > 0 && super[0]
	rec := do(h.Handler(), http.MethodGet, "/api/catalog", "", id, s)
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got agents.Catalog
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	return got
}

// ---- Agent secrets (INIT-OMNIAGENT-004) ----------------------------------

// fakeSecretStore is an in-memory AgentSecretStore for tests: it records
// writes so tests can assert set/delete without a real vault.
type fakeSecretStore struct {
	data map[uuid.UUID]map[string]string
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{data: map[uuid.UUID]map[string]string{}}
}

func (f *fakeSecretStore) SetAgentSecret(_ context.Context, id uuid.UUID, name, value string) error {
	if f.data[id] == nil {
		f.data[id] = map[string]string{}
	}
	f.data[id][name] = value
	return nil
}

func (f *fakeSecretStore) DeleteAgentSecret(_ context.Context, id uuid.UUID, name string) error {
	delete(f.data[id], name)
	return nil
}

func (f *fakeSecretStore) ListAgentSecretNames(_ context.Context, id uuid.UUID) ([]string, error) {
	names := make([]string, 0, len(f.data[id]))
	for n := range f.data[id] {
		names = append(names, n)
	}
	return names, nil
}

// setupAgentsSecretsHTTP wires the agents handler with the secrets surface: a
// fake store, a declaration lookup that gives "web-search" one required secret
// GITHUB_TOKEN, and an invalidation spy.
func setupAgentsSecretsHTTP(t *testing.T) (*AgentsHTTP, map[string]uuid.UUID, *fakeSecretStore, *[]uuid.UUID) {
	t.Helper()
	store := newFakeSecretStore()
	var invalidated []uuid.UUID
	decls := map[string][]SecretDecl{
		"web-search": {{Name: "GITHUB_TOKEN", Description: "A PAT", Env: "GITHUB_TOKEN", Required: true}},
	}
	ah, _, ids, _ := setupAgentsHTTPFull(t, func(cfg *AgentsHTTPConfig) {
		cfg.Secrets = store
		cfg.SkillSecretDecls = func(name string) []SecretDecl { return decls[name] }
		cfg.InvalidateAgent = func(id uuid.UUID) { invalidated = append(invalidated, id) }
	})
	return ah, ids, store, &invalidated
}

func TestAgentSecrets_SetListDelete(t *testing.T) {
	h, ids, store, invalidated := setupAgentsSecretsHTTP(t)
	bob := ids["bob"]
	id := createAgent(t, h, bob, "sec")
	// Enable the secret-declaring skill.
	if rec := do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/skills",
		`{"skills":["web-search"]}`, bob, false); rec.Code != http.StatusOK {
		t.Fatalf("set skills: %d %s", rec.Code, rec.Body.String())
	}

	// Catalog shows the declared secret, unset.
	rec := do(h.Handler(), http.MethodGet, "/api/agents/"+id.String()+"/secrets", "", bob, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("list secrets: %d %s", rec.Code, rec.Body.String())
	}
	var cat struct {
		Secrets []secretView `json:"secrets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cat); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cat.Secrets) != 1 || cat.Secrets[0].Name != "GITHUB_TOKEN" || cat.Secrets[0].Set || !cat.Secrets[0].Required {
		t.Fatalf("catalog = %+v, want one unset required GITHUB_TOKEN", cat.Secrets)
	}

	// Set it.
	if rec := do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/secrets",
		`{"name":"GITHUB_TOKEN","value":"ghp_abc"}`, bob, false); rec.Code != http.StatusOK {
		t.Fatalf("set secret: %d %s", rec.Code, rec.Body.String())
	}
	if store.data[id]["GITHUB_TOKEN"] != "ghp_abc" {
		t.Errorf("value not stored: %v", store.data[id])
	}
	if len(*invalidated) != 1 || (*invalidated)[0] != id {
		t.Errorf("invalidate = %v, want [%s]", *invalidated, id)
	}

	// Catalog now shows it set, and never returns the value.
	rec = do(h.Handler(), http.MethodGet, "/api/agents/"+id.String()+"/secrets", "", bob, false)
	_ = json.Unmarshal(rec.Body.Bytes(), &cat)
	if !cat.Secrets[0].Set {
		t.Error("secret should be marked set")
	}
	if bytes := rec.Body.String(); contains(bytes, "ghp_abc") {
		t.Errorf("catalog leaked the secret value: %s", bytes)
	}

	// Delete it.
	if rec := do(h.Handler(), http.MethodDelete, "/api/agents/"+id.String()+"/secrets/GITHUB_TOKEN", "", bob, false); rec.Code != http.StatusOK {
		t.Fatalf("delete secret: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := store.data[id]["GITHUB_TOKEN"]; ok {
		t.Error("secret not deleted")
	}
	if len(*invalidated) != 2 {
		t.Errorf("invalidate calls = %d, want 2 (set + delete)", len(*invalidated))
	}
}

func TestAgentSecrets_StrangerForbidden(t *testing.T) {
	h, ids, store, _ := setupAgentsSecretsHTTP(t)
	bob, carol := ids["bob"], ids["carol"]
	id := createAgent(t, h, bob, "sec")

	// A non-owner/maintainer cannot read or write the agent's secrets.
	if rec := do(h.Handler(), http.MethodGet, "/api/agents/"+id.String()+"/secrets", "", carol, false); rec.Code != http.StatusForbidden {
		t.Errorf("stranger list = %d, want 403", rec.Code)
	}
	if rec := do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/secrets",
		`{"name":"GITHUB_TOKEN","value":"x"}`, carol, false); rec.Code != http.StatusForbidden {
		t.Errorf("stranger set = %d, want 403", rec.Code)
	}
	if len(store.data[id]) != 0 {
		t.Error("stranger write should not have stored anything")
	}
}

func TestAgentSecrets_ValidationAndCSRF(t *testing.T) {
	h, ids, _, _ := setupAgentsSecretsHTTP(t)
	bob := ids["bob"]
	id := createAgent(t, h, bob, "sec")

	// Empty value is rejected.
	if rec := do(h.Handler(), http.MethodPut, "/api/agents/"+id.String()+"/secrets",
		`{"name":"GITHUB_TOKEN","value":""}`, bob, false); rec.Code != http.StatusBadRequest {
		t.Errorf("empty value = %d, want 400", rec.Code)
	}

	// Missing CSRF header on a mutation is rejected.
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+id.String()+"/secrets", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalCtxKey{}, &auth.Principal{UserID: bob}))
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("missing CSRF = %d, want 403", rec.Code)
	}
}

func TestAgentSecrets_RoutesAbsentWithoutVault(t *testing.T) {
	h, _, ids := setupAgentsHTTP(t) // no Secrets configured
	bob := ids["bob"]
	id := createAgent(t, h, bob, "sec")
	if rec := do(h.Handler(), http.MethodGet, "/api/agents/"+id.String()+"/secrets", "", bob, false); rec.Code != http.StatusNotFound {
		t.Errorf("secrets route without vault = %d, want 404 (not registered)", rec.Code)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
