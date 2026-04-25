# SPEC Review Report: SPEC-GOOSE-RITUAL-001
Iteration: 2/3
Verdict: FAIL
Overall Score: 0.71

Reasoning context ignored per M1 Context Isolation. Audit performed against `spec.md` only; prior report referenced solely for D1–D13 regression check.

---

## Must-Pass Results

### [PASS] MP-1 REQ Number Consistency
Evidence: REQ-RITUAL-001 through REQ-RITUAL-019 are sequential, no gaps, no duplicates, consistent 3-digit zero-padding.
- Ubiquitous: 001–004 (L181–187)
- Event-Driven: 005–008 (L191–197)
- State-Driven: 009–011 (L201–205)
- Ubiquitous Prohibitions: 012–015 (L215–221) — relabelled in v0.2 per D2
- Optional: 016–018 (L225–229)
- State Transitions: 019 (L253)
Total 19 REQs, fully sequential.

### [FAIL] MP-2 EARS Format Compliance

**(a) REQ-level: PASS.** REQs 012–015 (L215–221) are correctly relabelled `[Ubiquitous]` per v0.2 amendment (D2 resolution at L209–213 explanatory note). Pattern "The orchestrator shall not …" is a valid Ubiquitous negative form. REQ-019 (L253) is `[Ubiquitous]` and uses "Every RitualCompletion record shall follow…" — valid pattern. All 19 REQs match one of the five EARS patterns.

**(b) AC-level: FAIL.** Despite the v0.2 format declaration block (L277–284) that openly admits "Acceptance criteria in this section use Given–When–Then (Gherkin) scenario format, **not EARS syntax**", this is precisely the failure mode the audit checklist flags as MP-2. Per audit checklist AC-1: "Each AC matches one of the five EARS patterns". Format declaration cannot waive a must-pass criterion — declaring non-compliance does not constitute compliance. All 13 ACs (AC-RITUAL-001 through AC-RITUAL-013, L286–367) use Given/When/Then scenarios, which M3 rubric explicitly identifies as the score-0.50 band failure: "Given/When/Then test scenarios mislabeled as EARS".

The author's rationale (L282) — that EARS is for requirement statement and Gherkin for test scenarios — is technically defensible engineering practice, but the audit checklist as written treats this as a must-pass FAIL. The `**Verifies:** REQ-RITUAL-NNN` annotation does improve traceability (resolves D12), but does not convert Gherkin into EARS.

**Verdict: FAIL.** REQ-side resolved; AC-side remains in Gherkin. Recommendation: either rewrite ACs in EARS form, or escalate to user for explicit waiver of MP-2 (b) acknowledging the engineering tradeoff.

### [PASS] MP-3 YAML Frontmatter Validity
Frontmatter at L1–14:
- [PASS] `id: SPEC-GOOSE-RITUAL-001` (L2) — string, correct pattern
- [PASS] `version: 0.2.0` (L3) — string
- [PASS] `status: planned` (L4) — string
- [PASS] `created_at: 2026-04-22` (L5) — ISO date string, correct field name (D1 resolved — was `created` in iter 1)
- [PASS] `priority: P0` (L8) — string
- [PASS] `labels: [ritual, orchestration, bond-score, streak, mood-adaptive, state-machine]` (L13) — array of strings (D1 resolved — was missing in iter 1)

Bonus: `updated_at: 2026-04-25` (L6) refreshed per D13.

All six required fields present with correct types.

### [N/A] MP-4 Section 22 Language Neutrality
SPEC scoped to single-language Go implementation (`internal/ritual/orchestrator/`, L54, L378–392). No multi-language tooling references. Auto-passes.

---

