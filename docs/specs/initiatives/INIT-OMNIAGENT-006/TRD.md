# OmniAgent Agent/Skill Marketplace Primitives — Technical Requirements Document

> **Initiative:** `INIT-OMNIAGENT-006`
> **Status:** Draft
> **Date:** 2026-08-24
> **Home package:** `marketplace/` (module root of `github.com/plexusone/omniagent`)

## Overview

A new leaf package `marketplace/` defines the portable listing/filter/catalog
model, a `Provider` interface, an in-memory `StaticProvider`, and adapters from
existing OmniAgent types. It depends only on the standard library plus
`agent/registry` and `skills` (in the adapters file). It imports nothing from
`team/`, `gateway/`, or any storage/transport package, keeping it a
dependency-light seam host apps can adopt cheaply.

## Package layout

```
marketplace/
├── types.go          # portable listing/ref/catalog/filter types
├── provider.go       # Provider interface + StaticProvider + filtering
├── adapters.go       # AgentListingFromConfig, SkillListingFrom{Skill,Manager}
└── provider_test.go  # provider + filter + isolation tests
```

## Types (`types.go`)

- **`Visibility`** — `private` | `unlisted` | `listed`. Empty is treated as
  listed by the visibility gate.
- **`CapabilityRef`** — `{Name, Description, Scope, Risk, Metadata}`. `Name` is a
  host-owned opaque string (e.g. `uiforge.query.run`). OmniAgent never
  enumerates capability names.
- **`ToolRef`** — `{Name, Description, Capability, Risk, Metadata}`. An
  executable action; `Capability` optionally names the capability that
  authorizes/explains it.
- **`SkillRef`** — `{Name, Required, Metadata}`. Attaches a skill to an agent
  listing.
- **`SecretRef`** — `{Name, Description, Required, Env}`. Describes a secret a
  consumer may need to bind. **Never carries a value.**
- **`RequirementRef`** — `{Type, Name, Any}`. A local runtime prerequisite
  (`binary`, `binary_any`, `env`).
- **`AgentListing`** — portable agent description: id/slug/name/description/
  version/provider/model/category/icon/tags, visibility, featured, enabled,
  skills, tools, capabilities, metadata, created/updated timestamps.
  **No API key, no system prompt.**
- **`SkillListing`** — portable skill description: name/display-name/
  description/homepage/version/source/category/icon/tags, available, required
  secrets (names only), requirements, tools, capabilities, metadata.
- **`Catalog`** — `{FeaturedAgents, Agents, Skills}` — a view a host app renders
  directly.
- **`Filter`** — `{Query, Tags, Category, Capabilities, Tools, IncludePrivate,
  IncludeSkills}`.

Every exported field carries both `json` and `yaml` struct tags so host apps can
serialize in either format.

## Provider (`provider.go`)

```go
type Provider interface {
    Catalog(ctx context.Context, filter Filter) (*Catalog, error)
    GetAgent(ctx context.Context, id string) (*AgentListing, error)
    GetSkill(ctx context.Context, name string) (*SkillListing, error)
}
var ErrNotFound = errors.New("marketplace listing not found")
```

`StaticProvider` is an in-memory implementation:

- Backed by `map[string]AgentListing` / `map[string]SkillListing`, guarded by a
  `sync.RWMutex`. Keys are derived by `listingKey` (id ▸ slug ▸ name, normalized).
- `NewStaticProvider(agents, skills)`, `RegisterAgent`, `RegisterSkill` for
  population; register is idempotent (add-or-replace by key).
- **Context honored** — `Catalog`/`GetAgent`/`GetSkill` return `ctx.Err()` early
  if the context is already cancelled.
- **Visibility gate** — `agentVisible` hides non-listed agents unless
  `Filter.IncludePrivate`. Empty visibility counts as listed.
- **Filtering** — `agentMatches`/`skillMatches` AND together: case-insensitive
  substring `Query` across identifying fields, exact-ish `Category`, and
  subset-match for `Tags`, `Capabilities`, `Tools`. Empty filter fields match
  all. Skills only included when `Filter.IncludeSkills`.
- **Featured split** — matching agents partition into `FeaturedAgents` (those
  with `Featured=true`) and `Agents`.
- **Deterministic order** — `sortAgents`/`sortSkills` sort by a normalized
  display key (name ▸ id, or display-name ▸ name), stable, independent of map
  iteration.
- **Isolation** — every value crossing the boundary is deep-cloned
  (`cloneAgent`/`cloneSkill` copy slices and metadata maps), so a caller
  mutating a returned listing cannot corrupt provider state.

## Adapters (`adapters.go`)

- **`AgentListingFromConfig(*registry.AgentConfig) AgentListing`** — maps id,
  name, description, provider, model, enabled, allowed-tools → `ToolRef`s, and
  timestamps. Marks `VisibilityListed`. **Intentionally omits API keys and
  prompts.** Nil-safe (returns zero value).
- **`SkillListingFromSkill(*skills.Skill) SkillListing`** — maps name,
  description, homepage, source, emoji→icon, availability; projects
  `Metadata.OpenClaw.Requires` into `RequirementRef`s (`binary`/`binary_any`/
  `env`) and `SecretRef`s (**name/description/required/env only**, never
  values). Nil-safe.
- **`SkillListingsFromManager(*skills.Manager) []SkillListing`** — maps all
  loaded skills via `SkillListingFromSkill`. Nil-safe.

## Constraints & invariants

- **No secret values or prompts** ever enter a listing. This is a
  build-time/structural guarantee (adapters don't read values) reinforced by
  tests, and complements `internal/redact` (INIT-OMNIAGENT-004).
- **No new dependencies** beyond stdlib + already-vendored `agent/registry` and
  `skills`.
- **No team/gateway import** — the package stays adoptable by external host apps
  without dragging in team-mode storage.

## Testing (`provider_test.go`)

- Filter matching: query, category, tags, capabilities, tools; empty-filter
  match-all; case-insensitivity.
- Visibility: private/unlisted excluded unless `IncludePrivate`.
- Featured/listed split and deterministic ordering.
- `GetAgent`/`GetSkill` by id/slug/name and `ErrNotFound`.
- Isolation: mutating a returned listing does not affect subsequent reads.
- Cancelled-context short-circuit.

Package-scoped unit tests only; no Postgres, no network — the package has no
such dependencies.

## Documentation

`docs/guides/marketplace.md` describes the model, a Go usage example, the
UIForge integration shape (capability strings the app owns), and how team
mode's existing catalog can be adapted to the `Provider` interface without
changing its authorization. Linked from `mkdocs.yml`; referenced from
`README.md` and the `CLAUDE.md` package map.

## Out of scope (future work)

- A team-mode `Provider` adapter wrapping the `agents/` catalog service.
- A persistent/DB-backed or remote `Provider`.
- An HTTP surface or shipped marketplace UI in OmniAgent.
- Per-user install/entitlement state.
