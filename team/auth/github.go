package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// defaultGitHubAPIBaseURL is GitHub's REST API origin. Overridable per
// instance so tests can point at an httptest server.
const defaultGitHubAPIBaseURL = "https://api.github.com"

// GitHubProvider drives GitHub OAuth2 sign-in. Satisfies the gateway
// package's SSOProvider interface structurally (no import of gateway here).
type GitHubProvider struct {
	oauthCfg   oauth2.Config
	apiBaseURL string
}

// NewGitHubProvider builds a GitHub OAuth2 client. Unlike Google, no
// discovery call is made — this never fails.
func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{
		oauthCfg: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     github.Endpoint,
			// user:email is required to read /user/emails; a GitHub
			// account's public /user.email is often empty or unverified.
			Scopes: []string{"read:user", "user:email"},
		},
		apiBaseURL: defaultGitHubAPIBaseURL,
	}
}

// AuthURL returns the GitHub consent-screen redirect URL for state (CSRF).
// GitHub's plain OAuth2 flow has no nonce/ID-token concept; nonce is unused.
func (g *GitHubProvider) AuthURL(state, _ string) string {
	return g.oauthCfg.AuthCodeURL(state)
}

// Exchange trades an authorization code for GitHub's verified subject
// (numeric account id) and primary verified email.
func (g *GitHubProvider) Exchange(ctx context.Context, code, _ string) (subject, verifiedEmail string, err error) {
	token, err := g.oauthCfg.Exchange(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("exchange code: %w", err)
	}
	return g.resolveIdentity(ctx, token.AccessToken)
}

// resolveIdentity is Exchange's core, split out so tests can call it
// directly against an httptest server with a fixed access token, bypassing
// the real OAuth2 token exchange.
func (g *GitHubProvider) resolveIdentity(ctx context.Context, accessToken string) (subject, verifiedEmail string, err error) {
	var account struct {
		ID int64 `json:"id"`
	}
	if err := g.githubGet(ctx, "/user", accessToken, &account); err != nil {
		return "", "", fmt.Errorf("fetch user: %w", err)
	}
	if account.ID == 0 {
		return "", "", errors.New("github user response missing id")
	}

	var emails []githubEmail
	if err := g.githubGet(ctx, "/user/emails", accessToken, &emails); err != nil {
		return "", "", fmt.Errorf("fetch user emails: %w", err)
	}
	email, err := selectVerifiedPrimaryEmail(emails)
	if err != nil {
		return "", "", err
	}

	return strconv.FormatInt(account.ID, 10), email, nil
}

func (g *GitHubProvider) githubGet(ctx context.Context, path, accessToken string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.apiBaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github api %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// githubEmail mirrors one entry of GitHub's GET /user/emails response.
type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// selectVerifiedPrimaryEmail picks the Primary&&Verified entry. GitHub
// guarantees at most one primary email, but never trust an unverified one.
func selectVerifiedPrimaryEmail(emails []githubEmail) (string, error) {
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", errors.New("no verified primary email on github account")
}
