---
id: SPEC-GOOSE-LLM-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 0
size: 중(M)
lifecycle: spec-anchored
---

# SPEC-GOOSE-LLM-001 — LLM Provider 인터페이스 + Ollama 어댑터

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (ROADMAP Phase 0 row 04, tech §3.2) | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT의 모든 LLM 호출이 통과해야 할 **`LLMProvider` 인터페이스와 Ollama 어댑터 1종**을 정의한다. 본 SPEC은 Anthropic/OpenAI/Gemini 어댑터를 **포함하지 않는다** — 그것은 별도 SPEC에서 단일 인터페이스를 만족하는 추가 구현으로 붙는다.

수락 조건 통과 시점에서:

- `LLMProvider` 인터페이스가 `Complete`, `Stream`, `CountTokens`, `Capabilities` 메서드를 제공한다.
- `internal/llm/ollama` 어댑터가 `http://localhost:11434`의 Ollama 서버에 붙어 `qwen2.5:3b` 또는 설정된 모델로 응답/스트림을 반환한다.
- `internal/llm/registry`가 `CONFIG-001`의 `LLMConfig.Providers` 맵으로 부트스트랩되어 `registry.Get("ollama")`로 Provider를 획득한다.
- 네트워크 오류, 타임아웃, 5xx 응답에 대해 정의된 `LLMError` 계층으로 변환된다.

---

## 2. 배경 (Background)

### 2.1 왜 Ollama 1종만

- ROADMAP §4 Phase 0 row 04가 `LLM Provider 인터페이스 + Ollama 어댑터`로 범위를 한정. Ollama는 **로컬 실행 + 무료 + API key 불필요** → Phase 0 MVP에 최적.
- `.moai/project/tech.md` §9 "LLM 프로바이더 지원" 표에서 Anthropic/OpenAI/Gemini도 P0로 표기되어 있으나, 이들은 API key + 비용 + rate-limit 이슈가 커서 Phase 0에서 테스트를 복잡하게 함. **인터페이스만 확립**하고 어댑터 추가는 후속 SPEC.
- `.moai/project/product.md` §3.1이 "기본 로컬 우선" 정책을 지정(로컬 LLM 없으면 사용 불가가 아니라 비용 없이 시작할 수 있음).

### 2.2 Retry와 토큰 카운팅

- 사용자가 요구한 "스트리밍, 리트라이, 토큰 카운팅"은 전부 본 SPEC 범위.
- **Retry**: 네트워크 일시 오류(connect reset, 5xx)만 재시도. 4xx는 재시도 금지.
- **토큰 카운팅**: Ollama는 `eval_count`/`prompt_eval_count`를 응답에 포함 → 그대로 수확. 타 프로바이더(OpenAI tiktoken 등)는 후속 SPEC.

### 2.3 범위 경계

