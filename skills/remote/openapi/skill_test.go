// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/plexusone/omniagent/skills/compiled"
	skilltypes "github.com/plexusone/omniskill/skill"
)

// Minimal OpenAPI 3.0 spec for testing
const testSpec = `{
  "openapi": "3.0.0",
  "info": {
    "title": "Test API",
    "description": "A test API for unit tests",
    "version": "1.0.0"
  },
  "servers": [
    {"url": "https://api.example.com/v1"}
  ],
  "paths": {
    "/pets": {
      "get": {
        "operationId": "listPets",
        "summary": "List all pets",
        "tags": ["pets"],
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "description": "Maximum number of pets to return",
            "required": false,
            "schema": {"type": "integer", "default": 10}
          }
        ],
        "responses": {
          "200": {
            "description": "A list of pets"
          }
        }
      },
      "post": {
        "operationId": "createPet",
        "summary": "Create a pet",
        "tags": ["pets"],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["name"],
                "properties": {
                  "name": {"type": "string", "description": "Pet name"},
                  "tag": {"type": "string", "description": "Pet tag"}
                }
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Pet created"
          }
        }
      }
    },
    "/pets/{petId}": {
      "get": {
        "operationId": "getPet",
        "summary": "Get a pet by ID",
        "tags": ["pets"],
        "parameters": [
          {
            "name": "petId",
            "in": "path",
            "required": true,
            "description": "The pet ID",
            "schema": {"type": "string"}
          }
        ],
        "responses": {
          "200": {
            "description": "A pet"
          }
        }
      }
    },
    "/users": {
      "get": {
        "operationId": "listUsers",
        "summary": "List all users",
        "tags": ["users"],
        "responses": {
          "200": {
            "description": "A list of users"
          }
        }
      }
    }
  }
}`

func TestNewSkill(t *testing.T) {
	skill := NewSkill(Config{
		Name: "test",
	})

	if skill == nil {
		t.Fatal("NewSkill returned nil")
	}

	if skill.Name() != "test" {
		t.Errorf("Name() = %q, want %q", skill.Name(), "test")
	}

	if skill.Description() != "OpenAPI skill: test" {
		t.Errorf("Description() = %q, want %q", skill.Description(), "OpenAPI skill: test")
	}
}

func TestSkillImplementsInterface(t *testing.T) {
	var _ compiled.Skill = (*Skill)(nil)
}

func TestSkillInitFromFile(t *testing.T) {
	// Write test spec to temp file
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Check tools were generated
	tools := skill.Tools()
	if len(tools) != 4 {
		t.Errorf("len(Tools()) = %d, want 4", len(tools))
	}

	// Check specific tools exist
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name()] = true
	}

	expectedTools := []string{"listPets", "createPet", "getPet", "listUsers"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("Missing tool %q", name)
		}
	}
}

func TestSkillFilterByOperationID(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:              "petstore",
		SpecFile:          specFile,
		IncludeOperations: []string{"listPets", "getPet"},
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	tools := skill.Tools()
	if len(tools) != 2 {
		t.Errorf("len(Tools()) = %d, want 2", len(tools))
	}
}

func TestSkillFilterByTag(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:        "petstore",
		SpecFile:    specFile,
		IncludeTags: []string{"pets"},
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	tools := skill.Tools()
	if len(tools) != 3 { // listPets, createPet, getPet
		t.Errorf("len(Tools()) = %d, want 3", len(tools))
	}
}

func TestSkillExcludeOperations(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:              "petstore",
		SpecFile:          specFile,
		ExcludeOperations: []string{"listUsers"},
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	tools := skill.Tools()
	if len(tools) != 3 {
		t.Errorf("len(Tools()) = %d, want 3", len(tools))
	}

	for _, tool := range tools {
		if tool.Name() == "listUsers" {
			t.Error("listUsers should be excluded")
		}
	}
}

func TestSkillToolParameters(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Find listPets tool
	var listPetsTool skilltypes.Tool
	for _, tool := range skill.Tools() {
		if tool.Name() == "listPets" {
			listPetsTool = tool
			break
		}
	}

	if listPetsTool == nil {
		t.Fatal("listPets tool not found")
	}

	params := listPetsTool.Parameters()
	if params == nil {
		t.Fatal("Parameters() returned nil")
	}

	// Check limit parameter exists
	if _, ok := params["limit"]; !ok {
		t.Error("Missing 'limit' parameter")
	}
}

