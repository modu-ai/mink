# SPEC Review Report: SPEC-GOOSE-ERROR-CLASS-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.62

Reasoning context ignored per M1 Context Isolation. Audit based solely on
`spec.md` (683 lines) and `research.md` (379 lines).

---

## Must-Pass Results

### MP-1 REQ number consistency — PASS
Evidence: REQ-ERRCLASS-001 through REQ-ERRCLASS-024 are sequential with no
gaps or duplicates. Verified by end-to-end re-read:
- Ubiquitous: spec.md:L93, L95, L97, L99 (001-004)
- Event-Driven: spec.md:L103, L105, L107, L109, L111, L113, L115, L117, L119,
  L121, L123, L125 (005-016)
- State-Driven: spec.md:L129, L131 (017-018)
- Unwanted: spec.md:L135, L137, L139, L141 (019-022)
- Optional: spec.md:L145, L147 (023-024)
Zero-padding is consistent (3 digits).

### MP-2 EARS format compliance — PASS (with minor deviations)
Evidence:
- Ubiquitous pattern "The [X] shall [response]" — spec.md:L93, L95, L97, L99
- Event-Driven pattern "When ... the classifier shall return ..." — spec.md:L103-L125
- State-Driven pattern "While ..." — spec.md:L129, L131
- Unwanted pattern "If ... then shall" — REQ-022 (spec.md:L141) is canonical
  "If ... the classifier shall still ..."; REQ-019/020/021 (L135/L137/L139)
  use bare "shall not" rather than the classical "If undesired condition then
  shall" structure. Bare "shall not" is commonly accepted as EARS unwanted-
  behavior variant per IEEE practice; classified as minor, not blocking.
- Optional pattern "Where ..." — spec.md:L145, L147

### MP-3 YAML frontmatter validity — FAIL
Evidence (spec.md:L1-L13):
```
id: SPEC-GOOSE-ERROR-CLASS-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 4
size: 소(S)
lifecycle: spec-anchored
```
Defects:
- **labels field MISSING** — MP-3 requires `labels` (array or string); no
  such key exists. **This alone is a must-pass FAIL.**
- Field `created` used instead of `created_at` (ISO date string present, but
  key name deviates from MP-3 required schema).
- `status: Planned` — not in standard enum {draft, active, implemented,
  deprecated}. Project convention, not necessarily a blocker, but deviates.
- `priority: P0` — not in standard enum {critical, high, medium, low}.
  Project convention.

### MP-4 Language neutrality — N/A
Evidence: SPEC is explicitly scoped to a single Go package
(`internal/evolve/errorclass/`, spec.md:L64, L252-L264). Multi-provider
support (anthropic/openai/google/xai/deepseek/ollama) is an application-
domain concern, not the 16-language template-binding MP-4 protects against.
Auto-passes.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.70 | 0.75 band (minor ambiguity in 2 REQs) | REQ-021 exception list (spec.md:L139) cites Billing and ThinkingSignature as reasons that "explicitly define both retryable=true AND should_fallback=true", but the defaults table (spec.md:L377-L391) shows Billing=`{false,false,true,true}` and ThinkingSignature=`{false,false,false,true}` — retryable is **false** for both. Self-contradiction; see D3. AC-014 (L223) uses "신중히 (cautiously)" — weasel word. |
| Completeness | 0.55 | 0.50-0.75 band (frontmatter defect + structural gaps) | HISTORY (L17), Overview/WHY (L25), Background/WHY (L38), Scope/WHAT (L60), REQUIREMENTS (L87), ACCEPTANCE CRITERIA (L151), Dependencies (L605), Risks (L623), References (L639), Exclusions (L665) — all present. HOWEVER: MP-3 `labels` missing = frontmatter incomplete (see D1). 6 REQs lack ACs (see D2). |
| Testability | 0.75 | 0.75 band (one AC has weasel word) | AC-ERRCLASS-001 through AC-ERRCLASS-018 are mostly binary-testable Given-When-Then. AC-014 contains "신중히" (cautiously) — subjective, not binary. AC-016 references "panic recovered" message, testable. AC-011 uses specific numeric thresholds (125_000 > 120_000), testable. |
| Traceability | 0.50 | 0.50 band (multiple REQs uncovered) | 6 of 24 REQs have no corresponding AC: REQ-004 (RawError preservation), REQ-007 (403 AuthPermanent), REQ-013 (500/502 ServerError), REQ-018 (stage-1 skip for unsupported provider), REQ-020 (meta read-only), REQ-021 (flag combination invariant). AC→REQ references are implicit (thematic) except AC-006 (L183) which explicitly cites "REQ-022". Inconsistent citation style. |

