// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package openapi provides a compiled.Skill implementation that wraps an OpenAPI spec.
//
// This allows any HTTP API with an OpenAPI 3.x specification to be used as an
// agent skill. The skill parses the spec, generates tools from operations, and
// handles HTTP calls with authentication.
//
// # Example
//
//	skill := openapi.NewSkill(openapi.Config{
//		Name:    "petstore",
//		SpecURL: "https://petstore3.swagger.io/api/v3/openapi.json",
//		Auth: openapi.AuthConfig{
//			Type:   openapi.AuthAPIKey,
//			APIKey: os.Getenv("PETSTORE_API_KEY"),
//		},
//	})
//
//	agent, err := agent.New(config,
//		agent.WithCompiledSkill(skill),
//	)
package openapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omniskill/skill"
)

// Skill wraps an OpenAPI spec as a compiled.Skill.
//
// The skill parses the OpenAPI specification and exposes each operation
// as a tool that can be called by the agent.
type Skill struct {
	config Config
	spec   *openapi3.T
	tools  []skill.Tool
	mu     sync.RWMutex

	// For lazy loading
	loadOnce sync.Once
	loadErr  error
}

// operationInfo holds metadata for an operation needed for execution.
type operationInfo struct {
	method      string
	path        string
	operationID string
	pathParams  []string
}

// NewSkill creates a new OpenAPI skill with the given configuration.
//
// The spec is not loaded until Init() is called (or first tool call
// if LazyLoad is true).
func NewSkill(cfg Config) *Skill {
	cfg.setDefaults()

	return &Skill{
		config: cfg,
	}
}

// Name returns the skill identifier.
func (s *Skill) Name() string {
	return s.config.Name
}

// Description returns a human-readable description of the skill.
func (s *Skill) Description() string {
	return s.config.Description
}

// Tools returns the tools provided by this skill.
//
// If LazyLoad is enabled and the spec hasn't been loaded yet,
// this returns an empty slice.
func (s *Skill) Tools() []skill.Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.tools
}

// Init initializes the skill by loading and parsing the OpenAPI spec.
//
// If LazyLoad is false (default), this loads immediately and
// discovers all available operations.
//
// If LazyLoad is true, this returns immediately and loading
// happens on first tool call.
func (s *Skill) Init(ctx context.Context) error {
	if s.config.LazyLoad {
		return nil
	}

	return s.load(ctx)
}

// Close releases resources. For OpenAPI skills, this is a no-op
// since there are no persistent connections.
func (s *Skill) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.spec = nil
	s.tools = nil

	return nil
}

// load fetches and parses the OpenAPI spec.
func (s *Skill) load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.spec != nil {
		return nil // Already loaded
	}

	loader := openapi3.NewLoader()
	loader.Context = ctx
	loader.IsExternalRefsAllowed = true

	var spec *openapi3.T
	var err error

	if s.config.SpecURL != "" {
		spec, err = loader.LoadFromURI(&url.URL{
			Scheme: "https",
			Host:   "",
			Path:   s.config.SpecURL,
		})
		if err != nil {
			// Try loading as full URL
			u, parseErr := url.Parse(s.config.SpecURL)
			if parseErr != nil {
				return fmt.Errorf("openapi: invalid spec URL: %w", parseErr)
			}
			spec, err = loader.LoadFromURI(u)
		}
	} else if s.config.SpecFile != "" {
		data, readErr := os.ReadFile(s.config.SpecFile)
		if readErr != nil {
			return fmt.Errorf("openapi: failed to read spec file: %w", readErr)
		}
		spec, err = loader.LoadFromData(data)
	} else {
		return errors.New("openapi: no spec URL or file provided")
	}

	if err != nil {
		return fmt.Errorf("openapi: failed to load spec: %w", err)
	}

	// Validate the spec
	if err := spec.Validate(ctx); err != nil {
		return fmt.Errorf("openapi: invalid spec: %w", err)
	}

	s.spec = spec

	// Update description from spec if not set
	if s.config.Description == "OpenAPI skill: "+s.config.Name && spec.Info != nil {
		if spec.Info.Description != "" {
			s.config.Description = spec.Info.Description
		} else if spec.Info.Title != "" {
			s.config.Description = spec.Info.Title
		}
	}

	// Generate tools from operations
	s.generateTools()

	return nil
}

// ensureLoaded ensures the spec is loaded, using lazy load if needed.
func (s *Skill) ensureLoaded(ctx context.Context) error {
	s.mu.RLock()
	loaded := s.spec != nil
	s.mu.RUnlock()

	if loaded {
		return nil
	}

	s.loadOnce.Do(func() {
		s.loadErr = s.load(ctx)
	})

	return s.loadErr
}

