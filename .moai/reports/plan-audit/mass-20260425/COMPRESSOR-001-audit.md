# SPEC Review Report: SPEC-GOOSE-COMPRESSOR-001

Iteration: 1/3
Verdict: **FAIL**
Overall Score: 0.58

Reasoning context ignored per M1 Context Isolation. Audit based solely on:
- `/Users/goos/MoAI/AgentOS/.moai/specs/SPEC-GOOSE-COMPRESSOR-001/spec.md`
- `/Users/goos/MoAI/AgentOS/.moai/specs/SPEC-GOOSE-COMPRESSOR-001/research.md`
- Cross-reference (read-only, for REQ-018/AC-013 verification): `/Users/goos/MoAI/AgentOS/.moai/specs/SPEC-GOOSE-CONTEXT-001/spec.md`

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-COMPRESSOR-001 through REQ-COMPRESSOR-018 are sequential (spec.md:L97-139), no gaps, no duplicates, consistent 3-digit zero-padding.

- **[PASS] MP-2 EARS format compliance**: All 18 REQs match one of the five EARS patterns:
  - Ubiquitous (REQ-001..004): "shall [response]" form, spec.md:L97-103.
  - Event-Driven (REQ-005..010): "When [trigger], ... shall [response]", spec.md:L107-117.
  - State-Driven (REQ-011..012): "While [condition], ... shall [response]", spec.md:L121-123.
  - Unwanted (REQ-013..016): "shall not" / "If X, shall Y", spec.md:L127-133.
  - Optional (REQ-017..018): "Where [feature], ... shall [response]", spec.md:L137-139.

- **[FAIL] MP-3 YAML frontmatter validity**: Two required fields are missing or incorrectly named.
  - Evidence: spec.md:L1-13 shows frontmatter with `created: 2026-04-21` (spec.md:L5) but **no `created_at` field**. MP-3 explicitly requires `created_at` as the ISO date string field name.
  - Evidence: frontmatter at spec.md:L1-13 contains `id`, `version`, `status`, `created`, `updated`, `author`, `priority`, `issue_number`, `phase`, `size`, `lifecycle` — **`labels` field is absent**. MP-3 requires `labels` (array or string).
  - Any missing required field = FAIL per M5 rule.

- **[N/A] MP-4 Section 22 language neutrality**: This SPEC targets a single Go package (`internal/learning/compressor/`) and does not claim multi-language tooling coverage. Tokenizer references (tiktoken-go at spec.md:L565, Kimi-K2 Python rejection at research.md:L82,107) are correctly framed as implementation-injection choices, not universal-language enumeration. N/A: single-language SPEC.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.70 | 0.75 band (minor ambiguity in 1-2 locations) | AC-COMPRESSOR-001 "입력과 동일 포인터 내용(새 alloc이지만 값은 등가)" is self-contradictory (spec.md:L150). AC-COMPRESSOR-010 "head 4 + tail 1 겹침" (spec.md:L193) is inaccurate — TailProtectedTurns=4 over 5 turns protects indices {1,2,3,4}, not a "tail 1" configuration. |
| Completeness | 0.50 | 0.50 band (multiple sections missing or sparse; frontmatter missing fields) | YAML frontmatter missing `created_at` and `labels` (spec.md:L1-13). HISTORY has only a single entry (spec.md:L19-21), acceptable for a new SPEC. All textual sections (Overview/Background/Scope/Requirements/AC/Exclusions) are present and substantive (spec.md:L25-624). |
| Testability | 0.70 | 0.75 band (most AC binary-testable; 2-3 exceptions) | AC-COMPRESSOR-008 includes the weasel-ish "+5 오버헤드" (spec.md:L185). AC-COMPRESSOR-002 timing-free but measurable. AC-COMPRESSOR-001 ambiguity (see Clarity) affects testability. |
| Traceability | 0.50 | 0.50 band (multiple REQs lack AC) | Five REQs have **no dedicated AC**: REQ-001 (non-nil metrics invariant), REQ-004 (Tokenizer sole source), REQ-012 (accumulation short-fall → StillOverLimit), REQ-013 (no delete/reorder in protected regions), REQ-017 (custom prompt template). See spec.md:L97-139 vs L147-210. |

**Overall Score**: (0.70 + 0.50 + 0.70 + 0.50) / 4 = **0.60 category average**; weighted down by MP-3 failure → final **0.58**.

---

## Defects Found

### Critical (blocks PASS, integration-breaking or standards-breaking)

- **D1** [spec.md:L5] — YAML frontmatter uses `created: 2026-04-21` instead of required `created_at: 2026-04-21`. MP-3 required field naming. Severity: **critical** (breaks MP-3).

