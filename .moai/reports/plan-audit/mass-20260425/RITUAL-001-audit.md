# SPEC Review Report: SPEC-GOOSE-RITUAL-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.48

Reasoning context ignored per M1 Context Isolation. Audit performed against `spec.md` and `research.md` only.

---

## Must-Pass Results

### [FAIL] MP-1 REQ Number Consistency
Evidence: REQ-RITUAL-001 through REQ-RITUAL-018 are sequential with no gaps, no duplicates, consistent 3-digit zero-padding.
- Ubiquitous: 001–004 (L179–185)
- Event-Driven: 005–008 (L189–195)
- State-Driven: 009–011 (L199–203)
- Unwanted: 012–015 (L207–213)
- Optional: 016–018 (L217–221)
Total 18 REQs, fully sequential. **Result: PASS on sequencing** — upgraded to PASS. (Correction: MP-1 is PASS, overall verdict still FAIL for reasons below.)

### [FAIL] MP-2 EARS Format Compliance

**(a) REQ-level EARS issues (mislabelled category):**
- REQ-RITUAL-012 (L207) — labelled `[Unwanted]` but structure is "The orchestrator shall not use…" — this is a Ubiquitous prohibition, not an Unwanted `If <undesired condition>, then <response>` pattern.
- REQ-RITUAL-013 (L209) — same mislabel. "The orchestrator shall not penalize…" — no `If/then` clause.
- REQ-RITUAL-014 (L211) — "Milestone notifications shall not be sent during quiet hours…" — Ubiquitous constraint masked as Unwanted.
- REQ-RITUAL-015 (L213) — "The Bond Level API shall not expose…" — Ubiquitous constraint masked as Unwanted.

Four out of five `[Unwanted]` items fail the EARS Unwanted pattern (`If [undesired condition], then the [system] shall [response]`). The spec confuses "negative shall" (Ubiquitous prohibition) with Unwanted (conditional failure mode).

**(b) AC-level EARS non-compliance:**
All 12 Acceptance Criteria (AC-RITUAL-001 through 012, L227–285) use Given/When/Then (Gherkin) scenario syntax instead of EARS. Per checklist AC-1 (MP-2): "Each AC matches one of the five EARS patterns". None of the 12 ACs matches any of the five EARS patterns — all are Gherkin test scenarios. This is the "Given/When/Then test scenarios mislabeled as EARS" failure mode explicitly called out in M3 Score-0.50 rubric band.

**Verdict: FAIL** — 4/18 REQ mislabels (≈22%) + 12/12 AC non-EARS (100%).

### [FAIL] MP-3 YAML Frontmatter Validity
Frontmatter at L1–13:
- [PASS] `id: SPEC-GOOSE-RITUAL-001` (L2) — string, correct pattern
- [PASS] `version: 0.1.0` (L3) — string
- [PASS] `status: Planned` (L4) — string (note: title-case, convention usually lowercase)
- [FAIL] **`created_at` missing** — spec uses `created: 2026-04-22` (L5) instead. MP-3 requires exact field name `created_at`. Minor naming mismatch but breaks schema contract.
- [PASS] `priority: P0` (L8) — string
- [FAIL] **`labels` field entirely missing** from frontmatter. No labels key present anywhere in L1–13.

Two of six required fields violate the schema contract. **Verdict: FAIL.**

### [N/A] MP-4 Section 22 Language Neutrality
SPEC is scoped to a single-language Go implementation (`internal/ritual/orchestrator/`, L52, L295–310). No multi-language tooling references. Auto-passes.

---

