# RMI-310 Design — Agent-Scoped Secret Binding via OmniVault Multi-Tenancy

**Initiative:** `INIT-OMNIAGENT-005` (Phase 4) · **RMI:** `RMI-OMNIAGENT-310`
**Depends on:** `RMI-OMNIAGENT-309` (per-agent runtime instances — done),
`RMI-OMNIAGENT-207` (secret service — *unbuilt*; this design supersedes its
storage approach with an OmniVault-backed one)
**Status:** Implemented (mechanism + isolation) — 2026-08-09. `team/secrets`
(ScopedVault + Service), `compiled.SecretsAware` + MCP `SetSecrets`,
`agent.WithSecretEnv`, `agentruntime.SecretSource` + builder wiring, and the
`team.secrets` composition-root wiring are landed and tested. Deferred per §5/§6:
encryption-at-rest, `requires.secrets`, and upstreaming ScopedVault to omnivault.

> Direction (2026-08-09): use `github.com/plexusone/omnivault` as the vault/secrets
> abstraction; it has no multi-tenant/multi-user support today, so this note
> investigates how to add it and how RMI-310 consumes it.

## 1. Goal

`RMI-OMNIAGENT-310` acceptance: **agent secrets are injected into the agent's
runtime instance (including per-agent MCP subprocess env), and two agents load
disjoint skills/secrets with no cross-leak.**

The INIT-005 TRD (§ runtime, "Resolve the agent's secrets") already fixes the
shape: each per-agent instance built by `agentruntime.AgentBuilder` (RMI-309)
resolves *that agent's* secrets and threads them into its skills; MCP servers run
as per-agent subprocesses whose env carries only that agent's secrets. RMI-309
left this as the explicit follow-on. This design fills it.

## 2. Investigation — OmniVault multi-tenancy today

OmniVault (`v0.5.0`, the pinned version) is a **flat, path-keyed secret store**
behind one interface (`vault/interface.go`):

```go
type Vault interface {
    Get(ctx, path) (*Secret, error)
    Set(ctx, path, *Secret) error
    Delete(ctx, path) error
    Exists(ctx, path) (bool, error)
    List(ctx, prefix) ([]string, error)
    Name() string; Capabilities() Capabilities; Close() error
}
```

Plus a `Resolver` that routes `scheme://path#field` URIs to a registered `Vault`
per scheme (`op://`, `env://`, `file://`, …). `config/credentials.go` already
uses this to resolve vault-URI config fields.

**Findings:**

- **No tenancy/namespace concept.** Every caller sees the same flat keyspace.
  There is no per-user or per-agent partition, and nothing prevents one caller's
  path from addressing another's.
- **No public encrypted-at-rest provider.** The public providers are
  `env`, `file` (plaintext files, `0600`), and `memory`. The AES-256-GCM +
  Argon2id encrypted store is in `internal/store` — reachable only through the
  CLI daemon, **not importable** as a `vault.Vault`.
- The `Vault` interface is minimal and dependency-free, which makes it easy to
  **decorate**.

**Conclusion:** multi-tenancy is best added as **path-namespace scoping** — a
thin decorator over the existing `Vault` interface — not a change to the
interface itself. Encryption at rest is a *separate* gap (§5).

## 3. Proposed OmniVault addition — `ScopedVault`

A `ScopedVault` wraps any `vault.Vault` and confines it to a namespace by
prefixing every path:

```go
// scopedvault (new, in omnivault): confines a Vault to a namespace.
type ScopedVault struct { inner vault.Vault; ns string } // ns e.g. "agents/<uuid>"

func Scoped(inner vault.Vault, ns string) *ScopedVault

func (s *ScopedVault) Get(ctx, path)   -> inner.Get(ctx, s.ns+"/"+path)
func (s *ScopedVault) Set(ctx, p, sec) -> inner.Set(ctx, s.ns+"/"+p, sec)
func (s *ScopedVault) List(ctx, pfx)   -> inner.List(ctx, s.ns+"/"+pfx),
                                          then strip s.ns+"/" from each result
// Delete/Exists analogous. Close() is a no-op (the inner vault is shared).
```

**Isolation is structural.** An agent handed `Scoped(v, "agents/<A>")`
*cannot express* a path outside `agents/<A>/…` — the prefix is applied by the
wrapper, not chosen by the caller. Two agents get two `ScopedVault`s over the
same backing store with disjoint prefixes; neither can read the other's keys.
This directly satisfies the RMI-310 "no cross-leak" gate at the storage layer.

**Namespace convention** (keyspace layout on the backing vault):

| Scope        | Namespace          | Read by                                   |
|--------------|--------------------|-------------------------------------------|
| Deployment   | `global/`          | config + any skill (fallback)             |
| Per-user     | `users/<userID>/`  | that user's turns (INIT-004 per-user)     |
| Per-agent    | `agents/<agentID>/`| that agent's runtime instance (RMI-310)   |

**Precedence** (RMI-310 uses agent scope; general order for INIT-004):
per-agent ▸ per-user ▸ per-skill global ▸ global.

