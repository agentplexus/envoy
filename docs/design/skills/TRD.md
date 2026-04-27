# Skills System - Technical Requirements Document

**Version:** 1.0
**Date:** 2026-04-26
**Status:** Draft

## Overview

This document defines the technical architecture for OmniAgent's skills system, including loading strategies, interfaces, and integration points.

## Current State

OmniAgent v0.6.0 supports three skill integration approaches:

| Approach | Package | Status |
|----------|---------|--------|
| Markdown Skills | `skills/` | ✅ Implemented |
| Compiled Skills | `skills/compiled/` | ✅ Implemented |
| Remote MCP Skills | `skills/remote/mcp/` | ✅ Implemented |

### Markdown Skills (`skills/`)

```go
type Skill struct {
    Name        string    `yaml:"name"`
    Description string    `yaml:"description"`
    Homepage    string    `yaml:"homepage,omitempty"`
    Metadata    SkillMeta `yaml:"metadata"`
    Content     string    `yaml:"-"` // Markdown body
    Path        string    `yaml:"-"` // Directory path
    HasHooks    bool      `yaml:"-"` // Has hooks/ directory
    HasScripts  bool      `yaml:"-"` // Has scripts/ directory
}
```

**Loading:** Directory-based discovery via `LoadFromDirs()`

### Compiled Skills (`skills/compiled/`)

```go
type Skill interface {
    Name() string
    Description() string
    Tools() []skill.Tool
    Init(ctx context.Context) error
    Close() error
    SetStorage(kvs.Store)
}
```

**Loading:** Registered via `WithCompiledSkill()` option

### Remote MCP Skills (`skills/remote/mcp/`)

```go
type Config struct {
    Name        string
    Command     []string
    Env         map[string]string
    LazyConnect bool
}
```

**Loading:** Registered via `WithMCPSkill()` option

## Technical Requirements

### TR-0: Skill Pack Interface

Define the `SkillPack` interface in omniskill for markdown skill bundles.

**Package:** `github.com/plexusone/omniskill/pack`

```go
package pack

import "embed"

// SkillPack provides embedded markdown skills that can be loaded by agents.
// Skill packs are Go modules that bundle SKILL.md files using go:embed.
type SkillPack interface {
    // Name returns the pack identifier (e.g., "omniagent-skills").
    Name() string

    // Version returns the pack version or source commit hash.
    Version() string

    // FS returns the embedded filesystem containing skills.
    // Expected structure: skills/<skill-name>/SKILL.md
    FS() embed.FS
}
```

**Usage in omniagent:**

```go
// agent/options.go

func WithSkillPack(p pack.SkillPack) Option {
    return func(a *Agent) {
        a.skillPacks = append(a.skillPacks, p)
    }
}
```

### TR-1: Hybrid Loading Strategy

Implement a hybrid approach combining embedded and directory-based skill loading.

**Architecture:**

```
┌─────────────────────────────────────────────────────────┐
│                    SkillManager                         │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │  Embedded   │  │  Directory  │  │    Compiled     │ │
│  │   Skills    │  │   Skills    │  │     Skills      │ │
│  │  (go:embed) │  │  (runtime)  │  │  (Go packages)  │ │
│  └──────┬──────┘  └──────┬──────┘  └────────┬────────┘ │
│         │                │                   │          │
│         └────────────────┼───────────────────┘          │
│                          ▼                              │
│              ┌───────────────────────┐                  │
│              │   Merged Skill List   │                  │
│              │  (directory overrides │                  │
│              │      embedded)        │                  │
│              └───────────────────────┘                  │
└─────────────────────────────────────────────────────────┘
```

**Priority Order (highest to lowest):**

1. Directory skills (allow customization)
2. Embedded skills (bundled defaults)

**Implementation:**

```go
// skills/manager.go

type Manager struct {
    embedded  embed.FS         // Embedded skills
    dirs      []string         // Directory paths
    compiled  []compiled.Skill // Compiled skills
    loaded    map[string]*Skill
    mu        sync.RWMutex
}

type Config struct {
    EmbeddedFS   embed.FS
    SkillDirs    []string
    ExcludeNames []string // Skills to exclude
}

func NewManager(cfg Config) *Manager

func (m *Manager) Load(ctx context.Context) error
func (m *Manager) Get(name string) (*Skill, bool)
func (m *Manager) All() []*Skill
func (m *Manager) Available() []*Skill // Only skills with met requirements
func (m *Manager) InjectPrompt(base string) string
```

### TR-2: Embedded Skills Bundle

Bundle core skills using `go:embed`.

**Directory Structure:**

```
skills/
├── bundle/
│   ├── skills.go           # go:embed directives
│   ├── core/               # Embedded skill files
│   │   ├── github/
│   │   │   └── SKILL.md
│   │   ├── weather/
│   │   │   └── SKILL.md
│   │   ├── tmux/
│   │   │   └── SKILL.md
│   │   └── ... (18 skills)
│   └── bundle_test.go
├── loader.go               # Directory loader
├── manager.go              # Unified manager
└── skill.go                # Skill types
```

**Embedding:**

```go
// skills/bundle/skills.go
package bundle

import "embed"

//go:embed core/*
var Skills embed.FS
```

**Usage:**

```go
import "github.com/plexusone/omniagent/skills/bundle"

manager := skills.NewManager(skills.Config{
    EmbeddedFS: bundle.Skills,
    SkillDirs:  []string{"~/.omniagent/skills", "./skills"},
})
```

