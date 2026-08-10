# Team Mode

**Team mode** turns a single-operator OmniAgent into a multi-user deployment:
real user accounts, allowlist-closed magic-link sign-in, private and group
chats, and **virtual agents** that people discover in a catalog and chat with.
It is disabled by default — existing single-operator deployments are unaffected.

!!! note "Two different web-UI auth systems"
    Team mode uses **magic-link email** sign-in (this guide), served by
    `omniagent gateway run` when `team.enabled` is set. That is separate from the
    OAuth SSO login for the OpenAI-compatible server described in
    [Web UI Authentication](authentication.md) (`omniagent openai serve
    --web-ui`). Don't mix the two.

## Personal mode vs. team mode

| | Personal mode (default) | Team mode |
|---|---|---|
| Accounts | one implicit user, no login | many users, magic-link sign-in |
| Isolation | n/a | PostgreSQL row-level security (RLS) |
| Chats | one private chat with the agent | private DMs + group chats |
| Agents | one deployment agent | many virtual agents + a catalog |
| Store | SQLite | PostgreSQL (SQLite works for local trials) |

## Enabling team mode

Team mode is configured under the `team:` block and served by the gateway:

```yaml
# omniagent.yaml
team:
  enabled: true
  superadmin_email: you@example.com      # bootstrapped as superadmin on first login
  base_url: https://team.example.com     # external origin for magic links + cookies
  agent_handle: omniagent                # @-mention handle in group chats (default)
  database:
    app_dsn: postgres://omniagent_app:pw@db:5432/omniagent_team
    migrate_dsn: postgres://owner:pw@db:5432/omniagent_team
    app_role: omniagent_app
  smtp:
    host: smtp.example.com
    port: 587
    from: agent@example.com
    username: apikey
    # password: set via OMNIAGENT_TEAM_SMTP_PASSWORD (see below) — YAML
    # values are not shell-expanded, so a literal ${VAR} here is not
    # substituted.
  secrets:
    provider: file                       # optional per-agent secret vault
    dir: /var/lib/omniagent/secrets

# The web UI is implied by team mode; an LLM key lets agents actually reply.
web:
  enabled: true
agent:
  provider: anthropic
  model: claude-sonnet-5
  # api_key: set via ANTHROPIC_API_KEY (loadEnv falls back to the
  # provider-specific env var when api_key is left unset here)
```

Every `team.*` field can also be set via an `OMNIAGENT_TEAM_*` environment
variable instead of the file — the env value always wins:

| Variable | Config field |
|----------|--------------|
| `OMNIAGENT_TEAM_ENABLED` | `team.enabled` |
| `OMNIAGENT_TEAM_DATABASE_APP_DSN` | `team.database.app_dsn` |
| `OMNIAGENT_TEAM_DATABASE_MIGRATE_DSN` | `team.database.migrate_dsn` |
| `OMNIAGENT_TEAM_DATABASE_APP_ROLE` | `team.database.app_role` |
| `OMNIAGENT_TEAM_BASE_URL` | `team.base_url` |
| `OMNIAGENT_TEAM_SUPERADMIN_EMAIL` | `team.superadmin_email` |
| `OMNIAGENT_TEAM_AGENT_HANDLE` | `team.agent_handle` |
| `OMNIAGENT_TEAM_SMTP_HOST` | `team.smtp.host` |
| `OMNIAGENT_TEAM_SMTP_PORT` | `team.smtp.port` |
| `OMNIAGENT_TEAM_SMTP_USERNAME` | `team.smtp.username` |
| `OMNIAGENT_TEAM_SMTP_PASSWORD` | `team.smtp.password` |
| `OMNIAGENT_TEAM_SMTP_FROM` | `team.smtp.from` |

Start it:

```bash
omniagent gateway run --config omniagent.yaml
```

On start the gateway runs the database migrations (schema **and** RLS policies),
then serves the SPA at `/` and the team API under `/api/`.

### Database: PostgreSQL vs. SQLite

The store auto-detects the dialect from `app_dsn`: a `postgres://` (or
`postgresql://`) URL uses PostgreSQL; anything else (a file path, `sqlite://…`,
`:memory:`) uses SQLite.

- **PostgreSQL** is the production target. It applies **row-level security** as a
  defense-in-depth backstop under a two-role setup: `migrate_dsn` (owner, used
  only for migrations-on-start) and `app_dsn` (the non-owner application role,
  `app_role`, that requests run as).
