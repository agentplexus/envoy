# OmniAgent v0.9.0 Roadmap

> **Status**: In Progress
> **Target**: TBD

This document tracks the v0.9.0 release planning, incorporating feature parity analysis from OpenClaw.

## OpenClaw Sync Analysis

### Commit Range

| Field | Value |
|-------|-------|
| Previous Sync | `d4eb23652362a1b7d3fbcebd633a1c6f2a43c16f` (2026-04-22) |
| Current HEAD | `03125c8e132db59152c1b7b512e2a98f001aa26b` |
| Total Commits | 17,951 |
| Files Changed | 93,074 |
| Lines | +2,740,570 / -1,435,397 |

### Commit Summary by Category

| Category | Commits | % | Notes |
|----------|---------|---|-------|
| Tests | 7,333 | 40.8% | Heavy testing focus |
| Fixed | 6,094 | 33.9% | Bug fixes |
| Documentation | 1,285 | 7.2% | Docs updates |
| Changed | 922 | 5.1% | Modifications |
| **Added** | **668** | **3.7%** | New features |
| Infrastructure | 614 | 3.4% | CI/CD |
| Internal | 510 | 2.8% | Refactors |
| Performance | 311 | 1.7% | Optimizations |
| Build | 91 | 0.5% | Build changes |
| Security | 10 | 0.1% | Security fixes |

---

## Dependency Status

Features already implemented in plexusone dependencies:

| Feature | Library | Status | Notes |
|---------|---------|--------|-------|
| Tool plugin helpers | omniskill | ✅ Done | `NewTool`, `FuncTool`, `AddTool[In,Out]()` |
| Session workflow APIs | omniskill | ✅ Done | MCP server handlers, session management |
| Telnyx voice-call | omni-telnyx | ✅ Done | CallSystem, Media Streaming, SMS (v0.2.0) |
| STT/TTS providers | omnivoice | ✅ Done | OpenAI, Deepgram, ElevenLabs, Twilio |
| Slack thread support | omnichat | ✅ Done | `ThreadTimeStamp` in Slack provider |
| WhatsApp voice notes | omnichat | ✅ Done | Audio upload/download, PTT detection |
| LLM tracing | omniobserve | ✅ Done | Langfuse, Opik, Phoenix, slog |
| OpenTelemetry | omniobserve | ✅ Done | OTLP, metrics, traces, logs |

---

## Feature Parity Matrix (Updated)

| Category | OpenClaw | OmniAgent | Gap | v0.9.0 Focus |
|----------|----------|-----------|-----|--------------|
| **Channels** | 24+ | 7 (omnichat) | 17+ missing | Slack unfurl, Telegram buttons |
| **LLM Providers** | 50+ | 4+ (omnillm) | Most covered | Embedding provider |
| **Tools** | 10+ categories | 4 | 6+ categories | Browser, tool policies |
| **Skills** | 55+ bundled | SKILL.md + compiled | Feature parity | Git/local installs |
| **Voice** | Wake word, calls | Telnyx, Deepgram | Wake word | ✅ Telnyx done |
| **Sandbox** | Docker, SSH | WASM, Docker | SSH backend | GPU passthrough |
| **Observability** | OpenTelemetry | Full (omniobserve) | ✅ Covered | Gateway integration |

---

## Phases

### Phase 1: Embedding Provider (omnillm)

**Goal**: Add embedding provider interface to omnillm for vector/semantic search.

**Library**: `~/go/src/github.com/plexusone/omnillm-core`

- [x] **Embedding Provider Interface** - Define `EmbeddingProvider` interface ✅
  - Reference: `feat(plugins): add embedding provider contract (#84947)`
  - Implemented: `provider/interface.go` - `EmbeddingProvider` interface
  - Implemented: `provider/types.go` - `EmbeddingRequest`, `EmbeddingResponse`, `EmbeddingData`, `EmbeddingUsage`

- [x] **OpenAI Embedding Provider** - text-embedding-3-small/large ✅
  - Files: `providers/openai/adapter.go` - `EmbeddingProvider` struct
  - Tests: `providers/openai/embedding_test.go`

- [ ] **Voyage AI Provider** - voyage-3, voyage-code-3 (optional)
  - Files: `providers/voyage/`

- [x] **Provider Registry** - `GetEmbeddingProvider(name string)` ✅
  - Files: `registry.go` - `RegisterEmbeddingProvider`, `GetEmbeddingProviderFactory`, `ListEmbeddingProviders`, `GetEmbeddingProvider`
  - Tests: `embedding_test.go`

### Phase 2: Channel Enhancements (omnichat)

**Goal**: Improve channel integrations based on OpenClaw features.

