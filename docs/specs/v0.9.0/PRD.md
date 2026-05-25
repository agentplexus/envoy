# OmniAgent v0.9.0 Product Requirements

> **Status**: Draft
> **Version**: 0.9.0

## Overview

OmniAgent v0.9.0 focuses on plugin architecture, channel enhancements, and voice capabilities based on feature parity analysis with OpenClaw (17,951 commits since last sync).

## Goals

1. **Extensibility** - Establish Go-native plugin patterns for embeddings and tools
2. **Channel Maturity** - Improve Slack, Telegram, WhatsApp integrations
3. **Voice Expansion** - Add Telnyx voice-call support
4. **Tool Policies** - Per-sender tool restrictions for multi-user deployments
5. **Skill Management** - Git-based skill installation

## Non-Goals

- Native mobile apps (Android/iOS)
- Canvas/A2UI visual workspace
- Google Meet integration
- iMessage channel

## User Stories

### Plugin Architecture

**US-1**: As a developer, I want to register custom embedding providers so that I can use specialized embedding models.

**US-2**: As a developer, I want simple helpers for creating tool plugins so that I can extend agent capabilities without boilerplate.

### Channel Enhancements

**US-3**: As a Slack user, I want the agent to manage conversation threads properly so that responses stay organized.

**US-4**: As a Telegram user, I want to interact with web app buttons so that I can use rich UI interactions.

**US-5**: As a WhatsApp user, I want the agent to post to newsletters so that I can automate content distribution.

### Voice & Realtime

**US-6**: As a voice user, I want Telnyx integration for phone calls so that I can use voice agents over PSTN.

**US-7**: As a Discord user, I want the agent to join voice channels and follow configured users so that I can have voice conversations.

### Tools & Sandbox

**US-8**: As an admin, I want to set tool policies per user so that I can restrict dangerous tools for certain users.

**US-9**: As a developer, I want GPU passthrough in Docker sandbox so that I can run ML models in isolated environments.

**US-10**: As a user, I want browser evaluate timeouts so that long-running scripts don't hang my session.

### Skills & Agents

**US-11**: As a user, I want to install skills from git repositories so that I can use community skills easily.

**US-12**: As an admin, I want per-agent bootstrap profiles so that different agents can have different initialization behaviors.

## Requirements

### Functional Requirements

#### FR-1: Embedding Provider Contract

- Define `EmbeddingProvider` interface in `plugins/embedding/`
- Support registration of custom embedding providers
- Integrate with omnillm embedding interface

#### FR-2: Tool Plugin Helpers

- Provide `NewTool()` helper for simple tool creation
- Support tool metadata, input schema, and handler registration
- Include examples for common patterns

#### FR-3: Slack Thread Lifecycle

- Track assistant thread state across messages
- Support reply broadcasts to channels
- Implement unfurl controls for link previews

#### FR-4: Telegram Web App Buttons

- Support presentation buttons in messages
- Handle web app callbacks
- Localized command menu descriptions

#### FR-5: WhatsApp Newsletter Targets

- Extend message tool to support newsletter targets
- Support status reactions and emoji categories

#### FR-6: Telnyx Voice Integration

- Implement Telnyx media streaming provider
- Support voice-call realtime transcription
- Integrate with omnivoice abstraction

#### FR-7: Discord Voice Features

- Realtime voice bootstrap context
- Follow configured users in voice channels
- Voice channel allowlist for access control

#### FR-8: Per-Sender Tool Policies

- Define policy rules per sender/user
- Support allow/deny lists for tools
- Integrate with existing policy engine

#### FR-9: Docker GPU Passthrough

- Detect available GPUs
- Configure GPU device mapping in containers
- Support NVIDIA runtime configuration

#### FR-10: Git Skill Installation

- Support `skill install <git-url>` command
- Support `--global` flag for shared installation
- Handle skill versioning and updates

#### FR-11: Per-Agent Bootstrap Profiles

- Define bootstrap profile configuration
- Support per-agent initialization hooks
- Include lean mode for resource-constrained environments

### Non-Functional Requirements

#### NFR-1: Backward Compatibility

- Existing configurations must continue to work
- New features are opt-in via configuration

#### NFR-2: Documentation

- All new features documented in README
- API documentation for plugin interfaces

#### NFR-3: Testing

- Unit tests for all new functionality
- Integration tests for channel features

## Success Metrics

| Metric | Target |
|--------|--------|
| OpenClaw feature parity | +15 features |
| Test coverage (new code) | >80% |
| Documentation coverage | 100% of public APIs |

## Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| omnillm | latest | Embedding interface |
| omnivoice | latest | Telnyx provider |
| omnivault | latest | Credential management |

## Timeline

| Phase | Features | Status |
|-------|----------|--------|
| Phase 1 | Plugin Architecture | Planned |
| Phase 2 | Channel Enhancements | Planned |
| Phase 3 | Voice & Realtime | Planned |
| Phase 4 | Tools & Sandbox | Planned |
| Phase 5 | Skills & Agents | Planned |
| Phase 6 | Observability | Planned |

## Open Questions

1. Should embedding providers be registered at startup or dynamically?
2. Should tool policies be stored in config or separate policy files?
3. Should git skill installation support private repos with SSH keys?

## Appendix

### OpenClaw Sync Details

- **Commits analyzed**: 17,951
- **Previous sync**: `d4eb236` (2026-04-22)
- **Current sync**: `03125c8` (2026-05-21)
- **Key areas**: plugins (45), browser (12), telegram (11), agents (11)

See [ROADMAP.md](ROADMAP.md) for detailed sync analysis.
