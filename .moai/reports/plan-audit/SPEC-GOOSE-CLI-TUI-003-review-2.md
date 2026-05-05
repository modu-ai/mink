# SPEC Review Report: SPEC-GOOSE-CLI-TUI-003
Iteration: 2/3
Verdict: FAIL
Overall Score: 0.81

Reasoning context ignored per M1 Context Isolation.
Audited spec.md as primary input. Previous review report read for regression check only.

---

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: Document reading order is now strictly sequential. spec.md:L134 REQ-CLITUI3-001, L139 REQ-CLITUI3-002, L146 REQ-CLITUI3-003, L151 REQ-CLITUI3-004, L156 REQ-CLITUI3-005, L161 REQ-CLITUI3-006, L166 REQ-CLITUI3-007, L173 REQ-CLITUI3-008, L180 REQ-CLITUI3-009. No gaps, no duplicates, no out-of-order entries. D1 is resolved.

- [FAIL] MP-2 EARS format compliance: Three of ten Acceptance Criteria in spec.md §10 do not match any EARS pattern. AC-CLITUI3-004 (spec.md:L355), AC-CLITUI3-009 (spec.md:L360), and AC-CLITUI3-010 (spec.md:L361) all open with "Given [precondition], the [system] shall..." — "Given" is a Given/When/Then keyword and is not one of the five EARS trigger words (When / While / Where / If / [none for Ubiquitous]). Per MP-2: "Given/When/Then test scenarios mislabeled as EARS = FAIL." The remaining seven ACs (AC-001, AC-002, AC-003, AC-005, AC-006, AC-007, AC-008) now correctly use EARS Event-driven or Unwanted patterns; however, every criterion must comply and three do not. AC-CLITUI3-008 (spec.md:L359) additionally omits the required "then" connector in its "If...shall" construction, a minor structural deviation from the EARS Unwanted pattern.

- [PASS] MP-3 YAML frontmatter validity: All six required fields are present with correct types. spec.md:L2 `id: SPEC-GOOSE-CLI-TUI-003` (string); L3 `version: "0.1.0"` (string); L4 `status: draft` (string); L5 `created_at: 2026-05-05` (ISO date string); L8 `priority: P1` (string — non-standard value but correct type); L10 `labels: [tui, cli, bubbletea, i18n, sessionmenu, edit-regenerate]` (array). No required field absent, no type mismatch.

- [N/A] MP-4 Section 22 language neutrality: N/A — SPEC is explicitly scoped to a single-language (Go) TUI project. Multi-language enumeration criterion does not apply.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension     | Score | Rubric Band | Evidence |
|---------------|-------|-------------|----------|
| Clarity       | 0.75  | 0.75        | REQ-CLITUI3-003 (spec.md:L146) is now atomic — navigation/selection behaviors moved to REQ-CLITUI3-008. REQ-CLITUI3-009 (spec.md:L180) correctly expresses Unwanted with "If...then...shall not". Minor ambiguity remains in REQ-CLITUI3-001 and REQ-CLITUI3-002 which still reference Go struct names (`Catalog struct`, `sessionmenu.Loader`) as behavioral subjects. REQ-CLITUI3-008 (spec.md:L173) bundles four related state-driven behaviors (key routing, cursor movement, Enter-load, Esc-dismiss) in one requirement, which is borderline but defensible for a single active state. |
| Completeness  | 0.75  | 0.75        | All required sections present: HISTORY (spec.md:L15), Background (L40), Scope (L70), Requirements (L130), Acceptance Criteria (L346), Exclusions (L116). Seven specific named exclusions with rationale. YAML frontmatter complete. One gap: no formal requirement captures Ctrl-R behavior when streaming is in progress (only mentioned in Risk §8 R3 at spec.md:L305 and integration test notes). |
| Testability   | 0.75  | 0.75        | Seven of ten ACs are now binary-testable EARS statements. AC-001 through AC-003 and AC-005 through AC-008 are precise and determinate. AC-004, AC-009, AC-010 use golden-file comparison ("byte-identical") which is inherently binary-testable, but the "Given" precondition framing is not EARS-compliant. No weasel words found in the compliant ACs. |
| Traceability  | 1.00  | 1.0         | Every REQ-CLITUI3-XXX has at least one AC: REQ-001→AC-009,010; REQ-002→AC-001,003,004; REQ-003→AC-001,004; REQ-004→AC-003; REQ-005→AC-005; REQ-006→AC-006; REQ-007→AC-007; REQ-008→AC-001,002; REQ-009→AC-008 (spec.md:L134-183). All 10 ACs reference valid REQs that exist in the document (spec.md:L350-361). No orphaned ACs, no uncovered REQs. Traceability after renumbering is fully consistent. |

---

## Defects Found

