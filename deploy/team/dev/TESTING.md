# Testing team mode locally (magic-link auth)

Phase 2 of `INIT-OMNIAGENT-003`. This walks through exercising the full
passwordless login flow on your machine, with no SMTP and no TLS.

## 1. Start PostgreSQL

```bash
docker compose -f deploy/team/dev/docker-compose.dev.yaml up -d --wait
```

This starts Postgres 16 on `127.0.0.1:5433` with two roles — `omniagent_owner`
(migrations) and `omniagent_app` (the app's RLS-scoped role).

## 2. Run the gateway in team mode

```bash
go run ./cmd/omniagent gateway run --config deploy/team/dev/config.dev.yaml
```

On startup you should see:

```
team mode enabled: database migrated (schema + RLS) superadmin_email=root@example.com cookie_secure=false
team mode: no SMTP configured — magic links will be LOGGED, not emailed (dev only)
team mode: auth API mounted at /api/, WebSocket cookie-authenticated
```

The database is migrated automatically (schema + row-level-security policies).

## 3. Log in as the superadmin

Request a link for the configured superadmin email:

```bash
curl -si http://127.0.0.1:8080/api/auth/magic-link \
  -H 'Content-Type: application/json' \
  -d '{"email":"root@example.com"}'
# → 200 {"status":"ok","message":"If that email is permitted, a sign-in link has been sent."}
```

Because no SMTP is configured, the link is printed in the **server log**:

```
email (log mailer — not actually sent) to=root@example.com subject="Sign in to OmniAgent"
  body="Sign in to OmniAgent ... http://127.0.0.1:8080/api/auth/verify?token=XXXX ..."
```

Copy that verify URL and open it (or curl it, keeping the cookie):

```bash
curl -si -c /tmp/oa.cookies \
  'http://127.0.0.1:8080/api/auth/verify?token=PASTE_TOKEN_HERE'
# → 303 See Other, Location: http://127.0.0.1:8080/ , Set-Cookie: oa_session=...
```

The first login by `root@example.com` creates that user **as the superadmin**.

Confirm the session:

```bash
curl -s -b /tmp/oa.cookies http://127.0.0.1:8080/api/auth/me
# → {"user_id":"...","username":"root","email":"root@example.com","superadmin":true}
```

## 4. Allowlist a family member (superadmin only, CSRF required)

```bash
curl -si -b /tmp/oa.cookies http://127.0.0.1:8080/api/admin/allowlist \
  -H 'X-OmniAgent-CSRF: 1' -H 'Content-Type: application/json' \
  -d '{"email":"kid@example.com","note":"my kid"}'
# → 200 ; without the X-OmniAgent-CSRF header → 403

curl -s -b /tmp/oa.cookies http://127.0.0.1:8080/api/admin/allowlist
# → {"allowlist":[{"email":"kid@example.com","note":"my kid",...}]}
```

## 5. Log in as the member (separate cookie jar)

```bash
curl -s http://127.0.0.1:8080/api/auth/magic-link \
  -H 'Content-Type: application/json' -d '{"email":"kid@example.com"}'
# copy the token from the server log, then:
curl -si -c /tmp/kid.cookies \
  'http://127.0.0.1:8080/api/auth/verify?token=PASTE_KID_TOKEN'
curl -s -b /tmp/kid.cookies http://127.0.0.1:8080/api/auth/me
# → superadmin:false
```

## Things to verify

- **Closed signup**: a request for a non-allowlisted, non-superadmin email
  returns the same `200` but no link appears in the log (nothing is issued).
- **Single-use**: opening the same verify link twice — the second redirects to
  `/?error=invalid_link`.
- **Expiry**: links expire after 15 minutes.
- **CSRF**: `POST`/`DELETE` under `/api/admin/*`, `/api/auth/logout`, and
  `/api/users/me/*` require the `X-OmniAgent-CSRF: 1` header.
- **RBAC**: the member (`kid`) gets `403` on `/api/admin/allowlist`.
- **Rename (US-3)**: `curl -b … -H 'X-OmniAgent-CSRF: 1' -d '{"username":"captain"}'
  http://127.0.0.1:8080/api/users/me/username` renames the caller.
- **Logout**: `POST /api/auth/logout` (with CSRF) then `/api/auth/me` → `401`.

## Automated equivalent

The same flow is covered by `go test`:

```bash
export TEAM_TEST_OWNER_DSN="postgres://omniagent_owner:owner_dev_password@127.0.0.1:5433/omniagent_team?sslmode=disable"
export TEAM_TEST_APP_DSN="postgres://omniagent_app:app_dev_password@127.0.0.1:5433/omniagent_team?sslmode=disable"
go test ./team/... ./gateway/ -run 'TestRLS|TestBootstrap|TestMagicLink|TestTeamHTTP' -v
```

## Teardown

```bash
docker compose -f deploy/team/dev/docker-compose.dev.yaml down -v
```
