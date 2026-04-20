# Getting Started

This guide will help you get OmniAgent up and running quickly.

## Installation

### From Source

```bash
go install github.com/plexusone/omniagent/cmd/omniagent@latest
```

### Verify Installation

```bash
omniagent version
```

## Quick Start with WhatsApp

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

A QR code will appear in your terminal. Scan it with WhatsApp (Settings → Linked Devices → Link a Device) to connect.

## Configuration File

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

## Using Different LLM Providers

### Anthropic (Claude)

```bash
export ANTHROPIC_API_KEY="sk-ant-..."

OMNIAGENT_AGENT_PROVIDER=anthropic \
OMNIAGENT_AGENT_MODEL=claude-sonnet-4-20250514 \
WHATSAPP_ENABLED=true \
omniagent gateway run
```

### Google Gemini

```bash
export GEMINI_API_KEY="..."

OMNIAGENT_AGENT_PROVIDER=gemini \
OMNIAGENT_AGENT_MODEL=gemini-2.0-flash \
WHATSAPP_ENABLED=true \
omniagent gateway run
```

## Testing & Debugging

Before connecting messaging channels, verify the agent is running correctly using the WebSocket gateway directly.

### Health Check

```bash
curl http://localhost:8080/health
```

Expected response: `{"status":"ok"}`

### WebSocket Smoke Test

Install websocat if needed:

```bash
brew install websocat  # macOS
# or: cargo install websocat
```

Connect to the WebSocket endpoint:

```bash
websocat ws://localhost:8080/ws
```

Then send JSON messages interactively:

```json
{"type":"ping"}
{"type":"chat","content":"What tools do you have?"}
{"type":"chat","content":"Search for recent AAPL news"}
```

### One-Liner Test

For quick validation in scripts or CI:

```bash
echo '{"type":"chat","content":"Search for recent AAPL news"}' | websocat ws://localhost:8080/ws
```

This tests the built-in `web_search` tool which supports web, news, and image searches via omniserp.

Expected response format:

```json
{"id":"...","type":"response","content":"Here are the recent news articles about AAPL...","timestamp":"..."}
```

### Debugging Tips

1. **Agent not responding?** Check that `OPENAI_API_KEY` (or your provider's key) is set
2. **Search tool failing?** Set `SERPER_API_KEY` or `SERPAPI_API_KEY` for web search
3. **Tool calls failing?** Verify compiled skills are registered in the agent logs
4. **Connection refused?** Ensure `STORAGE_PATH` points to a writable location (default `/data` requires root)

Set environment variables for local development:

```bash
export STORAGE_PATH="./omniagent.db"
export SERPER_API_KEY="your_serper_key"  # For web_search tool
```

## Next Steps

- [WhatsApp Setup Guide](guides/whatsapp.md) - Detailed WhatsApp configuration
- [Voice Integration](guides/voice.md) - Enable voice notes
- [Skills Development](guides/skills.md) - Create custom skills
- [Configuration Reference](reference/configuration.md) - Full configuration options
