package voice

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/plexusone/omnivoice"
)

// mockSTTProvider implements omnivoice.STTProvider for testing.
type mockSTTProvider struct {
	name           string
	transcribeFunc func(ctx context.Context, audio []byte, config omnivoice.TranscriptionConfig) (*omnivoice.TranscriptionResult, error)
}

func (m *mockSTTProvider) Name() string { return m.name }

func (m *mockSTTProvider) Transcribe(ctx context.Context, audio []byte, config omnivoice.TranscriptionConfig) (*omnivoice.TranscriptionResult, error) {
	if m.transcribeFunc != nil {
		return m.transcribeFunc(ctx, audio, config)
	}
	return &omnivoice.TranscriptionResult{
		Text:     "mock transcription",
		Language: "en",
	}, nil
}

func (m *mockSTTProvider) TranscribeFile(ctx context.Context, filePath string, config omnivoice.TranscriptionConfig) (*omnivoice.TranscriptionResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSTTProvider) TranscribeURL(ctx context.Context, url string, config omnivoice.TranscriptionConfig) (*omnivoice.TranscriptionResult, error) {
	return nil, errors.New("not implemented")
}

// mockTTSProvider implements omnivoice.TTSProvider for testing.
type mockTTSProvider struct {
	name           string
	synthesizeFunc func(ctx context.Context, text string, config omnivoice.SynthesisConfig) (*omnivoice.SynthesisResult, error)
}

func (m *mockTTSProvider) Name() string { return m.name }

func (m *mockTTSProvider) Synthesize(ctx context.Context, text string, config omnivoice.SynthesisConfig) (*omnivoice.SynthesisResult, error) {
	if m.synthesizeFunc != nil {
		return m.synthesizeFunc(ctx, text, config)
	}
	return &omnivoice.SynthesisResult{
		Audio:  []byte("mock audio data"),
		Format: "mp3",
	}, nil
}

func (m *mockTTSProvider) SynthesizeStream(ctx context.Context, text string, config omnivoice.SynthesisConfig) (<-chan omnivoice.TTSStreamChunk, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTTSProvider) ListVoices(ctx context.Context) ([]omnivoice.Voice, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTTSProvider) GetVoice(ctx context.Context, voiceID string) (*omnivoice.Voice, error) {
	return nil, errors.New("not implemented")
}

// newTestProcessor creates a processor with mock providers for testing.
func newTestProcessor(sttProv omnivoice.STTProvider, ttsProv omnivoice.TTSProvider, config Config) *Processor {
	responseMode := config.ResponseMode
	if responseMode == "" {
		responseMode = "auto"
	}
	return &Processor{
		sttProvider:  sttProv,
		ttsProvider:  ttsProv,
		config:       config,
		logger:       slog.Default(),
		responseMode: responseMode,
	}
}

func TestNew_MissingSTTProvider(t *testing.T) {
	config := Config{
		STT: STTConfig{
			Provider: "",
		},
		TTS: TTSConfig{
			Provider: "deepgram",
			APIKey:   "test-key",
		},
	}

	_, err := New(config, nil)
	if err == nil {
		t.Fatal("expected error for missing STT provider")
	}
	if err.Error() != "STT provider not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_MissingTTSProvider(t *testing.T) {
	config := Config{
		STT: STTConfig{
			Provider: "deepgram",
			APIKey:   "test-key",
		},
		TTS: TTSConfig{
			Provider: "",
		},
	}

	_, err := New(config, nil)
	if err == nil {
		t.Fatal("expected error for missing TTS provider")
	}
	if err.Error() != "TTS provider not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_UnsupportedSTTProvider(t *testing.T) {
	config := Config{
		STT: STTConfig{
			Provider: "unsupported",
		},
		TTS: TTSConfig{
			Provider: "deepgram",
			APIKey:   "test-key",
		},
	}

	_, err := New(config, nil)
	if err == nil {
		t.Fatal("expected error for unsupported STT provider")
	}
	// Error message includes available providers from registry
	if !strings.Contains(err.Error(), "STT provider not registered: unsupported") {
		t.Errorf("error = %q, want error containing 'STT provider not registered: unsupported'", err.Error())
	}
}