- **IN**: 인터페이스 정의, Ollama 어댑터, registry, retry/backoff, 컨텍스트 취소 전파, 구조화 오류, 스트리밍 채널, 모델 capability 메타데이터.
- **OUT**: Anthropic/OpenAI/Gemini/xAI/Mistral 어댑터, prompt caching(Claude 전용), cost tracking($/token), fallback 체인(1차 실패 시 2차 프로바이더), embedding API(VECTOR-001), 이미지/오디오 입력(multi-modal).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/llm/provider.go`: `LLMProvider` 인터페이스 + 관련 타입(`CompletionRequest`, `CompletionResponse`, `Chunk`, `Capabilities`).
2. `internal/llm/registry.go`: `Registry`로 name → provider 매핑. `CONFIG-001`의 `LLMConfig.Providers` 맵을 받아 팩토리 호출.
3. `internal/llm/errors.go`: `LLMError` 계층(`ErrRateLimited`, `ErrContextTooLong`, `ErrServerUnavailable`, `ErrInvalidRequest`, `ErrUnauthorized` 등).
4. `internal/llm/retry.go`: Exponential backoff + jitter, 재시도 가능 에러 분류.
5. `internal/llm/ollama/`: Ollama HTTP 어댑터 (`POST /api/chat`, `POST /api/generate`).
   - Unary: `Complete()` — 비-스트림 요청.
   - Streaming: `Stream()` — NDJSON chunk 파싱 → `<-chan Chunk`.
   - `CountTokens()`: 응답에서 `eval_count + prompt_eval_count` 추출.
   - `Capabilities()`: 모델 정보(max context, supports tools 등) — 첫 호출 시 `GET /api/show`로 조회 + 캐시.
6. `LLMConfig.Validate()` 구현 (CONFIG-001 위임 지점): `default_provider`가 registry에 존재하는지, 각 provider의 `kind` 지원 여부 확인.
7. context cancellation 전파: `http.Request.WithContext`, stream 중간 취소 시 즉시 goroutine 종료.
8. 메트릭 수집 hook: `OnComplete(usage Usage)` 콜백. Phase 0은 no-op 기본값, TELEM-001이 연결.

### 3.2 OUT OF SCOPE

- Anthropic, OpenAI, Google Gemini, xAI, OpenRouter 어댑터 — 각자 별도 SPEC.
- 비용 추적 / 가격 산출 (`CostTracker` 스텁만 정의하고 구현 안 함).
- Prompt caching (Claude Anthropic 전용 기능) — Claude SPEC 에서.
- Fallback 체인(1차 ollama 실패 → 2차 openai).
- Embedding(임베딩) API — VECTOR-001의 범위.
- Image/audio/video input (multi-modal) — 후속 SPEC.
- 로컬 모델 관리(모델 pull/unload 자동화) — Ollama CLI에 위임.
- Load balancing (여러 Ollama 인스턴스 분산).
- Structured output / JSON schema enforcement — 후속 SPEC.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-LLM-001 [Ubiquitous]** — The `LLMProvider` interface **shall** accept a `context.Context` as the first parameter of every method and honor its cancellation within 100ms of detection.

**REQ-LLM-002 [Ubiquitous]** — Every `CompletionResponse` **shall** include `Usage{PromptTokens, CompletionTokens, TotalTokens}` populated from the provider's response; if the provider does not expose counts, the field **shall** be `Usage{}` with zero values and `Usage.Unknown = true`.

**REQ-LLM-003 [Ubiquitous]** — All LLM errors returned to callers **shall** be one of the typed `LLMError` subclasses; raw `net.OpError` or `*url.Error` **shall** be wrapped, never surfaced directly.

### 4.2 Event-Driven

**REQ-LLM-004 [Event-Driven]** — **When** `Stream()` is invoked, the provider **shall** return a receive-only channel `<-chan Chunk` that emits each token group as it is decoded from the server, closing the channel when the stream ends normally or with an error sent on a dedicated error channel.

**REQ-LLM-005 [Event-Driven]** — **When** an HTTP `5xx` response or a `net.ErrClosed`/`io.ErrUnexpectedEOF` occurs mid-request, the retry layer **shall** back off with exponential+jitter and retry up to `MaxRetries` (default 3) before returning `ErrServerUnavailable`.

**REQ-LLM-006 [Event-Driven]** — **When** an HTTP `4xx` response other than `429` is received, the retry layer **shall** return the mapped error immediately without retry.

**REQ-LLM-007 [Event-Driven]** — **When** the Ollama adapter receives `HTTP 404` with body containing `"model ... not found"`, it **shall** return `ErrModelNotFound{Model: "..."}` without retry.

### 4.3 State-Driven

**REQ-LLM-008 [State-Driven]** — **While** a stream is in progress, cancelling the parent context **shall** close the chunk channel within 100ms and release the underlying HTTP connection (no goroutine leak).

**REQ-LLM-009 [State-Driven]** — **While** `Capabilities()` has not been queried for a given model, the first call **shall** fetch `/api/show` and cache the result in-memory for the lifetime of the provider instance.

### 4.4 Unwanted Behavior

**REQ-LLM-010 [Unwanted]** — **If** the Ollama server returns a malformed NDJSON line during streaming, **then** the adapter **shall** close the channel with a `Chunk{Err: ErrMalformedStream}` final message; it **shall not** panic or silently drop the stream.

**REQ-LLM-011 [Unwanted]** — **If** `MaxRetries` is exceeded, the final error **shall** wrap the last observed error via `errors.Wrap` style; retry count and total elapsed time **shall** be included in the error for observability.

**REQ-LLM-012 [Unwanted]** — The provider **shall not** log the raw prompt content at INFO level or higher; prompt bodies are DEBUG-only and redacted in structured logs by default.

**REQ-LLM-013 [Unwanted]** — The Ollama adapter **shall not** follow HTTP redirects; any `3xx` response is treated as a configuration error (`ErrInvalidConfig`).

### 4.5 Optional

**REQ-LLM-014 [Optional]** — **Where** `CompletionRequest.Stop []string` is non-empty, the adapter **shall** map the values to Ollama's `options.stop` field; other providers may interpret stop sequences differently.

**REQ-LLM-015 [Optional]** — **Where** `CompletionRequest.Temperature *float64` is set, the adapter **shall** pass it through; unset leaves the provider default untouched (no implicit 0.0 or 1.0).

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-LLM-001 — Ollama unary 완료**
- **Given** 테스트용 HTTP server가 `/api/chat`에 대해 200 + JSON `{"message":{"role":"assistant","content":"hello"},"done":true,"eval_count":5,"prompt_eval_count":10}` 반환
- **When** `provider.Complete(ctx, CompletionRequest{Model:"qwen2.5:3b", Messages:[...]})`
- **Then** `resp.Text == "hello"`, `resp.Usage.CompletionTokens == 5`, `resp.Usage.PromptTokens == 10`

**AC-LLM-002 — Ollama 스트림**
- **Given** 테스트 server가 `/api/chat`에 대해 NDJSON 3줄 전송 후 종료 (`{"message":{"content":"a"},"done":false}`, `..."b"...`, `{...,"done":true}`)
- **When** `ch := provider.Stream(ctx, req)`; collect all chunks
- **Then** 수신된 chunk 텍스트 합 == `"ab"`, 마지막 chunk에 `Done=true`, `err` 채널은 close된 이후 nil drain

**AC-LLM-003 — Retry on 503**
- **Given** 테스트 server가 2번 503 응답 후 3번째 200 반환
- **When** `provider.Complete(ctx, req)` (MaxRetries=3)
- **Then** 에러 없음, `resp.Text != ""`, 총 3번의 HTTP 시도가 관찰됨

**AC-LLM-004 — No retry on 400**
- **Given** 테스트 server가 400 Bad Request 반환
- **When** `provider.Complete(ctx, req)`
- **Then** 1번만 시도, `errors.Is(err, ErrInvalidRequest) == true`

**AC-LLM-005 — Context cancel during stream**
- **Given** stream이 진행 중 (server 측에서 느리게 chunk 전송)
- **When** 테스트가 `cancel()` 호출
- **Then** 100ms 이내 chunk 채널 close, underlying HTTP 요청 취소 (`server.observeContextDone == true`), goroutine leak 없음(`goleak.VerifyNone`)

**AC-LLM-006 — Model not found**
- **Given** server가 `/api/chat`에 대해 404 + `{"error":"model 'nonexistent' not found"}` 반환
- **When** `provider.Complete(ctx, req)` (Model="nonexistent")
- **Then** `var mErr *ErrModelNotFound; errors.As(err, &mErr) == true`, `mErr.Model == "nonexistent"`, 재시도 0회

**AC-LLM-007 — Registry lookup**
- **Given** `LLMConfig{DefaultProvider:"ollama", Providers: {"ollama": {Kind:"ollama", Host:"http://localhost:11434"}}}`
- **When** `registry := llm.NewRegistry(cfg); p, err := registry.Get("ollama")`
- **Then** `err == nil`, `p.Name() == "ollama"`

**AC-LLM-008 — Capabilities cache**
- **Given** Ollama `/api/show` endpoint이 200 반환 (model info)
- **When** `provider.Capabilities(ctx, "qwen2.5:3b")` 두 번 연속 호출
- **Then** 두 결과 동일, server-side `/api/show` 호출 1회만 관찰

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/llm/
├── provider.go                 # interface + request/response 타입
├── registry.go                 # name → Provider factory + cache
├── retry.go                    # backoff/jitter 로직
├── errors.go                   # LLMError 계층
├── usage.go                    # Usage struct + 병합 유틸
├── ollama/
│   ├── ollama.go               # type Provider struct + New()
│   ├── client.go               # HTTP 호출 (Complete, Stream)
│   ├── capabilities.go         # /api/show 래핑 + 캐시
│   ├── mapper.go               # CompletionRequest ↔ Ollama JSON
│   ├── ndjson.go               # NDJSON 스트림 디코더
│   └── *_test.go
└── testutil/
    └── fake_server.go          # net/http/httptest 기반 테스트 서버
```

