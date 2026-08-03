# OpenClaw Sync Plan: 2026-07-30

Sync OmniAgent with OpenClaw changes from `2001b15f` to `3e7ecb3a`.

## Summary

| Metric | Value |
|--------|-------|
| Previous sync | `2001b15f5b92d653464cbd847c28c136bdb465a7` (2026-06-28) |
| Target sync | `3e7ecb3aeaaf3a9dd4fa8c46c31b63c6f2031ccc` (2026-07-30) |
| Commits | 11,280 |
| Date range | 2026-06-28 to 2026-07-30 |

## Port Analysis (2026-08-03)

A diff-level cross-check of every unmapped/in-flight commit against the OpenClaw
source at `3e7ecb3a` and the plexusone Go targets is recorded in
[`PORT-SPEC.md`](PORT-SPEC.md). Net verdict across 24 analyzed commits:
**12 PORT / 12 N-A**.

Key outcomes:

- **N-A (no landing surface in omniagent):** `d9525663852` (RMI-060), `82af1bf7d40`
  (RMI-063), `5496cff9653` (RMI-065), `9560e171855` (RMI-069), `3f918d4b3ca`
  (RMI-062 — rider on 061), `e5cee36b46c`, `280c1627533`, `6b37d46b179`,
  `ad1dc602594`, `750dc999038`, `29497b4e1e8`, `46ec7a43a8a`.
- **Already satisfied:** `2899323fe3e` (plugin scans expose all calls),
  `83aca8a59c7` (blank-search early return).
- **New gaps the RMI set missed:** cron fail-closed authority (`cee6203f448`,
  security), auth-failure escalating delay (`ea967be0df6`, security), pre-tool
  assistant text drop (`c661061d4a7`, bug), session cache release on Close
  (`c63fdc631f0`, bug), realtime readLoop silent event drop (bug), temporal
  context in system prompt (`54e273e6953`), `agent_end` lifecycle event
  (`8fefe62ba41`, enabling), sticky session model (`eee5e029e36`, blocked).

Commit tables below are annotated with `[PORT]`, `[N-A]`, `[DONE]`, or
`[RIDER]` per those findings; unannotated rows were completed under RMIs 050–059.

## Priority Categories

### P0: Security - Bounded Reads (Critical)

OpenClaw continues hardening against unbounded reads in realtime/streaming contexts. These patterns should be applied to OmniAgent's OpenAI realtime implementation.

| Commit | Description | OmniAgent Module |
|--------|-------------|------------------|
| `039efadf592` | Bound realtime SDP answer reads | `api/openai/` |
| `280c1627533` | Bound GPT-Live sideband startup frames [N-A — no sideband subsystem; 16 MiB read limit already caps payload] | `api/openai/` |
| `3133abed325` | Bound pre-ready realtime audio | `api/openai/` |
| `567d04076a5` | Bound realtime playback marks | `api/openai/` |
| `6b37d46b179` | Bound GPT-Live delegation work [N-A — no delegation broker; analogous path already single-flight] | `api/openai/` |
| `750dc999038` | Preserve playback acknowledgement order [N-A — no playback marks; adjacent readLoop silent-drop bug filed as PORT] | `api/openai/` |
| `ad1dc602594` | Scope realtime readiness state [N-A — JS block-scoping hygiene, moot in Go] | `api/openai/` |

**Action items:**

1. Audit `api/openai/` for unbounded WebSocket message reads
2. Add `io.LimitReader` bounds to SDP negotiation
3. Bound audio frame buffers in realtime streaming
4. Verify playback mark accumulation is bounded

### P1: Security - Gateway & Hooks

| Commit | Description | OmniAgent Module |
|--------|-------------|------------------|
| `e02bb90f181` | Cancel unread cron webhook bodies before SSRF release | `gateway/` |
| `2899323fe3e` | Expose every executable call in plugin scans [DONE — already satisfied: omniskill loader/markdown.go + clawhub/security.go collect all calls] | `skills/` |
| `4823d7fe7b2` | Fence external hook job name in isolated-agent prompts | `hooks/` |
| `54a6dba0d3d` | Validate hook delivery accounts | `gateway/` |
| `814b9f6e36f` | Bound agent runner admission | `hooks/` |
| `cee6203f448` | Deny all tools when scheduled authority names removed account [PORT — new security RMI; omniagent cron is currently fail-open] | `cron/` |

**Action items:**

1. Review webhook body handling in `gateway/handlers.go` for SSRF vectors
2. Audit skill/hook execution for prompt injection via job names
3. Add account validation before hook delivery
4. Bound concurrent agent runner admissions

