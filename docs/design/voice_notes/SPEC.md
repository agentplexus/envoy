# Voice Note Support for OmniAgent

## Overview

Add voice note support to OmniAgent enabling:
1. **Inbound**: Transcribe voice notes from WhatsApp using OmniVoice STT (Deepgram provider)
2. **Outbound**: Respond with voice notes using OmniVoice TTS (auto mode: when user sends voice)

## Modifiable Repositories

The following repositories are in scope for modifications:

| Repository | Purpose |
|------------|---------|
| `github.com/plexusone/omniagent` | Main application - voice integration |
| `github.com/plexusone/omnichat` | WhatsApp audio download/upload |
| `github.com/plexusone/omnivoice` | Core STT/TTS interfaces (if needed) |
| `github.com/plexusone/omnivoice-deepgram` | Deepgram provider (if needed) |
| `github.com/grokify/*` | Any grokify libraries (if needed) |

## Architecture

```
WhatsApp Voice → Download OGG → OmniVoice STT → Agent → OmniVoice TTS → Upload PTT → WhatsApp
                                    ↓                        ↓
                              [stt.Provider]           [tts.Provider]
                                    ↓                        ↓
                            Deepgram/Whisper/etc    Deepgram/ElevenLabs/etc
```

**Key Design**: Use OmniVoice interfaces (`stt.Provider`, `tts.Provider`) for provider abstraction. This allows seamless switching between Deepgram, ElevenLabs, and future providers.

## Implementation Plan

### Phase 1: omnichat - WhatsApp Audio Support

**Repository**: `github.com/plexusone/omnichat`

#### 1.1 Update `providers/whatsapp/adapter.go` - Receive Audio

Modify `convertIncoming()` to detect and download audio messages:

```go
// After text extraction, add:
if audioMsg := evt.Message.GetAudioMessage(); audioMsg != nil {
    audioData, err := p.client.Download(context.Background(), audioMsg)
    if err != nil {
        p.logger.Error("failed to download audio", "error", err)
    } else {
        mediaType := provider.MediaTypeAudio
        if audioMsg.GetPTT() {
            mediaType = provider.MediaTypeVoice
        }
        msg.Media = append(msg.Media, provider.Media{
            Type:     mediaType,
            Data:     audioData,
            MimeType: audioMsg.GetMimetype(),
        })
    }
}
```

#### 1.2 Update `providers/whatsapp/adapter.go` - Send Audio

Modify `Send()` to handle outgoing voice messages:

```go
// Handle media attachments
for _, media := range msg.Media {
    if media.Type == provider.MediaTypeVoice || media.Type == provider.MediaTypeAudio {
        uploadResp, err := p.client.Upload(ctx, media.Data, whatsmeow.MediaAudio)
        if err != nil {
            return fmt.Errorf("upload audio: %w", err)
        }

        isPTT := media.Type == provider.MediaTypeVoice
        audioMsg := &waE2E.Message{
            AudioMessage: &waE2E.AudioMessage{
                URL:           proto.String(uploadResp.URL),
                DirectPath:    proto.String(uploadResp.DirectPath),
                MediaKey:      uploadResp.MediaKey,
                FileEncSHA256: uploadResp.FileEncSHA256,
                FileSHA256:    uploadResp.FileSHA256,
                FileLength:    proto.Uint64(uploadResp.FileLength),
                Mimetype:      proto.String(media.MimeType),
                PTT:           proto.Bool(isPTT),
            },
        }
        _, err = p.client.SendMessage(ctx, jid, audioMsg)
        if err != nil {
            return fmt.Errorf("send audio: %w", err)
        }
    }
}
```

#### 1.3 Add VoiceProcessor Interface to `provider/router.go`

```go
// VoiceProcessor handles voice transcription and synthesis.
type VoiceProcessor interface {
    TranscribeAudio(ctx context.Context, audio []byte, mimeType string) (string, error)
    SynthesizeSpeech(ctx context.Context, text string) ([]byte, string, error) // returns audio, mimeType, error
}

// ProcessWithVoice creates a handler with voice processing.
func (r *Router) ProcessWithVoice(processor VoiceProcessor, mode string) MessageHandler
```

---

### Phase 2: OmniAgent - Voice Processing with OmniVoice Interfaces

**Repository**: `github.com/plexusone/omniagent`

#### 2.1 Create `voice/` Package

**File: `voice/config.go`**
```go
package voice

type Config struct {
    Enabled      bool
    ResponseMode string // "auto", "always", "never"
    STT          STTConfig
    TTS          TTSConfig
}

type STTConfig struct {
    Provider string // "deepgram", "whisper", etc.
    APIKey   string
    Model    string // provider-specific model
    Language string // "" for auto-detect
}

type TTSConfig struct {
    Provider string // "deepgram", "elevenlabs", etc.
    APIKey   string
    Model    string // provider-specific model
    VoiceID  string // provider-specific voice ID
}
```

**File: `voice/processor.go`**

Uses OmniVoice interfaces for provider abstraction:

```go
package voice

import (
    "context"
    "fmt"
    "log/slog"

    "github.com/plexusone/omnivoice/stt"
    "github.com/plexusone/omnivoice/tts"

    // Provider implementations
    deepgramstt "github.com/plexusone/omnivoice-deepgram/omnivoice/stt"
    deepgramtts "github.com/plexusone/omnivoice-deepgram/omnivoice/tts"
    // Future: elevenlabstts "github.com/plexusone/go-elevenlabs/omnivoice/tts"
)

// Processor handles voice transcription and synthesis using OmniVoice interfaces.
type Processor struct {
    sttProvider stt.Provider  // OmniVoice STT interface
    ttsProvider tts.Provider  // OmniVoice TTS interface
    config      Config
    logger      *slog.Logger
}

// New creates a voice processor with the configured providers.
func New(config Config, logger *slog.Logger) (*Processor, error) {
    p := &Processor{
        config: config,
        logger: logger,
    }

    // Initialize STT provider based on config
    switch config.STT.Provider {
    case "deepgram":
        sttProv, err := deepgramstt.New(deepgramstt.WithAPIKey(config.STT.APIKey))
        if err != nil {
            return nil, fmt.Errorf("create deepgram stt: %w", err)
        }
        p.sttProvider = sttProv
    default:
        return nil, fmt.Errorf("unsupported STT provider: %s", config.STT.Provider)
    }

    // Initialize TTS provider based on config
    switch config.TTS.Provider {
    case "deepgram":
        ttsProv, err := deepgramtts.New(deepgramtts.WithAPIKey(config.TTS.APIKey))
        if err != nil {
            return nil, fmt.Errorf("create deepgram tts: %w", err)
        }
        p.ttsProvider = ttsProv
    // Future: case "elevenlabs": ...
    default:
        return nil, fmt.Errorf("unsupported TTS provider: %s", config.TTS.Provider)
    }

    return p, nil
}

// TranscribeAudio converts audio to text using the configured STT provider.
func (p *Processor) TranscribeAudio(ctx context.Context, audio []byte, mimeType string) (string, error) {
    config := stt.TranscriptionConfig{
        Model:    p.config.STT.Model,
        Language: p.config.STT.Language,
    }

    result, err := p.sttProvider.Transcribe(ctx, audio, config)
    if err != nil {
        return "", fmt.Errorf("transcribe: %w", err)
    }

    p.logger.Info("transcription complete",
        "provider", p.sttProvider.Name(),
        "text_length", len(result.Text))

    return result.Text, nil
}

// SynthesizeSpeech converts text to audio using the configured TTS provider.
// Returns audio bytes and MIME type.
func (p *Processor) SynthesizeSpeech(ctx context.Context, text string) ([]byte, string, error) {
    config := tts.SynthesisConfig{
        VoiceID:      p.config.TTS.VoiceID,
        Model:        p.config.TTS.Model,
        OutputFormat: "ogg",  // OGG Opus for WhatsApp compatibility
    }

    result, err := p.ttsProvider.Synthesize(ctx, text, config)
    if err != nil {
        return nil, "", fmt.Errorf("synthesize: %w", err)
    }

    p.logger.Info("synthesis complete",
        "provider", p.ttsProvider.Name(),
        "audio_size", len(result.Audio))

    return result.Audio, "audio/ogg; codecs=opus", nil
}

// Close releases provider resources.
func (p *Processor) Close() error {
    // Providers may implement io.Closer
    return nil
}
```

#### 2.2 Update `config/config.go`

Add to `Config` struct:
```go
Voice VoiceConfig `json:"voice" yaml:"voice"`
```

Add new types:
```go
type VoiceConfig struct {
    Enabled      bool      `json:"enabled" yaml:"enabled"`
    ResponseMode string    `json:"response_mode" yaml:"response_mode"`
    STT          STTConfig `json:"stt" yaml:"stt"`
    TTS          TTSConfig `json:"tts" yaml:"tts"`
}

type STTConfig struct {
    Provider string `json:"provider" yaml:"provider"`
    APIKey   string `json:"api_key" yaml:"api_key"`
    Model    string `json:"model" yaml:"model"`
    Language string `json:"language" yaml:"language"`
}

type TTSConfig struct {
    Provider string `json:"provider" yaml:"provider"`
    APIKey   string `json:"api_key" yaml:"api_key"`
    Model    string `json:"model" yaml:"model"`
    VoiceID  string `json:"voice_id" yaml:"voice_id"`
}
```

#### 2.3 Update `config/defaults.go`

Add defaults:
```go
Voice: VoiceConfig{
    Enabled:      false,
    ResponseMode: "auto",
    STT: STTConfig{
        Provider: "deepgram",
        Model:    "nova-2",
    },
    TTS: TTSConfig{
        Provider: "deepgram",
        Model:    "aura-asteria-en",
    },
},
```

#### 2.4 Update `cmd/omniagent/commands/gateway.go`

Add import:
```go
"github.com/plexusone/omniagent/voice"
```

