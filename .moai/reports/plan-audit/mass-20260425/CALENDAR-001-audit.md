# SPEC Review Report: SPEC-GOOSE-CALENDAR-001

Iteration: 1/3 (mass audit 2026-04-25)
Verdict: FAIL
Overall Score: 0.55

Reasoning context ignored per M1 Context Isolation. Audited from `spec.md` and `research.md` only.

---

## Must-Pass Results

- [FAIL] MP-1 REQ number consistency:
  - REQ-CAL-001 through REQ-CAL-019 present, sequential, zero-padded consistently.
  - Ubiquitous (001-004, L132-138), Event-Driven (005-009, L142-150), State-Driven (010-012, L154-158), Unwanted (013-016, L162-168), Optional (017-019, L172-176).
  - No gaps, no duplicates → PASS for MP-1.
  - (Correction: MP-1 is PASS.)

  Revised: **[PASS] MP-1** — 19 sequential REQs, zero-padded, no gaps/duplicates. Evidence: spec.md:L132–L176.

- [FAIL] MP-2 EARS format compliance:
  - Section 4 (요구사항) correctly uses EARS patterns with `[Ubiquitous]`, `[Event-Driven]`, `[State-Driven]`, `[Unwanted]`, `[Optional]` labels.
  - However, Section 5 (수용 기준, "Acceptance Criteria") uses **Given/When/Then** format exclusively for all 10 ACs (AC-CAL-001 through AC-CAL-010). Example: spec.md:L183-185 `- **Given** ... - **When** ... - **Then** ...`.
  - MP-2 explicitly requires **every acceptance criterion** to match an EARS pattern. G/W/T is not EARS.
  - Additional concern: REQ-CAL-013 through REQ-CAL-016 (Unwanted section) use prohibition form "shall not" rather than strict EARS Unwanted pattern "If [undesired condition], then [system] shall [mitigation]". Minor deviation, but propagates MP-2 weakness.

- [FAIL] MP-3 YAML frontmatter validity:
  - `id` present ✓ (spec.md:L2)
  - `version` present ✓ (spec.md:L3)
  - `status` present as "Planned" (spec.md:L4) — non-standard value; rubric requires draft/active/implemented/deprecated.
  - `created_at` **MISSING** — the field is spelled `created:` at spec.md:L5, not `created_at:`. This fails the required field name.
  - `priority` present ✓ (spec.md:L8, value "P0")
  - `labels` **MISSING ENTIRELY** — not present anywhere in frontmatter (spec.md:L1-13).
  - Frontmatter instead contains non-standard fields: `author`, `issue_number`, `phase`, `size`, `lifecycle`, `updated`.
  - Two required fields missing (`created_at`, `labels`) → FAIL.

- [N/A] MP-4 Section 22 language neutrality:
  - SPEC is single-language scoped to Go (`internal/ritual/calendar/` package, L240; external deps all Go libraries L340-345).
  - Single-language project → N/A per rubric.

---

## Category Scores

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 band | REQ language precise with concrete thresholds (spec.md:L142 "90 days", L158 "5xx for 3 consecutive calls within 60 seconds"). Minor issue: REQ-CAL-007 mixes English EARS prefix with Korean trigger clause ("When OAuth 토큰이 요청 중 만료되면", spec.md:L146) — understandable but inconsistent. No pronoun ambiguity. |
| Completeness | 0.50 | 0.50 band | All narrative sections present (HISTORY L17-21, 개요 L25, 배경 L44, 스코프 L68, 요구사항 L128, 수용 기준 L180, 기술적 접근 L234, 의존성 L373, 리스크 L392, 참고 L407, Exclusions L431). Frontmatter is materially broken: 2 required fields missing (`labels`, `created_at`). |
| Testability | 0.70 | 0.75 band (downgraded) | ACs contain concrete measurable values (151일 L189, 4주 L194, +9h 변환 L215, 5개 이벤트 L224, 2026-10-03 L229). No weasel words in ACs. Downgrade: ACs are G/W/T style, which is testable by scenario execution, but does not satisfy the MP-2 EARS requirement for "binary-testable in EARS form". |
| Traceability | 0.40 | 0.50 band (downgraded) | 10 ACs cover 10 REQs (001, 002, 003, 005, 006, 007, 009, 011, 019 + one dual mapping). **9 REQs uncovered**: REQ-CAL-004 (CREDPOOL usage), REQ-CAL-008 (attendees invitation), REQ-CAL-010 (skip missing creds), REQ-CAL-012 (circuit breaker), REQ-CAL-013 (minimum OAuth scopes), REQ-CAL-014 (normalized DTO), REQ-CAL-015 (cross-user cache isolation), REQ-CAL-016 (cross-origin redirect rejection), REQ-CAL-017 (Conferencing MeetLink), REQ-CAL-018 (NLP create). Over 50% of REQs have no explicit AC, including multiple security/isolation requirements (013-016) which are the highest-value testable claims in the SPEC. |

