# Hooks System

OmniAgent provides an event-driven hooks system for extending agent behavior. Hooks allow you to react to events like messages, tool calls, and session changes without modifying core agent logic.

## Overview

The hooks system supports three registration methods:

- **Quick Handlers** - Simple function handlers for straightforward use cases
- **Compiled Hooks** - Reusable hook implementations with lifecycle management
- **Webhook Hooks** - HTTP-based handlers for external integrations

## Quick Start

### Quick Handler

The simplest way to handle events:

```go
import (
    "context"
    "log"

    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/hooks"
)

a, _ := agent.New(config,
    agent.WithHook(hooks.EventMessageReceived, func(ctx context.Context, e hooks.Event) error {
        msg := e.Data.(hooks.MessageEvent)
        log.Printf("User said: %s", msg.Content)
        return nil
    }),
)
```

### Named Handler

Add a name for better logging:

```go
agent.WithNamedHook(hooks.EventToolCalled, "audit-log", func(ctx context.Context, e hooks.Event) error {
    tool := e.Data.(hooks.ToolEvent)
    log.Printf("[AUDIT] Tool called: %s", tool.Name)
    return nil
})
```

## Event Types

| Event | Trigger | Data Type |
|-------|---------|-----------|
| `EventMessageReceived` | User message received | `MessageEvent` |
| `EventMessageSent` | Assistant response sent | `MessageEvent` |
| `EventToolCalled` | Before tool execution | `ToolEvent` |
| `EventToolCompleted` | After tool execution | `ToolEvent` |
| `EventSessionCreated` | New session created | `SessionEvent` |
| `EventSessionUpdated` | Session saved | `SessionEvent` |
| `EventJobExecuted` | Cron job executed | `JobEvent` |

### Event Data Structures

```go
// MessageEvent for message.received and message.sent
type MessageEvent struct {
    Role    string `json:"role"`    // "user" or "assistant"
    Content string `json:"content"`
}

// ToolEvent for tool.called and tool.completed
type ToolEvent struct {
    Name   string         `json:"name"`
    Params map[string]any `json:"params,omitempty"`
    Result string         `json:"result,omitempty"`  // Only on completed
    Error  string         `json:"error,omitempty"`   // Only on completed
}

// SessionEvent for session.created and session.updated
type SessionEvent struct {
    SessionID string `json:"session_id"`
    Action    string `json:"action"`  // "created" or "updated"
}

// JobEvent for job.executed
type JobEvent struct {
    JobID   string `json:"job_id"`
    JobName string `json:"job_name"`
    Success bool   `json:"success"`
}
```

## Compiled Hooks

For complex hooks that need initialization, cleanup, or state management, implement the `Hook` interface:

```go
type Hook interface {
    Name() string
    Events() []EventType
    Handle(ctx context.Context, event Event) error
    Init(ctx context.Context) error
    Close() error
}
```

### Example: Audit Hook

```go
type AuditHook struct {
    logger *slog.Logger
    file   *os.File
}

func NewAuditHook(path string) (*AuditHook, error) {
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return nil, err
    }
    return &AuditHook{
        logger: slog.New(slog.NewJSONHandler(f, nil)),
        file:   f,
    }, nil
}

func (h *AuditHook) Name() string {
    return "audit"
}

func (h *AuditHook) Events() []hooks.EventType {
    return []hooks.EventType{
        hooks.EventMessageReceived,
        hooks.EventMessageSent,
        hooks.EventToolCalled,
        hooks.EventToolCompleted,
    }
}

func (h *AuditHook) Handle(ctx context.Context, event hooks.Event) error {
    h.logger.Info("event",
        "type", event.Type,
        "timestamp", event.Timestamp,
        "session_id", event.SessionID,
        "data", event.Data,
    )
    return nil
}

func (h *AuditHook) Init(ctx context.Context) error {
    h.logger.Info("audit hook initialized")
    return nil
}

func (h *AuditHook) Close() error {
    return h.file.Close()
}

// Register with agent
auditHook, _ := NewAuditHook("/var/log/omniagent-audit.json")
agent.New(config, agent.WithCompiledHook(auditHook))
```

### Storage-Aware Hooks

Hooks can receive storage injection by implementing `StorageAware`:

```go
type MetricsHook struct {
    storage kvs.Store
}

func (h *MetricsHook) SetStorage(s kvs.Store) {
    h.storage = s
}

func (h *MetricsHook) Handle(ctx context.Context, event hooks.Event) error {
    // Use h.storage to persist metrics
    key := fmt.Sprintf("metrics:%s:%d", event.Type, time.Now().Unix())
    data, _ := json.Marshal(event)
    return h.storage.Set(ctx, key, data, 24*time.Hour)
}
```

## Webhook Hooks

Send events to external HTTP endpoints:

```go
agent.New(config,
    agent.WithWebhookHook(&hooks.WebhookHook{
        HookName:   "slack-notify",
        HookEvents: []hooks.EventType{hooks.EventMessageSent},
        URL:        "https://hooks.slack.com/services/xxx",
        Method:     "POST",
        Headers: map[string]string{
            "Authorization": "Bearer " + os.Getenv("SLACK_TOKEN"),
        },
        Timeout:    5 * time.Second,
        RetryCount: 2,
        RetryDelay: time.Second,
    }),
)
```

### Webhook Configuration

