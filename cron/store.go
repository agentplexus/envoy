package cron

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/plexusone/omnistorage-core/kvs"
)

const (
	// jobKeyPrefix is the prefix for job keys in the KVS store.
	jobKeyPrefix = "cron:job:"
)

// ErrJobNotFound is returned when a job is not found.
var ErrJobNotFound = errors.New("job not found")

// Store manages persistent job storage.
type Store struct {
	backend kvs.Store
	cache   map[string]*Job
	mu      sync.RWMutex
}

// StoreConfig configures the job store.
type StoreConfig struct {
	// Backend is the KVS storage backend.
	Backend kvs.Store
}

// NewStore creates a new job store.
func NewStore(config StoreConfig) *Store {
	return &Store{
		backend: config.Backend,
		cache:   make(map[string]*Job),
	}
}

// Get retrieves a job by ID.
// Returns ErrJobNotFound if the job doesn't exist.
func (s *Store) Get(ctx context.Context, id string) (*Job, error) {
	// Check cache first
	s.mu.RLock()
	if job, ok := s.cache[id]; ok {
		s.mu.RUnlock()
		return job, nil
	}
	s.mu.RUnlock()

	// Load from backend
	key := jobKeyPrefix + id
	data, err := s.backend.Get(ctx, key)
	if err != nil {
		if errors.Is(err, kvs.ErrNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("get job: %w", err)
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}

	// Update cache
	s.mu.Lock()
	s.cache[id] = &job
	s.mu.Unlock()

	return &job, nil
}

// Save persists a job to storage.
func (s *Store) Save(ctx context.Context, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	key := jobKeyPrefix + job.ID
	// Jobs don't expire, use 0 TTL
	if err := s.backend.Set(ctx, key, data, 0); err != nil {
		return fmt.Errorf("set job: %w", err)
	}

	// Update cache
	s.mu.Lock()
	s.cache[job.ID] = job
	s.mu.Unlock()

	return nil
}

// Delete removes a job.
func (s *Store) Delete(ctx context.Context, id string) error {
	key := jobKeyPrefix + id
	if err := s.backend.Delete(ctx, key); err != nil {
		if !errors.Is(err, kvs.ErrNotFound) {
			return fmt.Errorf("delete job: %w", err)
		}
	}

	// Remove from cache
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()

	return nil
}

// List returns all jobs.
// This requires the backend to implement kvs.ListableStore.
func (s *Store) List(ctx context.Context) ([]*Job, error) {
	listable, ok := s.backend.(kvs.ListableStore)
	if !ok {
		// Fall back to cached jobs if backend doesn't support listing
		s.mu.RLock()
		jobs := make([]*Job, 0, len(s.cache))
		for _, job := range s.cache {
			jobs = append(jobs, job)
		}
		s.mu.RUnlock()
		return jobs, nil
	}

	keys, err := listable.List(ctx, jobKeyPrefix)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	// Load all jobs
	jobs := make([]*Job, 0, len(keys))
	prefixLen := len(jobKeyPrefix)
	for _, key := range keys {
		var id string
		if len(key) > prefixLen {
			id = key[prefixLen:]
		} else {
			id = key
		}

		job, err := s.Get(ctx, id)
		if err != nil {
			if errors.Is(err, ErrJobNotFound) {
				continue // Skip deleted jobs
			}
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// ListEnabled returns all enabled jobs.
func (s *Store) ListEnabled(ctx context.Context) ([]*Job, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	enabled := make([]*Job, 0, len(all))
	for _, job := range all {
		if job.Status == JobStatusEnabled {
			enabled = append(enabled, job)
		}
	}
	return enabled, nil
}

// ListByStatus returns jobs with the given status.
func (s *Store) ListByStatus(ctx context.Context, status JobStatus) ([]*Job, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*Job, 0, len(all))
	for _, job := range all {
		if job.Status == status {
			result = append(result, job)
		}
	}
	return result, nil
}

// ClearCache removes all cached jobs.
// This does not delete jobs from the backend.
func (s *Store) ClearCache() {
	s.mu.Lock()
	s.cache = make(map[string]*Job)
	s.mu.Unlock()
}

// Count returns the number of jobs.
func (s *Store) Count(ctx context.Context) (int, error) {
	jobs, err := s.List(ctx)
	if err != nil {
		return 0, err
	}
	return len(jobs), nil
}

// Close closes the underlying backend.
func (s *Store) Close() error {
	return s.backend.Close()
}
