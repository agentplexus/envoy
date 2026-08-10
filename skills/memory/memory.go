// Package memory provides a compiled skill for semantic memory operations.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/plexusone/omnimemory/core"
	"github.com/plexusone/omniskill/skill"

	"github.com/plexusone/omniagent/memscope"
)

const (
	// SkillName is the name of the memory skill.
	SkillName = "memory"

	// DefaultScope is the default memory scope.
	DefaultScope = core.ScopeSession
)

// Skill implements compiled.Skill for memory operations using omnimemory.
type Skill struct {
	client   *core.Client
	tenantID string
	agentID  string
	config   Config
}

// Config configures the memory skill.
type Config struct {
	// Client is an existing omnimemory client.
	// If nil, a new in-memory client will be created.
	Client *core.Client

	// TenantID is the default tenant for memory operations.
	TenantID string

	// AgentID is the agent identifier for memory attribution.
	AgentID string
}

// NewSkill creates a new memory skill.
func NewSkill(cfg Config) *Skill {
	return &Skill{
		client:   cfg.Client,
		tenantID: cfg.TenantID,
		agentID:  cfg.AgentID,
		config:   cfg,
	}
}

// Name implements compiled.Skill.
func (s *Skill) Name() string {
	return SkillName
}

// Description implements compiled.Skill.
func (s *Skill) Description() string {
	return "Semantic memory for storing and retrieving information using omnimemory"
}

// Init implements compiled.Skill.
func (s *Skill) Init(ctx context.Context) error {
	// If no client provided, create an in-memory client for testing
	if s.client == nil {
		client, err := core.NewClient(core.ClientConfig{
			Providers: []core.ProviderConfig{
				{Name: core.ProviderNameMemory},
			},
		})
		if err != nil {
			return fmt.Errorf("create memory client: %w", err)
		}
		s.client = client
	}

	// Set defaults
	if s.tenantID == "" {
		s.tenantID = "default"
	}
	if s.agentID == "" {
		s.agentID = "agent"
	}

	return nil
}

// Close implements compiled.Skill.
func (s *Skill) Close() error {
	// Only close the client if we created it
	if s.config.Client == nil && s.client != nil {
		return s.client.Close()
	}
	return nil
}

// SetMemory sets the omnimemory client.
// This is called when the skill is registered with an agent that has memory configured.
func (s *Skill) SetMemory(client *core.Client) {
	s.client = client
}

// Client returns the omnimemory client.
func (s *Skill) Client() *core.Client {
	return s.client
}

// Tools implements compiled.Skill.
func (s *Skill) Tools() []skill.Tool {
	return []skill.Tool{
		&memoryTool{
			name:        "memory_store",
			description: "Store information in semantic memory for later retrieval",
			params: map[string]skill.Parameter{
				"content": {
					Type:        "string",
					Description: "The content to store in memory",
					Required:    true,
				},
				"type": {
					Type:        "string",
					Description: "Memory type: observation, fact, preference, summary, trait, relationship (default: observation)",
					Required:    false,
				},
				"scope": {
					Type:        "string",
					Description: "Memory scope: user, agent, tenant, team, session, domain (default: session)",
					Required:    false,
				},
				"subject_id": {
					Type:        "string",
					Description: "Subject ID (who this memory is about)",
					Required:    false,
				},
				"metadata": {
					Type:        "object",
					Description: "Optional metadata key-value pairs",
					Required:    false,
				},
			},
			handler: s.handleStore,
		},
		&memoryTool{
			name:        "memory_search",
			description: "Search semantic memory for relevant information",
			params: map[string]skill.Parameter{
				"query": {
					Type:        "string",
					Description: "The search query",
					Required:    true,
				},
				"types": {
					Type:        "array",
					Description: "Filter by memory types (observation, fact, preference, etc.)",
					Required:    false,
				},
				"scopes": {
					Type:        "array",
					Description: "Filter by memory scopes (user, session, etc.)",
					Required:    false,
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results (default: 5)",
					Required:    false,
				},
			},
			handler: s.handleSearch,
		},
		&memoryTool{
			name:        "memory_recall",
			description: "Recall relevant memories for the current context",
			params: map[string]skill.Parameter{
				"query": {
					Type:        "string",
					Description: "Query or context to recall memories for",
					Required:    true,
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum number of memories to recall (default: 5)",
					Required:    false,
				},
				"types": {
					Type:        "array",
					Description: "Filter by memory types",
					Required:    false,
				},
			},
			handler: s.handleRecall,
		},
		&memoryTool{
			name:        "memory_list",
			description: "List memories with optional filters",
			params: map[string]skill.Parameter{
				"types": {
					Type:        "array",
					Description: "Filter by memory types",
					Required:    false,
				},
				"scopes": {
					Type:        "array",
					Description: "Filter by memory scopes",
					Required:    false,
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results (default: 20)",
					Required:    false,
				},
			},
			handler: s.handleList,
		},
		&memoryTool{
			name:        "memory_delete",
			description: "Delete a memory by ID",
			params: map[string]skill.Parameter{
				"id": {
					Type:        "string",
					Description: "The ID of the memory to delete",
					Required:    true,
				},
			},
			handler: s.handleDelete,
		},
	}
}

