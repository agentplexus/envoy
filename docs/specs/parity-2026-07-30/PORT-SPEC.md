# OpenClaw Parity — Port Spec (Track 1)

Diff-level analysis of the unmapped/in-flight commits behind `INIT-OMNIAGENT-002`,
verified against the actual OpenClaw source (TypeScript) checked out at the plan's
target commit and the plexusone (Go) target repos.

| Field | Value |
|-------|-------|
| Reference source | `~/go/src/github.com/openclaw/openclaw` @ `3e7ecb3a` (= plan target) |
| Sync base | `2001b15f` (2026-06-28) |
| Commits analyzed | 24 (10 RMIs 060–069 + 10 applicable gaps + 4 PARTIALs) |
| Method | Read the real diff per commit → locate the Go target → PORT / N-A verdict + acceptance criteria |
| Nature | **Cross-language behavior port** (TS→Go). Diffs are reference behavior, not applyable patches. |

## Executive summary

**Can we complete the roadmap as written? No.** The plan's four remaining phases were
triaged from commit *one-line messages*. Reading the actual diffs shows **4 of the 10
proposed RMIs (060, 063, 065, 069) target OpenClaw subsystems omniagent does not have**,
and a 5th (062) only exists as a rider on an unbuilt one (061). They cannot be delivered
as ports and must be struck or rescoped.

At the same time, the diff analysis surfaced **real, in-scope work the RMI set never
captured** — including two security items and three correctness bugs.

Net verdict: **12 PORT / 12 N-A**.

