# Roadmap

This document outlines planned features and improvements for OmniAgent.

## v0.2.0 - Authentication & Security

- [ ] Implement proper WebSocket authentication (`gateway/handlers.go`)
- [ ] Add origin checking for WebSocket connections (`gateway/gateway.go`)
- [ ] Add API key validation for gateway access
- [ ] Add rate limiting for message processing

## v0.3.0 - Channel Improvements

- [ ] Handle reply_to for Telegram messages (`channels/adapters/telegram/telegram.go`)
- [ ] Add Slack adapter
- [ ] Add WhatsApp adapter (via WhatsApp Business API)
- [ ] Add channel-specific message formatting

## v0.4.0 - Agent Enhancements

- [ ] Implement memory-aware processing using omnillm memory features (`agent/agent.go`)
- [ ] Add conversation summarization for long sessions
- [ ] Add persistent session storage (SQLite/PostgreSQL)
- [ ] Add tool result caching

## v0.5.0 - Observability & Monitoring

- [ ] Integrate omniobserve for LLM tracing
- [ ] Add Prometheus metrics endpoint
- [ ] Add structured logging with log levels
- [ ] Add health check endpoints with detailed status

## v0.3.0 - Skill System (Phase 1)

- [ ] Implement skill loader (SKILL.md parsing)
- [ ] Skill discovery from configurable directories
- [ ] Requirement checking (bins, env vars)
- [ ] Prompt injection for loaded skills
- [ ] CLI commands: `skills list`, `skills info`, `skills check`
- [ ] ClawHub/OpenClaw skill compatibility

## v0.4.0 - Hook Runner (Phase 2)

- [ ] Deno-based TypeScript hook execution
- [ ] Shell script hook execution
- [ ] OpenClaw hook API compatibility layer
- [ ] Permission restrictions per skill

## v0.5.0 - Tool Sandbox (Phase 3)

- [ ] WASM runtime via wazero
- [ ] Capability-based permissions
- [ ] Memory and CPU limits
- [ ] Built-in tools in WASM sandbox

## v0.11.0 - OmniMemory Integration (Memory Architecture Overhaul)

**Goal:** Replace omnistorage-based session memory with domain-specific omnimemory, providing semantic search, multi-tenancy, and structured memory types.

### Phase 1: Add KVS Provider to OmniMemory

Create `provider/kvs` in omnimemory that wraps `omnistorage-core/kvs`:

- [ ] Implement `provider/kvs/kvs.go` - KVS-backed memory provider
- [ ] Support SQLite, Redis, Memory backends via omnistorage
- [ ] In-memory similarity search for semantic queries
- [ ] Unit tests for KVS provider

### Phase 2: Migrate Session Storage

Replace `sessions.Store` (omnistorage wrapper) with omnimemory client:

- [ ] Add omnimemory client to `agent.Agent` struct
- [ ] Update `sessions/store.go` to use omnimemory
- [ ] Store conversation turns as `MemoryTypeObservation`
- [ ] Add `SessionID` to omnimemory Context struct
- [ ] Migrate existing session data format

### Phase 3: Add Memory Recall to Agent Processing

Inject relevant memories into conversation context:

- [ ] Update `Agent.ProcessWithSession()` to call `omnimemory.Recall()`
- [ ] Add configuration for recall behavior (max results, threshold)
- [ ] Format recalled memories for system prompt injection
- [ ] Add memory extraction from conversation responses

### Phase 4: Update Memory Skill

Replace `omniretrieve`-based memory skill with omnimemory:

- [ ] Update `skills/memory/memory.go` to use omnimemory
- [ ] Map existing tools to omnimemory operations
- [ ] Maintain backward compatibility for existing users
- [ ] Add new tools for memory types (facts, traits, preferences)

### Phase 5: Configuration & Documentation

- [ ] Add omnimemory provider configuration to `config/config.go`
- [ ] Update configuration examples (YAML)
- [ ] Document migration path from omnistorage
- [ ] Update API documentation for memory endpoints

### Architecture

```
omniagent (v0.11.0+)
    │
    ├── omnimemory/              ← All memory operations
    │   ├── provider/postgres    (production - pgvector)
    │   ├── provider/memory      (testing)
    │   ├── provider/kvs         (wraps omnistorage-core/kvs)
    │   └── provider/twilio      (via omni-twilio)
    │
    └── omnistorage/             ← Non-memory storage only
        ├── object/              (files, blobs, backups)
        └── kvs/                 (agent registry, configs)
```

### Breaking Changes

| Change | Migration |
|--------|-----------|
| `sessions.Store` removed | Use `omnimemory.Client` |
| Session JSON format | Auto-migrate to `Memory` objects |
| `omniretrieve` dependency | Replaced by omnimemory |

### Related PRs

- omnimemory: Add `provider/kvs` backend
- omnimemory: Add `SessionID` to Context
- omniagent: Integrate omnimemory for sessions
- omniagent: Update memory skill

## Future

- [ ] Multi-tenant support
- [ ] Web UI for configuration and monitoring
- [ ] Voice channel support via omnivoice
- [ ] Integration with omnichat for unified channel abstraction
- [ ] Integration with omnibrowser for enhanced browser automation

## Related Projects

| Project | Status | Purpose |
|---------|--------|---------|
| [omnillm](https://github.com/plexusone/omnillm) | Active | Multi-provider LLM abstraction |
| [omnimemory](https://github.com/plexusone/omnimemory) | Active | Vendor-neutral memory abstraction |
| [omni-twilio](https://github.com/plexusone/omni-twilio) | Active | Twilio integrations (Memory API) |
| [omniobserve](https://github.com/plexusone/omniobserve) | Active | LLM observability |
| [omnistorage](https://github.com/plexusone/omnistorage) | Active | Generic file/KV storage |
| [omnichat](https://github.com/plexusone/omnichat) | Planned | Channel abstraction |
| [omnibrowser](https://github.com/plexusone/omnibrowser) | Planned | Browser abstraction |
| [omnivoice](https://github.com/plexusone/omnivoice) | Active | Voice interactions |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to propose features or submit pull requests.
