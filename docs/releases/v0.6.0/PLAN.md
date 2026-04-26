# OmniAgent v0.6.0 Plan (Historical)

> **Status**: Completed 2026-04-25
> This document captures the plan as executed for v0.6.0.

# OmniAgent Extension Plan: Compiled Skills, Remote Skills, Storage & Platforms

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

### Feature Gap Categories

#### Priority 1: Core Infrastructure ✅ Complete

- [x] **Session Management** - Session listing, history, spawning (`sessions/`)
- [x] **Cron/Scheduling** - Scheduled jobs (cron, interval, one-time) (`cron/`)
- [x] **Context Engine** - Message context compilation, window management (`context/`)
- [x] **Hooks System** - Event-based extensibility (`hooks/`)

#### Priority 2: Tools

- [ ] **Canvas/A2UI** - Visual workspace (different approach for Go)
- [ ] **Node Tools** - Device control, screen capture, notifications
- [ ] **Image Generation** - Dall-E, Midjourney, Flux integration
- [ ] **Video Generation** - Runway, Sora integration
- [ ] **PDF Tool** - PDF reading and generation
- [ ] **Message Tool** - Cross-channel messaging

#### Priority 3: CLI & UX

- [ ] **Agent Management** - Multi-agent routing, agent CRUD
- [ ] **Doctor Tool** - Diagnostics, auto-repair, health checks
- [ ] **Onboarding** - Interactive setup wizard
- [ ] **TUI** - Terminal user interface
- [ ] **Daemon Management** - launchd/systemd service management

#### Priority 4: Voice

- [ ] **Wake Word** - Voice activation detection (OmniAgent-level)
- [ ] **Voice Call** - Telephony integration, call handling
- [ ] **Voice Providers** - ElevenLabs, Deepgram, etc. (in `omnivoice`)

#### Priority 5: Apps (Future)

- [ ] **macOS Menu Bar** - Menu bar control app
- [ ] **iOS Companion** - Device pairing, voice trigger
- [ ] **Android Node** - Continuous voice, node capabilities

#### Priority 6: Channels, LLMs & Voice Providers (User-Request Driven)

Additional channels, LLM providers, and voice providers are implemented in abstraction libraries rather than in OmniAgent directly. Add based on user requests.

| Library | Purpose |
|---------|---------|
| `omnichat` | Channel integrations (Slack, Teams, etc.) |
| `omnillm` | LLM providers (Groq, Mistral, Ollama, etc.) |
| `omnivoice` | Voice providers (ElevenLabs, Deepgram, etc.) |

- [ ] **Channels** - Slack, Teams, iMessage, Signal, Matrix, IRC, WebChat, etc.
- [ ] **LLM Providers** - Additional providers beyond OpenAI, Anthropic, Google, Bedrock
- [ ] **Voice Providers** - ElevenLabs, additional STT/TTS providers

---

## Library Extraction Plan

OmniAgent uses abstraction libraries for reusability and independent testing. These libraries follow the `omni*` naming convention and live under `github.com/plexusone/`.

### Current Libraries

| Library | Purpose | Status |
|---------|---------|--------|
| `omnichat` | Channel providers (WhatsApp, Telegram, etc.) | ✅ In use |
| `omnillm` | LLM providers (OpenAI, Anthropic, etc.) | ✅ In use |
| `omnivoice` | Voice providers (Deepgram, ElevenLabs, etc.) | ✅ In use |
| `omniobserve` | Observability/tracing | ✅ In use |
| `omniserp` | Web search | ✅ In use |
| `omniretrieve` | RAG (vector & graph retrieval) | ✅ Exists |
| `omnistorage` | File/object storage (rclone-style) | ✅ Exists at grokify/, move to plexusone/ |

### Planned Extractions

| Library | Purpose | Source | Priority |
|---------|---------|--------|----------|
| `omnisandbox` | Execution isolation (WASM, Docker, SSH) | `omniagent/sandbox/` | High |
| `omniskill` | Skill/tool framework (SKILL.md, registry) | `omniagent/skills/` | High |
| `omnimcp` | MCP client (tool discovery, execution) | New | Medium |

### Storage & Retrieval Architecture

Three distinct data access patterns, unified under `omnistorage` and `omniretrieve`:

| Library | Subpackage | Interface | Use Cases |
|---------|------------|-----------|-----------|
| `omnistorage` | `/object` | `Reader/Writer` + sync | Documents, media, backups |
| `omnistorage` | `/kvs` | `Get/Set/Delete` + TTL | Sessions, cache, state |
| `omniretrieve` | `/vector` | `Embed/Query/Retrieve` | Vector RAG (LanceDB, Pinecone) |
| `omniretrieve` | `/graph` | `Store/Traverse/Query` | Graph RAG (Neo4j, wiki-style) |

**omnistorage** (at `github.com/grokify/omnistorage`, move to `plexusone/`):
- `/object` - File/object storage (existing): Memory, File, S3, SFTP
- `/kvs` - Key-value storage (new): Memory, SQLite, Redis, DynamoDB

**omniretrieve** - RAG retrieval:
- `/vector` - Vector embeddings and similarity search
- `/graph` - Knowledge graphs and traversal

---

## Overview

This plan extends OmniAgent with:

1. **Compiled Skills** - Go packages implementing `skill.CompiledSkill` interface
2. **Remote Skills** - MCP servers, OpenAPI specs loaded at runtime
3. **Storage Interface** - SQLite, DynamoDB, memory backends
4. **Platform Adapters** - Standalone, Lightsail, Lambda, AgentCore deployment targets
5. **OmniChat Provider Integration** - Use omnichat providers for messaging channels

## Current Architecture

```
omniagent/
├── agent/          # Agent runtime with omnillm
├── skills/         # SKILL.md file loading (OpenClaw style)
├── gateway/        # WebSocket control plane
├── sandbox/        # WASM + Docker isolation
├── config/         # YAML configuration
├── tools/          # Browser, shell tools
└── cmd/omniagent/  # CLI
```

## Extended Architecture

```
omniagent/
├── agent/              # Agent runtime (enhanced)
├── skills/
│   ├── markdown/       # SKILL.md loader (existing, renamed)
│   ├── compiled/       # Compiled skill interface + registry
│   └── remote/         # MCP, OpenAPI skill loaders
├── storage/            # Storage interface
│   ├── storage.go      # Interface definition
│   ├── sqlite/         # SQLite backend
│   ├── dynamodb/       # DynamoDB backend
│   └── memory/         # In-memory backend
├── platform/           # Deployment platforms
│   ├── platform.go     # Platform interface
│   ├── standalone/     # Long-running process
│   ├── lightsail/      # Lightsail deployment helpers
│   ├── lambda/         # AWS Lambda adapter
│   └── agentcore/      # AWS Bedrock AgentCore adapter
├── provider/           # Channel provider integration
│   └── omnichat.go     # Wrap omnichat.Provider
├── gateway/            # WebSocket control plane (existing)
├── sandbox/            # WASM + Docker isolation (existing)
├── config/             # YAML configuration (enhanced)
└── cmd/omniagent/      # CLI
```

## Skill Types

### Type 1: Markdown Skills (Existing)

SKILL.md files from OpenClaw/ClawHub. Inject instructions into system prompt.

```yaml
# SKILL.md
---
name: github
description: GitHub CLI integration
metadata:
  openclaw:
    emoji: 🐙
    requires:
      bins: [gh]
---
You can use the `gh` CLI to interact with GitHub...
```

### Type 2: Compiled Skills (New)

Go packages implementing `skill.CompiledSkill`. Provide callable tools.

```go
// github.com/grokify/agent-team-invest/omniagent/skill
package skill

type Skill struct { ... }

func (s *Skill) Name() string { return "invest" }
func (s *Skill) Tools() []skill.Tool {
    return []skill.Tool{
        {Name: "analyze_stock", Handler: s.analyzeStock},
    }
}
```

Usage:
```go
import investskill "github.com/grokify/agent-team-invest/omniagent/skill"

agent := omniagent.New(
    omniagent.WithCompiledSkill(investskill.New(cfg)),
)
```

### Type 3: Remote Skills (New)

Skills loaded at runtime via configuration.

```yaml
# omniagent.yaml
skills:
  remote:
    - name: github
      type: mcp
      command: npx -y @modelcontextprotocol/server-github
      env:
        GITHUB_TOKEN: ${GITHUB_TOKEN}

    - name: petstore
      type: openapi
      url: https://petstore.swagger.io/v2/swagger.json
```

## Interfaces

### Compiled Skill Interface

