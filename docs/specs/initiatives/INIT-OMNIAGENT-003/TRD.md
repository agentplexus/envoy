# OmniAgent Team Agent — Technical Requirements Document

> **Initiative:** `INIT-OMNIAGENT-003`
> **Status:** Draft
> **Companion:** [PRD.md](PRD.md) · [PLAN.md](PLAN.md) · [ROADMAP.md](ROADMAP.md)

## 1. Architecture Overview

```
                    ┌──────────────────────── Lightsail instance ───────────────────────┐
  Browser ── HTTPS ─►  Caddy (TLS, reverse proxy)                                       │
                    │      │                                                            │
                    │      ▼                                                            │
                    │  omniagent (single Go binary)                                     │
                    │   ├─ embedded web UI (go:embed, web/dist)                         │
                    │   ├─ auth: magic link + cookie sessions        ┌─────────────┐    │
                    │   ├─ team service (users/allowlist/chats) ────►│ PostgreSQL  │    │
                    │   ├─ gateway WS (chat fan-out by membership)   │  (RLS)      │    │
                    │   └─ agent core (LLM, tools, memory)           └─────────────┘    │
                    │        └─ sessions/cron/memory: existing KVS (sqlite file)        │
                    └───────────────────────────────────────────────────────────────────┘
```

**Storage split (deliberate):**

| Data | Store | Rationale |
|------|-------|-----------|
| Users, identities, allowlist, auth sessions, magic-link tokens, chats, memberships, **messages** | PostgreSQL + RLS | Multi-user system of record; isolation is a security property |
| Agent conversation context (LLM window), cron jobs, skill state | Existing KVS (`omnistorage-core`, sqlite file) | Derived/truncatable state (rollover applies); no cross-user exposure — keyed by chat |
| Semantic memory | omnimemory (KVS provider at v1) | Scoped `TenantID=team`, `SubjectID=chat`; may move to its Postgres provider later |

The **canonical chat transcript lives in PostgreSQL**; the agent's
`sessions.Store` context (keyed `chat:<id>`) is a derived working set that
rollover/compaction may truncate without losing the durable record.

## 2. Data Model (Ent on PostgreSQL)

ORM: **Ent** (org default; Postgres supported). Versioned migrations
(Atlas-generated) embedded via `embed.FS`, applied at startup. RLS policies are
hand-written SQL appended as custom migration steps — Ent does not model them.

```
users            id (uuid pk) · email (citext unique) · username (citext unique)
                 display_name · role (superadmin|member) · status (active|disabled)
                 created_at · updated_at

identities       id · user_id fk · provider (magic_link|google|github)
                 provider_subject (unique per provider) · verified_email · created_at

allowlist        id · email (citext unique) · added_by fk users · note · created_at

magic_link_tokens id · email · token_hash (sha256, unique) · expires_at
                 consumed_at (null) · created_ip · created_at

auth_sessions    id · user_id fk · token_hash (sha256, unique) · created_at
                 last_seen_at · expires_at · user_agent · created_ip

chats            id (uuid pk) · agent_id fk agents · type (private|group)
                 name (null for private) · created_by fk users · created_at
                 -- partial unique index: one DM (private chat) per user per agent
                 UNIQUE (created_by, agent_id) WHERE type = 'private'

chat_members     chat_id fk · user_id fk · role (owner|member) · joined_at
                 PRIMARY KEY (chat_id, user_id)
                 -- chat roles govern membership only; they are orthogonal to
                 -- agent_roles (owner/maintainer) and confer no config rights

messages         id (uuid pk) · chat_id fk · author_type (user|agent)
                 author_user_id (fk, null when agent) · content (text)
                 created_at · idx (chat_id, created_at)
```

> **Rescope (INIT-OMNIAGENT-005 dependency):** the `agents`, `agent_roles`, and
> `agent_skills` tables that `chats.agent_id` references are defined and migrated
> by [INIT-OMNIAGENT-005](../INIT-OMNIAGENT-005/TRD.md) (Virtual Agents, Roles &
> Registry). Phase 3 here consumes that model; it does not define agents. See §5
> for how a chat binds to its agent, and §10 for the sequencing constraint.

## 3. Row-Level Security

