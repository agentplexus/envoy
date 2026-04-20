# OmniAgent

[![Go CI][go-ci-svg]][go-ci-url]
[![Go Lint][go-lint-svg]][go-lint-url]
[![Go SAST][go-sast-svg]][go-sast-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Docs][docs-mkdoc-svg]][docs-mkdoc-url]
[![Visualization][viz-svg]][viz-url]
[![License][license-svg]][license-url]

 [go-ci-svg]: https://github.com/plexusone/omniagent/actions/workflows/go-ci.yaml/badge.svg?branch=main
 [go-ci-url]: https://github.com/plexusone/omniagent/actions/workflows/go-ci.yaml
 [go-lint-svg]: https://github.com/plexusone/omniagent/actions/workflows/go-lint.yaml/badge.svg?branch=main
 [go-lint-url]: https://github.com/plexusone/omniagent/actions/workflows/go-lint.yaml
 [go-sast-svg]: https://github.com/plexusone/omniagent/actions/workflows/go-sast-codeql.yaml/badge.svg?branch=main
 [go-sast-url]: https://github.com/plexusone/omniagent/actions/workflows/go-sast-codeql.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/plexusone/omniagent
 [goreport-url]: https://goreportcard.com/report/github.com/plexusone/omniagent
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/plexusone/omniagent
 [docs-godoc-url]: https://pkg.go.dev/github.com/plexusone/omniagent
 [docs-mkdoc-svg]: https://img.shields.io/badge/Go-dev%20guide-blue.svg
 [docs-mkdoc-url]: https://plexusone.dev/omniagent
 [viz-svg]: https://img.shields.io/badge/Go-visualizaton-blue.svg
 [viz-url]: https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=plexusone%2Fomniagent
 [loc-svg]: https://tokei.rs/b1/github/plexusone/omniagent
 [repo-url]: https://github.com/plexusone/omniagent
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/plexusone/omniagent/blob/main/LICENSE

Your AI representative across communication channels.

OmniAgent is a personal AI assistant that routes messages across multiple communication platforms, processes them via an AI agent, and responds on your behalf.

## Features

- 💬 **Multi-Channel Support** - Telegram, Discord, Slack, WhatsApp, and more
- 🤖 **AI-Powered Responses** - Powered by omnillm (Claude, GPT, Gemini, etc.)
- 🎤 **Voice Notes** - Transcribe incoming voice, respond with synthesized speech via OmniVoice
- 🧩 **Skills System** - Extensible skills compatible with OpenClaw/ClawHub, plus compiled Go skills
- 🔒 **Secure Sandboxing** - WASM and Docker isolation for tool execution
- 🌐 **Browser Automation** - Built-in browser control via Rod
- 🔌 **WebSocket Gateway** - Real-time control plane for device connections
- 📊 **Observability** - Integrated tracing via omniobserve

## Installation

```bash
go install github.com/plexusone/omniagent/cmd/omniagent@latest
```

## Quick Start

### WhatsApp + OpenAI

The fastest way to get started is with WhatsApp and OpenAI:

```bash
# Set your OpenAI API key
export OPENAI_API_KEY="sk-..."

# Run with WhatsApp enabled
OMNIAGENT_AGENT_PROVIDER=openai \
OMNIAGENT_AGENT_MODEL=gpt-4o \
WHATSAPP_ENABLED=true \
omniagent gateway run
```

A QR code will appear in your terminal. Scan it with WhatsApp (Settings -> Linked Devices -> Link a Device) to connect.

### Configuration File

For more control, create a configuration file:

```yaml
# omniagent.yaml
gateway:
  address: "127.0.0.1:18789"

agent:
  provider: openai          # or: anthropic, gemini
  model: gpt-4o             # or: claude-sonnet-4-20250514, gemini-2.0-flash
  api_key: ${OPENAI_API_KEY}
  system_prompt: "You are OmniAgent, responding on behalf of the user."

channels:
  whatsapp:
    enabled: true
    db_path: "whatsapp.db"  # Session storage

  telegram:
    enabled: false
    token: ${TELEGRAM_BOT_TOKEN}

  discord:
    enabled: false
    token: ${DISCORD_BOT_TOKEN}

  twilio_sms:
    enabled: false
    account_sid: ${TWILIO_ACCOUNT_SID}
    auth_token: ${TWILIO_AUTH_TOKEN}
    phone_number: ${TWILIO_PHONE_NUMBER}
    webhook_path: /webhook/twilio/sms

voice:
  enabled: true
  response_mode: auto        # auto, always, never
  stt:
    provider: deepgram
    model: nova-2
  tts:
    provider: deepgram
    model: aura-asteria-en
    voice_id: aura-asteria-en

skills:
  enabled: true
  paths:                     # Additional skill directories
    - ~/.omniagent/skills
  max_injected: 20           # Max skills to inject into prompt
```

