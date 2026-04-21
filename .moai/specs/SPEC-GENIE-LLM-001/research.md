# SPEC-GENIE-LLM-001 — Research & Inheritance Analysis

> **목적**: LLM Provider 인터페이스 및 Ollama 어댑터 구현을 위한 자산 조사.
> **작성일**: 2026-04-21

---

## 1. 레포 상태 재확인

`internal/llm/`, `go.mod` 부재. 본 SPEC은 **신규 작성**.

---

## 2. 참조 자산별 분석

### 2.1 Claude Code TypeScript (`./claude-code-source-map/`)

LLM 관련 파일이 분명히 존재하나 TS 코드는 직접 포트 불가. 디자인 패턴만 참고.

```
$ find claude-code-source-map -name '*.ts' | xargs grep -l "openai\|anthropic\|complete\|stream" 2>/dev/null | head
claude-code-source-map/sdk/query.ts (대략)
claude-code-source-map/core/QueryEngine.ts
```

관찰:
- Claude Code는 Anthropic SDK 단일 프로바이더 중심. 복수 프로바이더 추상화가 얇음.
- Stream은 `AsyncGenerator<Chunk>` 패턴. Go의 `<-chan` 또는 `StreamReader`로 이식.

**계승 대상**: 없음. Go 재작성.

### 2.2 Hermes Agent (`./hermes-agent-main/`)

```
$ find hermes-agent-main/agent -name '*.py' | xargs grep -l "openai\|ollama\|anthropic" | head
hermes-agent-main/agent/llm_client.py (추정, 파일명 패턴)
```

- Hermes는 Python `openai` / `anthropic` SDK 호출. 역시 직접 포트 불가.
- 중요한 교훈: Hermes `trajectory_compressor.py`가 토큰 카운팅을 수행 → 본 SPEC의 `Usage` 설계 참고.

### 2.3 Ollama 공식 Go client (`github.com/ollama/ollama/api`)

WebFetch 없이 정적 분석 기반 결정:

- Ollama 레포의 Go client는 `api.Client` 타입 제공.
- 장점: 공식 유지보수.
- 단점:
  - 프로젝트 전체 레포의 일부이므로 `go get github.com/ollama/ollama/api`는 전체 Ollama 서버 코드를 모듈 의존성 그래프에 끌어들일 가능성.
  - 인터페이스 호환성이 우리 `LLMProvider`와 불일치.
- **결정**: 미사용. stdlib `net/http` + `encoding/json` + `bufio.Scanner`로 ~200 LoC 구현.

### 2.4 Eino (ByteDance) `cloudwego/eino`

tech.md §3.2 명시 LLM 프레임워크. 참고는 하되 **사용 안 함**:

- Eino는 "LLM + tool-use + chain" 프레임워크로 범위가 너무 큼 (Phase 0 LLM 레이어만 필요).
- 대신 Eino의 `model.ChatModel` 인터페이스 설계(`Generate`, `Stream`, `BindTools`)를 벤치마킹.

우리 `LLMProvider`는 Eino보다 단순:
- `BindTools`는 tool calling을 지원하므로 본 SPEC 제외 (후속 SPEC에서 인터페이스 확장 또는 별도 `ToolCallingProvider`).

---

## 3. Ollama API 파악

### 3.1 주요 엔드포인트

| Endpoint | 용도 | 본 SPEC 사용 |
|---------|------|-----------|
| `POST /api/chat` | 대화형 완료 (streaming 기본) | ✅ |
| `POST /api/generate` | 프롬프트 단일 완료 | ⚠️ 선택적 |
| `GET /api/show` | 모델 상세 정보 | ✅ Capabilities |
| `GET /api/tags` | 설치된 모델 목록 | ❌ (로컬 모델 관리는 OUT OF SCOPE) |
| `POST /api/embeddings` | 임베딩 생성 | ❌ (VECTOR-001) |
| `POST /api/pull` | 모델 다운로드 | ❌ |
| `DELETE /api/delete` | 모델 삭제 | ❌ |

### 3.2 /api/chat 요청/응답

요청 (예):
```json
{
  "model": "qwen2.5:3b",
  "messages": [
    {"role": "system", "content": "You are helpful."},
    {"role": "user",   "content": "Hello"}
  ],
  "stream": true,
  "options": {
    "temperature": 0.7,
    "num_predict": 256,
    "stop": ["</s>"]
  }
}
```

응답 (stream, NDJSON 각 줄):
```
{"model":"qwen2.5:3b","created_at":"...","message":{"role":"assistant","content":"H"},"done":false}
{"model":"qwen2.5:3b","created_at":"...","message":{"role":"assistant","content":"i"},"done":false}
{"model":"qwen2.5:3b","created_at":"...","message":{"role":"assistant","content":""},"done":true,"total_duration":..., "load_duration":..., "prompt_eval_count":10, "eval_count":5}
```

