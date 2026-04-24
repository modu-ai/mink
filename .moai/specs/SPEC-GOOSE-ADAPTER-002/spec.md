---
id: SPEC-GOOSE-ADAPTER-002
version: 1.0.0
status: implemented
created_at: 2026-04-24
updated_at: 2026-04-25
author: manager-spec
priority: high
issue_number: null
phase: 1
size: 대(L)
lifecycle: spec-anchored
labels: [llm, adapter, provider, openai-compat, glm, groq, openrouter, mistral, qwen, kimi]
---

# SPEC-GOOSE-ADAPTER-002 — 9 OpenAI-compat Provider 어댑터 확장 (GLM/Groq/OpenRouter/Together/Fireworks/Cerebras/Mistral/Qwen/Kimi)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-24 | 초안 작성 (SPEC-ADAPTER-001 재사용 + 9 provider 확장 + Z.ai GLM 공식 endpoint 이전) | manager-spec |
| 1.0.0 | 2026-04-25 | ADAPTER-002 감사 리포트 결함 수정 — (1) frontmatter 스키마 정합화(priority/status/labels/version), (2) REQ-ADP2-007 thinking-capable 모델 리스트에 `glm-4.5` 추가하여 구현과 일관화(D6), (3) REQ-ADP2-020/021/022 (OpenRouter PreferredProviders / GLM budget_tokens / Kimi 장문 context INFO)를 §11 Open Items로 이관하고 `[PENDING v0.3]` 주석 부착(D2/D3/D4), (4) RegisterAllProviders 15-way 등록은 Phase C1 코드 수정으로 해결됨을 명시(D1). | manager-spec |

---

## 1. 개요 (Overview)

GOOSE의 LLM provider 생태계를 **6개에서 15개로 확장**한다. SPEC-GOOSE-ADAPTER-001이 구현한 `internal/llm/provider/openai/` 어댑터를 **BaseURL override + Capabilities 주입** 패턴으로 재사용하여 9개 provider(Z.ai GLM, Groq, OpenRouter, Together AI, Fireworks AI, Cerebras, Mistral AI, Qwen, Kimi)를 추가한다.

본 SPEC이 Plan·Run을 통과한 시점에서:

- 15 provider 전부가 `ProviderRegistry`에 `AdapterReady=true`로 등록되며,
- 8 provider(Groq/OpenRouter/Together/Fireworks/Cerebras/Mistral/Qwen/Kimi)는 `func New(...) (*openai.OpenAIAdapter, error)` 팩토리 파일로 구현되고,
- 1 provider(GLM)는 thinking mode 래핑을 위한 `*glm.Adapter` 커스텀 타입으로 구현되며,
- Z.ai GLM endpoint가 구 `open.bigmodel.cn` → 공식 `api.z.ai/api/paas/v4`로 이전되고,
- Qwen과 Kimi는 국제판/중국판 base URL 선택을 config로 지원한다.

본 SPEC은 **SPEC-GOOSE-ADAPTER-001의 확장 SPEC**이며, SPEC-001이 제공하는 `openai.OpenAIAdapter`의 Options 팩토리 패턴에 직접 의존한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **2026-04 시점 시장 확장**: GLM-4.6(357B MoE), Kimi K2.6(1T MoE, 262K context), Qwen3.6 Max Preview(1T MoE) 등 주요 신모델이 연달아 출시되어, GOOSE가 이들을 routing할 수 있어야 competitive.
- **Z.ai GLM endpoint 불일치 해소**: SPEC-001의 router registry는 `https://open.bigmodel.cn/api/paas/v4`(구 ZhipuAI)를 등록했으나, 2026년 공식 endpoint는 `https://api.z.ai/api/paas/v4`. 본 SPEC에서 교체 필수.
- **CG Mode 비용 최적화 지원**: MoAI의 CG Mode(Claude + GLM cost optimization)가 제대로 동작하려면 GLM 실 어댑터 필요.
- **무료/저가 implementation 경로**: Groq(30 RPM 무료), OpenRouter(29 free models), Cerebras(무료 tier), Mistral Nemo($0.02/M) 확보로 개발 단계 비용 최소화.

### 2.2 상속 자산

- **SPEC-GOOSE-ADAPTER-001**: `internal/llm/provider/openai/` 어댑터 (BaseURL/Capabilities override가 가능한 `openai.NewWithBase(Options)` 팩토리), `Provider` interface, `ProviderRegistry`, `NewLLMCall`, `Capabilities` 구조체, `credential.CredentialPool`, `ratelimit.Tracker` 연계. **본 SPEC은 이들을 소비한다.**
- **SPEC-001의 xAI/DeepSeek 구현**: 동일 패턴(`base_url` override로 openai 어댑터 재사용)의 참조 구현. 본 SPEC의 8/9 provider가 이 패턴을 정확히 복제.
- **Router registry 기존 metadata**: GLM, Groq, OpenRouter, Mistral, Qwen, Kimi는 이미 metadata-only로 등록되어 있음. Cerebras, Together, Fireworks는 본 SPEC에서 신규 등록.

### 2.3 범위 경계

- **IN**: 9개 provider의 `func New(...)` 팩토리 구현, `Provider` interface 준수(`Name()`, `Capabilities()`, `Complete()`, `Stream()`), router registry 업데이트(metadata 수정 + AdapterReady=true + 신규 등록), GLM thinking mode 지원, Qwen/Kimi 지역 URL 선택, OpenRouter 선택적 ranking 헤더, 모든 provider의 httptest stub 기반 단위 테스트.
- **OUT**: SPEC-001에서 이미 완성된 6 provider(Anthropic/OpenAI/Google/xAI/DeepSeek/Ollama) 재구현, embedding 엔드포인트, audio/video modality, fine-tuning API, Perplexity/MiniMax/Nous 실 어댑터, Kimi Anthropic-compat endpoint(OpenAI-compat만), 새로운 외부 SDK 도입(`net/http` 직접 또는 SPEC-001의 `go-openai` 재사용).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. **GLM 어댑터** (`internal/llm/provider/glm/`):
   - `adapter.go`: `openai.OpenAIAdapter`를 embedding한 `*GLMAdapter` 타입
   - `thinking.go`: GLM 모델별 thinking mode on/off 판별 + 파라미터 주입
   - `func New(pool, tracker, secretStore, logger) (*GLMAdapter, error)`
2. **Groq 어댑터** (`internal/llm/provider/groq/`):
   - `client.go`: `openai.NewWithBase` 래핑 단일 팩토리
   - `func New(pool, tracker, secretStore, logger) (*openai.OpenAIAdapter, error)`
