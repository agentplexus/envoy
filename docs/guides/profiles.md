# Agent Profiles

OmniAgent supports bootstrap profiles for configuring agent behavior at startup. Profiles enable customizing system prompts, filtering tools, and optimizing resource usage.

## Overview

Profiles provide three key capabilities:

1. **Bootstrap Profiles** - Modify system prompts and filter available tools
2. **Lean Mode** - Reduce memory and token usage for constrained environments
3. **Progress Reporting** - Track and report tool execution progress

## Bootstrap Profiles

A bootstrap profile defines how the agent behaves for a specific use case.

### Defining a Profile

```go
import "github.com/plexusone/omniagent/agent/profiles"

profile := &profiles.BootstrapProfile{
    Name:        "customer-support",
    Description: "Profile for customer support interactions",

    // Modify the system prompt
    SystemPromptPrefix: "You are a customer support agent.\n",
    SystemPromptSuffix: "\nAlways be polite and helpful.",

    // Filter available tools
    AllowedTools: []string{"search_kb", "create_ticket", "lookup_order"},
    DeniedTools:  []string{"shell", "browser", "http_request"},

    // Tool execution policies
    ToolPolicies: map[string]profiles.ToolPolicy{
        "create_ticket": {
            RequiresConfirmation: true,
            MaxCallsPerSession:   5,
        },
    },
}
```

### Using Profiles with Agent

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/agent/profiles"
)

// Create agent with a profile
a, err := agent.New(config,
    agent.WithProfile(profile),
)

// Or register profiles and activate by name
registry := profiles.NewRegistry()
registry.Register(profile)

a, err := agent.New(config,
    agent.WithProfileRegistry(registry),
)

// Activate a profile dynamically
err = a.ActivateProfile(ctx, "customer-support")
```

### Profile Fields

| Field | Type | Description |
|-------|------|-------------|
| `Name` | string | Unique profile identifier |
| `Description` | string | Human-readable description |
| `SystemPromptPrefix` | string | Text prepended to system prompt |
| `SystemPromptSuffix` | string | Text appended to system prompt |
| `AllowedTools` | []string | Whitelist of allowed tools (empty = all) |
| `DeniedTools` | []string | Blacklist of denied tools |
| `ToolPolicies` | map | Per-tool execution policies |

### Tool Policies

Tool policies control how individual tools are invoked:

```go
policy := profiles.ToolPolicy{
    // Require user confirmation before execution
    RequiresConfirmation: true,

    // Limit calls per session
    MaxCallsPerSession: 10,

    // Rate limiting
    RateLimit: &profiles.RateLimit{
        MaxCalls: 5,
        Window:   time.Minute,
    },
}
```

## Lean Mode

Lean mode reduces memory and token usage by limiting context size and simplifying responses.

### Lean Levels

| Level | Description | Use Case |
|-------|-------------|----------|
| `Off` | No reduction | Default operation |
| `Light` | Minimal reduction | Slightly constrained environments |
| `Moderate` | Balanced reduction | Mobile or embedded systems |
| `Aggressive` | Maximum reduction | Severely constrained environments |

### Configuration

```go
import "github.com/plexusone/omniagent/agent/profiles"

leanMode := profiles.NewLeanMode(profiles.LeanLevelModerate)

// Configure specific limits
leanMode.MaxContextTokens = 4000
leanMode.MaxResponseTokens = 1000
leanMode.TruncateHistory = true
leanMode.SimplifyToolOutputs = true

// Apply to agent
a, err := agent.New(config,
    agent.WithLeanMode(leanMode),
)
```

### Lean Mode Fields

| Field | Type | Description |
|-------|------|-------------|
| `Level` | LeanLevel | Preset level (Off, Light, Moderate, Aggressive) |
| `Enabled` | bool | Whether lean mode is active |
| `MaxContextTokens` | int | Maximum context window size |
| `MaxResponseTokens` | int | Maximum response size |
| `TruncateHistory` | bool | Truncate old conversation history |
| `SimplifyToolOutputs` | bool | Reduce tool output verbosity |
| `DisableSkills` | []string | Skills to disable in lean mode |

### Memory Savings

Estimate memory savings for each level:

```go
leanMode := profiles.NewLeanMode(profiles.LeanLevelModerate)
savings := leanMode.EstimateMemorySavings()
fmt.Printf("Estimated savings: %.1f%%\n", savings*100)
```

## Progress Reporting

Track tool execution progress for long-running operations.

### Setup

```go
reporter := profiles.NewProgressReporter(profiles.ProgressReporterConfig{
    DetailMode: profiles.ProgressDetailNormal,
    Callback: func(event profiles.ProgressEvent) {
        fmt.Printf("[%s] %s: %s\n", event.Tool, event.Status, event.Message)
    },
})

a, err := agent.New(config,
    agent.WithProgressReporter(reporter),
)
```

### Detail Modes

| Mode | Description |
|------|-------------|
| `Quiet` | No progress output |
| `Minimal` | Start/end only |
| `Normal` | Key milestones |
| `Verbose` | Detailed progress |
| `Debug` | All internal events |

### Progress Events

```go
type ProgressEvent struct {
    Tool      string        // Tool name
    Status    string        // "started", "progress", "completed", "failed"
    Message   string        // Human-readable message
    Progress  float64       // 0.0 to 1.0 for progress percentage
    Metadata  map[string]any // Additional context
    Timestamp time.Time
}
```

## Profile Registry

Manage multiple profiles for different use cases:

```go
registry := profiles.NewRegistry()

// Register profiles
registry.Register(supportProfile)
registry.Register(adminProfile)
registry.Register(guestProfile)

// List available profiles
names := registry.List()

// Get a specific profile
profile, ok := registry.Get("admin")

// Unregister a profile
registry.Unregister("guest")
```

## Predefined Profiles

OmniAgent includes predefined profiles for common use cases:

```go
import "github.com/plexusone/omniagent/agent/profiles"

// Restrictive profile - minimal tool access
restrictive := profiles.Restrictive()

// Permissive profile - full tool access
permissive := profiles.Permissive()

// Read-only profile - no write operations
readOnly := profiles.ReadOnly()
```

## Best Practices

### Security

- Use `DeniedTools` to block dangerous tools for untrusted contexts
- Enable `RequiresConfirmation` for destructive operations
- Apply rate limits to prevent abuse

### Performance

- Use lean mode in constrained environments
- Disable unused skills to reduce prompt size
- Set appropriate `MaxCallsPerSession` limits

### Flexibility

- Create profiles for different user roles (admin, user, guest)
- Use the registry for dynamic profile switching
- Clone and modify profiles for variations:

```go
customProfile := baseProfile.Clone()
customProfile.Name = "custom"
customProfile.DeniedTools = append(customProfile.DeniedTools, "shell")
```
