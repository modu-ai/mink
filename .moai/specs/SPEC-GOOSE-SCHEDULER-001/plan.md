---
id: SPEC-GOOSE-SCHEDULER-001
artifact: plan
version: 0.2.0
spec_version: 0.2.0
status: audit-ready
created_at: 2026-04-22
updated_at: 2026-05-05
author: manager-spec
priority: critical
phase: 7
size: 중(M)
lifecycle: spec-anchored
labels: [scheduler, ritual, hook, phase-7, daily-companion, plan]
---

# Plan — SPEC-GOOSE-SCHEDULER-001

> 본 문서는 `spec.md` (v0.2.0, 22 REQ + 20 AC) 의 구현 계획이다. **시간 추정 없이** 우선순위·의존 순서·완료 기준만 정의한다. 모든 deliverable은 `spec.md` §4 EARS 요구사항 및 §5 AC와 1:1 추적된다.

## 1. 개요 (Plan Overview)

### 1.1 목표

`internal/ritual/scheduler/` 패키지를 SDD 2025 표준에 따라 4개 milestone (P1~P4)으로 분할 구현한다. 각 milestone은:

- **독립적으로 머지 가능한** PR 단위
- 자체 RED→GREEN→REFACTOR 사이클 (TDD mode, `quality.development_mode=tdd`)
- exit criteria 명시 (REQ + AC 단위)
- TRUST 5 quality gate 통과

### 1.2 Milestone 의존 그래프

```
P1 (cron + persistence)         ─────┐
                                     ├──► P3 (backoff + dispatcher decoupling)
P2 (timezone + holiday)         ─────┘                                    │
                                                                          ▼
                                                          P4 (catchup + pattern + quiet hours)
```

- P1, P2는 병렬 가능(공통 의존 없음).
- P3는 P1+P2 산출물 모두 필요.
- P4는 P1~P3 전부 필요 (PatternLearner는 cron + persist + dispatch 인프라 위에 동작).

### 1.3 외부 의존 (선행 SPEC 머지 상태)

| 선행 SPEC | 현재 상태 | 본 SPEC 의존 강도 |
|----------|---------|----------------|
| HOOK-001 | (확인 필요) | **HARD** — 5개 신규 HookEvent 등록 필수, P1 차단 가능 |
| INSIGHTS-001 | (확인 필요) | P4에서만 필요 (PatternLearner) |
| MEMORY-001 | implemented (v1.0+) | P1에서 필요 (영속) |
| CORE-001 | (확인 필요) | P1에서 필요 (zap, ctx, graceful shutdown) |
| CONFIG-001 | (확인 필요) | P1에서 필요 (config.yaml 로드) |
| QUERY-001 | (확인 필요) | P3에서만 필요 (TurnCounter) |

> **차단 요소 사전 점검**: P1 시작 전 HOOK-001 / CORE-001 / CONFIG-001 / MEMORY-001 status 확인 필수. 미머지 SPEC 발견 시 본 plan 진입 보류.

---

## 2. Milestone 분해

### 2.1 P1 — Cron Engine + Event Constants + Persistence

**목적**: scheduler 패키지의 골격(scheduler/cron/events/persist) 구축. mock clock 기반으로 cron entry → HookDispatcher 직결 경로를 확립한다.

#### 2.1.1 Deliverables

| 파일 | 역할 |
|-----|-----|
| `internal/ritual/scheduler/scheduler.go` | `Scheduler` struct, `New(cfg, deps)`, `Start(ctx)`, `Stop(ctx)`, `State()`. atomic.Int32 state, eventCh(buffered 32) skeleton. (단, dispatch worker 본격화는 P3) |
| `internal/ritual/scheduler/cron.go` | `robfig/cron/v3` 래핑, `registerEntry(rt RitualTime)`, `parseClock(s string)` |
| `internal/ritual/scheduler/events.go` | 5개 `HookEvent` 상수 + `ScheduledEvent` struct + `RitualTime` struct + `RegisteredEvents()` |
| `internal/ritual/scheduler/persist.go` | `SchedulePersister`: `~/.goose/ritual/schedule.json` write/read + MEMORY-001 `facts.ritual_schedule` 연동 |
| `internal/ritual/scheduler/config.go` | `SchedulerConfig` (enabled/timezone/rituals) + viper 로드 |
| `internal/ritual/scheduler/scheduler_test.go` (테스트 골격) | clockwork 기반 테이블 테스트 |

