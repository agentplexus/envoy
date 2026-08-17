package openai

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// modelsListResponse mirrors the shape of ogen.ListModelsResponse for decoding
// in tests without importing the internal ogen package.
type modelsListResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

func TestModelsEndpoint_List(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.agents["default"] = &AgentInfo{ID: "default", Name: "Default", CreatedAt: time.Now()}
	handler.agents["coder"] = &AgentInfo{ID: "coder", Name: "Coder", CreatedAt: time.Now()}

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp modelsListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Object != "list" {
		t.Errorf("object = %q, want list", resp.Object)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("got %d models, want 2", len(resp.Data))
	}
	for _, m := range resp.Data {
		if m.Object != "model" {
			t.Errorf("model.object = %q, want model", m.Object)
		}
		if m.ID != "default" && m.ID != "coder" {
			t.Errorf("unexpected model id %q", m.ID)
		}
	}
}

func TestModelsEndpoint_List_Empty(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp modelsListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Errorf("got %d models, want 0", len(resp.Data))
	}
}

func TestModelsEndpoint_List_HandlerError(t *testing.T) {
	srv, handler := setupTestServer(t)
	handler.listModelsErr = errors.New("backend unavailable")

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("expected a non-200 status when ListModels fails, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestModelsEndpoint_Retrieve(t *testing.T) {
	srv, handler := setupTestServer(t)
	handler.agents["default"] = &AgentInfo{ID: "default", Name: "Default", CreatedAt: time.Now()}

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models/default", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var m Model
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if m.ID != "default" {
		t.Errorf("id = %q, want default", m.ID)
	}
	if m.Object != "model" {
		t.Errorf("object = %q, want model", m.Object)
	}
}

func TestModelsEndpoint_Retrieve_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusNotFound, w.Body.String())
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Type != "invalid_request_error" {
		t.Errorf("error type = %q, want invalid_request_error", errResp.Error.Type)
	}
}

// --- API key authentication on the /openai/v1/models routes ---

func TestModelsEndpoint_RequiresAPIKey(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithAPIKeys("secret-key"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("expected request without an API key to be rejected, got 200: %s", w.Body.String())
	}
}

func TestModelsEndpoint_WrongAPIKey(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithAPIKeys("secret-key"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("expected request with a wrong API key to be rejected, got 200: %s", w.Body.String())
	}
}

func TestModelsEndpoint_CorrectAPIKey(t *testing.T) {
	handler := newMockAgentHandler()
	handler.agents["default"] = &AgentInfo{ID: "default", Name: "Default", CreatedAt: time.Now()}
	srv, err := New(handler, WithAPIKeys("secret-key"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

// --- Tool listing ---

func TestListTools_HandlerError(t *testing.T) {
	srv, handler := setupTestServer(t)
	handler.listToolsErr = errors.New("tool registry unavailable")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

func TestListTools_EmptyList(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Object string     `json:"object"`
		Data   []ToolInfo `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected 0 tools, got %d", len(resp.Data))
	}
}

func TestListTools_WithParameters(t *testing.T) {
	srv, handler := setupTestServer(t)
	handler.tools = []ToolInfo{
		{
			Name:        "search",
			Description: "Web search",
			Category:    "search",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Object string     `json:"object"`
		Data   []ToolInfo `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(resp.Data))
	}
	params, ok := resp.Data[0].Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map in parameters, got %+v", resp.Data[0].Parameters)
	}
	if _, ok := params["query"]; !ok {
		t.Errorf("expected 'query' property to survive round-trip, got %+v", params)
	}
}