func TestNew_UnsupportedTTSProvider(t *testing.T) {
	config := Config{
		STT: STTConfig{
			Provider: "deepgram",
			APIKey:   "test-key",
		},
		TTS: TTSConfig{
			Provider: "unsupported",
		},
	}

	_, err := New(config, nil)
	if err == nil {
		t.Fatal("expected error for unsupported TTS provider")
	}
	// Error message includes available providers from registry
	if !strings.Contains(err.Error(), "TTS provider not registered: unsupported") {
		t.Errorf("error = %q, want error containing 'TTS provider not registered: unsupported'", err.Error())
	}
}

func TestNew_DefaultResponseMode(t *testing.T) {
	config := Config{
		STT: STTConfig{
			Provider: "deepgram",
			APIKey:   "test-key",
		},
		TTS: TTSConfig{
			Provider: "deepgram",
			APIKey:   "test-key",
		},
		ResponseMode: "", // empty should default to "auto"
	}

	p, err := New(config, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if p.ResponseMode() != "auto" {
		t.Errorf("ResponseMode() = %q, want %q", p.ResponseMode(), "auto")
	}
}

func TestTranscribeAudio_Success(t *testing.T) {
	sttProv := &mockSTTProvider{
		name: "mock-stt",
		transcribeFunc: func(ctx context.Context, audio []byte, config omnivoice.TranscriptionConfig) (*omnivoice.TranscriptionResult, error) {
			return &omnivoice.TranscriptionResult{
				Text:     "hello world",
				Language: "en-US",
			}, nil
		},
	}
	ttsProv := &mockTTSProvider{name: "mock-tts"}

	p := newTestProcessor(sttProv, ttsProv, Config{})

	text, err := p.TranscribeAudio(context.Background(), []byte("audio"), "audio/ogg")
	if err != nil {
		t.Fatalf("TranscribeAudio() error = %v", err)
	}
	if text != "hello world" {
		t.Errorf("TranscribeAudio() = %q, want %q", text, "hello world")
	}
}

func TestTranscribeAudio_Error(t *testing.T) {
	sttProv := &mockSTTProvider{
		name: "mock-stt",
		transcribeFunc: func(ctx context.Context, audio []byte, config omnivoice.TranscriptionConfig) (*omnivoice.TranscriptionResult, error) {
			return nil, errors.New("transcription failed")
		},
	}
	ttsProv := &mockTTSProvider{name: "mock-tts"}

	p := newTestProcessor(sttProv, ttsProv, Config{})

	_, err := p.TranscribeAudio(context.Background(), []byte("audio"), "audio/ogg")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "transcribe: transcription failed" {
		t.Errorf("error = %q, want wrapped error", err.Error())
	}
}

func TestTranscribeAudio_MimeTypeMapping(t *testing.T) {
	tests := []struct {
		mimeType         string
		expectedEncoding string
	}{
		{"audio/ogg; codecs=opus", "opus"},
		{"audio/ogg", "opus"},
		{"audio/mpeg", "mp3"},
		{"audio/mp3", "mp3"},
		{"audio/wav", "wav"},
		{"audio/wave", "wav"},
		{"audio/flac", "flac"},
		{"unknown/type", ""}, // unrecognized types get no encoding
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			var capturedConfig omnivoice.TranscriptionConfig

			sttProv := &mockSTTProvider{
				name: "mock-stt",
				transcribeFunc: func(ctx context.Context, audio []byte, config omnivoice.TranscriptionConfig) (*omnivoice.TranscriptionResult, error) {
					capturedConfig = config
					return &omnivoice.TranscriptionResult{Text: "test"}, nil
				},
			}
			ttsProv := &mockTTSProvider{name: "mock-tts"}

			p := newTestProcessor(sttProv, ttsProv, Config{})
			_, err := p.TranscribeAudio(context.Background(), []byte("audio"), tt.mimeType)
			if err != nil {
				t.Fatalf("TranscribeAudio() error = %v", err)
			}

			if capturedConfig.Encoding != tt.expectedEncoding {
				t.Errorf("encoding = %q, want %q", capturedConfig.Encoding, tt.expectedEncoding)
			}
		})
	}
}

