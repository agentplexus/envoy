# Role System Guide

## Overview

The OmniAgent role system separates **organizational responsibilities (Roles)** from **runtime implementations (Agents)**. This enables:

- **Reuse**: Define a role once, instantiate with multiple agent implementations
- **Governance**: Roles define permissions, policies, and required outputs
- **Delegation**: Roles can orchestrate sub-agents with clear boundaries
- **Auditability**: Track agent behavior against role responsibilities

## Architecture

```
omniskill/role/           omniagent/agent/roles/
├── role.go              ├── role.go (Manager)
├── spec.go              ├── policy_engine.go
├── behavior.go          ├── behavior_engine.go
├── policy.go            ├── metrics_store.go
├── metric.go            └── delegator.go
└── delegation.go
```

Types live in `omniskill/role` (data definitions), while runtime engines live in `omniagent/agent/roles` (enforcement).

## Core Concepts

### Role Interface

Every role must implement the `role.Role` interface:

```go
type Role interface {
    Name() string
    Description() string
    Spec() *RoleSpec
    SystemPrompt(ctx context.Context) (string, error)
    RequiredSkills() []string
    Init(ctx context.Context, skills map[string]skill.Skill) error
    Close() error
    Workflows() []Workflow
}
```

### RoleSpec

The `RoleSpec` is the central data structure capturing everything about a role:

```go
type RoleSpec struct {
    ID              string
    Name            string
    Description     string
    Version         string
    Purpose         string
    Goals           []string
    Responsibilities []Responsibility
    Skills          SkillRequirements
    Policies        []Policy
    Memory          *MemoryPolicy
    Behaviors       []Behavior
    Artifacts       []ArtifactSpec
    Metrics         []MetricDefinition
    Delegation      *DelegationConfig
    Persona         *PersonaSpec
    Metadata        map[string]any
}
```

### Optional Interfaces

Not all roles need all capabilities. Implement optional interfaces as needed:

| Interface | Purpose |
|-----------|---------|
| `SkillRequirer` | Roles with optional skills beyond required |
| `BehaviorProvider` | Roles with context-aware behaviors |
| `MetricsProvider` | Roles with KPIs and success metrics |
| `DelegationProvider` | Roles that orchestrate sub-agents |
| `PolicyProvider` | Roles with governance rules |

## Using the Manager

The `Manager` integrates roles with the runtime engines:

```go
import "github.com/plexusone/omniagent/agent/roles"

// Create role
pmRole := meetingpm.New(meetingpm.Config{
    DefaultConfluenceSpace: "TEAM",
})

// Create manager with skills
mgr, err := roles.NewManager(pmRole, meetingSkill, googleSkill)
if err != nil {
    return err
}

// Initialize
if err := mgr.Init(ctx); err != nil {
    return err
}
defer mgr.Close()

// Access role capabilities
prompt, _ := mgr.SystemPrompt(ctx)
workflows := mgr.Workflows()
spec := mgr.Spec()

// Use engines
if err := mgr.CheckToolAccess(ctx, "confluence_publish"); err != nil {
    // Tool access denied by policy
}

mgr.SetBehaviorContext(role.BehaviorContextMeeting)
behaviors := mgr.GetActiveBehaviors(ctx)

mgr.RecordMetric(ctx, "meetings-facilitated", 1)
```

## Behaviors

Behaviors define context-specific actions:

```go
behavior := role.Behavior{
    ID:          "pre-meeting-prep",
    Name:        "Pre-meeting Preparation",
    Description: "Gather pre-reads before meetings",
    Context:     role.BehaviorContextAlways,
    Trigger: role.BehaviorTrigger{
        Type:     role.TriggerTypeSchedule,
        Schedule: "15 minutes before meeting",
    },
    Actions: []role.BehaviorAction{
        {ID: "gather-prereads", Type: role.ActionTypeWorkflow, Workflow: "prepare"},
    },
    Enabled: true,
}
```

### Behavior Contexts

- `BehaviorContextMeeting`: During active meetings
- `BehaviorContextChat`: During chat conversations
- `BehaviorContextAutonomous`: Running autonomously
- `BehaviorContextAlways`: All contexts