**Library**: `~/go/src/github.com/plexusone/omnichat`

- [x] **Slack Thread Support** - ✅ Already implemented via `ThreadTimeStamp`

- [x] **Slack Unfurl Controls** - Link preview management ✅
  - Files: `providers/slack/adapter.go` - `MetaUnfurlLinks`, `MetaUnfurlMedia` metadata keys
  - Methods: `getUnfurlLinks()`, `getUnfurlMedia()` for granular control

- [x] **Slack Reply Broadcasts** - Broadcast replies to channel ✅
  - Files: `providers/slack/adapter.go` - `MetaReplyBroadcast` metadata key
  - Uses `MsgOptionBroadcast()` when `ReplyTo` is set with broadcast flag

- [x] **Telegram Web App Buttons** - Presentation buttons ✅
  - Files: `providers/telegram/adapter.go`
  - Types: `InlineButton` with `WebAppURL` field
  - Metadata: `MetaInlineKeyboard` for inline keyboard rows
  - Handlers: `OnCallback()`, `OnWebAppData()` for button interactions

- [x] **Telegram Localized Commands** - i18n command menu ✅
  - Files: `providers/telegram/adapter.go`
  - Types: `Command`, `LocalizedCommands` (map[langCode][]Command)
  - Methods: `SetCommands()`, `SetLocalizedCommands()`, `DeleteCommands()`

- [x] **WhatsApp Newsletter Targets** - Newsletter support ✅
  - Files: `providers/whatsapp/adapter.go`
  - Types: `Newsletter`, `ChatTypeNewsletter`
  - Methods: `GetNewsletters()`, `FollowNewsletter()`, `UnfollowNewsletter()`
  - Events: `newsletter_join`, `newsletter_leave`

- [x] **WhatsApp Status Reactions** - Emoji reactions ✅
  - Files: `providers/whatsapp/adapter.go`
  - Types: `ChatTypeStatus`
  - Methods: `SendReaction()`, `RemoveReaction()`
  - Events: Reaction events via `EventTypeReaction`
  - Added: Image, video, document media handling

### Phase 3: Voice & Realtime

**Goal**: Expand voice capabilities with realtime features.

- [x] **Telnyx Voice-Call** - ✅ Already implemented in omni-telnyx v0.2.0
  - CallSystem, Media Streaming, SMS fully supported

**Library**: `~/go/src/github.com/plexusone/omnichat`

- [x] **Discord Realtime Voice** - Voice channel integration ✅
  - Reference: `feat(discord): add realtime voice bootstrap context`
  - Reference: `feat(discord): follow configured users in voice`
  - Files: `providers/discord/voice.go`
  - Features: Join/leave channels, send/receive audio, voice state tracking

- [x] **Discord Voice Channel Allowlist** - Access control ✅
  - Reference: `feat(discord): add voice channel allowlist`
  - Files: `providers/discord/voice.go` - `VoiceConfig.ChannelAllowlist`, `ChannelBlocklist`

- [x] **Discord Auto-Follow Users** - Follow users into voice ✅
  - Files: `providers/discord/voice.go` - `VoiceConfig.FollowUsers`
  - Auto-joins when configured users join voice channels

### Phase 4: Tools & Sandbox (omniagent)

**Goal**: Enhance tool execution and sandbox capabilities.

**Library**: `~/go/src/github.com/plexusone/omniagent`

- [x] **Per-Sender Tool Policies** - User-level tool restrictions ✅
  - Reference: `feat(tools): add per-sender tool policies (#66933)`
  - Files: `tools/policy/policy.go`, `tools/policy/registry.go`
  - Features: Allow/deny lists, rate limiting, concurrent execution limits

- [x] **Browser Evaluate Timeout** - JavaScript evaluation with timeout ✅
  - Reference: `feat(browser): add evaluate timeout CLI option (#83696)`
  - Files: `tools/browser/browser.go` - `evaluate` action

- [x] **Browser Dialog Surfacing** - Expose observed dialogs ✅
  - Reference: `feat(browser): surface observed dialogs (#83099)`
  - Files: `tools/browser/browser.go` - `Dialog` type, `get_dialogs`, `dismiss_dialog` actions
  - Features: Track alert/confirm/prompt dialogs, callback support

- [x] **Docker GPU Passthrough** - Sandbox GPU support ✅
  - Reference: `feat(sandbox): add Docker GPU passthrough`
  - Files: `sandbox/docker.go` - `GPUConfig`, `WithGPU()`, `WithAllGPUs()`
  - Features: NVIDIA GPU passthrough, device selection, capability configuration

### Phase 5: Skills & Agents (omniskill + omniagent)

