---
id: SPEC-GOOSE-SCHEDULER-001
version: 0.1.0
status: Planned
created: 2026-04-22
updated: 2026-04-22
author: manager-spec
priority: P0
issue_number: null
phase: 7
size: 중(M)
lifecycle: spec-anchored
---

# SPEC-GOOSE-SCHEDULER-001 — Proactive Ritual Scheduler (Cron-like, Timezone/Holiday-aware, User-pattern Learning)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | 초안 작성 (Phase 7 Daily Companion #31, HOOK-001 확장) | manager-spec |

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

**REQ-SCHED-013 [Unwanted]** — The scheduler **shall not** fire two triggers for the same `(event, calendar-date)` combination; duplicate suppression key is `{event, userLocalDate}` — if process restart replays a trigger whose userLocalDate already fired, suppression deduplicates.

**REQ-SCHED-014 [Unwanted]** — The scheduler **shall not** emit ritual events during `[23:00, 06:00]` local time even if configured; this is a HARD quiet-hours floor to prevent sleep disruption, overrideable only via explicit `config.scheduler.allow_nighttime: true`.

**REQ-SCHED-015 [Unwanted]** — The scheduler **shall not** invoke HOOK dispatcher from within the cron goroutine directly; events **shall** be submitted to a buffered channel (default size 32) and dispatched by a separate worker to preserve cron scheduling precision.

**REQ-SCHED-016 [Unwanted]** — `PatternLearner` **shall not** change ritual times by more than ±2 hours per adjustment cycle; abrupt jumps likely indicate one-off events (jet lag, illness) and **shall** require ≥ 3 consecutive observations to commit.

### 4.5 Optional (선택적)

**REQ-SCHED-017 [Optional]** — **Where** `config.scheduler.holidays.provider == "korean"`, the scheduler **shall** recognize 한국 공휴일(설날 3일, 추석 3일, 어린이날, 석가탄신일, 현충일, 광복절, 개천절, 한글날, 크리스마스, 삼일절) + 대체공휴일; dates **shall** be loaded from `rickar/cal/v2/kr` module.

**REQ-SCHED-018 [Optional]** — **Where** user sets `config.scheduler.rituals.morning.skip_weekends: true`, `MorningBriefingTime` events on Saturday/Sunday **shall** be suppressed; holidays follow the same weekend rule unless `skip_holidays: false` is also set.

**REQ-SCHED-019 [Optional]** — **Where** `config.scheduler.pattern_learner.enabled == true` (default true), `PatternLearner` **shall** run daily at 03:00 local to ingest the previous day's activity pattern and propose updates; users receive an AskUserQuestion-style notification via HOOK `EvNotification` for confirmation before commit.

**REQ-SCHED-020 [Optional]** — **Where** `config.scheduler.debug.fast_forward == true`, the scheduler **shall** accept a `FastForward(duration)` API for testing; wall-clock is mocked forward by `duration` and all due events fire immediately in correct order.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-SCHED-001 — 5개 이벤트 상수 등록**
- **Given** `internal/ritual/scheduler` 패키지
- **When** `scheduler.RegisteredEvents()` 호출
- **Then** 정확히 5개 문자열이 반환됨: `MorningBriefingTime, PostBreakfastTime, PostLunchTime, PostDinnerTime, EveningCheckInTime`. HOOK-001의 `HookEventNames()`에도 이 5개가 추가되어 총 29개 (AC-HK-001 확장).

**AC-SCHED-002 — cron 기반 emit + TZ 존중**
- **Given** `SchedulerConfig{TZ:"Asia/Seoul", Rituals.Morning.Time:"07:30"}`, mock `time.Now()` 제어, mock `HookDispatcher`
- **When** mock clock을 Asia/Seoul 07:30에 도달시키고 1초 대기
- **Then** `HookDispatcher.DispatchMorningBriefingTime` 이 정확히 1회 호출, 다른 TZ(UTC 22:30 등) 에는 호출 0회.

**AC-SCHED-003 — backoff 지연**
- **Given** `BackoffManager.ActiveWindowMin=10`, mock `QueryEngine.LastTurnAt = time.Now() - 5min`
- **When** `MorningBriefingTime` cron entry가 발화
- **Then** 즉시 dispatch 0회, 10분 후 재시도 스케줄 등록됨, zap INFO 로그에 `backoff_applied=true`.

**AC-SCHED-004 — 한국 공휴일 인식**
- **Given** `HolidayCalendar.Provider=korean`, 날짜 2026-10-03 (개천절)
- **When** `HolidayCalendar.IsHoliday(2026-10-03, "ko-KR")` 호출
- **Then** `(true, "개천절")` 반환, `ScheduledEvent.IsHoliday=true, HolidayName="개천절"` 로 dispatch.

