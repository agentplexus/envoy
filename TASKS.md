# Tasks: OmniAgent Extension Implementation

> For completed v0.6.0 tasks, see `docs/releases/v0.6.0/TASKS.md`

## Completed Phases

| Phase | Description | Status | Release |
|-------|-------------|--------|---------|
| Phase 1 | Compiled Skill Interface | ✅ Complete | v0.6.0 |
| Phase 2 | Storage Interface | ✅ Complete | v0.6.0 |
| Phase 3 | Platform Adapters | ✅ Complete | v0.6.0 |

Additional v0.6.0 packages: `sessions/`, `context/`, `hooks/`, `cron/`

---

## Phase 4: Remote Skills

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

**Last Release**: v0.6.0 (2026-04-25)
**Next Phase**: Phase 4 - Remote Skills

### Dependencies

| Package | Status | Notes |
|---------|--------|-------|
| `twilio-go` | Ready | omnichat provider available |
| MCP SDK | TBD | Need Go MCP client |

### Next Actions

1. Evaluate Go MCP client options
2. Create `skills/remote/mcp/` package (TASK-400, TASK-401)
3. Create `skills/remote/openapi/` package (TASK-402, TASK-403)
4. Move to OmniChat integration (Phase 5)
