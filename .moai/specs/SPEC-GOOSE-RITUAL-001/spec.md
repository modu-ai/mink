---
id: SPEC-GOOSE-RITUAL-001
version: 0.3.0
status: planned
created_at: 2026-04-22
updated_at: 2026-04-25
author: manager-spec
priority: P0
issue_number: null
phase: 7
size: 대(L)
lifecycle: spec-anchored
labels: [ritual, orchestration, bond-score, streak, mood-adaptive, state-machine]
---

# SPEC-GOOSE-RITUAL-001 — Daily Ritual Orchestrator (Morning + Meals×3 + Evening, Bond Level, Streaks, Mood-adaptive)

> **v0.2 Amendment (2026-04-24)**: SPEC-GOOSE-ARCH-REDESIGN-v0.2 와 정합 확인.
> 본 SPEC의 mood-adaptive 패턴은 v0.2 설계의 "Adaptive Ritual Engine" 개념과 일치한다.
> 추가 확장 포인트: `if_weather` / `if_day_of_week` / `if_growth_stage` adaptive 규칙.
> 실행 이력은 `./.goose/rituals/{id}/runs/` + `ritual_runs` DB 테이블에 기록.
> 구현 시 `.moai/design/goose-runtime-architecture-v0.2.md` §9 참조.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 #38, 전체 리추얼 통합) | manager-spec |
| 0.2.0 | 2026-04-25 | 감사 리포트 `RITUAL-001-audit.md` Must-Pass/Critical/Major 수정. (1) 프론트매터 `labels` 값 추가(MP-3/D1), `updated_at` 갱신. (2) REQ-012~015를 `[Unwanted]`→`[Ubiquitous]`로 재분류(MP-2/D2). (3) §5 서두에 Given-When-Then 시나리오 포맷 선언(AC↔EARS 매핑 명시)으로 MP-2 (b)/D3 해소. (4) **REQ-019 추가**: RitualCompletion 상태 전이(scheduled→triggered→{responded|skipped|failed}) 규칙 + 타임아웃 윈도우 + skip/failed 구분 명세(D5). (5) AC-013 추가(상태 전이). (6) REQ-012와 REQ-018 A2A 정책 모순 해소(D8, REQ-012에 "REQ-RITUAL-018 opt-in 경로는 예외" 절 추가). (7) REQ-016 "add 2× Bond score" → "multiply by 2" 수정(D9, AC-012 산식과 정합). | manager-spec |
| 0.3.0 | 2026-04-25 | 감사 리포트 `RITUAL-001-review-2.md` iter 3 후속 수정. (1) **§5 13개 AC 전부 EARS 패턴으로 직접 변환**(MP-2 b / D14·D3 해소): 각 AC를 단일 EARS sentence(Event-Driven / State-Driven / Ubiquitous)로 재작성하고 setup은 본문 내 "under conditions [...]" 절로 통합, 다중 assertion은 (a)/(b)/(c) nested clauses로 분리. Given-When-Then 포맷 선언 블록 폐기, EARS 패턴 라벨로 대체. (2) **REQ 본문 구현 누출 제거**(D6/D16): REQ-001 `goosed` startup → "orchestrator bootstrap phase", REQ-002 `ErrPersistFailed` → "typed persistence-failure error", REQ-003 zap 필드명 → 행위 단위 5필드 기술, REQ-005 6-step a–f → 행위 단위 5단계, REQ-009 `ErrRitualsDisabled` → "typed rituals-disabled error", REQ-010 `< 0.3` → "configured negative-mood threshold (default 0.3, parameterized)" parameterize. (3) **REQ-014 quiet hours config 분리**(D10/D18): 23:00–06:00 hardcode → `config.rituals.quiet_hours.{start,end}` (default 23:00/06:00). (4) **REQ-018 분리**(D11/D19): REQ-018a [Optional] aggregated 메트릭 opt-in 공유, REQ-018b [Ubiquitous] 원시 payload 공유 절대 금지. REQ-012 cross-reference를 REQ-018a/018b로 업데이트. (5) **§4.7 신규**(D7/D17 해소): full-day bonus(REQ-020 [State-Driven]) 및 streak day eligibility(REQ-021 [State-Driven])를 normative REQ로 승격. AC-004는 REQ-020을, AC-005는 REQ-021을 verifies. | manager-spec |

---

## 1. 개요 (Overview)

GOOSE v6.0 Daily Companion의 **최상위 orchestration 레이어**. Phase 7의 모든 하위 SPEC(SCHEDULER/BRIEFING/HEALTH/JOURNAL)을 **하나의 일관된 하루 경험**으로 묶는다.

```
🌅 Morning (07:30)   → BRIEFING (운세·날씨·일정)
🍽️ Breakfast (08:30) → HEALTH (약·식사)
🍽️ Lunch (12:30)     → HEALTH (약·식사)
🍽️ Dinner (19:00)    → HEALTH (약·식사)
🌙 Evening (22:30)    → JOURNAL (안부·일기)
```

5개 ritual point가 하루를 채운다. 본 SPEC은 이를:

1. **통합 관리**: 각 ritual의 실행 여부 / 완수도 추적 (`RitualCompletion`)
2. **Bond Level 연결**: 다마고치 Nurture Loop와 연동 — ritual 완수 → Bond +N
3. **Streak 집계**: 7일 / 30일 / 100일 연속 달성 트로피
4. **Mood-adaptive 조정**: 사용자 현재 감정에 따라 ritual 강도·톤 자동 변경
5. **Customization**: 사용자가 ritual on/off, 시간 조정, 내용 수정
6. **Milestone 알림**: 7일 연속, 첫 30일, 100일 등 기념 메시지

