# Plan — SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001

AGPL-3.0. ONBOARDING v0.3.1 위 amendment. audit 학습 사전 적용.

## 1. 코드 변경
- `internal/server/install/handler.go` 확장: 5 provider save + OAuth callback handler
- `internal/server/install/oauth.go` 신규: PKCE flow (Codex)
- `web/install/src/Step2Provider.tsx` 신규: 5 provider 카드 + 인증 UI
- `web/install/src/Step3Channel.tsx` 신규: 3 채널 카드
- `web/install/src/hooks/useProviderAuth.ts` 신규: OAuth callback 처리

## 2. 4 마일스톤

### M1 — Step 2 UI shell + 4 provider key paste
- 5 provider 카드 (Anthropic/DeepSeek/OpenAI/Codex/GLM)
- 4 provider key paste UI (Codex 제외)
- AUTH-CREDENTIAL Store/validate 위임
- **AC**: OB5-001, 002, 003, 004, 008, 009, 016, 017, 020, 021

### M2 — Codex OAuth PKCE flow
- 127.0.0.1:auto-port callback
- state token CSRF
- token exchange + refresh_token Store
- **AC**: OB5-010, 011, 018

### M3 — Step 3 채널 연결
- Telegram bot token + `/start` ping
- Slack OAuth v2 redirect
- Discord bot invite + Ed25519 key 입력
- **AC**: OB5-006, 013, 014, 015, 023

### M4 — drag-drop 우선순위 + Playwright E2E + WEB-CONFIG entry + AGPL
- provider priority drag-and-drop → LLM-ROUTING config
- 5 provider validation E2E (mocked)
- OAuth callback mock
- WEB-CONFIG entry 인터페이스 정의
- AGPL 헤더 신규 .go/.tsx + lint CI
- **AC**: OB5-005, 007, 012, 019, 022, 024, 025, 026, 027, 028

## 3. 의존 freeze
- LLM-ROUTING-V2-AMEND-001 (M1 진입 전): Router.Test(provider) + 5 provider client
- AUTH-CREDENTIAL-001 (M1 진입 전): Store/Load/Delete/List
- MSG-SLACK/DISCORD/TELEGRAM (M3 진입 전): channel connection API
- WEB-CONFIG-001 (M4 진입 전): entry 인터페이스
- ONBOARDING-001 v0.3.1 (source codebase, 변경 0)

## 4. 위험
| R | 완화 |
|---|---|
| R1: OAuth callback port 충돌 | auto-port + retry |
| R2: state token replay | timestamp + nonce + single-use |
| R3: 5 provider 동시 진입 | per-provider mutex |
| R4: ONBOARDING 코드베이스 drift | shared 패턴 + 회귀 |

checklist: 26 REQ + 28 AC + 4 milestones + 28 tasks. orphan 0, milestone 경계 0, REQ-ONB5-NNN 통일.
