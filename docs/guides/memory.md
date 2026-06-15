# Memory Skill

OmniAgent includes a semantic memory skill for storing and retrieving information using vector search. The memory skill enables agents to remember context across conversations and retrieve relevant information based on semantic similarity.

## Overview

The memory skill provides:

- **Semantic Search** - Find relevant memories based on meaning, not exact matches
- **Collections** - Organize memories into named collections
- **Persistence** - Store memories across sessions using kvs.Store
- **Metadata** - Attach key-value metadata to memories

## Quick Start

### Enable Memory Skill

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/skills/memory"
)

memorySkill := memory.NewSkill(memory.Config{
    // Optional: provide a custom embedder
    // Embedder: myEmbedder,
})

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

The memory skill provides five tools:

### memory_store

Store information in semantic memory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `content` | string | Yes | The content to store |
| `key` | string | No | Unique key (auto-generated if omitted) |
| `collection` | string | No | Collection name (default: "default") |
| `metadata` | object | No | Key-value metadata pairs |

**Example:**

```json
{
  "content": "User prefers dark mode and compact layouts",
  "collection": "preferences",
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
| `collection` | string | No | Collection to search (default: "default") |
| `limit` | integer | No | Max results (default: 5) |

**Response:**

```json
{
  "results": [
    {
      "key": "mem_1234567890",
      "content": "User prefers dark mode",
      "score": 0.92,
      "metadata": {"category": "ui"}
    }
  ],
  "count": 1
}
```

### memory_list

List all memories in a collection.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `collection` | string | No | Collection name (default: "default") |
| `limit` | integer | No | Max results (default: 20) |

### memory_delete

Delete a memory by key.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `key` | string | Yes | Key of memory to delete |
| `collection` | string | No | Collection name (default: "default") |

### memory_collections

List all memory collections.

No parameters required.

**Response:**

```json
{
  "collections": ["default", "preferences", "history"],
  "count": 3
}
```

## Collections

Collections organize memories into logical groups:

```
User: Store in the "projects" collection: The API refactor is due next Friday.
Agent: [calls memory_store with collection="projects"]
       Stored in the projects collection.

User: What's in my projects collection?
Agent: [calls memory_list with collection="projects"]
       The API refactor is due next Friday.
```

### Collection Use Cases

| Collection | Purpose |
|------------|---------|
| `default` | General memories |
| `preferences` | User preferences and settings |
| `projects` | Project-related information |
| `contacts` | Contact details and relationships |
| `history` | Conversation summaries |

## Embeddings

The memory skill uses vector embeddings for semantic search. By default, it uses a hash-based embedder suitable for testing. For production, provide a real embedding model:

### Using omnillm Embeddings

```go
import (
    "github.com/plexusone/omnillm"
    "github.com/plexusone/omniagent/skills/memory"
)

// Create embedder using omnillm
client := omnillm.NewClient()
embedder := omnillm.NewEmbedder(client, "text-embedding-3-small")

memorySkill := memory.NewSkill(memory.Config{
    Embedder: embedder,
})
```

### Custom Embedder

Implement the `vector.Embedder` interface:

```go
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float64, error)
    Dimensions() int
}
```

## Persistence

Memory persists across sessions when storage is configured:

```go
import (
    "github.com/plexusone/omnistorage-core/kvs"
    "github.com/plexusone/omnistorage-core/sqlite"
)

// Create storage backend
backend, _ := sqlite.Open("omniagent.db")
store := kvs.New(backend)

a, err := agent.New(config,
    agent.WithStorage(store),
    agent.WithCompiledSkill(memorySkill), // Receives storage automatically
)
```

The memory skill implements `compiled.StorageAware`, so storage is injected automatically.

## Configuration

```go
type Config struct {
    // Embedder computes vector embeddings for semantic search.
    // If nil, uses a hash-based embedder (not for production).
    Embedder vector.Embedder
}
```

## Best Practices

### Store Structured Information

Good:
```
Store: "User's birthday is March 15, 1990"
```

Bad:
```
Store: "their bday is next month lol"
```

### Use Descriptive Keys

```json
{
  "content": "API rate limit is 1000 requests per hour",
  "key": "api_rate_limits",
  "collection": "technical"
}
```

### Leverage Metadata

```json
{
  "content": "Meeting with client at 3pm",
  "metadata": {
    "type": "meeting",
    "date": "2026-06-15",
    "participants": "client"
  }
}
```

### Clean Up Old Memories

Periodically delete outdated memories:

```
User: Delete the memory about last week's meeting
Agent: [calls memory_search, then memory_delete]
       Deleted the memory about last week's meeting.
```

## Integration with omniretrieve

The memory skill is built on top of [`omniretrieve/memory`](https://github.com/plexusone/omniretrieve), which provides:

- **Memory Manager** - Collection-based document storage
- **Vector Index** - Semantic similarity search
- **BM25 Index** - Keyword-based search (hybrid mode)

For advanced use cases, access the underlying manager:

```go
memorySkill := memory.NewSkill(config)
manager := memorySkill.Manager()

// Direct access to memory manager
docs, _ := manager.Search(ctx, "default", "preferences", memory.SearchOptions{
    TopK:            10,
    IncludeMetadata: true,
})
```

## Troubleshooting

### Low-Quality Search Results

- Ensure you're using a production embedder, not the hash embedder
- Check that the query is semantically related to stored content
- Try increasing the `limit` parameter

### Memories Not Persisting

- Verify storage is configured with `WithStorage` or `WithSessionsFromStorage`
- Check that the storage backend is writable
- Ensure `Init()` was called on the skill

### Collection Not Found

Collections are created on first use. If a collection appears empty:

```
Agent: [calls memory_list with collection="preferences"]
       No memories found in the preferences collection.
```

This is normal for new or empty collections.
