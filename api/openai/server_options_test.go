package openai

import (
	"context"
	"log/slog"
	"testing"

	"github.com/plexusone/omniagent/api/openai/auth"
)

func TestWithOpenAIPrefix(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithOpenAIPrefix("/custom/v1"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	if srv.config.OpenAIPrefix != "/custom/v1" {
		t.Errorf("OpenAIPrefix = %q, want /custom/v1", srv.config.OpenAIPrefix)
	}
}

func TestWithAPIPrefix(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithAPIPrefix("/custom-api"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	if srv.config.APIPrefix != "/custom-api" {
		t.Errorf("APIPrefix = %q, want /custom-api", srv.config.APIPrefix)
	}
}

func TestWithBaseURL(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler, WithBaseURL("https://example.com"))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	if srv.config.BaseURL != "https://example.com" {
		t.Errorf("BaseURL = %q, want https://example.com", srv.config.BaseURL)
	}
}

func TestWithLogger(t *testing.T) {
	handler := newMockAgentHandler()
	logger := slog.Default()
	srv, err := New(handler, WithLogger(logger))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	if srv.config.Logger != logger {
		t.Error("Logger was not applied to config")
	}
}

func TestWithAuth_Disabled(t *testing.T) {
	handler := newMockAgentHandler()
	cfg := &auth.Config{Enabled: false}
	srv, err := New(handler, WithAuth(cfg))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	if srv.config.Auth != cfg {
		t.Error("Auth config was not applied")
	}
}

func TestWithAAuth_Disabled(t *testing.T) {
	handler := newMockAgentHandler()
	cfg := &auth.AAuthConfig{Enabled: false}
	srv, err := New(handler, WithAAuth(cfg))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	if srv.config.AAuth != cfg {
		t.Error("AAuth config was not applied")
	}
}

func TestWithAgentAuth_Disabled(t *testing.T) {
	handler := newMockAgentHandler()
	cfg := &auth.AgentAuthConfig{Enabled: false}
	srv, err := New(handler, WithAgentAuth(cfg))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	if srv.config.AgentAuth != cfg {
		t.Error("AgentAuth config was not applied")
	}
}

func TestWithImageHandler(t *testing.T) {
	handler := newMockAgentHandler()
	img := &stubImageHandler{}
	srv, err := New(handler, WithImageHandler(img))
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	if srv.config.ImageHandler != img {
		t.Error("ImageHandler was not applied to config")
	}
}

// stubImageHandler is a minimal ImageHandler for exercising WithImageHandler
// wiring; individual endpoints are not exercised here.
type stubImageHandler struct{}

func (s *stubImageHandler) CreateImage(ctx context.Context, req *CreateImageRequest) (*ImageResponse, error) {
	return &ImageResponse{}, nil
}

func (s *stubImageHandler) CreateImageEdit(ctx context.Context, req *CreateImageEditRequest) (*ImageResponse, error) {
	return &ImageResponse{}, nil
}

func (s *stubImageHandler) CreateImageVariation(ctx context.Context, req *CreateImageVariationRequest) (*ImageResponse, error) {
	return &ImageResponse{}, nil
}

func TestServer_Accessors(t *testing.T) {
	handler := newMockAgentHandler()
	srv, err := New(handler)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if srv.HumaAPI() == nil {
		t.Error("HumaAPI() returned nil")
	}
	if srv.Handler() == nil {
		t.Error("Handler() returned nil")
	}
	if srv.UsageStore() == nil {
		t.Error("UsageStore() returned nil")
	}
	if srv.ToolUsageStore() == nil {
		t.Error("ToolUsageStore() returned nil")
	}

	spec, err := srv.GetMergedSpec()
	if err != nil {
		t.Fatalf("GetMergedSpec failed: %v", err)
	}
	if spec == nil {
		t.Error("GetMergedSpec returned nil spec")
	}
}

func TestGetAAuthClaims_Empty(t *testing.T) {
	if claims := GetAAuthClaims(context.Background()); claims != nil {
		t.Errorf("expected nil claims from an empty context, got %+v", claims)
	}
}

func TestGetAgentAuthClaims_Empty(t *testing.T) {
	if claims := GetAgentAuthClaims(context.Background()); claims != nil {
		t.Errorf("expected nil claims from an empty context, got %+v", claims)
	}
}

func TestUnauthorizedError(t *testing.T) {
	if ErrUnauthorized.Error() != "unauthorized" {
		t.Errorf("Error() = %q, want unauthorized", ErrUnauthorized.Error())
	}
}
