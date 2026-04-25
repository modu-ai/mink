# SPEC Review Report: SPEC-GOOSE-SCHEDULER-001

Iteration: 1/3
Verdict: **FAIL**
Overall Score: 0.62

Reasoning context ignored per M1 Context Isolation. Audit based solely on `spec.md` and `research.md`.

---

## Must-Pass Results

### [PASS] MP-1 REQ number consistency

Evidence: REQ-SCHED-001 through REQ-SCHED-020 appear sequentially with consistent 3-digit zero-padding and no duplicates. Distribution across EARS categories:
- Ubiquitous: REQ-SCHED-001 to 004 (spec.md:L107–L113)
- Event-Driven: REQ-SCHED-005 to 009 (spec.md:L117–L125)
- State-Driven: REQ-SCHED-010 to 012 (spec.md:L129–L133)
- Unwanted: REQ-SCHED-013 to 016 (spec.md:L137–L143)
- Optional: REQ-SCHED-017 to 020 (spec.md:L147–L153)

Count verified: 20 unique sequential IDs, no gaps, no duplicates.

### [FAIL] MP-2 EARS format compliance

The 20 **REQ** entries correctly use EARS patterns (Ubiquitous/Event-Driven/State-Driven/Unwanted/Optional) with explicit tagging — good. **However**, the acceptance criteria at spec.md:L159–L202 are written in **Given/When/Then (Gherkin/BDD)** format, not EARS.

The SPEC template in this project requires EARS-compliant ACs. While Given/When/Then is a legitimate testing format, it is NOT one of the five EARS patterns, and this project's rubric (per plan-auditor M3) treats Given/When/Then scenarios mislabeled as "EARS-based ACs" as a MP-2 violation.

Evidence:
- spec.md:L160–L162 "**Given**... **When**... **Then**..." pattern applied to all 9 ACs
- spec.md:L103 heading "EARS 요구사항" labels only the REQ section, so ACs may be intentionally BDD-style. But the SPEC lacks a parallel "EARS AC restatement" for testability.

Severity: the REQs themselves are EARS-compliant, so this FAIL hinges on rubric interpretation. Under strict MP-2 (all ACs must match one of 5 EARS patterns), this is a FAIL. Recommend downgrading to "rubric ambiguity — flag for human reviewer" if the project convention permits Gherkin ACs.

### [FAIL] MP-3 YAML frontmatter validity

Frontmatter at spec.md:L1–L13:

```yaml
id: SPEC-GOOSE-SCHEDULER-001
version: 0.1.0
status: Planned
created: 2026-04-22          ← WRONG KEY (should be created_at)
updated: 2026-04-22
author: manager-spec
priority: P0                  ← NON-STANDARD VALUE
issue_number: null
phase: 7
size: 중(M)
lifecycle: spec-anchored
```

Violations:
1. **Missing required `created_at`** — the key is `created`, not `created_at`. MP-3 requires `created_at` as ISO date string.
2. **Missing required `labels`** field entirely. MP-3 requires `labels` (array or string).
3. **`priority: P0`** — MP-3 requires one of {critical, high, medium, low}. "P0" is non-standard. If P0 maps to "critical", the value still violates the string vocabulary.
4. **`status: Planned`** — MP-3 permits {draft, active, implemented, deprecated}. "Planned" is not on the list.

Four separate type/vocabulary violations. FAIL.

### [N/A] MP-4 Section 22 language neutrality

