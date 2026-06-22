package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddleware_RequireAuth_Disabled(t *testing.T) {
	cfg := &Config{Enabled: false}
	sm := NewSessionManager(&Config{SessionSecret: "test-secret-that-is-at-least-32-bytes-long"})
	mw := NewMiddleware(sm, cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw.RequireAuth(handler)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMiddleware_RequireAuth_PublicPaths(t *testing.T) {
	cfg := &Config{
		Enabled:       true,
		SessionSecret: "test-secret-that-is-at-least-32-bytes-long",
	}
	sm := NewSessionManager(cfg)
	mw := NewMiddleware(sm, cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw.RequireAuth(handler)

	publicPaths := []string{
		"/login",
		"/logout",
		"/auth/github",
		"/auth/github/callback",
		"/auth/google",
		"/auth/google/callback",
		"/health",
		"/docs",
		"/docs/openapi.json",
	}

	for _, path := range publicPaths {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200 for %s, got %d", path, w.Code)
			}
		})
	}
}

func TestMiddleware_RequireAuth_APIPaths(t *testing.T) {
	cfg := &Config{
		Enabled:       true,
		SessionSecret: "test-secret-that-is-at-least-32-bytes-long",
	}
	sm := NewSessionManager(cfg)
	mw := NewMiddleware(sm, cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw.RequireAuth(handler)

	apiPaths := []string{
		"/openai/v1/chat/completions",
		"/openai/v1/models",
		"/api/tools",
		"/api/agents",
	}

	for _, path := range apiPaths {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200 for %s (API paths bypass session auth), got %d", path, w.Code)
			}
		})
	}
}

func TestMiddleware_RequireAuth_NoSession(t *testing.T) {
	cfg := &Config{
		Enabled:       true,
		SessionSecret: "test-secret-that-is-at-least-32-bytes-long",
	}
	sm := NewSessionManager(cfg)
	sm.SetDevelopmentMode(true)
	mw := NewMiddleware(sm, cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw.RequireAuth(handler)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, r)

	// Should redirect to login
	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/login" {
		t.Errorf("expected redirect to /login, got %s", location)
	}
}

func TestMiddleware_RequireAuth_WithSession(t *testing.T) {
	cfg := &Config{
		Enabled:       true,
		SessionSecret: "test-secret-that-is-at-least-32-bytes-long",
	}
	sm := NewSessionManager(cfg)
	sm.SetDevelopmentMode(true)
	mw := NewMiddleware(sm, cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw.RequireAuth(handler)

	// First, set a session
	user := &User{
		Email:     "test@example.com",
		Name:      "Test User",
		Provider:  "github",
		LoginTime: time.Now(),
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	if err := sm.SetUser(w, r, user); err != nil {
		t.Fatalf("SetUser() error = %v", err)
	}

	// Get cookies
	cookies := w.Result().Cookies()

	// Make request with session cookie
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()

	wrapped.ServeHTTP(w2, r2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}
