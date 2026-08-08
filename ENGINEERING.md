# Engineering Notes

Build/test/tooling knowledge for OmniAgent, with an emphasis on the multi-user
"team" work (`INIT-OMNIAGENT-003/004/005`). These are the things that are easy to
get wrong and hard to rediscover. For the initiative map and current status see
[`docs/specs/initiatives/README.md`](docs/specs/initiatives/README.md).

## Team database (PostgreSQL + RLS)

### Two-role model (do not collapse to one role)

The team store uses **two PostgreSQL roles**, and this is load-bearing:

- **Owner role** (`MigrateDSN`) runs migrations and owns the tables. Owners
  bypass RLS — which the membership helpers rely on.
- **Application role** (`AppDSN`, non-owner, default `omniagent_app`) is what the
  running server connects as. Because it is a non-owner, plain
  `ENABLE ROW LEVEL SECURITY` binds every policy for it.

We deliberately use `ENABLE`, **not** `FORCE`, row level security. `FORCE` would
apply policies to the owner too and break the `SECURITY DEFINER` helper functions
(`team_is_chat_member/owner/creator`, and `team_is_agent_editor` in 005) that let
policies consult membership tables **without policy recursion**. Those helpers are
owner-owned and ride the owner's bypass by design. (See `INIT-OMNIAGENT-003`
TRD §3.)

### Per-request scoping — the only two entry points

All application queries go through `team/store`:

- `AsUser(ctx, userID, superadmin, fn)` and `AsSystem(ctx, fn)` open a
  transaction and set `SET LOCAL`-style GUCs (`app.current_user_id`,
  `app.is_superadmin`, `app.is_system`) via `set_config(..., true)`.
- The raw `*sql.DB` is **unexported** so no code path can bypass scoping. Keep it
  that way.
- `app.is_system` is the auth-layer/agent context: the only one that may touch
  `magic_link_tokens`, create users/identities/sessions, or author agent
  messages. Do not use it for user requests.

### RLS + Ent landmine (this will bite you)

**`ent.UpdateOneID(...).Save(ctx)` can return `nil` on an RLS-filtered
zero-row UPDATE.** Ent's `UpdateOne` reloads the entity via a `SELECT` after the
update; if the caller may `SELECT` the row but the `UPDATE` policy filtered it to
zero rows, ent returns the reloaded entity with no error rather than
`ent.ErrNotFound`.

Consequences:

- **Never infer authorization from ent update/delete errors.** A denied write may
  look like success at the ent layer.
- In tests, assert the **durable state** (re-read the row and check the field is
  unchanged / the row still exists), not ent's error mapping. See
  `team/store/rls_test.go` (the "messages are immutable" subtest) for the pattern.
- This is why messages have no `UPDATE`/`DELETE` policy at all (immutability by
  absence of policy), and why the service layer performs its own authorization
  checks in addition to RLS.

### Regenerating Ent code

Schemas live in `team/ent/schema/*.go`; generated code is committed under
`team/ent/`. After changing a schema:

```bash
GOFLAGS=-mod=mod go generate ./team/ent/
```

The `sql/execquery` feature is enabled (see `team/ent/generate.go`) so the store
can run `set_config` on `*ent.Tx`.

### RLS/crypto tests need a real Postgres

There is no in-memory substitute — `citext`, RLS, and `SECURITY DEFINER` are
Postgres-specific. Tests self-skip unless both DSNs are set:

```bash
docker compose -f deploy/team/dev/docker-compose.dev.yaml up -d --wait
export TEAM_TEST_OWNER_DSN="postgres://omniagent_owner:owner_dev_password@127.0.0.1:5433/omniagent_team?sslmode=disable"
export TEAM_TEST_APP_DSN="postgres://omniagent_app:app_dev_password@127.0.0.1:5433/omniagent_team?sslmode=disable"
go test ./team/... ./gateway/
```

`internal/pgtest.DSNs(t)` provisions an **isolated database per test suite**
(created/dropped per run) so packages parallelize without clobbering each other's
schema. Use it for any new Postgres-backed suite; do not `DROP SCHEMA` a shared DB.

