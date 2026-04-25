# SPEC Review Report: SPEC-GOOSE-CORE-001
Iteration: 2/3
Verdict: FAIL
Overall Score: 0.68

Reasoning context ignored per M1 Context Isolation.

---

## Must-Pass Results

- [PASS] **MP-1 REQ number consistency**: REQ-CORE-001..012 sequential, no gaps, no duplicates, consistent zero-padding (3-digit). Verified by enumeration at spec.md:L91, L93, L95, L99, L101, L103, L107, L109, L113, L115, L117, L121.
- [FAIL] **MP-2 EARS format compliance**: REQ-CORE-011 (spec.md:L117) is labeled `[Unwanted]` but the sentence structure is Ubiquitous (`The goosed process **shall not** write any log line at level DEBUG or below when GOOSE_LOG_LEVEL is set to info or higher.`). The Unwanted EARS pattern requires `If <undesired condition>, then the <system> shall <response>`. Either the label must change to `[Ubiquitous]` or the sentence must be restructured to `If GOOSE_LOG_LEVEL is set to info or higher, then the goosed process shall not write any log line at level DEBUG or below.` Carried over unchanged from iteration 1.
- [PASS] **MP-3 YAML frontmatter validity**: All required fields present at spec.md:L1-14 (id=string, version=string, status=string, created_at=ISO-date, priority=string `P0`, labels=array `[]`).
- [N/A] **MP-4 Section 22 language neutrality**: SPEC is Go-only, single-language scoped (spec.md:L52-55, L63 "Go 1.22+ 단일 바이너리"). Auto-passes.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band | Most REQ unambiguous; REQ-CORE-011 mismatch label vs sentence (spec.md:L117); §6.2 vs §10 R1 Go version contradiction (spec.md:L178 "Go 1.26" vs L305 "Go 1.22로 고정"). |
| Completeness | 0.50 | 0.50 band | All required sections present (HISTORY L18-22, WHY L38-55, WHAT L59-81, REQ L85-121, AC L125-157, Exclusions L335-345); but multiple REQ→AC traceability holes and §11 numbering still §10.1/10.2/10.3 (spec.md:L315, L322, L328). |
| Testability | 0.50 | 0.50 band | AC-CORE-001~006 binary-testable; but REQ-CORE-005 "50ms 이내" measurable but lacks corresponding assertion in any AC (spec.md:L101 vs L132); REQ-CORE-009 "stack trace 포함" not asserted in AC-CORE-005 (spec.md:L150-152); REQ-CORE-008/011/012 lack ACs entirely. |
| Traceability | 0.25 | 0.25 band | 3 of 12 REQ have zero AC coverage: REQ-CORE-008 (L109), REQ-CORE-011 (L117), REQ-CORE-012 (L121). 25% uncovered = poor traceability. Carried over unresolved from iteration 1. |

---

## Defects Found

### Carried over from iteration 1 (UNRESOLVED in spec.md)