// generateTools creates skill.Tool entries from OpenAPI operations.
// Must be called with s.mu held.
func (s *Skill) generateTools() {
	s.tools = nil

	includeOps := make(map[string]bool)
	for _, op := range s.config.IncludeOperations {
		includeOps[op] = true
	}

	excludeOps := make(map[string]bool)
	for _, op := range s.config.ExcludeOperations {
		excludeOps[op] = true
	}

	includeTags := make(map[string]bool)
	for _, tag := range s.config.IncludeTags {
		includeTags[tag] = true
	}

	for path, pathItem := range s.spec.Paths.Map() {
		for method, op := range pathItem.Operations() {
			if op == nil {
				continue
			}

			// Filter by operation ID
			if len(includeOps) > 0 && !includeOps[op.OperationID] {
				continue
			}
			if excludeOps[op.OperationID] {
				continue
			}

			// Filter by tags
			if len(includeTags) > 0 {
				hasTag := false
				for _, tag := range op.Tags {
					if includeTags[tag] {
						hasTag = true
						break
					}
				}
				if !hasTag {
					continue
				}
			}

			tool := s.operationToTool(method, path, pathItem, op)
			s.tools = append(s.tools, tool)
		}
	}
}

// operationToTool converts an OpenAPI operation to a skill.Tool.
func (s *Skill) operationToTool(method, path string, pathItem *openapi3.PathItem, op *openapi3.Operation) skill.Tool {
	// Generate tool name from operationID or method+path
	name := op.OperationID
	if name == "" {
		// Convert path to snake_case name
		name = strings.ToLower(method) + "_" + pathToName(path)
	}

	// Generate description
	description := op.Summary
	if description == "" {
		description = op.Description
	}
	if description == "" {
		description = fmt.Sprintf("%s %s", strings.ToUpper(method), path)
	}

	// Extract path parameters
	var pathParams []string
	for _, segment := range strings.Split(path, "/") {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			pathParams = append(pathParams, segment[1:len(segment)-1])
		}
	}

	// Build parameters
	params := s.buildParameters(pathItem, op)

	// Create operation info for the handler
	opInfo := operationInfo{
		method:      method,
		path:        path,
		operationID: op.OperationID,
		pathParams:  pathParams,
	}

	return skill.NewTool(
		name,
		description,
		params,
		s.makeToolHandler(opInfo, pathItem, op),
	)
}

// buildParameters extracts parameters from an OpenAPI operation.
func (s *Skill) buildParameters(pathItem *openapi3.PathItem, op *openapi3.Operation) map[string]skill.Parameter {
	params := make(map[string]skill.Parameter)

	// Collect all parameters (path item + operation level)
	allParams := append(pathItem.Parameters, op.Parameters...)

	for _, paramRef := range allParams {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}

		p := paramRef.Value
		param := skill.Parameter{
			Type:        schemaToType(p.Schema),
			Description: p.Description,
			Required:    p.Required,
		}

		if p.Schema != nil && p.Schema.Value != nil {
			if p.Schema.Value.Default != nil {
				param.Default = p.Schema.Value.Default
			}
			if len(p.Schema.Value.Enum) > 0 {
				param.Enum = p.Schema.Value.Enum
			}
		}

		params[p.Name] = param
	}

	// Handle request body
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		rb := op.RequestBody.Value

		// Find JSON content
		if content, ok := rb.Content["application/json"]; ok && content.Schema != nil {
			bodyParams := s.schemaToParameters(content.Schema, rb.Required)
			for name, param := range bodyParams {
				// Prefix body params to avoid conflicts
				params["body_"+name] = param
			}

			// Also add a "body" parameter for raw JSON
			params["body"] = skill.Parameter{
				Type:        "object",
				Description: "Request body as JSON object",
				Required:    rb.Required,
			}
		}
	}

	return params
}

// schemaToParameters converts a JSON schema to skill parameters.
func (s *Skill) schemaToParameters(schemaRef *openapi3.SchemaRef, required bool) map[string]skill.Parameter {
	params := make(map[string]skill.Parameter)

	if schemaRef == nil || schemaRef.Value == nil {
		return params
	}

	schema := schemaRef.Value

	// If it's an object, extract properties
	if schema.Type.Is("object") {
		requiredFields := make(map[string]bool)
		for _, r := range schema.Required {
			requiredFields[r] = true
		}

		for name, propRef := range schema.Properties {
			if propRef == nil || propRef.Value == nil {
				continue
			}

			prop := propRef.Value
			param := skill.Parameter{
				Type:        schemaTypeToString(prop.Type),
				Description: prop.Description,
				Required:    required && requiredFields[name],
			}

			if prop.Default != nil {
				param.Default = prop.Default
			}
			if len(prop.Enum) > 0 {
				param.Enum = prop.Enum
			}

			params[name] = param
		}
	}

	return params
}

// schemaToType extracts the type from a schema reference.
func schemaToType(schemaRef *openapi3.SchemaRef) string {
	if schemaRef == nil || schemaRef.Value == nil {
		return "string"
	}
	return schemaTypeToString(schemaRef.Value.Type)
}

