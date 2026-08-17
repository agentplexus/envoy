# Context Management

The context engine manages conversation context to keep requests within LLM token limits while preserving important conversation history.

## Overview

Long conversations can exceed LLM context windows, causing errors or degraded responses. The context engine provides:

- **Message Windowing** - Keep only the most recent N messages
- **Token Budgeting** - Stay within token limits
- **System Message Preservation** - Always keep the system prompt
- **Windowing Strategies** - Different approaches for different use cases
- **LLM-Based Compaction** - Condense older messages into a summary instead of dropping them (`WithCompaction`)

## Quick Start

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/context"
)

// Simple: limit to 50 messages
a, _ := agent.New(config,
    agent.WithMaxMessages(50),
)

// Advanced: full configuration
a, _ := agent.New(config,
    agent.WithContextConfig(context.Config{
        MaxMessages:   100,
        MaxTokens:     8000,
        ReserveTokens: 4096,
    }),
)
```

## Configuration

### Config Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `MaxMessages` | int | 100 | Maximum messages to keep |
| `MaxTokens` | int | 0 | Maximum token budget (0 = unlimited) |
| `ReserveTokens` | int | 4096 | Tokens reserved for response |
| `TokenCounter` | TokenCounter | SimpleTokenCounter | Token estimation strategy |
| `CompactionEnabled` | bool | false | Enable LLM-based summarization of older messages |
| `CompactionThreshold` | int | 50 | Message count that triggers compaction |
| `Summarizer` | SummarizeFunc | nil | Called to condense older messages; required for `CompactionEnabled` to do anything (set automatically by `agent.WithCompaction`) |

### Example Configurations

#### Chat Application (Memory Efficient)

```go
context.Config{
    MaxMessages:   30,
    MaxTokens:     4000,
    ReserveTokens: 2000,
}
```

#### Research Assistant (Deep Context)

```go
context.Config{
    MaxMessages:   200,
    MaxTokens:     32000,
    ReserveTokens: 8000,
}
```

#### Quick Q&A (Minimal Context)

```go
context.Config{
    MaxMessages: 10,
}
```

## How It Works

### Message Windowing

When messages exceed `MaxMessages`, the engine keeps:

1. The system message (always preserved)
2. The most recent messages up to the limit

```
Before (8 messages, limit 5):
  [System] [User1] [Asst1] [User2] [Asst2] [User3] [Asst3] [User4]

After:
  [System] [Asst2] [User3] [Asst3] [User4]
```

### Token Budgeting

When token count exceeds `MaxTokens - ReserveTokens`:

1. System message tokens are counted first
2. Messages are added from most recent backward
3. Older messages are dropped when budget is exceeded

```go
budget := MaxTokens - ReserveTokens
// System: 500 tokens
// Available for history: budget - 500
```

## Token Counting

### SimpleTokenCounter (Default)

Estimates tokens using character count (~4 chars/token for English):

```go
counter := &context.SimpleTokenCounter{
    CharsPerToken: 4,  // Default
}
```

### ModelTokenCounter

For more accurate counting with specific models:

```go
counter := &context.ModelTokenCounter{
    Model:    "gpt-4",
    Fallback: &context.SimpleTokenCounter{},
}
```

## Windowing Strategies

### Recent (Default)

Keeps the most recent messages:

```go
window := context.NewWindow(context.WindowConfig{
    Strategy:    context.WindowStrategyRecent,
    MaxMessages: 50,
})
```

### Important

Keeps user questions and tool calls, plus recent messages:

```go
window := context.NewWindow(context.WindowConfig{
    Strategy:    context.WindowStrategyImportant,
    MaxMessages: 50,
})
```

### Summarize

Condenses older messages into a single summary using an LLM, instead of
dropping them. The simplest way to enable this is `agent.WithCompaction`,
which wires the agent's own LLM client into the context engine:

```go
a, _ := agent.New(config,
    agent.WithMaxMessages(100), // still enforced as a hard ceiling
    agent.WithCompaction(50),   // summarize once history exceeds 50 messages
)
```

Call `WithCompaction` *after* `WithMaxMessages`/`WithContextConfig`/
`WithContextEngine` if combining them — those replace the context engine
wholesale, which would discard compaction settings applied before them.

Customize the summarization prompt via `Config.CompactionPrompt`:

```go
a, _ := agent.New(agent.Config{
    CompactionPrompt: "Summarize the discussion, focusing on action items.",
}, agent.WithCompaction(50))
```

Compaction keeps the system message and the most recent messages
verbatim, and replaces everything older with a single synthetic system
message containing the summary. If summarization fails (no summarizer
configured, or the LLM call errors), it falls back to the plain recency
window and the failure is logged — a turn never breaks because
compaction failed.

Using `Window` directly (e.g. for a custom engine or a standalone use of
the strategy) requires providing your own `Summarizer`:

```go
window := context.NewWindow(context.WindowConfig{
    Strategy:    context.WindowStrategySummarize,
    MaxMessages: 50,
    Summarizer: func(ctx context.Context, messages []provider.Message) (string, error) {
        // Call your own LLM here and return the summary text.
        return callYourLLM(ctx, messages)
    },
})
```

## Agent Integration

The context engine is automatically applied during message processing.
`Apply` returns an error alongside the (always usable) messages — non-nil
only when compaction was enabled and failed, in which case the plain
recency window was used as a fallback:

```go
// In agent.processInternal():
if a.contextEngine != nil {
    windowed, err := a.contextEngine.Apply(ctx, messages)
    if err != nil {
        logger.Warn("context compaction degraded, using recency fallback", "error", err)
    }
    messages = windowed
}
```

### Manual Usage

You can also use the engine directly:

```go
engine := context.New(context.DefaultConfig())