#### 2.1.2 REQ 커버리지

- REQ-SCHED-001 (IANA TZ 강제 + 명시적 TZ 필드)
- REQ-SCHED-002 (정확히 5개 이벤트)
- REQ-SCHED-003 (100ms 이내 영속화)
- REQ-SCHED-005 (cron 발화 → HookDispatcher) — **P1에서는 backoff 없는 직결 경로만**, REQ-SCHED-005의 backoff 분기는 P3로 이월
- REQ-SCHED-009 (Start 부분 실패 시 Stopped 불변)
- REQ-SCHED-010 (enabled=false 불활성)

#### 2.1.3 AC 커버리지

- AC-SCHED-001 (5 이벤트 등록)
- AC-SCHED-002 (cron + TZ 존중)
- AC-SCHED-007 (영속성 + reload)
- AC-SCHED-011 (Start 부분 실패)
- AC-SCHED-012 (enabled=false 불활성)

#### 2.1.4 Exit Criteria

- [ ] 위 5개 AC 테스트 GREEN
- [ ] 5개 신규 HookEvent 가 `internal/hook/event.go` (HOOK-001) 에 등록되고 HOOK-001의 dispatcher 단위 테스트가 회귀 없음
- [ ] `~/.goose/ritual/schedule.json` round-trip + MEMORY-001 `facts.ritual_schedule` round-trip 동시 검증
- [ ] golangci-lint clean, `go vet ./internal/ritual/...` clean
- [ ] 패키지 단위 coverage ≥ 80%
- [ ] PR 본문에 SPEC/REQ/AC trailer 포함

#### 2.1.5 의도적 비포함 (P1 OUT)

- BackoffManager (P3 책임)
- HolidayCalendar (P2 책임)
- TimezoneDetector 의 travel 감지 (P2 책임)
- PatternLearner (P4 책임)
- Missed event replay (P4 책임)
- FastForward build-tag-gated API (P4 책임)

---

### 2.2 P2 — Timezone Detector + Holiday Calendar

**목적**: 사용자 로케일·여행 상황·공휴일 인식 인프라 구축. P1의 cron 발화에 IsHoliday/HolidayName 페이로드와 TZ shift 일시정지 능력을 부여한다.

#### 2.2.1 Deliverables

| 파일 | 역할 |
|-----|-----|
| `internal/ritual/scheduler/timezone.go` | `TimezoneDetector`: 시스템 TZ + config override + 24h baseline 비교 + ≥2h shift 감지 |
| `internal/ritual/scheduler/holiday.go` | `HolidayCalendar` interface + `KoreanHolidayProvider` 기본 구현 (rickar/cal/v2/kr) + custom YAML override 경로 |
| `internal/ritual/scheduler/holiday_data.go` (또는 `holiday_korean.go`) | rickar/cal/v2 imports + 한국 공휴일·대체공휴일 매핑 + golden fixture (향후 3년) |
| `internal/ritual/scheduler/scheduler.go` (확장) | TZ shift 감지 시 24h pause + `EvNotification` 발생, IsHoliday/HolidayName 을 `ScheduledEvent` 페이로드에 주입 |

#### 2.2.2 REQ 커버리지

- REQ-SCHED-007 (Holiday → ScheduledEvent.IsHoliday/HolidayName)
- REQ-SCHED-008 (TZ shift ≥2h → 24h pause + EvNotification)
- REQ-SCHED-017 (한국 공휴일 인식 + 대체공휴일)
- REQ-SCHED-018 (skip_weekends + skip_holidays)

#### 2.2.3 AC 커버리지

- AC-SCHED-004 (개천절 + 추석 대체공휴일)
- AC-SCHED-009 (TZ shift 24h pause)
- AC-SCHED-016 (주말 스킵)

#### 2.2.4 Exit Criteria