---

## Defects Found

- **D1 [CRITICAL]** spec.md:L1-L13 — YAML frontmatter missing required
  `labels` field. MP-3 FAIL. Field `created` should be `created_at` to match
  MP-3 schema. Severity: **critical** (must-pass violation).

- **D2 [MAJOR]** Traceability gaps — 6 REQs have no acceptance criterion:
  - REQ-ERRCLASS-004 (spec.md:L99) "errors.Unwrap chain preservation" — no AC
    validates `errors.Unwrap(classified.RawError)` behavior.
  - REQ-ERRCLASS-007 (spec.md:L107) "403 + permission → AuthPermanent" — no
    AC covers HTTP 403 case (AC-003 covers 401, AC-004 covers 402, nothing
    for 403).
  - REQ-ERRCLASS-013 (spec.md:L119) "500/502 → ServerError" — no AC covers
    HTTP 500 or 502 (AC-008 covers 503, AC-009 covers 529).
  - REQ-ERRCLASS-018 (spec.md:L131) "stage-1 skipped for unsupported
    provider" — no AC validates that an unknown provider bypasses stage 1.
  - REQ-ERRCLASS-020 (spec.md:L137) "meta must not be mutated" — no AC
    validates read-only contract.
  - REQ-ERRCLASS-021 (spec.md:L139) "flag combination invariant" — no AC
    validates the retryable+fallback exception rule.
  Severity: **major**.

- **D3 [MAJOR]** spec.md:L139 vs L377-L391 — REQ-ERRCLASS-021 claims Billing
  and ThinkingSignature are among the reasons that "explicitly define both
  retryable=true AND should_fallback=true". The defaults table (§6.3)
  contradicts this: Billing row is `{false, false, true, true}` (retryable
  **false**) and ThinkingSignature row is `{false, false, false, true}`
  (retryable **false**). Only Overloaded `{true, false, false, true}` and
  ServerError `{true, false, false, true}` actually have both flags true.
  REQ-021's exception list is wrong by 2 of 4 entries. Severity: **major**
  (internal contradiction between normative REQ and normative defaults
  table).

- **D4 [MINOR]** spec.md:L223 (AC-ERRCLASS-014) — "기본적으로 한 번은 시도,
  REQ-ERRCLASS-022에 따라 신중히". The word "신중히" (cautiously) is a
  weasel word — a tester cannot determine binary pass/fail from
  "cautiously". Remove or replace with a measurable condition. Severity:
  **minor**.

- **D5 [MINOR]** spec.md:L131 (REQ-ERRCLASS-018) — The supported provider
  set is enumerated as `anthropic, openai, google, xai, deepseek, ollama`.
  However `BuiltinProviderPatterns` (spec.md:L403-L409) only contains
  anthropic and openai patterns. Providers declared "supported" at the
  REQ level but lacking any built-in pattern mean stage 1 is effectively a
  no-op for 4 of 6 providers. Either reduce the supported set to `{anthropic,
  openai}` or add patterns for google/xai/deepseek/ollama. Severity:
  **minor** (consistency gap, CN-3).