## Category Scores

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.80 | 0.75–1.0 band | State machine semantics now explicit (§4.6 L231–271, REQ-019 L253–261). Status enum transitions, timeout window (60 min default L251), skip-vs-failed distinction (L257–258) all defined. AC-013 L358–367 exercises three users (u1 responded, u2 timeout-skipped, u3 failed) with exact timestamps. Birthday multiplier ambiguity resolved (L225 "multiply by 2" + AC-012 L356 "5.5 × 2 = 11.0"). Minor: REQ-005 (L191) still chains 6 steps a–f without per-step failure handling. |
| Completeness | 0.80 | 0.75–1.0 band | All required sections present (HISTORY L24, Overview L33, Background L65, Scope L89, Requirements L177, Acceptance L275, Exclusions L632). Frontmatter complete. State transition table (L263–271) addresses prior gap. AC-013 added. Minor: weekly report schema still lacks dedicated AC (REQ-017 L227 uncovered). REQ-RITUAL-014 (quiet hours L219), REQ-RITUAL-015 (API surface L221), REQ-RITUAL-018 (A2A opt-in path L229) still lack covering ACs. |
| Testability | 0.75 | 0.75 band | AC-013 L358–367 is exemplary: clockwork-frozen time, three users, exact timestamps (07:30/07:45/08:30/09:00), exact assertions. Most ACs binary-testable. Weasel words absent. AC-009 (L334) gives concrete keyword whitelist/blacklist. Minor: AC-005 (L310) "5일째는 5, 6일째 0" assumes Evening skip semantics defined elsewhere; cross-section dependency increases test interpretation cost. Several REQs still uncovered (see D4 carryover). |
| Traceability | 0.85 | 0.75–1.0 band | Major improvement: every AC now has explicit `**Verifies:** REQ-RITUAL-NNN` line (D12 resolved). E.g. AC-001 L287, AC-002 L293, AC-013 L359. Some ACs verify multiple REQs/sections (AC-005 L311 "§6.4 streak contract", AC-010 L341 "REQ-012 + REQ-018"). Remaining gaps: REQ-003 (logs), REQ-007 (style injection), REQ-014 (quiet hours), REQ-015 (API surface), REQ-017 (weekly report), REQ-018 (opt-in path) still uncovered by any AC. |

---

## Defects Found

**D14 — [critical, carryover from D3]** `spec.md:L286-L367` — All 13 Acceptance Criteria use Given/When/Then (Gherkin) format, not EARS. The format declaration at L277–284 acknowledges this explicitly. Per M3 rubric, this triggers score-0.50 band ("Given/When/Then test scenarios mislabeled as EARS"). The `**Verifies:** REQ-RITUAL-NNN` annotation improves traceability but does not convert Gherkin into EARS. Author's engineering rationale is sound but conflicts with audit checklist AC-1. Resolution path: either rewrite ACs in EARS, or obtain explicit user waiver of MP-2 (b).

**D15 — [major, carryover from D4]** `spec.md:L275-L367` — 6 REQs still lack any covering AC even after AC-013 was added:
- REQ-RITUAL-003 (L185) zap log emission — no AC verifies field set
- REQ-RITUAL-007 (L195) RitualStyleOverride context injection — no AC
- REQ-RITUAL-014 (L219) quiet-hours milestone deferral — no AC
- REQ-RITUAL-015 (L221) Bond API surface restriction — no AC
- REQ-RITUAL-017 (L227) weekly report generation — template at §6.7 (L522–536) but no AC fixes contract
- REQ-RITUAL-018 (L229) A2A opt-in aggregated sharing — AC-010 L341 only tests opt-out (default-deny), not opt-in path

**D16 — [major, carryover from D6]** `spec.md:L181-L221` — Implementation leakage in REQs persists (RQ-4 violation):
- REQ-001 L181: cites `goosed` startup
- REQ-002 L183: cites `ErrPersistFailed`
- REQ-003 L185: enumerates exact zap field names
- REQ-005 L191: enumerates 6 implementation steps a–f
- REQ-009 L201: cites `ErrRitualsDisabled`
- REQ-010 L203: numeric threshold `< 0.3` without justification

These are HOW, not WHAT/WHY. Not addressed in v0.2 amendment.

**D17 — [major, carryover from D7]** `spec.md:L304-L314` — AC-RITUAL-004 (Full day bonus) and AC-RITUAL-005 (Streak) test behaviors that are not stated as normative REQs in §4. Full-day bonus (+1.0) appears only in §3.1 scope (L129) and §6.3 bond formula (L462). Streak break semantics defined only in §6.4 (L470–484). AC-004 explicitly cites "§6.3 base-score contract" and AC-005 cites "§6.4 streak contract" (L312)— acknowledging the gap, but the underlying requirement was not promoted to a REQ. Acceptance criteria should test REQs, not implementation sections.

**D18 — [minor, carryover from D10]** `spec.md:L219` (REQ-RITUAL-014) — quiet hours "23:00-06:00 local" still hard-coded in normative REQ with loose reference to "SCHEDULER-001's policy" (no specific REQ-ID cited). Drift risk persists.