> **Team mode only.** This entire section applies to the PostgreSQL dialect. In
> personal mode (SQLite, one implicit user) there is nothing to isolate: the
> `0001–0003_*.sql` policy/function/grant migrations are skipped, the two-role
> owner/app split collapses to a single connection, and `store.AsUser` /
> `AsSystem` become pass-throughs binding the sole user. The service-layer
> authorization checks (the primary gate) still run in both modes.

**Connection model (two roles, as built):** migrations run as the **owner**
role (`MigrateDSN`); the app connects as the **non-owner** `omniagent_app`
(`AppDSN`), so plain `ENABLE ROW LEVEL SECURITY` binds every policy for the
app. The owner's RLS bypass is deliberate: the `SECURITY DEFINER` membership
helpers (`team_is_chat_member/owner/creator`) are owner-owned so policies on
`chats`/`chat_members`/`messages` can consult `chat_members` without policy
recursion. (`FORCE` is intentionally not used — it would break those
helpers.) Per request, inside a transaction:

```sql
SELECT set_config('app.current_user_id', $1, true),  -- SET LOCAL semantics
       set_config('app.is_superadmin',  $2, true),
       set_config('app.is_system',      $3, true);   -- auth layer / agent
```

`team/store` exposes exactly two entry points — `AsUser(userID, superadmin,
fn)` and `AsSystem(fn)` — which open the transaction, set the GUCs, and hand
the scoped `*ent.Tx` to the service layer. **No query path may bypass them**
(the raw `*sql.DB` is unexported). `app.is_system` marks the auth layer and
agent plumbing: it is the only context that may touch `magic_link_tokens`,
create users/identities/sessions, or author `agent` messages.

**Policies (informal):**

| Table | SELECT | INSERT/UPDATE/DELETE |
|-------|--------|----------------------|
| `users` | own row; superadmin: all | own display fields; superadmin: all |
| `allowlist` | superadmin | superadmin |
| `auth_sessions` | own | own (logout); superadmin: revoke any |
| `chats` | member of chat | create (INSERT gated at the app layer by `Can(actor, agent_id, CapCreateChat)`; RLS backstop requires the creator be a permitted starter of the bound agent); owner/superadmin: update/delete |
| `chat_members` | member of same chat | chat owner invites/removes; self-leave |
| `messages` | member of chat | member of chat inserts (author = self) |
| `magic_link_tokens` | *no app-role access* — touched only by the auth layer via SECURITY DEFINER functions or a separate narrowly-granted role |

**Defense-in-depth:** the service layer performs the same authorization checks;
RLS is the backstop that makes an app-layer bug a non-event. Policy behavior is
pinned by SQL-level tests (member A cannot read member B's private chat even
with a hand-crafted query through the app role).

**Agent writes:** the agent authors messages with `author_type='agent'`. Agent
inserts run under a dedicated `app.current_user_id = <system sentinel>` context
that policies allow only for `messages` in chats where the write was triggered
by a member turn (service-layer invariant; RLS allows sentinel inserts to any
chat — acceptable at v1, tightened later if needed).

## 4. Authentication

### 4.1 Magic Link (v1)

1. `POST /auth/magic-link {email}` — **uniform response regardless of outcome**
   (no email enumeration). If the email is allowlisted (or is the configured
   superadmin email), issue a token: 256-bit random, stored **SHA-256 hashed**,
   TTL 15 min, single-use. Send via SMTP (`config: team.smtp.*`).
   Request path is rate-limited by the existing gateway escalating-delay
   limiter, keyed by client IP **and** by email.
2. `GET /auth/verify?token=...` — constant-time lookup by hash; must be
   unexpired and unconsumed; marks consumed; creates the user on first login
   (email → user; username defaults from the email local part, uniquified);
   opens an auth session.
3. **Session cookie:** `__Host-oa_session`, `HttpOnly`, `Secure`,
   `SameSite=Lax`; value is a 256-bit random token stored hashed in
   `auth_sessions`; sliding expiry 30 days; logout deletes the row.
4. **CSRF:** state-changing HTTP endpoints require the `X-OmniAgent-CSRF: 1`
   custom header (forbidden header pattern) in addition to `SameSite=Lax`.

### 4.2 Superadmin Bootstrap

`team.superadmin_email` in config. On first successful login by that email the
user is created with `role=superadmin`. Changing the config value later does
not demote the existing superadmin (explicit DB action required); it only
governs bootstrap. The superadmin can change their `username` (US-3) via the
users API — usernames are unique, `citext`.

### 4.3 SSO (v2)

