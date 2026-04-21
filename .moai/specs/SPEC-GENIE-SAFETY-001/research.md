# Research — SPEC-GENIE-SAFETY-001 (5-Layer Safety)

> **작성일**: 2026-04-21
> **대상 SPEC**: SPEC-GENIE-SAFETY-001
> **목적**: agency constitution §5 계승 근거, 5 layer 각각의 실패 모드 분석, TDD 테스트 전략 확정.

---

## 1. 원천 문서 정리

### 1.1 agency/constitution §5 전문 인용 (FROZEN)

agency constitution은 본 SPEC의 직접 원천이며 FROZEN 영역이다. 다섯 layer 각각의 원문:

**Layer 1 Frozen Guard**:
> "Before any evolution write operation, the system checks:
> - Target file is NOT in the FROZEN zone
> - Target field is NOT a frozen configuration key
> - Modification does not weaken safety thresholds
> Violation response: Block the write, log the attempt, notify the user."

**Layer 2 Canary Check**:
> "Before applying any evolved change, the Canary layer runs a shadow evaluation:
> - Apply the proposed change in memory (not on disk)
> - Re-evaluate the last 3 projects against the modified rules
> - If any project score drops by more than 0.10, reject the change
> - Log the canary result regardless of outcome."

**Layer 3 Contradiction Detector**:
> "When a new learning contradicts an existing rule or heuristic:
> - Flag the contradiction with both the old and new rule text
> - Present both options to the user with context
> - Never silently override an existing rule
> - Record the resolution in .agency/evolution/contradictions.log"

**Layer 4 Rate Limiter**:
> "Evolution velocity is bounded to prevent runaway self-modification:
> - Maximum 3 evolutions per week (max_evolution_rate_per_week)
> - Minimum 24-hour cooldown between evolutions (cooldown_hours)
> - No more than 50 active learnings at any time (max_active_learnings)
> - Older learnings are archived when the limit is reached"

**Layer 5 Human Oversight**:
> "All evolution proposals require human approval when require_approval is true:
> - Present the proposed change with before/after diff
> - Show supporting evidence (observation count, confidence score)
> - Provide one-click approve/reject via AskUserQuestion
> - Log the decision with timestamp and rationale"

### 1.2 learning-engine.md §2.3 직접 인용

```go
// FrozenGuard
var FrozenPaths = []string{
    "CLAUDE.md",
    ".claude/rules/core/*",
    ".claude/rules/moai/core/*",
    ".moai/project/learning-engine.md",  // 자기 자신!
    ".moai/config/sections/user.yaml",   // 사용자 식별자
}

// RateLimiter
MaxPerWeek   = 3
FileCooldown = 24 * time.Hour
MaxActive    = 50
HourlyLimit  = 1
DailyLimit   = 2

// Rollback
RegressionThreshold = 0.10
```

본 SPEC은 위 수치를 그대로 계승한다.

---

## 2. 핵심 설계 결정과 근거

### 2.1 "AskUserQuestion 직접 호출 금지" 원칙

constitution.md와 agent-common-protocol.md는 **subagent가 AskUserQuestion을 호출하지 않는다**를 HARD 규칙으로 명시한다. SAFETY-001의 HumanOversight layer는 genied 데몬 내부에서 실행되므로 subagent와 동등하게 취급하여 AskUserQuestion을 직접 호출하지 않는다.

**대신**: HOOK-001의 `HookEventApprovalRequest` 이벤트만 발행하고, 상위 호출자(CLI orchestrator 또는 MoAI orchestrator)가 AskUserQuestion을 표시한 뒤 `ReceiveDecision(id, option)`으로 결과를 주입한다.

이 결정은 향후 GENIE가 다양한 frontend(CLI, desktop, mobile)에서 동작할 때에도 승인 경로를 일관되게 유지하기 위함이다.

### 2.2 SafetyToken Ed25519 선택 이유

대안 1: HMAC-SHA256 + 공유 비밀. → 거부 이유: 대칭키는 탈취 시 임의 토큰 발급 가능. 개인 환경이지만 키 교체 UX가 복잡.
대안 2: JWT RS256. → 거부 이유: RSA 키는 4096비트 기준 512B+로 크고, 서명/검증 비용이 높다.
**결정**: Ed25519. 32B 개인키 + 32B 공개키, 서명 비용 < 1ms, Go 표준 라이브러리 `crypto/ed25519` 지원.