```go
// skills/compiled/skill.go
package compiled

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

type Tool struct {
    Name        string
    Description string
    Parameters  map[string]Parameter
    Handler     func(ctx context.Context, params map[string]any) (any, error)
}
```

### Storage Interface

```go
// storage/storage.go
package storage

type Storage interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
}

type DocumentStorage interface {
    Storage
    Query(ctx context.Context, collection string, filter map[string]any) ([]Document, error)
    Put(ctx context.Context, collection string, doc Document) error
}
```

### Platform Interface

```go
// platform/platform.go
package platform

type Platform interface {
    Run(ctx context.Context, agent *agent.Agent) error
}
```

## Configuration Enhancement

```yaml
# omniagent.yaml (extended)
platform:
  type: standalone          # standalone, lambda, agentcore
  addr: ":8080"

storage:
  type: sqlite              # sqlite, dynamodb, memory
  path: /data/omniagent.db

agent:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: ${ANTHROPIC_API_KEY}

# Existing channels (whatsapp, telegram, discord)
channels:
  whatsapp:
    enabled: true

# New: omnichat providers
providers:
  - type: twilio-sms
    account_sid: ${TWILIO_ACCOUNT_SID}
    auth_token: ${TWILIO_AUTH_TOKEN}
    phone_number: ${TWILIO_PHONE_NUMBER}

skills:
  # Existing: markdown skills
  enabled: true
  paths:
    - ~/.omniagent/skills

  # New: compiled skills (configured via code, listed here for visibility)
  compiled:
    - invest

  # New: remote skills
  remote:
    - name: github
      type: mcp
      command: npx -y @modelcontextprotocol/server-github
```

## Implementation Phases

### Phase 1: Compiled Skill Interface

1. Create `skills/compiled/` package with interfaces
2. Create skill registry for compiled skills
3. Integrate compiled skill tools with agent tool registry
4. Add `WithCompiledSkill()` option

### Phase 2: Storage Interface

1. Create `storage/` package with interface
2. Implement SQLite backend
3. Implement memory backend (for testing)
4. Add `WithStorage()` option
5. Pass storage to storage-aware skills

### Phase 3: Platform Adapters

1. Create `platform/` package with interface
2. Extract current gateway into `platform/standalone/`
3. Add Lightsail deployment helpers
4. (Future) Add Lambda adapter
5. (Future) Add AgentCore adapter

### Phase 4: Remote Skills

1. Create `skills/remote/` package
2. Implement MCP client adapter
3. Implement OpenAPI spec loader
4. Add configuration-based skill loading

### Phase 5: OmniChat Integration

1. Create `provider/` package
2. Wrap omnichat.Provider for agent use
3. Add provider routing (multiple channels)
4. Integrate webhook handling

## Deployment Scenarios

### Scenario 1: Lightsail VM (Primary Target)

```
┌─────────────────────────────────────────┐
│         Lightsail VM ($3.50-5/mo)       │
│                                         │
│  ┌─────────────────────────────────┐   │
│  │  Caddy (reverse proxy + HTTPS)  │   │
│  └─────────────────────────────────┘   │
│              │                          │
│  ┌─────────────────────────────────┐   │
│  │  systemd → omniagent binary     │   │
│  │    (standalone platform)        │   │
│  └─────────────────────────────────┘   │
│              │                          │
│  ┌───────────┴───────────┐             │
│  ▼                       ▼             │
│  SQLite              Twilio SMS        │
│  (storage)           (omnichat)        │
└─────────────────────────────────────────┘
```

### Scenario 2: Lightsail Container Service (Future)

