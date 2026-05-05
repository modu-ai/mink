# SPEC Review Report: SPEC-GOOSE-OBS-METRICS-001
Iteration: 2/3
Verdict: **FAIL**
Overall Score: 0.55

Reasoning context ignored per M1 Context Isolation. This report is built only from `.moai/specs/SPEC-GOOSE-OBS-METRICS-001/spec.md` and the prior `.moai/specs/SPEC-GOOSE-OBS-METRICS-001/audit-2026-04-30.md` (regression-check input). Caller-provided narrative summary was disregarded.

Inputs read:
- `.moai/specs/SPEC-GOOSE-OBS-METRICS-001/spec.md` (608 lines, mtime 2026-04-30 17:37 — modified after the prior audit at 17:32)
- `.moai/specs/SPEC-GOOSE-OBS-METRICS-001/audit-2026-04-30.md` (51 lines, prior iteration)

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-OBS-METRICS-001 through REQ-OBS-METRICS-020 sequential, no gaps, no duplicates, consistent zero-padding (spec.md:L138, L140, L142, L144, L146, L150, L152, L154, L156, L160, L162, L164, L168, L174, L176, L178, L180, L184, L186, L188).
- **[PASS] MP-2 EARS format compliance**: All 20 REQs match one of the five EARS patterns.
  - Ubiquitous: REQ-001..005 (`shall expose`, `shall provide`, `is compatible`, `returns`, `preserves`).
  - Event-Driven: REQ-006..009 prefixed `WHEN ... is invoked/called/checked/registered` (L150, L152, L154, L156).
  - State-Driven: REQ-010..012 prefixed `WHILE` / `IF` (L160, L162, L164).
  - Unwanted: REQ-013..017 prohibitive verbs (L168 "contract 위반", L174 "변형하지 않는다", L176 "panic 을 발생시키지 않는다", L178 "lifecycle ... 요구하지 않는다", L180 "작성하지 않는다").
  - Optional: REQ-018..020 prefixed `WHERE` (L184, L186, L188).
- **[PASS] MP-3 YAML frontmatter validity**: id, version, status, created_at, priority, labels all present with correct types (spec.md:L2-L13). `created_at: 2026-04-30` is ISO-8601. `labels` is a YAML array with 5 entries.
- **[N/A] MP-4 Section 22 language neutrality**: SPEC is single-language scoped (Go-only, `internal/observability/metrics/...`). No multi-language tooling claims. Auto-pass.

---

## Category Scores (rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 (minor ambiguity in 1-2 places) | Most REQs are precise; REQ-OBS-METRICS-012 "본 SPEC 의 `Sink` interface 자체는 nil receiver 를 다루지 않는다" (spec.md:L164) is informational rather than testable, and is acknowledged as caller-responsibility. |
| Completeness | 1.00 | 1.00 (all sections present) | HISTORY (L20-L24), Overview (L28-L39), Background (L43-L72), Scope (L75-L128), EARS Requirements (L132-L188), AC (L192-L275), Data Model (L279-L424), Integration (L427-L511), NFR (L515-L529), Risks (L533-L542), Exclusions (L546-L562), Deliverables (L566-L581), References (L585-L601). All 6 frontmatter fields present. |
| Testability | 0.75 | 0.75 (one or two ACs require interpretation) | AC-001..AC-017 are binary-testable. AC-018 (spec.md:L240-L243) is partially deferred to consumer-SPEC implementation but now mandates option (b) contract_test.go (`[필수]` at L242). |
| **Traceability** | **0.25** | **0.25 (largely absent / actively wrong)** | The newly-added §5.7 REQ↔AC matrix at spec.md:L247-L268 contains **systematic REQ-number-to-description mislabels** that contradict the canonical §4 REQ definitions. See Defect D1 below. The matrix purports to resolve prior D1/D2 but introduces a traceability worse than absence. |

---

## Defects Found

**D1. spec.md:L247-L268 — §5.7 REQ↔AC 커버리지 매트릭스 systematic mislabel — Severity: critical.**

The matrix added in this iteration to resolve the prior D1/D2 (REQ↔AC mapping absence) misidentifies the topic and EARS category of REQ-OBS-METRICS-005 through REQ-OBS-METRICS-018. The matrix appears to be from a different REQ numbering version than the §4 normative requirements at spec.md:L138-L188. Concrete divergences:

| Matrix row (spec.md line) | Matrix says | Actual §4 definition | Match? |
|---|---|---|---|
| L249 REQ-001 | "Sink interface 3 메서드 \| Ubiquitous" | L138: `Sink, Counter, Histogram, Gauge` 4 interface + `Labels` type — Ubiquitous | partial topic mismatch (3 vs 4) |
| L250 REQ-002 | "Counter / Histogram / Gauge handle \| Ubiquitous" | L140: 3 factory methods on Sink — Ubiquitous | topic shifted |
| L251 REQ-003 | "Labels map[string]string \| Ubiquitous" | L142: consumer SPEC TELEMETRY-001 compatibility — Ubiquitous | wrong topic |
| L252 REQ-004 | "handle reuse \| Ubiquitous" | L144: handle reuse — Ubiquitous | OK |
| L253 REQ-005 | "Labels static — handle 생성 후 mutate 금지" | L146: Labels is map preserved as caller set — Ubiquitous | partial (mutate-after-creation is not in §4 REQ-005) |
| L254 REQ-006 | "Counter monotonic \| Event-Driven" | L150: WHEN Sink.Counter called — Event-Driven | OK |
| L255 REQ-007 | "Histogram bucket 분류 \| Event-Driven" | L152: WHEN Sink.Histogram called — Event-Driven | OK |
| L256 REQ-008 | "Gauge Set/Add \| Event-Driven" | L154: WHEN env GOOSE_METRICS_ENABLED checked — Event-Driven | **wrong topic** |
| L257 REQ-009 | "env GOOSE_METRICS_ENABLED gating \| State-Driven" | L156: WHEN cardinality cap exceeded — Event-Driven | **wrong topic AND category** |
| L258 REQ-010 | "label cardinality 100 cap \| State-Driven" | L160: WHILE Sink used, race-free — State-Driven | **wrong topic** |
| L259 REQ-011 | "concurrent emission race-free \| Ubiquitous" | L162: IF Sink is noop — State-Driven | **wrong topic AND category** |
| L260 REQ-012 | "PII 금지 \| Unwanted" | L164: IF Sink is nil — State-Driven | **wrong topic AND category** |
| L261 REQ-013 | "panic isolation \| Unwanted" | L168: PII forbidden in Labels — Unwanted | **wrong topic** |
| L262 REQ-014 | "외부 import 0건 \| Unwanted" | L174: no normalization/hash/redaction — Unwanted | **wrong topic** |
| L263 REQ-015 | "zero-cost noop \| Optional" | L176: no panic on cardinality cap — Unwanted | **wrong topic AND category** |
| L264 REQ-016 | "handle lifecycle 0 — GC만 \| Ubiquitous" | L178: no caller lifecycle (init/shutdown/flush) — Unwanted | topic adjacent, **wrong category** |
| L265 REQ-017 | "metric name convention \| Optional" | L180: no consumer wiring — Unwanted | **wrong topic AND category** |
| L266 REQ-018 | "label key naming \| Optional" | L184: WHERE daemon /debug/vars exposed — Optional | **wrong topic** |
| L267 REQ-019 | "Logger.Debug fallback \| Optional" | L186: WHERE cardinality cap configurable — Optional | **wrong topic** |
| L268 REQ-020 | "consumer SPEC contract \| Ubiquitous" | L188: WHERE histogram bucket customizable — Optional | **wrong topic AND category** |

15 of 20 rows in the matrix carry an incorrect topic, an incorrect EARS category, or both. This is not a clerical typo — it is a complete-row drift suggesting the matrix was authored against a different REQ list (possibly an earlier or sibling SPEC) and pasted in without reconciliation against §4. The matrix's stated mapping ("AC-006 매핑 to REQ-004/005/016") at spec.md:L270 cannot be trusted because the underlying REQ identifiers in the matrix do not denote the actual REQ-004/005/016. A reader following the matrix to verify traceability will be misled in 75% of rows.

**D2. spec.md:L271-L274 — Self-claimed defect resolution is false — Severity: critical.**

L272-L274 explicitly claims:
> plan-audit defect 해소:
> - D1 (REQ-AC 매트릭스 부재): 본 §5.7 표 추가 (2026-04-30)
> - D2 (REQ-004/005/016 직접 매핑 부재): AC-006 에 다중 REQ 매핑 명시
> - D3 (AC-018 contract test 선택 → 필수): §5.6 본문 "선택" → "[필수]" 갱신

