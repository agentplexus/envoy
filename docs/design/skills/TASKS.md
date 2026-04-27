# Skills System - Implementation Tasks

**Version:** 1.0
**Date:** 2026-04-26
**Status:** Draft

## Task Overview

| Phase | Tasks | Status |
|-------|-------|--------|
| Phase 0: Skill Pack Interface | 2 | Pending |
| Phase 1: Infrastructure | 5 | Pending |
| Phase 2: Skills Bundle | 4 | Pending |
| Phase 3: CLI & Docs | 3 | Pending |
| Phase 4: Webdev Skill Pack | 2 | Pending |
| **Total** | **16** | |

---

## Phase 0: Skill Pack Interface

### TASK-490: Add SkillPack Interface to omniskill

**Priority:** P0
**Estimate:** Small
**Dependencies:** None

Add the `SkillPack` interface to omniskill for markdown skill bundles.

**Files:**

- [ ] `omniskill/pack/pack.go`

**Interface:**

```go
package pack

import "embed"

// SkillPack provides embedded markdown skills.
type SkillPack interface {
    // Name returns the pack identifier.
    Name() string

    // Version returns the pack version or source commit.
    Version() string

    // FS returns the embedded filesystem containing skills.
    // Skills are expected at skills/<name>/SKILL.md
    FS() embed.FS
}
```

**Acceptance Criteria:**

- [ ] Interface defined in omniskill
- [ ] Documentation with usage example
- [ ] Release omniskill with pack interface

---

### TASK-491: Create omniagent-skills Repository

**Priority:** P0
**Estimate:** Medium
**Dependencies:** TASK-490

Create the default markdown skill pack repository.

**Repository:** `github.com/plexusone/omniagent-skills`

**Structure:**

```
omniagent-skills/
├── go.mod
├── pack.go              # SkillPack implementation
├── VERSION              # OpenClaw commit hash
└── skills/              # (populated in Phase 2)
    └── .gitkeep
```

**Acceptance Criteria:**

- [ ] Repository created
- [ ] Implements `omniskill.SkillPack` interface
- [ ] VERSION file tracks OpenClaw commit
- [ ] go.mod with correct dependencies
- [ ] Basic tests

---

## Phase 1: Hybrid Loading Infrastructure

### TASK-500: Create Skill Manager

**Priority:** P0
**Estimate:** Medium
**Dependencies:** None

Create unified skill manager that handles embedded and directory loading.

**Files:**

- [ ] `skills/manager.go`
- [ ] `skills/manager_test.go`

**Acceptance Criteria:**

- [ ] `NewManager(cfg)` creates manager with config
- [ ] `Load(ctx)` loads from embedded FS and directories
- [ ] `Get(name)` returns skill by name
- [ ] `All()` returns all loaded skills
- [ ] `Available()` returns skills with met requirements
- [ ] `InjectPrompt(base)` injects available skills into prompt
- [ ] Directory skills override embedded skills with same name
- [ ] Unit tests pass

---

### TASK-501: Set Up Embedded Bundle Infrastructure

**Priority:** P0
**Estimate:** Small
**Dependencies:** None

Create go:embed structure for bundled skills.

**Files:**

- [ ] `skills/bundle/skills.go`
- [ ] `skills/bundle/bundle_test.go`
- [ ] `skills/bundle/VERSION`
- [ ] `skills/bundle/core/` (empty directory)

**Acceptance Criteria:**

- [ ] `bundle.Skills` is an `embed.FS`
- [ ] Bundle compiles with empty core/ directory
- [ ] VERSION file tracks OpenClaw commit hash
- [ ] Test verifies bundle is accessible

---

### TASK-502: Add Agent Skill Options

**Priority:** P0
**Estimate:** Small
**Dependencies:** TASK-500

Add functional options for skill configuration.

**Files:**

- [ ] `agent/options.go`

**Acceptance Criteria:**