D1. spec.md:L109 — REQ-CORE-008 `[State-Driven]` "drain 중 listener close" has no corresponding AC entry. Severity: **major** (traceability firewall).
D2. spec.md:L117 — REQ-CORE-011 `[Unwanted]` "DEBUG suppression" has no corresponding AC entry. Severity: **major**.
D3. spec.md:L121 — REQ-CORE-012 `[Optional]` "GOOSE_HEALTH_PORT override" has no corresponding AC entry. Severity: **major**.
D4. spec.md:L117 — REQ-CORE-011 EARS pattern mismatch: labeled `[Unwanted]` but sentence is Ubiquitous "shall not". MP-2 violation. Severity: **major**.
D5. spec.md:L101 vs L132 — REQ-CORE-005 specifies "within 50ms" but AC-CORE-001 has no timing assertion. Severity: **minor** (measurability gap).
D6. spec.md:L150-152 — AC-CORE-005 mentions "패닉 스택이 ERROR 로그에 포함됨" but lacks explicit assertion mechanism (e.g. log capture/inspection). Severity: **minor**.
D7. spec.md:L178 vs L305 — Go version self-contradiction: §6.2 says "go 1.26" while §10 R1 says "Go 1.22로 고정". Severity: **minor** (consistency).
D8. spec.md:L296 — §9 Dependencies still lists "Go 1.22+ toolchain", inconsistent with §6.2 "go 1.26". Severity: **minor**.
D9. spec.md:L95 — REQ-CORE-003 specifies "internal `atomic.Value`" but implementation uses `atomic.Int32` (verified at internal/core/state.go). SPEC text not realigned with code. Severity: **minor**.
D10. spec.md:L210 — §7.1 stipulates `cmd/goosed/main.go` is "15~30줄" but actual code is 110 LoC (verified at cmd/goosed/main.go). Severity: **minor** (SPEC unrealistic).
D11. spec.md:L315, L322, L328 — §11 References subsections are numbered `10.1`, `10.2`, `10.3` (orphan numbering). Severity: **minor**.
D12. spec.md:L188-199 — §6.3 Security Stack (Tier 1~5) and Phase 8/M5/M8 references are scope creep for Phase 0. Severity: **minor**.
D13. spec.md:L213 — §7.1 layout enumerates `internal/health/server_test.go` and `internal/config/bootstrap_config_test.go` but those files do NOT exist (verified `ls internal/health/`, `ls internal/config/`). Layout-vs-implementation drift. Severity: **minor**.
D14. spec.md:L137 — AC-CORE-002 requires "cleanup hook 3개가 모두 호출되었음이 로그로 확인됨" but main.go has no production hook registration site (verified at cmd/goosed/main.go:96 — `rt.Shutdown.RunAllHooks` called on empty hook list). AC unverifiable in current binary. Severity: **major**.
D15. spec.md:L150-152 — AC-CORE-005 expects exit code 1 from binary, but no binary-level test exists; only `internal/core/runtime_test.go` covers ShutdownManager unit-level. Severity: **major**.

### Newly identified in iteration 2

D16. spec.md:L4 — `status: planned` but the SPEC's M0 acceptance code is partially implemented and committed (B4-1, B4-2 fixes verified). Status should advance to `implementing` or `in_progress` to reflect Phase C1 progress. Severity: **minor** (lifecycle hygiene).
D17. spec.md:L13 — `labels: []` empty. Inconsistent with all other surveyed SPECs (e.g., adapter, router) which carry phase/area labels. Severity: **minor**.
D18. spec.md:L21-22 — HISTORY table has only 1 entry (0.1.0 / 2026-04-21). Phase C1 code fixes (B4-1 signal.NotifyContext, B4-2 parentCtx) are material clarifications of REQ-CORE-004 contract but not journaled. Severity: **minor** (HISTORY hygiene).

---

## Chain-of-Verification Pass

Second-look findings:

1. Re-read every REQ-CORE-001..012 — confirmed sequence integrity (D1-D3 traceability gaps stand).
2. Re-read every AC-CORE-001..006 — confirmed only 6 ACs exist; no new AC was added between iteration 1 (timestamp `Apr 22 21:27` for research.md, `Apr 24 16:15` for spec.md) and iteration 2. **However the spec.md mtime (Apr 24 16:15) is AFTER the audit (Apr 24 15:46) — yet the content for traceability defects D1-D3 is unchanged**. This means spec.md was edited (likely Phase C1 timestamp touch or unrelated edit) but the REQ↔AC gap was NOT addressed. Confirmed via diff: lines L107-L121, L125-L157 identical to iteration 1 content cited.
3. Verified Exclusions section (L335-345) has 7 specific entries — passes specificity check.
4. Re-verified spec.md §10 R1 (L305) text "Go 1.22로 고정" still contradicts §6.2 (L178) "go 1.26" — D7 unresolved.
5. Cross-referenced implementation: `cmd/goosed/main.go` at L53 contains `signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)` — B4-1 code fix CONFIRMED. `internal/core/shutdown.go` at L51-63 contains `select { case <-parentCtx.Done(): ... return }` parent-ctx cancellation watcher — B4-2 code fix CONFIRMED. `internal/core/runtime.go` at L23 exposes `RootCtx context.Context` — hook subscription path CONFIRMED.
6. Searched for new tests addressing flakiness (B5): `internal/core/runtime_test.go` contains `TestGoosedMain_SIGTERM_CancelsRootContext` (L378) and `TestRunAllHooks_ParentCtxCanceled_StopsIteration` (L442). Both pass under `go test -race -count=5`. Flakiness from iteration 1 (`TestSIGTERM_InvokesHooks_ExitZero`) NOT explicitly re-engineered to use `TestMain` build cache, but `-race -count=5` 240s timeout completed cleanly (`ok github.com/modu-ai/goose/internal/core 2.727s`).
7. No new defects discovered in second pass beyond D16-D18 already listed.

