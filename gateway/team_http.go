package gateway

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team"
	"github.com/plexusone/omniagent/team/auth"
	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/identity"
	entuser "github.com/plexusone/omniagent/team/ent/user"
)

// csrfHeader must be present on state-changing team requests. Combined with a
// SameSite=Lax session cookie this defeats cross-site form submissions: a
// cross-origin page cannot set a custom request header without a CORS
// preflight the server never approves.
const csrfHeader = "X-OmniAgent-CSRF"

// principalCtxKey carries the authenticated principal through the request.
type principalCtxKey struct{}

// SSOProvider drives one OAuth/OIDC sign-in provider's browser-facing
// redirect flow. Symmetric across providers — GitHub's implementation
// ignores nonce — so the gateway's start/callback handling is one shared
// implementation with no per-provider branching, and is fully testable with
// a fake implementation independent of real provider connectivity.
type SSOProvider interface {
	// AuthURL returns the redirect URL for the provider's consent screen,
	// carrying state (CSRF) and nonce (OIDC replay protection).
	AuthURL(state, nonce string) string
	// Exchange trades an authorization code for the provider's verified
	// subject (a stable per-account id) and verified email. nonce is echoed
	// back for providers that need it (Google); others ignore it.
	Exchange(ctx context.Context, code, nonce string) (subject, verifiedEmail string, err error)
}

// TeamHTTPConfig configures the team HTTP handler.
type TeamHTTPConfig struct {
	// CookieSecure sets the Secure attribute and selects the cookie name:
	// __Host-oa_session (secure) or oa_session (dev/plain HTTP).
	CookieSecure bool
	// SessionTTL bounds the cookie Max-Age (mirrors auth.Config.SessionTTL).
	SessionTTL time.Duration
	// BaseURL is the origin verify redirects land on.
	BaseURL string
	Logger  *slog.Logger

	// Personal, when true, serves personal single-account auth
	// (auth.enabled=true, team.enabled=false — TRD §4) instead of full
	// team mode: the admin allowlist endpoint is not registered. It must
	// stay unreachable in this mode because the personal SQLite store has
	// no row-level security — a second allowlisted account would see the
	// sole account's data with no isolation.
	Personal bool

	// GoogleProvider and GitHubProvider are nil-able: nil means that
	// provider is not configured, and its /api/auth/{provider}* routes are
	// not registered at all (mirrors the Personal-mode convention for
	// /api/admin/*).
	GoogleProvider SSOProvider
	GitHubProvider SSOProvider
}

// TeamHTTP serves the team auth and admin API.
type TeamHTTP struct {
	auth     *auth.Service
	team     *team.Service
	cfg      TeamHTTPConfig
	limiter  *authFailureLimiter
	cookieNm string
	logger   *slog.Logger
	mux      *http.ServeMux
}

// NewTeamHTTP builds the team HTTP handler.
func NewTeamHTTP(authSvc *auth.Service, teamSvc *team.Service, cfg TeamHTTPConfig) *TeamHTTP {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = auth.DefaultSessionTTL
	}
	h := &TeamHTTP{
		auth:     authSvc,
		team:     teamSvc,
		cfg:      cfg,
		limiter:  newAuthFailureLimiter(),
		cookieNm: cookieName(cfg.CookieSecure),
		logger:   cfg.Logger,
	}
	h.routes()
	return h
}

// cookieName picks the session cookie name. The __Host- prefix requires the
// Secure attribute, so plain-HTTP dev uses the unprefixed name.
func cookieName(secure bool) string {
	if secure {
		return "__Host-oa_session"
	}
	return "oa_session"
}

// Handler returns the routed http.Handler (mount at "/api/").
func (h *TeamHTTP) Handler() http.Handler { return h.mux }

// ConnectAuthorizer returns a gateway ConnectAuthorizer that authenticates a
// WebSocket upgrade from the session cookie.
func (h *TeamHTTP) ConnectAuthorizer() ConnectAuthorizer {
	return func(r *http.Request) (string, bool) {
		p, err := h.principalFromRequest(r)
		if err != nil {
			return "", false
		}
		return p.UserID.String(), true
	}
}