func TestSynthesizeSpeech_Success(t *testing.T) {
	sttProv := &mockSTTProvider{name: "mock-stt"}
	ttsProv := &mockTTSProvider{
		name: "mock-tts",
		synthesizeFunc: func(ctx context.Context, text string, config omnivoice.SynthesisConfig) (*omnivoice.SynthesisResult, error) {
			return &omnivoice.SynthesisResult{
				Audio:  []byte("synthesized audio"),
				Format: "mp3",
			}, nil
		},
	}

	p := newTestProcessor(sttProv, ttsProv, Config{
		TTS: TTSConfig{
			VoiceID: "test-voice",
			Model:   "test-model",
		},
	})

	audio, mimeType, err := p.SynthesizeSpeech(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SynthesizeSpeech() error = %v", err)
	}
	if string(audio) != "synthesized audio" {
		t.Errorf("audio = %q, want %q", string(audio), "synthesized audio")
	}
	if mimeType != "audio/mpeg" {
		t.Errorf("mimeType = %q, want %q", mimeType, "audio/mpeg")
	}
}

func TestSynthesizeSpeech_Error(t *testing.T) {
	sttProv := &mockSTTProvider{name: "mock-stt"}
	ttsProv := &mockTTSProvider{
		name: "mock-tts",
		synthesizeFunc: func(ctx context.Context, text string, config omnivoice.SynthesisConfig) (*omnivoice.SynthesisResult, error) {
			return nil, errors.New("synthesis failed")
		},
	}

	p := newTestProcessor(sttProv, ttsProv, Config{})

	_, _, err := p.SynthesizeSpeech(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "synthesize: synthesis failed" {
		t.Errorf("error = %q, want wrapped error", err.Error())
	}
}

func TestSynthesizeSpeech_FormatToMimeType(t *testing.T) {
	tests := []struct {
		format       string
		expectedMime string
	}{
		{"mp3", "audio/mpeg"},
		{"opus", "audio/ogg; codecs=opus"},
		{"ogg", "audio/ogg; codecs=opus"},
		{"wav", "audio/wav"},
		{"unknown", "audio/mpeg"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			sttProv := &mockSTTProvider{name: "mock-stt"}
			ttsProv := &mockTTSProvider{
				name: "mock-tts",
				synthesizeFunc: func(ctx context.Context, text string, config omnivoice.SynthesisConfig) (*omnivoice.SynthesisResult, error) {
					return &omnivoice.SynthesisResult{
						Audio:  []byte("audio"),
						Format: tt.format,
					}, nil
				},
			}

			p := newTestProcessor(sttProv, ttsProv, Config{})
			_, mimeType, err := p.SynthesizeSpeech(context.Background(), "test")
			if err != nil {
				t.Fatalf("SynthesizeSpeech() error = %v", err)
			}

			if mimeType != tt.expectedMime {
				t.Errorf("mimeType = %q, want %q", mimeType, tt.expectedMime)
			}
		})
	}
}

func TestResponseMode(t *testing.T) {
	tests := []struct {
		configMode   string
		expectedMode string
	}{
		{"auto", "auto"},
		{"always", "always"},
		{"never", "never"},
		{"", "auto"}, // empty defaults to auto
	}

	for _, tt := range tests {
		t.Run(tt.configMode, func(t *testing.T) {
			sttProv := &mockSTTProvider{name: "mock-stt"}
			ttsProv := &mockTTSProvider{name: "mock-tts"}

			p := newTestProcessor(sttProv, ttsProv, Config{
				ResponseMode: tt.configMode,
			})

			if p.ResponseMode() != tt.expectedMode {
				t.Errorf("ResponseMode() = %q, want %q", p.ResponseMode(), tt.expectedMode)
			}
		})
	}
}

func TestClose(t *testing.T) {
	sttProv := &mockSTTProvider{name: "mock-stt"}
	ttsProv := &mockTTSProvider{name: "mock-tts"}

	p := newTestProcessor(sttProv, ttsProv, Config{})

	err := p.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
