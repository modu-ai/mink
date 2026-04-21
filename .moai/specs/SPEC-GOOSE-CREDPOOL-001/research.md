# SPEC-GOOSE-CREDPOOL-001 — Research

> Hermes `credential_pool.py` 원형 분석 + Go 포팅 전략 + 테스트 설계.

## 1. Hermes 원형 분석 (hermes-llm.md §1-3 인용)

### 1.1 원문 구조 (Python)

`hermes-agent-main/agent/credential_pool.py` (약 1.3K LoC):

```
CredentialPool(provider, entries)
  ├── select() → PooledCredential | None
  │    ├── _available_entries(clear_expired, refresh)
  │    ├── _refresh_entry(entry, force)  # OAuth only
  │    └── strategy 적용 (FILL_FIRST | ROUND_ROBIN | RANDOM | LEAST_USED)
  ├── mark_exhausted_and_rotate(status_code, error_context)
  ├── acquire_lease(credential_id)
  └── release_lease(credential_id)
```

인용(hermes-llm.md §3.1):

```python
@dataclass
class PooledCredential:
    provider: str
    id: str                         # UUID hex[:6]
    label: str
    auth_type: str                  # "oauth" | "api_key"
    priority: int
    source: str
    access_token: str
    refresh_token: Optional[str]
    last_status: Optional[str]      # "ok" | "exhausted"
    last_status_at: Optional[float]
    last_error_code: Optional[int]  # HTTP status
    last_error_reset_at: Optional[float]
    base_url: Optional[str]
    expires_at: Optional[str]
    expires_at_ms: Optional[int]
    agent_key: Optional[str]        # Nous-specific
    request_count: int = 0
    extra: Dict[str, Any] = None
```

### 1.2 핵심 흐름 (인용 §1)

1. **select**: strategy 기반 + availability filtering + (필요시) refresh
2. **refresh**: OAuth entry 자동 갱신 (expiring 감지)
3. **Exhaustion**: HTTP 429/402 → STATUS_EXHAUSTED + cooldown 기록
4. **Rotation**: 현재 entry 소진 → 다음 가용 entry
5. **Persistence**: JSON 상태 저장

### 1.3 Storage 위치 (인용 §1)

- Anthropic Claude Code: `~/.claude/.credentials.json`
- OpenAI Codex: `~/.codex/auth.json`
- Nous: `~/.hermes/auth.json`

### 1.4 Selection Strategy 4종 (인용 §1)

- `FILL_FIRST` (우선순위 순서)
- `ROUND_ROBIN` (공평)
- `RANDOM` (부하 분산)
- `LEAST_USED` (사용 빈도 기반)

## 2. Python → Go 의미 보존 매핑

| Python 개념 | Go 매핑 | 비고 |
|---|---|---|
| `asyncio.Lock` | `sync.Mutex` | 본 SPEC은 단순 mutex로 충분 (네트워크 refresh는 Unlock 구간에서) |
| `Optional[float]` as timestamp | `time.Time` (IsZero() 검사) | ms epoch은 영속 JSON에서만 유지 |
| `Dict[str, Any] extra` | `map[string]any` | 확장 슬롯 |
| `dataclass(frozen=False)` | struct with unexported fields | mutation은 Pool 메서드 경유 강제 |
| `random.SystemRandom` | `crypto/rand` + `math/big` | Random 전략 |
| `time.time()` | `time.Now()` (혹은 `PoolOptions.Clock` 주입 for testability) | |
| Python `__repr__` | `String() string` (redacted) | 비밀값 마스킹 |
| `json.dumps(..., indent=2)` | `encoding/json` + temp file + rename | atomic write |

## 3. 고리스크 영역과 대응 (hermes-llm.md §12 인용)

### 3.1 "Anthropic OAuth PKCE 재구현" (원문 인용)

본 SPEC은 PKCE 재구현을 **ADAPTER-001에 위임**. 풀은 `Refresher` interface만 의존.

```go
type Refresher interface {
    Refresh(ctx context.Context, entry *PooledCredential) (RefreshResult, error)
}
```

