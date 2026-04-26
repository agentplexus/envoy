package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/grokify/mogo/log/slogutil"
	"github.com/robfig/cron/v3"
)

// Scheduler manages scheduled job execution.
type Scheduler struct {
	store   *Store
	cron    *cron.Cron
	handler ExecutionHandler

	// entryMap maps job IDs to cron entry IDs
	entryMap map[string]cron.EntryID
	mu       sync.RWMutex

	// stopCh signals the scheduler to stop
	stopCh chan struct{}

	// running indicates if the scheduler is active
	running bool
}

// SchedulerConfig configures the scheduler.
type SchedulerConfig struct {
	// Store is the job storage backend.
	Store *Store

	// Handler is called when a job executes.
	// If nil, jobs will be logged but not executed.
	Handler ExecutionHandler

	// Location is the timezone for cron expressions.
	// If nil, uses time.Local.
	Location *time.Location
}

// NewScheduler creates a new scheduler.
func NewScheduler(config SchedulerConfig) *Scheduler {
	loc := config.Location
	if loc == nil {
		loc = time.Local
	}

	return &Scheduler{
		store:    config.Store,
		cron:     cron.New(cron.WithLocation(loc), cron.WithSeconds()),
		handler:  config.Handler,
		entryMap: make(map[string]cron.EntryID),
		stopCh:   make(chan struct{}),
	}
}

// Start loads jobs from the store and begins scheduling.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	s.running = true
	s.mu.Unlock()

	logger := slogutil.LoggerFromContext(ctx, slog.Default())

	// Load enabled jobs
	jobs, err := s.store.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("load jobs: %w", err)
	}

	// Schedule each job
	for _, job := range jobs {
		if err := s.scheduleJob(ctx, job); err != nil {
			logger.Error("failed to schedule job", "job_id", job.ID, "error", err)
		}
	}

	// Start the cron scheduler
	s.cron.Start()
	logger.Info("cron scheduler started", "job_count", len(jobs))

	// Start interval and once job processor
	go s.processSpecialJobs(ctx)

	return nil
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)

	// Stop the cron scheduler and wait for running jobs
	stopCtx := s.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-ctx.Done():
		return ctx.Err()
	}

	logger := slogutil.LoggerFromContext(ctx, slog.Default())
	logger.Info("cron scheduler stopped")

	return nil
}

// scheduleJob adds a job to the cron scheduler.
func (s *Scheduler) scheduleJob(ctx context.Context, job *Job) error {
	// Only schedule cron-based jobs here
	if job.Schedule.Cron == "" {
		return nil // Interval and once jobs are handled separately
	}

	entryID, err := s.cron.AddFunc(job.Schedule.Cron, s.makeJobFunc(job.ID))
	if err != nil {
		return fmt.Errorf("add cron entry: %w", err)
	}

	s.mu.Lock()
	s.entryMap[job.ID] = entryID
	s.mu.Unlock()

	// Update next run time
	entry := s.cron.Entry(entryID)
	nextRun := entry.Next
	job.NextRunAt = &nextRun
	if err := s.store.Save(ctx, job); err != nil {
		logger := slogutil.LoggerFromContext(ctx, slog.Default())
		logger.Error("failed to save job next run time", "job_id", job.ID, "error", err)
	}

	return nil
}

// processSpecialJobs handles interval and one-time jobs.
func (s *Scheduler) processSpecialJobs(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkSpecialJobs(ctx)
		}
	}
}

// checkSpecialJobs checks for interval and once jobs that need to run.
func (s *Scheduler) checkSpecialJobs(ctx context.Context) {
	jobs, err := s.store.ListEnabled(ctx)
	if err != nil {
		logger := slogutil.LoggerFromContext(ctx, slog.Default())
		logger.Error("failed to list enabled jobs", "error", err)
		return
	}

	now := time.Now()
	for _, job := range jobs {
		// Skip cron-based jobs
		if job.Schedule.Cron != "" {
			continue
		}

		// Check one-time jobs
		if job.Schedule.Once != nil {
			if now.After(*job.Schedule.Once) || now.Equal(*job.Schedule.Once) {
				go s.executeJob(ctx, job)
				// Disable one-time jobs after execution
				job.Status = JobStatusDisabled
				if err := s.store.Save(ctx, job); err != nil {
					logger := slogutil.LoggerFromContext(ctx, slog.Default())
					logger.Error("failed to disable one-time job", "job_id", job.ID, "error", err)
				}
			}
			continue
		}

		// Check interval jobs
		if job.Schedule.Interval > 0 {
			var shouldRun bool
			if job.LastRunAt == nil {
				// Never run, check if interval has passed since creation
				shouldRun = now.Sub(job.CreatedAt) >= time.Duration(job.Schedule.Interval)
			} else {
				shouldRun = now.Sub(*job.LastRunAt) >= time.Duration(job.Schedule.Interval)
			}

			if shouldRun {
				go s.executeJob(ctx, job)
			}
		}
	}
}

// makeJobFunc creates a function that executes a job by ID.
func (s *Scheduler) makeJobFunc(jobID string) func() {
	return func() {
		ctx := context.Background()
		job, err := s.store.Get(ctx, jobID)
		if err != nil {
			logger := slogutil.LoggerFromContext(ctx, slog.Default())
			logger.Error("failed to get job for execution", "job_id", jobID, "error", err)
			return
		}

		if job.Status != JobStatusEnabled {
			return // Job was disabled
		}

		s.executeJob(ctx, job)
	}
}

