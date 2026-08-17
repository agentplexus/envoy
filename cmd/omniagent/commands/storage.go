package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/plexusone/omniagent/config"
	"github.com/plexusone/omnistorage-core/kvs"
	"github.com/plexusone/omnistorage-core/kvs/backend/memory"
	"github.com/plexusone/omnistorage-core/kvs/backend/redis"
	"github.com/plexusone/omnistorage-core/kvs/backend/sqlite"
)

// buildStorageBackend constructs the kvs.Store selected by cfg.Type
// (RMI-OMNIAGENT-007). The caller owns the returned backend's lifecycle
// and must call Close() when done with it.
func buildStorageBackend(cfg config.StorageConfig) (kvs.Store, error) {
	switch cfg.Type {
	case "", "sqlite":
		path := cfg.Path
		if path == "" {
			path = config.DefaultStoragePath()
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create storage dir: %w", err)
		}
		return sqlite.New(sqlite.Config{Path: path})
	case "redis":
		return redis.New(redis.Config{URL: cfg.Redis.URL})
	case "memory":
		return memory.New(), nil
	default:
		return nil, fmt.Errorf("unknown storage.type %q", cfg.Type)
	}
}