ADAPTER-001의 Anthropic 어댑터가 이 interface를 구현하며, `~/.claude/.credentials.json`의 `claudeAiOauth.refreshToken`으로 Anthropic 토큰 엔드포인트에 POST → single-use 새 refresh_token 반환.

### 3.2 "여러 provider async client 통합" (원문 인용)

Go는 goroutine + channel이 네이티브이므로 Python asyncio보다 단순. 본 SPEC은 async 없이 동기 mutex 경로. 동시성은 `Select()` 동시 호출 허용(REQ-CREDPOOL-003).

### 3.3 Refresh 연속 실패 처리

Python 원본은 명확한 정책 없음. 본 SPEC은 REQ-CREDPOOL-013에서 "3회 연속 실패 → 영구 고갈 + 운영자 개입 필요"로 확정.

이유:
- refresh_token 자체가 revoke되었을 가능성 高
- 무한 retry는 Anthropic rate limit 악화 초래
- 운영자 `Pool.Reset(id)` 또는 재로그인(ADAPTER-001 PKCE 플로우)으로 해결

## 4. 테스트 전략

### 4.1 테스트 레이어

**Layer 1 — Unit (패키지 내부)**:
- `strategy.go` 각 전략별 순서 검증
- `credential.go` `String()` redaction
- `storage.go` atomic write 원자성 (테스트: tmp 파일이 rename 이전에는 존재하지만 최종 파일과 다름)

**Layer 2 — Integration (패키지 단위)**:
- Stub `Refresher` (테스트용 `refresherSpy`)
- Stub `Storage` (in-memory map)
- Stub `CredentialSource` (in-memory slice)
- 12개 AC 각각에 대응하는 `*_test.go`

**Layer 3 — Cross-SPEC (후속 SPEC 조합)**:
- CREDPOOL + ADAPTER-001 fixture로 실제 Anthropic OAuth refresh 왕복 (제한된 rate, 수동 승인 필요한 테스트는 `_manual_test.go`)

### 4.2 stub 구현 예시

```go
type refresherSpy struct {
    callCount int
    result    RefreshResult
    err       error
}

func (r *refresherSpy) Refresh(ctx context.Context, entry *PooledCredential) (RefreshResult, error) {
    r.callCount++
    return r.result, r.err
}
```

### 4.3 고려해야 할 edge case

| # | Edge Case | 검증 방법 |
|---|-----------|---------|
| 1 | 엔트리 0개로 `Select` | `ErrExhausted{RetryAfter:0}` |
| 2 | 유일 엔트리가 refresh 실패 3회 | REQ-CREDPOOL-013 발동 → 영구 고갈 |
| 3 | 동일 ID에 `MarkExhaustedAndRotate` 중복 호출 | idempotent: 두 번째 호출은 `last_error_reset_at` 덮어쓰기만 |
| 4 | `RefreshSources`가 삭제된 엔트리 발견 | 풀에서 제거, `request_count`는 로그로 기록 |
| 5 | 외부 파일 파싱 실패 | source별 error, 풀 전체는 계속 (다른 source 사용 가능) |
| 6 | 시계 되감김(NTP sync) 시 `last_error_reset_at > now` 재계산 | `time.Now()` 기반 비교, `Clock` 인터페이스로 테스트 |
| 7 | refresh 중 ctx 취소 | `Refresher.Refresh`가 ctx 존중. 풀은 `ctx.Err() != nil` 체크 후 해당 엔트리 skip |
| 8 | atomic write 중 crash | 다음 start에서 temp 파일 존재 → 복구 로직으로 무시, 이전 영속 파일이 권위 |

## 5. Go 라이브러리 결정 (본 SPEC에서 채택만)

| 역할 | 라이브러리 | 근거 |
|-----|----------|------|
| 구조화 로깅 | `go.uber.org/zap` v1.27+ | CORE-001 상속 |
| JSON marshal | stdlib `encoding/json` | 표준 |
| 테스트 | `github.com/stretchr/testify` v1.9+ | CORE-001 상속 |
| 동시성 | stdlib `sync`, `context` | 표준 |
| 시간 mocking | `PoolOptions.Clock func() time.Time` | 외부 lib 없이 자체 주입 |

