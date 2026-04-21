# SPEC-GOOSE-ERROR-CLASS-001 — Research & Porting Analysis

> **목적**: Hermes `error_classifier.py` 28KB의 14종 FailoverReason 분류 체계를 Go로 이식할 때의 결정점, 정규식 재사용, provider별 특수 케이스를 정리한다.
> **작성일**: 2026-04-21
> **범위**: `internal/evolve/errorclass/` 패키지.

---

## 1. 레포 현재 상태 스캔

```
$ ls /Users/goos/MoAI/AgentOS/hermes-agent-main/agent/
error_classifier.py          # 28KB, 14 FailoverReason 본체
```

- `internal/evolve/errorclass/` → **전부 부재**. Phase 4에서 신규 작성.
- 상위 CORE-001이 수립할 zap 로거, context, Go module 구성 전제.
- ADAPTER-001(아직 미구현)이 본 SPEC의 주 consumer가 될 예정.

---

## 2. hermes-learning.md §5 원문 → SPEC 매핑

hermes-learning.md §5 FailoverReason 원문:

```python
class FailoverReason(Enum):
    auth = "auth"
    auth_permanent = "auth_permanent"
    billing = "billing"
    rate_limit = "rate_limit"
    overloaded = "overloaded"
    server_error = "server_error"
    context_overflow = "context_overflow"
    payload_too_large = "payload_too_large"
    model_not_found = "model_not_found"
    timeout = "timeout"
    format_error = "format_error"
    thinking_signature = "thinking_signature"
    unknown = "unknown"
```

**Hermes 원본 12종** + 본 SPEC 추가 2종:
- `transport_error` (네트워크 스택 오류로 timeout/overflow 둘 다 아닌 경우)
- `Unknown`은 Hermes도 존재

∴ 14종으로 확장.

### 2.1 분류 파이프라인 원문

hermes-learning.md §5:

```
1. Provider 특화 패턴 (Anthropic thinking_signature, long_context_tier)
2. HTTP 상태 코드 + 메시지 정제
   401→auth, 402→billing/rate_limit, 403→auth/billing, 404→model_not_found,
   413→payload_too_large, 429→rate_limit, 400→context_overflow/format,
   500/502→server_error, 503/529→overloaded
3. Error code 분류 (body.error.code)
   resource_exhausted→rate_limit, insufficient_quota→billing,
   context_length_exceeded→context_overflow
4. Message 패턴 매칭 (case-insensitive)
   _BILLING_PATTERNS, _RATE_LIMIT_PATTERNS, _CONTEXT_OVERFLOW_PATTERNS, _AUTH_PATTERNS
5. Transport 휴리스틱
   ReadTimeout/ConnectTimeout→timeout
   Server disconnect + (tokens>60% OR >120K OR msgs>200)→context_overflow
6. Fallback: unknown (retryable=True)
```

본 SPEC의 5단계는 Hermes 1-5단계와 1:1 동일(단계 6 fallback은 파이프라인 외).

### 2.2 ClassifiedError 원문

```python
@dataclass
class ClassifiedError:
    reason: FailoverReason
    status_code: Optional[int]
    retryable: bool
    should_compress: bool
    should_rotate_credential: bool
    should_fallback: bool
    message: str
```

Go 재작성:
- `Optional[int]` → `int` 값 0이 미지정(명시성 vs edge 방어: 0은 HTTP 유효 status 아님, 안전)
- `@dataclass` → Go struct + 필드 태그
- 추가: `MatchedBy string` — 어느 stage에서 매칭됐는지(디버깅 + INSIGHTS 집계 보조)
- 추가: `RawError error` — unwrap 체인 보존

---

## 3. Python → Go 이식 결정

### 3.1 Enum 표현

Python `Enum` → Go `iota int` + `String()` method.

```go
type FailoverReason int
const (
    Unknown FailoverReason = iota
    Auth
    // ...
)

func (r FailoverReason) String() string {
    switch r {
    case Unknown: return "unknown"
    case Auth: return "auth"
    // ...
    }
}
```

