# OmniAgent Team Agent — Product Requirements Document

> **Initiative:** `INIT-OMNIAGENT-003`
> **Status:** Draft
> **Date:** 2026-08-03

> **Rescope note (2026-08-04):** `INIT-OMNIAGENT-005` introduces the **agent** as
> a first-class multi-tenant entity with per-agent owner/maintainer roles and a
> registry. Phases 1–2 of this initiative (identity, magic-link auth) are
> unaffected and already built. Phase 3 (chats) is **rescoped**: a chat attaches
> to an `agent_id`; "private chat" becomes a **DM with an agent**; group chats are
> **with an agent** (invited members are conversants, no config rights); chat
> creation is gated by the agent registry / `Can(CapCreateChat)` from INIT-005.
> The flat superadmin/member split stays as the **deployment** tier; per-agent
> authority (owner/maintainer/conversant) lives in INIT-005. Group-chat
> participants are distinct from agent owners/maintainers.

## Problem Statement

OmniAgent today is a single-operator agent: one deployment serves one implicit
user, authentication is a shared API key, and every conversation is private to
whoever holds the key. Families and small teams want to share one hosted agent —
with real user identities, controlled membership, private conversations, and
shared group conversations — without running per-person deployments.

## Vision

Evolve OmniAgent into a **team/family agent**: a single self-hosted deployment
(AWS Lightsail–class) serving a small, closed set of users. One person
administers it; members log in without passwords; each member has private chats
with the agent; members can open group chats together where the agent
participates when addressed.

## Users

| Role | Description |
|------|-------------|
| **Superadmin** | The operator ("root"). Bootstrapped at install. Manages the allowlist, users, and their own username. Exactly one at v1. |
| **Member** | An allowlisted person. Logs in via magic link (later SSO). Has a private chat; can create/join group chats. |
| **Agent** | OmniAgent itself. A first-class chat participant, not a user account. |

## User Stories

### Access & Identity

- **US-1**: As the superadmin, I allowlist email addresses so only people I approve can create accounts — there is no open signup.
- **US-2**: As a member, I log in by entering my email and clicking a magic link — no password to create or remember.
- **US-3**: As the superadmin, I can change my own username (and members can set display names) — identity is not frozen at first login.
- **US-4** *(v2)*: As a member, I can sign in with Google or GitHub instead of email links; it resolves to the same account by verified email.
- **US-5**: As the superadmin, I can remove a member; their access ends immediately and their data remains scoped/inaccessible to others.

### Conversations

- **US-6**: As a member, I have a private chat with OmniAgent that no other member (including the superadmin, at the app layer) can read.
- **US-7**: As a member, I can create a group chat and invite other members into it.
- **US-8**: In a group chat, OmniAgent responds **only when @-mentioned**, so members can also talk to each other without the agent replying to everything.
- **US-9**: In my private chat, OmniAgent responds to every message (current behavior).
- **US-10**: As a member, I see chat history when I return, across devices.

### Operation

- **US-11**: As the operator, I deploy the whole stack (agent server + web UI + PostgreSQL + Caddy TLS) on a single small VM with one compose file.
- **US-12**: As the operator, member data survives restarts and upgrades, and I can back up the database.

## Requirements

### Must Have (v1)

1. **PostgreSQL system of record** with **Row-Level Security** enforcing per-user data isolation as defense-in-depth beneath the app layer.
2. **Magic-link authentication** — passwordless, email-delivered, single-use, expiring; closed by the allowlist.
3. **Superadmin** — bootstrapped from configuration; can rename self; manages allowlist and members.
4. **Private chats** — one per member, agent always responds.
5. **Group chats** — create/invite/leave; agent responds only on @-mention; members' messages fan out live.
6. **Embedded web UI** — served from the omniagent binary (`go:embed`): login, chat list, private + group chat, minimal admin.
7. **Compose deployment** — Caddy (auto-HTTPS) → omniagent → PostgreSQL on one Lightsail instance, with backups.

### Should Have (v2)

8. **Google OIDC and GitHub OAuth** login with identity linking by verified email.
9. Admin polish: member management UI, audit view of allowlist changes.

### Non-Goals (this initiative)

- Multiple teams/workspaces per deployment (one team per deployment).
- Public signup, passwords, or password reset flows.
- Federation with the existing messaging channels (Telegram/Discord/WhatsApp) — those remain single-operator features and are out of scope for the multi-user model at v1.
- Mobile/native apps.
- More than one superadmin, or fine-grained RBAC beyond superadmin/member.
- End-to-end encryption of messages.

## Success Criteria

- A family of ~5 can run the stack on one Lightsail instance; each member logs in via magic link within one minute of being allowlisted.
- A member's private chat is invisible to all other members — verified both at the API layer and by RLS policy tests.
- In a group chat, the agent answers @-mentions and stays silent otherwise.
- `docker compose up` + DNS record + allowlist entry is the complete setup path.

## Constraints & Assumptions

- Scale target is a **small** user count (≤ ~25); no horizontal scaling requirement.
- Email delivery via operator-supplied SMTP credentials (any provider).
- The existing single-operator gateway/channel behavior remains available and unbroken for deployments that do not enable team mode.
