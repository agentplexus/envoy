// Package config provides configuration types and loading for omniagent.
package config

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/plexusone/omnitoken"
	"github.com/plexusone/omnivault"
)

// TokenConfig configures OAuth token management for services that require
// access token refresh (Google, Zoom, RingCentral, etc.).
type TokenConfig struct {
	// VaultURI is the vault URI for storing credentials and tokens.
	// Examples: "op://MyVault", "bw://org-id", "keeper://"
	VaultURI string `json:"vault_uri" yaml:"vault_uri"`

	// Services maps service names to their credential configuration.
	// The credential name in the vault defaults to the service name.
	Services map[string]ServiceTokenConfig `json:"services" yaml:"services"`
}

// ServiceTokenConfig configures a single OAuth service.
type ServiceTokenConfig struct {
	// CredentialsName is the name of the credentials in the vault.
	// If empty, defaults to the service name.
	CredentialsName string `json:"credentials_name" yaml:"credentials_name"`

	// Scopes are the OAuth scopes to request (for Google, etc.).
	Scopes []string `json:"scopes" yaml:"scopes"`
}

// TokenManager provides OAuth token management with automatic refresh.
// It wraps omnitoken.TokenManager and provides vault-backed credential storage.
type TokenManager struct {
	mu      sync.RWMutex
	tm      *omnitoken.TokenManager
	config  TokenConfig
	clients map[string]*http.Client
}

// NewTokenManager creates a token manager from configuration.
// The token manager handles OAuth token lifecycle including automatic refresh
// and vault coordination for multi-process scenarios.
func NewTokenManager(ctx context.Context, config TokenConfig) (*TokenManager, error) {
	if config.VaultURI == "" {
		return nil, fmt.Errorf("vault_uri is required for token management")
	}

	// Create vault from URI
	vault, err := omnivault.VaultFromURI(config.VaultURI)
	if err != nil {
		return nil, fmt.Errorf("open vault: %w", err)
	}

	// Create omnitoken manager with vault backend
	tm, err := omnitoken.New(omnitoken.Config{
		Vault: vault,
	})
	if err != nil {
		return nil, fmt.Errorf("create token manager: %w", err)
	}

	return &TokenManager{
		tm:      tm,
		config:  config,
		clients: make(map[string]*http.Client),
	}, nil
}

// HTTPClient returns an HTTP client for the service with automatic token refresh.
// The client automatically:
// 1. Adds Authorization header with access token
// 2. Refreshes token when expired
// 3. Coordinates with vault for multi-process token sharing
//
// Example:
//
//	client := tm.HTTPClient(ctx, "google")
//	resp, err := client.Get("https://www.googleapis.com/...")
func (m *TokenManager) HTTPClient(ctx context.Context, service string) (*http.Client, error) {
	m.mu.RLock()
	if client, ok := m.clients[service]; ok {
		m.mu.RUnlock()
		return client, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := m.clients[service]; ok {
		return client, nil
	}

	// Get credentials name (defaults to service name)
	credName := service
	if svc, ok := m.config.Services[service]; ok && svc.CredentialsName != "" {
		credName = svc.CredentialsName
	}

	// Get HTTP client from omnitoken (uses GetClient method)
	client, err := m.tm.GetClient(ctx, credName)
	if err != nil {
		return nil, fmt.Errorf("get HTTP client for %s: %w", service, err)
	}

	m.clients[service] = client
	return client, nil
}

// RefreshToken forces a token refresh for the service.
// This is useful when you know the token is invalid and want to refresh immediately.
func (m *TokenManager) RefreshToken(ctx context.Context, service string) error {
	credName := service
	if svc, ok := m.config.Services[service]; ok && svc.CredentialsName != "" {
		credName = svc.CredentialsName
	}

	// Get fresh token (forces refresh)
	_, err := m.tm.GetToken(ctx, credName)
	if err != nil {
		return fmt.Errorf("refresh token for %s: %w", service, err)
	}

	// Clear cached client so next call gets fresh one
	m.mu.Lock()
	delete(m.clients, service)
	m.mu.Unlock()

	return nil
}

// LoadGoogleServiceAccount loads a Google service account from a JSON file
// and stores it in the vault for token management.
func (m *TokenManager) LoadGoogleServiceAccount(ctx context.Context, name, serviceAccountFile string, scopes []string) error {
	return m.tm.LoadGoogleServiceAccount(ctx, name, serviceAccountFile, scopes)
}

// Close releases resources held by the token manager.
func (m *TokenManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients = make(map[string]*http.Client)
	// omnitoken.TokenManager doesn't have Close, vault is closed separately
	return nil
}
