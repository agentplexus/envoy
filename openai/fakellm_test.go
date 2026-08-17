package openai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/plexusone/omniagent/agent"
)

// fakeLLM is a minimal OpenAI-compatible chat-completions endpoint used to
// drive agent.Process/ProcessWithSession without any real network calls.
// It records every request's messages so tests can assert on what the
// agent layer actually sent (session history growth, extracted user
// content, etc.). By default it echoes the last user message back as the
// assistant reply; set respond to script a different answer.
type fakeLLM struct {
	*httptest.Server

	mu       sync.Mutex
	requests [][]map[string]any

	// respond, if set, computes the assistant reply from the decoded
	// request messages. Defaults to echoing the last user message.
	respond func(messages []map[string]any) string

	// status, if non-zero, makes the handler return that HTTP status with
	// an OpenAI-style error body instead of a normal completion.
	status int
}

func newFakeLLM(t *testing.T) *fakeLLM {
	t.Helper()
	f := &fakeLLM{}
	f.Server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.Server.Close)
	return f
}

func (f *fakeLLM) handle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	f.requests = append(f.requests, body.Messages)
	status := f.status
	respondFn := f.respond
	f.mu.Unlock()

	if status != 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "fake upstream failure", "type": "server_error"},
		})
		return
	}

	content := f.lastUserContent(body.Messages)
	if respondFn != nil {
		content = respondFn(body.Messages)
	}

	resp := map[string]any{
		"id":      "chatcmpl-fake",
		"object":  "chat.completion",
		"created": 1,
		"model":   "fake-model",
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": content},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (f *fakeLLM) lastUserContent(messages []map[string]any) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i]["role"] == "user" {
			if s, ok := messages[i]["content"].(string); ok {
				return s
			}
		}
	}
	return ""
}

// callCount returns the number of chat-completion requests received so far.
func (f *fakeLLM) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

// lastRequestMessages returns the messages from the most recent request.
func (f *fakeLLM) lastRequestMessages() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) == 0 {
		return nil
	}
	return f.requests[len(f.requests)-1]
}

// newFakeAgent creates a real *agent.Agent wired to the fake LLM server so
// agent.Process/ProcessWithSession exercise the full request path without
// any real network call.
func newFakeAgent(t *testing.T, llm *fakeLLM, opts ...agent.Option) *agent.Agent {
	t.Helper()
	ag, err := agent.New(agent.Config{
		Provider: "openai",
		Model:    "fake-model",
		APIKey:   "test-key",
		BaseURL:  llm.URL,
	}, opts...)
	if err != nil {
		t.Fatalf("agent.New: %v", err)
	}
	return ag
}