**Goal**: Improve skill management and per-agent configuration.

- [x] **Tool Plugin Helpers** - ✅ Already in omniskill (`NewTool`, `FuncTool`)
- [x] **Session Workflow APIs** - ✅ Already in omniskill (MCP server)

**Library**: `~/go/src/github.com/plexusone/omniskill`

- [x] **Git Skill Installation** - Install skills from git repos ✅
  - Reference: `feat: support git and local skill installs (#84793)`
  - Files: `installer/source.go` - `SkillInstaller.InstallGit()`
  - Supports: refs (@v1.0.0), subdirs (repo/skills/weather)

- [x] **Local Skill Installation** - Install from local paths ✅
  - Files: `installer/source.go` - `SkillInstaller.InstallLocal()`
  - Supports: copy or symlink mode

- [x] **Global Skill Directory** - Shared skill installation ✅
  - Reference: `feat(cli): support installing skills to shared global directory via --global (#83705)`
  - Files: `installer/source.go` - `SkillInstaller.UseGlobal`, `DefaultGlobalDir()`
  - Default: `~/.omniskill/skills/`

- [x] **Unified Skill Loader** - Load Go and SKILL.md skills ✅
  - Files: `loader/loader.go` - `UnifiedLoader`, `SkillInfo`, `Inspect()`, `DiscoverSkills()`
  - Supports: SKILL.md (OpenClaw format), skill.go (Go implementation)
  - Priority: Go implementations over SKILL.md when both exist

**Library**: `~/go/src/github.com/plexusone/omniagent`

- [x] **Per-Agent Bootstrap Profiles** - Agent-specific initialization ✅
  - Reference: `feat(agents): support per-agent bootstrap profiles`
  - Files: `agent/profiles/bootstrap.go`
  - Features: BootstrapProfile, ProfileRegistry, tool filtering, system prompt customization
  - Predefined profiles: default, restricted, readonly, code_assistant

- [x] **Per-Agent Local Model Lean Mode** - Resource optimization ✅
  - Reference: `feat(agents): support per-agent local model lean mode (#84073)`
  - Files: `agent/profiles/lean.go`
  - Levels: off, light, moderate, aggressive
  - Predefined modes: LeanModeForOllama, LeanModeForLMStudio, LeanModeForLlamaCpp

- [x] **Tool Progress Detail Modes** - Configurable progress output ✅
  - Reference: `feat(agents): add tool progress detail modes`
  - Files: `agent/profiles/progress.go`
  - Modes: quiet, minimal, normal, verbose, debug
  - Features: ProgressReporter, callbacks, parameter redaction, result truncation

### Phase 6: Observability Integration (omniagent)

**Goal**: Integrate omniobserve with omniagent gateway.

**Library**: `~/go/src/github.com/plexusone/omniagent`

- [x] **Gateway Trace Instrumentation** - Trace support with observops ✅
  - Reference: `feat(gateway): add trace instrumentation`
  - Files: `gateway/observability.go`, `gateway/observability_test.go`
  - Integration: Uses omniobserve/observops for metrics and traces
  - Features: StartTrace/EndTrace, RecordClientConnect, RecordToolInvocation, workflow tracking

- [x] **Gateway SDK Tools RPC** - SDK-facing tools.invoke ✅
  - Reference: `feat(gateway): add SDK-facing tools.invoke RPC`
  - Files: `gateway/tools_rpc.go`, `gateway/tools_rpc_test.go`
  - Features: ToolsRPCHandler for /tools/invoke, ToolsListHandler for /tools/list
  - Integration: Uses agentops middleware for tool invocation tracking

- [x] **Channel Conformance Checks** - Policy enforcement ✅
  - Reference: `feat(policy): add channel conformance checks (#80407)`
  - Files: `channels/policy/conformance.go`, `channels/policy/conformance_test.go`
  - Features: ConformanceChecker, configurable rules, channel filtering
  - Predefined rules: NoEmptyContent, ValidSender, MaxLength, ContentPattern, RateLimit

---

## Implementation Priority

| Priority | Phase | Library | Rationale |
|----------|-------|---------|-----------|
| P0 | Phase 1 (Embedding) | omnillm | Foundation for RAG/search |
| P1 | Phase 4 (Tools) | omniagent | Core functionality |
| P1 | Phase 5 (Skills) | omniskill | User-requested git installs |
| P2 | Phase 2 (Channels) | omnichat | Incremental improvements |
| P2 | Phase 3 (Voice) | omnichat | Discord voice |
| P3 | Phase 6 (Observability) | omniagent | Operational improvements |

---

## Dependencies Between Libraries

