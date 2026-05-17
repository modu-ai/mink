---
id: SPEC-MINK-MSG-DISCORD-001
version: 0.2.0
status: planned
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI orchestrator
priority: medium
phase: 4
size: 중(M)
lifecycle: spec-first
labels: [messaging, discord, channel, sprint-4]
related:
  - SPEC-GOOSE-BRIDGE-001
  - SPEC-MINK-MSG-TELEGRAM-001
  - SPEC-MINK-MSG-SLACK-001
  - SPEC-MINK-AUTH-CREDENTIAL-001
  - SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001
trust_metrics:
  requirements_total: 22
  acceptance_total: 24
  milestones: 4
---

# SPEC-MINK-MSG-DISCORD-001 — Discord Bot 채널 어댑터

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | stub (PR #232) | MoAI orchestrator |
| 0.2.0 | 2026-05-16 | 본격 EARS spec 승격 | MoAI orchestrator |

> AGPL-3.0-only 헌장 (ADR-002). 3 채널 중 세 번째.

## 1. 개요

Discord Interactions endpoint (HTTP) + Ed25519 서명 검증 + Slash command + Message component 를 통해 Discord 서버 사용자가 MINK 와 대화. Gateway WebSocket vs HTTP-only 결정 = **HTTP-only default** (운영 단순성).

### 1.1 Surface Assumptions

- A1: Discord 서버 당 단일 bot installation
- A2: 사용자가 Discord Developer Portal 에서 application 등록 책임
- A3: HTTP Interactions endpoint 가 3초 응답 SLA 준수 (Discord 요구)
- A4: Ed25519 public key 가 application 단위로 고정 (rotation 시 사용자 재등록)

## 2. 스코프

### 2.1 IN

- Discord Interactions endpoint (HTTP POST) + Ed25519 signature 검증 (libsodium / `crypto/ed25519`)
- Slash command 등록 (Application Commands API)
- Message component (Buttons / Select Menus / Modals)
- Bot invite flow (OAuth2 scopes: `bot`, `applications.commands`)
- BRIDGE-001 흡수
- AUTH-CREDENTIAL-001 에 Ed25519 public key + bot token 저장
- 3초 응답 SLA: deferred response 활용 (type 5 = `DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE`)

### 2.2 OUT

- Gateway WebSocket connection (presence / typing indicator 같은 실시간 기능 — post-launch)
- Voice channel / Stage channel
- Forum / Thread 자동 분류
- Server boosting / Subscription 통합

## 3. EARS Requirements

### 3.1 Ubiquitous (6)

- **REQ-DCD-001 [P0]**: The MINK Discord adapter **shall** verify all Interactions endpoint requests via Ed25519 signature with `X-Signature-Ed25519` and `X-Signature-Timestamp` headers
- **REQ-DCD-002 [P0]**: The adapter **shall** respond to Interactions endpoint with HTTP 200 within 3 seconds
- **REQ-DCD-003 [P0]**: The adapter **shall** delegate LLM call to LLM-ROUTING-V2-AMEND-001
- **REQ-DCD-004 [P0]**: The adapter **shall** retrieve Ed25519 public key + bot token from AUTH-CREDENTIAL-001
- **REQ-DCD-005 [P1]**: All adapter .go files **shall** carry SPDX-License-Identifier: AGPL-3.0-only header
- **REQ-DCD-006 [P1]**: The adapter **shall** normalize Discord message to BRIDGE-001 canonical schema

### 3.2 Event-Driven (6)

- **REQ-DCD-007 [P0]**: When `PING` interaction type (1) arrives, the adapter **shall** respond with `PONG` (type 1) within 3 seconds
- **REQ-DCD-008 [P0]**: When `APPLICATION_COMMAND` interaction type (2) arrives, the adapter **shall** respond with `DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE` (type 5) within 3 seconds, then post follow-up
- **REQ-DCD-009 [P0]**: When `MESSAGE_COMPONENT` interaction type (3) arrives, the adapter **shall** parse component custom_id and dispatch
- **REQ-DCD-010 [P1]**: When bot is invited to a server, the adapter **shall** register slash commands via `applications.{app_id}.commands` API
- **REQ-DCD-011 [P1]**: When Ed25519 signature verification fails, the adapter **shall** respond HTTP 401 (not 500)
- **REQ-DCD-012 [P2]**: When rate limit response (HTTP 429) is received, the adapter **shall** honor `Retry-After` header

### 3.3 State-Driven (3)

- **REQ-DCD-013 [P0]**: While LLM call is in progress (after deferred response), the adapter **shall** use webhook follow-up endpoint to post final response
- **REQ-DCD-014 [P1]**: While in DM channel, the adapter **shall not** require slash command to respond
- **REQ-DCD-015 [P1]**: While in server channel, the adapter **shall** only respond to explicit slash command or @mention

### 3.4 Unwanted (4)

- **REQ-DCD-016 [P0]**: The adapter **shall not** start Gateway WebSocket connection by default (HTTP-only)
- **REQ-DCD-017 [P0]**: The adapter **shall not** store raw Discord message in plaintext (PII 마스킹 적용)
- **REQ-DCD-018 [P1]**: The adapter **shall not** post messages without user interaction trigger
- **REQ-DCD-019 [P1]**: The adapter **shall not** retry failed webhook follow-up more than 3 times

### 3.4.5 Additional Ubiquitous (audit D2 fix — Bot invite EARS REQ 신규)

- **REQ-DCD-023 [P1]**: When bot invite link is requested, the adapter **shall** generate URL with OAuth2 scopes (`bot` + `applications.commands`) and explicit permission bits

### 3.5 Optional (3)

- **REQ-DCD-020 [P2, OPT]**: Where `MINK_DISCORD_GATEWAY=1` env is set, the adapter **shall** connect to Gateway WebSocket (real-time features)
- **REQ-DCD-021 [P2, OPT]**: Where Discord ephemeral flag is requested (`/mink-private`), response **shall** use ephemeral message (flag 64)
- **REQ-DCD-022 [P2, OPT]**: Where `MINK_DISCORD_ALLOWED_GUILDS` env is set, the adapter **shall** ignore interactions from other servers

## 4. 마일스톤

- M1: Ed25519 signature 검증 + PING/PONG + URL verification
- M2: APPLICATION_COMMAND + Slash command 등록 + deferred response
- M3: MESSAGE_COMPONENT + Buttons/Select Menus
- M4: Bot invite flow + AUTH-CREDENTIAL 통합 + 3초 SLA 검증

## 5. 의존

- BRIDGE-001 (router)
- MSG-TELEGRAM-001 / MSG-SLACK-001 (패턴)
- LLM-ROUTING-V2-AMEND-001
- AUTH-CREDENTIAL-001 (Ed25519 public key, bot token)
- MEMORY-QMD-001 (옵션 PII redact)
- ONBOARDING-001 Phase 5 Step 3

## 6. 본격 plan 이월

- research.md: Discord API matrix, Interactions vs Gateway, Ed25519 verification 패턴
- plan.md: 4 마일스톤
- tasks.md: 16~18 task
- acceptance.md: 24 AC
- progress.md: 0% / 4 마일스톤 ⏸️
- plan-auditor pass

## 7. TRUST 5

22 REQ + 24 AC traceable, Ed25519, AGPL 헤더, 3초 SLA, PII 마스킹.
