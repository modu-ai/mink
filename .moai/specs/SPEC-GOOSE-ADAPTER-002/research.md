# SPEC-GOOSE-ADAPTER-002 — Research

**조사 일자**: 2026-04-24
**조사자**: manager-spec
**대상 SPEC**: SPEC-GOOSE-ADAPTER-002 (9 OpenAI-compat provider 확장)
**선행 SPEC**: SPEC-GOOSE-ADAPTER-001 (6 provider 기반 어댑터)

---

## 1. 조사 목적

### 1.1 배경

AI.GOOSE(`module github.com/modu-ai/goose`)의 LLM provider 생태계를 **6개에서 15개로 확장**하기 위한 조사이다. SPEC-GOOSE-ADAPTER-001이 OpenAI-compatible 기반의 `internal/llm/provider/openai/` 어댑터를 완성하는 시점(main merge 후), 동일한 어댑터를 **BaseURL override + Capabilities 주입** 패턴으로 9개 provider에 재사용하여 구현 비용을 극단적으로 낮춘다.

### 1.2 사용자 의사 결정의 문맥

사용자 요청의 핵심은 **비용 최적화 + Z.ai GLM 정식 지원**이다:

- **저렴/무료 경로 확보**: Groq(30 RPM 무료), OpenRouter(29개 free model), Cerebras(무료 tier), DeepSeek(이미 ADAPTER-001 포함)를 통해 **CG Mode(Claude + GLM cost optimization)** 기획과 호환되는 저비용 implementation 경로를 만든다.
- **Z.ai GLM 공식 endpoint 이전**: 기존 router metadata에 등록된 `open.bigmodel.cn`은 구 ZhipuAI 시절 URL이며, 2026년 시점 공식 endpoint는 `https://api.z.ai/api/paas/v4`. 본 SPEC은 이 불일치를 해소한다.
- **모델 다양성**: Together AI(173 models), Fireworks(209 models)를 포함해 DeepSeek R1, Qwen3 Coder, Llama 4, GLM-4.6를 router에서 선택 가능하게 한다.

### 1.3 2026-04 시점 시장 상황

- **Z.ai GLM-5 출시 임박**, GLM-4.6 정식 출시 (357B MoE, 200K context, thinking mode)
- **Kimi K2.6** 출시 (1T MoE, 262K context, 98K max output, Anthropic 호환도 지원)
- **Qwen3.6 Max Preview** 출시 (2026-04-20, 1T MoE)
- **Groq LPU**가 315 TPS로 inference 속도 1위 유지
- **Cerebras Wafer-Scale Engine**이 1,000+ TPS로 최고 속도 달성 (단 모델 소수)
- **OpenRouter**의 29 free model로 개발 단계 비용 0원 가능

### 1.4 전제: SPEC-001 의존

본 SPEC의 구현은 **SPEC-GOOSE-ADAPTER-001의 `internal/llm/provider/openai/` 패키지가 main에 merge된 후**에 착수 가능하다. SPEC-001은 `openai.OpenAIAdapter` 구조체를 BaseURL/Capabilities override가 가능한 Options 팩토리 패턴으로 제공하며, xAI/DeepSeek이 이를 재사용하는 참조 구현을 포함한다. 본 SPEC은 **그 패턴을 9 provider에 확장**하는 것이다.

---

## 2. 각 Provider 상세 분석

### 2.1 Tier 1 — OpenAI-compat 직접 팩토리 래핑 (5 provider)

Tier 1은 `func New(...) (*openai.OpenAIAdapter, error)` 팩토리 파일(각 ~35 LOC) 하나로 구현 가능한 provider이다. 단, GLM은 thinking mode로 인해 예외적으로 커스텀 타입 래핑이 필요하다.

#### 2.1.1 Z.ai GLM

- **회사**: Zhipu AI (Z.ai로 2026년 리브랜딩 진행 중)
- **Base URL**: `https://api.z.ai/api/paas/v4` (공식)
  - 기존 router metadata: `https://open.bigmodel.cn/api/paas/v4` (구 ZhipuAI, **교체 필요**)
