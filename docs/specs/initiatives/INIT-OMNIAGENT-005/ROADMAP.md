# OmniAgent Virtual Agents, Roles & Registry — Roadmap

**Initiative:** `INIT-OMNIAGENT-005`
**Repository:** `github.com/plexusone/omniagent`
**Status:** In progress — 11 of 14 items completed (Phases 1–4 complete; Phase 5 web UI remains)

> RMI IDs are stable and permanent. Commits carry `Refs: RMI-OMNIAGENT-<NNN>`.
> Phase status derives from member RMIs. Cross-initiative dependencies on
> `INIT-OMNIAGENT-003` (chats, UI) and `INIT-OMNIAGENT-004` (agent secrets) are
> noted per RMI. This initiative is the foundation both of those rescope onto.

## Phase 1 — Agent Entity & Configuration

**Theme:** An agent = persona + an enabled subset of skills; persisted, RLS-scoped.
**Status:** Completed — 3 of 3 items completed

- [x] `RMI-OMNIAGENT-300` `agents` + `agent_skills` schema, migration, RLS
  - Depends on: `RMI-OMNIAGENT-101` (RLS store)
  - Acceptance: private agents invisible to non-members by RLS; `team_is_agent_editor` helper; migration idempotent. Ent schemas `agent`/`agent_skill` (citext slug unique, `visibility` private|listed, superadmin-only `featured`, cascade-delete of skills/roles) generate the tables; `team/store/migrations/0001_functions.sql` adds the SECURITY DEFINER helpers `team_is_agent_editor`/`team_is_agent_owner`/`team_is_agent_creator`; `0002_policies.sql` binds `agents`/`agent_skills` RLS (SELECT = editor OR `visibility='listed'` OR superadmin OR system; writes = editor/superadmin/system). All migrations are idempotent (`CREATE OR REPLACE`, `DROP POLICY IF EXISTS`). Proven by `team/store/agents_rls_test.go` (private-agent invisibility to non-editors, superadmin administrative visibility, listed-agent visibility, editor-only writes).
- [x] `RMI-OMNIAGENT-301` Agent service: permissive create + CRUD
  - Depends on: `RMI-OMNIAGENT-300`
  - Acceptance: any allowlisted user creates an agent and is its owner in one tx; unique slug. `team/agents.Service`: `CreateAgent` inserts the agent + the creator's owner `AgentRole` in one transaction (the bootstrap authorized by the `team_is_agent_creator` insert policy), slug validated (3-32 chars, citext-unique) with a duplicate mapped to `ErrSlugTaken`. Adds `GetAgent`, `ListMyAgents`, `UpdateAgent` (editor-gated), `DeleteAgent` (owner-gated, cascades). Authorization is a system-context `lookup` of role + existence, so the service is the primary gate and RLS the backstop. Covered by sqlite-backed tests.
- [x] `RMI-OMNIAGENT-302` Enabled-skills subset + available-skills catalog
  - Depends on: `RMI-OMNIAGENT-301`
  - Acceptance: enabled skills must be a subset of the deployment catalog minus operator deny-list; unknown/blocked skills rejected. `NewService` resolves the catalog once (config `AvailableSkills` minus `BlockedSkills`, case-insensitive). `SetAgentSkills` validates the whole set before persisting — unknown → `ErrUnknownSkill`, deny-listed → `ErrBlockedSkill` — so a rejected set leaves the enabled skills intact (full replace, editor-gated). `AgentSkills` lists them; `AvailableSkills` exposes the catalog.

## Phase 2 — Per-Agent Roles & Authorization

**Theme:** Owner/maintainer per agent; conversing never grants configuration.
**Status:** Completed — 3 of 3 items completed

- [x] `RMI-OMNIAGENT-303` `agent_roles` schema + RLS
  - Depends on: `RMI-OMNIAGENT-300`
  - Acceptance: owner manages maintainers; self-leave; superadmin visible; RLS owner-only writes. Ent schema `agent_role` (`role` owner|maintainer, unique `(agent_id, user_id)`, cascade with the agent) generates the table; `0002_policies.sql` binds `agent_roles` RLS: SELECT = editor OR superadmin OR system; INSERT = owner OR superadmin OR system OR the creator's bootstrap self-insert (`team_is_agent_creator` before any role row exists); DELETE = self-leave OR owner OR superadmin OR system. Proven by `team/store/agents_rls_test.go` (creator bootstraps as owner, owner adds maintainer, maintainer cannot add another maintainer, self-leave, owner-only removal, owner-only delete).
