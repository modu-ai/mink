# SPEC Review Report: SPEC-GOOSE-LOCALE-001

Iteration: 1/3
Verdict: **FAIL**
Overall Score: 0.68

Reasoning context ignored per M1 Context Isolation. Audit conducted against `spec.md` and `research.md` only.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: REQ-LC-001 through REQ-LC-016 are sequential with no gaps or duplicates. Verified:
  - Ubiquitous: 001–004 (spec.md:L128–134)
  - Event-Driven: 005–008 (spec.md:L138–144)
  - State-Driven: 009–011 (spec.md:L148–152)
  - Unwanted: 012–015 (spec.md:L156–162)
  - Optional: 016 (spec.md:L166)

- **[FAIL] MP-3 YAML frontmatter validity**: Required field `labels` is ABSENT (spec.md:L1–13). Field `created_at` is present only as `created` (spec.md:L5) — key name mismatch with rubric requirement. Present fields: `id`, `version`, `status`, `created`, `updated`, `author`, `priority`, `issue_number`, `phase`, `size`, `lifecycle`. Missing: `labels` (required array/string). Key rename needed: `created` → `created_at`. Two missing/mismatched required fields = FAIL per MP-3 firewall.

- **[PASS with reservations] MP-2 EARS format compliance**: 13/16 REQs match strict EARS patterns. REQ-LC-012 (L156) and REQ-LC-014 (L160) labeled `[Unwanted]` but written as "shall not" Ubiquitous-negative form rather than "If X, then shall Y" Unwanted pattern. Acceptable as Ubiquitous-negative variant; classification mislabel is cosmetic. REQ-LC-013 (L158) and REQ-LC-015 (L162) use proper `If...then...shall` Unwanted structure. Overall strict-EARS substantive compliance verified.

- **[N/A] MP-4 Section 22 language neutrality**: SPEC targets locale/cultural-context detection, not multi-language tooling. No tool-name hardcoding. Auto-pass.

---

## Category Scores (rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.80 | 0.75 band (minor ambiguity) | REQ-LC-002 "95% of installations" (L130) and REQ-LC-007 "≤ 400 tokens" (L142) are measurable; AC-LC-011 "GPT-4 tokenizer 기준 추정" (L225) uses "추정" (estimate) — weasel hedge. |
| Completeness | 0.65 | 0.50 band — lower | HISTORY/WHY/WHAT/REQUIREMENTS/AC/Exclusions all present; but frontmatter missing `labels` and renamed `created_at`; LocaleContext omits number format, decimal separator, sort collation, date format templates (user-flagged concerns). |
| Testability | 0.70 | 0.75 band — lower | Most ACs are binary Given/When/Then. But REQ-LC-002 (95% detection rate, L130) is not unit-testable. AC-LC-011 "GPT-4 tokenizer 추정" (L225) not precisely binary. REQ-LC-003 "deterministic" (L132) has no AC verifying determinism via repeated calls. |
| Traceability | 0.50 | 0.50 band | 6/16 REQs lack direct AC coverage (see D4). ACs do not explicitly cite `REQ-LC-XXX` identifiers (AC-LC-001..012, L172–230) — traceability is implicit via shape/behavior only. |

---

## Defects Found

**D1 — spec.md:L1–13 — Frontmatter missing `labels` field — Severity: critical**
MP-3 requires `labels` (array or string). Frontmatter has `id`, `version`, `status`, `created`, `updated`, `author`, `priority`, `issue_number`, `phase`, `size`, `lifecycle` — no `labels`. This is a must-pass firewall failure.

**D2 — spec.md:L5 — Frontmatter uses `created` rather than `created_at` — Severity: major**
Rubric-required key name is `created_at`. Value format (ISO date `2026-04-22`) is correct; only the key spelling deviates. Either rename or document this as a project-specific convention deviation.

**D3 — spec.md:L172–230 — Acceptance Criteria do not cite REQ-LC-XXX identifiers — Severity: major**
AC-LC-001 through AC-LC-012 use Given/When/Then format but never explicitly state "covers REQ-LC-XXX". A reader must infer mapping from behavior alone. AC-4 criterion requires "each AC references a valid REQ-XXX"; this SPEC fails that form of traceability.

**D4 — spec.md:L124–166 — Six REQs lack direct AC coverage — Severity: major**
- REQ-LC-002 (95% OS detection rate, L130) — no AC tests detection rate
- REQ-LC-003 (CulturalContext determinism, L132) — no AC verifies repeated-call stability
- REQ-LC-011 (CONFIG-001 sub-struct schema, L152) — no AC verifies typed schema
- REQ-LC-012 (no third-party IP transmission, L156) — no AC asserts telemetry OFF
- REQ-LC-014 (no `os.Setenv` mutation, L160) — no AC verifies env var purity
- REQ-LC-016 (timezone_alternatives ambiguity, L166) — no AC exercises ambiguous TZ

