# Multi-Agent Configuration

OmniAgent supports running multiple agents with different models, configurations, and tool access. This enables specialized agents for different tasks while sharing a common infrastructure.

## Quick Start

Configure multiple agents in your config file:

```yaml
# omniagent.yaml
gateway:
  address: "127.0.0.1:18789"

# Default agent settings (used as fallback)
agent:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: ${ANTHROPIC_API_KEY}

# Multiple agent configurations
agents:
  - id: general
    name: General Assistant
    description: General-purpose assistant for everyday tasks
    # Inherits provider, model, api_key from agent section

  - id: research
    name: Research Agent
    description: Specialized for research and analysis
    provider: openai
    model: gpt-4o
    api_key: ${OPENAI_API_KEY}
    system_prompt: |
      You are a research assistant. Focus on finding accurate,
      well-sourced information. Always cite your sources.
    allowed_tools:
      - web_search
      - read_url
      - summarize

  - id: coder
    name: Coding Agent
    description: Specialized for code generation and review
    provider: anthropic
    model: claude-sonnet-4-20250514
    system_prompt: |
      You are a senior software engineer. Write clean, well-tested code.
      Follow best practices and explain your design decisions.
    allowed_tools:
      - shell
      - read_file
      - write_file
      - browser
    denied_tools:
      - web_search  # Focus on coding, not research
```

## Agent Configuration

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique identifier for the agent |
| `name` | string | No | Human-readable name |
| `description` | string | No | Description of agent's purpose |
| `provider` | string | No | LLM provider (inherits from `agent`) |
| `model` | string | No | Model name (inherits from `agent`) |
| `api_key` | string | No | API key (inherits from `agent`) |
| `base_url` | string | No | Custom API endpoint |
| `temperature` | float | No | Sampling temperature (0.0-2.0) |
| `max_tokens` | int | No | Maximum response tokens |
| `system_prompt` | string | No | Custom system prompt |
| `allowed_tools` | []string | No | Whitelist of allowed tools |
| `denied_tools` | []string | No | Blacklist of denied tools |
| `enabled` | bool | No | Whether agent is active (default: true) |

### Inheritance

Agents inherit settings from the `agent` section:

```yaml
agent:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: ${ANTHROPIC_API_KEY}
  temperature: 0.7

agents:
  - id: fast
    # Inherits all settings from agent section
    model: claude-3-haiku-20240307  # Override just the model

  - id: custom
    provider: openai  # Override provider and model
    model: gpt-4o
    api_key: ${OPENAI_API_KEY}  # Different API key
```

## Tool Access Control

### Allowed Tools

Restrict an agent to specific tools:

```yaml
agents:
  - id: safe
    name: Safe Agent
    allowed_tools:
      - web_search
      - calculator
      - weather
    # Only these tools are available
```

### Denied Tools

Block specific tools while allowing others:

```yaml
agents:
  - id: restricted
    name: Restricted Agent
    denied_tools:
      - shell
      - browser
      - write_file
    # All other tools are available
```

### Combining Allowed and Denied

When both are specified, `allowed_tools` takes precedence:

```yaml
agents:
  - id: strict
    allowed_tools:
      - web_search
      - read_file
    denied_tools:
      - shell  # Ignored, not in allowed list anyway
```

## Using Multiple Agents

### Via API

Specify the agent using the `model` parameter:

```bash
# Use the research agent
curl http://localhost:18789/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "research",
    "messages": [{"role": "user", "content": "Find papers on transformers"}]
  }'

# Use the coding agent
curl http://localhost:18789/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "coder",
    "messages": [{"role": "user", "content": "Write a Python function to sort a list"}]
  }'
```

### Via Python

```python
from openai import OpenAI

client = OpenAI(base_url="http://localhost:18789/v1", api_key="key")

# Research task
research_response = client.chat.completions.create(
    model="research",
    messages=[{"role": "user", "content": "Summarize recent AI developments"}]
)

# Coding task
coding_response = client.chat.completions.create(
    model="coder",
    messages=[{"role": "user", "content": "Write unit tests for this function"}]
)
```

### Default Agent

Use `omniagent` or the first enabled agent:

```python
response = client.chat.completions.create(
    model="omniagent",  # Routes to default agent
    messages=[{"role": "user", "content": "Hello"}]
)
```

## Agent Management API

### List Agents

