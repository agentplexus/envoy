package cron

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler_AddJob(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	scheduler := NewScheduler(SchedulerConfig{Store: store})
	ctx := context.Background()

	job := NewJob("job-1", "Test Job",
		Schedule{Cron: "0 0 9 * * *"},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	if err := scheduler.AddJob(ctx, job); err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	// Verify job is stored
	got, err := scheduler.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if got.Name != job.Name {
		t.Errorf("expected name %q, got %q", job.Name, got.Name)
	}
}

func TestScheduler_AddJob_Invalid(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	scheduler := NewScheduler(SchedulerConfig{Store: store})
	ctx := context.Background()

	// Job without schedule
	job := &Job{
		ID:   "job-1",
		Name: "Test Job",
		Action: Action{
			Type:      ActionTypeSendMessage,
			SessionID: "s1",
			Message:   "Hello",
		},
	}

	err := scheduler.AddJob(ctx, job)
	if err == nil {
		t.Error("expected error for invalid job")
	}
}

func TestScheduler_RemoveJob(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	scheduler := NewScheduler(SchedulerConfig{Store: store})
	ctx := context.Background()

	job := NewJob("job-1", "Test Job",
		Schedule{Cron: "0 0 9 * * *"},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	if err := scheduler.AddJob(ctx, job); err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	if err := scheduler.RemoveJob(ctx, "job-1"); err != nil {
		t.Fatalf("RemoveJob failed: %v", err)
	}

	_, err := scheduler.GetJob(ctx, "job-1")
	if err == nil {
		t.Error("expected error getting removed job")
	}
}

func TestScheduler_EnableDisable(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	scheduler := NewScheduler(SchedulerConfig{Store: store})
	ctx := context.Background()

	job := NewJob("job-1", "Test Job",
		Schedule{Cron: "0 0 9 * * *"},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	if err := scheduler.AddJob(ctx, job); err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	// Disable
	if err := scheduler.DisableJob(ctx, "job-1"); err != nil {
		t.Fatalf("DisableJob failed: %v", err)
	}

	got, _ := scheduler.GetJob(ctx, "job-1")
	if got.Status != JobStatusDisabled {
		t.Errorf("expected disabled, got %q", got.Status)
	}

	// Enable
	if err := scheduler.EnableJob(ctx, "job-1"); err != nil {
		t.Fatalf("EnableJob failed: %v", err)
	}

	got, _ = scheduler.GetJob(ctx, "job-1")
	if got.Status != JobStatusEnabled {
		t.Errorf("expected enabled, got %q", got.Status)
	}
}

func TestScheduler_TriggerJob(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})

	var executed atomic.Bool
	handler := func(_ context.Context, job *Job) ExecutionResult {
		executed.Store(true)
		return ExecutionResult{
			Success:    true,
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		}
	}

	scheduler := NewScheduler(SchedulerConfig{
		Store:   store,
		Handler: handler,
	})
	ctx := context.Background()

	job := NewJob("job-1", "Test Job",
		Schedule{Cron: "0 0 9 * * *"},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	if err := scheduler.AddJob(ctx, job); err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	result, err := scheduler.TriggerJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("TriggerJob failed: %v", err)
	}

	if !result.Success {
		t.Error("expected successful execution")
	}

	if !executed.Load() {
		t.Error("handler was not called")
	}

	// Verify run count increased
	got, _ := scheduler.GetJob(ctx, "job-1")
	if got.RunCount != 1 {
		t.Errorf("expected run count 1, got %d", got.RunCount)
	}
}

func TestScheduler_StartStop(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	scheduler := NewScheduler(SchedulerConfig{Store: store})
	ctx := context.Background()

	// Add a job before starting
	job := NewJob("job-1", "Test Job",
		Schedule{Cron: "0 0 9 * * *"},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)
	if err := store.Save(ctx, job); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Start
	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("expected scheduler to be running")
	}

	// Starting again should fail
	if err := scheduler.Start(ctx); err == nil {
		t.Error("expected error starting already running scheduler")
	}

	// Stop
	if err := scheduler.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if scheduler.IsRunning() {
		t.Error("expected scheduler to be stopped")
	}
}