Run with the config file:

```bash
omniagent gateway run --config omniagent.yaml
```

## Skills

OmniAgent supports skills compatible with the [OpenClaw](https://github.com/openclaw/openclaw) SKILL.md format. Skills extend the agent's capabilities by injecting domain-specific instructions into the system prompt.

### Managing Skills

```bash
# List all discovered skills
omniagent skills list

# Show details for a specific skill
omniagent skills info sonoscli

# Check requirements for all skills
omniagent skills check
```

### Skill Format

Skills are defined in `SKILL.md` files with YAML frontmatter:

```markdown
---
name: weather
description: Get weather forecasts
metadata:
  emoji: "🌤️"
  requires:
    bins: ["curl"]
  install:
    - name: curl
      brew: curl
      apt: curl
---

# Weather Skill

You can check the weather using the `curl` command...
```

### Skill Discovery

Skills are discovered from:

1. Built-in skills directory
2. `~/.omniagent/skills/`
3. Custom paths via `skills.paths` config

Skills with missing requirements (binaries, env vars) are automatically skipped.

### Compiled Skills

For better performance, you can register Go functions directly as skills:

```go
import "github.com/plexusone/omniagent/skills"

skill := skills.CompiledSkill{
    Name:        "get_weather",
    Description: "Get weather for a location",
    Handler: func(ctx context.Context, args map[string]any) (string, error) {
        return fetchWeather(args["location"].(string))
    },
}

skills.RegisterCompiled(skill)
```

## Sandboxing

OmniAgent provides layered security for tool execution:

### App-Level Permissions

Capability-based permissions control what tools can do:

- `fs_read` - Read files from allowed paths
- `fs_write` - Write files to allowed paths
- `net_http` - Make HTTP requests to allowed hosts
- `exec_run` - Execute allowed commands

### Docker Isolation

For OS-level isolation, tools can run inside Docker containers:

```go
sandbox, _ := sandbox.NewDockerSandbox(ctx, sandbox.DockerConfig{
    Image:       "alpine:latest",
    NetworkMode: "none",           // No network access
    CapDrop:     []string{"ALL"},  // Drop all capabilities
    Mounts: []sandbox.DockerMount{
        {HostPath: "/tmp/data", ContainerPath: "/data", ReadOnly: true},
    },
}, &appConfig)

result, _ := sandbox.Run(ctx, "cat", []string{"/data/file.txt"})
```

### WASM Runtime

For lightweight isolation, tools can run in a WASM sandbox (wazero):

```go
runtime, _ := sandbox.NewRuntime(ctx, sandbox.Config{
    Capabilities:  []sandbox.Capability{sandbox.CapFSRead},
    MemoryLimitMB: 16,
    Timeout:       30 * time.Second,
    AllowedPaths:  []string{"/tmp/data"},
})
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `GEMINI_API_KEY` | Google Gemini API key |
| `OMNIAGENT_AGENT_PROVIDER` | LLM provider: `openai`, `anthropic`, `gemini` |
| `OMNIAGENT_AGENT_MODEL` | Model name (e.g., `gpt-4o`, `claude-sonnet-4-20250514`) |
| `WHATSAPP_ENABLED` | Set to `true` to enable WhatsApp |
| `WHATSAPP_DB_PATH` | WhatsApp session storage path |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token (auto-enables Telegram) |
| `DISCORD_BOT_TOKEN` | Discord bot token (auto-enables Discord) |
| `TWILIO_ACCOUNT_SID` | Twilio Account SID (auto-enables SMS) |
| `TWILIO_AUTH_TOKEN` | Twilio Auth Token |
| `TWILIO_PHONE_NUMBER` | Twilio phone number in E.164 format |
| `TWILIO_WEBHOOK_PATH` | SMS webhook path (default: `/webhook/twilio/sms`) |
| `SERPER_API_KEY` | Serper API key for web search |
| `SERPAPI_API_KEY` | SerpAPI key for web search (alternative) |
| `DEEPGRAM_API_KEY` | Deepgram API key for voice STT/TTS |
| `OMNIAGENT_VOICE_ENABLED` | Set to `true` to enable voice processing |
| `OMNIAGENT_VOICE_RESPONSE_MODE` | Voice response mode: `auto`, `always`, `never` |

## CLI Commands

```bash
# Gateway
omniagent gateway run      # Start the gateway server

# Skills
omniagent skills list      # List all discovered skills
omniagent skills info NAME # Show skill details
omniagent skills check     # Validate skill requirements

# Channels
omniagent channels list    # List registered channels
omniagent channels status  # Show channel connection status

# Config
omniagent config show      # Display current configuration

# Version
omniagent version          # Show version information
```

## Architecture

```
+-------------------------------------------------------------+
|                     Messaging Channels                      |
|     Telegram  |  Discord  |  Slack  |  WhatsApp  |  ...     |
+---------------------------+---------------------------------+
                            |
+---------------------------v---------------------------------+
|              Gateway (WebSocket Control Plane)              |
|              ws://127.0.0.1:18789                           |
+---------------------------+---------------------------------+
                            |
+---------------------------v---------------------------------+
|                      Agent Runtime                          |
|  +------------------+  +------------------+                 |
|  |    Skills        |  |    Sandbox       |                 |
|  |  (SKILL.md)      |  |  (WASM/Docker)   |                 |
|  +------------------+  +------------------+                 |
|  - omnillm (LLM providers)                                  |
|  - omnivoice (STT/TTS)                                      |
|  - omniobserve (tracing)                                    |
|  - Tools (browser, shell, http)                             |
+-------------------------------------------------------------+
```

## Configuration Reference

### Gateway

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `gateway.address` | string | `127.0.0.1:18789` | WebSocket server address |
| `gateway.read_timeout` | duration | `30s` | Read timeout |
| `gateway.write_timeout` | duration | `30s` | Write timeout |
| `gateway.ping_interval` | duration | `30s` | WebSocket ping interval |

### Agent

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `agent.provider` | string | `anthropic` | LLM provider |
| `agent.model` | string | `claude-sonnet-4-20250514` | Model name |
| `agent.api_key` | string | - | API key (or use env var) |
| `agent.temperature` | float | `0.7` | Sampling temperature |
| `agent.max_tokens` | int | `4096` | Max response tokens |
| `agent.system_prompt` | string | - | Custom system prompt |

### Skills

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `skills.enabled` | bool | `true` | Enable skill loading |
| `skills.paths` | []string | `[]` | Additional skill directories |
| `skills.disabled` | []string | `[]` | Skills to skip |
| `skills.max_injected` | int | `20` | Max skills in prompt |

### Voice

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `voice.enabled` | bool | `false` | Enable voice processing |
| `voice.response_mode` | string | `auto` | `auto`, `always`, `never` |
| `voice.stt.provider` | string | - | STT provider (e.g., `deepgram`) |
| `voice.tts.provider` | string | - | TTS provider (e.g., `deepgram`) |

## Dependencies

| Package | Purpose |
|---------|---------|
| [omnichat](https://github.com/plexusone/omnichat) | Unified messaging (WhatsApp, Telegram, Discord) |
| [omnillm](https://github.com/plexusone/omnillm) | Multi-provider LLM abstraction |
| [omnivoice](https://github.com/plexusone/omnivoice) | Voice STT/TTS interfaces |
| [omniobserve](https://github.com/plexusone/omniobserve) | LLM observability |
| [omniserp](https://github.com/plexusone/omniserp) | Web search via Serper/SerpAPI |
| [wazero](https://github.com/tetratelabs/wazero) | WASM runtime for sandboxing |
| [moby](https://github.com/moby/moby) | Docker SDK for container isolation |
| [Rod](https://github.com/go-rod/rod) | Browser automation |
| [gorilla/websocket](https://github.com/gorilla/websocket) | WebSocket server |

## Related Projects

- [omnichat](https://github.com/plexusone/omnichat) - Unified messaging provider interface
- [omnillm](https://github.com/plexusone/omnillm) - Multi-provider LLM abstraction
- [omnivoice](https://github.com/plexusone/omnivoice) - Voice interactions
- [OpenClaw](https://github.com/openclaw/openclaw) - Compatible skill format

## License

MIT License - see [LICENSE](LICENSE) for details.
