package operations

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// ListAgentsOutput is the response for listing agents.
type ListAgentsOutput struct {
	Body struct {
		Object string      `json:"object" example:"list" doc:"Object type"`
		Data   []AgentInfo `json:"data" doc:"List of agents"`
	}
}

// GetAgentInput is the request for getting an agent.
type GetAgentInput struct {
	ID string `path:"id" doc:"Agent ID"`
}

// AgentOutput is the response containing a single agent.
type AgentOutput struct {
	Body AgentInfo
}

// CreateAgentInput is the request for creating an agent.
type CreateAgentInput struct {
	Body CreateAgentRequest
}

// UpdateAgentInput is the request for updating an agent.
type UpdateAgentInput struct {
	ID   string `path:"id" doc:"Agent ID"`
	Body UpdateAgentRequest
}

// DeleteAgentInput is the request for deleting an agent.
type DeleteAgentInput struct {
	ID string `path:"id" doc:"Agent ID"`
}

// CloneAgentInput is the request for cloning an agent.
type CloneAgentInput struct {
	ID   string `path:"id" doc:"Source agent ID"`
	Body CloneAgentRequest
}

// ReloadAgentInput is the request for reloading an agent.
type ReloadAgentInput struct {
	ID string `path:"id" doc:"Agent ID"`
}

// ReloadAgentOutput is the response for reloading an agent.
type ReloadAgentOutput struct {
	Body struct {
		ID     string    `json:"id" doc:"Agent ID"`
		Status string    `json:"status" example:"reloaded" doc:"Reload status"`
		Agent  AgentInfo `json:"agent" doc:"Reloaded agent info"`
	}
}

// RegisterAgentOperations registers agent management API operations.
func RegisterAgentOperations(api huma.API, manager AgentManager) {
	if manager == nil {
		return
	}

	// GET /v1/agents - List all agents
	huma.Register(api, huma.Operation{
		OperationID:   "listAgents",
		Method:        http.MethodGet,
		Path:          "/v1/agents",
		Summary:       "List configured agents",
		Description:   "Returns a list of all configured agents.",
		Tags:          []string{"Agents"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, _ *struct{}) (*ListAgentsOutput, error) {
		agents, err := manager.ListAgents(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list agents", err)
		}
		out := &ListAgentsOutput{}
		out.Body.Object = "list"
		out.Body.Data = agents
		return out, nil
	})

	// POST /v1/agents - Create a new agent
	huma.Register(api, huma.Operation{
		OperationID:   "createAgent",
		Method:        http.MethodPost,
		Path:          "/v1/agents",
		Summary:       "Create a new agent",
		Description:   "Creates a new agent configuration.",
		Tags:          []string{"Agents"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, input *CreateAgentInput) (*AgentOutput, error) {
		if input.Body.Name == "" {
			return nil, huma.Error400BadRequest("name is required")
		}
		agent, err := manager.CreateAgent(ctx, &input.Body)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				return nil, huma.Error409Conflict("agent already exists", err)
			}
			return nil, huma.Error500InternalServerError("failed to create agent", err)
		}
		return &AgentOutput{Body: *agent}, nil
	})

	// GET /v1/agents/{id} - Get agent details
	huma.Register(api, huma.Operation{
		OperationID:   "getAgent",
		Method:        http.MethodGet,
		Path:          "/v1/agents/{id}",
		Summary:       "Get agent details",
		Description:   "Returns details for a specific agent.",
		Tags:          []string{"Agents"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *GetAgentInput) (*AgentOutput, error) {
		agent, err := manager.GetAgent(ctx, input.ID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("agent not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to get agent", err)
		}
		return &AgentOutput{Body: *agent}, nil
	})

	// PUT /v1/agents/{id} - Update an agent
	huma.Register(api, huma.Operation{
		OperationID:   "updateAgent",
		Method:        http.MethodPut,
		Path:          "/v1/agents/{id}",
		Summary:       "Update an agent",
		Description:   "Updates an existing agent configuration.",
		Tags:          []string{"Agents"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *UpdateAgentInput) (*AgentOutput, error) {
		agent, err := manager.UpdateAgent(ctx, input.ID, &input.Body)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("agent not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to update agent", err)
		}
		return &AgentOutput{Body: *agent}, nil
	})

	// DELETE /v1/agents/{id} - Delete an agent
	huma.Register(api, huma.Operation{
		OperationID:   "deleteAgent",
		Method:        http.MethodDelete,
		Path:          "/v1/agents/{id}",
		Summary:       "Delete an agent",
		Description:   "Deletes an agent configuration.",
		Tags:          []string{"Agents"},
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, input *DeleteAgentInput) (*struct{}, error) {
		if err := manager.DeleteAgent(ctx, input.ID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("agent not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to delete agent", err)
		}
		return nil, nil
	})

	// POST /v1/agents/{id}/clone - Clone an agent
	huma.Register(api, huma.Operation{
		OperationID:   "cloneAgent",
		Method:        http.MethodPost,
		Path:          "/v1/agents/{id}/clone",
		Summary:       "Clone an agent",
		Description:   "Creates a copy of an existing agent with a new name.",
		Tags:          []string{"Agents"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, input *CloneAgentInput) (*AgentOutput, error) {
		if input.Body.NewName == "" {
			return nil, huma.Error400BadRequest("new_name is required")
		}
		agent, err := manager.CloneAgent(ctx, input.ID, &input.Body)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("agent not found", err)
			}
			if strings.Contains(err.Error(), "already exists") {
				return nil, huma.Error409Conflict("agent already exists", err)
			}
			return nil, huma.Error500InternalServerError("failed to clone agent", err)
		}
		return &AgentOutput{Body: *agent}, nil
	})

	// POST /v1/agents/{id}/reload - Reload an agent
	huma.Register(api, huma.Operation{
		OperationID:   "reloadAgent",
		Method:        http.MethodPost,
		Path:          "/v1/agents/{id}/reload",
		Summary:       "Reload an agent",
		Description:   "Reloads an agent from its configuration.",
		Tags:          []string{"Agents"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *ReloadAgentInput) (*ReloadAgentOutput, error) {
		// Reload is implemented by fetching the current state
		agent, err := manager.GetAgent(ctx, input.ID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("agent not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to reload agent", err)
		}
		out := &ReloadAgentOutput{}
		out.Body.ID = input.ID
		out.Body.Status = "reloaded"
		out.Body.Agent = *agent
		return out, nil
	})
}
