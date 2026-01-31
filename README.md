# Envoy

Your AI representative across communication channels.

Envoy is a personal AI assistant that routes messages across multiple communication platforms, processes them via an AI agent, and responds on your behalf.

## Features

- **Multi-Channel Support** - Telegram, Discord, Slack, WhatsApp, and more
- **AI-Powered Responses** - Powered by omnillm (Claude, GPT, Gemini, etc.)
- **Browser Automation** - Built-in browser control via Rod
- **WebSocket Gateway** - Real-time control plane for device connections
- **Observability** - Integrated tracing via omniobserve

## Installation

```bash
go install github.com/agentplexus/envoy/cmd/envoy@latest
```

## Quick Start

1. Create a configuration file:

```yaml
# envoy.yaml
gateway:
  address: "127.0.0.1:18789"

agent:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: ${ANTHROPIC_API_KEY}

channels:
  telegram:
    enabled: true
    token: ${TELEGRAM_BOT_TOKEN}
```

2. Start the gateway:

```bash
envoy gateway run --config envoy.yaml
```

## CLI Commands

```bash
envoy gateway run      # Start the gateway server
envoy channels list    # List registered channels
envoy channels status  # Show channel connection status
envoy config show      # Display current configuration
envoy version          # Show version information
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Messaging Channels                       │
│     Telegram  │  Discord  │  Slack  │  WhatsApp  │  ...    │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│              Gateway (WebSocket Control Plane)              │
│              ws://127.0.0.1:18789                           │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                      Agent Runtime                          │
│  • omnillm (LLM providers)                                  │
│  • omniobserve (tracing)                                    │
│  • Tools (browser, shell, http)                             │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

Envoy can be configured via:

- YAML/JSON configuration file
- Environment variables
- CLI flags

See [Configuration Reference](docs/configuration.md) for details.

## Dependencies

| Package | Purpose |
|---------|---------|
| [omnillm](https://github.com/agentplexus/omnillm) | Multi-provider LLM abstraction |
| [omniobserve](https://github.com/agentplexus/omniobserve) | LLM observability |
| [Rod](https://github.com/go-rod/rod) | Browser automation |
| [gorilla/websocket](https://github.com/gorilla/websocket) | WebSocket server |

## Related Projects

- [omnichat](https://github.com/agentplexus/omnichat) - Channel abstraction (planned)
- [omnibrowser](https://github.com/agentplexus/omnibrowser) - Browser abstraction (planned)
- [omnivoice](https://github.com/agentplexus/omnivoice) - Voice interactions

## License

MIT License - see [LICENSE](LICENSE) for details.
