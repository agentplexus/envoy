package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockAgentHandler implements AgentHandler and AgentManager for testing.
type mockAgentHandler struct {
	agents   map[string]*AgentInfo
	tools    []ToolInfo
	cronJobs map[string]*CronJobInfo
}

func newMockAgentHandler() *mockAgentHandler {
	return &mockAgentHandler{
		agents:   make(map[string]*AgentInfo),
		tools:    []ToolInfo{},
		cronJobs: make(map[string]*CronJobInfo),
	}
}

// AgentHandler methods

func (m *mockAgentHandler) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	return &ChatCompletionResponse{
		ID:      "chatcmpl-test",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "test response"},
				FinishReason: "stop",
			},
		},
	}, nil
}

func (m *mockAgentHandler) ChatCompletionStream(ctx context.Context, req *ChatCompletionRequest, onDelta func(*ChatCompletionChunk) error) error {
	return nil
}

func (m *mockAgentHandler) ListModels(ctx context.Context) ([]Model, error) {
	models := make([]Model, 0, len(m.agents))
	for _, a := range m.agents {
		models = append(models, Model{
			ID:      a.ID,
			Object:  "model",
			Created: a.CreatedAt.Unix(),
			OwnedBy: "test",
		})
	}
	return models, nil
}

func (m *mockAgentHandler) GetModel(ctx context.Context, modelID string) (*Model, error) {
	if a, ok := m.agents[modelID]; ok {
		return &Model{
			ID:      a.ID,
			Object:  "model",
			Created: a.CreatedAt.Unix(),
			OwnedBy: "test",
		}, nil
	}
	return nil, fmt.Errorf("model not found: %s", modelID)
}

func (m *mockAgentHandler) ListTools(ctx context.Context) ([]ToolInfo, error) {
	return m.tools, nil
}

func (m *mockAgentHandler) ListCronJobs(ctx context.Context) ([]CronJobInfo, error) {
	jobs := make([]CronJobInfo, 0, len(m.cronJobs))
	for _, j := range m.cronJobs {
		jobs = append(jobs, *j)
	}
	return jobs, nil
}

func (m *mockAgentHandler) GetCronJob(ctx context.Context, id string) (*CronJobInfo, error) {
	if j, ok := m.cronJobs[id]; ok {
		return j, nil
	}
	return nil, fmt.Errorf("job not found: %s", id)
}

