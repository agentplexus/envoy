# OmniAgent Control UI Roadmap

Feature roadmap for the OmniAgent web-based Control UI, inspired by OpenClaw's dashboard architecture.

## Current Features (Implemented)

- [x] Chat interface with message history
- [x] Conversation sidebar with localStorage persistence
- [x] Voice input (Web Speech API)
- [x] Translation feature
- [x] Chart rendering (ECharts + ChartIR)
- [x] Dark theme support
- [x] Stop button to cancel generation
- [x] Phone number display for voice calls
- [x] Model selection dropdown

## Phase 1: Tools & Skills Panel

**Status:** Implemented (Core Features)

Display registered agent tools and their capabilities.

### Features

- [x] Tools sidebar panel (collapsible)
- [x] List all registered tools with icons
- [x] Tool descriptions and parameter schemas
- [x] Tool detail modal with parameter info
- [x] Tools grouped by category
- [ ] Tool usage statistics (call count, last used)
- [ ] Tool invocation history in chat

### API Endpoints

- [x] `GET /v1/tools` - List available tools
- [ ] `GET /v1/tools/{name}` - Get tool details

### UI Components

- [x] Tools panel toggle button in header (gear icon)
- [x] Collapsible tools list (right sidebar)
- [x] Tool detail modal/popover
- [ ] Tool call indicators in chat messages

### Implementation Files

- `api/openai/handler.go` - Added `ListTools` to `AgentHandler` interface
- `api/openai/server.go` - Added `/v1/tools` endpoint
- `openai/adapter.go` - Implemented `ListTools` method
- `api/openai/web/index.html` - Added tools panel UI

## Phase 2: Reasoning & Tool Call Streams

**Status:** Implemented (Thinking Blocks)

Show the agent's thinking process and tool calls during responses.

### Features

- [x] Collapsible "Thinking" sections in assistant messages
- [x] Real-time thinking block display during streaming
- [x] Animated "Thinking..." indicator while processing
- [x] Parse `<thinking>` and `<think>` tags
- [ ] Tool call visualization with expand/collapse
- [ ] Tool arguments display (formatted JSON)
- [ ] Tool result preview
- [ ] Execution timeline

### Implementation

- [x] Parse `<thinking>` tags from responses (both streaming and completed)
- [x] Render thinking blocks with collapsible UI
- [x] Purple accent color for thinking blocks
- [ ] Detect tool_calls in streaming responses
- [ ] Render tool calls as expandable cards
- [ ] Show tool results inline or in modal

### UI Features

- Thinking blocks display with brain icon
- Real-time streaming inside thinking blocks
- "Thinking..." animates while processing, changes to "Thought" when complete
- Click to expand/collapse thinking content
- Proper handling of mixed thinking + regular content

## Phase 3: System Status Bar

**Status:** Implemented

Real-time system status and health monitoring.

### Features

- [x] Connection status indicator (connected/disconnected)
- [x] Current model info display
- [x] Token usage counter (current session)
- [x] Response time metrics
- [x] Health check polling (every 30s)
- [ ] Gateway version display

### API Endpoints

- [x] `GET /health` - Health check (already exists)
- [ ] `GET /v1/status` - Detailed status including model, tokens

### UI Components

- [x] Status bar at bottom of input area
- [x] Green/red connection indicator dot
- [x] Model name display (synced with model select)
- [x] Token count with icon
- [x] Response time display

### Implementation

- Health check polling every 30 seconds
- Token estimation (~4 chars/token)
- Response time tracked per message
- Token count updates on:
  - Sending messages
  - Receiving responses
  - Switching conversations
  - Creating new conversations

## Phase 4: Settings Panel

**Status:** Implemented

User-configurable agent settings from the UI.

### Features

- [x] Settings modal/drawer
- [x] Temperature slider (0.0 - 2.0)
- [x] Max tokens input
- [x] System prompt editor
- [x] API key configuration (masked input)
- [x] Theme toggle (light/dark/system)
- [x] Voice settings (language selection)
- [ ] Voice continuous mode toggle

### Persistence

- [x] Store settings in localStorage
- [ ] Sync to server via API (optional)

### UI Components

- [x] Settings sliders icon in header
- [x] Modal with grouped sections
- [x] Form validation
- [x] Reset to defaults button
- [x] Apply settings without reload

### Implementation

- Settings stored in `omniagent_settings` localStorage key
- Default values: temperature=0.7, maxTokens=4096, theme=system
- Theme applies immediately (light/dark/system with media query)
- API calls include temperature, maxTokens, and optional apiKey
- System prompt prepended to messages when set

