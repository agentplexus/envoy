package operations

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// ListToolsOutput is the response for listing tools.
type ListToolsOutput struct {
	Body struct {
		Object string     `json:"object" example:"list" doc:"Object type"`
		Data   []ToolInfo `json:"data" doc:"List of tools"`
	}
}

// RegisterToolOperations registers tool-related API operations.
func RegisterToolOperations(api huma.API, handler Handler) {
	huma.Register(api, huma.Operation{
		OperationID:   "listTools",
		Method:        http.MethodGet,
		Path:          "/api/v1/tools",
		Summary:       "List available tools",
		Description:   "Returns a list of all tools registered with the agent.",
		Tags:          []string{"Tools"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, _ *struct{}) (*ListToolsOutput, error) {
		tools, err := handler.ListTools(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list tools", err)
		}
		out := &ListToolsOutput{}
		out.Body.Object = "list"
		out.Body.Data = tools
		return out, nil
	})
}
