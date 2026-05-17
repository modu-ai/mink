# Plan — SPEC-MINK-MSG-DISCORD-001

AGPL-3.0-only 헌장.

## 1. Go 패키지

- `internal/channel/discord/` (신규)
  - `handler.go`: HTTP Interactions endpoint
  - `verify.go`: Ed25519 서명 검증
  - `normalize.go`: BRIDGE-001 canonical schema 변환
  - `register.go`: slash command 등록 (applications.{app_id}.commands)
  - `webhook.go`: follow-up message 발송
- `internal/channel/discord/client.go`: HTTP client (REST API)

## 2. 4 마일스톤

### M1 — Ed25519 verify + PING/PONG (audit P0 D 학습)

- HTTP Interactions endpoint 수립 (POST handler)
- Ed25519 verify (crypto/ed25519)
- PING interaction (type 1) → PONG (type 1) 응답
- URL verification challenge
- **책임 AC**: AC-DCD-001, 002, 007, 011

### M2 — APPLICATION_COMMAND + slash command 등록 + deferred response

- type 5 (DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE) 응답 즉시
- LLM 호출 후 webhook follow-up 으로 final response
- slash command 등록 (applications.{app_id}.commands)
- BRIDGE-001 router 위임
- **책임 AC**: AC-DCD-003, 008, 010, 013, 015

### M3 — MESSAGE_COMPONENT + Buttons/Select Menus

- type 3 (MESSAGE_COMPONENT) parsing
- custom_id dispatch
- Button / Select Menu / Modal 처리
- **책임 AC**: AC-DCD-009, 014, 022

### M4 — Bot invite + AUTH-CREDENTIAL + 3초 SLA 검증

- Bot invite link 생성 (OAuth2 scopes: bot, applications.commands)
- AUTH-CREDENTIAL-001 위임 (Ed25519 public key + bot token 저장)
- 3초 SLA 측정 + monitor
- PII 마스킹 (MEMORY-QMD-001 redact pipeline 위임)
- **책임 AC**: AC-DCD-004, 005, 006, 012, 016, 017, 018, 019, 020, 021, 023, 024

## 3. 의존 SPEC freeze 시점

- AUTH-CREDENTIAL-001: M4 진입 전 `AuthCredentialService` (Store/Load/Delete/List) freeze
- LLM-ROUTING-V2-AMEND-001: M2 진입 전 `Router.Route` freeze
- BRIDGE-001: M2 진입 전 canonical schema freeze
- MEMORY-QMD-001: M4 진입 전 redact pipeline interface freeze (Optional, PII 마스킹 활성 시)

## 4. 위험

| R | 설명 | 완화 |
|---|---|---|
| R1 | Ed25519 public key rotation 시 사용자 재등록 필요 | onboarding flow + warning 메시지 |
| R2 | Discord 3초 SLA 초과 시 interaction 실패 | type 5 deferred response 우선 사용 |
| R3 | Rate limit (HTTP 429) | Retry-After 헤더 준수 + exponential backoff (max 3) |
| R4 | LLM 응답 5MB 초과 | Discord 메시지 분할 또는 attachment 변환 |

## 5. checklist (audit B/C 학습 적용)

- Surface Assumptions A1~A4 ↔ Risk R1~R4 cross-read
- 카운트 정합: 22 REQ + 24 AC + 4 milestones + 18 tasks
- 의존 SPEC 인터페이스 명세 단일화 (audit D4 학습)
- 모든 AC 가 task ↔ AC ↔ REQ 매트릭스에서 책임 task 보유 (orphan 0, audit D1 학습)
- milestone 경계 위반 0 (M3 task 가 M2 AC 책임 0, audit D3 학습)
