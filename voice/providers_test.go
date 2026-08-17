package voice

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/plexusone/omnivoice-core/gateway"
	"github.com/plexusone/omnivoice-core/registry"
)

func TestConvertTools(t *testing.T) {
	tools := []ToolDefinition{
		{Name: "lookup", Description: "look something up", Parameters: map[string]any{"query": "string"}},
		{Name: "book", Description: "book an appointment", Parameters: nil},
	}

	got := convertTools(tools)
	if len(got) != len(tools) {
		t.Fatalf("convertTools() returned %d tools, want %d", len(got), len(tools))
	}
	for i, tool := range tools {
		if got[i].Name != tool.Name || got[i].Description != tool.Description {
			t.Errorf("convertTools()[%d] = %+v, want name/description from %+v", i, got[i], tool)
		}
	}
}

func TestConvertTools_Empty(t *testing.T) {
	got := convertTools(nil)
	if len(got) != 0 {
		t.Errorf("convertTools(nil) = %v, want empty slice", got)
	}
}

func TestConvertHandlers_Nil(t *testing.T) {
	if got := convertHandlers(nil); got != nil {
		t.Errorf("convertHandlers(nil) = %v, want nil", got)
	}
}

func TestConvertHandlers_PreservesPerNameBehavior(t *testing.T) {
	// Regression test: a naive loop-variable capture would make every
	// converted handler call the *last* registered handler. Verify each
	// name still dispatches to its own handler.
	handlers := map[string]ToolHandler{
		"a": func(ctx context.Context, args map[string]any) (string, error) { return "from-a", nil },
		"b": func(ctx context.Context, args map[string]any) (string, error) { return "from-b", nil },
	}

	got := convertHandlers(handlers)
	if len(got) != 2 {
		t.Fatalf("convertHandlers() returned %d handlers, want 2", len(got))
	}

	for name, want := range map[string]string{"a": "from-a", "b": "from-b"} {
		h, ok := got[name]
		if !ok {
			t.Fatalf("convertHandlers() missing handler %q", name)
		}
		result, err := h(context.Background(), nil)
		if err != nil {
			t.Fatalf("handler %q returned error: %v", name, err)
		}
		if result != want {
			t.Errorf("handler %q returned %q, want %q", name, result, want)
		}
	}
}

func TestBuildCommonGatewayOptions(t *testing.T) {
	logger := slog.Default()
	cfg := GatewayConfig{
		ListenAddr: ":9090",
		PublicURL:  "https://example.com",
		Config: Config{
			STT: STTConfig{Provider: "deepgram", APIKey: "stt-key", Model: "nova-2", Language: "en-US"},
			TTS: TTSConfig{Provider: "elevenlabs", APIKey: "tts-key", VoiceID: "voice-1", Model: "tts-model"},
		},
		LLMProvider:        "anthropic",
		LLMModel:           "claude-sonnet-4",
		LLMSystemPrompt:    "be helpful",
		Greeting:           "hello!",
		MaxSessionDuration: 15 * time.Minute,
		InterruptionMode:   "polite",
		Logger:             logger,
	}

	opts := buildCommonGatewayOptions(cfg)
	applied := registry.ApplyOptions(opts...)

	want := map[string]any{
		"listenAddr":         cfg.ListenAddr,
		"publicURL":          cfg.PublicURL,
		"sttProvider":        cfg.Config.STT.Provider,
		"sttAPIKey":          cfg.Config.STT.APIKey,
		"sttModel":           cfg.Config.STT.Model,
		"sttLanguage":        cfg.Config.STT.Language,
		"ttsProvider":        cfg.Config.TTS.Provider,
		"ttsAPIKey":          cfg.Config.TTS.APIKey,
		"ttsVoiceID":         cfg.Config.TTS.VoiceID,
		"ttsModel":           cfg.Config.TTS.Model,
		"llmProvider":        cfg.LLMProvider,
		"llmModel":           cfg.LLMModel,
		"llmSystemPrompt":    cfg.LLMSystemPrompt,
		"greeting":           cfg.Greeting,
		"maxSessionDuration": cfg.MaxSessionDuration,
		"interruptionMode":   cfg.InterruptionMode,
	}

	for key, wantVal := range want {
		gotVal, ok := applied.Extensions[key]
		if !ok {
			t.Errorf("Extensions[%q] missing", key)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("Extensions[%q] = %v, want %v", key, gotVal, wantVal)
		}
	}

	if applied.Extensions["logger"] != logger {
		t.Errorf("Extensions[\"logger\"] = %v, want the configured logger", applied.Extensions["logger"])
	}
}

