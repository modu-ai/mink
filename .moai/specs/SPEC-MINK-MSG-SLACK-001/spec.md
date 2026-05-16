---
id: SPEC-MINK-MSG-SLACK-001
version: 0.2.0
status: planned
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI orchestrator
priority: medium
phase: 4
size: 중(M)
lifecycle: spec-first
labels: [messaging, slack, channel, sprint-4]
related:
  - SPEC-GOOSE-BRIDGE-001
  - SPEC-MINK-MSG-TELEGRAM-001
  - SPEC-MINK-AUTH-CREDENTIAL-001
  - SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001
trust_metrics:
  requirements_total: 24
  acceptance_total: 26
  milestones: 4
---

# SPEC-MINK-MSG-SLACK-001 — Slack Bot 채널 어댑터

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | stub (PR #232) | MoAI orchestrator |
| 0.2.0 | 2026-05-16 | 본격 EARS spec 승격. 4 추가 산출물 후속 PR 이월 | MoAI orchestrator |

> AGPL-3.0-only 헌장 (ADR-002) 위에서 작성. MSG-TELEGRAM-001 v0.1.3 implemented 패턴 재사용.

## 1. 개요

3 채널 (Telegram / Slack / Discord) 중 두 번째. Slack Bolt-Go SDK 또는 native Block Kit + Events API webhook + signing secret 검증을 통해 워크스페이스 사용자가 MINK 와 대화.

### 1.1 Surface Assumptions

- A1: Slack workspace 당 단일 bot installation (multi-workspace 는 별도 SPEC)
- A2: 사용자가 Slack OAuth app 발급 책임 (사용자 워크스페이스 admin 권한 필요)
- A3: BRIDGE-001 router 가 Slack 메시지 normalize 형식 수용
- A4: Slack Events API 의 3초 응답 SLA 준수 (LLM 응답 비동기, ack 먼저)

## 2. 스코프

### 2.1 IN

- Slack Events API webhook 수신 + signing secret HMAC 검증
- Slash command `/mink`, `/mink-memory`, `/mink-briefing`
- Interactive components (Buttons / Select Menus / Modals via Block Kit)
- OAuth app installation (Slack OAuth 2.0 v2 flow)
- BRIDGE-001 흡수: webhook → normalize → router → LLM → reply
- AUTH-CREDENTIAL-001 에 signing secret + bot token + app-level token 저장
- 3초 응답 SLA: ack 200 OK 즉시, LLM 응답은 `chat.postMessage` 후속

### 2.2 OUT

- Slack Enterprise Grid (별도 SPEC, post-launch)
- Slack Connect (외부 워크스페이스 공유)
- Slack Canvas / Huddle / Workflow Builder 통합
- Voice 메시지 처리

## 3. EARS Requirements

### 3.1 Ubiquitous (7)

- **REQ-SLK-001 [P0]**: The MINK Slack adapter **shall** verify all incoming Events API requests via HMAC-SHA256 with workspace signing secret
- **REQ-SLK-002 [P0]**: The adapter **shall** respond to Events API webhook with HTTP 200 within 3 seconds (Slack SLA)
- **REQ-SLK-003 [P0]**: The adapter **shall** delegate LLM call to LLM-ROUTING-V2-AMEND-001 router (no direct provider call)
- **REQ-SLK-004 [P0]**: The adapter **shall** retrieve bot token and signing secret from AUTH-CREDENTIAL-001 (no plaintext in config)
- **REQ-SLK-005 [P1]**: All adapter .go files **shall** carry SPDX-License-Identifier: AGPL-3.0-only header
- **REQ-SLK-006 [P1]**: The adapter **shall** normalize Slack message to BRIDGE-001 canonical schema (user_id, channel_id, text, timestamp, thread_ts)
- **REQ-SLK-007 [P2]**: The adapter **shall** log inbound webhook payload (sanitized, PII-redacted) to MEMORY-QMD-001 sessions/ collection (opt-in)

### 3.2 Event-Driven (7)

- **REQ-SLK-008 [P0]**: When Slack Events API URL verification challenge arrives, the adapter **shall** respond with the challenge value within 3 seconds
- **REQ-SLK-009 [P0]**: When `app_mention` event arrives, the adapter **shall** trigger LLM routing with the mention text
- **REQ-SLK-010 [P0]**: When `message.im` event arrives (DM), the adapter **shall** treat as private conversation context
- **REQ-SLK-011 [P0]**: When `/mink <prompt>` slash command arrives, the adapter **shall** acknowledge within 3 seconds and post response via `chat.postMessage`
- **REQ-SLK-012 [P1]**: When OAuth installation completes, the adapter **shall** store workspace credentials in AUTH-CREDENTIAL-001 with workspace_id key
- **REQ-SLK-013 [P1]**: When interactive component (Button / Select Menu) is triggered, the adapter **shall** parse payload and route to corresponding service action
- **REQ-SLK-014 [P2]**: When rate limit response (HTTP 429) is received from Slack, the adapter **shall** honor `Retry-After` header

### 3.3 State-Driven (4)

- **REQ-SLK-015 [P0]**: While LLM call is in progress, the adapter **shall** post "MINK is thinking..." placeholder (chat.postMessage) and update via `chat.update` on completion
- **REQ-SLK-016 [P1]**: While in DM channel, the adapter **shall not** require @mention to respond
- **REQ-SLK-017 [P1]**: While in public channel, the adapter **shall** require @mention or slash command to respond
- **REQ-SLK-018 [P2]**: While in thread context (thread_ts set), reply **shall** be posted in same thread

### 3.4 Unwanted (4)

- **REQ-SLK-019 [P0]**: The adapter **shall not** post messages to channels without explicit user trigger (no proactive spam)
- **REQ-SLK-020 [P0]**: The adapter **shall not** store raw Slack message in plaintext (PII 마스킹 적용, MEMORY-QMD-001 redact pipeline 위임)
- **REQ-SLK-021 [P1]**: The adapter **shall not** call Slack Web API in synchronous response path (ack first, post async)
- **REQ-SLK-022 [P1]**: The adapter **shall not** retry failed `chat.postMessage` more than 3 times (exponential backoff)

### 3.5 Optional (2)

- **REQ-SLK-023 [P2, OPT]**: Where workspace admin enables "MINK memory share", channel history **shall** be queryable via `/mink-memory search <query>`
- **REQ-SLK-024 [P2, OPT]**: Where `MINK_SLACK_ALLOWED_CHANNELS` env is set, the adapter **shall** ignore events from other channels

## 4. 마일스톤

- M1: HMAC 검증 + Events API webhook + URL verification challenge
- M2: app_mention / message.im / slash command 처리 + BRIDGE-001 흡수
- M3: OAuth installation flow + AUTH-CREDENTIAL-001 통합
- M4: Interactive components + Block Kit + 3초 SLA 검증

## 5. 의존

- BRIDGE-001 (router, normalize schema)
- MSG-TELEGRAM-001 (패턴 reference)
- LLM-ROUTING-V2-AMEND-001 (LLM 호출)
- AUTH-CREDENTIAL-001 (signing secret, bot token)
- MEMORY-QMD-001 (옵션 session 색인, PII redact pipeline)
- ONBOARDING-001 Phase 5 Step 3 (Slack OAuth 카드 UI)

## 6. 본격 plan 이월 (후속 PR)

- research.md: Slack API matrix, Bolt SDK vs native, 3초 SLA 측정, Events API vs Socket Mode
- plan.md: 4 마일스톤 상세 + Go 패키지 (`internal/channel/slack/`)
- tasks.md: 18~20 task
- acceptance.md: 26 AC (Given-When-Then, REQ↔AC 1:N traceable)
- progress.md: 0% / 4 마일스톤 ⏸️
- plan-auditor pass

## 7. TRUST 5

24 REQ + 26 AC traceable, AGPL-3.0 헤더, PII 마스킹, 3초 SLA, AUTH 위임.