**D19 — [minor, carryover from D11]** `spec.md:L229` (REQ-RITUAL-018) — single REQ still mixes "may" (Optional permission for aggregated sharing) and "shall never be shared" (absolute prohibition for raw payloads). Recommend split into two REQs.

---

## Chain-of-Verification Pass

Second-pass re-read focused on:
1. **Every REQ end-to-end** — rechecked REQs 001-019 individually. Confirmed REQ-019 (L253–261) is a complete 8-rule state machine specification. Confirmed REQs 012–015 are now correctly Ubiquitous.
2. **REQ number sequencing** — verified 001 through 019 with no gaps, no duplicates. Format consistent.
3. **Traceability for EVERY REQ** — rebuilt REQ→AC map. AC-013 covers REQ-019 explicitly. Five ACs now cite multiple REQs/sections via `**Verifies:**` lines. Six REQs remain uncovered (D15).
4. **Exclusions specificity** — Section L632–645 has 12 specific entries with clear scope boundaries. PASS unchanged.
5. **Contradictions** — REQ-012 (L215) now explicitly references REQ-018 opt-in exception ("the opt-in aggregated-metrics path governed by REQ-RITUAL-018 … is explicitly permitted and does not violate this prohibition"). D8 fully resolved. AC-010 L341 cross-references both REQs.
6. **Birthday wording** — REQ-016 (L225) explicitly uses "multiply" + "multiplicative modifier, not additive". AC-012 (L356) confirms 5.5 × 2 = 11.0. D9 fully resolved.
7. **Format declaration audit** — re-read L277–284. Author's rationale is intellectually honest but the audit rubric is binary; declaring "we do not use EARS" does not satisfy "ACs match EARS pattern".

Second-pass yielded no new defects beyond carryovers. First-pass thorough.

---

## Regression Check (D1–D13 from iter 1)

| ID | Description | Status | Evidence |
|----|------------|--------|----------|
| D1 | Frontmatter `created`/`labels` MP-3 | **RESOLVED** | L5 `created_at: 2026-04-22`, L13 `labels: [...]` array of 6 strings |
| D2 | REQ-012/013/014/015 mislabelled `[Unwanted]` | **RESOLVED** | L207 §4.4 retitled "Ubiquitous Prohibitions"; explanatory note L209–213; all four now `[Ubiquitous]` (L215, L217, L219, L221) |
| D3 | All 12 ACs in Gherkin not EARS | **UNRESOLVED** | L277–284 declares Gherkin format intentionally; 13 ACs all Gherkin. See D14 / MP-2 (b). |
| D4 | 7 REQs lack covering AC | **PARTIALLY RESOLVED** | AC-013 covers REQ-019 (new). 6 REQs still uncovered (REQ-003, 007, 014, 015, 017, 018). REQ-005 partially via AC-002. See D15. |
| D5 | RitualCompletion state transitions undefined | **RESOLVED** | §4.6 L231–271 adds complete graph + REQ-019 (8 rules) + status semantics table. AC-013 L358–367 exercises three transition paths. |
| D6 | Implementation leakage in REQs | **UNRESOLVED** | All six leakage points (ErrPersistFailed, ErrRitualsDisabled, zap field names, 6-step chain, goosed startup, 0.3 threshold) unchanged in v0.2. See D16. |
| D7 | AC-004/AC-005 test behavior with no REQ | **UNRESOLVED** | AC-004 L304 still cites "§6.3 base-score contract", AC-005 L311 cites "§6.4 streak contract". No new REQ promotes full-day bonus or streak rules to normative status. See D17. |
| D8 | REQ-012 vs REQ-018 A2A contradiction | **RESOLVED** | REQ-012 L215 adds explicit "Exception: the opt-in aggregated-metrics path governed by REQ-RITUAL-018 … is explicitly permitted and does not violate this prohibition." |
| D9 | Birthday "add" vs "multiply" ambiguity | **RESOLVED** | REQ-016 L225 explicitly uses "multiply by 2 … multiplicative modifier, not an additive one". AC-012 L356 confirms arithmetic. |
| D10 | Quiet hours hard-coded constant | **UNRESOLVED** | REQ-014 L219 unchanged ("23:00-06:00 local … per SCHEDULER-001's policy"). No specific REQ-ID cited. See D18. |
| D11 | REQ-018 mixes "may" + "shall never" | **UNRESOLVED** | REQ-018 L229 unchanged ("only aggregated anonymized metrics … may be shared … specific ritual contents shall never be shared"). See D19. |
| D12 | ACs lack explicit REQ citations | **RESOLVED** | Every AC now has `**Verifies:** REQ-RITUAL-NNN` line (e.g. L287, L293, L299, L305, L311, L317, L323, L329, L335, L341, L347, L353, L359). |
| D13 | `updated` field stale | **RESOLVED** | L6 `updated_at: 2026-04-25`. HISTORY entry at L29 documents v0.2 amendments. |

