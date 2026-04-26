# Release Notes: v0.6.0

**Release Date:** 2026-04-25

## Overview

OmniAgent v0.6.0 introduces major infrastructure improvements including persistent sessions, context management, event hooks, scheduled jobs, and platform abstractions. Storage has been migrated to the unified `omnistorage-core` package.

## Highlights

- **Sessions** - Persistent conversation history with automatic loading/saving
- **Context Engine** - Token-aware context management with sliding windows
- **Event Hooks** - React to agent lifecycle events (session created, message processed, etc.)
- **Cron Scheduler** - Schedule periodic tasks with cron expressions, intervals, or one-time execution
- **Platform Abstraction** - Deploy agents to different environments (standalone, serverless)
- **Storage Migration** - Unified storage via `omnistorage-core/kvs`

## New Features

### Persistent Sessions

Maintain conversation history across agent restarts:

```go
agent, _ := omniagent.NewAgent(
    omniagent.WithSessionStore(sessionStore),
)

// Process with session history
response, err := agent.ProcessWithSession(ctx, "session-123", "Hello!")
```

Sessions automatically load history on start and save after each exchange.

### Context Management

Token-aware context compilation with sliding window support:

```go
import "github.com/plexusone/omniagent/context"

engine := context.NewEngine(context.Config{
    MaxTokens:    4096,
    WindowPolicy: context.SlidingWindow,
})

// Compile context within token budget
messages := engine.Compile(ctx, history, systemPrompt)
```

### Event Hooks

React to agent events for logging, analytics, or custom behavior:

```go
import "github.com/plexusone/omniagent/hooks"

registry := hooks.NewRegistry()
registry.On(hooks.EventSessionCreated, func(ctx context.Context, event hooks.Event) {
    log.Printf("New session: %s", event.SessionID)
})
registry.On(hooks.EventMessageProcessed, func(ctx context.Context, event hooks.Event) {
    log.Printf("Processed message in %v", event.Duration)
})

agent, _ := omniagent.NewAgent(
    omniagent.WithHooks(registry),
)
```

### Cron Scheduler

Schedule periodic agent tasks:

```go
import "github.com/plexusone/omniagent/cron"

scheduler := cron.NewScheduler()

// Cron expression
scheduler.Schedule("daily-report", "0 9 * * *", func(ctx context.Context) error {
    return agent.Process(ctx, "system", "Generate daily report")
})

// Interval-based
scheduler.Every("health-check", 5*time.Minute, healthCheckFunc)

// One-time
scheduler.Once("reminder", time.Now().Add(1*time.Hour), reminderFunc)

scheduler.Start()
```

### Platform Abstraction

Deploy agents to different environments:

```go
import "github.com/plexusone/omniagent/platform/standalone"

platform := standalone.New(standalone.Config{
    Addr: ":8080",
})

// Run agent on platform
platform.Run(ctx, agent)
```

## Breaking Changes

### Storage Migration

The local `storage/` package has been removed in favor of `omnistorage-core/kvs`:

```go
// Old (v0.5.0)
import "github.com/plexusone/omniagent/storage"
store := storage.NewSQLiteBackend("./data.db")

// New (v0.6.0)
import "github.com/plexusone/omnistorage-core/kvs"
store, _ := kvs.NewSQLite("./data.db")
```

## Dependency Changes

| Package | Change |
|---------|--------|
| `omnistorage-core` | Added v0.3.0 |
| `robfig/cron/v3` | Added for scheduling |
| `modernc.org/sqlite` | Now indirect (via omnistorage-core) |

## Upgrade Guide

### From v0.5.0

1. Update your dependency:

```bash
go get github.com/plexusone/omniagent@v0.6.0
go mod tidy
```

2. Migrate storage imports:

```go
// Replace
import "github.com/plexusone/omniagent/storage"

// With
import "github.com/plexusone/omnistorage-core/kvs"
```

3. (Optional) Add session support:

```go
agent, _ := omniagent.NewAgent(
    omniagent.WithSessionStore(store),
)

// Use ProcessWithSession for stateful conversations
response, _ := agent.ProcessWithSession(ctx, sessionID, message)
```

## Documentation

New guides added:

- [Sessions Guide](../../guides/sessions.md)
- [Context Management Guide](../../guides/context.md)
- [Event Hooks Guide](../../guides/hooks.md)
- [Cron Scheduler Guide](../../guides/cron.md)

## Planning Documents

Historical planning documents for this release:

- [Plan](PLAN.md) - Feature plan and architecture decisions
- [Tasks](TASKS.md) - Implementation tasks and completion status

## Full Changelog

See [CHANGELOG.md](https://github.com/plexusone/omniagent/blob/main/CHANGELOG.md) for the complete list of changes.
