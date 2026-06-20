package cron

import (
	"context"
	"testing"
	"time"
)

func setupSkill(t *testing.T) *Skill {
	t.Helper()

	skill := NewSkill()
	skill.SetStorage(newListableMockStore())

	if err := skill.Init(context.Background()); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	t.Cleanup(func() {
		skill.Close()
	})

	return skill
}

func TestSkill_Name(t *testing.T) {
	skill := NewSkill()
	if skill.Name() != "cron" {
		t.Errorf("expected name 'cron', got %q", skill.Name())
	}
}

func TestSkill_Description(t *testing.T) {
	skill := NewSkill()
	if skill.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestSkill_Tools(t *testing.T) {
	skill := NewSkill()
	tools := skill.Tools()

	expectedTools := []string{
		"cron_create",
		"cron_list",
		"cron_get",
		"cron_delete",
		"cron_enable",
		"cron_disable",
		"cron_trigger",
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(tools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name()] = true
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestSkill_CreateTool(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	params := map[string]any{
		"name":          "Test Job",
		"description":   "A test job",
		"schedule_cron": "0 0 9 * * *",
		"action_type":   "send_message",
		"session_id":    "session-1",
		"message":       "Hello World",
	}

	result, err := skill.handleCreate(ctx, params)
	if err != nil {
		t.Fatalf("handleCreate failed: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	if m["name"] != "Test Job" {
		t.Errorf("expected name 'Test Job', got %v", m["name"])
	}
	if m["status"] != JobStatusEnabled {
		t.Errorf("expected status 'enabled', got %v", m["status"])
	}
	if m["id"] == "" {
		t.Error("expected non-empty id")
	}
}

func TestSkill_CreateTool_Interval(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	params := map[string]any{
		"name":              "Interval Job",
		"schedule_interval": "1h30m",
		"action_type":       "call_webhook",
		"webhook_url":       "https://example.com/hook",
	}

	result, err := skill.handleCreate(ctx, params)
	if err != nil {
		t.Fatalf("handleCreate failed: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	id := m["id"].(string)
	job, _ := skill.scheduler.GetJob(ctx, id)
	if job.Schedule.Interval != Duration(time.Hour+30*time.Minute) {
		t.Errorf("expected interval 1h30m, got %v", time.Duration(job.Schedule.Interval))
	}
}

func TestSkill_CreateTool_Once(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	futureTime := time.Now().Add(time.Hour).Format(time.RFC3339)
	params := map[string]any{
		"name":          "Once Job",
		"schedule_once": futureTime,
		"action_type":   "call_tool",
		"tool_name":     "my_tool",
	}

	result, err := skill.handleCreate(ctx, params)
	if err != nil {
		t.Fatalf("handleCreate failed: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	id := m["id"].(string)
	job, _ := skill.scheduler.GetJob(ctx, id)
	if job.Schedule.Once == nil {
		t.Error("expected once schedule to be set")
	}
}

func TestSkill_CreateTool_MissingName(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	params := map[string]any{
		"schedule_cron": "0 0 9 * * *",
		"action_type":   "send_message",
		"session_id":    "session-1",
		"message":       "Hello",
	}

	_, err := skill.handleCreate(ctx, params)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestSkill_ListTool(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	// Create some jobs
	for i := 0; i < 3; i++ {
		params := map[string]any{
			"name":          "Job",
			"schedule_cron": "0 0 9 * * *",
			"action_type":   "send_message",
			"session_id":    "session-1",
			"message":       "Hello",
		}
		if _, err := skill.handleCreate(ctx, params); err != nil {
			t.Fatalf("handleCreate failed: %v", err)
		}
	}

	// List all
	result, err := skill.handleList(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("handleList failed: %v", err)
	}

	jobs, ok := result.([]map[string]any)
	if !ok {
		t.Fatalf("expected []map result, got %T", result)
	}

	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(jobs))
	}
}

func TestSkill_ListTool_ByStatus(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	// Create jobs
	createParams := map[string]any{
		"name":          "Job",
		"schedule_cron": "0 0 9 * * *",
		"action_type":   "send_message",
		"session_id":    "session-1",
		"message":       "Hello",
	}
	result1, _ := skill.handleCreate(ctx, createParams)
	id1 := result1.(map[string]any)["id"].(string)

	result2, _ := skill.handleCreate(ctx, createParams)
	id2 := result2.(map[string]any)["id"].(string)

	// Disable one
	if _, err := skill.handleDisable(ctx, map[string]any{"id": id2}); err != nil {
		t.Fatalf("handleDisable failed: %v", err)
	}

	// List enabled only
	result, err := skill.handleList(ctx, map[string]any{"status": "enabled"})
	if err != nil {
		t.Fatalf("handleList failed: %v", err)
	}

	jobs := result.([]map[string]any)
	if len(jobs) != 1 {
		t.Errorf("expected 1 enabled job, got %d", len(jobs))
	}
	if jobs[0]["id"] != id1 {
		t.Errorf("expected job %s, got %s", id1, jobs[0]["id"])
	}
}

func TestSkill_GetTool(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	// Create a job
	createParams := map[string]any{
		"name":          "Test Job",
		"description":   "Description",
		"schedule_cron": "0 0 9 * * *",
		"action_type":   "send_message",
		"session_id":    "session-1",
		"message":       "Hello",
	}
	createResult, _ := skill.handleCreate(ctx, createParams)
	id := createResult.(map[string]any)["id"].(string)

	// Get
	result, err := skill.handleGet(ctx, map[string]any{"id": id})
	if err != nil {
		t.Fatalf("handleGet failed: %v", err)
	}

	job, ok := result.(*Job)
	if !ok {
		t.Fatalf("expected *Job, got %T", result)
	}

	if job.Name != "Test Job" {
		t.Errorf("expected name 'Test Job', got %q", job.Name)
	}
	if job.Description != "Description" {
		t.Errorf("expected description 'Description', got %q", job.Description)
	}
}

func TestSkill_GetTool_MissingID(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	_, err := skill.handleGet(ctx, map[string]any{})
	if err == nil {
		t.Error("expected error for missing id")
	}
}

func TestSkill_DeleteTool(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	// Create a job
	createParams := map[string]any{
		"name":          "Test Job",
		"schedule_cron": "0 0 9 * * *",
		"action_type":   "send_message",
		"session_id":    "session-1",
		"message":       "Hello",
	}
	createResult, _ := skill.handleCreate(ctx, createParams)
	id := createResult.(map[string]any)["id"].(string)

	// Delete
	result, err := skill.handleDelete(ctx, map[string]any{"id": id})
	if err != nil {
		t.Fatalf("handleDelete failed: %v", err)
	}

	m := result.(map[string]any)
	if m["deleted"] != true {
		t.Error("expected deleted to be true")
	}

	// Verify deleted
	_, err = skill.handleGet(ctx, map[string]any{"id": id})
	if err == nil {
		t.Error("expected error getting deleted job")
	}
}

func TestSkill_EnableDisableTool(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	// Create a job
	createParams := map[string]any{
		"name":          "Test Job",
		"schedule_cron": "0 0 9 * * *",
		"action_type":   "send_message",
		"session_id":    "session-1",
		"message":       "Hello",
	}
	createResult, _ := skill.handleCreate(ctx, createParams)
	id := createResult.(map[string]any)["id"].(string)

	// Disable
	disableResult, err := skill.handleDisable(ctx, map[string]any{"id": id})
	if err != nil {
		t.Fatalf("handleDisable failed: %v", err)
	}
	if disableResult.(map[string]any)["status"] != "disabled" {
		t.Error("expected status 'disabled'")
	}

	// Enable
	enableResult, err := skill.handleEnable(ctx, map[string]any{"id": id})
	if err != nil {
		t.Fatalf("handleEnable failed: %v", err)
	}
	if enableResult.(map[string]any)["status"] != "enabled" {
		t.Error("expected status 'enabled'")
	}
}

func TestSkill_TriggerTool(t *testing.T) {
	skill := setupSkill(t)
	ctx := context.Background()

	// Create a job with send_message action (no agent configured, so trigger will fail)
	createParams := map[string]any{
		"name":          "Test Job",
		"schedule_cron": "0 0 9 * * *",
		"action_type":   "send_message",
		"session_id":    "session-1",
		"message":       "Hello",
	}
	createResult, _ := skill.handleCreate(ctx, createParams)
	id := createResult.(map[string]any)["id"].(string)

	// Trigger - should complete but with success=false since no agent configured
	result, err := skill.handleTrigger(ctx, map[string]any{"id": id})
	if err != nil {
		t.Fatalf("handleTrigger failed: %v", err)
	}

	m := result.(map[string]any)
	// Without an agent configured, send_message action should fail
	if m["success"] != false {
		t.Error("expected success to be false (no agent configured)")
	}
	// Error message should indicate the issue
	if m["error"] == "" {
		t.Error("expected error message")
	}
}

func TestSkill_GetScheduler(t *testing.T) {
	skill := setupSkill(t)

	scheduler := skill.GetScheduler()
	if scheduler == nil {
		t.Error("expected non-nil scheduler")
	}
}