응답 (non-stream, 단일 JSON): 동일하지만 전체 응답을 한 번에 (`content`가 긴 문자열 하나).

### 3.3 /api/show 응답

`POST /api/show` body: `{"name": "qwen2.5:3b"}`. 응답:
```json
{
  "modelfile": "...",
  "parameters": "...",
  "template": "...",
  "details": {
    "family": "qwen2",
    "parameter_size": "3B",
    "quantization_level": "Q4_K_M"
  },
  "model_info": {
    "general.architecture": "qwen2",
    "qwen2.context_length": 131072,
    ...
  }
}
```

`model_info.<family>.context_length` → `Capabilities.MaxContextTokens`.

### 3.4 에러 응답 패턴

- 404 + `{"error":"model 'xxx' not found, try pulling it first"}` — ErrModelNotFound.
- 400 + `{"error":"invalid format"}` — ErrInvalidRequest.
- 500 + `{"error":"..."}` — ErrServerUnavailable (retry 대상).

---

## 4. Go 이디엄

### 4.1 HTTP client 관리

- 프로바이더 lifetime 동안 하나의 `*http.Client` 재사용.
- `Transport` 커스터마이즈: `MaxIdleConnsPerHost`, `IdleConnTimeout`, `DisableKeepAlives=false`.
- `Timeout`은 client 레벨에 두지 **않음** — streaming에 치명적. 대신 `context.Context` + per-request timeout(`http.NewRequestWithContext`).

### 4.2 Streaming 패턴: `StreamReader` vs `<-chan Chunk`

둘 다 유효하나 **`StreamReader` 선택**:

```go
type StreamReader interface {
    Next(ctx context.Context) (Chunk, bool, error)  // (chunk, done, err)
    Close() error
}
```

장점:
- 에러 전파가 명시적 (`err` 리턴).
- `Close()`로 리소스 해제 계약 분명.
- 호출 측에서 channel select loop를 강제하지 않음(더 단순).

단점(허용):
- goroutine + channel 기반 코드가 상호작용 어려움 — 어댑터가 내부적으로 channel을 써도, 외부 API는 Reader.

### 4.3 에러 매핑 테이블

```go
func mapHTTPError(resp *http.Response, body []byte) error {
    switch resp.StatusCode {
    case 400: return &ErrInvalidRequest{Body: string(body)}
    case 401, 403: return &ErrUnauthorized{...}
    case 404:
        if isModelNotFound(body) {
            return &ErrModelNotFound{Model: extractModel(body)}
        }
        return &ErrNotFound{...}
    case 429: return &ErrRateLimited{RetryAfter: parseRetryAfter(resp)}
    case 500, 502, 503, 504: return &ErrServerUnavailable{...}
    default: return &ErrUnexpectedStatus{Code: resp.StatusCode}
    }
}
```

각 에러 타입에 `Retryable() bool` 메서드를 두어 `retry.ShouldRetry`가 단일 콜로 결정.

### 4.4 NDJSON 디코더 구조

```go
type ndjsonReader struct {
    scanner *bufio.Scanner
    resp    *http.Response
    cancel  context.CancelFunc
}

func (r *ndjsonReader) Next(ctx context.Context) (Chunk, bool, error) {
    select {
    case <-ctx.Done():
        _ = r.Close()
        return Chunk{}, false, ctx.Err()
    default:
    }
    if !r.scanner.Scan() {
        if err := r.scanner.Err(); err != nil && err != io.EOF {
            return Chunk{}, false, err
        }
        return Chunk{}, true, nil
    }
    var raw ollamaChatResponse
    if err := json.Unmarshal(r.scanner.Bytes(), &raw); err != nil {
        return Chunk{}, false, &ErrMalformedStream{Err: err}
    }
    return ollamaToChunk(raw), raw.Done, nil
}
```

버퍼 확장은 `scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)`로 최대 16MB 허용.

---

## 5. 외부 의존성 합계

| 모듈 | 용도 | 채택 |
|------|------|-----|
| 표준 `net/http`, `encoding/json`, `bufio`, `context` | HTTP + 파싱 + 취소 | ✅ |
| `go.uber.org/zap` | logging | ✅ (CORE-001 공유) |
| `github.com/stretchr/testify` | 테스트 | ✅ |
| `go.uber.org/goleak` | goroutine leak 탐지 | ✅ |
| `github.com/ollama/ollama/api` | Ollama 공식 client | ❌ 의존성 무거움 |
| `github.com/cloudwego/eino` | LLM 프레임워크 | ❌ 범위 과도 |
| `github.com/sashabaranov/go-openai` | OpenAI client | ❌ OOS (타 프로바이더) |
| `github.com/anthropic/anthropic-sdk-go` | Claude client | ❌ OOS |
| `github.com/tiktoken-go/tokenizer` | OpenAI 토큰 카운팅 | ❌ OOS (Ollama는 서버에서 count 제공) |

