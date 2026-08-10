package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleProvider drives Google OIDC sign-in. Satisfies the gateway package's
// SSOProvider interface structurally (no import of gateway here).
type GoogleProvider struct {
	oauthCfg oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// NewGoogleProvider performs OIDC discovery against accounts.google.com — a
// real network call. Bound it with a context deadline; a discovery failure
// should be treated as a fatal startup error (an operator who configured
// Google SSO intends it to work, not silently never offer the button).
func NewGoogleProvider(ctx context.Context, clientID, clientSecret, redirectURL string) (*GoogleProvider, error) {
	oidcProvider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("google oidc discovery: %w", err)
	}
	return &GoogleProvider{
		oauthCfg: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{oidc.ScopeOpenID, "email"},
		},
		verifier: oidcProvider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// AuthURL returns the Google consent-screen redirect URL for state (CSRF)
// and nonce (OIDC replay protection).
func (g *GoogleProvider) AuthURL(state, nonce string) string {
	return g.oauthCfg.AuthCodeURL(state, oidc.Nonce(nonce))
}

// Exchange trades an authorization code for Google's verified subject and
// email, checking the nonce echoes what AuthURL sent.
func (g *GoogleProvider) Exchange(ctx context.Context, code, nonce string) (subject, verifiedEmail string, err error) {
	token, err := g.oauthCfg.Exchange(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("exchange code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return "", "", errors.New("token response missing id_token")
	}
	idToken, err := g.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", "", fmt.Errorf("verify id token: %w", err)
	}
	var claims googleClaims
	if err := idToken.Claims(&claims); err != nil {
		return "", "", fmt.Errorf("parse id token claims: %w", err)
	}
	return extractGoogleIdentity(claims, idToken.Nonce, nonce)
}

// googleClaims is the subset of ID-token claims consumed.
type googleClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

// extractGoogleIdentity is Exchange's pure, unit-testable core: given
// already-parsed claims, validates the nonce and EmailVerified before
// trusting the email.
func extractGoogleIdentity(claims googleClaims, gotNonce, wantNonce string) (subject, verifiedEmail string, err error) {
	if gotNonce == "" || gotNonce != wantNonce {
		return "", "", errors.New("nonce mismatch")
	}
	if claims.Sub == "" {
		return "", "", errors.New("id token missing sub")
	}
	if claims.Email == "" {
		return "", "", errors.New("id token missing email")
	}
	if !claims.EmailVerified {
		return "", "", fmt.Errorf("google email %q is not verified", claims.Email)
	}
	return claims.Sub, claims.Email, nil
}
