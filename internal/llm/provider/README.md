# internal/llm/provider

**LLM Provider Adapter 패키지** — 다중 LLM Provider에 대한 통합 인터페이스 및 HTTP 어댑터 구현

## 개요

본 패키지는 AI.GOOSE Phase 1의 **LLM HTTP 호출 경계**를 정의합니다. `QUERY-001`의 `LLMCallFunc` 인터페이스에 대한 6개 주요 provider 구현(Anthropic, OpenAI, Google Gemini, xAI Grok, DeepSeek, Ollama)과 9개 metadata-only provider를 제공합니다.

모든 provider는 공통 `Provider` interface를 구현하며, OAuth PKCE refresh, tool schema 변환, prompt cache, rate limit 연계 등의 기능을 제공합니다.

## 핵심 구성 요소

### Provider Interface

모든 provider가 구현해야 하는 공통 인터페이스:

```go
type Provider interface {
    // Name returns the provider identifier (e.g., "anthropic", "openai")
    Name() string

    // Capabilities returns supported features
    Capabilities() Capabilities

    // Complete performs a blocking API call for non-streaming responses
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

    // Stream performs a streaming API call, returning a channel of StreamEvent
    Stream(ctx context.Context, req CompletionRequest) (<-chan message.StreamEvent, error)
}
```

### ProviderRegistry

Provider 문자열 식별자를 `Provider` 인스턴스에 매핑하는 레지스트리:

```go
registry := NewProviderRegistry()
registry.Register("anthropic", anthropicAdapter)
registry.Register("openai", openaiAdapter)

provider, err := registry.Lookup("anthropic")
if err != nil {
    return ErrProviderNotFound
}
```

### LLMCallFunc

`QUERY-001`의 `LLMCallFunc` 시그니처 구현:

```go
func LLMCall(ctx context.Context, req LLMCallReq) (<-chan message.StreamEvent, error) {
    provider, err := registry.Lookup(req.Route.Provider)
    if err != nil {
        return nil, err
    }

    credential, err := credentialPool.Select(ctx, req.Route.CredentialStrategy)
    if err != nil {
        return nil, err
    }

    return provider.Stream(ctx, req.CompletionRequest)
}
```

## 지원하는 Provider

### 주요 구현 Provider (6개)

| Provider | 패키지 경로 | 특징 |
|----------|-----------|------|
| **Anthropic** | `anthropic/` | OAuth PKCE 2.1, Adaptive Thinking, tool schema 변환, prompt cache |
| **OpenAI** | `openai/` | Chat Completions, Tools, Streaming (hand-rolled `net/http`) |
| **Google** | `google/gemini.go` | Gemini 2.0 Flash (유일한 SDK 기반: `google.golang.org/genai`) |
| **Ollama** | `ollama/` | 로컬 LLM (`localhost:11434 /api/chat JSON-L`) |
| **xAI** | `xai/` | OpenAI-compat wrapper (base_url override) |
| **DeepSeek** | `deepseek/` | OpenAI-compat wrapper (base_url override) |

### Metadata-only Provider (9개)

| Provider | 패키지 경로 | 비고 |
|----------|-----------|------|
| Cerebras | `cerebras/` | metadata만 등록 |
| Fireworks | `fireworks/` | metadata만 등록 |
| Groq | `groq/` | metadata만 등록 |
| Mistral | `mistral/` | metadata만 등록 |
| OpenRouter | `openrouter/` | metadata만 등록 |
| Kimi | `kimi/` | metadata만 등록 |
| Qwen | `qwen/` | metadata만 등록 |
| Together | `together/` | metadata만 등록 |
| GLM | `glm/` | metadata만 등록 |

## Provider별 특수 기능

### Anthropic Provider

**OAuth PKCE 2.1 + Single-use Refresh Token Rotation**:
- `~/.claude/.credentials.json`에서 토큰 읽기/쓰기
- refresh 후 rotated refresh_token 기록

**Model Alias Normalization**:
- `claude-3.5-sonnet` → `claude-3-5-sonnet-20241022`
- `anthropic/models.go`에서 alias map 유지

**Tool Schema Conversion**:
- OpenAI function calling format → Anthropic tool_use format
- `tools.go`에서 변환 로직 구현

**Content Conversion**:
- 이미지: base64 인코딩
- 코드: text type 지정
- thinking block: observational only StreamEvent

**Adaptive Thinking Mode** (Opus 4.7):
- `effort: high|xhigh|max` → API `thinking` parameter
- non-adaptive 모델에는 `budget_tokens` 사용

**Prompt Cache Integration**:
- `PromptCachePlanner.Plan()`으로 cache_control markers 획득
- system_and_3, system, last3 전략 지원

### OpenAI Provider

**Hand-rolled HTTP Client**:
- `net/http` + `encoding/json`로 직접 구현
- 공식 SDK 의존 없음

**Extra Headers/Fields** (ADAPTER-002 선행):
- `OpenAIOptions.ExtraHeaders`: 커스텀 HTTP 헤더
- `CompletionRequest.ExtraRequestFields`: provider-specific 확장

**Base URL Override**:
- xAI: `https://api.x.ai/v1`
- DeepSeek: `https://api.deepseek.com`

### Google Gemini Provider

