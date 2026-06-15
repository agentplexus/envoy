# Omniagent Feature Parity Plan (Plexusone Architecture)

Bring omniagent to feature parity with openclaw (7,162 commits since `03125c8e`) using the plexusone library architecture.

## Status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | OpenRouter Provider | Completed |
| 2 | Claude 4.x Model Support | Completed |
| 3 | Enhanced Search | Completed |
| 4 | ClawHub Skills Marketplace | Completed |
| 5 | Memory/Retrieval Enhancement | Completed |
| 6 | Workboard Project Management | Completed |
| 7 | Auto-reply with Verbose Progress | Completed |
| 8 | Voice Feature Parity | Completed |
| - | Integration/Wiring | Completed |

## Architecture Decision Summary

| Feature | Module Location | Rationale |
|---------|-----------------|-----------|
| OpenRouter | `omni-openrouter` (new) | Follow thick provider pattern |
| Model Support | Update `omnillm-core`, thick providers | Existing provider architecture |
| ClawHub | `omniskill/clawhub` → `omniskill/installer` | Separate logic + installer integration |
| Enhanced Search | Update `omniserp` | Extend existing Engine interface |
| Memory/Rerank | Update `omniretrieve` | Already has vector/, hybrid/, rerank/ |
| Workboard | `omniworkboard` (new) | Standalone module for reuse |
| Progress/Auto-reply | `omniagent/agent/profiles` | Agent-specific feature |

---

## Phase 1: OpenRouter Provider (P1) ✅

### New Module: `github.com/plexusone/omni-openrouter`

**Structure:**
```
omni-openrouter/
├── go.mod                    # Depends on OpenRouterTeam/go-sdk
├── omnillm/
│   ├── adapter.go            # Implements provider.Provider
│   └── doc.go                # Package documentation
├── auth/
│   ├── pkce.go               # OAuth PKCE flow
│   ├── callback.go           # Local callback server
│   └── token.go              # Token storage via omnivault
├── models.go                 # Model discovery/caching
└── README.md
```

**Key Implementation:**

```go
// omnillm/adapter.go
func init() {
    core.RegisterProvider(core.ProviderNameOpenRouter, newProviderFromConfig, core.PriorityThick)
}

type Provider struct {
    client *openrouter.Client
    config Config
}

func (p *Provider) CreateChatCompletion(ctx context.Context, req *core.ChatCompletionRequest) (*core.ChatCompletionResponse, error) {
    // Convert unified request to OpenRouter SDK format
    orReq := convertRequest(req)
    res, err := p.client.Chat.Send(ctx, orReq)
    // Convert back to unified response
    return convertResponse(res), err
}
```

**OAuth PKCE Flow:**
1. Generate code verifier (43-128 chars) + challenge (S256)
2. Start local HTTP server on random port
3. Open browser: `https://openrouter.ai/auth?code_challenge=...`
4. Receive callback with auth code
5. Exchange code for API key via SDK
6. Store in omnivault as `openrouter:default`

**Integration with omniagent:**
- Import in `omnillm/omnillm.go`: `_ "github.com/plexusone/omni-openrouter/omnillm"`
- Add CLI command: `omniagent auth login openrouter`

---

## Phase 2: Claude 4.x / Mythos Model Support (P0) ✅

### Updates to existing modules

**omnillm-core/constants.go:**
```go
const (
    // Claude 4.x models (Mythos foundation)
    ModelClaudeOpus4     = "claude-opus-4-20250514"
    ModelClaudeSonnet4   = "claude-sonnet-4-20250514"
    ModelClaudeHaiku45   = "claude-haiku-4-5-20250514"
)
```

**omni-anthropic/omnillm/adapter.go:**
- Verify SDK v1.46.0+ handles Claude 4.x
- Add thinking mode support for Opus/Sonnet 4
- Update model validation

**omni-aws (Bedrock):**
```go
const (
    BedrockClaudeOpus4   = "anthropic.claude-opus-4-20250514-v1:0"
    BedrockClaudeSonnet4 = "anthropic.claude-sonnet-4-20250514-v1:0"
)
```

**omni-google (Vertex AI):**
```go
const (
    VertexClaudeOpus4   = "claude-opus-4@20250514"
    VertexClaudeSonnet4 = "claude-sonnet-4@20250514"
)
```

**Verification:**
```bash
cd ~/go/src/github.com/plexusone/omnillm-core && go test ./...
cd ~/go/src/github.com/plexusone/omni-anthropic && go test ./...
```

---

## Phase 3: Enhanced Search (P1) ✅

### Updates to `omniserp`

**New engines added:**

```
omniserp/
├── client/
│   ├── brave/
│   │   └── brave.go         # Brave Search API
│   └── exa/
│       └── exa.go           # Exa.ai API
└── ...
```

