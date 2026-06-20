package operations

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// ListCronJobsOutput is the response for listing cron jobs.
type ListCronJobsOutput struct {
	Body struct {
		Object string        `json:"object" example:"list" doc:"Object type"`
		Data   []CronJobInfo `json:"data" doc:"List of cron jobs"`
	}
}

// GetCronJobInput is the request for getting a cron job.
type GetCronJobInput struct {
	ID string `path:"id" doc:"Cron job ID"`
}

// CronJobOutput is the response containing a single cron job.
type CronJobOutput struct {
	Body CronJobInfo
}

// CreateCronJobInput is the request for creating a cron job.
type CreateCronJobInput struct {
	Body CreateCronJobRequest
}

// UpdateCronJobInput is the request for updating a cron job.
type UpdateCronJobInput struct {
	ID   string `path:"id" doc:"Cron job ID"`
	Body UpdateCronJobRequest
}

// DeleteCronJobInput is the request for deleting a cron job.
type DeleteCronJobInput struct {
	ID string `path:"id" doc:"Cron job ID"`
}

// TriggerCronJobInput is the request for triggering a cron job.
type TriggerCronJobInput struct {
	ID string `path:"id" doc:"Cron job ID"`
}

// TriggerCronJobOutput is the response for triggering a cron job.
type TriggerCronJobOutput struct {
	Body CronJobResult
}

// EnableDisableCronJobInput is the request for enabling/disabling a cron job.
type EnableDisableCronJobInput struct {
	ID string `path:"id" doc:"Cron job ID"`
}

// EnableDisableCronJobOutput is the response for enabling/disabling a cron job.
type EnableDisableCronJobOutput struct {
	Body struct {
		ID     string `json:"id" doc:"Cron job ID"`
		Status string `json:"status" doc:"New status (enabled, disabled)"`
	}
}

// RegisterCronOperations registers cron job API operations.
func RegisterCronOperations(api huma.API, handler Handler) {
	// GET /api/v1/cron/jobs - List all cron jobs
	huma.Register(api, huma.Operation{
		OperationID:   "listCronJobs",
		Method:        http.MethodGet,
		Path:          "/api/v1/cron/jobs",
		Summary:       "List scheduled jobs",
		Description:   "Returns a list of all scheduled cron jobs.",
		Tags:          []string{"Cron"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, _ *struct{}) (*ListCronJobsOutput, error) {
		jobs, err := handler.ListCronJobs(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list cron jobs", err)
		}
		out := &ListCronJobsOutput{}
		out.Body.Object = "list"
		out.Body.Data = jobs
		return out, nil
	})

	// POST /api/v1/cron/jobs - Create a new cron job
	huma.Register(api, huma.Operation{
		OperationID:   "createCronJob",
		Method:        http.MethodPost,
		Path:          "/api/v1/cron/jobs",
		Summary:       "Create a scheduled job",
		Description:   "Creates a new scheduled cron job.",
		Tags:          []string{"Cron"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, input *CreateCronJobInput) (*CronJobOutput, error) {
		job, err := handler.CreateCronJob(ctx, &input.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to create cron job", err)
		}
		return &CronJobOutput{Body: *job}, nil
	})

	// GET /api/v1/cron/jobs/{id} - Get cron job details
	huma.Register(api, huma.Operation{
		OperationID:   "getCronJob",
		Method:        http.MethodGet,
		Path:          "/api/v1/cron/jobs/{id}",
		Summary:       "Get job details",
		Description:   "Returns details for a specific cron job.",
		Tags:          []string{"Cron"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *GetCronJobInput) (*CronJobOutput, error) {
		job, err := handler.GetCronJob(ctx, input.ID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("cron job not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to get cron job", err)
		}
		return &CronJobOutput{Body: *job}, nil
	})

	// PUT /api/v1/cron/jobs/{id} - Update a cron job
	huma.Register(api, huma.Operation{
		OperationID:   "updateCronJob",
		Method:        http.MethodPut,
		Path:          "/api/v1/cron/jobs/{id}",
		Summary:       "Update a job",
		Description:   "Updates an existing cron job.",
		Tags:          []string{"Cron"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *UpdateCronJobInput) (*CronJobOutput, error) {
		job, err := handler.UpdateCronJob(ctx, input.ID, &input.Body)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("cron job not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to update cron job", err)
		}
		return &CronJobOutput{Body: *job}, nil
	})

	// DELETE /api/v1/cron/jobs/{id} - Delete a cron job
	huma.Register(api, huma.Operation{
		OperationID:   "deleteCronJob",
		Method:        http.MethodDelete,
		Path:          "/api/v1/cron/jobs/{id}",
		Summary:       "Delete a job",
		Description:   "Deletes a cron job.",
		Tags:          []string{"Cron"},
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, input *DeleteCronJobInput) (*struct{}, error) {
		if err := handler.DeleteCronJob(ctx, input.ID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("cron job not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to delete cron job", err)
		}
		return nil, nil
	})

	// POST /api/v1/cron/jobs/{id}/trigger - Trigger a cron job
	huma.Register(api, huma.Operation{
		OperationID:   "triggerCronJob",
		Method:        http.MethodPost,
		Path:          "/api/v1/cron/jobs/{id}/trigger",
		Summary:       "Trigger a job",
		Description:   "Runs a cron job immediately.",
		Tags:          []string{"Cron"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *TriggerCronJobInput) (*TriggerCronJobOutput, error) {
		result, err := handler.TriggerCronJob(ctx, input.ID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("cron job not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to trigger cron job", err)
		}
		return &TriggerCronJobOutput{Body: *result}, nil
	})

	// POST /api/v1/cron/jobs/{id}/enable - Enable a cron job
	huma.Register(api, huma.Operation{
		OperationID:   "enableCronJob",
		Method:        http.MethodPost,
		Path:          "/api/v1/cron/jobs/{id}/enable",
		Summary:       "Enable a job",
		Description:   "Enables a disabled cron job.",
		Tags:          []string{"Cron"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *EnableDisableCronJobInput) (*EnableDisableCronJobOutput, error) {
		if err := handler.EnableCronJob(ctx, input.ID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("cron job not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to enable cron job", err)
		}
		out := &EnableDisableCronJobOutput{}
		out.Body.ID = input.ID
		out.Body.Status = "enabled"
		return out, nil
	})

	// POST /api/v1/cron/jobs/{id}/disable - Disable a cron job
	huma.Register(api, huma.Operation{
		OperationID:   "disableCronJob",
		Method:        http.MethodPost,
		Path:          "/api/v1/cron/jobs/{id}/disable",
		Summary:       "Disable a job",
		Description:   "Disables a cron job without deleting it.",
		Tags:          []string{"Cron"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *EnableDisableCronJobInput) (*EnableDisableCronJobOutput, error) {
		if err := handler.DisableCronJob(ctx, input.ID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("cron job not found", err)
			}
			return nil, huma.Error500InternalServerError("failed to disable cron job", err)
		}
		out := &EnableDisableCronJobOutput{}
		out.Body.ID = input.ID
		out.Body.Status = "disabled"
		return out, nil
	})
}
