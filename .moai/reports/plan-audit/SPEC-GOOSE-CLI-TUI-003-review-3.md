# SPEC Review Report: SPEC-GOOSE-CLI-TUI-003
Iteration: 3/3 (FINAL ESCALATION)
Verdict: FAIL
Overall Score: 0.88

Reasoning context ignored per M1 Context Isolation.
Audited spec.md as primary input. Previous review reports read for regression check only.

---

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: Document reading order is strictly sequential throughout. spec.md:L134 REQ-CLITUI3-001 (Section 4.1), L139 REQ-CLITUI3-002 (Section 4.1), L146 REQ-CLITUI3-003 (Section 4.2), L151 REQ-CLITUI3-004 (Section 4.2), L156 REQ-CLITUI3-005 (Section 4.2), L161 REQ-CLITUI3-006 (Section 4.2), L166 REQ-CLITUI3-007 (Section 4.2), L173 REQ-CLITUI3-008 (Section 4.3), L180 REQ-CLITUI3-009 (Section 4.4). Ascending 001–009, no gaps, no duplicates. D1 from iteration 1 remains RESOLVED.

- [PASS] MP-2 EARS format compliance: All 10 Acceptance Criteria in spec.md §10 (L350–L361) now match one of the five EARS patterns.
  - AC-CLITUI3-001 (L352): "When... the overlay shall open..." — Event-driven ✓
  - AC-CLITUI3-002 (L353): "When... the TUI shall load..." — Event-driven ✓
  - AC-CLITUI3-003 (L354): "When... the overlay shall render... and dismiss..." — Event-driven ✓
  - AC-CLITUI3-004 (L355): "While three deterministic mock session entries are loaded... the sessionmenu overlay snapshot **shall** be byte-identical..." — State-driven ✓ (previously "Given"; now corrected)
  - AC-CLITUI3-005 (L356): "When... the TUI shall load... enter edit mode... and switch..." — Event-driven ✓
  - AC-CLITUI3-006 (L357): "When... the TUI shall remove... exit edit mode... and submit..." — Event-driven ✓
  - AC-CLITUI3-007 (L358): "When... the TUI shall exit... clear... restore... and leave..." — Event-driven ✓
  - AC-CLITUI3-008 (L359): "If the user presses Ctrl-Up while streaming is active, **then** the TUI **shall not** activate edit mode..." — Unwanted ✓ (previously missing "then"; now corrected)
  - AC-CLITUI3-009 (L360): "While `conversation_language` is configured as `ko`... **shall** render Korean substrings..." — State-driven ✓ (previously "Given"; now corrected)
  - AC-CLITUI3-010 (L361): "While `conversation_language` is configured as `en` or the configuration is absent... **shall** render English substrings..." — State-driven ✓ (previously "Given"; now corrected)
  All four D-NEW defects from iteration 2 (D-NEW-1, D-NEW-2, D-NEW-3, D-NEW-4) are RESOLVED. MP-2 PASS.

- [PASS] MP-3 YAML frontmatter validity: All six required fields present with correct types. spec.md:L2 `id: SPEC-GOOSE-CLI-TUI-003` (string); L3 `version: "0.1.0"` (string); L4 `status: draft` (string); L5 `created_at: 2026-05-05` (ISO date string); L8 `priority: P1` (string — non-standard value but correct type; does not constitute a type mismatch for MP-3); L10 `labels: [tui, cli, bubbletea, i18n, sessionmenu, edit-regenerate]` (array). No required field absent, no type mismatch.