3. **OpenRouter 어댑터** (`internal/llm/provider/openrouter/`):
   - `client.go`: `openai.NewWithBase` 래핑 + optional `HTTP-Referer`/`X-Title` 헤더 주입
4. **Together 어댑터** (`internal/llm/provider/together/client.go`): `openai.NewWithBase` 단일 팩토리
5. **Fireworks 어댑터** (`internal/llm/provider/fireworks/client.go`): 동일 패턴
6. **Cerebras 어댑터** (`internal/llm/provider/cerebras/client.go`): 동일 패턴
7. **Mistral 어댑터** (`internal/llm/provider/mistral/client.go`): 동일 패턴
8. **Qwen 어댑터** (`internal/llm/provider/qwen/client.go`): 지역 선택(intl/cn) + `openai.NewWithBase`
9. **Kimi 어댑터** (`internal/llm/provider/kimi/client.go`): 지역 선택(intl/cn) + `openai.NewWithBase` (OpenAI-compat endpoint만)
10. **Router registry 업데이트** (`internal/llm/router/registry.go`):
    - GLM: BaseURL `open.bigmodel.cn` → `api.z.ai/api/paas/v4`, suggested_models 갱신(`glm-5`, `glm-4.7`, `glm-4.6`, `glm-4.5`, `glm-4.5-air`)
    - GLM/Groq/OpenRouter/Mistral/Qwen/Kimi: `AdapterReady=false` → `true`
    - Together/Fireworks/Cerebras: 신규 ProviderMeta 등록(`AdapterReady=true`)
    - suggested_models는 research.md §2 참조하여 최신화
11. **Provider 등록 helper** (`internal/llm/provider/registry_builder.go` 또는 기존 확장):
    - `RegisterAllProviders(reg, pool, tracker, secretStore, logger)` — 15 provider 일괄 등록 헬퍼
12. **httptest stub 기반 단위 테스트**: 각 provider별 최소 3개 (streaming happy path, rate limit 429 회전, capability mismatch 거부)
13. **통합 테스트**: DefaultRegistry의 15 provider 전부 `AdapterReady=true` 검증, `RegisterAllProviders` 무에러 동작 검증

### 3.2 OUT OF SCOPE

- **SPEC-001에서 이미 완성된 provider 재구현**: Anthropic/OpenAI/Google/xAI/DeepSeek/Ollama. 본 SPEC은 이들을 **수정하지 않는다**.
- **Embedding 엔드포인트**: 후속 SPEC (Memory/Vector Phase 6).
- **Audio/Video modality**: 후속 SPEC.
- **Fine-tuning/Training API**: 범위 외.
- **Perplexity 실 어댑터**: Web search 통합이 별도 SPEC 필요, 본 SPEC 제외.
- **MiniMax 실 어댑터**: 사용자 선택에 없으며 수요 검증 후 결정.
- **Nous Research 실 어댑터**: 사용자 선택에 없음(ADAPTER-001 metadata-only 유지).
- **Cohere 실 어댑터**: 사용자 선택에 없음(ADAPTER-001 metadata-only 유지).
- **Kimi Anthropic-compat endpoint** (`/anthropic/v1`): OpenAI-compat endpoint(`/v1`)만 본 SPEC 대상. Anthropic-compat은 후속 SPEC.
- **OAuth 인증 provider 추가**: 9 provider 모두 `api_key` 방식. OAuth 확장은 ADAPTER-001 Anthropic 패턴을 참고하여 후속 SPEC에서.
- **Usage/cost 계산**: 후속 메트릭 SPEC.
- **Live API 통합 테스트**: 본 SPEC은 httptest stub만. 실 API 호출 테스트는 CI secret + 수동 트리거로 후속.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-ADP2-001 [Ubiquitous]** — Every Tier 1/Tier 2 adapter defined in this SPEC **shall** reuse `openai.OpenAIAdapter` via `openai.NewWithBase(Options)` with BaseURL and Capabilities override; **shall not** duplicate HTTP request/response handling logic.

**REQ-ADP2-002 [Ubiquitous]** — Every adapter **shall** implement `Name() string` returning the canonical provider identifier (`glm`, `groq`, `openrouter`, `together`, `fireworks`, `cerebras`, `mistral`, `qwen`, `kimi`) and `Capabilities() Capabilities` with provider-specific flags.

**REQ-ADP2-003 [Ubiquitous]** — Every adapter **shall** propagate rate limit response headers to `ratelimit.Tracker.Parse(provider, resp.Header, now)` by reusing the underlying `openai.OpenAIAdapter` mechanism; no per-adapter header parser is required.

**REQ-ADP2-004 [Ubiquitous]** — Every adapter **shall** log only non-PII structured fields (`{provider, model, message_count, tokens_estimated}`); **shall not** log message content.

**REQ-ADP2-005 [Ubiquitous]** — The `ProviderRegistry` **shall** return 15 ProviderMeta entries with `AdapterReady=true` after `RegisterAllProviders` completes; lookup for any of the 9 new provider names **shall** succeed.

**REQ-ADP2-006 [Ubiquitous]** — Every adapter's factory function `func New(pool, tracker, secretStore, logger)` **shall** return an `error` on invalid inputs (nil pool, missing required config) without panicking.

### 4.2 Event-Driven

**REQ-ADP2-007 [Event-Driven]** — **When** `GLMAdapter.Stream(ctx, req)` is invoked with `req.Route.Model` matching a thinking-capable model (`glm-4.5`, `glm-4.6`, `glm-4.7`, `glm-5`), and `req.Thinking.Enabled == true`, the adapter **shall** inject the request body key `"thinking": {"type": "enabled"}` before delegating to the embedded `openai.OpenAIAdapter`; for non-thinking models or `Thinking.Enabled == false`, **shall not** inject the parameter. (v1.0.0: `glm-4.5` 추가 — D6 감사 수정, 구현 `glm/thinking.go:L14-L19`와 일관화)

**REQ-ADP2-008 [Event-Driven]** — **When** an OpenRouter adapter is initialized with `OpenRouterOptions{HTTPReferer: "...", XTitle: "..."}`, the adapter **shall** include `HTTP-Referer` and `X-Title` headers in every outgoing HTTP request; when these fields are empty, the headers **shall** be omitted.

**REQ-ADP2-009 [Event-Driven]** — **When** any adapter receives HTTP 429 with `Retry-After` header, the adapter **shall** trigger `CredentialPool.MarkExhaustedAndRotate` via the inherited `openai.OpenAIAdapter` logic and attempt one retry with the rotated credential; no new logic is introduced in the wrapper.

