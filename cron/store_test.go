package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/plexusone/omnistorage"
)

// mockStore is a simple in-memory kvs.Store for testing.
type mockStore struct {
	data map[string][]byte
}

func newMockStore() *mockStore {
	return &mockStore{
		data: make(map[string][]byte),
	}
}

func (m *mockStore) Get(_ context.Context, key string) ([]byte, error) {
	v, ok := m.data[key]
	if !ok {
		return nil, omnistorage.ErrKVSNotFound
	}
	return v, nil
}

func (m *mockStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *mockStore) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockStore) Close() error {
	return nil
}

// listableMockStore extends mockStore with List capability.
type listableMockStore struct {
	*mockStore
}

func newListableMockStore() *listableMockStore {
	return &listableMockStore{
		mockStore: newMockStore(),
	}
}

func (m *listableMockStore) List(_ context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range m.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func TestStore_SaveAndGet(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newMockStore()})
	ctx := context.Background()

	job := NewJob("job-1", "Test Job",
		Schedule{Cron: "0 0 9 * * *"},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	// Save
	if err := store.Save(ctx, job); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Get
	got, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.ID != job.ID {
		t.Errorf("ID mismatch: %q != %q", got.ID, job.ID)
	}
	if got.Name != job.Name {
		t.Errorf("Name mismatch: %q != %q", got.Name, job.Name)
	}
}

func TestStore_GetNotFound(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newMockStore()})
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestStore_Delete(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newMockStore()})
	ctx := context.Background()

	job := NewJob("job-1", "Test Job",
		Schedule{Cron: "0 0 9 * * *"},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	if err := store.Save(ctx, job); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete
	if err := store.Delete(ctx, "job-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err := store.Get(ctx, "job-1")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound after delete, got %v", err)
	}
}

func TestStore_List(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	ctx := context.Background()

	// Save multiple jobs
	jobs := []*Job{
		NewJob("job-1", "Job 1", Schedule{Cron: "0 0 9 * * *"}, Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"}),
		NewJob("job-2", "Job 2", Schedule{Interval: Duration(time.Hour)}, Action{Type: ActionTypeCallWebhook, WebhookURL: "https://example.com"}),
		NewJob("job-3", "Job 3", Schedule{Cron: "0 0 12 * * *"}, Action{Type: ActionTypeCallTool, ToolName: "my_tool"}),
	}

	for _, job := range jobs {
		if err := store.Save(ctx, job); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List
	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(listed) != len(jobs) {
		t.Errorf("expected %d jobs, got %d", len(jobs), len(listed))
	}
}

func TestStore_ListEnabled(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	ctx := context.Background()

	// Save jobs with different statuses
	job1 := NewJob("job-1", "Enabled", Schedule{Cron: "0 0 9 * * *"}, Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"})
	job1.Status = JobStatusEnabled

	job2 := NewJob("job-2", "Disabled", Schedule{Cron: "0 0 9 * * *"}, Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"})
	job2.Status = JobStatusDisabled

	job3 := NewJob("job-3", "Also Enabled", Schedule{Cron: "0 0 9 * * *"}, Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"})
	job3.Status = JobStatusEnabled

	for _, job := range []*Job{job1, job2, job3} {
		if err := store.Save(ctx, job); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List enabled only
	enabled, err := store.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("ListEnabled failed: %v", err)
	}

	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled jobs, got %d", len(enabled))
	}

	for _, job := range enabled {
		if job.Status != JobStatusEnabled {
			t.Errorf("expected enabled job, got status %q", job.Status)
		}
	}
}

func TestStore_ListByStatus(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	ctx := context.Background()

	// Save jobs with different statuses
	job1 := NewJob("job-1", "Enabled", Schedule{Cron: "0 0 9 * * *"}, Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"})
	job1.Status = JobStatusEnabled

	job2 := NewJob("job-2", "Disabled", Schedule{Cron: "0 0 9 * * *"}, Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"})
	job2.Status = JobStatusDisabled

	for _, job := range []*Job{job1, job2} {
		if err := store.Save(ctx, job); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List disabled
	disabled, err := store.ListByStatus(ctx, JobStatusDisabled)
	if err != nil {
		t.Fatalf("ListByStatus failed: %v", err)
	}

	if len(disabled) != 1 {
		t.Errorf("expected 1 disabled job, got %d", len(disabled))
	}
	if disabled[0].ID != "job-2" {
		t.Errorf("expected job-2, got %q", disabled[0].ID)
	}
}

func TestStore_Cache(t *testing.T) {
	backend := newMockStore()
	store := NewStore(StoreConfig{Backend: backend})
	ctx := context.Background()

	job := NewJob("job-1", "Test Job",
		Schedule{Cron: "0 0 9 * * *"},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	if err := store.Save(ctx, job); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// First get should populate cache
	got1, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Delete from backend directly (bypassing cache)
	delete(backend.data, jobKeyPrefix+"job-1")

	// Second get should return cached value
	got2, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("Second Get failed: %v", err)
	}

	if got1.ID != got2.ID {
		t.Errorf("cached value mismatch")
	}

	// Clear cache
	store.ClearCache()

	// Now should get not found
	_, err = store.Get(ctx, "job-1")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound after cache clear, got %v", err)
	}
}

func TestStore_Count(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	ctx := context.Background()

	// Empty store
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 jobs, got %d", count)
	}

	// Add some jobs
	for i := 0; i < 3; i++ {
		job := NewJob("job-"+string(rune('1'+i)), "Job",
			Schedule{Cron: "0 0 9 * * *"},
			Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
		)
		if err := store.Save(ctx, job); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	count, err = store.Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 jobs, got %d", count)
	}
}