**AC-SCHED-005 — quiet hours 차단**
- **Given** `config.scheduler.rituals.morning.time="02:30"` (유효하지 않은 시간), `allow_nighttime=false`
- **When** `Scheduler.Start(ctx)`
- **Then** 반환 error가 `ErrQuietHoursViolation`, 또는 경고 후 07:00으로 clamp (정책은 구현에서 결정, 테스트는 **둘 중 하나** 보장).

**AC-SCHED-006 — PatternLearner 자동 시간 학습**
- **Given** 7일간 사용자가 매일 08:15 경(±15분) 첫 activity를 기록 (INSIGHTS-001 ActivityPattern mock)
- **When** `PatternLearner.Predict(Breakfast)` 호출
- **Then** 반환 시간 ∈ [08:00, 08:30], 신뢰도 ≥ 0.7.

**AC-SCHED-007 — 시간표 영속성**
- **Given** 스케줄러가 학습된 시간표 `{morning:07:30, lunch:12:15, dinner:19:00}` 를 가진 상태
- **When** 프로세스 종료 → 재시작
- **Then** `Scheduler.Load()` 이후 동일 3 엔트리가 복원, MEMORY-001 `facts` 테이블에도 `ritual_schedule` 네임스페이스로 존재.

**AC-SCHED-008 — duplicate 억제 (restart 재발화 방지)**
- **Given** 오늘 07:30 `MorningBriefingTime` 이미 dispatch됨, 07:45 프로세스 재시작
- **When** 재시작 후 `Scheduler.Start()`
- **Then** 07:30 이벤트 재발화 0회 (userLocalDate 중복 key로 suppress), 내일 07:30은 정상 예약됨.

**AC-SCHED-009 — Timezone shift 감지**
- **Given** 사용자가 서울(KST, UTC+9)에서 뉴욕(EST, UTC-5)으로 이동, mock `TimezoneDetector.Detect()` 가 TZ 변경 감지
- **When** 다음 detection tick
- **Then** 24시간 동안 ritual emit 0회, `TimezoneShiftDetected` notification 1회 발생, 24시간 후 새 TZ로 리추얼 재개.

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
    SuppressionKey  string     // {event}:{yyyy-mm-dd-local}
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
func (s *Scheduler) FastForward(d time.Duration)  // test-only (REQ-SCHED-020)

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
4. RED: `TestKoreanHoliday_October3` — AC-SCHED-004.
5. RED: `TestQuietHoursRejected` — AC-SCHED-005.
6. RED: `TestPatternLearner_7DayConvergence` — AC-SCHED-006.
7. RED: `TestPersistAndReload` — AC-SCHED-007.
8. RED: `TestDuplicateSuppression_SameDay` — AC-SCHED-008.
9. RED: `TestTimezoneShift_24hPause` — AC-SCHED-009.
10. GREEN → REFACTOR.

### 6.6 라이브러리 결정

| 용도 | 라이브러리 | 결정 근거 |
|------|----------|---------|
| Cron scheduler | `github.com/robfig/cron/v3` v3.0.1+ | Go 생태계 사실상 유일 성숙 옵션, location-aware |
| 한국·글로벌 공휴일 | `github.com/rickar/cal/v2` v2.1.13+ | 20+개국 지원, 한국 음력 공휴일 포함 |
| 타임존 | stdlib `time` + `time/tzdata` | tzdata 임베드로 크로스플랫폼 |
| 로깅 | `go.uber.org/zap` | CORE-001 계승 |

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
| 외부 | Go 1.22+ | generics, atomic.Int32 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Cron이 시스템 suspend/resume 직후 정확하게 동작하지 않음 | 중 | 중 | `PersistedLastFire` 체크 + missed event replay (1시간 이상 지체 시 스킵, 그 이하는 즉시 1회 발화) |
| R2 | `rickar/cal/v2` 음력 공휴일 정확도 (설날/추석) | 낮 | 고 | 공휴일 제공자 interface로 추상화, custom override 경로 제공, 매년 CI에서 향후 3년 날짜 goldenfile 검증 |
| R3 | PatternLearner가 비정기 활동(주말 늦잠)을 오학습 | 중 | 중 | Weekday mask 분리(평일/주말), ±2시간 단일 조정 cap (REQ-SCHED-016) |
| R4 | Backoff window가 장시간 대화 세션 중 ritual을 완전 차단 | 중 | 중 | Backoff는 **defer**만, 스킵 아님. 최대 N회(3회) defer 후 강제 emit |
| R5 | Quiet hours가 야간 근무자(개발자 등)에게 과잉 차단 | 중 | 중 | `allow_nighttime: true` override + 사용자 첫 활동 시간 학습으로 자동 완화 |
| R6 | 중복 방지 key가 TZ 변경 시 잘못된 비교 | 중 | 고 | SuppressionKey는 `{event}:{userLocalDate}:{TZ}` 3-tuple, TZ 변경 시 새 key 사용 |
| R7 | Process downtime 중 리추얼 놓침 | 중 | 낮 | Missed event 정책: 1시간 이하 지체 → 즉시 1회 + "늦어서 죄송" 메시지; 이상 → 스킵 |

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