**REQ-ADP2-010 [Event-Driven]** — **When** `RegisterAllProviders(reg, pool, tracker, secretStore, logger)` is called, the function **shall** iterate through 15 provider factories (6 from SPEC-001 + 9 from this SPEC), invoke each `func New(...)`, and register the result via `reg.Register(provider)`; **shall** return the first error encountered.

### 4.3 State-Driven

**REQ-ADP2-011 [State-Driven]** — **While** `QwenOptions.Region == "cn"` or the environment variable `GOOSE_QWEN_REGION=cn` is set, the adapter **shall** use `https://dashscope.aliyuncs.com/compatible-mode/v1`; otherwise the default `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` is used.

**REQ-ADP2-012 [State-Driven]** — **While** `KimiOptions.Region == "cn"` or the environment variable `GOOSE_KIMI_REGION=cn` is set, the adapter **shall** use `https://api.moonshot.cn/v1`; otherwise `https://api.moonshot.ai/v1` is used.

**REQ-ADP2-013 [State-Driven]** — **While** a provider's `Capabilities.Vision == false` and `CompletionRequest.Vision != nil`, the adapter **shall** return `ErrCapabilityUnsupported{feature:"vision", provider:...}` before any HTTP call is made; the existing SPEC-001 error type **shall** be reused.

### 4.4 Unwanted Behavior

**REQ-ADP2-014 [Unwanted]** — **If** `GLMAdapter.Stream` is invoked with a model name that is not in the thinking-capable list AND `req.Thinking.Enabled == true`, **then** the adapter **shall** log a `WARN` and proceed WITHOUT injecting thinking param; **shall not** return an error (graceful degradation).

**REQ-ADP2-015 [Unwanted]** — The unit tests in this SPEC **shall not** issue HTTP calls to any external API (`api.z.ai`, `api.groq.com`, `openrouter.ai`, etc.); all tests **shall** use `httptest.NewServer` stubs. Live API smoke tests are OUT OF SCOPE for this SPEC.

**REQ-ADP2-016 [Unwanted]** — **If** any of the 9 new adapter factories returns a provider whose `Name()` collides with an existing registered provider (from SPEC-001 or another SPEC-002 adapter), **then** `RegisterAllProviders` **shall** return an error identifying the duplicate and **shall not** partially register.

**REQ-ADP2-017 [Unwanted]** — The adapters **shall not** introduce new external Go dependencies beyond what SPEC-001 already pulls in (`sashabaranov/go-openai`, `net/http`, `zap`). No `github.com/alibabacloud-go/dashscope-sdk` or provider-specific SDK is permitted.

**REQ-ADP2-018 [Unwanted]** — **If** a user sets `GOOSE_QWEN_REGION` to any value other than `cn` or `intl`, **then** the adapter **shall** return an `ErrInvalidRegion` at `New()` time; **shall not** silently fall back.

### 4.5 Optional

**REQ-ADP2-019 [Optional]** — **Where** `CompletionRequest.ResponseFormat == "json"` and the resolved provider's model supports JSON mode (Mistral, Qwen, GLM-4.6+), the adapter **shall** forward the `response_format: {type: "json_object"}` parameter by reusing SPEC-001's existing OpenAI-compat JSON mode logic.

**REQ-ADP2-020 [Optional]** `[PENDING v0.3]` — **Where** `OpenRouterOptions.PreferredProviders` is a non-empty slice, the adapter **shall** include the `"provider": {"order": [...], "allow_fallbacks": true}` field in the request body to route to specific upstream providers. (v1.0.0: 미구현 — `openrouter/client.go`의 `Options` 구조체에 `PreferredProviders` 필드 부재. §11 Open Items OI-1 참조. D4 감사.)

**REQ-ADP2-021 [Optional]** `[PENDING v0.3]` — **Where** `GLMOptions.ThinkingBudget` is explicitly set (int > 0), and the resolved model supports budget-based thinking, the adapter **shall** include `"thinking": {"type": "enabled", "budget_tokens": N}` instead of the default enabled-only form. (v1.0.0: 미구현 — `glm/thinking.go:L40-L44` `BuildThinkingField`가 `cfg.BudgetTokens`를 읽지 않음. §11 Open Items OI-2 참조. D2 감사.)

**REQ-ADP2-022 [Optional]** `[PENDING v0.3]` — **Where** Kimi is routing to a `moonshot-v1-128k`-class model with input messages exceeding 64K tokens, the adapter **shall** log an INFO-level advisory that a long-context model is in use; this is observational only. (v1.0.0: 미구현 — `kimi/client.go`에 token-count 추정 헬퍼 및 INFO 로그 부재. §11 Open Items OI-3 참조. D3 감사.)

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-ADP2-001 — GLM 기본 streaming (thinking off)**
- **Given** ProviderRegistry에 GLM 어댑터 등록, 유효한 API key, Route `{model:"glm-4.5-air", provider:"glm"}`, `req.Thinking=nil`
- **When** `LLMCall(ctx, req)` 호출 후 stream drain
- **Then** HTTP 요청이 `https://api.z.ai/api/paas/v4/chat/completions`로 전송, body에 `thinking` 파라미터 부재, SSE text_delta chunk 수신, `RateLimitTracker`에 `glm` 상태 갱신

**AC-ADP2-002 — GLM thinking mode on (GLM-4.6)**
- **Given** Route `{model:"glm-4.6", provider:"glm"}`, `req.Thinking={Enabled:true}`
- **When** `LLMCall` 호출
- **Then** HTTP request body에 `"thinking": {"type": "enabled"}` 포함, `budget_tokens` 필드 부재(REQ-ADP2-007), streaming 정상

**AC-ADP2-003 — GLM thinking graceful degradation**
- **Given** Route `{model:"glm-4.5-air", provider:"glm"}`(thinking 미지원), `req.Thinking={Enabled:true}`
- **When** `LLMCall` 호출
- **Then** WARN 로그 1건 기록, HTTP body에 `thinking` 부재(REQ-ADP2-014), streaming 정상(에러 없음)

**AC-ADP2-004 — Groq streaming + rate limit 전파**
- **Given** Groq 어댑터, 모델 `llama-3.3-70b-versatile`, stub이 `x-ratelimit-remaining-requests: 29` 헤더 반환
- **When** `LLMCall`
- **Then** HTTP 요청이 `https://api.groq.com/openai/v1/chat/completions`, `Tracker.Parse("groq", headers, now)` 호출, streaming 정상