### 6.2 핵심 타입

```go
type LLMProvider interface {
    Name() string
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (StreamReader, error)
    Capabilities(ctx context.Context, model string) (Capabilities, error)
    Close() error
}

type CompletionRequest struct {
    Model       string
    Messages    []Message
    Temperature *float64
    MaxTokens   *int
    Stop        []string
}

type Message struct {
    Role    string // system|user|assistant|tool
    Content string
}

type CompletionResponse struct {
    Text  string
    Usage Usage
    Model string
    Raw   any // provider-specific payload (debug only)
}

type StreamReader interface {
    Next(ctx context.Context) (Chunk, bool, error)
    Close() error
}

type Chunk struct {
    Delta string
    Done  bool
    Usage *Usage // 최종 chunk에만
}

type Capabilities struct {
    MaxContextTokens int
    SupportsTools    bool
    SupportsJSON     bool
    Family           string // "llama", "qwen", "gpt", ...
}
```

`Stream()`이 `<-chan Chunk` 대신 `StreamReader` 인터페이스를 반환하는 이유: Next/Close로 에러 전파가 더 명시적이며, 여러 프로바이더 구현의 유연성 확보.

### 6.3 Ollama API 매핑

Ollama chat API: `POST /api/chat`

- Request: `{ model, messages: [{role, content}], stream: bool, options: {temperature, stop, num_predict} }`
- Non-stream response: 단일 JSON with `{message: {role, content}, done:true, eval_count, prompt_eval_count}`
- Stream response: NDJSON lines, 각 line은 `{message:{content:"..."}, done: false}` 또는 최종 `{..., done:true, eval_count, prompt_eval_count}`.

