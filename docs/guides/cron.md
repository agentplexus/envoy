# Scheduled Jobs (Cron)

OmniAgent supports scheduled job execution through the `cron` package. Jobs can be scheduled using cron expressions, intervals, or one-time execution, and can perform actions like sending messages, calling webhooks, or invoking tools.

## Quick Start

Enable the cron scheduler when creating your agent:

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omnistorage-core/kvs/backend/sqlite"
)

// Create storage backend (required for job persistence)
backend, _ := sqlite.New(sqlite.Config{Path: "omniagent.db"})

// Create agent with cron support
a, _ := agent.New(config,
    agent.WithSessionsFromStorage(backend),
    agent.WithCronScheduler(),
)
```

Once enabled, the LLM can create and manage scheduled jobs through tool calls.

## Tools

The cron skill provides seven tools for job management:

| Tool | Description |
|------|-------------|
| `cron_create` | Create a new scheduled job |
| `cron_list` | List all jobs (optionally filter by status) |
| `cron_get` | Get detailed information about a job |
| `cron_delete` | Permanently delete a job |
| `cron_enable` | Enable a disabled job |
| `cron_disable` | Disable a job without deleting |
| `cron_trigger` | Run a job immediately |

## Schedule Types

### Cron Expressions

Standard cron expressions with seconds support (6 fields):

```
┌──────────── second (0-59)
│ ┌────────── minute (0-59)
│ │ ┌──────── hour (0-23)
│ │ │ ┌────── day of month (1-31)
│ │ │ │ ┌──── month (1-12)
│ │ │ │ │ ┌── day of week (0-6, Sun=0)
│ │ │ │ │ │
* * * * * *
```

Examples:

| Expression | Description |
|------------|-------------|
| `0 0 9 * * *` | Every day at 9:00 AM |
| `0 */5 * * * *` | Every 5 minutes |
| `0 0 0 * * 1` | Every Monday at midnight |
| `0 30 8 1 * *` | 8:30 AM on the 1st of each month |

### Intervals

Duration-based scheduling using Go duration strings:

| Interval | Description |
|----------|-------------|
| `5m` | Every 5 minutes |
| `1h` | Every hour |
| `24h` | Every day |
| `168h` | Every week |

### One-Time

A specific timestamp in RFC3339 format for single execution:

```
2026-04-25T14:30:00Z
```

After execution, one-time jobs are automatically disabled.

## Action Types

### send_message

Send a message to a session via the agent:

```json
{
  "action_type": "send_message",
  "session_id": "user-123",
  "message": "Don't forget to check your calendar!"
}
```

### call_webhook

Make an HTTP request to a URL:

```json
{
  "action_type": "call_webhook",
  "webhook_url": "https://api.example.com/notify",
  "webhook_method": "POST"
}
```

### call_tool

Invoke a registered tool:

```json
{
  "action_type": "call_tool",
  "tool_name": "weather_check",
  "tool_params": {"location": "San Francisco"}
}
```

## Job States

Jobs can be in one of three states:

| Status | Description |
|--------|-------------|
| `enabled` | Active and will run according to schedule |
| `disabled` | Inactive, will not run until enabled |
| `running` | Currently executing |

## Examples

### Daily Reminder

```
User: Schedule a daily reminder at 9am to check my calendar

LLM calls: cron_create(
  name="Calendar Reminder",
  schedule_cron="0 0 9 * * *",
  action_type="send_message",
  session_id="user-123",
  message="Good morning! Here's your calendar check reminder."
)
```

### Hourly Health Check

```
User: Set up an hourly health check for our API

LLM calls: cron_create(
  name="API Health Check",
  schedule_interval="1h",
  action_type="call_webhook",
  webhook_url="https://api.example.com/health"
)
```

### One-Time Notification

```
User: Remind me about the meeting at 2:30pm today

LLM calls: cron_create(
  name="Meeting Reminder",
  schedule_once="2026-04-22T14:30:00Z",
  action_type="send_message",
  session_id="user-123",
  message="Your meeting starts in 5 minutes!"
)
```

### List and Manage Jobs

```
User: Show me all my scheduled jobs

LLM calls: cron_list()

User: Pause the calendar reminder

LLM calls: cron_disable(id="abc123")

User: Run the health check now

LLM calls: cron_trigger(id="def456")
```

## Programmatic Usage

You can also use the cron package directly:

```go
import (
    "context"
    "time"

    "github.com/plexusone/omniagent/cron"
    "github.com/plexusone/omnistorage-core/kvs/backend/sqlite"
)

// Create store
backend, _ := sqlite.New(sqlite.Config{Path: "jobs.db"})
store := cron.NewStore(cron.StoreConfig{Backend: backend})

// Create scheduler with custom handler
scheduler := cron.NewScheduler(cron.SchedulerConfig{
    Store: store,
    Handler: func(ctx context.Context, job *cron.Job) cron.ExecutionResult {
        // Handle job execution
        return cron.ExecutionResult{
            Success:    true,
            StartedAt:  time.Now(),
            FinishedAt: time.Now(),
        }
    },
})

// Start scheduler
ctx := context.Background()
scheduler.Start(ctx)
defer scheduler.Stop(ctx)

// Create a job
job := cron.NewJob(
    "job-1",
    "Daily Report",
    cron.Schedule{Cron: "0 0 9 * * *"},
    cron.Action{
        Type:      cron.ActionTypeSendMessage,
        SessionID: "user-123",
        Message:   "Daily report is ready",
    },
)
scheduler.AddJob(ctx, job)
```

## Job Persistence

Jobs are persisted using the `kvs.Store` backend, which means:

- Jobs survive agent restarts
- Jobs are automatically loaded when the scheduler starts
- Job state (run count, last run time, etc.) is maintained

The storage key prefix is `cron:job:`.

## Execution Handler

For `send_message` actions to work, you need to provide an execution handler that can access the agent. The skill automatically handles this when registered via `WithCronScheduler()`.

For custom handlers:

```go
skill := cron.NewSkill()
skill.SetStorage(backend)
skill.SetExecutionHandler(func(ctx context.Context, job *cron.Job) cron.ExecutionResult {
    switch job.Action.Type {
    case cron.ActionTypeSendMessage:
        // Send message via agent
        _, err := agent.ProcessWithSession(ctx, job.Action.SessionID, job.Action.Message)
        if err != nil {
            return cron.ExecutionResult{Success: false, Error: err.Error()}
        }
        return cron.ExecutionResult{Success: true}
    // Handle other action types...
    }
})
```

## Best Practices

1. **Use descriptive names**: Job names appear in listings and logs
2. **Choose appropriate schedules**: Use cron for precise timing, intervals for regular polling
3. **Handle failures gracefully**: Jobs stay enabled after failures to allow retry
4. **Monitor job execution**: Check `last_run_at`, `run_count`, and `last_error` fields
5. **Clean up unused jobs**: Delete jobs that are no longer needed

## Limitations

- Maximum one schedule type per job (cron, interval, or once)
- Cron expressions require 6 fields (including seconds)
- One-time jobs are disabled after execution, not deleted
- Webhook actions currently support GET/POST methods