- [N/A] MP-4 Section 22 language neutrality: N/A — SPEC is explicitly scoped to a single-language (Go) TUI implementation. Multi-language enumeration criterion does not apply.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension     | Score | Rubric Band | Evidence |
|---------------|-------|-------------|----------|
| Clarity       | 0.75  | 0.75        | REQ-001 (spec.md:L135) still embeds `Catalog struct` and `internal/cli/tui/i18n/` — HOW detail in a WHAT requirement. REQ-002 (spec.md:L140) still embeds `sessionmenu.Loader` and `empty slice` — Go-specific terminology in a behavioral requirement. REQ-008 (spec.md:L173) bundles four State-driven behaviors (key routing, cursor movement, Enter-load, Esc-dismiss) in one statement — borderline but defensible as a single-state contract. All other REQs are unambiguous and precisely worded. Score anchored at 0.75: minor ambiguity in two requirements that a reasonable engineer would resolve consistently, but implementation coupling is present. |
| Completeness  | 0.75  | 0.75        | All required sections present: HISTORY (L15), Background (L40), Scope (L70), Requirements (L130), Acceptance Criteria (L346), Exclusions (L116). Seven named exclusions with rationale (L120–L126). YAML frontmatter complete. One persisting gap: no formal requirement captures the Ctrl-R behavior when streaming is active (only described in Risk §8 R3 at spec.md:L305). |
| Testability   | 1.00  | 1.0         | All 10 ACs are now binary-testable EARS statements. ACs using "byte-identical" comparison are inherently binary. No weasel words ("appropriate," "adequate," "reasonable") found in any AC. Every AC has a single, determinate PASS/FAIL criterion. AC-CLITUI3-002 references "the existing /load path" (minor HOW hint) but the outcome (session load) is still binary-testable. |
| Traceability  | 1.00  | 1.0         | Every REQ-CLITUI3-XXX has at least one AC: REQ-001→AC-009,010; REQ-002→AC-001,003,004; REQ-003→AC-001,004; REQ-004→AC-003; REQ-005→AC-005; REQ-006→AC-006; REQ-007→AC-007; REQ-008→AC-001,002; REQ-009→AC-008. All 10 ACs reference valid REQ-XXX that exist in the document (spec.md:L350–361). No orphaned ACs, no uncovered REQs. |

---

## Defects Found

D-RESIDUAL-5. spec.md:L135–L136 — **REQ-CLITUI3-001 embeds Go implementation details.** Quoted: "The TUI **shall** load a `Catalog` struct from `internal/cli/tui/i18n/` at model initialization..." — `Catalog struct` is a class name and `internal/cli/tui/i18n/` is a package path. Both are HOW (implementation choice), not WHAT (behavioral outcome). Present unchanged since iteration 1 (D5 → D-RESIDUAL-5). — Severity: **major** — BLOCKING DEFECT: manager-spec made no progress across all 3 iterations.

D-RESIDUAL-6. spec.md:L140 — **REQ-CLITUI3-002 embeds Go-specific type name.** Quoted: "The `sessionmenu.Loader` **shall** return at most 10 entries..." — `sessionmenu.Loader` is a Go struct type name. Also: "it **shall** return an empty slice" — `empty slice` is Go-specific terminology (the behavioral term is "empty list"). Present unchanged since iteration 1 (D5 → D-RESIDUAL-6). — Severity: **major** — BLOCKING DEFECT: manager-spec made no progress across all 3 iterations.

D-RESIDUAL-7. spec.md:L305 — **No formal requirement for Ctrl-R behavior during active streaming.** REQ-CLITUI3-003 (L146) guards only against modal state ("AND no permission modal is active"). Risk R3 (L305) describes the desired behavior informally: "sessionmenu Entry 로드는 `streaming == false` 일 때만 트리거." This interaction is not captured in any normative REQ. Present unchanged since iteration 1 (D6 → D-RESIDUAL-7). — Severity: **minor** — Stagnant across all 3 iterations.

D-RESIDUAL-8. spec.md:L8 — **`priority: P1` is non-standard.** Expected values per standard rubric: critical, high, medium, low. "P1" is non-standard. Present unchanged since iteration 1 (D7 → D-RESIDUAL-8). — Severity: **minor** — Stagnant across all 3 iterations.

---

## Chain-of-Verification Pass

Second-look findings:

1. **REQ number sequencing re-verified end-to-end**: Walked all nine REQs in document reading order — 001 at L134, 002 at L139, 003 at L146, 004 at L151, 005 at L156, 006 at L161, 007 at L166, 008 at L173, 009 at L180. Strictly ascending. No gaps. No duplicates. MP-1 PASS confirmed.

