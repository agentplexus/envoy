# Enhanced Role System - Technical Requirements Document

## Architecture Overview

The enhanced role system introduces a separation between role specifications (data) and role execution (runtime). This follows a clean architecture pattern where types live in `omniskill/role` and runtime engines live in `omniagent/agent/roles`.

```
omniskill/role/           omniagent/agent/roles/
├── role.go              ├── role.go (Manager)
├── spec.go              ├── policy_engine.go
├── behavior.go          ├── behavior_engine.go
├── policy.go            ├── metrics_store.go
├── metric.go            └── delegator.go
└── delegation.go
```

## Design Principles

### Interface Composition Pattern

Not all roles need meeting behaviors or delegation. Simple roles should remain simple. We use interface composition rather than one monolithic interface:

```go
// Base interface - all roles implement this
type Role interface {
    Name() string
    Description() string
    Spec() *RoleSpec
    SystemPrompt(ctx context.Context) (string, error)
    Init(ctx context.Context, skills map[string]skill.Skill) error
    Close() error
}

// Optional interfaces - implemented by roles that need them
type SkillRequirer interface {
    RequiredSkills() []string
    OptionalSkills() []string
}

type WorkflowProvider interface {
    Workflows() []Workflow
}

type BehaviorProvider interface {
    Behaviors() []Behavior
}

type MetricsProvider interface {
    Metrics() []MetricDefinition
}

type DelegationProvider interface {
    DelegationRules() []DelegationRule
}

type PolicyProvider interface {
    Policies() []Policy
}
```

### Data vs Enforcement Separation

Policy and behavior types in `omniskill/role` are pure data definitions. Enforcement happens in `omniagent/agent/roles` via engines:

- `Policy` (data) -> `PolicyEngine` (enforcement)
- `Behavior` (data) -> `BehaviorEngine` (matching)
- `MetricDefinition` (data) -> `MetricsStore` (collection)
- `DelegationRule` (data) -> `Delegator` (orchestration)

## Type Definitions

### RoleSpec

The central type defining a role's specification:

```go
type RoleSpec struct {
    ID              string            `json:"id"`
    Name            string            `json:"name"`
    Description     string            `json:"description"`
    Version         string            `json:"version,omitempty"`
    Purpose         string            `json:"purpose"`
    Goals           []string          `json:"goals,omitempty"`
    Responsibilities []Responsibility `json:"responsibilities,omitempty"`
    Skills          SkillRequirements `json:"skills"`
    Policies        []Policy          `json:"policies,omitempty"`
    Memory          *MemoryPolicy     `json:"memory,omitempty"`
    Behaviors       []Behavior        `json:"behaviors,omitempty"`
    Artifacts       []ArtifactSpec    `json:"artifacts,omitempty"`
    Metrics         []MetricDefinition `json:"metrics,omitempty"`
    Delegation      *DelegationConfig `json:"delegation,omitempty"`
    Persona         *PersonaSpec      `json:"persona,omitempty"`
    Metadata        map[string]any    `json:"metadata,omitempty"`
}
```

### Behavior Types

Context-specific behaviors enable roles to act differently in meetings vs chat:

```go
type BehaviorContext string
const (
    BehaviorContextMeeting    BehaviorContext = "meeting"
    BehaviorContextChat       BehaviorContext = "chat"
    BehaviorContextAutonomous BehaviorContext = "autonomous"
    BehaviorContextAlways     BehaviorContext = "always"
)

type Behavior struct {
    ID          string           `json:"id"`
    Name        string           `json:"name"`
    Description string           `json:"description"`
    Context     BehaviorContext  `json:"context"`
    Trigger     BehaviorTrigger  `json:"trigger"`
    Actions     []BehaviorAction `json:"actions"`
    Enabled     bool             `json:"enabled"`
}
```

### Policy Types

Policies define governance rules without enforcing them:

```go
type PolicyType string
const (
    PolicyTypeToolAccess   PolicyType = "tool_access"
    PolicyTypeDataAccess   PolicyType = "data_access"
    PolicyTypeActionLimit  PolicyType = "action_limit"
    PolicyTypeRateLimit    PolicyType = "rate_limit"
    PolicyTypeConfirmation PolicyType = "confirmation_required"
)

type Policy struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Type        PolicyType        `json:"type"`
    Rules       []PolicyRule      `json:"rules"`
    Enforcement PolicyEnforcement `json:"enforcement"`
}
```

### Metric Types

Success metrics enable KPI tracking:

```go
type MetricType string
const (
    MetricTypeCounter   MetricType = "counter"
    MetricTypeGauge     MetricType = "gauge"
    MetricTypeHistogram MetricType = "histogram"
)

type MetricDefinition struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Description string        `json:"description"`
    Type        MetricType    `json:"type"`
    Unit        string        `json:"unit,omitempty"`
    Target      *MetricTarget `json:"target,omitempty"`
}
```

### Delegation Types

Delegation enables sub-agent orchestration:

```go
type DelegationConfig struct {
    Enabled bool              `json:"enabled"`
    Rules   []DelegationRule  `json:"rules"`
    Budget  *DelegationBudget `json:"budget,omitempty"`
}

type DelegationRule struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`
    TaskPatterns []string `json:"task_patterns"`
    TargetRoles  []string `json:"target_roles"`
    Autonomous   bool     `json:"autonomous"`
}
```

## Runtime Engines

### PolicyEngine

Enforces policy rules at runtime:

```go
type PolicyEngine struct {
    policies []role.Policy
}

func NewPolicyEngine(policies []role.Policy) *PolicyEngine
func (e *PolicyEngine) CheckToolAccess(ctx context.Context, toolName string) error
func (e *PolicyEngine) CheckDataAccess(ctx context.Context, dataType string) error
func (e *PolicyEngine) CheckRateLimit(ctx context.Context, operation string) error
```

### BehaviorEngine

Matches behaviors to current context:

```go
type BehaviorEngine struct {
    behaviors []role.Behavior
    context   role.BehaviorContext
}

func NewBehaviorEngine(behaviors []role.Behavior) *BehaviorEngine
func (e *BehaviorEngine) SetContext(ctx role.BehaviorContext)
func (e *BehaviorEngine) GetActive(ctx context.Context) []role.Behavior
func (e *BehaviorEngine) MatchTrigger(ctx context.Context, event string) []role.Behavior
```

### MetricsStore

Collects and stores role metrics:

```go
type MetricsStore interface {
    Record(ctx context.Context, role, metric string, value float64) error
    Get(ctx context.Context, role, metric string) ([]MetricPoint, error)
}
```

### Delegator

Manages sub-agent delegation:

```go
type Delegator struct {
    rules []role.DelegationRule
}

func (d *Delegator) CanDelegate(ctx context.Context, taskType string) (bool, []string)
func (d *Delegator) SelectTargetRole(ctx context.Context, taskType string) (string, error)
```

## Integration with Manager

The enhanced Manager integrates all engines:

```go
type Manager struct {
    role           role.Role
    spec           *role.RoleSpec
    skills         map[string]skill.Skill
    policyEngine   *PolicyEngine
    behaviorEngine *BehaviorEngine
    metricsStore   MetricsStore
    delegator      *Delegator
}
```

## Migration Path

Existing roles continue to work via the default `Spec()` implementation in `BaseRole`:

```go
func (r *BaseRole) Spec() *RoleSpec {
    return &RoleSpec{
        ID:          r.RoleName,
        Name:        r.RoleName,
        Description: r.RoleDescription,
        Skills: SkillRequirements{
            Required: stringsToSkillRefs(r.RoleSkills),
        },
    }
}
```

## Testing Strategy

1. Unit tests for each new type file
2. Unit tests for each engine
3. Integration tests for Manager with enhanced roles
4. Backward compatibility tests with existing roles

## Security Considerations

- Policy enforcement must fail closed (deny by default)
- Delegation must respect permission boundaries
- Metrics should not expose sensitive data
