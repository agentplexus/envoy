# OmniAgent Virtual Agents, Roles & Registry — TRD

> **Initiative:** `INIT-OMNIAGENT-005`
> **Status:** Draft
> **Companion:** [PRD.md](PRD.md) · [PLAN.md](PLAN.md) · [ROADMAP.md](ROADMAP.md)

## 1. Architecture Overview

An **Agent** is a persisted configuration bound to runtime capabilities at chat
time. It sits between users and the OmniAgent runtime, and is the anchor that
INIT-003 chats and INIT-004 secrets attach to.

```
  users ──▶ agent_roles(owner/maintainer) ──▶ agents ──▶ agent_skills (subset of available)
                                               │  │  │
   catalog/registry ◀── visibility+featured ──┘  │  └──▶ agent secrets  (INIT-004, agent-scoped)
   chats(agent_id) ◀── start-chat authz ─────────┘
                                                  ▼
                              runtime: build an agent instance = persona
                              + enabled skills + resolved agent secrets
```

## 2. Data Model (Ent on the team PostgreSQL DB)

```
agents          id (uuid) · slug (citext unique) · name · description
                persona (text) · model (text) · provider (text)
                visibility (private|listed) · featured (bool)   -- featured = superadmin curation
                created_by fk users · created_at · updated_at

agent_skills    agent_id fk · skill (citext)                    -- one enabled skill
                PRIMARY KEY (agent_id, skill)

agent_roles     agent_id fk · user_id fk · role (owner|maintainer)
                created_at
                PRIMARY KEY (agent_id, user_id)
```

INIT-003's `chats` gains `agent_id fk agents`; INIT-004's agent secrets key on
`agent_id` (replacing the per-user model). `users.role` (superadmin|member) from
INIT-003 stays as the **deployment** tier; "member" simply means an allowlisted
user with no deployment-admin rights.

## 3. Authorization

The capability matrix (PRD) is enforced by a single service function plus RLS
backstop:

```go
// team/agents
type Capability int
const (
    CapChat Capability = iota   // DM / converse
    CapCreateChat               // start a chat with the agent
    CapConfigure                // skills, persona, model, secrets
    CapManageMaintainers        // add/remove maintainers (owner only)
    CapManageRegistry           // visibility / featured(owner+super) 
    CapAdminister               // superadmin deployment admin
)
func (s *Service) Can(ctx, actor, agentID, cap) (bool, error)
```

Resolution:

- **owner** → all of Configure, ManageMaintainers, ManageRegistry(visibility),
  CreateChat, Chat.
- **maintainer** → Configure, ManageRegistry(visibility), CreateChat, Chat — not
  ManageMaintainers.
- **superadmin** → Administer + everything **except** it is not auto-granted an
  agent's secret *values* (it can rotate roles/registry, not read secrets).
- **featured** flag is superadmin-only (curation), distinct from an owner's
  `visibility`.
- **any allowlisted user** → CreateChat/Chat iff `visibility = listed` OR they
  are an invited chat member; Chat only within chats they belong to.

### RLS (defense-in-depth)

- `agents` SELECT: creator/owner/maintainer, OR `visibility = listed`, OR system.
  UPDATE/DELETE: owner/maintainer (via `team_is_agent_editor(agent_id)` SECURITY
  DEFINER helper, mirroring the chat-member helpers).
- `agent_roles` SELECT: members of that agent + superadmin + system. INSERT/
  DELETE of `maintainer`: owner or superadmin; a user may delete their own role
  (leave).
- `agent_skills`: SELECT for anyone who can see the agent; write for editors.
- App-layer `Can()` is the primary gate; RLS is the backstop, per the established
  two-role model.

## 4. Creation (permissive) & Curation (curated)

- **Permissive:** `CreateAgent(ctx, actor, spec)` — any allowlisted user; the
  creator is inserted as `owner` in the same transaction. Slug uniqueness like
  usernames.
- **Curated:** `featured` is toggled only by the superadmin. The catalog API
  returns two sections: *featured* (superadmin-curated, any visibility the
  superadmin chooses to surface) and *listed* (owner-opted, `visibility=listed`).
  Both coexist per the product decision ("permissive creation + curated
  presentation").

## 5. Enabled-Skills Subset

Not every deployment skill may be loaded into an agent. The deployment exposes an
**available-skills catalog** (the registered compiled/MCP/markdown skills, minus
an operator deny-list in config: `agents.available_skills` / `agents.blocked_skills`).
`agent_skills` rows must be a subset; `SetAgentSkills` validates against the
catalog and rejects unknown/blocked skills. This bounds blast radius — e.g. a
shell/exec skill can be withheld from user-created agents.

## 6. Runtime Binding

A chat names an `agent_id`. When a turn runs:

1. Load the agent (persona, model/provider, enabled skills).
2. Resolve the agent's secrets (INIT-004 agent-scoped `resolveForAgent`).
3. Construct/obtain an **agent runtime instance** configured with exactly that
   persona + skill set + injected secrets.

Instance strategy: a per-`agent_id` runtime, built lazily and cached
(bounded LRU), rather than one global agent. The gateway routes a chat's turns to
its agent's instance. MCP subprocesses are per-agent (their env carries that
agent's secrets) — this also resolves INIT-004's per-user-MCP problem, since
secrets are now agent-scoped, not user-scoped.

## 7. HTTP & UI (team mode)

- `POST /api/agents` create · `GET /api/agents` my agents · `GET /api/agents/{id}`
  detail · `PATCH /api/agents/{id}` configure · `DELETE`.
- `PUT /api/agents/{id}/skills` set enabled skills · `GET /api/skills/available`.
- `GET/POST/DELETE /api/agents/{id}/maintainers`.
- `PUT /api/agents/{id}/visibility` (owner/maintainer) ·
  `PUT /api/agents/{id}/featured` (superadmin).
- `GET /api/catalog` — featured + listed sections for the current user.
- Chat creation (INIT-003) checks `Can(actor, agent, CapCreateChat)`.
- UI: an **Agents** area (create/configure/skills/secrets/maintainers/registry
  for owners) and a **Catalog** (browse + start chat for users).

## 8. Security Considerations

- **Config ≠ conversation**: conversing never grants Configure; enforced by
  `Can()` + RLS, independent of `chat_members`.
- **Skill blast-radius**: the available-skills catalog + operator deny-list bound
  what user-created agents may load.
- **Secret isolation**: agent secrets are readable only through the agent's
  editors and the runtime system context; the superadmin administers roles but is
  not handed secret values.
- **Slug/enumeration**: private agents are invisible in the catalog and by direct
  ID to non-members (RLS SELECT).

## 9. Open Questions

1. Should maintainers toggle `featured`? v1: **no** — `featured` is superadmin
   curation; owners/maintainers control `visibility` only.
2. Orphaned agents (owner disabled/removed): v1 superadmin reassigns; auto-
   transfer is a follow-on.
3. Runtime instance eviction policy (idle TTL vs LRU size) — tune during Phase 4.
