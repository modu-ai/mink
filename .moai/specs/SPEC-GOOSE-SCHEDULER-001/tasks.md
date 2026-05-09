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
