---
id: SPEC-GOOSE-SCHEDULER-001
version: 0.2.1
status: completed
created_at: 2026-04-22
updated_at: 2026-05-10
author: manager-spec
priority: critical
issue_number: null
phase: 7
size: 중(M)
lifecycle: spec-anchored
labels: [scheduler, ritual, hook, phase-7, daily-companion]
---

# SPEC-GOOSE-SCHEDULER-001 — Proactive Ritual Scheduler (Cron-like, Timezone/Holiday-aware, User-pattern Learning)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 Daily Companion #31, HOOK-001 확장) | manager-spec |
| 0.2.0 | 2026-04-25 | plan-auditor iter-1 결함 수정: MP-2(AC format 선언), MP-3(frontmatter 정규화), D6(AC-005 결정론화), D7+D21(SuppressionKey 3-tuple 통일), D11(REQ-004/009/010/013/014/015/016/018/019/020 AC 신설), D9/D10(REQ-021/022 승격), D12(RitualTimeProposal 정의), D16(FastForward build tag 명시), D19(clockwork 의존성 등재). research.md 무변경. | manager-spec |
| 0.2.1 | 2026-05-10 | implementation 완수 (5 PR admin-bypass merged): #133 P1 Cron+Events+Persist (5 AC, coverage 91.0%) → #135 P2 Timezone+Holiday (3 AC, 89.9%) → #136 P3 Backoff+Worker+QuietHours (5 AC, 89.1%) → #137 P4a PatternLearner+DailyCron (3 AC, 84.8%) → #138 P4b Suppression+LogSchema+FastForward+Replay (4 AC, 84.1%). 누적 20/20 AC GREEN, 39 production tests + 1 test_only test PASS, race-clean, golangci-lint 0 issues. status `audit-ready` → `completed`. 설계 결정 — DI seam 패턴 일관성 (ActivityClock/PatternReader/FiredKeyStore interface로 QUERY-001/INSIGHTS-001/MEMORY-001 침범 회피). | orchestrator (manager-tdd 1M context API 차단 정책 예외) |

---

## 1. 개요 (Overview)

GOOSE v6.0 **Daily Companion Edition**의 **Layer 3 (Daily Rituals)**를 구동하는 **proactive scheduler**를 정의한다. 사용자가 호출하지 않아도, 학습된 하루 리듬(기상 / 식사 ×3 / 취침)에 맞춰 **SCHEDULER가 먼저 이벤트를 emit**하면 하위 SPEC(BRIEFING / HEALTH / JOURNAL / RITUAL)이 리추얼을 실행한다.

SCHEDULER-001은 SPEC-GOOSE-HOOK-001의 lifecycle hook 시스템을 **time-based trigger로 확장**한다. HOOK-001이 QueryEngine의 tool-use / file-changed / session-start 같은 **agent-driven** 이벤트를 다뤘다면, SCHEDULER-001은 **wall-clock-driven** 이벤트(`MorningBriefingTime`, `PostMealTime`, `EveningCheckInTime`)를 생성하여 HOOK-001 dispatcher를 통해 전파한다.

본 SPEC이 통과한 시점에서 `internal/ritual/scheduler/` 패키지는:

- **Cron-like scheduler**가 사용자별 시간표(timezone-local)를 따라 5종 핵심 이벤트를 emit하고,
- **User pattern learner**가 최근 7~30일 활동을 분석하여 기상/식사/취침 시간을 자동 추정하며,
- **Backoff manager**가 사용자가 최근 N분 내 활발히 대화 중이면 ritual trigger를 지연·스킵하고,
- **Holiday calendar**가 한국 공휴일 + 사용자 로컬 공휴일을 인식하여 주말/명절 리추얼 강도를 조정하며,
- **Timezone detector**가 시스템 TZ 또는 사용자 명시 TZ를 따라 "아침 7시"를 로컬 기준으로 해석한다.

본 SPEC은 다마고치 Nurture Loop와도 연결된다 — **매일 리추얼 완수 1회 = Bond Level +N** (실제 적립은 RITUAL-001 책임, 본 SPEC은 trigger 생성까지만).

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- 사용자 지시(2026-04-22): "매일 사용자에게 아침마다 오늘의 운세와 날씨 정보, 하루 일정을 브리핑 해주고, 매 끼니 이후 건강/약 먹도록 안내, 저녁에 자기전 오늘 하루가 어땠는지 안부 묻고 일기 식으로 메모를 남기면..."
- 기존 Phase 0~5는 **reactive**(사용자가 물어야 응답). Phase 7의 본질은 **proactive**(GOOSE가 먼저 말 건넴). SCHEDULER-001이 없으면 Daily Rituals 자체가 성립 불가.
- HOOK-001은 시간-이벤트를 생성하지 않는다 (`Setup`/`SessionStart`/`PreToolUse` 등 24개는 모두 agent-driven). 시간축은 본 SPEC이 **새로운 이벤트 소스**로 추가한다.
- ROADMAP v2.0 §4 Phase 4~5 (INSIGHTS, MEMORY)가 "사용자 패턴을 학습"까지만 하고 "그 패턴에 따라 먼저 말 걸기"는 비어있었다. 본 SPEC이 그 공백을 메운다.

### 2.2 상속 자산

- **HOOK-001 lifecycle dispatcher**: `HookRegistry`, `DispatchXxx` 함수군. 본 SPEC은 **이 위에** 5개 새 이벤트(`MorningBriefingTime`, `PostBreakfastTime`, `PostLunchTime`, `PostDinnerTime`, `EveningCheckInTime`)를 등록.
- **INSIGHTS-001 ActivityPattern**: `ByHour[24]` 히스토그램에서 기상·식사·취침 시간 추정. 본 SPEC은 그 로직을 **온라인** 버전으로 변형(매일 증분 갱신).
- **MEMORY-001 `facts` 테이블**: 학습된 시간표를 `ritual_schedule` 네임스페이스로 영속.
- **robfig/cron/v3** v3.0+: cron 표현식 파서 + 스케줄러. 업계 표준, Go 생태계 사실상 유일 신뢰 가능 옵션.
- **rickar/cal/v2** v2.1+: 한국 포함 20+개국 공휴일 DB. 사용자가 로컬 공휴일 모듈 선택.