func TestSkillHTTPExecution(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/pets":
			if r.Method == "GET" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]map[string]any{
					{"id": "1", "name": "Fluffy"},
					{"id": "2", "name": "Buddy"},
				})
				return
			}
		case "/v1/pets/123":
			if r.Method == "GET" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{
					"id":   "123",
					"name": "Max",
				})
				return
			}
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
		BaseURL:  server.URL + "/v1",
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Find and call listPets
	for _, tool := range skill.Tools() {
		if tool.Name() == "listPets" {
			result, err := tool.Call(ctx, map[string]any{"limit": 10})
			if err != nil {
				t.Fatalf("listPets call error: %v", err)
			}

			pets, ok := result.([]any)
			if !ok {
				t.Fatalf("Expected []any, got %T", result)
			}
			if len(pets) != 2 {
				t.Errorf("len(pets) = %d, want 2", len(pets))
			}
			break
		}
	}

	// Find and call getPet with path parameter
	for _, tool := range skill.Tools() {
		if tool.Name() == "getPet" {
			result, err := tool.Call(ctx, map[string]any{"petId": "123"})
			if err != nil {
				t.Fatalf("getPet call error: %v", err)
			}

			pet, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("Expected map[string]any, got %T", result)
			}
			if pet["name"] != "Max" {
				t.Errorf("pet[name] = %v, want Max", pet["name"])
			}
			break
		}
	}
}

func TestSkillAuthAPIKey(t *testing.T) {
	var receivedAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
		BaseURL:  server.URL + "/v1",
		Auth: AuthConfig{
			Type:   AuthAPIKey,
			APIKey: "test-api-key-123",
		},
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Call any tool to trigger auth
	for _, tool := range skill.Tools() {
		if tool.Name() == "listPets" {
			_, err := tool.Call(ctx, map[string]any{})
			if err != nil {
				t.Fatalf("listPets call error: %v", err)
			}
			break
		}
	}

	if receivedAPIKey != "test-api-key-123" {
		t.Errorf("API key = %q, want %q", receivedAPIKey, "test-api-key-123")
	}
}

func TestSkillAuthBearer(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
		BaseURL:  server.URL + "/v1",
		Auth: AuthConfig{
			Type:  AuthBearer,
			Token: "my-bearer-token",
		},
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, tool := range skill.Tools() {
		if tool.Name() == "listPets" {
			_, err := tool.Call(ctx, map[string]any{})
			if err != nil {
				t.Fatalf("listPets call error: %v", err)
			}
			break
		}
	}

	if receivedAuth != "Bearer my-bearer-token" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer my-bearer-token")
	}
}

func TestSkillLazyLoad(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
		LazyLoad: true,
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Tools should be empty before first use
	if len(skill.Tools()) != 0 {
		t.Errorf("len(Tools()) = %d, want 0 before lazy load", len(skill.Tools()))
	}

	// Force load by calling ensureLoaded
	if err := skill.ensureLoaded(ctx); err != nil {
		t.Fatalf("ensureLoaded() error: %v", err)
	}

	// Now tools should be populated
	if len(skill.Tools()) != 4 {
		t.Errorf("len(Tools()) = %d, want 4 after lazy load", len(skill.Tools()))
	}
}

func TestSkillClose(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	if len(skill.Tools()) == 0 {
		t.Error("Tools should be populated after Init")
	}

	if err := skill.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	if len(skill.Tools()) != 0 {
		t.Error("Tools should be cleared after Close")
	}
}

func TestConfigDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		wantDesc string
		wantIn   string
		wantKey  string
	}{
		{
			name:     "empty config",
			config:   Config{Name: "test"},
			wantDesc: "OpenAPI skill: test",
			wantIn:   "header",
			wantKey:  "X-API-Key",
		},
		{
			name:     "query API key",
			config:   Config{Name: "test", Auth: AuthConfig{APIKeyIn: "query"}},
			wantDesc: "OpenAPI skill: test",
			wantIn:   "query",
			wantKey:  "api_key",
		},
		{
			name:     "custom description",
			config:   Config{Name: "test", Description: "My API"},
			wantDesc: "My API",
			wantIn:   "header",
			wantKey:  "X-API-Key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.setDefaults()

			if tt.config.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", tt.config.Description, tt.wantDesc)
			}
			if tt.config.Auth.APIKeyIn != tt.wantIn {
				t.Errorf("APIKeyIn = %q, want %q", tt.config.Auth.APIKeyIn, tt.wantIn)
			}
			if tt.config.Auth.APIKeyName != tt.wantKey {
				t.Errorf("APIKeyName = %q, want %q", tt.config.Auth.APIKeyName, tt.wantKey)
			}
		})
	}
}

