# OmniAgent Agent/Skill Marketplace Primitives — Product Requirements Document

> **Initiative:** `INIT-OMNIAGENT-006`
> **Status:** Draft
> **Date:** 2026-08-24

## Problem Statement

Host applications that embed OmniAgent — UIForge, OmniRoadmap, VisionStudio —
each want to present a **catalog of agents and skills** a user can browse,
filter, and enable. Today the only catalog concept lives inside OmniAgent's
team mode (`INIT-OMNIAGENT-005`, `agents/` service + catalog), tightly coupled
to team-mode storage, RLS, and authorization. There is no **portable,
storage-agnostic** description of "an agent you can offer" or "a skill you can
attach" that a host app can depend on without pulling in team-mode Postgres.

The predictable result is duplication: every host app invents its own
`AgentCard`/`SkillCard`/`Listing` shapes, its own filter semantics, and its own
notion of visibility and "featured", none of which interoperate. Worse, ad-hoc
listing structs risk leaking material that must never travel in a catalog
payload — API keys, prompts, secret values.

## Vision

OmniAgent **owns the reusable marketplace model** for agent and skill
discovery. It defines portable **listings**, a **filter**, a **catalog** view,
and a **provider interface** — and nothing about where listings are stored.
Host applications decide whether listings come from static config, team-mode
storage, a SpiceDB-backed policy layer, or a remote registry, by implementing
one small interface. The same marketplace UI and provider contract can be
reused across UIForge, OmniRoadmap, and VisionStudio.

Two properties are load-bearing:

- **Storage-agnostic.** The package defines types and an interface, plus one
  in-memory `StaticProvider` for embedded catalogs and tests. No database, no
  transport, no auth model baked in.
- **Safe by construction.** A listing is a *description*, not an install. Secret
  declarations are exposed by **name only**; secret values, API keys, and
  prompts are never part of a marketplace payload.

## Users

| Role | Need |
|------|------|
| **Host-app developer** (UIForge, OmniRoadmap) | Depend on one portable listing/filter/provider model instead of inventing their own; supply app-specific capability names as strings. |
| **OmniAgent maintainer** | A single marketplace vocabulary that team mode's existing catalog can adapt to without changing its authorization model. |
| **End user** (indirect) | Browse and filter a coherent catalog of agents/skills the host app renders directly from a `Catalog`. |

## User Stories

- **US-1**: As a host-app developer, I describe an agent I want to offer as an
  `AgentListing` (provider/model metadata, skills, tools, capabilities,
  visibility, featured) without embedding the agent's API key or system prompt.
- **US-2**: As a host-app developer, I describe a reusable skill as a
  `SkillListing`, and its required secrets appear by **name** only — never with
  values.
- **US-3**: As a host-app developer, I attach **my** application's capability
  strings (e.g. `uiforge.query.run`, `omniroadmap.roadmap.read`) to listings;
  OmniAgent filters and presents them without understanding my domain.
- **US-4**: As a host-app developer, I query a `Provider` with a `Filter`
  (free-text query, tags, category, capabilities, tools) and get back a
  `Catalog` split into featured and listed agents, plus optional skills, that I
  can render directly.
- **US-5**: As a host-app developer, I mark listings `private`/`unlisted`/
  `listed`, and private listings are excluded from a catalog unless I explicitly
  ask to include them.
- **US-6**: As an OmniAgent maintainer, I convert an existing runtime agent
  config (`agent/registry.AgentConfig`) or a loaded markdown skill into a
  portable listing with a provided adapter, so I don't hand-map fields (and
  don't accidentally copy secrets/prompts).
- **US-7**: As a host-app developer, the listings a provider returns are
  **isolated copies** — mutating a returned listing never corrupts the
  provider's stored state.

## Requirements

### Must Have

1. **Portable listing types** — `AgentListing` and `SkillListing` with
   provider/model metadata, category, tags, icon, visibility, featured/enabled,
   and `Capability`/`Tool`/`Skill`/`Secret`/`RequirementRef` sub-types.
   JSON + YAML tags on every field so host apps can serialize either way.
2. **Secret safety** — `SecretRef` and skill listings carry secret **names**
   (and the env var they inject as), never values. Adapters never read a secret
   value.
3. **Host-owned capability namespace** — a `CapabilityRef` is an opaque,
   app-owned string; OmniAgent does not enumerate or validate app capabilities.
4. **Catalog + Filter** — a `Catalog` (featured agents, listed agents, optional
   skills) and a `Filter` (query, tags, category, capabilities, tools,
   include-private, include-skills).
5. **Provider interface** — `Provider` with `Catalog`, `GetAgent`, `GetSkill`;
   `ErrNotFound` for misses. Implementations may be static, DB-backed, remote,
   or authorization-aware.
6. **Static provider** — an in-memory `StaticProvider` implementing `Provider`,
   with register methods, filter/visibility/featured handling, deterministic
   sort, and deep-clone isolation on every return.
7. **Adapters** — convert `agent/registry.AgentConfig` → `AgentListing` and
   loaded `skills.Skill`/`skills.Manager` → `SkillListing`(s), omitting keys and
   prompts and copying required-secret names only.
8. **Documentation** — a guide describing the model and the UIForge/OmniRoadmap
   integration shape, linked from the docs site.

### Should Have

- Filter matching that is case-insensitive and treats empty filter fields as
  "match all".
- Deterministic, stable ordering of catalog entries independent of map
  iteration order.

### Non-Goals

- A persistent or database-backed provider (host apps supply their own; team
  mode's catalog can be adapted later).
- An HTTP/transport layer or a shipped marketplace UI in OmniAgent (host apps
  render the `Catalog`).
- An authorization model in the marketplace package — the host app / team-mode
  service layer remains the source of truth for visibility, roles, and
  start-chat authorization.
- User **installs**/entitlements — listings are definitions, not per-user
  install state.
- Rewriting team mode's existing catalog; this initiative provides the portable
  model it can adopt without changing its authorization behavior.

## Success Criteria

- A host app can build a catalog and filter it (by query + capability) using
  only the `marketplace` package, with no team-mode/Postgres dependency.
- An `AgentListing` produced by the config adapter contains no API key and no
  system prompt; a `SkillListing` produced by the skill adapter contains
  required-secret names but no values.
- Private listings are excluded from a `Catalog` unless `IncludePrivate` is set;
  featured and listed agents are returned in separate, stably-sorted groups.
- Mutating a listing returned by `StaticProvider` does not change what a
  subsequent `Catalog`/`GetAgent` call returns.

## Relationship to Other Initiatives

- **Builds on `INIT-OMNIAGENT-005`** (Virtual Agents, Roles & Registry): that
  initiative created team mode's in-app catalog; this one extracts a **portable,
  storage-agnostic** model that team mode's catalog can be adapted to without
  changing its authorization.
- **Consumed by host apps** — UIForge and OmniRoadmap depend on this package and
  supply their own capability strings; the same provider contract is reusable in
  VisionStudio.
