# OmniAgent Implementation Plan

Implementation plan accompanying [`ROADMAP.md`](ROADMAP.md). This is the canonical
plan and supersedes the stale root `PLAN.md` (whose OpenClaw-parity history is
retained under `docs/specs/parity-*`).

Phases and RMI IDs match the roadmap. Each phase is executed and reviewed as a
unit; phase status is derived from its RMIs. Commits carry `Refs: RMI-OMNIAGENT-<NNN>`.

Statuses below were verified against the codebase on 2026-07-28: **3 of 20 complete**
(`RMI-OMNIAGENT-001`, `-009`, `-019`). Registered in prism-control as
`INIT-OMNIAGENT-001`; `prismctl roadmap import` is the source of truth for status.

---

## Phase 1 — Cloud Deployment

**Derived status: In Progress**

Deploy target is **omniagent itself**, not grokify-omniagent: omniagent has no
local `replace` directives (only two, to published versions), so it builds in a
standard multi-stage Dockerfile with no sibling-repo mounting.

### RMI-OMNIAGENT-001 — Discord message chunking · **Done**

Shipped in omnichat `v0.8.1` and propagated.

| File | Change |
|------|--------|
| `omnichat/providers/discord/adapter.go` | `splitMessage` (rune-based, blank-line → newline → space break points); `Send` chunks; only first chunk carries `ReplyTo` |
| `omnichat/providers/discord/adapter_test.go` | New — split coverage (limit, rune-safety, paragraph preference, hard split) |
| `omniagent/go.mod` | `omnichat v0.8.0 → v0.8.1` (committed; verified linked into `cmd/omniagent` via `go list -deps`) |

### RMI-OMNIAGENT-002 — Standalone multi-stage Dockerfile · Planned

| File | Change |
|------|--------|
| `Dockerfile` | New. Stage 1 `golang:1.26`: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o omniagent ./cmd/omniagent`. Stage 2 `alpine:3.20` + `ca-certificates`, non-root user, `EXPOSE 8080`, `HEALTHCHECK` on `/health`, `ENTRYPOINT ["/opt/omniagent/omniagent","gateway","run"]` |
| `Dockerfile.example` | Remove or supersede once `Dockerfile` lands |

Verify: `docker build --platform linux/amd64` succeeds; container runs `gateway run`
and serves `/health` (the bare root command only prints help — `gateway run` is required).

### RMI-OMNIAGENT-003 — Publish image to GHCR · Planned

| File | Change |
|------|--------|
| `.github/workflows/docker.yaml` | New (optional). Build + push `ghcr.io/plexusone/omniagent` on tag; public package so Lightsail pulls without ECR `PrivateRegistryAccess` |

Manual path: `docker build … -t ghcr.io/plexusone/omniagent:smoke && docker push …`.

### RMI-OMNIAGENT-004 — Lightsail deploy via omnideploy · Planned

| File | Change |
|------|--------|
| `deploy/lightsail/deploy.yaml` | New. omnideploy schema: `container.image` → GHCR ref, `ports:[8080/HTTP]`, `health_check:/health`, `service.replicas:1`, `resources.size: micro` |
| — | `environment`: `OMNIAGENT_GATEWAY_ADDRESS=0.0.0.0:8080`, `OMNIAGENT_AGENT_PROVIDER`, `OMNIAGENT_AGENT_MODEL`; secrets (`OPENAI_API_KEY`/`ANTHROPIC_API_KEY`, `SERPER_API_KEY`, `DISCORD_BOT_TOKEN`) as plaintext env for the smoke test (hardened in Phase 2) |

Prerequisites (operator-supplied): AWS credentials, `pulumi` CLI, Pulumi backend
(`PULUMI_BACKEND_URL=file://$HOME/.pulumi` + `PULUMI_CONFIG_PASSPHRASE`). IAM: Lightsail
only (GHCR public → no ECR; file backend → no S3). Run: `omnideploy up --config … --target lightsail --backend pulumi`.

