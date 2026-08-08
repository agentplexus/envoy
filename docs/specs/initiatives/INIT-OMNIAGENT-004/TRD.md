# OmniAgent Skill Secrets — Technical Requirements Document

> **Initiative:** `INIT-OMNIAGENT-004`
> **Status:** Draft
> **Companion:** [PRD.md](PRD.md) · [PLAN.md](PLAN.md) · [ROADMAP.md](ROADMAP.md)

## 1. Architecture Overview

Four layers, mirroring GitHub Actions (declare → store → bind → inject). Three
of the four already exist in the codebase and are reused, not rebuilt.

```
  SKILL.md frontmatter               config.yaml / team UI            omnivault + team DB
  ┌───────────────────┐             ┌────────────────────┐          ┌──────────────────┐
  │ requires.secrets: │  declares   │ secrets:           │  binds   │ op:// bw:// file://│
  │  - name: GITHUB…  │────────────▶│   GITHUB_TOKEN: op…│─────────▶│ env:// aws-sm://  │  (global/operator)
  │    env: GITHUB…   │             │ skills.config.<s>. │          │                   │
  │    required: true │             │   secrets: {…}     │          │ user_secrets(RLS, │  (per-user, team)
  └───────────────────┘             └────────────────────┘          │  AES-256-GCM)     │
            │                                                        └──────────────────┘
            │                          ┌───────────────────────────────────┐
            └─────────────────────────▶│  SecretResolver + Injector        │
                                       │  precedence: per-user ▸ global    │
                                       │  routes to: MCP Env / SecretsAware│
                                       │  / OpenAPI Auth ; redacts logs    │
                                       └───────────────────────────────────┘
```

**Reuse map:**

| Layer | Mechanism | Source |
|-------|-----------|--------|
| Store (global) | omnivault providers + `Resolver.ResolveString` | `omnivault` (dep) |
| Store (per-user) | new `user_secrets` ent table, RLS-scoped, encrypted | `team/ent` (INIT-003) |
| Bind | config `secrets:` + per-skill; team UI writes per-user | new + `config/credentials.go` engine |
| Declare | `Requires.Secrets` frontmatter | extends `skills/skill.go:44` |
| Inject | MCP `Env`, new `SecretsAware`, OpenAPI `Auth` | `skills/remote/mcp`, `skills/compiled/skill.go:32` pattern |

## 2. Declaration

Extend the existing `metadata.openclaw.requires` block (`skills/skill.go`):

```go
type SecretRequirement struct {
    Name        string `json:"name"`                  // logical secret name
    Description string `json:"description,omitempty"`  // shown in the UI
    Required    bool   `json:"required,omitempty"`     // gates skill availability
    Env         string `json:"env,omitempty"`          // env var to inject as; defaults to Name
}
// Requires gains: Secrets []SecretRequirement `json:"secrets,omitempty"`
```

The omniskill loader's `Requirements` struct (`loader/types.go`) gains the same
field to keep the two SKILL.md parsers in sync. Declaration is the **allowlist**:
a skill can only ever receive secrets it declared.

## 3. Resolution & Injection

### 3.1 SecretResolver

Generalize the resolver currently hardwired to a field list in
`config/credentials.go` into a reusable component:

```go
// package skills/secrets (new)
type Resolver interface {
    // Resolve returns the value for a bound reference: an omnivault URI is
    // resolved; a plain string passes through. Empty ref → "", false.
    Resolve(ctx context.Context, ref string) (value string, err error)
}
```

Backed by `omnivault.Resolver.ResolveString` for global bindings. `config/
credentials.go` is refactored to consume this so there is one resolver.

### 3.2 Binding sources & precedence

For skill `S`, secret declaration `D` (name `N`, env `E`), acting user `U`:

1. **Per-user** (team mode, `U` set): `user_secrets` row for `(U, S, N)` or
   `(U, "*", N)` — decrypt in memory.
2. **Per-skill global**: `skills.config.<S>.secrets[N]` → resolve.
3. **Global**: `secrets[N]` → resolve.
4. Else **unbound**. If `D.Required`, the skill is unavailable (single-operator)
   or the agent surfaces "add secret N" (team).

The resolved set for `S` is `{ E → value }`, containing only declared names.

### 3.3 Injection by skill type

| Skill type | Delivery | Notes |
|------------|----------|-------|
| **MCP** (`skills/remote/mcp`) | populate `Config.Env` at spawn | Global secrets only at v1 (subprocess env is fixed at spawn). Per-user MCP → §6. |
| **Compiled** (`compiled.Skill`) | new optional `SecretsAware{ SetSecrets(map[string]string) }`, injected in `RegisterCompiledSkill` beside `StorageAware`/`AgentAware`; per-user via context (§3.4) | github, web, etc. |
| **OpenAPI** | resolve into `Auth.APIKey`/`Token`/`Password` | |
| **Markdown/guidance** | no tools → no injection; `Requires.Secrets` upgrades the availability check from "env set?" to "resolvable?" | |

