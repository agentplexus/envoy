package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// Provider represents an OAuth provider.
type Provider string

const (
	// ProviderGitHub is the GitHub OAuth provider.
	ProviderGitHub Provider = "github"

	// ProviderGoogle is the Google OAuth provider.
	ProviderGoogle Provider = "google"
)

// Providers manages OAuth provider configurations.
type Providers struct {
	github *oauth2.Config
	google *oauth2.Config
}

// NewProviders creates OAuth provider configurations from the config.
// The baseURL should be the base URL of the server (e.g., "http://localhost:8080").
func NewProviders(cfg *Config, baseURL string) *Providers {
	p := &Providers{}

	if cfg.HasGitHub() {
		p.github = &oauth2.Config{
			ClientID:     cfg.GitHub.ClientID,
			ClientSecret: cfg.GitHub.ClientSecret,
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
			RedirectURL:  baseURL + "/auth/github/callback",
		}
	}

	if cfg.HasGoogle() {
		p.google = &oauth2.Config{
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint:    google.Endpoint,
			RedirectURL: baseURL + "/auth/google/callback",
		}
	}

	return p
}

// GetAuthURL returns the OAuth authorization URL for the given provider.
func (p *Providers) GetAuthURL(provider Provider, state string) (string, error) {
	cfg := p.getConfig(provider)
	if cfg == nil {
		return "", ErrProviderNotConfigured
	}

	return cfg.AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

// Exchange exchanges an authorization code for tokens and fetches user info.
func (p *Providers) Exchange(ctx context.Context, provider Provider, code string) (*User, error) {
	cfg := p.getConfig(provider)
	if cfg == nil {
		return nil, ErrProviderNotConfigured
	}

	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	return p.fetchUserInfo(ctx, provider, token)
}

// getConfig returns the OAuth config for the provider.
func (p *Providers) getConfig(provider Provider) *oauth2.Config {
	switch provider {
	case ProviderGitHub:
		return p.github
	case ProviderGoogle:
		return p.google
	default:
		return nil
	}
}

// fetchUserInfo fetches user information from the provider.
func (p *Providers) fetchUserInfo(ctx context.Context, provider Provider, token *oauth2.Token) (*User, error) {
	switch provider {
	case ProviderGitHub:
		return p.fetchGitHubUser(ctx, token)
	case ProviderGoogle:
		return p.fetchGoogleUser(ctx, token)
	default:
		return nil, ErrProviderNotConfigured
	}
}

// fetchGitHubUser fetches user info from GitHub.
func (p *Providers) fetchGitHubUser(ctx context.Context, token *oauth2.Token) (*User, error) {
	client := p.github.Client(ctx, token)

	// Fetch user profile
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("fetch github user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github user API error: %s", body)
	}

	var profile struct {
		Name      string `json:"name"`
		Login     string `json:"login"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("decode github user: %w", err)
	}

	// If no public email, fetch from emails API
	email := profile.Email
	if email == "" {
		email, err = p.fetchGitHubEmail(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("fetch github email: %w", err)
		}
	}

	name := profile.Name
	if name == "" {
		name = profile.Login
	}

	return &User{
		Email:    email,
		Name:     name,
		Picture:  profile.AvatarURL,
		Provider: string(ProviderGitHub),
	}, nil
}

// fetchGitHubEmail fetches the primary email from GitHub's emails API.
func (p *Providers) fetchGitHubEmail(_ context.Context, client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github emails API error: %s", body)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	// Find the primary verified email
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	// Fall back to any verified email
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("no verified email found")
}

// fetchGoogleUser fetches user info from Google.
func (p *Providers) fetchGoogleUser(ctx context.Context, token *oauth2.Token) (*User, error) {
	client := p.google.Client(ctx, token)

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("fetch google user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google userinfo API error: %s", body)
	}

	var profile struct {
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("decode google user: %w", err)
	}

	if !profile.VerifiedEmail {
		return nil, fmt.Errorf("google email not verified")
	}

	return &User{
		Email:    profile.Email,
		Name:     profile.Name,
		Picture:  profile.Picture,
		Provider: string(ProviderGoogle),
	}, nil
}

// HasProvider returns true if the specified provider is configured.
func (p *Providers) HasProvider(provider Provider) bool {
	return p.getConfig(provider) != nil
}

// AvailableProviders returns a list of configured providers.
func (p *Providers) AvailableProviders() []Provider {
	var providers []Provider
	if p.github != nil {
		providers = append(providers, ProviderGitHub)
	}
	if p.google != nil {
		providers = append(providers, ProviderGoogle)
	}
	return providers
}