- [ ] `WithEmbeddedSkills(fs)` sets embedded skill source
- [ ] `WithSkillDirs(dirs...)` sets skill directories
- [ ] `WithSkillExcludes(names...)` excludes specific skills
- [ ] Options are documented with examples

---

### TASK-503: Integrate Manager with Agent

**Priority:** P0
**Estimate:** Small
**Dependencies:** TASK-500, TASK-502

Wire skill manager into agent initialization.

**Files:**

- [ ] `agent/agent.go`

**Acceptance Criteria:**

- [ ] Agent creates skill manager during init
- [ ] Default: uses bundled skills + standard directories
- [ ] Skills are loaded before first LLM call
- [ ] Skill errors are logged but don't fail agent creation
- [ ] `agent.Skills()` returns manager for inspection

---

### TASK-504: Skill Requirement Caching

**Priority:** P1
**Estimate:** Small
**Dependencies:** TASK-500

Cache requirement check results for performance.

**Files:**

- [ ] `skills/requirements.go`

**Acceptance Criteria:**

- [ ] Binary existence checks are cached
- [ ] Environment variable checks are cached
- [ ] Cache invalidation via `InvalidateRequirementCache()`
- [ ] Cache is per-manager, not global

---

## Phase 2: Skills Bundle

### TASK-510: Bundle Core Skills

**Priority:** P0
**Estimate:** Medium
**Dependencies:** TASK-501

Copy core skills from OpenClaw to bundle.

**Skills (4):**

| Skill | Source |
|-------|--------|
| github | `openclaw/skills/github/SKILL.md` |
| tmux | `openclaw/skills/tmux/SKILL.md` |
| weather | `openclaw/skills/weather/SKILL.md` |
| coding-agent | `openclaw/skills/coding-agent/SKILL.md` |

**Files:**

- [ ] `skills/bundle/core/github/SKILL.md`
- [ ] `skills/bundle/core/tmux/SKILL.md`
- [ ] `skills/bundle/core/weather/SKILL.md`
- [ ] `skills/bundle/core/coding-agent/SKILL.md`

**Acceptance Criteria:**

- [ ] YAML frontmatter parses correctly
- [ ] Requirement checking works
- [ ] Skills load via bundle.Skills
- [ ] Update VERSION with OpenClaw commit hash

---

### TASK-511: Bundle Utility Skills

**Priority:** P1
**Estimate:** Medium
**Dependencies:** TASK-501

Copy utility skills from OpenClaw to bundle.

**Skills (10):**

| Skill | Source |
|-------|--------|
| notion | `openclaw/skills/notion/SKILL.md` |
| slack | `openclaw/skills/slack/SKILL.md` |
| trello | `openclaw/skills/trello/SKILL.md` |
| gh-issues | `openclaw/skills/gh-issues/SKILL.md` |
| summarize | `openclaw/skills/summarize/SKILL.md` |
| openai-whisper-api | `openclaw/skills/openai-whisper-api/SKILL.md` |
| xurl | `openclaw/skills/xurl/SKILL.md` |
| healthcheck | `openclaw/skills/healthcheck/SKILL.md` |
| blogwatcher | `openclaw/skills/blogwatcher/SKILL.md` |
| gemini | `openclaw/skills/gemini/SKILL.md` |

**Files:**

- [ ] `skills/bundle/core/notion/SKILL.md`
- [ ] `skills/bundle/core/slack/SKILL.md`
- [ ] `skills/bundle/core/trello/SKILL.md`
- [ ] `skills/bundle/core/gh-issues/SKILL.md`
- [ ] `skills/bundle/core/summarize/SKILL.md`
- [ ] `skills/bundle/core/openai-whisper-api/SKILL.md`
- [ ] `skills/bundle/core/xurl/SKILL.md`
- [ ] `skills/bundle/core/healthcheck/SKILL.md`
- [ ] `skills/bundle/core/blogwatcher/SKILL.md`
- [ ] `skills/bundle/core/gemini/SKILL.md`

**Acceptance Criteria:**

- [ ] All skills parse correctly
- [ ] Requirement checking works for each
- [ ] Bundle test verifies all 10 load

