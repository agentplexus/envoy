package openai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/cron"
	"github.com/plexusone/omniagent/skills/compiled"

	"github.com/plexusone/omnistorage-core/kvs/backend/memory"

	openaiapi "github.com/plexusone/omniagent/api/openai"
)

func TestNewAgentAdapter_Options(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)

	t.Run("defaults", func(t *testing.T) {
		a := NewAgentAdapter(ag)
		if a.modelID != "omniagent" {
			t.Errorf("modelID = %s, want omniagent", a.modelID)
		}
		if a.modelOwner != "plexusone" {
			t.Errorf("modelOwner = %s, want plexusone", a.modelOwner)
		}
		if a.useSession {
			t.Error("useSession should default to false")
		}
	})

	t.Run("with options", func(t *testing.T) {
		a := NewAgentAdapter(ag,
			WithModelID("custom-model"),
			WithModelOwner("custom-owner"),
			WithSession(true),
		)
		if a.modelID != "custom-model" {
			t.Errorf("modelID = %s, want custom-model", a.modelID)
		}
		if a.modelOwner != "custom-owner" {
			t.Errorf("modelOwner = %s, want custom-owner", a.modelOwner)
		}
		if !a.useSession {
			t.Error("useSession should be true")
		}
	})
}

func TestAgentAdapter_ChatCompletion_ExtractsLastUserMessage(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)
	a := NewAgentAdapter(ag, WithModelID("test-model"), WithModelOwner("test-owner"))

	req := &openaiapi.ChatCompletionRequest{
		Model: "test-model",
		Messages: []openaiapi.Message{
			{Role: "system", Content: "you are helpful"},
			{Role: "user", Content: "first question"},
			{Role: "assistant", Content: "first answer"},
			{Role: "user", Content: "second question"},
		},
	}

	resp, err := a.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("got %d choices, want 1", len(resp.Choices))
	}
	// The fake LLM echoes the last user message it received; the adapter
	// must have extracted "second question", not "first question".
	if resp.Choices[0].Message.Content != "second question" {
		t.Errorf("content = %q, want %q", resp.Choices[0].Message.Content, "second question")
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("role = %s, want assistant", resp.Choices[0].Message.Role)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %s, want stop", resp.Choices[0].FinishReason)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("object = %s, want chat.completion", resp.Object)
	}
	if resp.Model != "test-model" {
		t.Errorf("model = %s, want test-model", resp.Model)
	}
	if !strings.HasPrefix(resp.ID, "chatcmpl-") {
		t.Errorf("ID = %s, want chatcmpl- prefix", resp.ID)
	}
	if resp.Usage == nil {
		t.Fatal("usage should not be nil")
	}
	wantPrompt := estimateTokens("second question")
	wantCompletion := estimateTokens("second question") // echoed content is identical
	if resp.Usage.PromptTokens != wantPrompt {
		t.Errorf("prompt tokens = %d, want %d", resp.Usage.PromptTokens, wantPrompt)
	}
	if resp.Usage.CompletionTokens != wantCompletion {
		t.Errorf("completion tokens = %d, want %d", resp.Usage.CompletionTokens, wantCompletion)
	}
	if resp.Usage.TotalTokens != wantPrompt+wantCompletion {
		t.Errorf("total tokens = %d, want %d", resp.Usage.TotalTokens, wantPrompt+wantCompletion)
	}
}

func TestAgentAdapter_ChatCompletion_NoUserMessage(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)
	a := NewAgentAdapter(ag)

	req := &openaiapi.ChatCompletionRequest{
		Model: "test-model",
		Messages: []openaiapi.Message{
			{Role: "system", Content: "you are helpful"},
			{Role: "assistant", Content: "hi"},
		},
	}

	_, err := a.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing user message")
	}
	if !strings.Contains(err.Error(), "no user message found") {
		t.Errorf("error = %v, want mention of 'no user message found'", err)
	}
}

func TestAgentAdapter_ChatCompletion_UseSessionRequiresStore(t *testing.T) {
	llm := newFakeLLM(t)
	// No session store configured on the agent.
	ag := newFakeAgent(t, llm)
	a := NewAgentAdapter(ag, WithSession(true))

	req := &openaiapi.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openaiapi.Message{{Role: "user", Content: "hello"}},
	}

	_, err := a.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when useSession=true but no session store configured")
	}
	if !strings.Contains(err.Error(), "session store not configured") {
		t.Errorf("error = %v, want mention of missing session store", err)
	}
}

