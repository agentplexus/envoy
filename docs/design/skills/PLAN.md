# Skills System - Implementation Plan

**Version:** 1.0
**Date:** 2026-04-26
**Status:** Draft

## Overview

This plan outlines the implementation of hybrid skill loading and bundling of OpenClaw-compatible skills into OmniAgent.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  User Deployment                                                │
├─────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────┐  ┌────────────────────────────────┐ │
│  │   omniagent-skills     │  │   omniagent-skills-webdev      │ │
│  │   (markdown pack)      │  │   (compiled pack)              │ │
│  │                        │  │                                │ │
│  │  - github              │  │  - a11y (accessibility)        │ │
│  │  - weather             │  │  - release (release mgmt)      │ │
│  │  - tmux, notion, etc.  │  │                                │ │
│  └───────────┬────────────┘  └───────────────┬────────────────┘ │
│              │                               │                  │
│              └───────────┬───────────────────┘                  │
│                          │ depends on                           │
│               ┌──────────▼──────────┐                           │
│               │ plexusone/omniskill │  ◄── interfaces only      │
│               └──────────┬──────────┘                           │
│                          │ used by                              │
│               ┌──────────▼──────────┐                           │
│               │ plexusone/omniagent │  ◄── runtime              │
│               └─────────────────────┘                           │
└─────────────────────────────────────────────────────────────────┘
```

## Phase 0: Skill Pack Interface (omniskill)

Add `SkillPack` interface to omniskill:

```go
// omniskill/pack/pack.go
type SkillPack interface {
    Name() string
    Version() string
    FS() embed.FS
}
```

**Files:**

- `omniskill/pack/pack.go`

## Phase 1: Hybrid Loading Infrastructure

### 1.1 Skill Manager

Create a unified skill manager that handles multiple loading sources.

**Files:**

- `skills/manager.go` - Manager implementation
- `skills/manager_test.go` - Tests

**Implementation:**

```go
type Manager struct {
    embedded  embed.FS
    dirs      []string
    compiled  []compiled.Skill
    loaded    map[string]*Skill
    mu        sync.RWMutex
}

func NewManager(cfg Config) *Manager
func (m *Manager) Load(ctx context.Context) error
func (m *Manager) Get(name string) (*Skill, bool)
func (m *Manager) All() []*Skill
func (m *Manager) Available() []*Skill
func (m *Manager) InjectPrompt(base string) string
```

### 1.2 Embedded Skills Bundle

Set up go:embed infrastructure for bundled skills.

**Files:**

- `skills/bundle/skills.go` - Embed directives
- `skills/bundle/bundle_test.go` - Verify bundle loads correctly

**Directory Structure:**

```
skills/bundle/
├── skills.go
├── bundle_test.go
└── core/
    └── (skill files added in Phase 2)
```

### 1.3 Agent Integration

Wire skill manager into agent creation.

**Files:**

- `agent/options.go` - Add skill options
- `agent/agent.go` - Initialize manager

**New Options:**

```go
func WithEmbeddedSkills(fs embed.FS) Option
func WithSkillDirs(dirs ...string) Option
func WithSkillExcludes(names ...string) Option
```

## Phase 2: First Wave Skills (18 Skills)

### Skill Selection Criteria

Selected based on:

1. **Importance** - Critical or High value
2. **Ease** - E1 (no deps) or E2 (common CLI)
3. **Cross-platform** - Works on Linux/macOS/Windows

### 2.1 Core Skills (Critical)

| Skill | Binary | Priority |
|-------|--------|----------|
| github | `gh` | P0 |
| tmux | `tmux` | P0 |
| coding-agent | `claude`/`codex` | P1 |

### 2.2 Utility Skills (High)

| Skill | Binary | Priority |
|-------|--------|----------|
| weather | `curl` | P0 |
| notion | `curl` | P1 |
| slack | `curl` | P1 |
| trello | `curl` | P1 |
| gh-issues | `gh` | P1 |
| summarize | `summarize` | P2 |
| openai-whisper-api | `curl` | P1 |
| xurl | `curl` | P1 |
| healthcheck | `curl` | P2 |
| blogwatcher | `curl` | P2 |
| gemini | `curl` | P2 |

### 2.3 Meta Skills (Medium)

| Skill | Binary | Priority |
|-------|--------|----------|
| session-logs | None | P1 |
| oracle | None | P2 |
| skill-creator | None | P2 |
| goplaces | `curl` | P2 |

### 2.4 Copy Process

For each skill:

1. Copy `SKILL.md` from OpenClaw
2. Verify YAML frontmatter parses correctly
3. Test requirement checking
4. Add to bundle

**OpenClaw Source:** `/Users/johnwang/go/src/github.com/openclaw/openclaw/skills/`

## Phase 3: CLI Improvements

### 3.1 Enhanced Skills Command

```bash
# List with source indicator
omniagent skills list --source

