# SPEC Review Report: SPEC-GOOSE-RATELIMIT-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.62

> M1 Context Isolation: Reasoning context from prompt ignored. Audit performed against `spec.md` and `research.md` only.

## Must-Pass Results

- [FAIL] **MP-1 REQ number consistency**: PASS on this subcriterion — REQ-RL-001..013 sequential, no gaps, no duplicates, consistent zero-padding (spec.md:L93-L125). Marked PASS.
- [FAIL] **MP-2 EARS format compliance**:
  - REQ-RL-009 (spec.md:L115) labeled `[Unwanted]` but uses ubiquitous-negative form ("The tracker shall not mutate input headers map"). Missing "If [undesired condition] then" structure.
  - REQ-RL-011 (spec.md:L119) labeled `[Unwanted]` but uses ubiquitous-negative form ("The tracker shall not panic on nil headers..."). No "If...then" trigger.
  - REQ-RL-007 (spec.md:L109) starts with `While` but the consequent mixes two distinct shall statements ("RemainingSecondsNow() shall return 0.0, and UsagePct() is reported but flagged as stale") — the second clause lacks "shall" and is passive.
  - Three of thirteen REQs are structurally non-compliant with EARS patterns despite being labeled as such.
- [FAIL] **MP-3 YAML frontmatter validity**:
  - Field `created: 2026-04-21` (spec.md:L5) — required field name per MP-3 contract is `created_at`. Type mismatch / field-name mismatch.
  - Field `labels` is absent from frontmatter (spec.md:L1-L13). Required field missing.
  - Non-compliant values `status: Planned` (spec.md:L4) and `priority: P0` (spec.md:L8) are acceptable strings but deviate from canonical vocabulary (draft/active/implemented/deprecated; critical/high/medium/low). Marked as minor.
  - Two required-field violations (`created_at` missing as named, `labels` missing) — any single missing field is FAIL per MP-3.
- [N/A] **MP-4 Section 22 language neutrality**: SPEC is scoped to HTTP LLM provider rate-limit header parsing (OpenAI, Anthropic, OpenRouter, optional Nous). Not a multi-language tooling SPEC covering the 16 programming languages. Auto-passes.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band | Most REQs unambiguous with concrete thresholds (80%, 30s). Minor ambiguity: REQ-RL-007 "flagged as stale in Display()" is mixed with a shall-statement without separate AC (spec.md:L109). REQ-RL-004 uses "newly exceeded 80% since last parse" but 80% is hardcoded while REQ-RL-013 (L125) makes threshold configurable — a reasonable engineer could implement either consistently. |
| Completeness | 0.75 | 0.75 band | All mandatory sections present: HISTORY (L17), Overview (L25), Background (L41), Scope (L64), Requirements (L89), Acceptance Criteria (L129), Technical Approach (L168), Dependencies (L343), Risks (L357), References (L370), Exclusions (L392). Frontmatter sparse (missing labels + created_at naming). |
| Testability | 0.70 | between 0.50 and 0.75 | ACs AC-RL-001..007 mostly binary-testable with concrete Given/When/Then. AC-RL-006 (spec.md:L156-L159) says output string contains `"stale" or "reset passed"` — OR-condition requires tester judgment on which variant is emitted. AC-RL-007 asserts "final State consistency (마지막 완료된 Parse의 값이 유지)" but with 100 concurrent goroutines "last completed" is nondeterministic — not binary-testable as written. |
| Traceability | 0.55 | 0.50 band | Only seven ACs (AC-RL-001..007) cover thirteen REQs. REQ→AC mapping gaps below: |

## Defects Found

**D1.** spec.md:L5 — Frontmatter field `created: 2026-04-21` does not match required field name `created_at`. — Severity: **critical** (MP-3)

**D2.** spec.md:L1-L13 — Frontmatter missing required `labels` field. — Severity: **critical** (MP-3)

