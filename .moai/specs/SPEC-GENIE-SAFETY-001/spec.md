---
id: SPEC-GENIE-SAFETY-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P1
issue_number: null
phase: 5
size: 중(M)
lifecycle: spec-anchored
---

# SPEC-GENIE-SAFETY-001 — 5-Layer Safety Architecture (FrozenGuard + Canary + Contradiction + RateLimiter + HumanOversight)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (agency constitution §5 + learning-engine §2.3 계승, REFLECT-001 hand-off 계약 반영) | manager-spec |

---

## 1. 개요 (Overview)

REFLECT-001이 `HighConfidence`로 승격시킨 `LearningEntry`가 실제 사용자 상태(LoRA adapter, CLAUDE.md evolvable zone, skill body)에 적용되기 전에 반드시 통과해야 하는 **5-layer safety gate**를 정의한다.

각 layer는 순차적으로 평가되며, 어느 하나라도 실패하면 해당 entry는 즉시 차단되고 위반 사유가 audit log에 기록된다. 모든 layer를 통과하면 `SafetyToken`이 발급되어 REFLECT-001의 `MarkGraduated(id, approver, at, token)` 호출에 사용된다.

본 SPEC은 5 layer의 **구현 계약**과 **통과/거부 결정 로직**을 규정한다. 사용자와의 실제 상호작용(AskUserQuestion 표시)은 `SPEC-GENIE-HOOK-001`의 Hook 이벤트 발행을 통해서만 수행하며, 직접 AskUserQuestion을 호출하지 않는다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- REFLECT-001은 상태 머신만 구현하고 `MarkGraduated`에 SafetyToken을 요구한다. 이 토큰을 발급하는 유일한 컴포넌트가 SAFETY-001이다. 즉 SAFETY-001 없이는 어떤 학습도 실제로 Graduated 상태에 도달할 수 없다.
- `.claude/rules/agency/constitution.md §5`가 명시한 5-layer safety architecture를 GENIE 개인 학습 환경에 맞게 포팅하는 유일한 SPEC이다.
- ROLLBACK-001은 SAFETY의 CanaryCheck 결과와 HumanOversight 승인 이력에 의존한다. SAFETY가 선행되어야 ROLLBACK이 건설된다.

### 2.2 상속 자산

- **agency/constitution §5** (FROZEN): 5 layer 개념(Frozen Guard / Canary Check / Contradiction Detector / Rate Limiter / Human Oversight)과 각 layer의 실패 응답 원형. 본 SPEC은 이 구조를 그대로 계승하되 구현은 Go로 신규.
- **learning-engine.md §2.3**: `FrozenPaths` 목록(CLAUDE.md, learning-engine.md 자체 등), `RateLimiter` 수치(주 3 / 24h cooldown / 50 active / 1 hourly / 2 daily), `ApprovalRequest` 구조, `Rollback.RegressionThreshold=0.10`.
- **MoAI-ADK-Go SPEC-REFLECT-001**: 동일한 5-layer를 프로젝트 단위에서 이미 구현. 개념은 동일하되 대상이 project → personal로 바뀐다.

### 2.3 GENIE-specific 차이점