### 2.3 범위 경계 (한 줄)

- **IN**: RitualScheduler struct, 5개 이벤트 emit, 사용자 시간표 학습 (InsightsEngine 소비), Timezone detector, Holiday calendar, Backoff manager, HOOK-001 연동, MEMORY-001 영속.
- **OUT**: 각 리추얼 본체(BRIEFING/HEALTH/JOURNAL), TTS 음성(BRIEFING-001), Calendar 충돌 조회 실제 구현(CALENDAR-001의 read), Sleep 트래킹(외부 기기 연동은 별도 SPEC).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/ritual/scheduler/` 패키지.
2. `Scheduler` struct: start/stop lifecycle, cron 엔트리 관리, HOOK 디스패처 주입.
3. `RitualTime` 타입: `{EventName, LocalClock, TZ, WeekdayMask}` (cron spec 추상화).
4. 5개 신규 HookEvent 상수 등록 (HOOK-001 HookEvent enum 확장 요청 — 본 SPEC이 명시):
   - `EvMorningBriefingTime`, `EvPostBreakfastTime`, `EvPostLunchTime`, `EvPostDinnerTime`, `EvEveningCheckInTime`
5. `TimezoneDetector`: 시스템 TZ + 설정 override + 여행 감지(옵션).
6. `HolidayCalendar` 인터페이스 + `KoreanHolidayProvider` 기본 구현 (rickar/cal/v2 기반).
7. `BackoffManager`: 최근 N분 활동(QUERY-001 turn 카운터) 조회 후 ritual 지연·스킵 결정.
8. `PatternLearner`: INSIGHTS-001의 `ActivityPattern.ByHour`를 7/30일 rolling window로 분석하여 `RitualTime` 추천.
9. `SchedulerConfig` (config.yaml):
   - `scheduler.enabled: bool`
   - `scheduler.timezone: string` (IANA TZ, default=system)
   - `scheduler.rituals.morning.time: "07:30"` (override 가능, 비우면 learner 추론)
   - `scheduler.rituals.meals.breakfast/lunch/dinner.time`
   - `scheduler.rituals.evening.time`
   - `scheduler.backoff.active_window_min: 10`
   - `scheduler.holidays.provider: "korean"` (korean | us | japan | disabled)
10. `ScheduledEvent` 구조체: `{EventName, TriggerTime, LocalClock, TZ, IsHoliday, HolidayName?, BackoffApplied}` → HOOK dispatcher 에 payload로 전달.
11. 영속: `~/.goose/ritual/schedule.json` + MEMORY-001 `facts.ritual_schedule`.
12. Panic guard: cron 엔트리 실행 중 panic → recover + zap error, 전체 스케줄러 계속 동작.

### 3.2 OUT OF SCOPE

- **리추얼 본체 실행**: BRIEFING-001 / HEALTH-001 / JOURNAL-001 / RITUAL-001. 본 SPEC은 HOOK emit까지.
- **외부 알람 시스템 연동** (iOS notification, Android push): 별도 Gateway SPEC.
- **수면 트래킹 기기 연동** (Fitbit, Apple Health): 별도 SPEC.
- **미세한 event scheduling** (밀리초 단위): cron은 최소 1분 단위, 그 이하는 지원하지 않는다.
- **Cross-timezone 협업** (공동 작업 시 상대 TZ로 발송): A2A-001.
- **다중 사용자 분리 스케줄** (Family mode): adaptation.md §9.2 후속 SPEC.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-SCHED-001 [Ubiquitous]** — The `Scheduler` **shall** use IANA timezone identifiers exclusively; all stored `RitualTime` values **shall** carry an explicit `TZ` field and **shall not** rely on process-local timezone drift.

**REQ-SCHED-002 [Ubiquitous]** — The scheduler **shall** emit exactly 5 core ritual events: `MorningBriefingTime`, `PostBreakfastTime`, `PostLunchTime`, `PostDinnerTime`, `EveningCheckInTime`; custom user-defined events **shall** use a separate `CustomRitualTime` event channel (future extension, not in this SPEC).

**REQ-SCHED-003 [Ubiquitous]** — All scheduled triggers **shall** be persisted to `~/.goose/ritual/schedule.json` within 100ms of creation, and **shall** survive process restart.

**REQ-SCHED-004 [Ubiquitous]** — The scheduler **shall** emit structured zap logs `{event, scheduled_at, actual_at, tz, holiday, backoff_applied, skipped}` at INFO level for every trigger attempt.

### 4.2 Event-Driven (이벤트 기반)

**REQ-SCHED-005 [Event-Driven]** — **When** a cron entry fires for `MorningBriefingTime` at `RitualTime.LocalClock` in `RitualTime.TZ`, the scheduler **shall** (a) consult `BackoffManager.ShouldDefer()`, (b) if true, reschedule to +10min and log, (c) if false, construct `ScheduledEvent` and call `hook.DispatchMorningBriefingTime(ctx, event)`.

**REQ-SCHED-006 [Event-Driven]** — **When** `PatternLearner.Observe(activityPattern)` is called with a new day's ActivityPattern, the learner **shall** update rolling averages for 3 meal-peak hours and emit `RitualTimeProposal` if drift exceeds 30 minutes from current config.

**REQ-SCHED-007 [Event-Driven]** — **When** `HolidayCalendar.IsHoliday(date, locale)` returns `true`, the scheduler **shall** emit ritual events with `ScheduledEvent.IsHoliday=true` and `HolidayName=<resolved name>`; downstream consumers (BRIEFING/JOURNAL) may adjust tone (softer, longer greeting).

**REQ-SCHED-008 [Event-Driven]** — **When** `TimezoneDetector.Detect()` detects a travel-time change of ≥ 2 hours compared to the last 24h baseline, the scheduler **shall** log a WARN, pause ritual emission for 24h, and post a `TimezoneShiftDetected` notification via HOOK `EvNotification`.

**REQ-SCHED-009 [Event-Driven]** — **When** `Scheduler.Start(ctx)` is invoked, the scheduler **shall** (a) load persisted schedule, (b) register cron entries for all 5 events, (c) subscribe to QUERY-001 turn counter for backoff, (d) begin ticking; failure in any step **shall** return error, scheduler state **shall** remain `Stopped`.

### 4.3 State-Driven (상태 기반)

**REQ-SCHED-010 [State-Driven]** — **While** `config.scheduler.enabled == false`, the scheduler **shall** be inert — `Start()` returns nil without registering cron entries, `Stop()` is a no-op; no events are emitted regardless of wall clock.

**REQ-SCHED-011 [State-Driven]** — **While** the most recent QueryEngine turn occurred within `config.scheduler.backoff.active_window_min`, `BackoffManager.ShouldDefer()` **shall** return `true`; after the window passes (no activity for N minutes), subsequent triggers fire normally.

**REQ-SCHED-012 [State-Driven]** — **While** `config.scheduler.rituals.meals.breakfast.time == ""` (empty/unset), the scheduler **shall** use `PatternLearner.Predict(Breakfast)` result; if learner has fewer than 7 days of data, fall back to default `08:00` local.

### 4.4 Unwanted Behavior (방지)

**REQ-SCHED-013 [Unwanted]** — The scheduler **shall not** fire two triggers for the same `(event, calendar-date, timezone)` combination; duplicate suppression key **shall** be the 3-tuple `{event}:{userLocalDate}:{TZ}` (canonical: research.md §6.1) — if a process restart replays a trigger whose (event, userLocalDate, TZ) combination already fired, suppression deduplicates; TZ 변경 시에는 새 key가 되어 억제되지 않는다(travel-aware).

**REQ-SCHED-014 [Unwanted]** — The scheduler **shall not** emit ritual events during `[23:00, 06:00]` local time even if configured; this is a HARD quiet-hours floor to prevent sleep disruption, overrideable only via explicit `config.scheduler.allow_nighttime: true`.

**REQ-SCHED-015 [Unwanted]** — The scheduler **shall not** invoke HOOK dispatcher from within the cron goroutine directly; events **shall** be submitted to a buffered channel (default size 32) and dispatched by a separate worker to preserve cron scheduling precision.

**REQ-SCHED-016 [Unwanted]** — `PatternLearner` **shall not** change ritual times by more than ±2 hours per adjustment cycle; abrupt jumps likely indicate one-off events (jet lag, illness) and **shall** require ≥ 3 consecutive observations to commit.

### 4.5 Optional (선택적)

**REQ-SCHED-017 [Optional]** — **Where** `config.scheduler.holidays.provider == "korean"`, the scheduler **shall** recognize 한국 공휴일(설날 3일, 추석 3일, 어린이날, 석가탄신일, 현충일, 광복절, 개천절, 한글날, 크리스마스, 삼일절) + 대체공휴일; dates **shall** be loaded from `rickar/cal/v2/kr` module.

**REQ-SCHED-018 [Optional]** — **Where** user sets `config.scheduler.rituals.morning.skip_weekends: true`, `MorningBriefingTime` events on Saturday/Sunday **shall** be suppressed; holidays follow the same weekend rule unless `skip_holidays: false` is also set.

**REQ-SCHED-019 [Optional]** — **Where** `config.scheduler.pattern_learner.enabled == true` (default true), `PatternLearner` **shall** run daily at 03:00 local to ingest the previous day's activity pattern and propose updates; users receive an AskUserQuestion-style notification via HOOK `EvNotification` for confirmation before commit.

**REQ-SCHED-020 [Optional]** — **Where** `config.scheduler.debug.fast_forward == true` **AND** build tag `test_only` is active, the scheduler **shall** expose a `FastForward(duration)` API for testing; wall-clock is mocked forward by `duration` and all due events fire immediately in correct order. In production build (no `test_only` tag) the symbol **shall not** be linked into the binary.

### 4.6 Additional Requirements (v0.2.0 승격)

**REQ-SCHED-021 [Unwanted]** — The `BackoffManager` **shall not** defer the same ritual trigger more than `config.scheduler.backoff.max_defer_count` times (default 3); on the `max_defer_count + 1`-th cron tick while `ShouldDefer()` still returns true, the scheduler **shall** force-emit the ritual event with payload `DelayHint = N × active_window_min` and log a WARN-level entry including `force_emit:true`. (R4 Risk → 정식 REQ로 승격)

**REQ-SCHED-022 [Event-Driven]** — **When** `Scheduler.Start(ctx)` is invoked after a process downtime during which one or more ritual triggers were missed, the scheduler **shall** replay each missed trigger **exactly once** iff the gap between scheduled time and restart time is ≤ `config.scheduler.missed_event_replay_max_delay` (default 1h); when the gap exceeds this threshold, the scheduler **shall** skip replay and log INFO `{skipped:true, reason:"missed_event_too_stale"}`. Replayed events carry `ScheduledEvent.IsReplay=true` and `DelayMinutes=<actual>` so downstream consumers (BRIEFING-001 등) can adapt tone. (R7 Risk → 정식 REQ로 승격)

---

## 5. 수용 기준 (Acceptance Criteria)

### 5.0 AC Format Declaration

**Project Convention (MP-2 준수 선언):**

본 SPEC의 수용 기준(AC)은 **Given / When / Then 포맷(BDD)** 을 사용한다. 이는 본 프로젝트(goose-agent)의 고정 관례이며, EARS 5 패턴은 §4 **REQ 요구사항에서만** 사용된다. 각 AC는 다음 규칙을 따른다:

- **1:1 REQ 매핑**: 모든 AC 헤더에 `(REQ-SCHED-XXX[, YYY])` 형식으로 대응 REQ(s)를 명시한다. AC의 Given/When/Then 절은 해당 REQ의 EARS 조건을 검증 가능한 시나리오로 전개한 것이다.
- **이진 PASS/FAIL**: 모든 AC는 실행 결과가 PASS 또는 FAIL로 결정되어야 하며, 판단 애매성("or", "either", "optionally") 은 금지된다.
- **결정론적**: timing 의존 요소(`time.Sleep`, 실 wall-clock)는 mock clock(`clockwork.Clock`) 또는 동기적 대기(event-driven `<-done`)로 대체한다. 남아있는 "1초 대기" 등은 구현 시 동기 신호로 치환된다.

총 20개 AC(AC-SCHED-001 ~ AC-SCHED-020)는 REQ-SCHED-001 ~ REQ-SCHED-022 중 22개 REQ와 1:N 매핑된다(일부 AC는 2개 REQ를 커버).

---

**AC-SCHED-001 — 5개 이벤트 상수 등록 (REQ-SCHED-001, REQ-SCHED-002)**
- **Given** `internal/ritual/scheduler` 패키지
- **When** `scheduler.RegisteredEvents()` 호출
- **Then** 정확히 5개 문자열이 반환됨: `MorningBriefingTime, PostBreakfastTime, PostLunchTime, PostDinnerTime, EveningCheckInTime`. HOOK-001의 `HookEventNames()`에도 이 5개가 추가되어 총 29개 (AC-HK-001 확장, HOOK-001 동기화 전제).

**AC-SCHED-002 — cron 기반 emit + TZ 존중 (REQ-SCHED-005)**
- **Given** `SchedulerConfig{TZ:"Asia/Seoul", Rituals.Morning.Time:"07:30"}`, mock `clockwork.Clock`, mock `HookDispatcher`
- **When** mock clock을 Asia/Seoul 07:30:00에 도달시키고 dispatcher의 done 채널을 동기적으로 수신
- **Then** `HookDispatcher.DispatchMorningBriefingTime` 이 정확히 1회 호출됨. 동일 시각 UTC 22:30으로 mock한 별도 시나리오에서는 호출 0회.

**AC-SCHED-003 — backoff 지연 (REQ-SCHED-005, REQ-SCHED-011)**
- **Given** `BackoffManager.ActiveWindowMin=10`, mock `QueryEngine.LastTurnAt = clock.Now() - 5min`
- **When** `MorningBriefingTime` cron entry가 발화
- **Then** 즉시 dispatch 0회, 10분 후 재시도 스케줄 등록됨(cron entry count +1 확인), zap INFO 로그에 `{event:"MorningBriefingTime", backoff_applied:true, deferred_to:"+10m"}` 기록.

**AC-SCHED-004 — 한국 공휴일 인식 (REQ-SCHED-007, REQ-SCHED-017)**
- **Given** `HolidayCalendar.Provider=korean`, 날짜 2026-10-03 (개천절)
- **When** `HolidayCalendar.IsHoliday(2026-10-03, "ko-KR")` 호출
- **Then** `(true, "개천절")` 반환, `ScheduledEvent.IsHoliday=true, HolidayName="개천절"` 로 dispatch. 추가로 2026-09-28(추석 대체공휴일 시나리오 fixture)에 대해 `(true, "추석 대체공휴일")` 반환으로 대체공휴일 규칙도 검증.

**AC-SCHED-005 — quiet hours 차단 (REQ-SCHED-014)**
- **Given** `config.scheduler.rituals.morning.time="02:30"` (quiet-hours 범위 [23:00, 06:00] 내), `config.scheduler.allow_nighttime=false`
- **When** `Scheduler.Start(ctx)` 호출
- **Then** `Start` **는 `ErrQuietHoursViolation` error를 반환**하고, scheduler state는 `Stopped`로 남으며, cron 엔트리는 등록되지 않고, zap ERROR 로그에 `{event:"morning", configured_time:"02:30", reason:"quiet_hours_violation"}` 가 기록된다. (단일 결정론적 경로 — clamp 정책은 제거됨.)

**AC-SCHED-006 — PatternLearner 자동 시간 학습 (REQ-SCHED-006, REQ-SCHED-012)**
- **Given** 7일간 사용자가 매일 08:15 경(±15분) 첫 activity를 기록 (INSIGHTS-001 ActivityPattern mock, `config.scheduler.rituals.meals.breakfast.time=""` unset)
- **When** `PatternLearner.Predict(Breakfast)` 호출
- **Then** 반환 `LocalClock` ∈ [08:00, 08:30], `confidence` ≥ 0.7. 별도 시나리오에서 6일치 data만 주어질 때는 fallback default `08:00`이 반환됨(REQ-SCHED-012).

**AC-SCHED-007 — 시간표 영속성 (REQ-SCHED-003)**
- **Given** 스케줄러가 학습된 시간표 `{morning:07:30, lunch:12:15, dinner:19:00}` 를 가진 상태
- **When** `Scheduler.Save()` 호출 후 프로세스 종료 → 재시작하여 `Scheduler.Load()`
- **Then** (a) `~/.goose/ritual/schedule.json` 에 3 엔트리 존재 (저장 완료 후 100ms 이내 파일 stat 확인), (b) MEMORY-001 `facts` 테이블에도 `ritual_schedule` 네임스페이스로 동일 3 엔트리 존재, (c) 재시작 후 load된 RitualTime 3개의 `{EventName, LocalClock, TZ}` 가 저장 시와 완전 일치.

**AC-SCHED-008 — duplicate 억제 (restart 재발화 방지, TZ-aware) (REQ-SCHED-013)**
- **Given** 오늘(2026-04-25) 07:30 KST에 `MorningBriefingTime` 이미 dispatch됨 → MEMORY-001 `ritual_fired`에 key `"MorningBriefingTime:2026-04-25:Asia/Seoul"` 저장. 07:45에 프로세스 재시작.
- **When** 재시작 후 `Scheduler.Start()` → cron이 07:30 missed-event를 replay 시도
- **Then** (a) 동일 key(3-tuple) 매칭으로 07:30 재발화 0회, (b) 내일(2026-04-26) 07:30은 새 key `"MorningBriefingTime:2026-04-26:Asia/Seoul"` 로 정상 예약, (c) 별도 시나리오 — 같은 07:45에 TZ를 `Asia/Tokyo`로 변경 후 restart하면 key `"MorningBriefingTime:2026-04-25:Asia/Tokyo"` 는 기록에 없으므로 새 key로 취급되어 R7 missed-event replay 정책(REQ-SCHED-022) 적용됨.

**AC-SCHED-009 — Timezone shift 감지 (REQ-SCHED-008)**
- **Given** 사용자가 서울(KST, UTC+9)에서 뉴욕(EST, UTC-5)으로 이동, mock `TimezoneDetector.Detect()` 가 TZ 변경 감지 (Δ=14h, cap 적용 후 effective shift ≥2h)
- **When** 다음 detection tick에서 `Detect()` 반환값이 baseline과 다름
- **Then** (a) 이후 24시간(mock clock) 동안 ritual emit 0회, (b) `TimezoneShiftDetected` notification 이 HOOK `EvNotification`로 정확히 1회 발생, (c) 24시간 경과 후 다음 cron tick에서 새 TZ(EST) 기준으로 ritual 재개.

---

### 5.1 추가 수용 기준 (REQ-SCHED-004, 009, 010, 013~020 커버리지)

**AC-SCHED-010 — 로그 스키마 검증 (REQ-SCHED-004)**
- **Given** `Scheduler` running, mock `HookDispatcher`, zap logger with in-memory sink
- **When** 임의의 5개 ritual 중 하나가 트리거되어 dispatch되거나 backoff defer 또는 skip됨
- **Then** zap sink에 기록된 로그 엔트리는 INFO level이며, 필드 집합이 정확히 `{event, scheduled_at, actual_at, tz, holiday, backoff_applied, skipped}` 7개를 포함해야 한다. 누락 필드가 있으면 FAIL.

**AC-SCHED-011 — Start 부분 실패 시 Stopped 불변 (REQ-SCHED-009)**
- **Given** `SchedulerConfig`의 TZ가 잘못된 값 `"Invalid/Zone"` 또는 persist load 실패 주입
- **When** `Scheduler.Start(ctx)` 호출
- **Then** (a) `Start` 가 non-nil error 반환, (b) 반환 후 `Scheduler.State()` == `Stopped`, (c) `s.cron.Entries()` 는 빈 슬라이스 (등록된 cron entry 없음), (d) 이후 `Stop()` 호출은 no-op로 nil error.

**AC-SCHED-012 — enabled=false 불활성 (REQ-SCHED-010)**
- **Given** `config.scheduler.enabled=false`, 현재 mock clock이 07:30 Asia/Seoul
- **When** `Scheduler.Start(ctx)` 호출 후 mock clock을 24시간 전진
- **Then** (a) `Start` 는 nil error 반환, (b) `cron.Entries()` 가 empty, (c) 24시간 동안 `HookDispatcher` 호출 수 0, (d) `Stop()` 호출은 no-op.

**AC-SCHED-013 — quiet hours override (REQ-SCHED-014)**
- **Given** `config.scheduler.rituals.morning.time="02:30"`, `config.scheduler.allow_nighttime=true`
- **When** `Scheduler.Start(ctx)` 호출 후 mock clock을 02:30 도달시킴
- **Then** (a) `Start` nil error, (b) 02:30에 `DispatchMorningBriefingTime` 정확히 1회 호출, (c) zap WARN 로그에 `{event:"morning", nighttime_override:true}` 1회 기록. (AC-SCHED-005의 반대 케이스: override 시 발화 허용)

**AC-SCHED-014 — cron-dispatcher 디커플링 (REQ-SCHED-015)**
- **Given** mock `HookDispatcher.DispatchMorningBriefingTime` 이 2초간 block (slow consumer 시뮬레이션), cron 엔트리 다수 등록
- **When** 3개 이벤트가 1초 간격으로 연속 발화
- **Then** (a) cron goroutine은 3회 모두 즉시 반환(각 fire 소요 시간 < 10ms), (b) `eventCh`(버퍼 32) 에 이벤트 3개가 enqueue됨, (c) dispatcher worker가 순차 처리하여 6초 후 3회 dispatch 완료. cron 틱 정밀도는 dispatcher 지연에 영향받지 않음.

**AC-SCHED-015 — PatternLearner ±2h cap 및 3회 확정 (REQ-SCHED-016)**
- **Given** 현재 commit된 breakfast 시간 08:00, 1일치 activity peak가 11:30 (Δ=+3h30m, cap 초과), 7일 rolling window
- **When** `PatternLearner.Observe(day1)` 호출
- **Then** (a) 단일 1일 관측으로는 commit 되지 않음(proposal만 기록), (b) 연속 3일 동일 패턴(11:30 ±10min) 관측해도 `|Δ|=3h30m > 2h cap` 이므로 1 cycle 최대 +2h만 적용되어 10:00으로 이동 제안, (c) `RitualTimeProposal` payload 의 `NewLocalClock` 필드가 `"10:00"` 임을 검증.

**AC-SCHED-016 — 주말 스킵 (REQ-SCHED-018)**
- **Given** `config.scheduler.rituals.morning.skip_weekends=true`, mock clock = 2026-04-25 07:30 (토요일) 및 2026-04-27 07:30 (월요일)
- **When** 각각 cron 발화
- **Then** (a) 토요일(2026-04-25)에는 `DispatchMorningBriefingTime` 호출 0회, zap INFO `{event:"morning", skipped:true, reason:"weekend"}`, (b) 월요일(2026-04-27)에는 정확히 1회 호출.

**AC-SCHED-017 — 03:00 PatternLearner 실행 + 확인 Flow (REQ-SCHED-019)**
- **Given** `config.scheduler.pattern_learner.enabled=true`, 전일 ActivityPattern이 MEMORY-001에 존재, 제안 변화량 > 30분 (REQ-SCHED-006 threshold)
- **When** mock clock을 03:00 local 도달시킴
- **Then** (a) `PatternLearner.Observe(prevDay)` 정확히 1회 호출, (b) HOOK `EvNotification` 1회 발생하며 payload는 `{kind:"RitualTimeProposal", confirm_required:true}`, (c) 사용자 확인 전에는 config에 commit 되지 않음(`config.scheduler.rituals.morning.time` 불변).

**AC-SCHED-018 — FastForward gating (REQ-SCHED-020)**
- **Given** production binary 빌드 (`go build` 기본 build tag)
- **When** Scheduler 인스턴스에서 `FastForward(d)` 호출을 시도
- **Then** (a) production build 에서는 `FastForward` 심볼이 존재하지 않아 컴파일 에러 또는 `ErrFastForwardNotAvailable` panic; (b) test build (`go test` 시 `-tags=test_only`) 에서만 `FastForward(d)` 가 동작하여 clock을 `d` 만큼 전진시키고 대기 중인 모든 trigger를 순서대로 emit. Build tag `//go:build test_only` 로 파일 단위 gating된 심볼을 확인한다.

