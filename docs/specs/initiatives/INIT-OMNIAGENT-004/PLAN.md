# OmniAgent Skill Secrets — Implementation Plan

> **Initiative:** `INIT-OMNIAGENT-004`
> Phases and RMI IDs match [ROADMAP.md](ROADMAP.md). Each phase is executed and
> reviewed as a unit; phase status derives from its RMIs. Commits carry
> `Refs: RMI-OMNIAGENT-<NNN>`.

## Package Layout

```
skills/secrets/          # core: declaration types, Resolver, Injector, redactor
  ├── secret.go          # SecretRequirement, resolved-set, context accessor
  ├── resolver.go        # omnivault-backed Resolver (reused by config/)
  ├── inject.go          # per-skill injection (MCP Env / SecretsAware / OpenAPI)
  └── redact.go          # log/output redaction
team/                    # per-user store (team mode)
  ├── ent/schema/usersecret.go
  └── secrets/           # SetUserSecret/…/resolveForUser + AES-256-GCM sealing
gateway/team_secrets_http.go   # /api/secrets, /api/skills/secrets
web/                     # Settings › Secrets panel (INIT-003 UI)
```

The core (`skills/secrets`) has **no team dependency**; per-user pieces live under
`team/` and the gateway, gated by team mode.

## Phase 1 — Declaration, Binding & Injection (core, single-operator)

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-200 | `SecretRequirement` + `Requires.Secrets` in `skills/skill.go` and the omniskill `loader` struct; frontmatter parsing + docs |
| RMI-OMNIAGENT-201 | `skills/secrets.Resolver` over `omnivault.Resolver.ResolveString`; refactor `config/credentials.go` to consume it (one resolver); wire or drop `keeper://` |
| RMI-OMNIAGENT-202 | Config `secrets:` map + `skills.config.<name>.secrets`; validation; load-time resolution of global bindings |
| RMI-OMNIAGENT-203 | Injection: MCP `Env` population, `SecretsAware` interface + `RegisterCompiledSkill` wiring, OpenAPI `Auth`; required-secret availability gating |
| RMI-OMNIAGENT-204 | Log/output redaction of resolved values; unit tests across all skill types (fake vault) |

**Gate:** a declared secret bound in config reaches an MCP and a compiled skill;
an unbound required secret disables the skill with an actionable message; no
value appears in logs.

## Phase 2 — Per-User Encrypted Store (team mode)

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-205 | `user_secrets` ent schema + migration + RLS policies (owner-only; system context for injection) |
| RMI-OMNIAGENT-206 | Envelope encryption: KEK from `team.secrets.encryption_key` via omnivault; AES-256-GCM seal/open with name+skill as AAD |
| RMI-OMNIAGENT-207 | `team/secrets` service: SetUserSecret/DeleteUserSecret/ListUserSecretNames (write-only values) + `resolveForUser`; RLS + crypto tests |

**Depends:** `INIT-OMNIAGENT-101` (RLS store). **Gate:** per-user secrets
round-trip encrypted; RLS proves member A cannot read member B's rows; values
never returned by list.

## Phase 3 — Per-User Resolution & Injection

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-208 | `secrets.FromContext` acting-user secret set; precedence per-user ▸ per-skill global ▸ global |
| RMI-OMNIAGENT-209 | Compiled + OpenAPI skills read per-user secrets from context (one shared instance, per-user values); agent turn wires acting user → resolved set |
| RMI-OMNIAGENT-210 | MCP global-only documentation + guardrail (per-user MCP declarations warn); unset-required-secret prompt surfaced to the member in chat |

**Depends:** Phase 2, `INIT-OMNIAGENT-113` (acting-user/chat). **Gate:** a skill in
member A's chat uses A's secret, in B's chat uses B's; MCP falls back to global
with a clear log.

## Phase 4 — Secrets Management UI (team mode)

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-211 | HTTP: `/api/secrets` (list-names/put/delete, CSRF) + `/api/skills/secrets` (declared-secrets catalog with set-state) |
| RMI-OMNIAGENT-212 | UI: Settings › Secrets panel — per-skill declared secrets, paste-to-set, set/unset indicator, delete; values write-only |
| RMI-OMNIAGENT-213 | Admin view of global bindings (names + set-state, no values); docs + testing guide |

**Depends:** Phase 3, `INIT-OMNIAGENT-115`/`119` (web UI + admin surface).

## Milestones

- **Core (v1a)** = Phase 1 — usable by single-operator deployments immediately.
- **Team secrets (v1b)** = Phases 2–4 — per-user secrets with the UI, on top of
  team mode.

## Definition of Done (per plexusone org)

Every RMI: follows repo patterns; unit tests (RLS + crypto where applicable;
no secret values in test logs); `golangci-lint` clean; docs updated on
user-visible change; conventional commit with `Refs: RMI-OMNIAGENT-<NNN>`.