// schemaTypeToString converts OpenAPI schema type to string.
func schemaTypeToString(t *openapi3.Types) string {
	if t == nil || len(*t) == 0 {
		return "string"
	}
	return (*t)[0]
}

// pathToName converts a URL path to a snake_case name.
func pathToName(path string) string {
	// Remove leading slash and replace special chars
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.ReplaceAll(path, "-", "_")
	return path
}

// makeToolHandler creates a handler that executes the HTTP request.
func (s *Skill) makeToolHandler(opInfo operationInfo, pathItem *openapi3.PathItem, op *openapi3.Operation) skill.ToolFunc {
	return func(ctx context.Context, params map[string]any) (any, error) {
		// Ensure spec is loaded (for lazy load)
		if err := s.ensureLoaded(ctx); err != nil {
			return nil, err
		}

		// Build the request URL
		baseURL := s.getBaseURL()
		path := opInfo.path

		// Substitute path parameters
		for _, paramName := range opInfo.pathParams {
			if val, ok := params[paramName]; ok {
				path = strings.ReplaceAll(path, "{"+paramName+"}", fmt.Sprintf("%v", val))
			}
		}

		fullURL := baseURL + path

		// Collect query parameters
		queryParams := url.Values{}
		allParams := append(pathItem.Parameters, op.Parameters...)

		for _, paramRef := range allParams {
			if paramRef == nil || paramRef.Value == nil {
				continue
			}
			p := paramRef.Value
			if p.In == "query" {
				if val, ok := params[p.Name]; ok {
					queryParams.Set(p.Name, fmt.Sprintf("%v", val))
				}
			}
		}

		if len(queryParams) > 0 {
			fullURL += "?" + queryParams.Encode()
		}

		// Build request body
		var bodyReader io.Reader
		if op.RequestBody != nil && op.RequestBody.Value != nil {
			if body, ok := params["body"]; ok {
				bodyData, err := json.Marshal(body)
				if err != nil {
					return nil, fmt.Errorf("openapi: failed to marshal body: %w", err)
				}
				bodyReader = bytes.NewReader(bodyData)
			}
		}

		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, strings.ToUpper(opInfo.method), fullURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("openapi: failed to create request: %w", err)
		}

		// Set headers
		if bodyReader != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")

		// Add header parameters
		for _, paramRef := range allParams {
			if paramRef == nil || paramRef.Value == nil {
				continue
			}
			p := paramRef.Value
			if p.In == "header" {
				if val, ok := params[p.Name]; ok {
					req.Header.Set(p.Name, fmt.Sprintf("%v", val))
				}
			}
		}

		// Apply authentication
		s.applyAuth(req)

		// Execute request
		client := s.config.HTTPClient
		if client == nil {
			client = http.DefaultClient
		}

		if s.config.RequestTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(s.config.RequestTimeout)*time.Second)
			defer cancel()
			req = req.WithContext(ctx)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("openapi: request failed: %w", err)
		}
		defer resp.Body.Close()

		// Read response
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("openapi: failed to read response: %w", err)
		}

		// Parse response
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("openapi: API error %d: %s", resp.StatusCode, string(respBody))
		}

		// Try to parse as JSON
		var result any
		if err := json.Unmarshal(respBody, &result); err != nil {
			// Return as string if not JSON
			return string(respBody), nil
		}

		return result, nil
	}
}

// getBaseURL returns the base URL for API calls.
func (s *Skill) getBaseURL() string {
	if s.config.BaseURL != "" {
		return strings.TrimSuffix(s.config.BaseURL, "/")
	}

	// Use first server URL from spec
	if s.spec != nil && len(s.spec.Servers) > 0 {
		return strings.TrimSuffix(s.spec.Servers[0].URL, "/")
	}

	return ""
}

// applyAuth applies authentication to the request.
func (s *Skill) applyAuth(req *http.Request) {
	switch s.config.Auth.Type {
	case AuthAPIKey:
		if s.config.Auth.APIKeyIn == "header" {
			req.Header.Set(s.config.Auth.APIKeyName, s.config.Auth.APIKey)
		} else if s.config.Auth.APIKeyIn == "query" {
			q := req.URL.Query()
			q.Set(s.config.Auth.APIKeyName, s.config.Auth.APIKey)
			req.URL.RawQuery = q.Encode()
		}
	case AuthBearer:
		req.Header.Set("Authorization", "Bearer "+s.config.Auth.Token)
	case AuthBasic:
		auth := base64.StdEncoding.EncodeToString(
			[]byte(s.config.Auth.Username + ":" + s.config.Auth.Password),
		)
		req.Header.Set("Authorization", "Basic "+auth)
	}
}

// Verify Skill implements compiled.Skill at compile time.
var _ compiled.Skill = (*Skill)(nil)