**AC-SCHED-019 — 최대 3회 defer 후 강제 emit (REQ-SCHED-021)**
- **Given** `BackoffManager.ActiveWindowMin=10`, `config.scheduler.backoff.max_defer_count=3`, mock `QueryEngine.LastTurnAt` 가 지속적으로 `clock.Now() - 1min` (즉 항상 active)
- **When** 07:30 `MorningBriefingTime` cron 발화 → defer → +10min 후 재발화 → defer → ... 반복
- **Then** (a) 1회차(07:30), 2회차(07:40), 3회차(07:50) 모두 defer, (b) 4회차(08:00)에서는 active 상태여도 **강제 emit**, `DispatchMorningBriefingTime` 정확히 1회 호출, (c) zap WARN 로그 `{event:"morning", force_emit:true, defer_count:3}` 1회 기록, (d) BRIEFING-001 payload 의 `DelayHint` 필드가 `"30m"` 로 전달됨.

**AC-SCHED-020 — Process restart 시 missed event replay (REQ-SCHED-022)**
- **Given** 시나리오 A: 07:30 KST dispatch 예정이었으나 프로세스가 07:00 ~ 08:00 downtime. 재시작 시각 08:00 (지체 30분). 시나리오 B: 동일하나 재시작 시각 09:00 (지체 1h30m).
- **When** 각 시나리오에서 `Scheduler.Start(ctx)` 호출
- **Then** (a) 시나리오 A: 08:00 기준으로 `DispatchMorningBriefingTime` 정확히 1회 replay 발생, payload 에 `IsReplay:true, DelayMinutes:30` 포함, SuppressionKey `"MorningBriefingTime:2026-04-25:Asia/Seoul"` 저장됨. (b) 시나리오 B: `Dispatch*` 호출 0회 (1h 초과 → 스킵), zap INFO `{event:"morning", skipped:true, reason:"missed_event_too_stale", delay_min:90}` 1회 기록.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/
└── ritual/
    └── scheduler/
        ├── scheduler.go          # Scheduler struct + Start/Stop
        ├── cron.go               # robfig/cron 래핑
        ├── events.go             # 5개 EventName 상수 + ScheduledEvent
        ├── timezone.go           # TimezoneDetector
        ├── holiday.go            # HolidayCalendar + KoreanProvider
        ├── backoff.go            # BackoffManager
        ├── pattern.go            # PatternLearner (INSIGHTS 소비)
        ├── persist.go            # schedule.json + MEMORY-001 연동
        ├── config.go             # SchedulerConfig
        └── *_test.go