// memoryContext creates an omnimemory context for operations. A memory scope
// stamped on ctx (e.g. a chat turn scoped to TenantID=team, SubjectID="chat:<id>"
// — RMI-OMNIAGENT-114) sets the tenant and the default subject, so tool-driven
// memory lands in the same per-chat scope as the agent's automatic recall. An
// explicit subject_id argument still wins for that one operation; absent both a
// scope subject and an explicit subject, memory defaults to the tenant subject.
func (s *Skill) memoryContext(ctx context.Context, subjectID string) core.Context {
	tenantID := s.tenantID
	scope, scoped := memscope.FromContext(ctx)
	if scoped && scope.TenantID != "" {
		tenantID = scope.TenantID
	}
	if subjectID == "" {
		switch {
		case scoped && scope.SubjectID != "":
			subjectID = scope.SubjectID
		default:
			subjectID = tenantID // Default to tenant if no subject specified
		}
	}
	return core.Context{
		TenantID:  tenantID,
		SubjectID: subjectID,
		AgentID:   s.agentID,
	}
}

type storeInput struct {
	Content   string         `json:"content"`
	Type      string         `json:"type"`
	Scope     string         `json:"scope"`
	SubjectID string         `json:"subject_id"`
	Metadata  map[string]any `json:"metadata"`
}

func (s *Skill) handleStore(ctx context.Context, params map[string]any) (any, error) {
	data, _ := json.Marshal(params)
	var input storeInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	memType := core.MemoryTypeObservation
	if input.Type != "" {
		memType = core.MemoryType(input.Type)
		if !memType.Valid() {
			return nil, fmt.Errorf("invalid memory type: %s", input.Type)
		}
	}

	scope := DefaultScope
	if input.Scope != "" {
		scope = core.Scope(input.Scope)
		if !scope.Valid() {
			return nil, fmt.Errorf("invalid memory scope: %s", input.Scope)
		}
	}

	memCtx := s.memoryContext(ctx, input.SubjectID)
	memCtx.Scope = scope

	mem, err := s.client.Add(ctx, &core.AddRequest{
		Context:  memCtx,
		Type:     memType,
		Content:  input.Content,
		Metadata: input.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("store memory: %w", err)
	}

	return map[string]any{
		"success": true,
		"id":      mem.ID,
		"type":    string(mem.Type),
		"scope":   string(mem.Scope),
	}, nil
}

type searchInput struct {
	Query  string   `json:"query"`
	Types  []string `json:"types"`
	Scopes []string `json:"scopes"`
	Limit  int      `json:"limit"`
}

func (s *Skill) handleSearch(ctx context.Context, params map[string]any) (any, error) {
	data, _ := json.Marshal(params)
	var input searchInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 5
	}

	var types []core.MemoryType
	for _, t := range input.Types {
		types = append(types, core.MemoryType(t))
	}

	var scopes []core.Scope
	for _, s := range input.Scopes {
		scopes = append(scopes, core.Scope(s))
	}

	resp, err := s.client.Search(ctx, &core.SearchRequest{
		Context: s.memoryContext(ctx, ""),
		Query:   input.Query,
		Types:   types,
		Scopes:  scopes,
		Limit:   limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search memory: %w", err)
	}

	output := make([]map[string]any, len(resp.Results))
	for i, r := range resp.Results {
		output[i] = map[string]any{
			"id":       r.Memory.ID,
			"content":  r.Memory.Content,
			"type":     string(r.Memory.Type),
			"scope":    string(r.Memory.Scope),
			"score":    r.Score,
			"metadata": r.Memory.Metadata,
		}
	}

	return map[string]any{
		"results": output,
		"count":   len(resp.Results),
	}, nil
}

type recallInput struct {
	Query      string   `json:"query"`
	MaxResults int      `json:"max_results"`
	Types      []string `json:"types"`
}

func (s *Skill) handleRecall(ctx context.Context, params map[string]any) (any, error) {
	data, _ := json.Marshal(params)
	var input recallInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}

	var types []core.MemoryType
	for _, t := range input.Types {
		types = append(types, core.MemoryType(t))
	}

	resp, err := s.client.Recall(ctx, &core.RecallRequest{
		Context:      s.memoryContext(ctx, ""),
		Query:        input.Query,
		MaxResults:   maxResults,
		IncludeTypes: types,
	})
	if err != nil {
		return nil, fmt.Errorf("recall memories: %w", err)
	}

	output := make([]map[string]any, len(resp.Memories))
	for i, m := range resp.Memories {
		output[i] = map[string]any{
			"id":       m.ID,
			"content":  m.Content,
			"type":     string(m.Type),
			"scope":    string(m.Scope),
			"metadata": m.Metadata,
		}
	}

	result := map[string]any{
		"memories": output,
		"count":    len(resp.Memories),
	}
	if resp.Summary != "" {
		result["summary"] = resp.Summary
	}

	return result, nil
}

