package openai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/plexusone/omnistorage-core/kvs/backend/memory"

	openaiapi "github.com/plexusone/omniagent/api/openai"
	"github.com/plexusone/omniagent/cron"
)

// newTestCronHandler returns a cronHandler backed by a real, in-memory
// cron.Scheduler (not started — AddJob/GetJob/etc. all work without
// Start() since they operate on the store directly).
func newTestCronHandler(t *testing.T) *cronHandler {
	t.Helper()
	backend := memory.New()
	t.Cleanup(func() { _ = backend.Close() })

	store := cron.NewStore(cron.StoreConfig{Backend: backend})
	sched := cron.NewScheduler(cron.SchedulerConfig{Store: store})

	h := newCronHandler(func() *cron.Scheduler { return sched })
	return &h
}

// newUnconfiguredCronHandler returns a cronHandler whose getScheduler always
// returns nil, mimicking an adapter with no cron skill registered.
func newUnconfiguredCronHandler() *cronHandler {
	h := newCronHandler(func() *cron.Scheduler { return nil })
	return &h
}

func TestCronHandler_NotConfigured(t *testing.T) {
	h := newUnconfiguredCronHandler()
	ctx := context.Background()

	if _, err := h.ListCronJobs(ctx); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("ListCronJobs error = %v, want 'not configured'", err)
	}
	if _, err := h.GetCronJob(ctx, "x"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("GetCronJob error = %v, want 'not configured'", err)
	}
	if _, err := h.CreateCronJob(ctx, &openaiapi.CreateCronJobRequest{}); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("CreateCronJob error = %v, want 'not configured'", err)
	}
	if _, err := h.UpdateCronJob(ctx, "x", &openaiapi.UpdateCronJobRequest{}); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("UpdateCronJob error = %v, want 'not configured'", err)
	}
	if err := h.DeleteCronJob(ctx, "x"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("DeleteCronJob error = %v, want 'not configured'", err)
	}
	if _, err := h.TriggerCronJob(ctx, "x"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("TriggerCronJob error = %v, want 'not configured'", err)
	}
	if err := h.EnableCronJob(ctx, "x"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("EnableCronJob error = %v, want 'not configured'", err)
	}
	if err := h.DisableCronJob(ctx, "x"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("DisableCronJob error = %v, want 'not configured'", err)
	}
}

