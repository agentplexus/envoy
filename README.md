# OmniAgent

[![OmniAgent Mascot - The Versatile Golang Agent](docs/images/img_omniagent_hero_v4.png)](https://plexusone.dev/omniagent/)

[![Go CI][go-ci-svg]][go-ci-url]
[![Go Lint][go-lint-svg]][go-lint-url]
[![Go SAST][go-sast-svg]][go-sast-url]
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
- 📞 **Full-Duplex Phone Calls** - Real-time phone conversations via Twilio Media Streams
- ⚡ **Native Voice-to-Voice** - Ultra-low latency (~100ms) via OpenAI Realtime or Gemini Live APIs
- 🧩 **Skills System** - Markdown skills (OpenClaw compatible) and compiled Go skills
- 💾 **Persistent Sessions** - Conversation history with SQLite storage via omnistorage-core
- 🧠 **Semantic Memory** - Multi-backend memory with automatic recall via omnimemory
- ⏰ **Scheduled Jobs** - Cron expressions, intervals, and one-time job scheduling
- 🔒 **Secure Sandboxing** - WASM and Docker isolation with GPU passthrough
- 🌐 **Browser Automation** - Built-in browser control with dialog handling via Rod
- 🔌 **WebSocket Gateway** - Real-time control plane with tools RPC endpoint
- 📊 **Observability** - Integrated tracing via omniobserve
- 🎭 **Agent Profiles** - Bootstrap profiles and lean mode for resource optimization
- 🛡️ **Access Policies** - Per-sender tool access control and channel conformance
- 🔐 **Vault Credentials** - Secure credential storage via 1Password, Bitwarden, Keeper
- 🔗 **OpenAI-Compatible API** - Drop-in replacement for OpenAI client libraries with SSE streaming
- 🖼️ **Image Generation** - AI image generation via OpenAI (DALL-E) or Fal AI (FLUX)
- 👥 **Multi-Agent Support** - Run multiple agents with different models and configurations
- 📈 **Usage Analytics** - Token usage tracking, tool call statistics, and cost estimation
- 🔧 **Tool Visualization** - Real-time tool call display with arguments and results in web UI

## Ways to Connect

OmniAgent supports multiple connection methods for different use cases:

| Method | Protocol | Use Case | Latency |
|--------|----------|----------|---------|
| **WhatsApp** | WebSocket | Personal messaging, voice notes | Text: instant, Voice: ~2s |
| **Telegram/Discord/Slack** | Bot API | Team messaging, notifications | Instant |
| **OpenAI-Compatible API** | HTTP/SSE | Programmatic access, web apps | Streaming |
| **Phone Calls** | Twilio/Telnyx | PSTN voice conversations | Traditional: 500ms+, Realtime: ~100ms |
| **LiveKit (WebRTC)** | WebRTC | Browser/mobile voice, meetings | ~100-300ms |

**Quick links:**

- [WhatsApp Setup](#quick-start) - Scan QR code to connect
- [OpenAI API](#openai-compatible-api) - Use any OpenAI client library
- [Voice Gateway](#voice-gateway) - Phone calls via Twilio/Telnyx
- [LiveKit Agents](#livekit-voice-agents) - WebRTC voice in meetings

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
    messaging_service_sid: ${TWILIO_MESSAGING_SERVICE_SID}  # Optional: enables RCS with SMS/MMS fallback
    webhook_path: /webhook/twilio/sms  # Supports SMS, MMS, and RCS

voice:
  enabled: true
  response_mode: auto        # auto, always, never

  # Option 1: Native voice-to-voice (lowest latency, ~100ms)
  realtime:
    provider: openai         # or: gemini
    voice: alloy             # OpenAI: alloy, nova, etc. Gemini: Puck, Charon, etc.

  # Option 2: Traditional pipeline (custom STT/TTS providers)
  # stt:
  #   provider: deepgram
  #   model: nova-2
  # tts:
  #   provider: elevenlabs
  #   voice_id: your-voice-id

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

## OpenAI-Compatible API

OmniAgent exposes an OpenAI-compatible REST API, allowing you to use standard OpenAI client libraries:

```bash
# Start the gateway with API enabled
omniagent gateway run --config omniagent.yaml

# Use with any OpenAI client
curl http://localhost:18789/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "omniagent",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

### API Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /openai/v1/chat/completions` | Chat completions with SSE streaming |
| `GET /openai/v1/models` | List available models |
| `POST /openai/v1/images/generations` | Generate images from prompt |
| `POST /openai/v1/images/edits` | Edit images with mask |
| `POST /openai/v1/images/variations` | Create image variations |
| `GET /api/v1/tools` | List registered tools |
| `GET /api/v1/agents` | List configured agents |
| `POST /api/v1/agents` | Create a new agent |
| `GET /api/v1/cron/jobs` | List scheduled jobs |
| `GET /api/health` | Health check |
| `GET /docs` | Interactive API documentation (Scalar) |
| `GET /api/openapi.json` | OpenAPI 3.1 specification |

### Python Example

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:18789/openai/v1",
    api_key="your-api-key"
)

response = client.chat.completions.create(
    model="omniagent",
    messages=[{"role": "user", "content": "What tools do you have?"}],
    stream=True
)

for chunk in response:
    print(chunk.choices[0].delta.content, end="")
```

See [OpenAI API Guide](docs/guides/openai-api.md) for detailed documentation.

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

For better performance and type safety, register Go functions as LLM tools:

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/skills/compiled"
)

// Create a skill with tools
type WeatherSkill struct{}

func (s *WeatherSkill) Name() string        { return "weather" }
func (s *WeatherSkill) Description() string { return "Weather forecasts" }
func (s *WeatherSkill) Tools() []compiled.Tool {
    return []compiled.Tool{{
        Name:        "get_weather",
        Description: "Get weather for a location",
        Parameters: map[string]compiled.Parameter{
            "location": {Type: "string", Required: true},
        },
        Handler: func(ctx context.Context, params map[string]any) (any, error) {
            return fetchWeather(params["location"].(string))
        },
    }}
}
func (s *WeatherSkill) Init(ctx context.Context) error { return nil }
func (s *WeatherSkill) Close() error                   { return nil }

// Register with agent
agent.New(config, agent.WithCompiledSkill(&WeatherSkill{}))
```

### Remote Skills

Remote skills connect to external services and expose their capabilities as agent tools.

**MCP Skills** - Spawn external MCP servers and expose their tools:

```go
import "github.com/plexusone/omniagent/skills/remote/mcp"

agent, err := agent.New(config,
    agent.WithMCPSkill(mcp.Config{
        Name:    "github",
        Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"},
        Env: map[string]string{
            "GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN"),
        },
    }),
)
```

**OpenAPI Skills** - Parse OpenAPI 3.x specs and expose operations as tools:

```go
import openapi "github.com/plexusone/omniagent/skills/remote/openapi"

agent, err := agent.New(config,
    agent.WithOpenAPISkill(openapi.Config{
        Name:    "petstore",
        SpecURL: "https://petstore3.swagger.io/api/v3/openapi.json",
        Auth: openapi.AuthConfig{
            Type:   openapi.AuthBearer,
            Token:  os.Getenv("API_TOKEN"),
        },
    }),
)
```

See the [Skills Guide](docs/guides/skills.md#remote-skills) for configuration options.

## Roles

Roles are high-level agent personas that combine skills, workflows, and system prompts into cohesive behaviors. They separate organizational responsibilities from runtime implementations.

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/agent/roles"
    facilitator "github.com/plexusone/omnirole-facilitator"
)

// Create a role with configuration
pmRole := facilitator.New(facilitator.Config{
    DefaultConfluenceSpace: "TEAM",
    EnableActionTracking:   true,
})

// Create role manager with skills
mgr, _ := roles.NewManager(pmRole, meetingSkill, googleSkill, confluenceSkill)
mgr.Init(ctx)
defer mgr.Close()

// Access role capabilities
prompt, _ := mgr.SystemPrompt(ctx)
workflows := mgr.Workflows()
spec := mgr.Spec()

// Use policy enforcement
if err := mgr.CheckToolAccess(ctx, "confluence_publish"); err != nil {
    // Tool access denied by policy
}

// Context-aware behaviors
mgr.SetBehaviorContext(role.BehaviorContextMeeting)
behaviors := mgr.GetActiveBehaviors(ctx)

// Track metrics
mgr.RecordMetric(ctx, "meetings-facilitated", 1)
```

### Available Roles

| Role | Package | Description |
|------|---------|-------------|
| Meeting PM | `github.com/plexusone/omnirole-facilitator` | Meeting facilitation, notes, action tracking |

### Role Features

| Feature | Description |
|---------|-------------|
| **Behaviors** | Context-aware actions (meeting, chat, autonomous) |
| **Policies** | Tool access control, data access, rate limits |
| **Metrics** | KPIs and success measurements |
| **Delegation** | Sub-agent orchestration |
| **Workflows** | Structured multi-step operations |

See the [Roles Guide](docs/guides/roles.md) for complete documentation.

## Sessions

OmniAgent supports persistent conversation sessions:

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omnistorage-core/kvs/backend/sqlite"
)

// Create storage backend
backend, _ := sqlite.New(sqlite.Config{Path: "omniagent.db"})

// Create agent with sessions
a, _ := agent.New(config,
    agent.WithSessionsFromStorage(backend),
)

// Process with conversation history
response1, _ := a.ProcessWithSession(ctx, "user-123", "My name is Alice")
response2, _ := a.ProcessWithSession(ctx, "user-123", "What's my name?")
// Agent remembers: "Your name is Alice"
```

See [Sessions Guide](docs/guides/sessions.md) for details.

## Scheduled Jobs

OmniAgent supports scheduled job execution via the cron package:

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omnistorage-core/kvs/backend/sqlite"
)

// Create agent with cron support
backend, _ := sqlite.New(sqlite.Config{Path: "omniagent.db"})
a, _ := agent.New(config,
    agent.WithSessionsFromStorage(backend),
    agent.WithCronScheduler(),
)
```

The LLM can then create scheduled jobs via tool calls:

| Tool | Description |
|------|-------------|
| `cron_create` | Create a new scheduled job |
| `cron_list` | List all jobs (filterable by status) |
| `cron_get` | Get job details |
| `cron_delete` | Delete a job |
| `cron_enable` | Enable a disabled job |
| `cron_disable` | Disable without deleting |
| `cron_trigger` | Run job immediately |

Schedule types:

- **Cron expressions**: `0 0 9 * * *` (9am daily, with seconds)
- **Intervals**: `1h`, `30m`, `24h`
- **One-time**: RFC3339 timestamp for single execution

Action types:

- `send_message` - Send a message to a session
- `call_webhook` - Make an HTTP request
- `call_tool` - Invoke a registered tool

See [Cron Guide](docs/guides/cron.md) for details.

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

### GPU Passthrough

For GPU-accelerated workloads, enable NVIDIA GPU passthrough:

```go
sandbox, _ := sandbox.NewDockerSandbox(ctx, sandbox.DockerConfig{
    Image: "nvidia/cuda:12.0-base",
    GPU: &sandbox.GPUConfig{
        Enabled:      true,
        DeviceIDs:    []string{"0"},
        Capabilities: []string{"compute", "utility"},
    },
})
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

## Agent Profiles

Profiles customize agent behavior for different use cases:

```go
import "github.com/plexusone/omniagent/agent/profiles"

profile := &profiles.BootstrapProfile{
    Name:               "customer-support",
    SystemPromptPrefix: "You are a customer support agent.\n",
    AllowedTools:       []string{"search_kb", "create_ticket"},
    DeniedTools:        []string{"shell", "browser"},
}

a, _ := agent.New(config, agent.WithProfile(profile))
```

### Lean Mode

Optimize for constrained environments:

```go
leanMode := profiles.NewLeanMode(profiles.LeanLevelModerate)
a, _ := agent.New(config, agent.WithLeanMode(leanMode))
```

| Level | Memory Reduction | Use Case |
|-------|------------------|----------|
| `Off` | None | Default operation |
| `Light` | ~15% | Slightly constrained |
| `Moderate` | ~35% | Mobile/embedded |
| `Aggressive` | ~60% | Severely constrained |

See [Agent Profiles Guide](docs/guides/profiles.md) for details.

## Access Policies

### Tool Policies

Control which tools are available per sender:

```go
import "github.com/plexusone/omniagent/tools/policy"

manager := policy.NewManager()
manager.SetPolicy("guest", &policy.Policy{
    AllowedTools: []string{"search", "weather"},
    DeniedTools:  []string{"shell", "browser"},
    RateLimit: &policy.RateLimit{
        MaxCalls: 10,
        Window:   time.Minute,
    },
})
```

### Channel Policies

Validate messages against content rules:

```go
import "github.com/plexusone/omniagent/channels/policy"

checker := policy.NewConformanceChecker(config)
checker.AddRule(policy.ConformanceRule{
    Name:    "rate-limit",
    Action:  policy.ActionRateLimit,
    RateLimit: &policy.RateLimit{MaxMessages: 60, Window: time.Minute},
})
```

See [Access Policies Guide](docs/guides/policies.md) for details.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `GEMINI_API_KEY` | Google Gemini API key |
| `OPENROUTER_API_KEY` | OpenRouter API key |
| `OMNIAGENT_AGENT_PROVIDER` | LLM provider: `openai`, `anthropic`, `gemini`, `openrouter` |
| `OMNIAGENT_AGENT_MODEL` | Model name (e.g., `gpt-4o`, `claude-sonnet-4-20250514`) |
| `WHATSAPP_ENABLED` | Set to `true` to enable WhatsApp |
| `WHATSAPP_DB_PATH` | WhatsApp session storage path |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token (auto-enables Telegram) |
| `DISCORD_BOT_TOKEN` | Discord bot token (auto-enables Discord) |
| `TWILIO_ACCOUNT_SID` | Twilio Account SID (auto-enables SMS/MMS/RCS) |
| `TWILIO_AUTH_TOKEN` | Twilio Auth Token |
| `TWILIO_PHONE_NUMBER` | Twilio phone number in E.164 format |
| `TWILIO_MESSAGING_SERVICE_SID` | Messaging Service SID for RCS (enables RCS with SMS/MMS fallback) |
| `TWILIO_WEBHOOK_PATH` | SMS webhook path (default: `/webhook/twilio/sms`) |
| `SERPER_API_KEY` | Serper API key for web search |
| `SERPAPI_API_KEY` | SerpAPI key for web search (alternative) |
| `DEEPGRAM_API_KEY` | Deepgram API key for voice STT/TTS (traditional) |
| `OMNIAGENT_VOICE_ENABLED` | Set to `true` to enable voice processing |
| `OMNIAGENT_VOICE_RESPONSE_MODE` | Voice response mode: `auto`, `always`, `never` |
| `OMNIAGENT_VOICE_REALTIME_PROVIDER` | Native voice-to-voice: `openai`, `gemini`, `deepgram` |
| `ELEVENLABS_API_KEY` | ElevenLabs API key for voice TTS (traditional) |
| `GOOGLE_API_KEY` | Google API key for Gemini Live (native voice-to-voice) |
| `IMAGE_ENABLED` | Set to `true` to enable image generation |
| `IMAGE_PROVIDER` | Image provider: `openai`, `fal` (default: `openai`) |
| `IMAGE_MODEL` | Default image model (e.g., `gpt-image-2`, `fal-ai/flux-pro`) |
| `FAL_KEY` | Fal AI API key for image generation |

## Vault-Backed Credentials

OmniAgent supports storing credentials in password managers via [omnivault](https://github.com/plexusone/omnivault) and [omnitoken](https://github.com/plexusone/omnitoken).

### Supported Vault Providers

| Provider | URI Scheme | Environment Variable |
|----------|------------|---------------------|
| 1Password | `op://` | `OP_SERVICE_ACCOUNT_TOKEN` |
| Bitwarden | `bw://` | `BW_ACCESS_TOKEN`, `BW_ORGANIZATION_ID` |
| Keeper | `keeper://` | `KSM_TOKEN` or `KSM_CONFIG` |
| File | `file://` | - |
| Environment | `env://` | - |

### Static Credentials

API keys and tokens can be stored in vaults instead of config files:

```yaml
# omniagent.yaml
agent:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: "op://MyVault/anthropic/api-key"  # Resolved from 1Password

channels:
  telegram:
    enabled: true
    token: "bw://org-id/telegram-bot-token"  # Resolved from Bitwarden

  discord:
    enabled: true
    token: "keeper://Discord Bot/token"      # Resolved from Keeper

voice:
  enabled: true
  stt:
    provider: deepgram
    api_key: "op://MyVault/deepgram/api-key"
  tts:
    provider: deepgram
    api_key: "op://MyVault/deepgram/api-key"
```

Credentials are resolved once at startup. Plain string values still work for development.

### OAuth Token Management

For services requiring OAuth token refresh (Google, Zoom, RingCentral), use the `tokens` configuration:

```yaml
# omniagent.yaml
tokens:
  vault_uri: "op://MyVault"
  services:
    google:
      credentials_name: "google-service-account"
      scopes:
        - "https://www.googleapis.com/auth/calendar"
    zoom:
      credentials_name: "zoom-oauth"
    ringcentral:
      credentials_name: "ringcentral-oauth"
```

The token manager handles:

1. In-memory token caching
2. Automatic refresh when tokens expire
3. Vault coordination for multi-process deployments
4. Refresh token persistence

### Vault Environment Variables

| Variable | Provider | Description |
|----------|----------|-------------|
| `OP_SERVICE_ACCOUNT_TOKEN` | 1Password | Service account token (starts with `ops_`) |
| `BW_ACCESS_TOKEN` | Bitwarden | Access token |
| `BW_ORGANIZATION_ID` | Bitwarden | Organization ID |
| `BW_API_URL` | Bitwarden | Custom API URL (self-hosted) |
| `BW_IDENTITY_URL` | Bitwarden | Custom Identity URL (self-hosted) |
| `KSM_TOKEN` | Keeper | One-time token (format: `REGION:TOKEN`) |
| `KSM_CONFIG` | Keeper | Base64-encoded config JSON |
| `KSM_CONFIG_FILE` | Keeper | Path to config file |

## CLI Commands

```bash
# Gateway
omniagent gateway run      # Start the gateway server

# Voice (Full-Duplex Phone Calls)
omniagent voice serve      # Start the voice gateway server
omniagent voice status     # Show voice configuration status
omniagent voice call NUM   # Make an outbound call to NUM

# Skills
omniagent skills list      # List all discovered skills
omniagent skills info NAME # Show skill details
omniagent skills check     # Validate skill requirements

# Channels
omniagent channels list    # List registered channels
omniagent channels status  # Show channel connection status

# OpenAI API
omniagent openai spec      # Generate OpenAPI specification

# Config
omniagent config show      # Display current configuration

# Version
omniagent version          # Show version information
```

### Voice Gateway

Start a full-duplex voice gateway for phone calls:

```bash
# Native voice-to-voice (lowest latency, ~100ms)
omniagent voice serve \
  --listen :8081 \
  --public-url https://your-server.com \
  --realtime openai \
  --realtime-voice alloy

# Traditional pipeline (custom STT/TTS)
omniagent voice serve \
  --listen :8081 \
  --public-url https://your-server.com \
  --stt deepgram \
  --tts elevenlabs \
  --llm anthropic \
  --model claude-sonnet-4-20250514
```

Configure Twilio webhooks:

- Voice URL: `https://your-server.com/voice/inbound`
- Status Callback: `https://your-server.com/voice/status`

#### Local Development with ngrok

For local development, use ngrok to expose your local server to Twilio:

```bash
# Set ngrok auth token
export NGROK_AUTHTOKEN=your-ngrok-token

# Start voice server with ngrok tunnel (auto-generates public URL)
omniagent voice serve \
  --listen :8081 \
  --ngrok \
  --stt deepgram \
  --tts elevenlabs \
  --llm anthropic
```

The ngrok public URL will be displayed on startup. Configure this URL in your Twilio webhook settings.

With a custom ngrok domain (requires paid plan):

```bash
omniagent voice serve \
  --listen :8081 \
  --ngrok \
  --ngrok-domain myapp.ngrok.io
```

### LiveKit Voice Agents

Run OmniAgent as a voice participant in LiveKit meetings:

```bash
# Set credentials
export LIVEKIT_URL="wss://your-project.livekit.cloud"
export LIVEKIT_API_KEY="your-api-key"
export LIVEKIT_API_SECRET="your-api-secret"

# Option 1: Realtime mode (lowest latency, ~100-300ms)
export REALTIME_PROVIDER="deepgram"  # or: openai, gemini
export DEEPGRAM_API_KEY="your-deepgram-key"

# Option 2: Traditional pipeline (STT→LLM→TTS)
# export ANTHROPIC_API_KEY="your-anthropic-key"
# export STT_PROVIDER="deepgram"
# export STT_API_KEY="your-deepgram-key"
# export TTS_PROVIDER="openai"
# export TTS_API_KEY="your-openai-key"

# Optional: Enable avatar (displays image in video tile)
export AGENT_AVATAR="true"

# Run the generic voice agent
go run ./cmd/livekit-agent

# Or run the meeting facilitator (with Meeting PM role)
go run ./cmd/livekit-agent-facilitator
```

| Command | Description |
|---------|-------------|
| `cmd/livekit-agent` | Generic voice agent with web search |
| `cmd/livekit-agent-facilitator` | Meeting facilitator with Meeting PM role, supports realtime mode |

These agents use [omni-livekit](https://github.com/plexusone/omni-livekit) for LiveKit transport. The avatar is displayed at 640x360 (16:9) for optimal display in LiveKit video slots.

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
| `voice.realtime.provider` | string | - | Native voice-to-voice: `openai`, `gemini`, `deepgram` |
| `voice.realtime.voice` | string | - | Voice for realtime API |
| `voice.stt.provider` | string | - | STT provider (traditional): `deepgram`, `whisper` |
| `voice.tts.provider` | string | - | TTS provider (traditional): `elevenlabs`, `deepgram` |

## Omni* Library Ecosystem

OmniAgent is built on a modular ecosystem of `omni*` libraries:

```
                              OmniAgent
                          (Agent Runtime)
    ┌────────┬────────┬────────┬────────┬────────┬────────┬────────┐
    ▼        ▼        ▼        ▼        ▼        ▼        ▼        ▼
omnichat omnillm omnivoice omniimage omniobserve omniserp omnistorage ...
    │        │        │         │                          │
    │   ┌────┴────┐ ┌─┴──┐      │             ┌────────────┴────────┐
    │   │         │ │    │      │             │                     │
    ▼   ▼         ▼ ▼    ▼      ▼             ▼                     ▼
     omnillm-core  omnivoice-core          omnistorage-core
                                           ├── /object (files)
                                           └── /kvs (sessions)
    │        │        │         │                          │
    └────────┴────────┴─────────┴──────────────────────────┘
                        │
              Provider Modules
    ┌───────────────────┼───────────────────┐
    ▼                   ▼                   ▼
omni-aws           omni-google         omni-github
├── /omnillm       ├── /omnillm        └── /omnistorage
├── /omnistorage   └── /omnistorage
└── /omnivoice
```

See [Architecture Overview](docs/architecture/overview.md) for detailed documentation.

## Dependencies

### Omni* Libraries

| Package | Purpose |
|---------|---------|
| [omnichat](https://github.com/plexusone/omnichat) | Unified messaging (WhatsApp, Telegram, Discord) |
| [omnillm](https://github.com/plexusone/omnillm) | Multi-provider LLM abstraction |
| [omnivoice](https://github.com/plexusone/omnivoice) | Voice STT/TTS interfaces |
| [omni-twilio](https://github.com/plexusone/omni-twilio) | Full-duplex voice gateway via Twilio |
| [omniobserve](https://github.com/plexusone/omniobserve) | LLM observability |
| [omniserp](https://github.com/plexusone/omniserp) | Web search via Serper/SerpAPI |
| [omniimage](https://github.com/plexusone/omniimage) | Image generation (OpenAI, Fal AI) |
| [omnistorage-core](https://github.com/plexusone/omnistorage-core) | Object and key-value storage |
| [omnivault](https://github.com/plexusone/omnivault) | Secure credential storage |
| [omnitoken](https://github.com/plexusone/omnitoken) | OAuth token management |

### Infrastructure

| Package | Purpose |
|---------|---------|
| [wazero](https://github.com/tetratelabs/wazero) | WASM runtime for sandboxing |
| [moby](https://github.com/moby/moby) | Docker SDK for container isolation |
| [Rod](https://github.com/go-rod/rod) | Browser automation |
| [gorilla/websocket](https://github.com/gorilla/websocket) | WebSocket server |

## Related Projects

- [omnichat](https://github.com/plexusone/omnichat) - Unified messaging provider interface
- [omnillm](https://github.com/plexusone/omnillm) - Multi-provider LLM abstraction
- [omnivoice](https://github.com/plexusone/omnivoice) - Voice interactions
- [omniimage](https://github.com/plexusone/omniimage) - Image generation (OpenAI, Fal AI)
- [OpenClaw](https://github.com/openclaw/openclaw) - Compatible skill format

## License

MIT License - see [LICENSE](LICENSE) for details.
