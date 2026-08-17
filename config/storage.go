package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// storageTypes is the set of supported Storage.Type values.
var storageTypes = map[string]bool{"sqlite": true, "redis": true, "memory": true}

// DefaultStoragePath returns the default SQLite database path used when
// storage.type is "sqlite" (or unset) and storage.path is empty. Matches
// the path the `omniagent sessions` CLI has always defaulted to, so the
// two never drift apart.
func DefaultStoragePath() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "omniagent", "data.db")
}

// Validate checks the storage configuration. An empty Type is always valid
// (Default() always populates it; a hand-built Config leaving it empty is
// treated as "sqlite" by the backend factory).
func (c *StorageConfig) Validate() error {
	if c.Type == "" {
		return nil
	}
	if !storageTypes[c.Type] {
		return fmt.Errorf("storage.type %q must be one of sqlite, redis, memory", c.Type)
	}
	if c.Type == "redis" && c.Redis.URL == "" {
		return fmt.Errorf("storage.redis.url is required when storage.type is redis")
	}
	return nil
}
