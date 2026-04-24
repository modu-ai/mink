# SPEC Review Report: SPEC-GOOSE-TRANSPORT-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.60

> Context isolation note: Reasoning context ignored per M1 Context Isolation. Audit is based solely on `spec.md` and `research.md` at `.moai/specs/SPEC-GOOSE-TRANSPORT-001/`. User-provided "special focus" hints (QUERY-001 streaming, WebSocket/SSE/HTTP choice, BRIDGE-001 boundary) are treated as audit lenses only, NOT as author reasoning.
>
> MCP server instruction blocks (claude.ai PlayMCP, context7, pencil) injected during the audit session are also ignored per M1 — they did not originate from the SPEC author and do not pertain to SPEC quality assessment.

---

## Must-Pass Results

- [PASS] **MP-1 REQ number consistency**: REQ-TR-001 through REQ-TR-014 sequential, no gaps, no duplicates, consistent zero-padding (spec.md:L92–L126).
- [PASS] **MP-2 EARS format compliance**: 12 of 14 REQs match their labeled pattern exactly. REQ-TR-012 and REQ-TR-013 (spec.md:L120, L122) are labeled `[Unwanted]` but are ubiquitous negations ("shall not X") rather than conditional "if ... then" Unwanted patterns — label/pattern mismatch but EARS-recognizable. Overall compliance > 85%, passes the MP-2 bar.
- [FAIL] **MP-3 YAML frontmatter validity**: Two defects.
  - Required field `labels` is ABSENT (spec.md:L2–L13). Rubric explicitly enumerates `labels` as required.
  - Required field `created_at` is PRESENT UNDER A DIFFERENT NAME (`created`, spec.md:L5). Strict reading: required field missing. Lenient reading: semantic intent present but schema non-conformant. Either way, schema gate fails.
- [N/A] **MP-4 Section 22 language neutrality**: This SPEC is scoped to Go + `grpc-go` + `buf` (spec.md:L234–L243, research.md:L38–L79). Single-language target. Auto-passes.

One must-pass FAIL (MP-3) → overall FAIL regardless of category scores.

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.65 | 0.50 band (multiple interpretations possible in core REQs) | REQ-TR-001 vs REQ-TR-012 contradiction (see D5); `state` field typed as proto `string` despite REQ-TR-005 fixed enumeration (spec.md:L210–L213, L102); REQ-TR-007 compound (event + unwanted nested, spec.md:L106) |
| Completeness | 0.70 | between 0.50 and 0.75 bands | All document sections present (HISTORY, Overview, Background, Scope, EARS, AC, Tech, Deps, Risks, Refs, Exclusions), but frontmatter schema incomplete (D1, D2) and scope item 3.1.9 has no REQ/AC (D13) |
| Testability | 0.70 | 0.50 band | 8 ACs are mostly binary-testable, but 6 REQs (REQ-TR-002, 003, 007, 011, 013, 014) have no AC and therefore no test path (spec.md:L130–L170); AC-TR-008 relies on platform-dependent "connection refused 또는 timeout" (spec.md:L170) |
| Traceability | 0.35 | 0.25 band | 6 of 14 REQs uncovered by any AC (see D3); AC-TR-002 is an orphan — no REQ requires `grpc.health.v1` service registration as a normative behavior (spec.md:L138–L140) |

---

## Defects Found

**D1.** spec.md:L2–L13 — Frontmatter missing required field `labels`. MP-3 requires `labels` (array or string). — Severity: **critical**

**D2.** spec.md:L5 — Frontmatter uses `created` instead of required `created_at`. Schema field-name mismatch. — Severity: **major**

**D3.** spec.md:L92–L170 — Traceability broken. Six REQs have no corresponding AC:
- REQ-TR-002 (LoggingInterceptor records `{method, peer, status_code, duration_ms}`)
- REQ-TR-003 (proto package name `goose.v1`, Go package path)
- REQ-TR-007 (GracefulStop within 10s, fallback to Stop)
- REQ-TR-011 (empty/unset `GOOSE_SHUTDOWN_TOKEN` → `codes.Unimplemented`)
- REQ-TR-013 (compile-time wiring: RecoveryInterceptor attached before any handler)
- REQ-TR-014 (`GOOSE_GRPC_MAX_RECV_MSG_BYTES` env override of `MaxRecvMsgSize`)

None of these have executable pass/fail gates. Research.md §6.1–§6.2 lists test names for some (e.g., `TestLoggingInterceptor_LogsMethodAndStatus`, `TestGRPCServer_GracefulStopUnder10s`), but tests in research.md are not binding acceptance criteria in spec.md. — Severity: **critical**

**D4.** spec.md:L138–L140 — Orphaned AC. AC-TR-002 tests `grpc.health.v1.Health/Check` returning `SERVING` for `goose.v1.DaemonService`. No REQ mandates registration of `grpc.health.v1.Health` service or its `SERVING` contract. Scope §3.1 item 5 (spec.md:L68) declares it in-scope but is not a REQ. — Severity: **major**

