---
id: SPEC-MINK-WEB-CONFIG-001
version: 0.2.0
status: planned
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
  - SPEC-MINK-MEMORY-QMD-001
trust_metrics:
  requirements_total: 22
  acceptance_total: 24
  milestones: 4
---

# SPEC-MINK-WEB-CONFIG-001 — Web UI 설치 후 설정 페이지

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | stub (PR #232) | MoAI orchestrator |
| 0.2.0 | 2026-05-16 | 본격 EARS spec 승격 | MoAI orchestrator |

> AGPL-3.0-only 헌장 (ADR-002). Round 3 사용자 확정: *"Web UI 범위 = 설치 + 설정"*.

## 1. 개요

ONBOARDING-001 Phase 5 의 *설치 위저드* 와 분리된 *설치 후 설정* 페이지. 일상 운용은 CLI/TUI 또는 채널 (Telegram/Slack/Discord). Web UI 는 *설치/설정 한정*.

### 1.1 Surface Assumptions

- A1: ONBOARDING Phase 5 의 Web shell 코드베이스 (React + Vite + Tailwind + shadcn/ui) 재사용
- A2: `mink config web` 명령으로 127.0.0.1:auto-port 기동 (사용자 시작/종료 명시)
- A3: 동시 사용자 1명 (multi-user 미지원, CROSSPLAT-001 호환)
- A4: CSRF + Origin allowlist (ONBOARDING-001 패턴 재사용)

## 2. 스코프

### 2.1 IN

- `mink config web` 명령으로 localhost 페이지 기동
- 7 설정 카테고리:
  - **Provider**: 5 LLM provider 추가/제거/교체, 우선순위 변경, credential 갱신/revoke (AUTH-CREDENTIAL-001 위임)
  - **Channel**: 3 채널 (Telegram/Slack/Discord) 연결/해제
  - **Ritual**: journal/briefing/weather 빈도, 시간대, 인사말 tone (JOURNAL/BRIEFING/WEATHER amendment)
  - **Scheduler**: 정기 실행 cron 패턴
  - **Locale / I18N**: 언어·시간대·통화
  - **Memory**: QMD vault 경로, retention 정책, 인덱싱 모델 (MEMORY-QMD-001 위임)
  - **Privacy**: PII 마스킹 정책, AGPL 동의 갱신, audit log retention
- CLI/TUI 의 `mink config set/get/list` 와 동일 backend (피처 패리티, CLI-TUI-003 amend 정합)

### 2.2 OUT

- 대화 인터페이스 (CLI/TUI 또는 채널 전담)
- journal/ritual 내용 자체 표시 (별도 SPEC, post-launch)
- 다중 사용자 / 권한 분리

## 3. EARS Requirements

### 3.1 Ubiquitous (7)

- **REQ-WCF-001 [P0]**: The Web Config server **shall** bind to 127.0.0.1 only (no external interface)
- **REQ-WCF-002 [P0]**: The server **shall** select an unused port (auto-port) when started
- **REQ-WCF-003 [P0]**: Every config mutation request **shall** require CSRF token validation (double-submit pattern, ONBOARDING-001 재사용)
- **REQ-WCF-004 [P0]**: The server **shall** verify `Origin` header against allowlist (`http://127.0.0.1:{port}`)
- **REQ-WCF-005 [P0]**: All config mutations **shall** be persisted to `~/.mink/config/*.yaml` atomically (temp file + rename)
- **REQ-WCF-006 [P1]**: All new .go files **shall** carry SPDX-License-Identifier: AGPL-3.0-only
- **REQ-WCF-007 [P1]**: Every config API **shall** be equivalent to a `mink config set <key> <value>` CLI invocation (CLI-TUI-003 amend 정합)

### 3.2 Event-Driven (6)

- **REQ-WCF-008 [P0]**: When `mink config web` is invoked, the server **shall** start and open default browser to `http://127.0.0.1:{port}/config`
- **REQ-WCF-009 [P0]**: When user navigates away from page (window close / Ctrl-C), the server **shall** shutdown gracefully within 5 seconds
- **REQ-WCF-010 [P0]**: When provider credential is updated, the server **shall** invoke AUTH-CREDENTIAL-001 Store API and verify via test API call
- **REQ-WCF-011 [P1]**: When channel connection is removed, the server **shall** revoke OAuth tokens via channel-specific revoke endpoint
- **REQ-WCF-012 [P1]**: When memory retention policy is changed, the server **shall** queue MEMORY-QMD-001 `prune` operation (non-blocking)
- **REQ-WCF-013 [P2]**: When AGPL consent is updated, the server **shall** record timestamp in audit log

### 3.3 State-Driven (3)

- **REQ-WCF-014 [P0]**: While Web Config server is running, the server **shall** maintain at most 1 active session (multi-user 미지원)
- **REQ-WCF-015 [P1]**: While LLM provider credential validation is in progress (test call), the UI **shall** show "Validating..." spinner
- **REQ-WCF-016 [P2]**: While brand theme is set in onboarding, the Web Config UI **shall** apply identical theme

### 3.4 Unwanted (3)

- **REQ-WCF-017 [P0]**: The server **shall not** bind to 0.0.0.0 or any non-loopback interface
- **REQ-WCF-018 [P0]**: The server **shall not** log credential values in plaintext (mask with `****`)
- **REQ-WCF-019 [P1]**: The server **shall not** allow concurrent mutation requests on same config key (mutex per key)

### 3.5 Optional (3)

- **REQ-WCF-020 [P2, OPT]**: Where `MINK_CONFIG_WEB_PORT=N` env is set, the server **shall** use specified port instead of auto-port
- **REQ-WCF-021 [P2, OPT]**: Where audit log export is requested, the server **shall** download JSON file
- **REQ-WCF-022 [P2, OPT]**: Where dry-run mode is enabled, mutations **shall not** persist (preview only)

## 4. 마일스톤

- M1: Server skeleton + CSRF + Origin allowlist + auto-port + graceful shutdown
- M2: Provider / Channel / Memory 카테고리 (AUTH-CREDENTIAL / MEMORY-QMD 위임)
- M3: Ritual / Scheduler / Locale 카테고리 (JOURNAL/BRIEFING/WEATHER/SCHEDULER amendment 인터페이스)
- M4: Privacy / 옵션 features + E2E (Playwright)

## 5. 의존

- ONBOARDING-001 Phase 5 (Web shell 재사용)
- LLM-ROUTING-V2-AMEND-001 / AUTH-CREDENTIAL-001
- CLI-TUI-003 amend (동일 backend service layer)
- MEMORY-QMD-001 (vault 경로 / retention / 인덱싱 모델)
- MSG-SLACK / MSG-DISCORD / MSG-TELEGRAM (채널 토큰 관리)
- JOURNAL-001 / BRIEFING-001 / WEATHER-001 / RITUAL-001 / SCHEDULER-001 / LOCALE-001 / I18N-001 (각 설정 interface)

## 6. 본격 plan 이월

- research.md: ONBOARDING Phase 5 Web shell 분석, Vite + React + Tailwind 패턴
- plan.md: 4 마일스톤 + 7 카테고리 매트릭스
- tasks.md: 18~22 task (frontend 12 + backend 6~10)
- acceptance.md: 24 AC (Playwright E2E 8 + integration 12 + unit 4)
- progress.md: 0% / 4 마일스톤 ⏸️
- plan-auditor pass

## 7. TRUST 5

22 REQ + 24 AC traceable, CSRF + Origin, AGPL 헤더, 127.0.0.1 only, credential 마스킹.
