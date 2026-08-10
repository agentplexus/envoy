# OmniAgent Skill Secrets — Roadmap

**Initiative:** `INIT-OMNIAGENT-004`
**Repository:** `github.com/plexusone/omniagent`
**Status:** In progress — the agent-scoped management flow (declaration + API +
UI) has shipped; the single-operator config-binding path is a follow-on.

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
> HTTP API), RMI-212 (UI), and the redaction guarantee of RMI-204 (values never
> reach logs/prompt — injection targets skill env/auth only). **Follow-on
> (still open):** RMI-201/202 (reusable resolver + single-operator config
> bindings, `keeper://` wiring), the OpenAPI-skill injection half of RMI-209,
> and RMI-203/210's config-binding gating. **Cancelled:** RMI-205, RMI-206.

## Phase 1 — Declaration, Binding & Injection

**Theme:** GitHub-style secret declaration in SKILL.md, omnivault-backed global bindings, per-skill-type injection. Single-operator; no team dependency.
**Status:** Planned — 0 of 5 items completed

- [x] `RMI-OMNIAGENT-200` Secret declaration in SKILL.md frontmatter
  - Acceptance: `requires.secrets` (name/description/required/env) parses in both SKILL.md loaders; a skill can only receive declared secrets
- [ ] `RMI-OMNIAGENT-201` Reusable omnivault-backed SecretResolver
  - Acceptance: one resolver serves config + skills; op/bw/file/env/aws-sm refs resolve, plain values pass through; `keeper://` wired or removed
- [ ] `RMI-OMNIAGENT-202` Global + per-skill secret bindings in config
  - Depends on: `RMI-OMNIAGENT-201`
  - Acceptance: `secrets:` and `skills.config.<name>.secrets` resolve at load; validation errors are actionable
- [ ] `RMI-OMNIAGENT-203` Per-skill-type injection + required-secret gating
  - Depends on: `RMI-OMNIAGENT-200`, `RMI-OMNIAGENT-202`
  - Acceptance: MCP `Env`, compiled `SecretsAware`, OpenAPI `Auth` receive only declared secrets; unbound required secret disables the skill with a clear message
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

- [ ] `RMI-OMNIAGENT-208` Acting-user secret context + precedence
  - Depends on: `RMI-OMNIAGENT-207`
  - Acceptance: `secrets.FromContext` returns the acting user's set; precedence per-user ▸ per-skill global ▸ global
- [ ] `RMI-OMNIAGENT-209` Per-user injection for compiled + OpenAPI skills
  - Depends on: `RMI-OMNIAGENT-208`, `RMI-OMNIAGENT-003` chats (`RMI-OMNIAGENT-113`)
  - Acceptance: same skill instance uses A's secret in A's chat and B's in B's; no cross-user leakage
- [ ] `RMI-OMNIAGENT-210` MCP global-only guardrail + unset-secret prompt
  - Depends on: `RMI-OMNIAGENT-208`
  - Acceptance: per-user MCP declarations warn and fall back to global; a member is told which secret to add when a required one is unset

## Phase 4 — Secrets Management UI (team mode)

**Theme:** Members manage their own secrets by env-var name, GitHub-style.
**Status:** Planned — 0 of 3 items completed

- [x] `RMI-OMNIAGENT-211` Secrets HTTP API
  - Depends on: `RMI-OMNIAGENT-207`
  - Acceptance: `/api/secrets` list-names/put/delete (CSRF); `/api/skills/secrets` catalog with per-caller set-state; values never returned
- [x] `RMI-OMNIAGENT-212` Settings › Secrets UI panel
  - Depends on: `RMI-OMNIAGENT-211`, `RMI-OMNIAGENT-003` web UI (`RMI-OMNIAGENT-115`)
  - Acceptance: per-skill declared secrets shown with paste-to-set, set/unset indicator, delete; values write-only after save
- [ ] `RMI-OMNIAGENT-213` Admin global-bindings view + docs
  - Depends on: `RMI-OMNIAGENT-212`, `RMI-OMNIAGENT-003` admin UI (`RMI-OMNIAGENT-119`)
  - Acceptance: superadmin sees global binding names + set-state (no values); testing guide covers the full member flow