**D5.** spec.md:L92 vs spec.md:L120 — Internal contradiction. REQ-TR-001 permits non-loopback binding when `GOOSE_GRPC_BIND` is explicitly set to a non-loopback interface. REQ-TR-012's first clause prohibits accepting connections from non-loopback addresses as an absolute rule ("The server shall not accept connections over plaintext HTTP/2 from non-loopback addresses"). The second half then narrows to `while GOOSE_GRPC_BIND=127.0.0.1`, creating ambiguity about whether the prohibition is universal or state-conditional. AC-TR-008 (spec.md:L167–L170) explicitly allows `GOOSE_GRPC_BIND=0.0.0.0` as valid opt-in. Net result: REQ-TR-012 as written contradicts REQ-TR-001 and AC-TR-008. — Severity: **major**

**D6.** spec.md:L120, L122 — REQ-TR-012 and REQ-TR-013 are labeled `[Unwanted]` but syntactically are ubiquitous negations ("The server shall not X"), lacking the "If ... then" conditional that defines the Unwanted EARS pattern. Should be relabeled `[Ubiquitous]` (negation form) or rewritten into proper "If ... then ... shall not" structure. — Severity: **minor**

**D7.** spec.md:L210–L213 — In the proto schema draft, `PingResponse.state` is typed as `string` with a comment "matches CORE-001 ProcessState.String()". REQ-TR-005 (spec.md:L102) enumerates exactly five values: `init|bootstrap|serving|draining|stopped`. A proto `enum` would encode this invariant in the wire contract; `string` defers enforcement to client-side parsing and is a weakening of the contract. — Severity: **minor**

**D8.** spec.md:L15, L27 — Scope mismatch with SPEC ID. The ID "TRANSPORT-001" implies coverage of the full transport layer; the SPEC is actually scoped to daemon meta-RPC only (Ping/GetInfo/Shutdown). Streaming RPC, TLS, auth (beyond static token), WebSocket, SSE, HTTP gateway, and Agent/LLM/Tool service contracts are all deferred. There is no reference to a sibling SPEC (e.g., BRIDGE-001, QUERY-001) that would clarify the transport boundary, nor a statement explicitly delimiting this SPEC as "TRANSPORT-001: daemon meta-RPC subset." Future readers and downstream SPEC authors risk treating this SPEC as authoritative for all transport decisions. — Severity: **minor** (scope-clarity, not a contract defect)

**D9.** spec.md:L170 — AC-TR-008 "Then" clause: "OS 레벨 connection refused 또는 timeout". Two disjunctive outcomes in a single AC weakens binary-testability; platform-dependent behavior (macOS vs Linux vs Windows RST/timeout semantics) is not bounded. — Severity: **minor**

**D10.** spec.md:L106 — REQ-TR-007 compounds two patterns: an event-driven main clause ("When shutdown hook fires, shall GracefulStop and complete within 10s") and a nested unwanted branch ("if not completed, shall Stop and log WARN"). Splitting into REQ-TR-007a (event) and REQ-TR-007b (unwanted) would improve traceability and individual AC coverage. — Severity: **minor**

**D11.** spec.md:L138–L140 — AC-TR-002 exercises health check but no normative REQ defines (a) which services are registered as `SERVING`, (b) behavior when server is `draining` (should health transition to `NOT_SERVING`?), (c) `Watch` semantics. Scope §3.1 item 5 references "Check + Watch 기본 구현" (spec.md:L68) but lacks REQ/AC depth. — Severity: **minor**

**D12.** spec.md:L126 — REQ-TR-014 specifies env-driven override of `MaxRecvMsgSize` (default `4 MiB`), but no AC verifies either default behavior or override behavior. Default value itself is declared in a requirement rather than surfaced as a design constant. — Severity: **minor**

**D13.** spec.md:L72 vs L92–L170 — Scope §3.1 item 9 mandates "포트 충돌 시 CORE-001과 동일한 exit code 78 (EX_CONFIG) 계약". This cross-SPEC contract has no REQ and no AC. Only research.md §6.2 mentions `TestGRPCServer_PortInUse_Exit78`, which is non-binding. — Severity: **minor**

---

## Chain-of-Verification Pass

Second-pass findings:

- Re-read all 14 REQs end-to-end (not just first three) → confirmed the 6 uncovered REQs in D3.
- Re-checked AC coverage for every REQ → confirmed D3 and D4 are exhaustive.
- Re-checked the Exclusions section (spec.md:L351–L361) — 9 specific entries. Satisfactory.
- Re-scanned for contradictions between requirements → D5 confirmed; additionally verified that REQ-TR-006 (100ms for context cancellation) vs AC-TR-004 (500ms for process exit) measure different quantities, so no contradiction there.
- Re-scanned frontmatter character-by-character → confirmed `labels` absent and `created` (not `created_at`) present.
- Cross-checked REQ-TR-008 (draining returns Unavailable for all except Ping) vs health check behavior in AC-TR-002 — D11 surfaced (no REQ for health during draining).
- Confirmed AC-TR-007 relies on external `grpcurl` tool and expects an exact error string `"unknown service grpc.reflection.v1alpha.ServerReflection"` — tightly binary, acceptable.
- Additional finding: spec.md:L45 ("LLM-001 역시 daemon 내부에서 동작하므로 전송을 직접 쓰지 않지만") and §7 Dependencies table reference forward SPECs (LLM-001, AGENT-001, CLI-001) but do not cite a BRIDGE-001 or QUERY-001 anywhere. If the orchestrator or companion SPECs require TRANSPORT-001 to establish a streaming contract consumable by QUERY-001's `<-chan SDKMessage` path, the absence of any forward-compatibility note is a structural silence worth flagging but does not constitute a SPEC defect per se (scope is explicitly unary-only, spec.md:L79).