- **Auth**: `Authorization: Bearer <api_key>` 헤더 방식
- **Rate limit / Pricing**: GLM-4.5-Air 무료 tier 존재, GLM-4.6 유료 (가격 DeepSeek 대비 경쟁적)
- **모델 카탈로그 (suggested)**:
  - `glm-5` — 2026 초 flagship (TBC, 출시 시점 확정 후 반영)
  - `glm-4.7` — Alibaba Cloud Coding Plan 계약 포함 (출시 확인 필요)
  - `glm-4.6` — 주력 (357B MoE, 200K context, 30% token efficiency vs GLM-4.5)
  - `glm-4.5` — 전세대
  - `glm-4.5-air` — 무료 tier (경량)
- **특이 기능**:
  - **Thinking mode**: `thinking: {type: "enabled"}` 파라미터 지원 (Anthropic과 유사하나 형식 상이). `reasoning` boolean도 병행.
  - **Vision**: GLM-4.6+ (이미지 이해)
  - **Tool use**: 지원
  - **Streaming**: SSE (OpenAI-compat)
  - **Long context**: 200K tokens
- **재사용 난이도**: **Medium**
  - 근거: OpenAI-compat endpoint이나 `thinking` 파라미터 전송 로직이 커스텀. Anthropic의 `thinking.go` 패턴 일부 재사용 가능하지만 schema가 다름. 별도 `thinking.go` 필요.

#### 2.1.2 Groq

- **회사**: Groq Inc. (LPU 전용 inference 하드웨어)
- **Base URL**: `https://api.groq.com/openai/v1`
- **Auth**: `Authorization: Bearer <api_key>` (무료 tier, 신용카드 불필요)
- **Rate limit (무료 tier)**:
  - 30 RPM (requests per minute)
  - 6,000 TPM (tokens per minute)
  - 14,400 RPD (requests per day)
  - Llama 4 Maverick: 500 RPD로 감소됨
- **Pricing**: 무료 tier 외에도 development/production tier 존재. 속도 대비 가격 경쟁력.
- **모델 카탈로그 (suggested)**:
  - `llama-3.3-70b-versatile` — 범용
  - `llama-4-scout` — Meta 최신 (17B 활성)
  - `llama-4-maverick` — Meta 최신 고성능 (128 Expert MoE)
  - `deepseek-r1-distill-llama-70b` — DeepSeek R1 distill
  - `mistral-saba-24b`
  - `qwen-qwq-32b` — Qwen reasoning
  - `mixtral-8x7b-32768`
  - `gemma2-9b-it`
- **특이 기능**:
  - **속도**: 315 TPS (Tokens Per Second) — LPU 기반 inference 업계 1위
  - **Tool use**: 지원
  - **Streaming**: SSE
  - **Vision**: 일부 모델 (Llama 4)
- **재사용 난이도**: **Trivial**
  - 근거: 순수 OpenAI-compat. BaseURL만 주입하면 동작.

#### 2.1.3 OpenRouter

- **회사**: OpenRouter Inc. (multi-provider gateway)
- **Base URL**: `https://openrouter.ai/api/v1`
- **Auth**: `Authorization: Bearer <api_key>`
- **Rate limit**:
  - Free tier: 20 RPM, 200 RPD
  - Paid: 기준 provider rate + 소량 markup
- **Pricing**: Passthrough rate (원 provider 가격) + OpenRouter markup (일반적으로 5-10%)
- **모델 카탈로그 (suggested, 300+ 중 선별)**:
  - `openrouter/auto` — 자동 모델 선택 (task-aware routing)
  - `deepseek/deepseek-r1:free` — R1 무료
  - `meta-llama/llama-3.3-70b-instruct:free` — Llama 무료
  - `nvidia/llama-3.1-nemotron-70b-instruct:free` — Nvidia fine-tune
  - `google/gemini-2.5-flash` — 최신 Gemini
  - `openai/gpt-oss-120b:free` — OpenAI open-weights
  - `qwen/qwen3-coder-480b-a35b-instruct:free` — 코딩 특화
  - `z-ai/glm-4.6` — Z.ai GLM (OpenRouter 경유)
  - `minimax/minimax-m2.5`