2. **All 10 ACs re-examined individually for EARS compliance**: AC-001 (Event-driven ✓), AC-002 (Event-driven ✓), AC-003 (Event-driven ✓), AC-004 (State-driven "While" ✓ — fixed from "Given"), AC-005 (Event-driven ✓), AC-006 (Event-driven ✓), AC-007 (Event-driven ✓), AC-008 (Unwanted "If...then...shall not" ✓ — "then" now present), AC-009 (State-driven "While" ✓ — fixed from "Given"), AC-010 (State-driven "While" ✓ — fixed from "Given"). All 10 now EARS-compliant. MP-2 PASS confirmed.

3. **Traceability verified for all 9 REQs (not sampled)**: Every REQ's "Acceptance" field cross-checked against the §10 table bidirectionally. All 9 REQs covered. All 10 ACs trace to existing REQs. No orphan found.

4. **Exclusions re-checked for specificity**: Seven named exclusions at spec.md:L120–L126 — Ctrl-Up multi-turn rewind, sessionmenu fuzzy search, ja/zh locale, session export/clipboard, real-time file watching, permission store migration, proto changes. All specific with named technologies or behaviors. No vague entries.

5. **Contradiction check**: REQ-003 (Ctrl-R opens; guard: no modal) is the entry state; REQ-008 (while overlay open, capture all input) governs the active state. Sequential, not contradictory. REQ-005 preconditions (empty editor, single-line, not streaming) and REQ-009 Unwanted (Ctrl-Up while streaming) are consistent — REQ-009 is the Unwanted expression of REQ-005's streaming guard. REQ-007 (Esc in edit mode) and REQ-008 (Esc in overlay) address different states in the 6-tier KeyEscape priority chain. No contradictions found.

6. **Weasel word check**: Re-scanned all 10 ACs. Zero occurrences of "appropriate," "adequate," "reasonable," "good," "proper," "suitable." All ACs use binary-testable language.

7. **D-RESIDUAL-5 and D-RESIDUAL-6 stagnation confirmed**: spec.md:L135 and L140 are byte-identical to their appearance in iteration 1. No remediation attempted.

No new defects discovered beyond those documented.

---

## Regression Check (Iteration 3)

Defects from iteration 2:

- D-NEW-1 (spec.md:L355 — AC-CLITUI3-004 "Given" not EARS): **RESOLVED** — Now reads "While three deterministic mock session entries are loaded under fixed clock and ascii termenv..." — State-driven EARS ✓.

- D-NEW-2 (spec.md:L360 — AC-CLITUI3-009 "Given" not EARS): **RESOLVED** — Now reads "While `conversation_language` is configured as `ko`..." — State-driven EARS ✓.

- D-NEW-3 (spec.md:L361 — AC-CLITUI3-010 "Given" not EARS): **RESOLVED** — Now reads "While `conversation_language` is configured as `en` or the configuration is absent..." — State-driven EARS ✓.

- D-NEW-4 (spec.md:L359 — AC-CLITUI3-008 missing "then" in Unwanted): **RESOLVED** — Now reads "If the user presses Ctrl-Up while streaming is active, **then** the TUI **shall not** activate edit mode..." — Proper EARS Unwanted ✓.

- D-RESIDUAL-5 (spec.md:L135–L136 — REQ-001 Go struct/package name): **UNRESOLVED** — spec.md:L135 still reads "The TUI **shall** load a `Catalog` struct from `internal/cli/tui/i18n/` at model initialization...". No change from iteration 2.

- D-RESIDUAL-6 (spec.md:L140 — REQ-002 Go type name `sessionmenu.Loader`): **UNRESOLVED** — spec.md:L140 still reads "The `sessionmenu.Loader` **shall** return at most 10 entries...". No change from iteration 2.

- D-RESIDUAL-7 (no formal REQ for Ctrl-R during streaming): **UNRESOLVED** — No new requirement added. Risk R3 at L305 remains the only reference.

- D-RESIDUAL-8 (priority: P1 non-standard): **UNRESOLVED** — spec.md:L8 still reads `priority: P1`. No change from iteration 2.