func TestCronHandler_CreateListGetJob(t *testing.T) {
	h := newTestCronHandler(t)
	ctx := context.Background()

	jobs, err := h.ListCronJobs(ctx)
	if err != nil {
		t.Fatalf("ListCronJobs failed: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("got %d jobs, want 0", len(jobs))
	}

	req := &openaiapi.CreateCronJobRequest{
		Name:        "daily report",
		Description: "sends a daily report",
		Schedule:    openaiapi.CronScheduleInfo{Cron: "0 9 * * *"},
		Action: openaiapi.CronActionInfo{
			Type:      string(cron.ActionTypeSendMessage),
			SessionID: "sess-1",
			Message:   "good morning",
		},
	}

	created, err := h.CreateCronJob(ctx, req)
	if err != nil {
		t.Fatalf("CreateCronJob failed: %v", err)
	}
	if created.Name != "daily report" {
		t.Errorf("Name = %s, want 'daily report'", created.Name)
	}
	if created.Schedule.Cron != "0 9 * * *" {
		t.Errorf("Schedule.Cron = %s, want '0 9 * * *'", created.Schedule.Cron)
	}
	if created.Action.Type != string(cron.ActionTypeSendMessage) {
		t.Errorf("Action.Type = %s, want %s", created.Action.Type, cron.ActionTypeSendMessage)
	}
	if created.Action.Message != "good morning" {
		t.Errorf("Action.Message = %s, want 'good morning'", created.Action.Message)
	}
	if created.Status != string(cron.JobStatusEnabled) {
		t.Errorf("Status = %s, want %s", created.Status, cron.JobStatusEnabled)
	}
	if created.ID == "" {
		t.Error("expected a generated job ID")
	}

	got, err := h.GetCronJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCronJob failed: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetCronJob ID = %s, want %s", got.ID, created.ID)
	}

	jobs, err = h.ListCronJobs(ctx)
	if err != nil {
		t.Fatalf("ListCronJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
}

func TestCronHandler_CreateCronJob_InvalidSchedule(t *testing.T) {
	h := newTestCronHandler(t)
	ctx := context.Background()

	req := &openaiapi.CreateCronJobRequest{
		Name:     "bad once",
		Schedule: openaiapi.CronScheduleInfo{Once: "not-a-timestamp"},
	}
	if _, err := h.CreateCronJob(ctx, req); err == nil || !strings.Contains(err.Error(), "invalid schedule") {
		t.Errorf("error = %v, want 'invalid schedule'", err)
	}

	req2 := &openaiapi.CreateCronJobRequest{
		Name:     "bad interval",
		Schedule: openaiapi.CronScheduleInfo{Interval: "not-a-duration"},
	}
	if _, err := h.CreateCronJob(ctx, req2); err == nil || !strings.Contains(err.Error(), "invalid schedule") {
		t.Errorf("error = %v, want 'invalid schedule'", err)
	}
}

func TestCronHandler_GetCronJob_NotFound(t *testing.T) {
	h := newTestCronHandler(t)
	if _, err := h.GetCronJob(context.Background(), "nonexistent"); err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestCronHandler_UpdateCronJob(t *testing.T) {
	h := newTestCronHandler(t)
	ctx := context.Background()

	created, err := h.CreateCronJob(ctx, &openaiapi.CreateCronJobRequest{
		Name:     "original",
		Schedule: openaiapi.CronScheduleInfo{Cron: "0 9 * * *"},
		Action:   openaiapi.CronActionInfo{Type: string(cron.ActionTypeSendMessage), SessionID: "sess-1", Message: "hi"},
	})
	if err != nil {
		t.Fatalf("CreateCronJob failed: %v", err)
	}

	newName := "renamed"
	newDesc := "new description"
	newSchedule := openaiapi.CronScheduleInfo{Interval: "1h"}
	newAction := openaiapi.CronActionInfo{Type: string(cron.ActionTypeCallTool), ToolName: "do_thing"}

	updated, err := h.UpdateCronJob(ctx, created.ID, &openaiapi.UpdateCronJobRequest{
		Name:        &newName,
		Description: &newDesc,
		Schedule:    &newSchedule,
		Action:      &newAction,
	})
	if err != nil {
		t.Fatalf("UpdateCronJob failed: %v", err)
	}
	if updated.Name != "renamed" {
		t.Errorf("Name = %s, want renamed", updated.Name)
	}
	if updated.Description != "new description" {
		t.Errorf("Description = %s, want 'new description'", updated.Description)
	}
	if updated.Schedule.Interval != "1h0m0s" {
		t.Errorf("Schedule.Interval = %s, want 1h0m0s", updated.Schedule.Interval)
	}
	if updated.Action.Type != string(cron.ActionTypeCallTool) {
		t.Errorf("Action.Type = %s, want %s", updated.Action.Type, cron.ActionTypeCallTool)
	}
	if updated.Action.ToolName != "do_thing" {
		t.Errorf("Action.ToolName = %s, want do_thing", updated.Action.ToolName)
	}
}

func TestCronHandler_UpdateCronJob_NotFound(t *testing.T) {
	h := newTestCronHandler(t)
	newName := "x"
	_, err := h.UpdateCronJob(context.Background(), "nonexistent", &openaiapi.UpdateCronJobRequest{Name: &newName})
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestCronHandler_UpdateCronJob_InvalidSchedule(t *testing.T) {
	h := newTestCronHandler(t)
	ctx := context.Background()

	created, err := h.CreateCronJob(ctx, &openaiapi.CreateCronJobRequest{
		Name:     "original",
		Schedule: openaiapi.CronScheduleInfo{Cron: "0 9 * * *"},
		Action:   openaiapi.CronActionInfo{Type: string(cron.ActionTypeSendMessage), SessionID: "sess-1", Message: "hi"},
	})
	if err != nil {
		t.Fatalf("CreateCronJob failed: %v", err)
	}

	badSchedule := openaiapi.CronScheduleInfo{Once: "not-a-timestamp"}
	_, err = h.UpdateCronJob(ctx, created.ID, &openaiapi.UpdateCronJobRequest{Schedule: &badSchedule})
	if err == nil || !strings.Contains(err.Error(), "invalid schedule") {
		t.Errorf("error = %v, want 'invalid schedule'", err)
	}
}

func TestCronHandler_DeleteCronJob(t *testing.T) {
	h := newTestCronHandler(t)
	ctx := context.Background()

	created, err := h.CreateCronJob(ctx, &openaiapi.CreateCronJobRequest{
		Name:     "to-delete",
		Schedule: openaiapi.CronScheduleInfo{Cron: "0 9 * * *"},
		Action:   openaiapi.CronActionInfo{Type: string(cron.ActionTypeSendMessage), SessionID: "sess-1", Message: "hi"},
	})
	if err != nil {
		t.Fatalf("CreateCronJob failed: %v", err)
	}

	if err := h.DeleteCronJob(ctx, created.ID); err != nil {
		t.Fatalf("DeleteCronJob failed: %v", err)
	}

	if _, err := h.GetCronJob(ctx, created.ID); err == nil {
		t.Error("expected error getting deleted job")
	}
}

func TestCronHandler_TriggerCronJob(t *testing.T) {
	h := newTestCronHandler(t)
	ctx := context.Background()

	created, err := h.CreateCronJob(ctx, &openaiapi.CreateCronJobRequest{
		Name:     "trigger-me",
		Schedule: openaiapi.CronScheduleInfo{Cron: "0 9 * * *"},
		Action:   openaiapi.CronActionInfo{Type: string(cron.ActionTypeSendMessage), SessionID: "sess-1", Message: "hi"},
	})
	if err != nil {
		t.Fatalf("CreateCronJob failed: %v", err)
	}

	result, err := h.TriggerCronJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("TriggerCronJob failed: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success when no handler wired (default success), got error=%s", result.Error)
	}
	if result.StartedAt == "" {
		t.Error("expected a non-empty StartedAt timestamp")
	}

	// RunCount should have incremented as a side effect of triggering.
	got, err := h.GetCronJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCronJob failed: %v", err)
	}
	if got.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", got.RunCount)
	}
	if got.LastRunAt == nil {
		t.Error("expected LastRunAt to be set after trigger")
	}
}

func TestCronHandler_EnableDisableCronJob(t *testing.T) {
	h := newTestCronHandler(t)
	ctx := context.Background()

	created, err := h.CreateCronJob(ctx, &openaiapi.CreateCronJobRequest{
		Name:     "togglable",
		Schedule: openaiapi.CronScheduleInfo{Cron: "0 9 * * *"},
		Action:   openaiapi.CronActionInfo{Type: string(cron.ActionTypeSendMessage), SessionID: "sess-1", Message: "hi"},
	})
	if err != nil {
		t.Fatalf("CreateCronJob failed: %v", err)
	}

	if err := h.DisableCronJob(ctx, created.ID); err != nil {
		t.Fatalf("DisableCronJob failed: %v", err)
	}
	got, err := h.GetCronJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCronJob failed: %v", err)
	}
	if got.Status != string(cron.JobStatusDisabled) {
		t.Errorf("Status = %s, want %s", got.Status, cron.JobStatusDisabled)
	}

	if err := h.EnableCronJob(ctx, created.ID); err != nil {
		t.Fatalf("EnableCronJob failed: %v", err)
	}
	got, err = h.GetCronJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCronJob failed: %v", err)
	}
	if got.Status != string(cron.JobStatusEnabled) {
		t.Errorf("Status = %s, want %s", got.Status, cron.JobStatusEnabled)
	}
}