**AC-ADP2-005 — OpenRouter ranking 헤더 주입**
- **Given** OpenRouter 어댑터를 `HTTPReferer:"https://goose.modu-ai.dev", XTitle:"GOOSE CLI"`로 초기화, 모델 `deepseek/deepseek-r1:free`
- **When** `LLMCall`
- **Then** HTTP 요청 헤더에 `HTTP-Referer: https://goose.modu-ai.dev`와 `X-Title: GOOSE CLI` 포함(REQ-ADP2-008), body routing은 정상

**AC-ADP2-006 — Together streaming**
- **Given** Together 어댑터, 모델 `meta-llama/Llama-3.3-70B-Instruct-Turbo`
- **When** `LLMCall`
- **Then** HTTP 요청이 `https://api.together.xyz/v1/chat/completions`, streaming 정상

**AC-ADP2-007 — Fireworks streaming**
- **Given** Fireworks 어댑터, 모델 `accounts/fireworks/models/deepseek-r1`
- **When** `LLMCall`
- **Then** HTTP 요청이 `https://api.fireworks.ai/inference/v1/chat/completions`, streaming 정상

**AC-ADP2-008 — Cerebras streaming**
- **Given** Cerebras 어댑터, 모델 `llama-3.3-70b`
- **When** `LLMCall`
- **Then** HTTP 요청이 `https://api.cerebras.ai/v1/chat/completions`, streaming 정상. stub 응답 latency 계측이 어댑터 함수 내에서 추가 오버헤드를 발생시키지 않음

**AC-ADP2-009 — Mistral streaming + JSON mode**
- **Given** Mistral 어댑터, 모델 `mistral-small-latest`, `req.ResponseFormat="json"`
- **When** `LLMCall`
- **Then** HTTP body에 `"response_format": {"type": "json_object"}` 포함(REQ-ADP2-019), streaming 정상

**AC-ADP2-010 — Qwen 지역 URL 선택 (intl 기본)**
- **Given** `QwenOptions{}` (Region 미지정), `GOOSE_QWEN_REGION` 환경변수 없음
- **When** `New(...)` 호출 후 `LLMCall`
- **Then** HTTP 요청이 `https://dashscope-intl.aliyuncs.com/compatible-mode/v1/chat/completions`(REQ-ADP2-011)

**AC-ADP2-011 — Qwen 지역 URL 선택 (cn 환경변수)**
- **Given** 환경변수 `GOOSE_QWEN_REGION=cn`, `QwenOptions{}` (Region 미지정)
- **When** `New(...)` 호출 후 `LLMCall`
- **Then** HTTP 요청이 `https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions`

**AC-ADP2-012 — Qwen 잘못된 지역 거부**
- **Given** 환경변수 `GOOSE_QWEN_REGION=foo`
- **When** `qwen.New(...)` 호출
- **Then** `ErrInvalidRegion{region:"foo"}` 반환(REQ-ADP2-018), provider 생성 실패

**AC-ADP2-013 — Kimi 장문 context 경고** `[PENDING v0.3]`
- **Given** Kimi 어댑터, 모델 `moonshot-v1-128k`, 입력 메시지 token 합계 70K
- **When** `LLMCall`
- **Then** INFO 로그 1건 기록(REQ-ADP2-022), streaming 정상
- **v1.0.0 상태**: 미구현. token 추정 로직 + INFO 로그는 §11 Open Items OI-3에서 후속 이터레이션에 배정. D3 감사.

**AC-ADP2-014 — Kimi 지역 URL 선택**
- **Given** `KimiOptions{Region:"cn"}`
- **When** `LLMCall`
- **Then** HTTP 요청이 `https://api.moonshot.cn/v1/chat/completions`(REQ-ADP2-012)

**AC-ADP2-015 — Vision 미지원 거부**
- **Given** Route `{provider:"groq", model:"llama-3.3-70b-versatile"}` (Capabilities.Vision=false), `CompletionRequest.Vision` non-nil
- **When** `LLMCall`
- **Then** `ErrCapabilityUnsupported{feature:"vision", provider:"groq"}` 반환(REQ-ADP2-013), HTTP 호출 발생하지 않음

**AC-ADP2-016 — DefaultRegistry 15 provider 전부 AdapterReady**
- **Given** 본 SPEC Run phase 완료 후 `router.DefaultRegistry()` 호출
- **When** `reg.List()` 실행
- **Then** 15개 ProviderMeta 반환, 전부 `AdapterReady==true`, `glm.DefaultBaseURL == "https://api.z.ai/api/paas/v4"`, `cerebras`/`together`/`fireworks` 3종 모두 존재

**AC-ADP2-017 — RegisterAllProviders 무에러**
- **Given** 빈 `*ProviderRegistry`, 유효한 pool/tracker/secretStore/logger
- **When** `RegisterAllProviders(reg, pool, tracker, secretStore, logger)` 호출
- **Then** nil 에러 반환, `reg.List()`가 15개 `provider.Provider` 인스턴스 반환, 각 인스턴스의 `Name()`이 고유

**AC-ADP2-018 — Provider 이름 중복 거부**
- **Given** `reg`에 이미 `glm` provider가 등록된 상태
- **When** `glm.New(...)` 결과를 다시 `reg.Register()` 시도
- **Then** 중복 에러 반환(REQ-ADP2-016), registry 상태 불변

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/llm/provider/
├── glm/
│   ├── adapter.go       # openai.OpenAIAdapter embedding + Stream override
│   └── thinking.go      # glm-5/4.7/4.6 adaptive 판별, thinking param 주입
├── groq/
│   └── client.go        # openai.NewWithBase 래핑 단일 팩토리 (~35 LOC)
├── openrouter/
│   └── client.go        # + optional HTTP-Referer/X-Title 헤더 (~50 LOC)
├── together/
│   └── client.go        # ~35 LOC
├── fireworks/
│   └── client.go        # ~35 LOC
├── cerebras/
│   └── client.go        # ~35 LOC
├── mistral/
│   └── client.go        # ~35 LOC
├── qwen/
│   ├── client.go        # 지역 선택 (intl/cn)
│   └── region.go        # ErrInvalidRegion + resolveBaseURL
├── kimi/
│   ├── client.go        # 지역 선택 (intl/cn)
│   └── region.go        # Kimi-specific region resolution
└── registry_builder.go  # RegisterAllProviders(15 provider 일괄 등록)
```

### 6.2 핵심 타입

```go
// internal/llm/provider/glm/adapter.go
package glm

