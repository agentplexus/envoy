package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/plexusone/omniagent/internal/pgtest"
	"github.com/plexusone/omniagent/team"
	"github.com/plexusone/omniagent/team/auth"
	"github.com/plexusone/omniagent/team/mail"
	"github.com/plexusone/omniagent/team/store"
)

// teamHTTPFixture wires a real auth stack over PostgreSQL behind an
// httptest server, with a capturing mailer so tests can follow magic links.
type teamHTTPFixture struct {
	h      *TeamHTTP
	server *httptest.Server
	mailer *captureMailer
	client *http.Client
}

type captureMailer struct{ sent []mail.Message }

func (m *captureMailer) Send(_ context.Context, msg mail.Message) error {
	m.sent = append(m.sent, msg)
	return nil
}

func (m *captureMailer) lastToken(t *testing.T) string {
	t.Helper()
	if len(m.sent) == 0 {
		t.Fatal("no email captured")
	}
	body := m.sent[len(m.sent)-1].TextBody
	i := strings.Index(body, "token=")
	if i < 0 {
		t.Fatalf("no token in %q", body)
	}
	tok := body[i+len("token="):]
	if nl := strings.IndexAny(tok, "\r\n"); nl >= 0 {
		tok = tok[:nl]
	}
	return strings.TrimSpace(tok)
}

func setupTeamHTTP(t *testing.T) *teamHTTPFixture {
	t.Helper()
	return setupTeamHTTPMode(t, false)
}

// setupTeamHTTPMode is setupTeamHTTP with control over TeamHTTPConfig.Personal,
// so personal single-account mode (admin allowlist route excluded) can reuse
// the same fixture and helpers.
func setupTeamHTTPMode(t *testing.T, personal bool) *teamHTTPFixture {
	t.Helper()
	return setupTeamHTTPWithConfig(t, TeamHTTPConfig{CookieSecure: false, Personal: personal})
}

