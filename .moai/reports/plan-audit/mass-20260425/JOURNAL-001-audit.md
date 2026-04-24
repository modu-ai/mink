# SPEC Review Report: SPEC-GOOSE-JOURNAL-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.55

M1 notice: Reasoning context ignored per M1 Context Isolation. Only spec.md and research.md were read.

## Must-Pass Results

- [FAIL] MP-1 REQ number consistency: REQ-JOURNAL-001 through REQ-JOURNAL-019 appear sequential with no gaps or duplicates (spec.md:L167–L211). Zero-padding (3 digits) consistent. PASS in sequencing. However, MP-1 itself is PASS on pure sequencing; recorded here as PASS for completeness.
- Correction: MP-1 is PASS.

- [FAIL] MP-2 EARS format compliance: Multiple Unwanted-category requirements deviate from the canonical EARS Unwanted pattern "If [undesired condition], then the [system] shall [response]":
  - REQ-JOURNAL-013 (spec.md:L197) — pure prohibition ("shall not use language...") with no "If" clause.
  - REQ-JOURNAL-014 (spec.md:L199) — pure prohibition ("shall not transmit...under any circumstances") with no "If" clause.
  - REQ-JOURNAL-016 (spec.md:L203) — prohibition ("shall not include other users' data") with no "If" clause.
  - REQ-JOURNAL-015 (spec.md:L201) — compound requirement mixing prohibition ("shall not diagnose") and positive response ("entries containing ... shall trigger a canned response"). Combines two distinct requirements in one entry.
  Ubiquitous negations would be more accurate, but the author marked these [Unwanted]. Pattern-label mismatch = MP-2 FAIL.

- [FAIL] MP-3 YAML frontmatter validity: Required `labels` field is absent (spec.md:L1–L13). The `created_at` field is named `created` instead of `created_at`, deviating from required schema. Additional fields (`author`, `updated`, `issue_number`, `phase`, `size`, `lifecycle`) are present but do not compensate for missing required ones. One missing required field + one renamed required field = FAIL.

- [N/A] MP-4 Section 22 language neutrality: SPEC is scoped to a single Go package (`internal/ritual/journal/`, spec.md:L284–L303). No multi-language LSP/tooling claims. Auto-pass.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.70 | 0.75 band (minor ambiguity, mostly unambiguous) | EARS and CJK mixed phrasing clear; minor ambiguity in REQ-JOURNAL-012 queue semantics ("discarded on process exit") and REQ-JOURNAL-009 trigger window ("recent 3 entries" — last-3-saved vs last-3-days?) spec.md:L185, L193. |
| Completeness | 0.65 | 0.50–0.75 mid band | All main sections (HISTORY, Overview, Background, Scope, EARS, AC, Technical, Dependencies, Risks, References, Exclusions) present. However YAML frontmatter is incomplete (labels missing, L1–L13) and several features in scope lack REQ entries (see Defects D3, D4). |
| Testability | 0.65 | 0.50–0.75 mid band | Most ACs are binary with concrete thresholds (AC-003 `Valence >= 0.7`, AC-012 `LLM mock 호출 0회`, spec.md:L230, L275). Some ACs reference fields not in the schema: AC-JOURNAL-006 (spec.md:L245) returns `distance_years`, but `distance_years` is not defined in StoredEntry (spec.md:L315–L327) or Anniversary type. AC-JOURNAL-007 (spec.md:L250) accepts "결혼기념일 또는 특별한 날" — OR testable but loose. |
| Traceability | 0.40 | 0.25–0.50 band | 10 of 19 REQs have NO corresponding AC: REQ-JOURNAL-002 (file perms), 005 (evening prompt dispatch), 009 (low-valence soft tone), 010 (LoRA default), 011 (retention cleanup), 012 (MEMORY unavailable buffering), 013 (neutral prompts), 017 (LLM prompt content), 018 (weekly summary), 019 (INSIGHTS integration). ACs additionally do NOT cite REQ-JOURNAL-XXX IDs explicitly; mapping is inferred. |

## Defects Found

D1. spec.md:L1–L13 — YAML frontmatter missing required `labels` field; `created` should be `created_at`. MP-3 FAIL. — Severity: critical

D2. spec.md:L197, L199, L201, L203 — REQ-JOURNAL-013/014/015/016 tagged [Unwanted] but do not match canonical Unwanted EARS pattern ("If [undesired condition], then the [system] shall [response]"). These are either ubiquitous negations or compound requirements. REQ-015 in particular mixes a prohibition ("shall not diagnose") with a positive response ("shall trigger canned response") — violates one-requirement-per-entry principle. MP-2 FAIL. — Severity: critical

D3. spec.md:L167–L211 vs L217–L276 — 10 of 19 REQs uncovered by any AC: REQ-002, 005, 009, 010, 011, 012, 013, 017, 018, 019. Over half of requirements have no binary acceptance test. — Severity: critical

D4. spec.md:L88–L91 — Section 3.1 scope advertises `JournalWriter.Search(ctx, userID, query) []StoredEntry` (FTS5 local search) and `TrendAggregator.WeeklyTrend / MonthlyTrend / RenderChart` but no REQ-JOURNAL entry nor any AC covers search or trend rendering behavior. Declared scope is under-specified. — Severity: major