- [ ] 위 3개 AC GREEN
- [ ] `rickar/cal/v2` 의존성 추가, go.mod 잠금
- [ ] 향후 3년(2026~2028) 한국 공휴일 goldenfile 검증 테스트 추가
- [ ] custom holiday YAML override 경로 1개 통합 테스트
- [ ] coverage ≥ 80%

#### 2.2.5 의도적 비포함 (P2 OUT)

- US/Japan provider (interface는 정의하되 구현 미루어 SCHEDULER-002로)
- GPS/IP 기반 TZ 감지 (시스템 TZ override 만 검증)

---

### 2.3 P3 — Backoff Manager + Dispatcher Decoupling + Quiet Hours

**목적**: 사용자 활동 동시성 제어와 야간 보호. P1의 직결 dispatch를 buffered channel + worker 모델로 진화시키고, defer/skip/force-emit 정책을 도입한다.

#### 2.3.1 Deliverables

| 파일 | 역할 |
|-----|-----|
| `internal/ritual/scheduler/backoff.go` | `BackoffManager`: QUERY-001 TurnCounter 의존, `ShouldDefer()`, `RecordDefer()`, `DeferCount(key)` |
| `internal/ritual/scheduler/scheduler.go` (확장) | dispatch worker goroutine, eventCh 버퍼 32, defer 재스케줄 +10min, max_defer_count=3 후 force-emit, quiet hours validation @ Start, `allow_nighttime` override |
| `internal/ritual/scheduler/errors.go` | `ErrQuietHoursViolation` 등 sentinel errors |
| `internal/ritual/scheduler/config.go` (확장) | `backoff.active_window_min`, `backoff.max_defer_count`, `allow_nighttime`, `rituals.X.skip_weekends` 필드 추가 |

#### 2.3.2 REQ 커버리지

- REQ-SCHED-005 (cron 발화 → backoff 상담 분기 완성)
- REQ-SCHED-011 (활동 윈도우 동안 ShouldDefer=true)
- REQ-SCHED-014 (quiet hours HARD floor `[23:00, 06:00]`)
- REQ-SCHED-015 (cron goroutine ↔ dispatcher 분리, buffered channel)
- REQ-SCHED-021 (max_defer_count=3 후 force-emit + DelayHint)

#### 2.3.3 AC 커버리지

- AC-SCHED-003 (backoff 10분 defer + 재스케줄)
- AC-SCHED-005 (quiet hours 거부 + Stopped 불변, 결정론적)
- AC-SCHED-013 (allow_nighttime override 발화)
- AC-SCHED-014 (cron-dispatcher 디커플링, buffered channel)
- AC-SCHED-019 (3회 defer 후 강제 emit)

#### 2.3.4 Exit Criteria

- [ ] 위 5개 AC GREEN
- [ ] race detector 통과 (`go test -race ./internal/ritual/...`)
- [ ] eventCh 버퍼 포화 시 (32) backpressure 시나리오 명시적 테스트 추가
- [ ] zap 로그 스키마: `{event, backoff_applied, deferred_to, force_emit, defer_count, nighttime_override}` 필드 일관
- [ ] coverage ≥ 80%

#### 2.3.5 의도적 비포함 (P3 OUT)

- PatternLearner 가 야간 활동 감지 후 자동 `allow_nighttime=true` 권유 (P4)

---

### 2.4 P4 — Pattern Learner + Missed Event Replay + Quiet Hours Logging + FastForward

**목적**: Daily Companion proactive 학습 루프 완성. 사용자 시간표 자동 추론, 프로세스 재시작 시 missed event 회복, structured log schema 통일, 테스트 가속(FastForward)을 달성한다.

#### 2.4.1 Deliverables

| 파일 | 역할 |
|-----|-----|
| `internal/ritual/scheduler/pattern.go` | `PatternLearner`: INSIGHTS-001 ActivityPattern 소비, 7일 rolling window, ±2h cap, 3일 연속 확정, `RitualTimeProposal` 생성 |
| `internal/ritual/scheduler/proposal.go` | `RitualTimeProposal` struct + serialization (HOOK `EvNotification` payload) |
| `internal/ritual/scheduler/replay.go` | Missed event 탐지: 영속화된 last-fire 기준, gap ≤ `missed_event_replay_max_delay`(default 1h)면 replay, 초과 시 skip |
| `internal/ritual/scheduler/scheduler.go` (확장) | Start 시 replay 통합, 03:00 daily learner cron, `EvNotification` 발생 |
| `internal/ritual/scheduler/scheduler_test_only.go` | `//go:build test_only` gating, `FastForward(d time.Duration)` |
| `internal/ritual/scheduler/log.go` | 7-필드 zap schema 통일 wrapper: `{event, scheduled_at, actual_at, tz, holiday, backoff_applied, skipped}` |