func TestAgentAdapter_ChatCompletion_StatelessProcessDoesNotPersistHistory(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)
	// useSession left at its default (false): every turn should go through
	// the stateless a.agent.Process path, so the second call's outbound
	// request must NOT carry the first turn's history.
	a := NewAgentAdapter(ag)

	ctx := context.Background()
	req1 := &openaiapi.ChatCompletionRequest{
		Model:    "test-model",
		User:     "session-a",
		Messages: []openaiapi.Message{{Role: "user", Content: "turn one"}},
	}
	if _, err := a.ChatCompletion(ctx, req1); err != nil {
		t.Fatalf("first ChatCompletion failed: %v", err)
	}
	firstLen := len(llm.lastRequestMessages())

	req2 := &openaiapi.ChatCompletionRequest{
		Model:    "test-model",
		User:     "session-a",
		Messages: []openaiapi.Message{{Role: "user", Content: "turn two"}},
	}
	if _, err := a.ChatCompletion(ctx, req2); err != nil {
		t.Fatalf("second ChatCompletion failed: %v", err)
	}
	secondLen := len(llm.lastRequestMessages())

	if secondLen > firstLen {
		t.Errorf("second request had %d messages (> first request's %d); stateless Process should not accumulate session history", secondLen, firstLen)
	}
}

func TestAgentAdapter_ChatCompletion_SessionModeAccumulatesHistory(t *testing.T) {
	llm := newFakeLLM(t)
	backend := memory.New()
	t.Cleanup(func() { _ = backend.Close() })
	ag := newFakeAgent(t, llm, agent.WithSessionsFromStorage(backend))
	a := NewAgentAdapter(ag, WithSession(true))

	ctx := context.Background()
	req1 := &openaiapi.ChatCompletionRequest{
		Model:    "test-model",
		User:     "session-a",
		Messages: []openaiapi.Message{{Role: "user", Content: "turn one"}},
	}
	if _, err := a.ChatCompletion(ctx, req1); err != nil {
		t.Fatalf("first ChatCompletion failed: %v", err)
	}
	firstLen := len(llm.lastRequestMessages())

	req2 := &openaiapi.ChatCompletionRequest{
		Model:    "test-model",
		User:     "session-a",
		Messages: []openaiapi.Message{{Role: "user", Content: "turn two"}},
	}
	if _, err := a.ChatCompletion(ctx, req2); err != nil {
		t.Fatalf("second ChatCompletion failed: %v", err)
	}
	secondLen := len(llm.lastRequestMessages())

	if secondLen <= firstLen {
		t.Errorf("second request had %d messages (want > first request's %d); session mode should accumulate history", secondLen, firstLen)
	}
}

func TestAgentAdapter_ChatCompletion_LLMError(t *testing.T) {
	llm := newFakeLLM(t)
	llm.status = 500
	ag := newFakeAgent(t, llm)
	a := NewAgentAdapter(ag)

	req := &openaiapi.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openaiapi.Message{{Role: "user", Content: "hello"}},
	}

	_, err := a.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when upstream LLM call fails")
	}
}

func TestAgentAdapter_ChatCompletionStream(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)
	a := NewAgentAdapter(ag)

	req := &openaiapi.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openaiapi.Message{{Role: "user", Content: "stream this response please"}},
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
		t.Fatalf("got %d chunks, want at least 2 (role + final)", len(chunks))
	}

	first := chunks[0]
	if first.Choices[0].Delta.Role != "assistant" {
		t.Errorf("first chunk role = %s, want assistant", first.Choices[0].Delta.Role)
	}
	if first.Choices[0].FinishReason != nil {
		t.Error("first chunk should not carry a finish reason")
	}

	last := chunks[len(chunks)-1]
	if last.Choices[0].FinishReason == nil || *last.Choices[0].FinishReason != "stop" {
		t.Error("last chunk should carry finish_reason=stop")
	}

	var content strings.Builder
	for _, c := range chunks[1 : len(chunks)-1] {
		content.WriteString(c.Choices[0].Delta.Content)
	}
	if content.String() != "stream this response please" {
		t.Errorf("reassembled content = %q, want %q", content.String(), "stream this response please")
	}
}

func TestAgentAdapter_ChatCompletionStream_NoUserMessage(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)
	a := NewAgentAdapter(ag)

	req := &openaiapi.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openaiapi.Message{{Role: "system", Content: "hi"}},
	}

	err := a.ChatCompletionStream(context.Background(), req, func(c *openaiapi.ChatCompletionChunk) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "no user message found") {
		t.Errorf("error = %v, want mention of 'no user message found'", err)
	}
}

func TestAgentAdapter_ChatCompletionStream_OnDeltaErrorPropagates(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)
	a := NewAgentAdapter(ag)

	req := &openaiapi.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openaiapi.Message{{Role: "user", Content: "hello"}},
	}

	boom := context.Canceled // any distinct sentinel error
	err := a.ChatCompletionStream(context.Background(), req, func(c *openaiapi.ChatCompletionChunk) error {
		return boom
	})
	if err != boom {
		t.Errorf("error = %v, want the onDelta error to propagate unchanged", err)
	}
}