// RequireAuth wraps next, requiring a valid session cookie before it runs
// (401 otherwise). It lets HTTP surfaces outside this package (e.g. the
// personal-mode chat API) gate themselves on the same cookie/session logic
// without duplicating it.
func (h *TeamHTTP) RequireAuth(next http.Handler) http.Handler {
	return h.requireAuth(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r) })
}

func (h *TeamHTTP) routes() {
	h.mux = http.NewServeMux()
	// Auth
	h.mux.HandleFunc("/api/auth/magic-link", h.handleMagicLink)   // POST
	h.mux.HandleFunc("/api/auth/verify", h.handleVerify)          // GET
	h.mux.HandleFunc("/api/auth/password", h.handlePasswordLogin) // POST
	h.mux.HandleFunc("/api/auth/logout", h.requireCSRF(h.handleLogout))
	h.mux.HandleFunc("/api/auth/me", h.requireAuth(h.handleMe)) // GET
	// Self-service
	h.mux.HandleFunc("/api/users/me/username", h.requireCSRF(h.requireAuth(h.handleRename)))
	h.mux.HandleFunc("/api/users/me/password", h.requireCSRF(h.requireAuth(h.handleChangePassword)))
	// SSO — each provider's routes are only registered when configured.
	if h.cfg.GoogleProvider != nil {
		h.mux.HandleFunc("GET /api/auth/google", h.handleSSOStart("google", h.cfg.GoogleProvider))
		h.mux.HandleFunc("GET /api/auth/google/callback", h.handleSSOCallback("google", identity.ProviderGoogle, h.cfg.GoogleProvider))
	}
	if h.cfg.GitHubProvider != nil {
		h.mux.HandleFunc("GET /api/auth/github", h.handleSSOStart("github", h.cfg.GitHubProvider))
		h.mux.HandleFunc("GET /api/auth/github/callback", h.handleSSOCallback("github", identity.ProviderGithub, h.cfg.GitHubProvider))
	}
	// Admin (superadmin only) — excluded in personal mode, see
	// TeamHTTPConfig.Personal.
	if !h.cfg.Personal {
		h.mux.HandleFunc("/api/admin/allowlist", h.handleAllowlist)
		h.mux.HandleFunc("GET /api/admin/users", h.handleListUsers)
		h.mux.HandleFunc("PATCH /api/admin/users/{id}", h.handleUpdateUser)
	}
}

// ---- Auth endpoints ------------------------------------------------------

type magicLinkRequest struct {
	Email string `json:"email"`
}

func (h *TeamHTTP) handleMagicLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req magicLinkRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ip := clientIP(r)

	// Escalating delay per source throttles enumeration/spam, keyed by both
	// client IP and email (TRD §4.1): IP alone misses a distributed probe
	// against one target address, email alone misses a single-source spray
	// across many addresses. Applied before the (uniform) response;
	// loopback is delayed but never locked.
	keys := []string{"magic:ip:" + ip}
	if email := strings.ToLower(strings.TrimSpace(req.Email)); email != "" {
		keys = append(keys, "magic:email:"+email)
	}
	_ = h.limiter.recordFailureAndDelayAll(r.Context(), keys...) //nolint:errcheck // delay is best-effort; ctx cancel just skips it

	err := h.auth.RequestMagicLink(r.Context(), req.Email, ip)
	if errors.Is(err, team.ErrInvalidEmail) {
		writeError(w, http.StatusBadRequest, "invalid email address")
		return
	}
	// Uniform success regardless of allowlist membership.
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": "If that email is permitted, a sign-in link has been sent.",
	})
}

func (h *TeamHTTP) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token := r.URL.Query().Get("token")
	sessionToken, _, err := h.auth.VerifyMagicLink(r.Context(), token)
	if err != nil {
		// Redirect to the app with an error so the SPA can show a message.
		http.Redirect(w, r, h.appURL("/?error=invalid_link"), http.StatusSeeOther)
		return
	}
	h.setSessionCookie(w, sessionToken)
	http.Redirect(w, r, h.appURL("/"), http.StatusSeeOther)
}

// ---- SSO -------------------------------------------------------------

// ssoStateTTL bounds how long a start request's state/nonce cookies remain
// valid — long enough for a real consent-screen round trip, short enough to
// limit exposure if abandoned mid-flow.
const ssoStateTTL = 10 * time.Minute

