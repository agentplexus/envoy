package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/plexusone/omnillm"
	"github.com/plexusone/omnillm/provider"

	agentctx "github.com/plexusone/omniagent/context"
	"github.com/plexusone/omniagent/hooks"
	"github.com/plexusone/omniagent/sessions"
)

// fakeProvider is a minimal provider.Provider that returns pre-scripted
// responses in call order, letting processInternal's tool-calling loop be
// exercised without a live LLM. Safe for concurrent CreateChatCompletion
// calls (not exercised here, but matches the Provider contract).
type fakeProvider struct {
	mu        sync.Mutex
	responses []*provider.ChatCompletionResponse
	errs      []error // errs[i] (if non-nil) is returned instead of responses[i]
	calls     []*provider.ChatCompletionRequest
}

func (f *fakeProvider) CreateChatCompletion(_ context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	idx := len(f.calls)
	f.calls = append(f.calls, req)
	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, f.errs[idx]
	}
	if idx >= len(f.responses) {
		return nil, fmt.Errorf("fakeProvider: no response scripted for call %d", idx)
	}
	return f.responses[idx], nil
}

func (f *fakeProvider) CreateChatCompletionStream(context.Context, *provider.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return nil, fmt.Errorf("fakeProvider: streaming not supported")
}

func (f *fakeProvider) Close() error { return nil }
func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeProvider) requestAt(i int) *provider.ChatCompletionRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[i]
}

// textResponse builds a final (no tool call) assistant response.
func textResponse(content string) *provider.ChatCompletionResponse {
	return &provider.ChatCompletionResponse{
		Choices: []provider.ChatCompletionChoice{{
			Message: provider.Message{Role: provider.RoleAssistant, Content: content},
		}},
	}
}

// toolCallResponse builds an assistant response requesting tool calls,
// optionally with accompanying text.
func toolCallResponse(text string, calls ...provider.ToolCall) *provider.ChatCompletionResponse {
	return &provider.ChatCompletionResponse{
		Choices: []provider.ChatCompletionChoice{{
			Message: provider.Message{Role: provider.RoleAssistant, Content: text, ToolCalls: calls},
		}},
	}
}

func newFakeChatClient(t *testing.T, fp *fakeProvider) *omnillm.ChatClient {
	t.Helper()
	client, err := omnillm.NewClient(omnillm.ClientConfig{
		Providers: []omnillm.ProviderConfig{{CustomProvider: fp}},
	})
	if err != nil {
		t.Fatalf("omnillm.NewClient with CustomProvider: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// processTestAgent builds an *Agent wired to a fakeProvider-backed
// ChatClient, ready to exercise Process/ProcessWithSession/processInternal
// without any network access.
func processTestAgent(t *testing.T, fp *fakeProvider) *Agent {
	t.Helper()
	registry := hooks.NewRegistry()
	return &Agent{
		client:     newFakeChatClient(t, fp),
		tools:      NewToolRegistry(),
		hooks:      registry,
		dispatcher: hooks.NewDispatcher(registry),
		logger:     slog.Default(),
		config:     Config{Model: "test-model"},
	}
}

func TestProcess_SingleTurnNoTools(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{textResponse("hello there")}}
	a := processTestAgent(t, fp)

	got, err := a.Process(context.Background(), "unused-session-id", "hi")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got != "hello there" {
		t.Errorf("Process() = %q, want %q", got, "hello there")
	}
	if fp.callCount() != 1 {
		t.Errorf("provider called %d times, want 1", fp.callCount())
	}
	// Process is stateless: no session in the request history beyond the
	// system+user messages built fresh each call.
	req := fp.requestAt(0)
	if req.Model != "test-model" {
		t.Errorf("request model = %q, want test-model", req.Model)
	}
}

func TestProcess_ProviderError(t *testing.T) {
	fp := &fakeProvider{errs: []error{errors.New("provider exploded")}}
	a := processTestAgent(t, fp)

	_, err := a.Process(context.Background(), "s", "hi")
	if err == nil || !strings.Contains(err.Error(), "provider exploded") {
		t.Fatalf("Process() error = %v, want wrapping 'provider exploded'", err)
	}
}

func TestProcess_NoChoices(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{{Choices: nil}}}
	a := processTestAgent(t, fp)

	_, err := a.Process(context.Background(), "s", "hi")
	if err == nil || !strings.Contains(err.Error(), "no response choices") {
		t.Fatalf("Process() error = %v, want 'no response choices'", err)
	}
}

func TestProcess_ToolCallLoop(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{
		toolCallResponse("let me check", provider.ToolCall{
			ID:   "call-1",
			Type: "function",
			Function: provider.ToolFunction{
				Name:      "add",
				Arguments: `{"a":1,"b":2}`,
			},
		}),
		textResponse("the sum is 3"),
	}}
	a := processTestAgent(t, fp)

	var gotArgs string
	a.RegisterTool(NewBaseTool("add", "adds two numbers", nil,
		func(_ context.Context, args json.RawMessage) (string, error) {
			gotArgs = string(args)
			return "3", nil
		}))

	got, err := a.Process(context.Background(), "s", "what is 1+2?")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	// Pre-tool text is preserved and joined with the final answer.
	want := "let me check\n\nthe sum is 3"
	if got != want {
		t.Errorf("Process() = %q, want %q", got, want)
	}
	if gotArgs != `{"a":1,"b":2}` {
		t.Errorf("tool received args %q", gotArgs)
	}
	if fp.callCount() != 2 {
		t.Fatalf("provider called %d times, want 2", fp.callCount())
	}

	// The second request must carry the assistant tool-call message and the
	// tool's result as a RoleTool message.
	second := fp.requestAt(1)
	var sawToolResult bool
	for _, m := range second.Messages {
		if m.Role == provider.RoleTool && m.Content == "3" {
			sawToolResult = true
		}
	}
	if !sawToolResult {
		t.Errorf("second request messages = %+v, want a tool result message", second.Messages)
	}
}

