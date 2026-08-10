# OmniAgent Virtual Agents, Roles & Registry — Roadmap

**Initiative:** `INIT-OMNIAGENT-005`
**Repository:** `github.com/plexusone/omniagent`
**Status:** In progress — 2 of 14 items completed

> RMI IDs are stable and permanent. Commits carry `Refs: RMI-OMNIAGENT-<NNN>`.
> Phase status derives from member RMIs. Cross-initiative dependencies on
> `INIT-OMNIAGENT-003` (chats, UI) and `INIT-OMNIAGENT-004` (agent secrets) are
> noted per RMI. This initiative is the foundation both of those rescope onto.

## Phase 1 — Agent Entity & Configuration

**Theme:** An agent = persona + an enabled subset of skills; persisted, RLS-scoped.
**Status:** In progress — 1 of 3 items completed

- [x] `RMI-OMNIAGENT-300` `agents` + `agent_skills` schema, migration, RLS
  - Depends on: `RMI-OMNIAGENT-101` (RLS store)
  - Acceptance: private agents invisible to non-members by RLS; `team_is_agent_editor` helper; migration idempotent. Ent schemas `agent`/`agent_skill` (citext slug unique, `visibility` private|listed, superadmin-only `featured`, cascade-delete of skills/roles) generate the tables; `team/store/migrations/0001_functions.sql` adds the SECURITY DEFINER helpers `team_is_agent_editor`/`team_is_agent_owner`/`team_is_agent_creator`; `0002_policies.sql` binds `agents`/`agent_skills` RLS (SELECT = editor OR `visibility='listed'` OR superadmin OR system; writes = editor/superadmin/system). All migrations are idempotent (`CREATE OR REPLACE`, `DROP POLICY IF EXISTS`). Proven by `team/store/agents_rls_test.go` (private-agent invisibility to non-editors, superadmin administrative visibility, listed-agent visibility, editor-only writes).
- [ ] `RMI-OMNIAGENT-301` Agent service: permissive create + CRUD
  - Depends on: `RMI-OMNIAGENT-300`
  - Acceptance: any allowlisted user creates an agent and is its owner in one tx; unique slug
- [ ] `RMI-OMNIAGENT-302` Enabled-skills subset + available-skills catalog
  - Depends on: `RMI-OMNIAGENT-301`
  - Acceptance: enabled skills must be a subset of the deployment catalog minus operator deny-list; unknown/blocked skills rejected

## Phase 2 — Per-Agent Roles & Authorization

**Theme:** Owner/maintainer per agent; conversing never grants configuration.
**Status:** In progress — 1 of 3 items completed

- [x] `RMI-OMNIAGENT-303` `agent_roles` schema + RLS
  - Depends on: `RMI-OMNIAGENT-300`
  - Acceptance: owner manages maintainers; self-leave; superadmin visible; RLS owner-only writes. Ent schema `agent_role` (`role` owner|maintainer, unique `(agent_id, user_id)`, cascade with the agent) generates the table; `0002_policies.sql` binds `agent_roles` RLS: SELECT = editor OR superadmin OR system; INSERT = owner OR superadmin OR system OR the creator's bootstrap self-insert (`team_is_agent_creator` before any role row exists); DELETE = self-leave OR owner OR superadmin OR system. Proven by `team/store/agents_rls_test.go` (creator bootstraps as owner, owner adds maintainer, maintainer cannot add another maintainer, self-leave, owner-only removal, owner-only delete).
- [ ] `RMI-OMNIAGENT-304` Role management API + superadmin admin override
  - Depends on: `RMI-OMNIAGENT-303`
  - Acceptance: owner adds/removes maintainers; maintainers cannot; superadmin can administer any agent's roles
- [ ] `RMI-OMNIAGENT-305` `Can(actor, agent, capability)` authorization matrix
  - Depends on: `RMI-OMNIAGENT-304`
  - Acceptance: conversant→converse only; maintainer→config/secrets/visibility not maintainers; owner→all; superadmin→administer but not auto-granted secrets

## Phase 3 — Registry & Discovery

**Theme:** Per-agent visibility + superadmin curation; discovery + start-chat authz.
**Status:** Planned — 0 of 3 items completed

- [ ] `RMI-OMNIAGENT-306` Visibility (private/listed) + featured (superadmin)
  - Depends on: `RMI-OMNIAGENT-305`
  - Acceptance: owner/maintainer set visibility; only superadmin sets featured; enforced by authz + RLS
- [ ] `RMI-OMNIAGENT-307` Catalog API + start-chat authorization
  - Depends on: `RMI-OMNIAGENT-306`
  - Acceptance: catalog returns featured + listed for the caller; listed→any user may start, private→owner/maintainer/invitee
- [ ] `RMI-OMNIAGENT-308` Chat↔agent integration (`chats.agent_id`)
  - Depends on: `RMI-OMNIAGENT-307`, `RMI-OMNIAGENT-110` (chats)
  - Acceptance: chats reference an agent; chat creation consults `Can(CapCreateChat)`

## Phase 4 — Runtime: Per-Agent Skill & Secret Binding

**Theme:** A chat runs with its agent's skills + agent-scoped secrets, isolated.
**Status:** Planned — 0 of 2 items completed

- [ ] `RMI-OMNIAGENT-309` Per-agent runtime instances (lazy, bounded cache)
  - Depends on: `RMI-OMNIAGENT-302`, `RMI-OMNIAGENT-308`
  - Acceptance: gateway routes a chat's turns to its agent instance built from persona + enabled skills
- [ ] `RMI-OMNIAGENT-310` Agent-scoped secret binding + isolation
  - Depends on: `RMI-OMNIAGENT-309`, `RMI-OMNIAGENT-207` (agent secrets)
  - Acceptance: agent secrets injected (incl. per-agent MCP env); two agents load disjoint skills/secrets with no cross-leak

## Phase 5 — Web UI

**Theme:** Owners configure agents; users browse a catalog and start chats.
**Status:** Planned — 0 of 3 items completed

- [ ] `RMI-OMNIAGENT-311` Agents area (owner/maintainer: config, skills, maintainers, visibility)
  - Depends on: `RMI-OMNIAGENT-305`, `RMI-OMNIAGENT-115` (web UI)
- [ ] `RMI-OMNIAGENT-312` Catalog UI (browse featured/listed; start DM/group; conversant has no config surface)
  - Depends on: `RMI-OMNIAGENT-307`, `RMI-OMNIAGENT-311`
- [ ] `RMI-OMNIAGENT-313` Superadmin curation UI (featured) + docs
  - Depends on: `RMI-OMNIAGENT-312`, `RMI-OMNIAGENT-119` (admin UI)