본 SPEC이 통과한 시점에서 `internal/ritual/orchestrator/` 패키지는:

- `RitualOrchestrator` 가 5개 ritual 의 status를 통합 관리,
- `RitualCompletion` 이 MEMORY-001에 영속,
- `BondLevelCalculator` 가 완수도를 Nurture Loop 점수로 변환,
- `StreakTracker` 가 연속 일수 집계,
- `MoodAdaptiveStrategy` 가 사용자 mood (INSIGHTS-001) 기반 ritual 조정,
- `MilestoneNotifier` 가 기념 이벤트 발화.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- 사용자 지시(2026-04-22): 여러 리추얼을 별도로 분리 구현하면 **연결감 없음**. "반려AI" 컨셉은 하루 전체 리듬이 통합되어야 성립.
- Phase 7의 다른 7개 SPEC이 각자 독립 동작하면 사용자 경험 파편화. RITUAL-001이 **"하루"를 묶는 메타 레이어**.
- 다마고치 Nurture Loop (기존 Phase 4+) 와 Daily Rituals (Phase 7) 를 명시적으로 연결. 
- adaptation.md §12 Personalization Success 메트릭 (Session Frequency, D30 Retention, NPS) 의 실증 장소.

### 2.2 상속 자산

- **SCHEDULER-001**: 5개 이벤트 원천.
- **BRIEFING-001, HEALTH-001, JOURNAL-001**: ritual 본체.
- **MEMORY-001**: completion 영속.
- **INSIGHTS-001**: mood 정보.
- **기존 Phase (Tamagotchi UX)**: Bond Level, Affection Point 개념 (별도 UI SPEC에서 정의, 본 SPEC은 Bond score 계산 계약 제공).

### 2.3 범위 경계

- **IN**: 5개 ritual 통합, RitualCompletion 추적, Bond score 계산 API, Streak 집계, Milestone 발화, Mood-adaptive 조정, 사용자 customization, 주간/월간 리포트.
- **OUT**: Tamagotchi UI 자체 (sprite 애니메이션, affection 포인트 UI), 각 ritual 본체 로직, push notification, voice output, A2A 공유.

---

## 3. 스코프

### 3.1 IN SCOPE

1. `internal/ritual/orchestrator/` 패키지.
2. `RitualOrchestrator`:
   - `Start(ctx) error` — 5 ritual consumer 등록 (HOOK-001)
   - `RecordCompletion(userID, RitualKind, completion RitualCompletion) error`
   - `GetTodayStatus(userID) *DayStatus`
   - `WeeklyReport(userID) *WeeklyReport`
   - `MonthlyReport(userID) *MonthlyReport`
3. `RitualKind` enum: `Morning, Breakfast, Lunch, Dinner, Evening` (5종).
4. `RitualCompletion`:
   ```
   {
     UserID, RitualKind, Date,
     Status: "scheduled"|"triggered"|"responded"|"skipped"|"failed",
     ResponseLatencyMs *int,  // trigger → 사용자 응답까지
     Quality: "full"|"partial"|"none",  // briefing 읽음, meal log 완수 등
     Payload map[string]any,  // ritual별 추가 정보
     CreatedAt
   }
   ```
5. `DayStatus`:
   ```
   {
     Date, UserID,
     Rituals map[RitualKind]*RitualCompletion,
     CompletedCount int,
     FullCompleteCount int,
     IsFullDay bool,  // 5개 전부 full
     BondScoreEarned float64
   }
   ```
6. `BondLevelCalculator`:
   - `ScoreForCompletion(completion) float64`
   - 기본 점수:
     - Morning briefing 완수 (user acknowledged): +1.0
     - Meal log (식사 + 약) 완수: +0.5 × 3 = 1.5
     - Evening journal 완수: +2.0
     - 하루 전체 full: bonus +1.0 = 총 최대 5.5/day
   - `GetTotalScore(userID, from, to) float64`
7. `StreakTracker`:
   - `CurrentStreak(userID) int` — 현재 연속 일수 (최소 morning+evening 완수)
   - `LongestStreak(userID) int`
   - `StreakBreaks(userID, year int)` — 끊긴 날 로그
8. `MoodAdaptiveStrategy`:
   - `AdjustRitualStyle(ritual RitualKind, currentMood Vad) RitualStyleOverride`
   - 예: mood valence < 0.3 → `tone: "gentle", length: "short", skip_fortune: true`
   - 반대: mood 매우 높음 → `tone: "playful"`
9. `MilestoneNotifier`:
   - 7일 연속 → "일주일 연속 함께 해주셨네요. 감사합니다 🎊"
   - 30일 연속 → "한 달, 대단해요!"
   - 100일 → 특별 트로피
   - 사용자 생일 당일 → 축하 ritual 우선
10. 사용자 customization:
    - `UpdateRitualSchedule(userID, kind, schedule)`
    - `DisableRitual(userID, kind)`
    - `RestoreDefaultRituals(userID)`
11. Data privacy:
    - Completion 데이터는 MEMORY-001 `facts` (session_id="rituals") 로컬
    - A2A 공유 금지 (JOURNAL-001과 동일 원칙)
12. Config:
    ```yaml
    rituals:
      enabled: true  # Phase 7 전체 활성
      auto_adapt_to_mood: true
      bond_score_enabled: true
      milestones:
        enabled: true
        streak_targets: [7, 30, 100, 365]
    ```