**Gotcha:** the dev compose runs its role-creation init script only on an empty
data volume. If auth fails after changing credentials, recreate cleanly:
`docker compose -f deploy/team/dev/docker-compose.dev.yaml down -v && … up`.

## vistudio (PRISM) tracking

- **The live database is `~/.productbuildershq/prism/prismcontrol`**, not the
  `vistudio`/`prismctl` documented default `~/.productbuildershq/visionstudio`
  (that copy is stale — its newest data predates this work). If a query returns
  no rows for an initiative you know exists, you are pointed at the wrong store.
- Spec files must live at `docs/specs/initiatives/INIT-OMNIAGENT-00N/` for
  `vistudio spec validate` / the UI to find them. Older initiatives (001/002) use
  legacy paths (`docs/specs/…`) that `spec sync` flags for migration.
- **Handoff notes are the source of truth for uncommitted work.** RMIs left
  `in_progress` with a `state: implemented-uncommitted` handoff mean the code is
  in the working tree, not git. Read `vistudio work status` and each RMI's
  handoff before assuming something is unbuilt. Flip an RMI to `completed` only
  after its commit lands (and ideally after `vistudio ingest git`).

## Go module / dependency workflow

- Verify the latest version before adding a dependency (org convention). Because
  the repo builds with `-mod=readonly`, run version queries with module mode
  enabled: `GOFLAGS=-mod=mod go list -m -versions <module>`.
- Key team-mode deps (verified at add time): `entgo.io/ent v0.14.6`,
  `github.com/jackc/pgx/v5 v5.10.0`, `github.com/wneessen/go-mail v0.8.1`.
- `op://` and `bw://` vault providers require cgo and are absent on
  `!cgo`/Windows builds (only `env`/`file`/`memory` remain). `keeper://` is listed
  in `config.vaultSchemes` but has **no registered provider** — wire `omni-keeper`
  or drop it (tracked in `INIT-OMNIAGENT-004` RMI-201).

## Secrets (agent-scoped) — implementation notes

- Secrets belong to an **agent** and are managed by its owner/maintainers
  (`INIT-OMNIAGENT-004`, rescoped; `INIT-OMNIAGENT-005` owns the agent entity).
- Per-user secret injection is **not** v1. Because secrets are agent-scoped and
  runtime instances are per-agent, per-agent MCP subprocesses can carry the
  agent's secrets in their env — the earlier "per-user MCP env is fixed at spawn"
  problem dissolves.
- At-rest encryption uses an AES-256-GCM KEK resolved from omnivault
  (`team.secrets.encryption_key`), **never stored in the DB**. Losing the KEK
  means re-entering secrets — document this in any deploy runbook.
- Values are **write-only** across the API (set/replace/delete/list-names, never
  read-back) and must never enter the model prompt, transcript, or logs
  (register them with the redactor).

## Gateway integration points

- The gateway mux is built in `gateway.Run`. Embedders add routes via
  `gw.Handle(pattern, handler)` and gate WebSocket upgrades with
  `gw.SetConnectAuthorizer(...)` (team mode authenticates the session cookie
  before upgrade). Team wiring is assembled in
  `cmd/omniagent/commands/team.go::setupTeamMode`.
- Team mode is off by default (`team.enabled=false`); single-operator behavior
  must remain unbroken when it is off.
- CSRF: state-changing team endpoints require the `X-OmniAgent-CSRF` header in
  addition to the `SameSite=Lax` cookie. The magic-link **request** endpoint is
  unauthenticated (no CSRF), rate-limited by the escalating-delay limiter.

## Commit conventions (reminder)

- Conventional commits; trailer `Refs: RMI-OMNIAGENT-<NNN>` on each RMI's commit.
- Break a session's work into topical commits (feat/fix/test/docs), each building
  and passing tests. Do not use a single "initial"/"wip" commit.
- Pre-push: `go test ./...`, `golangci-lint run`, no local `replace` directives,
  no references to untracked files.