# Filter by availability
omniagent skills list --available

# Check specific skill
omniagent skills check github

# Show skill content
omniagent skills show github
```

### 3.2 Output Formatting

```
$ omniagent skills list
NAME            STATUS      SOURCE      REQUIREMENTS
github          ✓ ready     embedded    gh
weather         ✓ ready     embedded    curl
custom-skill    ✓ ready     directory   -
1password       ✗ missing   embedded    op (not found)

Summary: 3 available, 1 unavailable, 4 total
```

## Implementation Order

```
Phase 0 (omniskill):
├── Add SkillPack interface
└── Release omniskill v0.7.0

Phase 1 (omniagent + omniagent-skills):
├── Create omniagent-skills repo
├── Add WithSkillPack option to omniagent
├── Skill Manager with pack loading
└── Agent integration

Phase 2 (omniagent-skills):
├── Copy 18 skills from OpenClaw
├── Core: github, tmux, coding-agent
├── Utility: weather, notion, slack, etc.
└── Meta: session-logs, oracle, skill-creator

Phase 3 (omniagent):
├── CLI improvements
├── Documentation
└── Release v0.7.0

Phase 4 (webdev skill pack):
├── Create omniagent-skills-webdev repo
├── Add "a11y" skill (wraps agent-a11y)
└── Add "release" skill (wraps agent-team-release)
```

## Phase 4: Webdev Skill Pack

Create `omniagent-skills-webdev` - a compiled skill pack for web development.

**Repository:** `github.com/plexusone/omniagent-skills-webdev`
**Dependencies:** `omniskill` (interfaces only), `agent-a11y`, `agent-team-release`

### Structure

```
omniagent-skills-webdev/
├── go.mod
├── pack.go              # CompiledSkillPack implementation
├── a11y/
│   └── skill.go         # Wraps agent-a11y
└── release/
    └── skill.go         # Wraps agent-team-release
```

### a11y Skill

WCAG accessibility auditing (wraps `agent-a11y`).

| Tool | Description |
|------|-------------|
| `audit_page` | Audit a single page for WCAG compliance |
| `audit_site` | Crawl and audit an entire website |
| `audit_journey` | Test accessibility of user flows |
| `gen_vpat` | Generate VPAT 2.4 compliance report |

### release Skill

Release management (wraps `agent-team-release`).

| Tool | Description |
|------|-------------|
| `check_release` | Validate release readiness |
| `gen_changelog` | Generate changelog from commits |
| `create_release` | Create GitHub release |

### Usage

```go
import (
    skills "github.com/plexusone/omniagent-skills"
    webdev "github.com/plexusone/omniagent-skills-webdev"
)

agent, _ := omniagent.NewAgent(config,
    omniagent.WithSkillPack(skills.Default()),         // markdown
    omniagent.WithCompiledSkillPack(webdev.Default()), // compiled
)
```

## Testing Plan

### Unit Tests

| Test | Coverage |
|------|----------|
| Manager loading | Embedded, directory, merge |
| Requirement checks | Bins, env, anyBins |
| Prompt injection | Format, ordering |
| YAML parsing | Frontmatter, metadata |

### Integration Tests

| Test | Coverage |
|------|----------|
| Agent with skills | Skill loading on startup |
| CLI commands | List, check, show |
| End-to-end | Skill appears in LLM prompt |

### Manual Testing

1. Create agent with embedded skills
2. Add custom skill to directory
3. Verify directory skill overrides embedded
4. Test requirement checking for missing binary
5. Verify CLI shows correct status

## Rollback Plan

If issues arise:

1. Skills are additive - no breaking changes
2. `WithSkillDirs()` defaults to current behavior
3. Bundle can be disabled via `WithEmbeddedSkills(nil)`

## Documentation Updates

| Document | Update |
|----------|--------|
| README.md | Add skills section |
| docs/guides/skills.md | Update with bundle info |
| CHANGELOG.md | v0.7.0 skills features |

## Success Criteria

| Metric | Target |
|--------|--------|
| Skills bundled | 18 |
| Load time | <100ms |
| Tests passing | 100% |
| CLI functional | All commands work |

## Open Questions

1. **Skill versioning** - Should we track OpenClaw commit hash for bundled skills?
   - **Decision:** Yes, record in `skills/bundle/VERSION`

2. **Skill updates** - How to update bundled skills?
   - **Decision:** Copy from OpenClaw, bump VERSION

3. **Custom skill format** - Allow non-OpenClaw SKILL.md format?
   - **Decision:** Support OpenClaw format only for compatibility

## References

- [OpenClaw Skills](https://github.com/openclaw/openclaw/tree/main/skills)
- [PRD.md](PRD.md) - Product requirements
- [TRD.md](TRD.md) - Technical requirements
- [TASKS.md](TASKS.md) - Implementation tasks