- **특이 기능**:
  - **Gateway 기능**: 300+ 모델을 단일 API로 접근
  - **Free tier**: 29개 모델 무료 (rate 제한 있음)
  - **Preference routing**: `provider: {order: [...], allow_fallbacks: true}` 파라미터
  - **Ranking 헤더**: `HTTP-Referer`, `X-Title` (optional, leaderboard 노출용)
- **재사용 난이도**: **Easy**
  - 근거: OpenAI-compat + optional 헤더 추가. `internal/llm/provider/openrouter/client.go`에 헤더 주입 로직만 +10 LOC.

#### 2.1.4 Together AI

- **회사**: Together Computer (Red Pajama, Together Inference)
- **Base URL**: `https://api.together.xyz/v1`
- **Auth**: `Authorization: Bearer <api_key>` ($1 무료 credit)
- **Pricing**:
  - 55/101 공유 모델 기준 Fireworks 대비 저렴
  - Llama 3.3 70B: $0.88/M tokens
  - DeepSeek R1: $7/M (입력) / $7/M (출력)
- **모델 카탈로그 (suggested, 173 개 중)**:
  - `meta-llama/Llama-3.3-70B-Instruct-Turbo`
  - `meta-llama/Llama-4-Scout-17B-16E-Instruct`
  - `deepseek-ai/DeepSeek-R1`
  - `Qwen/Qwen2.5-72B-Instruct-Turbo`
  - `mistralai/Mixtral-8x22B-Instruct-v0.1`
  - `zai-org/GLM-4.6`
- **특이 기능**:
  - **Fine-tuning 지원** (본 SPEC 범위 외)
  - **173 models**
  - **Together Turbo**: 가속 엔진 (동일 모델 20% 빠름)
  - **Streaming**: SSE
  - **Tool use**: 지원 (대부분 모델)
- **재사용 난이도**: **Trivial**
  - 근거: 순수 OpenAI-compat.

#### 2.1.5 Fireworks AI

- **회사**: Fireworks AI (ex-Meta AI researchers)
- **Base URL**: `https://api.fireworks.ai/inference/v1`
- **Auth**: `Authorization: Bearer <api_key>` ($1 무료 credit)
- **Pricing**:
  - 50% cached batch 할인
  - Llama 3.3 70B: $0.9/M tokens (캐시 없이)
  - DeepSeek R1: $8/M
- **모델 카탈로그 (suggested, 209 개 중)**:
  - `accounts/fireworks/models/llama-v3p3-70b-instruct`
  - `accounts/fireworks/models/deepseek-r1`
  - `accounts/fireworks/models/qwen3-coder-480b`
  - `accounts/fireworks/models/mixtral-8x22b-instruct`
- **특이 기능**:
  - **145 TPS** (중상위권 속도)
  - **Batch 할인**: 50% (비동기 배치)
  - **Cached 할인**: 50% (반복 프롬프트)
  - **Function calling**: 지원
- **재사용 난이도**: **Trivial**
  - 근거: 순수 OpenAI-compat. 모델 ID가 `accounts/fireworks/models/...` 형식이지만 이는 router metadata의 suggested_models에만 영향.

### 2.2 Tier 2 — 이미 router metadata 등록, 실 구현만 필요 (4 provider)

Tier 2는 SPEC-ADAPTER-001 단계에서 metadata-only로 이미 등록된 provider로, 본 SPEC에서 `AdapterReady=true` 전환 + 실 코드만 추가한다.

#### 2.2.1 Cerebras

