---
id: SPEC-GOOSE-ADAPTER-001
version: 1.0.0
status: implemented
created_at: 2026-04-21
updated_at: 2026-04-25
author: manager-spec
priority: P0
issue_number: null
phase: 1
size: 대(L)
lifecycle: spec-anchored
labels: ["llm-provider", "phase-1", "adapter", "credpool-extension", "anthropic", "openai", "google", "ollama"]
---

# SPEC-GOOSE-ADAPTER-001 — 6 Provider 어댑터 (Anthropic/OpenAI/Google/xAI/DeepSeek/Ollama)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-llm.md §7-9 + ROADMAP v2.0 Phase 1 기반) | manager-spec |
| 0.2.0 | 2026-04-24 | Phase 2.X evaluator-fix: anthropic 429 lease Release, `pathSafe` 정리, `AnthropicThinkingParam.Type` json 태그, `TestAnthropic_ThinkingMode_EndToEnd` 추가 | manager-tdd |
| 0.3.0 | 2026-04-24 | Phase 2.Y: REQ-ADAPTER-013 heartbeat timeout watchdog 실구현 (`constants.go` + 4 provider), xAI/DeepSeek `New() → (Adapter, error)` 에러 전파 | manager-tdd |
| 0.4.0 | 2026-04-24 | Phase 2.Z: SPEC-GOOSE-ADAPTER-002 선행 gap 해소 — `OpenAIOptions.ExtraHeaders` + `CompletionRequest.ExtraRequestFields` 추가 | manager-tdd |
| 1.0.0 | 2026-04-25 | 구현 완료 반영 · plan-audit defect 수정 (MP-3 labels, D1 REQ→AC gap, D2 frontmatter stale, D3 AC format declaration, D4 의존성 사실화, D5 tiktoken-go 제거) · AC-013~017 신설 · §6.4 pseudocode를 hand-rolled `net/http`로 정정 · status: planned→implemented | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT Phase 1의 **LLM HTTP 호출 경계**를 정의한다. QUERY-001의 `LLMCallFunc` 인터페이스에 대한 6개 provider 구현(Anthropic, OpenAI, Google Gemini, xAI Grok, DeepSeek, Ollama)을 공통 `Provider` interface 아래 통합하는 `internal/llm/provider` 패키지를 구현한다.

본 SPEC이 Plan·Run을 통과한 시점에서:

- 모든 provider는 `Provider` interface(`Complete`, `Stream`, `Tools`, `Vision` capability query)를 구현하고,
- `Anthropic` 어댑터는 **OAuth PKCE 2.1 refresh**, **thinking mode**(Adaptive Thinking), **tool schema 변환**(OpenAI → Anthropic), **PROMPT-CACHE-001의 CachePlan 소비**, **content conversion**(image/code/thinking block)을 hand-rolled `net/http`로 제공하고,
- `OpenAI` 어댑터는 hand-rolled `net/http` 기반으로 Chat Completions + Tools + Streaming을 구현하며 xAI/DeepSeek에 `base_url` override로 재사용되고,
- `Google` 어댑터는 `google.golang.org/genai`(유일한 SDK 기반 어댑터)로 Gemini를 구현하며,
- `Ollama` 어댑터는 hand-rolled `net/http`로 localhost:11434 `/api/chat` JSON-L 기반 로컬 모델을 지원하고,
- 모든 어댑터는 `CREDPOOL-001`의 `Refresher`를 구현(OAuth provider), `RATELIMIT-001`에 응답 헤더 전달, `ROUTER-001`이 결정한 Route를 소비한다.

본 SPEC은 **QUERY-001의 `LLMCall` 시그니처 구현자**이며, QUERY-001 AC-QUERY-001~012의 모든 계약을 만족하는 stub 대체 실 구현을 제공한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- ROADMAP v2.0 Phase 1 row 10은 ADAPTER-001을 P0 마지막으로 배치. QUERY-001·CREDPOOL-001·ROUTER-001·RATELIMIT-001·PROMPT-CACHE-001의 결과를 **하나로 통합하는 경계**.
- `.moai/project/research/hermes-llm.md` §8은 Anthropic Adapter의 58KB Python 코드가 포함한 기능(OAuth PKCE, Token sync, Model normalization, Tool conversion, Content conversion, Thinking mode)을 상세화하며, Go 포팅 매핑(§9)을 제공.
- QUERY-001 `LLMCall` interface 수신측 구현 계약이 본 SPEC에서 확정되어야 Phase 1 MVP Milestone 1(`goose ask "hello"` → Claude/GPT 응답)이 동작.

### 2.2 상속 자산

- **Hermes Agent Python**: `providers/anthropic_adapter.py`(1.45K LoC, 복잡도 최대), `openai_compat.py`, `google_gemini.py`, `ollama_local.py`. 본 SPEC은 **30-50%를 재사용**(hermes-llm.md §11 판정표).
- **HTTP client 전략 (v1.0.0 정정)**: 초기 설계는 각 provider의 공식 SDK(`anthropic-sdk-go`, `sashabaranov/go-openai`, `ollama/ollama/api`) 재사용을 가정했으나, OAuth PKCE 제어·payload 확장성·테스트 가능성을 이유로 **모든 HTTP provider를 Go 표준 라이브러리 `net/http` + `encoding/json`으로 hand-rolling**하기로 결정. 구현 완료 후 `go.mod`에 해당 SDK가 포함되지 않는다. §7.2 참조.
- **Google genai SDK**: `google.golang.org/genai` — **유일한 SDK 기반 어댑터**. streaming/tool/vision 추상화가 hand-roll 대비 이득이 커 유지.

### 2.3 범위 경계

