package voice

import (
	"context"
	"errors"
	"time"

	"github.com/plexusone/omnillm-core/provider"
	"github.com/plexusone/omnivoice-core/gateway"
)

// fakeVoiceGateway is a minimal fake of gateway.Gateway used to test the
// Gateway wrapper's delegation logic without touching a real telephony
// provider or network.
type fakeVoiceGateway struct {
	name gateway.ProviderName

	startErr error
	stopErr  error

	startCalled bool
	stopCalled  bool

	onCallHandler gateway.CallHandler

	makeCallFn func(ctx context.Context, to string) (gateway.Session, error)

	sessions     map[string]gateway.Session
	listSessions []gateway.Session
}

func (f *fakeVoiceGateway) Name() gateway.ProviderName { return f.name }

func (f *fakeVoiceGateway) Start(ctx context.Context) error {
	f.startCalled = true
	return f.startErr
}

func (f *fakeVoiceGateway) Stop() error {
	f.stopCalled = true
	return f.stopErr
}

func (f *fakeVoiceGateway) OnCall(handler gateway.CallHandler) {
	f.onCallHandler = handler
}

func (f *fakeVoiceGateway) MakeCall(ctx context.Context, to string) (gateway.Session, error) {
	if f.makeCallFn != nil {
		return f.makeCallFn(ctx, to)
	}
	return nil, errors.New("MakeCall not configured")
}

func (f *fakeVoiceGateway) GetSession(callID string) (gateway.Session, bool) {
	s, ok := f.sessions[callID]
	return s, ok
}

func (f *fakeVoiceGateway) ListSessions() []gateway.Session { return f.listSessions }

// fakeSession is a minimal fake of gateway.Session.
type fakeSession struct {
	id         string
	from       string
	to         string
	direction  string
	startTime  time.Time
	duration   time.Duration
	transcript []gateway.Turn
	metrics    gateway.Metrics

	sendTextErr error
	sentTexts   []string
	interrupted bool
	closeErr    error
	closed      bool
}

func (s *fakeSession) ID() string                   { return s.id }
func (s *fakeSession) From() string                 { return s.from }
func (s *fakeSession) To() string                   { return s.to }
func (s *fakeSession) Direction() string            { return s.direction }
func (s *fakeSession) StartTime() time.Time         { return s.startTime }
func (s *fakeSession) Duration() time.Duration      { return s.duration }
func (s *fakeSession) Events() <-chan gateway.Event { return nil }
func (s *fakeSession) Transcript() []gateway.Turn   { return s.transcript }
func (s *fakeSession) Metrics() gateway.Metrics     { return s.metrics }

func (s *fakeSession) SendText(text string) error {
	s.sentTexts = append(s.sentTexts, text)
	return s.sendTextErr
}

func (s *fakeSession) Interrupt() { s.interrupted = true }

func (s *fakeSession) Close() error {
	s.closed = true
	return s.closeErr
}

// fakeLLMProvider implements omnillm-core/provider.Provider so we can inject
// it as a CustomProvider into a real *omnillm.ChatClient without any network
// access.
type fakeLLMProvider struct {
	name           string
	completionFunc func(ctx context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error)
	lastRequest    *provider.ChatCompletionRequest
}

func (f *fakeLLMProvider) CreateChatCompletion(ctx context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
	f.lastRequest = req
	if f.completionFunc != nil {
		return f.completionFunc(ctx, req)
	}
	return &provider.ChatCompletionResponse{
		Choices: []provider.ChatCompletionChoice{
			{Message: provider.Message{Role: provider.RoleAssistant, Content: "mock response"}},
		},
	}, nil
}

func (f *fakeLLMProvider) CreateChatCompletionStream(ctx context.Context, req *provider.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeLLMProvider) Close() error { return nil }

func (f *fakeLLMProvider) Name() string { return f.name }
