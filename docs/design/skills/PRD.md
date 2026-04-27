# Skills System - Product Requirements Document

**Version:** 1.0
**Date:** 2026-04-26
**Status:** Draft

## Overview

OmniAgent needs a comprehensive skills system that allows agents to be extended with domain-specific capabilities. This document defines the requirements for integrating OpenClaw-compatible skills into OmniAgent, including markdown skills, compiled skills, and remote MCP skills.

## Goals

1. **Extensibility** - Allow users to add new capabilities without modifying core agent code
2. **Compatibility** - Support OpenClaw SKILL.md format for ecosystem interoperability
3. **Flexibility** - Multiple loading strategies (embedded, directory, registry)
4. **Simplicity** - Easy to add, discover, and use skills

## User Stories

### US-1: Developer adds a custom skill

> As a developer, I want to add a custom markdown skill to my agent so that it can perform domain-specific tasks.

**Acceptance Criteria:**

- Place SKILL.md in a configured directory
- Agent discovers and loads the skill on startup
- Skill content is injected into the system prompt
- Skill requirements are checked before enabling

### US-2: Agent uses bundled skills

> As a user, I want the agent to come with useful skills out-of-the-box so I can be productive immediately.

**Acceptance Criteria:**

- Core skills (github, weather, etc.) are embedded in the binary
- No external file dependencies for bundled skills
- Skills work without additional configuration

### US-3: Developer creates a compiled skill

> As a developer, I want to create a Go-native skill with custom tools so I can implement complex functionality.

**Acceptance Criteria:**

- Implement the `compiled.Skill` interface
- Register skill with the agent at creation time
- Tools appear in the agent's tool list
- Tool execution works seamlessly

### US-4: Agent connects to MCP server

> As a user, I want to connect my agent to an MCP server so I can use external tools.

**Acceptance Criteria:**

- Configure MCP server connection (command or URL)
- Agent discovers tools from the MCP server
- Tools are callable like native tools
- Connection lifecycle is managed properly

## Skill Types

### Markdown Skills (SKILL.md)

Markdown-based skills following the OpenClaw format:

- YAML frontmatter with metadata
- Markdown body with instructions
- Optional hooks/ and scripts/ directories
- Requirement checking (bins, env vars)

**Use Cases:**

- Command-line tool wrappers (gh, curl, ffmpeg)
- API integration instructions
- Workflow documentation
- Prompt engineering patterns

### Compiled Skills

Go packages implementing the skill interface:

- Native Go tool implementations
- Full type safety and IDE support
- Direct access to OmniAgent internals
- Storage and session integration

**Use Cases:**

- Complex business logic
- Performance-critical operations
- Deep agent integration
- Custom tool implementations

### Remote MCP Skills

External MCP server connections:

- Connect to stdio or SSE transports
- Discover tools from remote servers
- Proxy tool calls to the server
- Manage process lifecycle

**Use Cases:**

- Language-specific tool servers (Python, Node.js)
- Shared tool infrastructure
- Third-party integrations
- Sandboxed execution

## Skill Importance Classification

### Critical (Core agent capabilities)

| Skill | Type | Description |
|-------|------|-------------|
| github | Markdown | GitHub CLI operations |
| coding-agent | Markdown | Sub-agent delegation |
| tmux | Markdown | Remote session control |
| 1password | Markdown | Secrets management |

### High (Significantly useful)

| Skill | Type | Description |
|-------|------|-------------|
| weather | Markdown | Weather information (demo quality) |
| notion | Markdown | Notion API integration |
| slack | Markdown | Slack messaging |
| trello | Markdown | Trello boards |
| summarize | Markdown | URL/video summarization |
| openai-whisper | Markdown | Speech-to-text |
| gh-issues | Markdown | GitHub issues |
| obsidian | Markdown | Note-taking |
| session-logs | Markdown | Session history |

### Medium (Nice to have)

| Skill | Type | Description |
|-------|------|-------------|
| video-frames | Markdown | Video frame extraction |
| nano-pdf | Markdown | PDF operations |
| xurl | Markdown | URL utilities |
| oracle | Markdown | Structured queries |
| taskflow | Markdown | Task orchestration |
| skill-creator | Markdown | Meta-skill creation |
| blogwatcher | Markdown | RSS/blog monitoring |
| healthcheck | Markdown | Service monitoring |
| gemini | Markdown | Gemini API |

### Low (Specialized/Niche)

| Skill | Type | Description |
|-------|------|-------------|
| spotify-player | Markdown | Music control |
| sonoscli | Markdown | Speaker control |
| openhue | Markdown | Smart lighting |
| himalaya | Markdown | Email CLI |

