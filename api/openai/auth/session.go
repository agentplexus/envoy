package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
)

const (
	// SessionName is the name of the session cookie.
	SessionName = "omniagent_session"

	// SessionKeyUser is the key for user data in the session.
	SessionKeyUser = "user"

	// SessionKeyState is the key for OAuth state in the session.
	SessionKeyState = "oauth_state"

	// DefaultMaxAge is the default session max age (7 days).
	DefaultMaxAge = 7 * 24 * 60 * 60
)

// User represents an authenticated user.
type User struct {
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture"`
	Provider  string    `json:"provider"`
	LoginTime time.Time `json:"login_time"`
}

// SessionManager manages user sessions using gorilla/sessions.
type SessionManager struct {
	store  *sessions.CookieStore
	config *Config
}

// NewSessionManager creates a new session manager.
func NewSessionManager(cfg *Config) *SessionManager {
	store := sessions.NewCookieStore([]byte(cfg.SessionSecret))

	// Configure cookie options
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   DefaultMaxAge,
		HttpOnly: true,
		Secure:   true, // Require HTTPS in production
		SameSite: http.SameSiteLaxMode,
	}

	// Set cookie domain if configured
	if cfg.CookieDomain != "" {
		store.Options.Domain = cfg.CookieDomain
	}

	return &SessionManager{
		store:  store,
		config: cfg,
	}
}

// SetDevelopmentMode configures the session manager for local development.
// This disables the Secure cookie flag to allow HTTP connections.
func (sm *SessionManager) SetDevelopmentMode(enabled bool) {
	if enabled {
		sm.store.Options.Secure = false
	}
}

// GetUser retrieves the authenticated user from the session.
// Returns nil if no valid session exists.
func (sm *SessionManager) GetUser(r *http.Request) *User {
	session, err := sm.store.Get(r, SessionName)
	if err != nil {
		return nil
	}

	userData, ok := session.Values[SessionKeyUser]
	if !ok {
		return nil
	}

	user, ok := userData.(*User)
	if !ok {
		return nil
	}

	return user
}

// SetUser stores the authenticated user in the session.
func (sm *SessionManager) SetUser(w http.ResponseWriter, r *http.Request, user *User) error {
	session, err := sm.store.Get(r, SessionName)
	if err != nil {
		// Create a new session if the existing one is invalid
		session, _ = sm.store.New(r, SessionName)
	}

	session.Values[SessionKeyUser] = user
	return session.Save(r, w)
}

// ClearSession removes the user session.
func (sm *SessionManager) ClearSession(w http.ResponseWriter, r *http.Request) error {
	session, err := sm.store.Get(r, SessionName)
	if err != nil {
		return nil // No session to clear
	}

	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1
	return session.Save(r, w)
}

// GenerateState generates a random state string for OAuth CSRF protection.
func (sm *SessionManager) GenerateState(w http.ResponseWriter, r *http.Request) (string, error) {
	state, err := generateRandomState()
	if err != nil {
		return "", err
	}

	session, err := sm.store.Get(r, SessionName)
	if err != nil {
		session, _ = sm.store.New(r, SessionName)
	}

	session.Values[SessionKeyState] = state
	if err := session.Save(r, w); err != nil {
		return "", err
	}

	return state, nil
}

// ValidateState validates the OAuth state parameter against the session.
func (sm *SessionManager) ValidateState(r *http.Request, state string) bool {
	session, err := sm.store.Get(r, SessionName)
	if err != nil {
		return false
	}

	expected, ok := session.Values[SessionKeyState].(string)
	if !ok {
		return false
	}

	return state != "" && state == expected
}

// ClearState removes the OAuth state from the session.
func (sm *SessionManager) ClearState(w http.ResponseWriter, r *http.Request) error {
	session, err := sm.store.Get(r, SessionName)
	if err != nil {
		return nil
	}

	delete(session.Values, SessionKeyState)
	return session.Save(r, w)
}

// generateRandomState generates a cryptographically secure random state string.
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// init registers User type with gob for session serialization.
func init() {
	// Register User type for gob encoding
	gob.Register(&User{})
}