func TestCronJobToInfo(t *testing.T) {
	once := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	job := cron.NewJob("job-1", "test job", cron.Schedule{Once: &once}, cron.Action{
		Type:           cron.ActionTypeCallWebhook,
		WebhookURL:     "https://example.com/hook",
		WebhookMethod:  "POST",
		WebhookHeaders: map[string]string{"X-Test": "1"},
		WebhookBody:    `{"ok":true}`,
	})
	job.Description = "a webhook job"

	info := cronJobToInfo(job)

	if info.ID != "job-1" {
		t.Errorf("ID = %s, want job-1", info.ID)
	}
	if info.Description != "a webhook job" {
		t.Errorf("Description = %s, want 'a webhook job'", info.Description)
	}
	if info.Schedule.Once != once.Format(time.RFC3339) {
		t.Errorf("Schedule.Once = %s, want %s", info.Schedule.Once, once.Format(time.RFC3339))
	}
	if info.Action.WebhookURL != "https://example.com/hook" {
		t.Errorf("Action.WebhookURL = %s, want https://example.com/hook", info.Action.WebhookURL)
	}
	if info.Action.WebhookHeaders["X-Test"] != "1" {
		t.Errorf("Action.WebhookHeaders[X-Test] = %s, want 1", info.Action.WebhookHeaders["X-Test"])
	}
}