### 2.3 15분 토큰 만료의 근거

- 사용자가 AskUserQuestion에 응답하는 평균 시간: 10초 ~ 수 분
- 72시간 deadline은 `PendingApproval` 단위이고, 토큰 자체는 승인 즉시 발급 → 즉시 MarkGraduated 호출 경로
- 15분이면 승인 후 LoRA 교체나 CLAUDE.md 수정 같은 후속 작업 충분
- 동시에 긴 유예는 키 노출 시 위험 증대

### 2.4 `used_tokens` 캐시 (1회성 사용)

토큰이 1회 Graduated 승격에만 사용되도록 강제하기 위해, 사용된 `TokenID`를 메모리 맵에 기록하고 만료 시각 이후까지 유지(+1h grace period). 재사용 시도는 즉시 거부.

SAFETY 프로세스 재시작 시 used cache는 소실되지만, 토큰 만료가 15분이므로 재시작 후 15분만 지나면 안전. 더 강한 보장이 필요하면 `.moai/evolution/used-tokens.json`에 영속화 가능하나 초기 구현은 in-memory.

### 2.5 CanaryCheck가 "최근 3 session"인 이유

MoAI(agency)는 "최근 3 project"를 기준으로 한다. GENIE는 개인 환경이므로 project 개념이 없고 대신 **trajectory session**을 단위로 삼는다.

3개 세션은 통계적 최소 표본이며, INSIGHTS-001이 제공하는 trajectory storage에서 가장 최근 성공(completed=true) 3개를 선택한다. 실패 trajectory는 scoring이 불가능하므로 제외.

### 2.6 ContradictionDetector의 보수적 설계

초기 버전은 **거짓 음성(false negative)을 허용**하고 **거짓 양성(false positive)을 최소화**한다.
- Strict rule: 동일 ZoneID + 대립 키워드 쌍(formal/casual, brief/detailed 등) → 감지
- 관대한 rule: 다른 ZoneID의 관련 영역 → 감지하지 않음

이유: false positive가 많으면 NeedsReview가 남발되어 사용자가 알림 피로를 겪고, 결국 승인을 기계적으로 누르는 `approval fatigue`가 발생한다. learning-engine.md §12.4 "Personalization Bubble" 리스크와 유사한 메커니즘.

---

## 3. Layer 간 실행 순서 근거

5 layer 순서는 agency constitution과 동일하게 고정:

1. **FrozenGuard (가장 빠름, 즉시 실패 가능)**: path 매칭만으로 판정. CPU μs 단위. 명백한 위반부터 걸러낸다.
2. **CanaryCheck (CPU-bound)**: 3 session replay + scoring. 수 ms ~ 100 ms. 의미론적 회귀 판정.
3. **ContradictionDetector (메모리 검색)**: 기존 Graduated entry 순회. 평균 수십 μs.
4. **RateLimiter (카운터 체크)**: 카운터 조회 및 비교. μs 단위.
5. **HumanOversight (가장 느림, 비동기)**: 사용자 응답 대기. 수 초 ~ 수 시간.

**첫 4개 layer는 동기 평가**, **Layer 5만 비동기**. 첫 4개가 통과하면 HOOK 이벤트 발행 → ReceiveDecision까지 대기 (차단 아님, caller는 NeedsReview로 응답 받고 나중에 결과 조회).

---

## 4. Go 패키지 레이아웃 상세

```
internal/evolve/safety/
├── types.go            # SafetyDecision, LayerResult
├── gate.go             # SafetyGate 인터페이스 + DefaultSafetyGate 조립
├── frozenguard.go      # FrozenGuard.Check glob 매칭
├── canary.go           # Canary.Check + shadow eval
├── contradiction.go    # ContradictionDetector rules
├── ratelimiter.go      # RateLimiter + persistent state
├── approval.go         # ApprovalRequest, PendingApproval
├── tokens.go           # TokenSigner (Ed25519)
├── audit.go            # JSON-L append-only
├── errors.go
├── internal/
│   ├── globmatch.go
│   └── clock.go
└── testdata/
    ├── frozen_paths.yaml
    ├── canary_trajectories/
    │   ├── session_001.jsonl
    │   ├── session_002.jsonl
    │   └── session_003.jsonl
    └── contradiction_rules.yaml
```

