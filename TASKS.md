# Tasks: OmniAgent Extension Implementation

## Phase 1: Compiled Skill Interface

### Setup

- [ ] **TASK-100**: Create `skills/compiled/` package
  - Create `skills/compiled/skill.go` with `Skill` interface
  - Create `skills/compiled/tool.go` with `Tool` type
  - Create `skills/compiled/registry.go` for skill registration
  - Create `skills/compiled/parameter.go` for JSON Schema parameters

- [ ] **TASK-101**: Define core interfaces
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

- [ ] **TASK-102**: Integrate compiled skills with agent
  - Add `compiledSkills` field to `agent.Agent`
  - Register compiled skill tools with existing `ToolRegistry`
  - Call `Init()` on startup, `Close()` on shutdown

- [ ] **TASK-103**: Add agent options for compiled skills
  - `WithCompiledSkill(skill compiled.Skill) Option`
  - `WithCompiledSkills(skills ...compiled.Skill) Option`

- [ ] **TASK-104**: Add configuration support
  - List compiled skills in config (documentation only)
  - Skills are registered via code, config shows which are loaded

### Testing

- [ ] **TASK-105**: Create mock skill for testing
  - Implement `compiled.Skill` with calculator tools
  - Test tool registration and execution
  - Test storage injection

- [ ] **TASK-106**: Integration test with agent
  - Load compiled skill
  - Verify tools appear in LLM requests
  - Test tool execution flow

---

## Phase 2: Storage Interface

### Setup

- [ ] **TASK-200**: Create `storage/` package
  - Create `storage/storage.go` with `Storage` interface
  - Create `storage/document.go` with `Document` type
  - Create `storage/errors.go` with common errors

- [ ] **TASK-201**: Define storage interface
  ```go
  type Storage interface {
      Get(ctx context.Context, key string) ([]byte, error)
      Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
      Delete(ctx context.Context, key string) error
      Close() error
  }
  ```

### Implementations

- [ ] **TASK-202**: Implement memory storage
  - Create `storage/memory/memory.go`
  - Thread-safe with `sync.RWMutex`
  - TTL support with background cleanup
  - Unit tests

- [ ] **TASK-203**: Implement SQLite storage
  - Create `storage/sqlite/sqlite.go`
  - Auto-create tables on init
  - TTL support via `expires_at` column
  - Unit tests

- [ ] **TASK-204**: (Future) Implement DynamoDB storage
  - Create `storage/dynamodb/dynamodb.go`
  - Use AWS SDK v2
  - TTL via DynamoDB TTL feature

### Integration

- [ ] **TASK-205**: Add storage to agent
  - Add `storage` field to `agent.Agent`
  - Add `WithStorage(s storage.Storage) Option`
  - Inject storage into `StorageAware` skills

- [ ] **TASK-206**: Add storage configuration
  ```yaml
  storage:
    type: sqlite  # sqlite, memory, dynamodb
    path: /data/omniagent.db
  ```

---

## Phase 3: Platform Adapters

### Setup

- [ ] **TASK-300**: Create `platform/` package
  - Create `platform/platform.go` with `Platform` interface
  - Create `platform/config.go` with platform configuration

- [ ] **TASK-301**: Define platform interface
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

- [ ] **TASK-302**: Create `platform/standalone/`
  - Extract current gateway logic into standalone platform
  - Support HTTP webhooks for omnichat providers
  - Graceful shutdown handling

- [ ] **TASK-303**: Add webhook server
  - HTTP server for incoming webhooks
  - Route to appropriate omnichat provider
  - Configurable address and TLS

### Lightsail Deployment

- [ ] **TASK-304**: Create `platform/lightsail/` helpers
  - Systemd service template generation
  - Caddy configuration template
  - Deploy script helpers

### Future Platforms

- [ ] **TASK-305**: (Future) Lambda adapter
  - AWS Lambda handler wrapper
  - API Gateway event handling

- [ ] **TASK-306**: (Future) AgentCore adapter
  - Bedrock AgentCore integration
  - Action group mapping

### Integration

- [ ] **TASK-307**: Add platform to agent startup
  - Add `WithPlatform(p platform.Platform) Option`
  - Default to standalone platform
  - Platform-specific configuration

---

## Phase 4: Remote Skills (Future)

### MCP Client

- [ ] **TASK-400**: Create `skills/remote/mcp/`
  - MCP client implementation
  - Tool discovery from MCP server
  - Tool execution via MCP protocol

- [ ] **TASK-401**: MCP skill wrapper
  - Implement `compiled.Skill` interface
  - Lazy connection on first tool call
  - Reconnection handling

### OpenAPI Loader

- [ ] **TASK-402**: Create `skills/remote/openapi/`
  - Parse OpenAPI 3.x specs
  - Generate tools from operations
  - Handle authentication

- [ ] **TASK-403**: OpenAPI skill wrapper
  - Implement `compiled.Skill` interface
  - HTTP client for API calls
  - Response parsing

### Configuration

- [ ] **TASK-404**: Remote skill configuration
  ```yaml
  skills:
    remote:
      - name: github
        type: mcp
        command: npx -y @modelcontextprotocol/server-github
      - name: api
        type: openapi
        url: https://api.example.com/openapi.json
  ```

---

## Phase 5: OmniChat Integration

### Provider Wrapper

- [ ] **TASK-500**: Create `provider/` package
  - Wrap `omnichat.Provider` for agent use
  - Message routing from multiple providers
  - Unified message handling

- [ ] **TASK-501**: Provider message handler
  - Convert omnichat messages to agent format
  - Send agent responses back via provider
  - Handle media attachments

### Configuration

- [ ] **TASK-502**: OmniChat provider configuration
  ```yaml
  providers:
    - type: twilio-sms
      account_sid_env: TWILIO_ACCOUNT_SID
      phone_number_env: TWILIO_PHONE_NUMBER
  ```

- [ ] **TASK-503**: Provider factory
  - Create providers from configuration
  - Support twilio-go/omnichat
  - Extensible for other omnichat providers

### Webhook Handling

- [ ] **TASK-504**: Integrate with standalone platform
  - Mount provider webhooks on HTTP server
  - Route incoming messages to agent
  - Send responses back

---

## Current Progress

**Started**: 2026-04-17
**Phase**: 1 - Compiled Skill Interface
**Status**: Planning complete, implementation starting

### Completed Phases (Previous)

- Phase 1 (Skills): SKILL.md loading, OpenClaw compatibility
- Phase 2 (Sandbox): WASM + Docker isolation

### Active Tasks

Starting with TASK-100 through TASK-106 (Compiled Skill Interface)

### Dependencies

| Package | Status | Notes |
|---------|--------|-------|
| `twilio-go` | Ready | omnichat provider available |

### Next Actions

1. Create `skills/compiled/` package (TASK-100, TASK-101)
2. Integrate with agent (TASK-102, TASK-103)
3. Create mock skill and tests (TASK-105, TASK-106)
4. Move to storage interface (Phase 2)