- [x] `RMI-OMNIAGENT-304` Role management API + superadmin admin override
  - Depends on: `RMI-OMNIAGENT-303`
  - Acceptance: owner adds/removes maintainers; maintainers cannot; superadmin can administer any agent's roles. `AddMaintainer` grants the maintainer role by username (resolved in system context since RLS hides other users), idempotent and never demoting an owner; `RemoveMaintainer` removes another holder (owners not removable this way, no self-remove); `LeaveAgent` is self-leave with the sole-owner orphan guard (`ErrLastOwner`; a lone owner may leave, orphaning the agent for superadmin reassignment per TRD §9); `Roles` lists holders with usernames (editor-gated). Owner/superadmin only for add/remove. Covered by sqlite-backed tests.
- [x] `RMI-OMNIAGENT-305` `Can(actor, agent, capability)` authorization matrix
  - Depends on: `RMI-OMNIAGENT-304`
  - Acceptance: conversant→converse only; maintainer→config/secrets/visibility not maintainers; owner→all; superadmin→administer but not auto-granted secrets. `Capability` = {Chat, CreateChat, Configure, ManageMaintainers, ManageRegistry, Administer}; `Can` resolves the matrix — owner → all bar Administer; maintainer → all bar ManageMaintainers/Administer; superadmin → Administer + everything except secret values; any user → CreateChat/Chat iff the agent is listed. Featured stays superadmin-only (`SetFeatured`), not `CapManageRegistry`. Facts read via the shared system-context `lookup`; a nonexistent/invisible agent → `(false, nil)`. Covered by a full owner/maintainer/stranger/superadmin × private/listed table.

## Phase 3 — Registry & Discovery

**Theme:** Per-agent visibility + superadmin curation; discovery + start-chat authz.
**Status:** Completed — 3 of 3 items completed

- [x] `RMI-OMNIAGENT-306` Visibility (private/listed) + featured (superadmin)
  - Depends on: `RMI-OMNIAGENT-305`
  - Acceptance: owner/maintainer set visibility; only superadmin sets featured; enforced by authz + RLS. `SetVisibility` (CapManageRegistry = editor/superadmin; invalid values → `ErrInvalidVisibility`) and `SetFeatured` (superadmin-only curation per TRD §9 Q1 — a non-superadmin gets `ErrForbidden` even for their own agent). Both backstopped by the `agents_update` RLS policy. Covered by sqlite-backed tests.
- [x] `RMI-OMNIAGENT-307` Catalog API + start-chat authorization
  - Depends on: `RMI-OMNIAGENT-306`
  - Acceptance: catalog returns featured + listed for the caller; listed→any user may start, private→owner/maintainer/invitee. `Catalog` returns Featured (superadmin curation) + Listed (owner-opted, excluding featured), computed in the actor's scope so RLS filters what the caller sees — a featured-but-private agent surfaces only to its editors/superadmin, never leaking to the deployment (v1 interpretation: featuring promotes a listed agent; a private-featured agent is not surfaced deployment-wide). Each entry carries `CanStart`; `CanStartChat`/`AuthorizeStartChat` are the start-chat gate (`Can(CapCreateChat)`). The invitee case is enforced by chat membership in the chats service, not here.
- [x] `RMI-OMNIAGENT-308` Chat↔agent integration (`chats.agent_id`)
  - Depends on: `RMI-OMNIAGENT-307`, `RMI-OMNIAGENT-110` (chats)
  - Acceptance: chats reference an agent; chat creation consults `Can(CapCreateChat)`. Added `chats.agent_id` (optional FK to `agents`) + a `(created_by, agent_id)` partial unique index (one DM per user per agent; agent-less personal DMs deduped at the service layer). The chats service gains an `AgentGate` (primitive-signature interface so chats stays decoupled from `agents`; `*agents.Service` satisfies it via `AuthorizeStartChat`); `StartAgentDM` and `CreateGroupWithAgent` gate on `Can(CapCreateChat)` and return `ErrNoAgentRegistry` when unwired (personal mode). This is the integration point **RMI-113** builds on. Covered by sqlite-backed tests with a stub gate.

## Phase 4 — Runtime: Per-Agent Skill & Secret Binding

**Theme:** A chat runs with its agent's skills + agent-scoped secrets, isolated.
**Status:** Completed — 2 of 2 items completed

