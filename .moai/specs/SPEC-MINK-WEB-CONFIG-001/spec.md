---
id: SPEC-MINK-WEB-CONFIG-001
version: 0.1.0
status: draft
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI orchestrator
priority: medium
phase: 4
size: 중(M)
lifecycle: spec-first
labels: [web, config, settings, ui, sprint-4]
related:
  - SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001
  - SPEC-MINK-LLM-ROUTING-V2-AMEND-001
  - SPEC-MINK-AUTH-CREDENTIAL-001
  - SPEC-MINK-CLI-TUI-003-AMEND-001
---

# SPEC-MINK-WEB-CONFIG-001 — Web UI 설치 후 설정 페이지

> **STUB / DRAFT (2026-05-16)**: Sprint 4 진입 표시. 본격 plan 은 별도 PR.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | Sprint 4 stub — Round 3 사용자 확정 (Web UI 범위 = 설치 + 설정) | MoAI orchestrator |

## 1. 개요

2026-05-16 Round 3 사용자 확정: *"Web UI 의 범위 = 설치 + 설정"*. ONBOARDING-001 Phase 5 가 *설치* 위저드, 본 SPEC 이 *설치 후 설정* 페이지. 일상 운용 (대화) 은 CLI/TUI 또는 채널로 처리.

## 2. 스코프

### 2.1 IN

- **`mink config web`** 명령으로 localhost 설정 페이지 기동 (127.0.0.1:auto-port)
- 설정 카테고리:
  - **Provider**: 5 LLM provider 추가/제거/교체, 우선순위 변경, credential 갱신/revoke
  - **Channel**: 3 채널 (Telegram/Slack/Discord) 연결/해제
  - **Ritual**: journal/briefing/weather 빈도, 시간대, 인사말 tone
  - **Scheduler**: 정기 실행 cron 패턴 수정
  - **Locale / I18N**: 언어·시간대·통화 변경
  - **Memory**: QMD vault 경로, retention 정책, 인덱싱 모델
  - **Privacy**: PII 마스킹 정책, AGPL 동의 갱신, audit log retention
- CLI/TUI 의 `mink config set/get/list` 와 동일 backend (피처 패리티)

### 2.2 OUT

- 대화 인터페이스 (Telegram/Slack/Discord 또는 CLI/TUI)
- journal/ritual 내용 자체 (읽기 전용 표시도 미포함, 향후 enhancement 후보)
- 멀티 사용자 / 권한 분리

## 3. 의존

- ONBOARDING-001 Phase 5 (Web shell, 동일 React + Vite 코드베이스 재사용)
- LLM-ROUTING-V2-AMEND-001 / AUTH-CREDENTIAL-001
- CLI-TUI-003 amendment (동일 backend service layer)
- MEMORY-QMD-001 (vault 경로 설정)

## 4. 본격 plan 이월

- 5 산출물 + expert-frontend + expert-backend spawn
- plan-auditor pass
- Playwright E2E (configuration round-trip)