`mapper.go`가 `CompletionRequest` ↔ Ollama JSON 변환. 알려지지 않은 옵션은 무시.

### 6.4 Retry 로직

```go
type RetryPolicy struct {
    MaxRetries   int           // default 3
    InitialDelay time.Duration // default 200ms
    MaxDelay     time.Duration // default 5s
    Multiplier   float64       // default 2.0
    JitterFrac   float64       // default 0.2
}

func (p RetryPolicy) ShouldRetry(err error, attempt int) bool
```

- 재시도 대상: `ErrServerUnavailable`, `ErrRateLimited`(429), 네트워크 일시 오류.
- 비대상: 4xx(429 제외), `ErrModelNotFound`, `ErrUnauthorized`, `ErrInvalidRequest`, `ctx.Done()`.

### 6.5 NDJSON 스트리밍 디코더

- `bufio.Scanner` 라인 단위 읽기, 각 라인에 대해 `json.Unmarshal`.
- Scanner buffer는 기본 64KB로 충분 (Ollama 청크 보통 수백 B). 넘어가면 `bufio.Scanner.Buffer()`로 확장.
- 채널 닫기는 defer 순서 명확히: scanner 에러 → final chunk에 에러 전달 → channel close.

### 6.6 Capabilities 캐시

```go
type capsCache struct {
    mu   sync.RWMutex
    data map[string]Capabilities
}
```

- 프로세스 lifetime 동안 유효. 모델 unload/pull 감지 없음 (OUT OF SCOPE).
- `Ollama /api/show` 응답의 `details.parameter_size`, `details.context_length` 등에서 추출.

### 6.7 Registry 초기화

```go
type FactoryFn func(cfg ProviderConfig, logger *zap.Logger) (LLMProvider, error)

var factories = map[string]FactoryFn{
    "ollama": ollama.New,
}

func NewRegistry(cfg LLMConfig, logger *zap.Logger) (*Registry, error)
```

후속 SPEC(Anthropic/OpenAI)이 `RegisterFactory("anthropic", anthropic.New)`로 추가. Phase 0 basline은 `ollama`만.

### 6.8 TDD 진입

1. **RED**: `TestOllama_Complete_ReturnsText` (AC-LLM-001).
2. **RED**: `TestOllama_Stream_YieldsChunks` (AC-LLM-002).
3. **RED**: `TestRetry_503TwiceThen200` (AC-LLM-003).
4. **RED**: `TestRetry_400_NoRetry` (AC-LLM-004).
5. **RED**: `TestContextCancel_StreamClosed` (AC-LLM-005) — `goleak`.
6. **RED**: `TestOllama_ModelNotFound` (AC-LLM-006).
7. **RED**: `TestRegistry_GetOllama` (AC-LLM-007).
8. **RED**: `TestCapabilities_Cache_Hits` (AC-LLM-008).
9. **GREEN**: 최소 구현.
10. **REFACTOR**: retry/error 매핑을 `errors.go`에 집약.

### 6.9 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| Tested | `httptest.Server` 활용, table-driven error mapping test, `goleak` for goroutine 누수 |
| Readable | `LLMProvider` doc-comment에 각 메서드 전제/사후 조건 명문화 |
| Unified | `golangci-lint` + errcheck + `errors.Is/As` 체계적 사용 |
| Secured | prompt 로그 redaction, 4xx 재시도 금지(토큰 낭비 방지), HTTP redirect 금지 |
| Trackable | 각 호출에 `req_id` (uuid) 부여, `zap.With(...)` 필드로 로그 일관성 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GOOSE-CORE-001** | zap logger, context 전파 |
| 선행 SPEC | **SPEC-GOOSE-CONFIG-001** | `LLMConfig.Providers`, `CONFIG-001`의 `ProviderConfig` 소비 |
| 선행 SPEC | SPEC-GOOSE-TRANSPORT-001 | 직접 의존 아님, 향후 LLM RPC 정의 시 proto 확장 |
| 후속 SPEC | SPEC-GOOSE-AGENT-001 | Provider 소비자 |
| 후속 SPEC | SPEC-GOOSE-TELEM-001 | `OnComplete` 콜백 연결 |
| 외부 | Ollama 서버 | 로컬 HTTP (`http://localhost:11434`) |
| 외부 | 표준 `net/http`, `encoding/json`, `bufio` | HTTP + NDJSON |
| 외부 | `go.uber.org/goleak` | 테스트 전용 |

