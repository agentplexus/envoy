package openai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/agent/registry"

	"github.com/plexusone/omnistorage-core/kvs/backend/memory"

	openaiapi "github.com/plexusone/omniagent/api/openai"
)

// setupRoutingRegistry builds a registry of two agents ("default" and
// "coder"), each wired to the given fake LLM server so ChatCompletion can be
// exercised end-to-end without any real network call. The factory attaches a
// fresh in-memory session store to every agent so session-mode tests can
// exercise ProcessWithSession too.
func setupRoutingRegistry(t *testing.T, llm *fakeLLM) *registry.Registry {
	t.Helper()

	factory := func(cfg *registry.AgentConfig) (*agent.Agent, error) {
		backend := memory.New()
		t.Cleanup(func() { _ = backend.Close() })
		return agent.New(agent.Config{
			Provider:     "openai",
			Model:        cfg.Model,
			APIKey:       "test-key",
			BaseURL:      llm.URL,
			SystemPrompt: cfg.SystemPrompt,
		}, agent.WithSessionsFromStorage(backend))
	}

	reg := registry.New(registry.RegistryConfig{Factory: factory})
	ctx := context.Background()

	if err := reg.Create(ctx, &registry.AgentConfig{
		ID:        "default",
		Name:      "Default Agent",
		Provider:  "openai",
		Model:     "fake-model",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("failed to create default agent: %v", err)
	}

	if err := reg.Create(ctx, &registry.AgentConfig{
		ID:           "coder",
		Name:         "Code Expert",
		Provider:     "openai",
		Model:        "fake-model",
		SystemPrompt: "You are an expert programmer.",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}); err != nil {
		t.Fatalf("failed to create coder agent: %v", err)
	}

	return reg
}

func TestMultiAgentAdapter_ChatCompletion_RoutesByExactID(t *testing.T) {
	llm := newFakeLLM(t)
	reg := setupRoutingRegistry(t, llm)
	defer reg.Close()

	a := NewMultiAgentAdapter(reg)
	req := &openaiapi.ChatCompletionRequest{
		Model:    "coder",
		Messages: []openaiapi.Message{{Role: "user", Content: "route to coder"}},
	}

	resp, err := a.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}
	if resp.Choices[0].Message.Content != "route to coder" {
		t.Errorf("content = %q, want %q (proves the coder agent's fake LLM call happened)", resp.Choices[0].Message.Content, "route to coder")
	}
	if resp.Model != "coder" {
		t.Errorf("Model = %s, want coder", resp.Model)
	}
}

func TestMultiAgentAdapter_ChatCompletion_RoutesByNameCaseInsensitive(t *testing.T) {
	llm := newFakeLLM(t)
	reg := setupRoutingRegistry(t, llm)
	defer reg.Close()

	a := NewMultiAgentAdapter(reg)
	req := &openaiapi.ChatCompletionRequest{
		Model:    "code expert", // lowercase, matches "Code Expert" by name
		Messages: []openaiapi.Message{{Role: "user", Content: "hello by name"}},
	}

	resp, err := a.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}
	if resp.Choices[0].Message.Content != "hello by name" {
		t.Errorf("content = %q, want %q", resp.Choices[0].Message.Content, "hello by name")
	}
}

func TestMultiAgentAdapter_ChatCompletion_DefaultFallback(t *testing.T) {
	llm := newFakeLLM(t)
	reg := setupRoutingRegistry(t, llm)
	defer reg.Close()

	a := NewMultiAgentAdapter(reg)

	for _, model := range []string{"", "omniagent", "OmniAgent"} {
		t.Run("model="+model, func(t *testing.T) {
			req := &openaiapi.ChatCompletionRequest{
				Model:    model,
				Messages: []openaiapi.Message{{Role: "user", Content: "fallback test"}},
			}
			resp, err := a.ChatCompletion(context.Background(), req)
			if err != nil {
				t.Fatalf("ChatCompletion failed: %v", err)
			}
			if resp.Choices[0].Message.Content != "fallback test" {
				t.Errorf("content = %q, want %q", resp.Choices[0].Message.Content, "fallback test")
			}
		})
	}
}