**SDK-based Implementation**:
- `google.golang.org/genai` 사용 (유일한 SDK 기반 어댑터)
- streaming, tool, vision 추상화 이득

### Ollama Provider

**Local LLM Support**:
- `localhost:11434 /api/chat` JSON-L streaming
- 로컬 모델 실행을 위한 간단한 HTTP client

## Fallback Model Chain

주요 모델 호출 실패 시 transparent fallback:

```go
// fallback.go
func (p *provider) StreamWithFallback(ctx context.Context, req CompletionRequest) (<-chan message.StreamEvent, error) {
    models := append([]string{req.Model}, req.FallbackModels...)
    for i, model := range models {
        req.Model = model
        stream, err := p.Stream(ctx, req)
        if err == nil {
            return stream, nil
        }
        // Try next model in chain
    }
    return nil, ErrAllProvidersFailed
}
```

## Rate Limit 연계

모든 adapter는 HTTP 응답 수신 직후 rate limit tracker에 헤더 전달:

```go
resp, err := http.DefaultClient.Do(httpReq)
if err != nil {
    return err
}

// RATELIMIT-001 연계
ratelimit.Tracker.Parse(providerName, resp.Header, time.Now())

body, err := io.ReadAll(resp.Body)
// ... process response
```

## OAuth Refresh 구현

Anthropic/OpenAI Codex provider는 `CREDPOOL-001`의 `Refresher` interface 구현:

```go
type Refresher interface {
    Refresh(ctx context.Context, cred *credential.Credential) (*credential.RefreshResult, error)
}

// anthropic/oauth.go
func (a *Anthropic) Refresh(ctx context.Context, cred *credential.Credential) (*credential.RefreshResult, error) {
    // POST to token endpoint with refresh_token
    // Receive new access_token and rotated refresh_token
    // Update credential pool
    // Write back to ~/.claude/.credentials.json
}
```

## 테스트

### 단위 테스트

```bash
# 전체 패키지 테스트
go test ./internal/llm/provider/...

# 특정 provider 테스트
go test ./internal/llm/provider/anthropic/...
go test ./internal/llm/provider/openai/...
go test ./internal/llm/provider/google/...
```

### 통합 테스트 (실제 API 호출)

```bash
# Anthropic 실제 API 테스트 (API key 필요)
ANTHROPIC_API_KEY=key go test ./internal/llm/provider/anthropic/ -run TestAnthropic_Real

# Google 실제 API 테스트 (API key 필요)
GOOGLE_API_KEY=key go test ./internal/llm/provider/google/ -run TestGoogle_Real
```

### 테스트 커버리지

현재 전체 패키지 테스트 커버리지: **86.7%** (ADAPTER-001 구현 완료 시점)

## 상호 의존성

본 패키지는 다음 SPEC와 통합됩니다:

- **QUERY-001**: `LLMCallFunc` 시그니처 구현
- **CREDPOOL-001**: `CredentialPool.Select()`, `Refresher` interface
- **ROUTER-001**: `Route.Provider` 결정 소비
- **RATELIMIT-001**: `Tracker.Parse()`에 응답 헤더 전달
- **PROMPT-CACHE-001**: `PromptCachePlanner.Plan()` 소비
- **TOOLS-001**: tool_use 전달 (tool 실행은 TOOLS-001 담당)

## 파일 구조

```
internal/llm/provider/
├── provider.go           # Provider interface 정의
├── registry.go           # ProviderRegistry 구현
├── llm_call.go           # QUERY-001 LLMCallFunc 구현
├── fallback.go           # Fallback model chain
├── secret.go             # Secret 관리 (API key, OAuth token)
├── constants.go          # 상수 정의
├── errors.go             # 패키지 에러 정의
├── anthropic/            # Anthropic provider (OAuth, thinking, cache)
├── openai/               # OpenAI provider (Chat Completions, Tools)
├── google/               # Google Gemini provider (SDK-based)
├── ollama/               # Ollama local provider
├── xai/                  # xAI Grok provider (OpenAI-compat)
├── deepseek/             # DeepSeek provider (OpenAI-compat)
├── cerebras/             # Cerebras (metadata-only)
├── fireworks/            # Fireworks (metadata-only)
├── groq/                 # Groq (metadata-only)
├── mistral/              # Mistral (metadata-only)
├── openrouter/           # OpenRouter (metadata-only)
├── kimi/                 # Kimi (metadata-only)
├── qwen/                 # Qwen (metadata-only)
├── together/             # Together (metadata-only)
├── glm/                  # GLM (metadata-only)
└── testhelper/           # 테스트 헬퍼
```

## 관련 SPEC

- **SPEC-GOOSE-ADAPTER-001**: 본 패키지의 주요 SPEC (P0, Phase 1)
- **SPEC-GOOSE-ADAPTER-002**: OpenAI ExtraHeaders/ExtraFields 확장
- **SPEC-GOOSE-QUERY-001**: LLMCallFunc 시그니처 정의
- **SPEC-GOOSE-CREDPOOL-001**: Credential pool 및 Refresher interface
- **SPEC-GOOSE-ROUTER-001**: Route 결정 로직
- **SPEC-GOOSE-RATELIMIT-001**: Rate limit tracking
- **SPEC-GOOSE-PROMPT-CACHE-001**: Prompt cache planning

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-ADAPTER-001
