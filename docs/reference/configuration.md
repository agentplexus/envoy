# Configuration Reference

Complete reference for OmniAgent configuration options.

## Configuration File

OmniAgent uses YAML or JSON configuration files:

```bash
omniagent gateway run --config omniagent.yaml
```

## Gateway

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `gateway.address` | string | `127.0.0.1:18789` | WebSocket server address |
| `gateway.read_timeout` | duration | `30s` | Read timeout |
| `gateway.write_timeout` | duration | `30s` | Write timeout |
| `gateway.ping_interval` | duration | `30s` | WebSocket ping interval |

```yaml
gateway:
  address: "127.0.0.1:18789"
  read_timeout: 30s
  write_timeout: 30s
  ping_interval: 30s
```

## Agent

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `agent.provider` | string | `anthropic` | LLM provider |
| `agent.model` | string | `claude-sonnet-4-20250514` | Model name |
| `agent.api_key` | string | - | API key (or use env var) |
| `agent.temperature` | float | `0.7` | Sampling temperature |
| `agent.max_tokens` | int | `4096` | Max response tokens |
| `agent.system_prompt` | string | - | Custom system prompt |

```yaml
agent:
  provider: openai
  model: gpt-4o
  api_key: ${OPENAI_API_KEY}
  temperature: 0.7
  max_tokens: 4096
  system_prompt: "You are OmniAgent, responding on behalf of the user."
```

### Supported Providers

| Provider | Models |
|----------|--------|
| `openai` | `gpt-4o`, `gpt-4-turbo`, `gpt-3.5-turbo` |
| `anthropic` | `claude-sonnet-4-20250514`, `claude-3-opus-20240229` |
| `gemini` | `gemini-2.0-flash`, `gemini-1.5-pro` |

## Multi-Agent Configuration

Configure multiple agents with different models and tool access:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `agents` | []AgentConfig | `[]` | List of agent configurations |
| `agents[].id` | string | - | Unique agent identifier (required) |
| `agents[].name` | string | - | Human-readable name |
| `agents[].description` | string | - | Agent description |
| `agents[].provider` | string | (from agent) | LLM provider |
| `agents[].model` | string | (from agent) | Model name |
| `agents[].api_key` | string | (from agent) | API key |
| `agents[].base_url` | string | - | Custom API endpoint |
| `agents[].temperature` | float | `0.7` | Sampling temperature |
| `agents[].max_tokens` | int | `4096` | Max response tokens |
| `agents[].system_prompt` | string | - | Custom system prompt |
| `agents[].allowed_tools` | []string | - | Whitelist of allowed tools |
| `agents[].denied_tools` | []string | - | Blacklist of denied tools |
| `agents[].enabled` | bool | `true` | Whether agent is active |

```yaml
# Default agent settings (used as fallback)
agent:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: ${ANTHROPIC_API_KEY}

# Multiple agent configurations
agents:
  - id: general
    name: General Assistant
    # Inherits from agent section

  - id: research
    name: Research Agent
    provider: openai
    model: gpt-4o
    api_key: ${OPENAI_API_KEY}
    system_prompt: You are a research assistant.
    allowed_tools:
      - web_search
      - read_url

  - id: coder
    name: Coding Agent
    system_prompt: You are a senior software engineer.
    denied_tools:
      - web_search
```

See [Multi-Agent Guide](../guides/multi-agent.md) for detailed usage.

## Channels

### WhatsApp

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `channels.whatsapp.enabled` | bool | `false` | Enable WhatsApp |
| `channels.whatsapp.db_path` | string | `whatsapp.db` | Session database |

```yaml
channels:
  whatsapp:
    enabled: true
    db_path: "whatsapp.db"
```

### Telegram

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `channels.telegram.enabled` | bool | `false` | Enable Telegram |
| `channels.telegram.token` | string | - | Bot token |

```yaml
channels:
  telegram:
    enabled: true
    token: ${TELEGRAM_BOT_TOKEN}
```

### Discord

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `channels.discord.enabled` | bool | `false` | Enable Discord |
| `channels.discord.token` | string | - | Bot token |

```yaml
channels:
  discord:
    enabled: true
    token: ${DISCORD_BOT_TOKEN}
```

## Skills

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `skills.enabled` | bool | `true` | Enable skill loading |
| `skills.paths` | []string | `[]` | Additional skill directories |
| `skills.disabled` | []string | `[]` | Skills to skip |
| `skills.max_injected` | int | `20` | Max skills in prompt |

```yaml
skills:
  enabled: true
  paths:
    - ~/.omniagent/skills
    - /opt/shared-skills
  disabled:
    - experimental-skill
  max_injected: 20
```

## Storage

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `storage.type` | string | `memory` | Backend type: `memory`, `sqlite` |
| `storage.path` | string | - | Database path (for sqlite) |

```yaml
storage:
  type: sqlite
  path: /data/omniagent.db
```

### Storage Backends

| Type | Persistence | Use Case |
|------|-------------|----------|
| `memory` | None | Development, testing |
| `sqlite` | Disk | Production, single instance |

## Sessions

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `sessions.enabled` | bool | `true` | Enable session persistence |
| `sessions.ttl` | duration | `168h` | Session time-to-live (7 days) |

