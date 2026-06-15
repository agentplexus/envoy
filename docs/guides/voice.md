# Voice Integration

OmniAgent supports multiple voice modes:

1. **Voice Notes** - STT/TTS for messaging channels (transcribe voice messages, respond with audio)
2. **Voice Gateway** - Full-duplex phone calls via Twilio, Telnyx, Vonage, or Plivo
3. **WebRTC Gateway** - Browser/mobile voice via LiveKit (no phone number required)

## Voice Architecture: Traditional vs Native

OmniAgent supports two fundamentally different approaches for real-time voice:

### Traditional Pipeline (STT → LLM → TTS)

```
Phone → Twilio → [Deepgram STT] → Text → [Claude/GPT] → Text → [ElevenLabs TTS] → Audio → Twilio → Phone
```

- **Latency**: 500-1500ms (sum of STT + LLM + TTS)
- **Providers**: Separate STT and TTS providers required
- **Flexibility**: Mix and match any STT, LLM, and TTS providers
- **Cost**: Pay for STT + LLM + TTS separately

### Native Voice-to-Voice (Recommended)

```
Phone → Twilio → [OpenAI Realtime / Gemini Live] → Audio → Twilio → Phone
```

- **Latency**: 100-200ms (model handles audio directly)
- **Providers**: Single provider handles everything
- **Simplicity**: No separate STT/TTS configuration
- **Cost**: Single API cost (often cheaper overall)

### Comparison

| Aspect | Traditional (STT→LLM→TTS) | Native Voice-to-Voice |
|--------|---------------------------|----------------------|
| Latency | 500-1500ms | 100-200ms |
| Configuration | 3 providers to configure | 1 provider |
| STT Provider | Deepgram, Whisper, Google | Built-in |
| TTS Provider | ElevenLabs, Deepgram, Cartesia | Built-in |
| Barge-in | Complex (coordinate STT+TTS) | Native support |
| Best for | Custom voice selection, specialized STT | Low latency, simplicity |

### Native Voice-to-Voice Providers

| Provider | Package | Latency | Voices |
|----------|---------|---------|--------|
| **OpenAI Realtime** | `omni-openai/omnivoice/realtime` | ~100ms | 11 voices |
| **Gemini Live** | `omni-google/omnivoice` | ~200ms | 5 voices |

### When to Use Each

**Use Native Voice-to-Voice when:**

- Low latency is critical (customer service, real-time conversations)
- You want simpler configuration
- Barge-in (user interruption) is important

**Use Traditional Pipeline when:**

- You need specific STT accuracy (e.g., medical/legal terminology)
- You want custom voice cloning (ElevenLabs, Cartesia)
- You need to support languages not available in native APIs