---

## 5. TDD 테스트 전략

### 5.1 Layer별 단위 테스트

**FrozenGuard** (`frozenguard_test.go`):
- `TestFrozenGuard_CLAUDE_md_blocked`
- `TestFrozenGuard_moai_core_rule_blocked`
- `TestFrozenGuard_symlink_resolved_before_match`
- `TestFrozenGuard_user_custom_path_added`
- `TestFrozenGuard_lora_adapter_path_allowed`

**Canary** (`canary_test.go`):
- `TestCanary_drop_0_09_passes` (경계 통과)
- `TestCanary_drop_0_10_passes` (임계값 통과)
- `TestCanary_drop_0_11_fails` (임계값 위반)
- `TestCanary_insufficient_sessions_returns_need_more_data`
- `TestCanary_custom_threshold_from_config`

**Contradiction** (`contradiction_test.go`):
- `TestContradiction_same_zone_opposite_keyword_detected`
- `TestContradiction_different_zone_not_detected`
- `TestContradiction_no_existing_graduated_returns_empty`

**RateLimiter** (`ratelimiter_test.go`):
- `TestRateLimiter_weekly_3_passes`
- `TestRateLimiter_weekly_4_fails`
- `TestRateLimiter_file_cooldown_24h` (23h 실패, 25h 통과)
- `TestRateLimiter_hourly_1_then_second_fails`
- `TestRateLimiter_daily_2_then_third_fails`
- `TestRateLimiter_persistent_state_restored_after_restart`

**TokenSigner** (`tokens_test.go`):
- `TestTokenSigner_issue_and_verify`
- `TestTokenSigner_expired_token_rejected`
- `TestTokenSigner_tampered_signature_rejected`
- `TestTokenSigner_reuse_blocked_after_mark_used`
- `TestTokenSigner_different_learning_id_rejected`

### 5.2 통합 테스트 (Gate 전체)

`gate_integration_test.go`:

- `TestGate_approved_path`: 모든 4 layer 통과 + HOOK 이벤트 발행 확인 + ApprovalApprove 수신 → token 발급
- `TestGate_rejected_at_layer_1`: FrozenGuard에서 즉시 실패, 나머지 layer 호출되지 않음
- `TestGate_rejected_at_layer_2`: Canary에서 실패, 나머지 layer 호출되지 않음
- `TestGate_needs_review_at_layer_3`: Contradiction에서 NeedsReview, Layer 4는 호출됨, Layer 5도 호출됨
- `TestGate_rate_limit_rejects_before_hook_emit`: RateLimiter 실패 시 HOOK 발행되지 않는지 확인
- `TestGate_audit_log_written_on_every_outcome`: 승인/거부/NeedsReview 3 케이스 모두 audit log에 기록 확인

### 5.3 Mock 설계

```go
type mockHookEmitter struct {
    emitted []safety.ApprovalRequest
}

func (m *mockHookEmitter) EmitApprovalRequest(ctx context.Context, req safety.ApprovalRequest) error {
    m.emitted = append(m.emitted, req)
    return nil
}

type mockReflectMemory struct {
    graduated []reflect.LearningEntry
}

// ... 등
```

### 5.4 Property-based 테스트

- 속성: "5 layer 중 하나라도 실패하면 최종 결정은 Rejected 또는 NeedsReview (Approved 아님)"
- 속성: "같은 LearningEntry를 Evaluate 2회 호출하면 audit log에 2 record 생성"
- 속성: "RateLimiter가 한도 초과 상태에서는 어떤 변경도 통과하지 못한다"

### 5.5 Race detector 필수

HumanOversight는 비동기이므로 Evaluate 진행 중 ReceiveDecision이 호출되는 race 시나리오가 현실적.

`TestGate_race_evaluate_and_receive_decision`을 `-race`와 함께 실행하여 데이터 경합 부재 확인.

---

## 6. 보안 고려사항

### 6.1 키 관리

- `~/.genie/keys/safety-signer.ed25519` (600 permission)
- 키 부재 시 첫 실행 시 자동 생성
- 백업: 사용자가 명시적으로 export 가능 (별도 CLI 커맨드, 본 SPEC 범위 외)
- 로테이션: 월 1회 권장 (향후 SPEC)

### 6.2 Audit log 보호

