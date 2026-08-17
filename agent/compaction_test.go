package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/plexusone/omnillm/provider"

	"github.com/plexusone/omniagent/sessions"
)

// summarizeMessagesForTest builds System + n numbered user messages, long
// enough to exceed small compaction thresholds in tests below.
func summarizeMessagesForTest(n int) []provider.Message {
	messages := []provider.Message{{Role: provider.RoleSystem, Content: "System"}}
	for i := 0; i < n; i++ {
		messages = append(messages, provider.Message{
			Role:    provider.RoleUser,
			Content: fmt.Sprintf("msg-%d", i),
		})
	}
	return messages
}

func TestSummarizeMessages_DefaultPrompt(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{textResponse("the summary")}}
	a := processTestAgent(t, fp)

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "hello"},
		{Role: provider.RoleAssistant, Content: "hi"},
	}
	got, err := a.summarizeMessages(context.Background(), messages)
	if err != nil {
		t.Fatalf("summarizeMessages: %v", err)
	}
	if got != "the summary" {
		t.Errorf("summarizeMessages() = %q, want %q", got, "the summary")
	}

	req := fp.requestAt(0)
	if req.Model != "test-model" {
		t.Errorf("request model = %q, want test-model", req.Model)
	}
	if len(req.Messages) != 3 { // default prompt + the 2 input messages
		t.Fatalf("request has %d messages, want 3", len(req.Messages))
	}
	if req.Messages[0].Role != provider.RoleSystem || !strings.Contains(req.Messages[0].Content, "Summarize") {
		t.Errorf("first request message = %+v, want the default compaction prompt", req.Messages[0])
	}
	if req.Messages[1].Content != messages[0].Content || req.Messages[2].Content != messages[1].Content {
		t.Error("request should forward the original messages after the prompt")
	}
	if req.MaxTokens == nil || *req.MaxTokens != compactionSummaryMaxTokens {
		t.Errorf("request MaxTokens = %v, want %d", req.MaxTokens, compactionSummaryMaxTokens)
	}
}

func TestSummarizeMessages_CustomPrompt(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{textResponse("ok")}}
	a := processTestAgent(t, fp)
	a.config.CompactionPrompt = "Custom compaction instructions."

	if _, err := a.summarizeMessages(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: "hi"},
	}); err != nil {
		t.Fatalf("summarizeMessages: %v", err)
	}

	req := fp.requestAt(0)
	if req.Messages[0].Content != "Custom compaction instructions." {
		t.Errorf("prompt = %q, want the custom prompt", req.Messages[0].Content)
	}
}

func TestSummarizeMessages_ProviderError(t *testing.T) {
	fp := &fakeProvider{errs: []error{errors.New("provider unavailable")}}
	a := processTestAgent(t, fp)

	_, err := a.summarizeMessages(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: "hi"},
	})
	if err == nil || !strings.Contains(err.Error(), "provider unavailable") {
		t.Errorf("summarizeMessages() error = %v, want it to wrap the provider error", err)
	}
}

func TestSummarizeMessages_EmptyChoices(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{{Choices: nil}}}
	a := processTestAgent(t, fp)

	_, err := a.summarizeMessages(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: "hi"},
	})
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Errorf("summarizeMessages() error = %v, want a no-choices error", err)
	}
}

// TestProcessWithSession_CompactionEndToEnd drives ProcessWithSession
// against a session already carrying enough history to exceed the
// compaction threshold, and verifies the follow-up request actually sent
// to the LLM contains the synthetic summary message instead of the raw
// older ones (RMI-OMNIAGENT-027, WithCompaction wired through
// contextEngine.Apply).
func TestProcessWithSession_CompactionEndToEnd(t *testing.T) {
	fp := &fakeProvider{responses: []*provider.ChatCompletionResponse{
		textResponse("a condensed recap"), // 1st call: summarization
		textResponse("final answer"),      // 2nd call: the real turn
	}}
	a := processTestAgent(t, fp)
	if err := WithCompaction(6)(a); err != nil {
		t.Fatalf("WithCompaction: %v", err)
	}
	store := sessions.NewStore(sessions.StoreConfig{Backend: newMemoryBackend(t)})
	a.sessions = store

	session := sessions.NewSession("sess-1")
	for i := 0; i < 8; i++ { // seed well past the threshold of 6
		session.AddMessage(provider.RoleUser, fmt.Sprintf("old-%d", i))
		session.AddMessage(provider.RoleAssistant, fmt.Sprintf("old-reply-%d", i))
	}
	if err := store.Save(context.Background(), session); err != nil {
		t.Fatalf("seed session save: %v", err)
	}

	got, err := a.ProcessWithSession(context.Background(), "sess-1", "what's new?")
	if err != nil {
		t.Fatalf("ProcessWithSession: %v", err)
	}
	if got != "final answer" {
		t.Errorf("ProcessWithSession() = %q, want %q", got, "final answer")
	}
	if fp.callCount() != 2 {
		t.Fatalf("provider call count = %d, want 2 (summarize + real turn)", fp.callCount())
	}

	realReq := fp.requestAt(1)
	var sawSummary, sawOldMessage bool
	for _, m := range realReq.Messages {
		if strings.Contains(m.Content, "a condensed recap") {
			sawSummary = true
		}
		if strings.Contains(m.Content, "old-0") {
			sawOldMessage = true
		}
	}
	if !sawSummary {
		t.Errorf("real turn's request should contain the summary, messages = %+v", realReq.Messages)
	}
	if sawOldMessage {
		t.Errorf("real turn's request should NOT contain the raw oldest messages, messages = %+v", realReq.Messages)
	}
}
