# Research — SPEC-GENIE-ROLLBACK-001 (Regression 자동 감지 + Rollback)

> **작성일**: 2026-04-21
> **대상 SPEC**: SPEC-GENIE-ROLLBACK-001
> **목적**: agency constitution §15 및 learning-engine §6.3 계승 근거, REFLECT/SAFETY와의 계약 확정, TDD 테스트 전략.

---

## 1. 원천 문서 정리

### 1.1 agency/constitution §15 전문 인용 (FROZEN)

> "If a graduated learning causes regression:
> - Automatic rollback triggered when next project score drops > 0.10
> - Reverted change logged in .agency/evolution/rollbacks.log
> - Learning status changed to 'rolled-back'
> - Learning cannot be re-proposed for 30 days (staleness_window_days)"

GENIE는 "next project score" → "next session score"로 치환하지만 **0.10 임계값과 30일 staleness는 그대로 계승**한다.

### 1.2 learning-engine.md §6.3 직접 인용

```go
type EvaluationStatus int
const (
    StatusOK       EvaluationStatus = iota  // 0.85+ or 회귀 < 0.05
    StatusWarning                            // 0.75-0.85 또는 회귀 0.05-0.10
    StatusRollback                           // < 0.75 또는 회귀 > 0.10
)

func EvaluateNewLoRA(oldLoRA, newLoRA, testSet) EvaluationResult {
    oldAvg := mean(oldScores)
    newAvg := mean(newScores)
    regression := (newAvg - oldAvg) / oldAvg

    if newAvg < 0.75 {
        result.Status = StatusRollback
    } else if regression > 0.10 {
        result.Status = StatusWarning
    }
}
```

**GENIE 적용**: learning-engine은 `newAvg < 0.75` OR `regression > 0.10` 두 조건 중 하나를 warning/rollback으로 나눠 다룬다. **본 SPEC은 단순화하여 "regression > 0.10 즉시 rollback"만 구현**하고, warning 레벨은 초기 버전에서 제외한다. 이유:

1. Warning → 사용자 알림만 하는 중간 상태가 GENIE에 아직 불필요 (사용자가 개입할 UI가 제한적)
2. Warning 로직 추가는 별도 SPEC(향후)에서 고도화
3. `newAvg < 0.75` 절대값 조건은 baseline 의존적이므로 신뢰도 낮음 — 상대 차이(regression ratio)만 사용

### 1.3 learning-engine.md §2.3 RollbackPolicy 인용

```go
type RollbackPolicy struct {
    Enabled bool
    Duration time.Duration  // 기본 30일

    RegressionThreshold float64  // 만족도 0.10 이상 하락

    CanUserInitiate bool
    CanUserSchedule bool
}
```

본 SPEC은 `RegressionThreshold=0.10`, `Duration=30*24h`를 기본값으로 채택. `CanUserInitiate=true` (사용자가 CLI/UI로 수동 롤백 요청 가능, 단 UI 자체는 본 SPEC 외).

---

## 2. 핵심 설계 결정과 근거

### 2.1 "사후 감지" 책임 범위를 ROLLBACK이 전담하는 이유

SAFETY-001의 CanaryCheck는 **승격 전** 예방 검증이다. 그러나 다음 한계가 있다:

1. Canary는 과거 3 session 기반 — 미래 사용자 행동 변화를 예측할 수 없음
2. 사용자 선호도가 시간이 지나며 변할 때(예: 출근 모드 vs 휴가 모드) 과거 trajectory로는 감지 불가
3. Shadow evaluation 이 완벽한 simulation이 아님

따라서 **승격 후 실제 만족도**를 지속 관찰하는 별도 layer가 필수. 이것이 ROLLBACK-001의 존재 이유.

### 2.2 Baseline 윈도우 7일 선택 근거

대안 A: 1일. → 단일 일은 특이값(outlier)에 취약.
대안 B: 30일. → 계절성 변화(학습 주기)를 놓침. 또한 승격 시점으로부터 30일 이전 데이터는 이미 오래되어 현재 사용자와 다를 수 있음.
**결정**: 7일. 일주일 내 평균은 요일 효과(주중/주말)를 포괄하면서도 최근 상태를 반영.