**선호 이유**:
- `MarshalText` / `UnmarshalText` 구현으로 JSON / YAML 자연스럽게 통합.
- TRAJECTORY-001의 `FailureReason` 필드에 그대로 직렬화.
- 성능: int 비교는 switch 기반, 수 ns.

### 3.2 Regex 전략

| 결정 | 근거 |
|---|---|
| Go 표준 `regexp` (RE2) | 백트래킹 없음, DoS 안전 |
| `(?i)` case-insensitive flag 사용 | Python `re.IGNORECASE` 대응 |
| Pre-compile at init (package var) | 매 호출 컴파일 피함, test에서 한 번만 verify |
| Pattern 테스트 케이스 표 | data-driven test (`t.Run("case name")`) |

### 3.3 Stage 2→4 Override 처리

Hermes는 HTTP 400이 ambiguous(context_length vs format_error)라 단계 2 직후 단계 4 검사. 본 SPEC은 이를 **일반화**:

```go
// Stage 2 matched but message is more specific → prefer message regex
if r, ok := matchHTTPStatus(meta.StatusCode); ok {
    if r2, ok := matchMessageRegex(err.Error()); ok && r2 != r {
        return build(r2, "stage4_message")   // override
    }
    return build(r, "stage2_http")
}
```

근거 (REQ-ERRCLASS-022): HTTP status는 hint, message가 final.

### 3.4 Transport 휴리스틱 임계치

Hermes: 60% / 120K / 200 msgs (3-way OR).

| 임계치 | 근거 | Override 가능? |
|---|---|---|
| `ContextLength * 0.6` | 대부분의 provider가 컨텍스트 60% 이후부터 성능 저하 | ✓ `ClassifierOptions.TransportThresholds` |
| `> 120K tokens` | Claude Sonnet 200K limit의 60% ≈ 120K | ✓ 동일 |
| `> 200 messages` | 경험칙, OpenAI/Anthropic 공통 | ✓ 동일 |

본 SPEC은 Hermes 값을 기본값으로 유지, 확장 가능 인터페이스 제공(R5 완화).

---

## 4. 14 FailoverReason 결정 근거

| Reason | Hermes 원본? | 본 SPEC 추가 근거 |
|---|:-:|---|
| `Unknown` | ✓ | fallback |
| `Auth` | ✓ | 일시적 401 |
| `AuthPermanent` | ✓ | 영구 403, key revoked |
| `Billing` | ✓ | 402, insufficient_quota |
| `RateLimit` | ✓ | 429 |
| `Overloaded` | ✓ | 503/529 |
| `ServerError` | ✓ | 500/502 |
| `ContextOverflow` | ✓ | 400 context_length + transport heuristic |
| `PayloadTooLarge` | ✓ | 413 |
| `ModelNotFound` | ✓ | 404 model_not_found |
| `Timeout` | ✓ | ReadTimeout/ConnectTimeout |
| `FormatError` | ✓ | 400 invalid JSON |
| `ThinkingSignature` | ✓ | Anthropic 특화 |
| `TransportError` | **신규** | 연결 재설정, EOF 등 — `Timeout`이나 `ContextOverflow`가 아닌 네트워크 오류 |

`TransportError` 추가 근거: Hermes는 Python `httpx` 오류를 `Unknown`으로 분류하지만, GOOSE는 Go `net` 패키지의 명확한 `net.Error`를 구분해 재시도 정책 다르게 적용할 수 있다(타임아웃은 즉시 재시도 OK, 일반 transport는 잠깐 대기 후 재시도).

---

## 5. 4-flag 정책 결정 근거

각 flag는 **downstream 호출자의 반응 유형**:

- `Retryable`: 즉시 동일 credential로 한 번 더 시도 가능?
- `ShouldCompress`: 컨텍스트 축소가 오류 해결에 도움?
- `ShouldRotateCredential`: 다른 API key로 회전?
- `ShouldFallback`: 다른 모델로 전환?

