# OpenClaw Feature Parity Roadmap

Feature roadmap tracking OpenClaw capabilities not yet implemented in OmniAgent.

**Last Sync**: 2026-06-28 (`2001b15f5b92d653464cbd847c28c136bdb465a7`)

## Summary

| Category | OpenClaw | OmniAgent | Gap |
|----------|----------|-----------|-----|
| Channels | 19+ | 4 | 15+ |
| LLM Providers | 35+ | 2 direct | Via omnillm |
| Skills | 53+ | 2 | 51+ |
| Media Generation | Full | None | All |
| Native Apps | 6 | 0 | 6 |
| CLI Commands | 50+ | ~10 | 40+ |

---

## P1: Channel Integrations

**Current**: Telegram, Discord, WhatsApp, Twilio SMS

### Missing Channels

| Channel | OpenClaw Location | Priority | Notes |
|---------|-------------------|----------|-------|
| Slack | `extensions/slack` | High | Enterprise standard |
| Signal | `extensions/signal` | High | Privacy-focused |
| Matrix | `extensions/matrix` | High | Open protocol |
| Microsoft Teams | `extensions/msteams` | Medium | Enterprise |
| IRC | `extensions/irc` | Medium | Developer community |
| iMessage | `extensions/imessage` | Medium | macOS only |
| Mattermost | `extensions/mattermost` | Medium | Self-hosted Slack |
| Feishu/Lark | `extensions/feishu` | Medium | China market |
| LINE | `extensions/line` | Low | Asia market |
| Nextcloud Talk | `extensions/nextcloud-talk` | Low | Self-hosted |
| Nostr | `extensions/nostr` | Low | Decentralized |
| QQ Bot | `extensions/qqbot` | Low | China market |
| Synology Chat | `extensions/synology-chat` | Low | NAS integration |
| Twitch | `extensions/twitch` | Low | Streaming |
| Zalo | `extensions/zalo` | Low | Vietnam market |
| WebChat | `extensions/webchat` | Low | Generic web widget |

### Implementation Plan

```
channels/
├── slack/
│   ├── client.go      # Slack Web API client
│   ├── events.go      # Event handling
│   ├── commands.go    # Slash commands
│   └── shortcuts.go   # Message shortcuts
├── signal/
│   ├── client.go      # Signal CLI integration
│   └── handler.go     # Message handling
└── matrix/
    ├── client.go      # Matrix SDK
    └── handler.go     # Room/message handling
```

---

## P2: LLM Provider Integrations

**Current**: Anthropic, OpenAI (via omnillm)

### Missing Direct Providers

| Provider | OpenClaw Location | Priority | Notes |
|----------|-------------------|----------|-------|
| Azure OpenAI | `extensions/azure-openai` | High | Enterprise |
| Google Vertex AI | `extensions/google-vertex` | High | GCP integration |
| Mistral | `extensions/mistral` | High | Open weights |
| Groq | `extensions/groq` | High | Fast inference |
| Cohere | `extensions/cohere` | Medium | Enterprise RAG |
| xAI (Grok) | `extensions/xai` | Medium | Twitter/X |
| Deepseek | `extensions/deepseek` | Medium | Code focused |
| Together AI | `extensions/together` | Medium | Open models |
| Fireworks | `extensions/fireworks` | Medium | Fast inference |
| Perplexity | `extensions/perplexity` | Medium | Search + LLM |
| Cerebras | `extensions/cerebras` | Low | Fast inference |
| Cloudflare AI | `extensions/cloudflare-ai` | Low | Edge |
| LM Studio | `extensions/lm-studio` | Low | Local |
| Ollama | `extensions/ollama` | Medium | Local |
| vLLM | `extensions/vllm` | Low | Self-hosted |
| OpenRouter | `extensions/openrouter` | Medium | Aggregator |
| Qwen (Alibaba) | `extensions/qwen` | Low | China |
| Baidu Qianfan | `extensions/qianfan` | Low | China |
| Kimi (Moonshot) | `extensions/kimi` | Low | China |
| BytePlus | `extensions/byteplus` | Low | China |

### Note

Most providers should be added to `omnillm` library rather than OmniAgent directly.

---

## P3: Built-in Skills

**Current**: Memory skill, compiled skill framework

### Missing Skills by Category

#### Development Tools

