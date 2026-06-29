# Memory Skill

OmniAgent includes a semantic memory skill powered by [omnimemory](https://github.com/plexusone/omnimemory) for storing and retrieving information. The memory skill enables agents to remember context across conversations and retrieve relevant information based on semantic similarity.

## Overview

The memory skill provides:

- **Semantic Search** - Find relevant memories based on meaning, not exact matches
- **Memory Types** - Observations, facts, preferences, summaries, traits, relationships
- **Memory Scopes** - User, agent, tenant, team, session, domain
- **Persistence** - Configurable backends for short-term and long-term storage
- **Metadata** - Attach key-value metadata to memories

## Short-Term vs Long-Term Memory

OmniAgent supports both short-term (active/working) memory and long-term (persistent) memory by configuring omnimemory with different backends:

| Memory Type | Backend | Scope | Persistence | Use Case |
|-------------|---------|-------|-------------|----------|
| **Short-term** | In-memory | Session | None (cleared on restart) | Working memory, scratchpad, current task context |
| **Long-term** | File/Database | User/Tenant | Persisted | User preferences, facts, relationships |

### Default Configuration

By default, the memory skill uses an **in-memory backend** suitable for short-term/active memory:

```go
memorySkill := memory.NewSkill(memory.Config{})
// Uses in-memory provider by default
```

### Dual Memory Setup

For both short-term and long-term memory, instantiate omnimemory twice with different backends:

```go
import (
    "github.com/plexusone/omnimemory/core"
    "github.com/plexusone/omniagent/skills/memory"
)

// Short-term memory (in-memory, session-scoped)
shortTermClient, _ := core.NewClient(core.ClientConfig{
    Providers: []core.ProviderConfig{
        {Name: core.ProviderNameMemory}, // In-memory provider
    },
})

shortTermMemory := memory.NewSkill(memory.Config{
    Client:   shortTermClient,
    TenantID: "session-123",
    AgentID:  "agent-1",
})

// Long-term memory (persistent)
longTermClient, _ := core.NewClient(core.ClientConfig{
    Providers: []core.ProviderConfig{
        {
            Name: core.ProviderNameFile,
            Config: map[string]any{
                "path": "/data/memories",
            },
        },
    },
})

longTermMemory := memory.NewSkill(memory.Config{
    Client:   longTermClient,
    TenantID: "user-456",
    AgentID:  "agent-1",
})

// Register both skills with different names
a, err := agent.New(config,
    agent.WithCompiledSkill(shortTermMemory), // memory_* tools
    // Optionally rename long-term tools to avoid conflicts
)
```

### Memory Scopes for Differentiation

Alternatively, use memory scopes to differentiate within a single omnimemory instance:

```go
// Store short-term note (session scope)
{
    "content": "User is asking about API pricing",
    "scope": "session",
    "type": "observation"
}

// Store long-term preference (user scope)
{
    "content": "User prefers dark mode",
    "scope": "user",
    "type": "preference"
}
```

Scope hierarchy:

| Scope | Lifetime | Visibility |
|-------|----------|------------|
| `session` | Current session only | Single conversation |
| `user` | Persistent | All user sessions |
| `agent` | Persistent | All sessions for this agent |
| `tenant` | Persistent | All users in tenant |
| `team` | Persistent | Team members |
| `domain` | Persistent | Domain-wide |

## Quick Start

### Enable Memory Skill

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/skills/memory"
)

// Default: in-memory backend (short-term/active memory)
memorySkill := memory.NewSkill(memory.Config{})

a, err := agent.New(config,
    agent.WithCompiledSkill(memorySkill),
)
```

### Usage in Conversation

Once enabled, the agent can use memory tools:

```
User: Remember that my favorite color is blue and I prefer dark mode.
Agent: [calls memory_store] I've stored that your favorite color is blue and you prefer dark mode.

User: What are my preferences?
Agent: [calls memory_search] Your favorite color is blue and you prefer dark mode.
```

## Memory Tools

The memory skill provides five tools powered by omnimemory:

### memory_store

Store information in semantic memory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `content` | string | Yes | The content to store |
| `type` | string | No | Memory type (default: "observation") |
| `scope` | string | No | Memory scope (default: "session") |
| `subject_id` | string | No | Who this memory is about |
| `metadata` | object | No | Key-value metadata pairs |

**Memory Types:**

- `observation` - Something noticed or observed
- `fact` - A verified piece of information
- `preference` - User preference or setting
- `summary` - A summary of content or conversation
- `trait` - A personality or behavioral trait
- `relationship` - A relationship between entities

**Example:**

```json
{
  "content": "User prefers dark mode and compact layouts",
  "type": "preference",
  "scope": "user",
  "metadata": {
    "category": "ui",
    "priority": "high"
  }
}
```

### memory_search

Search memories using semantic similarity.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query |
| `types` | array | No | Filter by memory types |
| `scopes` | array | No | Filter by memory scopes |
| `limit` | integer | No | Max results (default: 5) |

**Response:**

```json
{
  "results": [
    {
      "id": "mem_1234567890",
      "content": "User prefers dark mode",
      "type": "preference",
      "scope": "user",
      "score": 0.92,
      "metadata": {"category": "ui"}
    }
  ],
  "count": 1
}
```

### memory_recall

Recall relevant memories for the current context.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Query or context to recall memories for |
| `max_results` | integer | No | Maximum memories to recall (default: 5) |
| `types` | array | No | Filter by memory types |

**Response:**

```json
{
  "memories": [...],
  "count": 3,
  "summary": "User prefers dark mode, works on API projects..."
}
```

### memory_list

List memories with optional filters.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `types` | array | No | Filter by memory types |
| `scopes` | array | No | Filter by memory scopes |
| `limit` | integer | No | Max results (default: 20) |

### memory_delete

Delete a memory by ID.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | Yes | ID of memory to delete |

## Memory Types and Scopes

Memory types and scopes help organize and filter memories:

```
User: Remember that the API refactor is due next Friday.
Agent: [calls memory_store with type="fact", scope="session"]
       Stored that the API refactor is due next Friday.

