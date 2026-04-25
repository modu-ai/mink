# SPEC Review Report: SPEC-GOOSE-I18N-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.58

Reasoning context ignored per M1 Context Isolation. Injected system-reminders about MCP server instructions, workflow-modes.md, spec-workflow.md, moai-memory.md are NOT audit inputs and have been disregarded per protocol. Audit conducted solely against `spec.md` and `research.md`.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-I18N-001 through REQ-I18N-018 are sequential with no gaps and no duplicates, distributed across Ubiquitous (4.1 L176), Event-Driven (4.2 L186), State-Driven (4.3 L198), Unwanted (4.4 L206), Optional (4.5 L216). 18 total REQs, zero-padded consistently.

- **[FAIL] MP-2 EARS format compliance**:
  - All 15 acceptance criteria (AC-I18N-001..015, L226–299) are written in Given/When/Then test-scenario form, NOT one of the five EARS patterns. Per rubric M3 0.50 band: "Given/When/Then test scenarios mislabeled as EARS" is an explicit failure indicator.
  - REQ-I18N-013 (spec.md:L208), REQ-I18N-015 (L212), REQ-I18N-016 (L214) use bare "shall not" without the canonical Unwanted "If [undesired condition], then the [system] shall [response]" construct. Only REQ-I18N-014 (L210) follows proper Unwanted form.
  - REQ-I18N-018 (L220) uses "the UI may offer" inside an Optional pattern. "may" weakens the requirement to optional behavior with no testable trigger condition — either the condition for offering must be stated, or the verb must be "shall".

- **[FAIL] MP-3 YAML frontmatter validity**:
  - `created_at` field is MISSING. Frontmatter (L5) uses `created: 2026-04-22` (plain `created`, not `created_at`).
  - `labels` field is MISSING entirely. Frontmatter contains `lifecycle: spec-anchored` but `labels` (array or string) is absent.
  - Two required fields missing = FAIL per MP-3 strict reading.

- **[N/A] MP-4 Section 22 language neutrality**: N/A — this SPEC concerns UI internationalization across human languages (en, ko, ja, zh-CN, ...), NOT multi-language programming-tool coverage. The 16-programming-language enumeration criterion does not apply. The SPEC targets Go backend + TypeScript frontend, which is the project stack, not template-bound multi-tool content.

---

## Category Scores

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band (minor ambiguity in 1–2 requirements) | REQ-001..012 unambiguous; REQ-013/015/016 weak "shall not" form; REQ-018 "may" ambiguous (L220); REQ-016 vs REQ-008 tension on network I/O (see D9) |
| Completeness | 0.50 | 0.50 band (multiple sections substantively empty OR frontmatter missing fields) | Frontmatter missing `created_at` + `labels` (L2–13); no explicit fallback chain specification (research §1.1 mentions `fr-CA → fr → en` but SPEC only mentions `en-US` fallback at L188); no context-dependent translation support specified; gender handling mentioned as motivation (L80) but no REQ/AC |
| Testability | 0.75 | 0.75 band (one AC not precisely binary-testable) | ACs 001–015 mostly concrete with exact expected values; REQ-018 "may offer" untestable (L220); REQ-011 "synchronous fallback" has no AC to verify sync semantics; AC-I18N-015 uses "최소 16개" which is measurable (L299) |
| Traceability | 0.50 | 0.50 band (multiple REQs lack ACs) | Zero of 15 ACs cite "REQ-I18N-XXX" explicitly — traceability is implicit only. Six REQs have NO corresponding AC: REQ-003 (UTF-8/BOM/CRLF rejection), REQ-011 (sync fallback during async load), REQ-014 (YAML type mismatch), REQ-015 (no PII to LLM), REQ-016 (no network in production), REQ-017 (CLI override). REQ-018 has no AC but is "Optional may" so partially justifiable. |

---

## Defects Found

**D1. spec.md:L5 — Frontmatter field name mismatch — Severity: critical (MP-3)**
`created: 2026-04-22` must be renamed to `created_at: "2026-04-22"` per MP-3 canonical field set. Per project audit protocol, `created` and `created_at` are not interchangeable.

**D2. spec.md:L2–13 — `labels` field entirely missing — Severity: critical (MP-3)**
YAML frontmatter has `lifecycle: spec-anchored` but no `labels` array/string. Required field absent.

**D3. spec.md:L226–299 — All 15 ACs use Given/When/Then, not EARS — Severity: critical (MP-2)**
AC-I18N-001..015 are test-scenario format. Per rubric M3, Given/When/Then is explicitly enumerated as the 0.50 band failure case. MP-2 requires every AC to match one of the five EARS patterns.

**D4. spec.md:L208, L212, L214 — REQ-013/015/016 use bare "shall not" without "If...then..." — Severity: major (MP-2)**
Canonical Unwanted form is "If [undesired condition], then the [system] shall [response]". REQ-014 (L210) models this correctly; the other three do not.