```

### 6.2 핵심 Go 타입

```go
// 5 신규 HookEvent (HOOK-001 에 등록 요청)
const (
    EvMorningBriefingTime HookEvent = "MorningBriefingTime"
    EvPostBreakfastTime   HookEvent = "PostBreakfastTime"
    EvPostLunchTime       HookEvent = "PostLunchTime"
    EvPostDinnerTime      HookEvent = "PostDinnerTime"
    EvEveningCheckInTime  HookEvent = "EveningCheckInTime"
)

type RitualTime struct {
    EventName   string        // "MorningBriefingTime"
    LocalClock  string        // "07:30"
    TZ          string        // IANA, "Asia/Seoul"
    WeekdayMask uint8         // bitmask: 0b1111111 (Mon..Sun)
    Source      TimeSource    // UserConfig | Learned | Default
    Confidence  float64       // [0,1], Learned일 때만 유효
}

type ScheduledEvent struct {
    EventName       string
    TriggerTime     time.Time  // UTC
    LocalClock      string
    TZ              string
    IsHoliday       bool
    HolidayName     string
    BackoffApplied  bool
    SuppressionKey  string     // {event}:{yyyy-mm-dd-local}:{TZ}  (3-tuple per REQ-SCHED-013, research.md §6.1)
}

type Scheduler struct {
    cfg         SchedulerConfig
    cron        *cron.Cron           // robfig/cron/v3
    hookMgr     hook.Dispatcher      // HOOK-001
    tzDetector  *TimezoneDetector
    holiday     HolidayCalendar
    backoff     *BackoffManager
    learner     *PatternLearner
    persist     *SchedulePersister
    eventCh     chan ScheduledEvent  // buffered 32
    dispatchWG  sync.WaitGroup
    state       atomic.Int32         // Stopped | Running
    logger      *zap.Logger
}