- **IN**: `Provider` interface, 6 provider 어댑터 + registry, QUERY-001의 `LLMCall` 시그니처 구현, CREDPOOL-001의 `Refresher` 구현(OAuth provider), 응답 헤더를 RATELIMIT-001로 전달, PROMPT-CACHE-001의 Plan 소비, streaming 변환(provider → `<-chan message.StreamEvent`), tool schema 변환, Anthropic thinking mode, 이미지 content 변환(vision), fallback model chain 훅.
- **OUT**: Embedding 엔드포인트(후속 SPEC), 비-Phase 1 provider 9종(OpenRouter/Nous/Mistral/Groq/Qwen/Kimi/GLM/MiniMax/Custom — registry metadata만), audio/video(후속), fine-tuning/training(비범위), gRPC 자체 provider(본 SPEC은 HTTP만).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/llm/provider/` 패키지: `Provider` interface, `ProviderFactory`, `Capabilities`.
2. **Anthropic** 어댑터 (`anthropic/`): `adapter.go`, `oauth.go`(PKCE refresh), `tools.go`(schema 변환), `content.go`(image/code/thinking), `thinking.go`, `stream.go`.
3. **OpenAI** 어댑터 (`openai/`): `adapter.go`, `stream.go`, `tools.go` — **hand-rolled `net/http`**. `base_url` 치환으로 `xai`/`deepseek`에 재사용. v0.4.0에서 `ExtraHeaders` + `ExtraRequestFields` 확장점 추가(SPEC-002 선행).
4. **Google** 어댑터 (`google/gemini.go`): Gemini 2.0 Flash 기본.
5. **xAI** 어댑터 (`xai/grok.go`): OpenAI-compat 래퍼.
6. **DeepSeek** 어댑터 (`deepseek/client.go`): OpenAI-compat 래퍼.
7. **Ollama** 어댑터 (`ollama/local.go`): 로컬 LLM.
8. `Provider.Complete(ctx, req)` → blocking 호출 (tool 없는 단순 응답).
9. `Provider.Stream(ctx, req)` → `(<-chan message.StreamEvent, error)` 반환 (QUERY-001 요구).
10. `QUERY-001 LLMCallFunc` 구현: `func(ctx, LLMCallReq) (<-chan message.StreamEvent, error)` — `ProviderRegistry`에서 Route.Provider로 조회 후 `Provider.Stream` 호출.
11. **OAuth Refresher** 구현 (Anthropic/OpenAI Codex): `CREDPOOL-001`의 `Refresher` interface 구현체.
12. **Rate limit 연계**: 응답 수신 직후 `Tracker.Parse(provider, resp.Header, now)` 호출.
13. **Prompt cache 연계**: Anthropic 어댑터가 `PromptCachePlanner`를 주입받아 `messages`에 `cache_control` 적용.
14. **Anthropic 특수 기능**:
    - OAuth PKCE 2.1 + single-use refresh token rotation
    - Token sync: `~/.claude/.credentials.json` 읽기 + 쓰기(refresh 후)
    - Model alias 정규화 (e.g., `claude-3.5-sonnet` → `claude-3-5-sonnet-20241022`)
    - Tool schema 변환 (OpenAI function → Anthropic tool)
    - Content conversion: image(base64), code(text), thinking block
    - Adaptive Thinking mode (o1-style, Opus 4.7에서 `effort`, 외 모델 `budget_tokens`)
15. **Fallback model chain 훅**: QUERY-001 `FallbackModels`에 따라 어댑터가 primary 실패 시 fallback 시도.
16. `Provider.Capabilities()`: `{streaming, tools, vision, embed, adaptive_thinking}` 반환.

### 3.2 OUT OF SCOPE

- **Embedding 엔드포인트**: 후속 SPEC (Memory/Vector Phase 6).
- **Audio/Video modality**: 후속 SPEC.
- **Fine-tuning/Training API**: 범위 외.
- **9개 metadata-only provider 실 구현** (OpenRouter/Nous/Mistral/Groq/Qwen/Kimi/GLM/MiniMax/Custom): 후속 SPEC.
- **Claude Code 브라우저 PKCE 로그인 UI**: CLI-001.
- **Credential storage 자체**: CREDPOOL-001.
- **Rate limit 헤더 파싱**: RATELIMIT-001 (본 SPEC은 헤더 전달만).
- **CachePlan 생성**: PROMPT-CACHE-001 (본 SPEC은 소비).
- **Usage pricing/cost 계산**: 메트릭 SPEC.
- **Tool 실행**: TOOLS-001 (본 SPEC은 tool_use 전달만).

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-ADAPTER-001 [Ubiquitous]** — Every `Provider` implementation **shall** expose the `Provider` interface: `Complete(ctx, CompletionRequest) (*CompletionResponse, error)`, `Stream(ctx, CompletionRequest) (<-chan message.StreamEvent, error)`, `Capabilities() Capabilities`, `Name() string`.

**REQ-ADAPTER-002 [Ubiquitous]** — The `ProviderRegistry` **shall** map `Route.Provider` strings to `Provider` instances; lookup **shall** return `ErrProviderNotFound` if not registered.

**REQ-ADAPTER-003 [Ubiquitous]** — Every adapter **shall** propagate `ctx.Done()` to the underlying HTTP client; cancelling the context **shall** close the stream channel within 500ms (QUERY-001 REQ-QUERY-010 compliance).

**REQ-ADAPTER-004 [Ubiquitous]** — Every adapter **shall** call `RateLimitTracker.Parse(provider, response.Header, now)` immediately after receiving HTTP response headers (success or error), before consuming the body.

**REQ-ADAPTER-005 [Ubiquitous]** — Every adapter **shall** use `CredentialPool.Select(ctx, strategy)` to obtain credentials before each API call; on HTTP 429/402, adapter **shall** call `CredentialPool.MarkExhaustedAndRotate(id, statusCode, retryAfter)` and retry once with the rotated credential.

### 4.2 Event-Driven

**REQ-ADAPTER-006 [Event-Driven]** — **When** `LLMCall(ctx, req)` is invoked (QUERY-001 entry), the adapter **shall** (a) resolve `Provider` via `req.Route.Provider`, (b) acquire credential via `CredentialPool`, (c) if provider == "anthropic", consume `PromptCachePlanner.Plan(messages, strategy, ttl)` and apply markers, (d) convert `message.Message[]` to provider-specific request schema, (e) call `Provider.Stream(ctx, req)`, (f) return the stream channel.

**REQ-ADAPTER-007 [Event-Driven]** — **When** an OAuth credential's `expires_at - refreshMargin < now` triggers `CredentialPool.Refresh`, the provider-specific `Refresher` implementation **shall** POST to the provider's token endpoint with `refresh_token`, receive new `access_token` and (rotated) `refresh_token`, and return `RefreshResult`.

**REQ-ADAPTER-008 [Event-Driven]** — **When** an adapter receives an HTTP 5xx or network error after the primary model call, and `req.FallbackModels` is non-empty, the adapter **shall** transparently retry against the next model in the chain without altering the public stream channel; on all fallback exhaustion, emit an `error` `StreamEvent` and close the channel.

**REQ-ADAPTER-009 [Event-Driven]** — **When** the Anthropic adapter receives a streaming chunk with `content_block_delta.thinking`, it **shall** emit a `StreamEvent{Type: "thinking_delta", Delta: "..."}` to the channel; these events are observational only and **shall not** be included in the final assistant message text.

### 4.3 State-Driven

**REQ-ADAPTER-010 [State-Driven]** — **While** the Anthropic adapter is handling a model whose `Capabilities.AdaptiveThinking == true` (e.g., claude-opus-4-7) and `req.Thinking.Effort` is set, the adapter **shall** set the API parameter `thinking: {type: "enabled", effort: "high|xhigh|max"}` instead of `budget_tokens`; for non-adaptive models, `budget_tokens` path is used.

**REQ-ADAPTER-011 [State-Driven]** — **While** a request contains `tool_use_id` in any user message's tool_result block, the adapter **shall** include the tool_use_id in the provider-specific schema (Anthropic uses native tool_result, OpenAI compat uses `role: "tool"` with `tool_call_id`).

**REQ-ADAPTER-012 [State-Driven]** — **While** the OpenAI-compat adapter (including xAI/DeepSeek) is configured with a non-standard `base_url`, it **shall** use that URL for all requests; path suffix (`/chat/completions`) is appended by the adapter.

### 4.4 Unwanted Behavior

**REQ-ADAPTER-013 [Unwanted]** — **If** an adapter's HTTP response exceeds 30 seconds without any data (for non-streaming call) or without heartbeat for 60 seconds (streaming), **then** the adapter **shall** abort the connection and emit `error` `StreamEvent`; **shall not** hold the stream open indefinitely.

**REQ-ADAPTER-014 [Unwanted]** — The adapter **shall not** log request body content containing messages (PII risk); only `{provider, model, message_count, tokens_estimated}` structured fields are logged. Response body excerpts are logged only at DEBUG level with size cap 500 bytes.

**REQ-ADAPTER-015 [Unwanted]** — **If** `CachePlan` returned by `PromptCachePlanner.Plan` has empty markers (REQ-PC-006/010), the adapter **shall not** include any `cache_control` field in the request; **shall not** invent markers.

**REQ-ADAPTER-016 [Unwanted]** — The adapter **shall not** perform any disk write outside of `~/.goose/credentials/` (CREDPOOL) and `~/.claude/.credentials.json` (Anthropic token sync, write-back only after successful refresh).

### 4.5 Optional

**REQ-ADAPTER-017 [Optional]** — **Where** `CompletionRequest.Vision` is non-nil and the resolved provider's `Capabilities.Vision == false`, the adapter **shall** return `ErrCapabilityUnsupported{feature:"vision", provider:...}` before making the HTTP call.

**REQ-ADAPTER-018 [Optional]** — **Where** the Anthropic adapter encounters a `model` parameter matching a known alias (e.g., `claude-3-5-sonnet`), it **shall** normalize to the latest concrete version (`claude-3-5-sonnet-20241022`) before sending; the alias map is maintained in `anthropic/models.go`.

**REQ-ADAPTER-019 [Optional]** — **Where** `CompletionRequest.ResponseFormat` is `"json"` and the provider supports structured output, the adapter **shall** apply the provider-specific JSON mode parameter (OpenAI `response_format: {type: "json_object"}`, Gemini `response_mime_type: "application/json"`).

**REQ-ADAPTER-020 [Optional]** — **Where** `CompletionRequest.Metadata.UserID` is set, the adapter **shall** forward it as `user` (OpenAI), `metadata.user_id` (Anthropic) for abuse tracking per provider.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC 포맷 선언 (Format Declaration)** — 본 SPEC은 다음 이원 구조를 채택한다:

- §4 **REQ**는 EARS 패턴(Ubiquitous / Event-Driven / State-Driven / Unwanted / Optional)으로 시스템 요구사항을 기술한다.
- §5 **AC**는 Given/When/Then(BDD/Gherkin) 시나리오로 작성되어, 각 AC가 **실행 가능한 httptest 기반 Go 테스트 1개 이상**과 1:1로 매핑된다. AC 자체는 EARS 문장이 아니라 EARS REQ의 **검증 명세(verification specification)**다.
- REQ↔AC 매핑 표는 `tasks.md` §"Acceptance Criteria → Task Mapping"에 유지한다. 본 SPEC §5의 각 AC 머리말은 주 REQ 번호를 괄호로 표기한다.
- 어떤 REQ가 간접적으로만 검증되는 경우(예: `@MX:WARN` 또는 정적 분석으로 enforcement), 해당 AC 본문에 "indirect verification" 유형을 명시한다.

**AC-ADAPTER-001 — Anthropic 기본 streaming** *(주 REQ: REQ-ADAPTER-001, REQ-ADAPTER-004, REQ-ADAPTER-006)*
- **Given** ProviderRegistry에 Anthropic 어댑터 등록, 유효한 OAuth 자격 풀, CachePlanner 주입, Route `{model:"claude-opus-4-7", provider:"anthropic"}`, messages 2개 (system + user "hello")
- **When** `LLMCall(ctx, req)` 호출 후 `<-chan StreamEvent` drain
- **Then** `stream_request_start` → 1+개 `text_delta` chunks → 1개 `message_stop` 순서로 수신, 채널 close. `RateLimitTracker`에 `anthropic` 상태 갱신됨

**AC-ADAPTER-002 — Anthropic tool call round-trip** *(주 REQ: REQ-ADAPTER-011)*
- **Given** messages에 tool_result 포함, CachePlan는 system_and_3 적용
- **When** `LLMCall` 호출 후 응답이 `tool_use{name:"echo"}` 반환
- **Then** StreamEvent 시퀀스에 `content_block_start{type:"tool_use"}` + `input_json_delta` chunks 포함, QUERY-001의 queryLoop가 tool_use 감지 가능

**AC-ADAPTER-003 — Anthropic OAuth 자동 refresh** *(주 REQ: REQ-ADAPTER-007, REQ-ADAPTER-016)*
- **Given** 풀 엔트리 `expires_at == now + 2분`, refreshMargin = 5분. Anthropic 토큰 엔드포인트 stub이 새 `access_token`과 rotated `refresh_token`을 반환
- **When** `LLMCall` 호출
- **Then** `CredentialPool.Refresher.Refresh`가 1회 호출, 풀 엔트리의 `access_token` 갱신, HTTP 호출은 새 토큰으로 수행, `~/.claude/.credentials.json`에 rotated refresh_token 기록

**AC-ADAPTER-004 — OpenAI-compat streaming** *(주 REQ: REQ-ADAPTER-001, REQ-ADAPTER-004)*
- **Given** OpenAI 어댑터, API key 자격, 모델 `gpt-4o`, messages 2개
- **When** `LLMCall`
- **Then** OpenAI `/v1/chat/completions` SSE로 streaming 수신, `StreamEvent{Type:"text_delta"}` 방출, 응답 헤더가 RATELIMIT에 전달

**AC-ADAPTER-005 — xAI Grok (OpenAI-compat 재사용)** *(주 REQ: REQ-ADAPTER-012)*
- **Given** xAI 어댑터가 `openai` 어댑터를 base_url override로 감싸고 있음. 모델 `grok-2`
- **When** `LLMCall`
- **Then** HTTPS 호출이 `https://api.x.ai/v1/chat/completions`로 이루어지고, 성공 streaming 수신