import (
    "context"
    "github.com/modu-ai/goose/internal/llm/provider"
    "github.com/modu-ai/goose/internal/llm/provider/openai"
    "github.com/modu-ai/goose/internal/message"
    // ...
)

type GLMAdapter struct {
    *openai.OpenAIAdapter
    thinkingModels map[string]bool  // "glm-4.6", "glm-4.7", "glm-5"
    logger         *zap.Logger
}

func New(pool *credential.CredentialPool, tracker *ratelimit.Tracker, secretStore secret.Store, logger *zap.Logger) (*GLMAdapter, error) {
    base, err := openai.NewWithBase(openai.Options{
        Name:         "glm",
        BaseURL:      "https://api.z.ai/api/paas/v4",
        Pool:         pool,
        Tracker:      tracker,
        SecretStore:  secretStore,
        Logger:       logger,
        Capabilities: provider.Capabilities{
            Streaming:        true,
            Tools:            true,
            Vision:           true,  // GLM-4.6+
            Embed:            false,
            MaxContextTokens: 200_000,
        },
    })
    if err != nil {
        return nil, err
    }
    return &GLMAdapter{
        OpenAIAdapter:  base,
        thinkingModels: map[string]bool{
            "glm-4.6": true, "glm-4.7": true, "glm-5": true,
        },
        logger: logger,
    }, nil
}

func (g *GLMAdapter) Stream(ctx context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
    // thinking 파라미터 결정 (thinking.go 참조)
    if shouldInjectThinking(req, g.thinkingModels, g.logger) {
        req = injectThinkingParam(req)
    }
    return g.OpenAIAdapter.Stream(ctx, req)
}
```

```go
// internal/llm/provider/groq/client.go (~35 LOC)
package groq

import (
    "github.com/modu-ai/goose/internal/llm/provider"
    "github.com/modu-ai/goose/internal/llm/provider/openai"
    // ...
)

func New(pool *credential.CredentialPool, tracker *ratelimit.Tracker, secretStore secret.Store, logger *zap.Logger) (*openai.OpenAIAdapter, error) {
    return openai.NewWithBase(openai.Options{
        Name:    "groq",
        BaseURL: "https://api.groq.com/openai/v1",
        Pool:    pool,
        Tracker: tracker,
        SecretStore: secretStore,
        Logger:  logger,
        Capabilities: provider.Capabilities{
            Streaming: true,
            Tools:     true,
            Vision:    false,
            Embed:     false,
        },
    })
}
```

```go
// internal/llm/provider/qwen/region.go
package qwen

import (
    "errors"
    "os"
)

var ErrInvalidRegion = errors.New("qwen: invalid region; must be 'intl' or 'cn'")

func resolveBaseURL(optsRegion string) (string, error) {
    region := optsRegion
    if region == "" {
        region = os.Getenv("GOOSE_QWEN_REGION")
    }
    switch region {
    case "", "intl":
        return "https://dashscope-intl.aliyuncs.com/compatible-mode/v1", nil
    case "cn":
        return "https://dashscope.aliyuncs.com/compatible-mode/v1", nil
    default:
        return "", ErrInvalidRegion
    }
}
```

```go
// internal/llm/provider/registry_builder.go
package provider

func RegisterAllProviders(
    reg *router.ProviderRegistry,  // 또는 ProviderRegistry(provider.go)
    pool *credential.CredentialPool,
    tracker *ratelimit.Tracker,
    secretStore secret.Store,
    logger *zap.Logger,
) error {
    // SPEC-001의 6 provider
    // anthropic, openai, google, xai, deepseek, ollama
    // (SPEC-001에서 제공된 factory 호출)

    // SPEC-002의 9 provider
    glmAdapter, err := glm.New(pool, tracker, secretStore, logger)
    if err != nil { return err }
    if err := reg.Register(glmAdapter); err != nil { return err }

    groqAdapter, err := groq.New(pool, tracker, secretStore, logger)
    if err != nil { return err }
    if err := reg.Register(groqAdapter); err != nil { return err }

    // ... 나머지 7 provider 동일 패턴

    return nil
}
```

### 6.3 Router Registry 업데이트 (internal/llm/router/registry.go)

```go
// DefaultRegistry() 내부 수정 및 추가

// GLM — BaseURL 이전 + suggested_models 갱신 + AdapterReady=true
mustRegister(reg, &ProviderMeta{
    Name:            "glm",
    DisplayName:     "Z.ai GLM",                                 // 변경: "GLM (ZhipuAI)" → "Z.ai GLM"
    DefaultBaseURL:  "https://api.z.ai/api/paas/v4",             // 변경: open.bigmodel.cn → api.z.ai
    AuthType:        "api_key",
    SupportsStream:  true,
    SupportsTools:   true,
    SupportsVision:  true,
    SupportsEmbed:   false,
    AdapterReady:    true,                                        // 변경: false → true
    SuggestedModels: []string{"glm-5", "glm-4.7", "glm-4.6", "glm-4.5", "glm-4.5-air"}, // 변경
})

// Groq — AdapterReady=true + suggested_models 확장
mustRegister(reg, &ProviderMeta{
    Name:            "groq",
    DisplayName:     "Groq",
    DefaultBaseURL:  "https://api.groq.com/openai/v1",
    AuthType:        "api_key",
    SupportsStream:  true,
    SupportsTools:   true,
    SupportsVision:  false,
    SupportsEmbed:   false,
    AdapterReady:    true,
    SuggestedModels: []string{
        "llama-3.3-70b-versatile", "llama-4-scout", "llama-4-maverick",
        "deepseek-r1-distill-llama-70b", "mistral-saba-24b",
        "qwen-qwq-32b", "mixtral-8x7b-32768", "gemma2-9b-it",
    },
})

// OpenRouter — AdapterReady=true + suggested_models 확장
// Mistral — AdapterReady=true + suggested_models 확장
// Qwen — AdapterReady=true + suggested_models 확장
// Kimi — AdapterReady=true + suggested_models 확장

// 신규: Together AI
mustRegister(reg, &ProviderMeta{
    Name:            "together",
    DisplayName:     "Together AI",
    DefaultBaseURL:  "https://api.together.xyz/v1",
    AuthType:        "api_key",
    SupportsStream:  true,
    SupportsTools:   true,
    SupportsVision:  false,
    SupportsEmbed:   true,
    AdapterReady:    true,
    SuggestedModels: []string{
        "meta-llama/Llama-3.3-70B-Instruct-Turbo",
        "meta-llama/Llama-4-Scout-17B-16E-Instruct",
        "deepseek-ai/DeepSeek-R1",
        "Qwen/Qwen2.5-72B-Instruct-Turbo",
        "mistralai/Mixtral-8x22B-Instruct-v0.1",
        "zai-org/GLM-4.6",
    },
})

