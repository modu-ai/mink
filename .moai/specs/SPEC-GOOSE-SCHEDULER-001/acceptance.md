---
id: SPEC-GOOSE-SCHEDULER-001
artifact: acceptance
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
labels: [scheduler, ritual, hook, phase-7, daily-companion, acceptance]
---

# Acceptance Criteria — SPEC-GOOSE-SCHEDULER-001

> 본 문서는 `spec.md` §5 의 20개 AC 를 SDD 2025 표준의 별도 acceptance artifact 로 분리·정렬한 것이다. 모든 AC 는 `spec.md` §4 의 22개 EARS REQ 에 1:N 매핑되며, 각 시나리오는 **결정론적·이진 PASS/FAIL** 이어야 한다.

---

## 1. AC Format Declaration

### 1.1 포맷

본 SPEC 의 수용 기준은 **Given / When / Then (BDD)** 포맷을 사용한다. EARS 5 패턴은 `spec.md` §4 의 REQ 요구사항에서만 사용된다.

### 1.2 결정론 규칙

- **이진 PASS/FAIL**: 모든 AC 는 실행 결과가 PASS 또는 FAIL 로 결정되어야 한다. "or", "either", "optionally" 등의 애매성은 금지한다.
- **시간 의존 제거**: `time.Sleep`, 실 wall-clock 의존은 mock clock (`clockwork.Clock`) 또는 동기 신호 (`<-done`) 로 대체한다.
- **명시적 호출 횟수**: dispatcher 호출은 "정확히 N회" 로 검증하며, "≥ N회" 와 같은 부등식은 금지한다.
- **로그 스키마**: zap 로그 검증은 in-memory sink + 필드 정확 일치 (누락/추가 모두 FAIL) 로 검증한다.

### 1.3 1:N 매핑 (REQ ↔ AC)

`spec.md` §5 의 매핑이 정본이다. §4 (Coverage Map) 에 행렬화된 표가 있다.

---

## 2. 핵심 수용 기준 (AC-SCHED-001 ~ AC-SCHED-009)

### AC-SCHED-001 — 5개 이벤트 상수 등록

**Covers**: REQ-SCHED-001, REQ-SCHED-002

- **Given**: `internal/ritual/scheduler` 패키지가 빌드된 상태
- **When**: `scheduler.RegisteredEvents()` 호출
- **Then**: 정확히 5개 문자열이 반환됨 — `MorningBriefingTime`, `PostBreakfastTime`, `PostLunchTime`, `PostDinnerTime`, `EveningCheckInTime`. 추가로 HOOK-001 의 `HookEventNames()` 에 동일 5개가 포함되어 총 29개 (HOOK-001 동기화 전제). 6개 이상이거나 4개 이하이면 FAIL.

---

### AC-SCHED-002 — Cron 기반 emit + Timezone 존중

**Covers**: REQ-SCHED-005

- **Given**: `SchedulerConfig{TZ:"Asia/Seoul", Rituals.Morning.Time:"07:30"}`, mock `clockwork.Clock`, mock `HookDispatcher`
- **When**: mock clock 을 Asia/Seoul 07:30:00 에 도달시키고 dispatcher 의 `done` 채널을 동기적으로 수신
- **Then**: `HookDispatcher.DispatchMorningBriefingTime` 이 정확히 1회 호출됨. 동일 시각을 UTC 22:30 으로 mock 한 별도 시나리오에서는 호출 0회 (TZ 미존중 시 FAIL).

---

### AC-SCHED-003 — Backoff 지연 (10분)

**Covers**: REQ-SCHED-005, REQ-SCHED-011

- **Given**: `BackoffManager.ActiveWindowMin=10`, mock `QueryEngine.LastTurnAt = clock.Now() - 5min`
- **When**: `MorningBriefingTime` cron entry 가 발화
- **Then**: 즉시 dispatch 0회, 10분 후 재시도 스케줄 등록됨 (cron entry count +1 확인). zap INFO 로그에 `{event:"MorningBriefingTime", backoff_applied:true, deferred_to:"+10m"}` 정확히 1회 기록.

