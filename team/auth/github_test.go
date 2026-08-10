package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSelectVerifiedPrimaryEmail(t *testing.T) {
	tests := []struct {
		name    string
		emails  []githubEmail
		want    string
		wantErr bool
	}{
		{
			name:   "primary and verified",
			emails: []githubEmail{{Email: "a@example.com", Primary: false, Verified: true}, {Email: "b@example.com", Primary: true, Verified: true}},
			want:   "b@example.com",
		},
		{
			name:    "no primary",
			emails:  []githubEmail{{Email: "a@example.com", Primary: false, Verified: true}},
			wantErr: true,
		},
		{
			name:    "primary but unverified",
			emails:  []githubEmail{{Email: "a@example.com", Primary: true, Verified: false}},
			wantErr: true,
		},
		{
			name:    "empty list",
			emails:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectVerifiedPrimaryEmail(tt.emails)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitHubProvider_ResolveIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-token" {
			t.Errorf("missing/wrong Authorization header: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
		case "/user/emails":
			_ = json.NewEncoder(w).Encode([]githubEmail{
				{Email: "unverified@example.com", Primary: false, Verified: false},
				{Email: "primary@example.com", Primary: true, Verified: true},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := &GitHubProvider{apiBaseURL: server.URL}
	subject, email, err := p.resolveIdentity(context.Background(), "fake-token")
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	if subject != "12345" {
		t.Errorf("subject = %q, want 12345", subject)
	}
	if email != "primary@example.com" {
		t.Errorf("email = %q, want primary@example.com", email)
	}
}

func TestGitHubProvider_ResolveIdentity_NoVerifiedPrimaryEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
		case "/user/emails":
			_ = json.NewEncoder(w).Encode([]githubEmail{{Email: "unverified@example.com", Primary: true, Verified: false}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := &GitHubProvider{apiBaseURL: server.URL}
	if _, _, err := p.resolveIdentity(context.Background(), "fake-token"); err == nil {
		t.Fatal("expected error when no verified primary email exists")
	}
}

func TestGitHubProvider_AuthURL(t *testing.T) {
	p := NewGitHubProvider("client-id", "client-secret", "https://example.com/api/auth/github/callback")
	url := p.AuthURL("state-1", "unused-nonce")
	if url == "" {
		t.Fatal("empty auth URL")
	}
}