- **회사**: Cerebras Systems (WSE-3 Wafer-Scale Engine)
- **Base URL**: `https://api.cerebras.ai/v1`
- **Auth**: `Authorization: Bearer <api_key>` (무료 tier 존재, rate 제한)
- **Rate limit (무료)**: 30 RPM, 60K TPM (가량)
- **모델 카탈로그 (suggested, 소수)**:
  - `llama-3.3-70b`
  - `llama-3.1-8b`
  - `llama-4-scout` (TBC)
- **특이 기능**:
  - **속도**: 1,000+ TPS (Wafer-Scale Engine, Llama 3.1 8B 기준 2,100+ TPS 보고)
  - **Tool use**: 지원
  - **Streaming**: SSE
  - **모델 소수**: 대량 모델 부재 (속도 특화)
- **재사용 난이도**: **Trivial**
  - 근거: 순수 OpenAI-compat. BaseURL override만 필요.

#### 2.2.2 Mistral AI

- **회사**: Mistral AI (프랑스)
- **Base URL**: `https://api.mistral.ai/v1`
- **Auth**: `Authorization: Bearer <api_key>`
- **Pricing**:
  - Mistral Nemo: **$0.02/M input, $0.04/M output** (최저가 tier)
  - Mistral Small: $0.2/M input
  - Mistral Large: $2/M input
  - Codestral: $0.2/M input
- **모델 카탈로그 (suggested, 42 모델 중)**:
  - `mistral-nemo` — 최저가, 12B
  - `mistral-small-latest` — 경량 범용
  - `mistral-medium-3` — 중급
  - `mistral-large-latest` — flagship
  - `codestral-latest` — 코딩 특화 (256K context, 80+ 언어)
- **특이 기능**:
  - **Tool use**: 지원
  - **Function calling**: 지원
  - **Streaming**: SSE
  - **JSON mode**: 공식 지원
  - **OpenAI-compat 공식 문서**: Mistral이 자체 OpenAI-compat endpoint를 명시
- **재사용 난이도**: **Trivial**
  - 근거: Mistral 공식 문서가 OpenAI Python client 사용 예시 제공. 완전 호환.

#### 2.2.3 Qwen (Alibaba DashScope)

- **회사**: Alibaba Cloud
- **Base URL**:
  - 국제판: `https://dashscope-intl.aliyuncs.com/compatible-mode/v1`
  - 중국판: `https://dashscope.aliyuncs.com/compatible-mode/v1`
- **Auth**: `Authorization: Bearer <DASHSCOPE_API_KEY>` 환경변수 관례
- **Pricing**: Qwen Plus는 OpenAI GPT-4o 대비 30-50% 저렴
- **모델 카탈로그 (suggested)**:
  - `qwen3-max` — 현 flagship
  - `qwen3.6-max-preview` — **2026-04-20 출시, 1T MoE**
  - `qwen-plus` — 범용 tier
  - `qwen3.5-flash` — 저비용 tier
  - `qwen3-coder-plus` — 코딩 특화 (480B A35B)
  - `qwen3-vl` — 비전 특화
- **특이 기능**:
  - **OpenAI-compat 공식 지원**: 경로 `/compatible-mode/v1`
  - **Vision**: qwen3-vl
  - **Tool use**: 지원
  - **JSON mode**: 지원
  - **Long context**: 일부 모델 1M tokens
  - **지역 선택**: intl vs cn URL
- **재사용 난이도**: **Easy**
  - 근거: OpenAI-compat이며 BaseURL만 교체. 지역 선택 로직(환경변수 `QWEN_REGION` 또는 config)만 +5 LOC.

#### 2.2.4 Kimi (Moonshot AI)

- **회사**: Moonshot AI (中国, 2023 창립)
- **Base URL**:
  - 국제판: `https://api.moonshot.ai/v1`
  - 중국판: `https://api.moonshot.cn/v1`
- **Auth**: `Authorization: Bearer <api_key>`
- **Pricing**: Kimi K2.6 기준 Claude Sonnet 대비 60% 저렴
- **모델 카탈로그 (suggested)**:
  - `kimi-k2.6` — **2026-04-20 출시, 1T MoE, 262K context, 98K max output**
  - `kimi-k2.5`
  - `moonshot-v1-128k`
  - `moonshot-v1-32k`
  - `moonshot-v1-8k`