**AC-ADAPTER-006 — Google Gemini** *(주 REQ: REQ-ADAPTER-001, REQ-ADAPTER-003)*
- **Given** Google 어댑터, API key, 모델 `gemini-2.0-flash`
- **When** `LLMCall`
- **Then** `google.golang.org/genai` SDK 경유 streaming 수신, `StreamEvent{Type:"text_delta"}` 방출

**AC-ADAPTER-007 — Ollama localhost** *(주 REQ: REQ-ADAPTER-001, REQ-ADAPTER-003)*
- **Given** Ollama 서버 `localhost:11434` 동작 중, 모델 `llama3.2`
- **When** `LLMCall`
- **Then** `POST /api/chat` streaming JSON-L 수신, StreamEvent 변환 성공. Credential 미필요

**AC-ADAPTER-008 — 429 후 회전 재시도** *(주 REQ: REQ-ADAPTER-005)*
- **Given** 풀에 엔트리 2개(a, b), a 선택 후 HTTP 429(`Retry-After: 120`) 수신
- **When** `LLMCall`
- **Then** (1) `MarkExhaustedAndRotate("a", 429, 120s)` 호출됨, (2) b 자격으로 재시도, (3) `MarkExhaustedAndRotate`가 반환한 `next` credential은 rotation 경로 종료 시 반드시 `pool.Release(next)`로 lease 반환(v1.0 `anthropic/adapter.go`의 Phase 2.X fix 기준), (4) 재시도 성공 시 stream 정상, 실패 시 `error` StreamEvent 후 close

**AC-ADAPTER-009 — Fallback model chain** *(주 REQ: REQ-ADAPTER-008)*
- **Given** primary `claude-opus`가 HTTP 529 반환, FallbackModels=["claude-sonnet"], `NewLLMCall`이 fallback-aware production path로 wiring됨(`llm_call.go`가 `TryWithFallback`을 호출하거나 동등한 chaining을 수행)
- **When** `LLMCall`
- **Then** 투명하게 sonnet 재시도, 공개 StreamEvent에는 단일 응답처럼 보임(fallback 로그로만 식별). **Production wiring 검증**: `llm_call_test.go` 또는 `fallback_test.go`에서 단순 unit 호출이 아닌 `NewLLMCall()` 경유 full stack test로 `req.FallbackModels`가 실제 소비됨을 증명

**AC-ADAPTER-010 — Context cancellation** *(주 REQ: REQ-ADAPTER-003)*
- **Given** streaming 중 `ctx`가 `WithTimeout(500ms)`로 취소
- **When** 500ms 경과
- **Then** HTTP 요청 abort, 채널 500ms 이내 close, 마지막 StreamEvent는 `error{message:"context cancelled"}`

**AC-ADAPTER-011 — Capability 체크 (vision unsupported)** *(주 REQ: REQ-ADAPTER-017)*
- **Given** Route `{provider:"deepseek", model:"deepseek-chat"}`, DeepSeek `Capabilities.Vision == false`, CompletionRequest에 이미지 포함
- **When** `LLMCall`
- **Then** `ErrCapabilityUnsupported{feature:"vision", provider:"deepseek"}` 반환, HTTP 호출 발생하지 않음