## Phase 5: Activity Log

**Status:** Implemented

Real-time event stream showing agent activity.

### Features

- [x] Expandable activity log panel (bottom drawer)
- [x] Event types: chat, tool, error, system
- [x] Timestamp and duration display
- [x] Filter by event type
- [x] Search within logs
- [x] Export logs (JSON format)
- [x] Clear log functionality

### Event Types

```
CHAT    - User messages, assistant responses
TOOL    - Tool loading events
ERROR   - Connection failures, request errors
SYSTEM  - App startup, settings changes, generation stops
```

### UI Components

- [x] Toggle button in header (clipboard icon)
- [x] Bottom drawer panel (collapsible)
- [x] Filter buttons (All/Chat/Tools/System/Errors)
- [x] Search input with real-time filtering
- [x] Export and clear buttons
- [x] Color-coded event types with badges

### Implementation

- Activity events stored in memory (max 500 events)
- Relative timestamps ("Just now", "5m ago", "2h ago")
- Duration tracking for response times
- Events logged for:
  - User message sent
  - Assistant response received (with tokens and duration)
  - Generation stopped
  - Connection status changes
  - Tools loaded
  - Settings saved
  - App initialization

## Phase 6: Enhanced Session Management

**Status:** Implemented

Advanced conversation/session features.

### Features

- [x] Session metadata display (created, message count, tokens)
- [x] Export conversation as markdown
- [x] Export conversation as JSON
- [x] Session archival (hide without delete)
- [x] Session duplication (copy full conversation)
- [x] Session forking (fork from message) - infrastructure ready
- [x] Session search (across title, tags, and messages)
- [x] Session tags/labels (add, remove, display)
- [x] Session title editing

### UI Components

- [x] Session info modal (metadata, tags, export, actions)
- [x] Search bar in sidebar
- [x] Toggle archived conversations
- [x] Tags display in conversation list
- [x] Fork indicator for forked conversations
- [x] Message count in conversation list

### Implementation

- Client-side localStorage persistence
- Search filters title, tags, and message content
- Archived sessions hidden by default (toggle to show)
- Tags stored as array in conversation object
- Export formats: Markdown (readable) and JSON (structured)
- Duplicate creates full copy with "(Copy)" suffix
- Fork ready for per-message branching UI

## Phase 7: Cron/Scheduled Jobs

**Status:** Implemented

Schedule automated agent tasks.

### Features

- [x] List scheduled jobs
- [x] Create/edit/delete jobs
- [x] Cron expression builder (with presets)
- [x] Run job manually (trigger)
- [x] Enable/disable jobs
- [x] Interval and one-time scheduling
- [ ] Job execution history (future)

### Action Types

- **send_message** - Send a message to an agent session
- **call_webhook** - Make HTTP request to a webhook URL
- **call_tool** - Invoke a registered agent tool

### API Endpoints

- `GET /v1/cron/jobs` - List all scheduled jobs
- `POST /v1/cron/jobs` - Create a new job
- `GET /v1/cron/jobs/{id}` - Get job details
- `PUT /v1/cron/jobs/{id}` - Update a job
- `DELETE /v1/cron/jobs/{id}` - Delete a job
- `POST /v1/cron/jobs/{id}/trigger` - Run job immediately
- `POST /v1/cron/jobs/{id}/enable` - Enable a job
- `POST /v1/cron/jobs/{id}/disable` - Disable a job

### UI Components

- [x] Cron toggle button in header (clock icon)
- [x] Collapsible cron jobs panel (right sidebar)
- [x] Job cards with status, schedule, actions
- [x] Create/edit modal with:
  - Schedule type tabs (Cron/Interval/Once)
  - Cron expression presets
  - Action type selector
  - Dynamic action fields
- [x] Quick actions (trigger, enable/disable, delete)

### Implementation Files

- `cron/executor.go` - Job action execution handler
- `cron/tools.go` - Cron skill with AgentAware interface
- `api/openai/handler.go` - Cron types and AgentHandler interface
- `api/openai/server.go` - REST API endpoints
- `openai/adapter.go` - AgentHandler implementation
- `api/openai/web/index.html` - Web UI cron panel

## Phase 8: Multi-Agent Support

**Status:** Implemented

Support for multiple agent configurations.

### Features

- [x] Agent switcher dropdown (via model selector)
- [x] Agent configuration editor (create/edit modal)
- [x] Per-agent tool assignments (allowed_tools, denied_tools)
- [x] Agent-specific system prompts
- [x] Agent cloning
- [x] Agent registry with KVS persistence
- [x] Multi-agent adapter for request routing
- [x] REST API for agent management