- **특이 기능**:
  - **OpenAI-compat**: 기본
  - **Anthropic-compat**: Moonshot이 Anthropic API 호환 endpoint 별도 제공 (`/anthropic/v1`) — **본 SPEC 범위 외**
  - **Long context**: 262K tokens (K2.6)
  - **Max output**: 98K tokens (K2.6, 매우 긴 응답)
  - **Tool use**: 지원
  - **Streaming**: SSE
- **재사용 난이도**: **Easy**
  - 근거: OpenAI-compat endpoint만 본 SPEC에서 지원. 지역 선택 로직만 +5 LOC. Anthropic-compat은 후속 SPEC.

---

## 3. 경쟁 구도 요약

본 SPEC이 추가하는 9 provider의 포지셔닝:

| 축 | 1위 | 2위 | 3위 | Note |
|---|-----|-----|-----|------|
| **Inference 속도** | Cerebras (1000+ TPS) | Groq (315 TPS) | Fireworks (145 TPS) | 실시간 CLI UX에 결정적 |
| **모델 다양성** | OpenRouter (300+) | Fireworks (209) | Together (173) | 실험/비교 시 필수 |
| **최저 가격 (input)** | Mistral Nemo ($0.02/M) | Groq (무료 tier) | OpenRouter (29 free) | 개발 단계 비용 0 가능 |
| **Long context** | Kimi K2.6 (262K) | GLM-4.6 (200K) | Mistral Codestral (256K) | 전체 코드베이스 로딩 |
| **Thinking/reasoning** | GLM (thinking mode) | DeepSeek R1 (ADAPTER-001) | Groq (qwen-qwq-32b) | 추론 태스크 |
| **공식 SDK 성숙도** | Mistral | Qwen (DashScope) | GLM | OpenAI-compat 표준 준수 |
| **지역 중립성 (intl + cn)** | Qwen | Kimi | GLM | 중국 접근성 우려 대응 |

### 3.1 주요 사용 시나리오별 권장 provider

- **CG Mode (Claude + GLM cost optimization)**: GLM-4.6 주 + Groq Llama 3.3 fallback
- **개발/테스트 (비용 0)**: OpenRouter free models + Groq free tier
- **Production 고속 inference**: Cerebras (Llama 3.3 70B) + Groq fallback
- **Long context 분석**: Kimi K2.6 (262K) or GLM-4.6 (200K)
- **코딩 태스크**: Qwen3 Coder 480B (OpenRouter/Fireworks) or Codestral (Mistral)
- **R1-style reasoning 저가 경로**: DeepSeek R1 via Fireworks/Together (cached 50% 할인)

---

## 4. 재사용 근거

### 4.1 SPEC-001 `openai.OpenAIAdapter` 재사용 가능성

SPEC-GOOSE-ADAPTER-001은 `internal/llm/provider/openai/adapter.go`에 다음 패턴을 구현한다 (SPEC-001 §6.6 참조):

```go
type OpenAIOptions struct {
    Name    string       // provider 식별자 (e.g., "xai", "glm")
    BaseURL string       // API endpoint override
    Pool    *credential.CredentialPool
    Tracker *ratelimit.Tracker
    Logger  *zap.Logger
    Capabilities provider.Capabilities  // per-provider 능력 플래그
}

func NewWithBase(opts OpenAIOptions) *OpenAIAdapter
```

이 시점에서 **9개 provider 중 8개(GLM 제외)는 다음과 같이 단일 팩토리 파일로 구현 가능**:

```go
// internal/llm/provider/groq/client.go (예시, ~35 LOC)
package groq

import (
    "github.com/modu-ai/goose/internal/llm/provider"
    "github.com/modu-ai/goose/internal/llm/provider/openai"
    // ...
)

func New(pool, tracker, secretStore, logger) (*openai.OpenAIAdapter, error) {
    return openai.NewWithBase(openai.Options{
        Name:    "groq",
        BaseURL: "https://api.groq.com/openai/v1",
        Pool:    pool,
        Tracker: tracker,
        Logger:  logger,
        Capabilities: provider.Capabilities{
            Streaming: true,
            Tools:     true,
            Vision:    false,
            Embed:     false,
        },
    }), nil
}
```