**D5. spec.md:L220 — REQ-I18N-018 uses "may offer" — Severity: major**
"may offer" is not a testable requirement. Either state the condition under which the UI "shall offer" the button, or remove the requirement. Weasel verb in normative text violates RQ-5 / AC-3.

**D6. spec.md:L226–299 — Zero ACs cite REQ-I18N-XXX explicitly — Severity: major**
Traceability violation. No AC contains "REQ-I18N-" string. Reverse mapping is done only by implicit topic match, which fails AC-4.

**D7. spec.md — Six REQs have no corresponding AC — Severity: major**
- REQ-I18N-003 (L182, UTF-8/BOM/CRLF rejection) — Section 6.9 lists `TestLoader_RejectCRLF_BOM` as a TDD RED test, but no AC formalizes the acceptance criterion.
- REQ-I18N-011 (L202, synchronous English fallback during async bundle load) — no AC.
- REQ-I18N-014 (L210, YAML type mismatch tolerated without crash) — no AC.
- REQ-I18N-015 (L212, no PII to LLM pipeline) — no AC; security-critical requirement should have explicit AC with negative test.
- REQ-I18N-016 (L214, no network I/O in production) — no AC.
- REQ-I18N-017 (L218, CLI `goose i18n override` flag) — no AC.

**D8. spec.md:L214 vs L194 — REQ-I18N-016 ↔ REQ-I18N-008 potential contradiction — Severity: major (CN-1)**
REQ-016 states "The i18n system shall not depend on network I/O in production; all Tier 1/Tier 2 bundles are bundled at build time." REQ-008 requires the system to "invoke the LLM auto-translation pipeline (ADAPTER-001)" when a Tier 3 language is requested. A user on a production build requesting a Tier 3 language would trigger a network call via ADAPTER-001. The SPEC does not explicitly scope REQ-016 to "Tier 1/Tier 2 loading only" or restrict Tier 3 to development. Resolve by either (a) qualifying REQ-016 to "for Tier 1/Tier 2 bundle loading", or (b) stating that Tier 3 requires explicit user opt-in and network availability.

**D9. spec.md:L188 — Fallback chain under-specified — Severity: major**
REQ-I18N-005 states "if bundle is missing, fall back to `en-US`". research.md §1.1 praises Hermes's regional chain `fr-CA → fr → en`. The SPEC does not require a regional fallback chain. If the user sets `primary_language = "fr-CA"` and only `fr` (not `fr-CA`) bundle exists, behavior is unspecified. Add explicit chain resolution requirement.

**D10. spec.md:L80–84 vs REQs — Gender handling missing from normative text — Severity: minor**
Section 2.4 motivates ICU `select` for gender ("포르투갈어 {gender, select, masculine {...} feminine {...}}"). No REQ or AC formalizes gender handling. Risk R7 (L558) acknowledges gender-neutral language interaction but proposes only documentation, not a requirement. Add REQ for gender-select syntax support.

**D11. spec.md — Context-dependent translation not addressed — Severity: minor**
i18next supports disambiguation via `_context` suffix (e.g., "close" as verb vs. adjective). The SPEC does not address same-key-different-context cases. Given ICU `select` usage, this may be implicit, but REQ-018 could be clarified or a REQ added.

**D12. spec.md:L398 — `document.documentElement.lang = locale.primary_language` with BCP 47 — Severity: minor**
HTML `lang` attribute receives full BCP 47 tag. Script-specific tags (`zh-Hans` vs `zh-Hant`, `sr-Latn` vs `sr-Cyrl`) are discussed in research §12 as an open issue but not specified in SPEC. Low risk because research flags it, but worth tracking.

**D13. spec.md:L154 vs L485 — CI lint exit code inconsistency — Severity: minor**
Section 3.1 Item 9 (L154) states `goose i18n lint` "exit code 1로 CI fail" as blanket statement. Section 6.7 (L485) states "Exit code: 0(pass), 1(Tier 1 누락 또는 구문 오류), 2(Tier 2 누락, WARN만)". These are inconsistent — is exit code 2 still a CI failure or not? AC-I18N-011 (L278) only verifies exit 1 for Tier 1 missing. Clarify which exit codes gate CI.

**D14. spec.md:L534 — LocaleContext.calendar_system listed as consumed but unused — Severity: minor**
Dependencies table (L534) lists calendar_system as consumed from LOCALE-001, but no REQ or AC references calendar-system-dependent behavior (e.g., Japanese Imperial calendar, Thai Buddhist calendar for date formatting). Either add a REQ for calendar rendering or remove from dependency claim.

**D15. spec.md:L3 — `status: Planned` is non-canonical — Severity: minor**
Typical values are draft/active/implemented/deprecated (per MP-3 definition). "Planned" is a string so MP-3 passes on type, but not on conventional value set.

---

## Chain-of-Verification Pass

Second-pass findings (after re-reading each section carefully rather than skimming):