**Brave Search Implementation:**
- Uses `X-Subscription-Token` header
- Supports web, news, images, videos, autocomplete
- Implements `omniserp.Engine` interface

**Exa.ai Implementation:**
- Uses `x-api-key` header, POST requests
- Supports multiple search types: auto, instant, fast, deep, deep-lite, deep-reasoning

---

## Phase 4: ClawHub Skills Marketplace (P1) ✅

### New package in `omniskill`

**Structure:**
```
omniskill/
├── clawhub/
│   ├── hub.go            # ClawHub API client
│   ├── manifest.go       # CLAWHUB.json parsing
│   ├── resolver.go       # Dependency resolution
│   └── security.go       # Security scanning
├── installer/
│   ├── github.go         # GitHub release fetching
│   └── clawhub.go        # ClawHub marketplace integration
└── ...
```

**Manifest Format (CLAWHUB.json):**

```go
type Manifest struct {
    Name         string       `json:"name"`
    Version      string       `json:"version"`
    Description  string       `json:"description"`
    Author       string       `json:"author"`
    Repository   string       `json:"repository"`
    License      string       `json:"license"`
    Dependencies []Dependency `json:"dependencies"`
    Permissions  []string     `json:"permissions"`
    Signature    string       `json:"signature,omitempty"`
}
```

**CLI Commands (in omniagent):**
```bash
omniagent skills install github.com/user/skill
omniagent skills install @clawhub/official-skill
omniagent skills update [name]
omniagent skills remove <name>
omniagent skills list --installed
omniagent skills search <query>
```

---

## Phase 5: Memory/Retrieval Enhancement (P2) ✅

### Updates to `omniretrieve`

**New packages:**

```
omniretrieve/
├── bm25/
│   ├── bm25.go           # BM25 text search index
│   └── bm25_test.go      # Tests
├── memory/
│   ├── manager.go        # Memory manager with collections
│   └── ...
└── ...
```

**BM25 Index:**
```go
type Index struct {
    docs      map[string]*Document
    termFreqs map[string]map[string]int
    docFreqs  map[string]int
    avgDL     float64
}

func (i *Index) Search(query string, k int) []*ScoredDocument
```

**Memory Manager:**
```go
type MemoryManager struct {
    collections map[string]*Collection
    embedder    vector.Embedder
}

func (m *MemoryManager) Store(ctx context.Context, collection, key string, doc *Document) error
func (m *MemoryManager) Search(ctx context.Context, collection, query string, opts SearchOpts) ([]*Document, error)
```

---

## Phase 6: Workboard Project Management (P1) ✅

### New Module: `github.com/plexusone/omniworkboard`

**Structure:**
```
omniworkboard/
├── go.mod
├── board.go              # Board with columns
├── card.go               # Card struct
├── column.go             # Column types
├── tools.go              # Tool definitions for agents
├── skill.go              # compiled.Skill implementation
└── board_test.go
```

**Core Types:**

```go
type ColumnType string

const (
    ColumnBacklog    ColumnType = "backlog"
    ColumnTodo       ColumnType = "todo"
    ColumnInProgress ColumnType = "in_progress"
    ColumnReview     ColumnType = "review"
    ColumnDone       ColumnType = "done"
)

type Priority int

const (
    PriorityLow Priority = iota
    PriorityNormal
    PriorityHigh
    PriorityCritical
)

type Card struct {
    ID          string            `json:"id"`
    Title       string            `json:"title"`
    Description string            `json:"description"`
    Column      ColumnType        `json:"column"`
    Priority    Priority          `json:"priority"`
    DependsOn   []string          `json:"depends_on"`
    BlockedBy   []string          `json:"blocked_by"` // Computed
    Labels      []string          `json:"labels"`
    Metadata    map[string]any    `json:"metadata"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
    CompletedAt *time.Time        `json:"completed_at,omitempty"`
}
```

**Skill Implementation:**

```go
type WorkboardSkill struct {
    board  *Board
    store  kvs.Store
}

func (s *WorkboardSkill) Name() string { return "workboard" }

func (s *WorkboardSkill) Tools() []skill.Tool {
    return []skill.Tool{
        skill.NewTool("create_card", "Create a new task card", s.createCard),
        skill.NewTool("move_card", "Move card to different column", s.moveCard),
        skill.NewTool("list_cards", "List cards with optional filters", s.listCards),
        skill.NewTool("update_card", "Update card details", s.updateCard),
        skill.NewTool("add_dependency", "Add dependency between cards", s.addDependency),
    }
}
```

**Integration with omniagent:**
```go
import "github.com/plexusone/omniworkboard"

