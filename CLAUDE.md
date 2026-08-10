# CLAUDE.md — omniagent

Project-specific guidance for Claude Code. This complements the global
(`~/.claude/CLAUDE.md`) and plexusone org (`~/go/src/github.com/plexusone/.github/CLAUDE.md`)
instructions — follow those too (Ent, Cobra, official MCP Go SDK, library-first,
specs-before-implementation, `Refs: RMI-…` commit trailers, verify dependency
versions).

## What this is

OmniAgent is an AI agent that routes messages across channels (Telegram,
Discord, WhatsApp, Twilio, …), an OpenAI-compatible API, a WebSocket gateway,
and an embedded web UI. It runs in two shapes:

- **Personal mode** (default): one implicit user, SQLite, a single agent.
- **Team mode** (`team.enabled`): many users, PostgreSQL with row-level
  security, magic-link auth, private/group chats, and **virtual agents** with a
  catalog. See `docs/guides/team-mode.md` and `docs/guides/agents.md`.

## Package map

| Path | Responsibility |
|------|----------------|
| `agent/` | Core agent: config, options, tools, compiled-skill registration, session rollover. `agent.New` builds one instance. |
| `gateway/` | WebSocket gateway + all HTTP handlers. `team_http.go` (auth/admin + `RequireAuth`/CSRF middleware), `team_chat_http.go` (chats), `agents_http.go` (virtual-agents API), `web_http.go` (SPA + capabilities). |
| `team/` | Multi-user domain. `auth/` (magic-link + Principal), `agents/` (virtual-agents service: CRUD, roles, `Can` authz matrix, registry, catalog), `chats/` (DMs/groups, agent turns, mention policy, memory scope), `secrets/` (ScopedVault multi-tenancy + per-agent secret resolution over OmniVault), `agentruntime/` (lazy bounded per-agent runtime cache + builder), `store/` (Ent + RLS, dialect-aware), `ent/` (generated), `mail/`. |
| `skills/` | `compiled/` (Go skill interface + `StorageAware`/`AgentAware`/`SecretsAware` injection), `remote/mcp/` (MCP subprocess skill), `web/`, `memory/`, `github/`. |
| `config/` | Config structs + validation. `team.go` (team + secrets), `capabilities.go` (web-UI capability flags). |
| `web/dist/` | Embedded vanilla-JS SPA (`app.js`, `style.css`) — no build step, CSP-clean (no external assets). |
| `cmd/omniagent/commands/` | Cobra CLI + composition root. `gateway.go` mounts handlers; `team.go` (`setupTeamMode`) wires the team services. |
| `docs/specs/initiatives/` | Initiative specs + ROADMAPs (`INIT-OMNIAGENT-00N`). |

## Team-mode architecture (important patterns)

- **Two-layer authorization.** The service layer is the *primary* gate
  (`agents.Service.Can` / `requireEditor` / `requireOwner`, `chats` membership
  checks); PostgreSQL **RLS is a defense-in-depth backstop**. On the SQLite path
  there are no policies, so service-layer checks must stand alone — write tests
  accordingly (a SQLite test cannot assert RLS read-visibility; assert the
  capability gate and mutations instead).
- **System vs. user context.** Authorization *facts* are read via
  `store.AsSystem` (not filtered by RLS) so a decision doesn't depend on
  visibility; data operations run via `store.AsUser(userID, superadmin, …)`.
- **HTTP handler shape.** Each surface is an `XxxHTTP` struct owning its own
  `*http.ServeMux` built in `routes()` (Go 1.22 method+wildcard patterns),
  exposing `Handler()`. Mounted from `cmd/.../gateway.go` via `gw.Handle(...)`,
  wrapped in `teamHTTP.RequireAuth`. Mutations require the `X-OmniAgent-CSRF`
  header (`actorCSRF` helper); read the principal with `principalFrom(ctx)`.
  ServeMux longest-pattern-wins is relied on (exact `/api/capabilities` beats the
  `/api/` subtree).
- **Per-agent runtime.** `agentruntime.Cache` builds an agent's instance lazily
  on first turn (persona + enabled skills + agent-scoped secrets) and holds it in
  a bounded LRU. It depends only on seams (`ConfigLoader`, `Builder`,
  `SecretSource`); adapters live at the composition root so packages stay
  decoupled.
- **Agent-scoped secrets.** `team/secrets.ScopedVault` namespaces an OmniVault
  store per agent (`agents/<id>/…`); isolation is structural (a scoped caller
  cannot address another namespace). Injection flows through
  `compiled.SecretsAware`/`agent.WithSecretEnv`. `memory`/`file` providers are
  not encrypted at rest yet.

## Build / test / lint

```bash
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
go build ./...                 # package-scoped is faster on a cold cache
go test ./...                  # SQLite-backed team tests need no external DB
golangci-lint run
node --check web/dist/app.js   # the SPA has no build step; syntax-check it
```

- `go build ./...` can be slow cold — prefer package-scoped builds while iterating.
- gopls "No active builds contain …" diagnostics on new/edited files are LSP
  workspace noise; verify with real `go build`/`go test`/`golangci-lint`.

## Conventions specific to this repo

- **Never read secret files** (`.env*`, `credentials.json`, `*.pem`, `*.key`) —
  load them in Bash via `source`/`export`; check existence with `ls`/`test -f`.
- **Commit only when asked or when implementing an authorized RMI**; **never push
  unless explicitly asked.** Atomic topical commits (core → tests → docs) with
  `Refs: RMI-OMNIAGENT-<NNN>` trailers.
- Keep the SPA self-contained: inline all CSS/JS, no external/CDN assets (guarded
  by `web/embed_test.go`); set user text via `textContent`, never `innerHTML`.
- After major work, keep VisionStudio RMI/phase/initiative status in sync.