// ssoCookieName builds the state/nonce cookie name for one provider,
// following the same __Host- prefixing convention as the session cookie.
func ssoCookieName(provider, kind string, secure bool) string {
	prefix := "oa_sso_"
	if secure {
		prefix = "__Host-oa_sso_"
	}
	return prefix + provider + "_" + kind
}

// randomSSOToken returns a URL-safe random value for state/nonce use.
func randomSSOToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// handleSSOStart redirects the browser to the provider's consent screen,
// after stamping state (CSRF) and nonce (OIDC replay protection) as
// short-lived cookies the callback reads back and clears.
func (h *TeamHTTP) handleSSOStart(name string, p SSOProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := randomSSOToken()
		if err != nil {
			h.logger.Error("sso: generate state failed", "provider", name, "error", err)
			http.Redirect(w, r, h.appURL("/?error=sso_failed"), http.StatusSeeOther)
			return
		}
		nonce, err := randomSSOToken()
		if err != nil {
			h.logger.Error("sso: generate nonce failed", "provider", name, "error", err)
			http.Redirect(w, r, h.appURL("/?error=sso_failed"), http.StatusSeeOther)
			return
		}
		h.setSSOCookie(w, name, "state", state)
		h.setSSOCookie(w, name, "nonce", nonce)
		http.Redirect(w, r, p.AuthURL(state, nonce), http.StatusFound)
	}
}

// handleSSOCallback validates the state/nonce cookies, exchanges the
// authorization code, resolves the identity to a session via
// auth.Service.CompleteSSOLogin, and redirects — mirroring handleVerify's
// success/failure redirect shape.
func (h *TeamHTTP) handleSSOCallback(name string, provider identity.Provider, p SSOProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, hasState := h.readAndClearSSOCookie(w, r, name, "state")
		nonce, hasNonce := h.readAndClearSSOCookie(w, r, name, "nonce")
		if !hasState || !hasNonce || r.URL.Query().Get("state") != state {
			http.Redirect(w, r, h.appURL("/?error=sso_state"), http.StatusSeeOther)
			return
		}

		subject, verifiedEmail, err := p.Exchange(r.Context(), r.URL.Query().Get("code"), nonce)
		if err != nil {
			h.logger.Error("sso: exchange failed", "provider", name, "error", err)
			http.Redirect(w, r, h.appURL("/?error=sso_failed"), http.StatusSeeOther)
			return
		}

		sessionToken, _, err := h.auth.CompleteSSOLogin(r.Context(), provider, subject, verifiedEmail, r.UserAgent(), clientIP(r))
		switch {
		case errors.Is(err, auth.ErrNotAllowed):
			http.Redirect(w, r, h.appURL("/?error=not_allowed"), http.StatusSeeOther)
			return
		case errors.Is(err, auth.ErrAccountDisabled):
			http.Redirect(w, r, h.appURL("/?error=disabled"), http.StatusSeeOther)
			return
		case err != nil:
			h.logger.Error("sso: login failed", "provider", name, "error", err)
			http.Redirect(w, r, h.appURL("/?error=sso_failed"), http.StatusSeeOther)
			return
		}

		h.setSessionCookie(w, sessionToken)
		http.Redirect(w, r, h.appURL("/"), http.StatusSeeOther)
	}
}

func (h *TeamHTTP) setSSOCookie(w http.ResponseWriter, provider, kind, value string) {
	//nolint:gosec // G124: Secure mirrors CookieSecure, same as the session cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     ssoCookieName(provider, kind, h.cfg.CookieSecure),
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ssoStateTTL.Seconds()),
	})
}

