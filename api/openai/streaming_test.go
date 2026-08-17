package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// parseSSEEvents splits a raw SSE response body into its "data: ..." payloads,
// in order, stripping the "data: " prefix and trailing blank line.
func parseSSEEvents(t *testing.T, body string) []string {
	t.Helper()
	var events []string
	for _, chunk := range strings.Split(body, "\n\n") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		if !strings.HasPrefix(chunk, "data: ") {
			t.Fatalf("unexpected SSE line (missing 'data: ' prefix): %q", chunk)
		}
		events = append(events, strings.TrimPrefix(chunk, "data: "))
	}
	return events
}

func TestChatCompletion_Streaming_SSE(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.streamChunks = []*ChatCompletionChunk{
		{
			ID:      "chatcmpl-stream-1",
			Object:  "chat.completion.chunk",
			Model:   "test-model",
			Choices: []ChunkChoice{{Index: 0, Delta: ChunkDelta{Role: "assistant", Content: ""}}},
		},
		{
			ID:      "chatcmpl-stream-1",
			Object:  "chat.completion.chunk",
			Model:   "test-model",
			Choices: []ChunkChoice{{Index: 0, Delta: ChunkDelta{Content: "Hello"}}},
		},
		{
			ID:      "chatcmpl-stream-1",
			Object:  "chat.completion.chunk",
			Model:   "test-model",
			Choices: []ChunkChoice{{Index: 0, Delta: ChunkDelta{Content: " world"}, FinishReason: strPtr("stop")}},
			Usage:   &Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
		},
	}

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}

	events := parseSSEEvents(t, w.Body.String())
	if len(events) != 4 {
		t.Fatalf("got %d SSE events, want 4 (3 chunks + [DONE]): %v", len(events), events)
	}

	// First three events are JSON chunks matching what the agent emitted, in order.
	var firstChunk ChatCompletionChunk
	if err := json.Unmarshal([]byte(events[0]), &firstChunk); err != nil {
		t.Fatalf("failed to decode first chunk: %v", err)
	}
	if firstChunk.Choices[0].Delta.Role != "assistant" {
		t.Errorf("first chunk role = %q, want assistant", firstChunk.Choices[0].Delta.Role)
	}

	var secondChunk ChatCompletionChunk
	if err := json.Unmarshal([]byte(events[1]), &secondChunk); err != nil {
		t.Fatalf("failed to decode second chunk: %v", err)
	}
	if secondChunk.Choices[0].Delta.Content != "Hello" {
		t.Errorf("second chunk content = %q, want Hello", secondChunk.Choices[0].Delta.Content)
	}

	var thirdChunk ChatCompletionChunk
	if err := json.Unmarshal([]byte(events[2]), &thirdChunk); err != nil {
		t.Fatalf("failed to decode third chunk: %v", err)
	}
	if thirdChunk.Choices[0].Delta.Content != " world" {
		t.Errorf("third chunk content = %q, want ' world'", thirdChunk.Choices[0].Delta.Content)
	}
	if thirdChunk.Choices[0].FinishReason == nil || *thirdChunk.Choices[0].FinishReason != "stop" {
		t.Errorf("third chunk finish_reason = %v, want stop", thirdChunk.Choices[0].FinishReason)
	}

	// Final event is the literal [DONE] marker (not JSON).
	if events[3] != "[DONE]" {
		t.Errorf("last event = %q, want [DONE]", events[3])
	}

	// Usage from the final chunk should have been recorded.
	if got := srv.UsageStore().Count(); got != 1 {
		t.Fatalf("usage record count = %d, want 1", got)
	}
	records := srv.UsageStore().GetRecords(10)
	if len(records) != 1 {
		t.Fatalf("expected 1 usage record, got %d", len(records))
	}
	if records[0].PromptTokens != 5 || records[0].CompletionTokens != 2 || records[0].TotalTokens != 7 {
		t.Errorf("usage record = %+v, want prompt=5 completion=2 total=7", records[0])
	}
}