```
omnillm (embedding)
    ↓
omniagent (agent, tools, gateway)
    ↓
omniskill (skill installation)
    ↓
omnichat (channel enhancements)
    ↓
omniobserve (tracing integration)
```

---

## Out of Scope (v0.9.0)

- **Android/iOS Apps** - Native apps (47+ commits)
- **Google Meet Integration** - Significant new channel work
- **Canvas/A2UI** - Visual workspace system
- **iMessage** - Apple-only, limited audience
- **Wake Word** - Voice activation (deferred)

---

## Completion Tracking

### Phase 1: Embedding Provider (omnillm-core)

- [x] Embedding provider interface ✅
- [x] OpenAI embedding provider ✅
- [ ] Voyage AI provider (optional)
- [x] Provider registry ✅

### Phase 2: Channel Enhancements (omnichat)

- [x] Slack thread support (already done)
- [x] Slack unfurl controls ✅
- [x] Slack reply broadcasts ✅
- [x] Telegram web app buttons ✅
- [x] Telegram localized commands ✅
- [x] WhatsApp newsletter targets ✅
- [x] WhatsApp status reactions ✅

### Phase 3: Voice & Realtime

- [x] Telnyx voice-call (already done in omni-telnyx)
- [x] Discord realtime voice ✅
- [x] Discord voice channel allowlist ✅
- [x] Discord auto-follow users ✅

### Phase 4: Tools & Sandbox (omniagent)

- [x] Per-sender tool policies ✅
- [x] Browser evaluate timeout ✅
- [x] Browser dialog surfacing ✅
- [x] Docker GPU passthrough ✅

### Phase 5: Skills & Agents

- [x] Tool plugin helpers (already in omniskill)
- [x] Session workflow APIs (already in omniskill)
- [x] Git skill installation (omniskill) ✅
- [x] Local skill installation (omniskill) ✅
- [x] Global skill directory (omniskill) ✅
- [x] Unified skill loader (omniskill) ✅
- [x] Per-agent bootstrap profiles (omniagent) ✅
- [x] Per-agent local model lean mode (omniagent) ✅
- [x] Tool progress detail modes (omniagent) ✅

### Phase 6: Observability (omniagent)

- [x] Gateway trace instrumentation ✅
- [x] Gateway SDK tools RPC ✅
- [x] Channel conformance checks ✅

---

## Summary

**Already Complete** (from dependencies):

- ✅ Telnyx voice-call integration (omni-telnyx v0.2.0)
- ✅ Tool plugin helpers (omniskill)
- ✅ Session workflow APIs (omniskill)
- ✅ Slack thread support (omnichat)
- ✅ OpenTelemetry observability (omniobserve)

**Completed in v0.9.0**:

- ✅ Embedding provider interface (omnillm-core)
- ✅ OpenAI embedding provider (omnillm-core)
- ✅ Embedding provider registry (omnillm-core)
- ✅ Slack unfurl controls (omnichat)
- ✅ Slack reply broadcasts (omnichat)
- ✅ Telegram web app buttons (omnichat)
- ✅ Telegram localized commands (omnichat)
- ✅ WhatsApp newsletter targets (omnichat)
- ✅ WhatsApp status reactions (omnichat)
- ✅ Git skill installation (omniskill)
- ✅ Local skill installation (omniskill)
- ✅ Global skill directory (omniskill)
- ✅ Unified skill loader - Go + SKILL.md (omniskill)
- ✅ Discord realtime voice (omnichat)
- ✅ Discord voice channel allowlist (omnichat)
- ✅ Discord auto-follow users (omnichat)
- ✅ Per-sender tool policies (omniagent)
- ✅ Browser evaluate with timeout (omniagent)
- ✅ Browser dialog surfacing (omniagent)
- ✅ Docker GPU passthrough (omniagent)
- ✅ Per-agent bootstrap profiles (omniagent)
- ✅ Per-agent local model lean mode (omniagent)
- ✅ Tool progress detail modes (omniagent)
- ✅ Gateway trace instrumentation (omniagent)
- ✅ Gateway SDK tools RPC (omniagent)
- ✅ Channel conformance checks (omniagent)

**Remaining Work**:

| Library | Tasks |
|---------|-------|
| omnillm-core | 1 (Voyage AI provider - optional) |
| omnichat | 0 ✅ |
| omniskill | 0 ✅ |
| omniagent | 0 ✅ |

---

## References

- [OpenClaw Repository](https://github.com/openclaw/openclaw)
- Previous sync: `d4eb23652362a1b7d3fbcebd633a1c6f2a43c16f`
- Current sync: `03125c8e132db59152c1b7b512e2a98f001aa26b`
