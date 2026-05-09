## Task Decomposition
SPEC: SPEC-GOOSE-SCHEDULER-001 P1 (Cron Engine + Event Constants + Persistence)
Branch: feature/SPEC-GOOSE-SCHEDULER-001-P1
Methodology: TDD RED-GREEN-REFACTOR

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | 5 ritual HookEvent 등록 + HookEventNames() 갱신 + HOOK-001 dispatch_test 회귀 확인 | REQ-SCHED-002 | - | internal/hook/types.go (modify) | pending |
| T-002 | events.go: 5 ritual EventType re-export + ScheduledEvent + RitualTime + RegisteredEvents() | REQ-SCHED-002 | T-001 | internal/ritual/scheduler/events.go | pending |
| T-003 | config.go: SchedulerConfig (enabled/timezone/rituals.morning/meals/evening) + viper 로드 | REQ-SCHED-001, REQ-SCHED-010 | - | internal/ritual/scheduler/config.go | pending |
| T-004 | cron.go: robfig/cron/v3 래핑 + registerEntry(rt RitualTime) + parseClock("HH:MM") | REQ-SCHED-001, REQ-SCHED-005 | T-003 | internal/ritual/scheduler/cron.go (+ go.mod robfig/cron/v3) | pending |
| T-005 | persist.go: SchedulePersister — ~/.goose/ritual/schedule.json atomic write/read + MEMORY-001 facts.ritual_schedule round-trip | REQ-SCHED-003 | T-002, T-003 | internal/ritual/scheduler/persist.go | pending |
| T-006 | scheduler.go: Scheduler struct + New(cfg, deps) + Start(ctx) + Stop(ctx) + State() + atomic.Int32 + eventCh(buffered 32) skeleton (P3에서 worker 본격화) | REQ-SCHED-002, REQ-SCHED-009, REQ-SCHED-010 | T-002~T-005 | internal/ritual/scheduler/scheduler.go | pending |
| T-007 | scheduler_test.go: 5 RED tests with clockwork mock — TestRegisteredEvents_Exactly5 / TestCronEmitsInCorrectTZ / TestPersistAndReload / TestStartPartialFailure_StoppedInvariant / TestDisabled_Inert | AC-001/002/007/011/012 | T-001~T-006 | internal/ritual/scheduler/scheduler_test.go | pending |

### Drift Guard baseline (P1)
- Planned new files: 6 (events.go, config.go, cron.go, persist.go, scheduler.go, scheduler_test.go)
- Planned modifications: 1 (internal/hook/types.go)
- Total planned: 7 files
- Coverage gate: ≥80% for `internal/ritual/scheduler/` package
- LSP gate: 0 errors / 0 type errors / 0 lint errors (per quality.yaml run)

### P1 RED → GREEN → REFACTOR sequence
1. RED: T-007 의 5 테스트 작성 (실제 import 만 stub, 모두 실패)
2. GREEN: T-001 → T-002 → T-003 → T-004 → T-005 → T-006 순서로 최소 구현 (각 테스트 1건씩 GREEN)
3. REFACTOR: 중복 제거, 네이밍 정리, godoc 추가
4. coverage 측정 → ≥80% 검증
5. golangci-lint + go vet + gofmt clean
6. commit (squash 1개 PR)

---