D-NEW-1. spec.md:L355 — AC-CLITUI3-004 opens with "Given three mock entries under deterministic test setup..." — "Given" is not an EARS keyword. No EARS pattern uses "Given" as a trigger. Must be recast as State-driven ("While...") or Event-driven ("When...") EARS pattern. — Severity: **critical** (MP-2)

D-NEW-2. spec.md:L360 — AC-CLITUI3-009 opens with "Given the conversation language is set to `ko`..." — same defect as D-NEW-1. — Severity: **critical** (MP-2)

D-NEW-3. spec.md:L361 — AC-CLITUI3-010 opens with "Given the conversation language is set to `en` or the language configuration is absent..." — same defect as D-NEW-1. — Severity: **critical** (MP-2)

D-NEW-4. spec.md:L359 — AC-CLITUI3-008 uses "If the user presses Ctrl-Up while streaming is in progress, the TUI shall ignore..." — the EARS Unwanted pattern requires "If [condition], **then** the [system] shall [response]". The "then" connector is absent. REQ-CLITUI3-009 (spec.md:L180) correctly includes "then"; AC-008 does not. — Severity: **minor** (MP-2 structural)

D-RESIDUAL-5. spec.md:L135-136 — REQ-CLITUI3-001 still embeds Go implementation details: "`Catalog` struct from `internal/cli/tui/i18n/`". The package path and struct name are HOW, not WHAT. Behavioral expression: "The TUI shall load locale-specific display strings at initialization using the configured conversation language; if the configuration is absent or unrecognized, the TUI shall use English strings." — Severity: **major** (residual from D5, unresolved)

D-RESIDUAL-6. spec.md:L140-141 — REQ-CLITUI3-002 still references `sessionmenu.Loader` (Go struct name) and `empty slice` (Go-specific terminology). Behavioral expression: "The session list loader shall return at most 10 sessions from the user's session storage directory sorted by recency; if no sessions exist or the directory is absent, it shall return an empty list without error." — Severity: **major** (residual from D5, unresolved)

D-RESIDUAL-7. spec.md:L305 — Ctrl-R behavior when streaming is active is described only in Risk R3 and not captured as a formal requirement. No REQ addresses what happens when Ctrl-R is pressed during active streaming. — Severity: **minor** (D6 from iteration 1, unresolved)

D-RESIDUAL-8. spec.md:L8 — `priority: P1` is non-standard. Expected values per rubric are: critical, high, medium, low. — Severity: **minor** (D7 from iteration 1, unresolved)

---

## Chain-of-Verification Pass

Second-look findings:

1. **REQ sequencing end-to-end re-verified**: Walked all nine REQs in document reading order — 001 at L134, 002 at L139, 003 at L146, 004 at L151, 005 at L156, 006 at L161, 007 at L166, 008 at L173, 009 at L180. Strictly ascending. No gaps. D1 RESOLVED confirmed.

2. **All 10 ACs re-examined individually for EARS compliance**: AC-001 (Event-driven ✓), AC-002 (Event-driven ✓), AC-003 (Event-driven ✓), AC-004 (Given prefix ✗), AC-005 (Event-driven ✓), AC-006 (Event-driven ✓), AC-007 (Event-driven ✓), AC-008 (If...shall, missing "then" — minor structural gap), AC-009 (Given prefix ✗), AC-010 (Given prefix ✗). Three clear non-EARS ACs confirmed.

3. **Traceability verified for all 9 REQs**: Each REQ's "Acceptance" inline field cross-checked against the §10 table. All mappings are bidirectionally consistent. After renumbering (008↔009 swap), all references updated correctly.

4. **Exclusions re-checked for specificity**: Seven named exclusions at spec.md:L120-126 — Ctrl-Up multi-turn rewind, sessionmenu fuzzy search, ja/zh locale, session export, real-time file watching, permission store migration, proto changes. All specific with named technologies or behaviors. No vague entries.

