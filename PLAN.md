# OmniAgent Extension Plan

> **Last Updated**: 2026-07-30
> **Current Release**: v0.9.0 - See [`docs/specs/v0.9.0/ROADMAP.md`](docs/specs/v0.9.0/ROADMAP.md)
> **Historical Plans**: [`docs/releases/v0.6.0/PLAN.md`](docs/releases/v0.6.0/PLAN.md)

## OpenClaw Feature Parity Tracking

This project tracks functionality from [OpenClaw](https://github.com/openclaw/openclaw) implemented with alternate approaches (Go, security-focused, etc.).

### Sync History

| Date | OpenClaw Commit | Commits | Notes |
|------|-----------------|---------|-------|
| 2026-07-30 | `3e7ecb3aeaaf3a9dd4fa8c46c31b63c6f2031ccc` | 11,280 | Realtime bounded reads, agent edit fixes, memory compaction |
| 2026-06-28 | `2001b15f5b92d653464cbd847c28c136bdb465a7` | 4,111 | Security hardening, UTF-16 safety, provider updates |
| 2026-06-10 | `bd96e4d22dafe64f558fb7f3ba5977aa3a93aee6` | 7,162+ | Feature parity implementation complete |
| 2026-05-21 | `03125c8e132db59152c1b7b512e2a98f001aa26b` | 17,951 | v0.9.0 sync analysis |
| 2026-04-22 | `d4eb23652362a1b7d3fbcebd633a1c6f2a43c16f` | - | Initial gap analysis |

### Latest Parity Implementation (2026-07-30)

See [`docs/specs/parity-2026-07-30/PLAN.md`](docs/specs/parity-2026-07-30/PLAN.md) for the detailed implementation plan.

**Focus Areas:**

| Priority | Items |
|----------|-------|
| P0 | Realtime bounded reads (SDP, audio frames, playback marks) |
| P1 | Gateway SSRF prevention, hook prompt injection fencing |
| P2 | Edit tool line ending fixes, agent fan-out stalls |
| P3 | Memory compaction note preservation, session rollover |
| P4 | Cron slot replay fixes, hook timezone handling |
| P5 | Per-session tool overrides, MCP tool identity exposure |

### Previous Parity Implementation (2026-06-28)

See [`docs/specs/parity-2026-06-28/PLAN.md`](docs/specs/parity-2026-06-28/PLAN.md) for details.

**Completed:**

| Module | Changes |
|--------|---------|
| `internal/httputil` | New - Bounded HTTP response reads (OOM prevention) |
| `internal/strutil` | New - UTF-16 safe string truncation |
| `sandbox` | Process tree cleanup on exec timeout |
| `api/openai/auth` | Bounded reads in OAuth providers |
| `hooks/webhook` | Bounded response draining |
| `skills/remote/openapi` | Bounded API response reads |
| `skills/github` | New - GitHub skill (issues, PRs, code search) |
| `cmd/omniagent/commands/doctor` | New - Diagnostic check command |
| `cmd/omniagent/commands/sessions` | New - Session management CLI |
| `cmd/omniagent/commands/setup` | New - Interactive setup wizard |

**Pending (see plan for details):**

| Priority | Items |
|----------|-------|
| P1 | Compaction summaries, memory search modes |
| P2 | Channel UTF-16 truncation integration |
| P3 | Plugin auto-approval modes |
| P4 | Cohere provider, GLM-5.2, Bedrock streaming fixes |
| P5 | CLI features (--message-file) |

### Previous Implementation (2026-06-10)

| Module | Changes |
|--------|---------|
| `omni-openrouter` | New - OpenRouter provider with OAuth PKCE |
| `omnillm-core` | Added Claude 4.x/4.5 Bedrock & Vertex models |
| `omniserp` | Added Brave and Exa.ai search engines |
| `omniskill` | Added ClawHub marketplace integration |
| `omniretrieve` | Added BM25 index, memory manager |
| `omniworkboard` | New - Project management workboard |
| `omniagent` | Added memory skill, auto-reply/commentary, integrated providers |

### Feature Parity Matrix

| Category | OpenClaw | OmniAgent | Gap | v0.9.0 |
|----------|----------|-----------|-----|--------|
| **Channels** | 24+ | 4 | 20+ missing | Slack, Telegram, WhatsApp improvements |
| **LLM Providers** | 50+ | 4+ (omnillm) | Most covered | - |
| **Tools** | 10+ categories | 4 | 6+ categories | Browser, tool policies |
| **Skills** | 55+ bundled | SKILL.md + compiled | Feature parity | Git/local installs, GitHub skill |
| **Scheduling** | Cron, webhooks | Cron, interval | Feature parity | - |
| **Voice** | Wake word, calls | Deepgram (omnivoice) | Wake word, calls | Telnyx, Discord voice |
| **Sandbox** | Docker, SSH | WASM, Docker | SSH backend | GPU passthrough |
| **Plugins** | 117+ (JS) | N/A (Go) | Different arch | Embedding provider |
| **Apps** | macOS, iOS, Android | None | All 3 | Out of scope |
| **CLI Commands** | 50+ | ~14 | 36+ missing | doctor, setup, sessions |
| **Memory/RAG** | LanceDB, Wiki | omniretrieve | Feature parity | - |
| **Visual** | Canvas/A2UI | None | Canvas system | Out of scope |

---

## Release Planning

### v0.9.0 (Current)

See [`docs/specs/v0.9.0/ROADMAP.md`](docs/specs/v0.9.0/ROADMAP.md) for detailed planning.

**Focus Areas**:

- Plugin architecture (embedding providers, tool helpers)
- Channel enhancements (Slack threads, Telegram buttons, WhatsApp newsletters)
- Voice expansion (Telnyx, Discord realtime voice)
- Tool policies (per-sender restrictions)
- Skill management (git installs, global directory)

### v0.8.0 (Completed)

- Vault-backed credentials (omnivault, omnitoken integration)
- OAuth token management with refresh
- See [`docs/specs/v0.8.0/ROADMAP.md`](docs/specs/v0.8.0/ROADMAP.md) for implementation details

### v0.7.0 (Completed)

- See [`docs/releases/v0.7.0.md`](docs/releases/v0.7.0.md)

### v0.6.0 (Completed)

- Session Management, Cron/Scheduling, Context Engine, Hooks System
- Compiled Skills, Platform Adapters, Storage Migration
- See [`docs/releases/v0.6.0/PLAN.md`](docs/releases/v0.6.0/PLAN.md)

---

## Remaining Feature Gaps (Long-term)

### Tools

- [ ] **Canvas/A2UI** - Visual workspace (different approach for Go)
- [ ] **Node Tools** - Device control, screen capture, notifications
- [ ] **Image Generation** - Dall-E, Midjourney, Flux integration
- [ ] **Video Generation** - Runway, Sora integration
- [ ] **PDF Tool** - PDF reading and generation
- [ ] **Message Tool** - Cross-channel messaging

### CLI & UX

- [ ] **Agent Management** - Multi-agent routing, agent CRUD
- [x] **Doctor Tool** - Diagnostics, auto-repair, health checks (v0.9.0)
- [x] **Onboarding** - Interactive setup wizard (v0.9.0)
- [ ] **TUI** - Terminal user interface
- [ ] **Daemon Management** - launchd/systemd service management

### Voice

- [ ] **Wake Word** - Voice activation detection
- [x] **Voice Call** - Telephony integration (v0.9.0: Telnyx)

### Apps (Future)

- [ ] **macOS Menu Bar** - Menu bar control app
- [ ] **iOS Companion** - Device pairing, voice trigger
- [ ] **Android Node** - Continuous voice, node capabilities

---

## Abstraction Libraries

| Library | Purpose | Status |
|---------|---------|--------|
| `omnichat` | Channel integrations | Active |
| `omnillm` | LLM providers | Active |
| `omnivoice` | Voice providers | Active |
| `omnivault` | Secret management | Active |
| `omnitoken` | OAuth token management | Active |
| `omniretrieve` | RAG/Memory | Active |
| `omnistorage` | File storage | Active |