---

## Defects Found

D1. spec.md:L1–L13 — `labels` required field is entirely absent from YAML frontmatter — Severity: critical (MP-3)
D2. spec.md:L5 — Field is spelled `created:` instead of required `created_at:` — Severity: critical (MP-3)
D3. spec.md:L4 — `status: Planned` uses non-enum value; rubric requires draft/active/implemented/deprecated — Severity: minor
D4. spec.md:L183–L230 — All 10 acceptance criteria (AC-CAL-001..010) use Given/When/Then format instead of EARS patterns — Severity: critical (MP-2)
D5. spec.md:L128–L176 vs L180–L230 — 9 of 19 REQs have no corresponding AC: REQ-CAL-004, 008, 010, 012, 013, 014, 015, 016, 017, 018 — Severity: major (traceability; includes security-critical 013-016)
D6. spec.md:L146 — REQ-CAL-007 mixes English EARS prefix with Korean trigger ("When OAuth 토큰이 요청 중 만료되면"); other REQs use consistent English or consistent Korean — Severity: minor (consistency)
D7. spec.md:L162–L168 — REQ-CAL-013..016 use simple prohibition form "shall not" rather than strict EARS Unwanted pattern "If [undesired condition], then the [system] shall [mitigate]" — Severity: minor (EARS strictness)
D8. spec.md:L64 vs L148 — Apparent scope contradiction: Section 2.3 OUT excludes "attendee 관리 (초대장 발송)" but REQ-CAL-008 mandates native provider **shall** send the invitation when Attendees non-empty. The boundary between "initial create-with-attendees" (in scope) and "invitation lifecycle management" (out of scope) is not delineated — Severity: minor (scope ambiguity)
D9. spec.md:L96 — Google rate limit stated as "250/100s" without units clarification (requests per 100 seconds per user? per project?); also no AC verifies rate-limit behavior — Severity: minor
D10. spec.md:L119 — OUT excludes "Offline sync" but Section 2.3 IN enumerates no offline strategy while Risk R8 (L404) mentions TTL 5분 + ETag revalidation; cache policy is stated as a risk mitigation, not a requirement, and has no REQ coverage — Severity: minor (underspecified caching)
D11. research.md:L16 — Decision records "Naver: CalDAV 확인 후, 불가 시 scope 제외 (v0.2 재검토)" while spec.md:L31 asserts "Google, iCloud, Outlook, Naver 모두 CalDAV 표준 지원" as a premise. Research itself flags Naver as uncertain (L10 "△ 확인 필요"). The SPEC should not assert Naver CalDAV support as a premise when research flags it as unverified — Severity: major (unverified premise drives entire provider strategy)
D12. spec.md:L173 — REQ-CAL-018 depends on ADAPTER-001 (LLM NLP parsing) but ADAPTER-001 is not listed in Section 7 Dependencies (L373-389) — Severity: minor (missing dependency link)

---

## Chain-of-Verification Pass

Second-look findings (re-read sections methodically, not just spot-checks):