---

## Regression Check

Defects from iteration 1 (CORE-001-audit.md):

### Code-level (Should-Fix Major)

- **B4-1** (root context cancel via signal.NotifyContext) — **RESOLVED**. Evidence: `cmd/goosed/main.go:53` — `rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)`; `internal/core/runtime.go:23` exposes `RootCtx context.Context` for hook subscription; new test `TestGoosedMain_SIGTERM_CancelsRootContext` at `internal/core/runtime_test.go:383` passes 5x under `-race`.
- **B4-2** (RunAllHooks parentCtx 만료 감시) — **RESOLVED**. Evidence: `internal/core/shutdown.go:55-63` — `select { case <-parentCtx.Done(): m.logger.Warn("shutdown timeout: skipping remaining hooks", ...); return panicOccurred; default: }`; new test `TestRunAllHooks_ParentCtxCanceled_StopsIteration` at `internal/core/runtime_test.go:442` passes 5x under `-race`.
- **B5** (`TestSIGTERM_InvokesHooks_ExitZero` flakiness) — **PARTIALLY RESOLVED**. The test still uses per-test `buildGoosed`; `TestMain` consolidation NOT performed. However 5x `-race` runs (240s timeout) all PASS, suggesting flakiness has reduced to acceptable levels. Marking as **probably resolved**, but no structural fix applied.

### SPEC-level (Should-Fix Major / Could-Fix Minor)

- **A2** REQ-CORE-008 lacks AC — **UNRESOLVED**. spec.md:L109 still has no corresponding AC. Phase C2 Batch 5 did not touch spec.md content for traceability.
- **A2** REQ-CORE-011 lacks AC — **UNRESOLVED**. spec.md:L117 still has no AC.
- **A2** REQ-CORE-012 lacks AC — **UNRESOLVED**. spec.md:L121 still has no AC.
- **A1** REQ-CORE-011 EARS label mismatch — **UNRESOLVED**. spec.md:L117 still labeled `[Unwanted]` with Ubiquitous sentence.
- **A4** Go version contradiction (§6.2 vs §10 R1) — **UNRESOLVED**. spec.md:L178 vs L305.
- **A4** main.go LoC budget (15~30줄) — **UNRESOLVED** (now 110 LoC after Phase C1 expansion). spec.md:L210.
- **B3-3** atomic.Int32 vs atomic.Value mismatch — **UNRESOLVED**. spec.md:L95 still says `atomic.Value`.
- **B5** test file split (`server_test.go`, `bootstrap_config_test.go`) — **UNRESOLVED**. Both files still missing.
- **AC-CORE-005** stack trace assertion mechanism — **UNRESOLVED**. spec.md:L150-152 unchanged.

### Stagnation Detection

