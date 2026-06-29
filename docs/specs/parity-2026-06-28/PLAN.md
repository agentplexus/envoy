# OpenClaw Sync Plan: 2026-06-28

Sync OmniAgent with OpenClaw changes from `bd96e4d2` to `2001b15f`.

## Summary

| Metric | Value |
|--------|-------|
| Previous sync | `bd96e4d22dafe64f558fb7f3ba5977aa3a93aee6` (2026-06-10) |
| Target sync | `2001b15f5b92d653464cbd847c28c136bdb465a7` (2026-06-28) |
| Commits | 4,111 |
| Features | 89 |
| Fixes | 2,444 |
| Files changed | 7,493 |

## Priority Categories

### P0: Security (Critical)

These security fixes should be evaluated and ported immediately.

| Commit | Description | OmniAgent Impact |
|--------|-------------|------------------|
| `f37e45ecc1` | Prototype pollution guard in resolveConfigPath | Review config path resolution |
| `26f92a4f8d` | Replace SHA-1 with SHA-256 for config fingerprinting | Audit crypto usage |
| `2001b15f5b` | Bound Drive document export reads (OOM) | Apply bounded reads pattern |
| `1bccd29304` | Bound SSE buffer to prevent OOM | Review SSE streaming |
| `2d2a50c00d` | Bound Discord REST response body (OOM) | Apply to HTTP clients |
| `a07d59e014` | Bound Telegram Bot API response reads (OOM) | Apply to channel clients |
| `39dc92efb7` | Kill timed out exec process trees | Review sandbox timeout handling |
| `12c34fc3a9` | Bound trusted package URL prefixes | Review package installation |

**Action items:**

1. Audit all HTTP response body reads - add `io.LimitReader` bounds
2. Audit config path resolution for path traversal
3. Verify SHA-256 usage for fingerprinting (Go already uses SHA-256)
4. Review exec timeout process cleanup

### P1: Core Agent/Memory

| Commit | Description | OmniAgent Module |
|--------|-------------|------------------|
| `b1ef12055e` | Preserve compactionSummary in limitHistoryTurns | `agent/` |
| `5a7857dc18` | Trace compaction summarization model calls | `agent/` |
| `cd31cbcd17` | Don't run LLM reranker in vsearch/search modes | `omniretrieve` |
| `94e6255666` | Apply outputDimensionality truncation to GGUF | `omniretrieve` |
| `414c250af9` | Memory store relocation with migration warning | `sessions/` |

### P2: Channel Improvements

UTF-16 boundary handling (important for multi-byte chars):

| Commit | Scope | Description |
|--------|-------|-------------|
| `4109755592` | Discord | Truncate model picker on UTF-16 boundary |
| `352f47f888` | iMessage | Coalesce merged text on UTF-16 boundary |
| `1841c4caf5` | Feishu | Truncate comment prompt on UTF-16 boundary |
| `e8d7b1feaf` | Slack | Truncate rich text preview on UTF-16 boundary |
| `686a2876c7` | Cron | Truncate failure alert on UTF-16 boundary |
| `78029e43d2` | Shared | UTF-16 safe truncation in subagent display |
| `19617b0b45` | Agent | Avoid splitting surrogate pairs in grep |

**Action:** Create shared `utf16.TruncateSafe()` utility in Go.

Channel-specific fixes:

| Commit | Channel | Description |
|--------|---------|-------------|
| `b28fcfe843` | Telegram | Fall back to plain text on rich validation fail |
| `46434f0c71` | Telegram | Reject surrogate/out-of-range HTML entities |
| `69b0604f31` | Telegram | Expose sender bot status in context |
| `39e1be080c` | Slack | Support alternate Web API roots |
| `341ae21d03` | Slack | Handle global and message shortcuts |
| `8f31b3218f` | Discord | Fix gzip response parsing |
| `0647260f87` | LINE | Load accounts.default, default-enable named accounts |
| `13ecb5c55e` | Mattermost | Persist participated threads |

### P3: Plugin System

| Commit | Description | OmniAgent Equivalent |
|--------|-------------|---------------------|
| `c5d34c8376` | Always plugin approval mode | `hooks/` approval modes |
| `e82d19fb06` | Auto plugin approvals | `hooks/` auto-approve |
| `bc5081c587` | Remote app-server plugins | Out of scope (Go plugins) |
| `bd479958c0` | Extensible channel identity hook | `hooks/` context |
| `b86b891326` | Emit security events for installs | `hooks/` audit events |
| `10a4c7c10b` | Surface plugin health | `gateway/` health |