### P2: Agent Core Fixes

Critical fixes for the edit/patch tools and agent orchestration.

#### Edit Tool Line Ending Fixes

| Commit | Description | Priority |
|--------|-------------|----------|
| `51241c4e00f` | Edit tool rewrites line endings on lines it did not touch | High |
| `8ae0a896bcf` | apply_patch converts CRLF files to mixed line endings | High |
| `8ce1e18d2f2` | apply_patch rewrites bytes on hunk context lines | High |
| `edf4aca7bc2` | apply_patch destroys existing file when patch creates that path | High |
| `1ab48a71df4` | Edit rejects a unique match as ambiguous | Medium |

**Action:** Review `agent/tools/` edit implementation for line ending preservation.

#### Agent Orchestration

| Commit | Description | OmniAgent Module |
|--------|-------------|------------------|
| `c4b82609b70` | Compare shell tool names case-insensitively | `agent/tools/` |
| `d30d22d33c1` | Prevent registry stalls during large fan-outs | `agent/` |
| `b942db4d569` | Throw CompactionError when summarization fails | `agent/` |
| `29497b4e1e8` | Keep subagent run ID mask from splitting UTF-16 surrogate pairs [N-A — UTF-16 slice defect cannot occur on Go UTF-8 strings; no run-ID masking exists] | `agent/` |
| `46ec7a43a8a` | Prevent model runtime startup timeout [N-A — no model catalog/plugin-metadata subsystem; model is config-string passthrough] | `agent/` |
| `54e273e6953` | Keep date reasoning current in long-running sessions [PORT — inject fresh temporal context per turn; omniagent injects no date at all] | `agent/` |
| `c661061d4a7` | Keep streamed pre-tool assistant text in final output [PORT — live bug: run loop drops pre-tool Content at agent.go:365-369] | `agent/` |
| `8fefe62ba41` | Settle aborted runs through after-turn so agent_end fires [PORT — enabling: requires introducing agent_end event + finalize step first] | `agent/` |

### P3: Memory & Session Fixes

| Commit | Description | OmniAgent Module |
|--------|-------------|------------------|
| `d9525663852` | MEMORY.md compaction deletes user notes under promotion-style heading [N-A — RMI-060 blocked: no MEMORY.md promotion/budget-compaction subsystem exists] | `memory/` |
| `e5cee36b46c` | Retry failed queued session targets [N-A — sessions Save is synchronous; no queued-sync path to retry] | `sessions/` |
| `c63fdc631f0` | Release closed SQLite entry caches [PORT — sessions/store.go Close() never calls ClearCache; S-size fix] | `sessions/` |
| `73808753757` | Keep degraded status actionable [N-A — no memory health/degraded status concept] | `memory/` |
| `83aca8a59c7` | Return early for blank searches [DONE — already satisfied: omniretrieve bm25.go:204 early-returns on empty query] | `memory/` |
| `cf927f7b1b1` | Keep rollbacks compatible after recall metadata upgrade [N-A — no recall-metadata versioning subsystem] | `memory/` |
| `ef63df8afdb` | Preserve taint across transcript runtimes [N-A — no taint/transcript-runtime concept] | `memory/` |
| `a2ead0a9292` | Recover divergent dreaming journals [N-A — no dreaming/journal subsystem] | `memory/` |
| `216f45af8ec` | Preserve Windows session ownership [N-A — sessions are KVS-backed, no file ownership] | `sessions/` |

**Action:** Review `omniretrieve` memory compaction to preserve user notes.

### P4: Hooks & Scheduling

| Commit | Description | OmniAgent Module |
|--------|-------------|------------------|
| `9560e171855` | Add persistent hook session mode [N-A — RMI-069: no inbound hook→session gateway; cron path already implicitly persistent] | `hooks/` |
| `73b5e7aab49` | Save memory on automatic session rollover [PORT — RMI-061; net-new: needs rollover trigger + session.rollover event + memory hook] | `hooks/` |
| `3f918d4b3ca` | Honor user timezone in session memory [RIDER — folds into RMI-061 as an acceptance criterion] | `hooks/` |
| `9e041cd3867` | Preserve multi-account agent delivery [N-A — no account-partitioned delivery] | `hooks/` |
| `fc95d7190cb` | Route mapped wake events to configured sessions [N-A — no wake-event concept] | `hooks/` |
| `43945b836ab` | Retain manual reset memory admission [N-A — no memory-admission gating] | `hooks/` |
| `215e49b1a2f` | Report eventless hooks as not ready [N-A — Hook interface has no readiness/status surface] | `hooks/` |
| `819961a292d` | Avoid implicit hook delivery targets [N-A — no delivery-target resolution model] | `hooks/` |
| `82af1bf7d40` | Stop replaying old schedule slots after cron job edited [N-A — RMI-063: no restart catch-up state machine; UpdateJob recomputes from now] | `cron/` |
| `a877fe1b32a` | Prevent stalled schedules, duplicate runs, lost heartbeats [PORT — RMI-064 narrowed to duplicate-run prevention; stall/heartbeat/lock parts N-A] | `cron/` |

