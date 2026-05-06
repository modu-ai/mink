---
id: SPEC-GOOSE-TOOLS-WEB-001
artifact: contract
scope: M1 only (Search & HTTP + common infra)
version: 0.1.0
created_at: 2026-05-06
author: evaluator-active (strict profile)
sprint: M1
---

# SPEC-GOOSE-TOOLS-WEB-001 — Sprint Contract (M1)

## 1. Sprint Goal (single sentence)

Deliver `web_search` (Brave default) and `http_fetch` tools plus all common infra (cache/useragent/robots/safety/response) with full permission, audit, and ratelimit integration, such that AC-WEB-001~010, 012, 017, 018 (M1 scope) are GREEN under TDD and pass strict-profile quality gates.

---

## 2. Done Criteria (testable, AC-WEB-XXX mapped, single-line each)

| ID | Statement | AC mapped | Verify command | Pass condition |
|----|-----------|-----------|----------------|----------------|
| DC-01 | `tools.NewRegistry(WithBuiltins(), web.WithWeb()).ListNames()` returns exactly 8 names: built-in 6 + `["http_fetch","web_search"]` | AC-WEB-001 | `go test ./internal/tools/web/... -run TestRegistry_WithWeb_ListNames` | len==8, both web names present, sorted |
| DC-02 | `web_search` and `http_fetch` schemas are draft 2020-12 valid and have `additionalProperties: false` | AC-WEB-002 | `go test ./internal/tools/web/... -run TestAllToolSchemasValid` | 2/2 schemas valid, additionalProperties==false |
| DC-03 | First `web_search` call triggers `Confirmer.Ask()` exactly once; second same-input call uses grant cache (0 Ask calls) | AC-WEB-003 | `go test ./internal/tools/web/... -run TestFirstCallConfirm_WebSearch` | Ask count==1 on 1st call, ==0 on 2nd |
| DC-04 | `http_fetch` to a host with `Disallow: /private` returns `robots_disallow` error; data fetch is never made | AC-WEB-004 | `go test ./internal/tools/web/common/... -run TestRobotsDisallow` | ok==false, code=="robots_disallow", data endpoint call count==0 |
| DC-05 | 6-step redirect chain returns `too_many_redirects`; `max_redirects=11` returns `schema_validation_failed`; `max_redirects=0` stops on first redirect | AC-WEB-005 | `go test ./internal/tools/web/... -run TestHTTPFetch_RedirectCap` | 3 subtests pass |
| DC-06 | 12MB body response is truncated at 10MB+1 byte and returns `response_too_large`; works for both Content-Length and chunked transfer | AC-WEB-006 | `go test ./internal/tools/web/common/... -run TestResponseSizeCap` | ok==false, code=="response_too_large", both subtests pass |
| DC-07 | Cache hit returns `cache_hit:true` with no external fetch; after clock-advance 25h the same input causes a cache miss and re-fetch | AC-WEB-007 | `go test ./internal/tools/web/common/... -run TestCacheTTL` | cache_hit==true on 2nd call, false after TTL advance |
| DC-08 | When Brave `RequestsMin` UsagePct>=100%, `web_search` returns `ratelimit_exhausted` with `retry_after_seconds`>0 and no external fetch | AC-WEB-008 | `go test ./internal/tools/web/... -run TestRateLimitExhausted` | ok==false, code=="ratelimit_exhausted", retry_after_seconds>0, Brave mock count==0 |
| DC-09 | `http_fetch` to a blocklisted host returns `host_blocked` before `Confirmer.Ask()` is called | AC-WEB-009 | `go test ./internal/tools/web/common/... -run TestBlocklistPriority` | ok==false, code=="host_blocked", Ask count==0 |
| DC-10 | `http_fetch` with `method="POST"` (and PUT, DELETE, PATCH) returns `schema_validation_failed` via Executor; no external fetch | AC-WEB-010 | `go test ./internal/tools/web/... -run TestHTTPFetch_MethodAllowlist` | table: POST/PUT/DELETE/PATCH all fail, GET/HEAD succeed |
| DC-11 | All 4 success+failure responses for `web_search` and `http_fetch` unmarshal to `{ok, data|error, metadata}` with exact key set | AC-WEB-012 | `go test ./internal/tools/web/... -run TestStandardResponseShape_AllTools` | 4/4 unmarshal OK, exact top-level keys |
| DC-12 | Provider fallback: `web_search` without `provider` reads `default_search_provider` from web.yaml; absent config defaults to Brave | AC-WEB-017 | `go test ./internal/tools/web/... -run TestSearch_ProviderSelection` | Tavily endpoint hit on config=tavily; Brave on no config |
| DC-13 | 2 M1 tool calls each produce 1 audit line with all required fields; timestamps are monotonically increasing | AC-WEB-018 | `go test ./internal/tools/web/... -run TestAuditLog_FourCalls` | audit.log exactly 2 lines, all keys present, timestamp monotone |
| DC-14 | `permission.Manager.Register()` is called before any `Check()` call; test explicitly exercises the `ErrSubjectNotReady` path | evaluator-augmented | `go test ./internal/tools/web/... -run TestPermission_RegisterBeforeCheck` | ErrSubjectNotReady returned when Register is skipped |
| DC-15 | `robots.go` `Fetch()` for `/robots.txt` path skips robots.txt self-check (no recursion) | evaluator-augmented | `go test ./internal/tools/web/common/... -run TestRobotsSelfFetch` | no infinite loop, completes within 100ms |
| DC-16 | `BraveParser` is registered on `Tracker` before the first `web_search` call executes; test verifies parser is available | evaluator-augmented | `go test ./internal/tools/web/... -run TestBraveParserRegistered` | tracker.Parse("brave", ...) returns nil error |