// gatewayWrapper implements the `interface{ Gateway() gateway.Gateway }` seam
// that extractGateway looks for.
type gatewayWrapper struct {
	inner gateway.Gateway
}

func (w *gatewayWrapper) Name() string             { return string(w.inner.Name()) }
func (w *gatewayWrapper) Start(ctx any) error      { return w.inner.Start(context.Background()) }
func (w *gatewayWrapper) Stop() error              { return w.inner.Stop() }
func (w *gatewayWrapper) Gateway() gateway.Gateway { return w.inner }

// plainRegistryGateway implements only registry.Gateway (no Gateway() seam),
// forcing extractGateway down the adapter fallback path.
type plainRegistryGateway struct {
	name     string
	startErr error
	stopErr  error
	startCtx any
}

func (p *plainRegistryGateway) Name() string { return p.name }
func (p *plainRegistryGateway) Start(ctx any) error {
	p.startCtx = ctx
	return p.startErr
}
func (p *plainRegistryGateway) Stop() error { return p.stopErr }

func TestExtractGateway_UnwrapsWrapper(t *testing.T) {
	inner := &fakeVoiceGateway{name: gateway.ProviderTwilio}
	wrapper := &gatewayWrapper{inner: inner}

	got := extractGateway(wrapper)
	if got != inner {
		t.Errorf("extractGateway() = %v, want the unwrapped inner gateway %v", got, inner)
	}
}

func TestExtractGateway_FallsBackToAdapter(t *testing.T) {
	plain := &plainRegistryGateway{name: "telnyx"}

	got := extractGateway(plain)
	adapter, ok := got.(*registryGatewayAdapter)
	if !ok {
		t.Fatalf("extractGateway() = %T, want *registryGatewayAdapter", got)
	}

	if got.Name() != gateway.ProviderName("telnyx") {
		t.Errorf("adapter.Name() = %q, want %q", got.Name(), "telnyx")
	}

	if err := adapter.Start(context.Background()); err != nil {
		t.Fatalf("adapter.Start() error = %v", err)
	}
	if plain.startCtx == nil {
		t.Error("adapter.Start() did not delegate to the underlying registry.Gateway")
	}

	if err := adapter.Stop(); err != nil {
		t.Fatalf("adapter.Stop() error = %v", err)
	}

	// OnCall is a documented no-op via the adapter; it must not panic.
	adapter.OnCall(func(call *gateway.CallInfo) error { return nil })

	if _, err := adapter.MakeCall(context.Background(), "+15555550100"); err == nil {
		t.Error("adapter.MakeCall() should return an error (unsupported via adapter)")
	}

	if _, ok := adapter.GetSession("any"); ok {
		t.Error("adapter.GetSession() should always report ok=false")
	}

	if sessions := adapter.ListSessions(); sessions != nil {
		t.Errorf("adapter.ListSessions() = %v, want nil", sessions)
	}
}

func TestExtractGateway_AdapterPropagatesErrors(t *testing.T) {
	wantStartErr := errors.New("start failed")
	wantStopErr := errors.New("stop failed")
	plain := &plainRegistryGateway{name: "telnyx", startErr: wantStartErr, stopErr: wantStopErr}

	adapter := extractGateway(plain).(*registryGatewayAdapter)

	if err := adapter.Start(context.Background()); !errors.Is(err, wantStartErr) {
		t.Errorf("adapter.Start() error = %v, want %v", err, wantStartErr)
	}
	if err := adapter.Stop(); !errors.Is(err, wantStopErr) {
		t.Errorf("adapter.Stop() error = %v, want %v", err, wantStopErr)
	}
}

func TestResolveRealtimeAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		env      map[string]string
		want     string
	}{
		{
			name:     "openai uses OPENAI_API_KEY",
			provider: "openai",
			env:      map[string]string{"OPENAI_API_KEY": "openai-key"},
			want:     "openai-key",
		},
		{
			name:     "gemini prefers GEMINI_API_KEY",
			provider: "gemini",
			env:      map[string]string{"GEMINI_API_KEY": "gemini-key", "GOOGLE_API_KEY": "google-key"},
			want:     "gemini-key",
		},
		{
			name:     "gemini falls back to GOOGLE_API_KEY",
			provider: "gemini",
			env:      map[string]string{"GOOGLE_API_KEY": "google-key"},
			want:     "google-key",
		},
		{
			name:     "unknown provider returns empty",
			provider: "unknown",
			env:      nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENAI_API_KEY", "")
			t.Setenv("GEMINI_API_KEY", "")
			t.Setenv("GOOGLE_API_KEY", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			got := resolveRealtimeAPIKey(tt.provider)
			if got != tt.want {
				t.Errorf("resolveRealtimeAPIKey(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestCreateTwilioGateway_Success(t *testing.T) {
	cfg := GatewayConfig{
		Provider:         "twilio",
		TwilioAccountSID: "AC_fake",
		TwilioAuthToken:  "fake-token",
		TwilioPhone:      "+15555550100",
		PublicURL:        "https://example.com",
		ListenAddr:       ":0",
	}

	gw, err := createTwilioGateway(cfg)
	if err != nil {
		t.Fatalf("createTwilioGateway() error = %v", err)
	}
	if gw.Name() != gateway.ProviderTwilio {
		t.Errorf("Name() = %q, want %q", gw.Name(), gateway.ProviderTwilio)
	}
}

func TestCreateTwilioGateway_MissingCredentials(t *testing.T) {
	cfg := GatewayConfig{
		Provider: "twilio",
		// AccountSID/AuthToken intentionally omitted.
	}

	if _, err := createTwilioGateway(cfg); err == nil {
		t.Fatal("expected error for missing Twilio credentials")
	}
}

func TestCreateTelnyxGateway_Success(t *testing.T) {
	cfg := GatewayConfig{
		Provider:           "telnyx",
		TelnyxAPIKey:       "fake-key",
		TelnyxPhone:        "+15555550100",
		TelnyxConnectionID: "conn-123",
		PublicURL:          "https://example.com",
		ListenAddr:         ":0",
	}

	gw, err := createTelnyxGateway(cfg)
	if err != nil {
		t.Fatalf("createTelnyxGateway() error = %v", err)
	}
	if gw.Name() != gateway.ProviderTelnyx {
		t.Errorf("Name() = %q, want %q", gw.Name(), gateway.ProviderTelnyx)
	}
}

func TestCreateTelnyxGateway_MissingCredentials(t *testing.T) {
	cfg := GatewayConfig{Provider: "telnyx"}

	if _, err := createTelnyxGateway(cfg); err == nil {
		t.Fatal("expected error for missing Telnyx credentials")
	}
}

func TestBuildRealtimeOptions_UnknownProvider(t *testing.T) {
	_, err := buildRealtimeOptions(GatewayConfig{RealtimeProvider: "not-a-real-provider"})
	if err == nil {
		t.Fatal("expected error for unknown realtime provider")
	}
}

func TestBuildRealtimeOptions_Success(t *testing.T) {
	cfg := GatewayConfig{
		RealtimeProvider: "openai",
		RealtimeAPIKey:   "explicit-key",
		RealtimeModel:    "gpt-4o-realtime-preview",
		RealtimeVoice:    "alloy",
		LLMSystemPrompt:  "voice system prompt",
	}

	opts, err := buildRealtimeOptions(cfg)
	if err != nil {
		t.Fatalf("buildRealtimeOptions() error = %v", err)
	}
	if len(opts) != 3 {
		t.Fatalf("buildRealtimeOptions() returned %d options, want 3", len(opts))
	}

	applied := registry.ApplyOptions(opts...)
	if applied.Extensions["pipelineMode"] != "realtime" {
		t.Errorf("Extensions[\"pipelineMode\"] = %v, want %q", applied.Extensions["pipelineMode"], "realtime")
	}
}

func TestBuildRealtimeOptions_ResolvesAPIKeyFromEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-key")

	cfg := GatewayConfig{RealtimeProvider: "openai"} // RealtimeAPIKey left blank

	opts, err := buildRealtimeOptions(cfg)
	if err != nil {
		t.Fatalf("buildRealtimeOptions() error = %v", err)
	}
	if len(opts) != 3 {
		t.Fatalf("buildRealtimeOptions() returned %d options, want 3", len(opts))
	}
}
