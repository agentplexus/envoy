package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// SSEWriter wraps http.ResponseWriter for Server-Sent Events.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSE writer.
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	return &SSEWriter{w: w, flusher: flusher}, nil
}

// WriteEvent writes an SSE event with JSON data.
func (s *SSEWriter) WriteEvent(data any) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(s.w, "data: %s\n\n", bytes)
	if err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// WriteDone writes the final [DONE] event.
func (s *SSEWriter) WriteDone() error {
	_, err := fmt.Fprint(s.w, "data: [DONE]\n\n")
	s.flusher.Flush()
	return err
}

// WriteError writes an error as an SSE event.
func (s *SSEWriter) WriteError(err error) error {
	errResp := ErrorResponse{
		Error: Error{
			Message: err.Error(),
			Type:    "server_error",
		},
	}
	return s.WriteEvent(errResp)
}

// StreamingHandler creates an HTTP handler that supports both streaming and non-streaming requests.
// This is used to bypass ogen's limitations with SSE.
type StreamingHandler struct {
	agent       AgentHandler
	ogenHandler http.Handler
	usageStore  *UsageStore
}

// NewStreamingHandler creates a new streaming handler.
func NewStreamingHandler(agent AgentHandler, ogenHandler http.Handler) *StreamingHandler {
	return &StreamingHandler{
		agent:       agent,
		ogenHandler: ogenHandler,
	}
}

// SetUsageStore sets the usage store for tracking.
func (h *StreamingHandler) SetUsageStore(store *UsageStore) {
	h.usageStore = store
}

// ServeHTTP implements http.Handler.
func (h *StreamingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only intercept POST /chat/completions
	if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
		h.ogenHandler.ServeHTTP(w, r)
		return
	}

	// Decode request to check if streaming
	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	// If not streaming, we need to recreate the body and forward to ogen
	if !req.Stream {
		h.handleNonStreaming(w, r, &req)
		return
	}

	// Handle streaming request
	h.handleStreaming(w, r, &req)
}

func (h *StreamingHandler) handleNonStreaming(w http.ResponseWriter, r *http.Request, req *ChatCompletionRequest) {
	ctx := r.Context()
	startTime := time.Now()

	resp, err := h.agent.ChatCompletion(ctx, req)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Record usage
	h.recordUsage(req, resp.Usage, startTime)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Can't write error response if encoding fails partway through
		return
	}
}

func (h *StreamingHandler) handleStreaming(w http.ResponseWriter, r *http.Request, req *ChatCompletionRequest) {
	ctx := r.Context()
	startTime := time.Now()

	sse, err := NewSSEWriter(w)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	// Track usage from chunks
	var lastUsage *Usage

	err = h.agent.ChatCompletionStream(ctx, req, func(chunk *ChatCompletionChunk) error {
		// Capture usage if present in chunk
		if chunk.Usage != nil {
			lastUsage = chunk.Usage
		}
		return sse.WriteEvent(chunk)
	})
	if err != nil {
		_ = sse.WriteError(err)
		return
	}

	// Record usage (estimate if not provided in stream)
	h.recordUsage(req, lastUsage, startTime)

	_ = sse.WriteDone()
}

// recordUsage records a usage event to the store.
func (h *StreamingHandler) recordUsage(req *ChatCompletionRequest, usage *Usage, startTime time.Time) {
	if h.usageStore == nil {
		return
	}

	latency := time.Since(startTime).Milliseconds()

	var promptTokens, completionTokens, totalTokens int
	if usage != nil {
		promptTokens = usage.PromptTokens
		completionTokens = usage.CompletionTokens
		totalTokens = usage.TotalTokens
	} else {
		// Estimate tokens if not provided
		for _, msg := range req.Messages {
			promptTokens += estimateTokens(msg.Content)
		}
		// Rough estimate for completion (will be updated if we can track response)
		completionTokens = promptTokens / 2 // Conservative estimate
		totalTokens = promptTokens + completionTokens
	}

	record := UsageRecord{
		ID:               uuid.NewString()[:8],
		Timestamp:        startTime,
		Model:            req.Model,
		SessionID:        req.User,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		Cost:             CalculateCost(req.Model, promptTokens, completionTokens),
		Latency:          latency,
	}

	h.usageStore.Record(record)
}

// estimateTokens provides a rough estimate of token count.
func estimateTokens(text string) int {
	return (len(text) + 3) / 4
}

func writeJSONError(w http.ResponseWriter, statusCode int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := ErrorResponse{
		Error: Error{
			Message: message,
			Type:    errType,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
