# OmniAgent Team Agent — Roadmap

**Initiative:** `INIT-OMNIAGENT-003`
**Repository:** `github.com/plexusone/omniagent`
**Status:** Phases 1–2 completed — 17 of 29 items completed

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
**Status:** In progress — 3 of 5 items completed

> **Cross-initiative dependency:** rescoped onto `INIT-OMNIAGENT-005` (Virtual
> Agents, Roles & Registry) — a chat attaches to an `agent_id` and creation is
> gated by `Can(CapCreateChat)`. INIT-005 **RMI-308** adds `chats.agent_id` +
> the `Can()` gate on top of RMI-110; INIT-005 Phases 1–3 must land before this
> phase completes. RMI-110/111/112 (chat/membership/message/fan-out) carry no
> INIT-005 dependency and are complete; **RMI-113/114 remain blocked** on the
> INIT-005 agent entity + `Can()` + per-agent runtime binding.

- [x] `RMI-OMNIAGENT-110` Chat + membership service
  - Acceptance: DM (private) with an agent created on demand for permitted users, one per user per agent; group create/invite/leave/remove with chat owner/superadmin rules; invitees join as conversants with no agent-config rights. Implemented in `team/chats`: `CreateGroup`, `Invite` (by username, resolved in system context since RLS hides other users; idempotent; owner/superadmin only), `Leave` (self-leave with sole-owner orphan guard), `RemoveMember` (owner/superadmin; owners not removable, no self-remove), `ListChats`/`GetChat`/`Members`/`MembersDetailed` (member/superadmin scoped). Backstopped by the existing chats/chat_members RLS policies (Postgres) and enforced at the service layer for the SQLite path.
- [x] `RMI-OMNIAGENT-111` Message persistence + history API
  - Depends on: `RMI-OMNIAGENT-110`
  - Acceptance: keyset pagination; length cap; RLS-scoped reads/writes. Team chat HTTP (`gateway/TeamChatHTTP`) exposes `GET /api/chats`, `POST /api/chats`, `GET /api/chats/dm`, `GET /api/chats/{id}`, `GET /api/chats/{id}/messages?before=&limit=` (backward keyset via `chats.HistoryBefore`, `hasMore` probe, limit clamped to 100), `POST /api/chats/{id}/messages` (`MaxMessageBytes` cap), and `/members`/`/leave` routes; CSRF-guarded mutations, cookie-authenticated principal.
- [x] `RMI-OMNIAGENT-112` WebSocket chat rooms + fan-out
  - Depends on: `RMI-OMNIAGENT-110`
  - Acceptance: membership-validated subscribe; commit-then-broadcast to all connected members; no cross-chat leakage. `Gateway.BroadcastToUsers` delivers a persisted message only to the sockets whose bound `user_id` is in the chat's member set (computed per send); non-members and unauthenticated sockets never receive it. Persist-then-fan-out: the HTTP handler persists the user message, then broadcasts a `chat.message` event (channel = chat ID) to members; private DMs additionally run the agent turn out-of-band and fan out the reply.
- [ ] `RMI-OMNIAGENT-113` Mention policy + agent turns
  - Depends on: `RMI-OMNIAGENT-111`, `RMI-OMNIAGENT-112`, `INIT-OMNIAGENT-005` (agent entity + `Can()` + runtime binding)
  - Acceptance: group = respond only on `@<agent-slug>`; private = always; chat runs on the bound agent's runtime (persona + enabled skills + agent secrets); agent session keyed `chat:<id>`; reply persisted then broadcast; never self-replies
- [ ] `RMI-OMNIAGENT-114` Memory/tool scoping per chat
  - Depends on: `RMI-OMNIAGENT-113`
  - Acceptance: TenantID=team, SubjectID=chat; skills/model/persona are agent config (owner/maintainer-managed via INIT-005), not per-chat overrides; rollover works per chat

## Phase 4 — Embedded Web UI

**Theme:** One capability-driven go:embed SPA serving both personal and team modes; Caddy does TLS only (team).
**Status:** In progress — 4 of 5 items completed

> Capability-driven (TRD §1a/§6): the same SPA reads `GET /api/capabilities`.
> Login (116) shows only when `authRequired`; group chat (118) and admin (119)
> only when `multiUser`. Personal mode = chat list + history (117) alone.

- [x] `RMI-OMNIAGENT-115` UI scaffold + embedded serving + capabilities endpoint
  - Acceptance: `web/dist` embedded; served at `/` whenever the web UI is enabled (personal or team); `GET /api/capabilities` returns the active mode's flags; team-only routes gated on `team.enabled`; no external assets (CSP-clean)
- [x] `RMI-OMNIAGENT-116` Login UI (magic link)
  - Depends on: `RMI-OMNIAGENT-115`
  - Acceptance: rendered only when `authRequired`; supports single-account personal auth and team allowlist auth — personal single-account auth (`auth.enabled=true`, `team.enabled=false`) reuses team mode's magic-link/cookie stack against the SQLite store with `auth.owner_email` as the sole always-allowed account and the admin allowlist route unregistered (not merely denied)
- [x] `RMI-OMNIAGENT-117` Private chat UI
  - Depends on: `RMI-OMNIAGENT-115`
  - Acceptance: works in personal mode with no login when auth is off; history keyset scroll-back (`GET /api/chat` newest page + `hasMore`, `GET /api/chat/history?before=&limit=` for older pages, backed by `chats.HistoryBefore` backward keyset); live agent replies over WS (`POST /api/chat/messages` persists the user message and returns 202 immediately; the agent turn runs on a detached context and its reply is broadcast as a `chat.message` WS event — the composer re-enables on WS delivery, and a reconnect resyncs missed replies). WS upgrade is cookie-gated in personal-auth mode so replies never reach an unauthenticated socket.
- [x] `RMI-OMNIAGENT-118` Group chat UI
  - Depends on: `RMI-OMNIAGENT-117`
  - Acceptance: rendered only when `multiUser`. The SPA's `multiUser` branch renders `renderTeamChat`: a two-pane surface (chat-list sidebar + message pane) over the RMI-110/111/112 endpoint set. "Chat with agent" get-or-creates the private DM (`GET /api/chats/dm`); "New group" creates a group (`POST /api/chats`); selecting a chat loads its newest page with keyset scroll-back (`GET /api/chats/{id}` + `/messages?before=&limit=`). One shared WebSocket delivers `chat.message` fan-out filtered by `chatId` (unread dots on non-active chats); messages are attributed per-author via the new `authorUserId` message field resolved against the member map (self/other/agent). Groups get a member panel: list with roles, owner-only invite-by-username and remove, and leave (`/members`, `DELETE /members/{id}`, `/leave`), refreshed live on `chat.member.added`/`removed`. CSRF header on all mutations; CSP-clean (no external assets, guarded by `web` tests). Agent participation in groups (@-mention turns) stays deferred to RMI-113/INIT-005 — group sends fan out with no agent reply.
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
