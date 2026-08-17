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
    trials; configure SMTP for any real deployment. Or use **password login**
    (below), which needs no SMTP at all.

### Password login (email + password)

Email + password sign-in is an **additive** third method alongside magic-link
and SSO — the login screen offers all configured options. It's allowlist-safe
by construction: a password only exists on an already-created (therefore
allowlisted) user, and login also requires an active account. Wrong password
and unknown email return the same uniform error (no account enumeration), and
the endpoint is rate-limited like magic-link. Passwords are stored argon2id-
hashed (never in plaintext or logs).

A password can be set three ways:

- **Startup bootstrap (no SMTP needed).** Set the superadmin's password from
  config/env so you can log in immediately on a fresh deployment:

  ```bash
  OMNIAGENT_TEAM_SUPERADMIN_PASSWORD='a-strong-passphrase' \
    omniagent gateway run --config omniagent.yaml
  ```

  or `team.superadmin_password` in the config file (prefer an env/vault
  reference over a literal). It's **set-once**: applied only if the superadmin
  has no password yet, so it never clobbers a password later changed in the UI.
- **Superadmin sets any user's** password from the **Admin → Members** panel
  (or `PATCH /api/admin/users/{id}` with `{"password":"…"}`).
- **A user changes their own** in the **Account** tab (or
  `POST /api/users/me/password`); changing an existing password requires the
  current one.

Minimum length is 8 characters.

!!! note "Config format"
    The config file may be **JSON or YAML** (the loader picks by extension),
    and every `team.*` field also has an `OMNIAGENT_TEAM_*` env var that
    overrides it — so a deployment can be driven entirely by environment
    variables (as the Compose stack does). Don't commit literal passwords;
    source them from env vars or a vault reference.

### Managing the allowlist and members

Only allowlisted emails (plus the superadmin) can sign in. Superadmins see an
**Admin** tab in the web UI (visible only to the superadmin) covering:

- **Allowlist** — add or remove approved emails, with an optional note.
- **Members** — every user's role and status, with a Disable/Enable toggle.
  Disabling ends a member's access immediately; their data stays scoped and
  inaccessible to others, and it is a disable, not a delete. A superadmin
  cannot disable their own account (lockout guard).
