package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omniskill/skill"
	"github.com/plexusone/omnistorage-core/kvs"
)

// Skill provides cron job management tools.
type Skill struct {
	store     *Store
	scheduler *Scheduler
	agent     AgentInterface // Agent for executing actions
}

// Ensure Skill implements the interfaces.
var (
	_ compiled.Skill        = (*Skill)(nil)
	_ compiled.StorageAware = (*Skill)(nil)
	_ compiled.AgentAware   = (*Skill)(nil)
)

// NewSkill creates a new cron skill.
func NewSkill() *Skill {
	return &Skill{}
}

// Name returns the skill identifier.
func (s *Skill) Name() string {
	return "cron"
}

// Description returns a human-readable description.
func (s *Skill) Description() string {
	return "Schedule and manage recurring jobs"
}

// SetStorage implements compiled.StorageAware.
func (s *Skill) SetStorage(storage kvs.Store) {
	s.store = NewStore(StoreConfig{Backend: storage})
}

// SetAgent implements compiled.AgentAware.
func (s *Skill) SetAgent(a interface{}) {
	// Type assert to AgentInterface
	if agent, ok := a.(AgentInterface); ok {
		s.agent = agent
	}
}

// Init initializes the skill.
func (s *Skill) Init(ctx context.Context) error {
	if s.store == nil {
		return fmt.Errorf("storage not configured")
	}

	// Create executor with agent
	executor := NewExecutor(ExecutorConfig{
		Agent: s.agent,
	})

	s.scheduler = NewScheduler(SchedulerConfig{
		Store:   s.store,
		Handler: executor.Execute, // Wire up the executor
	})

	return s.scheduler.Start(ctx)
}

// Close releases resources.
func (s *Skill) Close() error {
	if s.scheduler != nil {
		return s.scheduler.Stop(context.Background())
	}
	return nil
}

// SetExecutionHandler sets the handler for job execution.
// This should be called after NewSkill() and before Init().
func (s *Skill) SetExecutionHandler(handler ExecutionHandler) {
	if s.scheduler != nil {
		s.scheduler.handler = handler
	}
}

// GetScheduler returns the scheduler instance.
func (s *Skill) GetScheduler() *Scheduler {
	return s.scheduler
}

// Tools returns the tools provided by this skill.
func (s *Skill) Tools() []skill.Tool {
	return []skill.Tool{
		s.createTool(),
		s.listTool(),
		s.getTool(),
		s.deleteTool(),
		s.enableTool(),
		s.disableTool(),
		s.triggerTool(),
	}
}

func (s *Skill) createTool() skill.Tool {
	return skill.NewTool(
		"cron_create",
		"Create a new scheduled job. Use cron expressions (e.g., '0 0 9 * * *' for 9am daily), interval (e.g., '1h' for hourly), or a specific time for one-time execution.",
		map[string]skill.Parameter{
			"name": {
				Type:        "string",
				Description: "Human-readable name for the job",
				Required:    true,
			},
			"description": {
				Type:        "string",
				Description: "Optional description of what the job does",
			},
			"schedule_cron": {
				Type:        "string",
				Description: "Cron expression with seconds (e.g., '0 0 9 * * *' for 9am daily, '0 */5 * * * *' for every 5 minutes)",
			},
			"schedule_interval": {
				Type:        "string",
				Description: "Interval duration (e.g., '30m', '1h', '24h')",
			},
			"schedule_once": {
				Type:        "string",
				Description: "One-time execution timestamp in RFC3339 format",
			},
			"action_type": {
				Type:        "string",
				Description: "Type of action to perform",
				Required:    true,
				Enum:        []any{"send_message", "call_webhook", "call_tool"},
			},
			"session_id": {
				Type:        "string",
				Description: "Target session ID (required for send_message action)",
			},
			"message": {
				Type:        "string",
				Description: "Message content (required for send_message action)",
			},
			"webhook_url": {
				Type:        "string",
				Description: "Webhook URL (required for call_webhook action)",
			},
			"webhook_method": {
				Type:        "string",
				Description: "HTTP method for webhook (default: POST)",
			},
			"tool_name": {
				Type:        "string",
				Description: "Tool name (required for call_tool action)",
			},
			"tool_params": {
				Type:        "object",
				Description: "Tool parameters (for call_tool action)",
			},
		},
		s.handleCreate,
	)
}