func New(cfg SchedulerConfig, deps Dependencies) (*Scheduler, error)
func (s *Scheduler) Start(ctx context.Context) error
func (s *Scheduler) Stop(ctx context.Context) error

// FastForward는 별도 파일 scheduler_test_only.go 에 정의되며, 빌드 태그 `//go:build test_only` 로 gating된다.
// Production build (기본)에서는 이 심볼이 링크되지 않는다 (REQ-SCHED-020, AC-SCHED-018).
//
//   //go:build test_only
//   package scheduler
//   func (s *Scheduler) FastForward(d time.Duration) { s.clock.Advance(d) }

// RitualTimeProposal은 PatternLearner가 생성하는 time-change proposal 이다 (REQ-SCHED-006, REQ-SCHED-016).
// HOOK-001의 EvNotification payload로 전달되며, 사용자 확인 전에는 config에 commit되지 않는다.
type RitualTimeProposal struct {
    Kind            RitualKind  // Morning | Breakfast | Lunch | Dinner | Evening
    OldLocalClock   string      // e.g. "08:00"
    NewLocalClock   string      // e.g. "10:00" (±2h cap 적용 후)
    DriftMinutes    int         // 관측된 총 drift (cap 적용 전)
    SupportingDays  int         // 연속 관측 일수 (commit은 ≥ 3일 필요)
    Confidence      float64     // [0.0, 1.0], research.md §3.3 Bayesian 공식
    ConfirmRequired bool        // REQ-SCHED-019에 의해 항상 true
}

