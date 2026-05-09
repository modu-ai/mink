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
