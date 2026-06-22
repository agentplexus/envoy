package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestSessionManager() *SessionManager {
	cfg := &Config{
		SessionSecret: "test-secret-that-is-at-least-32-bytes-long",
	}
	sm := NewSessionManager(cfg)
	sm.SetDevelopmentMode(true) // Disable secure cookies for testing
	return sm
}

func TestSessionManager_SetAndGetUser(t *testing.T) {
	sm := newTestSessionManager()

	// Create a test user
	user := &User{
		Email:     "test@example.com",
		Name:      "Test User",
		Picture:   "https://example.com/pic.jpg",
		Provider:  "github",
		LoginTime: time.Now(),
	}

	// Create request and response recorder
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Set user
	if err := sm.SetUser(w, r, user); err != nil {
		t.Fatalf("SetUser() error = %v", err)
	}

	// Get cookies from response
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set")
	}

	// Create new request with cookies
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r2.AddCookie(c)
	}

	// Get user
	gotUser := sm.GetUser(r2)
	if gotUser == nil {
		t.Fatal("GetUser() returned nil")
	}

	if gotUser.Email != user.Email {
		t.Errorf("Email = %q, want %q", gotUser.Email, user.Email)
	}
	if gotUser.Name != user.Name {
		t.Errorf("Name = %q, want %q", gotUser.Name, user.Name)
	}
	if gotUser.Provider != user.Provider {
		t.Errorf("Provider = %q, want %q", gotUser.Provider, user.Provider)
	}
}

func TestSessionManager_GetUser_NoSession(t *testing.T) {
	sm := newTestSessionManager()

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	user := sm.GetUser(r)
	if user != nil {
		t.Errorf("GetUser() = %v, want nil", user)
	}
}

func TestSessionManager_ClearSession(t *testing.T) {
	sm := newTestSessionManager()

	user := &User{
		Email:     "test@example.com",
		Name:      "Test User",
		Provider:  "github",
		LoginTime: time.Now(),
	}

	// Set user
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	if err := sm.SetUser(w, r, user); err != nil {
		t.Fatalf("SetUser() error = %v", err)
	}

	// Get cookies
	cookies := w.Result().Cookies()

	// Clear session
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()

	if err := sm.ClearSession(w2, r2); err != nil {
		t.Fatalf("ClearSession() error = %v", err)
	}

	// Verify session is cleared (cookie should have MaxAge -1)
	clearedCookies := w2.Result().Cookies()
	if len(clearedCookies) == 0 {
		t.Fatal("expected cleared cookie")
	}
	if clearedCookies[0].MaxAge >= 0 {
		t.Errorf("expected MaxAge < 0, got %d", clearedCookies[0].MaxAge)
	}
}

func TestSessionManager_State(t *testing.T) {
	sm := newTestSessionManager()

	// Generate state
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	state, err := sm.GenerateState(w, r)
	if err != nil {
		t.Fatalf("GenerateState() error = %v", err)
	}

	if state == "" {
		t.Fatal("GenerateState() returned empty state")
	}

	// State should be URL-safe base64
	if len(state) < 32 {
		t.Errorf("state too short: %d", len(state))
	}

	// Get cookies
	cookies := w.Result().Cookies()

	// Validate state
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r2.AddCookie(c)
	}

	if !sm.ValidateState(r2, state) {
		t.Error("ValidateState() = false, want true")
	}

	// Wrong state should fail
	if sm.ValidateState(r2, "wrong-state") {
		t.Error("ValidateState(wrong) = true, want false")
	}

	// Empty state should fail
	if sm.ValidateState(r2, "") {
		t.Error("ValidateState(empty) = true, want false")
	}
}

func TestSessionManager_ClearState(t *testing.T) {
	sm := newTestSessionManager()

	// Generate state
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	state, err := sm.GenerateState(w, r)
	if err != nil {
		t.Fatalf("GenerateState() error = %v", err)
	}

	// Get cookies
	cookies := w.Result().Cookies()

	// Clear state
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()

	if err := sm.ClearState(w2, r2); err != nil {
		t.Fatalf("ClearState() error = %v", err)
	}

	// Get new cookies
	newCookies := w2.Result().Cookies()

	// State should no longer be valid
	r3 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range newCookies {
		r3.AddCookie(c)
	}

	if sm.ValidateState(r3, state) {
		t.Error("state should be invalid after clear")
	}
}

func TestSessionManager_SetDevelopmentMode(t *testing.T) {
	cfg := &Config{
		SessionSecret: "test-secret-that-is-at-least-32-bytes-long",
	}
	sm := NewSessionManager(cfg)

	// Default should be secure
	if !sm.store.Options.Secure {
		t.Error("default should be secure")
	}

	// Enable dev mode
	sm.SetDevelopmentMode(true)
	if sm.store.Options.Secure {
		t.Error("dev mode should disable secure")
	}
}
