---
id: SPEC-MINK-LLM-ROUTING-V2-AMEND-001
version: 0.1.0
status: draft
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI orchestrator
priority: high
issue_number: null
phase: 3
size: 중(M)
lifecycle: spec-first
labels: [llm, routing, oauth, provider, amendment, sprint-3]
amends: [SPEC-GOOSE-LLM-ROUTING-V2-001]
supersedes: []
superseded_by: []
related:
  - ADR-001 (QLoRA/RL 폐기)
  - ADR-002 (AGPL-3.0 전환)
  - SPEC-MINK-MEMORY-QMD-001
  - SPEC-MINK-AUTH-CREDENTIAL-001
---

# SPEC-MINK-LLM-ROUTING-V2-AMEND-001 — 5-Provider 외부 LLM 라우팅 amendment

> **STUB / DRAFT (2026-05-16)**: 본 SPEC 은 사용자 결정 진입을 표시하는 *amendment plan stub* 이다. 본격 EARS 요구사항 작성·plan-auditor pass 는 후속 PR (manager-spec spawn) 에서 처리한다. 산출물 5종 (research.md / plan.md / tasks.md / acceptance.md / progress.md) 도 후속 PR 에서 추가한다.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | amendment stub — 5라운드 사용자 결정 totals 진입 표시 | MoAI orchestrator |

---

## 1. 개요 (Overview)

기존 SPEC-GOOSE-LLM-ROUTING-V2-001 (v0.2.0 implemented) 는 15-direct provider 풀을 가정했다. 2026-05-16 사용자 결정으로 다음과 같이 amendment 한다.

### 1.1 5 Provider 정선 (OpenClaw 정책 동일)

| Provider | 인증 방식 | OpenAI-compat | 노트 |
|---|---|---|---|
| **Anthropic Claude** | API key paste (`console.anthropic.com`) | No (자체 messages API) | claude-sonnet-4-7 / claude-opus-4-7 |
| **DeepSeek** | API key paste (`platform.deepseek.com`) | Yes | deepseek-chat / deepseek-reasoner |
| **OpenAI GPT (API mode)** | API key paste | Yes (native) | gpt-5 / gpt-5.5 |
| **Codex (ChatGPT OAuth)** | OAuth (PKCE + browser callback) | Codex-specific | gpt-5.5 + fast mode + ChatGPT 구독 크레딧 |
| **z.ai GLM-5-Turbo** | API key paste (`z.ai/manage-apikey`) | Yes | Coding Plan base URL `https://api.z.ai/api/coding/paas/v4` |

15-direct → 5 정선. 단 OpenAI-compat custom endpoint (사용자 정의 Ollama / vLLM / lm-studio) 옵션 보존 (REQ-LR2-AM-006 예정).

### 1.2 폐기 결정 흡수

- ADR-001 (PR #228) 의 QLoRA/RL 폐기 와 정합: *외부 GOAT LLM 호출* 만 사용, 모델 호스팅 0.
- ADR-002 (PR #230) 의 AGPL-3.0 전환과 정합: 본 SPEC 도 AGPL 헌장 위에서 작성됨.

### 1.3 라우팅 정책 (요지, 본격 EARS 는 후속)

- **비용 우선**: DeepSeek + GLM-5-Turbo (PAYG 가장 저렴)
- **품질 우선**: Claude Opus 4.7 + GPT-5.5
- **코딩 우선**: Codex (GPT-5.5 + fast mode) + GLM-5 Coding Plan
- **default fallback chain**: 활성 provider 중 사용자 우선순위 → 첫 실패 시 next → 모두 실패 시 사용자 알림

## 2. 배경 (Background)

### 2.1 기존 SPEC 과의 차이

| 항목 | LLM-ROUTING-V2 (v0.2.0) | 본 amendment |
|---|---|---|
| Provider 수 | 15 direct | **5 (정선)** + custom endpoint |
| 인증 흐름 | API key 중심 | **API key paste (4) + ChatGPT OAuth (1)** |
| OAuth 흐름 | 미지원 | **Codex CLI OAuth (PKCE + device code)** |
| credential 저장 | 평문 config | **AUTH-CREDENTIAL-001 으로 위임** (keyring default + 평문 fallback) |
| 모델 호스팅 | (없음, 동일) | (없음, 동일) |
| OpenRouter passthrough | 옵션 검토 중 | **옵션 (custom endpoint 로 흡수, 별도 OpenRouter SPEC 미생성)** |

### 2.2 OpenClaw 정책 흡수

OpenClaw 의 LLM 정책 = "Claude / DeepSeek / GPT 외부, brower-paste 또는 OAuth". 본 amendment 는 동일 패턴 + z.ai GLM-5-Turbo 추가 + Codex OAuth 흐름 정식 포함.

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본격 plan 에서 EARS 화 예정)

- 5 provider 통합 (Claude / DeepSeek / GPT / Codex / GLM-5-Turbo)
- 인증 2 패턴: API key paste (4) + ChatGPT OAuth (1)
- `mink login {provider}` 통합 UX
- 라우팅 정책 (비용/품질/코딩 카테고리 + 사용자 우선순위)
- fallback chain
- Codex idle 8일 OAuth 재로그인 핸들링
- ONBOARDING-001 Phase 5 Web Step 2 wiring 인터페이스 정의
- AUTH-CREDENTIAL-001 위임 인터페이스

### 3.2 OUT OF SCOPE

- 자체 모델 호스팅
- 자체 임베딩 모델 (MEMORY-QMD-001 의 ollama 가 담당)
- OpenRouter 전용 어댑터 (custom endpoint 로 흡수 가능, 별도 SPEC 없음)
- 사용자 정의 prompt template 시스템 (별도 SPEC)
- A/B 테스트 / 트래픽 분할 (별도 SPEC)

## 4. 의존 SPEC

- **SPEC-MINK-AUTH-CREDENTIAL-001**: credential 저장 (keyring + fallback)
- **SPEC-MINK-MEMORY-QMD-001**: session export hook (LLM 응답을 QMD 에 색인)
- **SPEC-MINK-ONBOARDING-001 Phase 5**: Web Step 2 provider 로그인 UI
- **SPEC-MINK-CLI-TUI-003 amendment**: `mink login` / `mink model` CLI=TUI 패리티
- **ADR-001**: QLoRA 폐기 — 외부 LLM 만 사용
- **ADR-002**: AGPL-3.0 전환 — 헌장

## 5. 본격 plan 으로 이월된 작업 (후속 PR)

- research.md: OpenAI/Anthropic/DeepSeek/z.ai/Codex 각 SDK·API 매트릭스, OAuth PKCE 흐름 상세, 8일 idle 재인증 동작 검증
- plan.md: 5 마일스톤 (M1 provider 어댑터 / M2 인증 흐름 통합 / M3 라우팅 정책 / M4 fallback chain / M5 Web wiring 인터페이스)
- tasks.md: 패키지·함수 단위 분해
- acceptance.md: 35+ EARS REQ ↔ 35+ AC 1:1 매핑
- plan-auditor pass

## 6. References

- 사용자 결정 (2026-05-16 Round 1~3 AskUserQuestion totals)
- ADR-001 (QLoRA/RL 폐기)
- ADR-002 (AGPL-3.0 전환)
- SPEC-GOOSE-LLM-ROUTING-V2-001 (기존 v0.2.0 implemented)
- Z.AI GLM-5 Coding Plan: https://docs.z.ai/guides/llm/glm-5
- Codex CLI Authentication: https://developers.openai.com/codex/auth
- Anthropic API: https://docs.anthropic.com/
- DeepSeek API: https://platform.deepseek.com/