### 3.2 OUT OF SCOPE

- **Tamagotchi sprite 애니메이션 UI** — 별도 UI SPEC.
- **Affection Point, Mood Gauge visual** — UI layer.
- **Push notification 발송** — Gateway.
- **Voice output** — BRIEFING-001 이 이미 담당.
- **각 ritual 본체 로직** — BRIEFING/HEALTH/JOURNAL 에 위임.
- **A2A 공유** — RITUAL 데이터는 로컬 only.
- **Social features (친구와 streak 비교)** — 범위 외.
- **Gamification badges UI** — 점수 계산까지만, 시각화는 UI SPEC.
- **Reminders beyond SCHEDULER** — 추가 알림 로직 없음.
- **Cross-user synchronization** — 단일 사용자 세션.

---

## 4. EARS 요구사항

### 4.1 Ubiquitous

**REQ-RITUAL-001 [Ubiquitous]** — The orchestrator **shall** register HOOK-001 consumers for exactly 5 ritual events (MorningBriefing, PostBreakfast, PostLunch, PostDinner, EveningCheckIn); a registration failure occurring during the orchestrator's bootstrap phase **shall** prevent the orchestrator from entering its operational state and **shall** be surfaced to the caller as a typed bootstrap-failure error.

**REQ-RITUAL-002 [Ubiquitous]** — Every `RitualCompletion` **shall** be persisted to MEMORY-001 within 1 second of status update; a persistence failure **shall** be retried 3 times with exponential backoff and, if all retries fail, **shall** be surfaced to the caller as a typed persistence-failure error.

**REQ-RITUAL-003 [Ubiquitous]** — For every completion recording, the orchestrator **shall** emit a structured INFO-level log event whose fields cover (a) hashed user identifier, (b) ritual kind, (c) terminal status, (d) Bond score delta applied, and (e) the user's current streak length; raw user identifiers **shall not** appear in any log field.

**REQ-RITUAL-004 [Ubiquitous]** — All Bond score calculations **shall** be deterministic — identical `RitualCompletion` inputs **shall** produce identical scores regardless of call time.

### 4.2 Event-Driven

**REQ-RITUAL-005 [Event-Driven]** — **When** a sub-ritual SPEC (BRIEFING/HEALTH/JOURNAL) reports completion via its callback API, the orchestrator **shall** (a) translate the callback payload into a `RitualKind`-typed `RitualCompletion` record, (b) persist the record via the completion-recording pathway, (c) apply the resulting Bond score delta to the user's accumulated total, (d) update streak state for the affected user and date, and (e) when this update crosses a streak threshold for the first time, dispatch a milestone notification.

**REQ-RITUAL-006 [Event-Driven]** — **When** `StreakTracker.CurrentStreak(userID)` exceeds a `streak_targets` value for the first time, `MilestoneNotifier.Notify(milestone)` **shall** be called once; the emitted event **shall** be recorded to prevent duplicate notifications.

