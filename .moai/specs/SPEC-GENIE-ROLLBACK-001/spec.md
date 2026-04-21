---
id: SPEC-GENIE-ROLLBACK-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P1
issue_number: null
phase: 5
size: 소(S)
lifecycle: spec-anchored
---

# SPEC-GENIE-ROLLBACK-001 — Regression 자동 감지 및 30일 Staleness Rollback

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (agency constitution §15 + learning-engine §6.3 계승, REFLECT+SAFETY hand-off 계약) | manager-spec |

---

## 1. 개요 (Overview)

REFLECT-001이 `Graduated` 상태로 승격한 `LearningEntry`가 실제 사용자 상태에 적용된 이후, **적용 결과가 사용자 만족도를 의미 있게 저하시키는 경우**를 자동 감지하여 즉시 롤백하는 회귀 감지 파이프라인을 정의한다.

본 SPEC은 다음 세 요소를 묶는다:

1. **Regression Detector** — 매 trajectory session 종료 시 또는 주기적(일 1회) Tick 시점에 집계된 사용자 만족도가 baseline 대비 0.10 이상 하락했는지 판정.
2. **Rollback Executor** — 감지 시 REFLECT-001의 `MarkRolledBack`을 호출하고, LORA-001의 `RestoreVersion(previousVersion)` hook을 트리거하여 이전 LoRA로 복귀.
3. **Staleness Window** — 롤백된 entry는 30일간 `Promoter.Ingest` 시 재제안 금지 (REFLECT-001 REQ-REFLECT-015와 직접 연계).

본 SPEC은 **감지 로직과 롤백 트리거**만 책임진다. 실제 LoRA 파일 교체나 CLAUDE.md evolvable zone 복원은 각 consumer SPEC(LORA-001 등)이 구현하는 `Restorer` 인터페이스에 위임한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- REFLECT-001의 `MarkRolledBack`은 호출자가 필요하지만, 현재까지 호출자가 없다. ROLLBACK-001이 유일한 호출자다.
- SAFETY-001의 CanaryCheck는 **승격 전 예방적 회귀 검증**이고, ROLLBACK-001은 **승격 후 사후 회귀 감지**다. 두 계층이 결합해야 "모르고 승격된 나쁜 변경"을 복구할 수 있다.
- learning-engine.md §6.3 "Continual Learning Evaluation Pipeline"이 명시한 회귀 감지 다이어그램의 `< 0.75` 또는 `> 0.10 회귀` 조건이 본 SPEC의 판정 규칙이다.

### 2.2 상속 자산

- **agency/constitution §15 Evolution Rollback** (FROZEN):
  > "If a graduated learning causes regression:
  > - Automatic rollback triggered when next project score drops > 0.10
  > - Reverted change logged in .agency/evolution/rollbacks.log
  > - Learning status changed to 'rolled-back'
  > - Learning cannot be re-proposed for 30 days (staleness_window_days)"
- **learning-engine.md §6.3**:
  ```go
  if newAvg < 0.75 {
      result.Status = StatusRollback
  } else if regression > 0.10 {
      result.Status = StatusWarning
  }
  ```
  GENIE는 warning 없이 0.10 이상 회귀 즉시 rollback (위 임계값 그대로).
- **SPEC-REFLECT-001 (MoAI)**: 동일 rollback 메커니즘. GENIE는 `Restorer` hand-off를 LoRA까지 확장.

### 2.3 GENIE-specific 차이점

