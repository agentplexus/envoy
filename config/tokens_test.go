package config

import (
	"context"
	"testing"
)

func TestNewTokenManager_EmptyVaultURI(t *testing.T) {
	cfg := TokenConfig{
		VaultURI: "",
	}

	ctx := context.Background()
	_, err := NewTokenManager(ctx, cfg)
	if err == nil {
		t.Error("NewTokenManager() with empty VaultURI should return error")
	}
}

func TestNewTokenManager_InvalidVaultURI(t *testing.T) {
	cfg := TokenConfig{
		VaultURI: "invalid://scheme",
	}

	ctx := context.Background()
	_, err := NewTokenManager(ctx, cfg)
	if err == nil {
		t.Error("NewTokenManager() with invalid VaultURI should return error")
	}
}

func TestNewTokenManager_MemoryVault(t *testing.T) {
	cfg := TokenConfig{
		VaultURI: "memory://",
		Services: map[string]ServiceTokenConfig{
			"test-service": {
				CredentialsName: "test-creds",
				Scopes:          []string{"scope1", "scope2"},
			},
		},
	}

	ctx := context.Background()
	tm, err := NewTokenManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}
	defer tm.Close()

	if tm == nil {
		t.Error("NewTokenManager() returned nil")
	}
}

//nolint:gosec // G101: Test fixture with credential config names
func TestTokenConfig_ServicesMap(t *testing.T) {
	cfg := TokenConfig{
		VaultURI: "memory://",
		Services: map[string]ServiceTokenConfig{
			"google": {
				CredentialsName: "google-sa",
				Scopes:          []string{"https://www.googleapis.com/auth/calendar"},
			},
			"zoom": {
				CredentialsName: "zoom-oauth",
			},
		},
	}

	if len(cfg.Services) != 2 {
		t.Errorf("Services count = %d, want 2", len(cfg.Services))
	}

	google, ok := cfg.Services["google"]
	if !ok {
		t.Error("Services[\"google\"] not found")
	}
	if google.CredentialsName != "google-sa" {
		t.Errorf("google.CredentialsName = %q, want %q", google.CredentialsName, "google-sa")
	}
	if len(google.Scopes) != 1 {
		t.Errorf("google.Scopes count = %d, want 1", len(google.Scopes))
	}
}

func TestTokenManager_Close(t *testing.T) {
	cfg := TokenConfig{
		VaultURI: "memory://",
	}

	ctx := context.Background()
	tm, err := NewTokenManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}

	// Close should not panic or error
	err = tm.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Double close should be safe
	err = tm.Close()
	if err != nil {
		t.Errorf("Close() second call error = %v", err)
	}
}

func TestTokenManager_HTTPClient_NoCredentials(t *testing.T) {
	cfg := TokenConfig{
		VaultURI: "memory://",
	}

	ctx := context.Background()
	tm, err := NewTokenManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}
	defer tm.Close()

	// Getting client for non-existent service should error
	_, err = tm.HTTPClient(ctx, "nonexistent")
	if err == nil {
		t.Error("HTTPClient() for nonexistent service should return error")
	}
}

func TestTokenManager_RefreshToken_NoCredentials(t *testing.T) {
	cfg := TokenConfig{
		VaultURI: "memory://",
	}

	ctx := context.Background()
	tm, err := NewTokenManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}
	defer tm.Close()

	// Refreshing token for non-existent service should error
	err = tm.RefreshToken(ctx, "nonexistent")
	if err == nil {
		t.Error("RefreshToken() for nonexistent service should return error")
	}
}
