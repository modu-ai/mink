# Research — SPEC-GOOSE-REFLECT-001 (5단계 승격 파이프라인)

> **작성일**: 2026-04-21
> **대상 SPEC**: SPEC-GOOSE-REFLECT-001
> **목적**: MoAI SPEC-REFLECT-001 계승 근거, Hermes 파이프라인 연계, TDD 테스트 전략 확정.

---

## 1. 원천 문서 정리

### 1.1 MoAI-ADK-Go `SPEC-REFLECT-001` 계승

MoAI-ADK-Go는 이미 프로젝트 단위 학습(`.agency/learnings/*.md` + `.claude/rules/agency/constitution.md`)에서 다음 5-tier state machine을 구현하고 있다:

```
Observation (1회) → Heuristic (3회+) → Rule (5회+, conf≥0.80)
                 → HighConfidence (10회+, conf≥0.95)
                 → Graduated (승인 완료)
```

격리 경로:
- `Rejected` (사용자 거부)
- `Archived` (1년 미사용)
- `RolledBack` (회귀 감지)
- `AntiPattern` (critical failure 1회 즉시)

**GOOSE 차이점**: 대상이 프로젝트 스킬/에이전트가 아니라 **개인 사용자 LoRA adapter, preference vector, user-style skill, CLAUDE.md evolvable zone**이다. 수치 임계값(1/3/5/10, 0.80/0.95, 50 active, 30일 staleness)은 동일하게 계승한다.

### 1.2 learning-engine.md §2 직접 인용

```yaml
# learning-engine.md §2.2 LearningEntry 구조
LearningEntry:
  ID: LEARN-YYYYMMDD-NNN
  Category: style | pattern | preference | skill
  TargetComponent: <skill ID | agent ID | LoRA ID>
  ZoneID: <evolvable zone ID>
  Status: <5단계 중 하나>
  Observations: int (1 / 3 / 5 / 10)
  Confidence: float64 [0, 1]
  Evidence: []EvidenceEntry
  ProposedChange: ProposedChange
  ApprovalStatus: pending | approved | rejected
  ApprovedAt: timestamp
  ApprovedBy: "user@example.com"
  CreatedAt: timestamp
  UpdatedAt: timestamp
  RejectedAt: timestamp   # 거부 시
  RollbackDeadline: timestamp  # 30일 쿨다운
```

**Go 매핑 결정**: `ApprovalStatus` 필드는 중복 정보(`Status` 필드로 추론 가능)이므로 `LearningEntry` 본체에서 제거하고, 대신 `ApprovedAt *time.Time`, `ApprovedBy string`을 유지한다. `RejectedAt`도 `RollbackDeadline`으로 통합 관리한다.

### 1.3 hermes-learning.md §11-12 (SPEC-REFLECT 연계)

```
Layer 1: Trajectory 수집 → internal/learning/trajectory
Layer 2: 압축 → internal/learning/compressor
Layer 3: Insights 추출 → internal/evolve/{reflect, safety}
Layer 4: Memory 저장 → internal/memory/{provider, manager, sqlite}
Layer 5: Skill/Prompt 자동 진화 → internal/skill/{catalog, command, runtime}
```

REFLECT는 **Layer 3 중심**이며, INSIGHTS (Layer 3 상류)와 MEMORY (Layer 4)의 **게이트** 역할을 한다. 즉:

- 입력: INSIGHTS-001이 생성한 `InsightCandidate`
- 출력: `LearningEntry` (MEMORY-001에 영속)
- 부작용: `TransitionReport` 통해 SAFETY-001에 HighConfidence 이벤트 통지

### 1.4 agency/constitution §6 Learnings Pipeline 직접 인용

```
| Observations | Classification | Action |
|-------------|---------------|--------|
| 1x | Observation | Logged, no action taken |
| 3x | Heuristic | Promoted to heuristic, may influence suggestions |
| 5x | Rule | Eligible for graduation to evolvable zone |
| 10x | High-confidence | Auto-proposed for evolution (still needs approval) |
| 1x (critical failure) | Anti-Pattern | Immediately flagged, blocks similar patterns |
```