### P5: New Features Worth Porting

| Commit | Description | OmniAgent Module | Priority |
|--------|-------------|------------------|----------|
| `5496cff9653` | Include run time in cron failure alerts [N-A — RMI-065 blocked: no cron failure-alert subsystem to enrich] | `cron/` | Medium |
| `42e23c11e0f` | Allow per-turn tool narrowing in prompt hooks [PORT — RMI-066; requires new synchronous hook stage] | `hooks/` | Medium |
| `a37a5a65753` | Add per-session tool overrides [PORT — RMI-067; requires session-scoped tool resolution; sequence after 066] | `agent/` | Medium |
| `eee5e029e36` | Persist last-used session model as agent default [PORT — blocked on missing per-session model selection + writable config] | `agent/` | Low |
| `ea967be0df6` | Add loopback locality controls [PORT — auth-failure escalating-delay half only; pairing half N-A] | `gateway/` | Low |
| `adf3178ae6c` | Expose MCP tool identity in effective tools [PORT — RMI-068; identity fields independent, deniedBySession depends on 067] | `gateway/` | Medium |
| `662abec7541` | Push session PR indicators to subscribed clients [DEFERRED — low priority, no PR-status concept; not analyzed at diff level] | `gateway/` | Low |
| `4c4aa2ed126` | Manage audio/video attachments end to end [DEFERRED — low priority, no attachment path; not analyzed at diff level] | `gateway/` | Low |
| `fb788b79cf7` | Retain recent project scopes per session [N-A — no project-scope concept] | `memory/` | Low |
| `14940edf15f` | Add Skill Workshop lifecycle hooks [N-A — no Skill Workshop concept] | `skills/` | Low |

### P6: Channel Fixes (Lower Priority)

These are channel-specific fixes. Port as needed based on omnichat channel usage.

| Commit | Channel | Description |
|--------|---------|-------------|
| `b765ada174a` | WhatsApp | Stop restart loop after remote logout |
| `4e5bf66fb18` | WhatsApp | Silently drops inbound messages when >450 waiting |
| `21d35334601` | Channels | Ack mentions in groups that don't require them |
| `7c47e4ad0e5` | Channels | Collapse cumulative commentary snapshots |
| `c63241d3cea` | Channels | Resolve tool progress against caller's stream mode |
| `06aa81a73f8` | Channels | Show tool lines under progress status headline |
| `cfa98bdf00a` | Telegram | Emit sent hook for finalized previews |
| `91dca69dae2` | QQBot | Use Gateway timezone for reminders |
| `32a61cabf02` | Matrix | Observe automatic reply settlement |
| `b660662e013` | Discord | Preserve webhook deadline errors |
| `601a405430d` | Multi | Mark durable webhook acceptance (Zalo, Google Chat, SMS, Feishu, Nextcloud Talk, Synology Chat) |

### P7: Platform-Specific (Out of Scope)

These are iOS/macOS/Android native app fixes - not applicable to Go port.

| Commits | Platform | Notes |
|---------|----------|-------|
| 50+ | macOS | Voice wake, notifications, tunnel management |
| 30+ | iOS | Capability routing, i18n |
| 20+ | Android | MediaSession, video upload |

---

## Implementation Plan

### Phase 1: Security Hardening (P0 + P1)

**Estimated effort:** 2-3 days

1. [ ] Audit `api/openai/` realtime WebSocket handling
   - Add bounded reads for SDP negotiation
   - Bound audio frame buffers
   - Add playback mark limits

2. [ ] Audit `gateway/` webhook handling
   - Cancel unread bodies before SSRF release
   - Validate hook delivery accounts

3. [ ] Audit `hooks/` for prompt injection
   - Fence external job names in prompts
   - Bound agent runner admission

### Phase 2: Agent Core (P2)

**Estimated effort:** 3-4 days

1. [ ] Fix edit tool line ending handling
   - Preserve original line endings on untouched lines
   - Handle CRLF files correctly
   - Fix hunk context line byte preservation