5. **Contradiction check**: REQ-003 (Ctrl-R opens, guard: no modal), REQ-008 (while open, capture all keys), REQ-009 (Ctrl-Up while streaming = no-op), REQ-005 (Ctrl-Up guards: empty editor, single-line, not streaming). REQ-005 and REQ-009 are consistent (REQ-009 is the Unwanted statement of REQ-005's streaming guard). No contradictions found.

6. **REQ-008 bundling re-assessed**: State-driven requirement bundles key-routing, cursor-movement, Enter-load, and Esc-dismiss in one statement. All four are behaviors of the same state (overlay open). Per EARS State-driven, "While [condition], the [system] shall [response]" — one condition with multiple behaviors in the response is borderline but architecturally defensible as a single-state behavioral contract. Not escalated to defect.

No new defects beyond those documented above.

---

## Regression Check (Iteration 2)

Defects from iteration 1:

- D1 (spec.md:L173, L180 — REQ ordering non-sequential): **RESOLVED** — Document now reads 001→009 in strict ascending order. REQ-008 is State-driven (L173) and REQ-009 is Unwanted (L180), correcting the previous inversion.

- D2 (All 10 ACs use Given/When/Then — MP-2 failure): **PARTIALLY RESOLVED** — 7 of 10 ACs now use EARS patterns. AC-004, AC-009, AC-010 still use "Given" (not EARS). MP-2 remains FAIL. New defect IDs D-NEW-1, D-NEW-2, D-NEW-3 capture the specific residual violations.

- D3 (REQ-003 non-atomic — bundles 6 behaviors): **RESOLVED** — REQ-CLITUI3-003 (spec.md:L146) now specifies only the opening behavior (overlay opens, populated, cursor at first entry). Navigation/selection/dismiss behaviors moved to REQ-CLITUI3-008 (State-driven).

- D4 (REQ-008 Unwanted pattern incorrect — "shall not...when"): **RESOLVED** — REQ-CLITUI3-009 (spec.md:L180) now correctly uses "If...then the TUI shall not..." — proper EARS Unwanted structure.

- D5 (REQs embed implementation details): **PARTIALLY RESOLVED** — editingMessageIndex variable name, array index expressions, and field-level references removed from REQ-005 through REQ-007. REQ-001 and REQ-002 still contain Go struct names and package paths (D-RESIDUAL-5, D-RESIDUAL-6). Severity reduced but not eliminated.

- D6 (Ctrl-R + streaming undefined in formal requirements): **UNRESOLVED** — No formal requirement added for this interaction. Risk R3 (spec.md:L305) still captures it informally only. Retained as D-RESIDUAL-7.

- D7 (priority: P1 non-standard): **UNRESOLVED** — spec.md:L8 still reads `priority: P1`. Retained as D-RESIDUAL-8.

---

## Recommendation

The sole must-pass failure blocking approval is MP-2: three ACs use "Given" (a Given/When/Then keyword, not an EARS keyword).

**Fix 1 — MP-2 critical (D-NEW-1, D-NEW-2, D-NEW-3): Recast AC-004, AC-009, AC-010 using EARS patterns.**

AC-CLITUI3-004 (spec.md:L355) — Replace "Given three mock entries under deterministic test setup (en locale, ascii termenv, fixed clock, width 80), the rendered overlay shall be byte-identical to `session_menu_open.golden` on linux and macOS." with a State-driven EARS statement: "While the sessionmenu overlay is displaying three entries under a standard en-locale 80-column terminal environment, the rendered output shall be byte-identical to `session_menu_open.golden` on linux and macOS."

AC-CLITUI3-009 (spec.md:L360) — Replace "Given the conversation language is set to `ko`, the four surface snapshots... shall be byte-identical to their `*_ko.golden` counterparts on linux and macOS." with: "While the conversation language is configured as `ko`, the four user-facing surface snapshots (statusbar idle, /help, permission modal, session menu open) shall be byte-identical to their `*_ko.golden` counterparts on linux and macOS."

AC-CLITUI3-010 (spec.md:L361) — Replace "Given the conversation language is set to `en` or the language configuration is absent or unrecognized, the four surface snapshots shall be byte-identical to their `*_en.golden` counterparts on linux and macOS." with: "While the conversation language is configured as `en` or the language configuration is absent or unrecognized, the four user-facing surface snapshots shall be byte-identical to their `*_en.golden` counterparts on linux and macOS."

**Fix 2 — MP-2 minor (D-NEW-4): Add "then" to AC-CLITUI3-008 (spec.md:L359).**

Replace: "If the user presses `Ctrl-Up` while streaming is in progress, the TUI shall ignore..."
With: "If the user presses `Ctrl-Up` while streaming is in progress, then the TUI shall ignore..."

**Fix 3 — Major residual (D-RESIDUAL-5): Remove Go struct/package references from REQ-001.**

spec.md:L135-136 — Replace the reference to "`Catalog` struct from `internal/cli/tui/i18n/`" with behavioral language: "The TUI shall load locale-specific display strings at initialization using the configured conversation language; if the language configuration is absent or the language is unrecognized, the TUI shall use English strings without error."

**Fix 4 — Major residual (D-RESIDUAL-6): Remove Go-specific terms from REQ-002.**

spec.md:L140-141 — Replace `sessionmenu.Loader` and `empty slice` with behavioral language: "The session list loader shall return at most 10 sessions from the user's session storage directory sorted by modification time descending; if the directory is absent or contains no sessions, the loader shall return an empty list without error."

**Fix 5 — Minor (D-RESIDUAL-7): Add a formal requirement for Ctrl-R during streaming.**

Add a new event-driven REQ or extend REQ-CLITUI3-003's guards: "When the user presses Ctrl-R while streaming is in progress, the TUI shall display the session overlay but shall not allow session loading until the stream completes."

**Fix 6 — Minor (D-RESIDUAL-8): Correct priority value.**

spec.md:L8 — Change `priority: P1` to `priority: high` (or `critical` if P1 maps to that level).

---

Verdict: FAIL
