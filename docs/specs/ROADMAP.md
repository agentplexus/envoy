# OmniAgent Delivery — Deployment, Discord, and Gateway Hardening — Roadmap

**Initiative:** `INIT-OMNIAGENT-001`
**Repository:** `github.com/plexusone/omniagent`
**Status:** Executing — 16 of 30 items completed

> RMI IDs are stable and permanent. Commits implementing an item carry the trailer `Refs: RMI-OMNIAGENT-<NNN>`. Phase status is derived from member RMIs — a phase is complete only when all its required RMIs are complete.

## Phase 1 — Cloud Deployment

**Theme:** Ship OmniAgent as a Discord bot on AWS Lightsail via omnideploy from a public GHCR image.
**Status:** In progress — 1 of 5 items completed

- [x] `RMI-OMNIAGENT-001` Discord 2000-character message chunking in omnichat
  - - Acceptance: replies over 2000 chars split into multiple messages; rune-based, breaks on paragraph/newline/space; shipped as omnichat v0.8.1 and linked into the omniagent binary
- [x] `RMI-OMNIAGENT-002` Standalone multi-stage Dockerfile for omniagent
  - - Acceptance: `docker build --platform linux/amd64` succeeds with no local replace mounts; runs `gateway run`; serves `/health`
- [ ] `RMI-OMNIAGENT-003` Publish container image to GHCR (public)
  - Depends on: `RMI-OMNIAGENT-002`
- [ ] `RMI-OMNIAGENT-004` Lightsail deployment via omnideploy
  - Depends on: `RMI-OMNIAGENT-003`
  - - Acceptance: `omnideploy up --target lightsail --backend pulumi` provisions a running service with the Discord/agent/Serper env
- [ ] `RMI-OMNIAGENT-005` Deployment smoke test on Lightsail
  - Depends on: `RMI-OMNIAGENT-004`
  - - Acceptance: `/health` green; a Discord message triggers a chunked reply; `web_search` (Serper) returns current results

## Phase 2 — Deployment Hardening

**Theme:** Make the deployment production-safe: secrets, durable storage, and CI-buildable images.
**Status:** In progress — 2 of 5 items completed

- [ ] `RMI-OMNIAGENT-006` SSM-backed secret injection in omnideploy Lightsail target
  - - Acceptance: `SecretRef` (`ssm:`/`secretsmanager:`) resolved and injected; secrets never land in plaintext env or Pulumi state (schema exists; target does not yet consume `cfg.Secrets`)
- [x] `RMI-OMNIAGENT-007` Durable session/cron storage on Lightsail
  - - Acceptance: session and cron state survive a redeploy (Lightsail Container Service has no volumes; SQLite at `STORAGE_PATH` is ephemeral)
  - - Shipped: config-driven `storage.type` (`sqlite`/`redis`/`memory`) + `sessions.enabled`/`sessions.ttl`, matching the surface `docs/reference/configuration.md` had documented as "planned"; `gateway run` now builds the backend and wires `agent.WithStorage`/`WithSessionStore` for every agent, and `agent.WithCronScheduler()` for the single-agent path (multi-agent mode skips it — one shared job store, one scheduler, avoids duplicate firing). Also fixed a real gap found in the process: `gateway/handlers.go`'s Discord/WebSocket chat path called the stateless `Process` unconditionally despite a "conversation continuity" comment — it now dispatches to `ProcessWithSession` via a new `gateway.SessionAwareProcessor` capability check whenever a session store is configured, so persisted history is actually used, not just stored.
- [x] `RMI-OMNIAGENT-008` Single-replica gateway guard
  - - Acceptance: gateway mode enforces or documents `replicas == 1` (one Discord WebSocket; extra replicas double-answer)
- [x] `RMI-OMNIAGENT-009` Graceful shutdown on SIGINT/SIGTERM
  - - Acceptance: signal handler cancels context and runs `router.DisconnectAll` on shutdown (implemented in `cmd/omniagent/commands/gateway.go`)
- [ ] `RMI-OMNIAGENT-010` CI-buildable grokify-omniagent image
  - - Acceptance: grokify-omniagent builds in CI without local `replace` mounts (vendor or publish the 11 replaced deps)

## Phase 3 — Discord Channel Completeness

**Theme:** Close the documented-but-unimplemented Discord gaps (work lands in omnichat/providers/discord).
**Status:** Planned — 0 of 5 items completed

- [ ] `RMI-OMNIAGENT-011` Discord media send
  - - Acceptance: `Send` builds `discordgo.MessageSend` `Files`/`Embeds` from `OutgoingMessage.Media` (currently only `Content` is set)
- [ ] `RMI-OMNIAGENT-012` Discord media receive
  - - Acceptance: `convertIncoming` maps `m.Attachments` to `IncomingMessage.Media`
