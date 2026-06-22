package auth

import (
	"net/http"
	"strings"
)

// Middleware provides HTTP middleware for session-based authentication.
type Middleware struct {
	sessions *SessionManager
	config   *Config
}

// NewMiddleware creates a new auth middleware.
func NewMiddleware(sessions *SessionManager, cfg *Config) *Middleware {
	return &Middleware{
		sessions: sessions,
		config:   cfg,
	}
}

// RequireAuth returns a middleware that requires authentication.
// It protects the web UI paths while allowing public and API paths through.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If auth is disabled, allow all requests
		if !m.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Allow public paths
		if m.isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Allow API paths (they use Bearer token auth)
		if m.isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Check session
		user := m.sessions.GetUser(r)
		if user == nil {
			// Redirect to login page
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// User is authenticated
		next.ServeHTTP(w, r)
	})
}

// isPublicPath returns true if the path should be publicly accessible.
func (m *Middleware) isPublicPath(path string) bool {
	publicPaths := []string{
		"/login",
		"/logout",
		"/auth/",
		"/health",
		"/docs",
	}

	for _, prefix := range publicPaths {
		if path == prefix || strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// isAPIPath returns true if the path is an API endpoint using Bearer token auth.
func (m *Middleware) isAPIPath(path string) bool {
	apiPaths := []string{
		"/openai/",
		"/api/",
	}

	for _, prefix := range apiPaths {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}