```
┌─────────────────────────────────────────┐
│    Lightsail Container Service          │
│                                         │
│  ┌─────────────────────────────────┐   │
│  │  Managed HTTPS + Load Balancer  │   │
│  └─────────────────────────────────┘   │
│              │                          │
│  ┌─────────────────────────────────┐   │
│  │  omniagent container (auto-     │   │
│  │  scaling, managed restarts)     │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

### Scenario 3: AWS Serverless (Future)

```
┌─────────────────────────────────────────┐
│              AWS                         │
│                                         │
│  Lambda/AgentCore  ←→  DynamoDB         │
│         │                               │
│         ↓                               │
│  API Gateway  ←→  Twilio Webhooks       │
└─────────────────────────────────────────┘
```

---

## Lightsail Deployment Options

### Decision: VM-based with Deploy Scripts (Selected)

We evaluated multiple deployment approaches for Lightsail:

| Approach | Description | Status |
|----------|-------------|--------|
| **VM + Deploy Scripts** | Systemd + Caddy on Lightsail VM via SSH | ✅ Selected |
| **VM + Custom AMI** | Pre-baked AMI with omniagent installed | Future option |
| **Container Service** | Lightsail Container Service via Pulumi | Future option |

### Why VM + Deploy Scripts First

1. **Simplicity** - No container registry, no IaC complexity
2. **Cost** - $3.50-5/mo for smallest Lightsail instance
3. **Flexibility** - Easy to update, debug, and customize
4. **Quick Start** - SSH + SCP deployment, immediate iteration

### Future Options

#### Custom AMI (when needed)

- **Use case**: High-scale deployments, strict boot time requirements
- **Implementation**: Packer-based AMI creation
- **Trade-off**: More complex maintenance, must rebuild for updates

#### Container Service (when needed)

- **Use case**: Auto-scaling, managed infrastructure, team deployments
- **Implementation**: Pulumi provider in `agentkit/deploy/providers/lightsail/`
- **Trade-off**: Higher cost, container registry required

### Deployment Tooling Location

Deployment tooling lives in `agentkit` (not `omniagent`) for reusability:

```
agentkit/deploy/vm/
├── templates/
│   ├── systemd.service.tmpl      # omniagent.service
│   ├── Caddyfile.tmpl            # Reverse proxy + auto-HTTPS
│   └── omniagent.yaml.tmpl       # Config file template
├── generator.go                   # Render templates
├── deployer.go                    # SSH/SCP deployment
└── config.go                      # VM deployment config
```

### Deployment Flow

```bash
# 1. Generate deployment configs
agentkit deploy vm init --name myagent --domain agent.example.com

# 2. Push binary and configs to Lightsail instance
agentkit deploy vm push --host 1.2.3.4 --key ~/.ssh/lightsail.pem

# 3. Install systemd service and Caddy
agentkit deploy vm install --host 1.2.3.4 --key ~/.ssh/lightsail.pem

# 4. (Future) Build custom AMI
agentkit deploy vm build-ami --region us-east-1
```

## Migration Path

Existing omniagent functionality remains unchanged:

- Gateway WebSocket control plane
- WhatsApp/Telegram/Discord built-in channels
- SKILL.md markdown skills
- WASM + Docker sandbox
- Voice processing

New features are additive:

- Compiled skills supplement markdown skills
- Storage is optional (in-memory default)
- Platform defaults to standalone (current behavior)
- OmniChat providers supplement built-in channels

---

## v0.6.0 Completion Summary

This section documents what was actually implemented in v0.6.0 vs the original plan.

### Implemented in v0.6.0

| Phase | Component | Status | Notes |
|-------|-----------|--------|-------|
| Phase 1 | `skills/compiled/` | ✅ | Core interface and registry |
| Phase 2 | Storage | ✅ | Migrated to `omnistorage-core/kvs` instead of local package |
| Phase 3 | `platform/standalone/` | ✅ | Basic platform adapter |
| Additional | `sessions/` | ✅ | Not in original plan |
| Additional | `context/` | ✅ | Not in original plan |
| Additional | `hooks/` | ✅ | Not in original plan |
| Additional | `cron/` | ✅ | Not in original plan |

### Deferred from v0.6.0

| Phase | Component | Status | Notes |
|-------|-----------|--------|-------|
| Phase 3 | `platform/lightsail/` | ⏸️ | Deferred |
| Phase 3 | `platform/lambda/` | ⏸️ | Deferred |
| Phase 3 | `platform/agentcore/` | ⏸️ | Deferred |
| Phase 4 | `skills/remote/mcp/` | ⏸️ | Moved to future release |
| Phase 4 | `skills/remote/openapi/` | ⏸️ | Moved to future release |
| Phase 5 | `provider/` (OmniChat) | ⏸️ | Moved to future release |

### Architecture Changes from Plan

1. **Storage**: The plan called for a local `storage/` package. Instead, storage was implemented in `omnistorage-core/kvs` as a shared library.

2. **Additional packages**: The plan didn't originally include sessions, context, hooks, or cron packages. These were added during implementation to provide complete infrastructure for v0.6.0.

3. **Configuration**: YAML configuration support for skills and storage was deferred in favor of programmatic configuration via functional options.

See [TASKS.md](TASKS.md) for detailed task-level completion status.