Google (OIDC, `coreos/go-oidc`) and GitHub (OAuth2). Callback resolves the
provider's **verified** email → existing user (identity linked) or, if
allowlisted, creates one. A user may hold multiple identities. SSO never
bypasses the allowlist.

### 4.4 WebSocket Authentication

Browser WS connects with the session cookie; the upgrade handler authenticates
**before upgrade** (replacing the in-band `auth` message for team mode) and
binds the `Client` to the user ID. The legacy API-key path remains for
non-team/programmatic use behind the existing config.

## 5. Chats & Agent Participation

Every chat binds to exactly one **agent** (`chats.agent_id`, defined by
INIT-OMNIAGENT-005). "The agent" below means that chat's bound agent, not a
single deployment-wide agent. Which agents a user may start a chat with — and
therefore which agents they can DM or open a group around — is governed by the
INIT-005 registry via `Can(actor, agent, CapCreateChat)` (listed agents: any
allowlisted user; private agents: the agent's owner/maintainers and people they
invite). Per-agent roles (owner/maintainer) are orthogonal to chat membership:
being a chat member never confers the right to configure the bound agent.

- **Private chat (DM with an agent):** created on demand when a permitted user
  first opens a DM with an agent (`type=private`, sole member), not blanket at
  first login. At most one DM per (user, agent). Every member message triggers
  the agent (current single-agent behavior, now per bound agent).
- **Group chat:** any user permitted to start a chat with the agent creates one
  (`type=group`, becomes chat `owner`); invites by username/email (must be
  existing members of the team); invited members join as **conversants** — they
  can talk but hold no configuration rights over the agent. Members can leave;
  the chat owner or superadmin can remove members.
- **Fan-out:** the gateway maintains chat-scoped rooms. On message insert
  (transaction committed), the server broadcasts to connected members of that
  chat. Subscription requests are validated against `chat_members`.
- **Mention policy:** the agent's handle is its slug (INIT-005 `agents.slug`);
  no global `team.agent_handle`. In group chats the agent processes a message
  only when it matches `(^|\s)@<slug>\b` (case-insensitive). In private chats it
  always processes. The agent never responds to its own messages.
- **Agent session & runtime:** one session per chat, key `chat:<id>` — group
  members share context by design. `TenantID = team`, `SubjectID/SessionID =
  chat:<id>` for memory scoping. The chat's turns run on the bound agent's
  runtime instance (persona + enabled skills + agent-scoped secrets), per
  INIT-005 §6 — the deployment no longer runs one implicit agent. Skills, model,
  and persona are **agent** configuration (owner/maintainer-managed via INIT-005),
  not per-chat overrides mutable by chat members. Chat-scoped session features
  that remain per chat (rollover, memory) apply per chat.
- **Ordering/limits:** message length capped (config, default 32 KiB); history
  API paginates by `(created_at, id)` keyset.

## 6. Web UI (embedded)

- Lives in `web/` (this repo); built as a static SPA; output embedded with
  `go:embed` and served by the gateway mux at `/` (API under `/api/`, WS at
  `/ws`). No CDN/external assets. Caddy does TLS only.
- v1 surface: login (request/verify magic link) · chat list · private chat ·
  group chat (create/invite/member list, @-mention affordance) · minimal admin
  (allowlist CRUD, member list, rename username) · logout.
- The existing single-operator behavior is preserved: when team mode is
  disabled (`team.enabled=false`, default), none of the new routes register.

## 7. Deployment Profiles & Ecosystem Alignment

Two deliberate deployment profiles coexist under `PROG-OMNIAGENT-LIGHTSAIL`;
the deciding variable is **whether multiple users exist**, not storage
preference:

| Profile | Initiative | Shape | Storage |
|---------|-----------|-------|---------|
| **Bot** (single-operator) | `INIT-GROKIFYOMNIAGENT-001` | Static binary + systemd, outbound-only, SSH-only firewall, no Caddy | SQLite KVS on `/data` |
| **Team** (multi-user) | `INIT-OMNIAGENT-003` (this) | Compose: Caddy + omniagent + PostgreSQL, inbound HTTPS | PostgreSQL + RLS for user data; KVS for agent-internal state |