```yaml
sessions:
  enabled: true
  ttl: 168h  # 7 days
```

!!! note "Programmatic Configuration"
    Storage and sessions are currently configured programmatically:
    ```go
    backend, _ := sqlite.New(sqlite.Config{Path: "data.db"})
    agent.New(config, agent.WithSessionsFromStorage(backend))
    ```
    YAML configuration support is planned.

## Voice

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `voice.enabled` | bool | `false` | Enable voice processing |
| `voice.response_mode` | string | `auto` | `auto`, `always`, `never` |
| `voice.stt.provider` | string | - | STT provider |
| `voice.stt.model` | string | - | STT model |
| `voice.tts.provider` | string | - | TTS provider |
| `voice.tts.model` | string | - | TTS model |
| `voice.tts.voice_id` | string | - | TTS voice ID |

```yaml
voice:
  enabled: true
  response_mode: auto
  stt:
    provider: deepgram
    model: nova-2
  tts:
    provider: deepgram
    model: aura-asteria-en
    voice_id: aura-asteria-en
```

### Voice Providers

| Provider | STT Models | TTS Models |
|----------|------------|------------|
| `deepgram` | `nova-2` | `aura-asteria-en`, `aura-luna-en` |
| `openai` | `whisper-1` | `tts-1`, `tts-1-hd` |
| `elevenlabs` | - | Various voice IDs |

## Tokens

OAuth token management for services requiring refresh tokens.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `tokens.vault_uri` | string | - | Vault URI for storing tokens |
| `tokens.services` | map | - | Service configurations |

```yaml
tokens:
  vault_uri: "op://MyVault"
  services:
    google:
      credentials_name: "google-oauth"
      scopes:
        - "https://www.googleapis.com/auth/calendar"
    zoom:
      credentials_name: "zoom-oauth"
```

### Service Configuration

| Field | Type | Description |
|-------|------|-------------|
| `credentials_name` | string | Credential name in vault (defaults to service name) |
| `scopes` | []string | OAuth scopes to request |
| `auto_refresh` | bool | Auto-refresh tokens (default: true) |

## Image Generation

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image.enabled` | bool | `false` | Enable image generation |
| `image.provider` | string | `openai` | Provider: `openai`, `fal` |
| `image.model` | string | - | Default model |
| `image.api_key` | string | - | API key (overrides provider-specific) |
| `image.base_url` | string | - | Custom API base URL |

```yaml
image:
  enabled: true
  provider: openai
  model: gpt-image-2
  api_key: ${OPENAI_API_KEY}
```

### Image Providers

| Provider | Models |
|----------|--------|
| `openai` | `dall-e-3`, `dall-e-2`, `gpt-image-2` |
| `fal` | `fal-ai/flux-pro`, `fal-ai/flux-dev`, `fal-ai/flux-schnell` |

See [Image Generation Guide](../guides/images.md) for detailed usage.

## Observability

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `observability.enabled` | bool | `false` | Enable observability |
| `observability.service_name` | string | `omniagent` | Service name for traces |
| `observability.provider` | string | - | Provider: `otlp`, `jaeger` |

```yaml
observability:
  enabled: true
  service_name: "my-agent"
  provider: otlp
  otlp:
    endpoint: "localhost:4317"
```

## Environment Variable Expansion

Configuration values support environment variable expansion:

```yaml
agent:
  api_key: ${OPENAI_API_KEY}
  model: ${OMNIAGENT_MODEL:-gpt-4o}  # With default
```

## Vault-Backed Credentials

Credentials can be stored in password managers using URI schemes:

| Scheme | Provider | Example |
|--------|----------|---------|
| `op://` | 1Password | `op://MyVault/item/field` |
| `bw://` | Bitwarden | `bw://org-id/item-name` |
| `keeper://` | Keeper | `keeper://folder/record/field` |
| `file://` | File | `file:///path/to/secret` |
| `env://` | Environment | `env://VAR_NAME` |

```yaml
agent:
  api_key: "op://MyVault/anthropic/api-key"

channels:
  telegram:
    token: "bw://org-id/telegram-bot-token"
  discord:
    token: "keeper://Discord/bot-token"
```

Credentials are resolved once at startup. Plain string values still work.

## Complete Example

```yaml
# omniagent.yaml
gateway:
  address: "127.0.0.1:18789"
  read_timeout: 30s
  write_timeout: 30s

agent:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: ${ANTHROPIC_API_KEY}
  temperature: 0.7
  max_tokens: 4096
  system_prompt: |
    You are OmniAgent, an AI assistant responding on behalf of the user.
    Be helpful, concise, and professional.

storage:
  type: sqlite
  path: /data/omniagent.db

sessions:
  enabled: true
  ttl: 168h  # 7 days

channels:
  whatsapp:
    enabled: true
    db_path: "whatsapp.db"
  telegram:
    enabled: false
    token: ${TELEGRAM_BOT_TOKEN}
  discord:
    enabled: false
    token: ${DISCORD_BOT_TOKEN}

skills:
  enabled: true
  paths:
    - ~/.omniagent/skills
  max_injected: 20

voice:
  enabled: true
  response_mode: auto
  stt:
    provider: deepgram
    model: nova-2
  tts:
    provider: deepgram
    model: aura-asteria-en
    voice_id: aura-asteria-en
```
