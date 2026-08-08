# OmniAgent Team Agent — Implementation Plan

> **Initiative:** `INIT-OMNIAGENT-003`
> Phases and RMI IDs match [ROADMAP.md](ROADMAP.md). Each phase is executed and
> reviewed as a unit; phase status derives from member RMIs. Commits carry
> `Refs: RMI-OMNIAGENT-<NNN>`.

## Package Layout (library-first)

```
team/                    # importable service layer (no HTTP/WS types)
├── ent/                 # Ent schemas + generated code (users, chats, …)
├── store/               # DB open, migrations, RLS transaction scoping
├── auth/                # magic link, cookie sessions, (v2: oidc/github)
├── mail/                # SMTP delivery + templates
├── chats/               # chat/membership/message service, mention policy
└── team.go              # Service facade wired from config

gateway/                 # thin adapters over team.Service
├── team_http.go         # /api/auth/*, /api/chats/*, /api/admin/* + CSRF
├── team_ws.go           # cookie-auth upgrade, chat rooms, fan-out
web/                     # SPA source; web/dist embedded via go:embed
deploy/team/             # docker-compose.yaml, Caddyfile, backup script, docs
config/                  # TeamConfig section
```

Existing packages are extended, not forked: the agent core, sessions KVS,
hooks, and single-operator gateway behavior stay intact behind
`team.enabled=false` (default).

## Phase 1 — Identity & Data Foundation

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-100 | `team/ent` schemas (users, identities, allowlist, magic_link_tokens, auth_sessions, chats, chat_members, messages); Atlas versioned migrations embedded via `embed.FS`; RLS policies + `FORCE ROW LEVEL SECURITY` as custom migration SQL |
| RMI-OMNIAGENT-101 | `team/store`: open as `omniagent_app` role; migrations-on-start under advisory lock; `store.AsUser(ctx, userID, isSuperadmin, fn)` transaction helper setting `SET LOCAL app.current_user_id` / `app.is_superadmin`; raw DB kept unexported |
| RMI-OMNIAGENT-102 | `team` service: users (get/rename/display name), roles, allowlist CRUD; app-layer authorization mirroring RLS |
| RMI-OMNIAGENT-103 | Superadmin bootstrap from `team.superadmin_email`; rename-username support incl. superadmin (US-3); uniqueness via citext |
| RMI-OMNIAGENT-104 | `config.TeamConfig` (enabled, database DSN, base_url, superadmin_email, agent_handle, smtp.*) + validation + gateway command wiring; RLS policy test suite (cross-user isolation pinned in SQL tests) |

**Verification gate:** RLS tests prove member A cannot read member B's rows
through the app role for every table; migrations idempotent across restarts.

## Phase 2 — Magic Link Authentication

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-105 | `team/mail`: SMTP client from config; magic-link template (text+HTML); send timeout/backoff |
| RMI-OMNIAGENT-106 | `team/auth`: issue (allowlist-gated, hashed-at-rest, 15-min TTL, single-use) + verify (constant-time, consume, first-login user creation); uniform responses |
| RMI-OMNIAGENT-107 | Cookie sessions: `__Host-oa_session` HttpOnly/Secure/Lax; hashed server-side rows; sliding 30-day expiry; logout; revocation |
| RMI-OMNIAGENT-108 | Allowlist admin API (`/api/admin/allowlist` CRUD, superadmin-only) + enforcement ordering (checked before token issue) |
| RMI-OMNIAGENT-109 | HTTP middleware (auth + CSRF header) and cookie-authenticated WS upgrade binding Client→user; magic-link + verify endpoints rate-limited via the existing gateway escalating-delay limiter (IP + email keys); legacy API-key path preserved when team mode off |

**Verification gate:** end-to-end login test (request → mail capture → verify →
cookie → authed API call); non-allowlisted email gets uniform response and no
token row.

## Phase 3 — Private & Group Chats

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-110 | `team/chats`: private auto-chat at first login; group create/invite/leave/remove (owner/superadmin rules); membership queries |
| RMI-OMNIAGENT-111 | Message persistence + keyset-paginated history API (`/api/chats/{id}/messages`); length cap; RLS-scoped |
| RMI-OMNIAGENT-112 | Gateway chat rooms: membership-validated subscribe; commit-then-broadcast fan-out to connected members; room GC on disconnect |
| RMI-OMNIAGENT-113 | Mention policy + agent turn: `@<handle>` detection (group) vs always-respond (private); agent session key `chat:<id>`; agent reply persisted as `author_type=agent` then broadcast; no self-replies |
| RMI-OMNIAGENT-114 | Scoping integration: `TenantID=team`, `SubjectID=chat:<id>` for memory; per-chat tool/model overrides restricted to owner/superadmin in groups; rollover works per chat |

**Verification gate:** two-member group chat over two WS connections: plain
message reaches both with no agent reply; `@omniagent` message produces exactly
one agent reply visible to both; private chats remain isolated.

## Phase 4 — Embedded Web UI (v1 slice)

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-115 | `web/` scaffold (no external runtime deps; build to `web/dist`), `go:embed` + gateway mux serving `/` with API/WS passthrough; team-mode-only registration |
| RMI-OMNIAGENT-116 | Login UI: request magic link, verify landing, logged-in shell, logout |
| RMI-OMNIAGENT-117 | Private chat UI: history (keyset scroll-back), send, live agent replies over WS |
| RMI-OMNIAGENT-118 | Group chat UI: create, invite, member list, leave; @-mention autocomplete for the agent handle |
| RMI-OMNIAGENT-119 | Admin UI: allowlist CRUD, member list/disable, rename username |

## Phase 5 — SSO (v2)

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-120 | Google OIDC login (`coreos/go-oidc`), allowlist-gated |
| RMI-OMNIAGENT-121 | GitHub OAuth login, verified-primary-email resolution |
| RMI-OMNIAGENT-122 | Identity linking: verified email → existing user; multiple identities per user; UI buttons |
| RMI-OMNIAGENT-123 | SSO provider configuration + setup docs (redirect URIs, credentials) |

## Phase 6 — Hosted Deployment

| RMI | Work |
|-----|------|
| RMI-OMNIAGENT-124 | `deploy/team/docker-compose.yaml`: caddy + omniagent + postgres:16, volumes, env_file secrets, healthchecks; Postgres unexposed |
| RMI-OMNIAGENT-125 | Caddyfile: auto-HTTPS for the team domain, reverse proxy, sane headers (HSTS) |
| RMI-OMNIAGENT-126 | Backup/restore: nightly `pg_dump` + retention + documented, tested restore |
| RMI-OMNIAGENT-127 | Lightsail guide: instance sizing, DNS, compose bring-up, allowlist-first-login smoke test checklist |
| RMI-OMNIAGENT-128 | Ops runbook: upgrade path (image bump + auto-migrations), env matrix, troubleshooting |

## Milestones

- **v1** = Phases 1–4 + Phase 6 (deployable family/team agent, magic-link only).
- **v2** = Phase 5 (SSO) + admin polish + deferred items (TRD §10).

## Definition of Done (per plexusone org)

Every RMI's change: follows repo patterns; unit tests (RLS behavior pinned in
SQL tests where applicable); `golangci-lint` clean; docs updated on
user-visible change; conventional commit with `Refs: RMI-OMNIAGENT-<NNN>`
trailer; no local `replace` directives.