| Reason × Flag 조합 | 논리적 근거 |
|---|---|
| `Auth {R:T, Rc:T}` | 일시적 인증 오류 → 새 토큰으로 재시도 |
| `AuthPermanent {Rc:T, F:T}` | 영구 거부 → 다른 키 + 다른 모델 |
| `Billing {Rc:T, F:T}` | 잔액 없음 → 다른 계정 + 저렴한 모델 |
| `RateLimit {R:T, Rc:T}` | 한도 초과 → 다른 키(RATELIMIT-001이 Retry-After 처리) |
| `Overloaded {R:T, F:T}` | 서버 부하 → 잠시 후 재시도 OR 다른 모델 즉시 |
| `ContextOverflow {R:T, C:T}` | 컨텍스트 과다 → 압축 후 재시도 |
| `PayloadTooLarge {R:T, C:T}` | 요청 과다 → 압축 후 재시도 |
| `ModelNotFound {F:T}` | 모델 부재 → 다른 모델 |
| `Timeout {R:T}` | 타임아웃 → 재시도만 |
| `FormatError {all:F}` | JSON 오류 → 버그, 재시도 불가 |
| `ThinkingSignature {F:T}` | Anthropic 특화 프로토콜 오류 → 다른 모델 |
| `TransportError {R:T}` | 네트워크 오류 → 재시도만 |
| `Unknown {R:T}` | 미지, 한 번 시도 |

---

## 6. Go 라이브러리 결정

| 용도 | 채택 | 대안 | 근거 |
|---|---|---|---|
| Regex | 표준 `regexp` (RE2) | `dlclark/regexp2` | 보안(backtracking 없음) |
| Error wrapping | 표준 `errors` | `pkg/errors` | Go 1.20+의 errors.Is/As/Join 충분 |
| Net error | 표준 `net` | — | `net.Error` 인터페이스 표준 |
| HTTP status constants | 표준 `net/http` | — | `http.StatusUnauthorized` 등 |
| 테스트 | `testify` v1.9+ | — | 전 레포 일관 |

---

## 7. 테스트 전략

### 7.1 Data-Driven 테스트

14 reason × 다중 케이스 → table-driven test:

```go
func TestClassify_HTTPStatus(t *testing.T) {
    cases := []struct {
        name     string
        status   int
        provider string
        errMsg   string
        want     FailoverReason
        wantFlags FlagProfile
    }{
        {"401_auth",        401, "openai", "invalid api key",           Auth,      FlagProfile{R:true, Rc:true}},
        {"402_billing",     402, "openai", "insufficient_quota",        Billing,   FlagProfile{Rc:true, F:true}},
        {"403_permission",  403, "openai", "permission denied",         AuthPermanent, FlagProfile{Rc:true, F:true}},
        {"404_model",       404, "openai", "model 'gpt-99' not found",  ModelNotFound, FlagProfile{F:true}},
        {"413_payload",     413, "openai", "body too large",            PayloadTooLarge, FlagProfile{R:true, C:true}},
        {"429_rate",        429, "openai", "",                          RateLimit, FlagProfile{R:true, Rc:true}},
        {"500_server",      500, "openai", "",                          ServerError, FlagProfile{R:true, F:true}},
        {"503_overload",    503, "openai", "",                          Overloaded, FlagProfile{R:true, F:true}},
        {"529_anthropic",   529, "anthropic", "overloaded",             Overloaded, FlagProfile{R:true, F:true}},
        // ... 총 40+ 케이스
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := classifier.Classify(ctx, errors.New(tc.errMsg), ErrorMeta{
                Provider: tc.provider, StatusCode: tc.status,
            })
            require.Equal(t, tc.want, got.Reason)
            require.Equal(t, tc.wantFlags.Retryable, got.Retryable)
            // ...
        })
    }
}
```

### 7.2 Provider 특화 테스트

각 provider의 특화 오류:

| Provider | 특화 패턴 | 예상 Reason |
|---|---|---|
| anthropic | `thinking_signature` | ThinkingSignature |
| anthropic | `long_context_tier` | ContextOverflow |
| openai | `insufficient_quota` | Billing |
| openai | `context_length_exceeded` | ContextOverflow |
| google | `quota_exceeded` | RateLimit |
| google | `resource_exhausted` | RateLimit |
| xai | (탐색 필요) | (향후 확장) |
| deepseek | (탐색 필요) | (향후 확장) |
| ollama | connection refused | TransportError |