**AC-ADAPTER-012 — Anthropic thinking mode (Adaptive Thinking)** *(주 REQ: REQ-ADAPTER-009, REQ-ADAPTER-010)*
- **Given** 모델 `claude-opus-4-7` (AdaptiveThinking == true), CompletionRequest.Thinking = `{Enabled:true, Effort:"high"}`
- **When** `LLMCall`
- **Then** Anthropic API 요청 payload에 `thinking: {type:"enabled", effort:"high"}`가 포함되고(json 태그 `"type"` 직렬화됨, Phase 2.X `AnthropicThinkingParam.Type` 태그 수정 반영), `budget_tokens`는 부재. Streaming에서 `thinking_delta` StreamEvent 수신 가능. **End-to-end 검증**: `TestAnthropic_ThinkingMode_EndToEnd`가 (1) payload `thinking` 필드 존재, (2) SSE `content_block_delta.thinking_delta` 수신 → `message.TypeThinkingDelta` StreamEvent 방출을 함께 검증

**AC-ADAPTER-013 — Heartbeat timeout (stream watchdog)** *(주 REQ: REQ-ADAPTER-013)*
- **Given** httptest SSE 서버가 연결 수립 후 **아무 이벤트도 송신하지 않고 blocking** 상태로 유지됨. 어댑터는 `HeartbeatTimeout=200ms`로 생성됨 (production 기본값 60s는 테스트 시간 단축을 위해 override)
- **When** `LLMCall` 호출 후 스트림 drain
- **Then** (1) 채널이 **2초 이내에 close**됨 (heartbeat watchdog이 ctx/body cancel), (2) 마지막 `StreamEvent.Type == "error"`로 heartbeat 초과 메시지 포함, (3) goroutine leak 없음(`goleak.VerifyTestMain`). 검증 테스트: `TestAnthropic_HeartbeatTimeout_EmitsError`, `TestOpenAI_HeartbeatTimeout_EmitsError`, `TestOllama_HeartbeatTimeout_EmitsError`, `TestGoogle_HeartbeatTimeout_EmitsError`. 기본값 상수는 `internal/llm/provider/constants.go`에 정의(`DefaultStreamHeartbeatTimeout=60s`, `DefaultNonStreamDataTimeout=30s`)

**AC-ADAPTER-014 — PII log 금지 (indirect verification)** *(주 REQ: REQ-ADAPTER-014)*
- **Given** 어댑터가 임의의 request/response에서 zap logger에 기록한 모든 필드
- **When** log 필드 집합을 정적 분석(`grep` 또는 test hook)으로 수집
- **Then** 로그 필드 allowlist는 `{provider, model, message_count, tokens_estimated, route_signature, credential_id_redacted, trace_id, error_class}`에 한정. **Body/message/content/tool_arguments/response_body는 INFO 이상 레벨에 절대 기록되지 않는다**. DEBUG 레벨 response_body 발췌는 최대 500바이트 cap. 검증 방식: indirect — 코드 리뷰 + test-level log capture (`zaptest.NewLogger` + observed logs 검사). 리뷰 책임자: evaluator-active Security 축

**AC-ADAPTER-015 — Disk write 제한 (indirect verification)** *(주 REQ: REQ-ADAPTER-016)*
- **Given** 어댑터 실행 환경(`~/.goose/credentials/`, `~/.claude/.credentials.json` 경로는 임시 디렉터리로 리다이렉트)
- **When** 전체 adapter flow(Stream/Complete/Refresh 포함) 수행
- **Then** 파일 시스템 쓰기는 **정확히 두 경로로만** 발생: (1) `{FileSecretStore.BaseDir}/{keyringID}.json`, (2) `$HOME/.claude/.credentials.json` (Anthropic OAuth write-back 후). 로그 파일, 캐시 파일, temp 디렉터리(atomic write의 중간 파일 제외) 쓰기는 0건. 검증: `testhelper`의 `RecordingFS` 또는 `os.Chdir` + `HOME` override로 격리된 임시 경로 스캔. path traversal 방어: `FileSecretStore.CredentialFile()`이 `..` 포함 keyringID를 거부함(SPEC-GOOSE-ADAPTER-001 Phase 2.X `pathSafe` 정리로 `oauth.go`도 동일 경로 검증 경유)

**AC-ADAPTER-016 — JSON mode (REQ-019, Deferred)** *(주 REQ: REQ-ADAPTER-019 [Optional])*
- **Status**: **DEFERRED** to SPEC-GOOSE-ADAPTER-003. `CompletionRequest.ResponseFormat` 필드는 `provider.go`에 선언되어 있으나, 본 SPEC v1.0 시점 6개 어댑터 중 어느 것도 해당 필드를 소비하지 않는다. REQ-019는 Optional 분류로 MVP 스코프 외 처리.
- **Given** SPEC-GOOSE-ADAPTER-003에서 JSON mode 지원이 계획될 때
- **When** 해당 SPEC Plan phase 시작
- **Then** (1) OpenAI 계열은 `response_format: {"type":"json_object"}` 파라미터 주입, (2) Gemini는 `response_mime_type: "application/json"` 파라미터 주입, (3) 미지원 provider는 `ErrCapabilityUnsupported{feature:"json_mode"}` 반환하는 AC를 SPEC-003에 작성. 본 AC는 본 SPEC 합격 기준에 포함되지 않는 **지연 명세**로 기록됨

**AC-ADAPTER-017 — UserID forwarding (REQ-020, Deferred)** *(주 REQ: REQ-ADAPTER-020 [Optional])*
- **Status**: **DEFERRED** to SPEC-GOOSE-ADAPTER-003. `CompletionRequest.Metadata.UserID` 필드는 `provider.go`에 선언되어 있으나, 본 SPEC v1.0 시점 어느 어댑터도 해당 필드를 API 요청에 forwarding하지 않는다. REQ-020은 Optional 분류로 MVP 스코프 외 처리.
- **Given** SPEC-GOOSE-ADAPTER-003에서 user ID forwarding이 계획될 때
- **When** 해당 SPEC Plan phase 시작
- **Then** (1) OpenAI 계열은 `user` top-level 필드, (2) Anthropic은 `metadata.user_id` 필드로 forwarding하는 AC를 SPEC-003에 작성. 본 AC는 지연 명세

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/llm/provider/
├── provider.go                 # Provider interface + CompletionRequest/Response
├── registry.go                 # ProviderRegistry + NewLLMCall helper
├── capabilities.go             # Capabilities struct
├── errors.go                   # ErrProviderNotFound + ErrCapabilityUnsupported
├── llm_call.go                 # QUERY-001의 LLMCallFunc 구현 (registry → provider.Stream)
│
├── anthropic/
│   ├── adapter.go              # AnthropicAdapter struct + Complete/Stream
│   ├── oauth.go                # PKCE refresh (CREDPOOL Refresher 구현)
│   ├── token_sync.go           # ~/.claude/.credentials.json read/write
│   ├── models.go               # alias normalization map
│   ├── tools.go                # OpenAI-style → Anthropic tool schema
│   ├── content.go              # message/content block 변환
│   ├── thinking.go             # Adaptive Thinking vs budget_tokens
│   ├── stream.go               # SSE event → StreamEvent
│   └── cache_apply.go          # PROMPT-CACHE-001의 Plan 적용
│
├── openai/
│   ├── adapter.go              # go-openai wrapper
│   ├── stream.go
│   └── tools.go
│
├── google/
│   ├── gemini.go
│   └── stream.go
│
├── xai/
│   └── grok.go                 # openai adapter + base_url override
│
├── deepseek/
│   └── client.go               # openai adapter + base_url override
│
└── ollama/
    ├── local.go
    └── stream.go