### 3.4 Per-user injection (team mode)

Skills are constructed once per process, but per-user secrets must follow the
**acting user**. Resolution therefore happens at **tool-execution time**, not
construction:

- The agent turn carries the acting user (`chat` → member). A
  `secrets.FromContext(ctx)` accessor exposes the resolved, per-user secret set
  for the current turn.
- **Compiled** and **OpenAPI** skills read secrets from context, so one shared
  skill instance serves all members with per-user values.
- **MCP** subprocesses cannot re-env per turn; per-user MCP secrets are deferred
  (§6) — MCP uses global bindings at v1, documented as a limitation.

## 4. Per-User Secret Store (team mode)

### 4.1 Schema (Ent on the team PostgreSQL DB)

```
user_secrets  id (uuid) · user_id fk · skill (citext, "*" = any skill)
              name (citext) · env (text) · nonce (bytea) · ciphertext (bytea)
              created_at · updated_at
              UNIQUE (user_id, skill, name)
```

- **RLS**: `SELECT/INSERT/UPDATE/DELETE` only where `user_id =
  team_current_user_id()` (or system context for injection). The superadmin is
  **not** content-privileged — secrets are owner-only, like private chats.
- The `skill` column scopes a secret to one skill or `*` (all skills that
  declare that name), mirroring GitHub environment vs repo secrets.

### 4.2 Encryption at rest

Envelope encryption so a DB dump never yields plaintext:

- A **key-encryption key (KEK)** is resolved at startup from omnivault via
  config `team.secrets.encryption_key` (`op://…`, `file://…`, `env://…`). It is
  **never** stored in the DB. If lost, per-user secrets are unrecoverable and
  members re-enter them (documented).
- Each value is sealed with **AES-256-GCM**: random 96-bit nonce per write,
  ciphertext + nonce stored, secret name/skill as additional authenticated data.
- The service decrypts only in memory, only at injection time. Values are
  **write-only** across the API: create/update/delete and list-names, never
  read-back.

### 4.3 Service (team/secrets)

```go
SetUserSecret(ctx, actor, skill, name, env, plaintext) error   // upsert
DeleteUserSecret(ctx, actor, skill, name) error
ListUserSecretNames(ctx, actor) ([]SecretMeta, error)          // names/skill/env/set-at, NO values
resolveForUser(ctx, userID, skill, decls) (map[string]string, error) // system ctx, decrypt, inject
```

## 5. HTTP & UI (team mode)

- `GET  /api/secrets` — the member's secret names + which declared secrets are
  unset (values never returned).
- `PUT  /api/secrets` `{skill,name,value}` — upsert (CSRF).
- `DELETE /api/secrets?skill=&name=` — remove (CSRF).
- `GET  /api/skills/secrets` — declared secrets across loaded skills (name,
  description, env, required, whether the caller has set it) — drives the UI.
- UI (Phase 4): a Settings › Secrets panel listing each skill's declared secrets
  GitHub-style ("`github` needs `GITHUB_TOKEN`"), with paste-to-set, a "set ●"
  indicator, and delete. Values are never rendered after save.

## 6. Deferred: per-user MCP secrets

MCP subprocess env is fixed at spawn and the process is shared across members, so
per-user MCP injection needs one of: (a) lazy **per-user MCP instances** keyed by
user (memory/proc cost), (b) a per-request secret-passing MCP extension, or (c)
keeping MCP on global bindings only. v1 chooses (c) and documents it; (a) is a
follow-on if demand appears.

## 7. Security Considerations

- **Declaration allowlist**: no skill receives an undeclared secret.
- **Least exposure**: values reach only the target skill (subprocess env or
  in-process `SetSecrets`/context), never the model prompt or transcript.
- **Redaction**: resolved values are registered with a log redactor; the agent's
  output path scrubs them defensively.
- **At-rest encryption** with an externally-held KEK; DB compromise ≠ secret
  compromise.
- **RLS owner-only** per-user secrets; superadmin is administrative, not a
  content reader.
- **Write-only API**: the UI can set/replace/delete but never read a value back.

## 8. Open Questions

1. KEK rotation: v1 is manual re-encrypt (documented). A `rotate` command that
   re-seals all `user_secrets` under a new KEK is a candidate follow-on.
2. Should global bindings also support the team UI (admin-set, team-wide) or stay
   config-only? v1: config-only; UI is per-user.
3. `keeper://` is in `config.vaultSchemes` but unregistered — wire `omni-keeper`
   or drop it as part of Phase 1 cleanup.