**D3.** spec.md:L115 — REQ-RL-009 labeled `[Unwanted]` but does not use "If [undesired condition], then [system] shall [response]" EARS pattern. Reads as negative-ubiquitous. — Severity: **major** (MP-2)

**D4.** spec.md:L119 — REQ-RL-011 labeled `[Unwanted]` but uses negative-ubiquitous form. Mixes three conditions (nil headers, nil observer list, zero-time now) into one requirement with ambiguous response ("logs at DEBUG and returns"). Should be decomposed. — Severity: **major** (MP-2)

**D5.** spec.md:L97 — REQ-RL-003 hardcodes implementation primitive: "using `sync.RWMutex`". Requirements should state the contract (concurrent-safe) not the mechanism. Violates WHAT-not-HOW. — Severity: **major** (RQ-3/RQ-4)

**D6.** spec.md:L95 — REQ-RL-002 embeds formula `Used() = max(0, Limit - Remaining)` and `UsagePct() = (Used/Limit)*100` inside a requirement. These are implementation signatures and arithmetic. Should be in Technical Approach only. — Severity: **minor** (RQ-4)

**D7.** spec.md:L101 vs L125 — Inconsistency: REQ-RL-004 hardcodes "80% threshold" while REQ-RL-013 allows `ThresholdPct` configuration (50.0-100.0). REQ-RL-004 should reference "the configured threshold" or `opts.ThresholdPct`. — Severity: **major** (CN-1)

**D8.** spec.md:L103 — REQ-RL-005 hardcodes "30 seconds" cooldown while §6.2 (L222) shows `WarnCooldown` as a configurable option with 30s default. Requirement should say "the configured cooldown window (default 30s)". — Severity: **major** (CN-1, RQ-4)

**D9.** Traceability gap — REQ-RL-008 (spec.md:L111, empty-state `IsEmpty()==true`) has **no corresponding AC**. — Severity: **major** (AC-5)

**D10.** Traceability gap — REQ-RL-010 (spec.md:L117, `ErrParserNotRegistered` without state mutation) has **no corresponding AC**. — Severity: **major** (AC-5)

**D11.** Traceability gap — REQ-RL-011 (spec.md:L119, no panic on nil inputs) has **no corresponding AC**. AC-RL-005 covers malformed header text but not nil header map / nil observer list / zero-time now. — Severity: **major** (AC-5)

**D12.** Traceability gap — REQ-RL-012 (spec.md:L123, observer dispatch order, error tolerance) has **no corresponding AC**. — Severity: **major** (AC-5)

**D13.** Traceability gap — REQ-RL-013 (spec.md:L125, ThresholdPct override with 50-100 bounds validation) has **no corresponding AC**. The 50.0/100.0 boundary validation behavior is unspecified (reject at New()? clamp? error return?). — Severity: **major** (AC-5, Clarity)

**D14.** spec.md:L156-L159 — AC-RL-006 contains OR-condition: `"stale" or "reset passed"`. Tester must interpret which variant is expected. Binary-testability violated. — Severity: **minor** (AC-2, AC-3)

**D15.** spec.md:L161-L164 — AC-RL-007 asserts "최종 State 일관성(마지막 완료된 Parse의 값이 유지)" for 100 concurrent goroutines. "Last completed" is nondeterministic under goroutine scheduling — the assertion is not binary-testable. Should assert "State value is one of the 100 inputs, no torn writes, race detector clean". — Severity: **major** (AC-2)

**D16.** spec.md:L392-L401 — Exclusions list omits **circuit breaker**. Rate-limit handling systems commonly include a breaker; silence here creates ambiguity about whether ADAPTER-001 or CREDPOOL-001 owns that feature. Recommend explicit "no circuit breaker; failure isolation delegated to CREDPOOL-001". — Severity: **minor** (SC-6)

**D17.** spec.md:L147 — AC-RL-004 fixture shows `anthropic-ratelimit-requests-reset: 2026-04-21T12:00:00Z` but does not specify tolerance for `(reset - now).Seconds()` — floating-point equality on Sub() can be fragile. AC should specify tolerance (e.g., ±1s) or fix `now` deterministically. — Severity: **minor** (AC-2)