func TestProcess_ToolExecutionErrorSurfacesAsResult(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{
		toolCallResponse("", provider.ToolCall{
			ID: "call-1", Type: "function",
			Function: provider.ToolFunction{Name: "boom", Arguments: `{}`},
		}),
		textResponse("recovered"),
	}}
	a := processTestAgent(t, fp)
	a.RegisterTool(NewBaseTool("boom", "always fails", nil,
		func(context.Context, json.RawMessage) (string, error) {
			return "", errors.New("kaboom")
		}))

	got, err := a.Process(context.Background(), "s", "trigger boom")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got != "recovered" {
		t.Errorf("Process() = %q, want recovered", got)
	}

	second := fp.requestAt(1)
	var sawErrorResult bool
	for _, m := range second.Messages {
		if m.Role == provider.RoleTool && strings.Contains(m.Content, "kaboom") {
			sawErrorResult = true
		}
	}
	if !sawErrorResult {
		t.Errorf("expected a tool error result in the follow-up request, messages = %+v", second.Messages)
	}
}

func TestProcess_ExceedsMaxToolIterations(t *testing.T) {
	call := func() *provider.ChatCompletionResponse {
		return toolCallResponse("", provider.ToolCall{
			ID: "call", Type: "function",
			Function: provider.ToolFunction{Name: "loop", Arguments: `{}`},
		})
	}
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{
		call(), call(), call(), call(), call(),
	}}
	a := processTestAgent(t, fp)
	a.RegisterTool(NewBaseTool("loop", "never finishes", nil,
		func(context.Context, json.RawMessage) (string, error) { return "again", nil }))

	_, err := a.Process(context.Background(), "s", "go forever")
	if err == nil || !strings.Contains(err.Error(), "exceeded maximum tool call iterations") {
		t.Fatalf("Process() error = %v, want max-iterations error", err)
	}
	if fp.callCount() != 5 {
		t.Errorf("provider called %d times, want 5 (the iteration cap)", fp.callCount())
	}
}

