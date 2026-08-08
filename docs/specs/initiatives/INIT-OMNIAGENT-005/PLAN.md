# OmniAgent Virtual Agents, Roles & Registry — Implementation Plan

> **Initiative:** `INIT-OMNIAGENT-005`
> Phases and RMI IDs match [ROADMAP.md](ROADMAP.md). Reviewed by phase; status
> derives from RMIs. Commits carry `Refs: RMI-OMNIAGENT-<NNN>`.

## Package Layout

```
team/
  ├── ent/schema/{agent,agentskill,agentrole}.go
  └── agents/                 # service: CRUD, roles, registry, Can(), skill subset
gateway/team_agents_http.go   # /api/agents, /api/catalog, maintainers, visibility
agent/registry (existing)     # extended: per-agent runtime instances (skills+secrets)
web/                          # Agents area + Catalog (INIT-003 UI)
```

Builds on the INIT-003 store/auth and the two-role RLS model; agent secrets come
from the rescoped INIT-004.

## Phase 1 — Agent Entity & Configuration

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-300 | `agents` + `agent_skills` ent schemas + migration + RLS (see TRD §3); `team_is_agent_editor` SECURITY DEFINER helper |
| RMI-OMNIAGENT-301 | `team/agents` service: CreateAgent (permissive; creator→owner in one tx), Get/Update/Delete, slug uniqueness |
| RMI-OMNIAGENT-302 | Enabled-skills subset: available-skills catalog + operator deny-list config; `SetAgentSkills` validates against it |

**Depends:** `INIT-OMNIAGENT-101` (RLS store). **Gate:** a user creates an agent,
sets a valid skill subset; RLS proves non-members can't see a private agent.

## Phase 2 — Per-Agent Roles & Authorization

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-303 | `agent_roles` schema + RLS (owner manages maintainers; self-leave) |
| RMI-OMNIAGENT-304 | Owner/maintainer management API + service; superadmin admin override |
| RMI-OMNIAGENT-305 | `Can(actor, agent, capability)` authorization matrix (TRD §3) + tests: conversant/maintainer/owner/superadmin boundaries |

**Depends:** RMI-300. **Gate:** the capability matrix holds in tests — a
conversant gets no config rights; a maintainer can't add maintainers; superadmin
administers but isn't handed secrets.

## Phase 3 — Registry & Discovery

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-306 | Visibility (`private`/`listed`, owner/maintainer) + `featured` (superadmin) with RLS/authz |
| RMI-OMNIAGENT-307 | Catalog API: featured + listed sections scoped to the caller; start-chat authorization (`CapCreateChat`) |
| RMI-OMNIAGENT-308 | Chat↔agent integration: `chats.agent_id`; INIT-003 chat creation consults `Can()` |

**Depends:** RMI-305; **cross-init** `INIT-OMNIAGENT-110` (chats). **Gate:** a
listed agent is startable by any user; a private one only by owner/maintainer/
invitee; featured surfaces in every user's catalog.

## Phase 4 — Runtime: Per-Agent Skill & Secret Binding

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-309 | Per-`agent_id` runtime instances (lazy, bounded cache) built from persona + enabled skills; gateway routes a chat's turns to its agent instance |
| RMI-OMNIAGENT-310 | Bind agent-scoped secrets into the instance (INIT-004 `resolveForAgent`); per-agent MCP subprocess env; disjoint-skill/secret isolation between agents |

**Depends:** RMI-302, RMI-308; **cross-init** `INIT-OMNIAGENT-207` (agent
secrets service). **Gate:** two agents load disjoint skills/secrets; neither
leaks into the other's chats.

## Phase 5 — Web UI

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-311 | Agents area: create/configure/persona/skills/maintainers/visibility (owner/maintainer) |
| RMI-OMNIAGENT-312 | Catalog UI: featured + listed browse; start DM/group; conversant view has no config surface |
| RMI-OMNIAGENT-313 | Superadmin curation UI (`featured`) + docs/testing guide |

**Depends:** Phase 4; **cross-init** `INIT-OMNIAGENT-115`/`119` (web + admin UI).

## Milestones

- **Agents core** = Phases 1–2 (entity + roles/authz).
- **Discovery** = Phase 3 (registry, chat integration).
- **Live** = Phase 4 (runtime binding) + Phase 5 (UI).

## Definition of Done (per plexusone org)

Every RMI: repo patterns; unit tests (RLS + authz matrix where applicable);
`golangci-lint` clean; docs on user-visible change; `Refs: RMI-OMNIAGENT-<NNN>`.