- **Global Secret Bindings** — a read-only list of the deployment-wide
  `secrets:`/`skills.config.<name>.secrets` bindings from the config file
  (see [Secrets](../reference/configuration.md#secrets)), showing each
  binding's name, source (`global` or a skill name), and whether it's set —
  never the value. These are static for the process's lifetime; change them
  by editing the config, not through the UI. This is the deployment-wide
  counterpart to the per-agent Secrets panel on each agent's own page.

The same operations are available over the admin API, useful for scripting:

```bash
# Allowlist (superadmin session cookie + CSRF header required on mutations)
GET    /api/admin/allowlist
POST   /api/admin/allowlist        {"email":"teammate@example.com","note":"Design"}
DELETE /api/admin/allowlist?email=teammate@example.com

# Members
GET    /api/admin/users
PATCH  /api/admin/users/{id}       {"status":"disabled"}   # or {"username":"..."}

# Global secret bindings (read-only)
GET    /api/admin/secret-bindings
```

### SSO (Google, GitHub)

Magic-link email is always available; Google OIDC and GitHub OAuth are
optional, additive sign-in methods configured under `team.sso`:

```yaml
team:
  sso:
    google:
      client_id: "1234567890-abc.apps.googleusercontent.com"
      client_secret: "GOCSPX-..."
    github:
      client_id: "Iv1.abc123"
      client_secret: "..."
```

Each provider is independent — configure one, both, or neither. A provider
needs both `client_id` and `client_secret` set to be enabled; the web UI's
login screen shows a "Sign in with Google/GitHub" button only for providers
the server has configured (`GET /api/capabilities`'s `googleSso`/`githubSso`
flags).

**Redirect URI is fixed, not configurable** — `{base_url}/api/auth/{google,github}/callback`.
Register exactly that URL with each provider:

- **Google**: [Google Cloud Console](https://console.cloud.google.com/apis/credentials) →
  Create OAuth client ID → Application type **Web application** → Authorized
  redirect URI `{base_url}/api/auth/google/callback`. The OAuth consent
  screen must be configured first (internal or external, per your org).
- **GitHub**: your account or org's
  [Developer settings → OAuth Apps](https://github.com/settings/developers) →
  New OAuth App (not a GitHub App) → Authorization callback URL
  `{base_url}/api/auth/github/callback`.

**How SSO resolves an account** — SSO never bypasses the allowlist. On first
sign-in with a given provider, the provider's *verified* email is checked
against the allowlist exactly like a magic-link request; a non-allowlisted
email is rejected and no account is created. If the email matches an
existing user (e.g. someone who has only ever used magic-link so far), the
SSO identity links to that same account — it does not create a duplicate. A
user may hold multiple linked identities (magic link, Google, GitHub all at
once); the Admin tab's Members card shows each user's linked providers as
small badges.

Set the client credentials via `team.sso.*` in the config file, or the
equivalent `OMNIAGENT_TEAM_SSO_*` environment variables (see
[Environment Variables](../reference/environment.md#team-mode)) — handy for
the Docker Compose deployment in
[Team Deployment](team-deployment.md).

!!! warning "Google discovery happens at startup"
    Configuring Google SSO makes `gateway run` perform a real OIDC discovery
    call to `accounts.google.com` at boot. A network failure there is
    **fatal** — the process exits rather than silently starting without
    working Google sign-in. GitHub's plain OAuth2 flow makes no such call
    and never fails at boot.

## Chats

Signed-in users get a two-pane chat surface under the **Chats** tab:

- **Private DMs** — a one-to-one conversation with an agent. The agent always
  replies.
- **Group chats** — invite teammates by username. The bound agent replies only
  when a message **@-mentions** its handle (e.g. `@omniagent`), so it does not
  interject on every message.

Each chat's memory is scoped to that chat, so context never bleeds across
conversations.

### Speech-to-text and translate

The composer has two optional icons next to the send button:

- **Speech-to-text** (mic icon) dictates into the message box using the
  browser's native `SpeechRecognition` API — entirely client-side, no
  request ever leaves the browser.
- **Translate** (globe icon) posts the composer's current text to
  `POST /api/translate` and replaces it with the translation, via a
  language popover (Spanish, French, German, Chinese, Japanese, Korean,
  Portuguese, Italian). This is a one-shot, non-persisting LLM call — it
  never touches chat history or memory.

Both icons only render when supported: the mic icon only if the browser
exposes `SpeechRecognition`/`webkitSpeechRecognition`; the translate icon
only if the `translate` capability (`GET /api/capabilities`) is true, which
requires a deployment-wide LLM (`agent.api_key` configured) — see
[Configuration](../reference/configuration.md).

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

Skills **declare** the secrets they need in their `SKILL.md`, and an agent's
**owner/maintainer** sets the values in the agent's **Secrets** panel (or over
the API) — see [Virtual Agents → Secrets](agents.md#secrets) for the full
GitHub-style workflow. Values are write-only: the UI/API report only whether a
secret is set, never its value.

!!! warning "Current limitations"
    - The `memory` and `file` providers are **not encrypted at rest** — the
      isolation mechanism ships first; an encrypted store is planned. Protect the
      `dir` with filesystem permissions in the meantime.
    - Injection currently covers **compiled** and **MCP** skills. OpenAPI-skill
      auth injection and a single-operator config-file binding path are planned
      follow-ons.

## Deployment notes

For a production Caddy + PostgreSQL stack on a single VM (e.g. Lightsail),
including a provisioning walkthrough, upgrade/backup procedures, and a
troubleshooting table, see **[Team Deployment](team-deployment.md)**.

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
