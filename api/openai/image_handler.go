package openai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/plexusone/omniagent/config"
	"github.com/plexusone/omniimage"
	"github.com/plexusone/omniimage/provider"
)

// ImageHandler defines the interface for image generation operations.
type ImageHandler interface {
	// CreateImage generates images from a text prompt.
	CreateImage(ctx context.Context, req *CreateImageRequest) (*ImageResponse, error)

	// CreateImageEdit modifies an existing image based on a prompt.
	CreateImageEdit(ctx context.Context, req *CreateImageEditRequest) (*ImageResponse, error)

	// CreateImageVariation creates variations of an existing image.
	CreateImageVariation(ctx context.Context, req *CreateImageVariationRequest) (*ImageResponse, error)
}

// CreateImageRequest represents a request to generate images.
type CreateImageRequest struct {
	Model          string `json:"model,omitempty"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	Style          string `json:"style,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	User           string `json:"user,omitempty"`
}

// CreateImageEditRequest represents a request to edit an image.
type CreateImageEditRequest struct {
	Model          string `json:"model,omitempty"`
	Image          string `json:"image"`
	Mask           string `json:"mask,omitempty"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	User           string `json:"user,omitempty"`
}

// CreateImageVariationRequest represents a request to create image variations.
type CreateImageVariationRequest struct {
	Model          string `json:"model,omitempty"`
	Image          string `json:"image"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	User           string `json:"user,omitempty"`
}

// ImageResponse represents the response from image generation.
type ImageResponse struct {
	Created int64         `json:"created"`
	Data    []ImageObject `json:"data"`
}

// ImageObject represents a generated image.
type ImageObject struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// OmniImageHandler implements ImageHandler using the OmniImage library.
type OmniImageHandler struct {
	client       *omniimage.Client
	defaultModel string
	logger       *slog.Logger
}

// NewOmniImageHandler creates a new ImageHandler using OmniImage.
func NewOmniImageHandler(cfg config.ImageConfig, logger *slog.Logger) (*OmniImageHandler, error) {
	if logger == nil {
		logger = slog.Default()
	}

	providerName := omniimage.ProviderName(cfg.Provider)
	if providerName == "" {
		providerName = omniimage.ProviderNameOpenAI
	}

	client, err := omniimage.NewClient(omniimage.ClientConfig{
		Providers: []omniimage.ProviderConfig{
			{
				Provider: providerName,
				APIKey:   cfg.APIKey,
				BaseURL:  cfg.BaseURL,
			},
		},
		Logger: logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create omniimage client: %w", err)
	}

	return &OmniImageHandler{
		client:       client,
		defaultModel: cfg.Model,
		logger:       logger,
	}, nil
}

// CreateImage generates images from a text prompt.
func (h *OmniImageHandler) CreateImage(ctx context.Context, req *CreateImageRequest) (*ImageResponse, error) {
	model := req.Model
	if model == "" {
		model = h.defaultModel
	}

	n := req.N
	if n == 0 {
		n = 1
	}

	genReq := &provider.GenerateRequest{
		Model:          model,
		Prompt:         req.Prompt,
		N:              n,
		Size:           provider.ImageSize(req.Size),
		Quality:        provider.ImageQuality(req.Quality),
		Style:          provider.ImageStyle(req.Style),
		ResponseFormat: provider.ResponseFormat(req.ResponseFormat),
		User:           req.User,
	}

	resp, err := h.client.Generate(ctx, genReq)
	if err != nil {
		return nil, convertOmniImageError(err)
	}

	return convertGenerateResponse(resp), nil
}

// CreateImageEdit modifies an existing image based on a prompt.
func (h *OmniImageHandler) CreateImageEdit(ctx context.Context, req *CreateImageEditRequest) (*ImageResponse, error) {
	model := req.Model
	if model == "" {
		model = h.defaultModel
	}

	n := req.N
	if n == 0 {
		n = 1
	}

	editReq := &provider.EditRequest{
		Model:          model,
		Image:          req.Image,
		Mask:           req.Mask,
		Prompt:         req.Prompt,
		N:              n,
		Size:           provider.ImageSize(req.Size),
		ResponseFormat: provider.ResponseFormat(req.ResponseFormat),
		User:           req.User,
	}

	resp, err := h.client.Edit(ctx, editReq)
	if err != nil {
		return nil, convertOmniImageError(err)
	}

	return convertEditResponse(resp), nil
}

// CreateImageVariation creates variations of an existing image.
func (h *OmniImageHandler) CreateImageVariation(ctx context.Context, req *CreateImageVariationRequest) (*ImageResponse, error) {
	model := req.Model
	if model == "" {
		model = h.defaultModel
	}

	n := req.N
	if n == 0 {
		n = 1
	}

	varReq := &provider.VariationsRequest{
		Model:          model,
		Image:          req.Image,
		N:              n,
		Size:           provider.ImageSize(req.Size),
		ResponseFormat: provider.ResponseFormat(req.ResponseFormat),
		User:           req.User,
	}

	resp, err := h.client.Variations(ctx, varReq)
	if err != nil {
		return nil, convertOmniImageError(err)
	}

	return convertVariationsResponse(resp), nil
}

// Close closes the underlying OmniImage client.
func (h *OmniImageHandler) Close() error {
	return h.client.Close()
}

// convertGenerateResponse converts an OmniImage generate response to the OpenAI format.
func convertGenerateResponse(resp *provider.GenerateResponse) *ImageResponse {
	data := make([]ImageObject, len(resp.Images))
	for i, img := range resp.Images {
		data[i] = ImageObject{
			URL:           img.URL,
			B64JSON:       img.B64JSON,
			RevisedPrompt: img.RevisedPrompt,
		}
	}
	return &ImageResponse{
		Created: resp.Created.Unix(),
		Data:    data,
	}
}

// convertEditResponse converts an OmniImage edit response to the OpenAI format.
func convertEditResponse(resp *provider.EditResponse) *ImageResponse {
	data := make([]ImageObject, len(resp.Images))
	for i, img := range resp.Images {
		data[i] = ImageObject{
			URL:           img.URL,
			B64JSON:       img.B64JSON,
			RevisedPrompt: img.RevisedPrompt,
		}
	}
	return &ImageResponse{
		Created: resp.Created.Unix(),
		Data:    data,
	}
}

// convertVariationsResponse converts an OmniImage variations response to the OpenAI format.
func convertVariationsResponse(resp *provider.VariationsResponse) *ImageResponse {
	data := make([]ImageObject, len(resp.Images))
	for i, img := range resp.Images {
		data[i] = ImageObject{
			URL:           img.URL,
			B64JSON:       img.B64JSON,
			RevisedPrompt: img.RevisedPrompt,
		}
	}
	return &ImageResponse{
		Created: resp.Created.Unix(),
		Data:    data,
	}
}

// ImageError represents an OpenAI-compatible image API error.
type ImageError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *ImageError) Error() string {
	return e.Message
}

