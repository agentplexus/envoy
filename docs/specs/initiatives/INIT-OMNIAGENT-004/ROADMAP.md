# OmniAgent Skill Secrets — Roadmap

**Initiative:** `INIT-OMNIAGENT-004`
**Repository:** `github.com/plexusone/omniagent`
**Status:** In progress — the agent-scoped management flow (declaration + API +
UI) has shipped; the single-operator config-binding path has also shipped.

> RMI IDs are stable and permanent. Commits carry `Refs: RMI-OMNIAGENT-<NNN>`.
> Phase status is derived from member RMIs. **Core (v1a)** = Phase 1;
> **Team secrets (v1b)** = Phases 2–4. Cross-initiative dependencies on
> `INIT-OMNIAGENT-003` are noted per RMI.

> **Rescope (2026-08-10), reconciling this roadmap with the PRD's 2026-08-04
> agent-scoped note and two build decisions:**
>
> 1. **Agent-scoped, not per-user.** A secret belongs to an *agent*, set by its
>    owner/maintainer via `Can(CapConfigure)`; every chat with the agent uses
>    it (the PRD's model). The store, per-agent namespacing, authorization, and
>    injection were already built in `INIT-OMNIAGENT-005` RMI-310
>    (`team/secrets`); this initiative added the SKILL.md declaration, a
>    write-only management HTTP API, and the web UI on top.
> 2. **Reuse the existing OmniVault `memory`/`file` store**, not a new encrypted
>    Postgres table. This **descopes RMI-205 (encrypted `user_secrets` table +
>    RLS) and RMI-206 (AES-256-GCM envelope encryption)** — at-rest encryption
>    stays deferred and documented. The per-user `user_secrets` schema, per-turn
>    resolution, and cross-user isolation the original TRD described are not
>    built; agent-scoped resolution is once-per-instance and already isolated by
>    the per-agent namespace.
> 3. The omniskill `loader.Requirements` sync (mentioned under RMI-200/201) is
>    **descoped** — that struct lives in the external `omniskill` module and has
>    no consumer in omniagent; the sole live SKILL.md parser is `skills/skill.go`.
>
> **Shipped:** RMI-200 (declaration), RMI-207 + RMI-211 (agent secret service +
> HTTP API), RMI-212 (UI), RMI-209 (agent-scoped injection for both compiled
> and OpenAPI skills), RMI-203 (`Agent.filterSecretGated` wired into both
> `initSkillManager` and `LoadSkills` — a markdown skill with an unmet
> required secret is excluded with a logged reason instead of silently
> loading), RMI-201/202 (root `Config.Secrets` + per-skill
> `Skills.Config[name].Secrets` bindings, resolved by the same
> `ResolveCredentials` machinery already used for the 8 top-level infra
> credential fields, then merged and threaded into personal-mode's
> `agentFactory` via `agent.WithSecretEnv` — closing the loop with RMI-203's
> gating for personal mode, which previously never populated a secret env at
> all; `keeper://` **removed** rather than wired, since no Keeper provider
> exists anywhere in this module's dependency tree — it now gets the same
> immediate "unknown vault URI scheme" error as any other unrecognized
> scheme instead of an opaque failure deep inside omnivault; `aws-sm://` was
> never actually implemented and is not added here — no known provider
> package to add), RMI-208 (team-mode agent secret resolution now falls
> back through the RMI-201/202 config bindings when the per-agent vault
> doesn't have a value — per-agent secret ▸ per-skill global binding,
> scoped to that agent's own enabled skills ▸ global binding — closing the
> loop between the two secret systems that previously had zero awareness
> of each other), RMI-210 (rescoped in place: its original per-user MCP
> premise didn't survive the rescope — see its own checklist note below —
> shipped instead as `compiled.SecretRequirer`, extending RMI-203's
> exclude-don't-fail gating from markdown skills to compiled skills,
> implemented for MCP), and the redaction guarantee of RMI-204 (values
> never reach logs/prompt — injection targets skill env/auth only).
> RMI-213 also shipped (read-only `GET /api/admin/secret-bindings` +
> `renderAdmin()`'s Global Secret Bindings card, sourced from a
> `globalSecretBindings(cfg)` snapshot of `Config.Secrets` +
> `Skills.Config[*].Secrets` taken once at startup — names and set-state
> only, values never leave the process).
> **Follow-on (still open):** the rest of RMI-204 (no dedicated cross-type
> redaction test suite covering MCP/compiled/OpenAPI together — RMI-203's
> tests cover markdown-skill gating only; `SecretRequirer` could extend to
> OpenAPI as a natural follow-on).
> **Cancelled:** RMI-205, RMI-206.

## Phase 1 — Declaration, Binding & Injection

**Theme:** GitHub-style secret declaration in SKILL.md, omnivault-backed global bindings, per-skill-type injection. Single-operator; no team dependency.
**Status:** Planned — 0 of 5 items completed

- [x] `RMI-OMNIAGENT-200` Secret declaration in SKILL.md frontmatter
  - Acceptance: `requires.secrets` (name/description/required/env) parses in both SKILL.md loaders; a skill can only receive declared secrets
- [x] `RMI-OMNIAGENT-201` Reusable omnivault-backed SecretResolver
  - Shipped by extending the existing `config.ResolveCredentials` machinery
    (`resolveCredential`/`isVaultURI`/`isUnknownVaultURI`, already
    reusable/private) rather than adding a new exported resolver — one
    resolver instance now resolves the original 8 top-level infra fields
    plus `Config.Secrets` and every `Skills.Config[name].Secrets` map in the
    same pass. `op://`/`bw://`/`file://`/`env://` resolve as before, plain
    values pass through unchanged. `keeper://` **removed** from
    `vaultSchemes` (no provider registered anywhere in the dependency
    tree — was a silent runtime trap, not a working scheme). `aws-sm://`
    not added — never implemented, no known provider package.
- [x] `RMI-OMNIAGENT-202` Global + per-skill secret bindings in config
  - Depends on: `RMI-OMNIAGENT-201`
  - Shipped: root `Config.Secrets map[string]string` and
    `SkillsConfig.Config map[string]SkillConfig{Secrets}` (mirroring
    `TokenConfig.Services`'s map-by-name shape), resolved in place by
    `ResolveCredentials`, then merged (global base, per-skill overlay) and
    passed to `agent.WithSecretEnv` in `cmd/omniagent/commands/gateway.go`'s
    `agentFactory` — personal mode's single wiring point, since both the
    single-agent and multi-agent construction paths funnel through it.
    Errors are wrapped with the config path (`secrets.<name>` or
    `skills.config.<skill>.secrets.<name>`) for actionable messages. Known,
    documented limit: personal mode has one flat secret env per agent
    instance, so two skills binding the same env-var name to different
    values isn't representable — last-merged wins.
- [x] `RMI-OMNIAGENT-203` Per-skill-type injection + required-secret gating
  - Shipped without the RMI-202 dependency: compiled `SecretsAware` and OpenAPI
    `Auth` already receive only declared secrets (RMI-209's `SetSecrets`/
    `Auth.{APIKeyEnv,TokenEnv,PasswordEnv}` mapping). Required-secret gating
    applies to markdown SKILL.md skills via `Agent.filterSecretGated`
    (`agent/agent.go`, called from both `initSkillManager` and `LoadSkills`) —
    an unmet required secret drops the skill with a logged reason instead of
    a later opaque provider failure. Compiled/MCP/OpenAPI skills registered
    via `agent.Option` have no `Requires()`/declared-secrets concept, so
    there's nothing to gate on that path yet.
- [ ] `RMI-OMNIAGENT-204` Redaction + cross-type tests
  - Depends on: `RMI-OMNIAGENT-203`
  - Acceptance: resolved values are masked in logs and never enter prompt/transcript; tests cover MCP/compiled/OpenAPI with a fake vault

## Phase 2 — Per-User Encrypted Store (team mode)

**Theme:** Encrypted, RLS-scoped per-user secrets in the team PostgreSQL DB.
**Status:** Planned — 0 of 3 items completed

- [x] ~~`RMI-OMNIAGENT-205` `user_secrets` schema + RLS policies~~ — **cancelled** (reuse the OmniVault store; no new Postgres table — see rescope note)
- [x] ~~`RMI-OMNIAGENT-206` Envelope encryption (AES-256-GCM + omnivault KEK)~~ — **cancelled** (at-rest encryption deferred; `file`/`memory` providers protected by filesystem perms — see rescope note)
- [x] `RMI-OMNIAGENT-207` Per-user secret service (write-only values)
  - Depends on: `RMI-OMNIAGENT-205`, `RMI-OMNIAGENT-206`
  - Acceptance: set/delete/list-names + resolveForUser; RLS proves cross-user isolation; list never returns values

## Phase 3 — Per-User Resolution & Injection

**Theme:** Acting-user secret resolution with GitHub-style precedence.
**Status:** Planned — 0 of 3 items completed

- [x] `RMI-OMNIAGENT-208` Agent secret resolution and precedence
  - Depends on: `RMI-OMNIAGENT-207`
  - Shipped as agent-scoped (not per-user — see rescope note), and layered
    against the RMI-201/202 single-operator config bindings rather than a
    `secrets.FromContext` accessor (no such function exists anywhere in the
    repo; that naming predates the rescope). Precedence: per-agent secret
    (`team/secrets.Service.ResolveAgentSecrets`) ▸ per-skill global binding
    (`Skills.Config[name].Secrets`, scoped to exactly that agent's own
    enabled skills — never the deployment's full skill set, which would
    leak a binding into an agent that doesn't have the skill enabled) ▸
    global binding (`Config.Secrets`). Implemented in
    `cmd/omniagent/commands/team.go`'s `mergeSecretEnv` +
    `agentSecretSource`, fed by `team/agentruntime.Builder.Build` now
    passing the agent's `cfg.Skills` through the widened `SecretSource`
    interface.
- [x] `RMI-OMNIAGENT-209` Agent-scoped injection for compiled + OpenAPI skills
  - Shipped as agent-scoped (not per-user — see rescope note): `agent.WithSecretEnv` +
    `compiled.SecretsAware` for compiled skills, `openapi.Config.Auth.{APIKeyEnv,TokenEnv,PasswordEnv}`
    + `Skill.SetSecrets` for OpenAPI skills. One resolved secret set per agent instance,
    isolated by the per-agent OmniVault namespace — not per-acting-user.
- [x] `RMI-OMNIAGENT-210` Required-secret gating for compiled skills
  - Depends on: `RMI-OMNIAGENT-208`
  - **Rescoped in place** — the original acceptance criteria didn't survive
    the agent-scoped rescope: MCP already receives agent-scoped secrets
    identically to compiled/OpenAPI (a side effect of RMI-209, since
    `mcp.Skill` implements `compiled.SecretsAware`), so there's no
    disconnected per-user MCP env path left to warn about or fall back
    from — "subprocess env fixed at spawn" is simply how every agent
    instance's secrets already work now (one instance, one resolved
    secret set, one spawn). And "a member is told which secret to add"
    presumed a reactive in-chat prompt that exists nowhere, aimed at an
    actor with no path to act on it (only owner/maintainer can set an
    agent secret).
  - **What shipped instead** — the real remaining gap: RMI-203's gating
    (unmet required secret excludes a skill with a clear log, instead of
    a later opaque failure) only ever covered markdown `skills.Skill`;
    compiled skills (MCP, OpenAPI, anything registered via `agent.Option`)
    had no `Requires()`/declared-secrets concept at all, so one MCP skill
    with a missing required env var could fail however its subprocess's
    `Init()` reacted — and because `Agent.InitCompiledSkills` propagates
    any `Init()` error as a hard failure aborting the whole loop, that
    could take an agent's *entire* compiled-skill set down, not just
    itself. Added `compiled.SecretRequirer` (optional interface,
    `RequiredSecrets() []string`) and a soft-skip gate in
    `Agent.RegisterCompiledSkill` — mirrors `filterSecretGated`'s
    exclude-don't-fail behavior. Implemented for MCP via
    `Config.RequiredEnv`; not retrofitted onto OpenAPI in this pass (a
    natural, separately-scoped follow-on).

## Phase 4 — Secrets Management UI (team mode)

**Theme:** Members manage their own secrets by env-var name, GitHub-style.
**Status:** Planned — 0 of 3 items completed

- [x] `RMI-OMNIAGENT-211` Secrets HTTP API
  - Depends on: `RMI-OMNIAGENT-207`
  - Acceptance: `/api/secrets` list-names/put/delete (CSRF); `/api/skills/secrets` catalog with per-caller set-state; values never returned
- [x] `RMI-OMNIAGENT-212` Settings › Secrets UI panel
  - Depends on: `RMI-OMNIAGENT-211`, `RMI-OMNIAGENT-003` web UI (`RMI-OMNIAGENT-115`)
  - Acceptance: per-skill declared secrets shown with paste-to-set, set/unset indicator, delete; values write-only after save
- [x] `RMI-OMNIAGENT-213` Admin global-bindings view + docs
  - Depends on: `RMI-OMNIAGENT-212`, `RMI-OMNIAGENT-003` admin UI (`RMI-OMNIAGENT-119`)
  - Shipped: `GlobalSecretBinding{Name,Source,Set}` + `GET
    /api/admin/secret-bindings` (superadmin-only, mirrors
    `handleListUsers`'s `h.actor` gate; 401 unauthenticated, 403
    non-superadmin — covered by `TestTeamHTTP_AdminSecretBindings` against
    a real Postgres instance, and manually verified live against a
    trial gateway). Snapshot built once at startup by
    `cmd/omniagent/commands/team.go`'s `globalSecretBindings(cfg)`
    (sorted by source then name) and threaded into
    `TeamHTTPConfig.GlobalSecretBindings` — no per-request recomputation,
    since these config-file bindings are static for the process's
    lifetime. `web/dist/app.js`'s `globalSecretBindingsCard()` renders it
    read-only in the Admin tab, reusing RMI-212's `secret-row`/`badge`
    visual language. `docs/guides/team-mode.md`'s Admin section documents
    the card and route.
