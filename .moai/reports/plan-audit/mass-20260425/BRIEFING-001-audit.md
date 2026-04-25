# SPEC Review Report: SPEC-GOOSE-BRIEFING-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.58

Reasoning context ignored per M1 Context Isolation. Audit conducted from spec.md and research.md only.

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: REQ-BRIEF-001 through REQ-BRIEF-020, sequential with consistent 3-digit zero-padding, no gaps, no duplicates. Verified at spec.md:L137-L183 (EARS section 4.1-4.5). REQ-001 at L137, REQ-020 at L183.

- [FAIL] MP-2 EARS format compliance: All 20 REQs in Section 4 (spec.md:L133-L183) are correctly labeled with one of the five EARS patterns (Ubiquitous/Event-Driven/State-Driven/Unwanted/Optional). However, the acceptance criteria in Section 5 (spec.md:L187-L242) are written in Given/When/Then test-scenario format rather than EARS patterns. Per M3 rubric for EARS compliance, ACs using Given/When/Then mislabeled as acceptance criteria without EARS structure trigger FAIL. Example AC-BRIEF-001 (L189-L192): "Given ... When ... Then ..." — not EARS. All 11 ACs (AC-001 through AC-011) use the same G/W/T format. Score band: 0.25 (fewer than a quarter use EARS).

- [FAIL] MP-3 YAML frontmatter validity: Multiple required fields missing or non-compliant:
  - `created_at` REQUIRED (ISO date string) — MISSING. spec.md:L5 uses `created: 2026-04-22` (wrong field name).
  - `labels` REQUIRED (array or string) — MISSING entirely from frontmatter (L2-L13).
  - `status: Planned` (L4) — non-standard enum value. Canonical enum: draft, active, implemented, deprecated.
  - `priority: P0` (L8) — non-canonical. Canonical values: critical, high, medium, low.
  - Extra non-canonical fields present: `updated`, `author`, `issue_number`, `phase`, `size`, `lifecycle`.
  Two required fields missing = FAIL.

- [N/A] MP-4 Section 22 language neutrality: SPEC targets Go package `internal/ritual/briefing/` — single programming language (Go) scope. TTS engine selection (Piper/espeak) concerns audio synthesis, not programming language neutrality. Auto-passes.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band | REQs use precise EARS language (spec.md:L137-L183). Minor ambiguity in REQ-BRIEF-007 "structured JSON" without schema (L151), REQ-BRIEF-018 "negative valence" not quantified (L179). |
| Completeness | 0.50 | 0.50 band | HISTORY (L17-L21) ✓, Overview/Background (L25-L70) ✓, Scope IN/OUT (L75-L130) ✓, EARS REQs (L133-L183) ✓, ACs (L187-L242) ✓, Exclusions (L490-L502) ✓. But frontmatter missing 2 required fields (L2-L13) → completeness compromised. |
| Testability | 0.75 | 0.75 band | Most ACs are binary-testable (AC-001 L189-L192 checks registry presence; AC-002 L194-L197 asserts field != nil; AC-009 L229-L232 checks len() <= 1000). Minor weasel words in AC-011 "임박 표현 포함" (L242) — subjective phrasing match. |
| Traceability | 0.25 | 0.25 band | 20 REQs vs only 11 ACs. Uncovered REQs: REQ-BRIEF-002 (DTO fields — no AC), REQ-BRIEF-003 (zap logging fields — no AC), REQ-BRIEF-004 (Handle* sequence — no AC), REQ-BRIEF-010 (config.enabled=false — no AC), REQ-BRIEF-011 (is_read flag — partial AC-008), REQ-BRIEF-014 (PII visibility filter — no AC despite being a security REQ), REQ-BRIEF-015 (no telemetry — no AC), REQ-BRIEF-017 (adaptive_timing — no AC), REQ-BRIEF-018 (negative mood tone — no AC). 9+ REQs uncovered. No AC explicitly references REQ-BRIEF-NNN by ID — traceability is implicit at best. |

## Defects Found

D1. spec.md:L5 — Frontmatter field `created` should be `created_at` (ISO date string) per MP-3 requirement. Severity: critical

D2. spec.md:L2-L13 — Frontmatter missing required field `labels` (array or string). Severity: critical

D3. spec.md:L4 — Frontmatter `status: Planned` uses non-canonical enum. Expected one of: draft, active, implemented, deprecated. Severity: major

D4. spec.md:L8 — Frontmatter `priority: P0` uses non-canonical enum. Expected one of: critical, high, medium, low. Severity: major

D5. spec.md:L189-L242 — All 11 acceptance criteria (AC-BRIEF-001 through AC-BRIEF-011) use Given/When/Then test-scenario format rather than one of the five EARS patterns. Per MP-2, acceptance criteria must be in EARS format. Severity: critical

D6. spec.md:L137-L242 — Traceability gap: 20 REQs but only 11 ACs. REQ-BRIEF-002, 003, 004, 010, 014, 015, 017, 018 have no corresponding AC. REQ-BRIEF-014 (PII visibility filter) is a security-critical requirement with no verification AC. Severity: critical