본 SPEC은 위 표의 임계값을 그대로 계승한다. `.agency/constitution §6` 표는 REFLECT 구현의 정본이다.

---

## 2. 핵심 설계 결정과 근거

### 2.1 `AntiPattern`을 별도 상태로 둔 이유

대안 1: `Archived`의 sub-category로 저장. → 거부 이유: `AntiPattern`은 **재발 방지 차단**이 목적인데 `Archived`는 재제안 가능한 상태이므로 의미론이 충돌한다.
대안 2: `Rejected`로 표시. → 거부 이유: `Rejected`는 사용자 거부이고 30일 경과 후 재제안되지만, `AntiPattern`은 영구 격리다.

**결정**: 별도 상태 `AntiPattern`으로 두고 FROZEN 속성을 부여. 재분류는 사용자 개입(SAFETY-001 HumanOversight)만 가능.

### 2.2 SafetyToken 도입 이유

learning-engine.md는 `ApprovalStatus`를 단순 enum으로 표현하지만, **REFLECT가 SAFETY의 허가 없이 단독으로 Graduated로 승격시키는 경로**가 있으면 5-layer safety가 무의미해진다.

**결정**: `MarkGraduated`는 `SafetyToken`을 필수 인자로 받는다. 토큰의 발급/검증은 SAFETY-001 책임이고 REFLECT는 `Verify` 결과만 신뢰한다.

### 2.3 Confidence 계산 floor=3 도입

대안 1: `Σ(SignalStrength) / |Evidence|`. → 거부 이유: Evidence가 1개일 때 `SignalStrength=1.0`이면 즉시 `Confidence=1.0`이 되어 Rule 승격이 3회 관찰 후 바로 일어난다. overconfidence.
대안 2: `Σ(SignalStrength) / 10` (10으로 고정 분모). → 거부 이유: 관찰 수와 무관하게 분모가 고정되면 10회 이상 관찰이 쌓여도 최대값이 제한되어 HighConfidence 도달이 비현실적이다.

**결정**: `Σ(SignalStrength) / max(|Evidence|, 3)`. 초기 3회까지는 분모 3으로 고정(overconfidence 방지), 이후에는 관찰 수와 비례하여 평균.

### 2.4 용량 한도 50의 근거

agency/constitution §5 Layer 4 Rate Limiter:
> "No more than 50 active learnings at any time (max_active_learnings). Older learnings are archived when the limit is reached."

GOOSE는 개인 사용자 환경에서 프로젝트보다 더 다양한 선호도가 쌓일 가능성이 있으나, 50개 이상 동시 pending은 사용자가 검토할 수 있는 인지 한도를 초과한다. MoAI 수치 그대로 계승.

### 2.5 30일 staleness window

agency/constitution §15:
> "Learning cannot be re-proposed for 30 days (staleness_window_days)"

사용자가 거부했거나 롤백된 학습을 즉시 재제안하면 UX가 악화된다. 30일은 계절성 패턴(월간 루틴)을 넘지 않으면서도 일시적 거부 기분을 존중하는 균형점.

### 2.6 `confidence_decay_enabled`를 Optional로 둔 이유

지수 감쇠(decay=0.95/day)는 **변화하는 사용자 선호도를 빠르게 반영**하지만, 동시에 **안정적인 장기 선호도가 충분히 쌓일 기회를 박탈**한다. 초기 구현에서는 기본값 `false`로 두고, 사용자가 명시적으로 활성화할 수 있게 한다.

---

## 3. Go 패키지 구조 상세

```
internal/evolve/reflect/
├── entry.go          # 타입 정의 (public API)
├── state.go          # PromotionStatus 전이 테이블
├── promoter.go       # Promoter 인터페이스 + DefaultPromoter
├── confidence.go     # Confidence 계산 (floor, decay)
├── capacity.go       # 50 active LRU archive
├── staleness.go      # 30일 staleness 처리 (Tick 서브루틴)
├── errors.go         # sentinel errors
├── internal/         # 비공개 유틸
│   ├── lru.go
│   └── clock.go      # testability를 위한 Clock 인터페이스
└── testdata/
    ├── promotion_cases.yaml
    └── antipattern_cases.yaml
```