### Platform-Specific (macOS only)

| Skill | Description |
|-------|-------------|
| apple-notes | Apple Notes integration |
| apple-reminders | Apple Reminders |
| bear-notes | Bear app |
| things-mac | Things 3 |
| imsg | iMessage |
| peekaboo | Screen capture |
| camsnap | Camera capture |

## Non-Goals

1. **Custom skill runtimes** - No WASM/Docker skill execution (use sandbox/ package)
2. **Skill marketplace** - No built-in store (use ClawHub)
3. **Skill versioning** - No version constraints (rely on git/embed)
4. **Hot reloading** - No runtime skill updates (restart required)

## Success Metrics

| Metric | Target |
|--------|--------|
| Core skills bundled | 18+ |
| Skill load time | <100ms |
| Custom skill setup | <5 minutes |
| MCP connection time | <2 seconds |

## Dependencies

- `github.com/plexusone/omniskill` - Skill interface definitions
- `github.com/modelcontextprotocol/go-sdk` - MCP client
- OpenClaw SKILL.md format specification

## Skill Pack Architecture

Skill packs are generic omniskill packages that depend only on `omniskill` interfaces, not on `omniagent`. This makes them reusable by any omniskill-compatible agent.

```
┌─────────────────────────────────────────────────────────────────┐
│  User Deployment                                                │
├─────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────┐  ┌────────────────────────────────┐ │
│  │   omniagent-skills     │  │   omniagent-skills-webdev      │ │
│  │   (markdown pack)      │  │   (compiled pack)              │ │
│  │                        │  │                                │ │
│  │  - github              │  │  - a11y (accessibility audit)  │ │
│  │  - weather             │  │  - release (release mgmt)      │ │
│  │  - tmux                │  │                                │ │
│  │  - notion              │  │                                │ │
│  │  - ...18 skills        │  │                                │ │
│  └───────────┬────────────┘  └───────────────┬────────────────┘ │
│              │                               │                  │
│              └───────────────┬───────────────┘                  │
│                              │                                  │
│                   ┌──────────▼──────────┐                       │
│                   │ plexusone/omniskill │  ◄── interfaces only  │
│                   └──────────┬──────────┘                       │
│                              │                                  │
│                   ┌──────────▼──────────┐                       │
│                   │ plexusone/omniagent │  ◄── runtime          │
│                   └─────────────────────┘                       │
└─────────────────────────────────────────────────────────────────┘
```

### Skill Packs

| Pack | Type | Contents | Depends On |
|------|------|----------|------------|
| `omniagent-skills` | Markdown | 18 OpenClaw skills | `omniskill` only |
| `omniagent-skills-webdev` | Compiled | a11y, release | `omniskill`, `agent-a11y`, `agent-team-release` |

### Example: Markdown Skill Pack

```go
// plexusone/omniagent-skills/pack.go
package skills

import "embed"

//go:embed skills/*
var skillsFS embed.FS

const Version = "d4eb23652362a1b7d3fbcebd633a1c6f2a43c16f"

type Pack struct{}

func (Pack) Name() string    { return "omniagent-skills" }
func (Pack) Version() string { return Version }
func (Pack) FS() embed.FS    { return skillsFS }

func Default() *Pack { return &Pack{} }
```

### Example: Compiled Skill

```go
// plexusone/agent-a11y/skill/skill.go
package skill

type Skill struct {
    auditor *a11y.Auditor
}

func (s *Skill) Name() string        { return "a11y" }
func (s *Skill) Description() string { return "WCAG accessibility auditing tools" }
func (s *Skill) Tools() []skill.Tool { return []skill.Tool{...} }
```

### Usage in Agent Deployment

```go
import (
    "github.com/plexusone/omniagent"
    skills "github.com/plexusone/omniagent-skills"
    webdev "github.com/plexusone/omniagent-skills-webdev"
)

agent, _ := omniagent.NewAgent(config,
    // Base markdown skills (github, weather, tmux, etc.)
    omniagent.WithSkillPack(skills.Default()),

    // Web development compiled skills (a11y, release)
    omniagent.WithCompiledSkillPack(webdev.Default()),

    // Local overrides
    omniagent.WithSkillDirs("./custom-skills"),
)
```

## Timeline

| Phase | Scope | Target |
|-------|-------|--------|
| Phase 1 | Hybrid loading (embed + directory) | v0.7.0 |
| Phase 2 | First wave skills (18 skills) | v0.7.0 |
| Phase 3 | MCP skill improvements | v0.8.0 |
| Phase 4 | Registry integration | Future |
