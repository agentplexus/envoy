package operations

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// ListMemoriesInput is the request for listing memories.
type ListMemoriesInput struct {
	Collection string `query:"collection" default:"default" doc:"Collection name"`
	Limit      int    `query:"limit" default:"50" doc:"Maximum number of memories to return"`
}

// ListMemoriesOutput is the response for listing memories.
type ListMemoriesOutput struct {
	Body struct {
		Object     string         `json:"object" example:"list" doc:"Object type"`
		Collection string         `json:"collection" doc:"Collection name"`
		Data       []MemoryRecord `json:"data" doc:"List of memories"`
	}
}

// SearchMemoriesInput is the request for searching memories.
type SearchMemoriesInput struct {
	Query      string `query:"q" required:"true" doc:"Search query"`
	Collection string `query:"collection" default:"default" doc:"Collection name"`
	Limit      int    `query:"limit" default:"10" doc:"Maximum number of results"`
}

// SearchMemoriesOutput is the response for searching memories.
type SearchMemoriesOutput struct {
	Body struct {
		Object     string               `json:"object" example:"list" doc:"Object type"`
		Collection string               `json:"collection" doc:"Collection name"`
		Query      string               `json:"query" doc:"Search query"`
		Data       []MemorySearchResult `json:"data" doc:"Search results"`
	}
}

// StoreMemoryInput is the request for storing a memory.
type StoreMemoryInput struct {
	Body StoreMemoryRequest
}

// MemoryOutput is the response for a single memory.
type MemoryOutput struct {
	Body MemoryRecord
}

// DeleteMemoryInput is the request for deleting a memory.
type DeleteMemoryInput struct {
	Key        string `path:"key" doc:"Memory key"`
	Collection string `query:"collection" default:"default" doc:"Collection name"`
}

// ListCollectionsOutput is the response for listing collections.
type ListCollectionsOutput struct {
	Body struct {
		Object string             `json:"object" example:"list" doc:"Object type"`
		Data   []MemoryCollection `json:"data" doc:"List of collections"`
	}
}

// RegisterMemoryOperations registers memory API operations.
func RegisterMemoryOperations(api huma.API, handler MemoryHandler) {
	if handler == nil {
		return
	}

	// GET /v1/memories - List memories
	huma.Register(api, huma.Operation{
		OperationID:   "listMemories",
		Method:        http.MethodGet,
		Path:          "/v1/memories",
		Summary:       "List memories",
		Description:   "Returns memories from a collection.",
		Tags:          []string{"Memory"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *ListMemoriesInput) (*ListMemoriesOutput, error) {
		collection := input.Collection
		if collection == "" {
			collection = "default"
		}
		limit := input.Limit
		if limit <= 0 {
			limit = 50
		}
		memories, err := handler.ListMemories(ctx, collection, limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list memories", err)
		}
		out := &ListMemoriesOutput{}
		out.Body.Object = "list"
		out.Body.Collection = collection
		out.Body.Data = memories
		return out, nil
	})

	// POST /v1/memories - Store a memory
	huma.Register(api, huma.Operation{
		OperationID:   "storeMemory",
		Method:        http.MethodPost,
		Path:          "/v1/memories",
		Summary:       "Store a memory",
		Description:   "Stores a new memory in a collection.",
		Tags:          []string{"Memory"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, input *StoreMemoryInput) (*MemoryOutput, error) {
		if input.Body.Content == "" {
			return nil, huma.Error400BadRequest("content is required")
		}
		memory, err := handler.StoreMemory(ctx, &input.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to store memory", err)
		}
		return &MemoryOutput{Body: *memory}, nil
	})

	// GET /v1/memories/search - Search memories
	huma.Register(api, huma.Operation{
		OperationID:   "searchMemories",
		Method:        http.MethodGet,
		Path:          "/v1/memories/search",
		Summary:       "Search memories",
		Description:   "Performs semantic search on memories.",
		Tags:          []string{"Memory"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *SearchMemoriesInput) (*SearchMemoriesOutput, error) {
		if input.Query == "" {
			return nil, huma.Error400BadRequest("query parameter 'q' is required")
		}
		collection := input.Collection
		if collection == "" {
			collection = "default"
		}
		limit := input.Limit
		if limit <= 0 {
			limit = 10
		}
		results, err := handler.SearchMemories(ctx, collection, input.Query, limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to search memories", err)
		}
		out := &SearchMemoriesOutput{}
		out.Body.Object = "list"
		out.Body.Collection = collection
		out.Body.Query = input.Query
		out.Body.Data = results
		return out, nil
	})

	// DELETE /v1/memories/{key} - Delete a memory
	huma.Register(api, huma.Operation{
		OperationID:   "deleteMemory",
		Method:        http.MethodDelete,
		Path:          "/v1/memories/{key}",
		Summary:       "Delete a memory",
		Description:   "Deletes a memory by key.",
		Tags:          []string{"Memory"},
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, input *DeleteMemoryInput) (*struct{}, error) {
		collection := input.Collection
		if collection == "" {
			collection = "default"
		}
		if err := handler.DeleteMemory(ctx, collection, input.Key); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("memory not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to delete memory", err)
		}
		return nil, nil
	})

	// GET /v1/memories/collections - List collections
	huma.Register(api, huma.Operation{
		OperationID:   "listCollections",
		Method:        http.MethodGet,
		Path:          "/v1/memories/collections",
		Summary:       "List collections",
		Description:   "Returns all memory collections.",
		Tags:          []string{"Memory"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, _ *struct{}) (*ListCollectionsOutput, error) {
		collections, err := handler.ListCollections(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list collections", err)
		}
		out := &ListCollectionsOutput{}
		out.Body.Object = "list"
		out.Body.Data = collections
		return out, nil
	})
}