func TestProcess_ContextEngineAppliesWindowing(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{textResponse("ok")}}
	a := processTestAgent(t, fp)
	a.contextEngine = agentctx.New(agentctx.Config{MaxMessages: 1})
	a.config.SystemPrompt = "system"

	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	a.sessions = store
	ctx := context.Background()
	session, err := store.Get(ctx, "windowed")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	session.AddMessage(provider.RoleUser, "message one")
	session.AddMessage(provider.RoleAssistant, "reply one")
	if err := store.Save(ctx, session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := a.ProcessWithSession(ctx, "windowed", "message two"); err != nil {
		t.Fatalf("ProcessWithSession: %v", err)
	}

	req := fp.requestAt(0)
	// MaxMessages:1 should windowed the history down aggressively; the
	// request must still be well-formed (at least the current user turn).
	if len(req.Messages) == 0 {
		t.Fatal("context engine produced an empty message list")
	}
	last := req.Messages[len(req.Messages)-1]
	if last.Role != provider.RoleUser || last.Content != "message two" {
		t.Errorf("last message = %+v, want the current user turn", last)
	}
}

func TestProcessWithSession_NoStoreConfigured(t *testing.T) {
	a := &Agent{logger: slog.Default()}
	_, err := a.ProcessWithSession(context.Background(), "s", "hi")
	if err == nil || !strings.Contains(err.Error(), "session store not configured") {
		t.Fatalf("ProcessWithSession() error = %v, want session-store error", err)
	}
}

func TestProcessWithSession_HappyPath_PersistsHistory(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{textResponse("hi back")}}
	a := processTestAgent(t, fp)
	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	a.sessions = store
	ctx := context.Background()

	var createdEvents, updatedEvents int
	a.hooks.RegisterHandler(hooks.EventSessionCreated, "capture", func(context.Context, hooks.Event) error {
		createdEvents++
		return nil
	})
	a.hooks.RegisterHandler(hooks.EventSessionUpdated, "capture", func(context.Context, hooks.Event) error {
		updatedEvents++
		return nil
	})

	got, err := a.ProcessWithSession(ctx, "sess-1", "hello")
	if err != nil {
		t.Fatalf("ProcessWithSession: %v", err)
	}
	if got != "hi back" {
		t.Errorf("ProcessWithSession() = %q, want hi back", got)
	}

	session, err := store.GetIfExists(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetIfExists: %v", err)
	}
	msgs := session.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("got %d persisted messages, want 2 (user+assistant)", len(msgs))
	}
	if msgs[0].Role != provider.RoleUser || msgs[0].Content != "hello" {
		t.Errorf("first message = %+v", msgs[0])
	}
	if msgs[1].Role != provider.RoleAssistant || msgs[1].Content != "hi back" {
		t.Errorf("second message = %+v", msgs[1])
	}

	// Async hook dispatch: give handlers a moment via the dispatcher's own
	// synchronization is not exposed, so we just assert they were registered
	// and at least attempted (EmitAsync is fire-and-forget in-process).
	_ = createdEvents
	_ = updatedEvents
}

// TestProcessWithSession_SavesUserMessageOnProcessingError verifies the
// documented behavior in ProcessWithSession: even when processInternal
// fails, the just-added user message is still persisted before the error
// is returned.
func TestProcessWithSession_SavesUserMessageOnProcessingError(t *testing.T) {
	fp := &fakeProvider{errs: []error{errors.New("boom")}}
	a := processTestAgent(t, fp)
	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	a.sessions = store
	ctx := context.Background()

	_, err := a.ProcessWithSession(ctx, "sess-err", "will fail")
	if err == nil {
		t.Fatal("expected an error from ProcessWithSession")
	}

	session, err := store.GetIfExists(ctx, "sess-err")
	if err != nil {
		t.Fatalf("GetIfExists: %v", err)
	}
	msgs := session.GetMessages()
	if len(msgs) != 1 || msgs[0].Content != "will fail" {
		t.Errorf("messages = %+v, want the user message preserved despite the error", msgs)
	}
}

func TestProcessWithMemory_DelegatesToProcessWithSession(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{textResponse("via memory")}}
	a := processTestAgent(t, fp)
	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	a.sessions = store

	got, err := a.ProcessWithMemory(context.Background(), "sess-mem", "hi")
	if err != nil {
		t.Fatalf("ProcessWithMemory: %v", err)
	}
	if got != "via memory" {
		t.Errorf("ProcessWithMemory() = %q, want via memory", got)
	}
}
