# Tasks: OmniAgent v0.6.0 Implementation (Historical)

> **Status**: Completed 2026-04-25
> This document captures the tasks as completed for v0.6.0.

# Tasks: OmniAgent Extension Implementation

## Phase 1: Compiled Skill Interface

### Setup

- [x] **TASK-100**: Create `skills/compiled/` package (partial)
  - [x] Create `skills/compiled/skill.go` with `Skill` interface
  - [x] Create `skills/compiled/tool.go` with `Tool` type
  - [x] Create `skills/compiled/registry.go` for skill registration
  - [ ] ~~Create `skills/compiled/parameter.go` for JSON Schema parameters~~ (not needed)

- [x] **TASK-101**: Define core interfaces
  ```go
  type Skill interface {
      Name() string
      Description() string
      Tools() []Tool
      Init(ctx context.Context) error
      Close() error
  }

  type StorageAware interface {
      SetStorage(s storage.Storage)
  }
  ```

### Integration

- [x] **TASK-102**: Integrate compiled skills with agent
  - [x] Add `compiledSkills` field to `agent.Agent`
  - [x] Register compiled skill tools with existing `ToolRegistry`
  - [x] Call `Init()` on startup, `Close()` on shutdown

- [x] **TASK-103**: Add agent options for compiled skills
  - [x] `WithCompiledSkill(skill compiled.Skill) Option`
  - [ ] ~~`WithCompiledSkills(skills ...compiled.Skill) Option`~~ (not needed, use multiple WithCompiledSkill)

- [ ] **TASK-104**: Add configuration support (deferred)
  - [ ] List compiled skills in config (documentation only)
  - [ ] Skills are registered via code, config shows which are loaded

### Testing

- [x] **TASK-105**: Create mock skill for testing
  - [x] Implement `compiled.Skill` with test tools
  - [x] Test tool registration and execution
  - [x] Test storage injection

- [x] **TASK-106**: Integration test with agent
  - [x] Load compiled skill
  - [x] Verify tools appear in LLM requests
  - [x] Test tool execution flow

---

## Phase 2: Storage Interface

> **Note**: Storage was migrated to `omnistorage-core/kvs` instead of creating a local package.

### Setup

- [x] **TASK-200**: ~~Create `storage/` package~~ → Used `omnistorage-core/kvs`
  - [x] Storage interface provided by omnistorage-core
  - [x] Document type not needed (using raw bytes/JSON)
  - [x] Standard errors from omnistorage-core

- [x] **TASK-201**: ~~Define storage interface~~ → Using `kvs.Store` from omnistorage-core

### Implementations

- [x] **TASK-202**: ~~Implement memory storage~~ → Provided by `omnistorage-core/kvs/memory`

- [x] **TASK-203**: ~~Implement SQLite storage~~ → Provided by `omnistorage-core/kvs/sqlite`

- [ ] **TASK-204**: (Future) Implement DynamoDB storage
  - [ ] Create `storage/dynamodb/dynamodb.go`
  - [ ] Use AWS SDK v2
  - [ ] TTL via DynamoDB TTL feature

### Integration

- [x] **TASK-205**: Add storage to agent
  - [x] Add `storage` field to `agent.Agent`
  - [x] Add `WithStorage(s kvs.Store) Option`
  - [x] Add `WithSessionsFromStorage(s kvs.Store) Option`
  - [x] Inject storage into `StorageAware` skills

- [ ] **TASK-206**: Add storage configuration (deferred)
  - [ ] YAML config support for storage type/path

---

## Phase 3: Platform Adapters

### Setup

- [x] **TASK-300**: Create `platform/` package
  - [x] Create `platform/platform.go` with `Platform` interface
  - [ ] ~~Create `platform/config.go`~~ (config in standalone/config.go)

- [x] **TASK-301**: Define platform interface
  ```go
  type Platform interface {
      Run(ctx context.Context, agent *agent.Agent) error
  }

  type Config struct {
      Type    string         // standalone, lambda, agentcore
      Options map[string]any
  }
  ```

### Standalone Platform

