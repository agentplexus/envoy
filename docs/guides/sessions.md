# Sessions & Storage

OmniAgent provides persistent session management for maintaining conversation history across interactions. Sessions are backed by `omnistorage-core/kvs` for reliable storage.

## Overview

Sessions enable:

- **Conversation Continuity** - The agent remembers previous messages
- **Persistent Storage** - Conversations survive restarts (with SQLite backend)
- **Session Management** - List, clear, and delete sessions programmatically
- **Metadata Storage** - Attach custom data to sessions

## Quick Start

### Using Sessions

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/sessions"
    "github.com/plexusone/omnistorage-core/kvs/backend/sqlite"
)

// Create storage backend
backend, _ := sqlite.New(sqlite.Config{
    Path: "omniagent.db",
})

// Create agent with sessions
a, _ := agent.New(config,
    agent.WithSessionsFromStorage(backend),
)
defer a.Close()

// Process with conversation history
response1, _ := a.ProcessWithSession(ctx, "user-123", "My name is Alice")
response2, _ := a.ProcessWithSession(ctx, "user-123", "What's my name?")
// Agent remembers: "Your name is Alice"
```

## Storage Backends

### Memory (Development)

In-memory storage for testing. Data is lost on restart.

```go
import "github.com/plexusone/omnistorage-core/kvs/backend/memory"

backend := memory.New()
```

### SQLite (Recommended)

Persistent storage using SQLite. Recommended for production.

```go
import "github.com/plexusone/omnistorage-core/kvs/backend/sqlite"

backend, err := sqlite.New(sqlite.Config{
    Path: "omniagent.db",
})
```

## Agent Options

### WithSessionsFromStorage

Creates a session store from a KVS backend. Also sets the storage for compiled skills.

```go
agent.New(config,
    agent.WithSessionsFromStorage(backend),
)
```

### WithSessionStore

Use an existing session store with custom configuration.

```go
sessionStore := sessions.NewStore(sessions.StoreConfig{
    Backend: backend,
    TTL:     24 * time.Hour,  // Custom TTL
})

agent.New(config,
    agent.WithSessionStore(sessionStore),
)
```

## Processing Methods

### ProcessWithSession

Processes a message with full conversation history.

```go
response, err := agent.ProcessWithSession(ctx, sessionID, "Hello!")
```

The method:

1. Loads the session (creates if not exists)
2. Adds the user message to history
3. Sends conversation history + new message to LLM
4. Adds assistant response to history
5. Saves the session

### Process (Stateless)

Processes a single message without history. Useful for one-off queries.

```go
response, err := agent.Process(ctx, sessionID, "What time is it?")
```

## Session Management

### List Sessions

```go
ids, err := agent.ListSessions(ctx)
for _, id := range ids {
    fmt.Println("Session:", id)
}
```

### Get Session Details

```go
session, err := agent.GetSession(ctx, "user-123")
if err != nil {
    // Session not found
}

fmt.Printf("Messages: %d\n", len(session.GetMessages()))
fmt.Printf("Created: %s\n", session.CreatedAt)
```

### Clear History

Keep the session but remove all messages:

```go
err := agent.ClearSession(ctx, "user-123")
```

### Delete Session

Remove the session entirely:

```go
err := agent.DeleteSession(ctx, "user-123")
```

## Session Tools (Compiled Skill)

OmniAgent includes a built-in sessions skill that exposes session management as LLM tools:

```go
import "github.com/plexusone/omniagent/sessions"

// Register the sessions skill
sessionsSkill := sessions.NewSkill()
agent.New(config,
    agent.WithSessionsFromStorage(backend),
    agent.WithCompiledSkill(sessionsSkill),
)
```

### Available Tools

| Tool | Description |
|------|-------------|
| `sessions_list` | List all active sessions |
| `sessions_history` | Get conversation history for a session |
| `sessions_get` | Get session details (metadata, timestamps) |
| `sessions_clear` | Clear conversation history |
| `sessions_delete` | Delete a session |
| `sessions_metadata_set` | Set session metadata |
| `sessions_metadata_get` | Get session metadata |

## Session Metadata

Attach custom data to sessions:

```go
session, _ := agent.GetSession(ctx, "user-123")

// Set metadata
session.SetMetadata("language", "en")
session.SetMetadata("preferences", map[string]any{
    "timezone": "UTC",
    "format":   "markdown",
})

// Save changes
store := agent.SessionStore()
store.Save(ctx, session)

// Get metadata
lang, ok := session.GetMetadata("language")
```

## Configuration

### Session TTL

Sessions expire after a configurable TTL (default: 7 days):

```go
sessionStore := sessions.NewStore(sessions.StoreConfig{
    Backend: backend,
    TTL:     30 * 24 * time.Hour,  // 30 days
})
```

### YAML Configuration (Coming Soon)

```yaml
storage:
  type: sqlite
  path: /data/omniagent.db

sessions:
  enabled: true
  ttl: 168h  # 7 days
```

## Best Practices

### Use Meaningful Session IDs

Use identifiers that map to your users:

```go
// Good: User-specific sessions
sessionID := fmt.Sprintf("user-%s", userID)

// Good: Channel-specific sessions
sessionID := fmt.Sprintf("whatsapp-%s", phoneNumber)

// Avoid: Random IDs that can't be traced
sessionID := uuid.New().String()
```

### Handle Long Conversations

For very long conversations, consider trimming history:

```go
session, _ := store.Get(ctx, sessionID)
if len(session.GetMessages()) > 100 {
    session.Trim(50)  // Keep last 50 messages
    store.Save(ctx, session)
}
```

### Separate Concerns

Use different session IDs for different contexts:

```go
// Main conversation
agent.ProcessWithSession(ctx, "user-123-chat", message)

// Task-specific session
agent.ProcessWithSession(ctx, "user-123-code-review", codeReviewRequest)
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                     Agent                           │
│                                                     │
│  ProcessWithSession(ctx, sessionID, content)        │
│         │                                           │
│         ▼                                           │
│  ┌─────────────┐    ┌─────────────────────────┐    │
│  │   sessions  │───▶│   omnistorage-core/kvs  │    │
│  │    Store    │    │                         │    │
│  └─────────────┘    │  ┌─────────┐ ┌───────┐  │    │
│                     │  │ memory  │ │sqlite │  │    │
│                     │  └─────────┘ └───────┘  │    │
│                     └─────────────────────────┘    │
└─────────────────────────────────────────────────────┘
```

## See Also

- [Compiled Skills](skills.md#compiled-skills) - Creating Go-based skills
- [Architecture Overview](../architecture/overview.md) - System architecture
- [Configuration Reference](../reference/configuration.md) - Full config options
