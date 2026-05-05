# SPEC Review Report: SPEC-GOOSE-CLI-TUI-003
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.52

> Reasoning context ignored per M1 Context Isolation.
> Audited spec.md as primary input; acceptance.md read for cross-reference only.

---

## Must-Pass Results

- [FAIL] MP-1 REQ number consistency: REQ numbers 001-009 are all present with no gaps or duplicates, satisfying the completeness requirement. However, the in-document ordering is non-sequential: REQ-CLITUI3-009 appears at spec.md:L173 (Section 4.3 State-Driven) BEFORE REQ-CLITUI3-008 at spec.md:L180 (Section 4.4 Unwanted Behavior). The pattern 001, 002, 003, 004, 005, 006, 007, **009**, **008** violates sequential ordering. MP-1 requires sequential ordering with no breaks.

- [FAIL] MP-2 EARS format compliance: All 10 Acceptance Criteria fail EARS compliance. In spec.md §10 (L348-359), the AC column contains brief informal descriptions (e.g., "Ctrl-R 로 sessionmenu overlay 열림 (3 entry mtime sort)") — not EARS patterns. The full ACs in acceptance.md use Given/When/Then format throughout (acceptance.md:L79, L107, L131, L182, L214, L245, L276, L294, L321, L345). Per M3 rubric: "Given/When/Then test scenarios mislabeled as EARS" = FAIL. Additionally, two REQs violate EARS structural rules: REQ-CLITUI3-003 (spec.md:L147) bundles 5+ distinct behaviors in one event-driven statement (non-atomic); REQ-CLITUI3-008 (spec.md:L181) uses "The TUI shall not...when" — not a recognized EARS pattern (Unwanted requires: "If [undesired condition], then the [system] shall [response]").

- [PASS] MP-3 YAML frontmatter validity: All six required fields are present with correct types. spec.md:L2 `id: SPEC-GOOSE-CLI-TUI-003` (string); L3 `version: "0.1.0"` (string); L4 `status: draft` (string); L5 `created_at: 2026-05-05` (ISO date string); L8 `priority: P1` (string — type correct, value is non-standard but type matches); L10 `labels: [tui, cli, bubbletea, i18n, sessionmenu, edit-regenerate]` (array). No required field is absent; no type mismatch.

- [N/A] MP-4 Section 22 language neutrality: N/A — This SPEC is explicitly scoped to a single-language (Go) project. All library references (charmbracelet/bubbletea, lipgloss, gopkg.in/yaml.v3) are Go-specific dependencies consistent with a Go-only implementation scope. Multi-language enumeration criterion does not apply.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension     | Score | Rubric Band | Evidence |
|---------------|-------|-------------|----------|
| Clarity       | 0.50  | 0.50        | Multiple requirements require interpretation. REQ-CLITUI3-003 (spec.md:L147) bundles 6 behaviors. REQ-CLITUI3-001/002 mix behavioral outcomes with implementation paths (`internal/cli/tui/i18n/`, `sessionmenu.Loader`). Ctrl-R + streaming interaction is undefined in requirements (only in Risk §8 R3 and acceptance.md §2.2). REQ-CLITUI3-004 and REQ-CLITUI3-007 are clear and precise. |
| Completeness  | 0.75  | 0.75        | All required sections present: HISTORY (spec.md:L15-19), Background (L40-67), Scope (L70-127), Requirements (L130-184), Acceptance Criteria (L346-360), Exclusions (L116-126). Seven specific named exclusions. YAML frontmatter complete. One deficiency: streaming + Ctrl-R guard behavior not captured in formal requirements (only in Risk and integration test notes). |
| Testability   | 0.50  | 0.50        | ACs in acceptance.md contain concrete test scenarios with binary pass/fail criteria and specific preconditions. However, they validate implementation-level state variables (`editingMessageIndex`, `sessionMenuState.open`) which introduces implementation coupling. Performance gates (acceptance.md:§0.3) have specific numeric thresholds. But Given/When/Then format mislabeled as acceptance criteria instead of EARS means testability cannot be fully evaluated against the EARS rubric. |
| Traceability  | 1.00  | 1.0         | Every REQ-CLITUI3-XXX has at least one AC: REQ-001→AC-009,010; REQ-002→AC-001,003,004; REQ-003→AC-001,002,004; REQ-004→AC-003; REQ-005→AC-005; REQ-006→AC-006; REQ-007→AC-007; REQ-008→AC-008; REQ-009→AC-001,002 (spec.md:L134-184). All 10 ACs reference valid REQ-XXX that exist (spec.md:L348-359). No orphaned ACs, no uncovered REQs. |