func TestChatCompletion_Streaming_ToolCallDelta(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.streamChunks = []*ChatCompletionChunk{
		{
			ID:     "chatcmpl-tool-1",
			Object: "chat.completion.chunk",
			Model:  "test-model",
			Choices: []ChunkChoice{{
				Index: 0,
				Delta: ChunkDelta{
					Role: "assistant",
					ToolCalls: []ToolCall{
						{ID: "call_1", Type: "function", Function: FunctionCall{Name: "get_weather", Arguments: `{"city":"SF"}`}},
					},
				},
				FinishReason: strPtr("tool_calls"),
			}},
		},
	}

	body := `{"model":"test-model","messages":[{"role":"user","content":"weather?"}],"stream":true,"tools":[{"type":"function","function":{"name":"get_weather"}}]}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	events := parseSSEEvents(t, w.Body.String())
	if len(events) != 2 {
		t.Fatalf("got %d SSE events, want 2 (1 chunk + [DONE])", len(events))
	}

	var chunk ChatCompletionChunk
	if err := json.Unmarshal([]byte(events[0]), &chunk); err != nil {
		t.Fatalf("failed to decode chunk: %v", err)
	}
	if len(chunk.Choices[0].Delta.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call in delta, got %d", len(chunk.Choices[0].Delta.ToolCalls))
	}
	tc := chunk.Choices[0].Delta.ToolCalls[0]
	if tc.Function.Name != "get_weather" || tc.Function.Arguments != `{"city":"SF"}` {
		t.Errorf("tool call = %+v, want get_weather with SF args", tc)
	}

	// Confirm the request's tools were passed through to the agent handler.
	if handler.lastChatReq == nil || len(handler.lastChatReq.Tools) != 1 {
		t.Fatalf("expected agent to receive 1 tool, got %v", handler.lastChatReq)
	}
	if handler.lastChatReq.Tools[0].Function.Name != "get_weather" {
		t.Errorf("tool name = %q, want get_weather", handler.lastChatReq.Tools[0].Function.Name)
	}
}

func TestChatCompletion_Streaming_AgentError(t *testing.T) {
	srv, handler := setupTestServer(t)
	handler.streamErr = errors.New("upstream boom")

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	// The SSE writer already sent a 200 with event-stream headers before the
	// agent's error surfaces, so the error is communicated as an SSE event,
	// not an HTTP error status.
	events := parseSSEEvents(t, w.Body.String())
	if len(events) != 1 {
		t.Fatalf("got %d SSE events, want 1 (error event, no [DONE]): %v", len(events), events)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal([]byte(events[0]), &errResp); err != nil {
		t.Fatalf("failed to decode error event: %v", err)
	}
	if errResp.Error.Message != "upstream boom" {
		t.Errorf("error message = %q, want 'upstream boom'", errResp.Error.Message)
	}
	if errResp.Error.Type != "server_error" {
		t.Errorf("error type = %q, want server_error", errResp.Error.Type)
	}

	// No usage should have been recorded since the stream failed before completion.
	if got := srv.UsageStore().Count(); got != 0 {
		t.Errorf("usage record count = %d, want 0 after stream error", got)
	}
}

func TestChatCompletion_NonStreaming(t *testing.T) {
	srv, _ := setupTestServer(t)

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp ChatCompletionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Model != "test-model" {
		t.Errorf("model = %q, want test-model", resp.Model)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].FinishReason != "stop" {
		t.Errorf("choices = %+v, want 1 choice with finish_reason stop", resp.Choices)
	}

	// Non-streaming usage is estimated (no Usage from the mock) and still recorded.
	if got := srv.UsageStore().Count(); got != 1 {
		t.Errorf("usage record count = %d, want 1", got)
	}
}

func TestChatCompletion_NonStreaming_ToolCallResponse(t *testing.T) {
	srv, handler := setupTestServer(t)
	handler.chatCompletionResp = &ChatCompletionResponse{
		ID:     "chatcmpl-tool",
		Object: "chat.completion",
		Model:  "test-model",
		Choices: []Choice{{
			Index: 0,
			Message: Message{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_1", Type: "function", Function: FunctionCall{Name: "get_weather", Arguments: `{"city":"NYC"}`}},
				},
			},
			FinishReason: "tool_calls",
		}},
	}

	body := `{"model":"test-model","messages":[{"role":"user","content":"weather in NYC"}],"tool_choice":"auto","tools":[{"type":"function","function":{"name":"get_weather","description":"gets weather"}}]}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp ChatCompletionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish_reason = %q, want tool_calls", resp.Choices[0].FinishReason)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
	tc := resp.Choices[0].Message.ToolCalls[0]
	if tc.Function.Name != "get_weather" || tc.Function.Arguments != `{"city":"NYC"}` {
		t.Errorf("tool call = %+v, want get_weather with NYC args", tc)
	}

	// tool_choice and tools should have reached the agent handler unmodified.
	if handler.lastChatReq.ToolChoice != "auto" {
		t.Errorf("tool_choice = %v, want auto", handler.lastChatReq.ToolChoice)
	}
	if len(handler.lastChatReq.Tools) != 1 || handler.lastChatReq.Tools[0].Function.Description != "gets weather" {
		t.Errorf("tools not passed through correctly: %+v", handler.lastChatReq.Tools)
	}
}

func TestChatCompletion_NonStreaming_AgentError(t *testing.T) {
	srv, handler := setupTestServer(t)
	handler.chatCompletionErr = errors.New("agent exploded")

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Message != "agent exploded" {
		t.Errorf("error message = %q, want 'agent exploded'", errResp.Error.Message)
	}
	if errResp.Error.Type != "internal_error" {
		t.Errorf("error type = %q, want internal_error", errResp.Error.Type)
	}
}

func TestChatCompletion_InvalidJSON(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString("{not valid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Type != "invalid_request_error" {
		t.Errorf("error type = %q, want invalid_request_error", errResp.Error.Type)
	}
}

// --- SSEWriter unit tests ---