func (m *mockAgentHandler) CreateCronJob(ctx context.Context, req *CreateCronJobRequest) (*CronJobInfo, error) {
	job := &CronJobInfo{
		ID:          fmt.Sprintf("job-%d", len(m.cronJobs)+1),
		Name:        req.Name,
		Description: req.Description,
		Schedule:    req.Schedule,
		Action:      req.Action,
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.cronJobs[job.ID] = job
	return job, nil
}

func (m *mockAgentHandler) UpdateCronJob(ctx context.Context, id string, req *UpdateCronJobRequest) (*CronJobInfo, error) {
	if j, ok := m.cronJobs[id]; ok {
		if req.Name != nil {
			j.Name = *req.Name
		}
		if req.Description != nil {
			j.Description = *req.Description
		}
		j.UpdatedAt = time.Now()
		return j, nil
	}
	return nil, fmt.Errorf("job not found: %s", id)
}

func (m *mockAgentHandler) DeleteCronJob(ctx context.Context, id string) error {
	if _, ok := m.cronJobs[id]; ok {
		delete(m.cronJobs, id)
		return nil
	}
	return fmt.Errorf("job not found: %s", id)
}

func (m *mockAgentHandler) TriggerCronJob(ctx context.Context, id string) (*CronJobResult, error) {
	if _, ok := m.cronJobs[id]; ok {
		return &CronJobResult{
			Success:   true,
			Output:    "triggered",
			Duration:  "100ms",
			StartedAt: time.Now().Format(time.RFC3339),
		}, nil
	}
	return nil, fmt.Errorf("job not found: %s", id)
}

func (m *mockAgentHandler) EnableCronJob(ctx context.Context, id string) error {
	if j, ok := m.cronJobs[id]; ok {
		j.Status = "active"
		return nil
	}
	return fmt.Errorf("job not found: %s", id)
}

func (m *mockAgentHandler) DisableCronJob(ctx context.Context, id string) error {
	if j, ok := m.cronJobs[id]; ok {
		j.Status = "disabled"
		return nil
	}
	return fmt.Errorf("job not found: %s", id)
}

// AgentManager methods

func (m *mockAgentHandler) ListAgents(ctx context.Context) ([]AgentInfo, error) {
	agents := make([]AgentInfo, 0, len(m.agents))
	for _, a := range m.agents {
		agents = append(agents, *a)
	}
	return agents, nil
}

func (m *mockAgentHandler) GetAgent(ctx context.Context, id string) (*AgentInfo, error) {
	if a, ok := m.agents[id]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("agent not found: %s", id)
}

func (m *mockAgentHandler) CreateAgent(ctx context.Context, req *CreateAgentRequest) (*AgentInfo, error) {
	id := req.ID
	if id == "" {
		id = fmt.Sprintf("agent-%d", len(m.agents)+1)
	}
	if _, exists := m.agents[id]; exists {
		return nil, fmt.Errorf("agent already exists: %s", id)
	}
	agent := &AgentInfo{
		ID:           id,
		Name:         req.Name,
		Description:  req.Description,
		Provider:     req.Provider,
		Model:        req.Model,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		SystemPrompt: req.SystemPrompt,
		AllowedTools: req.AllowedTools,
		DeniedTools:  req.DeniedTools,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.agents[id] = agent
	return agent, nil
}

func (m *mockAgentHandler) UpdateAgent(ctx context.Context, id string, req *UpdateAgentRequest) (*AgentInfo, error) {
	if a, ok := m.agents[id]; ok {
		if req.Name != nil {
			a.Name = *req.Name
		}
		if req.Description != nil {
			a.Description = *req.Description
		}
		if req.Temperature != nil {
			a.Temperature = *req.Temperature
		}
		if req.Enabled != nil {
			a.Enabled = *req.Enabled
		}
		a.UpdatedAt = time.Now()
		return a, nil
	}
	return nil, fmt.Errorf("agent not found: %s", id)
}

func (m *mockAgentHandler) DeleteAgent(ctx context.Context, id string) error {
	if _, ok := m.agents[id]; ok {
		delete(m.agents, id)
		return nil
	}
	return fmt.Errorf("agent not found: %s", id)
}

func (m *mockAgentHandler) CloneAgent(ctx context.Context, id string, req *CloneAgentRequest) (*AgentInfo, error) {
	if a, ok := m.agents[id]; ok {
		newID := req.NewID
		if newID == "" {
			newID = fmt.Sprintf("%s-clone", id)
		}
		if _, exists := m.agents[newID]; exists {
			return nil, fmt.Errorf("agent already exists: %s", newID)
		}
		clone := &AgentInfo{
			ID:           newID,
			Name:         req.NewName,
			Description:  a.Description,
			Provider:     a.Provider,
			Model:        a.Model,
			Temperature:  a.Temperature,
			MaxTokens:    a.MaxTokens,
			SystemPrompt: a.SystemPrompt,
			AllowedTools: a.AllowedTools,
			DeniedTools:  a.DeniedTools,
			Enabled:      true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		m.agents[newID] = clone
		return clone, nil
	}
	return nil, fmt.Errorf("agent not found: %s", id)
}

// Test helpers

func setupTestServer(t *testing.T) (*Server, *mockAgentHandler) {
	t.Helper()
	handler := newMockAgentHandler()
	srv, err := New(handler)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	return srv, handler
}

// Health check tests

func TestHealthEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %s, want ok", resp["status"])
	}
}

// Tools endpoint tests

func TestListTools(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.tools = []ToolInfo{
		{Name: "search", Description: "Web search", Category: "search"},
		{Name: "read_file", Description: "Read a file", Category: "filesystem"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Object string     `json:"object"`
		Data   []ToolInfo `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Object != "list" {
		t.Errorf("object = %s, want list", resp.Object)
	}

	if len(resp.Data) != 2 {
		t.Errorf("tools count = %d, want 2", len(resp.Data))
	}
}

func TestListTools_MethodNotAllowed(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tools", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// Agent endpoint tests

func TestListAgents(t *testing.T) {
	srv, handler := setupTestServer(t)

	// Create some test agents
	handler.agents["default"] = &AgentInfo{
		ID:        "default",
		Name:      "Default Agent",
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	handler.agents["coder"] = &AgentInfo{
		ID:        "coder",
		Name:      "Code Expert",
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Object string      `json:"object"`
		Data   []AgentInfo `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Object != "list" {
		t.Errorf("object = %s, want list", resp.Object)
	}

	if len(resp.Data) != 2 {
		t.Errorf("agents count = %d, want 2", len(resp.Data))
	}
}

func TestCreateAgent(t *testing.T) {
	srv, _ := setupTestServer(t)

	body := `{"name": "Test Agent", "provider": "anthropic", "model": "claude-sonnet-4-20250514"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp AgentInfo
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Name != "Test Agent" {
		t.Errorf("name = %s, want Test Agent", resp.Name)
	}

	if resp.ID == "" {
		t.Error("ID should be auto-generated")
	}
}

func TestCreateAgent_MissingName(t *testing.T) {
	srv, _ := setupTestServer(t)

	body := `{"provider": "anthropic"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	// Huma returns 422 Unprocessable Entity for validation errors
	if w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d or %d", w.Code, http.StatusUnprocessableEntity, http.StatusBadRequest)
	}
}

func TestCreateAgent_Duplicate(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.agents["existing"] = &AgentInfo{ID: "existing", Name: "Existing"}

	body := `{"id": "existing", "name": "Duplicate"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestGetAgent(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.agents["test-id"] = &AgentInfo{
		ID:        "test-id",
		Name:      "Test Agent",
		Provider:  "anthropic",
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/test-id", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp AgentInfo
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != "test-id" {
		t.Errorf("ID = %s, want test-id", resp.ID)
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateAgent(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.agents["test-id"] = &AgentInfo{
		ID:          "test-id",
		Name:        "Original Name",
		Temperature: 0.7,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}

	body := `{"name": "Updated Name", "temperature": 0.5}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/agents/test-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp AgentInfo
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Name != "Updated Name" {
		t.Errorf("name = %s, want Updated Name", resp.Name)
	}

	if resp.Temperature != 0.5 {
		t.Errorf("temperature = %f, want 0.5", resp.Temperature)
	}
}

func TestUpdateAgent_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	body := `{"name": "Updated"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/agents/nonexistent", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDeleteAgent(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.agents["test-id"] = &AgentInfo{ID: "test-id", Name: "Test"}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/test-id", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}

	if _, exists := handler.agents["test-id"]; exists {
		t.Error("agent should be deleted")
	}
}

func TestDeleteAgent_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestCloneAgent(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.agents["original"] = &AgentInfo{
		ID:          "original",
		Name:        "Original Agent",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
		Temperature: 0.7,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}

	body := `{"new_id": "clone-id", "new_name": "Cloned Agent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/original/clone", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp AgentInfo
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != "clone-id" {
		t.Errorf("ID = %s, want clone-id", resp.ID)
	}

	if resp.Name != "Cloned Agent" {
		t.Errorf("name = %s, want Cloned Agent", resp.Name)
	}

	// Original should still exist
	if _, exists := handler.agents["original"]; !exists {
		t.Error("original agent should still exist")
	}
}

func TestCloneAgent_MissingName(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.agents["original"] = &AgentInfo{ID: "original", Name: "Original"}

	body := `{"new_id": "clone-id"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/original/clone", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	// Huma returns 422 Unprocessable Entity for validation errors
	if w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d or %d", w.Code, http.StatusUnprocessableEntity, http.StatusBadRequest)
	}
}

func TestCloneAgent_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	body := `{"new_name": "Clone"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/nonexistent/clone", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// Cron job endpoint tests

func TestListCronJobs(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.cronJobs["job-1"] = &CronJobInfo{
		ID:        "job-1",
		Name:      "Daily Report",
		Status:    "active",
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cron/jobs", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Object string        `json:"object"`
		Data   []CronJobInfo `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("jobs count = %d, want 1", len(resp.Data))
	}
}

func TestCreateCronJob(t *testing.T) {
	srv, _ := setupTestServer(t)

	body := `{
		"name": "Test Job",
		"schedule": {"cron": "0 * * * *"},
		"action": {"type": "send_message", "message": "Hello"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cron/jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp CronJobInfo
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Name != "Test Job" {
		t.Errorf("name = %s, want Test Job", resp.Name)
	}
}

func TestGetCronJob(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.cronJobs["job-1"] = &CronJobInfo{
		ID:        "job-1",
		Name:      "Test Job",
		Status:    "active",
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cron/jobs/job-1", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGetCronJob_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cron/jobs/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateCronJob(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.cronJobs["job-1"] = &CronJobInfo{
		ID:        "job-1",
		Name:      "Original",
		CreatedAt: time.Now(),
	}

	body := `{"name": "Updated Job"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cron/jobs/job-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestDeleteCronJob(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.cronJobs["job-1"] = &CronJobInfo{ID: "job-1", Name: "Test"}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cron/jobs/job-1", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestTriggerCronJob(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.cronJobs["job-1"] = &CronJobInfo{ID: "job-1", Name: "Test"}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cron/jobs/job-1/trigger", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp CronJobResult
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success = true")
	}
}

func TestEnableCronJob(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.cronJobs["job-1"] = &CronJobInfo{ID: "job-1", Status: "disabled"}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cron/jobs/job-1/enable", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if handler.cronJobs["job-1"].Status != "active" {
		t.Errorf("status = %s, want active", handler.cronJobs["job-1"].Status)
	}
}

func TestDisableCronJob(t *testing.T) {
	srv, handler := setupTestServer(t)

	handler.cronJobs["job-1"] = &CronJobInfo{ID: "job-1", Status: "active"}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cron/jobs/job-1/disable", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if handler.cronJobs["job-1"].Status != "disabled" {
		t.Errorf("status = %s, want disabled", handler.cronJobs["job-1"].Status)
	}
}

// Server options tests

func TestWithAPIKeys(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithAPIKeys("test-key"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if srv.config.APIKeys[0] != "test-key" {
		t.Errorf("APIKeys[0] = %s, want test-key", srv.config.APIKeys[0])
	}
}

func TestWithWebUI(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithWebUI(true))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if !srv.config.WebUI {
		t.Error("WebUI should be true")
	}

	// Test that root path serves index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %s, want text/html; charset=utf-8", contentType)
	}
}

func TestWithPhoneNumber(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithWebUI(true), WithPhoneNumber("+1234567890"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if srv.config.PhoneNumber != "+1234567890" {
		t.Errorf("PhoneNumber = %s, want +1234567890", srv.config.PhoneNumber)
	}

	// Test that phone number is injected into HTML
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	body := w.Body.String()
	if !bytes.Contains([]byte(body), []byte(`window.OMNIAGENT_PHONE="+1234567890"`)) {
		t.Error("phone number should be injected into HTML")
	}
}