```

### 6.2 핵심 타입

```go
// internal/llm/provider/provider.go

type Capabilities struct {
    Streaming         bool
    Tools             bool
    Vision            bool
    Embed             bool
    AdaptiveThinking  bool // Opus 4.7 style
    MaxContextTokens  int
    MaxOutputTokens   int
}

type CompletionRequest struct {
    Route           router.Route         // Router가 결정
    Messages        []message.Message
    Tools           []tool.Definition    // TOOLS-001 주입 (본 SPEC은 변환만)
    MaxOutputTokens int
    Temperature     float64
    Thinking        *ThinkingConfig      // optional
    Vision          *VisionConfig        // optional (이미지 블록 검증용)
    ResponseFormat  string               // "" | "json"
    FallbackModels  []string
    Metadata        RequestMetadata      // UserID 등
}

type ThinkingConfig struct {
    Enabled      bool
    Effort       string // "low" | "medium" | "high" | "xhigh" | "max"
    BudgetTokens int    // non-adaptive 모델용
}

type CompletionResponse struct {
    Message       message.Message
    StopReason    string // "end_turn" | "tool_use" | "max_tokens" | "max_output_tokens"
    Usage         UsageStats
    ResponseID    string
    RawHeaders    http.Header // RATELIMIT 전달용
}

type UsageStats struct {
    InputTokens        int
    OutputTokens       int
    CacheReadTokens    int // Anthropic prompt cache
    CacheCreateTokens  int
}

type Provider interface {
    Name() string
    Capabilities() Capabilities

    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan message.StreamEvent, error)
}
```

```go
// internal/llm/provider/registry.go

type ProviderRegistry struct {
    providers map[string]Provider
}

func NewRegistry() *ProviderRegistry
func (r *ProviderRegistry) Register(p Provider) error
func (r *ProviderRegistry) Get(name string) (Provider, bool)

// NewLLMCall은 QUERY-001의 LLMCallFunc 서명을 구현한 함수를 반환.
// QUERY-001은 LLMCallFunc를 QueryEngineConfig.LLMCall로 주입 받음.
func NewLLMCall(
    registry *ProviderRegistry,
    pool *credential.CredentialPool,
    tracker *ratelimit.Tracker,
    cachePlanner *cache.BreakpointPlanner,
    cacheStrategy cache.CacheStrategy,
    cacheTTL cache.TTL,
    logger *zap.Logger,
) query.LLMCallFunc
```

```go
// internal/llm/provider/anthropic/adapter.go
//
// 주의: 본 어댑터는 SDK를 사용하지 않고 표준 라이브러리(`net/http` + `encoding/json`)
// 만으로 Anthropic Messages API를 직접 호출한다. OAuth PKCE refresh, SSE 파싱,
// tool schema 변환, thinking mode payload 구성, prompt cache marker 적용은 모두
// 어댑터 내부 코드로 구현되며 외부 SDK 의존성이 없다.

type AnthropicAdapter struct {
    pool              *credential.CredentialPool
    tracker           *ratelimit.Tracker
    cachePlanner      *cache.BreakpointPlanner
    cacheStrategy     cache.CacheStrategy
    cacheTTL          cache.TTL
    httpClient        *http.Client        // hand-rolled client (SDK 없음)
    secretStore       provider.SecretStore
    heartbeatTimeout  time.Duration       // REQ-ADAPTER-013 watchdog
    logger            *zap.Logger
}

func New(opts AnthropicOptions) (*AnthropicAdapter, error)

// CREDPOOL Refresher interface 구현 (OAuth PKCE refresh)
func (a *AnthropicAdapter) Refresh(
    ctx context.Context,
    entry *credential.PooledCredential,
) (credential.RefreshResult, error)

func (a *AnthropicAdapter) Stream(
    ctx context.Context,
    req provider.CompletionRequest,
) (<-chan message.StreamEvent, error)
```

### 6.3 QUERY-001 인터페이스 수신측 구현

```go
// QUERY-001 spec.md §6.2에서:
// type LLMCallFunc func(ctx context.Context, req LLMCallReq) (<-chan message.StreamEvent, error)

func NewLLMCall(
    registry *ProviderRegistry,
    pool *credential.CredentialPool,
    tracker *ratelimit.Tracker,
    cachePlanner *cache.BreakpointPlanner,
    cacheStrategy cache.CacheStrategy,
    cacheTTL cache.TTL,
    logger *zap.Logger,
) query.LLMCallFunc {
    return func(ctx context.Context, req query.LLMCallReq) (<-chan message.StreamEvent, error) {
        // 1. Route 기반 provider 조회
        p, ok := registry.Get(req.Route.Provider)
        if !ok {
            return nil, ErrProviderNotFound{Name: req.Route.Provider}
        }

        // 2. CompletionRequest 구성
        compReq := provider.CompletionRequest{
            Route:           req.Route,
            Messages:        req.Messages,
            Tools:           req.Tools,
            MaxOutputTokens: req.MaxOutputTokens,
            Temperature:     req.Temperature,
            Thinking:        req.Thinking,
            FallbackModels:  req.FallbackModels,
        }

        // 3. Provider.Stream 호출 (어댑터 내부에서 credential/cache/ratelimit 통합)
        return p.Stream(ctx, compReq)
    }
}
```

### 6.4 Anthropic 어댑터 Stream 흐름 (hand-rolled `net/http`)

> **주의**: 아래 의사코드는 SDK를 사용하지 않는 hand-rolled HTTP 구현을 반영한다. 요청 payload는 `encoding/json`으로 직접 마샬링하고, SSE 응답은 `bufio.Scanner`로 라인 단위 파싱하여 `anthropic/stream.go`의 이벤트 디스패처에 전달한다.

```
Stream(ctx, req):
  // 1. credential 획득
  cred, err = pool.Select(ctx)
  if err: return nil, err
  lease = pool.AcquireLease(cred.ID)
  defer lease.Release()

  apiKey, err = secretStore.Resolve(ctx, cred.KeyringID)
  if err: return nil, err

  // 2. prompt cache plan 소비 (PROMPT-CACHE-001)
  plan, _ = cachePlanner.Plan(req.Messages, cacheStrategy, cacheTTL)

  // 3. messages → Anthropic native schema 변환
  anthropicMsgs = convertMessages(req.Messages)
  applyCacheMarkers(anthropicMsgs, plan.Markers)   // plan 비어있으면 no-op

  // 4. tool schema 변환 (OpenAI function → Anthropic tool)
  anthropicTools = convertTools(req.Tools)

  // 5. thinking payload 구성 (Adaptive vs budget_tokens 분기)
  thinkingParam = buildThinkingParam(req.Thinking, req.Route.Model)

  // 6. request body 조립 (JSON, SDK 없음)
  //    json.Marshal로 직접 직렬화. Thinking 구조체는 `json:"type"` 태그 필수
  //    (Phase 2.X 수정 사항). ExtraRequestFields는 top-level merge 지원.
  bodyMap = {
    "model":       normalizeModel(req.Route.Model),
    "messages":    stripSystem(anthropicMsgs),
    "system":      extractSystem(anthropicMsgs),
    "tools":       anthropicTools,         // nil 이면 생략
    "max_tokens":  req.MaxOutputTokens,
    "thinking":    thinkingParam,          // disabled 면 생략
    "stream":      true,
  }
  for k, v in req.ExtraRequestFields:     // Phase 2.Z — SPEC-002 호환
    bodyMap[k] = v                         // 사용자 override 허용
  body, _ = json.Marshal(bodyMap)

  // 7. HTTP request 구성
  httpReq, _ = http.NewRequestWithContext(
    ctx, "POST",
    "https://api.anthropic.com/v1/messages",
    bytes.NewReader(body),
  )
  httpReq.Header.Set("Content-Type", "application/json")
  httpReq.Header.Set("Anthropic-Version", "2023-06-01")
  httpReq.Header.Set("Accept", "text/event-stream")
  setAuthHeader(httpReq, cred, apiKey)    // OAuth Bearer 또는 x-api-key

  // 8. 호출 + rate limit 헤더 전달
  resp, err = a.httpClient.Do(httpReq)
  if err: return nil, err
  tracker.Parse("anthropic", resp.Header, time.Now())  // REQ-ADAPTER-004

  // 9. 429/402 → credential rotation (REQ-ADAPTER-005)
  if resp.StatusCode == 429 or resp.StatusCode == 402:
    retryAfter = parseRetryAfter(resp.Header)
    resp.Body.Close()
    next, _ = pool.MarkExhaustedAndRotate(ctx, cred.ID, resp.StatusCode, retryAfter)
    if next != nil:
      defer pool.Release(next)           // Phase 2.X fix — lease 반환 필수
      return a.retryWith(ctx, req, next) // 1회 재시도
    return nil, ErrExhausted

  if resp.StatusCode >= 400:
    return nil, parseErrorBody(resp)     // 구조화 에러 매핑

  // 10. SSE → StreamEvent 변환 goroutine (heartbeat watchdog 포함)
  out = make(chan message.StreamEvent, 8)
  go a.convertStream(ctx, resp.Body, out, heartbeatTimeout)
    // convertStream은 bufio.Scanner로 라인 단위 SSE 파싱
    // "event: " / "data: " 라인을 누적하여 완성된 event를 디스패치
    // heartbeat timer: 매 데이터 수신 시 reslide, timeout 초과 시 ctx cancel + body.Close
    // defer close(out) + defer resp.Body.Close() 필수 (goleak 검증)
  return out, nil
