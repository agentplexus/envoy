# Enhanced Role System - Roadmap

## Version Targets

### v0.10.0 - Foundation

**Target**: Initial release of enhanced role types

Features:

- [ ] RoleSpec type in omniskill/role
- [ ] Behavior, Policy, Metric, Delegation types
- [ ] Spec() method on Role interface
- [ ] Default Spec() in BaseRole
- [ ] Backward compatibility maintained

### v0.11.0 - Runtime Engines

**Target**: Policy and behavior enforcement

Features:

- [ ] PolicyEngine for tool/data access control
- [ ] BehaviorEngine for context-aware behavior matching
- [ ] Enhanced Manager integration
- [ ] Meeting PM role with full spec

### v0.12.0 - Metrics and Delegation

**Target**: Observability and orchestration

Features:

- [ ] MetricsStore implementation
- [ ] Delegator for sub-agent orchestration
- [ ] Investor role with full spec
- [ ] Metric collection and reporting

### v0.13.0 - Governance

**Target**: Enterprise features

Features:

- [ ] Role audit logging
- [ ] Policy violation alerts
- [ ] Delegation budget enforcement
- [ ] Role assignment tracking

## Feature Progression

### Behaviors

| Version | Capability |
|---------|------------|
| v0.10.0 | Behavior type definition |
| v0.11.0 | Context matching (meeting, chat, autonomous) |
| v0.12.0 | Trigger-based activation |
| v0.13.0 | Cross-role behavior coordination |

### Policies

| Version | Capability |
|---------|------------|
| v0.10.0 | Policy type definition |
| v0.11.0 | Tool access enforcement |
| v0.12.0 | Data access enforcement |
| v0.13.0 | Rate limiting, confirmations |

### Metrics

| Version | Capability |
|---------|------------|
| v0.10.0 | MetricDefinition type |
| v0.11.0 | In-memory metrics store |
| v0.12.0 | Persistent metrics, targets |
| v0.13.0 | Dashboards, alerts |

### Delegation

| Version | Capability |
|---------|------------|
| v0.10.0 | DelegationRule type |
| v0.11.0 | Basic delegation matching |
| v0.12.0 | Budget tracking |
| v0.13.0 | Dynamic role assignment |

## Migration Timeline

| Phase | Description | Target |
|-------|-------------|--------|
| 1 | Types and interfaces | v0.10.0 |
| 2 | Runtime engines | v0.11.0 |
| 3 | Existing role migration | v0.11.0 |
| 4 | New role creation guide | v0.12.0 |
| 5 | Enterprise features | v0.13.0 |

## Dependencies

### External

- No new external dependencies for v0.10.0
- Metrics storage (v0.12.0) may require time-series DB

### Internal

- omniskill/role (types)
- omniagent/agent/roles (runtime)
- omnirole-facilitator (reference implementation)

## Success Criteria

### v0.10.0

- All existing tests pass
- New types compile without errors
- BaseRole.Spec() returns valid RoleSpec

### v0.11.0

- PolicyEngine blocks unauthorized tool access
- BehaviorEngine returns correct behaviors per context
- Meeting PM spec defines all responsibilities

### v0.12.0

- Metrics are collected for defined MetricDefinitions
- Delegator correctly matches task patterns to roles
- Investor role spec defines all behaviors

### v0.13.0

- Audit log captures all policy decisions
- Delegation budget is enforced
- Role assignments are tracked and queryable
