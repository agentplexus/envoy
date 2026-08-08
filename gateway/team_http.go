package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/plexusone/omniagent/team"
	"github.com/plexusone/omniagent/team/auth"
)

// csrfHeader must be present on state-changing team requests. Combined with a
// SameSite=Lax session cookie this defeats cross-site form submissions: a
// cross-origin page cannot set a custom request header without a CORS
// preflight the server never approves.
const csrfHeader = "X-OmniAgent-CSRF"

// principalCtxKey carries the authenticated principal through the request.
type principalCtxKey struct{}

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

func (h *TeamHTTP) routes() {
	h.mux = http.NewServeMux()
	// Auth
	h.mux.HandleFunc("/api/auth/magic-link", h.handleMagicLink) // POST
	h.mux.HandleFunc("/api/auth/verify", h.handleVerify)        // GET
	h.mux.HandleFunc("/api/auth/logout", h.requireCSRF(h.handleLogout))
	h.mux.HandleFunc("/api/auth/me", h.requireAuth(h.handleMe)) // GET
	// Self-service
	h.mux.HandleFunc("/api/users/me/username", h.requireCSRF(h.requireAuth(h.handleRename)))
	// Admin (superadmin only)
	h.mux.HandleFunc("/api/admin/allowlist", h.handleAllowlist)
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

	// Escalating delay per source throttles enumeration/spam. Applied
	// before the (uniform) response; loopback is delayed but never locked.
	_ = h.limiter.recordFailureAndDelay(r.Context(), "magic:"+ip) //nolint:errcheck // delay is best-effort; ctx cancel just skips it

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
