# Tasks — SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001

28 tasks, 4 마일스톤.

## §0 패키지 매핑
- `internal/server/install/handler.go`: T-001~T-006 (확장)
- `internal/server/install/oauth.go`: T-007~T-011 (신규)
- `web/install/src/Step2Provider.tsx`: T-012~T-017
- `web/install/src/Step3Channel.tsx`: T-018~T-022
- `web/install/src/hooks/useProviderAuth.ts`: T-023, T-024
- E2E / 통합: T-025~T-028

## §1 M1 (10 task)
- T-001: 5 provider card model (handler)
- T-002: POST `/install/provider/save` (Anthropic key paste)
- T-003: POST `/install/provider/save` (DeepSeek)
- T-004: POST `/install/provider/save` (OpenAI key)
- T-005: POST `/install/provider/save` (GLM)
- T-006: validation status indicator API
- T-012: Step2Provider.tsx 5 카드 렌더링 (AC OB5-001, 002)
- T-013: provider 새 탭 열기 + key paste form (AC OB5-008, 009)
- T-014: validation spinner + 성공/실패 indicator (AC OB5-016, 017)
- T-015: AUTH 위임 + 평문 0 (AC OB5-003, 020, 021)

## §2 M2 (5 task)
- T-007: OAuth PKCE code_verifier + challenge
- T-008: state token CSRF
- T-009: 127.0.0.1:auto-port callback handler
- T-010: token exchange + refresh_token Store
- T-011: Codex 카드 OAuth 흐름 + spinner timeout 60s

## §3 M3 (5 task)
- T-018: Step3Channel.tsx 3 카드
- T-019: Telegram bot token paste + `/start` ping
- T-020: Slack OAuth v2 redirect + callback
- T-021: Discord bot invite + Ed25519 입력
- T-022: 채널 connection optional (Step 3 skip 허용)

## §4 M4 (8 task)
- T-023: useProviderAuth hook + OAuth callback wiring
- T-024: provider priority drag-and-drop → LLM-ROUTING config 저장
- T-025: Playwright E2E (5 provider validation mock)
- T-026: Playwright E2E (OAuth callback mock)
- T-027: Playwright E2E (3 channel connection)
- T-028: WEB-CONFIG entry 인터페이스 + AGPL SPDX 헤더 신규 파일 + lint CI gate

## §5 task↔AC↔REQ (orphan 0)

| task | M | 핵심 AC | 핵심 REQ |
|---|---|---|---|
| T-001 | M1 | OB5-001 | REQ-ONB5-001 |
| T-002 | M1 | OB5-008 (Anthropic) | REQ-ONB5-008 |
| T-003 | M1 | OB5-008 (DeepSeek) | REQ-ONB5-008 |
| T-004 | M1 | OB5-008 (OpenAI) | REQ-ONB5-008 |
| T-005 | M1 | OB5-008 (GLM) | REQ-ONB5-008 |
| T-006 | M1 | OB5-009 | REQ-ONB5-009 |
| T-012 | M1 | OB5-001, 002 | REQ-ONB5-001, 002 |
| T-013 | M1 | OB5-008 (UI), 009 | REQ-ONB5-008, 009 |
| T-014 | M1 | OB5-016, 017 | REQ-ONB5-016, 017 |
| T-015 | M1 | OB5-003, 020, 021 | REQ-ONB5-003, 020, 021 |
| T-007 | M2 | OB5-010 (PKCE) | REQ-ONB5-010 |
| T-008 | M2 | OB5-010 (state) | REQ-ONB5-010 |
| T-009 | M2 | OB5-010 (callback) | REQ-ONB5-010 |
| T-010 | M2 | OB5-011 | REQ-ONB5-011 |
| T-011 | M2 | OB5-018 | REQ-ONB5-018 |
| T-018 | M3 | OB5-006 | REQ-ONB5-006 |
| T-019 | M3 | OB5-013 | REQ-ONB5-013 |
| T-020 | M3 | OB5-014 | REQ-ONB5-014 |
| T-021 | M3 | OB5-015 | REQ-ONB5-015 |
| T-022 | M3 | OB5-023 | REQ-ONB5-023 |
| T-023 | M4 | OB5-019 | REQ-ONB5-019 |
| T-024 | M4 | OB5-012 | REQ-ONB5-012 |
| T-025 | M4 | OB5-022, 024 | REQ-ONB5-024 |
| T-026 | M4 | OB5-022 보강 | REQ-ONB5-010 (E2E) |
| T-027 | M4 | OB5-007 | REQ-ONB5-007 |
| T-028 | M4 | OB5-005, 025, 026, 027, 028 | REQ-ONB5-005 |

각 AC ≥1 task, 각 REQ ≥1 task. orphan 0.
