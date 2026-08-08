# OmniAgent Virtual Agents, Roles & Registry — PRD

> **Initiative:** `INIT-OMNIAGENT-005`
> **Status:** Draft
> **Date:** 2026-08-04

## Problem Statement

The team-agent model (INIT-OMNIAGENT-003) assumed a single implicit agent and a
flat superadmin/member split. Real use needs **many agents**, each a distinct
configuration (a chosen subset of skills + its own secrets + persona), with
**per-agent ownership**: whoever runs an agent configures it and manages its
secrets, while people who merely chat with it cannot. Users also need to
**discover** agents — some self-service, some curated by the deployment.

## Vision

Make **"agent" a first-class entity**: a named configuration =
*(subset of available skills) + (agent-scoped secrets) + persona/model*. Agents
have owners and maintainers who configure them, a registry that controls who can
start chats, and a catalog — part self-service, part curated — for discovery.
Conversing with an agent never confers the right to configure it.

## Roles (three tiers, orthogonal)

| Tier | Role | Scope |
|------|------|-------|
| Deployment | **Superadmin** | Administers users, the allowlist, and (as admin) any agent's roles/registry. Not auto-granted an agent's secrets. |
| Per-agent | **Owner** | Creator or assignee. Full control: config, skills, secrets, maintainers, registry visibility. |
| Per-agent | **Maintainer** | Added by an owner. Manages config, skills, secrets, registry visibility — but **not** maintainers. |
| Per-chat | **Conversant** | A participant in a chat. Converses only; no config/secret rights (unless they separately own/maintain the agent). |

Per-agent roles are **independent of chat membership**: being in a group chat
never grants configuration rights.

## Users & Stories

### Agent authors (owner / maintainer)

- **US-1**: As any allowlisted user, I can **create an agent** and become its
  owner — a permissive, self-service path.
- **US-2**: As an owner, I choose **which skills** load into my agent from those
  the deployment makes available (not every skill need be loaded), set its
  persona/model, and manage its **secrets** (per INIT-OMNIAGENT-004).
- **US-3**: As an owner, I add/remove **maintainers** who can also configure the
  agent and manage its secrets, but cannot add other maintainers.
- **US-4**: As an owner/maintainer, I **always** have a DM with my agent and can
  create group chats and invite others into them.
- **US-5**: As an owner/maintainer, I decide whether my agent is **listed** in
  the registry (any allowlisted user may then start chats) or **private** (only I
  and the people I invite).

### Deployment curation (superadmin)

- **US-6**: As the superadmin, I **feature/curate** which agents are presented
  prominently to users, independent of an owner's listed flag — so the deployment
  can showcase chosen agents.
- **US-7**: As the superadmin, I can administer any agent's roles and registry
  status (e.g. reassign an orphaned agent) without being handed its secrets.

### Users / conversants

- **US-8**: As an allowlisted user, I **browse** the catalog — featured agents and
  listed agents — and **start a DM or group chat** with any I'm permitted to,
  without the ability to configure it.
- **US-9**: As a conversant invited to a group chat, I can talk to the agent (on
  @-mention) but cannot see or change its skills or secrets.

## Requirements

### Must Have

1. **Agent entity** — a persisted agent = slug/name/description, persona/model,
   an **enabled-skills subset** validated against the deployment's available
   skills, and timestamps.
2. **Permissive creation** — any allowlisted user creates agents and becomes
   owner.
3. **Per-agent roles** — owner/maintainer, RLS-scoped; owner manages maintainers;
   maintainers manage config/skills/secrets/registry.
4. **Registry & visibility** — per-agent `private`/`listed`; a superadmin
   `featured` curation flag; a catalog API combining both.
5. **Start-chat authorization** — listed agents: any allowlisted user; private
   agents: owner/maintainer + invitees. Owners/maintainers always reachable.
6. **Runtime binding** — a chat with an agent runs with **that agent's** enabled
   skills and agent-scoped secrets; other agents' skills/secrets are not loaded.
7. **Superadmin administration** — reassign/administer any agent's roles and
   registry; not auto-granted secret access.

### Should Have

- Per-agent activity/audit of role and registry changes.
- Agent duplication ("clone this agent's config as a starting point").

### Non-Goals

- Cross-deployment agent sharing / a public marketplace.
- Per-conversant (end-user) secrets — secrets are **agent-scoped**, owner-managed
  (see INIT-OMNIAGENT-004, rescoped).
- Fine-grained per-skill permissions beyond the enabled-skills subset.
- Agent-to-agent orchestration (one agent invoking another).

## Success Criteria

- A user creates an agent, enables three skills, sets one secret, marks it
  listed; a second user finds it in the catalog and starts a DM that runs with
  exactly those three skills and that secret.
- A group-chat conversant can talk to the agent but gets 403 on any config,
  skill, secret, or maintainer operation.
- The superadmin features an agent; it appears in every user's curated catalog
  without the owner changing anything, and the superadmin never sees its secrets.
- Two agents in one deployment load disjoint skill/secret sets; neither leaks
  into the other's chats.

## Relationship to Other Initiatives

- **Foundation** for the rescoped INIT-OMNIAGENT-003 chats (a chat attaches to an
  `agent_id`; registry gates chat creation) and INIT-OMNIAGENT-004 secrets
  (secrets are agent-scoped, managed by owner/maintainers).
- **Builds on** INIT-OMNIAGENT-003 Phase 1 (identity, RLS store) and Phase 4
  (web UI).