agent.New(config,
    agent.WithCompiledSkill(omniworkboard.NewSkill()),
)
```

---

## Phase 7: Auto-reply with Verbose Progress (P2) ✅

### Updates to `omniagent/agent/profiles`

**New files:**
```
agent/profiles/
├── progress.go       # (existing) - Enhanced
├── autoreply.go      # Auto-reply configuration
└── commentary.go     # Inter-tool commentary
```

**Commentary System:**

```go
// commentary.go
type Commentary struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"` // thinking, progress, tool_summary
    Content   string    `json:"content"`
    ToolName  string    `json:"tool_name,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

type CommentaryEmitter struct {
    mode     CommentaryMode
    buffer   []*Commentary
    dispatch func(*Commentary)
}

func (e *CommentaryEmitter) EmitThinking(content string)
func (e *CommentaryEmitter) EmitToolSummary(tool string, result any)
func (e *CommentaryEmitter) Flush()
```

**Auto-reply Config:**

```go
// autoreply.go
type AutoReplyConfig struct {
    Enabled           bool
    CommentaryMode    CommentaryMode // None, Brief, Verbose
    InterToolDelay    time.Duration
    PersistCommentary bool
}

type CommentaryMode int

const (
    CommentaryNone CommentaryMode = iota
    CommentaryBrief
    CommentaryVerbose
)
```

---

## Integration (Wiring) ✅

### Agent Integration (`agent/agent.go`)

```go
import (
    // Thick providers - imported for side-effect registration
    _ "github.com/plexusone/omni-openrouter/omnillm"
)
```

### Memory Skill (`skills/memory/memory.go`)

New compiled skill wrapping `omniretrieve/memory.Manager`:

```go
type Skill struct {
    manager  *memory.Manager
    embedder vector.Embedder
}

// Tools: memory_store, memory_search, memory_list, memory_delete, memory_collections
```

### Local Development (`go.mod`)

Replace directives for unpublished modules:

```go
replace github.com/plexusone/omni-openrouter => ../omni-openrouter
replace github.com/plexusone/omniretrieve => ../omniretrieve
replace github.com/plexusone/omniworkboard => ../omniworkboard
```

---

## Module Dependencies

```
omniagent
├── omnillm (imports thick providers)
│   ├── omnillm-core
│   ├── omni-anthropic
│   ├── omni-openai
│   ├── omni-aws
│   ├── omni-google
│   └── omni-openrouter (NEW)
├── omniskill (with clawhub)
├── omniserp (with new engines)
├── omniretrieve (with BM25/memory)
└── omniworkboard (NEW)
```

---

---

## Phase 8: Voice Feature Parity (P1) - Completed

Voice calling feature parity across omnivoice and provider modules.

### New Packages

| Module | Package | Description |
|--------|---------|-------------|
| omnivoice-core | `storage/` | Session state persistence (MemoryStore, RedisStore) |
| omnivoice-core | `bargein/` | Barge-in detection for interruption handling |
| omni-openai | `omnivoice/realtime/` | OpenAI Realtime API (~100ms voice-to-voice) |
| omni-google | `omnivoice/` | Gemini Live API (~200ms voice-to-voice) |
| omniskill | `voicetools/` | Voice call control tools (transfer, hold, consult) |

### Key Features

- **Native voice-to-voice**: No separate STT/TTS pipeline, model handles audio directly
- **Session persistence**: Redis-backed storage for call recovery
- **Barge-in detection**: Natural interruption handling with configurable modes
- **Voice tools**: AI agents can transfer, hold, consult specialists, conference

See: [omniskill/docs/specs/features/voice-parity/PLAN.md](../../omniskill/docs/specs/features/voice-parity/PLAN.md)

---

## Remaining Work

### To Publish

1. Push `omni-openrouter` to GitHub and tag v0.1.0
2. Push `omniworkboard` to GitHub and tag v0.1.0
3. Tag new releases for `omniretrieve`, `omniskill`, `omniserp` with new packages
4. Remove local replace directives from `omniagent/go.mod`

### CLI Commands (Future)

```bash
# Skills management
omniagent skills install github.com/user/skill
omniagent skills install @clawhub/official-skill
omniagent skills update [name]
omniagent skills remove <name>
omniagent skills list --installed
omniagent skills search <query>

# Auth
omniagent auth login openrouter
```

---

## Verification

**Per-module tests:**
```bash
# After each phase
cd ~/go/src/github.com/plexusone/<module>
go test ./... -v
golangci-lint run
```

**Integration test (omniagent):**
```bash
cd ~/go/src/github.com/plexusone/omniagent
go mod tidy
go test ./... -v
```

**Manual verification:**
```bash
# OpenRouter
omniagent auth login openrouter

# ClawHub
omniagent skills install github.com/example/skill

# Workboard
omniagent chat
> create a card for "Fix login bug" with high priority

# Memory
> store this conversation in memory
> search memory for "login"
```
