# Research — SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001

AGPL-3.0 헌장. ONBOARDING-001 v0.3.1 implemented 위 Phase 5 amendment.

## 1. ONBOARDING v0.3.1 코드베이스 (재사용)
- `internal/server/install/`: handler/server/embed (CSRF + Origin + SessionStore + Go 1.22 ServeMux + JSON envelope)
- `web/install/`: Vite 6 + React 19 + TS strict + Tailwind 3 + shadcn/ui + Step1Locale 완성
- Step 2~7 placeholder 잔존 (본 amendment 의 wiring 대상)

## 2. 5 Provider 인증 흐름
- **Anthropic Claude**: console.anthropic.com → key paste (regex `^sk-ant-[A-Za-z0-9-_]+$`)
- **DeepSeek**: platform.deepseek.com → key paste
- **OpenAI GPT (API)**: platform.openai.com → key paste (regex `^sk-[A-Za-z0-9]{32,}$`)
- **Codex (ChatGPT OAuth)**: OAuth 2.1 PKCE + 127.0.0.1:auto-port browser callback + device-code fallback
- **z.ai GLM-5-Turbo**: z.ai/manage-apikey → key paste

## 3. OAuth PKCE 흐름 (Codex)
- code_verifier (random 64 bytes base64url) → code_challenge (SHA-256 + base64url)
- state token (CSRF 방지)
- 127.0.0.1:auto-port callback handler
- token endpoint POST → access_token + refresh_token + expires_in
- AUTH-CREDENTIAL-001 Store

## 4. 3 채널 연결 흐름
- **Telegram**: bot token paste (BotFather) + `/start` 발견 ping
- **Slack**: OAuth v2 (`/oauth/v2/authorize` redirect + callback)
- **Discord**: bot invite link 생성 + Ed25519 public key 입력

## 5. WEB-CONFIG-001 entry 인터페이스
- 본 Phase 5 가 *설치 위저드*, WEB-CONFIG-001 가 *설치 후 설정*
- 코드베이스 공유: `internal/server/{install,config}` 패키지 분리, `web/{install,config}` UI 분리

## 6. 의존 인터페이스 freeze
- LLM-ROUTING-V2-AMEND-001: 5 provider client + test endpoint (Router.Test(provider))
- AUTH-CREDENTIAL-001: Store/Load/Delete/List 4 메서드
- MSG-SLACK-001 / MSG-DISCORD-001 / MSG-TELEGRAM-001: 채널 connection API

## 7. 참조
- ONBOARDING v0.3.1 (PR #211~#217)
- OAuth 2.1 PKCE RFC 9700
- Slack OAuth v2 / Discord bot invite / Telegram Bot API
- LLM-ROUTING-V2-AMEND-001 / AUTH-CREDENTIAL-001 / MSG-* / WEB-CONFIG-001
- ADR-002 (AGPL)
