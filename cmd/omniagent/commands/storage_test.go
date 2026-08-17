package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/plexusone/omniagent/config"
)

func TestBuildStorageBackend_SQLite(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "nested")
	dbPath := filepath.Join(nestedDir, "data.db")

	backend, err := buildStorageBackend(config.StorageConfig{Type: "sqlite", Path: dbPath})
	if err != nil {
		t.Fatalf("buildStorageBackend() error = %v", err)
	}
	defer backend.Close()

	// The parent directory didn't exist beforehand (Lightsail-style fresh
	// volume/scratch path) — buildStorageBackend must create it.
	if fi, statErr := os.Stat(nestedDir); statErr != nil || !fi.IsDir() {
		t.Fatalf("expected %q to be created as a directory, stat err=%v", nestedDir, statErr)
	}
	if _, statErr := os.Stat(dbPath); statErr != nil {
		t.Fatalf("expected sqlite db file at %q, stat err=%v", dbPath, statErr)
	}
}

func TestBuildStorageBackend_SQLiteDefaultPath(t *testing.T) {
	// Empty Type defaults to sqlite; empty Path defaults to
	// config.DefaultStoragePath(). Don't actually touch the real default
	// path from a test — just verify the empty-Type branch is equivalent
	// to explicit "sqlite" for a temp path.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")

	backend, err := buildStorageBackend(config.StorageConfig{Path: dbPath})
	if err != nil {
		t.Fatalf("buildStorageBackend() error = %v", err)
	}
	defer backend.Close()
}

func TestBuildStorageBackend_Memory(t *testing.T) {
	backend, err := buildStorageBackend(config.StorageConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("buildStorageBackend() error = %v", err)
	}
	defer backend.Close()
}

func TestBuildStorageBackend_UnknownType(t *testing.T) {
	if _, err := buildStorageBackend(config.StorageConfig{Type: "postgres"}); err == nil {
		t.Error("expected an error for an unknown storage type")
	}
}

func TestBuildStorageBackend_RedisMissingURL(t *testing.T) {
	// redis.New itself rejects an empty URL — buildStorageBackend passes
	// the config straight through, so this doesn't require a live redis.
	if _, err := buildStorageBackend(config.StorageConfig{Type: "redis"}); err == nil {
		t.Error("expected an error for a redis config with no URL")
	}
}