---

### TASK-512: Bundle Meta Skills

**Priority:** P2
**Estimate:** Small
**Dependencies:** TASK-501

Copy meta/utility skills from OpenClaw to bundle.

**Skills (4):**

| Skill | Source |
|-------|--------|
| session-logs | `openclaw/skills/session-logs/SKILL.md` |
| oracle | `openclaw/skills/oracle/SKILL.md` |
| skill-creator | `openclaw/skills/skill-creator/SKILL.md` |
| goplaces | `openclaw/skills/goplaces/SKILL.md` |

**Files:**

- [ ] `skills/bundle/core/session-logs/SKILL.md`
- [ ] `skills/bundle/core/oracle/SKILL.md`
- [ ] `skills/bundle/core/skill-creator/SKILL.md`
- [ ] `skills/bundle/core/goplaces/SKILL.md`

**Acceptance Criteria:**

- [ ] All skills parse correctly
- [ ] Bundle test verifies all 4 load

---

### TASK-513: Bundle Integration Test

**Priority:** P1
**Estimate:** Small
**Dependencies:** TASK-510, TASK-511, TASK-512

Verify complete bundle works end-to-end.

**Files:**

- [ ] `skills/bundle/bundle_test.go` (expand)

**Acceptance Criteria:**

- [ ] All 18 skills load from bundle
- [ ] `InjectPrompt()` includes all available skills
- [ ] Skill names are unique
- [ ] No parsing errors

---

## Phase 3: CLI & Documentation

### TASK-520: Enhance Skills CLI

**Priority:** P1
**Estimate:** Medium
**Dependencies:** TASK-503

Improve skills CLI commands with better output.

**Files:**

- [ ] `cmd/skills.go`

**Commands:**

- [ ] `skills list` - Show all skills with status
- [ ] `skills list --available` - Filter to available
- [ ] `skills list --source` - Show embedded vs directory
- [ ] `skills check` - Check all requirements
- [ ] `skills check <name>` - Check specific skill
- [ ] `skills show <name>` - Display skill content

**Acceptance Criteria:**

- [ ] Formatted table output
- [ ] Status indicators (✓/✗)
- [ ] Source column (embedded/directory)
- [ ] Summary line with counts

---

### TASK-521: Update Skills Documentation

**Priority:** P2
**Estimate:** Small
**Dependencies:** TASK-513

Update documentation for new skills features.

**Files:**

- [ ] `docs/guides/skills.md`
- [ ] `README.md`

**Acceptance Criteria:**

- [ ] Document hybrid loading strategy
- [ ] Document bundled skills list
- [ ] Document custom skill creation
- [ ] Add CLI command examples

---

### TASK-522: Add CHANGELOG Entry

**Priority:** P2
**Estimate:** Small
**Dependencies:** TASK-521

Add v0.7.0 changelog entry for skills features.

**Files:**

- [ ] `CHANGELOG.json`
- [ ] Regenerate `CHANGELOG.md`

**Acceptance Criteria:**

- [ ] List all new skills features
- [ ] Note breaking changes (if any)
- [ ] Validate with schangelog

---

## Task Dependencies

```
TASK-500 (Manager) ─────┬──▶ TASK-502 (Options) ──▶ TASK-503 (Integration)
                        │
TASK-501 (Bundle Infra) ┼──▶ TASK-510 (Core Skills)
                        │
                        ├──▶ TASK-511 (Utility Skills)
                        │
                        └──▶ TASK-512 (Meta Skills)

TASK-510, 511, 512 ─────────▶ TASK-513 (Bundle Test)

TASK-503 (Integration) ─────▶ TASK-520 (CLI)

TASK-513 (Bundle Test) ─────▶ TASK-521 (Docs) ──▶ TASK-522 (Changelog)
```

## Phase 4: Webdev Skill Pack

Create `omniagent-skills-webdev` - a compiled skill pack for web development.

**Repository:** `github.com/plexusone/omniagent-skills-webdev`
**Dependencies:** `omniskill` (interfaces only), `agent-a11y`, `agent-team-release`