---

### AC-SCHED-004 — 한국 공휴일 인식 (개천절 + 추석 대체공휴일)

**Covers**: REQ-SCHED-007, REQ-SCHED-017

- **Given**: `HolidayCalendar.Provider="korean"`, 날짜 fixture: (a) 2026-10-03 개천절, (b) 2026-09-28 추석 대체공휴일
- **When**: `HolidayCalendar.IsHoliday(date, "ko-KR")` 각각 호출
- **Then**:
  - (a) `(true, "개천절")` 반환, 해당일 `MorningBriefingTime` dispatch 시 `ScheduledEvent.IsHoliday=true, HolidayName="개천절"`.
  - (b) `(true, "추석 대체공휴일")` 반환 — 대체공휴일 규칙 검증.
  - 둘 중 하나라도 false 거나 이름 불일치면 FAIL.

---

### AC-SCHED-005 — Quiet Hours 차단 (결정론)

**Covers**: REQ-SCHED-014

- **Given**: `config.scheduler.rituals.morning.time="02:30"` (quiet-hours 범위 [23:00, 06:00] 내), `config.scheduler.allow_nighttime=false`
- **When**: `Scheduler.Start(ctx)` 호출
- **Then**:
  - `Start` 가 `ErrQuietHoursViolation` error 를 반환,
  - `Scheduler.State()` == `Stopped`,
  - `s.cron.Entries()` 빈 슬라이스,
  - zap ERROR 로그에 `{event:"morning", configured_time:"02:30", reason:"quiet_hours_violation"}` 정확히 1회 기록.
- **반드시 단일 결정론 경로**. clamp 정책은 없으며, "또는 자동 조정" 같은 분기는 FAIL.

---

### AC-SCHED-006 — PatternLearner 자동 시간 학습

**Covers**: REQ-SCHED-006, REQ-SCHED-012

- **Given**: 7일간 사용자가 매일 08:15±15분 첫 activity 기록 (INSIGHTS-001 ActivityPattern mock), `config.scheduler.rituals.meals.breakfast.time=""` (unset)
- **When**: `PatternLearner.Predict(Breakfast)` 호출
- **Then**:
  - 반환 `LocalClock` ∈ [08:00, 08:30],
  - `confidence` ≥ 0.7.
  - 별도 시나리오에서 6일치 data 만 주어질 때는 fallback default `"08:00"` 반환 (REQ-SCHED-012). 7일 이상에서 0.7 미만이거나 6일에서 default 미반환이면 FAIL.

---

### AC-SCHED-007 — 시간표 영속성 (round-trip) [PARTIAL]

**Covers**: REQ-SCHED-003

- **Given**: 스케줄러가 학습된 시간표 `{morning:07:30, lunch:12:15, dinner:19:00}` 보유
- **When**: `Scheduler.Save()` 호출 후 프로세스 종료 → 재시작하여 `Scheduler.Load()`
- **Then**:
  - `~/.goose/ritual/schedule.json` 에 3 엔트리 존재 (저장 완료 후 100ms 이내 파일 stat 확인),
  - MEMORY-001 `facts` 테이블의 `ritual_schedule` 네임스페이스에 동일 3 엔트리 존재,
  - 재시작 후 load 된 RitualTime 3개의 `{EventName, LocalClock, TZ}` 가 저장 시와 완전 일치.
- 100ms 초과·둘 중 하나 누락·필드 불일치 시 FAIL.

> **PARTIAL 사유**: MEMORY-001 facts 테이블 round-trip은 MEMORY-001 SPEC 완성 후 통합 예정. 파일 round-trip(`schedule.json`)만 현재 GREEN. 출처: REVIEW-SCHEDULER-001-2026-05-10 W2.

---

### AC-SCHED-008 — Duplicate 억제 (3-tuple, TZ-aware)

**Covers**: REQ-SCHED-013