// flusherlessWriter implements only http.ResponseWriter (no http.Flusher),
// to exercise NewSSEWriter's "streaming not supported" error path.
type flusherlessWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (f *flusherlessWriter) Header() http.Header         { return f.header }
func (f *flusherlessWriter) Write(b []byte) (int, error) { return f.body.Write(b) }
func (f *flusherlessWriter) WriteHeader(status int)      { f.status = status }

func TestNewSSEWriter_NotSupported(t *testing.T) {
	w := &flusherlessWriter{header: http.Header{}}
	_, err := NewSSEWriter(w)
	if err == nil {
		t.Fatal("expected error for a ResponseWriter without Flush support")
	}
}

func TestNewSSEWriter_SetsHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	sse, err := NewSSEWriter(w)
	if err != nil {
		t.Fatalf("NewSSEWriter failed: %v", err)
	}
	if sse == nil {
		t.Fatal("expected non-nil SSEWriter")
	}
	if got := w.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	if got := w.Header().Get("Connection"); got != "keep-alive" {
		t.Errorf("Connection = %q, want keep-alive", got)
	}
	if got := w.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering = %q, want no", got)
	}
}

func TestSSEWriter_WriteEventAndDone(t *testing.T) {
	w := httptest.NewRecorder()
	sse, err := NewSSEWriter(w)
	if err != nil {
		t.Fatalf("NewSSEWriter failed: %v", err)
	}

	if err := sse.WriteEvent(map[string]string{"foo": "bar"}); err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
	}
	if err := sse.WriteDone(); err != nil {
		t.Fatalf("WriteDone failed: %v", err)
	}

	events := parseSSEEvents(t, w.Body.String())
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0] != `{"foo":"bar"}` {
		t.Errorf("event[0] = %q, want {\"foo\":\"bar\"}", events[0])
	}
	if events[1] != "[DONE]" {
		t.Errorf("event[1] = %q, want [DONE]", events[1])
	}
}

func TestSSEWriter_WriteEvent_MarshalError(t *testing.T) {
	w := httptest.NewRecorder()
	sse, err := NewSSEWriter(w)
	if err != nil {
		t.Fatalf("NewSSEWriter failed: %v", err)
	}

	// Channels cannot be marshaled to JSON.
	err = sse.WriteEvent(make(chan int))
	if err == nil {
		t.Fatal("expected an error marshaling an unmarshalable value")
	}
}

func TestSSEWriter_WriteError(t *testing.T) {
	w := httptest.NewRecorder()
	sse, err := NewSSEWriter(w)
	if err != nil {
		t.Fatalf("NewSSEWriter failed: %v", err)
	}

	if err := sse.WriteError(errors.New("kaboom")); err != nil {
		t.Fatalf("WriteError failed: %v", err)
	}

	events := parseSSEEvents(t, w.Body.String())
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	var errResp ErrorResponse
	if err := json.Unmarshal([]byte(events[0]), &errResp); err != nil {
		t.Fatalf("failed to decode error event: %v", err)
	}
	if errResp.Error.Message != "kaboom" {
		t.Errorf("message = %q, want kaboom", errResp.Error.Message)
	}
	if errResp.Error.Type != "server_error" {
		t.Errorf("type = %q, want server_error", errResp.Error.Type)
	}
}

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"abcd", 1},
		{"abcdefgh", 2},
		{strings.Repeat("a", 100), 25},
	}
	for _, c := range cases {
		if got := estimateTokens(c.text); got != c.want {
			t.Errorf("estimateTokens(%q) = %d, want %d", c.text, got, c.want)
		}
	}
}

func TestWriteJSONError(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSONError(w, http.StatusTeapot, "custom_error", "something went wrong")

	if w.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTeapot)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Type != "custom_error" || errResp.Error.Message != "something went wrong" {
		t.Errorf("error = %+v, want type=custom_error message='something went wrong'", errResp.Error)
	}
}

func strPtr(s string) *string { return &s }

// --- Auth enforcement on chat/completions (regression coverage for the
// auth-bypass fix: POST /chat/completions used to skip BearerAuth
// entirely because StreamingHandler intercepts POST before ogen's own
// security-checked routing ever runs). ---

func TestChatCompletion_RequiresAPIKey_NonStreaming(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithAPIKeys("secret-key"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("expected request without an API key to be rejected, got 200: %s", w.Body.String())
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestChatCompletion_RequiresAPIKey_Streaming(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithAPIKeys("secret-key"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("expected streaming request without an API key to be rejected, got 200: %s", w.Body.String())
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestChatCompletion_WrongAPIKey(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithAPIKeys("secret-key"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("expected request with a wrong API key to be rejected, got 200: %s", w.Body.String())
	}
}

func TestChatCompletion_CorrectAPIKey(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithAPIKeys("secret-key"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-key")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestChatCompletion_NoAPIKeysConfigured_Allowed(t *testing.T) {
	// No WithAPIKeys(...): server should remain open, matching
	// Config.APIKeys's documented "empty means no auth required" contract.
	srv, _ := setupTestServer(t)

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}