| Field | Description | Default |
|-------|-------------|---------|
| `HookName` | Identifier for logging | Required |
| `HookEvents` | Events to send | Required |
| `URL` | Endpoint URL | Required |
| `Method` | HTTP method | `POST` |
| `Headers` | Custom headers | None |
| `Timeout` | Request timeout | `10s` |
| `RetryCount` | Retries on failure | `0` |
| `RetryDelay` | Delay between retries | `1s` |

### Webhook Payload

Events are sent as JSON:

```json
{
    "type": "message.received",
    "timestamp": "2026-04-22T10:30:00Z",
    "session_id": "user-123",
    "data": {
        "role": "user",
        "content": "Hello, agent!"
    }
}
```

## Multiple Handlers

Register multiple handlers for the same event:

```go
agent.New(config,
    // Log all messages
    agent.WithNamedHook(hooks.EventMessageReceived, "logger", logHandler),

    // Track metrics
    agent.WithNamedHook(hooks.EventMessageReceived, "metrics", metricsHandler),

    // Notify external system
    agent.WithWebhookHook(&hooks.WebhookHook{
        HookName:   "notify",
        HookEvents: []hooks.EventType{hooks.EventMessageReceived},
        URL:        "https://api.example.com/events",
    }),
)
```

All handlers are called in registration order. Errors are logged but don't prevent other handlers from executing.

## Async vs Sync Dispatch

By default, events are dispatched asynchronously (fire-and-forget) to avoid blocking the main processing flow. The dispatcher:

1. Creates a background context for the goroutine
2. Copies the logger from the original context
3. Calls all handlers without blocking the caller

For synchronous dispatch (useful in tests), use the registry directly:

```go
registry := agent.HookRegistry()
err := registry.Dispatch(ctx, hooks.NewEvent(hooks.EventMessageReceived, data))
```

## Error Handling

Handler errors are logged but don't stop other handlers:

```go
agent.WithHook(hooks.EventToolCalled, func(ctx context.Context, e hooks.Event) error {
    // If this returns an error, it's logged but the next handler still runs
    return fmt.Errorf("handler failed")
})
```

The first error is returned from `Dispatch()` for synchronous calls, but all handlers still execute.

## Initialization

Compiled hooks are initialized when you call `InitHooks()`:

```go
a, _ := agent.New(config,
    agent.WithCompiledHook(myHook),
)

// Initialize all hooks
if err := a.InitHooks(ctx); err != nil {
    log.Fatal(err)
}
```

Quick handlers and webhook hooks don't require initialization.

## Use Cases

### Audit Logging

Log all interactions for compliance:

```go
agent.WithHook(hooks.EventMessageReceived, func(ctx context.Context, e hooks.Event) error {
    auditLog.Info("message", "direction", "in", "data", e.Data)
    return nil
}),
agent.WithHook(hooks.EventMessageSent, func(ctx context.Context, e hooks.Event) error {
    auditLog.Info("message", "direction", "out", "data", e.Data)
    return nil
}),
```

### Tool Usage Metrics

Track which tools are used:

```go
agent.WithHook(hooks.EventToolCompleted, func(ctx context.Context, e hooks.Event) error {
    tool := e.Data.(hooks.ToolEvent)
    metrics.IncrementCounter("tool_calls", map[string]string{
        "tool":    tool.Name,
        "success": strconv.FormatBool(tool.Error == ""),
    })
    return nil
}),
```

### Session Analytics

Track conversation patterns:

```go
agent.WithHook(hooks.EventSessionCreated, func(ctx context.Context, e hooks.Event) error {
    sess := e.Data.(hooks.SessionEvent)
    analytics.TrackEvent("session_started", map[string]string{
        "session_id": sess.SessionID,
    })
    return nil
}),
```

### External Notifications

Notify when important events occur:

```go
agent.WithWebhookHook(&hooks.WebhookHook{
    HookName:   "pagerduty",
    HookEvents: []hooks.EventType{hooks.EventToolCompleted},
    URL:        "https://events.pagerduty.com/v2/enqueue",
    Headers: map[string]string{
        "Authorization": "Token " + os.Getenv("PAGERDUTY_TOKEN"),
    },
}),
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                          Agent                               │
│                                                              │
│  processInternal()                                           │
│       │                                                      │
│       ├── dispatcher.EmitAsync(EventMessageReceived, ...)   │
│       │         │                                            │
│       │         ▼                                            │
│       │   ┌─────────────┐                                   │
│       │   │  Registry   │                                   │
│       │   │             │                                   │
│       │   │ ┌─────────┐ │                                   │
│       │   │ │Handlers │─┼──▶ Quick handlers                 │
│       │   │ └─────────┘ │                                   │
│       │   │             │                                   │
│       │   │ ┌─────────┐ │                                   │
│       │   │ │  Hooks  │─┼──▶ Compiled hooks                 │
│       │   │ └─────────┘ │                                   │
│       │   │             │                                   │
│       │   │ ┌─────────┐ │                                   │
│       │   │ │Webhooks │─┼──▶ HTTP endpoints                 │
│       │   │ └─────────┘ │                                   │
│       │   └─────────────┘                                   │
│       │                                                      │
│       ├── Execute tools...                                   │
│       │                                                      │
│       └── dispatcher.EmitAsync(EventMessageSent, ...)       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## See Also

- [Sessions](sessions.md) - Session management and storage
- [Cron](cron.md) - Scheduled job execution
- [Compiled Skills](skills.md#compiled-skills) - Creating Go-based skills
- [Observability](observability.md) - Building observability integrations
