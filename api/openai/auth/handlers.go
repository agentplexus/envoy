package auth

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"
)

// Handlers provides HTTP handlers for authentication.
type Handlers struct {
	config    *Config
	sessions  *SessionManager
	providers *Providers
	acl       *ACL
	templates *template.Template
	logger    *slog.Logger
}

// HandlersConfig configures the auth handlers.
type HandlersConfig struct {
	Config    *Config
	Sessions  *SessionManager
	Providers *Providers
	ACL       *ACL
	Assets    fs.FS
	Logger    *slog.Logger
}

// NewHandlers creates new auth handlers.
func NewHandlers(cfg HandlersConfig) (*Handlers, error) {
	// Parse login template
	tmplContent, err := fs.ReadFile(cfg.Assets, "login.html")
	if err != nil {
		return nil, fmt.Errorf("read login template: %w", err)
	}

	tmpl, err := template.New("login").Parse(string(tmplContent))
	if err != nil {
		return nil, fmt.Errorf("parse login template: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handlers{
		config:    cfg.Config,
		sessions:  cfg.Sessions,
		providers: cfg.Providers,
		acl:       cfg.ACL,
		templates: tmpl,
		logger:    logger,
	}, nil
}

// LoginData holds data for the login template.
type LoginData struct {
	HasGitHub bool
	HasGoogle bool
	Error     string
}

// HandleLogin renders the login page.
func (h *Handlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to home
	if user := h.sessions.GetUser(r); user != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	data := LoginData{
		HasGitHub: h.config.HasGitHub(),
		HasGoogle: h.config.HasGoogle(),
		Error:     r.URL.Query().Get("error"),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "login", data); err != nil {
		h.logger.Error("render login template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleLogout clears the session and redirects to login.
func (h *Handlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.ClearSession(w, r); err != nil {
		h.logger.Error("clear session", "error", err)
	}

	http.Redirect(w, r, "/login", http.StatusFound)
}

// HandleOAuthStart initiates the OAuth flow for a provider.
func (h *Handlers) HandleOAuthStart(provider Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.providers.HasProvider(provider) {
			http.Error(w, "Provider not configured", http.StatusNotFound)
			return
		}

		state, err := h.sessions.GenerateState(w, r)
		if err != nil {
			h.logger.Error("generate state", "error", err)
			http.Redirect(w, r, "/login?error=internal_error", http.StatusFound)
			return
		}

		authURL, err := h.providers.GetAuthURL(provider, state)
		if err != nil {
			h.logger.Error("get auth URL", "error", err)
			http.Redirect(w, r, "/login?error=internal_error", http.StatusFound)
			return
		}

		// nolint:gosec // G710: authURL is generated from trusted OAuth provider configs
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

// HandleOAuthCallback handles the OAuth callback from a provider.
func (h *Handlers) HandleOAuthCallback(provider Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for OAuth error
		if errCode := r.URL.Query().Get("error"); errCode != "" {
			errDesc := r.URL.Query().Get("error_description")
			h.logger.Warn("OAuth error", "provider", provider, "error", errCode, "description", errDesc)
			http.Redirect(w, r, "/login?error=oauth_error", http.StatusFound)
			return
		}

		// Validate state
		state := r.URL.Query().Get("state")
		if !h.sessions.ValidateState(r, state) {
			h.logger.Warn("invalid OAuth state", "provider", provider)
			http.Redirect(w, r, "/login?error=invalid_state", http.StatusFound)
			return
		}

		// Clear state after validation
		if err := h.sessions.ClearState(w, r); err != nil {
			h.logger.Error("clear state", "error", err)
		}

		// Exchange code for token and get user info
		code := r.URL.Query().Get("code")
		user, err := h.providers.Exchange(r.Context(), provider, code)
		if err != nil {
			h.logger.Error("OAuth exchange", "provider", provider, "error", err)
			http.Redirect(w, r, "/login?error=exchange_failed", http.StatusFound)
			return
		}

		// Check ACL
		if !h.acl.IsAllowed(user.Email) {
			h.logger.Warn("email not allowed", "email", user.Email, "provider", provider)
			http.Redirect(w, r, "/login?error=not_allowed", http.StatusFound)
			return
		}

		// Set login time
		user.LoginTime = time.Now()

		// Create session
		if err := h.sessions.SetUser(w, r, user); err != nil {
			h.logger.Error("set user session", "error", err)
			http.Redirect(w, r, "/login?error=session_error", http.StatusFound)
			return
		}

		h.logger.Info("user logged in", "email", user.Email, "provider", provider)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
