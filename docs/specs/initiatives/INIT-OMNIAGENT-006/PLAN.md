# OmniAgent Agent/Skill Marketplace Primitives — Implementation Plan

> **Initiative:** `INIT-OMNIAGENT-006`
> **Status:** Draft
> **Date:** 2026-08-24

## Approach

One themed phase delivering a single leaf package plus its docs. The work
decomposes bottom-up: types first (no dependencies), then the provider and its
filtering/isolation logic over those types, then adapters bridging existing
OmniAgent types into listings, and finally the guide and docs wiring. Each RMI
maps to one file group and one atomic commit carrying a
`Refs: RMI-OMNIAGENT-<NNN>` trailer.

The package is intentionally dependency-light (stdlib + `agent/registry` +
`skills`) and imports nothing from `team/`/`gateway/`, so it can be adopted by
external host apps and verified with package-scoped unit tests alone.

## Phase 1 — Portable marketplace primitives

Sequenced by dependency order:

1. **RMI-OMNIAGENT-400 — Portable types** (`types.go`)
   Define `Visibility`, the `Ref` sub-types, `AgentListing`, `SkillListing`,
   `Catalog`, `Filter`, all with json+yaml tags. No behavior. Establishes the
   vocabulary every later RMI builds on. Foundational — no dependencies.

2. **RMI-OMNIAGENT-401 — Provider + StaticProvider** (`provider.go`,
   `provider_test.go`)
   Define the `Provider` interface and `ErrNotFound`; implement `StaticProvider`
   with register/catalog/get, filter matching, visibility gate, featured split,
   deterministic sort, and deep-clone isolation. Tests cover filtering,
   visibility, isolation, and context cancellation. Depends on 400.

3. **RMI-OMNIAGENT-402 — Adapters** (`adapters.go`)
   Bridge `agent/registry.AgentConfig` → `AgentListing` and
   `skills.Skill`/`skills.Manager` → `SkillListing`(s), omitting keys/prompts and
   copying required-secret names only. Depends on 400 (and on `agent/registry` +
   `skills` already existing in the module).

4. **RMI-OMNIAGENT-403 — Guide + docs wiring** (chore)
   Add `docs/guides/marketplace.md`; link it in `mkdocs.yml`; reference the
   package in `README.md` and the `CLAUDE.md` package map. Depends on 400–402
   being the thing documented.

## Validation

Per the repo's build/test/lint conventions:

- `go build ./marketplace/...` and `go build ./...`
- `go test ./marketplace/...` (package-scoped unit tests; no Postgres required)
- `golangci-lint run`
- Markdown/docs: confirm `mkdocs` nav resolves the new guide.

No Postgres/team test env is needed — the package has no `team/`/`gateway/`
dependency.

## Commit strategy

Four atomic commits, core → tests-with-code → docs, each with its RMI trailer:

- `feat(marketplace): portable agent/skill listing types` — `Refs: RMI-OMNIAGENT-400`
- `feat(marketplace): provider interface + in-memory static provider` — `Refs: RMI-OMNIAGENT-401`
- `feat(marketplace): adapters from agent/registry + skills` — `Refs: RMI-OMNIAGENT-402`
- `docs(marketplace): marketplace guide + docs wiring` — `Refs: RMI-OMNIAGENT-403`

(Tests ship in the same commit as the provider they exercise, per the file
grouping above.)

## Risks & mitigations

| Risk | Mitigation |
|------|------------|
| A listing accidentally carries a secret value or prompt | Adapters never read values; secret refs are name-only; a test asserts the public surface never exposes a value. Reinforced by `internal/redact` (INIT-004). |
| Map iteration makes catalog order nondescript | Deterministic `sortAgents`/`sortSkills` on a normalized key; tests assert ordering. |
| Caller mutates a returned listing and corrupts provider state | Deep-clone on every boundary crossing; isolation test. |
| Package accreting team/transport dependencies over time | Keep it a leaf: stdlib + `agent/registry` + `skills` only; team-mode/HTTP adapters live in the host app or a separate follow-on. |

## Out of scope (documented follow-ons)

- Team-mode `Provider` adapter over the `agents/` catalog service.
- Persistent/remote provider implementations.
- HTTP surface / shipped marketplace UI.
- Per-user install/entitlement state.