## P2 Task Decomposition (Timezone Detector + Holiday Calendar)
SPEC: SPEC-GOOSE-SCHEDULER-001 P2
Branch: feature/SPEC-GOOSE-SCHEDULER-001-P2 (main HEAD = ddee87f)
External dep: rickar/cal/v2 v2.1.27 (P2 분기에서 go get 완료)

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-008 | timezone.go: TimezoneDetector — 시스템 TZ + config override + 24h baseline + ≥2h shift 감지 + EvNotification 발생 hook | REQ-SCHED-008 | - | internal/ritual/scheduler/timezone.go | pending |
| T-009 | holiday.go: HolidayCalendar interface + KoreanHolidayProvider (rickar/cal/v2/kr) + custom YAML override 경로 | REQ-SCHED-007, REQ-SCHED-017, REQ-SCHED-018 | - | internal/ritual/scheduler/holiday.go | pending |
| T-010 | holiday_data.go: rickar/cal/v2 imports + 한국 공휴일·대체공휴일 매핑 + 향후 3년(2026~2028) goldenfile fixture | REQ-SCHED-017 | T-009 | internal/ritual/scheduler/holiday_data.go | pending |
| T-011 | scheduler.go 확장 — TZ shift 감지 시 24h pause + EvNotification 발생, IsHoliday/HolidayName 을 ScheduledEvent 페이로드 주입, skip_weekends config 적용 | REQ-SCHED-007, REQ-SCHED-008, REQ-SCHED-018 | T-008, T-009, T-010 | internal/ritual/scheduler/scheduler.go (modify) | pending |
| T-012 | scheduler_test.go 확장 — 3 RED tests + custom YAML override 통합 1건 | AC-SCHED-004, AC-SCHED-009, AC-SCHED-016 | T-011 | internal/ritual/scheduler/scheduler_test.go (extend) | pending |

### P2 RED → GREEN → REFACTOR sequence
1. RED: T-012 의 3 테스트 작성 (TestKoreanHoliday_October3_And_SubstituteHoliday / TestTimezoneShift_24hPause / TestSkipWeekends) — stub 단계 모두 실패
2. GREEN: T-009 → T-010 → T-008 → T-011 순서로 최소 구현 (각 테스트 1건씩 GREEN)
3. REFACTOR: godoc 추가 (English), @MX 태그 갱신
4. coverage ≥80% 패키지 단위 (P1 91.0% 회귀 0)
5. golangci-lint + go vet + gofmt clean
6. commit (squash 1개 PR)

### P2 Exit Criteria
- AC-SCHED-004 GREEN (개천절 + 추석 대체공휴일)
- AC-SCHED-009 GREEN (TZ shift ≥2h → 24h pause)
- AC-SCHED-016 GREEN (주말 스킵)
- 향후 3년(2026~2028) 한국 공휴일 goldenfile 검증
- custom holiday YAML override 통합 테스트 1건
- 누적 8/20 AC GREEN

### Drift Guard baseline (P2)
- Planned new files: 3
- Planned modifications: 2
- Total planned: 5 files
- 외부 의존 신규: rickar/cal/v2 v2.1.27

---

## P3 Task Decomposition (BackoffManager + Dispatcher Worker + Quiet Hours)
SPEC: SPEC-GOOSE-SCHEDULER-001 P3
Branch: feature/SPEC-GOOSE-SCHEDULER-001-P3 (main HEAD = d4f2167)
External dep: 신규 없음 (clockwork v0.4.0 재사용)

### 의사결정 — QUERY-001 TurnCounter 추상화 (orchestrator + user 합의 2026-05-09)
- **결정**: Option A (Interface + DI 패턴) 채택.
- **근거**: QUERY-001 spec.md `internal/query/engine.go` 에 `LastTurnAt()` / `TurnCount()` public API 부재. SCHEDULER-001 에서 QUERY-001 spec amendment 를 강제하면 P3 범위가 두 패키지로 확장되어 SPEC trail 이 흐려짐.
- **구현**: `internal/ritual/scheduler/activity.go` 에 `ActivityClock interface { LastActivityAt() time.Time }` 정의. BackoffManager 가 의존. 테스트는 fake `ActivityClock` 함수 mock. 실 wiring (QueryEngine → ActivityClock 어댑터) 은 P4 또는 후속 SPEC 책임.
- **추후**: BRIEFING-001 / RITUAL-001 등이 ActivityClock 어댑터 추가 시 fan_in 증가하면 ANCHOR 부여.

