# Plan — SPEC-MINK-MSG-SLACK-001

AGPL-3.0 헌장. audit 학습 사전 적용 (orphan 0, milestone 경계 0, REQ-SLK-NNN 통일).

## 1. Go 패키지
- `internal/channel/slack/` 신규
  - `handler.go`: Events API POST handler
  - `verify.go`: HMAC-SHA256 signing secret
  - `normalize.go`: BRIDGE canonical schema
  - `oauth.go`: workspace installation flow
  - `blockkit.go`: Block Kit message 생성

## 2. 4 마일스톤

### M1 — HMAC 검증 + Events API + URL verification
- POST handler + HMAC verify + url_verification challenge 응답
- **AC**: SLK-001, 002, 008

### M2 — app_mention / message.im / slash command + BRIDGE
- 3초 ack + chat.postMessage async
- BRIDGE-001 router 위임
- **AC**: SLK-003, 006, 009, 010, 011, 015, 017

### M3 — OAuth installation + AUTH-CREDENTIAL 통합
- Slack OAuth v2 flow
- workspace_id 키로 AUTH-CREDENTIAL Store
- **AC**: SLK-004, 012, 016

### M4 — Interactive components + Rate limit + PII + AGPL
- Block Kit Buttons/Select/Modal payload 처리
- HTTP 429 Retry-After
- PII 마스킹 (MEMORY-QMD redact)
- AGPL 헤더 + DM/public 라우팅 + ephemeral
- **AC**: SLK-005, 007, 013, 014, 018, 019, 020, 021, 022, 023, 024, 025, 026

## 3. 의존 SPEC freeze
- AUTH-CREDENTIAL-001: M3 진입 전 Store/Load/Delete/List
- LLM-ROUTING-V2-AMEND-001: M2 진입 전 Router.Route
- BRIDGE-001: M2 진입 전 canonical schema
- MEMORY-QMD-001: M4 진입 전 (PII 마스킹 옵션)

## 4. 위험 / checklist
| R | 완화 |
|---|---|
| R1: 3초 SLA 초과 | ack 200 즉시 + async post |
| R2: Slack rate limit | Retry-After + backoff max 3 |
| R3: signing secret 노출 | AUTH-CREDENTIAL 위임, mode 0600 |
| R4: OAuth state 위조 | state token + CSRF |

checklist: 26 AC traceable, orphan 0, milestone 경계 0, REQ-SLK-NNN 통일, 의존 SPEC 인터페이스 단일.