After agent initialization, add:
```go
// Initialize voice processor if enabled
var voiceProcessor *voice.Processor
if cfg.Voice.Enabled {
    voiceProcessor, err = voice.New(voice.Config{
        Enabled:      true,
        ResponseMode: cfg.Voice.ResponseMode,
        STT: voice.STTConfig{
            Provider: cfg.Voice.STT.Provider,
            APIKey:   cfg.Voice.STT.APIKey,
            Model:    cfg.Voice.STT.Model,
            Language: cfg.Voice.STT.Language,
        },
        TTS: voice.TTSConfig{
            Provider: cfg.Voice.TTS.Provider,
            APIKey:   cfg.Voice.TTS.APIKey,
            Model:    cfg.Voice.TTS.Model,
            VoiceID:  cfg.Voice.TTS.VoiceID,
        },
    }, logger)
    if err != nil {
        return fmt.Errorf("create voice processor: %w", err)
    }
    defer voiceProcessor.Close()
    logger.Info("voice processor initialized",
        "stt_provider", cfg.Voice.STT.Provider,
        "tts_provider", cfg.Voice.TTS.Provider)
}
```

Update router setup:
```go
if agentInstance != nil {
    router.SetAgent(agentInstance)
    if voiceProcessor != nil {
        router.OnMessage(provider.All(), router.ProcessWithVoice(voiceProcessor, cfg.Voice.ResponseMode))
    } else {
        router.OnMessage(provider.All(), router.ProcessWithAgent())
    }
}
```

#### 2.5 Add Dependencies to `go.mod`

```
github.com/plexusone/omnivoice v0.x.x
github.com/plexusone/omnivoice-deepgram v0.x.x
```

Note: Verify latest versions before adding.

---

### Phase 3: OmniVoice Libraries (If Needed)

**Repositories**:

- `github.com/plexusone/omnivoice`
- `github.com/plexusone/omnivoice-deepgram`

If any interface changes or bug fixes are needed in the OmniVoice libraries during implementation, they will be addressed in-place. Potential modifications:

- Add OGG Opus output format support in Deepgram TTS (if not already present)
- Add encoding detection helper for WhatsApp audio formats
- Fix any compatibility issues discovered during integration

---

### Phase 4: Integration & Testing

#### 4.1 Configuration Example

```yaml
voice:
  enabled: true
  response_mode: auto  # auto | always | never

  stt:
    provider: deepgram
    api_key: ${DEEPGRAM_API_KEY}
    model: nova-2
    language: ""  # auto-detect

  tts:
    provider: deepgram
    api_key: ${DEEPGRAM_API_KEY}
    model: aura-asteria-en
    voice_id: aura-asteria-en
```

#### 4.2 Environment Variables

- `DEEPGRAM_API_KEY` - For Deepgram STT/TTS
- `ELEVENLABS_API_KEY` - For ElevenLabs TTS (future)

---

## Provider Abstraction Benefits

Using OmniVoice interfaces (`stt.Provider`, `tts.Provider`) enables:

1. **Easy provider switching**: Change `provider: deepgram` to `provider: elevenlabs` in config
2. **Consistent API**: Same `TranscribeAudio()` and `SynthesizeSpeech()` calls regardless of provider
3. **Future extensibility**: Add new providers without changing voice processor logic

```go
// Future: Adding ElevenLabs support
case "elevenlabs":
    ttsProv, err := elevenlabstts.New(elevenlabstts.WithAPIKey(config.TTS.APIKey))
    p.ttsProvider = ttsProv
```

---

## Files to Modify/Create

### omnichat (modify)

- `providers/whatsapp/adapter.go` - Add audio download/upload
- `provider/router.go` - Add VoiceProcessor interface and ProcessWithVoice handler

### omniagent (create/modify)

- `voice/config.go` - NEW: Voice configuration types
- `voice/processor.go` - NEW: Voice processor using OmniVoice interfaces
- `config/config.go` - Add VoiceConfig
- `config/defaults.go` - Add voice defaults
- `cmd/omniagent/commands/gateway.go` - Initialize voice processor

### omnivoice / omnivoice-deepgram (if needed)

- Bug fixes or enhancements discovered during implementation

---

## Implementation Order

1. **omnichat**: WhatsApp audio download in `convertIncoming()`
2. **omnichat**: WhatsApp audio upload in `Send()`
3. **omnichat**: Add `VoiceProcessor` interface and `ProcessWithVoice()` to router
4. **omniagent**: Create `voice/` package with OmniVoice-based processor
5. **omniagent**: Update config with VoiceConfig
6. **omniagent**: Wire up voice processor in gateway command
7. **omnivoice libs**: Fix any issues discovered during integration
8. **Test**: Send voice note → verify transcription → verify voice response

---

## Verification

1. **Unit test**: Mock OmniVoice providers, verify processor logic
2. **Integration test**:
   - Configure with Deepgram API key
   - Send WhatsApp voice note
   - Verify logs show transcription
   - Verify agent processes transcribed text
   - Verify voice response sent back as PTT
3. **Manual test**:
   ```bash
   export DEEPGRAM_API_KEY=your_key
   omniagent gateway run
   # Send voice note via WhatsApp
   # Should receive voice note response
   ```
