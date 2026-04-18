package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/plexusone/omniagent/storage"
)

func TestStorage_SetGet(t *testing.T) {
	s := newTestStorage(t)
	defer s.Close()

	ctx := context.Background()

	// Set a value
	if err := s.Set(ctx, "key1", []byte("value1"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get the value
	got, err := s.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != "value1" {
		t.Errorf("Get() = %q, want %q", string(got), "value1")
	}
}

func TestStorage_GetNotFound(t *testing.T) {
	s := newTestStorage(t)
	defer s.Close()

	ctx := context.Background()

	_, err := s.Get(ctx, "nonexistent")
	if err != storage.ErrNotFound {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestStorage_Delete(t *testing.T) {
	s := newTestStorage(t)
	defer s.Close()

	ctx := context.Background()

	if err := s.Set(ctx, "key1", []byte("value1"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := s.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := s.Get(ctx, "key1")
	if err != storage.ErrNotFound {
		t.Errorf("Get() after Delete() error = %v, want ErrNotFound", err)
	}
}

func TestStorage_TTL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TTL test in short mode")
	}

	s := newTestStorage(t)
	defer s.Close()

	ctx := context.Background()

	// Set with 1 second TTL (SQLite uses second-level Unix timestamp precision)
	// Expiration check is: time.Now().Unix() > expiresAt
	// So we need to wait until Unix time advances past the expiration
	if err := s.Set(ctx, "key1", []byte("value1"), 1*time.Second); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Should exist initially
	if _, err := s.Get(ctx, "key1"); err != nil {
		t.Errorf("Get() before expiry error = %v", err)
	}

	// Wait 2+ seconds to ensure Unix timestamp advances past expiration
	time.Sleep(2100 * time.Millisecond)

	// Should be expired
	_, err := s.Get(ctx, "key1")
	if err != storage.ErrNotFound {
		t.Errorf("Get() after expiry error = %v, want ErrNotFound", err)
	}
}

func TestStorage_Close(t *testing.T) {
	s := newTestStorage(t)

	ctx := context.Background()
	if err := s.Set(ctx, "key1", []byte("value1"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Operations after close should fail
	if err := s.Set(ctx, "key2", []byte("value2"), 0); err != storage.ErrClosed {
		t.Errorf("Set() after Close() error = %v, want ErrClosed", err)
	}

	if _, err := s.Get(ctx, "key1"); err != storage.ErrClosed {
		t.Errorf("Get() after Close() error = %v, want ErrClosed", err)
	}
}

func TestStorage_Overwrite(t *testing.T) {
	s := newTestStorage(t)
	defer s.Close()

	ctx := context.Background()

	if err := s.Set(ctx, "key1", []byte("value1"), 0); err != nil {
		t.Fatalf("Set(value1) error = %v", err)
	}
	if err := s.Set(ctx, "key1", []byte("value2"), 0); err != nil {
		t.Fatalf("Set(value2) error = %v", err)
	}

	got, err := s.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != "value2" {
		t.Errorf("Get() = %q, want %q", string(got), "value2")
	}
}

func TestStorage_LargeValue(t *testing.T) {
	s := newTestStorage(t)
	defer s.Close()

	ctx := context.Background()

	// Create a large value (1MB)
	largeValue := make([]byte, 1024*1024)
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	if err := s.Set(ctx, "large", largeValue, 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := s.Get(ctx, "large")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(got) != len(largeValue) {
		t.Errorf("Get() len = %d, want %d", len(got), len(largeValue))
	}
}

func TestStorage_Path(t *testing.T) {
	s := newTestStorage(t)
	defer s.Close()

	if s.Path() == "" {
		t.Error("Path() returned empty string")
	}
}

// newTestStorage creates a storage instance for testing with a temp database.
func newTestStorage(t *testing.T) *Storage {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	s, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Cleanup the temp file on test cleanup
	t.Cleanup(func() {
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})

	return s
}