| 축 | MoAI | GENIE |
|----|------|------|
| Baseline 단위 | project | trajectory session |
| 회귀 대상 | skill body / agent prompt | LoRA adapter / CLAUDE.md evolvable zone / skill body |
| Restorer | 파일 git revert | LoRA version switch (LORA-001 hook) + 파일 revert |
| Staleness | 30일 (동일) | 30일 (동일) |

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/evolve/rollback/` 패키지 생성.
2. `RegressionDetector` 인터페이스:
   - `Evaluate(ctx, entry LearningEntry) (RegressionResult, error)`
   - 최근 N(=5)개 session의 satisfaction score를 집계하여 baseline(승격 직전 7일 평균) 대비 비교
3. `RegressionResult` 구조체 (Detected bool, ScoreDrop float64, BaselineScore, CurrentScore float64, EvaluatedSessions int, Threshold float64).
4. `RollbackExecutor` 인터페이스:
   - `Execute(ctx, entry LearningEntry, trigger RollbackTrigger) error`
   - 내부적으로 `Restorer.Restore(previousVersion)` 호출 + REFLECT-001의 `MarkRolledBack` 호출
5. `RollbackTrigger` enum: `AutomaticRegression`, `UserInitiated`, `SafetyOverride`.
6. `Restorer` 인터페이스 (외부 consumer가 구현):
   - `PreviousVersion(entry LearningEntry) (VersionRef, error)`
   - `Restore(entry LearningEntry, version VersionRef) error`
7. `StalenessWindow` 관리:
   - 롤백된 entry의 `RollbackDeadline = rollbackAt + 30일` 설정 위임(REFLECT-001이 수행)
   - 본 SPEC은 `MarkRolledBack` 호출 시 `triggeredBy`, `at` 인자를 정확히 전달
8. Rollback log:
   - 모든 롤백 이벤트를 `.moai/evolution/rollbacks.log`에 append-only JSON-L로 기록
   - 필드: `ts`, `learning_id`, `trigger`, `score_before`, `score_after`, `score_drop`, `evaluated_sessions`, `restorer_result`
9. `Daemon.Tick` 통합 훅:
   - `RegressionWatcher` 구조체가 주기적(기본 24h) Tick 호출
   - REFLECT-001에서 `Graduated` 상태 entry를 순회하며 `RegressionDetector.Evaluate` 호출
   - `Detected == true`면 `RollbackExecutor.Execute` 호출
10. 설정 제어:
    - `rollback.regression_threshold: 0.10` (기본값, learning-engine 준수)
    - `rollback.min_sessions_for_detection: 5` (최소 평가 세션 수)
    - `rollback.evaluation_interval: 24h`
    - `rollback.baseline_window: 7d` (승격 직전 며칠을 baseline으로 삼을지)

### 3.2 OUT OF SCOPE

- LearningEntry 상태 머신 자체 → `REFLECT-001`
- SafetyToken 발급/검증 → `SAFETY-001`
- Trajectory 수집/저장 → `TRAJECTORY-001`
- Satisfaction score 계산 알고리즘 → `INSIGHTS-001`
- LoRA adapter 버전 관리, 복원 → `LORA-001`
- CLAUDE.md evolvable zone 버전 관리 → (별도, 향후 SPEC)
- Canary 재평가 (승격 전 예방) → `SAFETY-001` Layer 2
- 사용자 수동 롤백 UI → `CLI-001` 또는 향후 desktop client
- Staleness 이후 재제안 경로 → `REFLECT-001`이 `Archived` 전이로 처리

---

## 4. 요구사항 (EARS Requirements)

### 4.1 Ubiquitous

- **REQ-ROLLBACK-001**: 시스템은 모든 롤백 이벤트를 `.moai/evolution/rollbacks.log`에 append-only JSON-L로 기록**해야 한다**.
- **REQ-ROLLBACK-002**: 시스템은 `Graduated` 상태 entry만을 회귀 감지 대상으로 취급**해야 한다** (다른 상태는 무시).

### 4.2 Event-Driven

- **REQ-ROLLBACK-003**: **When** `RegressionWatcher.Tick`이 호출될 때, 시스템은 REFLECT-001의 `ListByStatus(StatusGraduated)`를 조회하고 각 entry에 대해 `RegressionDetector.Evaluate`를 실행**해야 한다**.
- **REQ-ROLLBACK-004**: **When** `RegressionDetector.Evaluate`가 `ScoreDrop > 0.10`을 반환할 때, 시스템은 `RollbackExecutor.Execute`를 `trigger=AutomaticRegression`으로 호출**해야 한다**.
- **REQ-ROLLBACK-005**: **When** `RollbackExecutor.Execute`가 호출될 때, 시스템은 `Restorer.PreviousVersion` → `Restorer.Restore` → REFLECT-001 `MarkRolledBack` 순서로 실행**해야 한다**.
- **REQ-ROLLBACK-006**: **When** `Restorer.Restore`가 오류를 반환할 때, 시스템은 `MarkRolledBack`을 호출하지 않고 rollback log에 `restorer_result: failed`로 기록하며 오류를 호출자에 전파**해야 한다**.

### 4.3 State-Driven

- **REQ-ROLLBACK-007**: **While** 특정 entry에 대해 `EvaluatedSessions < min_sessions_for_detection(=5)` 상태일 때, 시스템은 해당 entry의 회귀 판정을 연기하고 추가 데이터를 기다**려야 한다**.

### 4.4 Optional

- **REQ-ROLLBACK-008**: **Where** 구성 `rollback.regression_threshold`가 제공된 경우, 시스템은 기본값 0.10 대신 해당 값을 임계값으로 사용**해야 한다**.
- **REQ-ROLLBACK-009**: **Where** 구성 `rollback.evaluation_interval`이 제공된 경우, 시스템은 기본값 24h 대신 해당 값을 Tick 주기로 사용**해야 한다**.

### 4.5 Unwanted Behavior

- **REQ-ROLLBACK-010**: **If** 동일 entry에 대해 `RollbackExecutor.Execute`가 이미 성공한 상태에서 중복 호출**되면 then** 시스템은 `ErrAlreadyRolledBack`을 반환하고 Restorer 호출을 생략**해야 한다**.
- **REQ-ROLLBACK-011**: **If** `Restorer.PreviousVersion`이 `ErrNoPreviousVersion`을 반환**하면 then** 시스템은 롤백을 중단하고 사용자 개입을 요청하는 HOOK 이벤트(`HookEventRollbackManualIntervention`)를 발행**해야 한다**.

### 4.6 Complex

- **REQ-ROLLBACK-012**: **While** `Daemon`이 실행 중이고, **when** 매 `evaluation_interval`마다 자동 Tick이 발생하면, 시스템은 `Graduated` entry 전수 평가를 수행하고 결과를 `rollbacks.log`에 통합 기록**해야 한다**.

---

## 5. Go 타입 시그니처

```go
// internal/evolve/rollback/types.go
package rollback