- **D2** [spec.md:L1-13] — YAML frontmatter is missing the required `labels` field entirely. MP-3 mandates presence. Severity: **critical** (breaks MP-3).

- **D3** [spec.md:L384-385 vs SPEC-GOOSE-CONTEXT-001/spec.md:L319-322] — `CompactorAdapter` signature mismatch with CONTEXT-001's `Compactor` interface.
  - COMPRESSOR-001 declares: `ShouldCompact(state *loop.State) bool` and `Compact(state *loop.State) (*loop.State, CompactBoundary, error)` — **pointer receivers**.
  - CONTEXT-001 mandates: `ShouldCompact(s loop.State) bool` and `Compact(s loop.State) (loop.State, CompactBoundary, error)` — **value receivers**.
  - REQ-COMPRESSOR-018 (spec.md:L139) and AC-COMPRESSOR-013 (spec.md:L207-210) both claim compatibility, but the declared Go signatures cannot satisfy CONTEXT-001's interface. `var _ context.Compactor = (*CompactorAdapter)(nil)` (asserted in research.md:L249) **will fail to compile**.
  - Severity: **critical** (contract violation with declared dependency).

- **D4** [spec.md:L498-501 vs SPEC-GOOSE-CONTEXT-001/spec.md:L110,L120] — `ShouldCompact` semantic drift. COMPRESSOR-001's adapter returns `tokenizer.CountTrajectory(traj) > cfg.TargetMaxTokens` (single threshold). CONTEXT-001 REQ-CTX-007 requires trigger when "≥80% of TokenLimit OR ReactiveTriggered OR MaxMessageCount exceeded". COMPRESSOR-001 ignores the 80% ratio, `ReactiveTriggered`, and `MaxMessageCount` conditions entirely. CONTEXT-001 REQ-CTX-011 also requires override when `WarningLevel == Red` — not honored. Severity: **critical** (silent semantic drift; adapter will under-trigger).

- **D5** [spec.md:L503-519 vs SPEC-GOOSE-CONTEXT-001/spec.md:L112] — `Compact` strategy selection contract violated. CONTEXT-001 REQ-CTX-008 requires strategy selection in order `AutoCompact → ReactiveCompact → Snip` with `Summarizer == nil` fallback to `Snip`. COMPRESSOR-001's adapter calls `a.inner.Compress` directly, which only implements the head/tail + LLM-middle strategy. There is no Snip fallback, no ReactiveCompact path, and no "Summarizer nil → Snip" branch. Severity: **critical** (adapter does not implement the contract it claims to satisfy).

- **D6** [spec.md:L139, L207-210 vs SPEC-GOOSE-CONTEXT-001/spec.md:L100] — `redacted_thinking` preservation not addressed. CONTEXT-001 REQ-CTX-003 mandates that "no snip or summarization pass shall drop or mutate `redacted_thinking` blocks". COMPRESSOR-001's middle-region summarization replaces the entire slice with a single `{From: human, Value: summary}` entry (spec.md:L441-444), discarding any `redacted_thinking` blocks in the middle region. The adapter path therefore silently violates REQ-CTX-003. Severity: **critical** (contract violation).

### Major (incomplete traceability, overclaims)

- **D7** [spec.md:L97, §5 absence] — REQ-COMPRESSOR-001 ("non-nil `*TrajectoryMetrics` on every path including error") has **no dedicated AC**. This is a cross-cutting invariant guaranteed to INSIGHTS-001, and its absence means the contract is unverified by the AC set. Severity: **major**.

- **D8** [spec.md:L103, §5 absence] — REQ-COMPRESSOR-004 ("Tokenizer sole source of token counts; no hardcoded ratios outside Tokenizer") has **no AC**. Enforcement requires static grep check or code review, neither scheduled. Severity: **major**.

- **D9** [spec.md:L123, §5 absence] — REQ-COMPRESSOR-012 ("accumulation fails to reach target_compress → still summarize with StillOverLimit") has **no AC**. This edge case (middle region too thin) cannot be regression-tested. Severity: **major**.

- **D10** [spec.md:L127, §5 absence] — REQ-COMPRESSOR-013 ("no delete/reorder in protected head/tail") has **no dedicated AC**. AC-COMPRESSOR-003/004 verify protected indices are *computed* correctly, but not that `Compress` *preserves* them post-compression. Severity: **major**.

- **D11** [spec.md:L137, §5 absence] — REQ-COMPRESSOR-017 ("custom SummarizerPromptTemplate with variables `{{.Turns}}`, `{{.ModelName}}`, `{{.TargetTokens}}`") has **no AC**. Optional feature ships untested. Severity: **major**.