func TestMultiAgentAdapter_ChatCompletion_UnknownModel(t *testing.T) {
	llm := newFakeLLM(t)
	reg := setupRoutingRegistry(t, llm)
	defer reg.Close()

	a := NewMultiAgentAdapter(reg)
	req := &openaiapi.ChatCompletionRequest{
		Model:    "does-not-exist",
		Messages: []openaiapi.Message{{Role: "user", Content: "hi"}},
	}

	_, err := a.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unknown model/agent")
	}
	if !strings.Contains(err.Error(), "unknown model/agent") {
		t.Errorf("error = %v, want mention of 'unknown model/agent'", err)
	}
	if llm.callCount() != 0 {
		t.Errorf("LLM should never have been called for an unrouteable request, got %d calls", llm.callCount())
	}
}

func TestMultiAgentAdapter_ChatCompletion_NoUserMessage(t *testing.T) {
	llm := newFakeLLM(t)
	reg := setupRoutingRegistry(t, llm)
	defer reg.Close()

	a := NewMultiAgentAdapter(reg)
	req := &openaiapi.ChatCompletionRequest{
		Model:    "default",
		Messages: []openaiapi.Message{{Role: "system", Content: "no user here"}},
	}

	_, err := a.ChatCompletion(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "no user message found") {
		t.Errorf("error = %v, want 'no user message found'", err)
	}
}

func TestMultiAgentAdapter_ChatCompletion_SessionModeAccumulatesHistory(t *testing.T) {
	llm := newFakeLLM(t)
	reg := setupRoutingRegistry(t, llm)
	defer reg.Close()

	a := NewMultiAgentAdapter(reg, WithMultiSession(true))
	ctx := context.Background()

	req1 := &openaiapi.ChatCompletionRequest{
		Model:    "default",
		User:     "shared-session",
		Messages: []openaiapi.Message{{Role: "user", Content: "turn one"}},
	}
	if _, err := a.ChatCompletion(ctx, req1); err != nil {
		t.Fatalf("first ChatCompletion failed: %v", err)
	}
	firstLen := len(llm.lastRequestMessages())

	req2 := &openaiapi.ChatCompletionRequest{
		Model:    "default",
		User:     "shared-session",
		Messages: []openaiapi.Message{{Role: "user", Content: "turn two"}},
	}
	if _, err := a.ChatCompletion(ctx, req2); err != nil {
		t.Fatalf("second ChatCompletion failed: %v", err)
	}
	secondLen := len(llm.lastRequestMessages())

	if secondLen <= firstLen {
		t.Errorf("second request had %d messages (want > first request's %d) when useSession=true", secondLen, firstLen)
	}
}

func TestMultiAgentAdapter_ChatCompletion_StatelessByDefault(t *testing.T) {
	llm := newFakeLLM(t)
	reg := setupRoutingRegistry(t, llm)
	defer reg.Close()

	// useSession defaults to false.
	a := NewMultiAgentAdapter(reg)
	ctx := context.Background()

	req1 := &openaiapi.ChatCompletionRequest{
		Model:    "default",
		User:     "shared-session",
		Messages: []openaiapi.Message{{Role: "user", Content: "turn one"}},
	}
	if _, err := a.ChatCompletion(ctx, req1); err != nil {
		t.Fatalf("first ChatCompletion failed: %v", err)
	}
	firstLen := len(llm.lastRequestMessages())

	req2 := &openaiapi.ChatCompletionRequest{
		Model:    "default",
		User:     "shared-session",
		Messages: []openaiapi.Message{{Role: "user", Content: "turn two"}},
	}
	if _, err := a.ChatCompletion(ctx, req2); err != nil {
		t.Fatalf("second ChatCompletion failed: %v", err)
	}
	secondLen := len(llm.lastRequestMessages())

	if secondLen > firstLen {
		t.Errorf("second request had %d messages (> first request's %d); stateless routing should not accumulate history", secondLen, firstLen)
	}
}