**Resolution summary:** 8/13 RESOLVED, 1/13 PARTIALLY RESOLVED (D4), 4/13 UNRESOLVED (D3, D6, D7, D10, D11 — note D11 is in unresolved list, total 5). Recount: D3, D6, D7, D10, D11 = 5 unresolved. Resolution rate: 8 fully + 1 partial / 13 = **62% full + 8% partial = 70% effective**.

**Stagnation flag:** None of D6, D7, D10, D11 changed at all between iter 1 and iter 2. If iter 3 repeats without progress on these four, they become "blocking defects — manager-spec made no progress" per harness contract.

---

## Recommendation (for manager-spec)

**Critical (must-fix for iter 3):**

1. **D14/D3 — MP-2 (b) AC EARS format** — Two paths:
   - Path A: Rewrite all 13 ACs in EARS form. Each AC becomes a single When/While/Where/If… shall sentence; move Gherkin scenarios into a non-normative §5.x "test scenarios" subsection.
   - Path B: Escalate to user via plan-auditor → MoAI orchestrator with explicit waiver request for MP-2 (b), acknowledging Gherkin is engineering best practice for executable specs and EARS is best for normative requirements. If approved, document the waiver in spec.md HISTORY.

**Major (should fix for iter 3):**

2. **D15/D4 — uncovered REQs** — Add at minimum:
   - AC-014 Verifies REQ-RITUAL-003 (zap log fields verified via log capture)
   - AC-015 Verifies REQ-RITUAL-007 (style override context injection)
   - AC-016 Verifies REQ-RITUAL-014 (milestone defer during quiet hours)
   - AC-017 Verifies REQ-RITUAL-015 (Bond API surface restriction)
   - AC-018 Verifies REQ-RITUAL-017 (weekly report schema)
   - AC-019 Verifies REQ-RITUAL-018 (A2A opt-in aggregated path)

3. **D16/D6 — implementation leakage** — Move concrete identifiers (`ErrPersistFailed`, `ErrRitualsDisabled`, exact zap field set, 6-step chain a–f, `goosed` startup, `< 0.3` threshold) from §4 REQs to §6 Technical Approach. Keep REQs at WHAT/WHY level.

4. **D17/D7 — promote full-day bonus + streak to REQs** — Add new REQ (or extend REQ-004) for "full-day bonus = +1.0 when FullCompleteCount >= 5" and new REQ for "streak day requires Morning OR Evening responded". AC-004 and AC-005 then verify normative REQs, not implementation sections.

**Minor (nice to have):**

5. **D18/D10 — quiet hours ownership** — either move 23:00-06:00 to a config key (preferred: `config.rituals.quiet_hours: {start: "23:00", end: "06:00"}`) or cite a specific SCHEDULER-001 REQ-ID.

6. **D19/D11 — split REQ-018** — REQ-018a `[Optional]` for "may share aggregated", REQ-018b `[Ubiquitous]` for "shall never share raw payloads".

---

**Must-pass firewall status:** MP-1 PASS, MP-2 FAIL (AC-side), MP-3 PASS, MP-4 N/A. Single must-pass FAIL forces overall **FAIL** regardless of category scores (which are now 0.80/0.80/0.75/0.85 = 0.80 average, materially improved from iter 1's 0.48).

**Trajectory:** Strong progress (8 of 13 defects resolved, score +0.32). Iteration 3 should focus on D14 (MP-2 b waiver or rewrite) as the sole remaining must-pass blocker. D15–D19 are below-firewall improvements that can land in iter 3 alongside the must-pass fix.

---

**Audit file**: `/Users/goos/MoAI/AgentOS/.moai/reports/plan-audit/mass-20260425/RITUAL-001-review-2.md`
**Next action**: Return to manager-spec for iter 3. Single must-pass blocker (D14/MP-2 b). Recommend either AC EARS rewrite or user waiver before iter 3 audit.