**D18.** spec.md:L71 — Section 3.1 item 4 says "NousParser is metadata-only (실 구현은 Nous Portal 응답 후속)" — this is an internal contradiction with Section 2.2 header list which mentions Nous Portal compatibility, and with Section 3.2 which defers Nous to Phase 1 range. Phase 1 scope of Nous is unclear: metadata-only stub vs. completely excluded? — Severity: **minor** (CN-1)

**D19.** spec.md:L67-L71 — IN SCOPE lists "Provider별 헤더 파싱기 3종" but Dependencies section (L343) and Nous Portal rows (L366 R6) are inconsistent. Also: xAI/DeepSeek/Groq are mentioned in research.md §2.4 as OpenAI-compat reuse but not called out in IN SCOPE. — Severity: **minor** (Completeness)

**D20.** spec.md:L109 — REQ-RL-007 conflates two distinct statements: (1) "RemainingSecondsNow() shall return 0.0" is a shall, (2) "UsagePct() is reported but flagged as stale in Display()" lacks "shall" and introduces Display() semantics mid-requirement. Should split into REQ-RL-007a and 007b. — Severity: **minor** (MP-2 structure)

## Chain-of-Verification Pass

Second-look findings performed by re-reading:
- Frontmatter block (spec.md:L1-L13) — confirmed `created_at` and `labels` absent. Additional observation: `status: Planned` and `priority: P0` use non-canonical vocabulary — added as minor note but not elevated to defect (strings are still strings).
- Every REQ entry L93-L125 re-read end-to-end — confirmed REQ number sequence (001..013 no gaps), found the REQ-RL-004 vs REQ-RL-013 threshold inconsistency (D7) and REQ-RL-005 cooldown inconsistency (D8) missed on first pass.
- Every AC entry L131-L164 re-read — confirmed no AC cites a specific REQ-XXX identifier. Traceability is by topic match only, leaving REQ-RL-008/010/011/012/013 formally uncovered. Added D9-D13.
- Exclusions L392-L401 re-read — found circuit breaker omission (D16).
- Cross-checked Section 6.2 `TrackerOptions.WarnCooldown` configurability against REQ-RL-005 hardcode — inconsistency confirmed (D8).
- Cross-checked research.md §4.4 cooldown test (30s) — consistent with §6.2 impl but still inconsistent with REQ-RL-005 prose.
- AC-RL-007 race test re-read — determinism claim added (D15).

New defects surfaced in second pass: D7, D8, D15, D16, D17, D20.

## Special Focus Audit (per reviewer request)

| Focus | Finding | Evidence |
|-------|---------|----------|
| CREDPOOL-001 429 boundary | Properly delegated. 429 auto-retry in OUT scope (L81). Dependencies row (L348) and Exclusions (L396) both explicit. No boundary violation. | spec.md:L81, L348, L396 |
| Token bucket vs leaky bucket vs sliding window | **Not applicable** — SPEC explicitly does not implement any algorithm. Passive header-driven tracker, not a local quota manager. Exclusions L395 confirms ("pre-emptive throttle wait 수행하지 않는다"). Scope is appropriate. | spec.md:L60, L395 |
| Provider rate-limit header parsing | Well-specified with per-provider parsers (OpenAIParser, AnthropicParser, OpenRouterParser) and normalization to 4-bucket state. ISO 8601 normalization for Anthropic explicit (L296). Gap: case-insensitivity contract only in code comment (L252), not as REQ. | spec.md:L271-L300 |
| Distributed shared state | Explicitly out of scope: single `goosed` process assumption (L82). Appropriate Phase 1 narrowing. | spec.md:L82 |
| Circuit breaker inclusion | **Gap** — Not mentioned anywhere. Neither included nor explicitly excluded. Recommend adding to Exclusions section to remove ambiguity. | spec.md:L392-L401 (absence) |

## Regression Check

N/A — iteration 1.

## Recommendation

