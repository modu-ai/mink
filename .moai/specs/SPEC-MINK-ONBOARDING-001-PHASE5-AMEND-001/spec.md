---
id: SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001
version: 0.2.0
status: planned
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
  - SPEC-MINK-MSG-SLACK-001
  - SPEC-MINK-MSG-DISCORD-001
trust_metrics:
  requirements_total: 26
  acceptance_total: 28
  milestones: 4
---

# SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001 — Web Step 2 5-Provider 로그인 wiring + Step 3 채널 연결

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | stub (PR #232) | MoAI orchestrator |
| 0.2.0 | 2026-05-16 | 본격 EARS spec 승격. 코드 구현은 별도 PR 필수 (expert-frontend + expert-backend) | MoAI orchestrator |

> AGPL-3.0-only 헌장 (ADR-002). ONBOARDING-001 v0.3.1 implemented 의 Phase 3B Step 2~7 위에 5-provider 정식 wiring + 3 채널 연결 추가.

## 1. 개요

기존 ONBOARDING-001 의 Web Step 2 (LLM provider 로그인) 가 현재 placeholder 상태. 본 Phase 5 amendment 는 LLM-ROUTING-V2-AMEND-001 의 5 provider 를 정식 wiring + Step 3 채널 연결 (3 채널) 추가.

### 1.1 Surface Assumptions

- A1: ONBOARDING-001 v0.3.1 의 React + Vite + Tailwind + shadcn/ui + Step1Locale 완성 코드베이스 재사용
- A2: CSRF + Origin allowlist + SessionStore (per-session sync.Mutex 30분 TTL) 패턴 그대로 재사용
- A3: 5 provider 의 인증 라이브러리 (Anthropic SDK / DeepSeek OpenAI-compat / OpenAI SDK / Codex OAuth / z.ai OpenAI-compat) 가 LLM-ROUTING-V2-AMEND-001 의 인터페이스로 추상화
- A4: 3 채널 (Telegram/Slack/Discord) 의 어댑터 SPEC 이 동시 진행 (MSG-SLACK / MSG-DISCORD plan SPEC 완료, run 단계는 동시 또는 후속)
- A5: OAuth callback port 127.0.0.1:0 (auto-port) 가 5개 provider 동시 진입 시 충돌 없음 (단일 진행 사용자 모델)
- A6: AUTH-CREDENTIAL-001 의 Store/Load/Delete/List 4 메서드 인터페이스가 M2 진입 전 freeze

## 2. 스코프

### 2.1 IN

- **Step 2: LLM Provider 로그인 UI**:
  - 5 provider 카드 (Anthropic / DeepSeek / OpenAI / Codex / z.ai GLM)
  - 2 인증 흐름:
    - Key paste × 4: 브라우저 새 탭 → 사용자 paste → POST `/install/provider/save` → AUTH-CREDENTIAL-001 위임 저장
    - OAuth × 1 (Codex): PKCE flow → 127.0.0.1:auto-port callback → state token 검증 → AUTH 저장
  - 각 provider 별 test API call (credential validation) + 성공 indicator
  - 우선순위 변경 (drag-and-drop)
- **Step 3: 채널 연결 UI**:
  - 3 채널 카드 (Telegram / Slack / Discord)
  - Telegram: bot token paste + `/start` 명령 발견 확인
  - Slack: OAuth app installation flow (workspace 선택)
  - Discord: bot invite link + Ed25519 public key 입력 + 권한 scope 확인
- WEB-CONFIG-001 의 entry 인터페이스 정의 (`mink config web` → 동일 페이지 재진입)
- Playwright E2E (OAuth 콜백 mock + 5 provider validation)

### 2.2 OUT

- 일상 사용 UI (CLI/TUI 또는 채널 전담)
- 채널 어댑터 자체 구현 (MSG-SLACK-001 / MSG-DISCORD-001 책임)
- 다중 사용자 / 권한 분리

## 3. EARS Requirements

### 3.1 Ubiquitous (7)

- **REQ-ONB5-001 [P0]**: The Web Step 2 UI **shall** display exactly 5 provider cards in the order: Anthropic / DeepSeek / OpenAI / Codex / z.ai GLM
- **REQ-ONB5-002 [P0]**: Each provider card **shall** display authentication mode (key paste / OAuth) + credential status indicator
- **REQ-ONB5-003 [P0]**: All credential save operations **shall** delegate to AUTH-CREDENTIAL-001 (no plaintext in Web layer)
- **REQ-ONB5-004 [P0]**: All credential validation **shall** invoke LLM-ROUTING-V2-AMEND-001 provider test endpoint (no direct API call from Web)
- **REQ-ONB5-005 [P0]**: All new .go and .tsx files **shall** carry AGPL-3.0 header (ADR-002 정합)
- **REQ-ONB5-006 [P1]**: The Web Step 3 UI **shall** display exactly 3 channel cards (Telegram / Slack / Discord)
- **REQ-ONB5-007 [P1]**: All requests **shall** be protected by CSRF + Origin allowlist (ONBOARDING-001 v0.3.1 패턴 재사용)

### 3.2 Event-Driven (8)

- **REQ-ONB5-008 [P0]**: When user clicks "Login with key paste" on Anthropic/DeepSeek/OpenAI/GLM cards, a new browser tab **shall** open to provider API key page (`console.anthropic.com` / `platform.deepseek.com` / `platform.openai.com` / `z.ai/manage-apikey`)
- **REQ-ONB5-009 [P0]**: When user submits paste form, the server **shall** validate format (regex per provider), invoke test API, and store via AUTH on success
- **REQ-ONB5-010 [P0]**: When user clicks "Login with ChatGPT" on Codex card, OAuth PKCE flow **shall** initiate with 127.0.0.1:auto-port callback
- **REQ-ONB5-011 [P0]**: When OAuth callback arrives, server **shall** verify state token, exchange code for tokens, and store refresh_token in AUTH
- **REQ-ONB5-012 [P0]**: When user reorders providers via drag-and-drop, priority **shall** be persisted to LLM-ROUTING-V2-AMEND config
- **REQ-ONB5-013 [P1]**: When user clicks Telegram card, the UI **shall** display bot token input + `/start` discovery instruction
- **REQ-ONB5-014 [P1]**: When user clicks Slack card, the UI **shall** redirect to Slack OAuth app installation URL (workspace selection)
- **REQ-ONB5-015 [P1]**: When user clicks Discord card, the UI **shall** display bot invite link generator + Ed25519 public key input

### 3.3 State-Driven (4)

- **REQ-ONB5-016 [P0]**: While at least 1 provider is successfully validated, the "Next" button **shall** be enabled
- **REQ-ONB5-017 [P0]**: While 0 provider is validated, the "Next" button **shall** be disabled with hint message
- **REQ-ONB5-018 [P1]**: While OAuth flow is in progress (state token pending), the Codex card **shall** show "Waiting for browser..." spinner with 60s timeout
- **REQ-ONB5-019 [P2]**: While in dev mode (`MINK_DEV=1`), the validation **shall** allow mocked credentials

### 3.4 Unwanted (4)

- **REQ-ONB5-020 [P0]**: The Web layer **shall not** log credential values in plaintext (mask with `sk-****` etc.)
- **REQ-ONB5-021 [P0]**: The Web layer **shall not** store credentials in browser localStorage / sessionStorage / cookies
- **REQ-ONB5-022 [P1]**: The Web layer **shall not** allow concurrent provider validation on same provider key
- **REQ-ONB5-023 [P1]**: Step 3 channel connections **shall not** be required for installation completion (Step 3 is optional)

### 3.5 Optional (3)

- **REQ-ONB5-024 [P2, OPT]**: Where user provides custom OpenAI-compat endpoint, an additional "Custom Endpoint" card **shall** be available
- **REQ-ONB5-025 [P2, OPT]**: Where Brand theme variants are configured, the UI **shall** apply theme
- **REQ-ONB5-026 [P2, OPT]**: Where `MINK_ONBOARDING_LANG=ko|en` env is set, UI labels **shall** use specified language

## 4. 마일스톤

- M1: Step 2 UI shell (5 provider cards, key paste flow for 4 providers)
- M2: Codex OAuth PKCE flow (browser callback + state verification)
- M3: Step 3 channel cards (Telegram bot token + Slack OAuth + Discord invite + Ed25519)
- M4: Playwright E2E (5 provider 모킹 + OAuth 콜백 mock + 채널 connection flow) + WEB-CONFIG-001 entry 인터페이스

## 5. 의존

- ONBOARDING-001 v0.3.1 (Web shell + Step1Locale + SessionStore + CSRF)
- LLM-ROUTING-V2-AMEND-001 (5 provider 백엔드, test endpoint)
- AUTH-CREDENTIAL-001 (4 메서드 인터페이스 freeze 필수)
- MSG-SLACK-001 / MSG-DISCORD-001 (Step 3 어댑터 인터페이스)
- MSG-TELEGRAM-001 (v0.1.3 implemented, Telegram bot token 검증 패턴)
- WEB-CONFIG-001 (Web shell 코드베이스 공유)

## 6. 본격 plan 이월 (후속 PR)

- research.md: ONBOARDING v0.3.1 코드 분석 + 5 provider auth pattern + Discord/Slack OAuth flow
- plan.md: 4 마일스톤 + frontend (React 컴포넌트) / backend (Go handler) 분담
- tasks.md: 28~32 task (frontend 18 + backend 10~14)
- acceptance.md: 28 AC (Playwright E2E 12 + integration 10 + unit 6)
- progress.md: 0% / 4 마일스톤 ⏸️
- plan-auditor pass
- **코드 구현 PR (별도, expert-frontend + expert-backend 병렬 spawn)**

## 7. TRUST 5

26 REQ + 28 AC traceable, CSRF + Origin, AGPL 헤더, credential 마스킹, AUTH 위임, OAuth PKCE.
