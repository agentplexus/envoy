# Tasks: OmniAgent Extension Implementation

> For completed v0.6.0 tasks, see `docs/releases/v0.6.0/TASKS.md`

## Completed Phases

| Phase | Description | Status | Release |
|-------|-------------|--------|---------|
| Phase 1 | Compiled Skill Interface | ✅ Complete | v0.6.0 |
| Phase 2 | Storage Interface | ✅ Complete | v0.6.0 |
| Phase 3 | Platform Adapters | ✅ Complete | v0.6.0 |
| Phase 4 | Remote Skills | ✅ Complete | v0.7.0 |
| Phase 5 | OmniChat Integration | ✅ Complete | v0.6.0 |

Additional v0.6.0 packages: `sessions/`, `context/`, `hooks/`, `cron/`, `gateway/`, `voice/`

---

## Phase 4: Remote Skills ✅

### MCP Client

- [x] **TASK-400**: Create `skills/remote/mcp/`
  - MCP client implementation via `omniskill/mcp/client/`
  - Tool discovery from MCP server
  - Tool execution via MCP protocol

- [x] **TASK-401**: MCP skill wrapper
  - Implement `compiled.Skill` interface
  - Lazy connection on first tool call (`LazyConnect: true`)
  - `WithMCPSkill()` convenience option in `agent/options.go`

### OpenAPI Loader

- [x] **TASK-402**: Create `skills/remote/openapi/`
  - Parse OpenAPI 3.x specs via kin-openapi
  - Generate tools from operations
  - Handle API key, Bearer, Basic authentication

- [x] **TASK-403**: OpenAPI skill wrapper
  - Implement `compiled.Skill` interface
  - HTTP client for API calls with auth
  - Response parsing (JSON/text)
  - `WithOpenAPISkill()` convenience option

### Configuration

- [x] **TASK-404**: Remote skill configuration
  - `WithOpenAPISkill()` in `agent/options.go`
  - Supports filtering by operation ID, tags
  - Supports include/exclude operations

---

## Phase 5: OmniChat Integration ✅

> Already implemented in v0.6.0 via `omnichat` v0.6.0 dependency.

### Provider Wrapper

- [x] **TASK-500**: Provider integration via `omnichat`
  - Uses `omnichat/provider.Router` for message routing
  - Supports Telegram, Discord, WhatsApp, Twilio SMS
  - Unified message handling via router

- [x] **TASK-501**: Provider message handler
  - `router.OnMessage()` for message callbacks
  - `router.ProcessWithAgent()` for AI processing
  - `router.ProcessWithVoice()` for voice integration

### Configuration

- [x] **TASK-502**: OmniChat provider configuration
  - `config/config.go` defines `ChannelsConfig`
  - Supports Telegram, Discord, WhatsApp, TwilioSMS configs
  - Environment variable support for tokens

- [x] **TASK-503**: Provider factory
  - `cmd/omniagent/commands/gateway.go` initializes providers
  - Creates providers from configuration
  - Supports all omnichat providers

### Webhook Handling

- [x] **TASK-504**: Gateway integration
  - `gateway/gateway.go` mounts webhook handlers
  - Twilio webhook at `/webhook/twilio/sms`
  - Routes incoming messages to agent

### Implementation Details

**Supported Channels:**

| Channel | Provider Package | Config Type |
|---------|-----------------|-------------|
| Telegram | `omnichat/providers/telegram` | `TelegramConfig` |
| Discord | `omnichat/providers/discord` | `DiscordConfig` |
| WhatsApp | `omnichat/providers/whatsapp` | `WhatsAppConfig` |
| Twilio SMS | `omnichat/providers/twilio` | `TwilioSMSConfig` |

**Key Files:**

- `config/config.go` - Channel configuration structs
- `cmd/omniagent/commands/gateway.go` - Provider initialization
- `gateway/gateway.go` - WebSocket control plane, webhook mounting
- `voice/processor.go` - Voice (STT/TTS) integration

---

## Current Progress

**Last Release**: v0.7.0 (2026-04-27)
**Status**: Phase 4 & 5 Complete

### Dependencies

| Package | Status | Notes |
|---------|--------|-------|
| `omnichat` | ✅ v0.6.0 | Provider abstraction for messaging channels |
| `omniskill` | ✅ v0.7.0 | Skill interface and MCP client |
| `kin-openapi` | ✅ v0.137.0 | OpenAPI 3.x spec parsing |
| MCP SDK | ✅ v1.5.0 | Model Context Protocol |

### Next Actions

1. ~~Create `skills/remote/openapi/` package (TASK-402, TASK-403)~~ ✅
2. ~~Add remote skill configuration support (TASK-404)~~ ✅
3. ~~OmniChat integration (Phase 5)~~ ✅ (Already implemented)
4. Skills Bundle System (see `docs/design/skills/TASKS.md`)
