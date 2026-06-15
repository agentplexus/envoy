// Package memory provides a compiled skill for semantic memory operations.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/plexusone/omniretrieve/memory"
	"github.com/plexusone/omniretrieve/vector"
	"github.com/plexusone/omniskill/skill"
	"github.com/plexusone/omnistorage-core/kvs"
)

const (
	// SkillName is the name of the memory skill.
	SkillName = "memory"

	// DefaultCollection is the default memory collection name.
	DefaultCollection = "default"
)

// Skill implements compiled.Skill for memory operations.
type Skill struct {
	manager  *memory.Manager
	embedder vector.Embedder
	storage  kvs.Store
}

// Config configures the memory skill.
type Config struct {
	// Embedder computes vector embeddings for semantic search.
	Embedder vector.Embedder
}

// NewSkill creates a new memory skill.
func NewSkill(cfg Config) *Skill {
	embedder := cfg.Embedder
	if embedder == nil {
		// Use hash embedder for testing (not suitable for production)
		embedder = memory.NewHashEmbedder(384)
	}

	return &Skill{
		embedder: embedder,
	}
}

// Name implements compiled.Skill.
func (s *Skill) Name() string {
	return SkillName
}

// Description implements compiled.Skill.
func (s *Skill) Description() string {
	return "Semantic memory for storing and retrieving information using vector search"
}

// SetStorage implements compiled.StorageAware.
func (s *Skill) SetStorage(store kvs.Store) {
	s.storage = store
}

// Init implements compiled.Skill.
func (s *Skill) Init(ctx context.Context) error {
	s.manager = memory.NewManager(memory.ManagerConfig{
		Embedder: s.embedder,
	})

	// Create default collection
	_, _ = s.manager.GetOrCreateCollection(ctx, DefaultCollection, "Default memory collection")

	return nil
}

// Close implements compiled.Skill.
func (s *Skill) Close() error {
	return nil
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
				"key": {
					Type:        "string",
					Description: "Optional unique key for the memory (auto-generated if not provided)",
					Required:    false,
				},
				"collection": {
					Type:        "string",
					Description: "Optional collection name (default: 'default')",
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
				"collection": {
					Type:        "string",
					Description: "Optional collection name (default: 'default')",
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
			name:        "memory_list",
			description: "List all memories in a collection",
			params: map[string]skill.Parameter{
				"collection": {
					Type:        "string",
					Description: "Optional collection name (default: 'default')",
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
			description: "Delete a memory by key",
			params: map[string]skill.Parameter{
				"key": {
					Type:        "string",
					Description: "The key of the memory to delete",
					Required:    true,
				},
				"collection": {
					Type:        "string",
					Description: "Optional collection name (default: 'default')",
					Required:    false,
				},
			},
			handler: s.handleDelete,
		},
		&memoryTool{
			name:        "memory_collections",
			description: "List all memory collections",
			params:      map[string]skill.Parameter{},
			handler:     s.handleListCollections,
		},
	}
}

// Manager returns the memory manager.
func (s *Skill) Manager() *memory.Manager {
	return s.manager
}

type storeInput struct {
	Content    string            `json:"content"`
	Key        string            `json:"key"`
	Collection string            `json:"collection"`
	Metadata   map[string]string `json:"metadata"`
}

func (s *Skill) handleStore(ctx context.Context, params map[string]any) (any, error) {
	data, _ := json.Marshal(params)
	var input storeInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	collection := input.Collection
	if collection == "" {
		collection = DefaultCollection
	}

	key := input.Key
	if key == "" {
		key = fmt.Sprintf("mem_%d", time.Now().UnixNano())
	}

	doc := &memory.Document{
		ID:       key,
		Content:  input.Content,
		Metadata: input.Metadata,
	}

	if err := s.manager.Store(ctx, collection, key, doc); err != nil {
		return nil, fmt.Errorf("store memory: %w", err)
	}

	return map[string]any{
		"success":    true,
		"key":        key,
		"collection": collection,
	}, nil
}

type searchInput struct {
	Query      string `json:"query"`
	Collection string `json:"collection"`
	Limit      int    `json:"limit"`
}

func (s *Skill) handleSearch(ctx context.Context, params map[string]any) (any, error) {
	data, _ := json.Marshal(params)
	var input searchInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	collection := input.Collection
	if collection == "" {
		collection = DefaultCollection
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 5
	}

	results, err := s.manager.Search(ctx, collection, input.Query, memory.SearchOptions{
		TopK:            limit,
		IncludeMetadata: true,
	})
	if err != nil {
		return nil, fmt.Errorf("search memory: %w", err)
	}

	output := make([]map[string]any, len(results))
	for i, r := range results {
		output[i] = map[string]any{
			"key":      r.Document.ID,
			"content":  r.Document.Content,
			"score":    r.Score,
			"metadata": r.Document.Metadata,
		}
	}

	return map[string]any{
		"results": output,
		"count":   len(results),
	}, nil
}

type listInput struct {
	Collection string `json:"collection"`
	Limit      int    `json:"limit"`
}

func (s *Skill) handleList(ctx context.Context, params map[string]any) (any, error) {
	data, _ := json.Marshal(params)
	var input listInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	collection := input.Collection
	if collection == "" {
		collection = DefaultCollection
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	docs, err := s.manager.List(ctx, collection, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}

	output := make([]map[string]any, len(docs))
	for i, d := range docs {
		output[i] = map[string]any{
			"key":        d.ID,
			"content":    truncate(d.Content, 100),
			"created_at": d.CreatedAt,
		}
	}

	return map[string]any{
		"memories": output,
		"count":    len(docs),
	}, nil
}

type deleteInput struct {
	Key        string `json:"key"`
	Collection string `json:"collection"`
}

func (s *Skill) handleDelete(ctx context.Context, params map[string]any) (any, error) {
	data, _ := json.Marshal(params)
	var input deleteInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	collection := input.Collection
	if collection == "" {
		collection = DefaultCollection
	}

	if err := s.manager.Delete(ctx, collection, input.Key); err != nil {
		return nil, fmt.Errorf("delete memory: %w", err)
	}

	return map[string]any{
		"success": true,
		"key":     input.Key,
	}, nil
}

func (s *Skill) handleListCollections(_ context.Context, _ map[string]any) (any, error) {
	collections := s.manager.ListCollections()
	return map[string]any{
		"collections": collections,
		"count":       len(collections),
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