2. [ ] Improve agent orchestration
   - Case-insensitive tool name comparison
   - Prevent registry stalls on fan-out
   - Add CompactionError for failed summarization
   - UTF-16 surrogate pair safety in run ID masks

### Phase 3: Memory & Hooks (P3 + P4) — rescoped 2026-08-03

**Estimated effort:** 3-4 days (see PORT-SPEC.md)

1. [ ] ~~Fix memory compaction~~ — rescoped
   - ~~Preserve user notes under promotion-style headings~~ [N-A — RMI-060 blocked: no MEMORY.md promotion subsystem]
   - ~~Keep degraded status actionable~~ [N-A — no memory health status]
   - Early return for blank searches [DONE — already satisfied]
   - [ ] Release session cache on `Store.Close` (`c63fdc631f0`, S)

2. [ ] Improve hook session handling — rescoped
   - ~~Add persistent session mode~~ [N-A — RMI-069: no inbound hook gateway]
   - [ ] Save memory on session rollover (RMI-061, L; prereq: rollover trigger + `session.rollover` event; builds on `agent_end` substrate)
   - Honor user timezone [RIDER — acceptance criterion of RMI-061]

3. [ ] Fix cron scheduling — rescoped
   - ~~Stop replaying old slots after edit~~ [N-A — RMI-063: no catch-up state machine]
   - [ ] Prevent duplicate concurrent runs (RMI-064 narrowed, M)

### Phase 4: Features (P5) — rescoped 2026-08-03

**Estimated effort:** 4-6 days (066/067 require new shared substrate)

1. ~~Add run time to cron failure alerts~~ [N-A — RMI-065 blocked: no failure-alert subsystem]
2. [ ] Add per-turn tool narrowing in hooks (RMI-066, L — introduces synchronous hook stage)
3. [ ] Add per-session tool overrides (RMI-067, L — introduces session-scoped tool resolution; after 066)
4. [ ] Expose MCP tool identity in effective tools (RMI-068, M — identity fields independent; deniedBySession after 067)

### Phase 5: Ported Gap Fixes (new 2026-08-03)

Security and correctness gaps surfaced by the diff-level port analysis
(PORT-SPEC.md) that the original triage missed:

1. [ ] **[security]** Cron fail-closed on removed authorizing account (`cee6203f448`, L) — cron currently fail-open
2. [ ] **[security]** Gateway auth-failure escalating delay (`ea967be0df6`, M)
3. [ ] **[bug]** Preserve pre-tool assistant text in final output (`c661061d4a7`, M)
4. [ ] **[bug]** Realtime readLoop silent event drop (omni-openai `client.go:306-308`, M)
5. [ ] **[correctness]** Temporal context in system prompt (`54e273e6953`, M)
6. [ ] **[enabling]** `agent_end` lifecycle event on all exit paths (`8fefe62ba41`, L)

---

## Files to Review

Based on the OpenClaw changes, review these OmniAgent files:

| OmniAgent Path | OpenClaw Equivalent | Changes |
|----------------|---------------------|---------|
| `api/openai/realtime.go` | `packages/openai-realtime/` | Bounded reads, playback marks |
| `gateway/handlers.go` | `packages/gateway/` | Webhook body handling, SSRF |
| `gateway/hooks.go` | `packages/gateway/hooks/` | Account validation, delivery |
| `agent/tools/edit.go` | `packages/agents/tools/` | Line ending preservation |
| `agent/orchestration.go` | `packages/agents/` | Fan-out, compaction |
| `hooks/webhook/` | `packages/hooks/` | Session mode, timezone |
| `cron/scheduler.go` | `packages/cron/` | Slot replay, heartbeats |
| `memory/compaction.go` | `packages/memory/` | Note preservation |

---

## Testing Requirements

1. **Security tests:**
   - Verify bounded reads reject oversized payloads
   - Test SSRF prevention in webhook handling
   - Test prompt injection escaping in hook job names

2. **Edit tool tests:**
   - Test CRLF file editing preserves line endings
   - Test mixed line ending files
   - Test ambiguous match rejection

3. **Memory tests:**
   - Test compaction preserves user notes
   - Test blank search early return

4. **Cron tests:**
   - Test slot replay prevention after edit
   - Test heartbeat recovery

---

## Notes

- OpenClaw commit count since last sync: 11,280
- Most commits are test/CI/platform-specific (iOS/macOS/Android)
- Core backend changes relevant to Go port: ~150-200 commits
- Security-related changes: ~20 commits (priority)
- Agent/memory core fixes: ~50 commits