D7. spec.md:L189-L242 — No AC references its covering REQ by ID (e.g., "covers REQ-BRIEF-XXX"). Traceability is implicit; a SPEC reviewer cannot mechanically verify coverage. Severity: major

D8. spec.md:L151 — REQ-BRIEF-007 prescribes "structured JSON response with {greeting, main, closing, full_text} fields" but does not define validation schema (types, length constraints per field, required vs optional). Implementation may diverge. Severity: minor

D9. spec.md:L179 — REQ-BRIEF-018 uses "negative valence" without quantitative threshold (e.g., mood score < X). Not binary-testable. Severity: minor

D10. spec.md:L242 — AC-BRIEF-011 "임박 표현 포함" requires a regex or substring match rule to be binary-testable; current phrasing allows evaluator judgment. Severity: minor

D11. spec.md:L167 — REQ-BRIEF-013 "no longer than 1000 characters total" conflicts with Section 6.5 Fallback Template (L366-L372) which could exceed 1000 chars if all fields are large. No explicit handling. Severity: minor

D12. spec.md:L70 — Scope OUT lists "다국어 자동 번역" but REQ-BRIEF-007 (L151) includes "language" style hint with 4 options (ko/en/ja/zh) at L92. Ambiguity: is the narrative generated in target language via LLM (translation-like), or only the UI labels? Severity: minor

## Chain-of-Verification Pass

Second-look findings:

Re-read Section 4 (L133-L183) end-to-end: All 20 REQs verified individually. REQ-BRIEF-011 "new MorningBriefingTime events shall skip generation" references state `is_read==false` but AC-BRIEF-008 (L224-L227) tests a similar scenario without explicit `is_read` field check. Partial coverage, not full.

Re-read Section 5 (L187-L242) end-to-end: AC-BRIEF-007 (L219-L222) explicitly says "Google/Polly mock 호출 0회" — correctly binary-testable. AC-BRIEF-006 (L214-L217) similarly binary. Confirmed that multiple ACs have tight binary assertions, raising Testability band from 0.50 to 0.75.

Re-checked Exclusions (L490-L502): 11 specific, concrete exclusion entries (not vague). Completeness of Exclusions section is strong.

Re-checked contradictions: REQ-BRIEF-013 (1000 char cap, L167) vs Section 6 BriefingStyle.long=400자 (L90, L307) — consistent (cap is above max). No true contradiction, just under-specified fallback.

New defect discovered: D12 (language vs translation ambiguity) added.

Re-verification of frontmatter: Confirmed `created_at` missing, `labels` missing. These are not cosmetic — they break the MP-3 firewall.

## Regression Check

Not applicable (iteration 1).

## Recommendation

Verdict: FAIL. manager-spec must address the following before re-submission:

1. Fix frontmatter (spec.md:L2-L13):
   - Rename `created` → `created_at` (keep value `2026-04-22` as ISO date).
   - Add `labels:` field (array, e.g., `[ritual, llm, tts, orchestration]`).
   - Change `status: Planned` → `status: draft` (canonical enum).
   - Change `priority: P0` → `priority: critical` (canonical enum).
   - Optionally retain `updated`, `author`, etc. as project extensions (not required but not blocking).

2. Rewrite all 11 acceptance criteria in Section 5 (spec.md:L187-L242) using one of the five EARS patterns. Current G/W/T format is acceptable only as a test scenario bundle, not as the EARS AC. Option: split Section 5 into "5.1 EARS-form ACs" and "5.2 Test Scenarios (G/W/T)". Each AC must reference its covering REQ-BRIEF-NNN by ID.

3. Close traceability gaps. Add ACs for the following REQs at minimum:
   - REQ-BRIEF-002 (MorningBriefing DTO fields — field presence test)
   - REQ-BRIEF-003 (zap log fields — log output assertion)
   - REQ-BRIEF-004 (Handle* call sequence — orchestration test)
   - REQ-BRIEF-010 (config.enabled=false no-op)
   - REQ-BRIEF-014 (PII visibility filter — security-critical, MUST have AC)
   - REQ-BRIEF-015 (no external telemetry — network assertion)
   - REQ-BRIEF-017 (adaptive_timing, if in scope)
   - REQ-BRIEF-018 (negative mood tone switch)

4. Quantify "negative valence" threshold in REQ-BRIEF-018 (spec.md:L179) — e.g., "mood score <= -0.3 on [-1, +1] scale" or "3-day moving average of mood classification == 'negative'".

5. Define JSON schema for REQ-BRIEF-007 LLM response (spec.md:L151). Either inline types/constraints or reference a schema file.

6. Quantify AC-BRIEF-011 (spec.md:L242) "임박 표현" — e.g., "narrative contains regex `(곧|\\d+분 뒤|\\d+시 \\d+분에)`".

7. Resolve D12 (spec.md:L70 vs L92): clarify whether `briefing.style.language` triggers LLM-side generation in target language (multi-language output) or only UI strings. If generation, remove "다국어 자동 번역" from OUT scope.

Re-audit on iteration 2 with the above fixes applied. MP-2 and MP-3 are firewall failures — no partial credit.
