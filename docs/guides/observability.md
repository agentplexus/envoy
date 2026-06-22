# Observability Integrations

OmniAgent's hooks system can be used to send telemetry to external observability platforms. This guide shows how to build observability hooks that bridge omniagent events to tracing backends.

## Design Principles

OmniAgent itself remains **observability-agnostic**—it doesn't bundle any specific telemetry SDK. Instead:

1. **Generic events** - OmniAgent emits standard hook events (messages, tools, sessions)
2. **Your app bridges** - You create a hook in your application that adapts these events
3. **Any backend** - Connect to Opik, OpenTelemetry, Datadog, or any tracing system

This keeps omniagent lean and lets you choose your observability stack.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Your Application                                             │
│   └── ObservabilityHook (implements hooks.Hook)              │
│        └── converts hooks.Event → your backend's format      │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│ Telemetry Backend (Opik, OTEL, Datadog, etc.)                │
└──────────────────────────────────────────────────────────────┘
```

## Building an Observability Hook

### Step 1: Implement hooks.Hook

```go
type MyObservabilityHook struct {
    client    *telemetry.Client  // Your telemetry SDK
    traces    map[string]*Trace  // session → active trace
    mu        sync.RWMutex
}

func (h *MyObservabilityHook) Name() string {
    return "my-telemetry"
}

func (h *MyObservabilityHook) Events() []hooks.EventType {
    return []hooks.EventType{
        hooks.EventSessionCreated,
        hooks.EventSessionUpdated,
        hooks.EventMessageReceived,
        hooks.EventMessageSent,
        hooks.EventToolCalled,
        hooks.EventToolCompleted,
        hooks.EventJobExecuted,
    }
}

func (h *MyObservabilityHook) Init(ctx context.Context) error {
    return nil
}

func (h *MyObservabilityHook) Close() error {
    // Flush pending traces
    h.mu.Lock()
    defer h.mu.Unlock()
    for _, trace := range h.traces {
        trace.End()
    }
    return h.client.Flush()
}
```

### Step 2: Map Events to Traces

Create traces for sessions and spans for activities:

```go
func (h *MyObservabilityHook) Handle(ctx context.Context, event hooks.Event) error {
    switch event.Type {
    case hooks.EventSessionCreated:
        return h.startTrace(ctx, event.SessionID)
    case hooks.EventMessageReceived:
        return h.recordMessageSpan(ctx, event, "user")
    case hooks.EventToolCalled:
        return h.startToolSpan(ctx, event)
    case hooks.EventToolCompleted:
        return h.endToolSpan(ctx, event)
    case hooks.EventMessageSent:
        return h.recordMessageSpan(ctx, event, "assistant")
    }
    return nil
}

func (h *MyObservabilityHook) startTrace(ctx context.Context, sessionID string) error {
    h.mu.Lock()
    defer h.mu.Unlock()

    trace := h.client.StartTrace("agent.session." + sessionID)
    h.traces[sessionID] = trace
    return nil
}

func (h *MyObservabilityHook) recordMessageSpan(ctx context.Context, event hooks.Event, role string) error {
    h.mu.RLock()
    trace := h.traces[event.SessionID]
    h.mu.RUnlock()

    if trace == nil {
        return nil
    }

    msg := event.Data.(hooks.MessageEvent)
    span := trace.StartSpan("message." + role)
    span.SetAttribute("content", msg.Content)
    span.End()
    return nil
}
```

### Step 3: Handle Stale Sessions

Agent sessions may become inactive without explicit termination. Implement cleanup:

```go
type MyObservabilityHook struct {
    // ...
    staleTimeout time.Duration
    sweepTicker  *time.Ticker
}

func (h *MyObservabilityHook) Init(ctx context.Context) error {
    h.sweepTicker = time.NewTicker(1 * time.Minute)
    go h.sweepStaleTraces()
    return nil
}

func (h *MyObservabilityHook) sweepStaleTraces() {
    for range h.sweepTicker.C {
        h.mu.Lock()
        for sessionID, trace := range h.traces {
            if time.Since(trace.LastActivity) > h.staleTimeout {
                trace.End()
                delete(h.traces, sessionID)
            }
        }
        h.mu.Unlock()
    }
}

func (h *MyObservabilityHook) Close() error {
    h.sweepTicker.Stop()
    // ... flush traces
}
```

### Step 4: Register with Agent

```go
obsHook, _ := NewMyObservabilityHook(telemetryClient)

agent, _ := agent.New(config,
    agent.WithCompiledHook(obsHook),
)

agent.InitHooks(ctx)
defer agent.HookRegistry().Close()
```

## Event Mapping Reference

| OmniAgent Event | Typical Telemetry Action |
|-----------------|--------------------------|
| `EventSessionCreated` | Start trace for session |
| `EventSessionUpdated` | Touch/extend trace TTL |
| `EventMessageReceived` | Create span with user input |
| `EventToolCalled` | Start tool span |
| `EventToolCompleted` | End tool span with result/error |
| `EventMessageSent` | Create span with assistant output |
| `EventJobExecuted` | Create span for job execution |

## Content Sanitization

Before sending content to telemetry backends, consider sanitizing:

- **Secrets** - API keys, tokens, passwords
- **PII** - Personal information if compliance requires
- **Internal markers** - System prompts, metadata blocks

```go
func sanitize(content string) string {
    // Remove API key patterns
    content = apiKeyPattern.ReplaceAllString(content, "[REDACTED]")
    // Remove system metadata
    content = metadataPattern.ReplaceAllString(content, "")
    return content
}
```

## Example: Opik Integration

Here's an example using `opik-go/agentobs` to build an Opik integration:

```go
import (
    opikintegration "my-agent/integrations/opik"  // Your app's integration
    "github.com/plexusone/opik-go"
)

opikClient, _ := opik.NewClient(opik.WithProjectName("my-agent"))

opikHook, _ := opikintegration.New(
    opikintegration.WithClient(opikClient),
    opikintegration.WithTags("env:production"),
    opikintegration.WithStaleTimeout(10 * time.Minute),
)

agent, _ := agent.New(config,
    agent.WithCompiledHook(opikHook),
)
```

The `opik-go/agentobs` package provides reusable utilities:

- `TraceManager` - Session-based trace lifecycle with stale cleanup
- `Sanitize()` - Content sanitization utilities
- `ExtractMedia()` - Media extraction from content
- `AgentEvent` - Generic event type for event conversion

## Best Practices

1. **Keep omniagent dependency-free** - Build integrations in your app, not in omniagent

2. **Handle stale sessions** - Sessions may not explicitly close; implement TTL-based cleanup

3. **Sanitize content** - Remove secrets and PII before sending to external services

4. **Buffer and batch** - Batch trace submissions to reduce overhead

5. **Graceful shutdown** - Flush pending traces in `Close()`

6. **Use meaningful names** - Trace names should be descriptive (`agent.session.{id}`, `tool.{name}`)

## See Also

- [Hooks System](hooks.md) - Core hooks documentation
- [Sessions](sessions.md) - Session management