**본 SPEC은 외부 OAuth 라이브러리를 도입하지 않는다**. 이유: Refresher 인터페이스가 ADAPTER-001로 위임되므로, 풀 자체는 OAuth spec-unaware.

ADAPTER-001에서 채택 예정:
- `github.com/anthropics/anthropic-sdk-go` (공식 Anthropic Go SDK, OAuth 포함)
- `github.com/sashabaranov/go-openai` (OpenAI-compat. xAI/DeepSeek/Groq 공용)
- `google.golang.org/genai` (Gemini)
- `github.com/ollama/ollama/api` (Ollama)
- `github.com/pkoukk/tiktoken-go` (토큰 카운팅)

## 6. 영속 스키마 버전 정책

v1 스키마(본 SPEC). 미래 확장 시 `"version": 2`로 증가하며, loader는:
- v1 파일 → v1 loader 경로
- v2+ → 새 loader 경로
- 미지 버전 → WARN + 메모리 상태만 사용(파일 무시)

**비밀값은 영속 파일에 저장하지 않는다** (REQ-CREDPOOL-014 보강). 따라서 스키마 마이그레이션은 상태(status/count/timestamps)만 대상.

## 7. 성능 목표

| 메트릭 | 목표 | 측정 방법 |
|-----|------|-------|
| `Select()` p99 (refresh 없음) | < 100μs | `go test -bench` |
| `Select()` p99 (refresh 포함, stub) | < 1ms | 〃 |
| `MarkExhaustedAndRotate` p99 | < 500μs | 〃 |
| atomic write throughput | > 1,000 ops/sec (tmp+rename) | 〃 |

## 8. 관측성

`PoolStats` 노출:
```go
type PoolStats struct {
    Provider         string
    Total            int
    Available        int
    Exhausted        int
    Pending          int
    RefreshesTotal   int64
    RotationsTotal   int64
    SelectsTotal     int64
    RefreshFailures  int64
}
```

후속 SPEC(메트릭 export)에서 Prometheus 라벨로 변환 예정.

## 9. 구현 순서 (TDD 계획)

순번은 SPEC §6.6과 일치.

| # | Test Name | AC |
|---|---|---|
| 1 | TestPool_Select_FillFirst_ReturnsHighestPriority | AC-001 |
| 2 | TestPool_Select_RoundRobin_WrapsAround | AC-002 |
| 3 | TestPool_OAuthRefreshAutoOnExpiring | AC-003 |
| 4 | TestPool_MarkExhaustedAndRotate_OnHTTP429 | AC-004 |
| 5 | TestPool_AllExhausted_ReturnsErrExhausted | AC-005 |
| 6 | TestPool_PersistsStateAcrossRecreation | AC-006 |
| 7 | TestSources_AnthropicClaudeFileSync | AC-007 |
| 8 | TestPool_LeaseCounterAffectsLeastUsed | AC-008 |
| 9 | TestPool_Refresh3FailuresMarksPermanentExhausted | AC-009 |
| 10 | TestPool_LogsRedactSecrets | AC-010 |

## 10. 오픈 이슈

- **Q1**: ROUND_ROBIN 인덱스를 영속할지? 현 설계는 in-memory만. 프로세스 재시작 시 0부터 다시 시작 → 약간의 불공평 허용.
- **Q2**: `RefreshSources`는 CONFIG-001 변경과 어떻게 연계? 현 설계는 명시적 호출. 향후 CONFIG의 `Reload()`(hot reload SPEC)이 자동 호출 가능.
- **Q3**: `permanent exhausted` 해제는 `Pool.Reset(id)`만. CLI 명령(`goose pool reset <id>`)은 CLI-001 범위.

---

**End of Research (SPEC-GOOSE-CREDPOOL-001)**
