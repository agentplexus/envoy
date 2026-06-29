# CLI Commands

Complete reference for OmniAgent CLI commands.

## Global Options

```bash
omniagent [command] --config <path>  # Specify config file
omniagent [command] --help           # Show help
```

## Gateway

### gateway run

Start the gateway server.

```bash
omniagent gateway run [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--config` | Path to config file |
| `--address` | Override gateway address |

**Examples:**

```bash
# Start with default settings
omniagent gateway run

# Start with config file
omniagent gateway run --config omniagent.yaml

# Start with custom address
omniagent gateway run --address 0.0.0.0:8080
```

## Voice

### voice serve

Start the voice gateway server for full-duplex phone calls.

```bash
omniagent voice serve [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--listen` | Listen address | `:8080` |
| `--public-url` | Public URL for Twilio webhooks | - |
| `--ngrok` | Use ngrok tunnel for public URL | `false` |
| `--ngrok-domain` | Custom ngrok domain (paid plan) | - |
| `--phone` | Twilio phone number for outbound | - |
| `--stt` | STT provider: `deepgram`, `whisper`, `google` | `deepgram` |
| `--tts` | TTS provider: `elevenlabs`, `openai`, `google` | `elevenlabs` |
| `--voice` | TTS voice ID | - |
| `--llm` | LLM provider: `anthropic`, `openai` | `anthropic` |
| `--model` | LLM model | `claude-sonnet-4-20250514` |
| `--system-prompt` | Custom system prompt | - |

**Examples:**

```bash
# With public server
omniagent voice serve \
  --listen :8081 \
  --public-url https://your-server.com \
  --stt deepgram \
  --tts elevenlabs \
  --llm anthropic

# With ngrok (local development)
omniagent voice serve --listen :8081 --ngrok

# With custom ngrok domain
omniagent voice serve --listen :8081 --ngrok --ngrok-domain myapp.ngrok.io

# With OpenAI
omniagent voice serve \
  --listen :8081 \
  --ngrok \
  --stt whisper \
  --tts openai \
  --llm openai \
  --model gpt-4o
```

### voice status

Show voice gateway configuration and provider status.

```bash
omniagent voice status
```

**Output:**

```
Voice Gateway Configuration
===========================

Twilio:
  Account SID:  ACxx...xxxx
  Auth Token:   Configured

Ngrok:
  Auth Token:   Configured (use --ngrok flag)

STT Providers:
  Deepgram:     Configured
  OpenAI:       Configured (Whisper)

TTS Providers:
  ElevenLabs:   Configured
  OpenAI:       Configured

LLM Providers:
  Anthropic:    Configured (Claude)
  OpenAI:       Configured (GPT-4o)
```

### voice call

Make an outbound call to a phone number.

```bash
omniagent voice call <phone-number> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--public-url` | Public URL for webhooks (required) |

**Examples:**

```bash
omniagent voice call +1234567890 --public-url https://your-server.com
```

## Skills

### skills list

List all discovered skills.

```bash
omniagent skills list
```

**Output:**

```
✓ 🎵 sonoscli - Control Sonos speakers via CLI
✓ 🐙 github - GitHub CLI operations
✗ ☀️ weather - Weather forecasts (missing: weather binary)
```

- `✓` - Skill is available (requirements met)
- `✗` - Skill unavailable (missing requirements)

### skills info

Show detailed information about a skill.

```bash
omniagent skills info <name>
```

**Example:**

```bash
omniagent skills info sonoscli
```

**Output:**

```
Name:        sonoscli
Description: Control Sonos speakers via CLI
Path:        /Users/john/.omniagent/skills/sonoscli
Emoji:       🎵

Requirements:
  Binaries: sonos
  Env Vars: (none)

Status: ✓ Available
```

### skills check

Check requirements for all skills.

```bash
omniagent skills check
```

**Output:**

```
Checking skill requirements...

✓ sonoscli
  - sonos binary: found at /usr/local/bin/sonos

✓ github
  - gh binary: found at /usr/local/bin/gh

✗ weather
  - weather binary: NOT FOUND
    Install: brew install weather

Summary: 2/3 skills available
```

## Channels

### channels list

List registered channels.

```bash
omniagent channels list
```

**Output:**

```
CHANNEL     ENABLED  STATUS
whatsapp    true     connected
telegram    false    -
discord     false    -
```

### channels status

Show detailed channel status.

```bash
omniagent channels status
```

**Output:**

```
WhatsApp:
  Status: connected
  Phone: +1 555-123-4567
  Session: whatsapp.db

Telegram:
  Status: disabled

Discord:
  Status: disabled
```

## OpenAI

### openai spec

