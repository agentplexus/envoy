package operations

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// HealthOutput is the response for health check.
type HealthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok" doc:"Health status"`
	}
}

// RegisterHealthOperation registers the health check endpoint.
func RegisterHealthOperation(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "healthCheck",
		Method:        http.MethodGet,
		Path:          "/health",
		Summary:       "Health check",
		Description:   "Returns the health status of the API server.",
		Tags:          []string{"System"},
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, _ *struct{}) (*HealthOutput, error) {
		out := &HealthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}
