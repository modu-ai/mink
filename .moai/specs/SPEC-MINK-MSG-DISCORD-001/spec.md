---
id: SPEC-MINK-MSG-DISCORD-001
version: 0.1.0
status: draft
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
  - SPEC-MINK-AUTH-CREDENTIAL-001
  - SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001
---

# SPEC-MINK-MSG-DISCORD-001 — Discord Bot 채널 어댑터

> **STUB / DRAFT (2026-05-16)**: Sprint 4 진입 표시. 본격 plan 은 별도 PR.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | Sprint 4 stub | MoAI orchestrator |

## 1. 개요

Telegram / Slack 패턴 위에서 Discord Bot 어댑터를 추가. 3 채널 중 세 번째.

## 2. 스코프

### 2.1 IN

- Discord Interactions endpoint (HTTP) — Ed25519 서명 검증
- Slash command 등록
- Message component (Buttons / Select Menus)
- Bot invite flow (OAuth2 scopes: bot, applications.commands)
- Gateway WebSocket vs HTTP-only 결정 (default HTTP-only)
- BRIDGE-001 router 흡수
- AUTH-CREDENTIAL-001 에 Ed25519 public key + bot token 저장

### 2.2 OUT

- Voice channel 통합
- Stage channel
- Forum / Thread 자동 분류

## 3. 의존

- BRIDGE-001
- MSG-TELEGRAM-001 / MSG-SLACK-001 (패턴)
- AUTH-CREDENTIAL-001
- ONBOARDING-001 Phase 5 Step 3

## 4. 본격 plan 이월

- 5 산출물 + plan-auditor pass
- Discord 공식 sandbox bot 으로 E2E