### Trigger Types

- `event`: Triggered by events like `meeting_start`, `meeting_end`
- `schedule`: Triggered at scheduled times
- `condition`: Triggered when a CEL condition evaluates to true
- `manual`: Triggered by user command

## Policies

Policies define governance rules:

```go
policy := role.Policy{
    ID:          "no-external-apis",
    Name:        "Restrict External API Access",
    Type:        role.PolicyTypeToolAccess,
    Rules: []role.PolicyRule{
        {
            ID:     "block-external",
            Action: role.PolicyActionDeny,
            Target: role.PolicyTarget{
                Type:    "tool",
                Pattern: "external_*",
            },
            Reason: "External API access requires approval",
        },
    },
    Enforcement: role.PolicyEnforcement{
        Mode:    role.EnforcementModeBlock,
        Message: "This operation requires explicit approval",
    },
    Enabled: true,
}
```

### Policy Types

- `tool_access`: Control which tools can be used
- `data_access`: Control access to data types
- `action_limit`: Restrict certain actions
- `rate_limit`: Enforce usage limits
- `confirmation_required`: Require user confirmation

### Enforcement Modes

- `block`: Prevent the action
- `warn`: Allow but log a warning
- `audit`: Log for review, take no action
- `confirm`: Require user confirmation

## Metrics

Define success metrics for roles:

```go
metric := role.MetricDefinition{
    ID:          "action-capture-rate",
    Name:        "Action Capture Rate",
    Description: "Percentage of action items captured",
    Type:        role.MetricTypeGauge,
    Unit:        role.UnitPercent,
    Target: &role.MetricTarget{
        Value:    95,
        Operator: ">=",
    },
}
```

### Metric Types

- `counter`: Cumulative count
- `gauge`: Point-in-time value
- `histogram`: Distribution of values
- `summary`: Quantiles over time

## Delegation

Enable sub-agent orchestration:

```go
delegation := role.DelegationConfig{
    Enabled: true,
    Rules: []role.DelegationRule{
        {
            ID:           "security-review",
            Name:         "Security Review Delegation",
            TaskPatterns: []string{"security_*", "audit_*"},
            TargetRoles:  []string{"security-analyst"},
            Autonomous:   false,
        },
    },
    Budget: &role.DelegationBudget{
        MaxConcurrent: 3,
        MaxDaily:      10,
    },
}
```

## Example: Meeting PM Role

```go
func (r *MeetingPMRole) Spec() *role.RoleSpec {
    return &role.RoleSpec{
        ID:          "meeting-pm",
        Name:        "Meeting Program Manager",
        Description: "Facilitates meetings and creates documentation",
        Purpose:     "Transform meetings into actionable outcomes",
        Goals: []string{
            "Ensure all meetings have clear agendas",
            "Capture all action items and decisions",
            "Publish notes within 1 hour",
        },
        Responsibilities: []role.Responsibility{
            {ID: "prepare", Name: "Meeting Preparation", Phase: "pre-meeting"},
            {ID: "facilitate", Name: "Meeting Facilitation", Phase: "meeting"},
            {ID: "document", Name: "Documentation", Phase: "post-meeting"},
        },
        Skills: role.SkillRequirements{
            Required: []role.SkillRef{
                {Name: "meeting", Purpose: "Join meetings via OmniMeet"},
                {Name: "confluence", Purpose: "Publish notes"},
            },
        },
        Behaviors: []role.Behavior{
            // Pre-meeting, during-meeting, post-meeting behaviors
        },
        Metrics: []role.MetricDefinition{
            role.NewGaugeMetric("action-capture-rate", "Action Capture Rate", "", "percent"),
            role.NewHistogramMetric("notes-published-time", "Publication Time", "", "seconds", nil),
        },
    }
}
```

## Best Practices

1. **Keep roles focused**: One primary responsibility per role
2. **Define clear metrics**: Measurable success criteria
3. **Use behaviors sparingly**: Only for truly context-dependent actions
4. **Policies for governance**: Use for security and compliance
5. **Test your spec**: Validate that Spec() returns expected values
