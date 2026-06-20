# OpenAI-Compatible REST API

OmniAgent exposes an OpenAI-compatible REST API that allows you to use standard OpenAI client libraries to interact with your agent. The API supports SSE streaming, tool calling, and includes interactive documentation.

## Quick Start

Start the gateway server:

```bash
omniagent gateway run --config omniagent.yaml
```

The API is available at `http://localhost:18789/v1`.

### Using curl

```bash
curl http://localhost:18789/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "omniagent",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

### Using Python

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:18789/v1",
    api_key="your-api-key"
)

# Streaming chat completion
response = client.chat.completions.create(
    model="omniagent",
    messages=[{"role": "user", "content": "What tools do you have?"}],
    stream=True
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### Using JavaScript/TypeScript

```typescript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:18789/v1',
  apiKey: 'your-api-key',
});

const stream = await client.chat.completions.create({
  model: 'omniagent',
  messages: [{ role: 'user', content: 'Hello!' }],
  stream: true,
});

for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content || '');
}
```

## API Endpoints

### Chat Completions

#### POST /v1/chat/completions

Create a chat completion with optional streaming.

**Request:**

```json
{
  "model": "omniagent",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "stream": true,
  "temperature": 0.7,
  "max_tokens": 4096
}
```

**Response (non-streaming):**

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1719360000,
  "model": "omniagent",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 8,
    "total_tokens": 18
  }
}
```

**Response (streaming):**

Server-Sent Events (SSE) with `data:` prefixed JSON chunks:

```
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

### Models

#### GET /v1/models

List available models.

**Response:**

```json
{
  "object": "list",
  "data": [
    {
      "id": "omniagent",
      "object": "model",
      "created": 1719360000,
      "owned_by": "plexusone"
    }
  ]
}
```

#### GET /v1/models/{model}

Get details for a specific model.

### Tools

#### GET /v1/tools

List all registered tools.

**Response:**

```json
{
  "object": "list",
  "data": [
    {
      "name": "web_search",
      "description": "Search the web for information",
      "category": "search",
      "parameters": {
        "type": "object",
        "properties": {
          "query": {"type": "string", "description": "Search query"}
        },
        "required": ["query"]
      }
    }
  ]
}
```

### Agents

#### GET /v1/agents

List all configured agents.

**Response:**

```json
{
  "object": "list",
  "data": [
    {
      "id": "default",
      "name": "OmniAgent",
      "description": "Default agent",
      "model": "claude-sonnet-4-20250514",
      "provider": "anthropic",
      "enabled": true,
      "created_at": "2026-06-20T00:00:00Z"
    }
  ]
}
```

#### POST /v1/agents

Create a new agent.

**Request:**

```json
{
  "id": "research",
  "name": "Research Agent",
  "description": "Agent specialized for research tasks",
  "provider": "openai",
  "model": "gpt-4o",
  "system_prompt": "You are a research assistant.",
  "allowed_tools": ["web_search", "read_url"],
  "denied_tools": ["shell"]
}
```

#### GET /v1/agents/{id}

Get agent details.

#### PUT /v1/agents/{id}

Update an agent.

#### DELETE /v1/agents/{id}

Delete an agent.

### Cron Jobs

#### GET /v1/cron/jobs

List all scheduled jobs.

#### POST /v1/cron/jobs

Create a new job.

#### GET /v1/cron/jobs/{id}

Get job details.

#### DELETE /v1/cron/jobs/{id}

Delete a job.

#### POST /v1/cron/jobs/{id}/trigger

Trigger immediate execution.

### Usage

#### GET /v1/usage

Get usage statistics.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `start_time` | string | Start time (RFC3339) |
| `end_time` | string | End time (RFC3339) |
| `agent_id` | string | Filter by agent ID |

### Health

#### GET /health

Health check endpoint.

**Response:**

```json
{
  "status": "healthy",
  "version": "0.11.0",
  "uptime": "2h30m15s"
}
```

## Authentication

The API supports Bearer token authentication:

```bash
curl -H "Authorization: Bearer your-api-key" http://localhost:18789/v1/models
```

Configure API keys in your config file:

```yaml
gateway:
  api_keys:
    - "sk-your-api-key-1"
    - "sk-your-api-key-2"
```

Or via environment variable:

```bash
export OMNIAGENT_GATEWAY_API_KEYS="sk-key1,sk-key2"
```

## API Documentation

Interactive API documentation is available at `/docs` using Scalar UI:

```
http://localhost:18789/docs
```

The OpenAPI 3.1 specification is available at:

- JSON: `http://localhost:18789/openapi.json`
- YAML: `http://localhost:18789/openapi.yaml`

### Generating Static Spec

Generate a static OpenAPI specification file:

```bash
omniagent openai spec --output docs/api/openapi.yaml
omniagent openai spec --format json --output docs/api/openapi.json
```

## Tool Calling

When tools are available, the API supports OpenAI-style tool calling:

```python
response = client.chat.completions.create(
    model="omniagent",
    messages=[{"role": "user", "content": "Search for recent AI news"}],
)

# If the model wants to call a tool
if response.choices[0].message.tool_calls:
    tool_call = response.choices[0].message.tool_calls[0]
    print(f"Tool: {tool_call.function.name}")
    print(f"Args: {tool_call.function.arguments}")
```

Tool results are automatically processed by the agent when using OmniAgent's streaming endpoint.

## Session Management

Use the `session_id` parameter to maintain conversation context:

```json
{
  "model": "omniagent",
  "messages": [{"role": "user", "content": "Remember my name is Alice"}],
  "session_id": "user-123"
}
```

Subsequent requests with the same `session_id` will have access to conversation history.

## Multi-Agent Routing

When multiple agents are configured, specify the agent using `model`:

```json
{
  "model": "research",
  "messages": [{"role": "user", "content": "Search for papers on transformers"}]
}
```

Or use the default agent:

```json
{
  "model": "omniagent",
  "messages": [{"role": "user", "content": "Hello"}]
}
```

See [Multi-Agent Guide](multi-agent.md) for configuring multiple agents.

## Rate Limiting

The API includes built-in rate limiting. Configure limits per API key:

```yaml
gateway:
  rate_limits:
    default:
      requests_per_minute: 60
      tokens_per_minute: 100000
    premium:
      requests_per_minute: 600
      tokens_per_minute: 1000000
```

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1719360060
```

## Error Handling

Errors follow the OpenAI error format:

```json
{
  "error": {
    "message": "Invalid API key",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

| HTTP Status | Error Type | Description |
|-------------|------------|-------------|
| 400 | `invalid_request_error` | Malformed request |
| 401 | `authentication_error` | Invalid or missing API key |
| 404 | `not_found_error` | Resource not found |
| 429 | `rate_limit_error` | Too many requests |
| 500 | `api_error` | Internal server error |

## Architecture

The API is built on a hybrid architecture:

- **Chi Router**: Base HTTP router with middleware
- **ogen**: OpenAI-compatible endpoints generated from OpenAPI spec
- **Huma**: Custom endpoints with automatic OpenAPI generation
- **Scalar**: Interactive API documentation

```
Chi Router (base)
├── /v1/chat/completions  → ogen (SSE streaming)
├── /v1/models            → ogen
├── /v1/tools             → Huma
├── /v1/agents/*          → Huma
├── /v1/cron/jobs/*       → Huma
├── /v1/usage/*           → Huma
├── /health               → Huma
├── /openapi.json         → Merged spec
└── /docs                 → Scalar UI
```