- [x] `RMI-OMNIAGENT-309` Per-agent runtime instances (lazy, bounded cache)
  - Depends on: `RMI-OMNIAGENT-302`, `RMI-OMNIAGENT-308`
  - Acceptance: gateway routes a chat's turns to its agent instance built from persona + enabled skills. New `team/agentruntime.Cache` is a lazy, bounded-LRU cache of per-agent instances satisfying the `chats.AgentRuntime` seam (`Slug` = cheap @-mention read; `Processor` = build-on-first-use then cache): an instance is built the first time its agent takes a turn and evicted (LRU, `DefaultMaxInstances`=64, closing the instance) when the deployment exceeds capacity, so only hot agents stay resident. It depends only on two seams — a `ConfigLoader` (reads an agent's persona/model/provider/enabled-skills by ID in **system context**, since the runtime is a system principal not a user) and a `Builder` — keeping it independent of the LLM/skill stack and unit-tested with fakes (lazy single-build under concurrency via per-entry `sync.Once`, LRU eviction+close, build/load errors not cached, `Invalidate`/`Close`). `agentruntime.AgentBuilder` is the production `Builder`: it constructs a real `*agent.Agent` from the agent's persona (→ system prompt) + enabled skills (→ `WithSkillIncludes` over the deployment's shared skill source), model/provider falling back to deployment defaults. `agents.Service` gains the system-context `LoadRuntimeConfig`/`AgentSlugByID` loaders. The team composition root (`cmd/omniagent/commands/team.go`) now constructs the agents service (also wired as the chats `AgentGate`, so agent-bound chats can be created — RMI-308 goes live) and the runtime cache, passing it as `chats.Config.Runtime`; wired only when an LLM API key is configured (otherwise agent-bound chats stay silent, unchanged). Combined with the RMI-113 mention policy and RMI-114 per-chat memory scope, an @-mentioned agent now runs on its own persona+skills instance. Agent-scoped **secret** injection (per-agent MCP env, disjoint-secret isolation) is the remaining RMI-310.
- [x] `RMI-OMNIAGENT-310` Agent-scoped secret binding + isolation
  - Depends on: `RMI-OMNIAGENT-309`, ~~`RMI-OMNIAGENT-207`~~ (superseded — see below)
  - Acceptance: agent secrets injected (incl. per-agent MCP env); two agents load disjoint skills/secrets with no cross-leak. Direction (2026-08-09): use **OmniVault** (`github.com/plexusone/omnivault`) as the secret abstraction and add the missing multi-tenancy. OmniVault is a flat, path-keyed store with no native tenancy, so `team/secrets.ScopedVault` adds it as a namespace-prefixing decorator (a caller handed `Scoped(v, "agents/<id>")` cannot address another agent's keys — isolation is structural, not policy). `team/secrets.Service` turns an agent's `agents/<id>` namespace into the env map its instance injects (`ResolveAgentSecrets`), plus `SetAgentSecret`/`DeleteAgentSecret` write paths. Injection uses a new `compiled.SecretsAware` interface (mirroring `StorageAware`/`AgentAware`): the MCP skill's `SetSecrets` merges secrets into its subprocess env without mutating shared config, and `agent.WithSecretEnv`/`Agent.SetSecretEnv` push the resolved env into every secrets-aware compiled skill (order-independent). `agentruntime.AgentBuilder` grows a `SecretSource` seam and resolves each agent's secrets in `Build`, appending `WithSecretEnv` — so each per-agent instance (RMI-309) is built with only its own secrets; two agents' MCP subprocesses run with disjoint environments. The team composition root builds the vault (`memory`/`file` provider, config `team.secrets`) and wires the source, gated like the RMI-309 runtime; unset config injects nothing (unchanged). Disjoint-isolation is proven end-to-end (`team/secrets` service, `agentruntime` builder, `agent` option tests). **Deferred (per 2026-08-09 direction):** encryption-at-rest for the team store (`memory`/`file` are unencrypted; a Postgres/ent + envelope-crypto or promoted OmniVault AES provider is follow-on INIT-004 work), `requires.secrets` SKILL.md declaration, and upstreaming `ScopedVault` into omnivault. Design: `DESIGN-310-agent-secrets.md`.

## Phase 5 — Web UI

**Theme:** Owners configure agents; users browse a catalog and start chats.
**Status:** Planned — 0 of 3 items completed

- [ ] `RMI-OMNIAGENT-311` Agents area (owner/maintainer: config, skills, maintainers, visibility)
  - Depends on: `RMI-OMNIAGENT-305`, `RMI-OMNIAGENT-115` (web UI)
- [ ] `RMI-OMNIAGENT-312` Catalog UI (browse featured/listed; start DM/group; conversant has no config surface)
  - Depends on: `RMI-OMNIAGENT-307`, `RMI-OMNIAGENT-311`
- [ ] `RMI-OMNIAGENT-313` Superadmin curation UI (featured) + docs
  - Depends on: `RMI-OMNIAGENT-312`, `RMI-OMNIAGENT-119` (admin UI)