**Verdict: FAIL.** Two MP-3 violations (missing `labels`, `created_at` field name mismatch) are individually disqualifying. Additional EARS non-compliance (D3, D4), traceability gaps (D9-D13), and inconsistencies (D7, D8) compound.

Actionable fixes for manager-spec (priority-ordered):

1. **(Fix MP-3 — spec.md:L1-L13)** Rename `created: 2026-04-21` → `created_at: 2026-04-21T00:00:00Z` (use full ISO-8601 with time zone). Add `labels:` field (suggest `[rate-limit, llm, provider, tracker, phase-1]`).
2. **(Fix MP-2 D3) REQ-RL-009**: Rewrite as Unwanted EARS: "**If** input headers map is modified during Parse, **then** the tracker shall abort and log ERROR." Or convert label to `[Ubiquitous]` since negative-ubiquitous is a valid pattern.
3. **(Fix MP-2 D4) REQ-RL-011**: Decompose into three Unwanted REQs, each with "If [condition] then shall [response]" structure, covering nil headers, nil observer list, zero-time now separately. Each must have a corresponding AC.
4. **(Fix RQ D5-D6) REQ-RL-002, REQ-RL-003**: Strip implementation primitives. REQ-RL-003 should say "Parse/State/Display shall be safe for concurrent invocation from multiple goroutines" without naming `sync.RWMutex`. REQ-RL-002 should describe the contract ("Used is never negative; UsagePct returns 0 when Limit is zero") without embedding formulas.
5. **(Fix CN D7) REQ-RL-004**: Replace "exceeded 80%" with "exceeded the configured threshold (opts.ThresholdPct, default 80.0)".
6. **(Fix CN D8) REQ-RL-005**: Replace "less than 30 seconds ago" with "within the configured cooldown window (opts.WarnCooldown, default 30s)".
7. **(Fix Traceability D9-D13)** Add ACs covering REQ-RL-008 (IsEmpty on unseen provider), REQ-RL-010 (ErrParserNotRegistered with no state mutation), REQ-RL-011 (nil panic avoidance — three separate Given/When/Then cases), REQ-RL-012 (observer dispatch ordering + error isolation), REQ-RL-013 (ThresholdPct bounds: reject at `New()` if out of [50,100]).
8. **(Fix AC D14) AC-RL-006**: Pick one exact string to assert or test for a specific stale-flag token (e.g., `[STALE]` as suggested by research.md §5). Remove "or".
9. **(Fix AC D15) AC-RL-007**: Reword to testable invariant: "**Then** (1) race detector passes, (2) `State('openai').RequestsMin.Limit == 1000`, (3) `State('openai').RequestsMin.Remaining` equals one of the 100 input values, (4) no goroutine panics."
10. **(Fix Scope D16)** Add to Exclusions: "본 SPEC은 circuit breaker / failure isolation을 포함하지 않는다. 연속 429/5xx 시 credential rotation은 CREDPOOL-001, 재시도 backoff는 ADAPTER-001 소관."
11. **(Fix AC D17) AC-RL-004**: Pin `now` to a fixed timestamp (e.g., `now = 2026-04-21T11:59:34Z`) and assert `ResetSeconds == 60.0 ± 0.001` for determinism.
12. **(Fix CN D18-D19)** Section 3.1 item 4: Clarify — is NousParser stub registered in Phase 1 (metadata-only, returns empty state), or excluded entirely? Align with research.md §7 Q1 and add `NousParser` row to §6.1 layout if included. Explicitly list xAI/DeepSeek/Groq as OpenAIParser reuse targets in IN SCOPE (cross-reference research.md §2.4).
13. **(Fix D20) REQ-RL-007**: Split into REQ-RL-007a (stale detection shall statement) and REQ-RL-007b (Display() flag behavior shall statement). Add AC reference for both.

After applying these fixes, re-submit for iteration 2. Critical must-pass items (1-3) alone change the overall verdict; ignoring traceability gaps will cause iteration 2 to FAIL again.