import (
    "context"
    "time"

    "github.com/genie/internal/evolve/reflect"
)

// RegressionResult 는 회귀 감지 결과이다.
type RegressionResult struct {
    Detected           bool
    BaselineScore      float64
    CurrentScore       float64
    ScoreDrop          float64
    Threshold          float64
    EvaluatedSessions  int
    BaselineWindowDays int
    EvaluatedAt        time.Time
}

// RollbackTrigger 는 롤백 사유이다.
type RollbackTrigger int

const (
    TriggerAutomaticRegression RollbackTrigger = iota
    TriggerUserInitiated
    TriggerSafetyOverride
)

// VersionRef 는 Restorer 가 이해하는 이전 버전 식별자이다.
// 구현은 LoRA adapter 파일 경로, git commit SHA 등 무엇이든 될 수 있다.
type VersionRef struct {
    Kind string // "lora" | "file" | "git_sha"
    Ref  string
    Meta map[string]string
}

// Restorer 는 실제 복원을 수행하는 외부 consumer 인터페이스이다.
type Restorer interface {
    // PreviousVersion 은 entry 적용 이전 상태의 version ref 를 반환한다.
    PreviousVersion(entry reflect.LearningEntry) (VersionRef, error)

    // Restore 는 지정된 version 으로 복원한다.
    Restore(entry reflect.LearningEntry, version VersionRef) error
}

// RegressionDetector 는 회귀 판정을 수행한다.
type RegressionDetector interface {
    Evaluate(ctx context.Context, entry reflect.LearningEntry) (RegressionResult, error)
}

// RollbackExecutor 는 감지 결과를 받아 실제 롤백을 실행한다.
type RollbackExecutor interface {
    Execute(ctx context.Context, entry reflect.LearningEntry, trigger RollbackTrigger) error
}

