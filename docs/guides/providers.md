# LLM Providers

OmniAgent supports multiple LLM providers through [`omnillm`](https://github.com/plexusone/omnillm), a unified interface for chat completion APIs. Providers are registered automatically via thick provider imports.

## Overview

OmniAgent supports these LLM providers:

| Provider | Module | Models |
|----------|--------|--------|
| **Anthropic** | `omni-anthropic` | Claude 4.x, Claude 3.5, Claude 3 |
| **OpenAI** | `omni-openai` | GPT-4o, GPT-4, GPT-3.5 |
| **OpenRouter** | `omni-openrouter` | 200+ models from various providers |
| **AWS Bedrock** | `omni-aws` | Claude, Titan, Llama via AWS |
| **Google Vertex** | `omni-google` | Gemini, Claude via GCP |

## Quick Start

### Using Environment Variables

```bash
# Anthropic (default)
export ANTHROPIC_API_KEY="sk-ant-..."
omniagent gateway run

# OpenAI
export OPENAI_API_KEY="sk-..."
OMNIAGENT_AGENT_PROVIDER=openai \
OMNIAGENT_AGENT_MODEL=gpt-4o \
omniagent gateway run

# OpenRouter
export OPENROUTER_API_KEY="sk-or-..."
OMNIAGENT_AGENT_PROVIDER=openrouter \
OMNIAGENT_AGENT_MODEL=anthropic/claude-3.5-sonnet \
omniagent gateway run
```

### Using Configuration

```yaml
# config.yaml
agent:
  provider: anthropic
  model: claude-sonnet-4-20250514
```

## Anthropic

Direct API access to Claude models.

### Configuration

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

### Supported Models

| Model | ID |
|-------|---|
| Claude Opus 4 | `claude-opus-4-20250514` |
| Claude Sonnet 4 | `claude-sonnet-4-20250514` |
| Claude Haiku 4.5 | `claude-haiku-4-5-20250514` |
| Claude 3.5 Sonnet | `claude-3-5-sonnet-20241022` |
| Claude 3.5 Haiku | `claude-3-5-haiku-20241022` |
| Claude 3 Opus | `claude-3-opus-20240229` |

### Thinking Mode

Claude 4.x models support extended thinking:

```go
import "github.com/plexusone/omnillm-core"

req := &core.ChatCompletionRequest{
    Model: "claude-opus-4-20250514",
    Messages: messages,
    ThinkingConfig: &core.ThinkingConfig{
        Enabled:   true,
        BudgetMs:  30000, // 30 second thinking budget
    },
}
```

## OpenAI

Access to GPT models via OpenAI API.

### Configuration

```bash
export OPENAI_API_KEY="sk-..."
```

### Supported Models

| Model | ID |
|-------|---|
| GPT-4o | `gpt-4o` |
| GPT-4o Mini | `gpt-4o-mini` |
| GPT-4 Turbo | `gpt-4-turbo` |
| GPT-4 | `gpt-4` |
| GPT-3.5 Turbo | `gpt-3.5-turbo` |

## OpenRouter

Access 200+ models from multiple providers through a single API. OpenRouter provides model routing, fallbacks, and cost optimization.

### Setup

#### Option 1: API Key

```bash
export OPENROUTER_API_KEY="sk-or-..."
```

#### Option 2: OAuth PKCE (Browser Login)

```bash
omniagent auth login openrouter
```

This opens a browser for OAuth authentication and stores the credential in your vault.

### Configuration

```yaml
agent:
  provider: openrouter
  model: anthropic/claude-3.5-sonnet
```

### Model Selection

OpenRouter uses `provider/model` format:

```bash
# Anthropic models
OMNIAGENT_AGENT_MODEL=anthropic/claude-3.5-sonnet

# OpenAI models
OMNIAGENT_AGENT_MODEL=openai/gpt-4o

# Google models
OMNIAGENT_AGENT_MODEL=google/gemini-pro-1.5

# Meta models
OMNIAGENT_AGENT_MODEL=meta-llama/llama-3.1-70b-instruct
```

### Features

- **Model Discovery** - Automatically fetches available models
- **Cost Tracking** - Per-request cost information
- **Fallbacks** - Automatic fallback to alternative models
- **Rate Limiting** - Built-in rate limit handling

### OAuth PKCE Flow

The OAuth flow:

1. Generates code verifier + challenge (S256)
2. Starts local HTTP server on random port
3. Opens browser: `https://openrouter.ai/auth?code_challenge=...`
4. Receives callback with auth code
5. Exchanges code for API key
6. Stores in vault as `openrouter:default`

## AWS Bedrock

Access Claude and other models via AWS Bedrock.

### Configuration

```bash
# AWS credentials (via environment or ~/.aws/credentials)
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"
```

### Supported Models

| Model | Bedrock ID |
|-------|-----------|
| Claude Opus 4 | `anthropic.claude-opus-4-20250514-v1:0` |
| Claude Sonnet 4 | `anthropic.claude-sonnet-4-20250514-v1:0` |
| Claude 3.5 Sonnet | `anthropic.claude-3-5-sonnet-20241022-v2:0` |
| Claude 3.5 Haiku | `anthropic.claude-3-5-haiku-20241022-v1:0` |

### Configuration

```yaml
agent:
  provider: bedrock
  model: anthropic.claude-sonnet-4-20250514-v1:0
```

## Google Vertex AI

Access Gemini and Claude models via Google Cloud.

### Configuration

```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
export GOOGLE_CLOUD_PROJECT="my-project"
export GOOGLE_CLOUD_LOCATION="us-central1"
```

### Supported Models

| Model | Vertex ID |
|-------|----------|
| Claude Opus 4 | `claude-opus-4@20250514` |
| Claude Sonnet 4 | `claude-sonnet-4@20250514` |
| Gemini 1.5 Pro | `gemini-1.5-pro` |
| Gemini 1.5 Flash | `gemini-1.5-flash` |

## Provider Priority

When multiple providers are configured, omnillm uses priority-based selection:

1. **Thick providers** (highest) - Direct SDK integrations
2. **Thin providers** - Generic API adapters
3. **Fallback** - Default provider

### Custom Provider Selection

```go
import "github.com/plexusone/omnillm"

client := omnillm.NewClient(
    omnillm.WithProvider("openrouter"), // Force specific provider
)
```

## Thick Provider Architecture

OmniAgent uses "thick providers" - standalone modules that register themselves via import:

```go
import (
    // Thick providers - imported for side-effect registration
    _ "github.com/plexusone/omni-anthropic/omnillm"
    _ "github.com/plexusone/omni-openai/omnillm"
    _ "github.com/plexusone/omni-openrouter/omnillm"
    _ "github.com/plexusone/omni-aws/omnillm"
    _ "github.com/plexusone/omni-google/omnillm"
)
```

Each thick provider:

1. Registers with `omnillm-core` on import
2. Uses the vendor's official SDK
3. Handles authentication, retries, and errors
4. Converts to/from unified request/response types

## Environment Variables

### Per-Provider

| Provider | Variables |
|----------|-----------|
| Anthropic | `ANTHROPIC_API_KEY` |
| OpenAI | `OPENAI_API_KEY` |
| OpenRouter | `OPENROUTER_API_KEY` |
| Bedrock | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` |
| Vertex | `GOOGLE_APPLICATION_CREDENTIALS`, `GOOGLE_CLOUD_PROJECT` |

### Agent Configuration

| Variable | Description |
|----------|-------------|
| `OMNIAGENT_AGENT_PROVIDER` | Provider name |
| `OMNIAGENT_AGENT_MODEL` | Model ID |
| `OMNIAGENT_AGENT_TEMPERATURE` | Sampling temperature (0.0-1.0) |
| `OMNIAGENT_AGENT_MAX_TOKENS` | Maximum response tokens |

## Troubleshooting

### Provider Not Found

Ensure the provider module is imported:

```go
import _ "github.com/plexusone/omni-openrouter/omnillm"
```

### Authentication Errors

- Verify API key is set correctly
- Check key has appropriate permissions
- For OAuth, re-run `omniagent auth login <provider>`

### Model Not Available

- Verify model ID is correct for the provider
- Check model is available in your region (Bedrock, Vertex)
- For OpenRouter, use `provider/model` format

### Rate Limiting

- Implement exponential backoff
- Use OpenRouter for automatic rate limit handling
- Consider upgrading API tier