---

## Defects Found

D1. spec.md:L173, L180 — **REQ number ordering is non-sequential in document.** REQ-CLITUI3-009 (L173, Section 4.3 State-Driven) appears before REQ-CLITUI3-008 (L180, Section 4.4 Unwanted Behavior). Document order: 001, 002, 003, 004, 005, 006, 007, **009, 008**. Sequential ordering requires ascending appearance order. — Severity: **critical** (MP-1)

D2. spec.md:L348-359; acceptance.md:L79, L107, L131, L182, L214, L245, L276, L294, L321, L345 — **All 10 Acceptance Criteria use Given/When/Then format, not EARS.** The spec.md §10 AC table contains brief informal descriptions. The detailed ACs in acceptance.md use Given/When/Then throughout. Per MP-2 rubric, Given/When/Then test scenarios mislabeled as EARS is a disqualifying failure. None of the 10 ACs match any of the 5 EARS patterns (Ubiquitous, Event-driven, State-driven, Optional, Unwanted). — Severity: **critical** (MP-2)

D3. spec.md:L147-148 — **REQ-CLITUI3-003 is non-atomic: bundles 6 distinct behaviors in one event-driven requirement.** A single "When Ctrl-R" EARS entry specifies: (a) overlay opens with entries, (b) arrow keys move cursor, (c) cursor is clamped, (d) Enter loads session, (e) Esc dismisses without side effect, (f) main editor disabled while open. EARS event-driven requires one trigger → one response. Six behaviors cannot be independently tested from one REQ. — Severity: **major**

D4. spec.md:L181-183 — **REQ-CLITUI3-008 does not conform to EARS Unwanted pattern.** "The TUI **shall not** activate edit mode when `streaming == true`" uses a "shall not...when" construction. Proper EARS Unwanted: "If [undesired condition], then the [system] shall [response]." The current form lacks the required "If...then" structure and inverts the expected pattern. — Severity: **major**

D5. spec.md:L135, L140, L157-158, L162-163, L181 — **Multiple REQs embed implementation details (HOW), not behavioral outcomes (WHAT/WHY).** Examples: REQ-CLITUI3-001 (L135) specifies `internal/cli/tui/i18n/` package path and `Catalog struct` name; REQ-CLITUI3-002 (L140) specifies `sessionmenu.Loader` Go type; REQ-CLITUI3-005 (L157) specifies `messages[lastUserIndex].Content` field access and `editingMessageIndex = lastUserIndex` variable assignment; REQ-CLITUI3-006 (L162-163) specifies `editingMessageIndex >= 0` check and `messages[editingMessageIndex+1]` array indexing; REQ-CLITUI3-008 (L181) specifies `editingMessageIndex remains -1`. These constrain implementation rather than specifying required behavior. — Severity: **major**

D6. spec.md:L147 — **REQ-CLITUI3-003 does not specify Ctrl-R behavior during streaming state.** The requirement guards only against modal active state ("AND no permission modal is active"). Risk R3 (spec.md:L305) mentions streaming blocks session loading; acceptance.md §2.2 (L362-363) says overlay opens but Enter is blocked during streaming. No formal requirement captures this interaction. — Severity: **minor**

D7. spec.md:L8 — **Priority field value "P1" does not match the expected enum (critical, high, medium, low).** The field is present as a string (MP-3 type check passes), but the value "P1" is non-standard. Expected values are: critical, high, medium, low. — Severity: **minor**

---

## Chain-of-Verification Pass

Second-look findings: The following was confirmed on re-read:

1. **REQ number sequencing re-verified end-to-end**: Walked every REQ in document order — confirmed 009 at L173 precedes 008 at L180. First pass was accurate.

2. **Traceability verified for all 9 REQs (not sampled)**: Every REQ's "Acceptance" field was checked against the §10 table. All 9 REQs map to existing ACs. All 10 ACs reference existing REQs. No orphan found on complete scan.

3. **Exclusions re-read for specificity**: 7 entries, all specific (named feature or technology excluded with rationale). No vague entries.

4. **Contradiction check re-run**: The Ctrl-R + streaming ambiguity (D6) was found during second pass. Initial pass noted it but did not escalate. Confirmed as minor defect — it's an incompleteness, not a direct contradiction between two stated requirements.

5. **EARS compliance for ACs re-confirmed**: Re-read acceptance.md §1 (all 10 ACs). Every AC section header uses "Given / When / Then" labels explicitly. Zero ACs use EARS structure. MP-2 FAIL is confirmed.