// executeJob runs a job and records the result.
func (s *Scheduler) executeJob(ctx context.Context, job *Job) {
	logger := slogutil.LoggerFromContext(ctx, slog.Default())

	// Mark as running
	job.Status = JobStatusRunning
	if err := s.store.Save(ctx, job); err != nil {
		logger.Error("failed to mark job as running", "job_id", job.ID, "error", err)
	}

	var result ExecutionResult
	if s.handler != nil {
		result = s.handler(ctx, job)
	} else {
		// Default handler just logs
		result = ExecutionResult{
			Success:    true,
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		}
		logger.Info("job executed (no handler)", "job_id", job.ID, "job_name", job.Name)
	}

	// Update job state
	now := time.Now()
	job.LastRunAt = &now
	job.RunCount++

	if result.Success {
		job.LastError = ""
		job.Status = JobStatusEnabled
	} else {
		job.LastError = result.Error
		job.Status = JobStatusEnabled // Stay enabled for retry
		logger.Error("job execution failed", "job_id", job.ID, "error", result.Error)
	}

	// Calculate next run time for cron jobs
	s.mu.RLock()
	entryID, hasCronEntry := s.entryMap[job.ID]
	s.mu.RUnlock()

	if hasCronEntry {
		entry := s.cron.Entry(entryID)
		job.NextRunAt = &entry.Next
	} else if job.Schedule.Interval > 0 {
		next := now.Add(time.Duration(job.Schedule.Interval))
		job.NextRunAt = &next
	} else {
		job.NextRunAt = nil
	}

	if err := s.store.Save(ctx, job); err != nil {
		logger.Error("failed to save job after execution", "job_id", job.ID, "error", err)
	}

	logger.Info("job completed",
		"job_id", job.ID,
		"success", result.Success,
		"duration", result.Duration,
		"run_count", job.RunCount,
	)
}

// AddJob adds a new job to the scheduler.
func (s *Scheduler) AddJob(ctx context.Context, job *Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	// Save to store
	if err := s.store.Save(ctx, job); err != nil {
		return fmt.Errorf("save job: %w", err)
	}

	// Schedule if running and enabled
	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()

	if running && job.Status == JobStatusEnabled {
		if err := s.scheduleJob(ctx, job); err != nil {
			return fmt.Errorf("schedule job: %w", err)
		}
	}

	return nil
}

// UpdateJob updates an existing job.
func (s *Scheduler) UpdateJob(ctx context.Context, job *Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	job.UpdatedAt = time.Now()

	// Remove old schedule if exists
	s.mu.Lock()
	if entryID, ok := s.entryMap[job.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.entryMap, job.ID)
	}
	s.mu.Unlock()

	// Save to store
	if err := s.store.Save(ctx, job); err != nil {
		return fmt.Errorf("save job: %w", err)
	}

	// Re-schedule if running and enabled
	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()

	if running && job.Status == JobStatusEnabled {
		if err := s.scheduleJob(ctx, job); err != nil {
			return fmt.Errorf("schedule job: %w", err)
		}
	}

	return nil
}

// RemoveJob removes a job from the scheduler.
func (s *Scheduler) RemoveJob(ctx context.Context, id string) error {
	// Remove from cron
	s.mu.Lock()
	if entryID, ok := s.entryMap[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entryMap, id)
	}
	s.mu.Unlock()

	// Remove from store
	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}

	return nil
}

// TriggerJob runs a job immediately, regardless of schedule.
func (s *Scheduler) TriggerJob(ctx context.Context, id string) (*ExecutionResult, error) {
	job, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Execute synchronously
	var result ExecutionResult
	if s.handler != nil {
		result = s.handler(ctx, job)
	} else {
		result = ExecutionResult{
			Success:    true,
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		}
	}

	// Update job state
	now := time.Now()
	job.LastRunAt = &now
	job.RunCount++

	if result.Success {
		job.LastError = ""
	} else {
		job.LastError = result.Error
	}

	if err := s.store.Save(ctx, job); err != nil {
		logger := slogutil.LoggerFromContext(ctx, slog.Default())
		logger.Error("failed to save job after manual trigger", "job_id", id, "error", err)
	}

	return &result, nil
}

// EnableJob enables a disabled job.
func (s *Scheduler) EnableJob(ctx context.Context, id string) error {
	job, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	if job.Status == JobStatusEnabled {
		return nil // Already enabled
	}

	job.Status = JobStatusEnabled
	job.UpdatedAt = time.Now()

	// Save and schedule
	if err := s.store.Save(ctx, job); err != nil {
		return fmt.Errorf("save job: %w", err)
	}

	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()

	if running {
		if err := s.scheduleJob(ctx, job); err != nil {
			return fmt.Errorf("schedule job: %w", err)
		}
	}

	return nil
}

// DisableJob disables a job without deleting it.
func (s *Scheduler) DisableJob(ctx context.Context, id string) error {
	job, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	if job.Status == JobStatusDisabled {
		return nil // Already disabled
	}

	// Remove from cron
	s.mu.Lock()
	if entryID, ok := s.entryMap[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entryMap, id)
	}
	s.mu.Unlock()

	job.Status = JobStatusDisabled
	job.NextRunAt = nil
	job.UpdatedAt = time.Now()

	return s.store.Save(ctx, job)
}

// GetJob retrieves a job by ID.
func (s *Scheduler) GetJob(ctx context.Context, id string) (*Job, error) {
	return s.store.Get(ctx, id)
}

// ListJobs returns all jobs.
func (s *Scheduler) ListJobs(ctx context.Context) ([]*Job, error) {
	return s.store.List(ctx)
}

// IsRunning returns true if the scheduler is active.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}