**REQ-RITUAL-007 [Event-Driven]** — **When** `MoodAdaptiveStrategy.AdjustRitualStyle` is invoked for an upcoming ritual AND `config.rituals.auto_adapt_to_mood == true`, the returned `RitualStyleOverride` **shall** be passed to the ritual executor (e.g., BRIEFING-001's `BriefingStyle`) via orchestrator-injected context.

**REQ-RITUAL-008 [Event-Driven]** — **When** a user requests `DisableRitual(userID, Morning)`, the orchestrator **shall** (a) unsubscribe the SCHEDULER trigger for morning for this user, (b) record the customization in MEMORY-001, (c) acknowledge via UI.

### 4.3 State-Driven

**REQ-RITUAL-009 [State-Driven]** — **While** `config.rituals.enabled == false`, the orchestrator **shall not** register any HOOK consumers; no ritual events **shall** be processed; `GetTodayStatus` **shall** return a typed "rituals-disabled" error to its caller.

**REQ-RITUAL-010 [State-Driven]** — **While** a user's mood trend over the last 3 days shows persistent negative valence (defined as average valence below the configured negative-mood threshold, default 0.3, parameterized via `config.rituals.mood_adaptive.negative_valence_threshold`), `MoodAdaptiveStrategy` **shall** default all rituals to `tone: "gentle", length: "short"`; the morning fortune section **shall** be auto-disabled if the resolved style is "gentle" and the fortune style is "mystical".

**REQ-RITUAL-011 [State-Driven]** — **While** today's `EveningCheckIn` completion status is "skipped" OR "failed" for 3 consecutive days, the orchestrator **shall** send a gentle nudge at the next morning briefing: "요즘 저녁에 못 보인 것 같아 조금 걱정됐어요. 잘 지내시죠?"

### 4.4 Ubiquitous Prohibitions

> **Note (v0.2.0)**: The four requirements below were originally labelled `[Unwanted]` in v0.1.0.
> Per audit `RITUAL-001-audit.md` D2, they are system-wide prohibitions ("shall not …"),
> not conditional failure-mode responses (`If <undesired>, then <response>`).
> They are therefore EARS **Ubiquitous** (negative form) and relabelled as such.
> REQ numbers are preserved for traceability with v0.1.0.

**REQ-RITUAL-012 [Ubiquitous]** — The orchestrator **shall not** use completion data for ML training or external sharing; completions remain local (MEMORY-001) and serve only as input to Bond score / Streak / Mood-adaptive logic within this process. **Exception**: the opt-in aggregated-metrics path governed by REQ-RITUAL-018a (anonymized counters only, no payloads) is explicitly permitted and does not violate this prohibition; raw payload sharing remains absolutely prohibited under REQ-RITUAL-018b.

**REQ-RITUAL-013 [Ubiquitous]** — The orchestrator **shall not** penalize users for missed rituals via guilt-inducing language ("왜 안 오셨어요?", "실망이에요"); all nudges **shall** be empathetic ("괜찮아요", "편할 때 와주세요").

**REQ-RITUAL-014 [Ubiquitous]** — Milestone notifications **shall not** be dispatched during the configured quiet-hours window (parameterized via `config.rituals.quiet_hours.start` and `config.rituals.quiet_hours.end`, default values `23:00` and `06:00` local time respectively); deferred notifications **shall** be re-scheduled to the next non-quiet-hours morning.

**REQ-RITUAL-015 [Ubiquitous]** — The Bond Level API **shall not** expose raw completion history to non-internal callers; only aggregate scores and day-status summaries are surfaced.

### 4.5 Optional

**REQ-RITUAL-016 [Optional]** — **Where** the user's birthday is in IDENTITY-001 `important_dates` AND today matches, the orchestrator **shall** prepend a special greeting to all rituals of the day ("생일 축하합니다 🎂") and **multiply** the day's total Bond score by 2 (applied to the sum of base scores and full-day bonus; see §6.3). This is a multiplicative modifier, not an additive one; AC-RITUAL-012 (5.5 × 2 = 11.0) fixes the arithmetic.

**REQ-RITUAL-017 [Optional]** — **Where** `config.rituals.weekly_report == true`, every Sunday 22:00 local, a weekly report **shall** be generated summarizing completion rates, Bond total, streak status, and mood trend.

**REQ-RITUAL-018a [Optional]** — **Where** A2A-001 is active AND the user has explicitly opted in via the documented consent surface, the orchestrator **shall** be permitted to share **only aggregated anonymized metrics** (completion count, streak length; no payloads, no identifiers, no narrative content) with trusted peer agents.

**REQ-RITUAL-018b [Ubiquitous]** — The orchestrator **shall not** share raw `RitualCompletion` payloads, individual ritual contents, narrative text, or any user-identifying fields with any external agent under any circumstances. This prohibition is absolute and **shall** apply even when the REQ-RITUAL-018a opt-in is active; REQ-RITUAL-018a's permission **shall** never be interpreted as authorizing any disclosure beyond aggregated anonymized metrics.

### 4.6 State Transitions (v0.2.0 addition — resolves audit D5)

`RitualCompletion.Status` (Section 3.1 item 4) is a 5-state finite enum. The following requirement defines the full transition graph, the event that causes each transition, and the skip-vs-failed distinction.

**Legal transition graph:**

```
                    (hook fires)              (user responds within window)
     scheduled ───────────────────▶ triggered ──────────────────────────────▶ responded [terminal]
         │                              │
         │                              ├─ (timeout window elapses, no user action)
         │                              │      ────────────────────────▶ skipped [terminal]
         │                              │
         │                              └─ (sub-ritual SPEC callback reports error)
         │                                     ────────────────────────▶ failed [terminal]
         │
         └─ (orchestrator shutdown / config.rituals.enabled=false before HOOK fires)
                ──────────────▶ (record deleted, no terminal state written)
```

**Timeout window** = the per-ritual `prompt_timeout_min` (default 60 minutes; see §6.3 and research.md §7.1 config). Measured from the `triggered` timestamp; on expiry the record transitions to `skipped`.

**REQ-RITUAL-019 [Ubiquitous]** — Every `RitualCompletion` record **shall** follow the finite-state transition graph defined in §4.6. Specifically:
1. A record is created in state `scheduled` at the moment the orchestrator subscribes the HOOK-001 consumer for that ritual day.
2. The transition `scheduled → triggered` **shall** occur exactly when the HOOK-001 consumer callback fires for that ritual event; the `triggered_at` timestamp is recorded.
3. The transition `triggered → responded` **shall** occur when a sub-ritual SPEC (BRIEFING/HEALTH/JOURNAL) callback reports successful user engagement within the per-ritual timeout window (default 60 min; overridable via `config.rituals.<kind>.prompt_timeout_min`).
4. The transition `triggered → skipped` **shall** occur automatically when the timeout window elapses without any sub-ritual SPEC callback; skipped is **timer-initiated** (not user-initiated) and is **not** a failure.
5. The transition `triggered → failed` **shall** occur when the sub-ritual SPEC callback reports a non-timeout error (e.g., downstream service unavailable, payload validation failure); failed is **error-initiated** and is distinct from skipped.
6. States `responded`, `skipped`, and `failed` are **terminal**; no further transitions are legal from these states within the same ritual day.
7. Any attempted transition not in the graph above **shall** be rejected by `RecordCompletion` with a typed error and **shall not** mutate the stored record.
8. Retries (REQ-RITUAL-002) apply only to the persistence layer; they **shall not** cause a terminal state to be re-entered or change the record's status.

**Table — Status semantics:**

| Status | Who triggers transition | Meaning | Bond score contribution |
|--------|------------------------|---------|-------------------------|
| `scheduled` | Orchestrator `Start` | Consumer registered, HOOK not yet fired | 0 |
| `triggered` | HOOK-001 consumer callback | Ritual delivered, awaiting user | 0 (intermediate) |
| `responded` | Sub-ritual callback (success) | User engaged within window | Per §6.3 (base × quality) |
| `skipped` | Timer (orchestrator clock) | Window elapsed, no response | 0, streak impact per §6.4 |
| `failed` | Sub-ritual callback (error) | Downstream error, not user-caused | 0, does **not** break streak for current day |

### 4.7 Bond Score & Streak Normative Rules (v0.3.0 addition — resolves audit D7/D17)

The Bond score base values (Section 6.3) and streak day eligibility (Section 6.4) are technical contracts described in §6 for implementation guidance. The following two requirements promote the directly user-observable rules to normative status so that AC-004 and AC-005 verify EARS requirements rather than implementation sections.

**REQ-RITUAL-020 [State-Driven]** — **While** a single ritual day's `RitualCompletion` records satisfy [`FullCompleteCount >= 5`, meaning all 5 ritual kinds reached state `responded` with `quality == "full"`], the day's total Bond score **shall** include an additive full-day bonus of `+1.0` on top of the sum of base scores; this bonus **shall** be applied at most once per `(userID, date)` tuple and **shall** be visible via `GetTodayStatus.BondScoreEarned`.

**REQ-RITUAL-021 [State-Driven]** — **While** evaluating whether a given calendar day counts as a streak day for a user, the orchestrator **shall** treat the day as a streak day if and only if at least one of the user's `Morning` or `Evening` `RitualCompletion` records for that day is in state `responded` with `quality` of `"full"` or `"partial"`; days where both Morning and Evening are in any non-`responded` terminal state (i.e., `skipped` or `failed`) **shall** break the current streak.

---

## 5. 수용 기준

> **Format declaration (v0.3.0 — resolves audit D14 / MP-2 (b))**
>
> Acceptance criteria in this section are written in **EARS syntax**. Each AC consists of a single normative `shall` statement classified by EARS pattern (Ubiquitous / Event-Driven / State-Driven / Optional). Multi-condition setup that would traditionally be expressed as Given clauses is folded into the EARS sentence as a `under conditions [...]` or `while [...]` qualifier. Multi-part observable outcomes are expressed as nested `(a)`, `(b)`, `(c)` clauses within a single `shall` predicate.
>
> Each AC includes a `**Verifies:** REQ-RITUAL-NNN` annotation establishing traceability to the normative §4 requirement(s) it tests, and an `[EARS pattern]` label after the AC heading.
>
> Tooling consumers (e.g., `manager-ddd` / `manager-tdd`) should treat each AC as a directly testable contract and the §6.9 TDD entry list as the canonical mapping from AC to test function.

**AC-RITUAL-001 — 5개 HOOK consumer 등록** [EARS Event-Driven]
**Verifies:** REQ-RITUAL-001

**When** `orchestrator.Start(ctx)` is invoked at orchestrator bootstrap under conditions [`config.rituals.enabled == true`], the orchestrator **shall** register exactly 5 consumers in the HOOK-001 registry, one for each of the ritual events Morning, PostBreakfast, PostLunch, PostDinner, and EveningCheckIn, such that the registry exposes (a) exactly 5 ritual-class consumers attributable to this orchestrator and (b) one consumer per ritual kind with no duplicates.

**AC-RITUAL-002 — Completion 영속** [EARS Event-Driven]
**Verifies:** REQ-RITUAL-002

**When** `orchestrator.RecordCompletion(u1, Morning, {status: "responded", ...})` is invoked under conditions [a sub-ritual SPEC has just reported completion via its callback API], the orchestrator **shall** (a) persist exactly one `RitualCompletion` record to the local memory layer with `session_id == "rituals"`, and (b) make the persisted record retrievable via `GetTodayStatus(u1).Rituals[Morning]` immediately after the call returns.

**AC-RITUAL-003 — Bond score 결정론** [EARS Ubiquitous]
**Verifies:** REQ-RITUAL-004

The Bond score calculator **shall** be deterministic: given the input `RitualCompletion{kind: Evening, status: "responded", quality: "full"}`, 100 consecutive invocations of `BondLevelCalculator.ScoreForCompletion` **shall** each return the identical numeric value `2.0` with zero variance attributable to call timing, invocation order, or process state.

**AC-RITUAL-004 — Full day bonus** [EARS State-Driven]
**Verifies:** REQ-RITUAL-020 (full-day bonus, §4.7), REQ-RITUAL-004 (determinism)

**While** today's `RitualCompletion` records satisfy [all 5 ritual kinds in state `responded` with `quality == "full"`], a query of `GetTodayStatus.BondScoreEarned` **shall** return exactly `5.5`, decomposed as (a) the sum of base scores 1.0 (Morning) + 0.5 × 3 (Breakfast/Lunch/Dinner) + 2.0 (Evening) = 4.5, plus (b) the full-day bonus of 1.0 awarded when all 5 rituals reach `responded` with `quality == "full"`.

**AC-RITUAL-005 — Streak 집계** [EARS State-Driven]
**Verifies:** REQ-RITUAL-021 (streak day eligibility, §4.7), REQ-RITUAL-005 step (d)

**While** the user `u1`'s ritual history satisfies [5 consecutive days where Morning AND Evening transitioned to `responded`, followed by day 6 where Evening transitioned to `skipped`], a query of `StreakTracker.CurrentStreak(u1)` **shall** return (a) `5` when evaluated at the close of day 5, and (b) `0` when evaluated at the close of day 6, reflecting an immediate streak break upon the day-6 Evening skip per §6.4.

**AC-RITUAL-006 — 7일 Milestone 1회 발화** [EARS Event-Driven]
**Verifies:** REQ-RITUAL-006

**When** `StreakTracker.CurrentStreak(u1)` reaches the configured target value `7` for the first time under conditions [`streak_targets == [7, 30, 100]` and no prior `milestone:u1:streak:7` record exists], the orchestrator **shall** invoke `MilestoneNotifier.Notify` exactly once for the 7-day milestone; subsequent `RecordCompletion` calls on the same continuous streak **shall** produce zero additional milestone invocations for the same threshold.

**AC-RITUAL-007 — Mood low → gentle tone** [EARS State-Driven]
**Verifies:** REQ-RITUAL-010

**While** the user's mood trend over the last 3 days satisfies [valence average `0.25`, which is below the configured negative-mood threshold], a call to `AdjustRitualStyle(Morning, currentMood)` **shall** return a `RitualStyleOverride` whose (a) `tone` field equals `"gentle"`, and (b) `length` field equals `"short"`.

**AC-RITUAL-008 — Evening 3일 연속 skip → 다음 morning nudge** [EARS Event-Driven]
**Verifies:** REQ-RITUAL-011

**When** the next morning briefing prompt is being assembled under conditions [the previous 3 consecutive `Evening` `RitualCompletion` records have status `"skipped"`], the orchestrator **shall** inject contextual nudge text into the briefing such that the assembled narrative contains at least one of the empathetic phrases `"요즘 저녁에"` or `"잘 지내시죠"`.

**AC-RITUAL-009 — Guilt-free 언어** [EARS Ubiquitous]
**Verifies:** REQ-RITUAL-013

The orchestrator **shall** avoid guilt-inducing language in user-facing nudges: under conditions [a user has skipped all rituals for 2 consecutive days], the next morning nudge text emitted **shall** (a) contain zero occurrences of any of the keywords `"실망"`, `"왜 안"`, or `"서운"`, and (b) contain at least one keyword from the empathetic set `{"괜찮", "편할 때"}`.

**AC-RITUAL-010 — A2A 데이터 격리 (default-deny)** [EARS State-Driven]
**Verifies:** REQ-RITUAL-012, REQ-RITUAL-018b (absolute payload prohibition)

**While** the A2A connection is active and the user has not provided opt-in consent for aggregated metrics, every `RecordCompletion` invocation **shall** result in exactly zero outbound A2A transmissions attributable to that completion, regardless of completion status, ritual kind, or quality.

**AC-RITUAL-011 — 비활성 시 no-op** [EARS State-Driven]
**Verifies:** REQ-RITUAL-009

**While** `config.rituals.enabled == false`, an invocation of `orchestrator.Start(ctx)` **shall** (a) result in zero HOOK consumer registrations attributable to this orchestrator, and (b) cause every subsequent `GetTodayStatus` invocation to return a typed "rituals-disabled" error to the caller.

**AC-RITUAL-012 — 생일 2x 점수** [EARS State-Driven]
**Verifies:** REQ-RITUAL-016

**While** today matches the user's birthday in IDENTITY-001 `important_dates` AND all 5 ritual kinds are in state `responded` with `quality == "full"`, a query of `GetTodayStatus.BondScoreEarned` **shall** return exactly `11.0`, computed as the regular full-day total (`5.5`) multiplied by the birthday modifier `2` (multiplicative, not additive).

**AC-RITUAL-013 — RitualCompletion 상태 전이 (resolves audit D5)** [EARS Event-Driven]
**Verifies:** REQ-RITUAL-019 (§4.6 state transition graph)

**When** the event sequences described below execute under conditions [`config.rituals.enabled == true`, Morning ritual registered for users `u1`, `u2`, `u3` with `prompt_timeout_min == 60`, clockwork-frozen time simulation], the orchestrator **shall** enforce the §4.6 finite-state transition graph such that:

(a) For user `u1`, given `Start(ctx)` at T0 (record initialized in state `scheduled`), HOOK-001 firing at `07:30` (state transitions to `triggered` with `triggered_at == 07:30`), and a BRIEFING-001 success callback at `07:45`, the stored `Status` transitions **shall** be exactly `scheduled → triggered → responded` with `responded_at == 07:45` recorded; any later `RecordCompletion` call attempting a non-terminal status for the same `(user, kind, date)` tuple **shall** return a typed `"illegal transition"` error and **shall not** mutate the stored record;

(b) For user `u2`, given the same setup but no sub-ritual callback within the 60-minute timeout window, when the clockwork clock advances past `08:30` the orchestrator **shall** transition the record from `triggered` to `skipped` (timer-initiated, not user-initiated), and a late callback arriving at `09:00` **shall** be rejected with a typed `"already terminal"` error such that `Status == "skipped"` is preserved;

(c) For user `u3`, given a BRIEFING-001 callback that reports a downstream error at `07:35`, the orchestrator **shall** transition the record from `triggered` to `failed` (error-initiated, distinct from `skipped`), and the streak for `u3` on this day **shall not** be broken by this `failed` state alone (per §4.6 status semantics table row 5).

---

## 6. 기술적 접근

### 6.1 패키지 레이아웃

```
internal/
└── ritual/
    └── orchestrator/
        ├── orchestrator.go        # RitualOrchestrator
        ├── completion.go          # RecordCompletion + persist
        ├── status.go              # GetTodayStatus, weekly/monthly
        ├── bond.go                # BondLevelCalculator
        ├── streak.go              # StreakTracker
        ├── milestone.go           # MilestoneNotifier
        ├── mood_adapter.go        # MoodAdaptiveStrategy
        ├── customization.go       # Enable/Disable/Update schedule
        ├── nudge.go               # empathy nudges
        ├── reports.go             # WeeklyReport / MonthlyReport
        ├── types.go
        ├── config.go
        └── *_test.go
```

### 6.2 핵심 타입 (의사코드)

```
RitualKind enum: Morning, Breakfast, Lunch, Dinner, Evening

RitualCompletion {
  UserID, RitualKind, Date time.Time
  Status string  ("scheduled"|"triggered"|"responded"|"skipped"|"failed")
  ResponseLatencyMs *int
  Quality string ("full"|"partial"|"none")
  Payload map[string]any
  CreatedAt time.Time
}

DayStatus {
  Date time.Time
  UserID
  Rituals map[RitualKind]*RitualCompletion
  CompletedCount, FullCompleteCount int
  IsFullDay bool
  BondScoreEarned float64
  StreakDays int
}

WeeklyReport {
  From, To time.Time
  TotalDays int
  FullDays int
  PartialDays int
  TotalBondScore float64
  LongestStreakThisWeek int
  MoodTrendSparkline string
  TopMood string
}

RitualStyleOverride {
  Tone string
  Length string
  SkipFortune bool
  SkipMealPrompts bool
  EmojiLevel int
}

RitualOrchestrator struct
  - Start(ctx) error
  - RecordCompletion(ctx, userID, kind, completion) error
  - GetTodayStatus(ctx, userID) (*DayStatus, error)
  - WeeklyReport(ctx, userID, week) (*WeeklyReport, error)
  - AdjustStyle(kind, mood) RitualStyleOverride
  - DisableRitual(ctx, userID, kind) error
  - RestoreDefaults(ctx, userID) error
```

### 6.3 Bond Score 공식

```
base_score(completion):
  if status != "responded": 0
  if kind == Morning:     base = 1.0
  if kind in [Breakfast, Lunch, Dinner]: base = 0.5
  if kind == Evening:     base = 2.0
  
  if quality == "full":    multiplier = 1.0
  if quality == "partial": multiplier = 0.5
  if quality == "none":    multiplier = 0.0
  
  return base * multiplier

day_bonus(day_status):
  if day_status.FullCompleteCount >= 5: +1.0
  if user_birthday(day_status.Date): multiply total × 2

total_day_score = Σ base_score + day_bonus
```

### 6.4 Streak 판정

```
is_streak_day(userID, date):
  rituals = GetRituals(userID, date)
  // 최소 조건: Morning 또는 Evening 중 하나라도 responded + full/partial
  return rituals[Morning].responded OR rituals[Evening].responded

current_streak(userID):
  count = 0
  d = today
  while is_streak_day(userID, d):
    count++
    d = d - 1
  return count
```

### 6.5 Milestone 발화 방지 중복

각 milestone: MEMORY-001에 `milestone:{userID}:{type}:{threshold}` 저장.
- `7일 연속` 첫 달성 후: `milestone:u1:streak:7` 저장
- 같은 streak 연속 유지 중 재발화 없음
- Streak break 후 다시 7일 달성 시: 이전 key 확인 → 이미 존재 → 재발화 없음 (단일 "최초 달성만")
- 30일/100일은 별도 key.

### 6.6 Nudge Generation

```
type NudgeRule struct {
  Condition func(userID, history) bool
  Message   string
  Priority  int
}

rules := []NudgeRule{
  {
    Condition: func(u, h) bool { return evening_skip_count(h, 3) >= 3 },
    Message: "요즘 저녁에 못 보인 것 같아 조금 걱정됐어요. 잘 지내시죠?",
    Priority: 10,
  },
  {
    Condition: func(u, h) bool { return streak_broken_yesterday(h) },
    Message: "어제 잠시 쉬었네요. 오늘 또 시작해봐요, 부담 없이.",
    Priority: 5,
  },
  ...
}
```

BRIEFING-001 prompt 조립 시 orchestrator가 context로 nudge를 전달.

### 6.7 Weekly Report 템플릿

```
📖 지난 주 돌아보기 (2026-04-15 ~ 2026-04-21)

✅ 전체 완수: 5/7일 (71%)
🏆 최장 연속: 3일
💝 얻은 Bond Score: 28.5

🌅 아침 브리핑: 6/7
🍽️ 식사+약: 15/21
🌙 저녁 일기: 5/7

감정 트렌드: ▃▅▆▇▇▁▂  평균 0.58

"수요일이 조금 힘들었지만, 주말엔 활기가 많았어요."
```

### 6.8 라이브러리 결정

| 용도 | 라이브러리 | 근거 |
|------|----------|-----|
| 영속 | MEMORY-001 | |
| Streak / Bond math | stdlib | |
| Sparkline | 자체 | JOURNAL-001 재사용 |
| 로깅 | zap | |

### 6.9 TDD 진입

1. RED: `Test5HookConsumers_Registered` — AC-RITUAL-001
2. RED: `TestCompletion_Persisted` — AC-RITUAL-002
3. RED: `TestBondScore_Deterministic` — AC-RITUAL-003
4. RED: `TestFullDayBonus` — AC-RITUAL-004
5. RED: `TestStreak_5DaysThenBreak` — AC-RITUAL-005
6. RED: `TestMilestone_7Days_OnceOnly` — AC-RITUAL-006
7. RED: `TestMoodLow_GentleTone` — AC-RITUAL-007
8. RED: `Test3EveningSkips_NextMorningNudge` — AC-RITUAL-008
9. RED: `TestGuiltFreeLanguage` — AC-RITUAL-009
10. RED: `TestA2A_Blocked` — AC-RITUAL-010
11. RED: `TestDisabled_NoHookRegistration` — AC-RITUAL-011
12. RED: `TestBirthday_DoubleScore` — AC-RITUAL-012
13. RED: `TestStatusTransitions_ScheduledTriggeredResponded_And_TimeoutSkipped_And_CallbackFailed` — AC-RITUAL-013 (covers REQ-RITUAL-019 state machine; use clockwork time freeze)
14. GREEN → REFACTOR

### 6.10 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| **T**ested | 85%+, 12 AC 전수, 3년치 fake history로 streak/milestone 검증 |
| **R**eadable | bond/streak/milestone/mood_adapter 명확 분리 |
| **U**nified | RitualCompletion DTO 통일, 5 kind enum |
| **S**ecured | 로컬 only, A2A 격리, 로그 redaction, opt-out 즉시 반영 |
| **T**rackable | 모든 completion 로그, weekly/monthly report auto-generation |

---

## 7. 의존성

| 타입 | 대상 | 설명 |
|-----|------|-----|
| 선행 SPEC | SCHEDULER-001 | 5 이벤트 원천 |
| 선행 SPEC | HOOK-001 | consumer 등록 |
| 선행 SPEC | BRIEFING-001 | Morning 본체 + callback |
| 선행 SPEC | HEALTH-001 | Meal×3 본체 + callback |
| 선행 SPEC | JOURNAL-001 | Evening 본체 + callback |
| 선행 SPEC | MEMORY-001 | completion 영속 |
| 선행 SPEC | INSIGHTS-001 | mood 트렌드 |
| 선행 SPEC | IDENTITY-001 | birthday, important_dates |
| 후속 SPEC | (미래) Tamagotchi UI SPEC | Bond score 소비자 |
| 후속 SPEC | INSIGHTS-001 | completion 데이터 → 감정 트렌드 원료 |

---

## 8. 리스크 & 완화

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 5 SPEC 간 순환 의존으로 배포 순서 꼬임 | 중 | 고 | Callback interface 기반 loose coupling, 구현 순서: HOOK → SCHEDULER → BRIEFING/HEALTH/JOURNAL → RITUAL |
| R2 | Bond score 인플레이션 (사용자 gaming) | 낮 | 낮 | 점수 체계 고정, 조작 유인 거의 없음 (UI 제한) |
| R3 | Streak 끊기면 사용자 의욕 저하 | 중 | 중 | guilt-free 메시지, "다시 시작" 격려, streak 복원 못하지만 total score 유지 |
| R4 | Mood-adaptive 가 사용자 기대와 다름 | 중 | 중 | sensitivity 조정 가능 config, user override API |
| R5 | Evening nudge 가 부담스러움 | 중 | 중 | 3일 조건 + 1회 limit, "언제든 괜찮아요" 어조 |
| R6 | 생일 2x bonus 가 주객전도 | 낮 | 낮 | UI에서 명시 ("생일 보너스!"), 하루 한정 |
| R7 | Milestone 재발화 버그 | 중 | 낮 | unique key 기반 dedup, 테스트 코퍼스 |
| R8 | 5개 ritual 모두 opt-out 시 빈 orchestrator | 중 | 낮 | config 검증, 최소 1개 활성 권장 안내 |

---

## 9. 참고

### 9.1 프로젝트 문서

- `.moai/specs/SPEC-GOOSE-SCHEDULER-001/spec.md` — 이벤트 소스
- `.moai/specs/SPEC-GOOSE-BRIEFING-001/spec.md` — Morning 본체
- `.moai/specs/SPEC-GOOSE-HEALTH-001/spec.md` — Meal×3 본체
- `.moai/specs/SPEC-GOOSE-JOURNAL-001/spec.md` — Evening 본체
- `.moai/project/adaptation.md` §12 Personalization Success, §13 로드맵
- (기존 Phase 4) Tamagotchi Nurture Loop 개념

### 9.2 외부 참조

- Habit formation research: BJ Fogg's Tiny Habits
- Streak psychology: Duolingo / Snapchat streak design
- Gamification ethics: https://octalysisgroup.com/ (윤리적 gamification 가이드)
- VAD mood model: Russell & Mehrabian 1977

### 9.3 부속 문서

- `./research.md` — Bond 점수 공식 설계 근거, milestone 심리학, mood-adaptive 실험

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **Tamagotchi sprite UI를 구현하지 않는다** (별도 UI SPEC).
- 본 SPEC은 **각 ritual 본체를 구현하지 않는다** (BRIEFING/HEALTH/JOURNAL에 위임).
- 본 SPEC은 **Push notification 발송을 포함하지 않는다** (Gateway).
- 본 SPEC은 **Voice output 을 포함하지 않는다** (BRIEFING-001).
- 본 SPEC은 **Social features (친구 streak 비교) 를 포함하지 않는다**.
- 본 SPEC은 **A2A 공유를 기본 제공하지 않는다** (opt-in + aggregate only).
- 본 SPEC은 **Guilt-inducing 언어를 금지한다**.
- 본 SPEC은 **Cross-user ritual 동기화를 포함하지 않는다**.
- 본 SPEC은 **ML 기반 sentiment 분석 고도화를 포함하지 않는다** (JOURNAL-001 VAD 재사용).
- 본 SPEC은 **Gamification badges UI를 포함하지 않는다** (점수만).
- 본 SPEC은 **Reminders beyond SCHEDULER를 추가하지 않는다**.
- 본 SPEC은 **Completion 데이터를 LoRA 훈련에 자동 포함하지 않는다** (사용자 opt-in).

---

**End of SPEC-GOOSE-RITUAL-001**
