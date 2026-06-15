# OmniAgent

Your AI representative across communication channels.

OmniAgent is a personal AI assistant that routes messages across multiple communication platforms, processes them via an AI agent, and responds on your behalf.

## Features

- 💬 **Multi-Channel Support** - Telegram, Discord, Slack, WhatsApp, and more
- 🤖 **AI-Powered Responses** - Powered by omnillm (Claude, GPT, Gemini, OpenRouter)
- 🎤 **Voice Notes** - Transcribe incoming voice, respond with synthesized speech via OmniVoice
- 🧩 **Skills System** - Markdown skills (OpenClaw compatible) and compiled Go skills
- 🧠 **Semantic Memory** - Store and retrieve information with vector search via omniretrieve
- 📋 **Workboard** - Project management with cards, columns, and dependencies
- 💾 **Persistent Sessions** - Conversation history with SQLite storage via omnistorage-core
- ⏰ **Scheduled Jobs** - Cron expressions, intervals, and one-time job scheduling
- 🔒 **Secure Sandboxing** - WASM and Docker isolation with GPU passthrough
- 🌐 **Browser Automation** - Built-in browser control with dialog handling via Rod
- 🔌 **WebSocket Gateway** - Real-time control plane with tools RPC endpoint
- 📊 **Observability** - Integrated tracing via omniobserve
- 🎭 **Agent Profiles** - Bootstrap profiles, lean mode, and auto-reply with commentary
- 🛡️ **Access Policies** - Per-sender tool access control and channel conformance
- 🔐 **Vault Credentials** - Secure credential storage via 1Password, Bitwarden, Keeper

## Quick Start

### Installation

```bash
go install github.com/plexusone/omniagent/cmd/omniagent@latest
```

### WhatsApp + OpenAI

The fastest way to get started:

```bash
# Set your OpenAI API key
export OPENAI_API_KEY="sk-..."

# Run with WhatsApp enabled
OMNIAGENT_AGENT_PROVIDER=openai \
OMNIAGENT_AGENT_MODEL=gpt-4o \
WHATSAPP_ENABLED=true \
omniagent gateway run
```

A QR code will appear in your terminal. Scan it with WhatsApp to connect.

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

## Dependencies

| Package | Purpose |
|---------|---------|
| [omnichat](https://github.com/plexusone/omnichat) | Unified messaging (WhatsApp, Telegram, Discord) |
| [omnillm](https://github.com/plexusone/omnillm) | Multi-provider LLM abstraction |
| [omni-openrouter](https://github.com/plexusone/omni-openrouter) | OpenRouter provider with OAuth PKCE |
| [omniskill](https://github.com/plexusone/omniskill) | Skill infrastructure with ClawHub marketplace |
| [omniserp](https://github.com/plexusone/omniserp) | Multi-provider search (Brave, Exa.ai, Serper) |
| [omniretrieve](https://github.com/plexusone/omniretrieve) | Vector search, BM25, semantic memory |
| [omniworkboard](https://github.com/plexusone/omniworkboard) | Project management workboard |
| [omnivoice](https://github.com/plexusone/omnivoice) | Voice STT/TTS interfaces |
| [omniobserve](https://github.com/plexusone/omniobserve) | LLM observability |
| [omnistorage-core](https://github.com/plexusone/omnistorage-core) | Key-value and object storage |
| [omnivault](https://github.com/plexusone/omnivault) | Secure credential storage |
| [omnitoken](https://github.com/plexusone/omnitoken) | OAuth token management |
| [robfig/cron](https://github.com/robfig/cron) | Cron expression parsing and scheduling |
| [wazero](https://github.com/tetratelabs/wazero) | WASM runtime for sandboxing |
| [moby](https://github.com/moby/moby) | Docker SDK for container isolation |
| [Rod](https://github.com/go-rod/rod) | Browser automation |

## License

MIT License - see [LICENSE](https://github.com/plexusone/omniagent/blob/main/LICENSE) for details.
