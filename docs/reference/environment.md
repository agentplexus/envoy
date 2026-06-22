# Environment Variables

Complete reference for OmniAgent environment variables.

## LLM Providers

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `GEMINI_API_KEY` | Google Gemini API key |

## Agent Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `OMNIAGENT_AGENT_PROVIDER` | LLM provider: `openai`, `anthropic`, `gemini` | `anthropic` |
| `OMNIAGENT_AGENT_MODEL` | Model name | `claude-sonnet-4-20250514` |
| `OMNIAGENT_AGENT_TEMPERATURE` | Sampling temperature | `0.7` |
| `OMNIAGENT_AGENT_MAX_TOKENS` | Max response tokens | `4096` |

## Channels

### WhatsApp

| Variable | Description | Default |
|----------|-------------|---------|
| `WHATSAPP_ENABLED` | Enable WhatsApp channel | `false` |
| `WHATSAPP_DB_PATH` | Session database path | `whatsapp.db` |

### Telegram

| Variable | Description |
|----------|-------------|
| `TELEGRAM_BOT_TOKEN` | Telegram bot token (auto-enables channel) |

### Discord

| Variable | Description |
|----------|-------------|
| `DISCORD_BOT_TOKEN` | Discord bot token (auto-enables channel) |

### Twilio SMS/MMS/RCS

| Variable | Description | Default |
|----------|-------------|---------|
| `TWILIO_ACCOUNT_SID` | Twilio Account SID (auto-enables channel) | - |
| `TWILIO_AUTH_TOKEN` | Twilio Auth Token | - |
| `TWILIO_PHONE_NUMBER` | Twilio phone number in E.164 format | - |
| `TWILIO_MESSAGING_SERVICE_SID` | Messaging Service SID for RCS (enables RCS with SMS/MMS fallback) | - |
| `TWILIO_WEBHOOK_PATH` | SMS/MMS webhook path | `/webhook/twilio/sms` |

**MMS Support**: Incoming MMS messages with media attachments (images, videos, audio) are automatically extracted. Media URLs are available in `msg.Media`.

**RCS Support**: When `TWILIO_MESSAGING_SERVICE_SID` is set, messages are sent via RCS with automatic fallback to SMS/MMS. RCS features include branded sender identity, rich cards, and suggested actions. Requires carrier-approved RCS sender in your Messaging Service.

## Voice

| Variable | Description | Default |
|----------|-------------|---------|
| `DEEPGRAM_API_KEY` | Deepgram API key | - |
| `ELEVENLABS_API_KEY` | ElevenLabs API key | - |
| `OMNIAGENT_VOICE_ENABLED` | Enable voice processing | `false` |
| `OMNIAGENT_VOICE_RESPONSE_MODE` | Response mode: `auto`, `always`, `never` | `auto` |

## Voice Gateway

### Twilio

| Variable | Description |
|----------|-------------|
| `TWILIO_ACCOUNT_SID` | Twilio Account SID |
| `TWILIO_AUTH_TOKEN` | Twilio Auth Token |

### Telnyx

| Variable | Description |
|----------|-------------|
| `TELNYX_API_KEY` | Telnyx API Key |
| `TELNYX_CONNECTION_ID` | Telnyx Connection ID |

### Vonage

| Variable | Description |
|----------|-------------|
| `VONAGE_APPLICATION_ID` | Vonage Application ID |
| `VONAGE_PRIVATE_KEY` | Path to Vonage private key file |

### Plivo

| Variable | Description |
|----------|-------------|
| `PLIVO_AUTH_ID` | Plivo Auth ID |
| `PLIVO_AUTH_TOKEN` | Plivo Auth Token |

### LiveKit (WebRTC)

| Variable | Description |
|----------|-------------|
| `LIVEKIT_URL` | LiveKit server URL (e.g., `wss://your-app.livekit.cloud`) |
| `LIVEKIT_API_KEY` | LiveKit API Key |
| `LIVEKIT_API_SECRET` | LiveKit API Secret |

### Tunneling

| Variable | Description |
|----------|-------------|
| `NGROK_AUTHTOKEN` | ngrok auth token (for `--ngrok` flag) |

## Web UI Authentication

OAuth 2.0 SSO authentication for the web UI. See [Authentication Guide](../guides/authentication.md) for setup instructions.

| Variable | Description | Default |
|----------|-------------|---------|
| `AUTH_ENABLED` | Enable OAuth authentication | `false` |
| `AUTH_SESSION_SECRET` | Secret for signing session cookies (min 32 bytes) | - |
| `AUTH_COOKIE_DOMAIN` | Cookie domain for multi-subdomain setups | (auto) |
| `AUTH_BASE_URL` | Public URL for OAuth callbacks | `http://localhost:8080` |

