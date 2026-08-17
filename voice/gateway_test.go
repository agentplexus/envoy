package voice

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/plexusone/omnillm"
	"github.com/plexusone/omnillm-core/provider"
	"github.com/plexusone/omnivoice-core/gateway"
)

// newTestLLMClient builds a real *omnillm.ChatClient backed by a fake
// provider.Provider (via CustomProvider), so ProcessWithAgent/Generate
// exercise the genuine omnillm request/response path with no network calls.
func newTestLLMClient(t *testing.T, fake *fakeLLMProvider) *omnillm.ChatClient {
	t.Helper()
	client, err := omnillm.NewClient(omnillm.ClientConfig{
		Providers: []omnillm.ProviderConfig{
			{CustomProvider: fake},
		},
	})
	if err != nil {
		t.Fatalf("omnillm.NewClient() error = %v", err)
	}
	return client
}

func newTestGateway(t *testing.T, voiceProvider *fakeVoiceGateway, fake *fakeLLMProvider, cfg GatewayConfig) *Gateway {
	t.Helper()
	return &Gateway{
		config:   cfg,
		provider: voiceProvider,
		llm:      newTestLLMClient(t, fake),
		llmModel: "test-model",
		logger:   slog.Default(),
	}
}

func TestGateway_OnCall(t *testing.T) {
	fakeGW := &fakeVoiceGateway{}
	gw := newTestGateway(t, fakeGW, &fakeLLMProvider{}, GatewayConfig{})

	called := false
	handler := func(call *gateway.CallInfo) error {
		called = true
		return nil
	}
	gw.OnCall(handler)

	if fakeGW.onCallHandler == nil {
		t.Fatal("OnCall() did not register a handler with the underlying provider")
	}
	if err := fakeGW.onCallHandler(&gateway.CallInfo{CallID: "call-1"}); err != nil {
		t.Fatalf("registered handler returned error: %v", err)
	}
	if !called {
		t.Error("registered handler was not the one passed to OnCall")
	}
}

func TestGateway_OnSessionStartEnd(t *testing.T) {
	fakeGW := &fakeVoiceGateway{}
	gw := newTestGateway(t, fakeGW, &fakeLLMProvider{}, GatewayConfig{})

	var started, ended gateway.Session
	gw.OnSessionStart(func(session gateway.Session) { started = session })
	gw.OnSessionEnd(func(session gateway.Session) { ended = session })

	session := &fakeSession{id: "s1"}
	gw.onSessionStart(session)
	gw.onSessionEnd(session)

	if started != session {
		t.Error("OnSessionStart callback was not invoked with the session")
	}
	if ended != session {
		t.Error("OnSessionEnd callback was not invoked with the session")
	}
}

func TestGateway_StartStop(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fakeGW := &fakeVoiceGateway{name: gateway.ProviderTwilio}
		gw := newTestGateway(t, fakeGW, &fakeLLMProvider{}, GatewayConfig{ListenAddr: ":0"})

		if err := gw.Start(context.Background()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if !fakeGW.startCalled {
			t.Error("Start() did not delegate to the underlying provider")
		}

		if err := gw.Stop(); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if !fakeGW.stopCalled {
			t.Error("Stop() did not delegate to the underlying provider")
		}
	})

	t.Run("start error propagates", func(t *testing.T) {
		wantErr := errors.New("listen failed")
		fakeGW := &fakeVoiceGateway{startErr: wantErr}
		gw := newTestGateway(t, fakeGW, &fakeLLMProvider{}, GatewayConfig{})

		if err := gw.Start(context.Background()); !errors.Is(err, wantErr) {
			t.Fatalf("Start() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("stop error propagates", func(t *testing.T) {
		wantErr := errors.New("shutdown failed")
		fakeGW := &fakeVoiceGateway{stopErr: wantErr}
		gw := newTestGateway(t, fakeGW, &fakeLLMProvider{}, GatewayConfig{})

		if err := gw.Stop(); !errors.Is(err, wantErr) {
			t.Fatalf("Stop() error = %v, want %v", err, wantErr)
		}
	})
}

func TestGateway_MakeCall(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wantSession := &fakeSession{id: "call-1"}
		fakeGW := &fakeVoiceGateway{
			makeCallFn: func(ctx context.Context, to string) (gateway.Session, error) {
				if to != "+15555550100" {
					t.Errorf("MakeCall() to = %q, want %q", to, "+15555550100")
				}
				return wantSession, nil
			},
		}
		gw := newTestGateway(t, fakeGW, &fakeLLMProvider{}, GatewayConfig{})

		session, err := gw.MakeCall(context.Background(), "+15555550100")
		if err != nil {
			t.Fatalf("MakeCall() error = %v", err)
		}
		if session != wantSession {
			t.Error("MakeCall() did not return the session from the underlying provider")
		}
	})

	t.Run("error propagates", func(t *testing.T) {
		wantErr := errors.New("carrier rejected")
		fakeGW := &fakeVoiceGateway{
			makeCallFn: func(ctx context.Context, to string) (gateway.Session, error) {
				return nil, wantErr
			},
		}
		gw := newTestGateway(t, fakeGW, &fakeLLMProvider{}, GatewayConfig{})

		_, err := gw.MakeCall(context.Background(), "+15555550100")
		if !errors.Is(err, wantErr) {
			t.Fatalf("MakeCall() error = %v, want %v", err, wantErr)
		}
	})
}