The SPEC is scoped to a single-language Go project (`internal/ritual/scheduler/` Go package, `robfig/cron/v3`, `rickar/cal/v2`, `go.uber.org/zap`). No multi-language tooling claims. MP-4 auto-passes.

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.70 | 0.50–0.75 | REQs are precise. AC-SCHED-005 (spec.md:L182) permits "둘 중 하나 보장" — non-deterministic test. R1 "1시간 이상 지체 시 스킵, 그 이하는 즉시 1회 발화" (spec.md:L381) and REQ-SCHED-013 SuppressionKey differs from research.md §6.1 (2-tuple vs 3-tuple). |
| Completeness | 0.70 | 0.50–0.75 | All major sections present (HISTORY, Overview, Background, Scope, Requirements, AC, Technical Approach, Dependencies, Risks, References, Exclusions). Frontmatter incomplete (see MP-3). AC count (9) undercovers REQ count (20). |
| Testability | 0.55 | 0.50 | Most ACs are measurable. Weaknesses: (a) AC-SCHED-005 permits two contradictory outcomes; (b) AC-SCHED-002 "mock clock을 Asia/Seoul 07:30에 도달시키고 1초 대기" — "1초 대기" is timing-dependent; (c) AC-SCHED-006 confidence threshold 0.7 is testable but PatternLearner Predict API signature at spec.md:L293 returns `(LocalClock string, confidence float64, err error)` with no hint that Predict accepts 7-day history input. |
| Traceability | 0.45 | 0.25–0.50 | **11 of 20 REQs have no corresponding AC**. AC→REQ mapping is implicit (no explicit REQ references in AC text). Uncovered REQs: REQ-SCHED-004 (log format), REQ-SCHED-009 (Start lifecycle errors), REQ-SCHED-010 (disabled state inert), REQ-SCHED-013 (dup suppression — AC-SCHED-008 is the restart variant only), REQ-SCHED-014 (quiet hours overrideable nighttime), REQ-SCHED-015 (buffered channel decoupling), REQ-SCHED-016 (±2h cap + 3 consecutive observations), REQ-SCHED-017 (Korean holidays — AC-SCHED-004 covers 1 holiday only), REQ-SCHED-018 (skip_weekends), REQ-SCHED-019 (03:00 daily learner + confirmation), REQ-SCHED-020 (FastForward API). |

---

## Defects Found

**D1.** spec.md:L5 — Frontmatter key `created` should be `created_at` (ISO date string). MP-3 violation. — Severity: **critical**

**D2.** spec.md:L1–L13 — Frontmatter missing required `labels` field (array or string). MP-3 violation. — Severity: **critical**

**D3.** spec.md:L8 — `priority: P0` uses non-standard vocabulary. Required: {critical, high, medium, low}. MP-3 violation. — Severity: **critical**

**D4.** spec.md:L4 — `status: Planned` not in permitted set {draft, active, implemented, deprecated}. MP-3 violation. — Severity: **major**

**D5.** spec.md:L159–L202 — All 9 acceptance criteria use Given/When/Then format, not one of the five EARS patterns. MP-2 potential violation; at minimum requires a parallel EARS restatement or explicit project convention note. — Severity: **major**

**D6.** spec.md:L182 — AC-SCHED-005 "반환 error가 ErrQuietHoursViolation, 또는 경고 후 07:00으로 clamp (정책은 구현에서 결정, 테스트는 **둘 중 하나** 보장)" is non-deterministic and not binary-testable. A single AC MUST specify exactly one outcome. This contradicts the AC-2 rule (binary PASS/FAIL without judgment). — Severity: **critical**

**D7.** spec.md:L137 (REQ-SCHED-013) vs research.md:L115–L117 (§6.1) — SuppressionKey definition conflicts:
- spec.md:L137: `{event, userLocalDate}` (2-tuple)
- research.md:L115: `key = fmt.Sprintf("%s:%s:%s", event, userLocalDate, tz)` (3-tuple)
- Risk R6 at spec.md:L386 further specifies `{event}:{userLocalDate}:{TZ}` (3-tuple)

The SPEC is internally contradictory on a security-relevant invariant (duplicate suppression under TZ change). — Severity: **critical**

**D8.** spec.md:L139–L141 (REQ-SCHED-014) — Quiet hours floor `[23:00, 06:00]` permits override via `config.scheduler.allow_nighttime: true`, but AC-SCHED-005 uses `morning.time="02:30"` with `allow_nighttime=false` and expects either `ErrQuietHoursViolation` or clamp to 07:00. The override semantics are unclear: does `allow_nighttime=true` also bypass PatternLearner recommendations in the 23:00–06:00 window? Unspecified. — Severity: **major**

**D9.** spec.md:L384 (R4) — "Backoff는 **defer**만, 스킵 아님. 최대 N회(3회) defer 후 강제 emit" is NOT expressed as a requirement. Should be a REQ-SCHED-NNN covering the max-defer-count behavior. Currently only in the Risks section. — Severity: **major**

**D10.** spec.md:L387 (R7) — Missed event replay policy "1시간 이하 지체 → 즉시 1회 + '늦어서 죄송' 메시지; 이상 → 스킵" is in Risks only, not requirements. No REQ covers post-restart replay semantics. — Severity: **major**

**D11.** spec.md:L159–L202 — 11 of 20 REQs have no corresponding AC (see Traceability evidence above). Traceability floor violated. — Severity: **critical**

**D12.** spec.md:L119 (REQ-SCHED-006) — Introduces `RitualTimeProposal` event/type that is never defined elsewhere in spec.md. Technical approach (§6.2, spec.md:L228–L295) does not include this type. — Severity: **major**