- **D6 [MINOR]** spec.md:L141 (REQ-ERRCLASS-022) vs AC-ERRCLASS-006
  (spec.md:L180-L183) — AC-006 tests the HTTP 400 → stage-4 message override
  path. However, the general REQ-022 claim ("HTTP status is a hint, not
  final" for ANY 4xx/5xx, not just 400) is broader than what AC-006 covers.
  No AC validates the override for, e.g., a 429 whose message contradicts
  RateLimit. Severity: **minor** (partial test coverage of a broad
  normative rule).

- **D7 [MINOR]** spec.md:L295 — "AllFailoverReasons() slice" — comment
  `// 14 values (exclude Unknown or include? spec: include)` indicates the
  author was undecided at drafting time. AC-ERRCLASS-001 (L158) says "정확히
  14개 반환" and enumerates 14 including Unknown. The iota block (L274-L289)
  shows Unknown at position 0. So Unknown is included → 14 total. The
  drafting-note comment should be removed from the final SPEC. Severity:
  **minor** (documentation hygiene).

---

## Chain-of-Verification Pass

Second-pass findings:
- Re-read all 24 REQs end-to-end (not spot-check): confirmed D2 traceability
  gaps (6 uncovered REQs). First pass missed REQ-018 initially; second pass
  caught it.
- Re-read defaults table (L377-L391) line by line against REQ-005 through
  REQ-015 to verify flag consistency. Found D3 (Billing/ThinkingSignature
  cited wrongly in REQ-021) — this defect was NOT surfaced in the first
  skim and is only visible when cross-checking REQ-021's prose against the
  defaults table.
- Re-read Exclusions section (L665-L678): specific entries present, no
  vague items. PASS.
- Checked for contradictions between REQs and §6 technical approach:
  confirmed D3. No other contradictions found.
- Re-verified REQ number sequencing end-to-end. Confirmed no gaps/duplicates.
- Checked AC→REQ citation style: only AC-006 (L183) and AC-014 (L223)
  explicitly name a REQ-ID. The remaining 16 ACs rely on thematic match.
  This is acceptable but inconsistent — flagged implicitly under D2.

---

## Recommendation

FAIL. manager-spec must address the following before re-audit:

1. **Fix frontmatter (D1, blocks MP-3)**: Add `labels: [error-handling, go,
   phase-4, evolve]` (or equivalent array) to the YAML block at spec.md:L1-
   L13. Rename `created` → `created_at` to match the MoAI spec schema. If
   project convention intentionally uses `created`, update MP-3 schema docs;
   otherwise conform to schema.

2. **Close traceability gaps (D2)**: Add the following ACs:
   - AC-ERRCLASS-019: Given any `ClassifiedError`, When caller invokes
     `errors.Unwrap(classified.RawError)`, Then the innermost wrapped error
     is returned (covers REQ-004).
   - AC-ERRCLASS-020: Given StatusCode=403 and message "permission denied",
     When Classify, Then Reason==AuthPermanent with retryable=false,
     should_rotate_credential=true, should_fallback=true (covers REQ-007).
   - AC-ERRCLASS-021: Given StatusCode=500 (and separately 502), When
     Classify, Then Reason==ServerError with retryable=true,
     should_fallback=true (covers REQ-013).
   - AC-ERRCLASS-022: Given meta.Provider="groq" (unsupported), When
     Classify with message matching an anthropic-specific pattern, Then
     stage 1 is skipped and classification proceeds to stage 2+ (covers
     REQ-018).
   - AC-ERRCLASS-023: Given input meta, When Classify, Then meta fields
     remain unchanged after the call (covers REQ-020, check via deep-equal
     or field-by-field).
   - AC-ERRCLASS-024: Given any reason outside {Overloaded, ServerError},
     When lookup defaults, Then retryable AND should_fallback are NOT both
     true simultaneously (covers REQ-021).

3. **Resolve contradiction (D3)**: REQ-ERRCLASS-021 (L139) currently lists
   `Billing, Overloaded, ServerError, ThinkingSignature` as exceptions. Per
   the defaults table only `Overloaded` and `ServerError` actually satisfy
   retryable=true AND should_fallback=true. Fix by either:
   - (a) Remove Billing and ThinkingSignature from REQ-021's exception list,
     OR
   - (b) Change the defaults for Billing to `{true, ...}` and ThinkingSignature
     to `{true, ...}` (semantically wrong — both are non-retryable by
     design), OR
   - (c) Rewrite REQ-021 to state the invariant more narrowly ("retryable=true
     AND should_fallback=true co-occur only for Overloaded and ServerError").
   Option (c) recommended.

4. **Remove weasel word (D4)**: AC-ERRCLASS-014 (L223) — delete "신중히"
   or replace with a measurable condition (e.g., "Retryable=true with
   RetryCount<=1").

5. **Align provider set (D5)**: Either shrink the REQ-018 supported-provider
   list to match actually-implemented BuiltinProviderPatterns, OR add
   patterns for google, xai, deepseek, ollama at spec.md:L403 area.

6. **Broaden override-path coverage (D6)**: Consider adding one more AC
   demonstrating stage-2→stage-4 override for a non-400 status (e.g., 429
   with a message that overrides to Billing) to fully exercise REQ-022.

7. **Remove drafting-note comment (D7)**: spec.md:L295 — delete the
   parenthetical `// 14 values (exclude Unknown or include? spec: include)`.
   The comment suggests unresolved design intent but the rest of the SPEC
   confirms inclusion.

Once D1 (MP-3 blocker), D2 (traceability), and D3 (contradiction) are
resolved, the SPEC should pass on the next iteration. D4-D7 are minor and
will not block a PASS but should be cleaned up for quality.

---

**End of Report**