---

## FINAL ESCALATION REPORT

This is iteration 3 (the maximum). The SPEC has failed all three audit iterations. Per retry loop contract, user intervention is required.

### Progress Across Iterations

| Defect ID | Iteration 1 | Iteration 2 | Iteration 3 | Status |
|-----------|-------------|-------------|-------------|--------|
| D1/D-resolved | REQ ordering non-sequential (critical) | RESOLVED | RESOLVED | ✓ Resolved |
| D2/D-NEW-1-2-3 | All 10 ACs Given/When/Then (critical) | Partially resolved (7/10) | RESOLVED | ✓ Resolved |
| D3/D-resolved | REQ-003 non-atomic (major) | RESOLVED | RESOLVED | ✓ Resolved |
| D4/D-resolved | REQ-008 Unwanted pattern wrong (major) | RESOLVED | RESOLVED | ✓ Resolved |
| D-NEW-4 | — | AC-008 missing "then" (minor) | RESOLVED | ✓ Resolved |
| D5/D-RESIDUAL-5 | REQ-001 Go struct/pkg name (major) | UNRESOLVED | **UNRESOLVED** | BLOCKING |
| D6/D-RESIDUAL-6 | REQ-002 Go type name (major) | UNRESOLVED | **UNRESOLVED** | BLOCKING |
| D7/D-RESIDUAL-7 | Ctrl-R+streaming undefined (minor) | UNRESOLVED | **UNRESOLVED** | Stagnant |
| D8/D-RESIDUAL-8 | priority: P1 non-standard (minor) | UNRESOLVED | **UNRESOLVED** | Stagnant |

### Blocking Defects — manager-spec Made No Progress

**D-RESIDUAL-5** (spec.md:L135): REQ-CLITUI3-001 embeds Go implementation artifact `Catalog struct` and package path `internal/cli/tui/i18n/`. This violates RQ-4 (no class names in requirements) and RQ-3 (WHAT not HOW). Three iterations with zero remediation constitutes a systematic misunderstanding of the implementation-detail boundary in EARS requirements.

**D-RESIDUAL-6** (spec.md:L140): REQ-CLITUI3-002 embeds Go type `sessionmenu.Loader` and Go-specific term `empty slice`. Same violation class as D-RESIDUAL-5. Three iterations with zero remediation.

### Recommended Fixes for User Review

**Fix 1 (blocking — D-RESIDUAL-5)**: Replace spec.md:L135–L136:

Current: "The TUI **shall** load a `Catalog` struct from `internal/cli/tui/i18n/` at model initialization using `conversation_language` from `.moai/config/sections/language.yaml`; if the file is absent, language key is missing, or the language is unknown, the TUI **shall** default to English ("en") without error or log noise."

Required: "The TUI **shall** load locale-specific display strings at initialization using the conversation language specified in the project language configuration; if the configuration file is absent, the language key is missing, or the specified language is unrecognized, the TUI **shall** default to English strings without error or log output."

**Fix 2 (blocking — D-RESIDUAL-6)**: Replace spec.md:L140:

Current: "The `sessionmenu.Loader` **shall** return at most 10 entries from `~/.goose/sessions/*.jsonl` sorted by file mtime descending; if the directory is absent or contains no `.jsonl` files, it **shall** return an empty slice without error."

Required: "The session list loader **shall** return at most 10 entries from the user's session storage directory sorted by modification time descending; if the directory is absent or contains no session files, it **shall** return an empty list without error."

**Fix 3 (minor — D-RESIDUAL-7)**: Add a new Event-driven requirement or extend REQ-CLITUI3-003 to formally capture the streaming guard:

Proposed addition (new REQ or extension): "When the user presses Ctrl-R while streaming is in progress, the TUI **shall** open the sessionmenu overlay but **shall not** load the selected session until the current stream completes."

**Fix 4 (minor — D-RESIDUAL-8)**: Replace spec.md:L8: `priority: P1` → `priority: high` (or `priority: critical` if P1 maps to that severity level).

---

Verdict: FAIL