func (s *Skill) handleCreate(ctx context.Context, params map[string]any) (any, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	description, _ := params["description"].(string)

	// Parse schedule
	var schedule Schedule
	if cronExpr, ok := params["schedule_cron"].(string); ok && cronExpr != "" {
		schedule.Cron = cronExpr
	}
	if intervalStr, ok := params["schedule_interval"].(string); ok && intervalStr != "" {
		dur, err := time.ParseDuration(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid interval: %w", err)
		}
		schedule.Interval = Duration(dur)
	}
	if onceStr, ok := params["schedule_once"].(string); ok && onceStr != "" {
		t, err := time.Parse(time.RFC3339, onceStr)
		if err != nil {
			return nil, fmt.Errorf("invalid once time: %w", err)
		}
		schedule.Once = &t
	}

	// Parse action
	actionType, _ := params["action_type"].(string)
	if actionType == "" {
		return nil, fmt.Errorf("action_type is required")
	}

	action := Action{
		Type: ActionType(actionType),
	}

	switch action.Type {
	case ActionTypeSendMessage:
		action.SessionID, _ = params["session_id"].(string)
		action.Message, _ = params["message"].(string)
	case ActionTypeCallWebhook:
		action.WebhookURL, _ = params["webhook_url"].(string)
		action.WebhookMethod, _ = params["webhook_method"].(string)
		if action.WebhookMethod == "" {
			action.WebhookMethod = "POST"
		}
	case ActionTypeCallTool:
		action.ToolName, _ = params["tool_name"].(string)
		if toolParams, ok := params["tool_params"].(map[string]any); ok {
			action.ToolParams = toolParams
		}
	}

	job := NewJob(uuid.New().String(), name, schedule, action)
	job.Description = description

	if err := s.scheduler.AddJob(ctx, job); err != nil {
		return nil, fmt.Errorf("add job: %w", err)
	}

	return map[string]any{
		"id":          job.ID,
		"name":        job.Name,
		"status":      job.Status,
		"next_run_at": job.NextRunAt,
	}, nil
}

func (s *Skill) listTool() skill.Tool {
	return skill.NewTool(
		"cron_list",
		"List all scheduled jobs, optionally filtered by status",
		map[string]skill.Parameter{
			"status": {
				Type:        "string",
				Description: "Filter by status",
				Enum:        []any{"enabled", "disabled", "running"},
			},
		},
		s.handleList,
	)
}

func (s *Skill) handleList(ctx context.Context, params map[string]any) (any, error) {
	var jobs []*Job
	var err error

	if statusStr, ok := params["status"].(string); ok && statusStr != "" {
		jobs, err = s.store.ListByStatus(ctx, JobStatus(statusStr))
	} else {
		jobs, err = s.scheduler.ListJobs(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	result := make([]map[string]any, len(jobs))
	for i, job := range jobs {
		result[i] = map[string]any{
			"id":          job.ID,
			"name":        job.Name,
			"description": job.Description,
			"status":      job.Status,
			"run_count":   job.RunCount,
			"last_run_at": job.LastRunAt,
			"next_run_at": job.NextRunAt,
		}
	}

	return result, nil
}

func (s *Skill) getTool() skill.Tool {
	return skill.NewTool(
		"cron_get",
		"Get detailed information about a specific job",
		map[string]skill.Parameter{
			"id": {
				Type:        "string",
				Description: "Job ID",
				Required:    true,
			},
		},
		s.handleGet,
	)
}

func (s *Skill) handleGet(ctx context.Context, params map[string]any) (any, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	job, err := s.scheduler.GetJob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}

	return job, nil
}

func (s *Skill) deleteTool() skill.Tool {
	return skill.NewTool(
		"cron_delete",
		"Delete a scheduled job",
		map[string]skill.Parameter{
			"id": {
				Type:        "string",
				Description: "Job ID",
				Required:    true,
			},
		},
		s.handleDelete,
	)
}

func (s *Skill) handleDelete(ctx context.Context, params map[string]any) (any, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	if err := s.scheduler.RemoveJob(ctx, id); err != nil {
		return nil, fmt.Errorf("delete job: %w", err)
	}

	return map[string]any{
		"deleted": true,
		"id":      id,
	}, nil
}

func (s *Skill) enableTool() skill.Tool {
	return skill.NewTool(
		"cron_enable",
		"Enable a disabled job to resume scheduling",
		map[string]skill.Parameter{
			"id": {
				Type:        "string",
				Description: "Job ID",
				Required:    true,
			},
		},
		s.handleEnable,
	)
}

func (s *Skill) handleEnable(ctx context.Context, params map[string]any) (any, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	if err := s.scheduler.EnableJob(ctx, id); err != nil {
		return nil, fmt.Errorf("enable job: %w", err)
	}

	job, _ := s.scheduler.GetJob(ctx, id)

	return map[string]any{
		"id":          id,
		"status":      "enabled",
		"next_run_at": job.NextRunAt,
	}, nil
}

func (s *Skill) disableTool() skill.Tool {
	return skill.NewTool(
		"cron_disable",
		"Disable a job without deleting it",
		map[string]skill.Parameter{
			"id": {
				Type:        "string",
				Description: "Job ID",
				Required:    true,
			},
		},
		s.handleDisable,
	)
}

func (s *Skill) handleDisable(ctx context.Context, params map[string]any) (any, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	if err := s.scheduler.DisableJob(ctx, id); err != nil {
		return nil, fmt.Errorf("disable job: %w", err)
	}

	return map[string]any{
		"id":     id,
		"status": "disabled",
	}, nil
}

func (s *Skill) triggerTool() skill.Tool {
	return skill.NewTool(
		"cron_trigger",
		"Run a job immediately, regardless of its schedule",
		map[string]skill.Parameter{
			"id": {
				Type:        "string",
				Description: "Job ID",
				Required:    true,
			},
		},
		s.handleTrigger,
	)
}

func (s *Skill) handleTrigger(ctx context.Context, params map[string]any) (any, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	result, err := s.scheduler.TriggerJob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("trigger job: %w", err)
	}

	return map[string]any{
		"id":       id,
		"success":  result.Success,
		"output":   result.Output,
		"error":    result.Error,
		"duration": result.Duration.String(),
	}, nil
}
