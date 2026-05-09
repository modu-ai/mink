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