// RegressionWatcher 는 Daemon 에서 주기적 Tick 을 유발하는 컴포넌트이다.
type RegressionWatcher interface {
    Start(ctx context.Context) error // 백그라운드 goroutine 실행
    Stop(ctx context.Context) error
    Tick(ctx context.Context, now time.Time) (TickReport, error)
}

// TickReport 는 Tick 1회 결과 요약이다.
type TickReport struct {
    EvaluatedEntries int
    Rolledback       []string // entry ID list
    DeferredEntries  int      // 최소 세션 미달로 연기
    Errors           []error
    StartedAt        time.Time
    FinishedAt       time.Time
}

// internal/evolve/rollback/watcher.go

// DefaultRegressionWatcher 는 default 구현이다.
type DefaultRegressionWatcher struct {
    interval         time.Duration
    detector         RegressionDetector
    executor         RollbackExecutor
    memory           reflect.ReflectMemory
    logger           RollbackLogger
    hookEmitter      HookEmitter // HOOK-001 hand-off (RollbackManualIntervention)
}

// RollbackLogger 는 rollbacks.log append 를 담당한다.
type RollbackLogger interface {
    Append(entry RollbackLogEntry) error
}

// RollbackLogEntry 는 JSON-L 1줄에 해당한다.
type RollbackLogEntry struct {
    Timestamp         time.Time         `json:"ts"`
    LearningID        string            `json:"learning_id"`
    Trigger           string            `json:"trigger"`
    BaselineScore     float64           `json:"score_before"`
    CurrentScore      float64           `json:"score_after"`
    ScoreDrop         float64           `json:"score_drop"`
    EvaluatedSessions int               `json:"evaluated_sessions"`
    RestorerResult    string            `json:"restorer_result"` // "ok" | "failed" | "skipped"
    RestorerError     string            `json:"restorer_error,omitempty"`
    VersionRef        *VersionRef       `json:"version_ref,omitempty"`
}

// HookEmitter 는 HOOK-001 에 수동 개입 이벤트를 발행한다.
type HookEmitter interface {
    EmitRollbackManualIntervention(ctx context.Context, entry reflect.LearningEntry, reason string) error
}
```

---

## 6. 수락 기준 (Acceptance Criteria)

- **AC-ROLLBACK-001** (REQ-ROLLBACK-003, 004 자동 감지)
  - **Given** `Graduated` entry 1개 + mock RegressionDetector가 `ScoreDrop=0.15` 반환,
  - **When** `RegressionWatcher.Tick(now)`이 호출되면,
  - **Then** `RollbackExecutor.Execute(entry, TriggerAutomaticRegression)`이 정확히 1회 호출되고 `TickReport.Rolledback`에 해당 entry ID가 포함된다.

- **AC-ROLLBACK-002** (REQ-ROLLBACK-005 실행 순서)
  - **Given** `RollbackExecutor`와 mock Restorer, mock REFLECT,
  - **When** `Execute(entry, TriggerAutomaticRegression)`이 호출되면,
  - **Then** 호출 순서는 `Restorer.PreviousVersion` → `Restorer.Restore` → `reflect.MarkRolledBack`이며, rollback log에 `restorer_result: "ok"`로 기록된다.

- **AC-ROLLBACK-003** (REQ-ROLLBACK-006 Restorer 실패 처리)
  - **Given** mock Restorer가 `Restore`에서 `errors.New("disk full")` 반환,
  - **When** `Execute`가 호출되면,
  - **Then** `MarkRolledBack`은 호출되지 않고, rollback log에 `restorer_result: "failed"`, `restorer_error: "disk full"`로 기록되며, 오류가 caller에 전파된다.

- **AC-ROLLBACK-004** (REQ-ROLLBACK-007 최소 세션 미달)
  - **Given** `EvaluatedSessions=3` (< 5) 이고 `ScoreDrop=0.20`인 상황,
  - **When** `RegressionDetector.Evaluate`가 호출되면,
  - **Then** `Detected=false`이고, `Execute`는 호출되지 않으며, `TickReport.DeferredEntries`가 1 증가한다.

- **AC-ROLLBACK-005** (REQ-ROLLBACK-010 중복 호출 차단)
  - **Given** 이미 `RolledBack` 상태인 entry,
  - **When** `Execute`가 중복 호출되면,
  - **Then** `ErrAlreadyRolledBack`이 반환되고 `Restorer.Restore`는 호출되지 않는다.

- **AC-ROLLBACK-006** (REQ-ROLLBACK-011 수동 개입 요청)
  - **Given** Restorer가 `PreviousVersion`에서 `ErrNoPreviousVersion` 반환,
  - **When** `Execute`가 호출되면,
  - **Then** `Restore`와 `MarkRolledBack`은 모두 호출되지 않고, HOOK에 `HookEventRollbackManualIntervention`이 발행된다.

- **AC-ROLLBACK-007** (REQ-ROLLBACK-001, 012 로그 및 통합)
  - **Given** 임의의 `Execute` 호출,
  - **When** 실행이 완료되면 (성공/실패 무관),
  - **Then** `.moai/evolution/rollbacks.log`에 JSON-L 1줄이 append되며, 해당 line은 `ts`, `learning_id`, `trigger`, `restorer_result`를 모두 포함하고 유효한 JSON이다.

---

## 7. 기술적 접근

### 7.1 패키지 레이아웃

```
internal/evolve/rollback/
├── types.go           # RegressionResult, RollbackTrigger, VersionRef, TickReport
├── detector.go        # RegressionDetector 기본 구현 (baseline 계산 + drop 판정)
├── executor.go        # RollbackExecutor 구현 (3단계 orchestration)
├── watcher.go         # RegressionWatcher (Daemon 통합)
├── logger.go          # rollbacks.log append-only
├── errors.go          # ErrAlreadyRolledBack, ErrNoPreviousVersion
├── internal/
│   └── clock.go       # testability
└── testdata/
    └── baseline_fixtures/
        └── sessions_N.jsonl