type HolidayCalendar interface {
    IsHoliday(date time.Time, locale string) (bool, string)
    UpcomingHolidays(from time.Time, days int) []Holiday
}

type BackoffManager struct {
    queryEngine query.TurnCounter    // QUERY-001
    activeWindow time.Duration
}
func (b *BackoffManager) ShouldDefer() bool

type PatternLearner struct {
    insights insights.Reader         // INSIGHTS-001
    rollingWindow int                // days, default 7
}
func (p *PatternLearner) Predict(kind RitualKind) (LocalClock string, confidence float64, err error)
func (p *PatternLearner) Observe(pattern insights.ActivityPattern) error
```

### 6.3 Cron entry 생성

```go
// 예: "07:30 Asia/Seoul" → cron 스펙 "30 7 * * *" + cron.WithLocation(tz)
func (s *Scheduler) registerEntry(rt RitualTime) error {
    loc, err := time.LoadLocation(rt.TZ)
    if err != nil { return err }

    // HH:MM → cron "MM HH * * *"
    h, m, err := parseClock(rt.LocalClock)
    if err != nil { return err }

    spec := fmt.Sprintf("%d %d * * *", m, h)
    _, err = s.cron.AddFunc(spec, func() {
        s.fireEvent(rt)
    })
    return err
}
```

### 6.4 Holiday Provider

`rickar/cal/v2/kr` 모듈 사용. 주요 한국 공휴일:
- 신정 (1/1), 설날 (음력 1/1 ± 1), 삼일절 (3/1), 어린이날 (5/5), 부처님 오신 날 (음력 4/8), 현충일 (6/6), 광복절 (8/15), 추석 (음력 8/15 ± 1), 개천절 (10/3), 한글날 (10/9), 크리스마스 (12/25).
- 대체공휴일: 어린이날·추석·설날이 주말 겹치면 다음 평일.

### 6.5 TDD 진입 순서

1. RED: `TestRegisteredEvents_Exactly5` — AC-SCHED-001.
2. RED: `TestCronEmitsInCorrectTZ` — AC-SCHED-002.
3. RED: `TestBackoffDefers10Min` — AC-SCHED-003.
4. RED: `TestKoreanHoliday_October3_And_SubstituteHoliday` — AC-SCHED-004.
5. RED: `TestQuietHoursRejectedDeterministic` — AC-SCHED-005.
6. RED: `TestPatternLearner_7DayConvergence` — AC-SCHED-006.
7. RED: `TestPersistAndReload` — AC-SCHED-007.
8. RED: `TestDuplicateSuppression_3Tuple_TZAware` — AC-SCHED-008.
9. RED: `TestTimezoneShift_24hPause` — AC-SCHED-009.
10. RED: `TestLogSchema_Exactly7Fields` — AC-SCHED-010.
11. RED: `TestStartPartialFailure_StoppedInvariant` — AC-SCHED-011.
12. RED: `TestDisabled_Inert` — AC-SCHED-012.
13. RED: `TestQuietHoursOverride_AllowNighttime` — AC-SCHED-013.
14. RED: `TestCronDispatcherDecoupling_BufferedChannel` — AC-SCHED-014.
15. RED: `TestPatternLearner_2hCap_3DayCommit` — AC-SCHED-015.
16. RED: `TestSkipWeekends` — AC-SCHED-016.
17. RED: `TestDailyLearnerRun_0300_Confirmation` — AC-SCHED-017.
18. RED: `TestFastForward_BuildTagGating` — AC-SCHED-018 (test_only build tag 활성 시에만).
19. RED: `TestMaxDeferCount_3_ForceEmit` — AC-SCHED-019.
20. RED: `TestMissedEventReplay_1hThreshold` — AC-SCHED-020.
21. GREEN → REFACTOR.

### 6.6 라이브러리 결정

| 용도 | 라이브러리 | 결정 근거 |
|------|----------|---------|
| Cron scheduler | `github.com/robfig/cron/v3` v3.0.1+ | Go 생태계 사실상 유일 성숙 옵션, location-aware |
| 한국·글로벌 공휴일 | `github.com/rickar/cal/v2` v2.1.13+ | 20+개국 지원, 한국 음력 공휴일 포함 |
| 타임존 | stdlib `time` + `time/tzdata` | tzdata 임베드로 크로스플랫폼 |
| 로깅 | `go.uber.org/zap` | CORE-001 계승 |
| Mock clock (test-only) | `github.com/jonboulle/clockwork` v0.4+ | `time.Now()` DI로 결정론적 테스트, research.md §7.1 채택 |

### 6.7 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| **T**ested | 85%+, AC 9종 전부 테스트, mock clock + mock hook dispatcher |
| **R**eadable | scheduler/cron/timezone/holiday/backoff/pattern 파일 분리 |
| **U**nified | golangci-lint, 모든 시간 비교 UTC 통일 + TZ 메타데이터 |
| **S**ecured | quiet hours HARD floor, panic recovery, 프로세스 재시작 시 중복 발화 차단 |
| **T**rackable | 모든 trigger zap 로그 (scheduled vs actual, backoff, holiday, skipped) |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | HOOK-001 | HookEvent enum 확장 + DispatchXxx 계약, 본 SPEC은 5 이벤트 추가 |
| 선행 SPEC | INSIGHTS-001 | ActivityPattern.ByHour 소비 (PatternLearner) |
| 선행 SPEC | MEMORY-001 | `facts` 테이블에 학습된 schedule 영속 |
| 선행 SPEC | CORE-001 | zap, context, graceful shutdown |
| 선행 SPEC | CONFIG-001 | scheduler.yaml 로드 |
| 선행 SPEC | QUERY-001 | TurnCounter (BackoffManager) |
| 후속 SPEC | BRIEFING-001 | MorningBriefingTime 소비 |
| 후속 SPEC | HEALTH-001 | PostMealTime 소비 |
| 후속 SPEC | JOURNAL-001 | EveningCheckInTime 소비 |
| 후속 SPEC | RITUAL-001 | 리추얼 완수 → Bond Level 증가 |
| 외부 | `robfig/cron/v3` | Cron parser |
| 외부 | `rickar/cal/v2` | 공휴일 DB |
| 외부 | `jonboulle/clockwork` | Mock clock (test-only build tag) |
| 외부 | Go 1.22+ | generics, atomic.Int32 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Cron이 시스템 suspend/resume 직후 정확하게 동작하지 않음 | 중 | 중 | `PersistedLastFire` 체크 + missed event replay (1시간 이상 지체 시 스킵, 그 이하는 즉시 1회 발화) |
| R2 | `rickar/cal/v2` 음력 공휴일 정확도 (설날/추석) | 낮 | 고 | 공휴일 제공자 interface로 추상화, custom override 경로 제공, 매년 CI에서 향후 3년 날짜 goldenfile 검증 |
| R3 | PatternLearner가 비정기 활동(주말 늦잠)을 오학습 | 중 | 중 | Weekday mask 분리(평일/주말), ±2시간 단일 조정 cap (REQ-SCHED-016) |
| R4 | Backoff window가 장시간 대화 세션 중 ritual을 완전 차단 | 중 | 중 | Backoff는 **defer**만, 스킵 아님. 최대 3회 defer 후 강제 emit → REQ-SCHED-021 정식 승격 (v0.2.0) |
| R5 | Quiet hours가 야간 근무자(개발자 등)에게 과잉 차단 | 중 | 중 | `allow_nighttime: true` override + 사용자 첫 활동 시간 학습으로 자동 완화 |
| R6 | 중복 방지 key가 TZ 변경 시 잘못된 비교 | 중 | 고 | SuppressionKey는 `{event}:{userLocalDate}:{TZ}` 3-tuple (REQ-SCHED-013, research.md §6.1 canonical), TZ 변경 시 새 key 사용 |
| R7 | Process downtime 중 리추얼 놓침 | 중 | 낮 | Missed event 정책: 1시간 이하 지체 → 즉시 1회 + `IsReplay=true` 플래그; 이상 → 스킵 → REQ-SCHED-022 정식 승격 (v0.2.0) |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/specs/ROADMAP.md` — Phase 7 신규 #31 (v3.0 로드맵)
- `.moai/specs/SPEC-GOOSE-HOOK-001/spec.md` — HookEvent enum, DispatchXxx 계약
- `.moai/specs/SPEC-GOOSE-INSIGHTS-001/spec.md` — ActivityPattern.ByHour 공급자
- `.moai/specs/SPEC-GOOSE-MEMORY-001/spec.md` — 학습 시간표 영속소
- `.moai/project/adaptation.md` §6 Time-based Adaptation (6-10시 아침 등 시간대별 모드 정의)

