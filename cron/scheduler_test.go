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

func TestExecuteJob_NoDuplicateConcurrentRuns(t *testing.T) {
	ctx := context.Background()
	store := NewStore(StoreConfig{Backend: newListableMockStore()})

	var invocations atomic.Int32
	blockCh := make(chan struct{})
	started := make(chan struct{}, 4)

	scheduler := NewScheduler(SchedulerConfig{
		Store: store,
		Handler: func(ctx context.Context, job *Job) ExecutionResult {
			invocations.Add(1)
			started <- struct{}{}
			<-blockCh
			return ExecutionResult{Success: true, StartedAt: time.Now(), FinishedAt: time.Now()}
		},
	})

	job := NewJob("job-dup", "Long Job",
		Schedule{Interval: Duration(time.Millisecond)},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)
	if err := store.Save(ctx, job); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// First launch runs and blocks in the handler.
	done1 := make(chan struct{})
	go func() {
		scheduler.executeJob(ctx, job)
		close(done1)
	}()
	<-started

	// A second launch while the first is in flight must be skipped.
	done2 := make(chan struct{})
	go func() {
		scheduler.executeJob(ctx, job)
		close(done2)
	}()
	<-done2 // returns immediately without invoking the handler

	if got := invocations.Load(); got != 1 {
		t.Fatalf("expected 1 handler invocation while first run in flight, got %d", got)
	}

	// After the first run finishes, the job can run again.
	close(blockCh)
	<-done1

	blockCh = make(chan struct{})
	close(blockCh) // don't block the next run
	scheduler.executeJob(ctx, job)

	if got := invocations.Load(); got != 2 {
		t.Fatalf("expected 2 handler invocations after first run completed, got %d", got)
	}
}

func TestCheckSpecialJobs_IntervalJobNoOverlap(t *testing.T) {
	ctx := context.Background()
	store := NewStore(StoreConfig{Backend: newListableMockStore()})

	var invocations atomic.Int32
	blockCh := make(chan struct{})
	started := make(chan struct{}, 8)

	scheduler := NewScheduler(SchedulerConfig{
		Store: store,
		Handler: func(ctx context.Context, job *Job) ExecutionResult {
			invocations.Add(1)
			started <- struct{}{}
			<-blockCh
			return ExecutionResult{Success: true, StartedAt: time.Now(), FinishedAt: time.Now()}
		},
	})

	// Interval far shorter than the run duration and past-due since creation.
	job := NewJob("job-interval", "Overlapping Interval Job",
		Schedule{Interval: Duration(time.Nanosecond)},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)
	job.CreatedAt = time.Now().Add(-time.Hour)
	if err := store.Save(ctx, job); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Several ticks while the first run is still in flight.
	scheduler.checkSpecialJobs(ctx)
	<-started
	scheduler.checkSpecialJobs(ctx)
	scheduler.checkSpecialJobs(ctx)

	// Give skipped goroutines a moment to run their guard check.
	time.Sleep(50 * time.Millisecond)

	if got := invocations.Load(); got != 1 {
		t.Fatalf("expected 1 handler invocation despite repeated due ticks, got %d", got)
	}

	close(blockCh)
}

func TestCheckSpecialJobs_OnceJobFiresExactlyOnce(t *testing.T) {
	ctx := context.Background()
	store := NewStore(StoreConfig{Backend: newListableMockStore()})

	var invocations atomic.Int32
	done := make(chan struct{}, 4)

	scheduler := NewScheduler(SchedulerConfig{
		Store: store,
		Handler: func(ctx context.Context, job *Job) ExecutionResult {
			invocations.Add(1)
			done <- struct{}{}
			return ExecutionResult{Success: true, StartedAt: time.Now(), FinishedAt: time.Now()}
		},
	})

	past := time.Now().Add(-time.Minute)
	job := NewJob("job-once", "One Time Job",
		Schedule{Once: &past},
		Action{Type: ActionTypeSendMessage, SessionID: "s1", Message: "Hello"},
	)
	if err := store.Save(ctx, job); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	scheduler.checkSpecialJobs(ctx)
	<-done

	// Later ticks must not fire the job again: it was disabled before launch
	// and stays disabled after execution.
	scheduler.checkSpecialJobs(ctx)
	scheduler.checkSpecialJobs(ctx)
	time.Sleep(50 * time.Millisecond)

	if got := invocations.Load(); got != 1 {
		t.Fatalf("expected exactly 1 execution of a one-time job, got %d", got)
	}

	got, err := scheduler.GetJob(ctx, "job-once")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if got.Status != JobStatusDisabled {
		t.Errorf("one-time job status after execution = %q, want %q", got.Status, JobStatusDisabled)
	}
}
