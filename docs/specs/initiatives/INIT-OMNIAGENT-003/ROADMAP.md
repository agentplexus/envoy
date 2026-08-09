# OmniAgent Team Agent — Roadmap

**Initiative:** `INIT-OMNIAGENT-003`
**Repository:** `github.com/plexusone/omniagent`
**Status:** Planned — 0 of 29 items completed

> RMI IDs are stable and permanent. Commits implementing an item carry the
> trailer `Refs: RMI-OMNIAGENT-<NNN>`. Phase status is derived from member
> RMIs — a phase is complete only when all its required RMIs are complete.
> **v1** = Phases 1–4 + 6 · **v2** = Phase 5.

## Phase 1 — Identity & Data Foundation

**Theme:** PostgreSQL + Ent + Row-Level Security as the multi-user system of record.
**Status:** Planned — 0 of 5 items completed

- [ ] `RMI-OMNIAGENT-100` Ent schemas + versioned migrations + RLS policies
  - Acceptance: all eight tables migrate from empty; RLS enabled with FORCE; policies match TRD §3; migrations embedded and idempotent
- [ ] `RMI-OMNIAGENT-101` RLS-scoped store (`AsUser` transaction helper)
  - Depends on: `RMI-OMNIAGENT-100`
  - Acceptance: every query path flows through per-request `SET LOCAL` GUCs; raw DB unexported; advisory-locked migrations-on-start
- [ ] `RMI-OMNIAGENT-102` Team service: users, roles, allowlist
  - Depends on: `RMI-OMNIAGENT-101`
  - Acceptance: rename/display-name, allowlist CRUD superadmin-only at app layer AND by policy
- [ ] `RMI-OMNIAGENT-103` Superadmin bootstrap + rename
  - Acceptance: configured email becomes superadmin on first login; later config change does not demote; username change persists and is unique
- [ ] `RMI-OMNIAGENT-104` Team config section + RLS isolation test suite
  - Acceptance: `team.*` config validated; SQL tests prove cross-user isolation per table

## Phase 2 — Magic Link Authentication

**Theme:** Passwordless, allowlist-closed login with server-side cookie sessions.
**Status:** Planned — 0 of 5 items completed

- [ ] `RMI-OMNIAGENT-105` SMTP delivery + magic-link template
  - Acceptance: operator SMTP config sends a working link; failures surfaced without leaking outcome to the requester
- [ ] `RMI-OMNIAGENT-106` Magic-link issue/verify
  - Depends on: `RMI-OMNIAGENT-105`
  - Acceptance: hashed-at-rest, 15-min TTL, single-use; first login creates the user; uniform responses (no enumeration)
- [ ] `RMI-OMNIAGENT-107` Cookie session management
  - Depends on: `RMI-OMNIAGENT-106`
  - Acceptance: `__Host-` HttpOnly/Secure/Lax cookie; hashed server-side rows; sliding 30-day expiry; logout revokes
- [ ] `RMI-OMNIAGENT-108` Allowlist enforcement + admin API
  - Acceptance: non-allowlisted email never yields a token row; superadmin CRUD endpoints
- [ ] `RMI-OMNIAGENT-109` Auth middleware + cookie WS upgrade + rate limiting
  - Depends on: `RMI-OMNIAGENT-107`
  - Acceptance: CSRF header on mutations; WS authenticates pre-upgrade and binds user; magic-link endpoints pay the escalating delay under abuse; legacy API-key mode untouched when team mode off

## Phase 3 — Private & Group Chats

**Theme:** Agent-anchored chats with membership fan-out and @-mention agent policy.
**Status:** Planned — 0 of 5 items completed

> **Cross-initiative dependency:** rescoped onto `INIT-OMNIAGENT-005` (Virtual
> Agents, Roles & Registry) — a chat attaches to an `agent_id` and creation is
> gated by `Can(CapCreateChat)`. INIT-005 **RMI-308** adds `chats.agent_id` +
> the `Can()` gate on top of RMI-110; INIT-005 Phases 1–3 must land before this
> phase completes.