6. **REQ-CLITUI3-002 accepts paths as scope definition**: `~/.goose/sessions/*.jsonl` could be interpreted as scope definition (WHAT), not HOW. However, `sessionmenu.Loader` (Go struct name) and the sort algorithm specification are clear HOW details. D5 stands.

No new defects beyond those found in first pass. Seven defects total: 2 critical (D1, D2), 3 major (D3, D4, D5), 2 minor (D6, D7).

---

## Recommendation

The SPEC has two must-pass failures (MP-1 and MP-2) requiring mandatory resolution before this SPEC can be approved.

**Fix 1 — MP-1 (D1): Renumber or reorder REQ-CLITUI3-008 and REQ-CLITUI3-009**

Option A (preferred): Move REQ-CLITUI3-009 from Section 4.3 to Section 4.4, and renumber it REQ-CLITUI3-008. Renumber the current REQ-CLITUI3-008 to REQ-CLITUI3-009. Update all Acceptance cross-references (spec.md:L149, L176, L349, L351) accordingly.

Option B: Swap Sections 4.3 and 4.4 in the document so State-Driven (REQ-CLITUI3-009) comes after Unwanted (REQ-CLITUI3-008). This restores sequential ordering without renumbering.

**Fix 2 — MP-2 (D2): Rewrite all 10 ACs in EARS format**

All 10 ACs in spec.md §10 must be rewritten to match one of the five EARS patterns. Examples:

- AC-CLITUI3-001 (Ctrl-R open): "When the user presses Ctrl-R and no permission modal is active, the TUI shall display a session overlay populated with the three most recent sessions sorted by modification time descending, with cursor positioned at the first entry."
- AC-CLITUI3-003 (empty state): "When the sessionmenu opens with zero session entries, the TUI shall render the locale-specific empty-state message and close the overlay within the same update cycle without requiring user input."
- AC-CLITUI3-008 (streaming no-op): "If the user presses Ctrl-Up while streaming is in progress, then the TUI shall ignore the key press and continue streaming without activating edit mode."

Each AC must be a single binary-testable statement in one EARS pattern. The Given/When/Then scenarios in acceptance.md may remain as supplementary test documentation but must not substitute for EARS-format ACs in spec.md.

**Fix 3 — Major (D3): Split REQ-CLITUI3-003 into atomic requirements**

Decompose the six behaviors currently packed into REQ-CLITUI3-003 into separate REQs:
- REQ-CLITUI3-003: Opening behavior (Ctrl-R triggers overlay with entries)
- Move cursor behavior to REQ-CLITUI3-009 (currently State-Driven — already partially captures this)
- Add new REQ for Enter-to-load and Esc-to-dismiss behaviors, or incorporate them into existing REQs with single-behavior scope

**Fix 4 — Major (D4): Rewrite REQ-CLITUI3-008 in proper EARS Unwanted pattern**

Replace spec.md:L181-183:
- Current: "The TUI **shall not** activate edit mode when `streaming == true`"
- Required: "If the user presses Ctrl-Up while streaming is in progress, then the TUI shall ignore the key press, leaving editingMessageIndex unchanged and the editor buffer unmodified."

**Fix 5 — Major (D5): Remove implementation details from REQs**

Rewrite REQs to express behavioral outcomes, not implementation structure:
- REQ-CLITUI3-001: Remove `internal/cli/tui/i18n/` and `Catalog struct`. Express as: "The TUI shall load locale-specific display strings at initialization based on the configured conversation language; if the language configuration is absent or the language is unrecognized, the TUI shall use English strings."
- REQ-CLITUI3-002: Remove `sessionmenu.Loader`, `~/.goose/sessions/*.jsonl`. Express as: "The session list loader shall return at most 10 sessions from the user's session storage directory sorted by recency; if no sessions exist, it shall return an empty list without error."
- REQ-CLITUI3-005, -006, -008: Remove `editingMessageIndex` variable references; describe state transitions in behavioral terms.

**Fix 6 — Minor (D6): Add formal requirement for Ctrl-R + streaming interaction**

Add a new requirement or extend REQ-CLITUI3-003 with an explicit streaming guard: "When the user presses Ctrl-R while streaming is in progress, the TUI shall display the session overlay but shall not permit session loading until streaming completes."

**Fix 7 — Minor (D7): Correct priority field value**

spec.md:L8 — Change `priority: P1` to one of the standard values: `priority: high` (or `critical` if P1 maps to critical priority).

---

Verdict: FAIL
