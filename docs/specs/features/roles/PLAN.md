# Enhanced Role System - Implementation Plan

## Overview

This document outlines the file-by-file implementation plan for the enhanced role system.

## Phase 1: Create Spec Documents

**Status**: Complete

- [x] PRD.md
- [x] TRD.md
- [x] PLAN.md (this file)
- [x] ROADMAP.md

## Phase 2: New Types in omniskill/role/

**Location**: `/Users/johnwang/go/src/github.com/plexusone/omniskill/role/`

### New Files

| File | Description |
|------|-------------|
| `spec.go` | RoleSpec and supporting types (Responsibility, SkillRequirements, SkillRef, ArtifactSpec, PersonaSpec, MemoryPolicy) |
| `behavior.go` | Behavior types (BehaviorContext, Behavior, BehaviorTrigger, BehaviorAction) |
| `policy.go` | Policy types (PolicyType, Policy, PolicyRule, PolicyEnforcement) |
| `metric.go` | Metric types (MetricType, MetricDefinition, MetricTarget) |
| `delegation.go` | Delegation types (DelegationConfig, DelegationRule, DelegationBudget) |

### Modified Files

| File | Changes |
|------|---------|
| `role.go` | Add `Spec() *RoleSpec` to Role interface, add optional interfaces (SkillRequirer, BehaviorProvider, MetricsProvider, DelegationProvider, PolicyProvider), add default Spec() to BaseRole |

## Phase 3: Runtime Engines in omniagent/agent/roles/

**Location**: `/Users/johnwang/go/src/github.com/plexusone/omniagent/agent/roles/`

### New Files

| File | Description |
|------|-------------|
| `policy_engine.go` | PolicyEngine for runtime policy enforcement |
| `behavior_engine.go` | BehaviorEngine for context-aware behavior matching |
| `metrics_store.go` | MetricsStore interface and in-memory implementation |
| `delegator.go` | Delegator for sub-agent orchestration |

### Modified Files

| File | Changes |
|------|---------|
| `role.go` | Enhance Manager with spec, engines; add CheckToolAccess, GetActiveBehaviors, RecordMetric, CanDelegate methods |

## Phase 4: Update Existing Roles

### meeting-pm Role

**Location**: `/Users/johnwang/go/src/github.com/plexusone/omniagent-role-meeting-pm/`

| File | Changes |
|------|---------|
| `role.go` | Add `Spec() *role.RoleSpec` method with comprehensive spec |

Spec content:

- Responsibilities: prepare, facilitate, document
- Behaviors: pre-meeting-prep, during-meeting-notes, post-meeting-wrapup
- Artifacts: meeting-notes, action-items, decisions
- Metrics: action-capture-rate, notes-published-time

### Custom Roles

Any custom role implementations should add a `Spec() *role.RoleSpec` method following the same pattern as meeting-pm.

Example spec content for a domain-specific role:

- Responsibilities: domain-specific accountabilities
- Behaviors: context-aware actions
- Metrics: KPIs and success measurements

## Phase 5: Documentation

### omniagent/docs/

| File | Description |
|------|-------------|
| `roles.md` | Complete role system documentation |
| `roles-migration.md` | Migration guide for existing roles |

### omniskill/docs/

| File | Description |
|------|-------------|
| `role-interface.md` | Role interface reference |

## Verification Checklist

After each phase, run:

```bash
# omniskill
cd ~/go/src/github.com/plexusone/omniskill && go test -v ./role/... && golangci-lint run ./role/...

# omniagent
cd ~/go/src/github.com/plexusone/omniagent && go test -v ./agent/roles/... && golangci-lint run ./agent/roles/...

# omniagent-role-meeting-pm
cd ~/go/src/github.com/plexusone/omniagent-role-meeting-pm && go test -v ./... && golangci-lint run
```

## Critical Dependencies

The implementation order matters due to dependencies:

1. **omniskill/role types** must be created first (no dependencies)
2. **omniagent/agent/roles engines** depend on omniskill types
3. **Existing roles** depend on both omniskill types and interface changes
4. **Documentation** documents the completed implementation

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Breaking existing roles | Default Spec() implementation returns minimal valid RoleSpec |
| Interface bloat | Use interface composition (optional interfaces) |
| Circular dependencies | Keep types in omniskill, engines in omniagent |
| Test failures | Run existing tests after each change |