Generate OpenAPI specification from the API server.

```bash
omniagent openai spec [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--output`, `-o` | Output file path | stdout |
| `--format`, `-f` | Output format: `yaml`, `json` | `yaml` |

**Examples:**

```bash
# Output to stdout (YAML)
omniagent openai spec

# Generate JSON file
omniagent openai spec --format json --output docs/api/openapi.json

# Generate YAML file
omniagent openai spec -f yaml -o docs/api/openapi.yaml
```

The generated spec includes:

- All API endpoints (chat completions, models, tools, agents, cron, usage)
- Request/response schemas
- Authentication requirements
- OpenAPI 3.1 compatibility

## Config

### config show

Display current configuration.

```bash
omniagent config show
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--format` | Output format: `yaml`, `json` |

**Example:**

```bash
omniagent config show --format json
```

## Version

### version

Show version information.

```bash
omniagent version
```

**Output:**

```
omniagent v0.4.0 (abc1234) built 2026-03-01 with go1.25 for darwin/arm64
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

```bash
omniagent version --json
```

```json
{
  "version": "0.4.0",
  "commit": "abc1234",
  "build_date": "2026-03-01",
  "go_version": "go1.25",
  "platform": "darwin/arm64"
}
```

## Setup

### setup

Interactive setup wizard for first-time configuration.

```bash
omniagent setup [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--output`, `-o` | Output config file path | `omniagent.yaml` |
| `--force` | Overwrite existing config | `false` |

The wizard guides you through:

1. **LLM Provider** - Choose Anthropic (Claude) or OpenAI (GPT)
2. **API Key** - Enter your provider API key
3. **System Prompt** - Optional custom system prompt
4. **Channels** - Configure Telegram, Discord, etc.
5. **Observability** - Optional OpenTelemetry setup

**Example:**

```bash
# Run interactive setup
omniagent setup

# Output to custom path
omniagent setup --output ~/.config/omniagent/config.yaml

# Overwrite existing config
omniagent setup --force
```

## Doctor

### doctor

Run diagnostic checks on your OmniAgent installation.

```bash
omniagent doctor [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--verbose`, `-v` | Show detailed output |

**Checks performed:**

- Configuration file validity
- API key presence
- Network connectivity to API endpoints
- Optional dependencies (git, docker, chrome, ffmpeg)
- Environment variables for channels

**Example:**

```bash
omniagent doctor
```

**Output:**

```
OmniAgent Doctor
================

Version: 0.11.0
Go:      go1.26.4
OS/Arch: darwin/arm64

Checks
------
[✓] Config file: Found at omniagent.yaml
[✓] API key: API key configured
[✓] HOME: /Users/john
[✓] Data directory: /Users/john/.local/share/omniagent
[✓] Anthropic API: Reachable
[✓] OpenAI API: Reachable
[✓] Git: Found (skill installation from git repositories)
[!] Docker: Not found (sandbox execution)
[✓] Chrome/Chromium: Found as chromium (browser automation)

Summary: 8 passed, 1 warnings, 0 errors
```

## Sessions

### sessions list

List all stored conversation sessions.

```bash
omniagent sessions list [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--db` | Database path (default: `~/.local/share/omniagent/data.db`) |

**Example:**

```bash
omniagent sessions list
```

**Output:**

```
SESSION ID                            MESSAGES  CREATED           UPDATED
telegram:123456789                    42        2026-06-28 10:30  2026-06-28 14:22
discord:987654321                     15        2026-06-27 09:15  2026-06-28 11:45
whatsapp:1234567890@s.whatsapp.net    8         2026-06-28 08:00  2026-06-28 08:30

Total: 3 sessions
```

### sessions show

Display details of a specific session.

```bash
omniagent sessions show <session-id> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--db` | Database path |

**Example:**

```bash
omniagent sessions show telegram:123456789
```

**Output:**

```
Session: telegram:123456789
Created: 2026-06-28T10:30:00Z
Updated: 2026-06-28T14:22:00Z
Messages: 42

Conversation:
-------------
[1] user: Hello!
[2] assistant: Hello! How can I help you today?
[3] user: What's the weather like?
...
```

### sessions delete

Remove a session and its conversation history.

```bash
omniagent sessions delete <session-id> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--db` | Database path |

**Example:**

```bash
omniagent sessions delete telegram:123456789
```

### sessions clear

Remove all sessions.

```bash
omniagent sessions clear [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--force` | Skip confirmation prompt |
| `--db` | Database path |

**Example:**

```bash
# With confirmation prompt
omniagent sessions clear

# Skip confirmation
omniagent sessions clear --force
```

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error |
| 3 | Connection error |