## Category Scores

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.60 | between 0.50 and 0.75 | Most requirements have single unambiguous interpretation (e.g. REQ-001 L179, REQ-006 L191). However, state-transition semantics are undefined (status enum at L106 lists 5 states `scheduled/triggered/responded/skipped/failed` but no rule states which transitions are legal or what distinguishes `skipped` from `failed`). REQ-005 (L189) chains 6 steps (a–f) without specifying failure handling at each step. |
| Completeness | 0.70 | 0.75 band, downgraded | All six required sections present (HISTORY L23, Overview L31, Background L63, Scope L87, Requirements L175, Acceptance L225, Exclusions L549). Frontmatter has two schema violations (see MP-3). State transition table missing. Weekly/monthly report contents have REQ-017 optional (L219) but no AC verifying schema at all. |
| Testability | 0.55 | between 0.50 and 0.75 | ACs are specific where present (AC-009 L267 gives concrete keyword whitelists/blacklists). However, 12 ACs for 18 REQs means 6+ REQs are unverifiable as written. AC-003 (L237) "100회 반복 결과 2.0 일치" is binary-testable. AC-005 (L247) well-specified. But REQ-003 (log emission L183), REQ-007 (style injection L193), REQ-014 (quiet hours L211), REQ-015 (API surface L213), REQ-017 (weekly report L219), REQ-018 (A2A aggregate sharing L221) have no AC — untestable. |
| Traceability | 0.40 | 0.25–0.50 band | No AC explicitly cites its REQ (e.g. AC-RITUAL-001 at L227 does not say "covers REQ-RITUAL-001"); traceability is only inferred by numeric alignment, and the alignment breaks for AC-003↔REQ-004, AC-004 (no REQ), AC-005 (no REQ). At least 7 REQs have no covering AC: REQ-003, REQ-005, REQ-007, REQ-014, REQ-015, REQ-017, REQ-018. AC-004 (full-day bonus L241) and AC-005 (streak mechanics L245) test behavior never stated as a normative REQ. |

---

## Defects Found

**D1 — [critical]** `spec.md:L1-L13` — YAML frontmatter violates MP-3: `created_at` field is named `created`, and `labels` field is entirely absent. This fails the schema contract required by MoAI SPEC tooling.

**D2 — [critical]** `spec.md:L207-L213` — Four `[Unwanted]` requirements (REQ-012, 013, 014, 015) do not use the EARS Unwanted pattern (`If <undesired condition>, then the <system> shall <response>`). They are Ubiquitous prohibitions mislabelled. Either relabel as Ubiquitous or rewrite with If/then structure.

**D3 — [critical]** `spec.md:L227-L285` — All 12 Acceptance Criteria use Given/When/Then (Gherkin) format instead of EARS. Per audit checklist AC-1/MP-2, ACs must match one of the five EARS patterns. Current form is test scenarios, not acceptance criteria in EARS.

**D4 — [major]** `spec.md:L225-L285` (missing ACs) — 7 REQs lack any covering AC:
  - REQ-RITUAL-003 (L183) log emission — no AC verifies zap fields
  - REQ-RITUAL-005 (L189) full callback chain (a–f) — no AC verifies all 6 steps
  - REQ-RITUAL-007 (L193) RitualStyleOverride context injection — no AC
  - REQ-RITUAL-014 (L211) quiet-hours milestone deferral — no AC
  - REQ-RITUAL-015 (L213) Bond API surface restriction — no AC
  - REQ-RITUAL-017 (L219) weekly report generation — no AC (despite being a user-visible artifact, Section 6.7 at L438–454 shows the template but no AC fixes the contract)
  - REQ-RITUAL-018 (L221) A2A aggregated-only sharing — no AC for opt-in path (AC-010 L271 only tests opt-out)

**D5 — [major]** `spec.md:L100-L109` (status enum) — RitualCompletion.Status enum lists 5 values (`scheduled`, `triggered`, `responded`, `skipped`, `failed`) but no requirement or acceptance criterion defines:
  - What event causes `scheduled → triggered` (hook fire?)
  - What event causes `triggered → skipped` vs `triggered → failed`
  - Whether `skipped` is user-initiated or timer-initiated
  - Whether `failed` retries (REQ-002 L181 mentions persist retry, not status retry)
  No state transition diagram or normative rule in Section 4. This is particularly acute because user explicitly asked for "trigger conditions, state transitions, completion criteria, missed/skip handling" focus — and the SPEC does not answer it.