func TestScheduler_ListJobs(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	scheduler := NewScheduler(SchedulerConfig{Store: store})
	ctx := context.Background()

	// Add jobs
	jobs := []*Job{
		NewJob("job-1", "Job 1", Schedule{Cron: "0 0 9 * * *"}, Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"}),
		NewJob("job-2", "Job 2", Schedule{Cron: "0 0 12 * * *"}, Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hi"}),
	}

	for _, job := range jobs {
		if err := scheduler.AddJob(ctx, job); err != nil {
			t.Fatalf("AddJob failed: %v", err)
		}
	}

	listed, err := scheduler.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	if len(listed) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(listed))
	}
}

func TestScheduler_UpdateJob(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	scheduler := NewScheduler(SchedulerConfig{Store: store})
	ctx := context.Background()

	job := NewJob("job-1", "Test Job",
		Schedule{Cron: "0 0 9 * * *"},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	if err := scheduler.AddJob(ctx, job); err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	// Update the job
	job.Name = "Updated Job"
	job.Schedule.Cron = "0 0 10 * * *"

	if err := scheduler.UpdateJob(ctx, job); err != nil {
		t.Fatalf("UpdateJob failed: %v", err)
	}

	got, _ := scheduler.GetJob(ctx, "job-1")
	if got.Name != "Updated Job" {
		t.Errorf("expected name 'Updated Job', got %q", got.Name)
	}
	if got.Schedule.Cron != "0 0 10 * * *" {
		t.Errorf("expected cron '0 0 10 * * *', got %q", got.Schedule.Cron)
	}
}

func TestScheduler_IntervalJob(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	scheduler := NewScheduler(SchedulerConfig{Store: store})
	ctx := context.Background()

	job := NewJob("job-1", "Interval Job",
		Schedule{Interval: Duration(time.Hour)},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	if err := scheduler.AddJob(ctx, job); err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	// Verify job is stored
	got, err := scheduler.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if got.Schedule.Interval != Duration(time.Hour) {
		t.Errorf("expected interval 1h, got %v", time.Duration(got.Schedule.Interval))
	}
}

func TestScheduler_OnceJob(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})
	scheduler := NewScheduler(SchedulerConfig{Store: store})
	ctx := context.Background()

	futureTime := time.Now().Add(time.Hour)
	job := NewJob("job-1", "Once Job",
		Schedule{Once: &futureTime},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	if err := scheduler.AddJob(ctx, job); err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	// Verify job is stored
	got, err := scheduler.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if got.Schedule.Once == nil {
		t.Error("expected once schedule to be set")
	}
}

func TestScheduler_HandlerError(t *testing.T) {
	store := NewStore(StoreConfig{Backend: newListableMockStore()})

	handler := func(_ context.Context, _ *Job) ExecutionResult {
		return ExecutionResult{
			Success:    false,
			Error:      "something went wrong",
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		}
	}

	scheduler := NewScheduler(SchedulerConfig{
		Store:   store,
		Handler: handler,
	})
	ctx := context.Background()

	job := NewJob("job-1", "Test Job",
		Schedule{Cron: "0 0 9 * * *"},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)

	if err := scheduler.AddJob(ctx, job); err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	result, err := scheduler.TriggerJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("TriggerJob failed: %v", err)
	}

	if result.Success {
		t.Error("expected failed execution")
	}
	if result.Error != "something went wrong" {
		t.Errorf("expected error 'something went wrong', got %q", result.Error)
	}

	// Verify last error is recorded
	got, _ := scheduler.GetJob(ctx, "job-1")
	if got.LastError != "something went wrong" {
		t.Errorf("expected last error 'something went wrong', got %q", got.LastError)
	}
}
