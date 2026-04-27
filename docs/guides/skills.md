# Skills Development

OmniAgent supports two types of skills:

1. **Markdown Skills** - SKILL.md files compatible with [OpenClaw](https://github.com/openclaw/openclaw) format
2. **Compiled Skills** - Go packages that register callable tools with the agent

## Overview

**Markdown skills** inject instructions into the system prompt, teaching the agent how to use external tools.

**Compiled skills** register actual Go functions as LLM tools, enabling direct function calling.

## Skill Format

Skills are defined in `SKILL.md` files:

```markdown
---
name: weather
description: Get weather forecasts
metadata:
  emoji: "🌤️"
  requires:
    bins: ["curl"]
  install:
    - name: curl
      brew: curl
      apt: curl
---

# Weather Skill

You can check the weather using the `curl` command:

## Get Current Weather

```bash
curl "wttr.in/London?format=3"
```

## Get Detailed Forecast

```bash
curl "wttr.in/London"
```
```

## Skill Packs

Skill packs are pre-bundled collections of markdown skills embedded via `go:embed`. They provide a convenient way to distribute and share skills.

### Using a Skill Pack

```go
import (
    "github.com/plexusone/omniagent/agent"
    skills "github.com/plexusone/omniagent-skills"
)

// Load all skills from a skill pack
agent, err := agent.New(config,
    agent.WithSkillPack(skills.Default().FS()),
)
```

### Filtering Skills

Load only specific skills using includes:

```go
agent, err := agent.New(config,
    agent.WithSkillPack(skills.Default().FS()),
    agent.WithSkillIncludes("github", "weather", "tmux"),
)
```

Or exclude specific skills:

```go
agent, err := agent.New(config,
    agent.WithSkillPack(skills.Default().FS()),
    agent.WithSkillExcludes("notion", "slack"),
)
```

### Configuration File

Control skill loading via config:

```yaml
skills:
  enabled: true
  packs:
    - omniagent-skills  # Informational only
  paths:
    - ~/.omniagent/skills
  includes:
    - github
    - weather
  excludes:
    - deprecated-skill
  max_injected: 20
```

### Directory Override

Skills from directories take precedence over embedded pack skills with the same name. This allows customizing bundled skills:

```
~/.omniagent/skills/
└── github/
    └── SKILL.md  # Overrides the github skill from the pack
```

### Available Skill Packs

| Pack | Description | Skills |
|------|-------------|--------|
| [omniagent-skills](https://github.com/plexusone/omniagent-skills) | Default skill pack | 18 OpenClaw-compatible skills |

### Agent Options

| Option | Description |
|--------|-------------|
| `WithSkillPack(fs.FS)` | Register an embedded skill pack |
| `WithSkillDirs(...string)` | Set skill directories |
| `WithSkillIncludes(...string)` | Only load named skills |
| `WithSkillExcludes(...string)` | Skip named skills |
| `WithSkillManager(*Manager)` | Use a custom skill manager |

## Skill Discovery

Skills are discovered from:

1. Embedded skill packs (via `WithSkillPack`)
2. `~/.omniagent/skills/`
3. Custom paths via `skills.paths` config

```yaml
skills:
  enabled: true
  paths:
    - ~/.omniagent/skills
    - /opt/shared-skills
  max_injected: 20
```

## Managing Skills

### List Skills

```bash
omniagent skills list
```

Output:
```
✓ 🎵 sonoscli - Control Sonos speakers via CLI
✓ 🐙 github - GitHub CLI operations
✗ ☀️ weather - Weather forecasts (missing: weather binary)
```

### Show Skill Details

```bash
omniagent skills info sonoscli
```

### Check Requirements

```bash
omniagent skills check
```

## Requirements

Skills can declare requirements that must be met:

### Binary Requirements

```yaml
metadata:
  requires:
    bins: ["gh", "jq"]  # All required
    anyBins: ["curl", "wget"]  # At least one required
```

### Environment Variables

```yaml
metadata:
  requires:
    env: ["GITHUB_TOKEN", "OPENAI_API_KEY"]
```

### Install Hints

```yaml
metadata:
  install:
    - name: gh
      brew: gh
      apt: gh
    - name: jq
      brew: jq
      apt: jq
```

## Creating a Skill

### 1. Create Directory

```bash
mkdir -p ~/.omniagent/skills/myskill
```

### 2. Create SKILL.md

```markdown
---
name: myskill
description: My custom skill
metadata:
  emoji: "🔧"
---

# My Skill

Instructions for the AI agent on how to use this skill...

## Available Commands

- `mytool list` - List all items
- `mytool add <name>` - Add a new item
```

### 3. Verify

```bash
omniagent skills list
omniagent skills info myskill
```

## Best Practices

### Keep Instructions Clear

Write instructions as if explaining to a human who knows nothing about your tool.

### Include Examples

Show concrete examples of commands and expected output.

### Declare Requirements

Always declare binary and environment requirements so users know what's needed.

### Use Emojis Sparingly

One emoji in the metadata helps identify skills visually.

## ClawHub Compatibility

OmniAgent is compatible with skills from [ClawHub](https://github.com/clawhub). Install skills using:

```bash
# Coming soon
bunx clawhub install sonoscli
```

Or manually clone to your skills directory:

```bash
git clone https://github.com/user/skill ~/.omniagent/skills/skill
```

---

## Compiled Skills

Compiled skills are Go packages that register functions as LLM tools. They provide better performance and type safety compared to markdown skills.

### Creating a Compiled Skill

#### 1. Implement the Skill Interface

```go
package myskill

import (
    "context"
    "github.com/plexusone/omniagent/skills/compiled"
)

type MySkill struct {
    // skill state
}

func New() *MySkill {
    return &MySkill{}
}

func (s *MySkill) Name() string {
    return "myskill"
}

func (s *MySkill) Description() string {
    return "My custom skill description"
}

func (s *MySkill) Tools() []compiled.Tool {
    return []compiled.Tool{
        {
            Name:        "myskill_action",
            Description: "Performs an action",
            Parameters: map[string]compiled.Parameter{
                "input": {
                    Type:        "string",
                    Description: "The input value",
                    Required:    true,
                },
            },
            Handler: s.handleAction,
        },
    }
}

func (s *MySkill) Init(ctx context.Context) error {
    // Initialize resources
    return nil
}

func (s *MySkill) Close() error {
    // Cleanup resources
    return nil
}

func (s *MySkill) handleAction(ctx context.Context, params map[string]any) (any, error) {
    input := params["input"].(string)
    return map[string]any{"result": "Processed: " + input}, nil
}
```

#### 2. Register with Agent

```go
import "github.com/example/myskill"

skill := myskill.New()
agent, err := agent.New(config,
    agent.WithCompiledSkill(skill),
)
```

### Storage-Aware Skills

Skills that need persistent storage implement `StorageAware`:

```go
import "github.com/plexusone/omnistorage-core/kvs"

type MySkill struct {
    storage kvs.Store
}

func (s *MySkill) SetStorage(store kvs.Store) {
    s.storage = store
}
```

Storage is automatically injected when using `WithSessionsFromStorage` or `WithStorage`:

```go
agent.New(config,
    agent.WithSessionsFromStorage(backend),  // Injects storage
    agent.WithCompiledSkill(mySkill),        // Receives storage
)
```

### Parameter Types

| Type | JSON Schema Type | Go Type |
|------|------------------|---------|
| `"string"` | string | `string` |
| `"integer"` | integer | `int`, `int64` |
| `"number"` | number | `float64` |
| `"boolean"` | boolean | `bool` |
| `"array"` | array | `[]any` |
| `"object"` | object | `map[string]any` |

### Built-in Compiled Skills

OmniAgent includes these compiled skills:

| Skill | Tools | Description |
|-------|-------|-------------|
| `sessions` | `sessions_list`, `sessions_history`, etc. | Session management |

### Skill Lifecycle

```
agent.New()
    │
    ├── WithCompiledSkill(skill)
    │       └── RegisterCompiledSkill()
    │               ├── SetStorage() if StorageAware
    │               └── Register tools with ToolRegistry
    │
    ├── InitCompiledSkills()
    │       └── skill.Init(ctx) for each skill
    │
    ... agent runs ...
    │
    └── agent.Close()
            └── CloseCompiledSkills()
                    └── skill.Close() for each skill
```

### Best Practices

1. **Return Structured Data** - Return maps or structs, not plain strings
2. **Handle Errors Gracefully** - Return user-friendly error messages
3. **Validate Parameters** - Check required parameters before processing
4. **Use Descriptive Names** - Tool names should be `skillname_action` format
5. **Document Parameters** - Descriptions help the LLM use tools correctly