type listInput struct {
	Types  []string `json:"types"`
	Scopes []string `json:"scopes"`
	Limit  int      `json:"limit"`
}

func (s *Skill) handleList(ctx context.Context, params map[string]any) (any, error) {
	data, _ := json.Marshal(params)
	var input listInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	var types []core.MemoryType
	for _, t := range input.Types {
		types = append(types, core.MemoryType(t))
	}

	var scopes []core.Scope
	for _, s := range input.Scopes {
		scopes = append(scopes, core.Scope(s))
	}

	resp, err := s.client.List(ctx, &core.ListRequest{
		Context: s.memoryContext(ctx, ""),
		Types:   types,
		Scopes:  scopes,
		Limit:   limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}

	output := make([]map[string]any, len(resp.Memories))
	for i, m := range resp.Memories {
		output[i] = map[string]any{
			"id":         m.ID,
			"content":    truncate(m.Content, 100),
			"type":       string(m.Type),
			"scope":      string(m.Scope),
			"created_at": m.CreatedAt.Format(time.RFC3339),
		}
	}

	return map[string]any{
		"memories":    output,
		"count":       len(resp.Memories),
		"total_count": resp.TotalCount,
		"has_more":    resp.HasMore,
	}, nil
}

type deleteInput struct {
	ID string `json:"id"`
}

func (s *Skill) handleDelete(ctx context.Context, params map[string]any) (any, error) {
	data, _ := json.Marshal(params)
	var input deleteInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	if err := s.client.Delete(ctx, &core.DeleteRequest{
		Context: s.memoryContext(ctx, ""),
		ID:      input.ID,
	}); err != nil {
		return nil, fmt.Errorf("delete memory: %w", err)
	}

	return map[string]any{
		"success": true,
		"id":      input.ID,
	}, nil
}

// memoryTool implements skill.Tool.
type memoryTool struct {
	name        string
	description string
	params      map[string]skill.Parameter
	handler     func(ctx context.Context, params map[string]any) (any, error)
}

func (t *memoryTool) Name() string {
	return t.name
}

func (t *memoryTool) Description() string {
	return t.description
}

func (t *memoryTool) Parameters() map[string]skill.Parameter {
	return t.params
}

func (t *memoryTool) Call(ctx context.Context, params map[string]any) (any, error) {
	return t.handler(ctx, params)
}

// truncate shortens a string to the given length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