func TestParseScheduleInfo(t *testing.T) {
	t.Run("cron expression", func(t *testing.T) {
		s, err := parseScheduleInfo(openaiapi.CronScheduleInfo{Cron: "*/5 * * * *"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Cron != "*/5 * * * *" {
			t.Errorf("Cron = %s, want */5 * * * *", s.Cron)
		}
	})

	t.Run("once timestamp", func(t *testing.T) {
		ts := "2026-06-01T10:00:00Z"
		s, err := parseScheduleInfo(openaiapi.CronScheduleInfo{Once: ts})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Once == nil || s.Once.Format(time.RFC3339) != ts {
			t.Errorf("Once = %v, want %s", s.Once, ts)
		}
	})

	t.Run("invalid once timestamp", func(t *testing.T) {
		_, err := parseScheduleInfo(openaiapi.CronScheduleInfo{Once: "garbage"})
		if err == nil || !strings.Contains(err.Error(), "invalid once time") {
			t.Errorf("error = %v, want 'invalid once time'", err)
		}
	})

	t.Run("interval duration", func(t *testing.T) {
		s, err := parseScheduleInfo(openaiapi.CronScheduleInfo{Interval: "30m"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if time.Duration(s.Interval) != 30*time.Minute {
			t.Errorf("Interval = %v, want 30m", time.Duration(s.Interval))
		}
	})

	t.Run("invalid interval", func(t *testing.T) {
		_, err := parseScheduleInfo(openaiapi.CronScheduleInfo{Interval: "not-a-duration"})
		if err == nil || !strings.Contains(err.Error(), "invalid interval") {
			t.Errorf("error = %v, want 'invalid interval'", err)
		}
	})
}

func TestParseActionInfo(t *testing.T) {
	info := openaiapi.CronActionInfo{
		Type:           string(cron.ActionTypeCallTool),
		ToolName:       "search",
		ToolParams:     map[string]any{"q": "golang"},
		SessionID:      "sess-1",
		Message:        "hello",
		WebhookURL:     "https://example.com",
		WebhookMethod:  "PUT",
		WebhookHeaders: map[string]string{"A": "B"},
		WebhookBody:    "body",
	}

	action := parseActionInfo(info)

	if action.Type != cron.ActionTypeCallTool {
		t.Errorf("Type = %s, want %s", action.Type, cron.ActionTypeCallTool)
	}
	if action.ToolName != "search" {
		t.Errorf("ToolName = %s, want search", action.ToolName)
	}
	if action.ToolParams["q"] != "golang" {
		t.Errorf("ToolParams[q] = %v, want golang", action.ToolParams["q"])
	}
	if action.SessionID != "sess-1" {
		t.Errorf("SessionID = %s, want sess-1", action.SessionID)
	}
	if action.WebhookMethod != "PUT" {
		t.Errorf("WebhookMethod = %s, want PUT", action.WebhookMethod)
	}
}