func TestPathToName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/pets", "pets"},
		{"/pets/{petId}", "pets_petId"},
		{"/users/{userId}/pets", "users_userId_pets"},
		{"/api-v1/items", "api_v1_items"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pathToName(tt.path)
			if got != tt.want {
				t.Errorf("pathToName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSkillInitFromURL(t *testing.T) {
	// Serve the spec from a mock server
	specServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(testSpec))
	}))
	defer specServer.Close()

	skill := NewSkill(Config{
		Name:    "petstore",
		SpecURL: specServer.URL,
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	tools := skill.Tools()
	if len(tools) != 4 {
		t.Errorf("len(Tools()) = %d, want 4", len(tools))
	}
}

func TestSkillAuthBasic(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
		BaseURL:  server.URL + "/v1",
		Auth: AuthConfig{
			Type:     AuthBasic,
			Username: "testuser",
			Password: "testpass",
		},
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, tool := range skill.Tools() {
		if tool.Name() == "listPets" {
			_, err := tool.Call(ctx, map[string]any{})
			if err != nil {
				t.Fatalf("listPets call error: %v", err)
			}
			break
		}
	}

	// Basic auth: base64("testuser:testpass") = "dGVzdHVzZXI6dGVzdHBhc3M="
	expected := "Basic dGVzdHVzZXI6dGVzdHBhc3M="
	if receivedAuth != expected {
		t.Errorf("Authorization = %q, want %q", receivedAuth, expected)
	}
}

func TestSkillAuthAPIKeyQuery(t *testing.T) {
	var receivedAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.URL.Query().Get("api_key")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
		BaseURL:  server.URL + "/v1",
		Auth: AuthConfig{
			Type:     AuthAPIKey,
			APIKey:   "query-api-key-456",
			APIKeyIn: "query",
		},
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, tool := range skill.Tools() {
		if tool.Name() == "listPets" {
			_, err := tool.Call(ctx, map[string]any{})
			if err != nil {
				t.Fatalf("listPets call error: %v", err)
			}
			break
		}
	}

	if receivedAPIKey != "query-api-key-456" {
		t.Errorf("API key = %q, want %q", receivedAPIKey, "query-api-key-456")
	}
}

func TestSkillPOSTWithBody(t *testing.T) {
	var receivedBody map[string]any
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/pets" && r.Method == "POST" {
			receivedContentType = r.Header.Get("Content-Type")
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"id":   "new-pet-id",
				"name": receivedBody["name"],
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
		BaseURL:  server.URL + "/v1",
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, tool := range skill.Tools() {
		if tool.Name() == "createPet" {
			result, err := tool.Call(ctx, map[string]any{
				"body": map[string]any{
					"name": "Whiskers",
					"tag":  "cat",
				},
			})
			if err != nil {
				t.Fatalf("createPet call error: %v", err)
			}

			// Verify content type was set
			if receivedContentType != "application/json" {
				t.Errorf("Content-Type = %q, want %q", receivedContentType, "application/json")
			}

			// Verify body was sent
			if receivedBody["name"] != "Whiskers" {
				t.Errorf("body.name = %v, want Whiskers", receivedBody["name"])
			}

			// Verify response
			resp, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("Expected map[string]any, got %T", result)
			}
			if resp["name"] != "Whiskers" {
				t.Errorf("response.name = %v, want Whiskers", resp["name"])
			}
			break
		}
	}
}

func TestSkillAPIErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error":   "not_found",
			"message": "Pet not found",
		})
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
		BaseURL:  server.URL + "/v1",
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, tool := range skill.Tools() {
		if tool.Name() == "getPet" {
			_, err := tool.Call(ctx, map[string]any{"petId": "999"})
			if err == nil {
				t.Fatal("Expected error for 404 response")
			}
			// Error should contain status code
			if !strings.Contains(err.Error(), "404") {
				t.Errorf("Error should contain 404: %v", err)
			}
			break
		}
	}
}

func TestSkillServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
		BaseURL:  server.URL + "/v1",
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, tool := range skill.Tools() {
		if tool.Name() == "listPets" {
			_, err := tool.Call(ctx, map[string]any{})
			if err == nil {
				t.Fatal("Expected error for 500 response")
			}
			if !strings.Contains(err.Error(), "500") {
				t.Errorf("Error should contain 500: %v", err)
			}
			break
		}
	}
}

func TestSkillTextResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Plain text response"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
		BaseURL:  server.URL + "/v1",
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, tool := range skill.Tools() {
		if tool.Name() == "listPets" {
			result, err := tool.Call(ctx, map[string]any{})
			if err != nil {
				t.Fatalf("listPets call error: %v", err)
			}

			// Non-JSON should return as string
			text, ok := result.(string)
			if !ok {
				t.Fatalf("Expected string, got %T", result)
			}
			if text != "Plain text response" {
				t.Errorf("result = %q, want %q", text, "Plain text response")
			}
			break
		}
	}
}

func TestSkillUseSpecServerURL(t *testing.T) {
	// Create API server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{"pet1", "pet2"})
	}))
	defer apiServer.Close()

	// Create spec with server URL pointing to our API server
	specWithServer := `{
		"openapi": "3.0.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"servers": [{"url": "` + apiServer.URL + `/v1"}],
		"paths": {
			"/pets": {
				"get": {
					"operationId": "listPets",
					"responses": {"200": {"description": "OK"}}
				}
			}
		}
	}`

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(specWithServer), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Don't set BaseURL - should use server from spec
	skill := NewSkill(Config{
		Name:     "petstore",
		SpecFile: specFile,
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, tool := range skill.Tools() {
		if tool.Name() == "listPets" {
			result, err := tool.Call(ctx, map[string]any{})
			if err != nil {
				t.Fatalf("listPets call error: %v", err)
			}

			pets, ok := result.([]any)
			if !ok {
				t.Fatalf("Expected []any, got %T", result)
			}
			if len(pets) != 2 {
				t.Errorf("len(pets) = %d, want 2", len(pets))
			}
			break
		}
	}
}

func TestSkillRequestTimeout(t *testing.T) {
	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(testSpec), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:           "petstore",
		SpecFile:       specFile,
		BaseURL:        server.URL + "/v1",
		RequestTimeout: 1, // 1 second timeout
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, tool := range skill.Tools() {
		if tool.Name() == "listPets" {
			_, err := tool.Call(ctx, map[string]any{})
			if err == nil {
				t.Fatal("Expected timeout error")
			}
			// Should be a context deadline exceeded or timeout error
			if !strings.Contains(err.Error(), "context deadline exceeded") &&
				!strings.Contains(err.Error(), "timeout") {
				t.Errorf("Expected timeout error, got: %v", err)
			}
			break
		}
	}
}

func TestSkillNoSpecProvided(t *testing.T) {
	skill := NewSkill(Config{
		Name: "empty",
	})

	ctx := context.Background()
	err := skill.Init(ctx)
	if err == nil {
		t.Fatal("Expected error when no spec URL or file provided")
	}
	if !strings.Contains(err.Error(), "no spec") {
		t.Errorf("Error should mention no spec: %v", err)
	}
}

func TestSkillInvalidSpecFile(t *testing.T) {
	skill := NewSkill(Config{
		Name:     "invalid",
		SpecFile: "/nonexistent/path/spec.json",
	})

	ctx := context.Background()
	err := skill.Init(ctx)
	if err == nil {
		t.Fatal("Expected error for nonexistent spec file")
	}
}

func TestSkillHeaderParameter(t *testing.T) {
	var receivedHeader string

	// Spec with header parameter
	specWithHeader := `{
		"openapi": "3.0.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"paths": {
			"/items": {
				"get": {
					"operationId": "listItems",
					"parameters": [
						{
							"name": "X-Custom-Header",
							"in": "header",
							"required": false,
							"schema": {"type": "string"}
						}
					],
					"responses": {"200": {"description": "OK"}}
				}
			}
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom-Header")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.json")
	if err := os.WriteFile(specFile, []byte(specWithHeader), 0o600); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	skill := NewSkill(Config{
		Name:     "test",
		SpecFile: specFile,
		BaseURL:  server.URL,
	})

	ctx := context.Background()
	if err := skill.Init(ctx); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, tool := range skill.Tools() {
		if tool.Name() == "listItems" {
			_, err := tool.Call(ctx, map[string]any{
				"X-Custom-Header": "custom-value-123",
			})
			if err != nil {
				t.Fatalf("listItems call error: %v", err)
			}
			break
		}
	}

	if receivedHeader != "custom-value-123" {
		t.Errorf("X-Custom-Header = %q, want %q", receivedHeader, "custom-value-123")
	}
}