### API Endpoints

- `GET /v1/agents` - List all configured agents
- `POST /v1/agents` - Create a new agent
- `GET /v1/agents/{id}` - Get agent details
- `PUT /v1/agents/{id}` - Update an agent
- `DELETE /v1/agents/{id}` - Delete an agent
- `POST /v1/agents/{id}/clone` - Clone an agent

### Configuration

Agents can be configured via YAML:

```yaml
# Single agent (backward compatible)
agent:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: ${ANTHROPIC_API_KEY}

# Multiple agents
agents:
  - id: default
    name: General Assistant
    provider: anthropic
    model: claude-sonnet-4-20250514
    system_prompt: "You are a helpful assistant."

  - id: coder
    name: Code Expert
    provider: anthropic
    model: claude-sonnet-4-20250514
    system_prompt: "You are an expert programmer..."
    allowed_tools: [read, write, edit, glob, grep]
```

### UI Components

- [x] Agents toggle button in header (users icon)
- [x] Collapsible agents panel (right sidebar)
- [x] Agent cards with name, model, status
- [x] Create/Edit modal with form fields
- [x] Quick actions (switch, edit, clone, delete)

### Implementation Files

- `agent/registry/config.go` - AgentConfig type
- `agent/registry/store.go` - KVS persistence
- `agent/registry/registry.go` - Registry management
- `agent/registry/registry_test.go` - Registry unit tests
- `config/config.go` - Config enhancements
- `openai/multi_adapter.go` - Multi-agent routing
- `openai/multi_adapter_test.go` - Multi-agent adapter tests
- `openai/cron_handler.go` - Shared cron handler (embedded by adapters)
- `api/openai/handler.go` - Agent types and interface
- `api/openai/server.go` - REST API endpoints
- `api/openai/server_test.go` - API handler tests
- `cmd/omniagent/commands/openai.go` - CLI integration
- `cmd/omniagent/commands/gateway.go` - Gateway integration
- `api/openai/web/index.html` - Web UI agents panel

## Phase 9: Usage Analytics

**Status:** Implemented

Token usage tracking, cost estimation, and visualization.

### Features

- [x] Token usage tracking per request
- [x] Cost estimation based on model pricing
- [x] Latency metrics
- [x] Time-series data aggregation (hourly/daily)
- [x] Summary statistics (total requests, tokens, cost)
- [x] Usage breakdown by model
- [x] ECharts visualizations (tokens, cost, model distribution)
- [x] Recent requests history table

### API Endpoints

- `GET /v1/usage` - Usage summary for time range
- `GET /v1/usage/timeseries` - Time-bucketed usage data
- `GET /v1/usage/records` - Recent usage records
- `GET /v1/usage/chart/tokens` - Token chart (ChartIR format)
- `GET /v1/usage/chart/cost` - Cost chart (ChartIR format)
- `GET /v1/usage/chart/models` - Model distribution chart (ChartIR format)

### Query Parameters

- `since` - Start time (RFC3339 or relative: 1h, 24h, 7d, 30d)
- `until` - End time (RFC3339, defaults to now)
- `interval` - Bucket size (minute, hour, day)
- `limit` - Max records for /records endpoint

### Model Pricing

Default pricing for cost estimation:

| Model | Input $/1M tokens | Output $/1M tokens |
|-------|-------------------|-------------------|
| claude-3-5-sonnet* | 3.00 | 15.00 |
| claude-3-opus* | 15.00 | 75.00 |
| claude-3-haiku* | 0.25 | 1.25 |
| gpt-4* | 30.00 | 60.00 |
| gpt-3.5-turbo* | 0.50 | 1.50 |
| default | 1.00 | 2.00 |

### UI Components

- [x] Usage toggle button in header (chart icon)
- [x] Usage analytics modal with:
  - Time range selector (1h, 24h, 7d, 30d)
  - Summary cards (requests, tokens, cost, latency)
  - Token usage stacked bar chart
  - Cost line chart with area fill
  - Model distribution pie chart
  - Model breakdown table
  - Recent requests table

### Implementation Files

- `api/openai/usage.go` - Usage types, store, and chart generation
- `api/openai/handler.go` - UsageHandler interface
- `api/openai/server.go` - Usage API endpoints
- `api/openai/streaming.go` - Usage tracking in chat handler
- `api/openai/web/index.html` - Usage analytics UI

### Dependencies

- `github.com/grokify/echartify/chartir` - ECharts IR generation