### 9.2 외부 참조

- robfig/cron/v3: https://github.com/robfig/cron
- rickar/cal/v2: https://github.com/rickar/cal
- IANA Time Zone Database: https://www.iana.org/time-zones
- 한국 공휴일 법률 (관공서의 공휴일에 관한 규정): https://www.law.go.kr/

### 9.3 부속 문서

- `./research.md` — PatternLearner 알고리즘 상세, holiday provider 선정 근거, backoff heuristic 튜닝
- `../SPEC-GOOSE-BRIEFING-001/spec.md` — MorningBriefingTime 소비자
- `../SPEC-GOOSE-HEALTH-001/spec.md` — PostMealTime 소비자
- `../SPEC-GOOSE-JOURNAL-001/spec.md` — EveningCheckInTime 소비자
- `../SPEC-GOOSE-RITUAL-001/spec.md` — 리추얼 완수 집계

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **리추얼 본체를 실행하지 않는다**. HOOK emit 까지만; 실제 아침 브리핑 UI/음성 생성은 BRIEFING-001, 식사 체크인은 HEALTH-001, 저녁 일기는 JOURNAL-001.
- 본 SPEC은 **외부 푸시 알림 발송을 포함하지 않는다** (APNS, FCM). Gateway SPEC의 책임.
- 본 SPEC은 **iOS/Android 백그라운드 실행을 구현하지 않는다**. goosed 데몬이 foreground process로 전제.
- 본 SPEC은 **수면 데이터 수집을 포함하지 않는다** (외부 wearable 연동). 별도 SPEC.
- 본 SPEC은 **가족 모드의 사용자별 분리 스케줄을 지원하지 않는다**. adaptation.md §9.2 후속.
- 본 SPEC은 **1분 미만의 정밀 스케줄링을 지원하지 않는다**. cron은 분 단위.
- 본 SPEC은 **custom user-defined events를 처리하지 않는다** (5 고정 이벤트만). 후속 SCHEDULER-002에서 확장.
- 본 SPEC은 **리추얼 완수 여부 추적을 포함하지 않는다**. Trigger dispatch까지가 책임, 사용자가 실제 응답했는지는 RITUAL-001이 측정.
- 본 SPEC은 **외부 cron daemon 연동을 포함하지 않는다** (systemd timer, launchd). 전부 in-process cron.

---

**End of SPEC-GOOSE-SCHEDULER-001**
