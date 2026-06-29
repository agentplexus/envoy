package operations

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/plexusone/omniagent/internal/version"
)

// StatusOutput is the response for the status endpoint.
type StatusOutput struct {
	Body struct {
		Status  string      `json:"status" example:"ok" doc:"Server status"`
		Version VersionInfo `json:"version" doc:"Version information"`
	}
}

// VersionInfo contains version details.
type VersionInfo struct {
	Version   string `json:"version" example:"0.12.0" doc:"OmniAgent version"`
	Commit    string `json:"commit" example:"abc1234" doc:"Git commit hash"`
	BuildDate string `json:"build_date" example:"2026-06-29" doc:"Build date"`
	GoVersion string `json:"go_version" example:"go1.26.4" doc:"Go version used to build"`
	Platform  string `json:"platform" example:"darwin/arm64" doc:"Build platform"`
}

// RegisterStatusOperation registers the status endpoint.
func RegisterStatusOperation(api huma.API, prefix string) {
	huma.Register(api, huma.Operation{
		OperationID:   "getStatus",
		Method:        http.MethodGet,
		Path:          prefix + "/status",
		Summary:       "Get server status",
		Description:   "Returns detailed server status including version information.",
		Tags:          []string{"System"},
		DefaultStatus: http.StatusOK,
	}, func(_ context.Context, _ *struct{}) (*StatusOutput, error) {
		info := version.Get()
		out := &StatusOutput{}
		out.Body.Status = "ok"
		out.Body.Version = VersionInfo{
			Version:   info.Version,
			Commit:    info.Commit,
			BuildDate: info.BuildDate,
			GoVersion: info.GoVersion,
			Platform:  info.Platform,
		}
		return out, nil
	})
}