---

## 6. 테스트 전략

### 6.1 Unit (예상 15~20)

- `TestMapHTTPError_400_Invalid`
- `TestMapHTTPError_404_ModelNotFound`
- `TestMapHTTPError_429_RateLimited_WithRetryAfter`
- `TestMapHTTPError_500_ServerUnavailable_Retryable`
- `TestRetryPolicy_ShouldRetry_5xx`
- `TestRetryPolicy_ShouldNotRetry_4xx`
- `TestRetryPolicy_ShouldNotRetry_ModelNotFound`
- `TestRetryPolicy_MaxRetries_StopsAfter3`
- `TestRetryPolicy_BackoffGrows`
- `TestMapper_CompletionRequest_ToOllamaJSON`
- `TestMapper_OllamaResponse_ToCompletionResponse`
- `TestMapper_Options_Temperature`
- `TestMapper_Options_Stop`
- `TestNDJSONReader_EmitsAllChunks`
- `TestNDJSONReader_MalformedLine_Error`
- `TestNDJSONReader_BufferTooSmall_Extends`
- `TestCapsCache_HitAndMiss`

### 6.2 Integration with httptest.Server

- `TestOllamaProvider_Complete_E2E` → AC-LLM-001.
- `TestOllamaProvider_Stream_E2E` → AC-LLM-002.
- `TestOllamaProvider_Retry5xx_E2E` → AC-LLM-003.
- `TestOllamaProvider_No4xxRetry_E2E` → AC-LLM-004.
- `TestOllamaProvider_StreamCancel_E2E` → AC-LLM-005 (`goleak.VerifyNone`).
- `TestOllamaProvider_ModelNotFound_E2E` → AC-LLM-006.
- `TestRegistry_BootFromConfig_E2E` → AC-LLM-007.
- `TestOllamaProvider_CapabilitiesCache_E2E` → AC-LLM-008.

### 6.3 선택적 로컬 Ollama 통합 테스트

- `//go:build integration && ollama_local` 태그.
- 실제 로컬 Ollama + `qwen2.5:3b`로 end-to-end.
- CI default off, 개발자 로컬에서만.

### 6.4 커버리지 목표

- `internal/llm/`: 85%+
- `internal/llm/ollama/`: 90%+
- retry/errors: 100%

---

## 7. 오픈 이슈

1. **`Stream()` 리턴 타입 최종화**: `StreamReader` 인터페이스 vs `<-chan Chunk`. 본 SPEC은 Reader 선택. Anthropic 어댑터 SPEC 구현 시 재검토 가능.
2. **Tool calling 인터페이스 확장**: 현재 `LLMProvider`에 `BindTools` 없음. AGENT-001이 tool calling을 요구하면 그 시점에 `ToolCallingProvider` 서브 인터페이스 추가.
3. **토큰 정확도**: Ollama의 `eval_count`는 서버가 report하는 값 — 실제 token boundary와 일치. 다른 프로바이더가 다른 tokenizer를 쓸 경우 `Usage.TokenizerID` 필드 추가 검토.
4. **HTTP/2 지원**: Ollama 서버가 HTTP/1.1만 지원 — default Go transport 문제 없음. 향후 HTTP/2 필요 시 별도 설정.
5. **프롬프트 로깅 정책**: REQ-LLM-012는 DEBUG 레벨 + redaction. 실제 redaction 구현(e.g., 앞 80자 + hash)은 별도 유틸 필요.

---

## 8. 결론

- **이식 자산**: 없음. LLM 추상화 + Ollama 어댑터 전부 신규.
- **참조 자산**: Eino 인터페이스 디자인, Hermes `trajectory_compressor`의 token 카운팅 접근.
- **기술 스택**: stdlib HTTP + JSON + NDJSON scanner. 외부 Ollama client 미사용.
- **구현 규모 예상**: 800~1,200 LoC (테스트 포함 1,800~2,500 LoC). Phase 0 상한(4,000 LoC, M 사이즈) 내.
- **주요 리스크**: Stream 취소 경로의 goroutine leak(R2). `goleak` 테스트로 gate.

GREEN 완료 시 AGENT-001은 `registry.Get("ollama").Stream(ctx, req)` 한 줄로 LLM 응답을 소비할 수 있게 된다.

---

**End of research.md**