- [ ] `RMI-OMNIAGENT-013` Discord slash commands
  - - Acceptance: `ApplicationCommand` registration + `InteractionCreate` handler
- [ ] `RMI-OMNIAGENT-014` Discord HTTP interactions with Ed25519 verification
  - - Acceptance: webhook endpoint verifies `X-Signature-Ed25519`; usable as an alternative to gateway mode
- [ ] `RMI-OMNIAGENT-015` guildID enforcement and message events
  - - Acceptance: configured `guildID` is enforced (currently stored but unused); reaction/edit/delete events mapped

## Phase 4 — Gateway Security and Observability

**Theme:** Harden the gateway control plane and add production observability.
**Status:** Complete — 5 of 5 items completed

- [x] `RMI-OMNIAGENT-016` WebSocket origin checking
  - - Acceptance: `CheckOrigin` enforces an allowlist (currently returns `true` with a `// TODO` in `gateway/gateway.go`)
- [x] `RMI-OMNIAGENT-017` Gateway WebSocket authentication
  - - Acceptance: real auth on `/ws` (the `handleAuth` handler currently accepts all requests via a `// TODO` stub)
- [x] `RMI-OMNIAGENT-018` Per-sender rate limiting
  - - Acceptance: message-processing path rate-limits per sender/channel
- [x] `RMI-OMNIAGENT-019` Observability tracing hook
  - - Acceptance: `ObservabilityHook` (omniobserve/llmops) applied to the agent in `gateway run`; slog/langfuse providers available (Opik provider is a follow-on)
- [x] `RMI-OMNIAGENT-020` Prometheus metrics endpoint
  - - Acceptance: `/metrics` exposes gateway/agent metrics for scraping

## Phase 5 — Test Coverage

**Theme:** Increase test coverage for critical low-coverage packages.
**Status:** In progress — 2 of 5 items completed

- [ ] `RMI-OMNIAGENT-021` Agent package test coverage (target: 50%+)
  - - Acceptance: `agent` package coverage increases from 7% to 50%+; covers core Process flow, tool execution, session handling
- [ ] `RMI-OMNIAGENT-022` OpenAI API server test coverage (target: 50%+)
  - - Acceptance: `api/openai` package coverage increases from 15.6% to 50%+; covers streaming, models endpoint, tool listing
- [x] `RMI-OMNIAGENT-023` OpenAI adapter test coverage (target: 50%+)
  - - Acceptance: `openai` adapter package coverage increases from 19.9% to 50%+; covers multi-agent routing, chat completion, cron handler
  - - Shipped: 19.9% → 91.4%. Covers routing precedence, the `useSession` gate, streaming reassembly, and the cron handler end-to-end against a real in-memory scheduler; a shared httptest-backed fake LLM drives a real `*agent.Agent` with no network access.
- [ ] `RMI-OMNIAGENT-024` Voice package test coverage (target: 50%+)
  - - Acceptance: `voice` package coverage increases from 22.6% to 50%+; covers gateway, processor, providers
- [x] `RMI-OMNIAGENT-025` Skills package test coverage (target: 50%+)
  - - Acceptance: `skills` package coverage increases from 37.8% to 50%+; covers skill loading, execution, validation
  - - Shipped: 41.2% → 96.7%. Covers discovery/loading (directory + embedded-pack sources, dedup precedence, malformed manifests), the `Manager` (Load/Get/All/Count/Available, Includes/Excludes ordering), and `CheckRequirements` install-hint formatting.

## Phase 6 — Context and Token Management

**Theme:** Implement the deferred context window and token counting features.
**Status:** In progress — 4 of 5 items completed

- [x] `RMI-OMNIAGENT-026` Model-specific token counting
  - - Acceptance: `ModelTokenCounter` uses tiktoken for OpenAI models, provider-specific counting for Anthropic/others; accurate to within 5% of actual usage
- [ ] `RMI-OMNIAGENT-027` LLM-based conversation summarization
  - - Acceptance: `WindowStrategySummarize` calls LLM to summarize older messages; configurable summarization prompt; summary replaces N oldest messages
- [x] `RMI-OMNIAGENT-028` Autoreply template rendering
  - - Acceptance: `autoreply` package template TODO implemented; supports variable substitution in auto-reply messages
- [x] `RMI-OMNIAGENT-029` WebSocket origin allowlist
  - - Acceptance: `CheckOrigin` in `gateway/gateway.go` enforces configurable allowlist; rejects requests from unlisted origins
- [x] `RMI-OMNIAGENT-030` Gateway WebSocket authentication
  - - Acceptance: `handleAuth` in `gateway/handlers.go` validates tokens/credentials; supports API key and JWT authentication
