package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/plexusone/omnillm"
)

// maxTranslateBytes bounds the request body accepted by /api/translate — the
// composer text being translated is a single chat message, never a document.
const maxTranslateBytes = 8 << 10

// maxTranslateTokens bounds the completion so a translation request can never
// balloon into an unbounded generation.
const maxTranslateTokens = 1024

// chatCompleter is the narrow seam TranslateHTTP depends on — satisfied by
// *omnillm.ChatClient (its CreateChatCompletion method matches exactly) and
// by a fake in tests, so a unit test never needs a real API key.
type chatCompleter interface {
	CreateChatCompletion(ctx context.Context, req *omnillm.ChatCompletionRequest) (*omnillm.ChatCompletionResponse, error)
}

// TranslateHTTP serves POST /api/translate: a one-shot, non-persisting LLM
// completion that translates composer text before it's sent. Unlike the chat
// surfaces (team/chats.Service, gateway/personal_chat_http.go), this never
// touches a chat ID or writes a Message row — it calls the LLM directly via
// omnillm.ChatClient, bypassing agent.Agent's tool loop and session/memory
// entirely (the same pattern voice/gateway.go uses for its own one-shot
// completions). It is mode-agnostic: mounted behind whichever RequireAuth the
// deployment already uses (team or personal), so it needs no actor/principal
// type of its own — authentication is enforced by the wrapping middleware.
type TranslateHTTP struct {
	llm   chatCompleter
	model string
	mux   *http.ServeMux
}

// NewTranslateHTTP builds the translate handler. llm is a client constructed
// from the deployment's single global cfg.Agent provider/API key/base URL
// (never per-virtual-agent — see config/capabilities.go's Translate flag,
// which is only true when cfg.Agent.APIKey is set).
func NewTranslateHTTP(llm chatCompleter, model string) *TranslateHTTP {
	h := &TranslateHTTP{llm: llm, model: model}
	h.routes()
	return h
}

// Handler returns the routed handler. Mount it at "/api/translate", wrapped
// in the deployment's RequireAuth (TeamHTTP or PersonalAuthHTTP).
func (h *TranslateHTTP) Handler() http.Handler { return h.mux }

func (h *TranslateHTTP) routes() {
	h.mux = http.NewServeMux()
	h.mux.HandleFunc("POST /api/translate", h.handleTranslate)
}

type translateRequest struct {
	Text       string `json:"text"`
	TargetLang string `json:"targetLang"`
}

type translateResponse struct {
	Translation string `json:"translation"`
}

func (h *TranslateHTTP) handleTranslate(w http.ResponseWriter, r *http.Request) {
	if !hasCSRF(r) {
		writeError(w, http.StatusForbidden, "missing CSRF header")
		return
	}

	var req translateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxTranslateBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	text := strings.TrimSpace(req.Text)
	lang := strings.TrimSpace(req.TargetLang)
	if text == "" || lang == "" {
		writeError(w, http.StatusBadRequest, "text and targetLang are required")
		return
	}

	maxTokens := maxTranslateTokens
	resp, err := h.llm.CreateChatCompletion(r.Context(), &omnillm.ChatCompletionRequest{
		Model: h.model,
		Messages: []omnillm.Message{
			{
				Role: omnillm.RoleSystem,
				Content: "You are a translator. Translate the user's text to " + lang +
					". Return ONLY the translation, nothing else.",
			},
			{Role: omnillm.RoleUser, Content: text},
		},
		MaxTokens: &maxTokens,
	})
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		writeError(w, http.StatusBadGateway, "translation failed")
		return
	}

	writeJSON(w, http.StatusOK, translateResponse{Translation: resp.Choices[0].Message.Content})
}