### 4.2 GLM 예외 처리

GLM은 thinking mode 파라미터가 OpenAI Chat Completions schema에 존재하지 않는다. 따라서:

```go
// internal/llm/provider/glm/adapter.go (~80 LOC)
type GLMAdapter struct {
    *openai.OpenAIAdapter  // embedding으로 기본 기능 상속
    thinkingEnabled bool   // per-request 결정
}

// Stream은 override하여 thinking param 주입 후 상위 호출
func (g *GLMAdapter) Stream(ctx, req) (<-chan StreamEvent, error) {
    if shouldEnableThinking(req.Route.Model, req.Thinking) {
        req = injectThinkingParam(req)  // body에 {"thinking": {"type": "enabled"}} 추가
    }
    return g.OpenAIAdapter.Stream(ctx, req)
}
```

GLM의 thinking 파라미터는 Anthropic의 `thinking.go`(SPEC-001 §6.8)를 일부 재사용하나, JSON schema는 독자적으로 관리한다.

### 4.3 재사용 효율 추정

| 구분 | 예상 LOC (구현) | 예상 LOC (테스트) |
|------|---------------|----------------|
| Groq client.go | 35 | 80 |
| OpenRouter client.go | 50 (헤더 주입 포함) | 100 |
| Together client.go | 35 | 80 |
| Fireworks client.go | 35 | 80 |
| Cerebras client.go | 35 | 80 |
| Mistral client.go | 35 | 80 |
| Qwen client.go | 55 (지역 선택) | 100 |
| Kimi client.go | 55 (지역 선택) | 100 |
| GLM adapter.go + thinking.go | 150 | 200 |
| Router registry 업데이트 | 50 | 40 |
| **합계** | **~535 LOC** | **~940 LOC** |
| **총 예상** | **~1,475 LOC** | |

SPEC-001이 만들어놓은 `openai.OpenAIAdapter`(1,000+ LOC 추정)을 거의 0 LOC 복제 없이 재사용한다. 9 provider 추가 비용은 provider당 평균 **59 LOC 구현 + 100 LOC 테스트**로 극히 낮다.

---

## 5. 테스트 전략

### 5.1 Httptest stub 기반 격리

모든 provider 테스트는 실제 API를 호출하지 않고 `httptest.NewServer`로 stub 응답을 반환한다. SPEC-001의 `openai/adapter_test.go` 패턴을 그대로 계승.

### 5.2 Per-provider 테스트 요구

| Provider | 기본 테스트 | 특수 테스트 |
|----------|-----------|-----------|
| GLM | streaming, tool, vision | **thinking mode on/off**, model별 adaptive 판별 |
| Groq | streaming, tool | rate limit 헤더(`x-ratelimit-remaining-requests`) 파싱 |
| OpenRouter | streaming, tool | HTTP-Referer 헤더 검증, free model routing |
| Together | streaming, tool | 모델 ID 형식(`meta-llama/...`) |
| Fireworks | streaming, tool | 모델 ID 형식(`accounts/fireworks/...`) |
| Cerebras | streaming | 속도 검증 (stub latency 계측만) |
| Mistral | streaming, tool, json mode | - |
| Qwen | streaming, tool | **지역 URL 선택**(intl vs cn) |
| Kimi | streaming, tool | **262K context 처리**, 지역 URL 선택 |

### 5.3 통합 테스트

`TestDefaultRegistry_AllAdaptersReady`: DefaultRegistry의 15 provider 전부 AdapterReady=true 여부 검증.

---

## 6. 출처 (Sources)

본 조사에 사용된 URL은 2026-04-24 기준 WebSearch 결과로부터 수집되었다.