For detailed comparison including latency breakdown, cost analysis, and feature matrix, see the [Voice Architecture Guide](https://plexusone.dev/omnivoice-core/voice-architecture).

---

## Voice Gateway (Phone Calls)

The voice gateway enables real-time phone conversations with bidirectional audio streaming.

### Supported Telephony Providers

Twilio, Telnyx, Vonage, and Plivo provide **telephony transport only** (phone connectivity). They do NOT provide STT/TTS - you must configure a voice processing approach above.

| Provider | Protocol | SMS | Notes |
|----------|----------|-----|-------|
| **Twilio** | Media Streams (WS) | Yes | Most popular, extensive documentation |
| **Telnyx** | Media Streaming (WS) | Yes | Competitive pricing, good API |
| **Vonage** | Voice WebSocket | Yes | JWT auth, NCCO call control |
| **Plivo** | Stream API (WS) | Yes | Simple pricing, good international coverage |
| **LiveKit** | WebRTC | No | Browser/mobile apps, low latency |

### Architecture (Traditional)

```
┌──────────┐        ┌─────────────────┐        ┌───────────────────┐
│  Caller  │◄──────►│    Provider     │◄──────►│   Voice Gateway   │
│  (PSTN)  │  PSTN  │  (Twilio/Telnyx │   WS   │   (STT→LLM→TTS)   │
│          │        │  /Vonage/Plivo) │        │                   │
└──────────┘        └─────────────────┘        └───────────────────┘
```

### Architecture (Native Voice-to-Voice)

```
┌──────────┐        ┌─────────────────┐        ┌───────────────────┐
│  Caller  │◄──────►│    Provider     │◄──────►│   Voice Gateway   │
│  (PSTN)  │  PSTN  │  (Twilio/Telnyx │   WS   │ (OpenAI Realtime) │
│          │        │  /Vonage/Plivo) │        │                   │
└──────────┘        └─────────────────┘        └───────────────────┘
```

### Quick Start (Native Voice-to-Voice)

For lowest latency, use native voice-to-voice APIs:

```bash
# OpenAI Realtime (~100ms latency)
export TWILIO_ACCOUNT_SID="AC..."
export TWILIO_AUTH_TOKEN="..."
export OPENAI_API_KEY="sk-..."
omniagent voice serve --provider twilio --realtime openai --public-url https://your-server.com

# Gemini Live (~200ms latency)
export TWILIO_ACCOUNT_SID="AC..."
export TWILIO_AUTH_TOKEN="..."
export GOOGLE_API_KEY="..."
omniagent voice serve --provider twilio --realtime gemini --public-url https://your-server.com
```

### Quick Start (Traditional Pipeline)

For custom STT/TTS provider selection:

```bash
# With Twilio
export TWILIO_ACCOUNT_SID="AC..."
export TWILIO_AUTH_TOKEN="..."
omniagent voice serve --provider twilio --public-url https://your-server.com

# With Telnyx
export TELNYX_API_KEY="KEY..."
export TELNYX_CONNECTION_ID="..."
omniagent voice serve --provider telnyx --public-url https://your-server.com

# With Vonage
export VONAGE_APPLICATION_ID="..."
export VONAGE_PRIVATE_KEY="/path/to/private.key"
omniagent voice serve --provider vonage --public-url https://your-server.com

# With Plivo
export PLIVO_AUTH_ID="..."
export PLIVO_AUTH_TOKEN="..."
omniagent voice serve --provider plivo --public-url https://your-server.com

# With ngrok (local development)
export NGROK_AUTHTOKEN=your-token
omniagent voice serve --provider twilio --listen :8081 --ngrok
```

### Webhook Configuration

Configure webhooks in your provider's console:

#### Twilio

| Webhook | URL |
|---------|-----|
| Voice URL | `https://your-server.com/voice/inbound` |
| Status Callback | `https://your-server.com/voice/status` |

#### Telnyx

| Webhook | URL |
|---------|-----|
| Voice URL | `https://your-server.com/voice/inbound` |
| Status Callback | `https://your-server.com/voice/status` |

#### Vonage

| Webhook | URL |
|---------|-----|
| Answer URL | `https://your-server.com/voice/answer` |
| Event URL | `https://your-server.com/voice/event` |

#### Plivo

| Webhook | URL |
|---------|-----|
| Answer URL | `https://your-server.com/voice/answer` |
| Hangup URL | `https://your-server.com/voice/hangup` |
| Fallback URL | `https://your-server.com/voice/fallback` |

## WebRTC Gateway (Browser/Mobile)

For web and mobile applications, use LiveKit instead of PSTN providers:

```
┌───────────────┐        ┌─────────────────┐        ┌───────────────────┐
│  Browser/App  │◄──────►│  LiveKit Cloud  │◄──────►│   Voice Gateway   │
│   (WebRTC)    │ WebRTC │    or Server    │ WebRTC │   (STT→LLM→TTS)   │
└───────────────┘        └─────────────────┘        └───────────────────┘
```

### LiveKit Quick Start

```bash
# Set LiveKit credentials
export LIVEKIT_URL="wss://your-app.livekit.cloud"
export LIVEKIT_API_KEY="..."
export LIVEKIT_API_SECRET="..."

# Start WebRTC voice gateway
omniagent voice serve --provider livekit --room voice-agent
```

### Key Differences from PSTN

| Aspect | PSTN (Twilio, etc.) | WebRTC (LiveKit) |
|--------|---------------------|------------------|
| Connection | Phone number | Room name |
| Clients | Phone calls | Browser/mobile apps |
| Latency | 500ms+ | <200ms |
| Cost | Per-minute charges | Infrastructure only |
| No webhooks | Needs public URL | Direct WebRTC |

### Local Development with ngrok

For local development, use the built-in ngrok integration:

```bash
# Set ngrok auth token
export NGROK_AUTHTOKEN=your-ngrok-token

# Start with auto-generated public URL
omniagent voice serve --listen :8081 --ngrok

# With custom ngrok domain (requires paid plan)
omniagent voice serve --listen :8081 --ngrok --ngrok-domain myapp.ngrok.io
```

The public URL is displayed on startup. Configure this in your provider's webhook settings.

### Voice Gateway Commands

```bash
omniagent voice serve      # Start the voice gateway server
omniagent voice status     # Show configuration and provider status
omniagent voice call NUM   # Make an outbound call
```

### Voice Gateway Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--provider` | Telephony provider (`twilio`, `telnyx`, `vonage`, `plivo`) | `twilio` |
| `--listen` | Listen address | `:8080` |
| `--public-url` | Public URL for webhooks | - |
| `--phone` | Phone number for outbound calls | - |
| `--ngrok` | Use ngrok tunnel | `false` |
| `--ngrok-domain` | Custom ngrok domain | - |
| `--stt` | STT provider | `deepgram` |
| `--tts` | TTS provider | `elevenlabs` |
| `--voice` | TTS voice ID | - |
| `--llm` | LLM provider | `anthropic` |
| `--model` | LLM model | `claude-sonnet-4-20250514` |
| `--system-prompt` | Custom system prompt | - |
| `--realtime` | Native voice-to-voice provider (`openai`, `gemini`) | - |
| `--realtime-voice` | Voice for realtime API | `alloy` (OpenAI), `Puck` (Gemini) |

## Complete Configuration Examples

This section provides complete `omniagent.yaml` configurations for both voice pipeline modes.

### Native Voice-to-Voice (Recommended)

The realtime pipeline provides the lowest latency (~100-200ms) by using native speech-to-speech APIs.

#### OpenAI Realtime Configuration

```yaml
# omniagent.yaml - OpenAI Realtime voice-to-voice
voice:
  enabled: true

  # Gateway configuration
  gateway:
    provider: twilio              # or: telnyx
    listen_addr: ":8080"
    public_url: "https://your-server.ngrok.io"  # Required for webhooks

    # Pipeline mode
    mode: realtime                # Use native voice-to-voice

    # Realtime provider configuration
    realtime:
      provider: openai
      # model: gpt-4o-realtime-preview-2024-12-17  # Optional, uses latest
      voice: alloy                # Options: alloy, ash, ballad, coral, echo, nova, etc.

    # System prompt for the voice agent
    system_prompt: |
      You are a helpful customer service agent for Acme Corp.
      Keep responses concise and conversational.
      If you need to look up information, use the available tools.

    # Session limits
    max_session_duration: 30m
    interruption_mode: immediate  # Options: immediate, after_sentence, disabled

    # Greeting spoken when call connects
    greeting: "Hello! Thank you for calling Acme Corp. How can I help you today?"

    # Tools available during voice conversation
    tools:
      - name: lookup_order
        description: Look up an order by order ID
        parameters:
          type: object
          properties:
            order_id:
              type: string
              description: The order ID to look up
          required: [order_id]

      - name: check_inventory
        description: Check if a product is in stock
        parameters:
          type: object
          properties:
            product_name:
              type: string
          required: [product_name]

# Twilio credentials (can also use environment variables)
channels:
  twilio_sms:
    enabled: false
    account_sid: ${TWILIO_ACCOUNT_SID}
    auth_token: ${TWILIO_AUTH_TOKEN}
    phone_number: ${TWILIO_PHONE_NUMBER}
```

#### Gemini Live Configuration

```yaml
# omniagent.yaml - Gemini Live voice-to-voice
voice:
  enabled: true

  gateway:
    provider: twilio
    listen_addr: ":8080"
    public_url: "https://your-server.ngrok.io"

    mode: realtime

    realtime:
      provider: gemini
      # model: gemini-2.0-flash-live  # Optional, uses latest
      voice: Puck                  # Options: Puck, Charon, Kore, Fenrir, Aoede

    system_prompt: |
      You are a friendly assistant. Keep responses brief and natural.

    greeting: "Hi there! How can I help?"
```

### Traditional Pipeline (STT → LLM → TTS)

The traditional pipeline offers more flexibility in provider selection at the cost of higher latency (~500-1500ms).

#### Deepgram STT + Claude + ElevenLabs TTS

```yaml
# omniagent.yaml - Traditional pipeline with premium providers
voice:
  enabled: true

  gateway:
    provider: twilio
    listen_addr: ":8080"
    public_url: "https://your-server.ngrok.io"

    mode: text                    # Traditional STT→LLM→TTS pipeline

    # Speech-to-Text configuration
    stt:
      provider: deepgram
      model: nova-2               # Best accuracy
      language: en-US
      # api_key: ${DEEPGRAM_API_KEY}  # Or set via environment

    # Text-to-Speech configuration
    tts:
      provider: elevenlabs
      voice_id: 21m00Tcm4TlvDq8ikWAM  # Rachel - natural female voice
      model: eleven_multilingual_v2
      # api_key: ${ELEVENLABS_API_KEY}

    # LLM configuration
    llm:
      provider: anthropic
      model: claude-sonnet-4-20250514
      # api_key: ${ANTHROPIC_API_KEY}

    system_prompt: |
      You are a helpful voice assistant. Keep responses concise (1-3 sentences).
      Speak naturally - avoid bullet points or markdown formatting.

    greeting: "Hello! How can I assist you today?"
    max_session_duration: 30m
    interruption_mode: immediate
```

#### Whisper STT + GPT-4o + OpenAI TTS (All OpenAI)

```yaml
# omniagent.yaml - All-OpenAI traditional pipeline
voice:
  enabled: true

  gateway:
    provider: twilio
    listen_addr: ":8080"
    public_url: "https://your-server.ngrok.io"

    mode: text

    stt:
      provider: openai
      model: whisper-1

    tts:
      provider: openai
      model: tts-1
      voice_id: nova              # Options: alloy, echo, fable, onyx, nova, shimmer

    llm:
      provider: openai
      model: gpt-4o

    system_prompt: "You are a helpful assistant. Be concise."
```

#### Deepgram STT + Deepgram TTS (Budget-Friendly)

```yaml
# omniagent.yaml - Cost-effective Deepgram-only pipeline
voice:
  enabled: true

  gateway:
    provider: twilio
    listen_addr: ":8080"
    public_url: "https://your-server.ngrok.io"

    mode: text

    stt:
      provider: deepgram
      model: nova-2

    tts:
      provider: deepgram
      model: aura-asteria-en      # Options: aura-asteria-en, aura-luna-en, aura-stella-en

    llm:
      provider: anthropic
      model: claude-sonnet-4-20250514
```

### Telnyx Gateway Examples

#### Telnyx + OpenAI Realtime

```yaml
# omniagent.yaml - Telnyx with OpenAI Realtime
voice:
  enabled: true

  gateway:
    provider: telnyx
    listen_addr: ":8080"
    public_url: "https://your-server.ngrok.io"

    # Telnyx-specific configuration
    telnyx:
      # api_key: ${TELNYX_API_KEY}
      phone_number: "+15551234567"
      connection_id: "your-connection-id"  # From Telnyx portal

    mode: realtime

    realtime:
      provider: openai
      voice: coral

    system_prompt: "You are a helpful assistant."
```

#### Telnyx + Traditional Pipeline

```yaml
# omniagent.yaml - Telnyx with traditional pipeline
voice:
  enabled: true

  gateway:
    provider: telnyx
    listen_addr: ":8080"
    public_url: "https://your-server.ngrok.io"

    telnyx:
      phone_number: "+15551234567"
      connection_id: "your-connection-id"

    mode: text

    stt:
      provider: deepgram
      model: nova-2

    tts:
      provider: elevenlabs
      voice_id: your-voice-id

    llm:
      provider: anthropic
      model: claude-sonnet-4-20250514
```

### Environment Variables Reference

```bash
# Telephony providers
export TWILIO_ACCOUNT_SID="AC..."
export TWILIO_AUTH_TOKEN="..."
export TWILIO_PHONE_NUMBER="+15551234567"

export TELNYX_API_KEY="KEY..."
export TELNYX_CONNECTION_ID="..."

# Realtime voice-to-voice
export OPENAI_API_KEY="sk-..."      # For OpenAI Realtime
export GOOGLE_API_KEY="..."          # For Gemini Live
export GEMINI_API_KEY="..."          # Alternative for Gemini

# Traditional pipeline - STT
export DEEPGRAM_API_KEY="..."

# Traditional pipeline - TTS
export ELEVENLABS_API_KEY="..."

# Traditional pipeline - LLM
export ANTHROPIC_API_KEY="..."

# ngrok (for local development)
export NGROK_AUTHTOKEN="..."
```

### Local Development with ngrok

```yaml
# omniagent.yaml - Local development setup
voice:
  enabled: true

  gateway:
    provider: twilio
    listen_addr: ":8081"          # Local port
    # public_url not needed - ngrok will provide it

    # Enable ngrok tunnel
    ngrok:
      enabled: true
      # domain: myapp.ngrok.io    # Optional: custom domain (paid plan)

    mode: realtime
    realtime:
      provider: openai
      voice: alloy
```

Start with:

```bash
omniagent voice serve --config omniagent.yaml
# Or with CLI flags:
omniagent voice serve --provider twilio --realtime openai --listen :8081 --ngrok
```

---

### Native Voice-to-Voice Configuration

When using `--realtime`, the STT/TTS flags are ignored - the realtime API handles audio directly.

#### OpenAI Realtime

```bash
omniagent voice serve \
  --provider twilio \
  --realtime openai \
  --realtime-voice alloy \
  --system-prompt "You are a helpful customer service agent." \
  --public-url https://your-server.com
```

Available voices: `alloy`, `ash`, `ballad`, `coral`, `echo`, `fable`, `nova`, `onyx`, `sage`, `shimmer`, `verse`

Audio format: PCM16 24kHz mono (native, no conversion needed for LiveKit)

#### Gemini Live

```bash
omniagent voice serve \
  --provider twilio \
  --realtime gemini \
  --realtime-voice Puck \
  --system-prompt "You are a helpful customer service agent." \
  --public-url https://your-server.com
```

Available voices: `Puck`, `Charon`, `Kore`, `Fenrir`, `Aoede`

Audio format: PCM16 16kHz input, 24kHz output

#### With Function Calling

Native voice-to-voice APIs support function calling during conversation:

```yaml
# omniagent.yaml
voice:
  realtime:
    provider: openai
    voice: alloy
    tools:
      - name: lookup_order
        description: Look up an order by ID
        parameters:
          type: object
          properties:
            order_id:
              type: string
          required: [order_id]
```

### Provider Comparison

| Feature | Twilio | Telnyx | Vonage | Plivo |
|---------|--------|--------|--------|-------|
| Audio Format | mulaw 8kHz | L16 16kHz | L16 16kHz | L16 16kHz |
| WebSocket | Media Streams | Media Streaming | Voice WebSocket | Stream API |
| Auth | Account SID + Token | API Key | JWT (RS256) | Auth ID + Token |
| Call Control | TwiML | TeXML | NCCO (JSON) | Plivo XML |

---

## Voice Notes (Messaging)

OmniAgent supports voice notes via [OmniVoice](https://github.com/plexusone/omnivoice), providing speech-to-text (STT) and text-to-speech (TTS) capabilities for messaging channels.

### Overview

When voice processing is enabled:

1. Incoming voice messages are transcribed to text
2. The text is processed by the AI agent
3. Responses can be synthesized back to speech

## Supported STT/TTS Providers

| Provider | STT | TTS | Notes |
|----------|-----|-----|-------|
| Deepgram | Yes | Yes | Nova-2 for STT, Aura voices for TTS |
| OpenAI | Yes | Yes | Whisper for STT, TTS-1 for TTS |
| ElevenLabs | No | Yes | High-quality voice synthesis |

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `DEEPGRAM_API_KEY` | Deepgram API key |
| `OPENAI_API_KEY` | OpenAI API key (for Whisper/TTS) |
| `ELEVENLABS_API_KEY` | ElevenLabs API key |
| `OMNIAGENT_VOICE_ENABLED` | Enable voice processing |
| `OMNIAGENT_VOICE_RESPONSE_MODE` | Response mode: `auto`, `always`, `never` |

### Config File

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

## Response Modes

| Mode | Behavior |
|------|----------|
| `auto` | Reply with voice only to voice messages |
| `always` | Always reply with voice |
| `never` | Never reply with voice (text only) |

## Provider Setup

### Deepgram

1. Sign up at [deepgram.com](https://deepgram.com)
2. Create an API key
3. Set `DEEPGRAM_API_KEY` environment variable

```yaml
voice:
  stt:
    provider: deepgram
    model: nova-2
  tts:
    provider: deepgram
    model: aura-asteria-en
```

### OpenAI

Uses your existing OpenAI API key:

```yaml
voice:
  stt:
    provider: openai
    model: whisper-1
  tts:
    provider: openai
    model: tts-1
    voice_id: alloy
```

### ElevenLabs (TTS only)

1. Sign up at [elevenlabs.io](https://elevenlabs.io)
2. Create an API key
3. Set `ELEVENLABS_API_KEY` environment variable

```yaml
voice:
  tts:
    provider: elevenlabs
    voice_id: your-voice-id
```

## Architecture

OmniVoice uses a provider registry pattern:

```go
import (
    "github.com/plexusone/omnivoice"
    _ "github.com/plexusone/omnivoice/providers/all" // Register all providers
)

// Get providers by name
stt, _ := omnivoice.GetSTTProvider("deepgram", omnivoice.WithAPIKey(key))
tts, _ := omnivoice.GetTTSProvider("elevenlabs", omnivoice.WithAPIKey(key))
```

## Troubleshooting

### Voice Not Working

1. Verify API keys are set correctly
2. Check that voice is enabled in config
3. Ensure the provider supports your chosen model

### Poor Transcription Quality

- Use Deepgram Nova-2 or OpenAI Whisper for best results
- Ensure audio quality is reasonable
- Check language settings match the spoken language

### TTS Sounds Robotic

- Try different voice IDs
- ElevenLabs offers the most natural-sounding voices
- Adjust model settings if available