### 3.1 Clock 인터페이스 (테스트 용이성)

```go
type Clock interface {
    Now() time.Time
}

type realClock struct{}
func (realClock) Now() time.Time { return time.Now() }
```

테스트에서는 `fakeClock`을 주입하여 staleness 시나리오를 결정적으로 검증.

---

## 4. TDD 테스트 전략

### 4.1 RED 단계 테스트 우선순위 (실패 테스트 먼저 작성)

1. **Promotion threshold boundary**:
   - `TestIngest_Observation_to_Heuristic_at_3rd_call`
   - `TestIngest_Heuristic_to_Rule_requires_conf_0_80`
   - `TestIngest_Rule_to_HighConfidence_requires_conf_0_95`

2. **Anti-Pattern 격리**:
   - `TestIngestCritical_locks_immediately`
   - `TestMarkGraduated_fails_on_AntiPattern`

3. **용량 관리**:
   - `TestIngest_evicts_oldest_Observation_at_cap_50`
   - `TestIngest_does_not_evict_when_cap_not_reached`

4. **Staleness**:
   - `TestTick_archives_RolledBack_after_30_days`
   - `TestIngest_rejected_within_rollback_deadline`

5. **승인 토큰**:
   - `TestMarkGraduated_with_invalid_token_returns_error`
   - `TestMarkGraduated_with_expired_token_returns_error`
   - `TestMarkGraduated_with_valid_token_transitions_to_Graduated`

6. **Confidence 계산**:
   - `TestConfidence_floor_3_prevents_overconfidence`
   - `TestConfidence_decay_reduces_old_evidence_weight`
   - `TestConfidence_clamps_to_unit_interval`

7. **영속화**:
   - `TestIngest_calls_SyncLearningEntry`
   - `TestMarkGraduated_calls_SyncLearningEntry_with_updated_status`

8. **Concurrency**:
   - `TestIngest_race_free_under_concurrent_calls`
   - `TestTick_and_Ingest_interleave_safely`

### 4.2 Table-driven 테스트 예시 (boundary)

```go
// confidence_test.go
func TestConfidence_boundary_0_80(t *testing.T) {
    cases := []struct {
        name       string
        evidence   []Evidence
        wantStatus PromotionStatus
    }{
        {"0.79 stays Heuristic", evidenceWithConf(0.79, 5), StatusHeuristic},
        {"0.80 promotes to Rule", evidenceWithConf(0.80, 5), StatusRule},
        {"0.81 promotes to Rule", evidenceWithConf(0.81, 5), StatusRule},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            entry := &LearningEntry{Status: StatusHeuristic, Evidence: tc.evidence, Observations: 5}
            gotStatus := evaluateTransition(entry)
            if gotStatus != tc.wantStatus {
                t.Errorf("want %v, got %v", tc.wantStatus, gotStatus)
            }
        })
    }
}
```

### 4.3 Property-based 테스트

`testing/quick` 또는 `github.com/leanovate/gopter`:

- 속성: "동일 키로 N회 Ingest 후 `Observations == N`"
- 속성: "임의의 Evidence 순서에서 최종 `Confidence`는 순서 불변"
- 속성: "`Tick`을 호출해도 이미 `Graduated` 상태인 entry는 변경되지 않는다"

### 4.4 Race detector CI 강제

`.github/workflows/ci.yml`에 `go test -race -count=3 ./internal/evolve/reflect/...`을 필수로 포함. race 발견 시 CI 실패.

### 4.5 Golden test (상태 전이 기록)

`testdata/promotion_cases.yaml`:

```yaml
- name: happy_path
  ingests:
    - { sessionID: s1, signal: 0.9, timestamp: 2026-04-21T09:00 }
    - { sessionID: s2, signal: 0.9, timestamp: 2026-04-22T09:00 }
    - { sessionID: s3, signal: 0.9, timestamp: 2026-04-23T09:00 }
  expectedTransitions:
    - { at: call_3, from: Observation, to: Heuristic }
  finalStatus: Heuristic
  finalObservations: 3
```

---

## 5. 성능 고려사항

- `Promoter` 단일 인스턴스, 내부 `map[string]*LearningEntry` (key = `fmt.Sprintf("%d/%s/%s", cat, target, zone)`).
- 50 active 한도이므로 `map` 크기는 최대 O(50) + archived 수. Tick 시 전체 순회해도 수천 개 이하면 밀리초 단위.
- 동시성: RWMutex로 read 많이, write 적게. `ListByStatus`는 read lock으로 카피 반환.
- MEMORY Sync는 goroutine으로 async 호출하되, 실패 시 다음 Tick에서 재시도.

---

## 6. 오픈 질문

1. **Evidence 최대 보존 개수**: entry당 Evidence가 무제한이면 메모리 누수. 최근 100개로 제한? (`MaxEvidencePerEntry`)
2. **SafetyToken 포맷**: HMAC-SHA256? Ed25519? SAFETY-001에서 결정하되 REFLECT는 `interface { Verify() error }`만 의존하는 게 안전.
3. **ZoneID naming convention**: CLAUDE.md block은 `goose:evolvable-start id="..."`, LoRA는 `lora:user:{userID}:v{n}`, skill은 `skill:{skillName}`. 본 SPEC은 string으로만 취급하고 convention은 consumer SPEC에서 확정.
4. **TransitionReport 소비 메커니즘**: SAFETY-001이 polling? channel? event bus? → SAFETY-001 설계 시 결정, REFLECT는 `Tick`의 반환값을 제공하면 충분.
5. **Archived entry 복구 경로**: 90일 후 자동 재제안? 사용자가 명시적으로 요청? → 초기 버전은 "요청 기반 복구"만 지원.

---

## 7. 구현 순서 (TDD 루프)

| 순서 | 테스트 파일 | 대상 코드 |
|-----|-----------|---------|
| 1 | `entry_test.go` | `entry.go` (타입 + enum String 메서드) |
| 2 | `state_test.go` | `state.go` (전이 테이블) |
| 3 | `confidence_test.go` | `confidence.go` (floor, clamp) |
| 4 | `promoter_ingest_test.go` | `promoter.go` Ingest 기본 경로 |
| 5 | `promoter_critical_test.go` | IngestCritical + AntiPattern 격리 |
| 6 | `capacity_test.go` | LRU archive |
| 7 | `staleness_test.go` | Tick + 30일 처리 |
| 8 | `graduate_test.go` | MarkGraduated + SafetyToken 검증 |
| 9 | `concurrency_test.go` | race detector 시나리오 |
| 10 | `memory_sync_test.go` | MemoryProvider hand-off 통합 |

각 단계: RED (실패 테스트) → GREEN (최소 구현) → REFACTOR → 다음 단계.

---

## 8. 참조 문서 인덱스

- `.moai/project/learning-engine.md` §2 (원천)
- `.moai/project/research/hermes-learning.md` §11-12 (5-layer 연계)
- `.claude/rules/agency/constitution.md` §6-7 (learning pipeline, graduation protocol)
- `.moai/config/sections/quality.yaml` (85%+ coverage 요구)
- `.claude/rules/moai/workflow/workflow-modes.md` (TDD RED-GREEN-REFACTOR)
- SPEC-GOOSE-INSIGHTS-001 (상류 consumer)
- SPEC-GOOSE-MEMORY-001 (저장 백엔드)
- SPEC-GOOSE-SAFETY-001 (SafetyToken 발급자, HumanOversight gate)
- SPEC-GOOSE-ROLLBACK-001 (MarkRolledBack 호출자)

---

Version: 0.1.0
Created: 2026-04-21
Status: Research complete, ready for RED phase test authoring
