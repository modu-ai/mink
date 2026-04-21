# AgentOS LLM 라우팅 및 구독 공유 레이어 연구 보고서

> **작성일**: 2026년 4월 10일  
> **주제**: AgentOS 가상 에이전트 사회를 위한 LLM 라우팅 계층 및 다중 테넌트 구독 공유 아키텍처 설계

---

## 목차

1. [Codex CLI OAuth 모델](#1-codex-cli-oauth-모델)
2. [Claude Code 구독 인증](#2-claude-code-구독-인증)
3. [OpenRouter 멀티 프로바이더 아키텍처](#3-openrouter-멀티-프로바이더-아키텍처)
4. [LiteLLM 통합 프록시 플랫폼](#4-litellm-통합-프록시-플랫폼)
5. [LLM 게이트웨이 패턴](#5-llm-게이트웨이-패턴)
6. [로컬 LLM 런타임](#6-로컬-llm-런타임)
7. [모델 라우팅 전략](#7-모델-라우팅-전략)
8. [컨텍스트 윈도우 및 도구 호출 호환성](#8-컨텍스트-윈도우-및-도구-호출-호환성-매트릭스)
9. [스트리밍, 캐싱, 구조화된 출력](#9-스트리밍-캐싱-구조화된-출력)
10. [다중 테넌트 구독 공유](#10-다중-테넌트-구독-공유-및-보안)
11. [가격 추적 및 비용 귀속](#11-가격-추적-및-비용-귀속)
12. [프로덕션 레벨 장애 처리](#12-프로덕션-레벨-장애-처리)
13. [AgentOS 설계 원칙](#agentos-llm-라우팅-설계-원칙)

---

## 1. Codex CLI OAuth 모델

### 소개
Codex CLI는 OpenAI의 ChatGPT Plus/Pro 구독을 IDE 환경에서 직접 사용할 수 있게 해주는 커맨드라인 인터페이스입니다.

**주요 소스**: [OpenAI Codex Authentication Documentation](https://developers.openai.com/codex/auth)

### 토큰 플로우
- **PKCE 기반 OAuth 플로우**: Authorization Code Flow with Proof Key for Code Exchange
- **인증 엔드포인트**: `https://auth.openai.com/oauth/authorize`
- **토큰 엔드포인트**: `https://auth.openai.com/oauth/token`
- **Bearer Token**: 로컬 브라우저 윈도우에서 OAuth 완료 후 토큰 반환

### 제한사항 및 기능
- 로컬 인증: `~/.codex/auth.json` 또는 OS 네이티브 자격증명 스토어에 캐시
- **Fast Mode**: ChatGPT 크레딧이 필요한 기능 (구독자만 가능)
- 토큰 자동 갱신: 만료 5분 전에 자동 갱신
- **Device Code 인증** (Beta): 헤드리스 환경을 위한 대안
- RBAC 지원: ChatGPT 워크스페이스 권한 및 엔터프라이즈 설정 적용

### AgentOS 설계 의미
- OAuth 토큰 캐싱 및 갱신 메커니즘 구현 필요
- Device Code Flow를 원격/헤드리스 환경 지원용 구현

---

## 2. Claude Code 구독 인증

### 소개
Claude Pro/Max 구독은 Claude Code를 통해 OAuth 기반 인증으로 사용 가능합니다.

**주요 소스**: 
- [Claude Code Authentication Documentation](https://code.claude.com/docs/en/authentication)
- [Using Claude Code Max Subscription - liteLLM](https://docs.litellm.ai/docs/tutorials/claude_code_max_subscription)

### OAuth 플로우
- **기본 방식**: Pro/Max 구독자는 `/login` 커맨드로 OAuth 인증
- **토큰 수명**: 접근 토큰은 약 60분 내에 만료
- **갱신 토큰**: 더 오래 유지되는 갱신 토큰으로 재인증 없이 새 접근 토큰 획득

### 멀티 테넌트 구현
- **API 게이트웨이 전달**: `forward_client_headers_to_llm_api: true`로 사용자 OAuth 토큰을 Anthropic API에 전달
- 각 사용자의 구독 계획을 자동으로 추적

### 법적 제한사항
- Anthropic이 제3자 애플리케이션에서의 Claude OAuth 지원 철회 (법적 요청)
- OpenAI, GitHub, GitLab 등의 대안으로 리디렉션

### AgentOS 설계 의미
- Claude OAuth는 제3자 통합에서 제한되므로 OpenAI OAuth 우선 고려
- 토큰 갱신 메커니즘은 필수 (60분 TTL)

---

## 3. OpenRouter 멀티 프로바이더 아키텍처

### 소개
OpenRouter는 수십 개 프로바이더의 수백 개 LLM에 접근할 수 있는 통합 API 레이어입니다.

**주요 소스**: 
- [OpenRouter Provider Routing Guide](https://openrouter.ai/docs/guides/routing/provider-selection)
- [OpenRouter Model Fallbacks](https://openrouter.ai/docs/guides/routing/model-fallbacks)

### 라우팅 전략
1. **기본 로드 밸런싱**: 가격 우선 정렬, 과거 30초 동안 심각한 장애가 없었던 프로바이더 우선
2. **가격 기반 선택**: 최저 비용 후보 선택, 가격의 역제곱으로 가중치 적용
3. **자동 장애 조치**: 프로바이더 다운 또는 속도 제한 시 다음 프로바이더로 투명하게 전환

### 모델 파티셔닝 및 변형
- **Variants**: `:free`, `:extended`, `:thinking` 같은 접미사로 모델 동작 변경
- **partition: "model"**: 기본값, 주 모델의 엔드포인트를 항상 먼저 시도
- **partition: "none"**: 전역 정렬로 모든 엔드포인트를 성능 특성에 따라 선택

### 고급 라우팅 옵션
- **require_parameters**: 모든 파라미터를 지원하는 프로바이더로만 라우팅
- **data_collection**: 데이터 정책 준수 프로바이더로만 제한

### AgentOS 설계 의미
- OpenRouter의 자동 장애 조치 및 로드 밸런싱 메커니즘 참고
- 모델 변형 시스템으로 경제 모드와 고급 모드 선택 가능하게 함

---

## 4. LiteLLM 통합 프록시 플랫폼

### 소개
LiteLLM은 100+ LLM API를 OpenAI 형식으로 통합된 인터페이스로 제공하는 오픈소스 프록시 서버입니다.

**주요 소스**:
- [LiteLLM GitHub Repository](https://github.com/BerriAI/litellm)
- [LiteLLM Documentation](https://docs.litellm.ai/)
- [Spend Tracking - LiteLLM](https://docs.litellm.ai/docs/proxy/cost_tracking)
- [Budget Routing - LiteLLM](https://docs.litellm.ai/docs/proxy/provider_budget_routing)

### 핵심 기능

**지원 프로바이더**
- Bedrock, Azure, OpenAI, VertexAI, Cohere, Anthropic, Sagemaker, HuggingFace, VLLM, NVIDIA NIM

**비용 추적**
- 키, 사용자, 팀 단위의 지출 추적 (100+ LLM)
- 자동 모델 가격 매핑
- 커스텀 가격 설정 가능

**라우팅 및 로드 밸런싱**
- 여러 배포 간 자동 재시도/장애 조치
- 라우터 클래스를 통한 배포 수준 장애 조치
- 다중 테넌트 복원력

**프록시 기능**
- OpenAI 호환 게이트웨이 (코드 변경 없음)
- 중앙식 인증 및 권한 부여
- 다중 테넌트 비용 추적 및 지출 관리
- 가상 키를 통한 보안 액세스 제어
- 관리 대시보드

### AgentOS 설계 의미
- AgentOS의 핵심 LLM 게이트웨이로 LiteLLM 프록시 채택
- 에이전트별 비용 추적을 위해 가상 키 시스템 활용
- 로컬 Ollama와 API 프로바이더를 통합하는 라우터로 활용

---

## 5. LLM 게이트웨이 패턴 (Portkey, Helicone, Langfuse)

### 개요
**주요 소스**:
- [Langfuse - Portkey Integration](https://portkey.ai/docs/integrations/tracing-providers/langfuse)
- [Helicone - LLM Observability](https://www.helicone.ai/)
- [Langfuse Documentation](https://langfuse.com/)
- [Top 5 LLM Gateways Comparison 2025](https://www.helicone.ai/blog/top-llm-gateways-comparison-2025)

### Portkey
- **역할**: AI 게이트웨이 (라우팅 중심)
- **특징**: 250+ 모델 통합, 조건부 라우팅, 폴백, 캐싱, 자동 재시도

### Helicone
- **역할**: LLM 관찰성 플랫폼
- **특징**: 자세한 분석, 비용 분석, 지연시간 모니터링

### Langfuse
- **역할**: 오픈소스 LLM 엔지니어링 플랫폼
- **특징**: 트레이싱, 모니터링, 평가 및 테스팅, 통합 가능

### 권장 조합
Portkey(라우팅) + Langfuse(관찰성)

### AgentOS 설계 의미
- Portkey 또는 LiteLLM을 라우팅 계층으로
- Langfuse를 관찰성 계층으로 통합

---

## 6. 로컬 LLM 런타임

### 6.1 Ollama

**주요 소스**: [Ollama GitHub Repository](https://github.com/ollama/ollama)

**특징**
- CLI 및 REST API를 통한 로컬 LLM 실행
- 모델 다운로드 및 관리 자동화

**지원 모델**
- **Gemma 3**: 1b~27b 파라미터, 128K 컨텍스트, 140+ 언어
- **Qwen 3/2.5**: 최대 128K 토큰, 다국어, MoE 모델
- **Llama 3.1** 및 기타

**성능**
- 소비자 GPU: 300+ 토큰/초
- 고성능: 1,200+ 토큰/초
- 8GB RAM: 양자화된 8B 모델 실행 가능

### 6.2 LM Studio

**특징**
- GUI 기반 로컬 LLM 관리
- llama.cpp와 MLX 엔진 혼합 사용
- OpenAI 호환 API

### 6.3 MLX (Apple Silicon)

**성능 비교** (M2 Pro)
- MLX: ~230 토큰/초
- llama.cpp: ~150 토큰/초
- vLLM-MLX: 400+ 토큰/초

### AgentOS 설계 의미
- Ollama: 통합 로컬 LLM 관리
- MLX (Apple Silicon): 최대 성능
- vLLM-MLX: 프로덕션 서버 (OpenAI 호환)

---

## 7. 모델 라우팅 전략

### 7.1 RouteLLM

**주요 소스**: [RouteLLM - Cost-Aware Routing](https://arxiv.org/abs/2508.12491)

**성과**
- MT Bench: 85% 비용 절감
- MMLU: 45% 비용 절감
- GSM8K: 35% 비용 절감

### 7.2 FrugalGPT

**개념**: 캐스케이딩 접근으로 경량 모델에서 시작, 필요시 강력한 모델로 에스컬레이션

### 7.3 Cascade Routing

**주요 소스**: [Cascade Routing - A Unified Approach](https://files.sri.inf.ethz.ch/website/papers/dekoninck2024cascaderouting.pdf)

**전략**
1. **비용 중심**: 응답 품질 임계값까지 저비용 모델 사용
2. **능력 중심**: 쿼리 복잡도에 따라 모델 선택

### AgentOS 설계 의미
- 에이전트 쿼리 복잡도 분류 시스템 구현
- 비용-성능 trade-off 자동 최적화

---

## 8. 컨텍스트 윈도우 및 도구 호출 호환성 매트릭스

### 컨텍스트 윈도우 현황 (2026)

**주요 소스**: [LLM Context Window Comparison 2026](https://www.morphllm.com/llm-context-window-comparison)

| 프로바이더 | 모델 | 표준 | 최대 |
|---------|------|------|------|
| Anthropic | Claude 3.5 Sonnet | 200K | 1M |
| OpenAI | GPT-4.1 | 128K | 1M |
| Google | Gemini | 변동 | 2M+ |
| Meta | Llama 4 Scout | 8K | 10M |

**주의사항**: 실제 효과적인 컨텍스트는 광고된 것의 50-65%

### 도구 호출 지원

- **OpenAI**: Function Calling + JSON Mode
- **Claude**: XML 형식 도구 정의
- **Gemini**: 도구 지원 + Grounding
- **오픈소스**: 프롬핑 또는 미세조정

### AgentOS 설계 의미
- 프로바이더별 컨텍스트 윈도우 매트릭스 구축
- 도구 호출 호환성 계층 추상화

---

## 9. 스트리밍, 캐싱, 구조화된 출력

### 9.1 Anthropic Prompt Caching

**주요 소스**: [Prompt Caching - Claude API](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)

**가격**
- 캐시 읽기: 기본 입력 토큰의 0.1배
- 캐시 쓰기 (5분): 1.25배
- 캐시 쓰기 (1시간): 2배

### 9.2 구조화된 출력

**주요 소스**: [Structured Outputs - Claude API](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)

- JSON 스키마를 문법으로 컴파일
- 100% 신뢰도 있는 구조화된 응답

### 9.3 스트리밍
- 기본: 모두 도달 후 스트리밍
- 세밀한 스트리밍 (Beta): 증분 도구 입력 스트리밍 가능

### AgentOS 설계 의미
- 반복적인 에이전트 프롬프트에 Anthropic 캐싱 활용
- 모든 구조화된 응답에 native structured outputs 사용

---

## 10. 다중 테넌트 구독 공유 및 보안

### 10.1 다중 테넌트 아키텍처

**주요 소스**:
- [Multi-Tenant Architecture with LiteLLM](https://docs.litellm.ai/docs/proxy/multi_tenant_architecture)
- [Prompt Leakage via KV-Cache Sharing](https://www.ndss-symposium.org/wp-content/uploads/2025-1772-paper.pdf)

**구조**
```
Organization
  ├── Team 1
  │   ├── User A (Key A)
  │   └── User B (Key B)
  └── Team 2
      └── User C (Key C)
```

### 10.2 보안 위협

- **KV-Cache 부채널**: 다중 테넌트 서버에서 정보 유출 가능
- **API 키 유출**: 데이터 누출, 프롬프트 인젝션 위험
- **비용 제어 부족**: 무제한 사용으로 급증

### 10.3 방어 전략

- **테넌트 격리**: 테넌트별 자격증명 분리, RBAC 구현
- **속도 제한**: Premium 1000 req/hr, Free 100 req/hr
- **최소 권한**: API 키는 필요한 권한만 부여
- **모니터링**: API 키 사용 분석, 비정상 탐지

### AgentOS 설계 의미
- 에이전트별 독립적 API 키 풀 유지
- LiteLLM의 다중 테넌트 기능 활용
- KV-Cache 공유 위험을 고려한 로컬 LLM 분리

---

## 11. 가격 추적 및 비용 귀속

### 11.1 비용 추적 기초

**주요 소스**:
- [From Bills to Budgets: Token Usage Tracking](https://www.traceloop.com/blog/from-bills-to-budgets-how-to-track-llm-token-usage-and-cost-per-user)
- [Token & Cost Tracking - Langfuse](https://langfuse.com/docs/observability/features/token-and-cost-tracking)

**핵심**: 유일한 비용 제어 방법 = 토큰 사용량 추적 + 특정 차원에 귀속

### 11.2 메타데이터 기반 귀속

**태깅 시스템**
```json
{
  "metadata": {
    "user_id": "user_123",
    "agent_id": "research_agent",
    "session_id": "session_456",
    "feature": "document_analysis"
  }
}
```

**다차원 귀속**: 사용자별, 에이전트별, 기능별, 팀별, 프로젝트별

### 11.3 프로바이더별 가격

- **OpenAI**: $0.003-0.15/1K input tokens
- **Anthropic**: $0.003-0.80/1K input tokens
- **Meta Llama (API)**: $0.001-0.002/1K input tokens
- **로컬 (Ollama)**: 인프라 비용만

### AgentOS 설계 의미
- 모든 LLM 호출에 agent_id, feature_name 메타데이터 추가
- LiteLLM 비용 추적 기능 활용
- 월간 에이전트별 비용 리포트 생성

---

## 12. 프로덕션 레벨 장애 처리

### 12.1 오류 분류

**주요 소스**: [Retries, Fallbacks, and Circuit Breakers](https://portkey.ai/blog/retries-fallbacks-and-circuit-breakers-in-llm-apps/)

**일시적 오류** (재시도 적합)
- 429: Rate Limit
- 503: Service Unavailable
- 네트워크 오류

**영구적 오류** (재시도 부적합)
- 401: Unauthorized
- 403: Forbidden
- 404: Not Found

### 12.2 재시도 전략 (Exponential Backoff + Jitter)

**핵심**
- 지수 백오프: 1s → 2s → 4s → 8s → 16s (캡: 30s)
- 지터: "Thundering Herd" 문제 방지 (±20% 랜덤)

### 12.3 장애 조치 (Fallback)

```
Primary (OpenAI GPT-4)
  ↓ fails
Secondary (Anthropic Claude)
  ↓ fails
Tertiary (OpenRouter)
  ↓ fails
Local (Ollama Gemma)
  ↓ fails
Queue for Retry
```

### 12.4 서킷 브레이커

**상태 전이**: CLOSED → OPEN (5개 실패) → HALF_OPEN (60초) → CLOSED

### 12.5 속도 제한 및 할당량

**주요 소스**: [Rate Limiting for LLM Apps - Portkey](https://portkey.ai/blog/tackling-rate-limiting-for-llm-apps/)

**계층 구조**
```
Global (10,000 req/min)
  └── Organization (1,000 req/min)
      └── Team (100 req/min)
          └── User (10 req/min)
```

**토큰 기반 제한**: 요청 수 제한보다 정확, 비용 반영

### AgentOS 설계 의미
- 모든 LLM 호출에 재시도 로직 추가
- 다단계 장애 조치 체인 구현
- 프로바이더별 서킷 브레이커
- 에이전트별 토큰 할당량 관리

---

## AgentOS LLM 라우팅 설계 원칙

### 아키텍처 개요

```
┌─────────────────────────────────────────────────┐
│  AgentOS Virtual Agent Society                   │
│  └─ Agent Orchestration Layer                   │
└──────────────────┬──────────────────────────────┘
                   ↓
┌─────────────────────────────────────────────────┐
│  LLM Routing & Gateway Layer                     │
│  ├─ Smart Router (Capability & Cost Aware)      │
│  ├─ Failover & Retry Management                │
│  └─ Cost Tracking & Attribution                │
└──────────────────┬──────────────────────────────┘
       ┌───────────┼───────────┬──────────┐
       ↓           ↓           ↓          ↓
    OpenAI    Anthropic   OpenRouter  Ollama
```

### 1. 계층적 모델 팀 (Tiered Model Fleet)

**3계층 구조**
- **Economy**: Ollama Gemma (비용 $0.0001/MTok)
- **Standard**: GPT-3.5/Claude Haiku (비용 $0.005/MTok)
- **Premium**: GPT-4/Claude Sonnet (비용 $0.03/MTok)

### 2. 적응형 라우팅 알고리즘

```
1. 쿼리 복잡도 분석
   ↓
2. 필요한 능력 결정
   ↓
3. 비용-이점 계산
   ↓
4. 최적 모델 선택
```

### 3. 다중 테넌트 구독 공유

- 테넌트별 API 키 관리
- 토큰 할당량 추적
- 속도 제한 적용

### 4. 장애 처리 및 복원력

- Circuit Breaker per Provider
- Exponential Backoff + Jitter
- Cascade Fallback Chain

### 5. 관찰성 및 모니터링

- Langfuse 트레이싱
- Prometheus 메트릭
- 에이전트별 비용 리포트

### 6. 보안 및 데이터 격리

- 에이전트별 격리된 클라이언트
- API 키 안전 관리
- RBAC 구현

### 7. 구현 우선순위

**Phase 1 (MVP - 4주)**
- LiteLLM 프록시 설정
- 기본 라우팅 (비용 기반)
- 지수 백오프 재시도
- 기본 비용 추적

**Phase 2 (6주)**
- 적응형 라우팅 (복잡도 기반)
- 다단계 장애 조치
- 서킷 브레이커
- Langfuse 통합

**Phase 3 (8주)**
- RouteLLM 기반 동적 라우팅
- 공유 구독 풀 관리
- 에이전트별 예산 할당
- 고급 모니터링 대시보드

### 8. 기술 스택 권장사항

| 계층 | 컴포넌트 | 기술 |
|------|---------|------|
| 라우팅 | LLM Gateway | LiteLLM Proxy |
| 로컬 | Local Runtime | Ollama + MLX |
| 관찰성 | Tracing | Langfuse |
| 비용 | Cost Analytics | Langfuse + Custom |
| 인증 | Auth | LiteLLM Virtual Keys |
| 모니터링 | Metrics | Prometheus + Grafana |

---

## 참고자료 및 소스 (29개)

1. [OpenAI Codex Authentication](https://developers.openai.com/codex/auth)
2. [Claude Code Authentication](https://code.claude.com/docs/en/authentication)
3. [Claude Code Max Subscription - liteLLM](https://docs.litellm.ai/docs/tutorials/claude_code_max_subscription)
4. [OpenRouter Provider Routing Guide](https://openrouter.ai/docs/guides/routing/provider-selection)
5. [OpenRouter Model Fallbacks](https://openrouter.ai/docs/guides/routing/model-fallbacks)
6. [LiteLLM GitHub Repository](https://github.com/BerriAI/litellm)
7. [LiteLLM Spend Tracking Documentation](https://docs.litellm.ai/docs/proxy/cost_tracking)
8. [Portkey - Langfuse Integration](https://portkey.ai/docs/integrations/tracing-providers/langfuse)
9. [Helicone - LLM Observability](https://www.helicone.ai/)
10. [Langfuse Documentation](https://langfuse.com/)
11. [Top 5 LLM Gateways Comparison 2025](https://www.helicone.ai/blog/top-llm-gateways-comparison-2025)
12. [Ollama GitHub Repository](https://github.com/ollama/ollama)
13. [LM Studio with Apple MLX](https://lmstudio.ai/blog/lmstudio-v0.3.4)
14. [Production-Grade Local LLM on Apple Silicon](https://arxiv.org/abs/2511.05502)
15. [vLLM-MLX for Apple Silicon](https://github.com/waybarrios/vllm-mlx)
16. [RouteLLM - Cost-Aware Routing](https://arxiv.org/abs/2508.12491)
17. [Cascade Routing - A Unified Approach](https://files.sri.inf.ethz.ch/website/papers/dekoninck2024cascaderouting.pdf)
18. [LLM Context Window Comparison 2026](https://www.morphllm.com/llm-context-window-comparison)
19. [Anthropic Prompt Caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)
20. [Claude Structured Outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)
21. [Multi-Tenant Architecture with LiteLLM](https://docs.litellm.ai/docs/proxy/multi_tenant_architecture)
22. [Prompt Leakage via KV-Cache Sharing](https://www.ndss-symposium.org/wp-content/uploads/2025-1772-paper.pdf)
23. [Multi-Cloud LLM Security Best Practices](https://latitude.so/blog/10-best-practices-for-multi-cloud-llm-security)
24. [From Bills to Budgets - Token Usage Tracking](https://www.traceloop.com/blog/from-bills-to-budgets-how-to-track-llm-token-usage-and-cost-per-user)
25. [Langfuse Token & Cost Tracking](https://langfuse.com/docs/observability/features/token-and-cost-tracking)
26. [LLM Pricing Calculator](https://www.llm-prices.com/)
27. [Retries, Fallbacks, and Circuit Breakers in LLM Apps](https://portkey.ai/blog/retries-fallbacks-and-circuit-breakers-in-llm-apps/)
28. [Rate Limiting for LLM Apps - Portkey](https://portkey.ai/blog/tackling-rate-limiting-for-llm-apps/)
29. [LLM Error Handling for Production](https://www.buildmvpfast.com/blog/building-with-unreliable-ai-error-handling-fallback-strategies-2026)

---

**작성**: 2026년 4월 10일  
**버전**: 1.0  
**상태**: 기술 검토 준비 완료