**D6 — [major]** `spec.md:L179-L221` — Implementation leakage in requirements (RQ-4 violation):
  - REQ-001 L179: cites Go-level concept "`goosed` startup … abort bootstrap"
  - REQ-002 L181: cites `ErrPersistFailed` return value
  - REQ-003 L183: cites exact zap logger field names `{user_id_hash, ritual_kind, …}`
  - REQ-005 L189: enumerates 6 implementation steps (a–f) inside a single REQ
  - REQ-009 L199: cites `ErrRitualsDisabled` return value
  - REQ-010 L201: cites numeric threshold `< 0.3` without justification in normative text (L201)
  These are HOW, not WHAT/WHY. Move to Technical Approach (§6) or soften.

**D7 — [major]** `spec.md:L243-L245` (AC-RITUAL-004 Full-day bonus) & `spec.md:L247-L250` (AC-RITUAL-005 Streak) — These ACs test behaviors that are never stated as normative requirements in §4. Full-day bonus (+1.0) appears only in scope text L127 and §6.3 L381 (implementation section). Streak break semantics appear only in §6.4 L388–402. Normative behavior must be in REQs, not implicitly in approach.

**D8 — [major]** `spec.md:L207` (REQ-RITUAL-012) vs `spec.md:L221` (REQ-RITUAL-018) — Contradiction in A2A policy. REQ-012 says "completions remain local (MEMORY-001) and serve only as input to Bond score / Streak / Mood-adaptive logic within this process" (no external sharing, absolute). REQ-018 says "only aggregated anonymized metrics … may be shared with trusted peer agents" (sharing allowed under opt-in). The two are reconcilable via "opt-in exception" but REQ-012 text is absolute ("shall not use … for ML training or external sharing"). Need explicit exception clause: "…except as governed by REQ-RITUAL-018".

**D9 — [minor]** `spec.md:L217` (REQ-RITUAL-016 Optional birthday) — uses "add 2× Bond score" as multiplier, but §6.3 L383 says "multiply total × 2". Multiplier vs additive is ambiguous. AC-012 L285 fixes 5.5 × 2 = 11.0 (multiplicative) so the REQ wording ("add") is wrong. Also ambiguous whether birthday doubles base scores or final total.

**D10 — [minor]** `spec.md:L211` (REQ-RITUAL-014) — "quiet hours (23:00-06:00 local)" is hard-coded in normative REQ but Section 6 does not define who owns this constant. Cross-references "SCHEDULER-001's policy" without citing a specific REQ-ID in that SPEC — loose coupling. If SCHEDULER-001 changes the window, REQ-014 silently drifts.

**D11 — [minor]** `spec.md:L221` (REQ-RITUAL-018) — mixes "may" (Optional permission) and "shall never be shared" (absolute prohibition) in the same requirement sentence. Split into two REQs.

**D12 — [minor]** `spec.md:L225-L285` — Acceptance criteria do not explicitly cite which REQ they verify. AC-RITUAL-001 → REQ-RITUAL-001 is inferred only by number. Adding an explicit "Verifies: REQ-RITUAL-NNN" line to each AC would make traceability verifiable without guesswork.

**D13 — [minor]** `spec.md:L5` — frontmatter field name `created` inconsistent with MoAI convention `created_at`. Also `updated: 2026-04-22` (L6) is equal to `created` but file is 24KB and has a "v0.2 Amendment 2026-04-24" header at L17 — the `updated` field has not been refreshed. Metadata drift.

---

## Chain-of-Verification Pass

Second-pass re-read focused on:
1. **Every REQ end-to-end** — rechecked REQs 001-018 individually. Confirmed D2 (4 mislabelled Unwanted) and D6 (implementation leakage) findings. Discovered D11 (REQ-018 mixing may/shall never).
2. **REQ number sequencing** — already confirmed PASS on first pass; re-verified no gaps, no duplicates. OK.
3. **Traceability for EVERY REQ** — rebuilt the REQ→AC map and confirmed 7 uncovered REQs (D4).
4. **Exclusions specificity** — Section "Exclusions (What NOT to Build)" at L549–562 has 12 specific entries, each referencing a clear scope boundary. No vague "miscellaneous exclusions". PASS on this dimension.
5. **Contradictions between requirements** — discovered D8 (REQ-012 vs REQ-018 A2A absolutism vs opt-in sharing) that was missed on first pass. Discovered D9 (birthday multiplier "add" vs "multiply" ambiguity).