// convertOmniImageError converts OmniImage errors to OpenAI-compatible errors.
func convertOmniImageError(err error) error {
	if err == nil {
		return nil
	}

	// Check for specific OmniImage errors
	switch {
	case errors.Is(err, omniimage.ErrRateLimited):
		return &ImageError{
			StatusCode: 429,
			Code:       "rate_limit_exceeded",
			Message:    "Rate limit exceeded. Please try again later.",
		}
	case errors.Is(err, omniimage.ErrContentPolicy):
		return &ImageError{
			StatusCode: 400,
			Code:       "content_policy_violation",
			Message:    "Your request was rejected due to content policy violations.",
		}
	case errors.Is(err, omniimage.ErrInvalidRequest):
		return &ImageError{
			StatusCode: 400,
			Code:       "invalid_request_error",
			Message:    err.Error(),
		}
	case errors.Is(err, omniimage.ErrNotSupported):
		return &ImageError{
			StatusCode: 400,
			Code:       "invalid_request_error",
			Message:    "This operation is not supported by the current provider.",
		}
	case errors.Is(err, omniimage.ErrModelNotFound):
		return &ImageError{
			StatusCode: 404,
			Code:       "model_not_found",
			Message:    err.Error(),
		}
	}

	// Check for API errors
	var apiErr *omniimage.APIError
	if errors.As(err, &apiErr) {
		return &ImageError{
			StatusCode: apiErr.StatusCode,
			Code:       apiErr.Code,
			Message:    apiErr.Message,
		}
	}

	// Return generic error
	return &ImageError{
		StatusCode: 500,
		Code:       "internal_error",
		Message:    err.Error(),
	}
}

// LoadImageConfigFromEnv loads image configuration from environment variables.
func LoadImageConfigFromEnv() *config.ImageConfig {
	return loadImageConfigFromEnv()
}

// loadImageConfigFromEnv is the internal implementation.
func loadImageConfigFromEnv() *config.ImageConfig {
	cfg := &config.ImageConfig{}

	// Check if image generation is enabled
	if enabled := getEnv("IMAGE_ENABLED"); enabled == "true" || enabled == "1" {
		cfg.Enabled = true
	}

	// Get provider
	if provider := getEnv("IMAGE_PROVIDER"); provider != "" {
		cfg.Provider = provider
	} else {
		cfg.Provider = "openai"
	}

	// Get default model
	if model := getEnv("IMAGE_MODEL"); model != "" {
		cfg.Model = model
	}

	// Get API key (provider-specific fallback)
	if apiKey := getEnv("IMAGE_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
	} else {
		// Fall back to provider-specific keys
		switch cfg.Provider {
		case "openai":
			cfg.APIKey = getEnv("OPENAI_API_KEY")
		case "fal":
			cfg.APIKey = getEnv("FAL_KEY")
		}
	}

	// Get base URL
	if baseURL := getEnv("IMAGE_BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}

	// Auto-enable if API key is present
	if cfg.APIKey != "" && !cfg.Enabled {
		// Only auto-enable if explicitly set or has key
		if getEnv("IMAGE_ENABLED") == "" && getEnv("IMAGE_PROVIDER") != "" {
			cfg.Enabled = true
		}
	}

	return cfg
}

// getEnv is a helper to get environment variables.
func getEnv(key string) string {
	return os.Getenv(key)
}

// Ensure time import is used for response timestamps.
var _ = time.Now
