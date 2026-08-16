package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/plexusone/omnillm"
)

// fakeChatCompleter is a chatCompleter test double — no real API key or
// network call needed, unlike an integration test against *omnillm.ChatClient.
type fakeChatCompleter struct {
	resp *omnillm.ChatCompletionResponse
	err  error
	// lastReq captures the most recent request for assertions.
	lastReq *omnillm.ChatCompletionRequest
}

func (f *fakeChatCompleter) CreateChatCompletion(_ context.Context, req *omnillm.ChatCompletionRequest) (*omnillm.ChatCompletionResponse, error) {
	f.lastReq = req
	return f.resp, f.err
}

func doTranslate(t *testing.T, h http.Handler, body string, withCSRF bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/translate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if withCSRF {
		req.Header.Set(csrfHeader, "1")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTranslateHTTP_Success(t *testing.T) {
	fake := &fakeChatCompleter{
		resp: &omnillm.ChatCompletionResponse{
			Choices: []omnillm.ChatCompletionChoice{
				{Message: omnillm.Message{Content: "Hola"}},
			},
		},
	}
	h := NewTranslateHTTP(fake, "test-model")

	rec := doTranslate(t, h.Handler(), `{"text":"Hello","targetLang":"Spanish"}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got translateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Translation != "Hola" {
		t.Errorf("translation = %q, want %q", got.Translation, "Hola")
	}
	if fake.lastReq == nil || fake.lastReq.Model != "test-model" {
		t.Errorf("lastReq.Model = %+v, want test-model", fake.lastReq)
	}
	if len(fake.lastReq.Messages) != 2 || fake.lastReq.Messages[1].Content != "Hello" {
		t.Errorf("lastReq.Messages = %+v, want a system + user(%q) pair", fake.lastReq.Messages, "Hello")
	}
}

func TestTranslateHTTP_MissingCSRF(t *testing.T) {
	h := NewTranslateHTTP(&fakeChatCompleter{}, "test-model")
	rec := doTranslate(t, h.Handler(), `{"text":"Hello","targetLang":"Spanish"}`, false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestTranslateHTTP_EmptyFields(t *testing.T) {
	h := NewTranslateHTTP(&fakeChatCompleter{}, "test-model")
	for _, body := range []string{
		`{"text":"","targetLang":"Spanish"}`,
		`{"text":"Hello","targetLang":""}`,
		`{"text":"   ","targetLang":"Spanish"}`,
	} {
		rec := doTranslate(t, h.Handler(), body, true)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body=%s: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestTranslateHTTP_InvalidJSON(t *testing.T) {
	h := NewTranslateHTTP(&fakeChatCompleter{}, "test-model")
	rec := doTranslate(t, h.Handler(), `not json`, true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestTranslateHTTP_LLMError(t *testing.T) {
	h := NewTranslateHTTP(&fakeChatCompleter{err: errors.New("provider unavailable")}, "test-model")
	rec := doTranslate(t, h.Handler(), `{"text":"Hello","targetLang":"Spanish"}`, true)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

func TestTranslateHTTP_NoChoices(t *testing.T) {
	h := NewTranslateHTTP(&fakeChatCompleter{resp: &omnillm.ChatCompletionResponse{}}, "test-model")
	rec := doTranslate(t, h.Handler(), `{"text":"Hello","targetLang":"Spanish"}`, true)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}