#### 2.4.2 REQ 커버리지

- REQ-SCHED-004 (구조화 로그 7 필드)
- REQ-SCHED-006 (PatternLearner.Observe + 30분 drift threshold)
- REQ-SCHED-012 (config 비어있을 때 learner.Predict, 7일 미만 fallback default)
- REQ-SCHED-013 (3-tuple SuppressionKey, TZ-aware)
- REQ-SCHED-016 (±2h cap + 3일 연속 commit)
- REQ-SCHED-019 (03:00 daily learner + 사용자 확인 전 commit 금지)
- REQ-SCHED-020 (FastForward build-tag gating)
- REQ-SCHED-022 (missed event replay ≤1h, IsReplay=true, DelayMinutes)

#### 2.4.3 AC 커버리지

- AC-SCHED-006 (7일 학습 + confidence ≥0.7, 6일 fallback)
- AC-SCHED-008 (3-tuple SuppressionKey + TZ 변경 시 새 key)
- AC-SCHED-010 (로그 스키마 7 필드 정확 일치)
- AC-SCHED-015 (PatternLearner ±2h cap + 3일 확정)
- AC-SCHED-017 (03:00 learner + 확인 flow)
- AC-SCHED-018 (FastForward build-tag gating)
- AC-SCHED-020 (missed event replay 시나리오 A/B)

#### 2.4.4 Exit Criteria

- [ ] 위 7개 AC GREEN
- [ ] 전체 20 AC 합산 GREEN, 회귀 0
- [ ] coverage ≥ 85% (TRUST 5 Tested 기준)
- [ ] `go test -tags test_only ./internal/ritual/...` 통과 + production build (`go build`) 에서 `FastForward` 심볼 부재 확인
- [ ] zap 로그 스키마 검증 테스트 (정확히 7 필드, 누락 시 FAIL)
- [ ] PR 본문에 누적 AC 매핑 + REQ 커버리지 보고

#### 2.4.5 의도적 비포함 (P4 OUT)

- 야간 근무자 자동 모드 전환 권유 (research.md §5.2 — SCHEDULER-002 후속)
- 다중 사용자 분리 스케줄 (Family mode)
- Multi-device MEMORY 동기화 (A2A-001 책임)

---

## 3. 기술 접근 요약 (Technical Approach Summary)

> 상세는 `spec.md` §6 참조.

- **Cron**: `robfig/cron/v3` v3.0.1+ (`cron.WithLocation` + `cron.WithChain(Recover, SkipIfStillRunning)`)
- **Holiday**: `rickar/cal/v2` v2.1.13+ (`kr` 모듈) + custom YAML override
- **Mock clock**: `jonboulle/clockwork` v0.4+ (test-only)
- **Logging**: `go.uber.org/zap` (CORE-001 계승), 7-필드 schema 통일
- **TZ**: stdlib `time` + `time/tzdata` (크로스플랫폼 임베드)
- **Persistence**: `~/.goose/ritual/schedule.json` (atomic write) + MEMORY-001 `facts.ritual_schedule` (SQL transaction)
- **Concurrency**: cron goroutine → buffered eventCh(32) → dispatch worker, atomic.Int32 state, race-clean

### 3.1 핵심 데이터 구조 (스니펫)

```go
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
    clock       clockwork.Clock      // mockable
}
```

### 3.2 SuppressionKey 정규형 (REQ-SCHED-013, research.md §6.1)

```
key = fmt.Sprintf("%s:%s:%s", event, userLocalDate, tz)
예: "MorningBriefingTime:2026-04-25:Asia/Seoul"
```