```bash
curl http://localhost:18789/v1/agents
```

```json
{
  "object": "list",
  "data": [
    {
      "id": "general",
      "name": "General Assistant",
      "model": "claude-sonnet-4-20250514",
      "enabled": true
    },
    {
      "id": "research",
      "name": "Research Agent",
      "model": "gpt-4o",
      "enabled": true
    }
  ]
}
```

### Create Agent (Runtime)

```bash
curl -X POST http://localhost:18789/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "id": "translator",
    "name": "Translation Agent",
    "provider": "openai",
    "model": "gpt-4o",
    "system_prompt": "You are a translator. Translate text accurately."
  }'
```

### Update Agent

```bash
curl -X PUT http://localhost:18789/v1/agents/translator \
  -H "Content-Type: application/json" \
  -d '{
    "system_prompt": "You are a translator specializing in technical documents."
  }'
```

### Disable Agent

```bash
curl -X PUT http://localhost:18789/v1/agents/translator \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}'
```

### Delete Agent

```bash
curl -X DELETE http://localhost:18789/v1/agents/translator
```

## Programmatic Usage

### Creating Agents in Go

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/agent/registry"
)

// Create registry with factory
reg := registry.New(registry.RegistryConfig{
    Factory: func(cfg *registry.AgentConfig) (*agent.Agent, error) {
        return agent.New(agent.Config{
            Provider:     cfg.Provider,
            Model:        cfg.Model,
            APIKey:       cfg.APIKey,
            SystemPrompt: cfg.SystemPrompt,
        })
    },
    Defaults: &registry.AgentConfig{
        Provider: "anthropic",
        Model:    "claude-sonnet-4-20250514",
    },
})

// Register agents
reg.Create(ctx, &registry.AgentConfig{
    ID:          "research",
    Name:        "Research Agent",
    Provider:    "openai",
    Model:       "gpt-4o",
    SystemPrompt: "You are a research assistant.",
})

// Use an agent
agent, err := reg.Get("research")
if err != nil {
    log.Fatal(err)
}

response, err := agent.Process(ctx, "Find papers on transformers")
```

### Getting the Default Agent

```go
// Get the first enabled agent
defaultAgent, err := reg.Default()
if err != nil {
    log.Fatal("no agents available")
}
```

## Session Isolation

Each agent maintains separate session histories:

```python
# Research agent session
client.chat.completions.create(
    model="research",
    messages=[{"role": "user", "content": "My name is Alice"}],
    extra_body={"session_id": "user-123"}
)

# Coding agent doesn't know the name
client.chat.completions.create(
    model="coder",
    messages=[{"role": "user", "content": "What's my name?"}],
    extra_body={"session_id": "user-123"}
)
# Response: "I don't know your name..."
```

To share context across agents, use explicit message passing.

## Use Cases

### Specialized Teams

```yaml
agents:
  - id: researcher
    name: Research Specialist
    system_prompt: Find and summarize information.
    allowed_tools: [web_search, read_url]

  - id: writer
    name: Content Writer
    system_prompt: Write engaging content.
    denied_tools: [shell, browser]

  - id: reviewer
    name: Code Reviewer
    system_prompt: Review code for bugs and improvements.
    allowed_tools: [read_file, shell]
```

### Cost Optimization

```yaml
agents:
  - id: fast
    name: Quick Queries
    provider: anthropic
    model: claude-3-haiku-20240307  # Cheaper, faster

  - id: complex
    name: Complex Tasks
    provider: anthropic
    model: claude-sonnet-4-20250514  # More capable
```

### Provider Redundancy

```yaml
agents:
  - id: primary
    name: Primary Agent
    provider: anthropic
    model: claude-sonnet-4-20250514

  - id: fallback
    name: Fallback Agent
    provider: openai
    model: gpt-4o
```

## Best Practices

1. **Use descriptive IDs**: Agent IDs are used in API calls, make them memorable
2. **Set clear system prompts**: Each agent should have a focused purpose
3. **Restrict tool access**: Only give agents the tools they need
4. **Monitor usage**: Track which agents are used most via `/v1/usage`
5. **Start simple**: Begin with 2-3 agents and expand as needed

## Limitations

- Agent configurations are stored in memory by default
- Runtime-created agents don't persist across restarts (use config file)
- Each agent maintains its own session history
- Tool registrations are shared across all agents
