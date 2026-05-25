# Access Policies

OmniAgent provides fine-grained access control through tool policies and channel conformance checking.

## Overview

Policies control access at two levels:

1. **Tool Policies** - Per-sender control over which tools can be invoked
2. **Channel Policies** - Message validation and rate limiting per channel

## Tool Policies

Tool policies restrict which tools are available to specific senders (users, channels, sessions).

### Basic Usage

```go
import "github.com/plexusone/omniagent/tools/policy"

// Create a policy manager
manager := policy.NewManager()

// Set a policy for a user
manager.SetPolicy("user-123", &policy.Policy{
    AllowedTools: []string{"search", "calculator"},
    DeniedTools:  []string{"shell", "browser"},
})

// Check access before tool invocation
err := manager.CheckAccess(ctx, "user-123", "shell")
if err != nil {
    // Access denied
}
```

### Policy Fields

| Field | Type | Description |
|-------|------|-------------|
| `AllowedTools` | []string | Whitelist of allowed tools (empty = all allowed) |
| `DeniedTools` | []string | Blacklist of denied tools |
| `RateLimit` | *RateLimit | Rate limiting configuration |
| `ExpiresAt` | time.Time | Policy expiration time |

### Rate Limiting

Limit tool invocations per sender:

```go
manager.SetPolicy("user-123", &policy.Policy{
    RateLimit: &policy.RateLimit{
        MaxCalls:  10,           // Maximum calls
        Window:    time.Minute,  // Time window
        BurstSize: 3,            // Allow burst
    },
})
```

Rate limit errors include retry information:

```go
err := manager.CheckAccess(ctx, "user-123", "search")
if rateLimitErr, ok := err.(*policy.RateLimitError); ok {
    fmt.Printf("Rate limited. Retry after: %v\n", rateLimitErr.RetryAfter)
}
```

### Policy Inheritance

Policies can inherit from parent policies:

```go
// Base policy for all users
manager.SetPolicy("*", &policy.Policy{
    DeniedTools: []string{"shell"},
})

// Admin users get additional access
manager.SetPolicy("admin-*", &policy.Policy{
    AllowedTools: []string{"shell", "browser", "http_request"},
})

// Specific user policy
manager.SetPolicy("user-alice", &policy.Policy{
    AllowedTools: []string{"search"},
    RateLimit:    &policy.RateLimit{MaxCalls: 100, Window: time.Hour},
})
```

### Integration with Agent

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/tools/policy"
)

policyManager := policy.NewManager()
policyManager.SetPolicy("guest", &policy.Policy{
    AllowedTools: []string{"search", "weather"},
})

a, err := agent.New(config,
    agent.WithToolPolicyManager(policyManager),
)
```

## Channel Policies

Channel policies validate messages against content rules and rate limits.

### Basic Usage

```go
import "github.com/plexusone/omniagent/channels/policy"

// Create a conformance checker
checker := policy.NewConformanceChecker(policy.ConformanceConfig{
    Logger: logger,
})

// Add rules
checker.AddRule(policy.ConformanceRule{
    Name:        "no-profanity",
    Description: "Block profane content",
    Pattern:     `\b(badword1|badword2)\b`,
    Action:      policy.ActionReject,
})

// Check a message
result := checker.Check(ctx, policy.Message{
    SenderID:  "user-123",
    ChannelID: "telegram",
    Content:   "Hello, world!",
})

if !result.Allowed {
    fmt.Printf("Message rejected: %s\n", result.Reason)
}
```

### Rule Configuration

| Field | Type | Description |
|-------|------|-------------|
| `Name` | string | Rule identifier |
| `Description` | string | Human-readable description |
| `Pattern` | string | Regex pattern to match |
| `Channels` | []string | Channels this rule applies to (empty = all) |
| `Action` | Action | What to do on match |
| `RateLimit` | *RateLimit | Rate limiting for this rule |

### Actions

| Action | Description |
|--------|-------------|
| `ActionAllow` | Allow the message |
| `ActionReject` | Reject the message |
| `ActionFlag` | Allow but flag for review |
| `ActionRateLimit` | Apply rate limiting |

### Rate Limiting by Channel

```go
checker.AddRule(policy.ConformanceRule{
    Name:        "telegram-rate-limit",
    Description: "Rate limit Telegram messages",
    Channels:    []string{"telegram"},
    Action:      policy.ActionRateLimit,
    RateLimit: &policy.RateLimit{
        MaxMessages: 30,
        Window:      time.Minute,
    },
})
```

### Content Filtering

```go
// Block URLs
checker.AddRule(policy.ConformanceRule{
    Name:    "no-urls",
    Pattern: `https?://[^\s]+`,
    Action:  policy.ActionReject,
})

// Flag sensitive content
checker.AddRule(policy.ConformanceRule{
    Name:    "flag-pii",
    Pattern: `\b\d{3}-\d{2}-\d{4}\b`,  // SSN pattern
    Action:  policy.ActionFlag,
})

// Allow specific patterns
checker.AddRule(policy.ConformanceRule{
    Name:    "allow-support-links",
    Pattern: `https://support\.example\.com`,
    Action:  policy.ActionAllow,
})
```

### Observability Integration

Channel policies integrate with omniobserve for tracking:

```go
checker := policy.NewConformanceChecker(policy.ConformanceConfig{
    ObservopsProvider: observopsProvider,
    AgentopsStore:     agentopsStore,
    Logger:            logger,
})

// Policy violations are recorded as spans
// Metrics track rejection rates per rule
```

## Policy Registry

Manage policies centrally with the registry:

```go
import "github.com/plexusone/omniagent/tools/policy"

registry := policy.NewRegistry()

// Register policies by name
registry.Set("guest", guestPolicy)
registry.Set("user", userPolicy)
registry.Set("admin", adminPolicy)

// Look up policies
p, ok := registry.Get("admin")

// List all policies
names := registry.List()

// Remove a policy
registry.Delete("guest")
```

## Best Practices

### Defense in Depth

Apply policies at multiple levels:

```go
// 1. Channel level - filter incoming messages
channelChecker.AddRule(policy.ConformanceRule{
    Name:    "rate-limit",
    Action:  policy.ActionRateLimit,
    RateLimit: &policy.RateLimit{MaxMessages: 60, Window: time.Minute},
})

// 2. Tool level - restrict tool access
toolManager.SetPolicy("guest", &policy.Policy{
    AllowedTools: []string{"search"},
    DeniedTools:  []string{"shell", "browser"},
})

// 3. Profile level - customize agent behavior
profile := &profiles.BootstrapProfile{
    DeniedTools: []string{"http_request"},
    ToolPolicies: map[string]profiles.ToolPolicy{
        "search": {MaxCallsPerSession: 20},
    },
}
```

### Audit Logging

Log policy decisions for compliance:

```go
checker := policy.NewConformanceChecker(policy.ConformanceConfig{
    Logger: slog.New(slog.NewJSONHandler(auditFile, nil)),
})
```

### Graceful Degradation

Handle policy errors gracefully:

```go
result := checker.Check(ctx, message)
switch result.Action {
case policy.ActionReject:
    return errors.New("message not allowed: " + result.Reason)
case policy.ActionFlag:
    flagForReview(message, result.Reason)
    // Continue processing
case policy.ActionRateLimit:
    return &RateLimitError{RetryAfter: result.RetryAfter}
}
```

### Dynamic Policies

Update policies at runtime:

```go
// Watch for policy updates
go func() {
    for update := range policyUpdates {
        manager.SetPolicy(update.SenderID, update.Policy)
    }
}()
```