### Tasks

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-013 | activity.go: `ActivityClock` interface (`LastActivityAt() time.Time`) + zero-value `noActivityClock` 헬퍼 (`zero time` 반환) | REQ-SCHED-011 | - | internal/ritual/scheduler/activity.go (신규) | pending |
| T-014 | backoff.go: `BackoffManager` struct + `ShouldDefer(now)` + `RecordDefer(key)` + `Reset(key)` + max_defer 카운터 (sync.Map keyed by `{event}:{userLocalDate}`) | REQ-SCHED-011, REQ-SCHED-021 | T-013 | internal/ritual/scheduler/backoff.go (신규) | pending |
| T-015 | config.go 확장: `AllowNighttime bool`, `Backoff BackoffConfig{ActiveWindow time.Duration, MaxDeferCount int}` (기본 10min/3) + `Validate()` 에 quiet-hours 검사 ([23:00, 06:00] 범위 + AllowNighttime=false → `ErrQuietHoursViolation`) | REQ-SCHED-014 | - | internal/ritual/scheduler/config.go (modify) | pending |
| T-016 | scheduler.go 확장 — eventCh worker 분리 (cron callback 은 send-only, dispatch.DispatchGeneric 직접 호출 제거) + Start 시 worker goroutine + Stop 시 graceful drain (`<-time.After(3s)` 보존) | REQ-SCHED-015 | - | internal/ritual/scheduler/scheduler.go (modify) | pending |
| T-017 | scheduler.go 확장 — BackoffManager 결합: cron callback 에서 ShouldDefer 검사 → defer 면 `clock.AfterFunc(active_window, retry)` reschedule + zap INFO `backoff_applied:true`. max_defer 초과 시 force emit + WARN `force_emit:true, defer_count:3`. ScheduledEvent 페이로드에 `BackoffApplied bool, DelayHint time.Duration` 필드 추가. AllowNighttime=true 케이스에서 첫 dispatch 시 WARN `nighttime_override:true` 로그 1회 | REQ-SCHED-014, REQ-SCHED-021 | T-014, T-015, T-016 | internal/ritual/scheduler/scheduler.go (modify) + events.go (modify) | pending |
| T-018 | scheduler_test.go 확장 — 5 RED tests (clockwork mock + fake ActivityClock + slow HookDispatcher mock):  TestBackoffDefers10Min(AC-003) / TestQuietHoursRejectedDeterministic(AC-005) / TestQuietHoursOverride_AllowNighttime(AC-013) / TestCronDispatcherDecoupling_BufferedChannel(AC-014) / TestMaxDeferCount_3_ForceEmit(AC-019) | AC-SCHED-003, 005, 013, 014, 019 | T-013~T-017 | internal/ritual/scheduler/scheduler_test.go (extend) | pending |

### P3 RED → GREEN → REFACTOR sequence
1. RED: T-018 의 5 테스트 작성 (모두 stub 단계 실패)
2. GREEN: T-013 → T-014 → T-015 → T-016 → T-017 순서로 최소 구현 (각 테스트 1건씩 GREEN)
3. REFACTOR: 중복 제거, English godoc, @MX 태그 갱신
4. coverage 측정 → ≥80% 패키지 단위 (P2 89.9% 회귀 0)
5. golangci-lint + go vet + gofmt clean
6. commit (squash 1개 PR)

### P3 Exit Criteria
- AC-SCHED-003 GREEN (backoff defer 10min, fake ActivityClock 5min 전 활동 mock)
- AC-SCHED-005 GREEN (quiet hours rejection — `Start` 가 `ErrQuietHoursViolation` 반환, state=Stopped 불변)
- AC-SCHED-013 GREEN (quiet hours override — AllowNighttime=true 시 발화 + WARN 로그 1회)
- AC-SCHED-014 GREEN (cron-dispatcher 디커플링 — cron goroutine 즉시 반환, eventCh 3 enqueue, worker 순차 처리)
- AC-SCHED-019 GREEN (max_defer 3회 후 force emit, DelayHint=30m, defer_count reset)
- 누적 13/20 AC GREEN (잔여 7건은 P4: AC-006/008/010/015/017/018/020)

### Drift Guard baseline (P3)
- Planned new files: 2 (activity.go, backoff.go)
- Planned modifications: 3 (config.go, scheduler.go, events.go) + 1 test extend (scheduler_test.go)
- Total planned: 6 files
- 외부 의존 신규: 없음
- 누적 lesson:
  - isolation 미사용 14회 무사고 (Sprint 1 전구간) → P3 동일 적용
  - LSP stale 10회 reproduction → P3 도 orchestrator 직접 build/vet verify
  - agent gofmt self-report 불신 → orchestrator 직접 `gofmt -l` verify