func TestGateway_GetSession(t *testing.T) {
	wantSession := &fakeSession{id: "call-1"}
	fakeGW := &fakeVoiceGateway{
		sessions: map[string]gateway.Session{"call-1": wantSession},
	}
	gw := newTestGateway(t, fakeGW, &fakeLLMProvider{}, GatewayConfig{})

	session, ok := gw.GetSession("call-1")
	if !ok || session != wantSession {
		t.Fatalf("GetSession(%q) = (%v, %v), want (%v, true)", "call-1", session, ok, wantSession)
	}

	_, ok = gw.GetSession("missing")
	if ok {
		t.Error("GetSession() for unknown call ID should return ok=false")
	}
}

func TestGateway_ListSessions(t *testing.T) {
	want := []gateway.Session{&fakeSession{id: "a"}, &fakeSession{id: "b"}}
	fakeGW := &fakeVoiceGateway{listSessions: want}
	gw := newTestGateway(t, fakeGW, &fakeLLMProvider{}, GatewayConfig{})

	got := gw.ListSessions()
	if len(got) != len(want) {
		t.Fatalf("ListSessions() returned %d sessions, want %d", len(got), len(want))
	}
}

func TestGateway_ProcessWithAgent_Success(t *testing.T) {
	fake := &fakeLLMProvider{
		completionFunc: func(ctx context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
			return &provider.ChatCompletionResponse{
				Choices: []provider.ChatCompletionChoice{
					{Message: provider.Message{Role: provider.RoleAssistant, Content: "hello there"}},
				},
			}, nil
		},
	}
	fakeGW := &fakeVoiceGateway{}
	gw := newTestGateway(t, fakeGW, fake, GatewayConfig{LLMSystemPrompt: "be nice"})

	session := &fakeSession{
		transcript: []gateway.Turn{
			{Role: "user", Text: "hi"},
			{Role: "agent", Text: "hello, how can I help?"},
		},
	}

	resp, err := gw.ProcessWithAgent(context.Background(), session, "what's the weather?")
	if err != nil {
		t.Fatalf("ProcessWithAgent() error = %v", err)
	}
	if resp != "hello there" {
		t.Errorf("ProcessWithAgent() = %q, want %q", resp, "hello there")
	}

	if fake.lastRequest == nil {
		t.Fatal("provider was not called")
	}
	msgs := fake.lastRequest.Messages
	if len(msgs) != 4 {
		t.Fatalf("got %d messages, want 4 (system, user, agent, user): %+v", len(msgs), msgs)
	}
	if msgs[0].Role != omnillm.RoleSystem || msgs[0].Content != "be nice" {
		t.Errorf("messages[0] = %+v, want system prompt", msgs[0])
	}
	if msgs[1].Role != omnillm.RoleUser || msgs[1].Content != "hi" {
		t.Errorf("messages[1] = %+v, want user 'hi'", msgs[1])
	}
	if msgs[2].Role != omnillm.RoleAssistant || msgs[2].Content != "hello, how can I help?" {
		t.Errorf("messages[2] = %+v, want assistant turn", msgs[2])
	}
	if msgs[3].Role != omnillm.RoleUser || msgs[3].Content != "what's the weather?" {
		t.Errorf("messages[3] = %+v, want current user text", msgs[3])
	}
}

