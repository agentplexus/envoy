package memory

import (
	"context"
	"testing"
	"time"

	"github.com/plexusone/omniagent/storage"
)

func TestStorage_SetGet(t *testing.T) {
	s := New()
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
	s := New()
	defer s.Close()

	ctx := context.Background()

	_, err := s.Get(ctx, "nonexistent")
	if err != storage.ErrNotFound {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestStorage_Delete(t *testing.T) {
	s := New()
	defer s.Close()

	ctx := context.Background()

	s.Set(ctx, "key1", []byte("value1"), 0)

	if err := s.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := s.Get(ctx, "key1")
	if err != storage.ErrNotFound {
		t.Errorf("Get() after Delete() error = %v, want ErrNotFound", err)
	}
}

func TestStorage_TTL(t *testing.T) {
	s := New()
	defer s.Close()

	ctx := context.Background()

	// Set with short TTL
	if err := s.Set(ctx, "key1", []byte("value1"), 50*time.Millisecond); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Should exist initially
	if _, err := s.Get(ctx, "key1"); err != nil {
		t.Errorf("Get() before expiry error = %v", err)
	}

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, err := s.Get(ctx, "key1")
	if err != storage.ErrNotFound {
		t.Errorf("Get() after expiry error = %v, want ErrNotFound", err)
	}
}

func TestStorage_Close(t *testing.T) {
	s := New()

	ctx := context.Background()
	s.Set(ctx, "key1", []byte("value1"), 0)

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
	s := New()
	defer s.Close()

	ctx := context.Background()

	s.Set(ctx, "key1", []byte("value1"), 0)
	s.Set(ctx, "key1", []byte("value2"), 0)

	got, err := s.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != "value2" {
		t.Errorf("Get() = %q, want %q", string(got), "value2")
	}
}
