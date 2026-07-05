# Role Migration Guide

This guide helps migrate existing roles to the enhanced role system with `RoleSpec`.

## What Changed

The `Role` interface now includes a `Spec()` method:

```go
type Role interface {
    Name() string
    Description() string
    Spec() *RoleSpec  // NEW
    SystemPrompt(ctx context.Context) (string, error)
    RequiredSkills() []string
    Init(ctx context.Context, skills map[string]skill.Skill) error
    Close() error
    Workflows() []Workflow
}
```

## Migration Steps

### Step 1: Add the Spec() Method

Every role must now implement `Spec()`. At minimum:

```go
func (r *MyRole) Spec() *role.RoleSpec {
    return &role.RoleSpec{
        ID:          r.Name(),
        Name:        r.Name(),
        Description: r.Description(),
        Skills: role.SkillRequirements{
            Required: role.SkillRefsFromStrings(r.RequiredSkills()),
        },
    }
}
```

### Step 2: Enhance with Optional Features

Add features as needed:

```go
func (r *MyRole) Spec() *role.RoleSpec {
    return &role.RoleSpec{
        ID:          "my-role",
        Name:        "My Role",
        Description: "Does something useful",
        Version:     "1.0.0",
        Purpose:     "Why this role exists",

        // Add goals
        Goals: []string{
            "Accomplish X",
            "Ensure Y",
        },

        // Add responsibilities
        Responsibilities: []role.Responsibility{
            {ID: "task-1", Name: "Main Task", Priority: "high"},
        },

        // Skills are required
        Skills: role.SkillRequirements{
            Required: []role.SkillRef{
                {Name: "skill-a", Purpose: "For doing A"},
            },
            Optional: []role.SkillRef{
                {Name: "skill-b", Purpose: "For optional B"},
            },
        },

        // Add if your role has context-aware behaviors
        Behaviors: []role.Behavior{
            // ...
        },

        // Add for policy enforcement
        Policies: []role.Policy{
            // ...
        },

        // Add for KPI tracking
        Metrics: []role.MetricDefinition{
            // ...
        },

        // Add for sub-agent delegation
        Delegation: &role.DelegationConfig{
            // ...
        },
    }
}
```

### Step 3: Implement Optional Interfaces

If you added behaviors, metrics, or policies, implement the corresponding interfaces:

```go
// For behaviors
func (r *MyRole) Behaviors() []role.Behavior {
    return r.Spec().Behaviors
}
var _ role.BehaviorProvider = (*MyRole)(nil)

// For metrics
func (r *MyRole) Metrics() []role.MetricDefinition {
    return r.Spec().Metrics
}
var _ role.MetricsProvider = (*MyRole)(nil)

// For optional skills
func (r *MyRole) OptionalSkills() []string {
    return []string{"skill-b"}
}
var _ role.SkillRequirer = (*MyRole)(nil)
```

### Step 4: Update Interface Assertions

Update your compile-time interface checks:

```go
// Before
var _ role.Role = (*MyRole)(nil)

// After
var _ role.Role = (*MyRole)(nil)
var _ role.SkillRequirer = (*MyRole)(nil)      // If implementing
var _ role.BehaviorProvider = (*MyRole)(nil)   // If implementing
var _ role.MetricsProvider = (*MyRole)(nil)    // If implementing
```

### Step 5: Test

Verify your role still works:

```go
func TestSpec(t *testing.T) {
    r := New(Config{})
    spec := r.Spec()

    if spec.ID == "" {
        t.Error("Spec ID should not be empty")
    }
    if spec.Name == "" {
        t.Error("Spec Name should not be empty")
    }
    if len(spec.Skills.Required) == 0 {
        t.Error("Spec should have required skills")
    }
}
```

## Using BaseRole

If you embed `BaseRole`, it provides a default `Spec()` implementation:

```go
type MyRole struct {
    role.BaseRole
}

func New() *MyRole {
    return &MyRole{
        BaseRole: role.BaseRole{
            RoleName:        "my-role",
            RoleDescription: "Does something useful",
            RoleSkills:      []string{"skill-a"},
        },
    }
}

// BaseRole.Spec() returns:
// &RoleSpec{
//     ID:          "my-role",
//     Name:        "my-role",
//     Description: "Does something useful",
//     Skills: SkillRequirements{
//         Required: []SkillRef{{Name: "skill-a"}},
//     },
// }
```

Override `Spec()` for more detailed specifications.

## Manager Integration

The `Manager` automatically initializes engines from your spec:

```go
mgr, _ := roles.NewManager(myRole, skills...)

// These work automatically if your spec defines them:
mgr.CheckToolAccess(ctx, "some-tool")     // Uses policies
mgr.GetActiveBehaviors(ctx)               // Uses behaviors
mgr.RecordMetric(ctx, "metric-id", 1.0)   // Uses metrics
mgr.CanDelegate(ctx, "task-type")         // Uses delegation
```

## Checklist

- [ ] Add `Spec()` method returning `*role.RoleSpec`
- [ ] Set required fields: `ID`, `Name`, `Description`, `Skills`
- [ ] Add optional fields as needed: `Behaviors`, `Policies`, `Metrics`, `Delegation`
- [ ] Implement optional interfaces: `SkillRequirer`, `BehaviorProvider`, `MetricsProvider`
- [ ] Update interface assertions with `var _`
- [ ] Run existing tests to ensure backward compatibility
- [ ] Add tests for new `Spec()` method