### RMI-OMNIAGENT-005 — Deployment smoke test · Planned

Validated locally already: Discord connects as the bot (needs Message Content
privileged intent enabled), and `web_search` (Serper) answers. Repeat against the
Lightsail deployment: `/health` green, a Discord message triggers a chunked reply,
`web_search` returns current results. **Cutover rule:** stop any local gateway first —
two connected instances double-answer.

---

## Phase 2 — Deployment Hardening

**Derived status: In Progress** (1 of 5 done)

| RMI | Status | Primary change |
|-----|--------|----------------|
| RMI-OMNIAGENT-006 | Planned | omnideploy: implement secret injection (SSM/Secrets Manager) so `DISCORD_BOT_TOKEN` etc. never enter Pulumi state. `SecretRef` schema exists in `config/types.go` but the Lightsail target never reads `cfg.Secrets` |
| RMI-OMNIAGENT-007 | Planned | Durable storage for `sessions`/`cron` (external DB or object store); or document + accept statelessness |
| RMI-OMNIAGENT-008 | Planned | Guard `replicas == 1` for gateway mode; scaling guidance in deploy docs |
| RMI-OMNIAGENT-009 | **Done** | Implemented: `signal.Notify(SIGINT, SIGTERM)` → `<-sigCh` → `cancel()` → deferred `router.DisconnectAll` in `cmd/omniagent/commands/gateway.go` |
| RMI-OMNIAGENT-010 | Planned | grokify-omniagent: `go mod vendor` (or publish the 11 replaced deps) so its image is CI-buildable |

---

## Phase 3 — Discord Channel Completeness

**Derived status: Planned** (0 of 5 done) · work lands in `omnichat/providers/discord`

| RMI | Status | Primary change |
|-----|--------|----------------|
| RMI-OMNIAGENT-011 | Planned | `Send`: build `discordgo.MessageSend.Files`/`Embeds` from `OutgoingMessage.Media` |
| RMI-OMNIAGENT-012 | Planned | `convertIncoming`: map `m.Attachments` → `IncomingMessage.Media` |
| RMI-OMNIAGENT-013 | Planned | Register `ApplicationCommand`s + `InteractionCreate` handler |
| RMI-OMNIAGENT-014 | Planned | HTTP interactions endpoint with `X-Signature-Ed25519` verification (webhook mode) |
| RMI-OMNIAGENT-015 | Planned | Enforce configured `guildID`; add reaction/edit/delete event mapping |

After each: bump omnichat, propagate the version into `omniagent/go.mod`.

---

## Phase 4 — Gateway Security & Observability

**Derived status: In Progress** (1 of 5 done)

| RMI | Status | Primary change |
|-----|--------|----------------|
| RMI-OMNIAGENT-016 | Planned | Real `CheckOrigin` in `gateway/gateway.go` (allowlist), replacing the `return true` TODO |
| RMI-OMNIAGENT-017 | Planned | Auth on `/ws` (API key/token). A `handleAuth` stub exists in `gateway/handlers.go` but accepts all requests (`// TODO`) — replace with real auth |
| RMI-OMNIAGENT-018 | Planned | Per-sender rate limiter in the message-processing path |
| RMI-OMNIAGENT-019 | **Done** | Implemented: `ObservabilityHook` (omniobserve/llmops) constructed and applied to the agent in `gateway run`; slog/langfuse/metrics/trace providers available. Opik is an unbuilt follow-on provider |
| RMI-OMNIAGENT-020 | Planned | `/metrics` Prometheus endpoint on the gateway mux |

---

## Definition of Done (per plexusone org)

Every RMI's PR must satisfy: implementation follows repo patterns; unit tests for
new functions; `golangci-lint run` clean; README/MkDocs updated on user-visible
change; changelog entry or conventional-commit; `Refs: RMI-OMNIAGENT-<NNN>` trailer;
no local `replace` directives and no references to untracked files before push.
