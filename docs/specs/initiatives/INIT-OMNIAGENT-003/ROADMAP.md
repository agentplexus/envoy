# OmniAgent Team Agent — Roadmap

**Initiative:** `INIT-OMNIAGENT-003`
**Repository:** `github.com/plexusone/omniagent`
**Status:** Phases 1–2 completed — 12 of 29 items completed

> RMI IDs are stable and permanent. Commits implementing an item carry the
> trailer `Refs: RMI-OMNIAGENT-<NNN>`. Phase status is derived from member
> RMIs — a phase is complete only when all its required RMIs are complete.
> **v1** = Phases 1–4 + 6 · **v2** = Phase 5.

## Phase 1 — Identity & Data Foundation

**Theme:** PostgreSQL + Ent + Row-Level Security as the multi-user system of record.
**Status:** Completed — 5 of 5 items completed

- [x] `RMI-OMNIAGENT-100` Ent schemas + versioned migrations + RLS policies
  - Acceptance: all eight tables migrate from empty; `ENABLE ROW LEVEL SECURITY` binds every policy for the non-owner app role (FORCE intentionally omitted — it would break the owner-bypass `SECURITY DEFINER` membership helpers, TRD §3); policies match TRD §3; migrations embedded and idempotent
- [x] `RMI-OMNIAGENT-101` Dialect-aware store (`AsUser` transaction helper)
  - Depends on: `RMI-OMNIAGENT-100`
  - Acceptance: dialect selected (postgres\|sqlite) from DSN; on postgres every query path flows through per-request `SET LOCAL` GUCs with advisory-locked migrations-on-start; on sqlite `AsUser`/`AsSystem` pass through (single user) and RLS steps are skipped; raw DB unexported in both
- [x] `RMI-OMNIAGENT-102` Team service: users, roles, allowlist
  - Depends on: `RMI-OMNIAGENT-101`
  - Acceptance: rename/display-name, allowlist CRUD superadmin-only at app layer AND by policy
- [x] `RMI-OMNIAGENT-103` Superadmin bootstrap + rename
  - Acceptance: configured email becomes superadmin on first login; later config change does not demote; username change persists and is unique
- [x] `RMI-OMNIAGENT-104` Config (modes/auth axes) + RLS isolation test suite
  - Acceptance: `team.*` config validated; `auth.enabled` independent of `team.enabled` (team implies auth); dialect inferred from DSN; SQL tests prove cross-user isolation per table (postgres)

## Phase 2 — Magic Link Authentication

**Theme:** Passwordless, allowlist-closed login with server-side cookie sessions.
**Status:** Completed — 5 of 5 items completed

- [x] `RMI-OMNIAGENT-105` SMTP delivery + magic-link template
  - Acceptance: operator SMTP config sends a working link; failures surfaced without leaking outcome to the requester
- [x] `RMI-OMNIAGENT-106` Magic-link issue/verify
  - Depends on: `RMI-OMNIAGENT-105`
  - Acceptance: hashed-at-rest, 15-min TTL, single-use; first login creates the user; uniform responses (no enumeration)
- [x] `RMI-OMNIAGENT-107` Cookie session management
  - Depends on: `RMI-OMNIAGENT-106`
  - Acceptance: `__Host-` HttpOnly/Secure/Lax cookie; hashed server-side rows; sliding 30-day expiry; logout revokes
- [x] `RMI-OMNIAGENT-108` Allowlist enforcement + admin API
  - Acceptance: non-allowlisted email never yields a token row; superadmin CRUD endpoints
- [x] `RMI-OMNIAGENT-109` Auth middleware + cookie WS upgrade + rate limiting
  - Depends on: `RMI-OMNIAGENT-107`
  - Acceptance: CSRF header on mutations (`X-OmniAgent-CSRF`); WS authenticates pre-upgrade and binds user; magic-link request endpoint pays the escalating delay keyed by both client IP and email (max of the two, not summed); legacy API-key mode untouched when team mode off

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
  - Depends on: `RMI-OMNIAGENT-111`, `RMI-OMNIAGENT-112`, `INIT-OMNIAGENT-005` (agent entity + `Can()` + runtime binding)
  - Acceptance: group = respond only on `@<agent-slug>`; private = always; chat runs on the bound agent's runtime (persona + enabled skills + agent secrets); agent session keyed `chat:<id>`; reply persisted then broadcast; never self-replies
- [ ] `RMI-OMNIAGENT-114` Memory/tool scoping per chat
  - Depends on: `RMI-OMNIAGENT-113`
  - Acceptance: TenantID=team, SubjectID=chat; skills/model/persona are agent config (owner/maintainer-managed via INIT-005), not per-chat overrides; rollover works per chat

## Phase 4 — Embedded Web UI

**Theme:** One capability-driven go:embed SPA serving both personal and team modes; Caddy does TLS only (team).
**Status:** In progress — 2 of 5 items completed

> Capability-driven (TRD §1a/§6): the same SPA reads `GET /api/capabilities`.
> Login (116) shows only when `authRequired`; group chat (118) and admin (119)
> only when `multiUser`. Personal mode = chat list + history (117) alone.

- [x] `RMI-OMNIAGENT-115` UI scaffold + embedded serving + capabilities endpoint
  - Acceptance: `web/dist` embedded; served at `/` whenever the web UI is enabled (personal or team); `GET /api/capabilities` returns the active mode's flags; team-only routes gated on `team.enabled`; no external assets (CSP-clean)
- [x] `RMI-OMNIAGENT-116` Login UI (magic link)
  - Depends on: `RMI-OMNIAGENT-115`
  - Acceptance: rendered only when `authRequired`; supports single-account personal auth and team allowlist auth — personal single-account auth (`auth.enabled=true`, `team.enabled=false`) reuses team mode's magic-link/cookie stack against the SQLite store with `auth.owner_email` as the sole always-allowed account and the admin allowlist route unregistered (not merely denied)
- [ ] `RMI-OMNIAGENT-117` Private chat UI
  - Depends on: `RMI-OMNIAGENT-115`
  - Acceptance: works in personal mode with no login when auth is off; history keyset scroll-back; live agent replies over WS
- [ ] `RMI-OMNIAGENT-118` Group chat UI
  - Depends on: `RMI-OMNIAGENT-117`
  - Acceptance: rendered only when `multiUser`
- [ ] `RMI-OMNIAGENT-119` Admin UI (allowlist, members, rename)
  - Depends on: `RMI-OMNIAGENT-116`
  - Acceptance: rendered only when `multiUser`

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
