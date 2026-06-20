package openai

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/api/openai"
	"github.com/plexusone/omniagent/cron"
)

// cronHandler provides shared cron job operations for adapters.
// It takes a function that returns the cron scheduler, allowing different
// adapters to provide their own way of obtaining the scheduler.
type cronHandler struct {
	getScheduler func() *cron.Scheduler
}

// newCronHandler creates a new cron handler with the given scheduler getter.
func newCronHandler(getScheduler func() *cron.Scheduler) cronHandler {
	return cronHandler{getScheduler: getScheduler}
}

// ListCronJobs returns all cron jobs.
func (h *cronHandler) ListCronJobs(ctx context.Context) ([]openai.CronJobInfo, error) {
	scheduler := h.getScheduler()
	if scheduler == nil {
		return nil, fmt.Errorf("cron scheduler not configured")
	}

	jobs, err := scheduler.ListJobs(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]openai.CronJobInfo, len(jobs))
	for i, job := range jobs {
		result[i] = cronJobToInfo(job)
	}

	return result, nil
}

// GetCronJob returns a cron job by ID.
func (h *cronHandler) GetCronJob(ctx context.Context, id string) (*openai.CronJobInfo, error) {
	scheduler := h.getScheduler()
	if scheduler == nil {
		return nil, fmt.Errorf("cron scheduler not configured")
	}

	job, err := scheduler.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}

	info := cronJobToInfo(job)
	return &info, nil
}

// CreateCronJob creates a new cron job.
func (h *cronHandler) CreateCronJob(ctx context.Context, req *openai.CreateCronJobRequest) (*openai.CronJobInfo, error) {
	scheduler := h.getScheduler()
	if scheduler == nil {
		return nil, fmt.Errorf("cron scheduler not configured")
	}

	schedule, err := parseScheduleInfo(req.Schedule)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule: %w", err)
	}

	action := parseActionInfo(req.Action)
	job := cron.NewJob(uuid.New().String(), req.Name, schedule, action)
	job.Description = req.Description

	if err := scheduler.AddJob(ctx, job); err != nil {
		return nil, err
	}

	info := cronJobToInfo(job)
	return &info, nil
}

// UpdateCronJob updates an existing cron job.
func (h *cronHandler) UpdateCronJob(ctx context.Context, id string, req *openai.UpdateCronJobRequest) (*openai.CronJobInfo, error) {
	scheduler := h.getScheduler()
	if scheduler == nil {
		return nil, fmt.Errorf("cron scheduler not configured")
	}

	job, err := scheduler.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		job.Name = *req.Name
	}
	if req.Description != nil {
		job.Description = *req.Description
	}
	if req.Schedule != nil {
		schedule, err := parseScheduleInfo(*req.Schedule)
		if err != nil {
			return nil, fmt.Errorf("invalid schedule: %w", err)
		}
		job.Schedule = schedule
	}
	if req.Action != nil {
		job.Action = parseActionInfo(*req.Action)
	}

	if err := scheduler.UpdateJob(ctx, job); err != nil {
		return nil, err
	}

	info := cronJobToInfo(job)
	return &info, nil
}

// DeleteCronJob deletes a cron job.
func (h *cronHandler) DeleteCronJob(ctx context.Context, id string) error {
	scheduler := h.getScheduler()
	if scheduler == nil {
		return fmt.Errorf("cron scheduler not configured")
	}

	return scheduler.RemoveJob(ctx, id)
}

// TriggerCronJob triggers a cron job to run immediately.
func (h *cronHandler) TriggerCronJob(ctx context.Context, id string) (*openai.CronJobResult, error) {
	scheduler := h.getScheduler()
	if scheduler == nil {
		return nil, fmt.Errorf("cron scheduler not configured")
	}

	result, err := scheduler.TriggerJob(ctx, id)
	if err != nil {
		return nil, err
	}

	return &openai.CronJobResult{
		Success:   result.Success,
		Output:    result.Output,
		Error:     result.Error,
		Duration:  result.Duration.String(),
		StartedAt: result.StartedAt.Format(time.RFC3339),
	}, nil
}

// EnableCronJob enables a cron job.
func (h *cronHandler) EnableCronJob(ctx context.Context, id string) error {
	scheduler := h.getScheduler()
	if scheduler == nil {
		return fmt.Errorf("cron scheduler not configured")
	}

	return scheduler.EnableJob(ctx, id)
}

// DisableCronJob disables a cron job.
func (h *cronHandler) DisableCronJob(ctx context.Context, id string) error {
	scheduler := h.getScheduler()
	if scheduler == nil {
		return fmt.Errorf("cron scheduler not configured")
	}

	return scheduler.DisableJob(ctx, id)
}

// cronJobToInfo converts a cron.Job to openai.CronJobInfo.
func cronJobToInfo(job *cron.Job) openai.CronJobInfo {
	info := openai.CronJobInfo{
		ID:          job.ID,
		Name:        job.Name,
		Description: job.Description,
		Status:      string(job.Status),
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		LastRunAt:   job.LastRunAt,
		NextRunAt:   job.NextRunAt,
		RunCount:    job.RunCount,
		LastError:   job.LastError,
	}

	// Convert schedule
	info.Schedule = openai.CronScheduleInfo{
		Cron: job.Schedule.Cron,
	}
	if job.Schedule.Once != nil {
		info.Schedule.Once = job.Schedule.Once.Format(time.RFC3339)
	}
	if job.Schedule.Interval > 0 {
		info.Schedule.Interval = time.Duration(job.Schedule.Interval).String()
	}

	// Convert action
	info.Action = openai.CronActionInfo{
		Type:           string(job.Action.Type),
		SessionID:      job.Action.SessionID,
		Message:        job.Action.Message,
		WebhookURL:     job.Action.WebhookURL,
		WebhookMethod:  job.Action.WebhookMethod,
		WebhookHeaders: job.Action.WebhookHeaders,
		WebhookBody:    job.Action.WebhookBody,
		ToolName:       job.Action.ToolName,
		ToolParams:     job.Action.ToolParams,
	}

	return info
}

// parseScheduleInfo converts openai.CronScheduleInfo to cron.Schedule.
func parseScheduleInfo(info openai.CronScheduleInfo) (cron.Schedule, error) {
	var schedule cron.Schedule

	if info.Cron != "" {
		schedule.Cron = info.Cron
	}

	if info.Once != "" {
		t, err := time.Parse(time.RFC3339, info.Once)
		if err != nil {
			return schedule, fmt.Errorf("invalid once time: %w", err)
		}
		schedule.Once = &t
	}

	if info.Interval != "" {
		dur, err := time.ParseDuration(info.Interval)
		if err != nil {
			return schedule, fmt.Errorf("invalid interval: %w", err)
		}
		schedule.Interval = cron.Duration(dur)
	}

	return schedule, nil
}

// parseActionInfo converts openai.CronActionInfo to cron.Action.
func parseActionInfo(info openai.CronActionInfo) cron.Action {
	return cron.Action{
		Type:           cron.ActionType(info.Type),
		SessionID:      info.SessionID,
		Message:        info.Message,
		WebhookURL:     info.WebhookURL,
		WebhookMethod:  info.WebhookMethod,
		WebhookHeaders: info.WebhookHeaders,
		WebhookBody:    info.WebhookBody,
		ToolName:       info.ToolName,
		ToolParams:     info.ToolParams,
	}
}