---

## P4 분할 정책 (orchestrator + user 합의 2026-05-09)
- **P4a (3 AC)**: AC-006 / AC-015 / AC-017 — PatternLearner 7일 수렴 + ±2h cap + 03:00 daily learner cron
- **P4b (4 AC)**: AC-008 / AC-010 / AC-018 / AC-020 — 3-tuple 중복 억제 + log schema 7 fields + FastForward build tag + missed event replay
- 사유: 7 AC 단일 PR 은 ~700 LOC review 부담, P3 (5 AC, +790 LOC) 와 균형 맞춤. INSIGHTS-001 의존 영역 (P4a) 과 영속·CI build tag 영역 (P4b) 자연스러운 도메인 분리.

---

## P4a Task Decomposition (PatternLearner + Daily Learner Cron)
SPEC: SPEC-GOOSE-SCHEDULER-001 P4a
Branch: feature/SPEC-GOOSE-SCHEDULER-001-P4a (main HEAD = 0a0d053)
External dep: 신규 없음

### 의사결정 — INSIGHTS-001 ActivityPattern 추상화 (orchestrator + user 합의 2026-05-09)
- **결정**: PatternReader interface DI 패턴 채택 (P3 ActivityClock 동일 패턴).
- **근거**: `internal/learning/insights/types.go:81` `type ActivityPattern struct { ByHour []HourBucket }` 는 public 이지만 `aggregateActivity()` 는 private. 실 wiring (InsightsEngine 어댑터) 은 P5/후속 SPEC 책임.
- **구현**: `internal/ritual/scheduler/pattern.go` 에 `PatternReader interface { ReadActivityPattern(ctx) (ActivityPattern, error) }` 정의. ActivityPattern 은 scheduler 자체 minimal struct 로 정의 (`ByHour [24]int + DaysObserved int`) — INSIGHTS-001 import 회피하여 의존성 단방향 유지.
- **추후**: REFLECT-001 / RITUAL-001 등이 PatternReader 어댑터 추가 시 fan_in 증가하면 ANCHOR 부여.

### Tasks

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-019 | pattern.go: `PatternReader` interface + `ActivityPattern{ByHour [24]int, DaysObserved int}` minimal struct + `RitualKind` enum (Morning/Breakfast/Lunch/Dinner/Evening) + `RitualTimeProposal{Kind, OldLocalClock, NewLocalClock, DriftMinutes, SupportingDays, Confidence, ConfirmRequired}` | REQ-SCHED-006, REQ-SCHED-016 | - | internal/ritual/scheduler/pattern.go (신규) | pending |
| T-020 | learner.go: `PatternLearner` struct + `Predict(kind RitualKind) (LocalClock string, confidence float64)` + `Observe(ActivityPattern) (*RitualTimeProposal, error)` + 7-day rolling window + ±2h cap + 3-day commit threshold + 6-day fallback `default 08:00` | REQ-SCHED-006, REQ-SCHED-012, REQ-SCHED-016 | T-019 | internal/ritual/scheduler/learner.go (신규) | pending |
| T-021 | config.go 확장: `PatternLearner PatternLearnerConfig{Enabled bool, RollingWindowDays int, DriftThresholdMinutes int, DefaultBreakfast, DefaultLunch, DefaultDinner string}` + 기본값 `Enabled=true, RollingWindow=7, DriftThreshold=30, Default=08:00/12:30/19:00` | REQ-SCHED-019 | - | internal/ritual/scheduler/config.go (modify) | pending |
| T-022 | scheduler.go 확장 — 03:00 daily learner cron entry 등록 (Start 시 PatternReader != nil 이면 자동) + `runDailyLearner(ctx)` callback: PatternReader.ReadActivityPattern → PatternLearner.Observe → drift > 30min 이면 EvNotification dispatch with `RitualTimeProposal` payload + `ConfirmRequired:true` | REQ-SCHED-006, REQ-SCHED-019 | T-019, T-020, T-021 | internal/ritual/scheduler/scheduler.go (modify) | pending |
| T-023 | scheduler.go 추가 wiring — `WithPatternReader(p PatternReader)` Option + Predict 로 빈 LocalClock 자동 채움 (config.Rituals.Meals.Breakfast.Time == "" → learner.Predict(Breakfast) → fallback) | REQ-SCHED-012 | T-020, T-022 | internal/ritual/scheduler/scheduler.go (modify) | pending |
| T-024 | scheduler_test.go 확장 — 3 RED tests (clockwork mock + fake PatternReader): TestPatternLearner_7DayConvergence(AC-006) / TestPatternLearner_2hCap_3DayCommit(AC-015) / TestDailyLearnerRun_0300_Confirmation(AC-017) | AC-SCHED-006, 015, 017 | T-019~T-023 | internal/ritual/scheduler/scheduler_test.go (extend) | pending |