D3 is genuinely resolved (verified at spec.md:L242, "(b) **[필수]**"). D1 is **not** resolved because the matrix introduced (D1 of this iteration) is broken. D2 is **not** resolved because the matrix row purportedly mapping "REQ-004/005/016" maps strings whose contents do not correspond to the actual REQ-004/005/016 in §4 — REQ-016 in the matrix talks about "handle lifecycle 0 — GC만" but the actual REQ-016 (L178) talks about "lifecycle (init/shutdown/flush) 을 caller 에게 요구하지 않는다". These are adjacent but not identical. A claim of resolution that is not factually true is a Severity-critical traceability defect.

**D3. spec.md:L264, L270 — AC-006 over-mapped — Severity: major.**

The matrix asserts AC-006 maps REQ-004, REQ-005, and REQ-016 (L252, L253, L264, L270). AC-006 itself (spec.md:L208) reads:

> `TestExpvarSink_Counter_Increments` 테스트는 (a) `Counter("test.counter", nil)` 의 `Inc()` 100회 호출 후 expvar 의 `test.counter` 변수 값이 정확히 100, (b) `Add(2.5)` 호출 후 값이 102.5 임을 검증한다.

This test verifies counter increment monotonicity (REQ-006). It does not verify (a) handle reuse for identical (name, labels) pairs (actual REQ-004 at L144), (b) Labels caller-preservation (actual REQ-005 at L146), or (c) absence of caller lifecycle requirement (actual REQ-016 at L178). Reusing AC-006 to claim coverage of three additional REQs is inflation, not coverage.

**D4. spec.md:L256 — REQ-008 (Gauge) AC mapping missing — Severity: major.**

Even charitably treating the matrix descriptions as the canonical mapping rather than the §4 numbers, the matrix never assigns an AC to the §4 REQ-008 (env GOOSE_METRICS_ENABLED gating, L154). The matrix row at L257 maps "env gating" to AC-010 + AC-011, but AC-010 is the cardinality cap test (L216) and AC-011 is the noop sink no-side-effect test (L220). Neither verifies env-toggle behavior. The actual env-toggle wiring is delegated to a different SPEC (CLI-INTEG / DAEMON-INTEG, spec.md:L154 final sentence), so an AC-less status here is defensible — but the matrix should say "out-of-scope, deferred to CLI-INTEG-001" rather than fabricate a mapping to unrelated ACs.

**D5. spec.md:L243 — AC-018 still partially deferred — Severity: minor.**

AC-018 option (b) is now `[필수]` (good). Option (a) (TELEMETRY-001 alias) remains "consumer SPEC implementation 시점에 해소" (L243 last sentence). This is acceptable as a single-SPEC contract test since (b) is mandated, but the AC text retains the ambiguity that a tester running the run-phase test suite for *this* SPEC must accept (b) and not attempt (a). Recommend tightening the AC wording to "(b) is the verification path; (a) is informational for downstream consumer SPEC."

---

## Chain-of-Verification Pass

Second-pass findings: I re-read spec.md §4 and §5.7 in full (not spot-check). The mismatch between matrix-row REQ descriptions and §4-REQ topics is not a one-off typo — it is consistent across 15 of 20 rows, indicating the matrix was authored against a stale or sibling REQ numbering. I also re-verified:

- REQ-OBS-METRICS-001..020 sequentiality at every line: confirmed continuous (L138, 140, 142, 144, 146, 150, 152, 154, 156, 160, 162, 164, 168, 174, 176, 178, 180, 184, 186, 188).
- AC-OBS-METRICS-001..018 sequentiality at every line: confirmed continuous (L196, 198, 200, 202, 204, 208, 210, 212, 214, 216, 220, 222, 226, 228, 232, 234, 236, 240).
- Exclusions at L546-L562 enumerate 13 specific items with named follow-up SPEC IDs where applicable — PASS.
- HISTORY at L22-L24 records only one entry (0.1.0). The post-audit fix (matrix addition) was made silently without a HISTORY entry — minor process defect, not raised as a numbered defect since spec versioning convention may permit pre-implementation patching.

No defects from the first pass were withdrawn after second pass.

---

## Regression Check (Iteration 2)

Defects from the prior `audit-2026-04-30.md`:

| Prior Defect | Status | Evidence |
|---|---|---|
| D1 (Major): "spec.md §5에 REQ↔AC 매핑 표 부재" | **UNRESOLVED — regressed to worse** | Matrix added at L247-L268 but is systematically mislabeled (this iteration's D1). Absence replaced with active misinformation. |
| D2 (Major): "REQ-004/005/016 직접 매핑 AC 부재" | **UNRESOLVED — false-claim regression** | Matrix claims AC-006 covers all three (L270), but AC-006 (L208) is a counter-increment test that does not verify handle-reuse, Labels-static, or no-caller-lifecycle properties. Self-claim of resolution is false (this iteration's D2/D3). |
| D3 (Note): "AC-018 contract_test.go 선택 → 필수 격상 권장" | **RESOLVED** | spec.md:L242 explicitly mandates option (b) as `[필수]` with cited rationale "plan-audit 2026-04-30 권장에 따라". |
| D4 (Note): NFR-004/005 임계값 사전 통지 권장 | **NOT-ACTED** | NFR-004/005 still carry the "plan phase 추정" caveat at L529 — minor, not a blocker. |
| D5 (Note): PII 정적 검증 SPEC ID 사전 채번 권장 | **NOT-ACTED** | Exclusions item #10 (L559) still says "정적 분석 도입은 별도 SPEC" without an ID. Minor, not a blocker. |

**Stagnation flag**: D1 has appeared in both iterations. In iteration 1 it was "matrix absent". In iteration 2 it is "matrix present but wrong". This is **regression, not stagnation** — manager-spec attempted a fix but the fix introduced a worse defect. Given iteration 2 is not the final iteration (3 max), the loop should continue but with a sharply scoped fix instruction.

---

## Recommendation

**FAIL — requires re-revision before iteration 3 audit. Do not proceed to `/moai run` Phase 1.**

Required fixes for the next manager-spec revision (in order):

1. **Replace spec.md §5.7 matrix (L245-L275) entirely.** Author the matrix freshly by reading §4 (L132-L188) line-by-line and recording, for each REQ-OBS-METRICS-001 through REQ-OBS-METRICS-020:
   - The actual REQ topic (one short noun phrase derived from §4 text)
   - The actual EARS category (per §4 subsection header: §4.1 Ubiquitous, §4.2 Event-Driven, §4.3 State-Driven, §4.4 Unwanted, §4.5 Optional)
   - Mapped AC IDs (only ones that actually exercise the REQ; do not over-claim)
   - For REQs with no AC (e.g., REQ-008 env-toggle is wiring-SPEC scope), explicitly write "deferred to <SPEC-ID>" or "informational, verified by godoc/NFR" instead of fabricating a mapping

2. **Add new ACs (or explicitly document scope deferral) for the genuinely uncovered REQs** identified by the corrected matrix. At minimum:
   - REQ-OBS-METRICS-004 (handle reuse): add `TestExpvarSink_HandleReuse_SameKey` — verify `sink.Counter("x", L1) == sink.Counter("x", L1)` returns the same handle (or that increments accumulate to the same counter).
   - REQ-OBS-METRICS-005 (Labels caller-preservation): add a test that mutates the input Labels map post-call and verifies the registered label set is unchanged, OR explicitly document this as a godoc contract verified by code review.
   - REQ-OBS-METRICS-016 (no caller lifecycle): document as "verified by absence of `Close()` / `Shutdown()` methods on Sink interface — see §6.1" — no test required, but the matrix should record the verification path.

3. **Rewrite the L271-L274 self-claim block** to reflect the actual resolution status of prior D1/D2 after the matrix is corrected. Do not claim resolution that is not factually true. Add a HISTORY row to L22-L24 capturing the matrix correction.

4. **Optional (not blocking):** Address prior D5 by reserving `SPEC-GOOSE-OBS-METRICS-LINT-001` as a TBD ID in Exclusions item #10 (L559).

Rationale for FAIL verdict citing must-pass evidence: MP-1, MP-2, MP-3 all PASS, MP-4 N/A. However, Traceability (a non-must-pass dimension) scored 0.25 due to the systematic matrix-row mislabel. Per audit policy "Grade each audit dimension independently. A PASS in one area does NOT offset a FAIL in another", a 0.25 in traceability — combined with a self-claimed resolution that is factually false — is a sufficient basis for FAIL when the SPEC has explicit cross-SPEC contract responsibility (it is the source-of-truth for TELEMETRY-001 / CLI-INTEG-001 / DAEMON-INTEG-001 / Phase-2 OTEL / Phase-3 PROM). Inaccurate traceability in a multi-consumer source-of-truth SPEC has high downstream cost.