User: What do you remember about my preferences?
Agent: [calls memory_search with types=["preference"]]
       You prefer dark mode and compact layouts.
```

### Type Use Cases

| Type | Use Case |
|------|----------|
| `observation` | Something noticed during conversation |
| `fact` | Verified information (dates, names, etc.) |
| `preference` | User preferences and settings |
| `summary` | Conversation or content summaries |
| `trait` | Personality or behavioral patterns |
| `relationship` | Connections between people/entities |

### Scope Use Cases

| Scope | Use Case |
|-------|----------|
| `session` | Current conversation only (short-term) |
| `user` | Persists across all user sessions |
| `agent` | Shared across all sessions for this agent |
| `tenant` | Organization-wide knowledge |

## Configuration

```go
type Config struct {
    // Client is an existing omnimemory client.
    // If nil, a new in-memory client will be created (default).
    Client *core.Client

    // TenantID is the default tenant for memory operations.
    TenantID string

    // AgentID is the agent identifier for memory attribution.
    AgentID string
}
```

### Persistent Backend

For long-term memory with persistence:

```go
import "github.com/plexusone/omnimemory/core"

// Create client with file backend
client, _ := core.NewClient(core.ClientConfig{
    Providers: []core.ProviderConfig{
        {
            Name: core.ProviderNameFile,
            Config: map[string]any{
                "path": "/data/memories",
            },
        },
    },
})

memorySkill := memory.NewSkill(memory.Config{
    Client:   client,
    TenantID: "my-tenant",
    AgentID:  "agent-1",
})
```

### Available Backends

| Backend | Provider Name | Use Case |
|---------|---------------|----------|
| In-Memory | `memory` | Short-term, testing, development |
| File | `file` | Simple persistence, single instance |
| SQLite | `sqlite` | Local persistence, embedded |
| PostgreSQL | `postgres` | Production, multi-instance |

## Best Practices

### Store Structured Information

Good:
```
Store: "User's birthday is March 15, 1990"
Type: fact
Scope: user
```

Bad:
```
Store: "their bday is next month lol"
```

### Use Appropriate Types and Scopes

```json
{
  "content": "API rate limit is 1000 requests per hour",
  "type": "fact",
  "scope": "tenant",
  "metadata": {
    "category": "technical",
    "source": "documentation"
  }
}
```

### Leverage Metadata

```json
{
  "content": "Meeting with client at 3pm",
  "type": "observation",
  "scope": "session",
  "metadata": {
    "date": "2026-06-15",
    "participants": "client"
  }
}
```

### Clean Up Session Memories

Session-scoped memories are cleared automatically. For persistent memories:

```
User: Delete the memory about last week's meeting
Agent: [calls memory_search, then memory_delete]
       Deleted the memory about last week's meeting.
```

## Integration with omnimemory

The memory skill is built on top of [omnimemory](https://github.com/plexusone/omnimemory), which provides:

- **Multi-backend support** - In-memory, file, SQLite, PostgreSQL
- **Semantic search** - Vector-based similarity search
- **Memory types** - Structured categorization
- **Scopes** - Hierarchical visibility control

For advanced use cases, access the underlying client:

```go
memorySkill := memory.NewSkill(config)
client := memorySkill.Client()

// Direct access to omnimemory client
resp, _ := client.Search(ctx, &core.SearchRequest{
    Context: core.Context{TenantID: "my-tenant"},
    Query:   "user preferences",
    Types:   []core.MemoryType{core.MemoryTypePreference},
    Limit:   10,
})
```

## Troubleshooting

### Low-Quality Search Results

- Check that the query is semantically related to stored content
- Try increasing the `limit` parameter
- Use `memory_recall` for context-aware retrieval

### Memories Not Persisting

- Verify you're using a persistent backend (file, SQLite, PostgreSQL)
- Check that the backend path/connection is writable
- Ensure `Init()` was called on the skill
- Session-scoped memories are intentionally not persisted

### Short-Term Memory Clearing

In-memory backend (default) clears on restart. This is intentional for short-term/active memory. For persistence, configure a file or database backend.