### P4a RED → GREEN → REFACTOR sequence
1. RED: T-024 의 3 테스트 작성 (모두 stub 단계 실패)
2. GREEN: T-019 → T-020 → T-021 → T-022 → T-023 순서로 최소 구현
3. REFACTOR: 중복 제거, English godoc, @MX 태그 갱신
4. coverage 측정 → ≥80% (P3 89.1% 회귀 0)
5. golangci-lint + go vet + gofmt clean
6. commit (squash 1개 PR)

### P4a Exit Criteria
- AC-SCHED-006 GREEN (7일 수렴 confidence ≥0.7, 6일 fallback default 08:00)
- AC-SCHED-015 GREEN (±2h cap, 1 cycle 최대 +2h, 3일 연속 commit threshold)
- AC-SCHED-017 GREEN (03:00 daily learner cron, EvNotification ConfirmRequired:true, config 불변)
- 누적 16/20 AC GREEN (P4b 잔여 4건: AC-008/010/018/020)

### Drift Guard baseline (P4a)
- Planned new files: 2 (pattern.go, learner.go)
- Planned modifications: 2 (config.go, scheduler.go) + 1 test extend (scheduler_test.go)
- Total planned: 5 files
- 외부 의존 신규: 없음
- 누적 lesson (P3 이후):
  - isolation 미사용 15회 무사고
  - LSP stale 11회 reproduction → orchestrator 직접 verify
  - agent self-report 11회 일치 (lint/vet/gofmt) — LSP false-positive 만 차이

---

## P4b Task Decomposition (Suppression + Log Schema + FastForward + Missed Replay)
SPEC: SPEC-GOOSE-SCHEDULER-001 P4b
Branch: feature/SPEC-GOOSE-SCHEDULER-001-P4b (main HEAD = 4433e49)
External dep: 신규 없음

### 의사결정 — 영속 위치 (orchestrator + user 합의 2026-05-09)
- **결정**: AC-008 3-tuple suppression + AC-020 missed event replay 모두 JSON 파일 영속 (`~/.goose/ritual/fired_log.json`, schedule.json 동일 디렉토리). MEMORY-001 facts dispatcher 침범 회피.
- **자료구조**: `fired_log.json` = `map[string]time.Time` (key = "{event}:{userLocalDate}:{TZ}", value = UTC FiredAt). 단일 파일이 suppression key 와 last fire time 모두 충족.
- **AC-008 + AC-020 통합 로직**: Start 시 fired_log 로드 → 각 ritual의 today scheduled local time 계산 → 이미 지나간 + key 없음 → delta ≤ MissedEventReplayMaxDelay 면 replay 1회 + IsReplay=true, DelayMinutes=delta; delta > 면 skip + INFO log.
- **TZ-aware**: TZ 변경 시 새 key 발생 → suppression 효과 우회 (REQ-013 명시 동작).