// Estimate tokens
tokens := engine.EstimateTokens(messages)

// Apply windowing (and compaction, if enabled)
windowed, err := engine.Apply(ctx, messages)

// Check available budget
available := engine.AvailableTokens(messages)
```

## Token Budget Tracking

Track token usage over time:

```go
budget := &context.TokenBudget{
    Total:    8000,
    Reserved: 2000,
}

// Check available
fmt.Println(budget.Available())  // 6000

// Consume tokens
budget.Consume(1500)
fmt.Println(budget.Available())  // 4500

// Check if over budget
if budget.OverBudget() {
    // Handle overflow
}

// Reset for new conversation
budget.Reset()
```

## Best Practices

### 1. Set Appropriate Limits

Match limits to your LLM's context window:

| Model | Context Window | Suggested MaxTokens |
|-------|----------------|---------------------|
| GPT-4 | 8K | 6000 |
| GPT-4-32K | 32K | 28000 |
| Claude 3 | 200K | 150000 |
| Gemini 1.5 | 1M | 500000 |

### 2. Reserve Response Tokens

Always reserve tokens for the response:

```go
ReserveTokens: 4096,  // Typical max response
```

### 3. Consider Use Case

- **Chat**: Moderate limits, recent strategy
- **Research**: High limits, important strategy
- **Quick tasks**: Low limits, minimal context

### 4. Monitor Token Usage

Log token usage to understand patterns:

```go
tokens := engine.EstimateTokens(messages)
logger.Info("context tokens", "count", tokens)
```

## Architecture

```
┌───────────────────────────────────────────────────────────┐
│                     processInternal()                     │
│                                                           │
│  messages = buildMessages(session, content)               │
│         │                                                 │
│         ▼                                                 │
│  ┌───────────────────────────────────────────────────┐   │
│  │                   Context Engine                  │   │
│  │                                                    │   │
│  │  1. Compaction (if CompactionEnabled + over        │   │
│  │     CompactionThreshold) — delegates to a Window   │   │
│  │     configured for WindowStrategySummarize,        │   │
│  │     calling Summarizer for the older messages      │   │
│  │  2. applyMessageWindow() ◄── MaxMessages            │   │
│  │  3. applyTokenWindow()  ◄── MaxTokens, TokenCounter │   │
│  └───────────────────────────────────────────────────┘   │
│         │                                                 │
│         ▼                                                 │
│  windowed (and possibly compacted) messages → LLM request │
└───────────────────────────────────────────────────────────┘
```

`agent.WithCompaction` is what wires step 1's `Summarizer` to a real LLM
call, reusing the agent's own client (`agent/compaction.go`).

## See Also

- [Sessions & Storage](sessions.md) - Persistent conversation history
- [Configuration Reference](../reference/configuration.md) - Full config options
