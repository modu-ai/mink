---
id: SPEC-MINK-MSG-SLACK-001
version: 0.1.0
status: draft
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
---

# SPEC-MINK-MSG-SLACK-001 — Slack Bot 채널 어댑터

> **STUB / DRAFT (2026-05-16)**: Sprint 4 진입 표시. 본격 plan 은 별도 PR.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | Sprint 4 stub | MoAI orchestrator |

## 1. 개요

Telegram (MSG-TELEGRAM-001 v0.1.3 implemented) 패턴 위에서 Slack Bot 어댑터를 추가. 3 채널 (Telegram / Slack / Discord) 중 두 번째.

## 2. 스코프

### 2.1 IN

- Slack Bolt SDK (Go) 통합
- Events API webhook (signing secret 검증)
- Slash command (`/mink ...`)
- Interactive components (Block Kit)
- OAuth app installation flow (workspace 단위)
- BRIDGE-001 router 흡수: webhook → normalize → router → LLM → reply formatter → outbound
- AUTH-CREDENTIAL-001 에 signing secret + bot token 저장

### 2.2 OUT

- Slack Enterprise Grid 다중 workspace (단일 workspace MVP)
- Slack Connect (외부 사용자 채널 공유) — 별도 SPEC
- Slack Canvas 통합 — 후속 enhancement

## 3. 의존

- BRIDGE-001 (router 재사용)
- MSG-TELEGRAM-001 (패턴 reference)
- AUTH-CREDENTIAL-001 (signing secret, bot token)
- ONBOARDING-001 Phase 5 Step 3 (Slack OAuth 카드)

## 4. 본격 plan 이월

- research.md, plan.md, tasks.md, acceptance.md, progress.md
- plan-auditor pass
- Slack 공식 sandbox workspace 에서 E2E