- **SQLite** has no RLS, so it is meant for local trials and tests only. The
  service layer is the primary authorization gate and stands on its own there,
  but for true multi-tenant isolation use PostgreSQL.

## Signing in

Sign-in is **allowlist-closed magic links**:

1. A user enters their email on the web UI; if the address is allowed, a
   one-time sign-in link is emailed.
2. Clicking the link sets a secure, HttpOnly session cookie and drops them into
   the app.

The account named by `superadmin_email` is bootstrapped as the **superadmin** on
its first successful sign-in. Everyone else must be on the allowlist.

!!! tip "Local dev without SMTP"
    If no `smtp` block is configured, magic links are **logged to the gateway
    console** instead of emailed. Request a link, copy the
    `/api/auth/verify?token=…` URL from the logs, and open it. Handy for local
    trials; configure SMTP for any real deployment.

### Managing the allowlist and members

Only allowlisted emails (plus the superadmin) can sign in. Superadmins see an
**Admin** tab in the web UI (visible only to the superadmin) covering:

- **Allowlist** — add or remove approved emails, with an optional note.
- **Members** — every user's role and status, with a Disable/Enable toggle.
  Disabling ends a member's access immediately; their data stays scoped and
  inaccessible to others, and it is a disable, not a delete. A superadmin
  cannot disable their own account (lockout guard).

The same operations are available over the admin API, useful for scripting:

```bash
# Allowlist (superadmin session cookie + CSRF header required on mutations)
GET    /api/admin/allowlist
POST   /api/admin/allowlist        {"email":"teammate@example.com","note":"Design"}
DELETE /api/admin/allowlist?email=teammate@example.com

# Members
GET    /api/admin/users
PATCH  /api/admin/users/{id}       {"status":"disabled"}   # or {"username":"..."}
```

## Chats

Signed-in users get a two-pane chat surface under the **Chats** tab:

- **Private DMs** — a one-to-one conversation with an agent. The agent always
  replies.
- **Group chats** — invite teammates by username. The bound agent replies only
  when a message **@-mentions** its handle (e.g. `@omniagent`), so it does not
  interject on every message.

Each chat's memory is scoped to that chat, so context never bleeds across
conversations.

## Virtual agents

The headline of team mode is **virtual agents**: named personas, each bound to a
chosen subset of the deployment's skills, with their own owner/maintainer roles
and a private/listed + featured registry. Creating, configuring, discovering,
and chatting with agents is covered in its own guide:

➡️ **[Virtual Agents](agents.md)**

The available-skills catalog an agent may enable is drawn from `skills.includes`
(see [Configuration](../reference/configuration.md)).

## Agent-scoped secrets

Agents often need credentials — an API token for an MCP server, say. Team mode
can inject **per-agent** secrets into an agent's skills (including the
environment of its MCP subprocesses), configured under `team.secrets`:

```yaml
team:
  secrets:
    provider: file       # "memory" (tests) or "file" (local store)
    dir: /var/lib/omniagent/secrets
```

Secrets are stored in an [OmniVault](vault-credentials.md)-backed store and
**namespaced per agent** as `agents/<agentID>/<ENV_VAR>`. Because each agent is
handed a namespace-scoped view, two agents load disjoint secrets with no
cross-leak, and each agent's runtime instance is built with only its own secrets.

!!! warning "Current limitations"
    - The `memory` and `file` providers are **not encrypted at rest** — the
      isolation mechanism ships first; an encrypted store is planned. Protect the
      `dir` with filesystem permissions in the meantime.
    - Secret **values** are intentionally kept off the web UI. They are
      provisioned into the vault out-of-band today (e.g. the `file` provider
      keeps them under `dir`, keyed by the `agents/<agentID>/<ENV_VAR>` path); a
      management surface is planned.

## Deployment notes

- **HTTPS.** When `base_url` is `https://`, session cookies are set `Secure` with
  the `__Host-` prefix. Plain-HTTP localhost drops the prefix for dev.
- **CSRF.** State-changing API calls require the `X-OmniAgent-CSRF` header;
  combined with `SameSite=Lax` cookies this blocks cross-site submissions.
- **Two-role database.** Keep `migrate_dsn` (owner) separate from `app_dsn`
  (application role) so day-to-day requests never run with owner privileges — RLS
  only constrains non-owner roles.
- **Agent replies need an LLM key.** Without `agent.api_key`, the management UI,
  catalog, and chats all work, but agent-bound chats stay silent (a wrong or
  failed reply is worse than none).