```

**보조 함수 요약:**

- `convertMessages([]message.Message) []AnthropicMessage`: role 매핑(user/assistant/tool → Anthropic 형식), ContentBlock → native block 변환
- `convertTools([]tool.Definition) []AnthropicTool`: OpenAI function schema → Anthropic tool schema, tool_choice 매핑
- `buildThinkingParam(*ThinkingConfig, model string) map[string]any`: `{"type":"enabled","effort":cfg.Effort}` 또는 `{"type":"enabled","budget_tokens":cfg.BudgetTokens}`. 비활성 시 nil 반환하여 payload에서 생략
- `setAuthHeader(req, cred, apiKey)`: OAuth credential → `Authorization: Bearer <token>`, API key credential → `x-api-key: <key>`
- `convertStream(ctx, body, out, heartbeat)`: SSE 이벤트 파싱, `content_block_delta.thinking_delta`/`text_delta`/`input_json_delta` 분기, heartbeat watchdog은 `time.Timer.Reset` 패턴

### 6.5 Stream event 변환 (Anthropic)

| Anthropic SSE event | message.StreamEvent |
|---|---|
| `message_start` | `StreamEvent{Type:"message_start", Role:"assistant"}` |
| `content_block_start{type:"text"}` | `StreamEvent{Type:"content_block_start", BlockType:"text"}` |
| `content_block_delta{delta.type:"text_delta"}` | `StreamEvent{Type:"text_delta", Delta:"..."}` |
| `content_block_delta{delta.type:"thinking_delta"}` | `StreamEvent{Type:"thinking_delta", Delta:"..."}` |
| `content_block_start{type:"tool_use"}` | `StreamEvent{Type:"content_block_start", BlockType:"tool_use", ToolUseID:"..."}` |
| `content_block_delta{delta.type:"input_json_delta"}` | `StreamEvent{Type:"input_json_delta", Delta:"..."}` |
| `content_block_stop` | `StreamEvent{Type:"content_block_stop"}` |
| `message_delta{delta.stop_reason:"..."}` | `StreamEvent{Type:"message_delta", StopReason:"..."}` |
| `message_stop` | `StreamEvent{Type:"message_stop"}` |
| error | `StreamEvent{Type:"error", Error:"..."}` |

### 6.6 OpenAI-compat 재사용 (xAI/DeepSeek)

> **주의**: OpenAI 계열 어댑터도 SDK 없이 `net/http`로 직접 구현한다. `base_url` override만 다르고 payload/SSE 포맷은 OpenAI 표준 그대로. `ExtraHeaders` 및 `ExtraRequestFields`는 Phase 2.Z SPEC-GOOSE-ADAPTER-002 선행 gap 해소로 추가된 provider-specific 확장점이다. v0.3.0(Phase 2.Y)부터 `New()`는 에러를 반환하며 xAI/DeepSeek wrapper도 동일 시그니처를 전파한다.

```go
// openai/adapter.go (발췌)
type OpenAIOptions struct {
    Name              string
    BaseURL           string
    Pool              *credential.CredentialPool
    SecretStore       provider.SecretStore
    Tracker           *ratelimit.Tracker
    HeartbeatTimeout  time.Duration          // 0이면 DefaultStreamHeartbeatTimeout(60s)
    ExtraHeaders      map[string]string      // Phase 2.Z — provider-specific HTTP 헤더
    Capabilities      provider.Capabilities  // Vision 등 per-provider override
    Logger            *zap.Logger
}

func New(opts OpenAIOptions) (*OpenAIAdapter, error)

// xai/grok.go
func New(pool *credential.CredentialPool, secretStore provider.SecretStore,
         tracker *ratelimit.Tracker, logger *zap.Logger,
) (*openai.OpenAIAdapter, error) {
    return openai.New(openai.OpenAIOptions{
        Name:        "xai",
        BaseURL:     "https://api.x.ai/v1",
        Pool:        pool,
        SecretStore: secretStore,
        Tracker:     tracker,
        Logger:      logger,
    })
}