**Where it lives.** `ScopedVault` is generic and useful to every OmniVault
consumer, so the natural home is **omnivault** (a `scopedvault` subpackage or a
`vault.Scoped` constructor) — this *is* the "add multi-tenancy to omnivault"
work. It is a pure decorator: no new dependencies, no interface change, additive
and release-safe. See §6 for the cross-repo sequencing question.

## 4. RMI-310 consumption in omniagent

```
agents.Service.ResolveAgentSecrets(ctx, agentID, declared []string)
     │  (system context; opens Scoped(teamVault, "agents/"+agentID))
     ▼
map[envVar]value ──▶ agentruntime.SecretSource seam  (new, mirrors ConfigLoader)
     ▼
AgentBuilder.Build: thread resolved secrets into the agent's own skills
     ▼
per-agent MCP subprocess env  +  compiled SecretsAware  (per-agent instance)
```

New pieces:

1. **`team/secrets` (or `agents`) `ResolveAgentSecrets`** — reads the agent's
   namespace from the team vault in **system context** (the runtime is a system
   principal, consistent with RMI-309's `LoadRuntimeConfig`). Returns only the
   *declared* secret env vars (a skill receives only what it declares — needs
   RMI-200 `requires.secrets`; until then, resolve by the agent's enabled-skill
   env-var set / all keys in the agent namespace).
2. **`agentruntime.SecretSource` seam** — `ResolveSecrets(ctx, agentID) ->
   map[string]string`, injected into `AgentBuilder` like `ConfigLoader` is into
   `Cache`. Keeps the builder testable with a fake; nil source = no secrets
   (current behavior).
3. **Per-agent injection into skills** — `AgentBuilder` already builds each
   instance its *own* `skillManager` (`agent.New` → fresh manager unless one is
   passed), so per-agent MCP env is mechanically reachable. It needs a new
   `agent`-level option (e.g. `agent.WithSecretEnv(map[string]string)`) that the
   skill layer applies to MCP `Config.Env` (`skills/remote/mcp/config.go`) and to
   `SecretsAware` compiled skills. This is the one piece of net-new plumbing in
   the `agent`/`skills` packages.

**Disjoint-isolation test** (the RMI-310 gate): back a `memory` vault with
`agents/A/TOKEN=aaa` and `agents/B/TOKEN=bbb`; build both instances; assert A's
MCP env has `TOKEN=aaa` and *no* `bbb`, and vice-versa — proving the ScopedVault
partition holds end-to-end through the builder.

## 5. Encryption-at-rest gap (needs a decision)

Team secrets must be encrypted at rest. OmniVault's public providers are not,
and its AES store is `internal/`. Options:

- **(a) Promote OmniVault's encrypted store to a public provider** — best reuse;
  a small omnivault PR exposing the `internal/store` crypto as a `vault.Vault`.
- **(b) New Postgres-backed OmniVault provider** — secrets live in the team DB
  (RLS + envelope encryption per INIT-004 RMI-206), wrapped by `ScopedVault`.
  Keeps everything in one datastore; most aligned with the existing INIT-004
  ent+envelope plan, just re-homed behind the `Vault` interface.
- **(c) Back `ScopedVault` with the INIT-004 ent+envelope store as the `Vault`
  impl** — same as (b) without a formal omnivault provider.

Recommendation: **(b)/(c)** for the team datastore (one DB, RLS backstop,
envelope crypto), with `ScopedVault` providing tenancy on top. **(a)** is worth
doing in omnivault regardless for single-operator/local use.

## 6. Scope & sequencing for RMI-310

RMI-310 should land the **mechanism and isolation**, not the full encrypted
team-secret store (that stays INIT-004):

1. **omnivault:** add `ScopedVault` (namespace scoping) — the "multi-tenancy"
   addition. *(cross-repo — see decision below.)*
2. **omniagent:** `SecretSource` seam + `ResolveAgentSecrets` (system context)
   over a `Scoped` team vault.
3. **omniagent:** `agent.WithSecretEnv` (+ skills MCP/`SecretsAware` injection),
   wired through `AgentBuilder`.
4. **omniagent:** disjoint-isolation tests (two agents, memory-backed vault).
5. Wire the team composition root to construct the team vault + secret source,
   gated like the RMI-309 runtime.

Deferred to INIT-004: `requires.secrets` declaration parsing (RMI-200), the
encrypted team store / provider (RMI-205/206/207 re-homed per §5), the Secrets
management UI, and per-user resolution precedence.

### Open decision — where `ScopedVault` lands first

Adding to **omnivault** is the honest "add multi-tenancy to omnivault" outcome,
but it is a separate repo with its own CI + release cycle (org standard: push,
wait for CI, then tag). Two paths:

- **A — omnivault first:** implement `ScopedVault` in omnivault, release a new
  tag, bump the omniagent pin, then build RMI-310 on it. Cleanest; slower.
- **B — prototype local, upstream after:** implement the scoping wrapper in
  omniagent (`team/secrets`), ship RMI-310, and upstream the identical decorator
  to omnivault as a follow-up. Faster; one interim copy to reconcile.