- [ ] `RMI-OMNIAGENT-110` Chat + membership service
  - Acceptance: DM (private) with an agent created on demand for permitted users, one per user per agent; group create/invite/leave/remove with chat owner/superadmin rules; invitees join as conversants with no agent-config rights
- [ ] `RMI-OMNIAGENT-111` Message persistence + history API
  - Depends on: `RMI-OMNIAGENT-110`
  - Acceptance: keyset pagination; length cap; RLS-scoped reads/writes
- [ ] `RMI-OMNIAGENT-112` WebSocket chat rooms + fan-out
  - Depends on: `RMI-OMNIAGENT-110`
  - Acceptance: membership-validated subscribe; commit-then-broadcast to all connected members; no cross-chat leakage
- [ ] `RMI-OMNIAGENT-113` Mention policy + agent turns
  - Depends on: `RMI-OMNIAGENT-111`, `RMI-OMNIAGENT-112`
  - Acceptance: group = respond only on `@<handle>`; private = always; agent session keyed `chat:<id>`; reply persisted then broadcast; never self-replies
- [ ] `RMI-OMNIAGENT-114` Memory/tool scoping per chat
  - Depends on: `RMI-OMNIAGENT-113`
  - Acceptance: TenantID=team, SubjectID=chat; group tool/model overrides owner/superadmin-only; rollover works per chat

## Phase 4 — Embedded Web UI

**Theme:** go:embed SPA served from the omniagent binary; Caddy does TLS only.
**Status:** Planned — 0 of 5 items completed

- [ ] `RMI-OMNIAGENT-115` UI scaffold + embedded serving
  - Acceptance: `web/dist` embedded; served at `/` only when team mode enabled; no external assets (CSP-clean)
- [ ] `RMI-OMNIAGENT-116` Login UI (magic link)
  - Depends on: `RMI-OMNIAGENT-115`
- [ ] `RMI-OMNIAGENT-117` Private chat UI
  - Depends on: `RMI-OMNIAGENT-116`
- [ ] `RMI-OMNIAGENT-118` Group chat UI
  - Depends on: `RMI-OMNIAGENT-117`
- [ ] `RMI-OMNIAGENT-119` Admin UI (allowlist, members, rename)
  - Depends on: `RMI-OMNIAGENT-116`

## Phase 5 — SSO (v2)

**Theme:** Google/GitHub sign-in resolving to the same allowlisted accounts.
**Status:** Planned — 0 of 4 items completed

- [ ] `RMI-OMNIAGENT-120` Google OIDC login
- [ ] `RMI-OMNIAGENT-121` GitHub OAuth login
- [ ] `RMI-OMNIAGENT-122` Identity linking by verified email
  - Depends on: `RMI-OMNIAGENT-120`, `RMI-OMNIAGENT-121`
- [ ] `RMI-OMNIAGENT-123` SSO configuration + docs

## Phase 6 — Hosted Deployment

**Theme:** One Lightsail instance: Caddy → omniagent → PostgreSQL via compose.
**Status:** Planned — 0 of 5 items completed

- [ ] `RMI-OMNIAGENT-124` Compose stack (caddy/omniagent/postgres)
  - Acceptance: `docker compose up` yields a working stack; Postgres unexposed; volumes persist across restart
- [ ] `RMI-OMNIAGENT-125` Caddyfile auto-HTTPS
  - Depends on: `RMI-OMNIAGENT-124`
- [ ] `RMI-OMNIAGENT-126` Backup + tested restore
  - Depends on: `RMI-OMNIAGENT-124`
- [ ] `RMI-OMNIAGENT-127` Lightsail deployment guide + smoke checklist
  - Depends on: `RMI-OMNIAGENT-125`
- [ ] `RMI-OMNIAGENT-128` Ops runbook (upgrades, env matrix)
