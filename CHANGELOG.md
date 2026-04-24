# 변경 이력

모든 주목할 만한 프로젝트 변경사항을 이 문서에 기록한다.
형식은 [Keep a Changelog](https://keepachangelog.com/ko/1.1.0/)를 따르며 SemVer를 준수한다.

## [Unreleased]

### Added — SPEC-GOOSE-ADAPTER-001 (6 LLM Provider 어댑터 + 의존 타입 skeleton)

#### Provider 인터페이스 + Registry

- `internal/llm/provider.Provider` 공통 인터페이스
  - `Complete(ctx, req) (*CompletionResponse, error)` — 단회 API 호출
  - `Stream(ctx, req) (*StreamReader, error)` — SSE 스트리밍
  - `Capabilities() ProviderCapabilities` — 모델별 capability 조회 (vision, function-calling 등)

- `internal/llm/provider.ProviderRegistry` 인스턴스 레지스트리
  - 런타임 provider 라우팅을 위한 in-process 등록소
  - REQ-ADAPTER-001 "모든 provider는 공통 인터페이스" 구현

- `internal/llm/provider.NewLLMCall` → QUERY-001 수신자 팩토리
  - QUERY-001의 LLMCallReq를 수신하여 provider별 Call 객체 생성

- Vision capability pre-check (ErrCapabilityUnsupported)
  - REQ-ADAPTER-017 "vision unsupported인 provider에는 vision 요청 거절"

#### Anthropic 어댑터

- `internal/llm/provider/anthropic/` — claude-3.5-sonnet, claude-opus-4-7 지원
- OAuth PKCE refresh (Claude API credentials → session token)
- Thinking mode 듀얼 경로
  - Claude 4.6 이하: fixed `budget_tokens` (LEGACY)
  - Opus 4.7 Adaptive: `effort: xhigh` (Adaptive Thinking, 동적 할당)
- SSE streaming with `stream: true`
- Tool schema 변환 (MoAI → Anthropic)
- 429 회전 재시도 (REQ-ADAPTER-008, MarkExhaustedAndRotate)
- 60s heartbeat timeout (REQ-ADAPTER-013)
- MaxTokens clamping (각 모델의 최대값 준수)

#### OpenAI 어댑터

- `internal/llm/provider/openai/` — 일반 OpenAI-compatible 팩토리
- GPT-4o, GPT-4 turbo, GPT-3.5-turbo, o1-preview 지원
- base_url 교체 가능 (Azure, Fireworks, Anyscale 등)
- tool_calls aggregation (중복 호출 병합)
- 60s heartbeat timeout
- ExtraHeaders 주입 (provider-specific 헤더, REQ-ADAPTER-016)
- ExtraRequestFields body merge (provider-specific 파라미터 pass-through)

#### xAI Grok 어댑터

- `internal/llm/provider/xai/` — OpenAI 팩토리 래핑
- api.x.ai BaseURL 자동 설정
- Grok-2, Grok-3 vision 지원

#### DeepSeek 어댑터

- `internal/llm/provider/deepseek/` — OpenAI 팩토리 래핑
- api.deepseek.com BaseURL
- DeepSeek-Chat, DeepSeek-Reasoner (r1) 지원
- Vision=false capability 명시 (REQ-ADAPTER-017)

#### Google Gemini 어댑터

- `internal/llm/provider/google/` — `google.golang.org/genai` SDK 기반
- fake client 추상화 (테스트용 mock 지원)
- Gemini 2.0 Flash, Pro 지원
- 60s heartbeat timeout

#### Ollama 어댑터

- `internal/llm/provider/ollama/` — 로컬 모델 지원
- `/api/chat` JSON-L 스트리밍
- llama2, mistral, neural-chat, openchat 등
- 무인증 (localhost 기본값)
- 모델 동적 발견 (LIST /api/tags)

#### Fallback Model Chain

- `TryWithFallback(ctx, providers, models)` helper
- 5xx / network error 시 FallbackModels 순차 재시도 (REQ-ADAPTER-009)
- 모든 fallback 모두 실패 시 최후의 오류 반환

#### SecretStore Interface

- `internal/llm/credential.SecretStore` 공개 인터페이스
- `FileSecretStore` MVP (`~/.goose/credentials/` 디렉토리 기반)
  - env-less 로컬 개발 지원 (REQ-ADAPTER-018)
  - 권한 600 (소유자만 읽기)

#### DefaultRegistry Factory

- `internal/llm/factory/registry_defaults.go`
- import cycle 방지하면서 6 provider 기본 등록
- Anthropic(기본) + OpenAI + xAI + DeepSeek + Google + Ollama

### Added — 의존 타입 skeleton

후속 SPEC(QUERY-001, TOOLS-001, RATELIMIT-001, PROMPT-CACHE-001)이 확장할 공용 타입 최소 구현:

- `internal/message`
  - `Message` 구조체 (role, content)
  - `ContentBlock` (텍스트, 이미지, 도구 호출 등)
  - `StreamEvent` (10종 Type 상수: text_chunk, tool_use_input, final_message 등)

- `internal/tool`
  - `Definition` 구조체 (name, description, input_schema)
  - Tool schema TOON 또는 JSON Schema 지원

- `internal/query`
  - `LLMCallReq` 구조체 (model, messages, system, max_tokens 등)
  - `LLMCallFunc` 함수 시그니처 (factory 패턴용)

- `internal/llm/ratelimit`
  - `Tracker.Parse(statusCode int, retryAfter string) backoffDuration`
  - noop stub (RATELIMIT-001에서 구현)

- `internal/llm/cache`
  - `BreakpointPlanner` 인터페이스
  - empty-plan stub (PROMPT-CACHE-001에서 구현)
  - `CacheStrategy`, `CacheTTL` 상수 정의

### Changed — internal/llm/credential

SPEC-GOOSE-CREDPOOL-001 §3.1 rule 6/7에서 약속된 API:

- `MarkExhaustedAndRotate(ctx context.Context, id string, statusCode int, retryAfter string) error`
  - Atomic credential 회전 (429 status)
  - 호출자는 해당 credential을 버릴 수 있음

- `AcquireLease(id string) *Lease`
  - 명시적 lease 획득
  - Lease.Release() 호출 시 풀로 반환

### Added — 품질 인프라

#### 의존성 추가

- `github.com/stretchr/testify v1.9.0` — 테스트 assertion (assert, require 패키지)
- `go.uber.org/goleak v1.3.0` — goroutine leak 검증 (go test 내 통합)
- `google.golang.org/genai v1.54.0` — Google Gemini SDK

#### 테스트 커버리지

- AC-ADAPTER-001~012 전수 GREEN (12/12 acceptance criteria passed)
- 평균 provider 커버리지 분석
  - cache/ratelimit/xai/deepseek: 100%
  - router: 97.2%
  - credential: 87.5%
  - provider: 81.1%
  - openai: 79.6%
  - anthropic: 77.0%
  - ollama: 77.8%
  - google: 51.7%

#### 품질 검증

- `go test -race` 전 패키지 통과
- `go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...` 0 warnings
- `go vet ./...` 0 errors
- `gofmt -d .` clean (스타일 일관성)

#### 평가자 점수 (evaluator-active 4차원)

- **Functionality**: 0.78 (전체 AC 구현, 경계 케이스 일부 미흡)
- **Security**: 0.80 (credential 보호, API 재사용 가능)
- **Craft**: 0.74 (코드 가독성, 추상화 수준 일관)
- **Consistency**: 0.86 (API contract 일관성, error handling 표준화)
- **종합 점수**: 0.789 (PASS threshold 0.75)

### Added — SPEC-GOOSE-ADAPTER-002 (9 OpenAI-compat Provider 확장)

SPEC-001이 제공한 `openai` 어댑터 팩토리 및 `ExtraHeaders` / `ExtraRequestFields` 확장을 활용해 9종 신규 provider 추가. 단일 Provider 인터페이스 아래 총 **15 provider adapter-ready** 달성.

#### Tier 1 — OpenAI-compat Simple Factory (6종)

- `internal/llm/provider/groq/` — Groq LPU (315 TPS, 무료 tier, Llama 3.3/4 · DeepSeek R1 Distill · Mixtral 8x7B)
- `internal/llm/provider/cerebras/` — Cerebras Wafer-Scale (1,000+ TPS, Llama 3.3 70B)
- `internal/llm/provider/mistral/` — Mistral AI (Nemo $0.02/M 최저가, 42 모델)
- `internal/llm/provider/together/` — Together AI (173 모델)
- `internal/llm/provider/fireworks/` — Fireworks AI (209 모델, 145 TPS)
- `internal/llm/provider/openrouter/` — OpenRouter gateway (300+ 모델, ExtraHeaders 활용한 `HTTP-Referer` / `X-Title` 랭킹 헤더 주입)

#### Tier 2 — Region 선택 (2종)

- `internal/llm/provider/qwen/` — Alibaba DashScope (4 region: `intl` / `cn` / `sg` / `hk`)
  - `Options.Region` → `GOOSE_QWEN_REGION` env → `RegionIntl` 기본값 3단계 우선순위
  - `ErrInvalidRegion`으로 사전 거부
  - `qwen3-max`, `qwen3.6-max-preview`, `qwen3-coder-plus` 등 2026-04 최신
- `internal/llm/provider/kimi/` — Moonshot AI (2 region: `intl` / `cn`)
  - 동일 3단계 우선순위 (`GOOSE_KIMI_REGION`)
  - Kimi K2.6 (1T MoE, 262K context, 98K max output)

#### Tier 3 — GLM (Z.ai) with thinking mode (1종)

- `internal/llm/provider/glm/adapter.go` + `thinking.go`
  - `*openai.OpenAIAdapter` Go embedding + `Stream` / `Complete` override
  - `ExtraRequestFields`에 `thinking:{type:enabled}` 주입 (SPEC-001 필드 활용)
  - caller map 보호를 위한 deep-copy
- 5 모델 alias 지원 — `glm-5` · `glm-4.7` · `glm-4.6` · `glm-4.5` · `glm-4.5-air`
- Thinking 지원 4 모델 (air 제외)
- **비지원 모델 요청 시 WARN log + 무시 (graceful degradation, REQ-ADP2-014)**

#### Registry 업데이트

- `internal/llm/router/registry.go` **`glm` 엔드포인트 이관**: `open.bigmodel.cn` → `api.z.ai/api/paas/v4`
- DisplayName: "GLM (ZhipuAI)" → "Z.ai GLM"
- `glm` suggested_models 갱신: `["glm-5", "glm-4.7", "glm-4.6", "glm-4.5", "glm-4.5-air"]`
- 5 신규 metadata 등록: `groq` / `openrouter` / `together` / `fireworks` / `cerebras`
- 4 기존 metadata-only → AdapterReady=true 전환: `glm` / `mistral` / `qwen` / `kimi`
- `internal/llm/factory/registry_builder.go` **신규** — `RegisterAllProviders` helper (import cycle 방지)
- `internal/llm/factory/registry_defaults.go` — SPEC-002 9개 provider 인스턴스 등록

#### 테스트 커버리지

- AC-ADP2-001~018 중 16 GREEN (-2 Optional/Infrastructure)
- 패키지별 커버리지
  - groq / cerebras / mistral / qwen / kimi: 100%
  - openrouter: 90.9%
  - glm: 83.8%
  - together / fireworks: 75%
  - factory: 77.0%
  - router: 97.2% (회귀 없음)
- `go test -race` 21 패키지 전부 PASS · `go vet` 0 warnings · `gofmt` clean

#### 주요 설계 결정

- **GLM embedding 패턴**: `*openai.OpenAIAdapter`를 embedding하고 `Stream`/`Complete`만 override — thinking 파라미터 주입을 최소 surface로 캡슐화
- **Region 3단계 우선순위**: 명시 옵션 > 환경변수 > 기본값 — 동일 패턴을 Qwen/Kimi에 통일 적용
- **OpenRouter ExtraHeaders 조건부**: 빈 값이면 nil map 유지 — 헤더 오염 방지
- **registry_builder는 factory 패키지에 배치**: import cycle 회피 (SPEC-001 `registry_defaults.go` 패턴 계승)

---

## 관련 SPEC

- **SPEC-GOOSE-CREDPOOL-001** (선행 완료) — credential 풀 관리, 회전 메커니즘
- **SPEC-GOOSE-QUERY-001** (후속) — LLM 쿼리 인터페이스 및 streaming
- **SPEC-GOOSE-TOOLS-001** (후속) — tool/function-calling 정의 및 실행
- **SPEC-GOOSE-RATELIMIT-001** (후속) — rate limiting 추적 및 백오프
- **SPEC-GOOSE-PROMPT-CACHE-001** (후속) — prompt caching & breakpoint planning

---

Version: 1.0.0 (최초 CHANGELOG)
Creation Date: 2026-04-24
Format: Keep a Changelog + SemVer
Language: 한국어