// readAndClearSSOCookie reads a state/nonce cookie and immediately expires
// it (single-use, matching the magic-link token's consume-once semantics).
func (h *TeamHTTP) readAndClearSSOCookie(w http.ResponseWriter, r *http.Request, provider, kind string) (string, bool) {
	name := ssoCookieName(provider, kind, h.cfg.CookieSecure)
	c, err := r.Cookie(name)
	//nolint:gosec // G124: Secure mirrors CookieSecure; this is a deletion cookie (MaxAge<0).
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

type passwordLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// handlePasswordLogin authenticates an email+password credential and sets the
// session cookie. Rate-limited by client IP and email (like magic-link) and
// returns a uniform 401 on any failure to avoid account enumeration.
func (h *TeamHTTP) handlePasswordLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req passwordLoginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ip := clientIP(r)
	keys := []string{"pw:ip:" + ip}
	if email := strings.ToLower(strings.TrimSpace(req.Email)); email != "" {
		keys = append(keys, "pw:email:"+email)
	}
	_ = h.limiter.recordFailureAndDelayAll(r.Context(), keys...) //nolint:errcheck // delay is best-effort; ctx cancel just skips it

	sessionToken, _, err := h.auth.LoginWithPassword(r.Context(), req.Email, req.Password, r.UserAgent(), ip)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	h.setSessionCookie(w, sessionToken)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// handleChangePassword lets the authenticated user set or change their own
// password. Requires the current password when one is already set.
func (h *TeamHTTP) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p := principalFrom(r.Context())
	var req changePasswordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.auth.SetPassword(r.Context(), p.Actor(), p.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *TeamHTTP) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if c, err := r.Cookie(h.cookieNm); err == nil {
		if lerr := h.auth.Logout(r.Context(), c.Value); lerr != nil {
			h.logger.Error("logout revoke failed", "error", lerr)
		}
	}
	h.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *TeamHTTP) handleMe(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":    p.UserID,
		"username":   p.Username,
		"email":      p.Email,
		"superadmin": p.Superadmin,
	})
}

type renameRequest struct {
	Username string `json:"username"`
}

func (h *TeamHTTP) handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p := principalFrom(r.Context())
	var req renameRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.team.RenameUser(r.Context(), p.Actor(), p.UserID, req.Username); err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "username": strings.ToLower(strings.TrimSpace(req.Username))})
}

// ---- Admin: allowlist ----------------------------------------------------

type allowlistRequest struct {
	Email string `json:"email"`
	Note  string `json:"note"`
}

func (h *TeamHTTP) handleAllowlist(w http.ResponseWriter, r *http.Request) {
	// All methods require an authenticated superadmin; mutations also CSRF.
	p, err := h.principalFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if !p.Superadmin {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	switch r.Method {
	case http.MethodGet:
		entries, err := h.team.AllowlistList(r.Context(), p.Actor())
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			out = append(out, map[string]any{"email": e.Email, "note": e.Note, "created_at": e.CreatedAt})
		}
		writeJSON(w, http.StatusOK, map[string]any{"allowlist": out})

	case http.MethodPost:
		if !hasCSRF(r) {
			writeError(w, http.StatusForbidden, "missing CSRF header")
			return
		}
		var req allowlistRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if _, err := h.team.AllowlistAdd(r.Context(), p.Actor(), req.Email, req.Note); err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})

	case http.MethodDelete:
		if !hasCSRF(r) {
			writeError(w, http.StatusForbidden, "missing CSRF header")
			return
		}
		email := r.URL.Query().Get("email")
		if err := h.team.AllowlistRemove(r.Context(), p.Actor(), email); err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ---- Admin: users ----------------------------------------------------