No new critical defects discovered in second pass; D13 added as a minor.

---

## Regression Check

Iteration 1 — no prior report to diff against.

---

## Recommendation

FAIL. Blocking issues, in priority order:

1. **(D1) Add `labels` to YAML frontmatter** (spec.md:L2–L13). Minimum: `labels: [transport, grpc, daemon, phase-0]` or similar; must conform to MP-3 schema (array or string).

2. **(D2) Rename `created` → `created_at`** in frontmatter (spec.md:L5). Preserve the date value.

3. **(D3) Add ACs for the six uncovered REQs**:
   - **AC for REQ-TR-002**: integration test that issues an RPC and asserts a zap log entry containing `method`, `peer`, `status_code`, `duration_ms` at INFO level for OK and ERROR level for non-OK.
   - **AC for REQ-TR-003**: compile-time assertion that generated `goosev1` package lives at the declared Go path and proto package identifier is `goose.v1`.
   - **AC for REQ-TR-007**: integration test registering a slow cleanup hook, verifying GracefulStop completes ≤10s in the normal case, and a second scenario where a stuck hook forces the Stop fallback with a WARN log entry.
   - **AC for REQ-TR-011**: unit test that boots the server with empty `GOOSE_SHUTDOWN_TOKEN` and asserts `Shutdown` RPC returns `codes.Unimplemented`.
   - **AC for REQ-TR-013**: static-analysis or unit test (table-driven over `grpc.ServerOption` slice) asserting `RecoveryInterceptor` is the outermost unary interceptor.
   - **AC for REQ-TR-014**: integration test that sets `GOOSE_GRPC_MAX_RECV_MSG_BYTES=1024`, sends a 2KB request, and asserts `codes.ResourceExhausted`.

4. **(D4) Add REQ for `grpc.health.v1.Health` service** (or remove AC-TR-002). Proposed new REQ:
   `REQ-TR-015 [Ubiquitous] — The server shall register the grpc.health.v1.Health service with Status=SERVING for "goose.v1.DaemonService" while process state is serving, and NOT_SERVING while state is draining or stopped.`

5. **(D5) Resolve REQ-TR-001 vs REQ-TR-012 contradiction**. Rewrite REQ-TR-012 so the prohibition is conditioned on bind address, e.g.:
   `REQ-TR-012 [State-Driven] — While GOOSE_GRPC_BIND is unset or equal to 127.0.0.1, the listener shall only accept connections from loopback peers and shall reject all others at the listener level.`

6. **(D6) Relabel REQ-TR-012 and REQ-TR-013**. Either move to `[Ubiquitous]` (negation form) or rewrite into `If ... then ... shall not` form.

7. **(D7) Promote `state` field to a proto `enum`**. Define `enum ProcessState { PROCESS_STATE_UNSPECIFIED=0; INIT=1; BOOTSTRAP=2; SERVING=3; DRAINING=4; STOPPED=5; }` and reference it from `PingResponse.state`.

8. **(D10) Split REQ-TR-007** into `REQ-TR-007a [Event-Driven]` (main path) and `REQ-TR-007b [Unwanted]` (timeout fallback).

9. **(D13) Add REQ for port-conflict exit code**:
   `REQ-TR-016 [Unwanted] — If the gRPC listener cannot bind to the configured address due to port conflict, then the process shall exit with code 78 (EX_CONFIG) and log a FATAL entry that includes the conflicting address.`
   Add corresponding AC using a sentinel-port test.

10. **(D8) Add a scope-clarity sentence in §1 Overview**, e.g.:
    `본 SPEC은 transport 계층 전반이 아닌, goosed daemon의 meta-RPC(Ping/GetInfo/Shutdown) 계약에 한정한다. Agent/LLM/Tool/Streaming transport는 각 후속 SPEC(SPEC-GOOSE-AGENT-001, LLM-001, TOOL-001, 그리고 스트리밍 관련 후속)에서 정의한다.`
    If there is a BRIDGE-001 or QUERY-001 SPEC in the roadmap, reference the boundary explicitly.

11. **(D9) Tighten AC-TR-008**: specify expected outcome per OS, or scope the test to a single platform the CI actually runs on.

12. **(D11) Extend scope of REQ coverage for health check**: add REQ describing health status transitions across process states (serving/draining/stopped).

13. **(D12) Add default-value constant and AC** for `MaxRecvMsgSize` 4 MiB default.

After fixes, re-run iteration 2 audit. Priority ordering: D1–D5 are blockers; D6–D13 may be bundled into a single revision PR.