- **12 PORT** — genuinely applicable; 3 are security/correctness bugs live in the tree today.
- **12 N-A** — no landing surface (subsystem absent, or the defect can't occur in Go).

## Master verdict table

| Commit | Title | RMI / kind | Verdict | Size |
|--------|-------|-----------|---------|------|
| `cee6203f448` | Deny all tools when scheduled authority names removed account | gap · **security** | **PORT** | L |
| `c661061d4a7` | Keep streamed pre-tool assistant text in final output | gap · **bug** | **PORT** | M |
| `c63fdc631f0` | Release closed session/SQLite entry caches | gap · **bug** | **PORT** | S |
| `ea967be0df6` | Loopback auth-failure escalating delay | gap · **security** | **PORT** (delay half) | M |
| `750dc999038`† | Realtime readLoop silently drops events | adjacent · **bug** | **PORT** | M |
| `54e273e6953` | Keep date reasoning current (temporal context) | gap · correctness | **PORT** | M |
| `a877fe1b32a` | Prevent duplicate concurrent cron runs | RMI-064 (narrowed) | **PORT** | M |
| `8fefe62ba41` | Emit agent_end on aborted runs | gap (enabling) | **PORT** | L |
| `adf3178ae6c` | Expose MCP tool identity in effective tools | RMI-068 | **PORT** | M–L |
| `42e23c11e0f` | Per-turn tool narrowing in prompt hooks | RMI-066 | **PORT** | L |
| `a37a5a65753` | Per-session tool overrides | RMI-067 | **PORT** | L |
| `73b5e7aab49` | Save memory on automatic session rollover | RMI-061 | **PORT** | L |
| `eee5e029e36` | Persist last-used session model as default | gap | PORT (blocked) | L |
| `d9525663852` | Preserve user notes in MEMORY.md compaction | RMI-060 | **N-A** (no subsystem) | — |
| `82af1bf7d40` | Stop replaying old schedule slots after edit | RMI-063 | **N-A** (no catch-up) | — |
| `5496cff9653` | Include run time in cron failure alerts | RMI-065 | **N-A** (no alert subsystem) | — |
| `9560e171855` | Add persistent hook session mode | RMI-069 | **N-A** (no inbound gateway) | — |
| `3f918d4b3ca` | Honor user timezone in session memory | RMI-062 | **N-A** (rider on 061) | — |
| `e5cee36b46c` | Retry failed queued session targets | gap | **N-A** (no async queue) | — |
| `280c1627533` | Bound GPT-Live sideband startup frames | PARTIAL | **N-A** (no sideband) | — |
| `6b37d46b179` | Bound GPT-Live delegation work | PARTIAL | **N-A** (no delegation broker) | — |
| `ad1dc602594` | Scope realtime readiness state | PARTIAL | **N-A** (Go block scoping) | — |
| `750dc999038` | Preserve playback acknowledgement order | ABSENT | **N-A** (no playback marks)† | — |
| `29497b4e1e8` | UTF-16 surrogate safety in run-ID mask | PARTIAL | **N-A** (can't occur in Go) | — |
| `46ec7a43a8a` | Prevent model runtime startup timeout | ABSENT | **N-A** (no model catalog) | — |

† `750dc999038` itself is N-A (no playback-mark subsystem), but the audit surfaced a
distinct, live bug in the same file — `readLoop` silently drops server events (audio /
transcript deltas) when the cap-100 channel is full — which IS actionable. Tracked as its
own PORT row above.

---

## PORT items (implementable)

Ordered by recommended priority. Each is a behavior port — implement the intent in Go,
do not transliterate the TypeScript.

### 1. `cee6203f448` — Cron fail-closed on removed authorizing account · **security · L**

- **Behavior:** A scheduled run executes under the authority of the account/session that
  created it. When that account is gone, tool policy must fail **closed** (deny all).
- **Go target:** `cron/job.go` (add authorizing principal to `Job`); `cron/executor.go:198`
  `executeCallTool` (add existence check + deny-all branch before dispatch).
- **Why it matters:** omniagent's cron is currently **fail-OPEN** — `executeCallTool` calls
  `agent.Tools().Execute(...)` unconditionally (`cron/executor.go:215`); `Job` has no
  owner field at all (`cron/job.go:44-83`). A stored `call_tool` job runs any tool
  (incl. `exec`-class) regardless of who created it or whether they still exist. This is
  strictly more permissive than the upstream bug.
- **Acceptance:** (1) `Job` carries a non-spoofable `OwnerAccountID`/`OwnerSessionKey`
  stamped at create time. (2) Before dispatch, resolve principal against configured
  accounts; if removed/unknown, return `Success:false` denying **all** tools. (3) Deny is a
  wildcard deny, never "empty allowlist = allow all". (4) Configured-principal jobs run as
  today. (5) Test: job whose owner is later removed → deny-all, tool not invoked.
- **Prereq:** needs an account/identity source the cron package doesn't yet import.

### 2. `c661061d4a7` — Stop dropping pre-tool assistant text · **bug · M**

- **Behavior:** When a turn emits answer text *and* a tool call, the pre-tool text must
  survive into the final output.
- **Go target:** `agent/agent.go:352-369` (run loop final-string construction).
- **Why it matters:** live user-visible content loss. On a tool-call turn the assistant
  message is appended with `ToolCalls` but **without** `choice.Message.Content`
  (`agent.go:365-369`), and only the terminal no-tool-call `Content` is returned
  (`agent.go:359`). The faked-streaming adapter then chunks exactly that string, so
  "Let me check X" before a tool call is gone forever. (Do **not** port the upstream
  JSONL parser — it's specific to their CLI streaming; port only the behavior.)
- **Acceptance:** (1) Non-empty `Content` on a tool-call turn is accumulated and joined
  (blank-line separator) into the returned string. (2) The appended tool-call assistant
  message also carries its `Content` for faithful history. (3) "text → tool → text" returns
  both segments. (4) No double-spacing. (5) Single-turn output unchanged.

### 3. `c63fdc631f0` — Release session cache on Close · **bug · S** *(quick win)*

- **Behavior:** A keyed entry cache must not outlive the resource it belongs to.
- **Go target:** `sessions/store.go:218` `Store.Close`.
- **Why it matters:** `ClearCache` exists (`store.go:211`) but `Close` never calls it
  (`store.go:218-220`) — a closed `Store` retains every cached `*Session`. (Go has no
  `WeakMap`; the idiomatic port is release-on-close, not weak refs.)
- **Acceptance:** (1) `Close` releases the cache under `s.mu`. (2) Idempotent, no race with
  Get/Save. (3) Backend still closed, error still returned. (4) Test: cache len 0 after Close.

### 4. `ea967be0df6` — Auth-failure escalating delay · **security · M**

- **Behavior:** Failed gateway auth incurs a bounded, escalating, **post-comparison** delay
  (base ~250ms, doubling, ~5s cap); loopback is never locked out but still pays the delay.
- **Go target:** new `gateway/auth_ratelimit.go`, wired into the failure branch of
  `handleAuth` (`gateway/handlers.go:147-159`); key off client IP from the HTTP upgrade
  (`gateway/gateway.go:157`).
- **Why it matters:** `handleAuth` returns immediately on mismatch — no delay, no failure
  counting (`gateway/handlers.go:138-159`); `gateway/ratelimit.go` is a message-throughput
  bucket, unrelated. **Scope down to the delay limiter only** — the commit's other half
  (`autoApproveLocal` device pairing) has no counterpart and is N-A.
- **Acceptance:** (1) Escalating delay on repeated failures. (2) Loopback delayed, never
  locked out. (3) Delay only after credential compare (correct creds never delayed).
  (4) Concurrent failures for one key share one bounded penalty (no timer/goroutine growth).
  (5) Success resets penalty.

### 5. `750dc999038`-adjacent — Realtime readLoop silent drop · **bug · M**

- **Behavior:** Never silently drop ordered server events under backpressure.
- **Go target:** `omni-openai/omnivoice/realtime/client.go:302-308`.
- **Why it matters:** `readLoop`'s `select`-send to `eventsCh` has a `default:` that drops
  parsed events (audio/transcript deltas) when the cap-100 channel is full — silent
  loss/reordering under load. (This is the true "preserve order / no loss" analog of the
  N-A playback-mark commit.)
- **Acceptance:** (1) Events are not silently dropped — block on send honoring `closeCh`, or
  surface an explicit overflow error. (2) No goroutine leak on close.

### 6. `54e273e6953` — Temporal context in system prompt · **correctness · M**

- **Behavior:** Inject a fresh **date + timezone** block, recomputed each turn, so
  long-running sessions don't reason with a stale date.
- **Go target:** `agent/agent.go:590` `buildSystemPromptWithMemories`.
- **Why it matters:** omniagent injects **no** date at all — the prompt has no
  date/time/timezone line (`agent.go:590-620`). (Upstream's subtlety of moving it out of the
  cached prefix only matters once prompt caching exists; building it fresh-per-turn from the
  start is correct regardless.)
- **Acceptance:** (1) Assembled prompt includes a temporal block with current date +
  timezone. (2) Recomputed at build time each turn (session spanning midnight updates).
  (3) Coarse date stamp, not precise HH:MM. (4) Configurable timezone, default UTC.
  (5) If caching lands later, block sits outside the cached prefix.

### 7. `a877fe1b32a` — Prevent duplicate concurrent cron runs · RMI-064 (narrowed) · **M**

- **Behavior:** An interval job whose execution outlasts its interval must not start a second
  concurrent run.
- **Go target:** `cron/scheduler.go:198-209` `checkSpecialJobs` + `executeJob`.
- **Why it matters:** `checkSpecialJobs` launches `go executeJob` whenever
  `now - LastRunAt >= interval`, but `LastRunAt` is written only at the *end* of
  `executeJob` (`scheduler.go:257-287`) — a long run is re-launched every tick. The
  in-memory `JobStatusRunning` set (`executeJob:238`) isn't consulted before launch. (The
  upstream commit's other two parts — stalled-slot reaper, heartbeat, file-lock canonical —
  are N-A: omniagent has none of those.)
- **Acceptance:** (1) No second concurrent execution while a prior run of the job ID is in
  flight (guard the in-flight set, set `LastRunAt` at start, or `cron.SkipIfStillRunning`).
  (2) One-time jobs fire exactly once. (3) A failing maintenance step never blocks future
  ticks (add regression test).

### 8. `8fefe62ba41` — Emit agent_end on all exit paths · enabling · **L**

- **Behavior:** Guaranteed single lifecycle emission on every terminal path (normal / error /
  abort), with abort-outranks-error classification.
- **Go target:** new `EventAgentEnd` in `hooks/event.go:28-49`; deferred finalize in the run
  loop `agent/agent.go:318-411`.
- **Why it matters:** no `agent_end`/after-turn event exists — the run loop returns via early
  `return` or max-iteration error with no finalization emit; abort is bare `ctx` cancellation.
  This event is **enabling substrate** several other behaviors (incl. RMI-061) would build on.
- **Acceptance:** (1) Introduce `agent_end` event + payload (messages, `success`, `error`,
  `durationMs`). (2) Emitted on every terminal path via `defer`. (3) On abort
  (`errors.Is(err, context.Canceled)`): `success=false`, empty `error`; abort outranks a
  concurrent error. (4) Timeout not reported as error when cause was abort. (5) Tests for
  normal/error/abort each emitting exactly one correctly-classified event.

### 9. `adf3178ae6c` — Expose MCP tool identity · RMI-068 · **M** (L with deniedBySession)

- **Behavior:** Effective-tools inventory entries gain `source` + `mcpServer` + `mcpToolName`
  (+ `deniedBySession` once 067 lands).
- **Go target:** `gateway/tools_rpc.go:273` `ToolInfo`/`ToolsListHandler`; MCP identity
  metadata originates in `skills/remote/mcp/skill.go`, surfaced via `agent/tools.go`.
- **Why it matters:** `ToolInfo` is name/description/parameters only; MCP servers are flat
  compiled skills with no server/tool-name metadata. No "effective tools" endpoint exists —
  extend `ToolsListHandler`.
- **Acceptance:** (1) MCP tools expose originating server + original tool name. (2) Listing
  includes `source:"mcp"` + identity fields. (3) [needs 067] denied MCP tools still listable
  with `deniedBySession:true`. (4) Non-MCP tools omit MCP fields.
- **Note:** criteria 1–2 are independent; criterion 3 depends on RMI-067.

### 10. `42e23c11e0f` — Per-turn tool narrowing in prompt hooks · RMI-066 · **L**

- **Behavior:** A before-turn hook can return a tool-allow set narrowing `req.Tools` for that
  turn only (omitted = unchanged; empty = drop optional; list = intersection).
- **Go target:** `agent/agent.go` `processInternal` tool-build (~310-333) + a **new
  synchronous, return-value-bearing hook stage** (`hooks/`, e.g. `hooks/prompt.go`).
- **Why it matters:** tools are resolved globally once (`agent.go:310` `GetTools()`); hooks
  are fire-and-forget (`EmitAsync`, return discarded — `hooks/registry.go:176-206`). This
  synchronous hook stage is **shared substrate** RMI-067 also needs.
- **Acceptance:** (1) Pre-turn hook narrows `req.Tools` without mutating the registry.
  (2) Omitted = unchanged; empty = drop optional; list = hook ∩ registered. (3) Per-turn
  (re-evaluated each iteration); later turn can widen. (4) Test: hook returns `["chart"]` →
  only `chart` sent.

### 11. `a37a5a65753` — Per-session tool overrides · RMI-067 · **L**

- **Behavior:** A session carries `ToolOverrides` (MCP servers on/off, per-tool deny, skills
  on/off, web-search on/off) applied to that session's tool set.
- **Go target:** `sessions/session.go` (new `ToolOverrides` struct + field); session-scoped
  tool resolution at `agent/agent.go:310`; MCP deny filter in `skills/remote/mcp/skill.go`;
  a gateway path to set overrides (`gateway/handlers.go`).
- **Why it matters:** `Session` has no overrides field; tools resolved globally, not per
  session; MCP `Tools()` returns all server tools with no deny filter.
- **Acceptance:** (1) `Session` persists `ToolOverrides` (round-trips through store JSON).
  (2) Disabled server / denied tool / disabled skill / web-search=false excluded for that
  session only. (3) Two concurrent sessions with different overrides get different tool sets.
  (4) Gateway path to set overrides.
- **Prereq:** session-scoped tool resolution (shared with 066); `mcpToolsDeny` needs 068's MCP
  identity. Sequence 066 → 067 → 068.

### 12. `73b5e7aab49` — Save memory on automatic session rollover · RMI-061 · **L**

- **Behavior:** When a session ends *automatically* (daily/idle), persist its context to
  memory (not just on manual reset).
- **Go target:** net-new: a rollover trigger in `sessions/` (e.g. `sessions/rollover.go`);
  a new `session.rollover` event in `hooks/event.go` carrying a `Reason`; a
  memory-persistence hook writing the ended session via omnimemory
  (`skills/memory/memory.go:276` `client.Add`).
- **Why it matters:** no rollover/auto-reset mechanism exists (sessions expire silently via
  TTL — `sessions/store.go:18-19`); no session-lifecycle end event; memory is tool-driven,
  never fired from a session event.
- **Acceptance:** (1) Auto rollover emits a lifecycle event with ended session + reason.
  (2) A subscribed hook writes ended-session context to omnimemory, tagged with reason, no
  double-write on manual clear. (3) Manual clears unchanged. (4) Test: one memory write per
  rollover with correct reason.
- **Prereq:** heavy — build auto-rollover + lifecycle event first (synergy with #8
  `agent_end`). Port the *behavior* (persist-on-auto-boundary), not the Markdown file layout.
  RMI-062 (timezone) rides on this as an S add-on.

### 13. `eee5e029e36` — Persist last-used session model as default · gap · **L (blocked)**

- **Behavior:** An admin-scoped session model change persists as the agent's sticky default,
  best-effort, skipped when config is immutable.
- **Go target:** `Model` field on `sessions.Session`; a gateway session-model-patch path;
  a config write-back for `agent.Config.Model` (`agent/agent.go:61`).
- **Blocked on:** omniagent has **no per-session model selection at all** (`Config.Model` is a
  static construction-time string; config is load-only — `config/loader.go`). Recommend a
  prerequisite RMI (per-session model selection + writable config) before this.

---

## N-A items (strike or rescope in PLAN.md)

Verified against the source — these target OpenClaw subsystems omniagent doesn't implement,
or fix defects that cannot occur in Go. Not gaps; annotate the plan so they stop reading as
unfinished parity work.

| Commit | RMI | Reason N-A |
|--------|-----|-----------|
| `d9525663852` | **060** | No `MEMORY.md` promotion/budget-compaction subsystem. (`context/` compaction is conversation-history summarization — different concept.) Blocked on porting the whole feature. |
| `82af1bf7d40` | **063** | No restart catch-up / missed-slot state machine; `UpdateJob` already recomputes next run from now. |
| `5496cff9653` | **065** | No cron failure-alert/notification subsystem to enrich. (`ExecutionResult.StartedAt` already exists; nothing consumes it.) |
| `9560e171855` | **069** | No inbound hook→agent-session gateway. The one external-trigger path (cron) is already implicitly persistent with no isolated mode. |
| `3f918d4b3ca` | **062** | Only refines the not-yet-ported session-memory handler. Rides on 061 as an S add-on. |
| `e5cee36b46c` | — | No async queued/coalesced session-sync path; `Save` is synchronous, no queue to retry. |
| `280c1627533` | — | No Quicksilver sideband subsystem. (16 MiB `SetReadLimit` already caps payload.) |
| `6b37d46b179` | — | No delegation/consult broker; the analogous function-call path is already single-flight. |
| `ad1dc602594` | — | Pure JS block-scoping hygiene; Go `switch` cases are independently scoped — moot. |
| `750dc999038` | — | No playback-mark/barge-in subsystem. (But see the adjacent readLoop-drop PORT item.) |
| `29497b4e1e8` | — | UTF-16 `.slice` surrogate split can't occur on Go UTF-8 strings; no run-ID masking exists. |
| `46ec7a43a8a` | — | No bundled static model catalog / plugin-metadata snapshot; models are direct config-string passthrough. |

---

## Recommended RMI changes

**Strike / rescope (4 N-A + 1 rider):**

- RMI-060, 063, 065, 069 → mark **N-A / blocked** with the reason above (or rescope 060/065
  to include building their prerequisite subsystem, which is much larger than a port).
- RMI-062 → fold into RMI-061 as an acceptance criterion.

**Keep, rescope in place (3):**

- RMI-064 → narrow to "prevent duplicate concurrent cron runs" (drop the stalled-slot /
  heartbeat language — N-A).
- RMI-066, 067, 068 → keep, but add the shared-substrate prereq (synchronous return-value
  hook stage + session-scoped tool resolution) and the sequence 066 → 067 → 068.

**New RMIs to file (the real gaps the plan missed):**

- **[security]** cron fail-closed on removed account (`cee6203f448`, L) — highest priority.
- **[security]** auth-failure escalating delay (`ea967be0df6`, M).
- **[bug]** pre-tool assistant text drop (`c661061d4a7`, M).
- **[bug]** session cache release on Close (`c63fdc631f0`, S).
- **[bug]** realtime readLoop silent event drop (`omni-openai`, M).
- **[correctness]** temporal context in system prompt (`54e273e6953`, M).
- **[enabling]** `agent_end` lifecycle event (`8fefe62ba41`, L) — substrate for 061.
- **[feature, blocked]** per-session model selection → then sticky default (`eee5e029e36`, L).

## Sequencing / dependencies

```
security/bugs (independent, do first):
  cee6203f448  c661061d4a7  c63fdc631f0  ea967be0df6  readLoop-drop  54e273e6953  a877fe1b32a(064)

lifecycle chain:
  8fefe62ba41 (agent_end event) ──▶ 73b5e7aab49 (061 rollover memory) ──▶ 3f918d4b3ca (062 tz)

tool-scoping chain (shared substrate: sync hook stage + per-session tool resolution):
  42e23c11e0f (066) ──▶ a37a5a65753 (067) ──▶ adf3178ae6c (068 deniedBySession)
                                              adf3178ae6c (068 MCP identity)  ── independent

model chain:
  [new] per-session model selection ──▶ eee5e029e36 (sticky default)
```