D5. spec.md:L245 (AC-JOURNAL-006) vs L315–L327 (StoredEntry schema) — AC asserts `distance_years=1` in the returned entry, but `distance_years` is NOT a field of `StoredEntry` or `Anniversary` types (only `type`, `name`, `year` are defined). AC references a phantom field. — Severity: major

D6. spec.md:L132–L140 (config sample) vs L177, L209 — `config.journal.prompt_timeout_min` (REQ-005) and `config.journal.weekly_summary` (REQ-018) are referenced by requirements but omitted from the canonical config sample at spec.md:L133–L140. Config schema is incomplete. — Severity: major

D7. spec.md:L193 (REQ-JOURNAL-012) — "`Write` returns ErrPersistFailed" yet the error taxonomy is not enumerated anywhere in the SPEC (contrast with AC-001 `ErrJournalDisabled` at spec.md:L220 — also not enumerated). Error types are introduced ad-hoc without a canonical list. — Severity: minor

D8. spec.md:L185 (REQ-JOURNAL-009) — "recent 3 entries show consistent negative valence (< 0.3)" — "recent 3 entries" is ambiguous: last 3 calendar days? last 3 saved? last 3 within a time window? Implementation will diverge. — Severity: minor

D9. spec.md:L177 (REQ-JOURNAL-005) — Step (a) checks "if a journal entry for today already exists". No definition of "today" boundary (user local tz vs server tz vs SCHEDULER tz). — Severity: minor

D10. spec.md:L201 (REQ-JOURNAL-015) — Crisis keyword list is defined in code block at spec.md:L383–L386 with 8 terms, but the SPEC gives no normative reference that REQ-015 relies on that list. A tester cannot definitively enumerate what "self-harm indicators" means without treating the code example as normative. — Severity: minor

D11. spec.md:L1–L13 — Frontmatter includes several non-standard fields (`phase: 7`, `size: 중(M)`, `lifecycle: spec-anchored`, `issue_number: null`) without schema declaration. Neutral observation but contributes to schema uncertainty. — Severity: minor

## Chain-of-Verification Pass

Second pass findings: Re-read the EARS section end-to-end. Confirmed all 19 REQs are numbered 001–019 contiguously (no skip). Re-read Acceptance Criteria section and cross-referenced every AC against REQ list — confirmed 10 uncovered REQs (D3). Re-read StoredEntry schema (spec.md:L315–L327) and Anniversary-related ACs (AC-006, AC-007) — confirmed `distance_years` is absent from schema (D5). Re-read Exclusions (spec.md:L541–L556) — PASS, specific and non-vague (13 concrete exclusions). Re-read config sample vs REQ references — confirmed `prompt_timeout_min` and `weekly_summary` missing from yaml (D6). Re-read REQ-005 time-zone handling — confirmed undefined (D9 added). No internal contradictions found between REQs; exclusions align with in-scope requirements.

## Recommendation

Overall verdict: FAIL (iteration 1 of max 3). Required fixes before re-audit:

1. (D1, MP-3) Add `labels` array to frontmatter (e.g., `labels: [phase-7, journal, privacy-critical, memory, identity]`). Rename `created` → `created_at` to match required schema. Remove or schema-declare non-standard fields (`phase`, `size`, `lifecycle`).

2. (D2, MP-2) Rewrite REQ-JOURNAL-013, 014, 015, 016 to match canonical EARS. Either re-tag as [Ubiquitous] (negations are valid ubiquitous form: "The system shall not...") OR rewrite with "If" clause per Unwanted pattern. Split REQ-015 into two distinct entries: one for prohibition ("shall not diagnose"), one for the positive crisis response trigger.

3. (D3) Add ACs for the 10 uncovered REQs: REQ-002 (verify file permissions 0600), REQ-005 (verify existing-entry skip + prompt timeout), REQ-009 (verify low-valence softened prompt), REQ-010 (verify AllowLoRATraining default false), REQ-011 (verify retention cleanup), REQ-012 (verify buffering + ErrPersistFailed), REQ-013 (verify prompt neutrality — forbidden phrase assertions), REQ-017 (verify LLM prompt contents when opt-in), REQ-018 (verify weekly summary cadence), REQ-019 (verify insights.OnJournalEntry invocation).

4. (D4) Add REQs for `JournalWriter.Search`, `TrendAggregator.WeeklyTrend/MonthlyTrend`, `RenderChart`. Corresponding ACs required.

5. (D5) Either add `distance_years int` to `StoredEntry` (or define a wrapping `AnniversaryMatch` type), or update AC-JOURNAL-006 to not assert on a non-existent field.

6. (D6) Extend the config yaml sample at spec.md:L133–L140 to include `prompt_timeout_min: 60` and `weekly_summary: false` (and any other referenced keys).

7. (D7) Add an Error taxonomy subsection enumerating `ErrJournalDisabled`, `ErrPersistFailed`, and any other sentinel errors.

8. (D8) Clarify REQ-JOURNAL-009: define "recent 3 entries" as "last 3 saved entries" OR "entries in the past 3 days" with explicit threshold semantics.

9. (D9) Add a definition in §6 or §4 specifying that "today" is computed in the user's local timezone as configured in IDENTITY-001 (or equivalent canonical source).

10. (D10) Promote `crisisKeywords` from the Go code example (spec.md:L383–L386) to a normative list in an EARS requirement, or reference an explicit goldenfile path with version pin.

Defect count: 11 (3 critical, 3 major, 5 minor).

## Regression Check

Iteration 1 — not applicable.