func TestGateway_ProcessWithAgent_NoSystemPrompt(t *testing.T) {
	fake := &fakeLLMProvider{}
	fakeGW := &fakeVoiceGateway{}
	gw := newTestGateway(t, fakeGW, fake, GatewayConfig{}) // no LLMSystemPrompt

	session := &fakeSession{}
	if _, err := gw.ProcessWithAgent(context.Background(), session, "hi"); err != nil {
		t.Fatalf("ProcessWithAgent() error = %v", err)
	}

	if len(fake.lastRequest.Messages) != 1 {
		t.Fatalf("got %d messages, want 1 (no system prompt configured): %+v", len(fake.lastRequest.Messages), fake.lastRequest.Messages)
	}
}

func TestGateway_ProcessWithAgent_LLMError(t *testing.T) {
	wantErr := errors.New("rate limited")
	fake := &fakeLLMProvider{
		completionFunc: func(ctx context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
			return nil, wantErr
		},
	}
	gw := newTestGateway(t, &fakeVoiceGateway{}, fake, GatewayConfig{})

	_, err := gw.ProcessWithAgent(context.Background(), &fakeSession{}, "hi")
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("ProcessWithAgent() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestGateway_ProcessWithAgent_NoChoices(t *testing.T) {
	fake := &fakeLLMProvider{
		completionFunc: func(ctx context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
			return &provider.ChatCompletionResponse{Choices: nil}, nil
		},
	}
	gw := newTestGateway(t, &fakeVoiceGateway{}, fake, GatewayConfig{})

	_, err := gw.ProcessWithAgent(context.Background(), &fakeSession{}, "hi")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
	if err.Error() != "no response from LLM" {
		t.Errorf("error = %q, want %q", err.Error(), "no response from LLM")
	}
}

func TestVoiceAgentLLMProvider_Generate(t *testing.T) {
	fake := &fakeLLMProvider{
		completionFunc: func(ctx context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
			return &provider.ChatCompletionResponse{
				Choices: []provider.ChatCompletionChoice{
					{Message: provider.Message{Role: provider.RoleAssistant, Content: "generated reply"}},
				},
			}, nil
		},
	}
	gw := newTestGateway(t, &fakeVoiceGateway{}, fake, GatewayConfig{LLMSystemPrompt: "voice sys prompt"})
	llmProvider := NewVoiceAgentLLMProvider(gw)

	history := []gateway.Turn{
		{Role: "user", Text: "hey", Timestamp: time.Now()},
		{Role: "agent", Text: "hi there", Timestamp: time.Now()},
	}

	text, toolCalls, err := llmProvider.Generate(context.Background(), "what's next?", history)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if text != "generated reply" {
		t.Errorf("Generate() text = %q, want %q", text, "generated reply")
	}
	if len(toolCalls) != 0 {
		t.Errorf("Generate() toolCalls = %v, want empty", toolCalls)
	}

	msgs := fake.lastRequest.Messages
	if len(msgs) != 4 {
		t.Fatalf("got %d messages, want 4: %+v", len(msgs), msgs)
	}
	if msgs[0].Content != "voice sys prompt" {
		t.Errorf("messages[0] = %+v, want system prompt", msgs[0])
	}
	if msgs[3].Content != "what's next?" {
		t.Errorf("messages[3] = %+v, want input text", msgs[3])
	}
}

func TestVoiceAgentLLMProvider_Generate_Error(t *testing.T) {
	wantErr := errors.New("provider down")
	fake := &fakeLLMProvider{
		completionFunc: func(ctx context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
			return nil, wantErr
		},
	}
	gw := newTestGateway(t, &fakeVoiceGateway{}, fake, GatewayConfig{})
	llmProvider := NewVoiceAgentLLMProvider(gw)

	_, _, err := llmProvider.Generate(context.Background(), "hi", nil)
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("Generate() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestVoiceAgentLLMProvider_Generate_NoChoices(t *testing.T) {
	fake := &fakeLLMProvider{
		completionFunc: func(ctx context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
			return &provider.ChatCompletionResponse{}, nil
		},
	}
	gw := newTestGateway(t, &fakeVoiceGateway{}, fake, GatewayConfig{})
	llmProvider := NewVoiceAgentLLMProvider(gw)

	_, _, err := llmProvider.Generate(context.Background(), "hi", nil)
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestNewGateway_UnsupportedProvider(t *testing.T) {
	_, err := NewGateway(GatewayConfig{Provider: "asterisk"})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	want := "unsupported voice provider: asterisk (supported: twilio, telnyx)"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestNewGateway_MissingLLMAPIKey(t *testing.T) {
	// Ensure no ambient credentials leak into the test.
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := NewGateway(GatewayConfig{
		Provider:         "twilio",
		TwilioAccountSID: "AC_fake",
		TwilioAuthToken:  "fake_token",
		LLMProvider:      "anthropic",
	})
	if err == nil {
		t.Fatal("expected error for missing LLM API key")
	}
}

func TestNewGateway_TwilioSuccessWithDefaults(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "fake-anthropic-key")

	gw, err := NewGateway(GatewayConfig{
		Provider:         "twilio",
		TwilioAccountSID: "AC_fake",
		TwilioAuthToken:  "fake_token",
		PublicURL:        "https://example.com",
		LLMProvider:      "anthropic",
	})
	if err != nil {
		t.Fatalf("NewGateway() error = %v", err)
	}
	if gw == nil {
		t.Fatal("NewGateway() returned nil gateway")
	}
	if gw.llmModel != "claude-sonnet-4-20250514" {
		t.Errorf("llmModel = %q, want default claude model", gw.llmModel)
	}
	if gw.config.ListenAddr != ":8080" {
		t.Errorf("ListenAddr default = %q, want %q", gw.config.ListenAddr, ":8080")
	}
	if gw.config.MaxSessionDuration != 30*time.Minute {
		t.Errorf("MaxSessionDuration default = %v, want 30m", gw.config.MaxSessionDuration)
	}
	if gw.config.InterruptionMode != "immediate" {
		t.Errorf("InterruptionMode default = %q, want %q", gw.config.InterruptionMode, "immediate")
	}
	if sessions := gw.ListSessions(); len(sessions) != 0 {
		t.Errorf("ListSessions() = %v, want empty for a freshly created gateway", sessions)
	}
	if _, ok := gw.GetSession("nope"); ok {
		t.Error("GetSession() should report false for a freshly created gateway")
	}
}

func TestNewGateway_TelnyxSuccessWithExplicitModel(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "fake-openai-key")

	gw, err := NewGateway(GatewayConfig{
		Provider:           "telnyx",
		TelnyxAPIKey:       "fake",
		TelnyxConnectionID: "conn-1",
		LLMProvider:        "openai",
		LLMModel:           "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("NewGateway() error = %v", err)
	}
	if gw.llmModel != "gpt-4o-mini" {
		t.Errorf("llmModel = %q, want explicit override %q", gw.llmModel, "gpt-4o-mini")
	}
}

func TestNewGateway_EnvCredentialDefaults(t *testing.T) {
	t.Setenv("TWILIO_ACCOUNT_SID", "AC_env")
	t.Setenv("TWILIO_AUTH_TOKEN", "token_env")
	t.Setenv("ANTHROPIC_API_KEY", "fake-anthropic-key")

	_, err := NewGateway(GatewayConfig{
		Provider:    "twilio",
		LLMProvider: "anthropic",
		// TwilioAccountSID/TwilioAuthToken intentionally left blank to
		// exercise the environment-variable fallback.
	})
	if err != nil {
		t.Fatalf("NewGateway() error = %v, want env credentials to be picked up", err)
	}
}

func TestNewGateway_RealtimeModeUnknownProvider(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "fake-anthropic-key")

	_, err := NewGateway(GatewayConfig{
		Provider:         "twilio",
		TwilioAccountSID: "AC_fake",
		TwilioAuthToken:  "fake_token",
		LLMProvider:      "anthropic",
		Mode:             "realtime",
		RealtimeProvider: "not-a-real-provider",
	})
	if err == nil {
		t.Fatal("expected error for unknown realtime provider")
	}
}
