package auth

import "errors"

var (
	// ErrMissingSessionSecret is returned when AUTH_SESSION_SECRET is not set.
	ErrMissingSessionSecret = errors.New("AUTH_SESSION_SECRET is required when auth is enabled")

	// ErrSessionSecretTooShort is returned when the session secret is less than 32 bytes.
	ErrSessionSecretTooShort = errors.New("AUTH_SESSION_SECRET must be at least 32 bytes")

	// ErrNoProviders is returned when no OAuth providers are configured.
	ErrNoProviders = errors.New("at least one OAuth provider (GitHub or Google) must be configured")

	// ErrInvalidState is returned when the OAuth state parameter is invalid.
	ErrInvalidState = errors.New("invalid OAuth state")

	// ErrEmailNotAllowed is returned when the user's email is not in the ACL.
	ErrEmailNotAllowed = errors.New("email not allowed")

	// ErrNoSession is returned when no valid session exists.
	ErrNoSession = errors.New("no valid session")

	// ErrProviderNotConfigured is returned when attempting to use an unconfigured provider.
	ErrProviderNotConfigured = errors.New("OAuth provider not configured")
)