// userView is the admin-facing shape of a user row.
type userView struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`   // "superadmin" | "member"
	Status      string    `json:"status"` // "active" | "disabled"
	CreatedAt   time.Time `json:"createdAt"`
	// Identities lists linked sign-in providers ("magic_link", "google",
	// "github"); empty until the user's next login backfills it.
	Identities []string `json:"identities"`
}

func toUserView(u *ent.User) userView {
	return userView{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		Role:        u.Role.String(),
		Status:      u.Status.String(),
		CreatedAt:   u.CreatedAt,
	}
}

// actor resolves the authenticated principal to a team Actor and requires
// superadmin (every admin endpoint is superadmin-only); responds 401/403 and
// reports false otherwise. This is a defense-in-depth check at the HTTP layer
// on top of team.Service's own actor.Superadmin gate on each method.
func (h *TeamHTTP) actor(w http.ResponseWriter, r *http.Request) (team.Actor, bool) {
	p, err := h.principalFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return team.Actor{}, false
	}
	if !p.Superadmin {
		writeError(w, http.StatusForbidden, "forbidden")
		return team.Actor{}, false
	}
	return p.Actor(), true
}

// actorCSRF is actor plus the custom-header CSRF check required on mutations.
func (h *TeamHTTP) actorCSRF(w http.ResponseWriter, r *http.Request) (team.Actor, bool) {
	if !hasCSRF(r) {
		writeError(w, http.StatusForbidden, "missing CSRF header")
		return team.Actor{}, false
	}
	return h.actor(w, r)
}

func (h *TeamHTTP) handleListUsers(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	users, err := h.team.ListUsers(r.Context(), actor)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	ids := make([]uuid.UUID, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	identities, err := h.team.ListIdentities(r.Context(), actor, ids)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	out := make([]userView, len(users))
	for i, u := range users {
		out[i] = toUserView(u)
		out[i].Identities = identities[u.ID]
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

type updateUserRequest struct {
	Username *string `json:"username"`
	Status   *string `json:"status"`
	Password *string `json:"password"`
}

func (h *TeamHTTP) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorCSRF(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req updateUserRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username != nil {
		if err := h.team.RenameUser(r.Context(), actor, id, *req.Username); err != nil {
			h.writeServiceError(w, err)
			return
		}
	}
	if req.Status != nil {
		var status entuser.Status
		switch *req.Status {
		case "active":
			status = entuser.StatusActive
		case "disabled":
			status = entuser.StatusDisabled
		default:
			writeError(w, http.StatusBadRequest, "status must be active or disabled")
			return
		}
		if err := h.team.SetUserStatus(r.Context(), actor, id, status); err != nil {
			h.writeServiceError(w, err)
			return
		}
	}
	if req.Password != nil {
		// Superadmin sets another user's password: no current-password check
		// (actor is a proven superadmin via the admin gate above).
		if err := h.auth.SetPassword(r.Context(), actor, id, "", *req.Password); err != nil {
			h.writeServiceError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// ---- Middleware ----------------------------------------------------------

// requireAuth wraps a handler, resolving the session cookie to a Principal
// placed in the request context; responds 401 when absent/invalid.
func (h *TeamHTTP) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := h.principalFromRequest(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalCtxKey{}, p)))
	}
}

// requireCSRF enforces the custom CSRF header on state-changing requests.
func (h *TeamHTTP) requireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !hasCSRF(r) {
			writeError(w, http.StatusForbidden, "missing CSRF header")
			return
		}
		next(w, r)
	}
}

func (h *TeamHTTP) principalFromRequest(r *http.Request) (*auth.Principal, error) {
	c, err := r.Cookie(h.cookieNm)
	if err != nil {
		return nil, err
	}
	return h.auth.Authenticate(r.Context(), c.Value)
}

// ---- Cookies -------------------------------------------------------------

func (h *TeamHTTP) setSessionCookie(w http.ResponseWriter, token string) {
	//nolint:gosec // G124: Secure is set from CookieSecure (true over HTTPS); dev over plain HTTP intentionally omits it and uses the unprefixed cookie name.
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieNm,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
	})
}

func (h *TeamHTTP) clearSessionCookie(w http.ResponseWriter) {
	//nolint:gosec // G124: Secure mirrors CookieSecure; this is a deletion cookie (MaxAge<0).
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieNm,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// ---- Helpers -------------------------------------------------------------

func (h *TeamHTTP) appURL(path string) string {
	base := strings.TrimRight(h.cfg.BaseURL, "/")
	if base == "" {
		return path
	}
	return base + path
}

func (h *TeamHTTP) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, team.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, team.ErrInvalidUsername):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, team.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "invalid email address")
	case errors.Is(err, team.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid email or password")
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
	default:
		h.logger.Error("team service error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func principalFrom(ctx context.Context) *auth.Principal {
	p, _ := ctx.Value(principalCtxKey{}).(*auth.Principal)
	return p
}

func hasCSRF(r *http.Request) bool {
	return r.Header.Get(csrfHeader) != ""
}

// clientIP extracts the source IP, preferring the first X-Forwarded-For hop
// (set by the trusted reverse proxy) and falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body) //nolint:errcheck // response write, nothing to do on failure
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
