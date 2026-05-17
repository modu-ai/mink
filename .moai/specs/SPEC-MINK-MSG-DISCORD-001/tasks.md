# Tasks — SPEC-MINK-MSG-DISCORD-001

18 tasks, 4 마일스톤.

## §0 패키지 매핑

| 패키지 | tasks |
|---|---|
| `internal/channel/discord/handler.go` | T-001, T-002, T-008 |
| `internal/channel/discord/verify.go` | T-003 |
| `internal/channel/discord/normalize.go` | T-004 |
| `internal/channel/discord/register.go` | T-007 |
| `internal/channel/discord/webhook.go` | T-006 |
| `internal/channel/discord/client.go` | T-005 |
| `internal/channel/discord/component.go` | T-010, T-011 |
| AUTH-CREDENTIAL-001 위임 | T-013 |
| MEMORY-QMD-001 redact | T-015 |
| BRIDGE-001 router | T-009 |
| Bot invite UI | T-012 |

## §1 M1 — Ed25519 verify + PING/PONG

- **T-001**: HTTP Interactions endpoint POST handler 골격 (책임 AC: AC-DCD-002 — 3초 SLA)
- **T-002**: PING interaction (type 1) → PONG 응답 (책임 AC: AC-DCD-007)
- **T-003**: Ed25519 서명 검증 helper (책임 AC: AC-DCD-001, AC-DCD-011)
- **T-004**: BRIDGE-001 canonical schema normalize (책임 AC: AC-DCD-006)

## §2 M2 — APPLICATION_COMMAND + slash + deferred

- **T-005**: HTTP client (Discord REST) + auth header (책임 AC: AC-DCD-010 webhook follow-up)
- **T-006**: webhook follow-up endpoint (책임 AC: AC-DCD-013)
- **T-007**: slash command 등록 API (책임 AC: AC-DCD-010 + 등록)
- **T-008**: APPLICATION_COMMAND type 2 처리 + type 5 deferred 응답 (책임 AC: AC-DCD-008)
- **T-009**: BRIDGE-001 router 호출 + LLM-ROUTING-V2-AMEND 위임 (책임 AC: AC-DCD-003, AC-DCD-015)

## §3 M3 — MESSAGE_COMPONENT + Buttons/Select

- **T-010**: MESSAGE_COMPONENT type 3 parsing + custom_id dispatch (책임 AC: AC-DCD-009)
- **T-011**: Button / Select Menu / Modal handler (책임 AC: AC-DCD-014, AC-DCD-022)

## §4 M4 — Bot invite + AUTH + 3초 SLA + PII

- **T-012**: Bot invite link 생성 + OAuth2 scopes (책임 AC: AC-DCD-016)
- **T-013**: AUTH-CREDENTIAL-001 Store/Load 위임 (Ed25519 public key + bot token) (책임 AC: AC-DCD-004)
- **T-014**: 3초 SLA 측정 + monitor (책임 AC: AC-DCD-002 보강 + AC-DCD-023)
- **T-015**: PII 마스킹 (MEMORY-QMD-001 redact pipeline 위임) (책임 AC: AC-DCD-017, AC-DCD-024)
- **T-016**: Rate limit (HTTP 429) Retry-After 핸들링 + exponential backoff max 3 (책임 AC: AC-DCD-012, AC-DCD-019)
- **T-017**: AGPL SPDX 헤더 신규 .go 일괄 + DM vs server 라우팅 (책임 AC: AC-DCD-005, AC-DCD-018, AC-DCD-020, AC-DCD-021)
- **T-018**: Gateway WebSocket env opt-in (`MINK_DISCORD_GATEWAY=1`), MINK_DISCORD_ALLOWED_GUILDS env (책임 AC: AC-DCD-020 (post-launch))

## §5 task ↔ AC ↔ REQ 매트릭스 (audit D1/D3/D6 학습 — orphan 0 + milestone 경계 0 + REQ-DCD-NNN 통일)

| task | M | 핵심 AC | 핵심 REQ |
|---|---|---|---|
| T-001 | M1 | AC-DCD-002 | REQ-DCD-002 |
| T-002 | M1 | AC-DCD-007 | REQ-DCD-007 |
| T-003 | M1 | AC-DCD-001, 011 | REQ-DCD-001, 011 |
| T-004 | M1 | AC-DCD-006 | REQ-DCD-006 |
| T-005 | M2 | AC-DCD-010 | REQ-DCD-010 |
| T-006 | M2 | AC-DCD-013 | REQ-DCD-013 |
| T-007 | M2 | AC-DCD-010 (보강) | REQ-DCD-010 |
| T-008 | M2 | AC-DCD-008 | REQ-DCD-008 |
| T-009 | M2 | AC-DCD-003, 015 | REQ-DCD-003, 015 |
| T-010 | M3 | AC-DCD-009 | REQ-DCD-009 |
| T-011 | M3 | AC-DCD-014, 022 | REQ-DCD-014, 022 |
| T-012 | M4 | AC-DCD-016 | REQ-DCD-016 (보강) |
| T-013 | M4 | AC-DCD-004 | REQ-DCD-004 |
| T-014 | M4 | AC-DCD-002 (보강), 023 | REQ-DCD-002, 023 |
| T-015 | M4 | AC-DCD-017, 024 | REQ-DCD-017 (UN PII), 024 (lint 보강) |
| T-016 | M4 | AC-DCD-012, 019 | REQ-DCD-012, 019 |
| T-017 | M4 | AC-DCD-005, 018, 020, 021 | REQ-DCD-005, 015 (server), 018, 021 |
| T-018 | M4 | AC-DCD-020 (post-launch boost) | REQ-DCD-020 |

각 AC ≥1 task GREEN. 각 REQ ≥1 task 처리. orphan AC 0. milestone 경계 위반 0.