**D13.** spec.md:L73 (Scope IN.4) promises 5 HookEvent constants, and AC-SCHED-001 (spec.md:L162) claims "HOOK-001의 HookEventNames()에도 이 5개가 추가되어 총 29개 (AC-HK-001 확장)". This cross-SPEC assertion (29 = 24 + 5) is unverifiable from this SPEC alone and creates an implicit coupling that should be flagged for HOOK-001 synchronization. — Severity: **minor**

**D14.** spec.md:L131 (REQ-SCHED-011) — "the most recent QueryEngine turn occurred within `config.scheduler.backoff.active_window_min`" but `config.scheduler.backoff.active_window_min: 10` at spec.md:L86 is an integer (minutes). The comparison semantic is clear, but AC-SCHED-003 mocks `LastTurnAt = time.Now() - 5min` (spec.md:L170) — 5 < 10, so defer expected. OK, but specification could be tighter on type. — Severity: **minor**

**D15.** spec.md:L147 (REQ-SCHED-017) — Lists Korean holidays but does NOT include "대체공휴일" (substitute holidays) in the enumerated set, though "+ 대체공휴일" is appended separately. Risk R2 and research.md §2 both claim cal/v2 supports substitute holidays — fine — but REQ-SCHED-017 should explicitly enumerate the substitute-holiday rule to be testable. No AC covers substitute holidays. — Severity: **minor**

**D16.** spec.md:L153 (REQ-SCHED-020) — `FastForward(duration)` API is a **production API** in the Scheduler struct (spec.md:L276 `func (s *Scheduler) FastForward(d time.Duration)` with comment "test-only"). Production struct exposing test-only method is a design smell; should be gated behind a test build tag or separate interface. No AC enforces this gating. — Severity: **major**

**D17.** spec.md:L117 (REQ-SCHED-005) — Backoff reschedule to "+10min" is hardcoded, but spec.md:L86 exposes `active_window_min: 10` as config. The reschedule interval should either reference the same config key or be separately configurable. Inconsistency between config flexibility and REQ hardcode. — Severity: **minor**

**D18.** spec.md:L123 (REQ-SCHED-008) — Timezone shift pause of 24h is hardcoded with no config override. User traveling for short trips may not want 24h full pause. — Severity: **minor**

**D19.** research.md:L130 — `clockwork.Clock` (jonboulle/clockwork) introduced as test dependency but not listed in spec.md §6.6 Library Decisions (spec.md:L340) or §7 Dependencies (spec.md:L358–L373). — Severity: **minor**

**D20.** spec.md:L66 (IN scope list) and spec.md:L92 (OUT of scope) — "다중 사용자 분리 스케줄 (Family mode): adaptation.md §9.2 후속 SPEC." OUT scope references `adaptation.md §9.2`, but research.md §8 Korean market considerations mentions "학생 모드" / "군대 시간표" / `persona.occupation` fields which would require multi-profile support. Internal inconsistency: research suggests per-persona tuning but scope excludes multi-user. — Severity: **minor**

---

## Chain-of-Verification Pass

Second-pass re-examination:

- **REQ numbering**: Re-counted REQ-SCHED-001 to REQ-SCHED-020 across 5 subsections. Sequential, no duplicates, no gaps. Confirmed.
- **AC coverage map**: Enumerated each REQ → AC mapping explicitly. Confirmed 9 of 20 REQs have ACs (REQ-001→AC-SCHED-001, REQ-002→AC-SCHED-001, REQ-003→AC-SCHED-007, REQ-005→AC-SCHED-002+003, REQ-006→AC-SCHED-006, REQ-007→AC-SCHED-004, REQ-008→AC-SCHED-009, REQ-011→AC-SCHED-003, REQ-012→AC-SCHED-006). Uncovered: REQ-SCHED-004, 009, 010, 013 (partial), 014, 015, 016, 017 (partial), 018, 019, 020. Traceability gap confirmed and expanded — D11 stands.
- **Exclusions check**: spec.md:L418–L428. 9 specific exclusion entries, each concrete (not vague). Confirmed PASS on SC-6.
- **Contradictions check**: Found SuppressionKey 2-tuple vs 3-tuple (D7 already reported). Re-checking: spec.md:L137 uses 2-tuple; spec.md:L255 (`SuppressionKey string // {event}:{yyyy-mm-dd-local}`) matches 2-tuple; spec.md:L386 (R6) and research.md:L115 use 3-tuple. The struct field comment (L255) aligns with REQ-SCHED-013 but contradicts R6 mitigation. This is a **more severe** contradiction than first pass suggested — the canonical code contract (L255) does NOT include TZ, meaning R6's mitigation is unimplementable under the current design.
- **Weasel words scan**: AC-SCHED-005 "둘 중 하나" (D6). AC-SCHED-002 "1초 대기" (timing-dependent, not weasel but fragile). AC-SCHED-006 "신뢰도 ≥ 0.7" (binary). No other weasel words detected.
- **Priority labels consistency**: spec.md:L8 `priority: P0` — Phase 7 Daily Companion core infrastructure. Consistent with Scope as a foundational blocker for BRIEFING/HEALTH/JOURNAL/RITUAL. OK.