**D5 — spec.md:L257–267 + L336–355 — LocaleContext omits number format, collation, date format templates — Severity: major**
User-requested focus area: "locale typically covers time zone, currency, number format, sorting collation, date format." LocaleContext covers country/language/timezone/currency/measurement/calendar but:
- No `number_format` field (decimal separator `.` vs `,`, thousand separator, grouping)
- No `collation` field (sort ordering — German phonebook vs dictionary, Swedish å placement, Chinese pinyin vs stroke)
- No `date_format` field (DD/MM/YYYY vs MM/DD/YYYY vs YYYY-MM-DD)
- No `time_format` field (12h vs 24h, AM/PM markers)
System prompt template (L338–355) omits these dimensions entirely, forcing LLM to infer from country alone. This is a scope gap — either include or explicitly exclude with rationale in Exclusions section.

**D6 — research.md:L288 (item #4) — Country → currency mapping source unresolved — Severity: major**
`LocaleContext.currency` (ISO 4217) is required per REQ-LC-001 (spec.md:L128), and AC-LC-001 (L175) asserts `currency:"KRW"` for Korea. But: SPEC does not specify where the country→currency mapping table lives (research.md §12 item #4 flags "x/text/currency 의존 vs 수동 매핑 240개" as OPEN). AC-LC-001/002 hinge on this unresolved mapping. Must be decided before implementation.

**D7 — spec.md:L88, L165–166 — Multi-timezone country resolution under-specified — Severity: major**
REQ-LC-016 (L166) mentions `Asia/Shanghai covers mainland China and parts of Xinjiang` for TZ ambiguity. But countries like US (6 zones), RU (11 zones), BR (4 zones), AU (5 zones), CA (6 zones) have the same ambiguity and are not addressed. REQ-LC-001 requires a single `timezone` field (IANA). How is the default timezone chosen for multi-zone countries? No AC covers this. CLDR uses "likelySubtags" — SPEC does not reference it.

**D8 — spec.md:L450 + research.md:L186–187, L295 — CLDR claim vs actual coverage mismatch — Severity: minor**
SPEC cites Unicode CLDR as reference (L450), but cultural mapping covers only ~20 countries (L313–333) with manual mapping. CLDR provides 200+ locale definitions. Calendar enum omits CLDR-defined systems (islamic-umalqura, islamic-civil, islamic-tbla, persian, japanese, republic_of_china, coptic, ethiopic, indian, dangi, etc.). This is acceptable as a v0.1 scope decision but should be stated explicitly: "This SPEC uses a CLDR-inspired subset of 20 countries + 5 calendar systems; full CLDR integration is future work."

**D9 — spec.md:L130 — REQ-LC-002 "95% of installations" not unit-testable — Severity: minor**
"at least 95% of installations where OS locale API is available" requires field telemetry or installation study, not unit tests. Either reformulate as "shall succeed on all three supported OS when any of (LANG, LC_ALL, LC_MESSAGES) is set" or move to a non-normative aspirational note.

**D10 — spec.md:L225 — AC-LC-011 "GPT-4 tokenizer 기준 추정" contains weasel hedge — Severity: minor**
"추정" (estimate) softens a measurable criterion. Replace with: "measured via `cl100k_base` tokenizer (tiktoken) ≤ 400 tokens — test asserts exact count."

**D11 — spec.md:L334 — Silent fallback for un-mapped countries — Severity: minor**
"누락 국가는 en-US + `none` honorific + `gdpr` 플래그 없음으로 폴백." Fallback to `en-US` for a user from a country not in the 20-country table is a strong assumption (an Ethiopian user gets en-US Cultural Context). Add a WARN log and expose `resolved_via_fallback` diagnostic flag in CulturalContext for observability.

**D12 — spec.md:L176 vs research.md:L115 — `zh-Hans-CN` / `zh-Hant-TW` handling inconsistency — Severity: minor**
Research (L115) notes `zh-Hans-CN` and `zh-Hant-TW` as subtags, but spec's AC-LC-002 uses bare `ja-JP`. Does `primary_language` always include script subtag for Chinese, or never? No AC covers `zh-Hans` vs `zh-Hant` disambiguation; cultural mapping table (L315) only lists `CN → zh-CN` (no script).

---

## Chain-of-Verification Pass

Second pass re-read sections:
- Full REQ list (L124–166): confirmed sequence 001–016, no gap, no duplicate.
- Full AC list (L172–230): re-counted 12 ACs, re-verified no explicit REQ-XXX citations — confirmed D3.
- Exclusions (L461–473): 11 specific entries, each substantive. Strong.
- Cultural mapping table (L313–333): 20 countries listed; re-verified against cultural mapping in research §6.1 (16 honorific systems) — table consistency holds.
- Traceability cross-check: walked each REQ to identify covering AC. Confirmed 6 uncovered REQs (D4). No REQ is duplicated across ACs erroneously.
- Consistency: REQ-LC-004 (no HTTP unless MaxMind fails, L134) and REQ-LC-015 (CN skip ipapi, L162) are non-contradictory — 015 is narrower specialization. PASS.
- New defect discovered in second pass: D12 (zh-Hans-CN vs zh-CN script subtag handling) — added to list.

Chain-of-Verification outcome: one new minor defect (D12) discovered; other 11 defects confirmed.

---

## Boundary Analysis: LOCALE-001 vs I18N-001 (user-requested focus)

| Concern | LOCALE-001 | I18N-001 (referenced) |
|---------|------------|----------------------|
| OS locale detection | IN (REQ-LC-005, L138) | OUT |
| Country/language/timezone/currency/measurement/calendar **data model** | IN (REQ-LC-001, L128) | OUT |
| Cultural context (honorifics, weekend days, legal flags) | IN (REQ-LC-003, L132) | OUT |
| LLM system prompt injection of locale+cultural context | IN (REQ-LC-007, L142) | OUT |
| UI text translation / string catalogues | OUT (L39, L114) | IN |
| Pluralization rules | OUT (L114) | IN |
| RTL layout | OUT (L114) | IN |
| Accept-Language HTTP parsing | OUT (L119) | IN |
| Number format (decimal, thousand separator) | **UNDEFINED** (D5) | Presumed IN but not stated |
| Date format template (DD/MM vs MM/DD) | **UNDEFINED** (D5) | Presumed IN but not stated |
| Sort collation (CLDR UCA) | **UNDEFINED** (D5) | Presumed IN but not stated |

Verdict on boundary: **mostly clear but has gaps**. Number format, date template, and collation fall in a gray zone — neither SPEC claims them. User's explicit question ("지역/통화/날짜/숫자 포맷?") exposes this gap. See D5.

CLDR compliance: **CLDR-inspired, not CLDR-compliant**. Manual 20-country table instead of full CLDR supplemental data. See D8.

---

## Recommendation (for manager-spec revision)

Required fixes to reach PASS (address critical/major items D1–D7):

1. **Frontmatter (D1, D2)**: Add `labels` field (e.g., `labels: ["phase-6", "localization", "foundation"]`). Rename `created` → `created_at` for rubric compliance (or justify deviation in a frontmatter-convention note).

2. **Traceability (D3, D4)**:
   - Append `→ REQ-LC-XXX` (or `REQ: REQ-LC-XXX` trailer) to every AC at L172–230.
   - Add ACs covering the 6 uncovered REQs:
     - AC for REQ-LC-002 (detection success rate — reformulate as deterministic per-OS test)
     - AC for REQ-LC-003 (CulturalContext determinism: 100 calls → identical output)
     - AC for REQ-LC-011 (CONFIG-001 schema round-trip)
     - AC for REQ-LC-012 (telemetry OFF by default: zero network calls when `geolocation_enabled=false`)
     - AC for REQ-LC-014 (process env unchanged after Detect)
     - AC for REQ-LC-016 (timezone_alternatives populated for ambiguous TZ)

3. **Scope decision (D5)**: Either add `number_format`, `date_format`, `time_format`, `collation` to `LocaleContext` (prefer CLDR locale-data approach) OR explicitly add to Exclusions section with rationale pointing to I18N-001 or a future SPEC. User flagged this gap — do not leave ambiguous.

4. **Currency mapping (D6)**: Decide and document: (a) `golang.org/x/text/currency` dependency, or (b) manual 240-pair mapping in `cultural.go`. Update Technical Approach §6.2 and REQ-LC-001/ACs accordingly.

5. **Multi-zone country resolution (D7)**: Add REQ-LC-017 or extend REQ-LC-001 to specify default timezone selection for multi-zone countries (likely "use OS TZ env if set; else use CLDR likelySubtags primary zone; else record as conflict"). Add AC-LC-013 covering US/RU/BR cases.

6. **Minor (D8–D12)**:
   - State CLDR scope explicitly ("subset of 20 countries + 5 calendars; full CLDR integration is future work").
   - Reformulate REQ-LC-002 to something unit-testable.
   - Replace "추정" with exact tokenizer identifier and test assertion in AC-LC-011.
   - Add `resolved_via_fallback` diagnostic field and WARN log for un-mapped countries.
   - Document script-subtag policy for Chinese (`zh-Hans-CN` vs `zh-CN`).

After these fixes, re-audit for iteration 2.

---

**End of audit — LOCALE-001 iteration 1**
