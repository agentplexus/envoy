# OmniAgent Extension Plan

> For historical plans, see `docs/releases/v0.6.0/PLAN.md`

## OpenClaw Feature Parity Tracking

This project tracks functionality from [OpenClaw](https://github.com/openclaw/openclaw) implemented with alternate approaches (Go, security-focused, etc.).

### Last Compared Commit

| Date | OpenClaw Commit | Notes |
|------|-----------------|-------|
| 2026-04-22 | `d4eb23652362a1b7d3fbcebd633a1c6f2a43c16f` | Initial gap analysis |

### Feature Parity Matrix

| Category | OpenClaw | OmniAgent | Gap |
|----------|----------|-----------|-----|
| **Channels** | 24+ | 4 | 20+ missing |
| **LLM Providers** | 50+ | 4+ (via omnillm) | Most covered |
| **Tools** | 10+ categories | 4 | 6+ categories |
| **Skills** | 55+ bundled | SKILL.md + compiled | Feature parity |
| **Scheduling** | Cron, webhooks | Cron, interval, one-time | Feature parity |
| **Voice** | Wake word, ElevenLabs | Deepgram (via omnivoice) | Wake word, call handling |
| **Sandbox** | Docker, SSH, OpenShell | WASM, Docker | SSH backend |
| **Apps** | macOS, iOS, Android | None | All 3 |
| **CLI Commands** | 50+ | ~10 | 40+ missing |
| **Plugins** | 117+ | N/A (Go approach) | Different architecture |
| **Memory/RAG** | LanceDB, Wiki | omniretrieve | Feature parity (in omniretrieve) |
| **File Storage** | N/A | N/A | omnistorage integration |
| **Visual** | Canvas/A2UI | None | Canvas system |

---

## Completed (v0.6.0)

- [x] **Session Management** - `sessions/`
- [x] **Cron/Scheduling** - `cron/`
- [x] **Context Engine** - `context/`
- [x] **Hooks System** - `hooks/`
- [x] **Compiled Skills** - `skills/compiled/`
- [x] **Platform Adapters** - `platform/standalone/`
- [x] **Storage Migration** - `omnistorage-core/kvs`

---

## Remaining Feature Gaps

### Priority 2: Tools

- [ ] **Canvas/A2UI** - Visual workspace (different approach for Go)
- [ ] **Node Tools** - Device control, screen capture, notifications
- [ ] **Image Generation** - Dall-E, Midjourney, Flux integration
- [ ] **Video Generation** - Runway, Sora integration
- [ ] **PDF Tool** - PDF reading and generation
- [ ] **Message Tool** - Cross-channel messaging

### Priority 3: CLI & UX

- [ ] **Agent Management** - Multi-agent routing, agent CRUD
- [ ] **Doctor Tool** - Diagnostics, auto-repair, health checks
- [ ] **Onboarding** - Interactive setup wizard
- [ ] **TUI** - Terminal user interface
- [ ] **Daemon Management** - launchd/systemd service management

### Priority 4: Voice

- [ ] **Wake Word** - Voice activation detection (OmniAgent-level)
- [ ] **Voice Call** - Telephony integration, call handling
- [ ] **Voice Providers** - ElevenLabs, Deepgram, etc. (in `omnivoice`)

### Priority 5: Apps (Future)

- [ ] **macOS Menu Bar** - Menu bar control app
- [ ] **iOS Companion** - Device pairing, voice trigger
- [ ] **Android Node** - Continuous voice, node capabilities

### Priority 6: Channels, LLMs & Voice Providers (User-Request Driven)

Additional integrations implemented in abstraction libraries:

| Library | Purpose |
|---------|---------|
| `omnichat` | Channel integrations (Slack, Teams, etc.) |
| `omnillm` | LLM providers (Groq, Mistral, Ollama, etc.) |
| `omnivoice` | Voice providers (ElevenLabs, Deepgram, etc.) |

---

## Upcoming Work

### Remote Skills (Phase 4)

- [ ] MCP client adapter (`skills/remote/mcp/`)
- [ ] OpenAPI spec loader (`skills/remote/openapi/`)
- [ ] Configuration-based skill loading

### OmniChat Integration (Phase 5)

- [ ] Provider wrapper (`provider/`)
- [ ] Webhook handling integration
- [ ] Multi-provider message routing