### 2.3 최소 5 세션 미달 시 defer하는 근거

세션 수가 적을 때 평균은 높은 분산을 가진다. 5개 미만에서 판정하면 우연한 나쁜 세션 1개로 롤백이 트리거될 위험. learning-engine.md의 "data quality >= 0.75" 기준과도 일치.

5개는 agency constitution §5 Canary의 "last 3 projects"보다 크다 — 이유는 **승격 후 복구**는 되돌릴 수 있지만(재승격 가능), 예방은 엄격할수록 안전하기 때문. 즉 사후 감지는 data 요구사항을 높이고 사전 예방은 낮춘다.

### 2.4 Restorer 인터페이스 위임 설계

대안 A: ROLLBACK이 LoRA 파일을 직접 교체. → 거부 이유: LORA-001의 내부 구조(atomic swap, checkpoint, shadow adapter)를 ROLLBACK이 알면 안 됨. 결합도 ↑.
대안 B: ROLLBACK이 REFLECT의 `ProposedChange.Before`를 그대로 파일에 쓰기. → 거부 이유: LoRA 바이너리는 `Before` 문자열로 표현 불가능.
**결정**: `Restorer` 인터페이스로 위임. 구현체는 LORA-001 또는 파일 기반 Restorer.

### 2.5 `MarkRolledBack`을 가장 마지막에 호출하는 이유

순서:
1. `Restorer.PreviousVersion` — 이전 버전이 존재하는지 확인
2. `Restorer.Restore` — 실제 복원
3. `MarkRolledBack` — 상태 전이

**2와 3이 바뀌면** REFLECT 상태는 `RolledBack`인데 실제 LoRA는 새 버전인 불일치 상태가 발생. 이 불일치는 다음 재승격 시도 시 혼란 야기. 따라서 **실제 복원이 성공한 뒤에만 상태 전이**.

다만 **3이 실패하고 2가 성공**한 경우(원자성 부재)도 처리 필요. 이 경우 로그에 `RestorerResult="ok"` + `RestorerError="mark_rolled_back_failed"`로 기록하여 후속 수동 복구 가능. 이는 Restore가 거의 항상 성공하는 LoRA 환경에서 현실적 타협.

### 2.6 Staleness 30일을 REFLECT가 관리하는 이유

`RollbackDeadline`은 REFLECT `LearningEntry`의 필드다. ROLLBACK은 `MarkRolledBack(id, trigger, at)` 호출 시 `at` 인자만 전달하고, REFLECT 내부에서 `RollbackDeadline = at + 30d`를 계산. 이유:

- 단일 책임: deadline 관리와 상태 전이가 같은 컴포넌트(REFLECT)에 속해야 일관성 유지
- ROLLBACK은 감지와 실행에만 집중
- 사용자 구성 `rollback.staleness_window_days`가 있다면 REFLECT가 참조

---

## 3. 회귀 감지 알고리즘 상세

### 3.1 session score 데이터 소스

TRAJECTORY-001이 저장하는 trajectory JSON-L에 INSIGHTS-001이 계산한 `satisfaction_score` 필드가 포함되는 것을 전제한다. 현재 Phase 4 SPEC들이 미작성 상태지만, 본 SPEC은 다음 최소 계약만 가정:

```go
type TrajectoryStore interface {
    ListByTimeRange(start, end time.Time) ([]TrajectorySession, error)
}

type TrajectorySession struct {
    SessionID          string
    StartedAt, EndedAt time.Time
    SatisfactionScore  float64 // [0, 1]
    Completed          bool
}
```

score 산출 알고리즘은 INSIGHTS-001에 전적으로 위임. ROLLBACK은 숫자만 사용.

### 3.2 baseline vs current 비교

```
baseline_window = [entry.ApprovedAt - 7d, entry.ApprovedAt)
current_window  = [entry.ApprovedAt, now)

baseline_sessions = filter(completed=true, start in baseline_window)
current_sessions  = filter(completed=true, start in current_window)

if len(current_sessions) < min_sessions_for_detection:
    return Detected=false, reason="insufficient_data"

baseline = mean([s.SatisfactionScore for s in baseline_sessions])
current  = mean([s.SatisfactionScore for s in current_sessions])

score_drop = baseline - current
detected = score_drop > threshold  # default 0.10
```