| Skill | OpenClaw Location | Priority |
|-------|-------------------|----------|
| GitHub Issues | `skills/github-issues` | High |
| GitHub PRs | `skills/github-prs` | High |
| Coding Agent | `skills/coding-agent` | High |
| Python Debugger | `skills/python-debugpy` | Medium |
| Node Connect | `skills/node-connect` | Medium |

#### Productivity

| Skill | OpenClaw Location | Priority |
|-------|-------------------|----------|
| Notion | `skills/notion` | High |
| Obsidian | `skills/obsidian` | Medium |
| Apple Notes | `skills/apple-notes` | Medium |
| Trello | `skills/trello` | Low |
| Things (macOS) | `skills/things-mac` | Low |

#### Web & Search

| Skill | OpenClaw Location | Priority |
|-------|-------------------|----------|
| Web Search | `skills/web-search` | High |
| Web Readability | `skills/web-readability` | High |
| Firecrawl | `skills/firecrawl` | Medium |
| URL Screenshot | `skills/url-screenshot` | Medium |

#### Media

| Skill | OpenClaw Location | Priority |
|-------|-------------------|----------|
| Image Generation | `skills/image-generation` | Medium |
| Video Frames | `skills/video-frames` | Low |
| Diagram Maker | `skills/diagram-maker` | Medium |
| Meme Maker | `skills/meme-maker` | Low |

#### Music & Audio

| Skill | OpenClaw Location | Priority |
|-------|-------------------|----------|
| Spotify | `skills/spotify` | Low |
| Sonos | `skills/sonos` | Low |

#### System

| Skill | OpenClaw Location | Priority |
|-------|-------------------|----------|
| Terminal (tmux) | `skills/terminal` | Medium |
| Hardware Info | `skills/hardware` | Low |
| Healthcheck | `skills/healthcheck` | Medium |

#### Authentication

| Skill | OpenClaw Location | Priority |
|-------|-------------------|----------|
| 1Password | `skills/1password` | Medium |
| Bitwarden | `skills/bitwarden` | Low |

---

## P4: Media Generation

**Current**: None

### Image Generation

| Feature | OpenClaw Location | Priority |
|---------|-------------------|----------|
| DALL-E | `extensions/image-generation-core` | Medium |
| Fal AI | `extensions/fal` | Medium |
| Stable Diffusion | `extensions/comfyui` | Low |
| Midjourney (unofficial) | N/A | Low |

### Video Generation

| Feature | OpenClaw Location | Priority |
|---------|-------------------|----------|
| Runway | `extensions/runway` | Low |
| Pixverse | `extensions/pixverse` | Low |

### Music Generation

| Feature | OpenClaw Location | Priority |
|---------|-------------------|----------|
| Suno | `extensions/music-generation-providers` | Low |
| Udio | `extensions/music-generation-providers` | Low |

---

## P5: CLI Features

**Current**: gateway, channels, skills, voice, openai

### Missing Commands

| Command | OpenClaw Location | Priority | Description |
|---------|-------------------|----------|-------------|
| `setup` | `src/cli/command-bootstrap.ts` | High | Interactive setup wizard |
| `doctor` | `src/cli/doctor.ts` | High | Diagnose configuration |
| `sessions` | `src/cli/sessions.ts` | Medium | Session management |
| `sessions compact` | `src/cli/sessions.ts` | Medium | Compact session history |
| `plugins` | `src/cli/plugin-cli.ts` | Medium | Plugin management |
| `plugins install` | `src/cli/plugin-cli.ts` | Medium | Install plugins |
| `plugins list` | `src/cli/plugin-cli.ts` | Medium | List plugins |
| `models` | `src/cli/models.ts` | Medium | Model management |
| `models aliases` | `src/cli/models.ts` | Low | Model alias management |
| `config resolve` | `src/cli/command-config-resolution.ts` | Low | Debug config resolution |
| `agent` | `src/cli/agents.commands.ts` | Medium | Run agent with message |
| `agent --message-file` | `src/cli/agents.commands.ts` | Medium | Message from file |
| `interactive` | `src/interactive/` | Low | Interactive shell mode |
| `tui` | `src/tui/` | Low | Terminal UI |

### Implementation Plan

```go
// cmd/omniagent/commands/
├── setup.go      // Interactive setup wizard
├── doctor.go     // Configuration diagnostics
├── sessions.go   // Session management
├── plugins.go    // Plugin management
└── models.go     // Model/alias management
```