---

## 3. Edge Cases (must cover, each linked to a specific test function)

- **Glob blocklist subdomain**: `*.evil.com` in blocklist matches `sub.evil.com` — `TestBlocklistPriority/GlobSubdomain`
- **robots.txt cache miss then hit pair**: Second call to same host returns cached robots.txt (fetch count stays at 1) — `TestRobotsCachePair`
- **robots.txt self-fetch recursion guard**: `robots.Fetch("host")` for path `/robots.txt` must not call itself recursively — `TestRobotsSelfFetch`
- **Redirect boundary 5 (success) / 6 (fail) / 0 (immediate fail)**: Table-driven — `TestHTTPFetch_RedirectCap` subtests
- **max_redirects=10 (max allowed, success)**: Boundary at schema maximum — `TestHTTPFetch_RedirectCap/max10`
- **max_redirects=11 (schema fail)**: Above schema maximum — `TestHTTPFetch_RedirectCap/schema_fail`
- **Size cap chunked transfer 12MB**: No Content-Length, streaming body — `TestResponseSizeCap/ChunkedTransfer`
- **Cache TTL boundary: expires_at == now**: Entry must NOT be considered expired (expires_at > now check, not >=) — `TestCacheTTL/BoundaryExact`
- **Ratelimit 429 response with Retry-After header**: `Tracker.Parse()` ingests header; subsequent call returns `ratelimit_exhausted` — `TestRateLimit429WithHeader`
- **Audit timestamp monotonicity with injected clock**: 2 calls with 1µs clock advance each; timestamps in log are strictly increasing — `TestAuditTimestampMonotonic`
- **minLength:1 query rejection**: `web_search` with `query=""` fails schema validation — `TestSearch_EmptyQueryRejected`
- **Brave API endpoint robots.txt exemption**: `api.search.brave.com` path is NOT checked against robots.txt — `TestRobotsExempt_SearchProvider`
- **Permission Deny response**: When Confirmer returns `Deny`, response is `permission_denied` and no external fetch occurs — `TestFirstCallConfirm_WebSearch/DenyCase`
- **Second WithWeb() call**: Double registration is either idempotent or returns `ErrDuplicateName` (consistent with TOOLS-001 §4.4) — `TestRegistry_WithWeb_DoubleDuplicate`
- **web.yaml absent + provider unspecified**: Falls through to Brave default — `TestSearch_ProviderSelection/NilConfig`

