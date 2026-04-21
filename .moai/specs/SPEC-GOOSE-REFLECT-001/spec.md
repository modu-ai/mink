---
id: SPEC-GOOSE-REFLECT-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P1
issue_number: null
phase: 5
size: 대(L)
lifecycle: spec-anchored
---

# SPEC-GOOSE-REFLECT-001 — 5단계 승격 파이프라인 (Observation → Graduated)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (MoAI SPEC-REFLECT-001 계승 + learning-engine §2 + hermes-learning §11 근거) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE의 자기진화 루프에서 **INSIGHTS-001이 추출한 관찰 후보**와 **MEMORY-001에 저장된 학습 이력**을 입력으로 받아, 관찰(Observation)을 순차적으로 Heuristic → Rule → HighConfidence → Graduated로 승격하거나 Anti-Pattern/Rolled-back/Archived로 격리하는 **상태 머신(state machine)**을 정의한다.

본 SPEC은 MoAI-ADK-Go의 `SPEC-REFLECT-001` (5-tier promotion)을 Go 코어로 계승하되, 대상 영역이 **프로젝트 스킬/에이전트가 아니라 사용자 개인의 LoRA adapter, preference vector, user-style skill, CLAUDE.md evolvable zone**이라는 차이를 반영한다. 수치(1/3/5/10 관찰, 0.80/0.95 신뢰도, 최대 50 active, 30일 rollback 쿨다운)는 MoAI와 동일하다.

본 SPEC은 상태 전이 규칙과 `LearningEntry` 영속화만 책임지며, 승격 결과를 실제 사용자 상태에 적용하는 작업(LoRA 교체, skill body 수정 등)은 `SPEC-GOOSE-SAFETY-001`의 Human Oversight gate와 하위 소비자 SPEC에 위임한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- `SPEC-GOOSE-INSIGHTS-001`이 Pattern/Preference/Error/Opportunity 4종 insight을 추출하지만, **관찰 한 번만으로 사용자 상태를 변경하면 위험하다** (오탐, 일시적 기분, 세션 특수 맥락). 이를 걸러내는 승격 시스템이 없으면 INSIGHTS의 출력이 실제 진화로 이어질 수 없다.
- `SPEC-GOOSE-MEMORY-001`이 학습 결과를 영속화하지만 **"어떤 상태로 저장할지"에 대한 의미론을 MEMORY는 알지 못한다.** REFLECT가 상태 계약을 독점한다.
- 본 SPEC이 없으면 Phase 5의 SAFETY-001과 ROLLBACK-001도 건설될 수 없다 — 두 SPEC 모두 REFLECT의 state transition 이벤트에 걸친다.

### 2.2 상속 자산 (패턴만 계승)

- **MoAI-ADK-Go SPEC-REFLECT-001**: 5-tier state machine (Observation → Heuristic → Rule → HighConfidence → Graduated), 관찰 임계값(1/3/5/10), 신뢰도 임계값(0.80/0.95), Anti-Pattern 즉시 격리 규칙. 본 SPEC은 수치와 구조를 그대로 계승한다.
- **learning-engine.md §2.1~2.2**: `LearningEntry` 스키마 전체 (ID, Category, TargetComponent, ZoneID, Status, Observations, Confidence, Evidence[], ProposedChange, ApprovalStatus, RollbackDeadline). 본 SPEC은 Go 구조체로 1:1 매핑한다.
- **hermes-learning.md §11**: Trajectory → Insights → Memory → Skill 파이프라인에서 REFLECT가 "Insights와 Memory 사이의 게이트" 역할을 한다.
- **agency/constitution §6 Learnings Pipeline**: Observations/Heuristic/Rule/HighConfidence 명명법과 Anti-Pattern 즉시 FROZEN 규칙의 원형. 본 SPEC이 개인 학습(GOOSE)에 적용하는 파생이다.

### 2.3 범위 경계