- [x] **TASK-302**: Create `platform/standalone/`
  - [x] Standalone platform implementation
  - [x] HTTP server with graceful shutdown
  - [x] Configurable address

- [ ] **TASK-303**: Add webhook server (deferred to OmniChat integration)
  - [ ] HTTP server for incoming webhooks
  - [ ] Route to appropriate omnichat provider
  - [ ] Configurable address and TLS

### Lightsail Deployment

- [ ] **TASK-304**: Create `platform/lightsail/` helpers (deferred)
  - [ ] Systemd service template generation
  - [ ] Caddy configuration template
  - [ ] Deploy script helpers

### Future Platforms

- [ ] **TASK-305**: (Future) Lambda adapter
  - [ ] AWS Lambda handler wrapper
  - [ ] API Gateway event handling

- [ ] **TASK-306**: (Future) AgentCore adapter
  - [ ] Bedrock AgentCore integration
  - [ ] Action group mapping

### Integration

- [ ] **TASK-307**: Add platform to agent startup (deferred)
  - [ ] Add `WithPlatform(p platform.Platform) Option`
  - [ ] Default to standalone platform
  - [ ] Platform-specific configuration

---

## Additional Packages (Not in Original Plan)

The following packages were implemented in v0.6.0 but were not part of the original task list:

### Sessions Package

- [x] **Sessions**: Create `sessions/` package
  - [x] `session.go` - Session type with conversation history
  - [x] `store.go` - Session store with KVS backend
  - [x] `store_test.go` - Unit tests
  - [x] `tools.go` - Session management tools for agents
  - [x] `tools_test.go` - Tool tests

### Context Package

- [x] **Context**: Create `context/` package
  - [x] `context.go` - Context engine for token management
  - [x] `window.go` - Sliding window implementation
  - [x] `tokens.go` - Token counting utilities
  - [x] `context_test.go` - Unit tests

### Hooks Package

- [x] **Hooks**: Create `hooks/` package
  - [x] `event.go` - Event types and data structures
  - [x] `handler.go` - Handler interface
  - [x] `hook.go` - Hook interface
  - [x] `registry.go` - Hook/handler registration
  - [x] `dispatcher.go` - Event dispatching
  - [x] `webhook.go` - Webhook-based hooks
  - [x] Unit tests for all components

### Cron Package

- [x] **Cron**: Create `cron/` package
  - [x] `scheduler.go` - Cron scheduler with robfig/cron
  - [x] `job.go` - Job types (cron, interval, once)
  - [x] `store.go` - Job persistence with KVS
  - [x] `tools.go` - Cron tools for agent
  - [x] Unit tests for all components

---

## Phase 4: Remote Skills (Deferred)

> Moved to post-v0.6.0 work. See top-level TASKS.md.

---

## Phase 5: OmniChat Integration (Deferred)

> Moved to post-v0.6.0 work. See top-level TASKS.md.

---

## Summary

**Started**: 2026-04-17
**Completed**: 2026-04-25
**Release**: v0.6.0

### Completed in v0.6.0

| Area | Status | Notes |
|------|--------|-------|
| Phase 1: Compiled Skills | ✅ Core complete | TASK-104 deferred |
| Phase 2: Storage | ✅ Complete | Migrated to omnistorage-core |
| Phase 3: Platform | ✅ Core complete | TASK-303/304/305/306/307 deferred |
| Sessions Package | ✅ Complete | Not in original plan |
| Context Package | ✅ Complete | Not in original plan |
| Hooks Package | ✅ Complete | Not in original plan |
| Cron Package | ✅ Complete | Not in original plan |

### Deferred to Future Releases

- **TASK-104**: Configuration file support for skills
- **TASK-206**: Configuration file support for storage
- **TASK-303**: Webhook server (part of OmniChat integration)
- **TASK-304**: Lightsail deployment helpers
- **TASK-305**: Lambda adapter
- **TASK-306**: AgentCore adapter
- **TASK-307**: WithPlatform option
- **Phase 4**: Remote Skills (MCP, OpenAPI)
- **Phase 5**: OmniChat Integration