- REQ number sequencing verified end-to-end: 001→004 (L132,134,136,138), 005→009 (L142,144,146,148,150), 010→012 (L154,156,158), 013→016 (L162,164,166,168), 017→019 (L172,174,176). All 19 present, consecutive.
- AC traceability re-checked line-by-line; list of 9 uncovered REQs in D5 is exhaustive against spec.md:L180-230.
- Exclusions section re-examined (L431-443): 11 specific bullet entries. Good specificity.
- Contradiction scan across IN/OUT (L68-124) vs REQs: only the attendees contradiction (D8) found.
- Frontmatter fields: confirmed `labels` is absent (no line in L1-13 contains `labels`).
- Requested focus items verified:
  - External calendar integration: Google/iCloud/Outlook/Naver covered in REQ-CAL-001 interface + Section 6.1 layout ✓
  - Local vs remote sync: SPEC explicitly excludes Offline sync (L119, L436). No local-only store specified.
  - RFC 5545 iCalendar conformance: spec.md:L38 "Event 통일 DTO (RFC 5545 iCalendar 기반)" and reference at L419 ✓
  - RRULE: REQ-CAL-006 + Section 6.5 rrule-go + research.md §4 edge cases ✓
  - SCHEDULER-001 / RITUAL-001 / BRIEFING-001 boundary: spec.md:L376-386 lists these as dependencies; SCHEDULER-001 HolidayCalendar is consumed (REQ-CAL-019); BRIEFING-001 is consumer; no RITUAL-001 reference found — unclear whether `internal/ritual/calendar/` path (L240) implies ownership by RITUAL-001 or merely package namespace. Possible undeclared dependency (see D12 pattern).
  - Overlap with SCHEDULER-001: HolidayCalendar is shared (not duplicated). Clean boundary.
  - Overlap with BRIEFING-001: CALENDAR-001 exposes GetTodaySchedule, BRIEFING-001 consumes. Clean boundary.

No new critical defects discovered beyond D1-D12.

---

## Regression Check

N/A (iteration 1).

---

## Recommendation

**FAIL**. The SPEC is FAIL-blocked on two independent must-pass criteria (MP-2, MP-3). Required fixes for manager-spec before next iteration:

1. **Fix frontmatter (MP-3, D1-D2)**:
   - Rename `created:` → `created_at:` at spec.md:L5.
   - Add `labels:` field (array) listing domain tags such as `[calendar, caldav, oauth, integration, phase-7]`.
   - Consider changing `status: Planned` → `status: draft` to match enum (D3).

2. **Convert all ACs to EARS (MP-2, D4)**:
   - Rewrite each AC-CAL-001..010 at spec.md:L182-230 in one of the five EARS patterns. Example conversion for AC-CAL-002:
     - Current (G/W/T): "Given from=..., When GetEvents, Then ErrRangeTooWide"
     - EARS-compliant: "If GetEvents is called with a time range exceeding 90 days, then the provider shall return ErrRangeTooWide without issuing a network request."

3. **Close traceability gaps (D5)**:
   - Add ACs for REQ-CAL-004, 008, 010, 012, 013, 014, 015, 016, 017, 018. Security-critical REQs (013 minimum scopes, 014 DTO normalization, 015 cross-user cache, 016 cross-origin redirect) MUST have dedicated ACs.

4. **Fix language consistency (D6)**:
   - REQ-CAL-007 at spec.md:L146: translate Korean trigger to English or vice versa for consistency.

5. **Strengthen EARS Unwanted form (D7)**:
   - REQ-CAL-013..016: rewrite as "If [undesired condition], then the provider shall [mitigation]". Example: "If an OAuth authorization request includes scopes beyond the minimum required, then the provider shall reject the request and log an audit warning."

6. **Resolve scope boundary for attendees (D8)**:
   - Clarify at spec.md:L64 and L148 whether "attendee 관리" exclusion only excludes **post-creation** invitation lifecycle, not initial creation with attendees.

7. **Validate Naver CalDAV premise (D11)**:
   - Either verify Naver CalDAV support and document the endpoint, or revise spec.md:L31 to accurately state that Naver CalDAV is unverified and may be dropped in v0.1 scope.

8. **Add ADAPTER-001 dependency (D12)** if REQ-CAL-018 remains in scope; otherwise defer REQ-CAL-018 to v0.2 and remove it from Section 4.5.

---

**Audited paths**:
- `/Users/goos/MoAI/AgentOS/.moai/specs/SPEC-GOOSE-CALENDAR-001/spec.md`
- `/Users/goos/MoAI/AgentOS/.moai/specs/SPEC-GOOSE-CALENDAR-001/research.md`
