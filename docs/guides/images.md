# Image Generation

OmniAgent supports AI image generation through OpenAI-compatible API endpoints, powered by the [OmniImage](https://github.com/plexusone/omniimage) library.

## Overview

The image generation feature provides:

- **OpenAI-compatible endpoints** - Works with existing OpenAI client libraries
- **Multiple providers** - OpenAI (DALL-E) and Fal AI (FLUX, Stable Diffusion)
- **Full API support** - Generate, edit, and create variations of images

## Quick Start

### Environment Variables

```bash
# Enable image generation
export IMAGE_ENABLED=true

# For OpenAI (DALL-E)
export IMAGE_PROVIDER=openai
export IMAGE_MODEL=gpt-image-2
export OPENAI_API_KEY=sk-...

# Or for Fal AI (FLUX)
export IMAGE_PROVIDER=fal
export IMAGE_MODEL=fal-ai/flux-pro
export FAL_KEY=...
```

### Start the Server

```bash
omniagent openai serve --address :8080
```

### Generate an Image

```bash
curl -X POST http://localhost:8080/openai/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "A serene mountain landscape at sunset",
    "n": 1,
    "size": "1024x1024"
  }'
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `IMAGE_ENABLED` | Enable image generation | `false` |
| `IMAGE_PROVIDER` | Provider: `openai` or `fal` | `openai` |
| `IMAGE_MODEL` | Default model | - |
| `IMAGE_API_KEY` | API key (overrides provider-specific keys) | - |
| `IMAGE_BASE_URL` | Custom API base URL | - |

Provider-specific API keys are used as fallback:

- `OPENAI_API_KEY` for OpenAI
- `FAL_KEY` for Fal AI

### Config File

```yaml
# omniagent.yaml
image:
  enabled: true
  provider: openai
  model: gpt-image-2
  api_key: ${OPENAI_API_KEY}
  base_url: ""  # Optional custom endpoint
```

## API Endpoints

### Generate Images

`POST /openai/v1/images/generations`

Creates images from a text prompt.

**Request:**

```json
{
  "model": "gpt-image-2",
  "prompt": "A white cat sitting on a windowsill",
  "n": 1,
  "size": "1024x1024",
  "quality": "standard",
  "style": "natural",
  "response_format": "url"
}
```

**Parameters:**

| Field | Type | Description |
|-------|------|-------------|
| `model` | string | Model to use (optional, uses default) |
| `prompt` | string | Text description of desired image (required) |
| `n` | int | Number of images (default: 1) |
| `size` | string | Image size (e.g., `1024x1024`, `512x512`) |
| `quality` | string | `standard` or `hd` (DALL-E 3 only) |
| `style` | string | `vivid` or `natural` (DALL-E 3 only) |
| `response_format` | string | `url` or `b64_json` |
| `user` | string | User identifier for tracking |

**Response:**

```json
{
  "created": 1719360000,
  "data": [
    {
      "url": "https://oaidalleapiprodscus.blob.core.windows.net/...",
      "revised_prompt": "A fluffy white cat peacefully sitting..."
    }
  ]
}
```

### Edit Images

`POST /openai/v1/images/edits`

Modifies an existing image based on a prompt. Requires a mask to indicate which areas to edit.

**Request:**

```json
{
  "model": "dall-e-2",
  "image": "base64-encoded-image-or-url",
  "mask": "base64-encoded-mask-or-url",
  "prompt": "Add a red hat to the cat",
  "n": 1,
  "size": "1024x1024"
}
```

**Parameters:**

| Field | Type | Description |
|-------|------|-------------|
| `image` | string | Source image (base64 or URL) |
| `mask` | string | Mask image (transparent = edit area) |
| `prompt` | string | Description of the edit |

!!! note "Provider Support"
    Image editing is supported by OpenAI (DALL-E 2) but not by Fal AI.

### Create Variations

`POST /openai/v1/images/variations`

Creates variations of an existing image.

**Request:**

```json
{
  "model": "dall-e-2",
  "image": "base64-encoded-image-or-url",
  "n": 2,
  "size": "1024x1024"
}
```

!!! note "Provider Support"
    Image variations are supported by OpenAI (DALL-E 2) but not by Fal AI.

## Providers

### OpenAI (DALL-E)

OpenAI provides DALL-E 2 and DALL-E 3 models.

| Model | Features | Max Images |
|-------|----------|------------|
| `dall-e-3` | High quality, `hd` quality, `vivid`/`natural` style | 1 |
| `dall-e-2` | Edit, variations, multiple images | 10 |
| `gpt-image-2` | Alias for DALL-E 2 | 10 |

**Supported Sizes:**

- `256x256` (DALL-E 2 only)
- `512x512` (DALL-E 2 only)
- `1024x1024`
- `1024x1792` (DALL-E 3 only)
- `1792x1024` (DALL-E 3 only)

### Fal AI

Fal AI provides FLUX and Stable Diffusion models.

| Model | Description |
|-------|-------------|
| `fal-ai/flux-pro` | FLUX Pro - high quality |
| `fal-ai/flux-dev` | FLUX Dev - development |
| `fal-ai/flux-schnell` | FLUX Schnell - fast |
| `fal-ai/stable-diffusion-xl` | Stable Diffusion XL |

**Supported Sizes:**

- `512x512`
- `768x1024`
- `1024x768`
- `1024x1024`

## Client Examples

### Python

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/openai/v1",
    api_key="your-api-key"
)

# Generate an image
response = client.images.generate(
    model="gpt-image-2",
    prompt="A futuristic city at night with neon lights",
    n=1,
    size="1024x1024"
)

print(response.data[0].url)
```

### JavaScript/TypeScript

```typescript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:8080/openai/v1',
  apiKey: 'your-api-key',
});

const response = await client.images.generate({
  model: 'gpt-image-2',
  prompt: 'A futuristic city at night with neon lights',
  n: 1,
  size: '1024x1024',
});

console.log(response.data[0].url);
```

### curl

```bash
# Generate image
curl -X POST http://localhost:8080/openai/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "prompt": "A futuristic city at night",
    "size": "1024x1024"
  }'

# Get base64 response instead of URL
curl -X POST http://localhost:8080/openai/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "prompt": "A futuristic city at night",
    "response_format": "b64_json"
  }'
```

## Error Handling

Image API errors follow the OpenAI error format:

```json
{
  "error": {
    "message": "Your request was rejected due to content policy violations.",
    "type": "invalid_request_error",
    "code": "content_policy_violation"
  }
}
```

| HTTP Status | Code | Description |
|-------------|------|-------------|
| 400 | `invalid_request_error` | Malformed request |
| 400 | `content_policy_violation` | Content violates policy |
| 404 | `model_not_found` | Model not available |
| 429 | `rate_limit_exceeded` | Too many requests |
| 500 | `internal_error` | Server error |

## Capabilities Matrix

| Feature | OpenAI | Fal AI |
|---------|--------|--------|
| Generate | Yes | Yes |
| Edit | Yes (DALL-E 2) | No |
| Variations | Yes (DALL-E 2) | No |
| HD Quality | Yes (DALL-E 3) | No |
| Style Control | Yes (DALL-E 3) | No |
| Multiple Images | Yes | Yes |
| Upscaling | No | Yes |