- **Given**: 오늘(2026-04-25) 07:30 KST 에 `MorningBriefingTime` 이미 dispatch 됨 → MEMORY-001 `ritual_fired` 에 key `"MorningBriefingTime:2026-04-25:Asia/Seoul"` 저장. 07:45 에 프로세스 재시작.
- **When**: `Scheduler.Start(ctx)` 호출 → cron 이 07:30 missed-event 를 replay 시도
- **Then**:
  - (a) 동일 3-tuple key 매칭으로 07:30 재발화 0회,
  - (b) 내일(2026-04-26) 07:30 은 새 key `"MorningBriefingTime:2026-04-26:Asia/Seoul"` 로 정상 예약,
  - (c) 별도 시나리오 — 07:45 에 TZ 를 `Asia/Tokyo` 로 변경 후 restart 하면 key `"MorningBriefingTime:2026-04-25:Asia/Tokyo"` 는 기록에 없으므로 새 key 로 취급되어 R7 missed-event replay 정책 (REQ-SCHED-022) 적용.

---

### AC-SCHED-009 — Timezone shift 24h pause

**Covers**: REQ-SCHED-008

- **Given**: 사용자가 서울(KST) → 뉴욕(EST) 이동, mock `TimezoneDetector.Detect()` 가 TZ 변경 감지 (Δ=14h, cap 적용 후 effective shift ≥2h)
- **When**: 다음 detection tick 에서 `Detect()` 가 baseline 과 다른 값 반환
- **Then**:
  - (a) 이후 24시간(mock clock) 동안 ritual emit 0회,
  - (b) `TimezoneShiftDetected` notification 이 HOOK `EvNotification` 으로 정확히 1회 발생,
  - (c) 24시간 경과 후 다음 cron tick 에서 새 TZ(EST) 기준으로 ritual 재개. 24h 이전 emit 발생·notification 미발생·24h 후 미재개 시 FAIL.

---

## 3. 추가 수용 기준 (AC-SCHED-010 ~ AC-SCHED-020)

### AC-SCHED-010 — 로그 스키마 검증 (정확히 7 필드)

**Covers**: REQ-SCHED-004

- **Given**: `Scheduler` running, mock `HookDispatcher`, zap logger with in-memory sink
- **When**: 임의의 5개 ritual 중 하나가 트리거되어 dispatch / backoff defer / skip 중 하나 발생
- **Then**: 기록된 INFO 레벨 로그 엔트리의 필드 집합이 정확히 `{event, scheduled_at, actual_at, tz, holiday, backoff_applied, skipped}` 7개를 포함. 누락 또는 추가 필드가 있으면 FAIL.

---

### AC-SCHED-011 — Start 부분 실패 시 Stopped 불변

**Covers**: REQ-SCHED-009

- **Given**: `SchedulerConfig.TZ="Invalid/Zone"` (또는 persist load 실패 주입)
- **When**: `Scheduler.Start(ctx)` 호출
- **Then**:
  - (a) `Start` 가 non-nil error 반환,
  - (b) `Scheduler.State()` == `Stopped`,
  - (c) `s.cron.Entries()` 빈 슬라이스,
  - (d) 이후 `Stop()` 호출은 nil error (no-op).

---

### AC-SCHED-012 — `enabled=false` 불활성

**Covers**: REQ-SCHED-010

- **Given**: `config.scheduler.enabled=false`, mock clock = Asia/Seoul 07:30
- **When**: `Scheduler.Start(ctx)` 호출 후 mock clock 을 24시간 전진
- **Then**:
  - (a) `Start` nil error,
  - (b) `cron.Entries()` empty,
  - (c) 24h 동안 `HookDispatcher` 호출 0회,
  - (d) `Stop()` no-op.

---

### AC-SCHED-013 — Quiet Hours Override (`allow_nighttime=true`)

**Covers**: REQ-SCHED-014

- **Given**: `config.scheduler.rituals.morning.time="02:30"`, `config.scheduler.allow_nighttime=true`
- **When**: `Scheduler.Start(ctx)` 호출 후 mock clock 을 02:30 도달
- **Then**:
  - (a) `Start` nil error,
  - (b) 02:30 에 `DispatchMorningBriefingTime` 정확히 1회 호출,
  - (c) zap WARN 로그에 `{event:"morning", nighttime_override:true}` 정확히 1회 기록.