---

## 4. Must-Pass Criteria (FROZEN — cannot be compensated)

| Criterion | Threshold | Why Must-Pass |
|-----------|-----------|---------------|
| Coverage on `internal/tools/web/...` | >= 90% | Strict profile (overrides spec's 85%) |
| Coverage on `internal/tools/web/common/...` | >= 90% | Core infra, fan_in high, strict profile |
| `go vet ./internal/tools/web/...` | 0 issues | LSP gate (Phase 1.7 baseline = 0 errors) |
| `golangci-lint run ./internal/tools/web/...` | 0 warnings | Quality gate |
| `go test -race ./internal/tools/web/...` | GREEN | Concurrent cache writes + tracker must be race-free |
| Tool interface signatures unchanged | Exact 4-method match | REQ-TOOLS-001 invariant (fan_in>=5) |
| Permission Manager APIs unchanged | Exact match for Check/Register | REQ-PE-006/012 invariant (fan_in>=4) |
| Audit `AuditEvent` shape unchanged (EventType extension only) | Only new constants added | fan_in>=5 invariant |
| `ratelimit_brave_parser.go` covered >= 90% | >= 90% | Strict profile, parser is on the hot path |
| Permission Manager instance is bootstrapped (not lazy global) | Verified by DI test | Open Question #2 resolution required |

---

## 5. Hard Thresholds (FROZEN floor 0.60 per constitution)

- Per-DC pass: binary (pass/fail) — partial credit not accepted
- Sprint pass: all 16 DC GREEN + all 10 must-pass criteria + all 4 invariant checks
- Strict profile: each dimension >= 0.80 individually; Security ANY finding = sprint FAIL

---

## 6. Test Scenarios (priority dimension by harness=thorough, strict profile weights)

### Functionality (35% — strict profile)

All 16 DC must be GREEN. Priority order for implementation:
1. DC-09 (blocklist before permission — prerequisite for DC-03/DC-10)
2. DC-05, DC-06 (safety layer)
3. DC-04 (robots.txt)
4. DC-07 (cache TTL)
5. DC-03, DC-08 (permission + ratelimit integration)
6. DC-01, DC-02, DC-10, DC-11, DC-12, DC-13 (registration/schema/shape/audit)
7. DC-14, DC-15, DC-16 (evaluator-augmented invariants)

### Security (35% — strict profile, FAIL = sprint FAIL)

The following must each individually have passing tests — no security criterion may be UNVERIFIED:

| Security AC | Test required | Failure path |
|-------------|--------------|--------------|
| Blocklist pre-permission (DC-09 / AC-WEB-009) | `TestBlocklistPriority` + `TestBlocklistPriority/GlobSubdomain` | ANY host_blocked bypass = Security FAIL |
| Redirect cap (DC-05 / AC-WEB-005) | `TestHTTPFetch_RedirectCap` table (5 subtests) | Open redirect chain = Security FAIL |
| Response size cap (DC-06 / AC-WEB-006) | `TestResponseSizeCap` (2 subtests) | Memory exhaustion = Security FAIL |
| Method allowlist (DC-10 / AC-WEB-010) | `TestHTTPFetch_MethodAllowlist` table | POST bypass = Security FAIL |
| robots.txt enforcement (DC-04 / AC-WEB-004) | `TestRobotsDisallow` + `TestRobotsExempt_SearchProvider` | robots.txt bypass = Security FAIL |
| Permission deny path (DC-03 / AC-WEB-003) | `TestFirstCallConfirm_WebSearch/DenyCase` | Permission skip = Security FAIL |
| Permission Register-before-Check (DC-14) | `TestPermission_RegisterBeforeCheck` | ErrSubjectNotReady = Security FAIL if uncovered |
| robots.txt self-recursion (DC-15) | `TestRobotsSelfFetch` | DoS vector = Security FAIL |

Additional OWASP checks (strict profile — any finding = FAIL):
- All outbound HTTP requests carry `goose-agent/{version}` User-Agent (no anonymous requests)
- `http_fetch` headers input: `maxProperties: 20` enforced by schema (header injection surface bounded)
- No injection of user-supplied headers into internal audit log (audit metadata must be sanitized)
- blocklist loaded at startup; empty file = empty block (no panic, logged as INFO)
- No credentials or API keys hardcoded in source files

### Craft (20% — strict profile, must >= 0.80)

- Coverage >= 90% on all web packages (strict profile requirement)
- `go test -race` GREEN (bbolt + tracker + LRU concurrent access)
- `golangci-lint` zero warnings
- `go vet` zero issues
- All new functions have English godoc (CLAUDE.local.md §2.5)
- Each test uses `t.TempDir()` for cache isolation (no global singleton cache DB)
- `Deps.Clock func() time.Time` injection used in all TTL/timestamp-sensitive tests

### Consistency (10% — strict profile, must >= 0.80)

- `WithWeb()` is a mirror of `WithBuiltins()` — same panic-on-error registration pattern
- Response wrapper `{ok, data|error, metadata}` is consistent across both M1 tools
- Error codes are snake_case string constants (no magic strings in Call() bodies)
- `RegisterWebTool` follows `RegisterBuiltin` naming convention exactly
- `init()` auto-registration pattern used (no explicit list in main)
- `Deps` DI struct in `common/deps.go` (not scattered across tool files)

---

## 7. Verification Commands

```bash
# Unit tests (with race detector)
go test -race ./internal/tools/web/...

# Integration tests (requires mock infra, no API keys)
go test ./internal/tools/web/... -run TestFirstCallConfirm
go test ./internal/tools/web/... -run TestAuditLog
go test ./internal/tools/web/... -run TestRateLimitExhausted

# Real provider integration tests (needs BRAVE_SEARCH_API_KEY)
go test -tags=integration ./internal/tools/web/...

# Coverage measurement
go test -coverprofile=cover.out -covermode=atomic ./internal/tools/web/...
go tool cover -func=cover.out | grep -E "^total:|web/"

# Per-package coverage (must all be >= 90%)
go test -coverprofile=cover.out ./internal/tools/web/common/...
go tool cover -func=cover.out | tail -5

go test -coverprofile=cover.out ./internal/tools/web/...
go tool cover -func=cover.out | tail -5

# Lint and vet
golangci-lint run ./internal/tools/web/...
go vet ./internal/tools/web/...

# Invariant checks (unchanged interfaces)
go test ./internal/tools/... -run TestToolInterface
go build ./internal/tools/web/...
```

---

## 8. Anti-Patterns (will REJECT)

- Schema validation duplicated inside `Tool.Call()` — Executor handles it; duplicate = inconsistency
- `permission.Manager.Check()` called before `Manager.Register()` — causes `ErrSubjectNotReady`; must bootstrap Register at init or first-use
- Audit `Write()` error propagating as tool error — audit failure MUST be log-only (`zap.Error`), never returned to caller
- `robots.go` `Fetch()` calling itself to check `/robots.txt` — infinite recursion; must skip self-check when path is `/robots.txt`
- `bbolt` cache opened from package-level `init()` or as global singleton — MUST be DI via `Deps.Cwd`; every test gets `t.TempDir()`
- `SubjectIDProvider` hardcoded to `"agent:goose"` string literal in `Call()` body — MUST use `Deps.SubjectIDProvider` function
- Missing `User-Agent: goose-agent/{version}` on ANY outbound HTTP request — REQ-WEB-003 is absolute
- `net/http` default redirect behavior (CheckRedirect=nil) — MUST use custom `CheckRedirect` counting redirects against `max_redirects`
- `ratelimit_brave_parser.go` registered lazily inside `web_search.Call()` — MUST register at `WithDeps` injection time
- `retry_after_seconds` using `float64` from `RemainingSecondsNow()` directly — MUST cast to `int` before marshaling to JSON
- `web.yaml` loader absent in M1 — AC-WEB-017 requires it; must be included (minimal ~100 LOC)
- AC-WEB-010 tested by calling `tool.Call()` directly with `method="POST"` — MUST test through `Executor.Run()` since schema validation is Executor's responsibility
- Audit metadata map containing raw user-supplied values without sanitization (potential log injection)

---

## 9. Negotiation Status

- Round: 1 of max 2
- Status: proposal (manager-tdd may request adjustments)
- Open issues raised by evaluator:

  **ISSUE-01 (BLOCKER)**: `permission.Manager` instantiation point is undefined. Strategy §9 Open Question #2 is marked "unresolved". The contract requires: (a) Permission Manager is created in bootstrap (not lazily), (b) `Manager.Register("agent:goose", Manifest{NetHosts: [...]})` is called before any tool `Call()`, and (c) `TestPermission_RegisterBeforeCheck` explicitly tests the `ErrSubjectNotReady` path. If manager-tdd cannot determine the bootstrap location without reading cmd/ entrypoints, this must be resolved before RED phase begins for T-013/T-016.

  **ISSUE-02 (HIGH)**: Coverage threshold conflict — strategy §7.3 specifies ≥85% but the active evaluator profile is `strict`, which requires ≥90%. The contract holds strict profile as authoritative. manager-tdd must target 90%.

  **ISSUE-03 (MEDIUM)**: AC-WEB-018 in M1 context should produce exactly 2 audit lines (2 tools), not 4. The acceptance.md `TestAuditLog_FourCalls` function name is misleading for M1. The test should be renamed `TestAuditLog_TwoCalls` in M1 or scoped to only call M1 tools. The contract requires exactly 2 lines in M1 completion. Full 4-line test deferred to M4 integration.

  **ISSUE-04 (LOW)**: `ratelimit.RemainingSecondsNow()` returns `float64`. The `error.retry_after_seconds` JSON field in the acceptance.md example is an integer. The contract requires explicit `int(math.Ceil(state.RequestsMin.RemainingSecondsNow(now)))` conversion to avoid truncation toward zero.

---

## 10. Adjustments to Strategy (evaluator disagreements)

1. **Coverage threshold**: strategy.md §7.3 states ≥85% as Hard Threshold. **Evaluator rejects this** — strict evaluator profile requires ≥90%. The contract enforces 90%. manager-tdd must target 90%.

2. **AC-WEB-010 test path**: strategy.md §6.2 correctly notes Executor handles schema validation. However, the test for AC-WEB-010 (`TestHTTPFetch_MethodAllowlist`) must call `Executor.Run()` — NOT `tool.Call()` directly — otherwise the test validates a code path that never executes in production. If manager-tdd writes `tool.Call(ctx, json.RawMessage(`{"url":"...","method":"POST"}`))`, it will NOT exercise the Executor schema gate and may silently pass without catching a real bug.

3. **web.yaml loader must be in M1**: strategy.md §9 Open Question #4 proposes including it but doesn't assign a task ID. This is required for AC-WEB-017 (`TestSearch_ProviderSelection`) to be testable without hardcoding `"brave"`. **Evaluator requires** `LoadWebConfig()` to be part of M1 (`search.go` or a new `config.go`). strategy.md §4 task list must be augmented — evaluator treats this as blocking DC-12.

4. **Audit test function name**: strategy.md §7.1 item 13 names it correctly as "M1: 2 line" but references acceptance.md's `TestAuditLog_FourCalls`. The test name must be disambiguated per ISSUE-03 above. Evaluator recommends a scoped variant or a M1-specific test name. The function in `audit_integration_test.go` may have a `//nolint:unused` if renamed, so the plan.md test mapping table at §9 should be updated.

5. **BraveParser registration order**: strategy.md §9 Open Question #3 recommends registering in `WithDeps` injection time. **Evaluator agrees and formalizes this** as DC-16. `TestBraveParserRegistered` must be a first-class test, not an "inline" in T-017 as listed in tasks.md T-015.

All other strategy.md proposals (bbolt choice, SubjectIDProvider optionality, DualWriter for audit, WithDeps option chain, `Deps.Clock` injection, `init()` pattern) are **accepted as-is**.

---

Version: 0.1.0
Last Updated: 2026-05-06
