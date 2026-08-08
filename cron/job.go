// Package cron provides scheduled job execution for omniagent.
//
// It supports cron expressions, one-time execution, and interval-based
// scheduling with actions that can send messages to sessions, call
// webhooks, or invoke registered tools.
package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// JobStatus represents the current state of a job.
type JobStatus string

const (
	// JobStatusEnabled indicates the job is active and will run according to schedule.
	JobStatusEnabled JobStatus = "enabled"

	// JobStatusDisabled indicates the job is inactive and will not run.
	JobStatusDisabled JobStatus = "disabled"

	// JobStatusRunning indicates the job is currently executing.
	JobStatusRunning JobStatus = "running"
)

// ActionType represents the type of action a job performs.
type ActionType string

const (
	// ActionTypeSendMessage sends a message to a session via the agent.
	ActionTypeSendMessage ActionType = "send_message"

	// ActionTypeCallWebhook makes an HTTP request to a webhook URL.
	ActionTypeCallWebhook ActionType = "call_webhook"

	// ActionTypeCallTool invokes a registered tool.
	ActionTypeCallTool ActionType = "call_tool"
)

// Job represents a scheduled job.
type Job struct {
	// ID is the unique identifier for the job.
	ID string `json:"id"`

	// Name is a human-readable name for the job.
	Name string `json:"name"`

	// Description provides additional context about the job.
	Description string `json:"description,omitempty"`

	// Schedule defines when the job runs.
	Schedule Schedule `json:"schedule"`

	// Action defines what the job does when it runs.
	Action Action `json:"action"`

	// OwnerPrincipal identifies the account/session under whose authority
	// the job executes. It is stamped by host code at creation time — never
	// from caller-supplied tool parameters, so it cannot be spoofed through
	// the cron tool surface. When set, the executor verifies the principal
	// is still configured before running agent/tool actions and denies all
	// tools (fail closed) when it is not. Empty on jobs created before
	// authority tracking existed; those execute unchecked (legacy behavior).
	OwnerPrincipal string `json:"owner_principal,omitempty"`

	// Status is the current state of the job.
	Status JobStatus `json:"status"`

	// CreatedAt is when the job was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the job was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// LastRunAt is when the job last ran (nil if never run).
	LastRunAt *time.Time `json:"last_run_at,omitempty"`

	// NextRunAt is when the job is scheduled to run next (nil if not scheduled).
	NextRunAt *time.Time `json:"next_run_at,omitempty"`

	// RunCount is the total number of times the job has run.
	RunCount int64 `json:"run_count"`

	// LastError is the error from the last run (empty if successful).
	LastError string `json:"last_error,omitempty"`

	// Metadata contains arbitrary key-value pairs for extensibility.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Schedule defines when a job runs.
// Exactly one of Cron, Once, or Interval should be set.
type Schedule struct {
	// Cron is a cron expression (e.g., "0 9 * * *" for 9am daily).
	Cron string `json:"cron,omitempty"`

	// Once is a specific time for one-time execution.
	Once *time.Time `json:"once,omitempty"`

	// Interval is the duration between runs (e.g., 1h for hourly).
	Interval Duration `json:"interval,omitempty"`
}

// Duration wraps time.Duration for JSON marshaling.
type Duration time.Duration

// MarshalJSON encodes the duration as a string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON decodes a duration string.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
	case string:
		if value == "" {
			*d = 0
			return nil
		}
		dur, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(dur)
	default:
		return fmt.Errorf("invalid duration: %v", v)
	}
	return nil
}

// Action defines what a job does when it runs.
type Action struct {
	// Type is the action type.
	Type ActionType `json:"type"`

	// SessionID is the target session for send_message actions.
	SessionID string `json:"session_id,omitempty"`

	// Message is the content to send for send_message actions.
	Message string `json:"message,omitempty"`

	// WebhookURL is the target URL for call_webhook actions.
	WebhookURL string `json:"webhook_url,omitempty"`

	// WebhookMethod is the HTTP method (default: POST).
	WebhookMethod string `json:"webhook_method,omitempty"`

	// WebhookHeaders are additional headers for webhook requests.
	WebhookHeaders map[string]string `json:"webhook_headers,omitempty"`

	// WebhookBody is the request body for webhook requests.
	WebhookBody string `json:"webhook_body,omitempty"`

	// ToolName is the tool to invoke for call_tool actions.
	ToolName string `json:"tool_name,omitempty"`

	// ToolParams are the parameters to pass to the tool.
	ToolParams map[string]any `json:"tool_params,omitempty"`
}

// NewJob creates a new job with the given parameters.
func NewJob(id, name string, schedule Schedule, action Action) *Job {
	now := time.Now()
	return &Job{
		ID:        id,
		Name:      name,
		Schedule:  schedule,
		Action:    action,
		Status:    JobStatusEnabled,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]any),
	}
}

// Validate checks if the job is valid.
func (j *Job) Validate() error {
	if j.ID == "" {
		return fmt.Errorf("job ID is required")
	}
	if j.Name == "" {
		return fmt.Errorf("job name is required")
	}

	// Validate schedule - exactly one type should be set
	scheduleCount := 0
	if j.Schedule.Cron != "" {
		scheduleCount++
	}
	if j.Schedule.Once != nil {
		scheduleCount++
	}
	if j.Schedule.Interval > 0 {
		scheduleCount++
	}
	if scheduleCount == 0 {
		return fmt.Errorf("schedule is required (cron, once, or interval)")
	}
	if scheduleCount > 1 {
		return fmt.Errorf("only one schedule type allowed (cron, once, or interval)")
	}

	// Validate action
	if err := j.Action.Validate(); err != nil {
		return fmt.Errorf("invalid action: %w", err)
	}

	return nil
}

// Validate checks if the action is valid.
func (a *Action) Validate() error {
	switch a.Type {
	case ActionTypeSendMessage:
		if a.SessionID == "" {
			return fmt.Errorf("session_id is required for send_message action")
		}
		if a.Message == "" {
			return fmt.Errorf("message is required for send_message action")
		}
	case ActionTypeCallWebhook:
		if a.WebhookURL == "" {
			return fmt.Errorf("webhook_url is required for call_webhook action")
		}
	case ActionTypeCallTool:
		if a.ToolName == "" {
			return fmt.Errorf("tool_name is required for call_tool action")
		}
	default:
		return fmt.Errorf("invalid action type: %s", a.Type)
	}
	return nil
}

// IsRecurring returns true if the job runs more than once.
func (j *Job) IsRecurring() bool {
	return j.Schedule.Cron != "" || j.Schedule.Interval > 0
}

// ExecutionResult represents the outcome of a job execution.
type ExecutionResult struct {
	// Success indicates if the execution completed successfully.
	Success bool `json:"success"`

	// Output is the result from the action.
	Output any `json:"output,omitempty"`

	// Error is the error message if execution failed.
	Error string `json:"error,omitempty"`

	// Duration is how long the execution took.
	Duration time.Duration `json:"duration"`

	// StartedAt is when the execution started.
	StartedAt time.Time `json:"started_at"`

	// FinishedAt is when the execution finished.
	FinishedAt time.Time `json:"finished_at"`
}

// ExecutionHandler is a callback function invoked when a job executes.
type ExecutionHandler func(ctx context.Context, job *Job) ExecutionResult
