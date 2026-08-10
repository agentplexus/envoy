# Team Deployment

Run a self-hosted, production team-mode stack on a single VM: Caddy
(automatic HTTPS) → omniagent → PostgreSQL, via Docker Compose. This is the
single-host model — no Kubernetes, no multi-region. For the config reference
and local-trial setup, see the [Team Mode guide](team-mode.md); for
deploying a single-operator (personal-mode) container via OmniDeploy instead,
see [Deployment](deployment.md).

All the compose assets referenced below live in
[`deploy/team/prod/`](https://github.com/plexusone/omniagent/tree/main/deploy/team/prod)
in the repository.

## Provisioning a Lightsail instance

1. Create an AWS Lightsail **VM instance** (not a container service) — the
   smallest plan with 2GB+ RAM is enough for a small team. Choose Ubuntu.
2. Attach a static IP and open ports 80/443 in the instance firewall (Caddy
   needs both for HTTP→HTTPS redirect and ACME HTTP-01 validation).
3. Point your domain's DNS `A` record at the static IP.
4. SSH in and install Docker + the Compose plugin:

   ```bash
   curl -fsSL https://get.docker.com | sh
   sudo usermod -aG docker $USER   # log out/in to pick this up
   ```

## Deploying the stack

```bash
git clone https://github.com/plexusone/omniagent.git
cd omniagent/deploy/team/prod
cp .env.example .env
$EDITOR .env   # fill in POSTGRES_PASSWORD, APP_DB_PASSWORD, DOMAIN,
               # SUPERADMIN_EMAIL, SMTP_*, and an LLM provider key
docker compose up -d --build
```

This builds the `omniagent` image from the repository root, starts
PostgreSQL (not published to the host — only Caddy exposes 80/443), and
Caddy requests a Let's Encrypt certificate for `DOMAIN` automatically. Named
volumes (`pgdata`, `caddy_data`, `caddy_config`) persist database contents
and the certificate cache across `docker compose down`/restart.

## Smoke checklist

Run through this after first deploy (and after any upgrade):

1. **Health check** — `curl -s https://$DOMAIN/api/health` returns `200`.
2. **Magic-link login** — request a link for `SUPERADMIN_EMAIL` at
   `https://$DOMAIN/`. If SMTP is configured, check your inbox; if not,
   `docker compose logs omniagent | grep verify` and open the logged
   `/api/auth/verify?token=…` URL manually (see
   [`deploy/team/dev/TESTING.md`](https://github.com/plexusone/omniagent/blob/main/deploy/team/dev/TESTING.md)
   for the same flow against the local dev stack).
3. **Superadmin confirmed** — after verifying, the web UI shows the **Admin**
   tab and `/api/auth/me` reports `"superadmin":true`.
4. **Allowlist a second member** — from the Admin tab, add a teammate's
   email, then have them sign in and confirm they land in chat with no
   Admin tab.
5. **Agent reply** — if an LLM key is set, message the agent in a private
   chat and confirm a reply arrives.
6. **SSO sign-in** (if `GOOGLE_CLIENT_ID`/`GITHUB_CLIENT_ID` are set) — click
   the Google/GitHub button on the login screen, complete the provider's
   consent screen, and confirm you land back in the app signed in. Sign in
   again with the *same* provider account using a second browser/incognito
   window and confirm no duplicate account is created. If an existing
   magic-link user signs in via SSO with the same email, confirm the Admin
   tab's Members card shows both `magic_link` and the SSO provider as
   badges on one row (not two separate users).

## Operations

### Upgrading

```bash
cd omniagent && git pull
cd deploy/team/prod
docker compose up -d --build omniagent
```

Schema migrations run automatically and idempotently on every `omniagent`
start (advisory-locked, so it's safe even if a deploy briefly overlaps
itself) — no separate migration step is needed. Postgres and Caddy are
unaffected by an omniagent-only rebuild.

### Rolling back

Check out the previous commit/tag and rebuild:

```bash
git checkout <previous-tag>
docker compose up -d --build omniagent
```

Rolling back across a schema migration that isn't backward-compatible is a
manual judgment call — check the target version's changelog for breaking
migrations before rolling back a database that's already been migrated
forward.

### Environment variable matrix

Every `team.*` config field has an `OMNIAGENT_TEAM_*` environment variable
equivalent, used by the compose file's `environment:` block — see the full
table in [Environment Variables → Team Mode](../reference/environment.md#team-mode).

### Backups

```bash
./backup.sh                       # writes deploy/team/prod/backups/omniagent-team-<timestamp>.sql.gz
./restore.sh backups/<file> --force   # OVERWRITES the current database
```

Run these from `deploy/team/prod/` with the stack up. Automate `backup.sh`
on a cron schedule and ship the output off-host. To verify a backup restores
cleanly without touching production, restore it into a scratch database:

```bash
docker compose exec postgres psql -U omniagent_owner -c \
  "CREATE DATABASE restore_check OWNER omniagent_owner;"
gunzip -c backups/<file> | docker compose exec -T postgres psql -U omniagent_owner -d restore_check
docker compose exec postgres psql -U omniagent_owner -d restore_check -c "SELECT count(*) FROM users;"
docker compose exec postgres psql -U omniagent_owner -d omniagent_team -c "DROP DATABASE restore_check;"
```

The same procedure runs automatically (against a disposable CI database, not
production) via the
[`team-backup-restore`](https://github.com/plexusone/omniagent/actions/workflows/team-backup-restore.yaml)
GitHub Actions workflow — trigger it manually with `workflow_dispatch` to
verify the restore path still works after a schema change.

### Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| `omniagent` container exits immediately | Bad or missing `POSTGRES_PASSWORD`/`APP_DB_PASSWORD`/`SUPERADMIN_EMAIL`/`DOMAIN` — these are required (`:?`-guarded) in the compose file's `environment:` block; check `docker compose logs omniagent`. |
| Magic links never arrive | SMTP misconfigured. Team mode does not fail startup on bad SMTP — it falls back to **logging** the link instead of emailing it. Check `docker compose logs omniagent` for the link, then fix `SMTP_*` in `.env`. |
| Caddy never gets a certificate | Port 80/443 not reachable from the internet (firewall/security group), or DNS hasn't propagated yet to the instance's IP. `docker compose logs caddy` shows the ACME error. |
| `docker compose up` fails on the `postgres` service | `APP_DB_PASSWORD` unset when the database first initialized — the app role is only created via `init/01-app-role.sh` on a **fresh** data volume. If you changed `APP_DB_PASSWORD` after first boot, update the role's password directly (`ALTER ROLE omniagent_app PASSWORD '...'`) rather than expecting the init script to re-run. |
| `omniagent` container exits immediately with Google SSO configured | OIDC discovery against `accounts.google.com` failed at startup (no outbound internet, DNS/firewall block). This is fatal by design — check `docker compose logs omniagent` for the discovery error; GitHub SSO makes no such call and is unaffected. |
| SSO redirects back with `?error=sso_state` or `?error=sso_failed` | The provider's registered redirect URI doesn't exactly match `https://$DOMAIN/api/auth/{google,github}/callback`, or the client ID/secret is wrong — re-check the provider's app settings against `.env`. |
| SSO redirects back with `?error=not_allowed` | The signed-in account's verified email isn't on the allowlist — add it from the Admin tab, then retry. |