func TestAgentAdapter_ListModels(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)
	a := NewAgentAdapter(ag, WithModelID("my-model"), WithModelOwner("my-owner"))

	models, err := a.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("got %d models, want 1", len(models))
	}
	if models[0].ID != "my-model" {
		t.Errorf("ID = %s, want my-model", models[0].ID)
	}
	if models[0].OwnedBy != "my-owner" {
		t.Errorf("OwnedBy = %s, want my-owner", models[0].OwnedBy)
	}
	if models[0].Object != "model" {
		t.Errorf("Object = %s, want model", models[0].Object)
	}
}

func TestAgentAdapter_GetModel(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)
	a := NewAgentAdapter(ag, WithModelID("my-model"))

	t.Run("known model", func(t *testing.T) {
		m, err := a.GetModel(context.Background(), "my-model")
		if err != nil {
			t.Fatalf("GetModel failed: %v", err)
		}
		if m.ID != "my-model" {
			t.Errorf("ID = %s, want my-model", m.ID)
		}
	})

	t.Run("unknown model", func(t *testing.T) {
		_, err := a.GetModel(context.Background(), "other-model")
		if err == nil {
			t.Error("expected error for unknown model")
		}
	})
}

func TestAgentAdapter_ListTools(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)

	ag.RegisterTool(agent.NewBaseTool("web_search", "search the web", map[string]interface{}{
		"type": "object",
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		return "", nil
	}))
	ag.RegisterTool(agent.NewBaseTool("custom_thing", "does a custom thing", nil,
		func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", nil
		}))

	a := NewAgentAdapter(ag)
	tools, err := a.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}

	byName := map[string]openaiapi.ToolInfo{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}

	if byName["web_search"].Category != "search" {
		t.Errorf("web_search category = %s, want search", byName["web_search"].Category)
	}
	if byName["custom_thing"].Category != "general" {
		t.Errorf("custom_thing category = %s, want general", byName["custom_thing"].Category)
	}
	if byName["web_search"].Description != "search the web" {
		t.Errorf("web_search description = %s", byName["web_search"].Description)
	}
}

func TestCategorizeTool(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"web_search", "search"},
		{"search", "search"},
		{"render_chart", "visualization"},
		{"chart", "visualization"},
		{"read_file", "filesystem"},
		{"write_file", "filesystem"},
		{"list_files", "filesystem"},
		{"execute", "execution"},
		{"run_command", "execution"},
		{"bash", "execution"},
		{"something_else", "general"},
		{"", "general"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := categorizeTool(tt.name); got != tt.want {
				t.Errorf("categorizeTool(%q) = %s, want %s", tt.name, got, tt.want)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"abcd", 1},
		{"abcde", 2},
		{"12345678", 2},
	}
	for _, tt := range tests {
		if got := estimateTokens(tt.text); got != tt.want {
			t.Errorf("estimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestAgentAdapter_GetCronScheduler_NilWhenNoCronSkill(t *testing.T) {
	llm := newFakeLLM(t)
	ag := newFakeAgent(t, llm)
	a := NewAgentAdapter(ag)

	if sched := a.getCronScheduler(); sched != nil {
		t.Error("expected nil scheduler when no cron skill is registered")
	}
}

func TestAgentAdapter_GetCronScheduler_ReturnsRegisteredScheduler(t *testing.T) {
	llm := newFakeLLM(t)
	backend := memory.New()
	t.Cleanup(func() { _ = backend.Close() })

	ag := newFakeAgent(t, llm,
		agent.WithStorage(backend),
		agent.WithCronScheduler(),
	)
	if err := ag.InitCompiledSkills(context.Background()); err != nil {
		t.Fatalf("InitCompiledSkills failed: %v", err)
	}
	t.Cleanup(func() { _ = ag.CloseCompiledSkills() })

	a := NewAgentAdapter(ag)
	sched := a.getCronScheduler()
	if sched == nil {
		t.Fatal("expected a non-nil scheduler once cron.Skill is registered")
	}

	// The cronHandler embedded in the adapter should route through the same
	// getter, proving the exposure end-to-end via the OpenAI-compatible
	// cron surface rather than just the raw getter.
	jobs, err := a.ListCronJobs(context.Background())
	if err != nil {
		t.Fatalf("ListCronJobs via adapter failed: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("got %d jobs, want 0 for a freshly-initialized scheduler", len(jobs))
	}
}

// compile-time check that cron.Skill still satisfies compiled.Skill, which
// the getCronScheduler tests above depend on.
var _ compiled.Skill = (*cron.Skill)(nil)