---

## P6: Specialized Features

**Current**: None

### Agent Communication

| Feature | OpenClaw Location | Priority | Description |
|---------|-------------------|----------|-------------|
| ACP | `src/acp/` | Medium | Agent Communication Protocol |
| Auto-Reply | `src/auto-reply/` | Low | Automatic response handling |
| Commitments | `src/commitments/` | Low | Task/promise tracking |

### Context & Memory

| Feature | OpenClaw Location | Priority | Description |
|---------|-------------------|----------|-------------|
| Active Memory | `extensions/active-memory` | Medium | Working memory |
| Memory Wiki | `extensions/memory-wiki` | Low | Knowledge base |
| LanceDB | `extensions/memory-lancedb` | Medium | Vector DB backend |

### Safety & Analysis

| Feature | OpenClaw Location | Priority | Description |
|---------|-------------------|----------|-------------|
| Crestodian | `src/crestodian/` | Low | Conversation safety |
| Trajectory | `src/trajectory/` | Low | Interaction tracking |
| Link Understanding | `src/link-understanding/` | Medium | URL preview/analysis |

### Gateway Features

| Feature | OpenClaw Location | Priority | Description |
|---------|-------------------|----------|-------------|
| Device Pairing | `extensions/device-pair` | Low | Mobile device pairing |
| Agent Pairing | `src/gateway/` | Low | Multi-agent coordination |
| Admin HTTP RPC | `src/gateway/` | Low | Admin API |

---

## P7: Native Applications

**Current**: None (CLI only)

| Platform | OpenClaw Location | Priority | Notes |
|----------|-------------------|----------|-------|
| Web UI | `ui/` | High | React dashboard |
| macOS | `apps/macos/` | Low | Native desktop |
| iOS | `apps/ios/` | Low | Native mobile |
| Android | `apps/android/` | Low | Native mobile |
| Windows | `apps/windows/` | Low | Hub app |
| Linux | `apps/linux/` | Low | Electron |

### Web UI Priority Features

| Feature | OpenClaw Location | Priority |
|---------|-------------------|----------|
| Canvas/Live Rendering | `ui/src/ui/views/canvas/` | Medium |
| Workboard | `ui/src/ui/views/workboard/` | Medium |
| Overview Dashboard | `ui/src/ui/views/overview/` | Medium |

---

## P8: Infrastructure

### Implemented ✓

- [x] Browser automation (Rod)
- [x] Docker sandboxing
- [x] WASM sandboxing (wazero)
- [x] Cron/scheduling
- [x] MCP support
- [x] OpenAI-compatible API
- [x] OAuth SSO
- [x] Session persistence
- [x] Observability (OTEL)
- [x] Bounded HTTP reads (OOM prevention)
- [x] Process tree cleanup on timeout
- [x] UTF-16 safe truncation

### Missing

| Feature | OpenClaw Location | Priority |
|---------|-------------------|----------|
| Policy Engine | `extensions/policy` | Medium |
| Phone Control | `extensions/phone-control` | Low |
| Heartbeat | `src/heartbeat/` | Low |

---

## Implementation Priorities

### Phase 1 (Q3 2026)

1. **Slack channel** - Enterprise adoption
2. **Signal channel** - Privacy users
3. **Matrix channel** - Open protocol
4. **Setup wizard CLI** - Better onboarding
5. **Doctor CLI** - Debug support
6. **GitHub skills** - Developer productivity

### Phase 2 (Q4 2026)

1. **Azure OpenAI provider** - Enterprise
2. **Groq provider** - Fast inference
3. **Web search skill** - Core capability
4. **Sessions CLI** - Session management
5. **Active memory** - Better context

### Phase 3 (2027)

1. **Image generation** - Media capabilities
2. **Web UI enhancements** - Dashboard
3. **Additional channels** - Market expansion
4. **Native apps** - Platform coverage

---

## Contributing

To add a feature from this roadmap:

1. Check the OpenClaw implementation for reference
2. Create a design doc in `docs/design/`
3. Implement with tests
4. Update this roadmap (move to implemented)
5. Submit PR with conventional commit

---

## References

- [OpenClaw Repository](https://github.com/openclaw/openclaw)
- [OpenClaw Documentation](https://docs.openclaw.ai)
- [OmniAgent Sync Plan](parity-2026-06-28/PLAN.md)