Policy: **PostgreSQL wherever users exist; KVS-SQLite for single-operator mode
and for agent-internal state in both profiles.** RLS is the requirement that
forces Postgres in team mode; nothing forces it on the bot profile, where the
zero-dependency single binary is a feature. The bot initiative's deploy
scripts and nightly-backup pattern are the reference implementations the team
profile's compose/backup RMIs consolidate (`relates` links recorded:
124↔GROKIFY-007, 126↔GROKIFY-010, 127↔GROKIFY-009).

### Sizing (PostgreSQL on a small Lightsail instance)

`postgres:16-alpine` idles at ~30–50 MB RSS; a ≤25-user team is single-digit
QPS at worst. Recommended: the **2 GB RAM / 1 vCPU instance (~$10/mo)** —
Caddy (~30 MB) + omniagent (~100–150 MB) + Postgres
(`shared_buffers=128–256MB`) with ample headroom. The 1 GB instance works with
`shared_buffers=64MB` + swap but is tight. Lightsail *managed* Postgres
(≥$15/mo) costs more than the whole instance and is unnecessary at this scale;
the co-located container plus tested `pg_dump` restore is the chosen path.

## 8. Deployment (Lightsail, compose)

```yaml
services:
  caddy:      # :443 → omniagent:8080; automatic HTTPS for team domain
  omniagent:  # single binary, embedded UI + migrations-on-start
  postgres:   # postgres:16; volume-backed; not exposed publicly
volumes: caddy_data, pg_data, omniagent_data   # omniagent_data = KVS sqlite
```

- Migrations apply on startup (advisory-locked so restarts are safe).
- Backups: nightly `pg_dump` sidecar (or host cron) to the instance disk with
  documented off-host copy step; restore procedure documented and tested.
- Secrets (SMTP, LLM API keys, DB password) via compose `env_file`; never in
  the image. Postgres reachable only on the compose network.

## 9. Security Considerations

- Closed signup: allowlist checked **before** any token is issued.
- Tokens (magic link, session) stored only as SHA-256 hashes; constant-time
  comparison; single-use links; short TTLs.
- Rate limiting: reuse `gateway` escalating-delay limiter for magic-link
  requests and verify attempts (keyed by IP and email); loopback never locked out.
- RLS as backstop + app-layer checks as primary; both tested.
- Uniform auth responses (no user/allowlist enumeration).
- Group invite restricted to existing team members — invites never create accounts.
- Audit trail: allowlist and membership changes recorded (`added_by`,
  timestamps); superadmin actions loggable via existing hooks (`agent_end`,
  `session.*` events unaffected).

## 10. Cross-Initiative Sequencing (INIT-005)

Phase 3 (Private & Group Chats) depends on
[INIT-OMNIAGENT-005](../INIT-OMNIAGENT-005/PRD.md) (Virtual Agents, Roles &
Registry), which owns the `agents`/`agent_roles`/`agent_skills` model,
`Can(...)` authorization, and per-agent runtime binding that `chats.agent_id`
and start-chat authz rest on. Phases 1–2 (identity, magic-link auth) and Phase 4
(web UI) and Phase 6 (deployment) carry no such dependency. Concretely:

- **INIT-003 RMI-110** (chat/membership service) provides the base that INIT-005
  **RMI-308** extends by adding `chats.agent_id` and the `Can(CapCreateChat)`
  gate — so RMI-110 lands first, then RMI-308, then the remaining Phase 3 RMIs
  (111–114) build on the agent-anchored model.
- INIT-005 Phases 1–3 (agent entity → per-agent roles → registry + start-chat
  authz) must be delivered before INIT-003 Phase 3 completes.
- Dependencies are modeled at **RMI granularity** in VisionStudio, where the
  graph stays acyclic: INIT-005 `RMI-308 requires RMI-110` (agent_id integration
  builds on the chat base), and INIT-003 `RMI-113 requires RMI-308` +
  `RMI-113 requires RMI-309` (agent turns need `chats.agent_id` and the per-agent
  runtime). No initiative-level `requires` edge is used: the two initiatives
  interleave at different phases, so projecting the relationship up to initiative
  granularity would falsely imply a cycle.

## 11. Open Questions

1. Should the superadmin be able to *read* group chats they are not a member
   of? v1: **no** — superadmin power is administrative (membership), not
   content access. Revisit if moderation is needed.
2. omnimemory on Postgres (its `postgres` provider) with RLS alignment —
   deferred; v1 keeps KVS memory keyed by chat.
3. Streaming responses in group chats (token streaming to all members) — v1
   sends complete agent messages; streaming is a UI enhancement later.