### TR-3: Skill Requirement Validation

Validate skill requirements before enabling.

**Requirements Types:**

| Type | Field | Description |
|------|-------|-------------|
| Binary | `bins` | Required executables on PATH |
| Any Binary | `anyBins` | At least one must exist |
| Environment | `env` | Required environment variables |

**Implementation:**

```go
// skills/requirements.go

type RequirementError struct {
    SkillName   string
    MissingBins []string
    MissingEnv  []string
}

func (s *Skill) CheckRequirements() []RequirementError
func (s *Skill) IsAvailable() bool
```

**Behavior:**

- Skills with unmet requirements are loaded but marked unavailable
- `Available()` returns only skills with met requirements
- `InjectPrompt()` only includes available skills
- CLI `skills check` shows requirement status

### TR-4: Agent Integration

Integrate skill manager with agent creation.

**Options:**

```go
// agent/options.go

func WithEmbeddedSkills(fs embed.FS) Option
func WithSkillDirs(dirs ...string) Option
func WithCompiledSkill(s compiled.Skill) Option
func WithMCPSkill(cfg mcp.Config) Option
func WithSkillExcludes(names ...string) Option
```

**Default Behavior:**

```go
// Default: use bundled skills + standard directories
agent, _ := omniagent.NewAgent(config)

// Custom: override embedded + add directories
agent, _ := omniagent.NewAgent(config,
    omniagent.WithEmbeddedSkills(customBundle),
    omniagent.WithSkillDirs("./my-skills"),
    omniagent.WithCompiledSkill(myGoSkill),
)
```

### TR-5: Skill Discovery CLI

Provide CLI commands for skill management.

**Commands:**

```bash
# List all skills with availability status
omniagent skills list

# Show skill details
omniagent skills info <name>

# Check requirements for all skills
omniagent skills check

# Show skill content (for debugging)
omniagent skills show <name>
```

**Output Format:**

```
$ omniagent skills list
NAME            STATUS      SOURCE      REQUIREMENTS
github          ✓ ready     embedded    gh
weather         ✓ ready     embedded    curl
tmux            ✓ ready     embedded    tmux
1password       ✗ missing   embedded    op (not found)
my-custom       ✓ ready     directory   -
```

## Data Flow

### Skill Loading Sequence

```
┌──────────┐     ┌─────────┐     ┌──────────┐     ┌─────────┐
│  Agent   │────▶│ Manager │────▶│  Loader  │────▶│  Skills │
│  Start   │     │  Load() │     │          │     │         │
└──────────┘     └────┬────┘     └──────────┘     └─────────┘
                      │
                      ▼
              ┌───────────────┐
              │ 1. Load embed │
              │ 2. Load dirs  │
              │ 3. Merge      │
              │ 4. Validate   │
              └───────────────┘
```

### Prompt Injection Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Agent     │────▶│  Manager    │────▶│   LLM       │
│ BuildPrompt │     │InjectPrompt │     │   Call      │
└─────────────┘     └─────────────┘     └─────────────┘
                           │
                           ▼
                    ┌─────────────────────┐
                    │ For each available: │
                    │ - Add skill header  │
                    │ - Add skill content │
                    │ - Add separator     │
                    └─────────────────────┘
```

## File Changes

### New Files

| File | Purpose |
|------|---------|
| `skills/manager.go` | Unified skill manager |
| `skills/bundle/skills.go` | Embedded skills via go:embed |
| `skills/bundle/core/*/SKILL.md` | 18 core skill files |
| `skills/bundle/bundle_test.go` | Bundle tests |

### Modified Files

| File | Change |
|------|--------|
| `agent/options.go` | Add `WithEmbeddedSkills`, `WithSkillDirs` |
| `agent/agent.go` | Initialize skill manager |
| `cmd/skills.go` | Update CLI commands |

## Testing Strategy

### Unit Tests

| Test | Description |
|------|-------------|
| `TestManager_LoadEmbedded` | Load skills from embed.FS |
| `TestManager_LoadDirectory` | Load skills from directory |
| `TestManager_MergeOverride` | Directory overrides embedded |
| `TestManager_RequirementCheck` | Validate requirements |
| `TestSkill_InjectPrompt` | Prompt injection format |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestAgent_WithSkills` | Agent creation with skills |
| `TestAgent_SkillInPrompt` | Skills appear in system prompt |
| `TestCLI_SkillsList` | CLI list command |

## Performance Considerations

| Aspect | Requirement |
|--------|-------------|
| Skill load time | <100ms for 50 skills |
| Memory per skill | <10KB average |
| Prompt overhead | <5KB for 20 skills |
| Requirement checks | Cached after first check |

## Security Considerations

1. **Skill content is trusted** - Injected into system prompt
2. **Directory permissions** - Only load from configured directories
3. **No code execution** - Markdown skills are passive content
4. **Binary checks** - Only check existence, don't execute

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `embed` | stdlib | Embedded file system |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML frontmatter parsing |
| `github.com/plexusone/omniskill` | v0.6.0 | Skill interfaces |

## Migration Path

### From v0.6.0

No breaking changes. Existing skill loading continues to work:

```go
// v0.6.0 (still works)
agent, _ := omniagent.NewAgent(config)

// v0.7.0 (new options available)
agent, _ := omniagent.NewAgent(config,
    omniagent.WithSkillDirs("./custom-skills"),
)
```

### Deprecations

None in this phase.