func TestMultiAgentAdapter_ChatCompletionStream_RoutesAndStreams(t *testing.T) {
	llm := newFakeLLM(t)
	reg := setupRoutingRegistry(t, llm)
	defer reg.Close()

	a := NewMultiAgentAdapter(reg)
	req := &openaiapi.ChatCompletionRequest{
		Model:    "coder",
		Messages: []openaiapi.Message{{Role: "user", Content: "streamed content"}},
	}

	var chunks []*openaiapi.ChatCompletionChunk
	err := a.ChatCompletionStream(context.Background(), req, func(c *openaiapi.ChatCompletionChunk) error {
		chunks = append(chunks, c)
		return nil
	})
	if err != nil {
		t.Fatalf("ChatCompletionStream failed: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("got %d chunks, want at least 2", len(chunks))
	}
	if chunks[0].Choices[0].Delta.Role != "assistant" {
		t.Errorf("first chunk role = %s, want assistant", chunks[0].Choices[0].Delta.Role)
	}
	last := chunks[len(chunks)-1]
	if last.Choices[0].FinishReason == nil || *last.Choices[0].FinishReason != "stop" {
		t.Error("last chunk should carry finish_reason=stop")
	}

	var content strings.Builder
	for _, c := range chunks[1 : len(chunks)-1] {
		content.WriteString(c.Choices[0].Delta.Content)
	}
	if content.String() != "streamed content" {
		t.Errorf("reassembled content = %q, want %q", content.String(), "streamed content")
	}
}

func TestMultiAgentAdapter_ChatCompletionStream_UnknownModel(t *testing.T) {
	llm := newFakeLLM(t)
	reg := setupRoutingRegistry(t, llm)
	defer reg.Close()

	a := NewMultiAgentAdapter(reg)
	req := &openaiapi.ChatCompletionRequest{
		Model:    "does-not-exist",
		Messages: []openaiapi.Message{{Role: "user", Content: "hi"}},
	}

	err := a.ChatCompletionStream(context.Background(), req, func(c *openaiapi.ChatCompletionChunk) error {
		t.Error("onDelta should never be called for an unrouteable model")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "unknown model/agent") {
		t.Errorf("error = %v, want 'unknown model/agent'", err)
	}
}

func TestMultiAgentAdapter_ListTools_UsesDefaultAgent(t *testing.T) {
	llm := newFakeLLM(t)
	reg := setupRoutingRegistry(t, llm)
	defer reg.Close()

	defaultAg, err := reg.Get("default")
	if err != nil {
		t.Fatalf("Get(default) failed: %v", err)
	}
	defaultAg.RegisterTool(agent.NewBaseTool("web_search", "search", nil,
		func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil }))

	a := NewMultiAgentAdapter(reg)
	tools, err := a.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools))
	}
	if tools[0].Name != "web_search" {
		t.Errorf("tool name = %s, want web_search", tools[0].Name)
	}
	if tools[0].Category != "search" {
		t.Errorf("tool category = %s, want search", tools[0].Category)
	}
}

func TestMultiAgentAdapter_ListTools_NoAgentsAvailable(t *testing.T) {
	reg := registry.New(registry.RegistryConfig{})
	defer reg.Close()

	a := NewMultiAgentAdapter(reg)
	_, err := a.ListTools(context.Background())
	if err == nil {
		t.Error("expected error when the registry has no agents")
	}
}

func TestMultiAgentAdapter_GetCronScheduler(t *testing.T) {
	llm := newFakeLLM(t)

	backend := memory.New()
	t.Cleanup(func() { _ = backend.Close() })

	factory := func(cfg *registry.AgentConfig) (*agent.Agent, error) {
		ag, err := agent.New(agent.Config{
			Provider: "openai",
			Model:    cfg.Model,
			APIKey:   "test-key",
			BaseURL:  llm.URL,
		}, agent.WithStorage(backend), agent.WithCronScheduler())
		if err != nil {
			return nil, err
		}
		if err := ag.InitCompiledSkills(context.Background()); err != nil {
			return nil, err
		}
		return ag, nil
	}

	reg := registry.New(registry.RegistryConfig{Factory: factory})
	defer reg.Close()

	if err := reg.Create(context.Background(), &registry.AgentConfig{
		ID:       "default",
		Name:     "Default Agent",
		Provider: "openai",
		Model:    "fake-model",
	}); err != nil {
		t.Fatalf("failed to create default agent: %v", err)
	}

	a := NewMultiAgentAdapter(reg)
	sched := a.getCronScheduler()
	if sched == nil {
		t.Fatal("expected a non-nil scheduler for the default agent's cron skill")
	}

	jobs, err := a.ListCronJobs(context.Background())
	if err != nil {
		t.Fatalf("ListCronJobs via multi-agent adapter failed: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("got %d jobs, want 0", len(jobs))
	}
}

func TestMultiAgentAdapter_GetCronScheduler_NoAgents(t *testing.T) {
	reg := registry.New(registry.RegistryConfig{})
	defer reg.Close()

	a := NewMultiAgentAdapter(reg)
	if sched := a.getCronScheduler(); sched != nil {
		t.Error("expected nil scheduler when the registry has no agents")
	}
}