3-tuple: `{event, userLocalDate, TZ}`. TZ 변경 시 새 key 생성으로 travel-aware.

---

## 4. 의존성 (Dependencies)

> 상세는 `spec.md` §7 참조.

### 4.1 SPEC 의존

- **HARD 선행**: HOOK-001, MEMORY-001, CORE-001, CONFIG-001
- **MEDIUM 선행**: INSIGHTS-001 (P4), QUERY-001 (P3)
- **후속 소비자**: BRIEFING-001, HEALTH-001, JOURNAL-001, RITUAL-001

### 4.2 외부 라이브러리

| 라이브러리 | 버전 | 도입 milestone |
|----------|------|--------------|
| `github.com/robfig/cron/v3` | v3.0.1+ | P1 |
| `github.com/rickar/cal/v2` | v2.1.13+ | P2 |
| `github.com/jonboulle/clockwork` | v0.4+ | P1 (test-only) |
| `go.uber.org/zap` | (CORE-001 기존) | P1 |
| Go stdlib `time/tzdata` | Go 1.22+ | P1 |

### 4.3 Config 키

| Key | Default | Milestone |
|-----|---------|-----------|
| `scheduler.enabled` | true | P1 |
| `scheduler.timezone` | system | P1 |
| `scheduler.rituals.morning.time` | "" (learner) | P1 |
| `scheduler.rituals.meals.{breakfast,lunch,dinner}.time` | "" | P1 |
| `scheduler.rituals.evening.time` | "" | P1 |
| `scheduler.rituals.morning.skip_weekends` | false | P2 |
| `scheduler.holidays.provider` | "korean" | P2 |
| `scheduler.backoff.active_window_min` | 10 | P3 |
| `scheduler.backoff.max_defer_count` | 3 | P3 |
| `scheduler.allow_nighttime` | false | P3 |
| `scheduler.pattern_learner.enabled` | true | P4 |
| `scheduler.missed_event_replay_max_delay` | 1h | P4 |
| `scheduler.debug.fast_forward` | false (+build tag) | P4 |

---

## 5. 리스크 & 완화 (Risks & Mitigations Plan)

> spec.md §8 의 R1~R7 을 milestone 별로 매핑.

| Risk | Milestone | 완화 작업 |
|------|----------|---------|
| R1 (suspend/resume) | P4 | Missed event replay (REQ-SCHED-022) |
| R2 (음력 공휴일 정확도) | P2 | `rickar/cal/v2/kr` + 향후 3년 goldenfile + custom override YAML |
| R3 (비정기 활동 오학습) | P4 | ±2h cap (REQ-SCHED-016) + 3일 연속 commit + Weekday mask 분리 |
| R4 (Backoff 무한 차단) | P3 | max_defer_count=3 후 force-emit (REQ-SCHED-021) |
| R5 (Quiet hours 야간 근무자 차단) | P3 (override) → P4 (자동 학습 권유는 SCHEDULER-002 이월) | `allow_nighttime: true` config override |
| R6 (TZ 변경 시 SuppressionKey 오비교) | P4 | 3-tuple 정규형 (REQ-SCHED-013) |
| R7 (Process downtime 리추얼 누락) | P4 | Replay ≤1h (REQ-SCHED-022) |

---

## 6. TDD 진입 순서 (Test-First Roadmap)

> spec.md §6.5 의 RED 순서를 milestone 매핑과 함께 재정렬.

### P1 RED 순서 (5건)

1. `TestRegisteredEvents_Exactly5` (AC-001)
2. `TestCronEmitsInCorrectTZ` (AC-002, mock clock)
3. `TestPersistAndReload` (AC-007, schedule.json + MEMORY round-trip)
4. `TestStartPartialFailure_StoppedInvariant` (AC-011)
5. `TestDisabled_Inert` (AC-012)

### P2 RED 순서 (3건)

6. `TestKoreanHoliday_October3_And_SubstituteHoliday` (AC-004)
7. `TestTimezoneShift_24hPause` (AC-009)
8. `TestSkipWeekends` (AC-016)

### P3 RED 순서 (5건)