### TASK-530: Create "a11y" Compiled Skill

**Priority:** P1
**Estimate:** Medium
**Dependencies:** None (parallel with other phases)

Create "a11y" compiled skill for WCAG accessibility auditing.

**Source:** `github.com/plexusone/agent-a11y`
**Skill Name:** `a11y`

**Files:**

- [ ] `skill/skill.go`
- [ ] `skill/skill_test.go`

**Tools to expose:**

| Tool | Description |
|------|-------------|
| `audit_page` | Audit a single page for WCAG compliance |
| `audit_site` | Crawl and audit an entire website |
| `audit_journey` | Test accessibility of user flows |
| `gen_vpat` | Generate VPAT 2.4 compliance report |

**Implementation:**

```go
package skill

import (
    "github.com/plexusone/agent-a11y"
    "github.com/plexusone/omniskill/skill"
)

type Skill struct {
    auditor *a11y.Auditor
    config  Config
}

type Config struct {
    // Level is the WCAG conformance level (A, AA, AAA)
    Level string
}

func New(cfg Config) (*Skill, error)

func (s *Skill) Name() string        { return "a11y" }
func (s *Skill) Description() string { return "WCAG accessibility auditing" }
func (s *Skill) Tools() []skill.Tool { ... }
```

**Acceptance Criteria:**

- [ ] Implements `compiled.Skill` interface
- [ ] Skill name is `a11y` (generic)
- [ ] Wraps existing auditor functionality
- [ ] Supports WCAG levels A, AA, AAA
- [ ] Unit tests pass

---

### TASK-531: Create "release" Compiled Skill

**Priority:** P1
**Estimate:** Medium
**Dependencies:** None (parallel with other phases)

Create "release" compiled skill for release management.

**Repository:** `github.com/plexusone/agent-team-release`
**Skill Name:** `release`

**Files:**

- [ ] `skill/skill.go`
- [ ] `skill/skill_test.go`

**Tools to expose:**

| Tool | Description |
|------|-------------|
| `check_release` | Validate release readiness |
| `gen_changelog` | Generate changelog from commits |
| `create_release` | Create GitHub release |

**Implementation:**

```go
package skill

import (
    "github.com/plexusone/agent-team-release/pkg/release"
    "github.com/plexusone/omniskill/skill"
)

type Skill struct {
    manager *release.Manager
    config  Config
}

type Config struct {
    // GitHubToken for GitHub API access
    GitHubToken string
}

func New(cfg Config) (*Skill, error)

func (s *Skill) Name() string        { return "release" }
func (s *Skill) Description() string { return "Release management" }
func (s *Skill) Tools() []skill.Tool { ... }
```

**Acceptance Criteria:**

- [ ] Implements `compiled.Skill` interface
- [ ] Skill name is `release` (generic)
- [ ] Integrates with agent-team-release workflow
- [ ] Supports GitHub releases
- [ ] Unit tests pass

---

## OpenClaw Reference

**Commit Hash:** `d4eb23652362a1b7d3fbcebd633a1c6f2a43c16f`
**Local Path:** `/Users/johnwang/go/src/github.com/openclaw/openclaw`
**Skills Directory:** `/Users/johnwang/go/src/github.com/openclaw/openclaw/skills/`

## Skill Ease Classification

For reference when implementing:

| Level | Description | Count |
|-------|-------------|-------|
| E1 | Pure markdown, no deps | 3 |
| E2 | Markdown + common CLI (curl, gh) | 11 |
| E3 | Needs specific binary | 4 |
| **Total** | | **18** |

## Completion Checklist

- [ ] Phase 1 complete (TASK-500 through TASK-504)
- [ ] Phase 2 complete (TASK-510 through TASK-513)
- [ ] Phase 3 complete (TASK-520 through TASK-522)
- [ ] All tests passing
- [ ] Documentation updated
- [ ] Changelog generated
- [ ] Ready for v0.7.0 release