- 기본 경로: `.moai/evolution/safety-audit.log` (project scope)
- 파일 권한 644 (읽기는 허용, 쓰기는 genied만)
- Append-only: 파일 핸들 `O_APPEND|O_WRONLY|O_CREATE` 플래그
- Rotation: 100MB 초과 시 `.1`, `.2` 로 gzip 회전 (별도 이슈)

### 6.3 FrozenPath 우회 방지

```go
func normalizePath(path string) (string, error) {
    abs, err := filepath.Abs(path)
    if err != nil { return "", err }
    resolved, err := filepath.EvalSymlinks(abs)
    if err != nil {
        if os.IsNotExist(err) {
            // 아직 생성 전 파일도 FrozenGuard 대상
            return abs, nil
        }
        return "", err
    }
    return resolved, nil
}
```

---

## 7. 성능 고려사항

| Layer | 목표 latency | 비고 |
|-------|------------|------|
| FrozenGuard | < 100μs | 10개 glob 패턴 매칭 |
| CanaryCheck | < 100ms | 3 session replay + scoring (INSIGHTS 호출) |
| ContradictionDetector | < 1ms | 최대 50 graduated entry 순회 |
| RateLimiter | < 100μs | 카운터 + 파일 맵 조회 |
| HumanOversight (async) | N/A | 사용자 응답 대기 |

첫 4 layer 합산 < 200ms. CanaryCheck 5분 TTL 캐시 적용 시 반복 평가는 < 1ms.

---

## 8. 오픈 질문

1. **Canary scoring 알고리즘**: INSIGHTS-001이 어떤 scoring 함수를 노출할지 (BLEU-like? 사용자 만족도 proxy?) → INSIGHTS-001에서 최종 결정, SAFETY는 `Score(trajectory, change) float64` 인터페이스만 의존
2. **ContradictionDetector의 ML 고도화**: 초기 rule-based → 향후 embedding 기반 semantic similarity로 확장할지 → Phase 5 이후 별도 SPEC
3. **SafetyToken 저장소**: 메모리 전용 vs 파일 영속 → 초기 메모리, 필요시 SPEC-GENIE-SAFETY-002에서 영속화 추가
4. **HookEmitter 실패 처리**: HOOK-001이 unavailable할 때 HumanOversight를 건너뛸지(fail-closed) vs 허용할지(fail-open) → fail-closed로 결정 (승인 없이는 결코 Approved 반환 안 함)
5. **Multi-user 시나리오**: 여러 사용자가 동일 genied 인스턴스 공유 시 FrozenPath가 사용자별? 전역? → 초기 전역, SPEC-GENIE-MULTIUSER-001(미존재)에서 개선

---

## 9. 구현 순서 (TDD 루프)

| 순서 | 테스트 파일 | 대상 코드 |
|-----|-----------|---------|
| 1 | `types_test.go` | SafetyDecision, LayerResult 정의 |
| 2 | `frozenguard_test.go` | Layer 1 |
| 3 | `ratelimiter_test.go` | Layer 4 (가장 단순) |
| 4 | `tokens_test.go` | TokenSigner (Ed25519) |
| 5 | `canary_test.go` | Layer 2 (mock INSIGHTS scoring) |
| 6 | `contradiction_test.go` | Layer 3 |
| 7 | `approval_test.go` | ApprovalRequest, PendingApproval |
| 8 | `audit_test.go` | JSON-L append 검증 |
| 9 | `gate_integration_test.go` | 5 layer 통합 + race |

---

## 10. 참조 인덱스

- `.claude/rules/agency/constitution.md` §5, §12, §15 (FROZEN 원천)
- `.moai/project/learning-engine.md` §2.3 (수치)
- `.claude/rules/moai/core/agent-common-protocol.md` (AskUserQuestion 직접 호출 금지 규칙)
- Go 표준 라이브러리 `crypto/ed25519` 문서
- SPEC-GENIE-REFLECT-001 (LearningEntry/SafetyToken 소비자)
- SPEC-GENIE-HOOK-001 (HookEventApprovalRequest 수신자)
- SPEC-GENIE-INSIGHTS-001 (trajectory scoring API)
- SPEC-GENIE-ROLLBACK-001 (CanaryCheck 결과 재활용)

---

Version: 0.1.0
Created: 2026-04-21
Status: Research complete, ready for RED phase test authoring
