---
id: SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001
version: 0.1.0
status: draft
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI orchestrator
priority: high
phase: 3
size: 대(L)
lifecycle: spec-first
labels: [onboarding, web, provider, oauth, phase5, sprint-3, amendment]
amends: [SPEC-MINK-ONBOARDING-001]
related:
  - SPEC-MINK-LLM-ROUTING-V2-AMEND-001
  - SPEC-MINK-AUTH-CREDENTIAL-001
  - SPEC-MINK-MEMORY-QMD-001
---

# SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001 — Web Step 2 5-Provider 로그인 wiring + Step 3 채널 연결

> **STUB / DRAFT (2026-05-16)**: 본 SPEC 은 사용자 결정 진입을 표시하는 *amendment stub*. 본격 plan + 코드 구현은 후속 PR 에서 expert-frontend + expert-backend 병렬 spawn 으로 진행.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | Phase 5 amendment stub | MoAI orchestrator |

## 1. 개요

ONBOARDING-001 (v0.3.1 implemented, PR #211~#217) 의 Web Step 2 (LLM provider 로그인) 가 현재 placeholder. 본 Phase 5 amendment 는 LLM-ROUTING-V2-AMEND-001 의 5 provider 를 Web UI 에 정식 wiring + Step 3 채널 연결 (Telegram/Slack/Discord 3 고정) 추가.

## 2. 스코프

### 2.1 IN

- **Web Step 2** (LLM provider 로그인 UI):
  - 5 provider 카드 (Anthropic Claude / DeepSeek / OpenAI GPT / Codex / z.ai GLM-5-Turbo)
  - 2 인증 흐름:
    - **Key paste** (4): 브라우저 `console.anthropic.com` / `platform.deepseek.com` / `platform.openai.com` / `z.ai/manage-apikey` 새 탭 → 사용자 paste → POST `/install/provider/save`
    - **OAuth** (1, Codex): PKCE flow → 127.0.0.1:auto-port callback → state token 검증 → AUTH-CREDENTIAL-001 에 저장
  - credential validation (test API call) + 성공 indicator
- **Web Step 3** (채널 연결):
  - 3 채널 카드 (Telegram / Slack / Discord)
  - Telegram: bot token paste + `/start` 명령 확인
  - Slack: OAuth app installation (workspace 선택)
  - Discord: bot invite + Ed25519 public key 검증
- Web UI 설치 후 *설정* 페이지 (WEB-CONFIG-001) 의 entry 인터페이스 정의

### 2.2 OUT

- 일상 사용 Web UI (CLI/TUI 또는 채널로 처리)
- 신규 channel adapter 구현 (MSG-SLACK / MSG-DISCORD SPEC 책임)

## 3. 의존

- LLM-ROUTING-V2-AMEND-001 (5 provider 백엔드)
- AUTH-CREDENTIAL-001 (credential 저장)
- MSG-SLACK-001 / MSG-DISCORD-001 (Sprint 4, 채널 어댑터)
- WEB-CONFIG-001 (Sprint 4, 설치 후 설정)
- CROSSPLAT-001 §5.1 (OS keyring 가드)

## 4. 본격 plan 이월 (후속 PR)

- expert-frontend + expert-backend 병렬 spawn (isolation 미사용 foreground, 누적 22+ 회 검증 패턴)
- 5 산출물
- Playwright E2E (OAuth 콜백 모킹)
- plan-auditor pass