### Tasks

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-025 | suppression.go: FiredKeyStore interface + JSONFiredKeyStore impl (Load/Mark/Has) + buildFiredKey(event, localDate, TZ) 3-tuple formatter + atomic write 보존 | REQ-SCHED-013 | - | internal/ritual/scheduler/suppression.go (신규) | pending |
| T-026 | events.go 확장: ScheduledEvent.IsReplay bool + DelayMinutes int 필드 추가 (REQ-SCHED-022) | REQ-SCHED-022 | - | internal/ritual/scheduler/events.go (modify) | pending |
| T-027 | config.go 확장: MissedEventReplayMaxDelay time.Duration (default 1h) | REQ-SCHED-022 | - | internal/ritual/scheduler/config.go (modify) | pending |
| T-028 | scheduler.go: callback에서 buildFiredKey → store.Has check → 이미 fired 면 skip + INFO log; emit 후 store.Mark(key, FiredAt). Start에서 replayMissedEvents(ctx) 추가 — 각 ritual의 today scheduled local time 계산 → 이미 지나간 + 미발화 → delta ≤ MaxDelay 면 dispatch with IsReplay=true; > 면 INFO `{skipped:true, reason:"missed_event_too_stale"}` | REQ-SCHED-013, REQ-SCHED-022 | T-025, T-026, T-027 | internal/ritual/scheduler/scheduler.go (modify) | pending |
| T-029 | logfields.go: emitFireLog(logger, ev, skipped, reason) 헬퍼 — INFO level zap 호출, exactly 7 fields {event, scheduled_at, actual_at, tz, holiday, backoff_applied, skipped} | REQ-SCHED-004 | T-026 | internal/ritual/scheduler/logfields.go (신규) + scheduler.go 통합 | pending |
| T-030 | scheduler_test_only.go (build tag `//go:build test_only`): Scheduler.FastForward(d time.Duration) 메서드 — clock.Advance(d) + 대기 중 이벤트 emit 트리거 (clockwork FakeClock 만 동작) | REQ-SCHED-020 | - | internal/ritual/scheduler/scheduler_test_only.go (신규) | pending |
| T-031 | scheduler_test.go 확장 — 4 RED tests (clockwork mock + tempdir fired_log + zaptest log capture): TestDuplicateSuppression_3Tuple_TZAware(AC-008) / TestLogSchema_Exactly7Fields(AC-010) / TestFastForward_BuildTagGating(AC-018) / TestMissedEventReplay_1hThreshold(AC-020) | AC-SCHED-008, 010, 018, 020 | T-025~T-030 | internal/ritual/scheduler/scheduler_test.go (extend) + scheduler_test_only_test.go (신규, build tag test_only) | pending |

### P4b RED → GREEN → REFACTOR sequence
1. RED: T-031 의 4 테스트 작성 (모두 stub 단계 실패)
2. GREEN: T-025 → T-026 → T-027 → T-029 → T-028 → T-030 순서로 최소 구현
3. REFACTOR: 중복 제거, English godoc, @MX 태그 갱신
4. coverage 측정 → ≥80% (P4a 84.8% 회귀 0)
5. golangci-lint + go vet + gofmt clean
6. (FastForward) `go test -tags=test_only` 별도 검증 — production binary 컴파일 안 됨 확인
7. commit (squash 1개 PR)

### P4b Exit Criteria
- AC-SCHED-008 GREEN (3-tuple TZ-aware suppression: 같은 날 같은 TZ → suppress, TZ 변경 → 새 key 처리)
- AC-SCHED-010 GREEN (zap INFO 로그 정확히 7 fields)
- AC-SCHED-018 GREEN (production build에 FastForward 미링크, test_only build tag 시에만 동작)
- AC-SCHED-020 GREEN (시나리오 A 30min → replay; 시나리오 B 1h30m → skip)
- 누적 20/20 AC GREEN — SPEC v0.2.0 완수, sync 진입 가능

### Drift Guard baseline (P4b)
- Planned new files: 3 (suppression.go, logfields.go, scheduler_test_only.go)
- Planned modifications: 3 (events.go, config.go, scheduler.go) + 1 test extend (scheduler_test.go)
- Total planned: 7 files
- 외부 의존 신규: 없음
- 누적 lesson (P4a 이후):
  - isolation 미사용 16회 무사고
  - LSP stale 12회 reproduction
  - 1M context API 차단 (P4a 1회) → orchestrator 직접 구현 정책 예외 적용 패턴