// 신규: Fireworks AI
// 신규: Cerebras
```

### 6.4 GLM thinking 파라미터 주입

```go
// internal/llm/provider/glm/thinking.go
package glm

import (
    "github.com/modu-ai/goose/internal/llm/provider"
    "go.uber.org/zap"
)

func shouldInjectThinking(req provider.CompletionRequest, thinkingModels map[string]bool, logger *zap.Logger) bool {
    if req.Thinking == nil || !req.Thinking.Enabled {
        return false
    }
    if !thinkingModels[req.Route.Model] {
        logger.Warn("glm: thinking requested but model does not support it; skipping",
            zap.String("model", req.Route.Model))
        return false
    }
    return true
}

func injectThinkingParam(req provider.CompletionRequest) provider.CompletionRequest {
    // openai.OpenAIAdapter는 req.ExtraRequestFields (map[string]any) 지원 가정 (SPEC-001에서 제공)
    // 없다면 SPEC-001 확장 협의 필요 — research.md의 전제 조건
    if req.ExtraRequestFields == nil {
        req.ExtraRequestFields = make(map[string]any)
    }
    thinkingPayload := map[string]any{"type": "enabled"}
    if req.Thinking.BudgetTokens > 0 {
        thinkingPayload["budget_tokens"] = req.Thinking.BudgetTokens
    }
    req.ExtraRequestFields["thinking"] = thinkingPayload
    return req
}
```

### 6.5 OpenRouter 선택적 헤더 주입

SPEC-001의 `openai.Options`에 `ExtraHeaders map[string]string` 필드가 있다는 가정 하에 (없다면 SPEC-001 확장 협의):

```go
// internal/llm/provider/openrouter/client.go
func New(pool, tracker, secretStore, logger, opts Options) (*openai.OpenAIAdapter, error) {
    extraHeaders := map[string]string{}
    if opts.HTTPReferer != "" {
        extraHeaders["HTTP-Referer"] = opts.HTTPReferer
    }
    if opts.XTitle != "" {
        extraHeaders["X-Title"] = opts.XTitle
    }
    return openai.NewWithBase(openai.Options{
        Name:         "openrouter",
        BaseURL:      "https://openrouter.ai/api/v1",
        Pool:         pool, Tracker: tracker, SecretStore: secretStore, Logger: logger,
        Capabilities: provider.Capabilities{Streaming: true, Tools: true, Vision: true, Embed: false},
        ExtraHeaders: extraHeaders,
    })
}
```

### 6.6 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1**: `TestGLM_Stream_NoThinking_HappyPath` — AC-ADP2-001
2. **RED #2**: `TestGLM_Stream_ThinkingEnabled_ModelSupports` — AC-ADP2-002
3. **RED #3**: `TestGLM_Stream_ThinkingRequested_ModelUnsupported` — AC-ADP2-003
4. **RED #4**: `TestGroq_Stream_HappyPath` — AC-ADP2-004
5. **RED #5**: `TestOpenRouter_RankingHeadersInjected` — AC-ADP2-005
6. **RED #6**: `TestTogether_Stream_HappyPath` — AC-ADP2-006
7. **RED #7**: `TestFireworks_Stream_HappyPath` — AC-ADP2-007
8. **RED #8**: `TestCerebras_Stream_HappyPath` — AC-ADP2-008
9. **RED #9**: `TestMistral_Stream_JSONMode` — AC-ADP2-009
10. **RED #10**: `TestQwen_Region_DefaultIntl` — AC-ADP2-010
11. **RED #11**: `TestQwen_Region_EnvCN` — AC-ADP2-011
12. **RED #12**: `TestQwen_Region_InvalidRejected` — AC-ADP2-012
13. **RED #13**: `TestKimi_LongContext_Advisory` — AC-ADP2-013
14. **RED #14**: `TestKimi_Region_CN` — AC-ADP2-014
15. **RED #15**: `TestVisionCapability_Rejected` — AC-ADP2-015
16. **RED #16**: `TestDefaultRegistry_All15Ready` — AC-ADP2-016
17. **RED #17**: `TestRegisterAllProviders_Success` — AC-ADP2-017
18. **RED #18**: `TestRegisterAllProviders_DuplicateError` — AC-ADP2-018
19. **GREEN**: 9 어댑터 최소 구현 (Groq 먼저 — 가장 단순, 이후 Together/Fireworks/Cerebras/Mistral 동일 패턴, Qwen/Kimi 지역 로직, OpenRouter 헤더, GLM thinking)
20. **REFACTOR**: 공통 팩토리 헬퍼 추출, `RegisterAllProviders`의 DRY 개선, 문자열 중복 상수화

### 6.7 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| Tested | httptest stub 기반 85%+ 커버리지, race detector 통과, live API 호출 금지 |
| Readable | provider별 단일 파일(client.go 또는 adapter.go) 35-150 LOC 유지, openai embedding 패턴 통일 |
| Unified | 9/9 provider가 동일 `func New(pool, tracker, secretStore, logger)` 시그니처, 예외는 GLM(반환 타입)과 OpenRouter(Options 파라미터)만 |
| Secured | API key redaction은 SPEC-001의 `openai.OpenAIAdapter`에서 상속, message 로깅 금지(REQ-ADP2-004) |
| Trackable | 모든 구현 커밋에 `SPEC-GOOSE-ADAPTER-002` 참조, 각 provider 추가는 독립 커밋 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| **선행 SPEC (Critical)** | **SPEC-GOOSE-ADAPTER-001** | `internal/llm/provider/openai/` 패키지(특히 `openai.NewWithBase(Options)`, `OpenAIAdapter` 타입, `Options.ExtraHeaders`, `CompletionRequest.ExtraRequestFields`) 가 main에 merge된 후에만 본 SPEC 구현 착수 가능 |
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | `LLMCallFunc`, `message.StreamEvent` |
| 선행 SPEC | SPEC-GOOSE-CREDPOOL-001 | `CredentialPool` |
| 선행 SPEC | SPEC-GOOSE-ROUTER-001 | `Route`, `ProviderRegistry` metadata |
| 선행 SPEC | SPEC-GOOSE-RATELIMIT-001 | `Tracker.Parse` (헤더 전달) |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger |
| 후속 SPEC | (후속) Perplexity 어댑터 | Web search 통합 SPEC으로 분리 |
| 후속 SPEC | (후속) Kimi Anthropic-compat | `/anthropic/v1` endpoint 지원 SPEC |
| 후속 SPEC | (후속) Embedding endpoint | 9 provider 중 embed 지원 provider(Together) embedding API |
| **외부** | **없음 (new external)** | 본 SPEC은 신규 Go 외부 의존성 도입 금지(REQ-ADP2-017). 기존 `sashabaranov/go-openai`, `net/http`, `zap`만 사용 |

**라이브러리 결정**: 9 provider 모두 OpenAI-compat이므로 `go-openai` 기반 SPEC-001 어댑터 재사용. 별도 SDK(DashScope, Moonshot 등) 도입 불필요.

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | SPEC-001의 `openai.Options`에 `ExtraHeaders`/`ExtraRequestFields` 필드 없음 | 중 | 고 | SPEC-001 Run phase 완료 후 확인. 부재 시 SPEC-001에 minor 수정 PR로 추가(OpenRouter 헤더 + GLM thinking 모두 이 필드 필요). 또는 본 SPEC에서 per-provider 미들웨어 로직 직접 구현(대안, 복잡도 증가) |
| R2 | Z.ai GLM API가 `api.z.ai`에서 OpenAI-compat schema로 완전 호환되지 않음 | 중 | 중 | Z.ai 공식 docs 재확인(research.md §6.1), 필요 시 `glm/adapter.go`에서 request body 변환 layer 추가. 특히 `thinking` 외에 `reasoning` boolean 필드도 있을 수 있음 |
| R3 | OpenRouter rate limit 변경 / free model 목록 빈번 변경 | 고 | 낮 | suggested_models는 router metadata에만 영향. 실제 routing은 사용자가 지정한 모델 그대로 전송하므로 adapter는 영향 없음. README에 "OpenRouter 모델은 자주 변경됨" 명시 |
| R4 | Qwen DashScope 국제판/중국판 응답 schema 차이 | 낮 | 중 | 공식 문서가 동일 schema 명시(`/compatible-mode/v1` 두 region 모두). 통합 테스트 시 stub으로 양쪽 URL 확인 |
| R5 | Kimi K2.6 출시 직후 schema 변경 가능성 | 중 | 중 | OpenAI-compat endpoint가 안정적으로 유지된다고 Moonshot 공식 docs 명시. stub 테스트로 격리 |
| R6 | GLM thinking mode 파라미터가 z.ai 문서와 실제 API 차이 | 중 | 중 | GREEN phase에서 실 API 소량 호출(개인 API key)로 검증. REQ-ADP2-021에 budget_tokens 선택 옵션 병행 |
| R7 | `RegisterAllProviders`가 너무 많은 의존성(pool, tracker, secretStore, logger)을 받음 | 낮 | 낮 | functional options 패턴으로 리팩터링 가능(후속 REFACTOR) |
| R8 | 9 provider 동시 추가로 인한 테스트 flakiness | 중 | 중 | 각 provider 테스트는 완전 격리(httptest), goleak으로 goroutine 누수 검증, `go test -race -count=5` CI 검증 |
| R9 | `GOOSE_QWEN_REGION` / `GOOSE_KIMI_REGION` 환경변수와 `Options.Region`의 우선순위 혼동 | 낮 | 낮 | 명시적 테스트 케이스: Options.Region이 env보다 우선, 둘 다 없으면 intl 기본 |
| R10 | Together/Fireworks 모델 ID의 슬래시(`meta-llama/...`, `accounts/...`)가 URL path에 섞여 misparsing | 낮 | 중 | OpenAI-compat schema는 모델 ID를 body `model` 필드에만 사용, URL path에 포함 안 함. SPEC-001의 openai 어댑터가 이미 검증 |

---

## 9. 성공 기준 (Success Criteria)

- [ ] 9개 provider 파일(glm/, groq/, openrouter/, together/, fireworks/, cerebras/, mistral/, qwen/, kimi/) 전부 구현 완료
- [ ] AC-ADP2-001 ~ AC-ADP2-018 **18개 전수 GREEN**
- [ ] 각 provider별 단위 테스트 커버리지 **75%+**, 전체 `internal/llm/provider/` 커버리지 **85%+** (SPEC-001 포함)
- [ ] `go test -race -count=5 ./internal/llm/provider/...` clean
- [ ] `golangci-lint run` errcheck/govet/staticcheck 통과
- [ ] `router.DefaultRegistry()` 호출 결과 **15 provider 전부 `AdapterReady=true`**
- [ ] `provider.RegisterAllProviders(...)` 호출로 15 provider 전부 registry에 등록되고 `registry.Get("<name>")`이 모든 이름에서 성공
- [ ] 커밋 히스토리가 provider별로 분리되어 있으며 각 커밋이 `SPEC-GOOSE-ADAPTER-002` 참조
- [ ] README 또는 docs에 15 provider 전체 목록 + 지역 URL 선택 가이드 업데이트

---

## 10. 참고 (References)

### 10.1 프로젝트 문서 (본 SPEC 근거)

- **`.moai/specs/SPEC-GOOSE-ADAPTER-002/research.md`** — 9 provider 상세 조사, 재사용 설계, 경쟁 포지셔닝 (본 SPEC과 쌍)
- `.moai/specs/SPEC-GOOSE-ADAPTER-001/spec.md` — 선행 SPEC, `openai.OpenAIAdapter` 원형 제공
- `.moai/specs/ROADMAP.md` — Phase 1 확장 컨텍스트
- `.moai/project/tech.md` §9 LLM Provider 지원
- `internal/llm/router/registry.go` — 수정 대상 파일

### 10.2 외부 참조

research.md §6에 수록된 **10개 provider의 공식 문서 및 벤치마크 출처** 참조.

핵심 출처 재기재:
- Z.ai GLM: https://docs.z.ai/guides/overview/pricing, https://z.ai/blog/glm-4.6
- Groq: https://console.groq.com/docs/overview, https://console.groq.com/docs/rate-limits
- OpenRouter: https://openrouter.ai/docs/api-reference/headers
- Together: https://docs.together.ai/docs/serverless-models
- Fireworks: https://docs.fireworks.ai/getting-started/introduction
- Cerebras: https://inference-docs.cerebras.ai/
- Mistral: https://docs.mistral.ai/capabilities/completion/
- Qwen: https://help.aliyun.com/zh/model-studio/developer-reference/compatibility-of-openai-with-dashscope
- Kimi: https://platform.moonshot.ai/docs/intro, https://moonshot.ai/blog/kimi-k2-6

---

## 11. Open Items (v1.0.0 시점 미구현 REQ — 후속 이터레이션 배정)

아래 항목은 v1.0.0 감사 결과 **SPEC-vs-Implementation 불일치**로 확인되었으며, `[PENDING v0.3]`로 마킹되어 후속 minor/point release에서 처리한다. 이들은 전부 REQ 분류상 **[Optional]** (Where-절)이며, 현재 구현은 해당 Where-절이 **성립하지 않도록 사용**되는 전제(즉, `PreferredProviders` 미입력, `ThinkingBudget=0`, 장문 context INFO 로그 미요구)에서 정상 동작한다. 따라서 기존 GREEN AC(001-012, 014-018)에는 영향이 없다.

| OI ID | 대응 REQ | 대응 AC | 결함 | 구현 범위 | 우선순위 | 목표 버전 |
|-------|---------|--------|------|---------|---------|---------|
| OI-1 | REQ-ADP2-020 | (AC 없음 — Optional) | D4 (major). `openrouter.Options`에 `PreferredProviders []string` 필드 부재. `"provider": {"order":[...], "allow_fallbacks":true}` request-body 주입 미구현. | `openrouter/client.go:L22-L41` Options 구조체 확장 + `ExtraRequestFields` 주입 + `openrouter/client_test.go` 신규 테스트 1건. | Medium (저수요 feature) | v0.3 |
| OI-2 | REQ-ADP2-021 | (AC 없음 — Optional) | D2 (critical, Optional). `glm/thinking.go:L40-L44` `BuildThinkingField`가 `cfg.BudgetTokens` 값을 읽지 않음. `budget_tokens: N` injection 미구현. `provider.ThinkingConfig.BudgetTokens` 필드(`provider.go:L40`)는 이미 존재. | `glm/thinking.go` `BuildThinkingField` 확장 + `glm/adapter_test.go` `BudgetTokens=4096` 케이스 추가. | High (CG Mode 비용 제어 영향) | v0.3 |
| OI-3 | REQ-ADP2-022 | AC-ADP2-013 | D3 (critical, Optional). `kimi/client.go`에 token-count 추정 헬퍼 및 `moonshot-v1-128k` 대상 INFO 로그 부재. progress.md `N/A — Optional`이 SPEC amendment 없이 단독 이탈. | `kimi/advisory.go` 신규 (token 추정 + INFO 로그) + `kimi/client_test.go` AC-ADP2-013 대응 테스트. 또는 SPEC Exclusions로 영구 이관(대안). | Low (관측용, 기능 영향 없음) | v0.3 또는 Exclusions |

### 처리 원칙

- OI-1/2/3는 모두 `[Optional]` EARS 패턴이므로 v1.0.0 GREEN 판정에 영향 없음. 단 **SPEC-vs-구현 정합성** 차원에서 후속 처리 필요.
- 후속 SPEC(SPEC-GOOSE-ADAPTER-003 또는 이후)에서 OI별 독립 AC로 재도입하고, 해당 REQ의 `[PENDING v0.3]` 주석을 제거한다.
- 처리 완료 시 본 §11의 해당 행을 "CLOSED in SPEC-XXX" 로 마킹하고 유지한다(삭제하지 않음 — traceability).

### 감사 참조

- 근거: `/Users/goos/MoAI/AgentOS/.moai/reports/plan-audit/mass-20260425/ADAPTER-002-audit.md` D2, D3, D4 (Critical/Major)
- D1(RegisterAllProviders 13→15)은 Phase C1 코드 수정으로 **해결됨** — 현 `registry_builder_test.go:L28` `assert.Len(t, names, 15)` + anthropic/google factory 포함. 본 §11 항목 아님.

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **SPEC-001의 6 provider(Anthropic/OpenAI/Google/xAI/DeepSeek/Ollama)를 재구현하지 않는다**. 이들은 SPEC-001에서 완성됨. 본 SPEC이 수정하는 유일한 기존 파일은 `internal/llm/router/registry.go`(GLM metadata 교체 + 신규 등록)뿐이다.
- 본 SPEC은 **Perplexity 어댑터를 구현하지 않는다**. Web search 통합이 필요하여 별도 SPEC.
- 본 SPEC은 **MiniMax 실 어댑터를 구현하지 않는다**. 사용자 선택에 없음. ADAPTER-001의 metadata-only 유지.
- 본 SPEC은 **Nous Research 실 어댑터를 구현하지 않는다**. 사용자 선택에 없음. metadata-only 유지.
- 본 SPEC은 **Cohere 실 어댑터를 구현하지 않는다**. 사용자 선택에 없음. metadata-only 유지.
- 본 SPEC은 **Kimi Anthropic-compat endpoint(`/anthropic/v1`)를 지원하지 않는다**. OpenAI-compat endpoint(`/v1`)만 대상. Anthropic-compat은 후속 SPEC.
- 본 SPEC은 **Embedding 엔드포인트를 구현하지 않는다**. 후속 SPEC (Together/Mistral 등 embed 지원 provider가 embedding API 추가).
- 본 SPEC은 **Audio/Video modality를 지원하지 않는다**. 후속 SPEC.
- 본 SPEC은 **Fine-tuning / Training API를 지원하지 않는다**. 범위 외.
- 본 SPEC은 **OAuth 인증 provider를 추가하지 않는다**. 9 provider 모두 `api_key` 인증. OAuth 확장은 Anthropic/OpenAI Codex 패턴(SPEC-001)을 재사용하는 후속 SPEC에서.
- 본 SPEC은 **live API 통합 테스트를 CI에서 실행하지 않는다**. 모든 단위 테스트는 httptest stub 기반. 실 API 호출 smoke test는 개발자 수동 트리거 또는 후속 SPEC에서 별도 CI secret으로 구성.
- 본 SPEC은 **Usage / pricing / cost 추적을 구현하지 않는다**. 후속 메트릭 SPEC.
- 본 SPEC은 **gRPC provider를 지원하지 않는다**. HTTP only.
- 본 SPEC은 **새로운 외부 Go SDK(예: alibabacloud-go/dashscope-sdk, moonshot-official-sdk)를 도입하지 않는다**. SPEC-001이 이미 사용하는 `sashabaranov/go-openai`만 사용.

---

**End of SPEC-GOOSE-ADAPTER-002**