### 3.3 Edge case 처리

- `baseline_sessions`가 비어있을 때: 이 entry는 애초에 Graduated 자격 없음 (REFLECT 승격 과정에서 충분한 Evidence 필요). 안전장치로 `Detected=false, reason="no_baseline"` 반환.
- 분산이 매우 큰 경우: 본 SPEC은 mean만 사용. 향후 SPEC-GENIE-ROLLBACK-002에서 t-test/Welch's test 등 통계적 유의성 검정 추가 고려.

---

## 4. Restorer 구현체 예상 카탈로그

본 SPEC은 인터페이스만 정의하지만, 다음이 예상 구현체:

### 4.1 LoRARestorer (by LORA-001)

```go
type LoRARestorer struct {
    versionStore lora.VersionStore
    loader       lora.Loader
}

func (r *LoRARestorer) PreviousVersion(entry reflect.LearningEntry) (rollback.VersionRef, error) {
    versions, err := r.versionStore.ListByComponent(entry.TargetComponent)
    if err != nil { return rollback.VersionRef{}, err }

    // Graduated 시점 직전 버전 찾기
    for i := len(versions) - 1; i >= 0; i-- {
        if versions[i].CreatedAt.Before(*entry.ApprovedAt) {
            return rollback.VersionRef{Kind: "lora", Ref: versions[i].ID}, nil
        }
    }
    return rollback.VersionRef{}, rollback.ErrNoPreviousVersion
}

func (r *LoRARestorer) Restore(entry reflect.LearningEntry, v rollback.VersionRef) error {
    return r.loader.HotSwap(v.Ref)
}
```

### 4.2 FileRestorer (CLAUDE.md evolvable zone)

```go
type FileRestorer struct {
    backupDir string // .moai/evolution/backups/
}

// 승격 전 파일을 미리 백업해 두는 책임은 SAFETY-001의 HumanOversight 단계
// (승격 승인 시 현재 파일을 backupDir/<learning_id>.bak로 복사)
// ROLLBACK은 백업 존재 여부 확인 후 파일 복사로 복원.
```

### 4.3 CompositeRestorer

여러 Restorer를 순차 실행. 실패 시 앞서 성공한 것을 되돌리는 로직은 LORA-001의 트랜잭션 설계와 연계 필요.

---

## 5. TDD 테스트 전략

### 5.1 RED 단계 테스트 우선순위

1. **Boundary**:
   - `TestDetector_drop_0_09_not_detected`
   - `TestDetector_drop_0_10_not_detected` (0.10은 임계값 초과 아님 — strict `>` 사용)
   - `TestDetector_drop_0_11_detected`

2. **Minimum sessions**:
   - `TestDetector_4_sessions_deferred`
   - `TestDetector_5_sessions_evaluated`

3. **Executor 실행 순서**:
   - `TestExecutor_calls_in_order_PreviousVersion_Restore_MarkRolledBack`
   - `TestExecutor_restore_error_stops_before_mark_rolled_back`
   - `TestExecutor_no_previous_version_emits_hook_event`

4. **중복 호출**:
   - `TestExecutor_already_rolled_back_returns_ErrAlreadyRolledBack`
   - `TestExecutor_already_rolled_back_does_not_call_restorer`

5. **Logger**:
   - `TestLogger_append_valid_jsonl`
   - `TestLogger_append_on_failure_includes_error`
   - `TestLogger_concurrent_appends_safe` (파일 lock 또는 channel-based writer)

6. **Watcher 통합**:
   - `TestWatcher_tick_evaluates_all_graduated_entries`
   - `TestWatcher_tick_skips_non_graduated`
   - `TestWatcher_stop_drains_in_flight_tick`

### 5.2 Table-driven 예시

```go
func TestDetector_boundary(t *testing.T) {
    cases := []struct {
        name     string
        baseline float64
        current  float64
        sessions int
        want     bool // Detected
    }{
        {"drop 0.09 passes", 0.85, 0.76, 10, false},
        {"drop 0.10 passes (strict >)", 0.85, 0.75, 10, false},
        {"drop 0.11 rolls back", 0.85, 0.74, 10, true},
        {"insufficient sessions", 0.85, 0.50, 4, false},
    }
    // ...
}
```