| 축 | MoAI (agency) | GENIE (개인) |
|----|--------------|------------|
| Frozen 대상 | `.claude/rules/agency/constitution.md`, safety architecture, pipeline phase ordering | `CLAUDE.md`, `.claude/rules/core/*`, `.claude/rules/moai/core/*`, `learning-engine.md`, `user.yaml` |
| Canary 대상 | 최근 3 project 재평가 | 최근 3 session shadow 재평가 (trajectory replay) |
| Rate limit 대상 | 프로젝트 진화 주 3회 | 사용자 진화 주 3회 + 24h 파일 쿨다운 |
| HumanOversight 채널 | AskUserQuestion (MoAI orchestrator) | Hook 이벤트 (HOOK-001) → 상위 호출자가 AskUserQuestion 연계 |

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/evolve/safety/` 패키지 생성.
2. `SafetyGate` 인터페이스: 5 layer를 순차 실행하고 `SafetyDecision`(Approved/Rejected/NeedsReview) 반환.
3. **Layer 1 — FrozenGuard**:
   - `FrozenZone` 구조체와 FROZEN path 목록 (glob pattern 지원)
   - `Check(change ProposedChange) error` — 변경 대상 path가 FROZEN 영역에 속하면 즉시 차단
   - 기본 목록: `CLAUDE.md`, `.claude/rules/core/**`, `.claude/rules/moai/core/**`, `.moai/project/learning-engine.md`, `.moai/config/sections/user.yaml`
4. **Layer 2 — CanaryCheck**:
   - `CanaryResult` 구조체 (EvaluatedSessions int, ScoreBefore, ScoreAfter, MaxScoreDrop float64)
   - 최근 3개 trajectory session을 replay하여 ProposedChange를 **in-memory로만** 적용한 상태로 재평가
   - `MaxScoreDrop > 0.10`이면 실패
5. **Layer 3 — ContradictionDetector**:
   - `Contradiction` 구조체 (ExistingEntry *LearningEntry, NewChange *ProposedChange, Reason string)
   - 기존 `Graduated` 상태 entry 중 동일 `ZoneID`나 overlapping `TargetComponent`를 가지면서 Before/After가 대립하는 경우 감지
   - 감지 시 `SafetyDecision.NeedsReview=true`로 설정하여 HumanOversight로 위임
6. **Layer 4 — RateLimiter**:
   - `RateLimitPolicy` 구조체 (MaxPerWeek, FileCooldown, MaxActive, HourlyLimit, DailyLimit)
   - 기본값: 주 3회, 24h 파일 cooldown, 최대 50 active, 시간당 1회, 일 2회
   - `Check(change, now) error` — 한도 초과 시 `ErrRateLimitExceeded`
7. **Layer 5 — HumanOversight**:
   - `ApprovalRequest` 구조체 (LearningID, Title, Description, BeforeAfterDiff, ImpactAnalysis, Options)
   - `RequestApproval(ctx, entry)` — HOOK-001의 `HookEventApprovalRequest` 이벤트 발행
   - `ReceiveDecision(learningID, decision ApprovalOption)` — 외부(상위 orchestrator)에서 결과 주입
   - 결정 `Approve` 시 `SafetyToken` 발급 (Ed25519 서명, 15분 만료)
8. `SafetyDecision` 결과 유형: `Approved(token SafetyToken)` / `Rejected(reason string)` / `NeedsReview(pendingID string)`.
9. Audit log: 모든 5-layer 평가 결과를 `.moai/evolution/safety-audit.log`에 append-only JSON-L로 기록.
10. SafetyToken 발급 및 검증:
    - Ed25519 키쌍(`~/.genie/keys/safety-signer.ed25519`)으로 서명
    - `TokenID`(UUIDv7) + `LearningID` + `IssuedAt` + `ExpiresAt`(+15분) + `Signature`
    - `Verify(token)` 는 서명 유효성, 만료, 1회성 사용(`used_tokens` 캐시) 검증

### 3.2 OUT OF SCOPE

- 상태 전이, LearningEntry 정의 → `REFLECT-001`
- 실제 AskUserQuestion 호출 → MoAI 상위 orchestrator (SAFETY는 HOOK-001 이벤트만 발행)
- LoRA adapter 교체 실행, skill body 수정 → 하위 consumer SPEC
- Regression 자동 감지, 자동 롤백 실행 → `ROLLBACK-001`
- Trajectory replay 상세 로직 (score 계산 알고리즘) → `INSIGHTS-001`의 scoring API 재사용
- Hook 이벤트 전달 메커니즘 구현 → `HOOK-001`

---

## 4. 요구사항 (EARS Requirements)

### 4.1 Ubiquitous

- **REQ-SAFETY-001**: 시스템은 모든 `Graduated` 승격 요청이 5 layer (FrozenGuard → CanaryCheck → ContradictionDetector → RateLimiter → HumanOversight) 순서로 평가되도록 보장**해야 한다**.
- **REQ-SAFETY-002**: 시스템은 SafetyToken 발급 시 Ed25519 서명, TokenID(UUIDv7), 15분 만료를 항상 포함**해야 한다**.
- **REQ-SAFETY-003**: 시스템은 5 layer 평가의 모든 결과(성공/실패/NeedsReview)를 `.moai/evolution/safety-audit.log`에 append-only JSON-L로 기록**해야 한다**.

### 4.2 Event-Driven

- **REQ-SAFETY-004**: **When** `SafetyGate.Evaluate(entry)`가 호출될 때, 시스템은 Layer 1 FrozenGuard부터 순서대로 각 layer를 실행하고 첫 실패에서 즉시 중단하여 `Rejected`를 반환**해야 한다**.
- **REQ-SAFETY-005**: **When** FrozenGuard가 `ProposedChange.TargetFile`을 FROZEN path 목록과 매칭할 때, 시스템은 `ErrFrozenPathViolation`을 반환하고 후속 layer를 실행하지 않**아야 한다**.
- **REQ-SAFETY-006**: **When** CanaryCheck가 최근 3 session replay에서 `MaxScoreDrop > 0.10`을 감지할 때, 시스템은 `SafetyDecision.Rejected(reason="canary_regression")`을 반환**해야 한다**.
- **REQ-SAFETY-007**: **When** ContradictionDetector가 기존 `Graduated` entry와 모순되는 변경을 감지할 때, 시스템은 `SafetyDecision.NeedsReview(pendingID)`를 반환하고 HumanOversight로 위임**해야 한다**.
- **REQ-SAFETY-008**: **When** Layer 5 HumanOversight가 호출될 때, 시스템은 HOOK-001의 `HookEventApprovalRequest` 이벤트를 발행하고 결정 대기 상태로 진입**해야 한다**.
- **REQ-SAFETY-009**: **When** `ReceiveDecision(id, ApprovalApprove)`이 호출될 때, 시스템은 새 `SafetyToken`을 발급하고 REFLECT-001의 `MarkGraduated` 호출자에게 반환**해야 한다**.
- **REQ-SAFETY-010**: **When** `ReceiveDecision(id, ApprovalReject)`이 호출될 때, 시스템은 REFLECT-001의 `MarkRejected(id, reason)`를 호출**해야 한다**.

### 4.3 State-Driven

- **REQ-SAFETY-011**: **While** RateLimiter가 `currentWeekCount >= MaxPerWeek(=3)` 상태일 때, 시스템은 새 `Graduated` 승격 요청을 `ErrRateLimitExceeded`로 거부**해야 한다**.
- **REQ-SAFETY-012**: **While** RateLimiter가 `now - lastChangeAt(file) < FileCooldown(=24h)` 상태일 때, 시스템은 동일 파일에 대한 변경 요청을 거부**해야 한다**.
- **REQ-SAFETY-013**: **While** 특정 `SafetyToken`이 `used_tokens` 캐시에 존재할 때, 시스템은 해당 토큰의 재사용 시도를 `ErrTokenAlreadyUsed`로 거부**해야 한다**.

### 4.4 Optional

- **REQ-SAFETY-014**: **Where** 구성 `safety.frozen_paths`에 추가 경로가 정의된 경우, 시스템은 기본 FROZEN 목록과 사용자 정의 경로를 병합하여 검사**해야 한다**.
- **REQ-SAFETY-015**: **Where** 구성 `safety.canary_threshold`가 설정된 경우, 시스템은 기본값 0.10 대신 해당 값을 `MaxScoreDrop` 임계로 사용**해야 한다**.

### 4.5 Unwanted Behavior

- **REQ-SAFETY-016**: **If** SafetyToken 서명 검증이 실패**하면 then** 시스템은 `ErrInvalidToken`을 반환하고 해당 토큰의 모든 사용을 거부**해야 한다**.

---

## 5. Go 타입 시그니처

```go
// internal/evolve/safety/types.go
package safety

import (
    "context"
    "time"

    "github.com/genie/internal/evolve/reflect"
)

// SafetyDecision 은 5-layer 평가의 최종 결과이다.
type SafetyDecision struct {
    Outcome       DecisionOutcome
    Token         *reflect.SafetyToken // Approved 인 경우에만
    RejectReason  string               // Rejected 인 경우
    PendingReview *PendingApproval     // NeedsReview 인 경우
    LayerResults  [5]LayerResult       // 각 layer 평가 기록
}

type DecisionOutcome int

const (
    DecisionApproved    DecisionOutcome = iota
    DecisionRejected
    DecisionNeedsReview
)

// LayerResult 는 각 layer 1회 평가의 audit 기록이다.
type LayerResult struct {
    Layer      int      // 1..5
    Name       string   // "FrozenGuard" etc.
    Passed     bool
    Reason     string
    Evaluated  time.Time
}

// SafetyGate 는 5 layer 통합 평가 인터페이스이다.
type SafetyGate interface {
    Evaluate(ctx context.Context, entry reflect.LearningEntry) (SafetyDecision, error)
    ReceiveDecision(ctx context.Context, learningID string, option ApprovalOption) error
    VerifyToken(token reflect.SafetyToken) error
}

// internal/evolve/safety/frozenguard.go

type FrozenZone struct {
    Paths []string // glob patterns
}

type FrozenGuard interface {
    Check(change reflect.ProposedChange) error
}

// internal/evolve/safety/canary.go

type CanaryResult struct {
    EvaluatedSessions int
    ScoreBefore       float64
    ScoreAfter        float64
    MaxScoreDrop      float64
    Threshold         float64
}

type Canary interface {
    Check(ctx context.Context, change reflect.ProposedChange) (CanaryResult, error)
}

// internal/evolve/safety/contradiction.go

type Contradiction struct {
    ExistingEntry *reflect.LearningEntry
    NewChange     *reflect.ProposedChange
    Reason        string
}

type ContradictionDetector interface {
    Detect(ctx context.Context, entry reflect.LearningEntry) ([]Contradiction, error)
}

// internal/evolve/safety/ratelimiter.go

type RateLimitPolicy struct {
    MaxPerWeek    int           // 기본 3
    FileCooldown  time.Duration // 기본 24h
    MaxActive     int           // 기본 50
    HourlyLimit   int           // 기본 1
    DailyLimit    int           // 기본 2
}

type RateLimiter interface {
    Check(ctx context.Context, change reflect.ProposedChange, now time.Time) error
    Record(ctx context.Context, change reflect.ProposedChange, now time.Time) error
}

// internal/evolve/safety/approval.go

type ApprovalRequest struct {
    LearningID       string
    Title            string
    Description      string
    BeforeAfterDiff  string // unified diff
    ImpactAnalysis   string
    Options          []ApprovalOption
    Deadline         time.Time
}

type ApprovalOption int

const (
    ApprovalApprove    ApprovalOption = iota
    ApprovalReject
    ApprovalAskLater
    ApprovalModify
)

type PendingApproval struct {
    LearningID string
    RequestID  string
    CreatedAt  time.Time
    Deadline   time.Time
}

// internal/evolve/safety/gate.go

// DefaultSafetyGate 는 5 layer 를 직렬 실행하는 구현체이다.
type DefaultSafetyGate struct {
    frozenGuard   FrozenGuard
    canary        Canary
    contradiction ContradictionDetector
    rateLimiter   RateLimiter
    tokenSigner   TokenSigner
    hookEmitter   HookEmitter // HOOK-001 hand-off
    auditLogger   AuditLogger
    memory        reflect.ReflectMemory
}

// TokenSigner 는 SafetyToken 발급/검증 인터페이스이다.
type TokenSigner interface {
    Issue(learningID string, now time.Time) (reflect.SafetyToken, error)
    Verify(token reflect.SafetyToken) error
    MarkUsed(tokenID string) error
}

// HookEmitter 는 HOOK-001 에 이벤트를 발행하는 얇은 인터페이스이다.
type HookEmitter interface {
    EmitApprovalRequest(ctx context.Context, req ApprovalRequest) error
}
```

---

## 6. 수락 기준 (Acceptance Criteria)

- **AC-SAFETY-001** (REQ-SAFETY-005 FrozenGuard)
  - **Given** `ProposedChange{TargetFile: "CLAUDE.md", ...}`,
  - **When** `SafetyGate.Evaluate`가 호출되면,
  - **Then** `SafetyDecision.Outcome == DecisionRejected`, `RejectReason == "frozen_path"`, Layer 2-5는 실행되지 않는다(audit log에 Evaluated 시각 없음).

- **AC-SAFETY-002** (REQ-SAFETY-006 CanaryCheck 회귀)
  - **Given** 최근 3 session replay에서 평균 점수가 0.85 → 0.72 (drop=0.13)인 상황,
  - **When** `SafetyGate.Evaluate`가 호출되면,
  - **Then** `Outcome == DecisionRejected`, `RejectReason == "canary_regression"`, `LayerResults[1].Passed == false`.

- **AC-SAFETY-003** (REQ-SAFETY-007 Contradiction)
  - **Given** 기존 `Graduated` entry가 동일 `ZoneID="user_style"`에 "formal tone"을 유지 중이고, 새 entry가 "casual tone"을 제안하는 상황,
  - **When** `SafetyGate.Evaluate`가 호출되면,
  - **Then** `Outcome == DecisionNeedsReview`, `PendingReview.LearningID == new_entry.ID`, HOOK-001에 ApprovalRequest 이벤트가 발행된다.

- **AC-SAFETY-004** (REQ-SAFETY-011 주간 한도)
  - **Given** 현재 주에 이미 3개 entry가 Graduated 승격된 상태,
  - **When** 4번째 `SafetyGate.Evaluate`가 호출되면,
  - **Then** `Outcome == DecisionRejected`, `RejectReason == "rate_limit_weekly"`, HOOK 이벤트는 발행되지 않는다.

- **AC-SAFETY-005** (REQ-SAFETY-008, 009 HumanOversight 승인 경로)
  - **Given** 4개 layer를 통과한 entry,
  - **When** Layer 5에서 HOOK에 ApprovalRequest가 발행된 뒤 `ReceiveDecision(id, ApprovalApprove)`이 호출되면,
  - **Then** 새 `SafetyToken`이 발급되고 `VerifyToken`이 성공하며, REFLECT-001의 `MarkGraduated`가 이 토큰으로 호출 가능하다.

- **AC-SAFETY-006** (REQ-SAFETY-010 사용자 거부)
  - **Given** HOOK 이벤트 발행 후 `ReceiveDecision(id, ApprovalReject)`이 호출될 때,
  - **When** 내부 상태가 업데이트되면,
  - **Then** REFLECT-001의 `MarkRejected(id, reason="user_rejected")`가 호출되고, entry는 `RolledBack`이 아닌 `Rejected`로 전이된다.

- **AC-SAFETY-007** (REQ-SAFETY-013 토큰 재사용 차단)
  - **Given** 유효한 `SafetyToken`으로 `MarkGraduated`가 이미 1회 호출된 상태,
  - **When** 동일 토큰으로 다시 `VerifyToken` 또는 `MarkGraduated`가 시도되면,
  - **Then** `ErrTokenAlreadyUsed`가 반환된다.

- **AC-SAFETY-008** (REQ-SAFETY-003 audit log)
  - **Given** 임의의 Evaluate 호출 (결과 무관),
  - **When** 평가가 완료되면,
  - **Then** `.moai/evolution/safety-audit.log`에 JSON-L 1줄이 append되며, 해당 line은 `LearningID`, `Decision`, `LayerResults`, `EvaluatedAt`를 포함한다.

- **AC-SAFETY-009** (Go build + 테스트)
  - **Given** `internal/evolve/safety` 패키지,
  - **When** `go build ./... && go test -race -cover ./internal/evolve/safety/...`,
  - **Then** 85%+ 커버리지로 모든 테스트가 통과한다.

---

## 7. 기술적 접근

### 7.1 패키지 레이아웃

```
internal/evolve/safety/
├── types.go           # SafetyDecision, LayerResult, DecisionOutcome
├── gate.go            # SafetyGate + DefaultSafetyGate
├── frozenguard.go     # Layer 1
├── canary.go          # Layer 2
├── contradiction.go   # Layer 3
├── ratelimiter.go     # Layer 4
├── approval.go        # Layer 5 + ApprovalRequest + PendingApproval
├── tokens.go          # TokenSigner (Ed25519)
├── audit.go           # JSON-L append-only log
├── errors.go
└── testdata/
    ├── frozen_paths.yaml
    └── canary_fixtures/  # trajectory replay mocks
```

### 7.2 FrozenGuard 기본 경로

```go
var DefaultFrozenPaths = []string{
    "CLAUDE.md",
    ".claude/rules/core/**",
    ".claude/rules/moai/core/**",
    ".moai/project/learning-engine.md",
    ".moai/config/sections/user.yaml",
    ".moai/specs/SPEC-GENIE-SAFETY-001/**",   // 자기 자신 보호
    ".moai/specs/SPEC-GENIE-REFLECT-001/**",  // 의존 SPEC 보호
}
```

glob 매칭은 `doublestar` 또는 `filepath.Match` + 수동 `**` 처리.

### 7.3 CanaryCheck 알고리즘

1. 최근 N(=3)개 성공 session의 trajectory를 INSIGHTS-001의 저장소에서 로드
2. `ProposedChange`를 in-memory shadow copy에 적용
3. 각 trajectory를 shadow copy로 재평가 (satisfaction score 재계산)
4. `MaxScoreDrop = max(score_before - score_after)` 계산
5. Threshold 비교

scoring 함수는 INSIGHTS-001이 제공하는 `Score(trajectory, appliedChange) float64`를 호출. SAFETY는 알고리즘 자체는 알지 않는다.

### 7.4 ContradictionDetector 판정 규칙

- 동일 `ZoneID` AND 동일 `Category` AND 기존 `Graduated` entry의 `After` 문자열이 새 entry의 `Before` 문자열과 일치 → contradiction
- 상이한 `ZoneID`여도 동일 `TargetComponent`에서 상반된 키워드 쌍 검출 (예: `formal` vs `casual`, `brief` vs `detailed`) → contradiction
- 미감지 시 통과, 감지 시 `NeedsReview`

### 7.5 RateLimiter 구현

- Time-window counters: 주간(ISO week), 일간(UTC date), 시간당(UTC hour) 카운터 3개
- 파일별 마지막 변경 시각 맵 (`map[string]time.Time`)
- Persistent: `.moai/evolution/ratelimiter-state.json` (재시작 복원)
- 50 active 체크는 REFLECT의 `ListByStatus`로 조회

### 7.6 HumanOversight 흐름 (HOOK 연계)

```
SafetyGate.Evaluate
   ├── Layer 1-4 모두 통과
   ├── Layer 5 Contradiction 없음 OR ContradictionDetector NeedsReview
   ↓
HookEmitter.EmitApprovalRequest
   → HOOK-001 dispatcher
   → 상위 orchestrator (genied QueryEngine or CLI)
   → AskUserQuestion 표시
   → 사용자 선택
   ↓
SafetyGate.ReceiveDecision(id, option)
   ├── ApprovalApprove: TokenSigner.Issue → 반환
   ├── ApprovalReject: REFLECT.MarkRejected
   ├── ApprovalAskLater: PendingApproval 유지, deadline 연장
   └── ApprovalModify: 새 ProposedChange로 재평가
```

SAFETY는 **AskUserQuestion을 직접 호출하지 않는다**. HOOK-001 이벤트만 발행하고 외부 decision을 기다린다.

### 7.7 TokenSigner (Ed25519)

```go
type ed25519TokenSigner struct {
    privateKey ed25519.PrivateKey
    publicKey  ed25519.PublicKey
    used       map[string]time.Time // TokenID → used time (TTL 1h cleanup)
    mu         sync.RWMutex
}

func (s *ed25519TokenSigner) Issue(learningID string, now time.Time) (reflect.SafetyToken, error) {
    tokenID := uuidv7()
    expiresAt := now.Add(15 * time.Minute)
    payload := append([]byte(tokenID), []byte(learningID)...)
    payload = append(payload, timestampBytes(now)...)
    payload = append(payload, timestampBytes(expiresAt)...)
    sig := ed25519.Sign(s.privateKey, payload)
    return reflect.SafetyToken{
        TokenID: tokenID,
        IssuedAt: now,
        ExpiresAt: expiresAt,
        Signature: sig,
    }, nil
}
```

### 7.8 Audit log 포맷

```jsonl
{"ts":"2026-04-21T09:15:00Z","learning_id":"LEARN-20260421-003","decision":"approved","token_id":"01928a...","layers":[{"name":"FrozenGuard","passed":true},{"name":"CanaryCheck","passed":true,"score_drop":0.02},{"name":"ContradictionDetector","passed":true},{"name":"RateLimiter","passed":true,"weekly_count":2},{"name":"HumanOversight","passed":true,"option":"approve"}]}
```

append-only, rotation은 월 단위 (별도 로드맵 이슈에서 관리).

### 7.9 테스트 전략 (TDD)

- Layer 별 단위 테스트 (mock 나머지 4 layer)
- 통합 테스트: 5 layer 전체 파이프라인 (mock HookEmitter/Memory)
- FrozenGuard glob 매칭 테이블-driven 테스트
- CanaryCheck: fixture trajectory + scoring mock으로 경계값 0.09/0.10/0.11 검증
- RateLimiter: `fakeClock`으로 주간/일간/시간당 boundary 검증
- TokenSigner: Issue → Verify → MarkUsed → Verify (재사용 차단) 순서 검증
- Audit log: `tempDir` fixture로 JSON-L append 내용 검증

---

## 8. 리스크 & 완화

| 리스크 | 영향 | 완화 |
|-------|------|------|
| FrozenGuard 우회 (심볼릭 링크, 상대 경로) | 헌법 변조 가능 | `filepath.Abs` + `filepath.EvalSymlinks`로 정규화 후 매칭, 테스트 케이스 추가 |
| CanaryCheck replay 비용 | 매 승격마다 trajectory replay는 100ms+ 소요 가능 | 캐시 (동일 change hash에 대한 캐시 5분 TTL), async evaluation + 결과 대기 |
| ContradictionDetector 오탐 | 유의미한 진화가 NeedsReview로 귀결되어 UX 저하 | 초기 구현은 보수적 규칙만 적용하고, 감지 로그를 축적하여 SPEC-GENIE-REFLECT-002에서 임계값 튜닝 |
| SafetyToken 키 노출 | 무단 Graduated 승격 가능 | `~/.genie/keys/` 퍼미션 600, 키 로테이션 월 1회, 이전 키로 발급된 토큰은 rotation 즉시 invalidate |
| HumanOversight 영구 대기 | PendingApproval이 소멸되지 않아 rate limit 저점유 | 기본 deadline 72h, 초과 시 자동 `ApprovalAskLater`로 전환 후 entry는 `HighConfidence` 유지 |

---

## 9. 참고 (References)

- `.moai/project/learning-engine.md` §2.3 (5-layer 원형)
- `.claude/rules/agency/constitution.md` §5 (FROZEN, 계승 원천)
- `.claude/rules/agency/constitution.md` §12 (Evaluator Leniency Prevention — Contradiction 검증 영감)
- SPEC-GENIE-REFLECT-001 (LearningEntry, SafetyToken 소비자)
- SPEC-GENIE-HOOK-001 (HookEventApprovalRequest 수신자)
- SPEC-GENIE-INSIGHTS-001 (trajectory scoring API 제공자)
- SPEC-GENIE-ROLLBACK-001 (canary regression 재활용)
