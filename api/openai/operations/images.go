package operations

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// ImageHandler defines the interface for image generation operations.
type ImageHandler interface {
	CreateImage(ctx context.Context, req *CreateImageRequest) (*ImageResponse, error)
	CreateImageEdit(ctx context.Context, req *CreateImageEditRequest) (*ImageResponse, error)
	CreateImageVariation(ctx context.Context, req *CreateImageVariationRequest) (*ImageResponse, error)
}

// CreateImageRequest represents a request to generate images.
type CreateImageRequest struct {
	Model          string `json:"model,omitempty" doc:"Model to use for generation"`
	Prompt         string `json:"prompt" doc:"Text description of the desired image(s)" required:"true"`
	N              int    `json:"n,omitempty" doc:"Number of images to generate (default: 1)"`
	Size           string `json:"size,omitempty" doc:"Image size (e.g., 1024x1024, 512x512)"`
	Quality        string `json:"quality,omitempty" doc:"Quality level (standard or hd)"`
	Style          string `json:"style,omitempty" doc:"Visual style (vivid or natural)"`
	ResponseFormat string `json:"response_format,omitempty" doc:"Response format (url or b64_json)"`
	User           string `json:"user,omitempty" doc:"Unique identifier for the end-user"`
}

// CreateImageEditRequest represents a request to edit an image.
type CreateImageEditRequest struct {
	Model          string `json:"model,omitempty" doc:"Model to use for editing"`
	Image          string `json:"image" doc:"Source image (base64 or URL)" required:"true"`
	Mask           string `json:"mask,omitempty" doc:"Mask image indicating areas to edit (base64 or URL)"`
	Prompt         string `json:"prompt" doc:"Description of the desired edit" required:"true"`
	N              int    `json:"n,omitempty" doc:"Number of edited images to generate (default: 1)"`
	Size           string `json:"size,omitempty" doc:"Output image size"`
	ResponseFormat string `json:"response_format,omitempty" doc:"Response format (url or b64_json)"`
	User           string `json:"user,omitempty" doc:"Unique identifier for the end-user"`
}

// CreateImageVariationRequest represents a request to create image variations.
type CreateImageVariationRequest struct {
	Model          string `json:"model,omitempty" doc:"Model to use for variations"`
	Image          string `json:"image" doc:"Source image (base64 or URL)" required:"true"`
	N              int    `json:"n,omitempty" doc:"Number of variations to generate (default: 1)"`
	Size           string `json:"size,omitempty" doc:"Output image size"`
	ResponseFormat string `json:"response_format,omitempty" doc:"Response format (url or b64_json)"`
	User           string `json:"user,omitempty" doc:"Unique identifier for the end-user"`
}

// ImageResponse represents the response from image generation.
type ImageResponse struct {
	Created int64         `json:"created" doc:"Timestamp when response was created"`
	Data    []ImageObject `json:"data" doc:"Generated image(s)"`
}

// ImageObject represents a generated image.
type ImageObject struct {
	URL           string `json:"url,omitempty" doc:"URL to the generated image"`
	B64JSON       string `json:"b64_json,omitempty" doc:"Base64-encoded image data"`
	RevisedPrompt string `json:"revised_prompt,omitempty" doc:"Revised prompt after model processing"`
}

// CreateImageInput is the Huma input for image generation.
type CreateImageInput struct {
	Body CreateImageRequest
}

// CreateImageEditInput is the Huma input for image editing.
type CreateImageEditInput struct {
	Body CreateImageEditRequest
}

// CreateImageVariationInput is the Huma input for image variations.
type CreateImageVariationInput struct {
	Body CreateImageVariationRequest
}

// CreateImageOutput is the Huma output for image operations.
type CreateImageOutput struct {
	Body ImageResponse
}

// ImageError represents an image API error.
type ImageError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *ImageError) Error() string {
	return e.Message
}

// RegisterImageOperations registers image-related API operations.
func RegisterImageOperations(api huma.API, handler ImageHandler, prefix string) {
	// POST /openai/v1/images/generations
	huma.Register(api, huma.Operation{
		OperationID:   "createImage",
		Method:        http.MethodPost,
		Path:          prefix + "/images/generations",
		Summary:       "Create image",
		Description:   "Creates an image given a prompt.",
		Tags:          []string{"Images"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *CreateImageInput) (*CreateImageOutput, error) {
		resp, err := handler.CreateImage(ctx, &input.Body)
		if err != nil {
			return nil, convertImageError(err)
		}
		return &CreateImageOutput{
			Body: ImageResponse{
				Created: resp.Created,
				Data:    convertImageObjects(resp.Data),
			},
		}, nil
	})

	// POST /openai/v1/images/edits
	huma.Register(api, huma.Operation{
		OperationID:   "createImageEdit",
		Method:        http.MethodPost,
		Path:          prefix + "/images/edits",
		Summary:       "Edit image",
		Description:   "Creates an edited or extended image given an original image and a prompt.",
		Tags:          []string{"Images"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *CreateImageEditInput) (*CreateImageOutput, error) {
		resp, err := handler.CreateImageEdit(ctx, &input.Body)
		if err != nil {
			return nil, convertImageError(err)
		}
		return &CreateImageOutput{
			Body: ImageResponse{
				Created: resp.Created,
				Data:    convertImageObjects(resp.Data),
			},
		}, nil
	})

	// POST /openai/v1/images/variations
	huma.Register(api, huma.Operation{
		OperationID:   "createImageVariation",
		Method:        http.MethodPost,
		Path:          prefix + "/images/variations",
		Summary:       "Create image variation",
		Description:   "Creates a variation of a given image.",
		Tags:          []string{"Images"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *CreateImageVariationInput) (*CreateImageOutput, error) {
		resp, err := handler.CreateImageVariation(ctx, &input.Body)
		if err != nil {
			return nil, convertImageError(err)
		}
		return &CreateImageOutput{
			Body: ImageResponse{
				Created: resp.Created,
				Data:    convertImageObjects(resp.Data),
			},
		}, nil
	})
}

// convertImageObjects converts internal ImageObject to operations ImageObject.
func convertImageObjects(data []ImageObject) []ImageObject {
	// Since the types are identical, we can return directly
	return data
}

// convertImageError converts image errors to Huma errors.
func convertImageError(err error) error {
	var imgErr *ImageError
	if errors.As(err, &imgErr) {
		switch imgErr.StatusCode {
		case 400:
			return huma.Error400BadRequest(imgErr.Message, err)
		case 401:
			return huma.Error401Unauthorized(imgErr.Message)
		case 404:
			return huma.Error404NotFound(imgErr.Message)
		case 429:
			return huma.Error429TooManyRequests(imgErr.Message)
		default:
			return huma.Error500InternalServerError(imgErr.Message, err)
		}
	}
	return huma.Error500InternalServerError("image generation failed", err)
}