- **D12** [spec.md:L36] — Overclaim: "Phase 0의 in-session compaction(turn 실행 중)과 Phase 4의 offline trajectory compaction(디스크 압축)이 동일 코드 경로를 공유한다." Given D3/D4/D5 (signature + semantic + strategy drift), the two do NOT share a single code path; the adapter requires significant translation logic and re-implements only a subset of CONTEXT-001's strategies. Severity: **major** (misleading rationale for architectural decision).

- **D13** [spec.md:L577] — Risk R5 identifies that adapter path wants `MaxRetries=1` (to avoid 21s UI-blocking), but `CompressionConfig` (spec.md:L241-251) and REQ-007 (spec.md:L111) do not document this override semantic. Risk acknowledged but unmitigated at the spec level. Severity: **major**.

### Minor

- **D14** [spec.md:L150] — AC-COMPRESSOR-001 phrasing "입력과 동일 포인터 내용(새 alloc이지만 값은 등가)" is self-contradictory. If it's a new allocation, pointer identity cannot be equal; only content equality can. The parenthetical attempts to rescue but leaves the tester unsure whether to assert pointer inequality or content equality. Severity: **minor** (clarity).

- **D15** [spec.md:L193] — AC-COMPRESSOR-010 "head 4 + tail 1 겹침" is inaccurate. With `TailProtectedTurns=4` and 5 turns total, the tail-protected set is indices {1,2,3,4} (4 entries), not "tail 1". Correct phrasing: "head-protected and tail-protected sets overlap covering all 5 turns". Severity: **minor** (clarity, but testers can still implement against the total-5-turn + all-over-target semantics).

- **D16** [spec.md:L185] — AC-COMPRESSOR-008 goroutine peak "≤ 55 (50 worker + 5 오버헤드)" — the "+5 오버헤드" is an undocumented slack. Under real go runtime, main goroutine, GC sweeper, and test scaffolding goroutines can push peak above 55 non-deterministically. A better formulation: "≤ 50 compressor-owned worker goroutines as measured at package boundary". Severity: **minor** (testability).

- **D17** [spec.md:L131] — REQ-COMPRESSOR-015 threshold "2 × SummaryTargetTokens" lacks justification. Why 2×? Neither research.md §8 nor spec.md §6.4 explains the choice. Could be 1.25× or 1.5×. Severity: **minor** (design rationale).

- **D18** [spec.md:L170] — AC-COMPRESSOR-005 expected delay ranges "[1s, 3s]" and "[2s, 6s]" are consistent with `delay = BaseDelay * 2^attempt * rand(0.5, 1.5)`, but the test assertion would need to account for total elapsed time including Summarizer stub execution. Not specified whether stub returns instantly. Severity: **minor** (testability).

- **D19** [spec.md:L441-444] — Rebuilt `Trajectory` struct at §6.3 copies `Metadata: t.Metadata` by reference (map sharing). If Metadata contains mutable state and downstream mutates `compressed.Metadata`, input `t.Metadata` is also affected. This violates REQ-002 (input unmutated) in spirit. Severity: **minor** (subtle implementation hazard not caught by AC-012).

---

## Chain-of-Verification Pass

**Second-pass re-read targets**: YAML frontmatter, every REQ (L97-139), every AC (L147-210), §6.6 adapter code (L497-520), and CONTEXT-001 cross-reference (lines 110, 112, 120, 319-322).

**New findings on re-read**:
- D4/D5/D6 were **not** surfaced on first reading of the spec in isolation. They only become visible after cross-referencing CONTEXT-001's `REQ-CTX-007`, `REQ-CTX-008`, `REQ-CTX-003`, and `REQ-CTX-011` against COMPRESSOR-001's adapter at spec.md:L497-520. On first pass I might have accepted REQ-018 ("shall satisfy the same ShouldCompact + Compact contract") at face value. Deep verification reveals the adapter does **not** satisfy that contract — it satisfies only a degraded subset.
- D19 (Metadata aliasing) is a second-pass discovery: first read interpreted REQ-002 as "slice-level immutability", but §6.3 pseudocode explicitly carries `Metadata: t.Metadata` (reference copy), which is incompatible with strong input-immutability under Go semantics.
- Confirmed all 18 REQs read individually (not skimmed) and all 13 ACs reviewed for trace-back.
- Confirmed Exclusions section (spec.md:L610-623) is specific and enumerated.

Chain-of-verification strengthens the FAIL verdict: in particular, the CONTEXT-001 integration claim is an overclaim that requires spec-level amendment, not just implementation diligence.

---

## Regression Check

Not applicable: iteration 1.

---

## Recommendation

**Verdict: FAIL.** manager-spec must revise the SPEC and re-submit for iteration 2. Prioritized fix list:

1. **[MP-3, D1]** Rename `created: 2026-04-21` → `created_at: 2026-04-21` in YAML frontmatter (spec.md:L5). If `updated` follows the same convention, also consider `updated_at`.

2. **[MP-3, D2]** Add `labels:` array to YAML frontmatter (spec.md:L1-13). Suggested: `labels: [learning, compressor, trajectory, llm-summary]`.

3. **[D3]** Reconcile `CompactorAdapter` signatures with CONTEXT-001's `Compactor` interface. Either:
   - (a) Change adapter signatures to value-receiver form matching CONTEXT-001 (spec.md:L384-385, L498, L503), OR
   - (b) File a matching amendment to CONTEXT-001 to switch to pointer receivers — but this is out-of-scope for COMPRESSOR-001 and requires coordinated change.
   - Add an explicit compile-time assertion in spec body: `var _ loop.Compactor = (*CompactorAdapter)(nil)` (or value form).

4. **[D4]** Rewrite the adapter's `ShouldCompact` semantics to match CONTEXT-001 REQ-CTX-007 + REQ-CTX-011. At minimum: evaluate `TokenCountWithEstimation(state.Messages) / state.TokenLimit >= 0.80 OR state.AutoCompactTracking.ReactiveTriggered OR len(state.Messages) > state.MaxMessageCount` and override to `true` when `WarningLevel == Red`. Add dedicated AC.

5. **[D5]** Add a strategy selection path to the adapter: `AutoCompact → ReactiveCompact → Snip`, with `Summarizer == nil → Snip` fallback. If COMPRESSOR-001 declines to implement Snip and ReactiveCompact (scope refusal), then REQ-018 must be restated: the adapter provides only the `AutoCompact` strategy and CONTEXT-001 must handle others. Either way, the current unconditional-compress implementation is non-conformant. Add AC covering Summarizer-nil fallback.

6. **[D6]** Document how `redacted_thinking` blocks are preserved across middle-region summarization. Options: (a) exclude messages containing `redacted_thinking` from the compressible middle via adjustments to `findProtectedIndices`; (b) prepend/append preserved `redacted_thinking` blocks around the summary entry. Add REQ + AC referencing CONTEXT-001 REQ-CTX-003.

7. **[D7-D11]** Add dedicated ACs:
   - AC covering REQ-001 (metrics non-nil on error path; e.g., inject Summarizer panic and assert metrics pointer).
   - AC covering REQ-004 (no hardcoded ratios outside Tokenizer — a static grep assertion or package-level test).
   - AC covering REQ-012 (middle region too thin to meet target_compress).
   - AC covering REQ-013 (post-compression protected head/tail unchanged — byte-exact assertion).
   - AC covering REQ-017 (custom prompt template with all three variables rendered).

8. **[D12]** Either substantiate "동일 코드 경로 공유" claim at spec.md:L36 (by actually sharing code) or soften to "본 SPEC은 CONTEXT-001이 요구하는 Compactor 인터페이스의 AutoCompact 변종을 제공한다; Snip/ReactiveCompact는 CONTEXT-001이 담당".

9. **[D13]** Document adapter-path override behavior in `CompressionConfig`: add field `AdapterMaxRetries` (default 1) or state that adapter should be constructed with a separate compressor instance whose config lowers `MaxRetries` to 1.

10. **[D14]** Rewrite AC-COMPRESSOR-001 "Then" clause: "반환된 `*Trajectory`는 입력과 **다른 포인터**이지만 `reflect.DeepEqual(got, input) == true`; `metrics.SkippedUnderTarget == true`; Summarizer mock 호출 0회".

11. **[D15]** Rewrite AC-COMPRESSOR-010 Given: "5턴 Trajectory(TailProtectedTurns=4 때문에 indices {1,2,3,4} 모두 tail 보호, head 보호 0번도 포함하여 모든 5턴 protected)".

12. **[D16]** Tighten AC-COMPRESSOR-008 goroutine assertion to a compressor-owned counter (e.g., an internal `atomic.Int64` incremented on acquire/decremented on release; assert max ≤ 50).

13. **[D17]** Add a one-line rationale for the `2×` overshoot threshold in §6.3 or research.md §8.

14. **[D19]** In §6.3 pseudocode at L447, change `Metadata: t.Metadata` to a deep-copy (or explicitly document the alias as acceptable and update REQ-002 wording to "shall not mutate Conversations slice"). Add an AC asserting `compressed.Metadata` is a distinct map instance from `t.Metadata`.

Upon revision, re-submit to plan-auditor for iteration 2, at which point the regression check will verify whether each of D1-D19 is resolved.

---

**End of Report**