### 7.3 Panic Guard 테스트

Regex 주입으로 panic 유발하는 건 어려우므로, `recover()` 경로를 직접 호출:

```go
func TestClassify_PanicRecovered(t *testing.T) {
    c := &defaultClassifier{opts: ClassifierOptions{
        ExtraPatterns: []ProviderPattern{{
            Provider: "test",
            Pattern:  nil,  // 의도적 nil → .MatchString panic
            Reason:   Timeout,
        }},
    }}
    result := c.Classify(ctx, errors.New("test"), ErrorMeta{Provider: "test"})
    assert.Equal(t, Unknown, result.Reason)
    assert.Contains(t, result.Message, "panic")
}
```

---

## 8. Hermes 재사용 평가

| Hermes 구성요소 | 재사용 가능성 | 재작성 필요 이유 |
|---|---|---|
| 14 FailoverReason enum | **95% 재사용** (12 → 14 확장) | TransportError 추가 |
| HTTP status → reason 매핑 | **100% 재사용** | 정책 그대로 |
| Message regex 패턴 | **95% 재사용** | Go RE2 호환 변환 (대부분 문제 없음) |
| Provider 특화 패턴 | **100% 재사용** | Anthropic/OpenAI 패턴 유지 |
| Transport 휴리스틱 임계치 | **100% 재사용** | 60%/120K/200 값 유지 |
| 4-flag 결정 로직 | **100% 재사용** | 정책 그대로 |
| Python `httpx` dependency | **0% 재사용** | `net.Error` 표준 사용 |
| asyncio | **0% 재사용** | Classify는 동기 함수 |

**추정 Go LoC** (hermes-learning.md §10: `safety: 500 LoC`):
- reasons.go: 80
- classifier.go: 180
- patterns.go: 150 (provider + message 20+ 패턴)
- http_status.go: 50
- transport.go: 70
- defaults.go: 60
- options.go: 40
- 테스트: 500+
- **합계**: ~630 production + 500 test ≈ 1,130 LoC

---

## 9. 통합 시나리오

### 9.1 ADAPTER-001이 분류 결과를 사용

```go
// (ADAPTER-001 코드, 본 SPEC scope 아님 — 통합 예시)
resp, err := httpClient.Do(req)
if err != nil {
    classified := classifier.Classify(ctx, err, errorclass.ErrorMeta{
        Provider:      "anthropic",
        Model:         req.Model,
        StatusCode:    httpStatusFromErr(err),
        ApproxTokens:  estimateTokens(req.Messages),
        ContextLength: modelCtxLen(req.Model),
        MessageCount:  len(req.Messages),
        RawError:      err,
    })
    return &ProviderError{Classified: classified}
}
```

### 9.2 ROUTER-001이 fallback 판단

```go
// (ROUTER-001, 본 SPEC scope 아님)
perr := &ProviderError{}
if errors.As(err, &perr) {
    if perr.Classified.ShouldFallback {
        return r.tryNextModel(perr.Classified.Reason)
    }
    if perr.Classified.ShouldRotateCredential {
        credPool.MarkExhausted(currentKey)
    }
    if perr.Classified.Retryable {
        return r.retryWithBackoff(req)
    }
}
```

---

## 10. 향후 SPEC 연계

- **SPEC-GOOSE-CREDPOOL-001**: `ShouldRotateCredential=true` 수신 시 현재 key를 exhausted 표시, 다음 key로 이동.
- **SPEC-GOOSE-ROUTER-001**: `ShouldFallback=true` 수신 시 fallback chain의 다음 모델 시도.
- **SPEC-GOOSE-CONTEXT-001**: `ShouldCompress=true` 수신 시 긴급 compaction 트리거.
- **SPEC-GOOSE-RATELIMIT-001**: `Reason=RateLimit` 수신 시 Retry-After 파싱 및 bucket 업데이트.
- **SPEC-GOOSE-TRAJECTORY-001**: 실패 궤적 기록 시 `FailureReason = classified.Reason.String()`.
- **SPEC-GOOSE-INSIGHTS-001**: 실패 reason별 히스토그램, `Unknown` 비율 alert.

---

**End of Research**