### OAuth Providers

| Variable | Description |
|----------|-------------|
| `AUTH_GITHUB_CLIENT_ID` | GitHub OAuth client ID |
| `AUTH_GITHUB_CLIENT_SECRET` | GitHub OAuth client secret |
| `AUTH_GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `AUTH_GOOGLE_CLIENT_SECRET` | Google OAuth client secret |

### Access Control

| Variable | Description |
|----------|-------------|
| `AUTH_ALLOWED_EMAILS` | Comma-separated allowed email addresses |
| `AUTH_ALLOWED_DOMAINS` | Comma-separated allowed domains (e.g., `@company.com`) |

## Gateway

| Variable | Description | Default |
|----------|-------------|---------|
| `OMNIAGENT_GATEWAY_ADDRESS` | Gateway listen address | `127.0.0.1:18789` |

### Address Format

The gateway address must include a colon and port number:

```bash
# Valid formats
OMNIAGENT_GATEWAY_ADDRESS=":8080"           # All interfaces, port 8080
OMNIAGENT_GATEWAY_ADDRESS="127.0.0.1:8080"  # Localhost only
OMNIAGENT_GATEWAY_ADDRESS="0.0.0.0:8080"    # All interfaces (explicit)

# Invalid (missing colon)
OMNIAGENT_GATEWAY_ADDRESS="8080"            # Won't work
```

## Usage Examples

### Minimal Setup (WhatsApp + OpenAI)

```bash
export OPENAI_API_KEY="sk-..."
export WHATSAPP_ENABLED=true

omniagent gateway run
```

### Twilio SMS/MMS/RCS Setup

```bash
# LLM Provider
export OPENAI_API_KEY="sk-..."

# Twilio SMS/MMS
export TWILIO_ACCOUNT_SID="AC..."
export TWILIO_AUTH_TOKEN="..."
export TWILIO_PHONE_NUMBER="+15551234567"

# Optional: Enable RCS (with SMS/MMS fallback)
export TWILIO_MESSAGING_SERVICE_SID="MG..."

# Gateway (for ngrok)
export OMNIAGENT_GATEWAY_ADDRESS=":8080"

omniagent gateway run
```

Configure your Twilio phone number webhook to point to:
`https://<your-domain>/webhook/twilio/sms` (POST)

This endpoint handles SMS, MMS, and RCS. Incoming MMS messages with images, videos, or other media will have their attachments automatically extracted and available to the agent. When `TWILIO_MESSAGING_SERVICE_SID` is set, outgoing messages use RCS with automatic fallback to SMS/MMS.

For local development, use ngrok:

```bash
ngrok http 8080
# Then configure Twilio webhook to: https://<ngrok-subdomain>.ngrok.io/webhook/twilio/sms
```

### Full Setup

```bash
# LLM Provider
export ANTHROPIC_API_KEY="sk-ant-..."
export OMNIAGENT_AGENT_PROVIDER=anthropic
export OMNIAGENT_AGENT_MODEL=claude-sonnet-4-20250514

# WhatsApp
export WHATSAPP_ENABLED=true
export WHATSAPP_DB_PATH=~/.omniagent/whatsapp.db

# Twilio SMS
export TWILIO_ACCOUNT_SID="AC..."
export TWILIO_AUTH_TOKEN="..."
export TWILIO_PHONE_NUMBER="+15551234567"

# Voice
export DEEPGRAM_API_KEY="..."
export OMNIAGENT_VOICE_ENABLED=true
export OMNIAGENT_VOICE_RESPONSE_MODE=auto

# Gateway
export OMNIAGENT_GATEWAY_ADDRESS=0.0.0.0:18789

omniagent gateway run
```

### Using .envrc (direnv)

Create a `.envrc` file in your project directory:

```bash
# .envrc
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export DEEPGRAM_API_KEY="..."

# Twilio SMS
export TWILIO_ACCOUNT_SID="AC..."
export TWILIO_AUTH_TOKEN="..."
export TWILIO_PHONE_NUMBER="+15551234567"

export OMNIAGENT_AGENT_PROVIDER=anthropic
export WHATSAPP_ENABLED=true
export OMNIAGENT_VOICE_ENABLED=true
```

Then allow it:

```bash
direnv allow
omniagent gateway run
```

## Precedence

Configuration values are resolved in this order (highest to lowest):

1. Environment variables
2. Config file values
3. Built-in defaults

Example:

```yaml
# omniagent.yaml
agent:
  provider: openai
```

```bash
# Environment overrides config file
export OMNIAGENT_AGENT_PROVIDER=anthropic
omniagent gateway run  # Uses anthropic
```