1. **REQ number sequencing verified end-to-end**, not just spot-checked. Re-counted: 001, 002, 003, 004 (Ubiquitous) + 005, 006, 007, 008, 009 (Event-Driven) + 010, 011, 012 (State-Driven) + 013, 014, 015, 016 (Unwanted) + 017, 018 (Optional) = 18 sequential. No gaps.

2. **Traceability re-verified by scanning every AC for "REQ-I18N" string**: zero matches. First-pass conclusion confirmed — implicit mapping only.

3. **Every REQ checked for AC coverage by keyword match**: newly confirmed that REQ-011 (sync fallback) has no test. Added to D7.

4. **Exclusions section re-read for specificity**: 11 specific exclusions (L593–603), each names what is NOT built and references where it belongs (v1.5+, other SPECs). PASS.

5. **Contradiction scan across REQs**: Caught REQ-008 ↔ REQ-016 tension (D8) that was initially overlooked as a simple scope issue.

6. **Second-look defect: AC coverage for REQ-I18N-014 was initially missed in the first pass.** Added to D7. The SPEC only describes YAML error handling behavior in Section 6.3/Section 6.7 but has no AC with a concrete malformed YAML and expected logged error.

7. **Verified rubric M3 anchoring for ACs**: Given/When/Then fits the 0.50 band description exactly ("Given/When/Then test scenarios mislabeled as EARS"). MP-2 FAIL is rubric-anchored, not subjective.

No additional defects beyond those listed in D1–D15. First pass was largely thorough after this second look.

---

## Regression Check

N/A — Iteration 1.

---

## Recommendation

**Verdict: FAIL** (two must-pass failures: MP-2 + MP-3).

Required fixes for iteration 2 (manager-spec):

1. **Frontmatter (D1, D2)**:
   - Rename `created: 2026-04-22` → `created_at: "2026-04-22"` (string form).
   - Add `labels:` with array of relevant tags (e.g., `[i18n, localization, ui, phase-6]`).

2. **ACs in EARS format (D3)**:
   - Either (a) rewrite all 15 ACs to match one of the five EARS patterns, OR (b) reorganize: keep Section 4 EARS REQs as the normative contract, and relabel Section 5 from "Acceptance Criteria" to "Test Scenarios" to clarify that Given/When/Then is test design, not requirement specification. The current structure presents Given/When/Then under an "Acceptance Criteria" heading, which triggers MP-2.
   - Recommend option (b): keep Given/When/Then (it is useful test design) but change the section header and add explicit "Verifies: REQ-I18N-XXX" line per scenario.

3. **REQ wording (D4, D5)**:
   - REQ-I18N-013: rewrite as "If the translator encounters a non-declarative ICU construct (e.g., function call syntax), then the translator shall reject the construct and skip the key."
   - REQ-I18N-015: rewrite as "If the LLM auto-translation pipeline is invoked, then the pipeline shall transmit only the source English string and target language code — no user-identifying context shall be included."
   - REQ-I18N-016: scope explicitly — "The i18n system shall not perform network I/O for Tier 1/Tier 2 bundle loading in production builds; all Tier 1/Tier 2 bundles are bundled at build time." (resolves D8 in the same stroke).
   - REQ-I18N-018: replace "may offer" with "Where the active language is Tier 1 AND the user has enabled feedback (setting X), the UI shall offer..." OR move to Section 11 Risks/Future work if truly optional.

4. **Add missing ACs (D7)**:
   - AC for REQ-003 (BOM/CRLF rejection) — concrete malformed file + expected rejection.
   - AC for REQ-011 (sync English fallback during async load) — expected sync-return behavior.
   - AC for REQ-014 (YAML type mismatch) — malformed YAML + expected log + skip.
   - AC for REQ-015 (no PII to LLM) — inspect outbound request payload, assert only source text + language code.
   - AC for REQ-016 (no network in production) — network isolation test + expected bundle load success.
   - AC for REQ-017 (CLI override) — override file + expected precedence.

5. **Explicit REQ references in ACs (D6)**:
   - Add "Verifies: REQ-I18N-XXX" line to each AC.

6. **Fallback chain specification (D9)**:
   - Add REQ-I18N-019: "When the primary_language bundle is missing but a regional parent (e.g., `fr-CA` → `fr` → `en-US`) exists, the system shall load the regional parent; chain resolution follows BCP 47 truncation rules."

7. **Gender handling (D10)**:
   - Add REQ for ICU `select` gender form support, or explicitly state in Exclusions that gender-aware translation is out of scope for v0.1.

8. **Minor fixes (D11–D15)**:
   - D13: Make exit code table in Section 6.7 consistent with Section 3.1; clarify which codes fail CI.
   - D14: Add REQ for calendar_system rendering OR remove from dependencies.
   - D15: Normalize `status: Planned` → `status: draft` or `status: active`.

After these fixes, re-submit for iteration 2 audit.

---

**End of Review**
