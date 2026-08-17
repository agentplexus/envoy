package config

import (
	"strings"
	"testing"
)

func TestStorageConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     StorageConfig
		wantErr string // empty = valid
	}{
		{name: "empty type valid (treated as sqlite by the factory)", cfg: StorageConfig{}},
		{name: "sqlite valid", cfg: StorageConfig{Type: "sqlite", Path: "/data/omniagent.db"}},
		{name: "sqlite valid without path (factory defaults it)", cfg: StorageConfig{Type: "sqlite"}},
		{name: "memory valid", cfg: StorageConfig{Type: "memory"}},
		{name: "redis valid", cfg: StorageConfig{Type: "redis", Redis: RedisConfig{URL: "redis://localhost:6379"}}},
		{name: "redis missing url", cfg: StorageConfig{Type: "redis"}, wantErr: "storage.redis.url"},
		{name: "unknown type", cfg: StorageConfig{Type: "postgres"}, wantErr: "storage.type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestDefault_StorageAndSessions(t *testing.T) {
	cfg := Default()

	if cfg.Storage.Type != "sqlite" {
		t.Errorf("Storage.Type = %q, want sqlite", cfg.Storage.Type)
	}
	if cfg.Storage.Path == "" {
		t.Error("Storage.Path should default to a non-empty path")
	}
	if cfg.Storage.Path != DefaultStoragePath() {
		t.Errorf("Storage.Path = %q, want %q", cfg.Storage.Path, DefaultStoragePath())
	}
	if !cfg.Sessions.Enabled {
		t.Error("Sessions.Enabled should default to true")
	}
	if err := cfg.Storage.Validate(); err != nil {
		t.Errorf("Default().Storage.Validate() = %v, want nil", err)
	}
}

func TestLoadEnv_Storage(t *testing.T) {
	t.Setenv("OMNIAGENT_STORAGE_TYPE", "redis")
	t.Setenv("OMNIAGENT_STORAGE_PATH", "/custom/data.db")
	t.Setenv("OMNIAGENT_STORAGE_REDIS_URL", "redis://cache:6379")
	t.Setenv("OMNIAGENT_SESSIONS_ENABLED", "false")
	t.Setenv("OMNIAGENT_SESSIONS_TTL", "24h")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Storage.Type != "redis" {
		t.Errorf("Storage.Type = %q, want redis", cfg.Storage.Type)
	}
	if cfg.Storage.Path != "/custom/data.db" {
		t.Errorf("Storage.Path = %q, want /custom/data.db", cfg.Storage.Path)
	}
	if cfg.Storage.Redis.URL != "redis://cache:6379" {
		t.Errorf("Storage.Redis.URL = %q, want redis://cache:6379", cfg.Storage.Redis.URL)
	}
	if cfg.Sessions.Enabled {
		t.Error("Sessions.Enabled should be false")
	}
	if cfg.Sessions.TTL.Duration().String() != "24h0m0s" {
		t.Errorf("Sessions.TTL = %v, want 24h0m0s", cfg.Sessions.TTL.Duration())
	}
}