**중요**: 본 SPEC은 `github.com/ollama/ollama/api` 공식 Go client를 **사용하지 않는다**. 이유:
- 공식 client는 의존성 무거움(`llama.cpp` 바인딩 포함 가능성).
- NDJSON + JSON POST만 있으면 stdlib로 충분 (~200 LoC).
- 테스트 시 `httptest.Server`로 서버 stub이 간편.

나중에 Ollama API가 크게 복잡해지면 재평가.

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Ollama API 변경 (NDJSON 스키마 변경) | 중 | 중 | `/api/version` 체크 + version skew 경고 로그. Breaking 변경 시 SPEC 수정 |
| R2 | 스트림 goroutine leak | 중 | 높 | `goleak` 테스트 필수, ctx 취소 경로 defer 다중 방어 |
| R3 | Retry 중 client prompt 재전송에 따른 중복 과금 (타 프로바이더) | 낮 | 중 | Phase 0은 Ollama 로컬이라 과금 없음. 타 프로바이더는 자기 SPEC에서 재평가 |
| R4 | Capabilities 캐시가 stale (모델 re-pull) | 낮 | 낮 | 수동 invalidate API 제공 (`provider.InvalidateCapabilities(model)`) |
| R5 | 대용량 응답(>4MB)에서 NDJSON scanner 버퍼 초과 | 낮 | 중 | `bufio.Scanner.Buffer(1MB, 16MB)`로 확장 |
| R6 | 인터페이스가 후속 프로바이더를 수용 못 함 | 중 | 높 | 본 SPEC의 인터페이스는 **세 번째 구현이 붙기 전까지는 임시**로 간주. Anthropic 어댑터 SPEC에서 리뷰 후 필요 시 인터페이스 소폭 수정 가능(마이너 버전 0.2.0) |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/tech.md` §3.2 (AI/LLM 스택), §9 (프로바이더 지원 매트릭스)
- `.moai/project/structure.md` §1 `internal/llm/` 패키지 스케치
- `.moai/project/product.md` §3.1 "로컬 우선" 원칙

### 9.2 외부 참조

- Ollama API 레퍼런스: https://github.com/ollama/ollama/blob/main/docs/api.md
- go-eino (ByteDance) 소스 참고(사용 안 함): LLM abstraction 디자인 패턴
- `go.uber.org/goleak` 문서

### 9.3 부속 문서

- `./research.md`
- `../ROADMAP.md` §4 Phase 0 row 04

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **Anthropic/OpenAI/Gemini/xAI/OpenRouter 등 타 프로바이더 어댑터를 구현하지 않는다**. 각자 별도 SPEC.
- 본 SPEC은 **비용 추적 / 가격 계산 / 토큰 예산 차감을 구현하지 않는다** (`cost_tracker.go` 스텁 파일도 만들지 않음).
- 본 SPEC은 **Anthropic prompt caching / ephemeral cache breakpoint를 지원하지 않는다**.
- 본 SPEC은 **Provider fallback chain(1차 실패 시 2차로 자동 전환)을 제공하지 않는다**.
- 본 SPEC은 **embedding API**(`Embed` 메서드)를 **`LLMProvider` 인터페이스에 포함하지 않는다**. VECTOR-001의 별도 인터페이스.
- 본 SPEC은 **이미지/오디오/비디오 입력(multi-modal)을 지원하지 않는다**. 텍스트-only.
- 본 SPEC은 **로컬 모델 관리(pull, delete, list)를 자동화하지 않는다**. 사용자가 `ollama pull qwen2.5:3b`를 미리 실행했다고 가정.
- 본 SPEC은 **structured output / JSON schema 강제를 지원하지 않는다**. 후속 SPEC.
- 본 SPEC은 **OpenAI-compatible 어댑터(tech.md §3.2 `openai_compat`)를 제공하지 않는다**. Ollama의 OpenAI 호환 endpoint도 대상 아님 — `/api/chat` 네이티브만.

---

**End of SPEC-GOOSE-LLM-001**