## Phase 10: Memory/Context Panel

**Status:** Implemented

Semantic memory for storing and retrieving information across conversations.

### Features

- [x] Memory panel (right sidebar)
- [x] Collection selector
- [x] Semantic search with query input
- [x] Memory list with content, key, metadata
- [x] Add memory modal
- [x] Delete memory functionality
- [x] Collection refresh

### API Endpoints

- `GET /v1/memories` - List memories in collection
- `POST /v1/memories` - Store new memory
- `GET /v1/memories/search` - Semantic search
- `DELETE /v1/memories/{key}` - Delete memory
- `GET /v1/memories/collections` - List collections

### Query Parameters

- `collection` - Collection name (default: "default")
- `q` - Search query (for /search endpoint)
- `limit` - Max results

### UI Components

- [x] Memory toggle button in header (lightbulb icon)
- [x] Collection dropdown with refresh button
- [x] Search input with Enter to search
- [x] Memory cards with content, key, metadata, score
- [x] Add memory modal with content, key, collection, metadata
- [x] Delete confirmation

### Integration

The memory panel connects to the existing `skills/memory` package which provides:

- Vector embeddings for semantic search
- Collection-based organization
- Metadata support
- KVS persistence

### Implementation Files

- `api/openai/handler.go` - MemoryHandler interface and types
- `api/openai/server.go` - Memory API endpoints
- `api/openai/web/index.html` - Memory panel UI
- `skills/memory/memory.go` - Memory skill (existing)

## Technical Architecture

### API Router Stack (Chi + Huma + ogen)

The API server uses a hybrid architecture combining three routing technologies:

```
Chi Router (base)
├── /v1/chat/completions  → StreamingHandler → ogen (SSE streaming)
├── /v1/models            → ogen handler
├── /v1/models/{model}    → ogen handler
├── /v1/tools             → Huma (auto-generated OpenAPI)
├── /v1/agents/*          → Huma
├── /v1/cron/jobs/*       → Huma
├── /v1/usage/*           → Huma
├── /v1/memories/*        → Huma
├── /health               → Huma
├── /openapi.json         → Merged spec (ogen input + Huma generated)
├── /docs                 → Scalar UI
└── /                     → Web UI (static)
```

#### Component Responsibilities

- **Chi Router (github.com/go-chi/chi/v5)**: Base HTTP router with middleware support (RealIP, Recoverer). Provides clean path-based routing and compatibility with standard `http.Handler`.

- **ogen (github.com/ogen-go/ogen)**: Handles OpenAI-compatible endpoints (`/v1/chat/completions`, `/v1/models`). Used for SSE streaming support and type-safe request/response handling generated from OpenAPI spec.

- **Huma (github.com/danielgtaylor/huma/v2)**: Handles OmniAgent extension endpoints (tools, agents, cron, usage, memory, health). Provides automatic OpenAPI 3.1 schema generation from Go types with validation.

- **Scalar (github.com/bdpiprava/scalar-go)**: Serves interactive API documentation at `/docs` with modern UI, dark mode, and search.

#### OpenAPI Spec Merging

The `/openapi.json` endpoint serves a merged OpenAPI specification that combines:

1. **ogen input spec** (`openapi/openai-minimal.yaml`): OpenAI-compatible chat completion and models endpoints
2. **Huma-generated spec**: Auto-generated from registered Huma operations (tools, agents, cron, usage, memory, health)

This provides a complete, accurate API reference for all endpoints.

#### Implementation Files

- `api/openai/server.go` - Chi router setup, Huma integration, route mounting
- `api/openai/operations/*.go` - Huma operation definitions (types, tools, agents, cron, usage, memory, health)
- `api/openai/openapi_merge.go` - OpenAPI spec merging logic
- `api/openai/docs.go` - Scalar documentation handler
- `api/openai/streaming.go` - SSE streaming handler for chat completions
- `api/openai/internal/ogen/` - Generated ogen code from OpenAPI spec

### Frontend Stack

- Vanilla JavaScript (current)
- Tailwind CSS for styling
- ECharts for visualization
- Web Speech API for voice

### Backend Integration

- OpenAI-compatible API endpoints
- Server-Sent Events (SSE) for streaming
- WebSocket for real-time updates (future)

### State Management

- localStorage for persistence
- In-memory state for session
- Server sync for settings (optional)

## References

- [OpenClaw Control UI](https://docs.openclaw.ai/web/dashboard)
- [OpenAI API Specification](https://platform.openai.com/docs/api-reference)
- [ECharts Documentation](https://echarts.apache.org/)
