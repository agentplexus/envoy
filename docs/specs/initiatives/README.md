# OmniAgent Initiatives — Orientation

Single map of the active initiatives, how they layer, the recommended build
order, and where reality currently sits. Start here when picking up this work in
a fresh session. Per-initiative detail lives in each
`docs/specs/initiatives/INIT-OMNIAGENT-00N/{PRD,TRD,PLAN,ROADMAP}.md`.

> **Tracking authority:** vistudio (PRISM). The database of record is at
> `~/.productbuildershq/prism/prismcontrol` — **not** the CLI's documented
> `~/.productbuildershq/visionstudio` default (that copy is stale). See
> [ENGINEERING.md](../../../ENGINEERING.md). Query with `vistudio initiative get
> INIT-OMNIAGENT-00N` and `vistudio rmi list --initiative …`.

## The initiatives

| ID | Title | Status | RMIs | What it is |
|----|-------|--------|------|-----------|
| `INIT-OMNIAGENT-001` | Delivery — Deployment, Discord, Gateway Hardening | executing | 001–030 | Original product delivery roadmap (Lightsail deploy, Discord, gateway security, test coverage, context/token mgmt). |
| `INIT-OMNIAGENT-002` | OpenClaw Parity Sync — July 2026 | executing | 050–077 | Port curated upstream OpenClaw fixes; see `parity-2026-07-30/PORT-SPEC.md` for the diff-level analysis. |
| `INIT-OMNIAGENT-003` | Team Agent — Multi-User Hosted Evolution | executing | 100–128 | PostgreSQL + RLS identity, magic-link auth, chats, embedded web UI, Lightsail compose deploy. |
| `INIT-OMNIAGENT-004` | Skill Secrets — GitHub-Style Secret Management | proposed | 200–213 | Skills declare secrets; omnivault-backed bindings; **agent-scoped** secrets managed by owner/maintainers; injection + redaction. |
| `INIT-OMNIAGENT-005` | Virtual Agents, Roles & Registry | proposed | 300–313 | **Agent as a first-class entity** = (skill subset + agent secrets + persona); per-agent owner/maintainer roles; registry (private/listed + superadmin featured); per-agent runtime binding. |

Also in the ecosystem (separate repos, one program):
`INIT-GROKIFYOMNIAGENT-001` (Discord-on-Lightsail bot profile) and
`INIT-OMNIDEPLOY-001`, grouped under `PROG-OMNIAGENT-LIGHTSAIL`.

## How 003 / 004 / 005 layer

`INIT-005` is the foundation the other two rescoped onto (decision 2026-08-04):

```
        INIT-005  Virtual Agents  ── agent = skills + agent-scoped secrets + persona
                  (owner / maintainer / conversant; registry)
                 ▲                                   ▲
   chats.agent_id│                                   │ agent-scoped
        INIT-003  Chats & Identity          INIT-004  Skill Secrets
        (DM/group attach to an agent;       (belong to the agent, managed by
         conversants converse only)          owner/maintainers; GitHub repo-secret model)
```

- A **chat** (003) attaches to an `agent_id`; "private chat" = a **DM with an agent**.
- A **secret** (004) belongs to an **agent**, managed by that agent's
  owner/maintainers — not per end-user. (This is a rescope; see the note at the
  top of `INIT-OMNIAGENT-004/PRD.md`.)
- Authorization is three tiers: **deployment** (superadmin, from 003) ·
  **per-agent** (owner/maintainer, from 005) · **per-chat** (conversant). Roles
  are orthogonal to chat membership — conversing never grants configuration.

## Recommended build order (critical path across initiatives)

Dependency edges are recorded in vistudio; the prose sequence:

1. **003 Phase 1–2** — identity + magic-link auth. *(Implemented, uncommitted — see below.)*
2. **005 Phase 1–2** — agent entity + per-agent roles/authz (needs the 003 RLS store, `RMI-101`).
3. **003 Phase 3** — chats, rescoped to attach to agents (needs 005 agent entity `RMI-301`/registry `RMI-308`).
4. **004 Phase 1** — skill secret declaration + global bindings + injection (single-operator; no team dependency; can proceed in parallel).
5. **004 Phase 2–3** — agent-scoped encrypted secret store + resolution (needs 005 agents `RMI-300` + roles `RMI-305`).
6. **005 Phase 4** — per-agent runtime binding of skills + secrets (needs 004 `RMI-207` + 003 chats).
7. **UIs**: 003 Phase 4 web UI → 004/005 management + catalog UIs → deployment (003 Phase 6).

Independent quick wins with no team dependency: **004 Phase 1** (core skill
secrets) and the pending **INIT-002 gap fixes** (Phase 5, RMIs 070–077).

## Current reality (2026-08-04)

**Implemented but UNCOMMITTED** — a large working-tree stack with no git history
behind it yet. `git log` will *not* reveal it; the record is in vistudio RMI
handoff notes (`vistudio work status`, and each RMI's handoff). Before building
on it, read those handoffs and run `git status`.

- **INIT-002 gap fixes (RMIs 064, 070, 071, 072*, 073, 074*, 075, 076*)** — the
  starred ones committed (`072`, `074`, `076`), the rest implemented-uncommitted.
- **INIT-003 Phases 1–2 (RMIs 100–109)** — **fully implemented, uncommitted**,
  proven against real PostgreSQL and a live server smoke test. Packages:
  `team/{store,ent,auth,mail}`, `gateway/team_http.go`, `internal/pgtest`,
  `cmd/omniagent/commands/team.go`, `deploy/team/dev/`. RMI status is
  `in_progress` pending commit.
- **INIT-003 Phases 3–6, INIT-004, INIT-005** — planned, not started. Specs are
  the source of truth.

**Committed baseline:** `2a4875f` (temporal context) and earlier.

## Running the team stack locally

See [`deploy/team/dev/TESTING.md`](../../../deploy/team/dev/TESTING.md) — a full
magic-link walkthrough. Short form:

```bash
docker compose -f deploy/team/dev/docker-compose.dev.yaml up -d --wait   # Postgres 16 on :5433, two roles
go run ./cmd/omniagent gateway run --config deploy/team/dev/config.dev.yaml
# request a magic link, copy the URL from the server log, open it
```

PostgreSQL-backed tests are skipped unless the two DSNs are set:

```bash
export TEAM_TEST_OWNER_DSN="postgres://omniagent_owner:owner_dev_password@127.0.0.1:5433/omniagent_team?sslmode=disable"
export TEAM_TEST_APP_DSN="postgres://omniagent_app:app_dev_password@127.0.0.1:5433/omniagent_team?sslmode=disable"
go test ./team/... ./gateway/
```

## Key decisions (so they are not relitigated)

- **Ent + PostgreSQL** for the team store (RLS forces Postgres; a deliberate
  deviation from the org's MySQL-compatible default, documented in the 003 TRD).
- **Two-role RLS model**: owner migrates, non-owner `omniagent_app` runs; plain
  `ENABLE` (not `FORCE`) so `SECURITY DEFINER` membership helpers avoid policy
  recursion. (003 TRD §3.)
- **Superadmin is administrative, not content-privileged** — cannot read private
  chats or agent secrets.
- **Secrets are agent-scoped**, owner/maintainer-managed (not per-end-user).
- **Agent creation is permissive** (any allowlisted user → owner); **presentation
  is curated** (superadmin `featured` flag) — both coexist.
- **Web UI is embedded** in the omniagent binary via `go:embed`; Caddy does TLS only.

See [ENGINEERING.md](../../../ENGINEERING.md) for build/test/tooling gotchas.