- AC-SCHED-005 의 반대 케이스. override 시 발화 허용 분기 검증.

---

### AC-SCHED-014 — Cron-Dispatcher 디커플링 (Buffered Channel)

**Covers**: REQ-SCHED-015

- **Given**: mock `HookDispatcher.DispatchMorningBriefingTime` 가 2초간 block (slow consumer 시뮬레이션), cron 엔트리 다수 등록
- **When**: 3개 이벤트가 1초 간격으로 연속 발화
- **Then**:
  - (a) cron goroutine 은 3회 모두 즉시 반환 (각 fire 소요 시간 < 10ms),
  - (b) `eventCh`(버퍼 32) 에 이벤트 3개 enqueue,
  - (c) dispatcher worker 가 순차 처리하여 6초 후 3회 dispatch 완료.
- cron 틱 정밀도가 dispatcher 지연에 영향받으면 FAIL.

---

### AC-SCHED-015 — PatternLearner ±2h Cap + 3일 확정

**Covers**: REQ-SCHED-016

- **Given**: 현재 commit 된 breakfast 시간 `08:00`, 1일치 activity peak 가 `11:30` (Δ=+3h30m, cap 초과), 7일 rolling window
- **When**: `PatternLearner.Observe(day1)` 호출
- **Then**:
  - (a) 1일 관측만으로는 commit 되지 않음 (proposal 만 기록),
  - (b) 연속 3일 동일 패턴(11:30 ±10min) 관측 후에도 `|Δ|=3h30m > 2h cap` 이므로 1 cycle 최대 +2h 만 적용되어 `10:00` 으로 이동 제안,
  - (c) `RitualTimeProposal.NewLocalClock == "10:00"`. cap 초과 commit 시 FAIL.

---

### AC-SCHED-016 — 주말 스킵

**Covers**: REQ-SCHED-018

- **Given**: `config.scheduler.rituals.morning.skip_weekends=true`, mock clock = (a) 2026-04-25 07:30 (토요일), (b) 2026-04-27 07:30 (월요일)
- **When**: 각각 cron 발화
- **Then**:
  - (a) 토요일(2026-04-25): `DispatchMorningBriefingTime` 호출 0회, zap INFO `{event:"morning", skipped:true, reason:"weekend"}` 정확히 1회 기록,
  - (b) 월요일(2026-04-27): 정확히 1회 호출.

---

### AC-SCHED-017 — 03:00 Daily PatternLearner + 확인 Flow

**Covers**: REQ-SCHED-019

- **Given**: `config.scheduler.pattern_learner.enabled=true`, 전일 ActivityPattern 이 MEMORY-001 에 존재, 제안 변화량 > 30분 (REQ-SCHED-006 threshold)
- **When**: mock clock 을 03:00 local 도달
- **Then**:
  - (a) `PatternLearner.Observe(prevDay)` 정확히 1회 호출,
  - (b) HOOK `EvNotification` 1회 발생, payload `{kind:"RitualTimeProposal", confirm_required:true}`,
  - (c) 사용자 확인 전에는 config 에 commit 되지 않음 (`config.scheduler.rituals.morning.time` 불변).

---

### AC-SCHED-018 — FastForward Build-Tag Gating

**Covers**: REQ-SCHED-020

- **Given**: production binary 빌드 (`go build`, 기본 build tag)
- **When**: Scheduler 인스턴스에서 `FastForward(d)` 호출 시도
- **Then**:
  - (a) production build: `FastForward` 심볼이 링크되지 않아 컴파일 에러 또는 `ErrFastForwardNotAvailable` panic,
  - (b) test build (`go test -tags=test_only`): `FastForward(d)` 가 동작하여 mock clock 을 `d` 만큼 전진시키고 대기 중인 모든 trigger 를 순서대로 emit.