9. `TestBackoffDefers10Min` (AC-003)
10. `TestQuietHoursRejectedDeterministic` (AC-005)
11. `TestQuietHoursOverride_AllowNighttime` (AC-013)
12. `TestCronDispatcherDecoupling_BufferedChannel` (AC-014)
13. `TestMaxDeferCount_3_ForceEmit` (AC-019)

### P4 RED 순서 (7건)

14. `TestPatternLearner_7DayConvergence` (AC-006)
15. `TestDuplicateSuppression_3Tuple_TZAware` (AC-008)
16. `TestLogSchema_Exactly7Fields` (AC-010)
17. `TestPatternLearner_2hCap_3DayCommit` (AC-015)
18. `TestDailyLearnerRun_0300_Confirmation` (AC-017)
19. `TestFastForward_BuildTagGating` (AC-018, `-tags=test_only`)
20. `TestMissedEventReplay_1hThreshold` (AC-020)

총 20개 테스트 = 20개 AC 1:1 매핑.

---

## 7. TRUST 5 매핑

> spec.md §6.7 의 매핑을 plan 단위로 재정리.

| 차원 | P1 | P2 | P3 | P4 | 최종 |
|-----|----|----|----|----|-----|
| **T**ested | AC 5/20 | AC 8/20 | AC 13/20 | AC 20/20 | ≥85% line coverage |
| **R**eadable | scheduler/cron/events/persist/config 분리 | timezone/holiday 분리 | backoff/errors 분리 | pattern/proposal/replay/log 분리 | golangci-lint clean |
| **U**nified | UTC 비교 + TZ 메타데이터 | IsHoliday payload 통일 | zap 로그 partial schema | 7-field schema 완성 | gofmt + golangci-lint clean |
| **S**ecured | atomic write (schedule.json) | (없음) | quiet hours HARD floor + race-clean | force-emit log + missed-stale skip | OWASP 비해당 (배경 daemon) |
| **T**rackable | SPEC/REQ/AC trailer | 동일 | 동일 | 동일 + Coverage Map 첨부 | 4 PR 각각 SPEC trailer |

---

## 8. Out-of-Scope (Plan 차원)

> spec.md §3.2 OUT SCOPE을 plan 차원에서 재확인.

- Tool registration (`scheduler_create_job`, `scheduler_list_jobs`, ...) — **본 SPEC 범위 아님**. SCHEDULER-002 후속에서 TOOLS-001 인터페이스로 노출 예정.
- CLI 관리 명령 (`goose scheduler list/add/remove/show`) — 본 SPEC 범위 아님. CLI-001 ~ CLI-TUI-003 의 후속 PR 또는 SCHEDULER-002 에서 처리.
- Custom user-defined cron jobs (5개 고정 외) — 본 SPEC 범위 아님.
- 외부 푸시 알림 (APNS/FCM) — Gateway SPEC 책임.
- 가족/멀티유저 모드 — adaptation.md §9.2 후속.

> 사용자 메시지에 언급된 "Tool 등록 / CLI / Catchup window / Concurrency advisory lock" 중 **Catchup (=missed event replay)** 와 **Concurrency (=cron dispatcher decoupling + atomic state)** 는 본 SPEC P3/P4에서 처리. **Tool 등록 / CLI** 는 명시적으로 SCHEDULER-002 이월 (spec.md §3.2 OUT SCOPE 보존).

---

## 9. 완료 기준 (Plan-level Definition of Done)

본 plan 전체가 완료되었다고 선언하기 위한 조건:

- [ ] P1~P4 4개 PR 전부 main 머지
- [ ] 20개 AC 모두 GREEN, 회귀 0
- [ ] 패키지 coverage ≥ 85%
- [ ] `go test -race ./internal/ritual/...` PASS
- [ ] `go build` (production) 에서 `FastForward` 심볼 부재 검증
- [ ] BRIEFING-001 / HEALTH-001 / JOURNAL-001 dispatcher 단에서 5개 신규 이벤트 수신 통합 테스트 1건 이상
- [ ] CHANGELOG 항목 추가 (Phase 7 Daily Companion 기반 인프라)
- [ ] SPEC status: `draft → audit-ready → in_progress → implemented`

---

**End of Plan — SPEC-GOOSE-SCHEDULER-001**