- **IN**: `LearningEntry` Go 구조체, `PromotionStatus` enum, 상태 전이 함수, `Promoter` 인터페이스, observation/heuristic/rule/high-confidence 임계값 평가, Anti-Pattern 즉시 격리, Evidence 수집 로직, Confidence 계산 알고리즘, 50 active entries 용량 관리, 30일 staleness window, 영속화 계약(MEMORY-001 hand-off), 상태 전이 이벤트 발행.
- **OUT**: 실제 LoRA 훈련/교체 (LORA-001), skill body 수정 (GOOSE-SKILLS-001의 consumer), 사용자 승인 다이얼로그 (SAFETY-001의 Human Oversight), frozen path 차단 (SAFETY-001의 FrozenGuard), canary 재평가 (SAFETY-001의 CanaryCheck), rollback 실행 (ROLLBACK-001), insight 생성 알고리즘 (INSIGHTS-001), 저장 백엔드 선택 (MEMORY-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/evolve/reflect/` 패키지 생성.
2. `LearningEntry` 구조체 (ID, Category, TargetComponent, ZoneID, Status, Observations, Confidence, Evidence[], ProposedChange, ApprovalStatus, CreatedAt/UpdatedAt, RollbackDeadline, ArchivedAt).
3. `PromotionStatus` enum: `Observation`, `Heuristic`, `Rule`, `HighConfidence`, `Graduated`, `Rejected`, `RolledBack`, `Archived`, `AntiPattern`.
4. `Category` enum: `Style`, `Pattern`, `Preference`, `Skill`, `Domain`, `ErrorRecovery`.
5. `Evidence` 구조체 (SessionID, Timestamp, Observation 문자열, Context any, SignalStrength float64).
6. `ProposedChange` 구조체 (TargetFile, Before, After, Rationale, ImpactAnalysis).
7. `Promoter` 인터페이스:
   - `Ingest(ctx context.Context, candidate InsightCandidate) (LearningEntry, error)`
   - `Tick(ctx context.Context, now time.Time) (TransitionReport, error)`
   - `GetEntry(id string) (LearningEntry, error)`
   - `ListByStatus(status PromotionStatus) ([]LearningEntry, error)`
   - `MarkGraduated(id string, approver string, approvedAt time.Time) error`
   - `MarkRejected(id string, reason string) error`
   - `MarkRolledBack(id string, triggeredBy string, at time.Time) error`
8. `DefaultPromoter` 구현체:
   - 관찰 누적 시 동일 `(Category, TargetComponent, ZoneID)` 키로 기존 entry 업데이트
   - 임계값 도달 시 상태 전이 트리거 (1회 → Observation; 3회 → Heuristic; 5회+conf≥0.80 → Rule; 10회+conf≥0.95 → HighConfidence)
   - Graduated는 외부 승인(SAFETY-001의 HumanOversight)을 통해서만 진입
   - Confidence 계산: `sum(SignalStrength) / max(observations, floor_count)` + exponential decay (decay=0.95/day)
9. Anti-Pattern 즉시 격리:
   - `IngestCritical(ctx, candidate, reason)` 경로 — critical failure 1회로 `AntiPattern` 상태 진입
   - `AntiPattern` entry는 FROZEN: 상태 전이 시도 시 `ErrAntiPatternLocked` 반환
   - 사용자 개입(SAFETY-001 Human Oversight)만 재분류 가능
10. 용량 관리:
    - 최대 50 active entries (`Observation`+`Heuristic`+`Rule`+`HighConfidence`)
    - 초과 시 LRU로 가장 오래된 `Observation` 자동 Archive
    - `Graduated`, `AntiPattern`, `RolledBack`은 active에 포함되지 않음
11. Staleness window:
    - `RolledBack` entry는 30일간 재제안 금지 (`RollbackDeadline`)
    - `Rejected` entry는 30일간 재제안 금지
    - 30일 경과 후 `Tick` 호출 시 자동 `Archived`
    - `Archived` entry는 `Observation`으로 재제안 가능
12. 영속화 계약 (MEMORY-001 hand-off):
    - 모든 상태 변화는 `MemoryProvider.SyncLearningEntry(entry)` 호출로 영속
    - `Tick` 종료 시 변경된 entry 목록과 함께 `TransitionReport` 반환
    - 저장 포맷은 MEMORY-001이 결정 (SQLite JSON column 또는 JSON-L append-only)
13. 상태 전이 이벤트 발행:
    - `TransitionReport` 구조체 (Transitions []Transition, UpdatedAt time.Time)
    - 각 `Transition`: EntryID, FromStatus, ToStatus, Trigger, OccurredAt
    - SAFETY-001이 이 report를 구독하여 HighConfidence 진입 시 승인 프로세스 시작

### 3.2 OUT OF SCOPE

- LoRA adapter 교체/훈련 실행 → `SPEC-GOOSE-LORA-001`
- 사용자 승인 다이얼로그, AskUserQuestion 연계 Hook 이벤트 발행 → `SPEC-GOOSE-SAFETY-001` + `SPEC-GOOSE-HOOK-001`
- Canary 재평가(최근 3 session shadow 평가) → `SAFETY-001`
- Contradiction 감지 → `SAFETY-001`
- Rate limiting (주 3회 진화, 24h 쿨다운) → `SAFETY-001`
- Frozen path 보호 (CLAUDE.md, learning-engine.md) → `SAFETY-001`
- Regression 감지 및 자동 롤백 실행 → `ROLLBACK-001`
- Insight 추출 알고리즘 (패턴 마이닝, 클러스터링) → `INSIGHTS-001`
- 영속화 백엔드 구현 (SQLite schema, 파일 포맷) → `MEMORY-001`

---

## 4. 요구사항 (EARS Requirements)

### 4.1 Ubiquitous (상시)

- **REQ-REFLECT-001**: 시스템은 `LearningEntry`의 상태를 `Observation`, `Heuristic`, `Rule`, `HighConfidence`, `Graduated`, `Rejected`, `RolledBack`, `Archived`, `AntiPattern` 9개 값 중 하나로 유지**해야 한다**.
- **REQ-REFLECT-002**: 시스템은 `LearningEntry`마다 `Observations int`와 `Confidence float64 ∈ [0, 1]`을 항상 함께 기록**해야 한다**.
- **REQ-REFLECT-003**: 시스템은 `active entries` (Observation/Heuristic/Rule/HighConfidence 합)의 개수를 50개 이하로 유지**해야 한다**.
- **REQ-REFLECT-004**: 시스템은 모든 상태 전이 결과를 `MemoryProvider.SyncLearningEntry`를 통해 영속**해야 한다**.

### 4.2 Event-Driven (트리거 기반)

- **REQ-REFLECT-005**: **When** `Promoter.Ingest`가 호출되어 `(Category, TargetComponent, ZoneID)` 키에 해당하는 entry가 존재하지 않을 때, 시스템은 `Observation` 상태의 새 entry를 생성하고 `Observations=1`로 설정**해야 한다**.
- **REQ-REFLECT-006**: **When** `Promoter.Ingest`가 호출되어 동일 키의 기존 entry가 발견될 때, 시스템은 `Observations`를 1 증가시키고 `Evidence`를 추가하며 `Confidence`를 재계산**해야 한다**.
- **REQ-REFLECT-007**: **When** `Observations >= 3` 조건을 만족하고 현재 상태가 `Observation`일 때, 시스템은 상태를 `Heuristic`으로 전이**해야 한다**.
- **REQ-REFLECT-008**: **When** `Observations >= 5` AND `Confidence >= 0.80` 조건을 만족하고 현재 상태가 `Heuristic`일 때, 시스템은 상태를 `Rule`로 전이**해야 한다**.
- **REQ-REFLECT-009**: **When** `Observations >= 10` AND `Confidence >= 0.95` 조건을 만족하고 현재 상태가 `Rule`일 때, 시스템은 상태를 `HighConfidence`로 전이**해야 한다**.
- **REQ-REFLECT-010**: **When** 상태가 `HighConfidence`로 진입할 때, 시스템은 `TransitionReport`에 해당 entry를 포함하여 구독자(SAFETY-001)에게 notification을 전달**해야 한다**.
- **REQ-REFLECT-011**: **When** `Promoter.IngestCritical`이 critical failure evidence와 함께 호출될 때, 시스템은 기존 상태와 관계없이 entry를 `AntiPattern` 상태로 즉시 격리**해야 한다**.
- **When** `Promoter.Tick`이 호출될 때 (REQ-REFLECT-012): 시스템은 `Rejected`/`RolledBack` 상태이면서 `RollbackDeadline < now`인 모든 entry를 `Archived`로 전이**해야 한다**.

### 4.3 State-Driven (상태 기반)

- **REQ-REFLECT-013**: **While** entry가 `AntiPattern` 상태일 때, 시스템은 모든 상태 전이 시도에 대해 `ErrAntiPatternLocked`를 반환**해야 한다**.
- **REQ-REFLECT-014**: **While** active entries 개수가 50에 도달한 상태일 때, 시스템은 새 `Observation` entry 생성 시 가장 오래된 `Observation` (UpdatedAt 최소) 1개를 `Archived`로 강등**해야 한다**.
- **REQ-REFLECT-015**: **While** entry가 `RolledBack` 상태일 때, 시스템은 동일 키의 `Promoter.Ingest` 요청에 대해 `RollbackDeadline` 이전까지 새 entry 생성을 거부**해야 한다**.

### 4.4 Optional (조건부 기능)

- **REQ-REFLECT-016**: **Where** 구성 `reflect.confidence_decay_enabled: true`가 설정된 경우, 시스템은 `Tick` 실행 시 각 entry의 Evidence에 대해 지수 감쇠(decay=0.95/day)를 적용하여 `Confidence`를 재계산**해야 한다**.
- **REQ-REFLECT-017**: **Where** 구성 `reflect.max_active_entries`가 제공된 경우, 시스템은 기본값 50 대신 해당 값을 용량 한도로 사용**해야 한다**.

### 4.5 Unwanted Behavior (금지/차단)

- **REQ-REFLECT-018**: **If** entry의 `Confidence`가 0 미만 또는 1 초과 값으로 계산**되면 then** 시스템은 `ErrInvalidConfidence`를 반환하고 해당 entry 상태 전이를 차단**해야 한다**.
- **REQ-REFLECT-019**: **If** `MarkGraduated`가 SAFETY-001의 승인 토큰 없이 호출**되면 then** 시스템은 `ErrUnauthorizedGraduation`을 반환하고 상태 전이를 거부**해야 한다**.

### 4.6 Complex (복합)

- **REQ-REFLECT-020**: **While** entry 상태가 `HighConfidence`이고, **when** SAFETY-001로부터 승인 토큰과 함께 `MarkGraduated` 호출이 들어오면, 시스템은 상태를 `Graduated`로 전이하고 `ApprovedAt`, `ApprovedBy`를 기록하며 `RollbackDeadline`을 `now + 30일`로 설정**해야 한다**.

---

## 5. Go 타입 시그니처 (참조 인터페이스)

```go
// internal/evolve/reflect/entry.go
package reflect

import (
    "time"
)

// PromotionStatus 는 학습 항목의 승격 단계를 나타내는 enum 이다.
type PromotionStatus int

const (
    StatusObservation     PromotionStatus = iota // 1회 관찰
    StatusHeuristic                              // 3회 이상 관찰
    StatusRule                                    // 5회 이상 + conf >= 0.80
    StatusHighConfidence                         // 10회 이상 + conf >= 0.95
    StatusGraduated                              // 사용자 승인 완료
    StatusRejected                               // 사용자 거부
    StatusRolledBack                             // 회귀 발생으로 되돌림
    StatusArchived                               // 30일 경과 또는 1년 미사용
    StatusAntiPattern                            // critical failure 즉시 격리
)

// Category 는 학습 영역 분류이다.
type Category int

const (
    CategoryStyle        Category = iota // 응답 스타일 선호
    CategoryPattern                      // 행동 패턴
    CategoryPreference                   // 명시적 선호도
    CategorySkill                        // skill 자동 생성 후보
    CategoryDomain                       // 도메인 지식
    CategoryErrorRecovery                // 오류 복구 전략
)

// Evidence 는 관찰 1건의 증거이다.
type Evidence struct {
    SessionID      string    // 어느 대화 세션에서?
    Timestamp      time.Time // 언제?
    Observation    string    // 관찰 내용 요약
    Context        any       // 추가 구조화 컨텍스트 (optional)
    SignalStrength float64   // [0, 1] 신호 강도
}

// ProposedChange 는 승격 시 적용될 변경 제안이다.
type ProposedChange struct {
    TargetFile      string // "CLAUDE.md" | "lora/adapter.bin" | "skills/user-style.md"
    Before          string
    After           string
    Rationale       string // 왜 이 변경이 필요한가
    ImpactAnalysis  string // 예상 영향
}

// LearningEntry 는 승격 파이프라인의 단위 항목이다.
type LearningEntry struct {
    ID               string          // LEARN-YYYYMMDD-NNN
    Category         Category
    TargetComponent  string          // skill id | agent id | LoRA id | CLAUDE.md section
    ZoneID           string          // evolvable zone 식별자
    Status           PromotionStatus
    Observations     int             // 1 / 3 / 5 / 10 ...
    Confidence       float64         // [0, 1]
    Evidence         []Evidence
    ProposedChange   *ProposedChange // nil until Heuristic 이상
    ApprovedAt       *time.Time
    ApprovedBy       string          // "user@local" etc.
    CreatedAt        time.Time
    UpdatedAt        time.Time
    RollbackDeadline *time.Time      // Rejected/RolledBack/Graduated 후 30일
    ArchivedAt       *time.Time
}

// internal/evolve/reflect/promoter.go

// InsightCandidate 는 INSIGHTS-001 이 전달하는 입력 DTO 이다.
type InsightCandidate struct {
    Category        Category
    TargetComponent string
    ZoneID          string
    Observation     string
    Context         any
    SignalStrength  float64
    SessionID       string
    Timestamp       time.Time
    Critical        bool // true 이면 AntiPattern 즉시 격리
}

// Transition 은 상태 전이 1건의 기록이다.
type Transition struct {
    EntryID    string
    FromStatus PromotionStatus
    ToStatus   PromotionStatus
    Trigger    string // "observation_threshold" | "confidence_threshold" | "critical_failure" | "staleness"
    OccurredAt time.Time
}

// TransitionReport 는 Tick 1회 결과 보고이다.
type TransitionReport struct {
    Transitions []Transition
    UpdatedAt   time.Time
}

// Promoter 는 승격 상태 머신의 주 인터페이스이다.
type Promoter interface {
    // Ingest 는 INSIGHTS 후보를 받아 새 entry 생성 또는 기존 entry 업데이트.
    Ingest(ctx context.Context, candidate InsightCandidate) (LearningEntry, error)

    // IngestCritical 은 critical failure 를 AntiPattern 으로 즉시 격리.
    IngestCritical(ctx context.Context, candidate InsightCandidate, reason string) (LearningEntry, error)

    // Tick 은 주기적 상태 점검 (staleness, confidence decay).
    Tick(ctx context.Context, now time.Time) (TransitionReport, error)

    GetEntry(id string) (LearningEntry, error)
    ListByStatus(status PromotionStatus) ([]LearningEntry, error)

    // MarkGraduated 는 SAFETY-001 의 승인 토큰을 필요로 한다.
    MarkGraduated(id string, approver string, approvedAt time.Time, approvalToken SafetyToken) error
    MarkRejected(id string, reason string) error
    MarkRolledBack(id string, triggeredBy string, at time.Time) error
}

// SafetyToken 은 SAFETY-001 에서 발급한 1회용 승인 토큰이다.
// 위조/재사용 방지는 SAFETY-001 이 책임진다. REFLECT 는 Verify 만 수행.
type SafetyToken struct {
    TokenID   string
    IssuedAt  time.Time
    ExpiresAt time.Time
    Signature []byte
}
```

---

## 6. 수락 기준 (Acceptance Criteria)

- **AC-REFLECT-001** (REQ-REFLECT-005, 006 검증)
  - **Given** `Promoter`가 비어있을 때,
  - **When** 동일 `(Category=Style, TargetComponent="response-length", ZoneID="user_style")` 키로 `Ingest`가 3회 호출되면,
  - **Then** 1개 entry가 존재하고 `Observations=3`, `Evidence.len==3`, 상태는 `Observation`에서 `Heuristic`으로 전이된다.

- **AC-REFLECT-002** (REQ-REFLECT-008 Rule 승격)
  - **Given** 상태 `Heuristic`, `Observations=4` entry가 있을 때,
  - **When** `SignalStrength=0.9`로 `Ingest`가 한 번 더 호출되고 `Confidence`가 0.80 이상으로 계산되면,
  - **Then** 상태는 `Rule`로 전이되고 `TransitionReport.Transitions`에 해당 전이가 포함된다.

- **AC-REFLECT-003** (REQ-REFLECT-009, 010 HighConfidence + notification)
  - **Given** 상태 `Rule`, `Observations=9` entry가 있을 때,
  - **When** `SignalStrength=1.0`로 `Ingest`가 호출되어 `Observations=10`, `Confidence≥0.95`에 도달하면,
  - **Then** 상태는 `HighConfidence`로 전이되고, 반환된 `TransitionReport`에 해당 entry ID가 `ToStatus=HighConfidence`로 기록된다.

- **AC-REFLECT-004** (REQ-REFLECT-011, 013 Anti-Pattern 격리)
  - **Given** 상태 `Rule` entry가 있을 때,
  - **When** 동일 키로 `IngestCritical(..., reason="data_leakage")`이 호출되면,
  - **Then** 상태는 `AntiPattern`으로 전이되며, 이후 `Ingest` 또는 `MarkGraduated` 호출은 `ErrAntiPatternLocked`를 반환한다.

- **AC-REFLECT-005** (REQ-REFLECT-003, 014 용량 관리)
  - **Given** active entries가 정확히 50개 있는 상태에서,
  - **When** 새 `Observation`을 생성하는 `Ingest`가 호출되면,
  - **Then** `UpdatedAt`이 가장 오래된 `Observation` 1개가 `Archived`로 전이되고 총 active는 50으로 유지된다.

- **AC-REFLECT-006** (REQ-REFLECT-012, 015 Staleness + rollback cooldown)
  - **Given** 상태 `RolledBack`, `RollbackDeadline = now - 1h`인 entry가 있을 때,
  - **When** `Tick(now)`이 호출되면,
  - **Then** 상태는 `Archived`로 전이되고, 이전 30일 내 동일 키 `Ingest`는 거부되었음을 로그에서 확인할 수 있다.

- **AC-REFLECT-007** (REQ-REFLECT-019 승인 토큰 검증)
  - **Given** 상태 `HighConfidence` entry가 있을 때,
  - **When** 유효하지 않은 `SafetyToken`으로 `MarkGraduated`가 호출되면,
  - **Then** `ErrUnauthorizedGraduation`이 반환되고 상태는 `HighConfidence`로 유지된다.

- **AC-REFLECT-008** (REQ-REFLECT-020 Graduated 승격)
  - **Given** 상태 `HighConfidence` entry와 유효한 `SafetyToken`이 있을 때,
  - **When** `MarkGraduated(id, "user@local", now, token)`이 호출되면,
  - **Then** 상태는 `Graduated`로 전이되고 `ApprovedAt=now`, `ApprovedBy="user@local"`, `RollbackDeadline=now+30일`이 설정된다.

- **AC-REFLECT-009** (REQ-REFLECT-004 영속화)
  - **Given** Mock `MemoryProvider`가 주입된 `Promoter`,
  - **When** 임의의 `Ingest` 또는 `MarkGraduated`/`MarkRejected`/`MarkRolledBack`이 호출되면,
  - **Then** Mock의 `SyncLearningEntry`가 호출되고 인자의 entry ID가 변경된 항목과 일치한다.

- **AC-REFLECT-010** (REQ-REFLECT-016 Confidence decay)
  - **Given** 구성 `confidence_decay_enabled: true`, Evidence 10개 중 가장 오래된 것이 7일 전인 entry,
  - **When** `Tick(now)`이 호출되면,
  - **Then** 재계산된 `Confidence`가 decay 미적용 값보다 작고, 전이 임계값 변경 여부가 정확히 반영된다.

- **AC-REFLECT-011** (REQ-REFLECT-018 경계값)
  - **Given** 결함이 있는 Evidence로 `Confidence`가 1.0001로 계산되는 상황,
  - **When** 상태 전이가 시도되면,
  - **Then** `ErrInvalidConfidence`가 반환되고 상태는 변경되지 않는다.

- **AC-REFLECT-012** (Go build 테스트)
  - **Given** `internal/evolve/reflect` 패키지,
  - **When** `go build ./internal/evolve/reflect/... && go test -race ./internal/evolve/reflect/...`가 실행되면,
  - **Then** 컴파일 에러 없이 빌드되고 모든 테스트가 85% 이상 커버리지로 통과한다.

---

## 7. 기술적 접근 (Technical Approach)

### 7.1 패키지 레이아웃

```
internal/evolve/reflect/
├── entry.go       # LearningEntry, Evidence, ProposedChange, PromotionStatus, Category
├── state.go       # 상태 전이 테이블, 허용 전이 validator
├── promoter.go    # Promoter interface + DefaultPromoter
├── confidence.go  # Confidence 계산, decay 알고리즘
├── capacity.go    # 50 active entries LRU, archive 정책
├── staleness.go   # 30일 staleness window 처리
├── errors.go      # ErrAntiPatternLocked, ErrUnauthorizedGraduation, ErrInvalidConfidence
└── testdata/      # golden test fixtures
```

### 7.2 상태 전이 테이블

| From | To | Trigger |
|-----|-----|---------|
| (none) | Observation | `Ingest` 신규 |
| Observation | Heuristic | Observations ≥ 3 |
| Heuristic | Rule | Observations ≥ 5 AND Confidence ≥ 0.80 |
| Rule | HighConfidence | Observations ≥ 10 AND Confidence ≥ 0.95 |
| HighConfidence | Graduated | SAFETY-001 승인 + 유효 토큰 |
| HighConfidence | Rejected | 사용자 거부 |
| Graduated | RolledBack | ROLLBACK-001 트리거 |
| (any ≠ AntiPattern) | AntiPattern | `IngestCritical` |
| Rejected | Archived | now > RollbackDeadline |
| RolledBack | Archived | now > RollbackDeadline |
| Observation (LRU) | Archived | 용량 초과 |

### 7.3 Confidence 계산 (간략)

```
confidence(entry, now) =
    if decay_enabled:
        Σ(ev.SignalStrength * 0.95^days_since(ev.Timestamp, now)) / max(|Evidence|, 3)
    else:
        Σ(ev.SignalStrength) / max(|Evidence|, 3)
clamp to [0, 1]
```

floor 값 3은 초기 관찰이 적을 때 overconfidence를 방지한다.

### 7.4 동시성 모델

- `DefaultPromoter`는 내부 `sync.RWMutex`로 entry map 보호.
- `Ingest`, `Tick`, `Mark*`는 단일 진입점. 외부 호출자(INSIGHTS-001, SAFETY-001)는 다른 goroutine에서 안전하게 호출 가능.
- 상태 전이 중 `MemoryProvider.SyncLearningEntry` 호출은 mutex 밖에서 실행 (deadlock 방지).

### 7.5 MEMORY-001 hand-off 계약

REFLECT는 `MemoryProvider` 인터페이스 중 다음 1개 메서드만 사용:

```go
type ReflectMemory interface {
    SyncLearningEntry(entry LearningEntry) error
    LoadLearningEntries() ([]LearningEntry, error) // 재시작 시 복원
}
```

저장소 선택(SQLite vs JSON-L)은 MEMORY-001이 결정. REFLECT는 구현에 무관하다.

### 7.6 SAFETY-001 hand-off 계약

- REFLECT는 `TransitionReport.Transitions`에서 `ToStatus == HighConfidence` 항목을 SAFETY-001이 폴링(또는 event channel 구독)하도록 한다.
- `MarkGraduated` 호출 경로는 반드시 SAFETY-001의 HumanOversight를 통과한 후 발급된 `SafetyToken`과 함께 호출되어야 한다.

### 7.7 테스트 전략 (TDD RED-GREEN-REFACTOR)

- 상태 전이 11종 각각에 대한 단위 테스트
- Table-driven 테스트로 임계값 boundary 검증 (Observations 2/3/4, Confidence 0.79/0.80/0.81)
- Mock `MemoryProvider`로 영속화 호출 검증
- Property-based 테스트 (예: 동일 키 10회 Ingest 후 `Observations==10` 보장)
- Race detector 필수 (`go test -race`)

---

## 8. 리스크 & 완화

| 리스크 | 영향 | 완화 |
|-------|-----|------|
| Confidence 계산 편향으로 조기 승격 | 사용자가 원치 않는 변경 승인 요청 | floor 값 3 도입, decay 활성화 시 추가 검증, SAFETY-001의 HumanOversight가 최종 안전망 |
| Anti-Pattern 오탐으로 정상 패턴 영구 차단 | 유용한 진화 기회 상실 | `AntiPattern`은 사용자 개입(SAFETY-001)만 재분류 가능하되, 로그에 original candidate 보존 |
| 50 active 용량 초과 시 유의미한 Observation 상실 | 진화 기회 축소 | LRU 기반 archive, 90일 후 재제안 가능, 구성값으로 조절 가능 |
| MEMORY-001 Sync 실패 시 상태 불일치 | 재시작 후 상태 복원 실패 | Sync 실패 시 in-memory rollback, 에러를 호출자에 전파 |
| 동시 Ingest 경합으로 Observations 카운트 오류 | 승격 지연/오류 | RWMutex 직렬화, race detector로 CI 검증 |
| SafetyToken 위조 | 무단 Graduated 승격 | 토큰 검증은 SAFETY-001 책임, REFLECT는 Verify 결과 수락만 |

---

## 9. 용어 및 참고 (Glossary & References)

- **LearningEntry**: 승격 파이프라인의 최소 단위. (Category, TargetComponent, ZoneID) 키로 식별.
- **ZoneID**: `CLAUDE.md`의 `<!-- goose:evolvable-start id="..." -->` block 식별자 또는 LoRA adapter 영역 식별자.
- **Evidence**: 관찰 1건의 증거 기록. SignalStrength는 [0, 1] 정규화된 신호.
- **Anti-Pattern**: critical failure 1회로 즉시 격리되는 FROZEN 상태. 사용자 개입만 재분류 가능.
- **SafetyToken**: SAFETY-001이 HumanOversight 통과 후 발급하는 1회용 승인 토큰.

### 참조 문서

- `.moai/project/learning-engine.md` §2 (5단계 승격 파이프라인, LearningEntry 스키마)
- `.moai/project/research/hermes-learning.md` §11-12 (SPEC-REFLECT 연계, 5-layer 구성)
- `.claude/rules/agency/constitution.md` §6 (Learnings Pipeline 원형)
- MoAI-ADK-Go `SPEC-REFLECT-001` (본 SPEC의 직접 계승 원천)
- `SPEC-GOOSE-INSIGHTS-001` (Ingest 입력 생산자)
- `SPEC-GOOSE-MEMORY-001` (영속화 백엔드)
- `SPEC-GOOSE-SAFETY-001` (HumanOversight, CanaryCheck, FrozenGuard, SafetyToken 발급)
- `SPEC-GOOSE-ROLLBACK-001` (MarkRolledBack 트리거)