- Build tag `//go:build test_only` 로 파일 단위 gating 된 심볼 확인. production build 에서 심볼 존재 시 FAIL.

---

### AC-SCHED-019 — 최대 3회 Defer 후 강제 Emit

**Covers**: REQ-SCHED-021

- **Given**: `BackoffManager.ActiveWindowMin=10`, `config.scheduler.backoff.max_defer_count=3`, mock `QueryEngine.LastTurnAt` 가 지속적으로 `clock.Now() - 1min` (항상 active)
- **When**: 07:30 `MorningBriefingTime` cron 발화 → defer → +10min 후 재발화 → defer → ... 반복
- **Then**:
  - (a) 1회차(07:30), 2회차(07:40), 3회차(07:50) 모두 defer,
  - (b) 4회차(08:00) 에서 active 상태여도 **강제 emit**, `DispatchMorningBriefingTime` 정확히 1회 호출,
  - (c) zap WARN 로그 `{event:"morning", force_emit:true, defer_count:3}` 정확히 1회 기록,
  - (d) BRIEFING-001 payload 의 `DelayHint == "30m"`.

---

### AC-SCHED-020 — Process Restart 시 Missed Event Replay

**Covers**: REQ-SCHED-022

- **Given**:
  - 시나리오 A: 07:30 KST dispatch 예정 → 프로세스 07:00~08:00 downtime → 재시작 08:00 (지체 30분)
  - 시나리오 B: 동일 dispatch 예정 → 재시작 09:00 (지체 1시간 30분)
- **When**: 각 시나리오에서 `Scheduler.Start(ctx)` 호출
- **Then**:
  - (a) 시나리오 A: 08:00 기준 `DispatchMorningBriefingTime` 정확히 1회 replay 발생, payload `{IsReplay:true, DelayMinutes:30}`, SuppressionKey `"MorningBriefingTime:2026-04-25:Asia/Seoul"` 저장.
  - (b) 시나리오 B: `Dispatch*` 호출 0회 (1h 초과 → 스킵), zap INFO `{event:"morning", skipped:true, reason:"missed_event_too_stale", delay_min:90}` 정확히 1회 기록.

---

## 4. Coverage Map (REQ ↔ AC 행렬)

| REQ | AC | Milestone |
|-----|----|----|
| REQ-SCHED-001 | AC-001 | P1 |
| REQ-SCHED-002 | AC-001 | P1 |
| REQ-SCHED-003 | AC-007 | P1 |
| REQ-SCHED-004 | AC-010 | P4 |
| REQ-SCHED-005 | AC-002, AC-003 | P1 (AC-002), P3 (AC-003) |
| REQ-SCHED-006 | AC-006 | P4 |
| REQ-SCHED-007 | AC-004 | P2 |
| REQ-SCHED-008 | AC-009 | P2 |
| REQ-SCHED-009 | AC-011 | P1 |
| REQ-SCHED-010 | AC-012 | P1 |
| REQ-SCHED-011 | AC-003 | P3 |
| REQ-SCHED-012 | AC-006 | P4 |
| REQ-SCHED-013 | AC-008 | P4 |
| REQ-SCHED-014 | AC-005, AC-013 | P3 |
| REQ-SCHED-015 | AC-014 | P3 |
| REQ-SCHED-016 | AC-015 | P4 |
| REQ-SCHED-017 | AC-004 | P2 |
| REQ-SCHED-018 | AC-016 | P2 |
| REQ-SCHED-019 | AC-017 | P4 |
| REQ-SCHED-020 | AC-018 | P4 |
| REQ-SCHED-021 | AC-019 | P3 |
| REQ-SCHED-022 | AC-020 | P4 |

**총계**: 22 REQ, 20 AC. AC-002, AC-003, AC-004, AC-006, AC-014 가 다중 REQ 커버 (1 AC ↔ N REQ). 모든 REQ 가 최소 1개 AC 에 매핑됨.

---

## 5. Edge Cases (테스트 시 고려 항목)

각 AC 가 명시적으로 다루지 않지만, 구현 시 회귀 테스트 후보:

1. **DST 전환** (Asia/Seoul 은 DST 없으나, America/New_York 등 사용 시): 03:00 daily learner 시각이 "존재하지 않거나 두 번 발생" 하는 경우 — robfig/cron/v3 의 location-aware 처리 의존.
2. **Leap second**: cron 이 분 단위이므로 무시 가능, 단 missed event replay 의 gap 계산은 `time.Time.Sub` 사용 (UTC monotonic).
3. **schedule.json 손상**: malformed JSON 발견 시 `Scheduler.Start` 가 `ErrPersistCorrupt` 반환 + 백업본(`schedule.json.bak`) 으로 fallback 시도 (구현 결정 필요, P1 검토).
4. **MEMORY-001 다운** (sqlite 락): persist 실패 시 schedule.json fallback 만으로 동작 (graceful degradation).
5. **eventCh 포화** (32 초과 누적): cron 발화는 즉시 반환하되 이벤트 drop 발생, zap WARN `{event:"...", channel_full:true}` 기록 (P3 backpressure 검토).
6. **HolidayCalendar.Provider="disabled"**: 모든 IsHoliday() 가 false 반환, skip_holidays 옵션 무력화.
7. **TZ Detect 이상치 진동** (5분 내 KST↔EST 반복): pause 가 연달아 reset 되지 않도록 24h debounce 구현.
8. **PatternLearner cold start** (history 0일): Predict() 가 default 시간 반환, confidence=0.0.

---

## 6. Definition of Done

본 SPEC 의 모든 acceptance 가 충족되었다고 선언하기 위한 조건:

- [ ] AC-SCHED-001 ~ AC-SCHED-020 모두 GREEN
- [ ] `go test -race ./internal/ritual/scheduler/...` PASS
- [ ] `go test -tags test_only ./internal/ritual/scheduler/...` PASS (FastForward 검증)
- [ ] Production `go build` 에서 `FastForward` 심볼 부재 검증 (`go tool nm` 또는 build 자동 검사)
- [ ] 패키지 line coverage ≥ 85%
- [ ] Branch coverage ≥ 75% (P3 backoff 분기 다수)
- [ ] golangci-lint 0 warnings (`gofmt`, `vet`, `staticcheck`, `errcheck`)
- [ ] 5개 신규 HookEvent 가 HOOK-001 의 dispatcher 회귀 테스트에서 통과
- [ ] BRIEFING-001 / HEALTH-001 / JOURNAL-001 의 receive end 통합 테스트 1건 이상 (E2E light)
- [ ] Coverage Map 의 모든 REQ ↔ AC 매핑이 PR 본문에 인용

---

## 7. Quality Gate Criteria (TRUST 5)

| 차원 | 기준 | 검증 방법 |
|-----|-----|---------|
| **T**ested | line coverage ≥ 85%, AC 20/20 GREEN | `go test -coverprofile` |
| **R**eadable | golangci-lint clean, 파일 단위 책임 분리 (10개 파일) | `golangci-lint run` |
| **U**nified | gofmt clean, UTC 비교 통일, 7-필드 zap schema | `gofmt -l`, log schema test |
| **S**ecured | quiet hours HARD floor 강제, race-clean, atomic write | `go test -race`, manual review |
| **T**rackable | 4 PR 모두 SPEC/REQ/AC trailer + 한국어 본문, CHANGELOG 항목 | `git log` 정규식, PR template |

---

## 8. 외부 검증 권장 (Optional)

- **Phase 7 Daily Companion 통합 데모**: 본 SPEC 머지 + BRIEFING-001 + RITUAL-001 머지 후, mock clock 을 사용하여 24h 시뮬레이션 (07:30 brief, 12:30 lunch, 19:00 dinner, 22:30 evening) 통합 시나리오 1건 작성.
- **장시간 안정성**: 실 wall-clock 으로 7일 연속 실행하여 panic / goroutine leak / memory growth 모니터링 (별도 SPEC 또는 manual QA).

---

**End of Acceptance — SPEC-GOOSE-SCHEDULER-001**