### P4: Provider Updates

| Commit | Provider | Description |
|--------|----------|-------------|
| `85d5d94519` | Cohere | Add provider plugin |
| `735f59af73` | GLM | GLM-5.2 support |
| `95dafc824e` | ClawRouter | Managed proxy |
| `bfffc77bfc` | Copilot | BYOK provider parity |
| `84bcd500c9` | xAI | OAuth device-code flow |
| `a083c76621` | Bedrock | Honor adaptive model max tokens |

**OmniAgent actions:**

- [ ] Add Cohere provider to `omnillm-core`
- [ ] Add GLM-5.2 models to `omnillm-core`
- [ ] Review Bedrock max_tokens handling

### P5: CLI/UX Features

| Commit | Description | Priority |
|--------|-------------|----------|
| `a0fedcfb7e` | Add --message-file to agent CLI | Medium |
| `e35e5f123d` | Add `sessions compact` command | Medium |
| `b48238aa88` | Add /name command to rename session | Low |
| `75af913ba6` | Scope usage-cost by agent | Medium |
| `affd7471b3` | Daily Token Usage / Daily Cost chart | Low |

### P6: Cron Improvements

| Commit | Description |
|--------|-------------|
| `ca1a53aca4` | Add compact list responses |
| `e99f254f1c` | Treat exact-second slots as valid in stale-future repair |
| `30b39c6eab` | Guard against undefined sourceDelivery |

### P7: Workboard

| Files | Lines Changed |
|-------|---------------|
| `workboard.ts` | +1,716 |
| `workboard.test.ts` | +2,115 |

The workboard module has significant enhancements. Review for `omniworkboard` parity.

## Out of Scope

The following are not applicable to OmniAgent's Go architecture:

- iOS/Android app changes
- TypeScript-specific refactors
- Node.js plugin SDK changes
- QA Lab test infrastructure
- E2E test harness changes

## Implementation Plan

### Phase 1: Security Hardening (Week 1)

1. **Bounded HTTP reads**
   - Create `httputil.LimitedBody(r io.Reader, n int64) io.Reader`
   - Apply to all external HTTP clients
   - Add tests for OOM scenarios

2. **Config security**
   - Audit path resolution for traversal
   - Verify crypto uses SHA-256

3. **Exec timeout cleanup**
   - Review sandbox process tree killing
   - Ensure orphan processes are cleaned up

### Phase 2: UTF-16 Safety (Week 1)

1. Create `unicode/utf16util` package:
   ```go
   func TruncateSafe(s string, maxCodeUnits int) string
   func SplitSafe(s string, maxCodeUnits int) []string
   ```

2. Apply to:
   - Channel message truncation
   - Error message formatting
   - Subagent display text

### Phase 3: Core Agent/Memory (Week 2)

1. Compaction improvements
   - Preserve summaries in history limiting
   - Trace summarization calls

2. Memory search modes
   - Skip LLM reranker in vsearch mode
   - Dimension truncation for GGUF embeddings

### Phase 4: Channel Updates (Week 2-3)

1. Telegram enhancements
2. Slack Web API root configuration
3. Discord response parsing fixes

### Phase 5: Provider Updates (Week 3)

1. Cohere provider (omnillm-core)
2. GLM-5.2 models
3. Bedrock max_tokens fix

### Phase 6: CLI/UX (Week 4)

1. `--message-file` flag
2. `sessions compact` command
3. Usage scoping by agent

## Verification

```bash
# After implementation, run:
golangci-lint run ./...
go test -v ./...

# Integration test with channels
omniagent gateway run --config test/integration.yaml
```

## Files to Update

| File | Changes |
|------|---------|
| `PLAN.md` | Update sync history table |
| `go.mod` | Any new dependencies |
| `agent/*.go` | Compaction, history limiting |
| `channels/**/*.go` | UTF-16 truncation |
| `gateway/http.go` | Bounded reads |
| `sandbox/*.go` | Timeout cleanup |

## Notes

- OpenClaw is TypeScript; changes require Go reimplementation
- Focus on behavior parity, not code parity
- Security fixes are highest priority
- UTF-16 handling important for international users