New defect discovered in second pass:

**D21.** spec.md:L255 — `SuppressionKey string // {event}:{yyyy-mm-dd-local}` in the ScheduledEvent struct comment omits TZ, contradicting R6 mitigation. The canonical Go struct is the authoritative contract; the Risks section cannot unilaterally extend it. Either the struct comment must be updated to `{event}:{yyyy-mm-dd-local}:{tz}` OR R6 must be revised to not claim TZ-aware suppression. — Severity: **critical**

---

## Regression Check

Not applicable — iteration 1.

---

## Recommendation

**FAIL.** manager-spec must address the following before resubmission:

1. **[D1–D4] Frontmatter fixes** (spec.md:L1–L13):
   - Rename `created` → `created_at`
   - Add `labels: [scheduler, ritual, hook, phase-7, daily-companion]`
   - Change `priority: P0` → `priority: critical`
   - Change `status: Planned` → `status: draft`

2. **[D5] Acceptance criteria format decision**:
   - Either rewrite all 9 ACs in EARS-style ("The scheduler shall..." with Given as "When X..." / "While Y..."), OR
   - Add an explicit project-level override note at top of §5 stating "ACs intentionally use Given/When/Then per project convention; each AC maps 1:1 to a REQ-SCHED-XXX EARS requirement" and add explicit REQ-XXX tags to each AC header.

3. **[D6] Fix AC-SCHED-005 non-determinism**:
   - Pick one policy (reject OR clamp) and specify the single expected outcome. AC cannot tolerate two valid outcomes.

4. **[D7, D21] Resolve SuppressionKey contradiction** (spec.md:L137 vs L255 vs L386 vs research.md:L115):
   - Canonical decision: include TZ in the key (3-tuple `{event}:{yyyy-mm-dd-local}:{tz}`)
   - Update REQ-SCHED-013 text, ScheduledEvent struct comment, and Risk R6 to match
   - Add dedicated AC verifying TZ-shift-aware suppression

5. **[D11] Fill traceability gaps** — add ACs for:
   - REQ-SCHED-004 (log schema verification)
   - REQ-SCHED-009 (Start partial-failure → Stopped state invariant)
   - REQ-SCHED-010 (disabled mode inertness)
   - REQ-SCHED-014 (quiet hours override)
   - REQ-SCHED-015 (cron-vs-dispatcher decoupling via buffered channel)
   - REQ-SCHED-016 (±2h cap + 3-observation commit rule)
   - REQ-SCHED-018 (skip_weekends)
   - REQ-SCHED-019 (03:00 learner run + confirmation flow)
   - REQ-SCHED-020 (FastForward gating)

6. **[D9, D10] Promote Risk mitigations to REQs**:
   - R4 max-defer-count → new REQ-SCHED-021 (Unwanted: shall not defer more than 3 times; 4th attempt forces emit)
   - R7 missed-event replay → new REQ-SCHED-022 (Event-Driven: on Start, replay last missed trigger if ≤1h stale)

7. **[D12] Define `RitualTimeProposal`**:
   - Add type definition in §6.2 with fields (OldTime, NewTime, Kind, Confidence, SupportingDays)
   - Clarify whether it is emitted as a HookEvent or returned from PatternLearner

8. **[D16] Gate FastForward production exposure**:
   - Use build tag (`//go:build test_only`) or separate interface `TestableScheduler`
   - Document in §6.6 that production build excludes this symbol

9. **[D19] Add clockwork to §6.6 and §7**:
   - `jonboulle/clockwork` v0.4+ as test-only dependency

10. **[D17, D18] Make timing constants configurable** where appropriate:
    - Backoff reschedule interval (currently hardcoded +10min in REQ-SCHED-005)
    - Timezone-shift pause duration (currently hardcoded 24h in REQ-SCHED-008)

Upon resubmission, the next audit iteration will verify resolution of each listed defect.

---

Version: plan-auditor 1.0 (M1–M6 active)
Generated: 2026-04-24