7 SPEC-level defects from iteration 1 carried forward unchanged into iteration 2. This indicates **Phase C2 Batch 5 did not perform SPEC document realignment for SPEC-GOOSE-CORE-001**. If iteration 3 also fails to address D1-D4 (the four major SPEC defects), this becomes a **blocking defect — manager-spec made no progress on SPEC textual fixes**.

---

## Recommendation

**FAIL — iteration 2 unresolved defects exceed pass threshold.**

Required SPEC fixes for iteration 3 (highest priority first):

1. **[major]** Add 3 new AC entries covering REQ-CORE-008, REQ-CORE-011, REQ-CORE-012:
   - **AC-CORE-007** (REQ-CORE-008): Given `goosed` is in `serving` state with active health listener; When `SIGTERM` is sent and process enters `draining`; Then a fresh `GET /healthz` connection attempt fails with `connection refused` (listener closed) within 100ms after draining state observed.
   - **AC-CORE-008** (REQ-CORE-011): Given `GOOSE_LOG_LEVEL=info`; When `goosed` runs and internal code emits `logger.Debug(...)` calls; Then stderr log capture contains zero JSON lines with `"level":"debug"`.
   - **AC-CORE-009** (REQ-CORE-012): Given `GOOSE_HEALTH_PORT=18999`; When `goosed` runs; Then `curl http://127.0.0.1:18999/healthz` returns 200 within 200ms while default port `:17890` is NOT bound by `goosed`.
2. **[major, MP-2]** Fix REQ-CORE-011 (spec.md:L117): change label to `[Ubiquitous]` (preferred — matches sentence) or rewrite as `**If** GOOSE_LOG_LEVEL is set to info or higher, **then** the goosed process **shall not** write any log line at level DEBUG or below.`
3. **[minor]** Reconcile Go version: replace spec.md:L178 `go 1.26` ↔ spec.md:L296 "Go 1.22+ toolchain" ↔ spec.md:L305 "Go 1.22로 고정". Recommended canonical: `Go 1.23+` (matches `.claude/rules/moai/languages/go.md` baseline) or align all three to the actual `go.mod` version.
4. **[minor]** Update spec.md:L95 to read `atomic.Int32` (matches implementation in `internal/core/state.go`).
5. **[minor]** Update spec.md:L210 main.go LoC budget from "15~30줄" to "≤120 LoC" (matches actual 110 LoC after Phase C1 root-ctx fix; or extract `run()` to separate package to restore tighter budget).
6. **[minor]** Renumber §11 References subsections from `10.1/10.2/10.3` to `11.1/11.2/11.3` (spec.md:L315, L322, L328).
7. **[minor]** Append HISTORY entry for Phase C1 code fixes:
   ```
   | 0.1.1 | 2026-04-24 | B4-1/B4-2 코드 수정 반영: signal.NotifyContext 도입, RunAllHooks parentCtx 만료 감시 추가 (REQ-CORE-004 b·c 절 강화) | manager-spec |
   ```
8. **[minor]** Either (a) move spec.md:L188-199 §6.3 Security Stack (Tier 1~5) to Appendix or external SPEC reference, or (b) add explicit "OUT OF SCOPE for Phase 0" header.
9. **[minor]** Either create `internal/health/server_test.go` and `internal/config/bootstrap_config_test.go` (and remove from §7.1 enumeration if not needed yet), or update spec.md:L213 to mark them as "to be added in next sprint".
10. **[minor]** Strengthen AC-CORE-005 (spec.md:L150-152): add explicit assertion `로그 캡처(zaptest/observer 등)에서 stack 필드가 'goroutine ' 패턴을 포함함`.

Iteration 2 recognizes that B4-1 and B4-2 **code fixes are correct and complete** (verified by `go test -race -count=5` passing). The FAIL verdict is driven entirely by **SPEC document drift**: the spec.md was not updated in Phase C2 Batch 5 to align with code reality and to close traceability gaps. If iteration 3 addresses items 1-2 (the four major defects D1-D4), overall verdict will move to PASS even if items 3-10 remain.

---

**End of CORE-001-review-2.md**