### 5.3 Mock 설계

```go
type mockRestorer struct {
    previousErr  error
    restoreErr   error
    restoreCalls int
    prevCalls    int
    lastVersion  rollback.VersionRef
}
```

### 5.4 Property-based

- 속성: "동일 entry에 대해 Execute를 N번 호출하면 Restorer.Restore는 최대 1회만 호출된다"
- 속성: "Tick 실행 중 새 Graduated entry 추가 시 해당 entry는 다음 Tick에서만 평가된다"
- 속성: "Rollback log의 모든 line은 독립적으로 파싱 가능한 JSON이다"

### 5.5 Race detector

- `TestWatcher_tick_and_reflect_mutation_race`: Tick 진행 중 REFLECT에서 새 Graduated entry 등록 시 race 없는지
- `TestLogger_concurrent_append`: 여러 goroutine이 동시에 Append 호출할 때

---

## 6. 오픈 질문

1. **Warning 레벨 도입 시기**: learning-engine §6.3의 `StatusWarning`(0.05~0.10 회귀)은 본 SPEC 범위 외. SPEC-GENIE-ROLLBACK-002에서 사용자 알림 채널 정의 시 도입.
2. **Baseline 계산의 통계적 유의성 검정**: t-test 등 고도화는 Phase 6 이후
3. **Restorer 실패 시 재시도 정책**: 현재는 즉시 실패. 향후 exponential backoff 재시도 옵션 고려
4. **Rollback 이후 재승격 경로**: 30일 staleness 후 동일 entry가 자연스럽게 다시 `Observation`으로 진입 가능해야 하는데, Evidence 이력 유지? 재출발? → REFLECT-001에서 결정되나 초기 버전은 "재출발"(새 entry로 간주)
5. **Multi-learning rollback 동시성**: 여러 entry가 동시에 회귀 트리거될 경우, LoRA 교체가 직렬화되어야 하는지 병렬 가능한지 → LORA-001의 atomic swap 구현에 따라 다름

---

## 7. 구현 순서 (TDD 루프)

| 순서 | 테스트 파일 | 대상 코드 |
|-----|-----------|---------|
| 1 | `types_test.go` | RegressionResult, RollbackTrigger, VersionRef |
| 2 | `detector_test.go` | 회귀 감지 boundary + insufficient sessions |
| 3 | `logger_test.go` | JSON-L append 검증 |
| 4 | `executor_test.go` | 3단계 실행 순서 + 실패 경로 |
| 5 | `watcher_test.go` | Daemon Tick 통합 |
| 6 | `integration_test.go` | 전체 파이프라인 (mock REFLECT, mock Restorer, real Logger) |

---

## 8. 성능 고려사항

- Tick 비용 = O(|Graduated entries|) × baseline/current score 조회
- Graduated 최대 수는 활성 진화 한도에 따라 다르나 수십 ~ 수백 범위
- TRAJECTORY store가 indexed일 때 각 entry당 조회 < 10ms → 100 entry = 1s 이내 Tick 완료
- Tick interval 기본 24h이므로 성능 병목 없음

---

## 9. 참조 인덱스

- `.moai/project/learning-engine.md` §6.3 (회귀 감지 파이프라인 원천), §2.3 (RollbackPolicy)
- `.claude/rules/agency/constitution.md` §15 (30일 staleness FROZEN 원천)
- SPEC-GENIE-REFLECT-001 (MarkRolledBack, RollbackDeadline 관리자)
- SPEC-GENIE-SAFETY-001 (CanaryCheck — 사전 예방, ROLLBACK과 역할 분리)
- SPEC-GENIE-TRAJECTORY-001 (session 데이터 소스)
- SPEC-GENIE-INSIGHTS-001 (satisfaction score 산출)
- SPEC-GENIE-LORA-001 (Restorer 주요 구현체)
- SPEC-GENIE-HOOK-001 (HookEventRollbackManualIntervention 수신자)

---

Version: 0.1.0
Created: 2026-04-21
Status: Research complete, ready for RED phase test authoring