```

### 7.2 Baseline 계산 알고리즘

```
baseline_score(entry, now) =
    mean(satisfaction_score for session in TRAJECTORY-001.List(
        start = entry.ApprovedAt - 7*24h,
        end   = entry.ApprovedAt
    ))

current_score(entry, now) =
    mean(satisfaction_score for session in TRAJECTORY-001.List(
        start = entry.ApprovedAt,
        end   = now
    ))

score_drop = baseline_score - current_score
```

- `baseline_window` 기본 7일
- `current` 기간은 `ApprovedAt` 이후 누적 (최근 N=5 이상)
- `EvaluatedSessions < 5`이면 `Detected=false` 반환 (REQ-ROLLBACK-007)

### 7.3 RollbackExecutor 순서 보장

```go
func (e *DefaultRollbackExecutor) Execute(
    ctx context.Context,
    entry reflect.LearningEntry,
    trigger RollbackTrigger,
) error {
    if entry.Status == reflect.StatusRolledBack {
        return ErrAlreadyRolledBack
    }

    // 1. PreviousVersion 조회
    version, err := e.restorer.PreviousVersion(entry)
    if errors.Is(err, ErrNoPreviousVersion) {
        // HOOK 발행 후 조기 복귀
        _ = e.hookEmitter.EmitRollbackManualIntervention(ctx, entry, "no_previous_version")
        e.logger.Append(RollbackLogEntry{
            LearningID: entry.ID, Trigger: trigger.String(),
            RestorerResult: "skipped", RestorerError: err.Error(),
        })
        return err
    }
    if err != nil { /* same skipped log */ return err }

    // 2. Restore 실행
    if err := e.restorer.Restore(entry, version); err != nil {
        e.logger.Append(RollbackLogEntry{
            LearningID: entry.ID, Trigger: trigger.String(),
            RestorerResult: "failed", RestorerError: err.Error(),
            VersionRef: &version,
        })
        return err
    }

    // 3. REFLECT MarkRolledBack
    now := e.clock.Now()
    if err := e.memory.MarkRolledBack(entry.ID, trigger.String(), now); err != nil {
        // LoRA 는 복원되었지만 상태 업데이트 실패 — 로그 기록 후 에러 전파
        e.logger.Append(RollbackLogEntry{
            LearningID: entry.ID, Trigger: trigger.String(),
            RestorerResult: "ok",
            RestorerError: "mark_rolled_back_failed: " + err.Error(),
        })
        return err
    }

    // 4. 성공 로그
    e.logger.Append(RollbackLogEntry{
        Timestamp: now, LearningID: entry.ID, Trigger: trigger.String(),
        RestorerResult: "ok", VersionRef: &version,
    })
    return nil
}
```

### 7.4 RegressionWatcher Daemon 통합

- `genied` 데몬 부트스트랩 시 `watcher.Start(ctx)` 호출
- 내부 goroutine은 `time.NewTicker(interval)`로 Tick
- graceful shutdown: `ctx.Done()` 감지 시 진행 중 Tick 완료 후 종료
- 재시작 복원: 이전 Tick 기록이 없어도 안전 (멱등적)

### 7.5 Restorer 위임 계약

LoRA 파일 교체는 LORA-001이 `Restorer` 인터페이스를 구현. 본 SPEC은 Restorer의 구체 구현을 강제하지 않는다. CLAUDE.md evolvable zone 복원은 별도 Restorer 구현체(향후 SPEC).

복수 Restorer가 필요하면 `CompositeRestorer`로 순차 실행 (실패 시 전체 롤백은 어려움 — 이는 LORA-001의 트랜잭션 설계와 연계된 문제).

### 7.6 테스트 전략 (TDD)

- Unit: `detector_test.go`, `executor_test.go`, `watcher_test.go`, `logger_test.go`
- Boundary: `ScoreDrop=0.09/0.10/0.11` 테이블-driven
- `EvaluatedSessions=4/5/6` 테이블-driven
- Mock Restorer로 성공/실패/`ErrNoPreviousVersion` 3 시나리오
- Mock HookEmitter로 이벤트 발행 검증
- 통합: `DefaultRegressionWatcher.Tick`이 3개 Graduated entry에 대해 올바른 수량의 Rollback/Deferred 카운트 생성하는지
- Race: `Tick` 실행 중 새 Graduated entry 추가 시 다음 Tick에서 처리되는지

---

## 8. 리스크 & 완화

| 리스크 | 영향 | 완화 |
|-------|-----|------|
| Baseline 7일 윈도우 부족 (승격 직후 데이터 적음) | 오탐 (false positive rollback) | `min_sessions_for_detection=5`로 최소 보장, 부족 시 defer |
| Restorer 실패 후 LearningEntry와 실제 상태 불일치 | 롤백 실패 상태에서 계속 적용된 상태 | 실패 시 `MarkRolledBack` 호출하지 않고 에러 전파, 사용자에게 HOOK으로 수동 개입 요청 |
| Tick이 너무 자주 실행되어 성능 저하 | Daemon CPU 낭비 | 기본 24h interval, 구성으로 조정 가능, 평가 대상 entry가 0이면 즉시 종료 |
| 동시 Tick 중복 실행 | 중복 롤백 시도 | REFLECT 상태가 `RolledBack`이면 `ErrAlreadyRolledBack` 반환 (REQ-ROLLBACK-010) |
| 30일 staleness 동안 새 학습도 차단될까 우려 | 동일 키에만 적용되므로 다른 영역은 영향 없음 | REFLECT-001 REQ-REFLECT-015가 키 단위(`(Category, TargetComponent, ZoneID)`) 스코프로만 차단 |

---

## 9. 참조 (References)

- `.moai/project/learning-engine.md` §6.3 (Evaluation Pipeline 다이어그램), §2.3 RollbackPolicy
- `.claude/rules/agency/constitution.md` §15 (FROZEN, 30일 staleness 원천)
- SPEC-GENIE-REFLECT-001 (MarkRolledBack 수신자, RollbackDeadline 설정)
- SPEC-GENIE-SAFETY-001 (CanaryCheck 사전 예방과의 역할 분리)
- SPEC-GENIE-TRAJECTORY-001 (session score 데이터 소스)
- SPEC-GENIE-INSIGHTS-001 (satisfaction score 계산)
- SPEC-GENIE-LORA-001 (Restorer 주요 구현체)
- SPEC-GENIE-HOOK-001 (HookEventRollbackManualIntervention)