// setupTeamHTTPWithConfig is the full fixture builder; other setup helpers
// fill in a TeamHTTPConfig and delegate here. CookieSecure/Personal are
// taken from cfg as given; the caller sets whichever fields the test needs
// (e.g. GoogleProvider/GitHubProvider for SSO tests).
func setupTeamHTTPWithConfig(t *testing.T, cfg TeamHTTPConfig) *teamHTTPFixture {
	t.Helper()
	ownerDSN, appDSN := pgtest.DSNs(t)
	ctx := context.Background()

	storeCfg := store.Config{AppDSN: appDSN, MigrateDSN: ownerDSN}
	if err := store.Migrate(ctx, storeCfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st, err := store.Open(ctx, storeCfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	teamSvc, err := team.NewService(st, team.Config{SuperadminEmail: "root@example.com"})
	if err != nil {
		t.Fatalf("team.NewService: %v", err)
	}
	mailer := &captureMailer{}

	// BaseURL is set to the httptest server below via a placeholder we
	// rewrite after the server starts; auth only uses it to build links,
	// and the test extracts the token from the email regardless of host.
	authSvc, err := auth.NewService(st, teamSvc, mailer, auth.Config{BaseURL: "http://example.test"})
	if err != nil {
		t.Fatalf("auth.NewService: %v", err)
	}

	h := NewTeamHTTP(authSvc, teamSvc, cfg)
	// Speed up the anti-enumeration delay so tests are fast.
	h.limiter.baseDelay = 0
	h.limiter.maxDelay = 0

	srv := httptest.NewServer(h.Handler())
	t.Cleanup(srv.Close)

	jar, _ := newJar()
	return &teamHTTPFixture{
		h:      h,
		server: srv,
		mailer: mailer,
		client: &http.Client{
			Jar: jar,
			// Do not auto-follow the verify redirect; the test inspects it.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func newJar() (http.CookieJar, error) {
	// net/http/cookiejar without public-suffix handling is fine for tests.
	return cookieJar{m: map[string][]*http.Cookie{}}, nil
}

// cookieJar is a minimal same-host cookie jar sufficient for the test server.
type cookieJar struct{ m map[string][]*http.Cookie }

func (j cookieJar) SetCookies(u *url.URL, cs []*http.Cookie) { j.m[u.Host] = cs }
func (j cookieJar) Cookies(u *url.URL) []*http.Cookie        { return j.m[u.Host] }

func (f *teamHTTPFixture) post(t *testing.T, path, body string, csrf bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrf {
		req.Header.Set(csrfHeader, "1")
	}
	resp, err := f.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (f *teamHTTPFixture) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := f.client.Get(f.server.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (f *teamHTTPFixture) patch(t *testing.T, path, body string, csrf bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrf {
		req.Header.Set(csrfHeader, "1")
	}
	resp, err := f.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// me decodes /api/auth/me for the fixture's logged-in principal.
func (f *teamHTTPFixture) me(t *testing.T) struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Superadmin bool   `json:"superadmin"`
} {
	t.Helper()
	resp := f.get(t, "/api/auth/me")
	defer resp.Body.Close()
	var out struct {
		UserID     string `json:"user_id"`
		Username   string `json:"username"`
		Email      string `json:"email"`
		Superadmin bool   `json:"superadmin"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	return out
}

// loginKid allowlists and logs in a second ("kid") member principal sharing
// the fixture's server/mailer, mirroring TestTeamHTTP_CSRFAndAdminRBAC.
func loginKid(t *testing.T, f *teamHTTPFixture, email string) *teamHTTPFixture {
	t.Helper()
	body := `{"email":"` + email + `"}`
	resp := f.post(t, "/api/admin/allowlist", body, true)
	resp.Body.Close()
	f.post(t, "/api/auth/magic-link", body, false).Body.Close()
	kidToken := f.mailer.lastToken(t)

	jar, _ := newJar()
	kid := &teamHTTPFixture{server: f.server, mailer: f.mailer, client: &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
	kid.get(t, "/api/auth/verify?token="+kidToken).Body.Close()
	return kid
}

// loginSuperadmin logs the fixture's client in as the configured superadmin
// (root@example.com).
func loginSuperadmin(t *testing.T, f *teamHTTPFixture) {
	t.Helper()
	f.post(t, "/api/auth/magic-link", `{"email":"root@example.com"}`, false).Body.Close()
	f.get(t, "/api/auth/verify?token="+f.mailer.lastToken(t)).Body.Close()
}

func TestTeamHTTP_MagicLinkLoginFlow(t *testing.T) {
	f := setupTeamHTTP(t)

	// Superadmin requests a link (allowed without an allowlist entry).
	resp := f.post(t, "/api/auth/magic-link", `{"email":"root@example.com"}`, false)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("magic-link status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Follow the verify link: sets a cookie and 303-redirects.
	token := f.mailer.lastToken(t)
	resp = f.get(t, "/api/auth/verify?token="+token)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("verify status = %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); strings.Contains(loc, "error") {
		t.Fatalf("verify redirected to error: %s", loc)
	}
	resp.Body.Close()

	// /api/auth/me now returns the superadmin.
	resp = f.get(t, "/api/auth/me")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me status = %d, want 200", resp.StatusCode)
	}
	var me struct {
		Email      string `json:"email"`
		Superadmin bool   `json:"superadmin"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&me)
	resp.Body.Close()
	if me.Email != "root@example.com" || !me.Superadmin {
		t.Fatalf("me = %+v, want root superadmin", me)
	}
}

func TestTeamHTTP_UniformResponseAndAuthGuards(t *testing.T) {
	f := setupTeamHTTP(t)

	// Non-allowlisted and allowlisted requests are indistinguishable (200).
	r1 := f.post(t, "/api/auth/magic-link", `{"email":"stranger@example.com"}`, false)
	r2 := f.post(t, "/api/auth/magic-link", `{"email":"root@example.com"}`, false)
	if r1.StatusCode != http.StatusOK || r2.StatusCode != http.StatusOK {
		t.Fatalf("magic-link statuses = %d/%d, want 200/200", r1.StatusCode, r2.StatusCode)
	}
	r1.Body.Close()
	r2.Body.Close()

	// Unauthenticated /me is 401.
	resp := f.get(t, "/api/auth/me")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth me = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	// Unauthenticated allowlist is 401.
	resp = f.get(t, "/api/admin/allowlist")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth allowlist = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	// Invalid verify token redirects with an error param, no cookie.
	resp = f.get(t, "/api/auth/verify?token=bogus")
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "error") {
		t.Fatalf("bad verify: status=%d loc=%s", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp.Body.Close()
}

func TestTeamHTTP_CSRFAndAdminRBAC(t *testing.T) {
	f := setupTeamHTTP(t)

	// Log in as superadmin.
	f.post(t, "/api/auth/magic-link", `{"email":"root@example.com"}`, false).Body.Close()
	f.get(t, "/api/auth/verify?token="+f.mailer.lastToken(t)).Body.Close()

	// Allowlist POST without the CSRF header is rejected.
	resp := f.post(t, "/api/admin/allowlist", `{"email":"kid@example.com"}`, false)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("allowlist add without CSRF = %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()

	// With CSRF header it succeeds.
	resp = f.post(t, "/api/admin/allowlist", `{"email":"kid@example.com"}`, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("allowlist add with CSRF = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// It appears in the list.
	resp = f.get(t, "/api/admin/allowlist")
	var listResp struct {
		Allowlist []struct {
			Email string `json:"email"`
		} `json:"allowlist"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&listResp)
	resp.Body.Close()
	if len(listResp.Allowlist) != 1 || listResp.Allowlist[0].Email != "kid@example.com" {
		t.Fatalf("allowlist = %+v, want [kid@example.com]", listResp.Allowlist)
	}

	// The allowlisted kid can now log in — proving enforcement precedes issue.
	f.post(t, "/api/auth/magic-link", `{"email":"kid@example.com"}`, false).Body.Close()
	kidToken := f.mailer.lastToken(t)

	// A second client (the kid) logs in and is NOT a superadmin.
	jar, _ := newJar()
	kid := &teamHTTPFixture{server: f.server, mailer: f.mailer, client: &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
	kid.get(t, "/api/auth/verify?token="+kidToken).Body.Close()

	// The kid cannot read the allowlist (member, not superadmin).
	resp = kid.get(t, "/api/admin/allowlist")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("kid allowlist read = %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()

	// The kid can rename themselves (with CSRF).
	resp = kid.post(t, "/api/users/me/username", `{"username":"kiddo"}`, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("kid rename = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Logout clears the session.
	resp = kid.post(t, "/api/auth/logout", ``, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	resp = kid.get(t, "/api/auth/me")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-logout me = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestTeamHTTP_PersonalModeHidesAdminAllowlist confirms that Personal:true
// (personal single-account auth, TRD §4) never registers the allowlist
// admin route at all — not merely denies it — since the personal SQLite
// store has no row-level security and a second allowlisted account would
// see the sole account's data with no isolation.
func TestTeamHTTP_PersonalModeHidesAdminAllowlist(t *testing.T) {
	f := setupTeamHTTPMode(t, true)

	// Log in as the sole (superadmin) account.
	f.post(t, "/api/auth/magic-link", `{"email":"root@example.com"}`, false).Body.Close()
	f.get(t, "/api/auth/verify?token="+f.mailer.lastToken(t)).Body.Close()

	resp := f.get(t, "/api/admin/allowlist")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("personal-mode allowlist route = %d, want 404 (not registered)", resp.StatusCode)
	}
	resp.Body.Close()

	// The rest of the auth surface (login, me, rename, logout) still works.
	resp = f.get(t, "/api/auth/me")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestTeamHTTP_RequireAuth exercises the exported RequireAuth wrapper used
// to gate HTTP surfaces outside this package (the personal-mode chat API)
// behind the same session-cookie logic, without a route of its own.
func TestTeamHTTP_RequireAuth(t *testing.T) {
	f := setupTeamHTTPMode(t, true)

	var called bool
	f.h.mux.Handle("/api/probe", f.h.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})))

	resp := f.get(t, "/api/probe")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated probe = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()
	if called {
		t.Fatal("handler ran without authentication")
	}

	f.post(t, "/api/auth/magic-link", `{"email":"root@example.com"}`, false).Body.Close()
	f.get(t, "/api/auth/verify?token="+f.mailer.lastToken(t)).Body.Close()

	resp = f.get(t, "/api/probe")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated probe = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	if !called {
		t.Fatal("handler did not run after authentication")
	}
}

// TestTeamHTTP_AdminListUsers covers RMI-OMNIAGENT-119: the member list shows
// role/status/displayName, and only a superadmin may read it.
func TestTeamHTTP_AdminListUsers(t *testing.T) {
	f := setupTeamHTTP(t)
	loginSuperadmin(t, f)
	kid := loginKid(t, f, "kid@example.com")

	resp := f.get(t, "/api/admin/users")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list users = %d, want 200", resp.StatusCode)
	}
	var listResp struct {
		Users []userView `json:"users"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&listResp)
	resp.Body.Close()
	if len(listResp.Users) != 2 {
		t.Fatalf("users = %+v, want 2 rows", listResp.Users)
	}
	var root, member *userView
	for i := range listResp.Users {
		switch listResp.Users[i].Email {
		case "root@example.com":
			root = &listResp.Users[i]
		case "kid@example.com":
			member = &listResp.Users[i]
		}
	}
	if root == nil || root.Role != "superadmin" || root.Status != "active" {
		t.Fatalf("root row = %+v", root)
	}
	if member == nil || member.Role != "member" || member.Status != "active" {
		t.Fatalf("member row = %+v", member)
	}

	// A member (kid) cannot list users.
	resp = kid.get(t, "/api/admin/users")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("kid list users = %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestTeamHTTP_AdminSetUserStatus covers the disable/re-enable round trip,
// which was untested even at the service layer before this RMI.
func TestTeamHTTP_AdminSetUserStatus(t *testing.T) {
	f := setupTeamHTTP(t)
	loginSuperadmin(t, f)
	kid := loginKid(t, f, "kid@example.com")
	kidID := kid.me(t).UserID

	resp := f.patch(t, "/api/admin/users/"+kidID, `{"status":"disabled"}`, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("disable = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	if status := userStatus(t, f, kidID); status != "disabled" {
		t.Fatalf("status after disable = %q, want disabled", status)
	}

	resp = f.patch(t, "/api/admin/users/"+kidID, `{"status":"active"}`, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("re-enable = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	if status := userStatus(t, f, kidID); status != "active" {
		t.Fatalf("status after re-enable = %q, want active", status)
	}
}

// userStatus looks up a user's status via the admin list endpoint.
func userStatus(t *testing.T, f *teamHTTPFixture, userID string) string {
	t.Helper()
	resp := f.get(t, "/api/admin/users")
	defer resp.Body.Close()
	var listResp struct {
		Users []userView `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	for _, u := range listResp.Users {
		if u.ID.String() == userID {
			return u.Status
		}
	}
	t.Fatalf("user %s not found in list", userID)
	return ""
}

// TestTeamHTTP_AdminSetUserStatus_SelfLockout confirms a superadmin cannot
// disable their own account via the admin endpoint.
func TestTeamHTTP_AdminSetUserStatus_SelfLockout(t *testing.T) {
	f := setupTeamHTTP(t)
	loginSuperadmin(t, f)
	rootID := f.me(t).UserID

	resp := f.patch(t, "/api/admin/users/"+rootID, `{"status":"disabled"}`, true)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("self-disable = %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestTeamHTTP_AdminRenameOtherUser confirms a superadmin may rename any
// member via the admin endpoint (team.Service.RenameUser already allows
// this; this test covers the new HTTP exposure).
func TestTeamHTTP_AdminRenameOtherUser(t *testing.T) {
	f := setupTeamHTTP(t)
	loginSuperadmin(t, f)
	kid := loginKid(t, f, "kid@example.com")
	kidID := kid.me(t).UserID

	resp := f.patch(t, "/api/admin/users/"+kidID, `{"username":"renamed-kid"}`, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	if got := kid.me(t).Username; got != "renamed-kid" {
		t.Fatalf("kid username = %q, want renamed-kid", got)
	}
}

// TestTeamHTTP_AdminUsers_RequiresCSRF confirms the mutating admin/users
// route rejects requests without the CSRF header.
func TestTeamHTTP_AdminUsers_RequiresCSRF(t *testing.T) {
	f := setupTeamHTTP(t)
	loginSuperadmin(t, f)
	rootID := f.me(t).UserID

	resp := f.patch(t, "/api/admin/users/"+rootID, `{"username":"nope"}`, false)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("patch without CSRF = %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestTeamHTTP_PersonalModeHidesAdminUsers mirrors
// TestTeamHTTP_PersonalModeHidesAdminAllowlist for the new admin/users
// routes: they must not be registered at all in personal mode.
func TestTeamHTTP_PersonalModeHidesAdminUsers(t *testing.T) {
	f := setupTeamHTTPMode(t, true)
	loginSuperadmin(t, f)
	rootID := f.me(t).UserID

	resp := f.get(t, "/api/admin/users")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("personal-mode list users = %d, want 404 (not registered)", resp.StatusCode)
	}
	resp.Body.Close()

	resp = f.patch(t, "/api/admin/users/"+rootID, `{"username":"nope"}`, true)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("personal-mode update user = %d, want 404 (not registered)", resp.StatusCode)
	}
	resp.Body.Close()
}

// ---- SSO ---------------------------------------------------------------

// fakeSSOProvider implements SSOProvider without any real network access,
// so the gateway's state/nonce/redirect/error-routing logic is fully
// testable independent of real Google/GitHub connectivity.
type fakeSSOProvider struct {
	authURL       string
	exchangeSub   string
	exchangeEmail string
	exchangeErr   error
}

func (f *fakeSSOProvider) AuthURL(state, nonce string) string {
	return f.authURL + "?state=" + state + "&nonce=" + nonce
}

func (f *fakeSSOProvider) Exchange(_ context.Context, _, _ string) (string, string, error) {
	if f.exchangeErr != nil {
		return "", "", f.exchangeErr
	}
	return f.exchangeSub, f.exchangeEmail, nil
}

func setupTeamHTTPWithGoogle(t *testing.T, p SSOProvider) *teamHTTPFixture {
	t.Helper()
	return setupTeamHTTPWithConfig(t, TeamHTTPConfig{CookieSecure: false, GoogleProvider: p})
}

func TestTeamHTTP_SSORoutesAbsentWhenNotConfigured(t *testing.T) {
	f := setupTeamHTTP(t) // no providers configured

	for _, path := range []string{"/api/auth/google", "/api/auth/google/callback", "/api/auth/github", "/api/auth/github/callback"} {
		resp := f.get(t, path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s = %d, want 404 (not registered)", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestTeamHTTP_SSOStart_SetsCookiesAndRedirects(t *testing.T) {
	fake := &fakeSSOProvider{authURL: "https://fake-provider.example/authorize"}
	f := setupTeamHTTPWithGoogle(t, fake)

	resp := f.get(t, "/api/auth/google")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("start status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, fake.authURL) {
		t.Errorf("redirect location = %q, want prefix %q", loc, fake.authURL)
	}
	var sawState, sawNonce bool
	for _, c := range resp.Cookies() {
		switch c.Name {
		case ssoCookieName("google", "state", false):
			sawState = true
		case ssoCookieName("google", "nonce", false):
			sawNonce = true
		}
	}
	if !sawState || !sawNonce {
		t.Errorf("cookies = %v, want state and nonce cookies set", resp.Cookies())
	}
	resp.Body.Close()
}

func TestTeamHTTP_SSOCallback_MissingStateCookie(t *testing.T) {
	fake := &fakeSSOProvider{authURL: "https://fake-provider.example/authorize"}
	f := setupTeamHTTPWithGoogle(t, fake)

	// Hitting the callback directly, with no prior /api/auth/google visit,
	// means no state/nonce cookie exists.
	resp := f.get(t, "/api/auth/google/callback?state=bogus&code=abc")
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "error=sso_state") {
		t.Fatalf("status=%d location=%q, want 303 with error=sso_state", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp.Body.Close()
}

func TestTeamHTTP_SSOCallback_StateMismatch(t *testing.T) {
	fake := &fakeSSOProvider{authURL: "https://fake-provider.example/authorize"}
	f := setupTeamHTTPWithGoogle(t, fake)

	// Real start sets the cookies in the client's jar...
	f.get(t, "/api/auth/google").Body.Close()
	// ...but the callback is hit with a different state query param.
	resp := f.get(t, "/api/auth/google/callback?state=not-the-real-state&code=abc")
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "error=sso_state") {
		t.Fatalf("status=%d location=%q, want 303 with error=sso_state", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp.Body.Close()

	// No session was granted.
	resp = f.get(t, "/api/auth/me")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me after state mismatch = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()
}

// ssoStart performs a real start request and extracts the state query param
// from the redirect Location, for use as a valid callback ?state=.
func ssoStart(t *testing.T, f *teamHTTPFixture, provider string) string {
	t.Helper()
	resp := f.get(t, "/api/auth/"+provider)
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse redirect location %q: %v", loc, err)
	}
	return u.Query().Get("state")
}

func TestTeamHTTP_SSOCallback_ExchangeError(t *testing.T) {
	fake := &fakeSSOProvider{authURL: "https://fake-provider.example/authorize", exchangeErr: errTestExchange}
	f := setupTeamHTTPWithGoogle(t, fake)

	state := ssoStart(t, f, "google")
	resp := f.get(t, "/api/auth/google/callback?state="+state+"&code=abc")
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "error=sso_failed") {
		t.Fatalf("status=%d location=%q, want 303 with error=sso_failed", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp.Body.Close()
}

func TestTeamHTTP_SSOCallback_NotAllowlisted(t *testing.T) {
	fake := &fakeSSOProvider{authURL: "https://fake-provider.example/authorize", exchangeSub: "sub-1", exchangeEmail: "stranger@example.com"}
	f := setupTeamHTTPWithGoogle(t, fake)

	state := ssoStart(t, f, "google")
	resp := f.get(t, "/api/auth/google/callback?state="+state+"&code=abc")
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "error=not_allowed") {
		t.Fatalf("status=%d location=%q, want 303 with error=not_allowed", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp.Body.Close()
}

func TestTeamHTTP_SSOCallback_HappyPath(t *testing.T) {
	fake := &fakeSSOProvider{authURL: "https://fake-provider.example/authorize", exchangeSub: "sub-1", exchangeEmail: "root@example.com"}
	f := setupTeamHTTPWithGoogle(t, fake)

	// root@example.com is the configured superadmin — always allowed.
	state := ssoStart(t, f, "google")
	resp := f.get(t, "/api/auth/google/callback?state="+state+"&code=abc")
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != f.h.appURL("/") {
		t.Fatalf("status=%d location=%q, want 303 to /", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp.Body.Close()

	me := f.me(t)
	if me.Email != "root@example.com" || !me.Superadmin {
		t.Fatalf("me after SSO login = %+v, want root superadmin", me)
	}
}

func TestTeamHTTP_SSOCallback_LandsInExistingMagicLinkAccount(t *testing.T) {
	fake := &fakeSSOProvider{authURL: "https://fake-provider.example/authorize", exchangeSub: "sub-1", exchangeEmail: "kid@example.com"}
	f := setupTeamHTTPWithGoogle(t, fake)
	loginSuperadmin(t, f)

	// The superadmin allowlists kid, then kid logs in via magic link first.
	kid := loginKid(t, f, "kid@example.com")
	magicUserID := kid.me(t).UserID

	// kid's second client authenticates via (fake) Google SSO, same email.
	jar, _ := newJar()
	sso := &teamHTTPFixture{server: f.server, mailer: f.mailer, client: &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}

	state := ssoStart(t, sso, "google")
	resp := sso.get(t, "/api/auth/google/callback?state="+state+"&code=abc")
	resp.Body.Close()

	ssoUserID := sso.me(t).UserID
	if ssoUserID != magicUserID {
		t.Fatalf("SSO login user id = %s, want %s (same account as magic-link)", ssoUserID, magicUserID)
	}
}

var errTestExchange = errors.New("fake exchange failure")