// deepseek/client.go: 동일 패턴, BaseURL="https://api.deepseek.com/v1" + Capabilities{Vision: false}
```

### 6.7 OAuth Refresher 구현 (Anthropic, hand-rolled)

> **주의**: Refresh 경로도 SDK 없이 `net/http` + `encoding/json`으로 직접 구현한다. Token endpoint URL, body 형식, rotated refresh_token 대응은 Anthropic 공식 문서 기준. 파일 write-back은 `anthropic/token_sync.go`의 atomic write (temp file + rename + `chmod 0600`) 경로를 거친다.

```go
func (a *AnthropicAdapter) Refresh(
    ctx context.Context,
    entry *credential.PooledCredential,
) (credential.RefreshResult, error) {
    // 1. refresh_token 획득 (SecretStore 경유)
    raw, err := a.secretStore.Resolve(ctx, entry.KeyringID)
    if err != nil {
        return credential.RefreshResult{}, err
    }
    refreshToken := parseRefreshTokenFromRaw(raw)

    // 2. POST https://console.anthropic.com/v1/oauth/token (hand-rolled HTTP)
    body, _ := json.Marshal(map[string]any{
        "grant_type":    "refresh_token",
        "refresh_token": refreshToken,
        "client_id":     entry.OAuth.ClientID,
    })
    httpReq, _ := http.NewRequestWithContext(ctx, "POST",
        "https://console.anthropic.com/v1/oauth/token",
        bytes.NewReader(body),
    )
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := a.httpClient.Do(httpReq)
    if err != nil {
        return credential.RefreshResult{}, err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return credential.RefreshResult{}, parseOAuthError(resp)
    }

    // 3. 응답 파싱
    var tokenResp struct {
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"` // rotated (single-use)
        ExpiresIn    int    `json:"expires_in"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return credential.RefreshResult{}, err
    }

    // 4. rotated refresh_token + access_token write-back
    //    - SecretStore.WriteBack(ctx, keyringID, newRaw)
    //    - AtomicWriteCredentialsFile(~/.claude/.credentials.json) — Phase 2.X에서 pathSafe
    //      검증을 FileSecretStore.CredentialFile() 경유로 통합. `..` 포함 keyringID 거부.
    if err := a.storeRotatedRefreshToken(ctx, entry.KeyringID, tokenResp); err != nil {
        return credential.RefreshResult{}, err
    }

    return credential.RefreshResult{
        AccessToken:  tokenResp.AccessToken,
        RefreshToken: tokenResp.RefreshToken,
        ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
    }, nil
}
```

### 6.8 Adaptive Thinking 결정

```go
// anthropic/thinking.go

var adaptiveThinkingModels = map[string]bool{
    "claude-opus-4-7":  true,
    "claude-opus-4-7-20260320": true,
    // ...
}

func buildThinkingParam(cfg *provider.ThinkingConfig, model string) anthropic.ThinkingParam {
    if cfg == nil || !cfg.Enabled {
        return anthropic.ThinkingParam{Type: "disabled"}
    }
    if adaptiveThinkingModels[model] {
        // Opus 4.7: effort level
        return anthropic.ThinkingParam{Type: "enabled", Effort: cfg.Effort}
    }
    // 이전 모델: budget_tokens
    return anthropic.ThinkingParam{Type: "enabled", BudgetTokens: cfg.BudgetTokens}
}
```

### 6.9 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1**: `TestRegistry_LookupByName_ReturnsProvider` — basic registry.
2. **RED #2**: `TestNewLLMCall_WithStubProvider_DrainsStream` — QUERY-001 integration 기본.
3. **RED #3**: `TestAnthropic_Stream_HappyPath` — AC-ADAPTER-001 (httptest stub).
4. **RED #4**: `TestAnthropic_ToolCall_RoundTrip` — AC-ADAPTER-002.
5. **RED #5**: `TestAnthropic_OAuthRefresh_Success` — AC-ADAPTER-003.
6. **RED #6**: `TestAnthropic_ThinkingMode_AdaptiveVsBudget` — AC-ADAPTER-012.
7. **RED #7**: `TestAnthropic_CacheMarkersApplied` — prompt cache integration.
8. **RED #8**: `TestOpenAI_Stream_HappyPath` — AC-ADAPTER-004.
9. **RED #9**: `TestXAI_UsesCustomBaseURL` — AC-ADAPTER-005.
10. **RED #10**: `TestGoogle_GeminiStream_HappyPath` — AC-ADAPTER-006.
11. **RED #11**: `TestOllama_Stream_HappyPath` — AC-ADAPTER-007.
12. **RED #12**: `TestAdapter_429Rotation` — AC-ADAPTER-008.
13. **RED #13**: `TestAdapter_FallbackModelChain` — AC-ADAPTER-009.
14. **RED #14**: `TestAdapter_ContextCancellation` — AC-ADAPTER-010.
15. **RED #15**: `TestAdapter_CapabilityUnsupported` — AC-ADAPTER-011.
16. **GREEN**: 각 어댑터 최소 구현 (Anthropic 먼저, OpenAI 둘째, 나머지 순차).
17. **REFACTOR**: OpenAI 어댑터를 xAI/DeepSeek이 공유하도록 base_url 옵션 분리, stream conversion goroutine 누수 방지 테스트 추가.

### 6.10 TRUST 5 매핑

| 차원 | 달성 방법 |
|-----|---------|
| Tested | 85%+ 커버리지, httptest stub 기반 격리 (실 API 호출 없음), race detector 통과, provider별 fixture |
| Readable | provider별 패키지 분리, 공통 `Provider` interface로 통일 |
| Unified | go fmt + golangci-lint (errcheck, govet, staticcheck), stream channel close 책임은 goroutine 소유 |
| Secured | credential redaction, request body 미로깅(REQ-ADAPTER-014), Anthropic PKCE + secure temp file for ~/.claude sync |
| Trackable | 모든 요청에 `{provider, model, route_signature, credential_id_redacted}` 로그, trace_id 전파 |

---

## 7. 의존성 (Dependencies)

> **중요 (v1.0.0 정정)**: 본 SPEC 초안(v0.1.0)은 `anthropic-sdk-go`, `sashabaranov/go-openai`, `ollama/ollama/api`, `tiktoken-go`를 의존 라이브러리로 선언했으나, **실 구현은 SDK를 사용하지 않고 표준 라이브러리 `net/http` + `encoding/json`으로 직접 HTTP 호출을 수행한다**. `go.mod`에는 해당 SDK가 포함되어 있지 않으며, 아래 표는 실제 빌드 그래프를 반영한다. Google Gemini만이 유일한 SDK-기반 어댑터다(`google.golang.org/genai`).

### 7.1 선행/후속 SPEC 의존성

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | `LLMCallFunc` 시그니처 수신측 구현, `message.Message` / `StreamEvent` 타입 |
| 선행 SPEC | SPEC-GOOSE-CREDPOOL-001 | `CredentialPool`, `Refresher` interface |
| 선행 SPEC | SPEC-GOOSE-ROUTER-001 | `Route` 소비, `ProviderRegistry` 메타 참조 |
| 선행 SPEC | SPEC-GOOSE-RATELIMIT-001 | 응답 헤더를 `Tracker.Parse`로 전달 |
| 선행 SPEC | SPEC-GOOSE-PROMPT-CACHE-001 | Anthropic 어댑터가 `Plan` 소비 |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger, context 루트 |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `LLMConfig.Providers[*]` |
| 후속 SPEC | SPEC-GOOSE-ADAPTER-002 | 9 OpenAI-compat Provider + Z.ai GLM (본 SPEC Phase 2.Z `ExtraHeaders`/`ExtraRequestFields` 소비) |
| 후속 SPEC | SPEC-GOOSE-ADAPTER-003 | REQ-019 JSON mode, REQ-020 UserID forwarding 등 Optional REQ 실구현 |
| 후속 SPEC | SPEC-GOOSE-TOOLS-001 | `tool.Definition` 소비, tool_use StreamEvent 전달 |
| 후속 SPEC | SPEC-GOOSE-ERROR-CLASS-001 | HTTP 에러 분류(14 FailoverReason) 연계 |

### 7.2 외부 라이브러리 (실제 `go.mod` 반영)

| 타입 | 대상 | 용도 |
|-----|------|------|
| 외부 | Go 1.22+ | generics, `context.Context` |
| 외부 (stdlib) | `net/http` | **모든 HTTP provider** (Anthropic/OpenAI/xAI/DeepSeek/Ollama) hand-rolled 클라이언트. 서드파티 SDK 없음 |
| 외부 (stdlib) | `encoding/json` | 요청 body 직렬화 + SSE/JSON-L 응답 파싱 |
| 외부 (stdlib) | `bufio` | SSE 라인 단위 스트림 파싱 |
| 외부 | `go.uber.org/zap` v1.27+ | 구조화 로깅 (PII 필드 제외, REQ-ADAPTER-014) |
| 외부 | `google.golang.org/genai` | **유일한 SDK 기반 어댑터** — Google Gemini 공식 Go SDK (streaming/tool/vision 추상화가 hand-roll보다 우위) |
| 외부 (test) | `github.com/stretchr/testify` v1.9+ | 테스트 assertion |
| 외부 (test) | `go.uber.org/goleak` v1.3+ | goroutine leak 검증 (REQ-ADAPTER-003) |

**라이브러리 결정 근거 (v1.0.0 정정)**:

- **Anthropic**: 공식 `anthropic-sdk-go`는 OAuth PKCE refresh 경로를 충분히 제어할 수 없어 **채택 불가**. `net/http`로 직접 구현하여 Bearer 헤더/PKCE flow/rotated refresh token을 완전 제어.
- **OpenAI 계열(OpenAI/xAI/DeepSeek)**: `sashabaranov/go-openai`는 추상화 비용 대비 이득이 적고, `base_url` override + SSE 파싱 + `tool_calls` aggregation을 직접 구현하는 편이 단순. SPEC-ADAPTER-002에서 `ExtraHeaders`/`ExtraRequestFields`로 확장 가능성을 확보하므로 hand-roll이 향후 provider 추가에도 유리.
- **Ollama**: `github.com/ollama/ollama/api`는 `/api/chat` JSON-L streaming만 필요하고 전체 SDK 도입은 과함. `net/http`로 직접 호출.
- **Google Gemini**: 공식 `google.golang.org/genai`는 streaming/tool/vision 추상화가 hand-roll 대비 이득이 크므로 **유지**. 단 테스트 용이성을 위해 내부 `geminiClient` interface로 래핑하여 fake client 주입 가능.
- **tiktoken-go**: **제거**. 본 SPEC v1.0 시점 `internal/llm/provider/` 하위에서 `tiktoken-go`를 import하는 코드가 없다(Usage 카운팅은 provider 응답 헤더/body에서 직접 추출). 잘못된 선언이었으므로 삭제.

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Anthropic SDK가 OAuth PKCE refresh를 지원하지 않아 직접 구현 필요 | 고 | 고 | 본 SPEC은 `oauth.go`를 자체 구현. SDK는 HTTP client 레이어만 사용 가능 |
| R2 | Streaming 변환 goroutine이 ctx 취소 시 누수 | 고 | 고 | 모든 고루틴은 `select { case <-ctx.Done(): return; case ...}` 패턴. `go test -race` + `goleak` 검증 |
| R3 | Anthropic tool schema와 OpenAI function schema의 차이 | 중 | 중 | `tools.go`에 변환 테이블 + 단위 테스트 10+개. tool_choice, required 등 corner case 명시 |
| R4 | Prompt cache marker가 multi-content-block에서 마지막 block 참조 실패 | 중 | 중 | PROMPT-CACHE-001 AC-PC-007 선행 검증. `cache_apply.go`가 `ContentBlockIndex` 엄격 준수 |
| R5 | Fallback chain이 ROUTER-001의 결정과 충돌 | 중 | 중 | FallbackModels는 provider-level alias override만. Route.Provider 변경 없음 |
| R6 | xAI/DeepSeek 응답 헤더가 OpenAI-compat에서 차이 | 중 | 중 | RATELIMIT-001의 OpenAIParser가 자동 처리(동일 prefix). 차이 확인 시 per-provider parser 분리 |
| R7 | Ollama가 tool을 구조적으로 지원하지 않는 버전 | 중 | 중 | `Capabilities.Tools`를 version-aware로 구성. 미지원 버전은 tool 없이 fallback |
| R8 | Google genai SDK가 streaming 변경 잦음 | 중 | 중 | SDK 버전 pin + 주기적 integration test |
| R9 | 30초/60초 타임아웃이 장기 thinking에 부족 | 중 | 중 | REQ-ADAPTER-013 기준. Adaptive Thinking 활성 시 타임아웃을 120s로 상향 (모델별 override) |
| R10 | ~/.claude/.credentials.json write 중 crash | 낮 | 중 | atomic write (temp file + rename), 실패 시 기존 파일 유지 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/hermes-llm.md` §7 Context Compressor(본 SPEC 영향 없음), §8 Anthropic Adapter 58KB 기능 상세, §9 Go 포팅 매핑, §10 SPEC 도출, §11 재사용 판정, §12 고리스크
- `.moai/specs/ROADMAP.md` §4 Phase 1 row 10, §13 핵심 설계 원칙 2·3
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` §6.2 `LLMCall` interface
- `.moai/specs/SPEC-GOOSE-CREDPOOL-001/spec.md` §6.2 `Refresher` interface
- `.moai/specs/SPEC-GOOSE-ROUTER-001/spec.md` §6.2 `Route`
- `.moai/specs/SPEC-GOOSE-RATELIMIT-001/spec.md` §6.2 `Tracker.Parse`
- `.moai/specs/SPEC-GOOSE-PROMPT-CACHE-001/spec.md` §6.2 `Plan`
- `.moai/project/tech.md` §3.2 Go LLM 스택, §9 LLM Provider 지원

### 9.2 외부 참조

- **Anthropic API 문서**: https://docs.anthropic.com/en/api
- **Anthropic Prompt Caching**: https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching
- **Anthropic Thinking**: https://docs.anthropic.com/en/docs/about-claude/extended-thinking
- **Anthropic OAuth**: https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/oauth
- **OpenAI API**: https://platform.openai.com/docs/api-reference
- **OpenAI streaming SSE**: https://platform.openai.com/docs/api-reference/chat/streaming
- **Google genai (Go SDK)**: https://pkg.go.dev/google.golang.org/genai — **본 SPEC에서 채택된 유일한 SDK**
- **Ollama API (/api/chat JSON-L)**: https://github.com/ollama/ollama/blob/main/docs/api.md
- **Hermes Agent Python**: `./hermes-agent-main/agent/providers/` — 원형 참고

> **참고**: 초안 v0.1.0에 포함되었던 `github.com/anthropics/anthropic-sdk-go`, `github.com/sashabaranov/go-openai`, `github.com/ollama/ollama/api`, `github.com/pkoukk/tiktoken-go` 레퍼런스는 v1.0.0에서 제거됨. 실 구현은 hand-rolled `net/http`이므로 해당 SDK 문서는 참고 가치가 없음. §7.2 참조.

### 9.3 부속 문서

- `./research.md` — Anthropic 58KB python의 기능별 분해, tool schema 변환 표, OAuth PKCE 상세, Ollama/Gemini 차이 분석, 테스트 전략

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **Embedding 엔드포인트를 구현하지 않는다**. 후속 SPEC.
- 본 SPEC은 **Audio/Video modality를 지원하지 않는다**. 후속 SPEC.
- 본 SPEC은 **Fine-tuning/Training API를 지원하지 않는다**. 범위 외.
- 본 SPEC은 **9개 metadata-only provider(OpenRouter/Nous/Mistral/Groq/Qwen/Kimi/GLM/MiniMax/Custom)의 실 어댑터를 구현하지 않는다**. ROUTER-001의 registry 메타만 유지. 후속 SPEC에서 단계 추가.
- 본 SPEC은 **Claude Code 브라우저 PKCE 로그인 UI를 포함하지 않는다**. CLI-001이 브라우저 플로우 담당. 본 SPEC은 refresh만.
- 본 SPEC은 **Credential 저장소 자체를 구현하지 않는다**. CREDPOOL-001.
- 본 SPEC은 **Rate limit 헤더 파싱을 구현하지 않는다**. RATELIMIT-001에 헤더 전달만.
- 본 SPEC은 **CachePlan 생성을 구현하지 않는다**. PROMPT-CACHE-001의 결과 소비만.
- 본 SPEC은 **Tool 실행 엔진을 구현하지 않는다**. TOOLS-001. 어댑터는 tool_use StreamEvent 전달만.
- 본 SPEC은 **Usage pricing/cost 계산을 포함하지 않는다**. 후속 메트릭 SPEC.
- 본 SPEC은 **Context compression을 수행하지 않는다**. CONTEXT-001.
- 본 SPEC은 **gRPC provider를 지원하지 않는다**. HTTP provider 전용.

---

**End of SPEC-GOOSE-ADAPTER-001**
