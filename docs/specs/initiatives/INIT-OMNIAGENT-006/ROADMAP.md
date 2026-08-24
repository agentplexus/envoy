# OmniAgent Agent/Skill Marketplace Primitives — Roadmap

**Initiative:** `INIT-OMNIAGENT-006`
**Repository:** `github.com/plexusone/omniagent`
**Status:** Delivered in the working tree; pending commit + status advance in
VisionStudio.

> RMI IDs are stable and permanent. Commits carry `Refs: RMI-OMNIAGENT-<NNN>`.
> Phase status is derived from member RMIs. Every item below is **implemented in
> the `marketplace/` package** already; the RMIs remain `proposed` in
> VisionStudio only until each is committed with its trailer, at which point it
> advances to `completed`. `origin=implementation` reflects that these RMIs were
> scoped from delivered code.

## Phase 1 — Portable marketplace primitives

**Theme:** Reusable, storage-agnostic agent/skill catalog model for host apps
(UIForge, OmniRoadmap, VisionStudio) — portable listings, filters, a provider
interface, an in-memory static provider, and adapters from existing OmniAgent
types.
**Status:** Implemented — 4 of 4 items delivered in-tree (pending commit)

- [x] `RMI-OMNIAGENT-400` Portable marketplace types (listings, refs, filter, catalog)
  - File: `marketplace/types.go`
  - Acceptance: `AgentListing`/`SkillListing`/`Catalog`/`Filter` plus
    `Capability`/`Tool`/`Skill`/`Secret`/`RequirementRef` and `Visibility`
    compile with json+yaml tags on every exported field; `SecretRef` exposes
    name/description/required/env only (no value); the capability namespace is a
    host-owned opaque string OmniAgent never enumerates.

- [x] `RMI-OMNIAGENT-401` Provider interface + in-memory StaticProvider with filtering
  - Depends on: `RMI-OMNIAGENT-400`
  - Files: `marketplace/provider.go`, `marketplace/provider_test.go`
  - Acceptance: `Provider` (`Catalog`/`GetAgent`/`GetSkill`, `ErrNotFound`) is
    satisfied by `StaticProvider`; filters match case-insensitively on
    query/tags/category/capabilities/tools with empty fields matching all;
    non-listed agents are hidden unless `Filter.IncludePrivate`; featured and
    listed agents return in separate, stably-sorted groups; skills included only
    when `IncludeSkills`; returned listings are deep-cloned (caller mutation
    can't corrupt provider state); a cancelled context short-circuits. Tests
    cover filtering, visibility, featured split, isolation, and cancellation.

- [x] `RMI-OMNIAGENT-402` Adapters from `agent/registry` + `skills`
  - Depends on: `RMI-OMNIAGENT-400`
  - File: `marketplace/adapters.go`
  - Acceptance: `AgentListingFromConfig` maps a `registry.AgentConfig` and
    **omits API keys and prompts**; `SkillListingFromSkill` projects
    `Requires` into `RequirementRef`s and copies required-**secret names** only
    (no values); `SkillListingsFromManager` maps all loaded skills; all three
    are nil-safe.

- [x] `RMI-OMNIAGENT-403` Marketplace guide + docs wiring
  - Type: chore · Depends on: `RMI-OMNIAGENT-400`, `-401`, `-402`
  - Files: `docs/guides/marketplace.md`, `mkdocs.yml`, `README.md`, `CLAUDE.md`
  - Acceptance: the guide documents the model, a Go usage example, and the
    UIForge/OmniRoadmap integration shape (app-owned capability strings); it is
    linked in the `mkdocs` nav; `README.md` and the `CLAUDE.md` package map
    reference the `marketplace/` package.

## Follow-ons (not scoped by this initiative)

- A team-mode `Provider` adapter wrapping the `agents/` catalog service (team
  mode's service layer stays the source of truth for visibility, roles, and
  start-chat authorization).
- A persistent/DB-backed or remote `Provider` implementation.
- An HTTP surface or shipped marketplace UI in OmniAgent (host apps render the
  `Catalog`).
- Per-user install/entitlement state.
- UIForge and OmniRoadmap adopting the package and supplying their capability
  strings.