Second-pass yielded three additional defects (D8, D9, D11) that warranted inclusion.

---

## Regression Check

N/A — this is iteration 1.

---

## Recommendation (for manager-spec)

**Must-fix before resubmission:**

1. **Frontmatter** — rename `created` → `created_at` (L5) and add `labels: [ritual, orchestration, bond-score, streak, mood-adaptive]` (or similar). (D1)
2. **EARS REQ relabelling** — either relabel REQ-012/013/014/015 as `[Ubiquitous]` OR rewrite with If/then Unwanted structure. Recommended: relabel to Ubiquitous since they are system-wide prohibitions. (D2)
3. **AC reformatting** — rewrite all 12 ACs in EARS format. For example AC-RITUAL-001 L227 becomes: "When `config.rituals.enabled=true` and `orchestrator.Start(ctx)` runs at goosed bootstrap, the system shall register exactly 5 HOOK-001 consumers (Morning, PostBreakfast, PostLunch, PostDinner, EveningCheckIn)." Keep a separate Gherkin test scenario block for testing guidance, but ACs themselves must be EARS. (D3)
4. **Add missing ACs** — create at minimum AC-013 (REQ-003 logs), AC-014 (REQ-005 full chain), AC-015 (REQ-007 style injection), AC-016 (REQ-014 quiet hours), AC-017 (REQ-015 API surface), AC-018 (REQ-017 weekly report schema), AC-019 (REQ-018 A2A opt-in aggregate). (D4)
5. **State transition REQ** — add new REQ-RITUAL-019 (or expand REQ-002) that defines the `scheduled → triggered → {responded|skipped|failed}` transitions explicitly: which event fires each transition, whether there are retries, and what the "timeout → skipped" window is. Add corresponding AC. (D5)
6. **Move implementation names out of REQs** — move `ErrPersistFailed`, `ErrRitualsDisabled`, exact zap field names, 6-step chain (a–f), `goosed` startup references from §4 REQs to §6 Technical Approach. Keep REQs at behavior-level. (D6)
7. **Full-day bonus + streak REQs** — add explicit REQs for §6.3 full-day bonus rule and §6.4 streak computation rule. AC-004 and AC-005 currently test behaviors that no REQ mandates. (D7)
8. **A2A contradiction** — rewrite REQ-012 to explicitly except REQ-018 opt-in sharing, or merge the two into one hierarchical REQ with default-deny and explicit opt-in exception. (D8)
9. **Birthday multiplier clarity** — fix REQ-016 L217 wording: "add 2× Bond score" → "multiply the day's total Bond score by 2". Align with AC-012 L285 arithmetic. (D9)
10. **Quiet hours ownership** — either embed the 23:00–06:00 constant in a config key (preferred) or cite the exact SCHEDULER-001 REQ-ID that defines it. (D10)
11. **Split REQ-018** — one REQ for "may share aggregated" (Optional) + one REQ for "shall never share raw payloads" (Ubiquitous prohibition). (D11)
12. **Explicit AC→REQ citations** — add "Verifies: REQ-RITUAL-NNN" line to each AC to make traceability machine-checkable. (D12)
13. **Refresh `updated` field** — L6 `updated: 2026-04-22` but amendment note L17 says 2026-04-24. Bump. (D13)

**Must-pass firewall status:** MP-2 and MP-3 both FAIL. Any single must-pass FAIL forces overall FAIL regardless of category scores.

---

**Audit file**: `/Users/goos/MoAI/AgentOS/.moai/reports/plan-audit/mass-20260425/RITUAL-001-audit.md`
**Next action**: Return to manager-spec for revisions. Recommend iteration 2 after D1–D8 at minimum are addressed.