### 6.1 Z.ai GLM

- `https://docs.z.ai/guides/llm/glm-4.6` — GLM-4.6 문서
- `https://docs.z.ai/guides/overview/pricing` — Z.ai pricing
- `https://z.ai/blog/glm-4.6` — GLM-4.6 출시 발표
- `https://www.anthropic.com/news/...` (비교 참조) — Anthropic thinking mode reference

### 6.2 Groq

- `https://console.groq.com/docs/overview` — Groq 공식 docs
- `https://console.groq.com/docs/rate-limits` — Rate limits
- `https://groq.com/pricing` — Pricing
- `https://artificialanalysis.ai/providers/groq` — 315 TPS 벤치마크

### 6.3 OpenRouter

- `https://openrouter.ai/docs` — OpenRouter 공식 docs
- `https://openrouter.ai/models?q=free` — Free models list
- `https://openrouter.ai/docs/api-reference/headers` — HTTP-Referer, X-Title 문서

### 6.4 Together AI

- `https://docs.together.ai/docs/quickstart` — Together AI quickstart
- `https://www.together.ai/pricing` — Pricing
- `https://docs.together.ai/docs/serverless-models` — 173 models 목록

### 6.5 Fireworks AI

- `https://docs.fireworks.ai/getting-started/introduction` — Fireworks docs
- `https://fireworks.ai/pricing` — Pricing
- `https://docs.fireworks.ai/guides/batch-inference` — 50% batch 할인

### 6.6 Cerebras

- `https://inference-docs.cerebras.ai/` — Cerebras Inference docs
- `https://cerebras.ai/inference` — 1000+ TPS 벤치마크
- `https://inference-docs.cerebras.ai/rate-limits` — Free tier 제한

### 6.7 Mistral AI

- `https://docs.mistral.ai/api/` — Mistral API 공식 docs
- `https://docs.mistral.ai/capabilities/completion/` — OpenAI-compat 예시
- `https://mistral.ai/pricing` — Mistral Nemo $0.02/M

### 6.8 Qwen (DashScope)

- `https://help.aliyun.com/zh/model-studio/developer-reference/compatibility-of-openai-with-dashscope` — OpenAI-compat 공식
- `https://qwenlm.github.io/blog/qwen3-max/` — Qwen3 Max 발표
- `https://help.aliyun.com/zh/model-studio/getting-started/models` — 모델 목록

### 6.9 Kimi (Moonshot AI)

- `https://platform.moonshot.ai/docs/intro` — Moonshot 공식 docs (intl)
- `https://platform.moonshot.cn/docs/intro` — Moonshot 공식 docs (cn)
- `https://moonshot.ai/blog/kimi-k2-6` — Kimi K2.6 출시 (2026-04)
- `https://platform.moonshot.ai/docs/api/anthropic` — Anthropic-compat endpoint (본 SPEC 범위 외)

### 6.10 통합 / 비교

- `https://artificialanalysis.ai/leaderboards/providers` — Provider 속도 벤치마크
- `https://artificialanalysis.ai/models` — 모델 가격/성능 비교

---

## 7. 조사 완료 요약

- **9 provider 전수 조사 완료**: Base URL, auth, rate limit, suggested models, 특이 기능, 재사용 난이도 확정
- **Z.ai GLM endpoint 이전 확정**: `open.bigmodel.cn` → `api.z.ai/api/paas/v4` 교체 필요
- **재사용 설계 검증**: 8/9 provider가 `openai.OpenAIAdapter` 팩토리 래핑으로 해결 가능, GLM만 thinking 래퍼 추가
- **예상 구현 규모**: ~535 LOC 구현 + ~940 LOC 테스트 = **~1,475 LOC**
- **경쟁 포지셔닝 파악**: 속도(Cerebras/Groq), 다양성(OpenRouter), 저가(Mistral Nemo), long context(Kimi), reasoning(GLM)

본 조사 결과는 `spec.md`의 EARS 요구사항 및 AC 작성에 반영된다.

**End of research.md**
