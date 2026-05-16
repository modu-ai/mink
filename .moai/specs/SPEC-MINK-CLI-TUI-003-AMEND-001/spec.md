---
id: SPEC-MINK-CLI-TUI-003-AMEND-001
version: 0.2.0
status: planned
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI orchestrator
priority: medium
phase: 3
size: 중(M)
lifecycle: spec-first
labels: [cli, tui, parity, audit, sprint-3, amendment]
amends: [SPEC-GOOSE-CLI-TUI-003]
related:
  - ADR-002 (AGPL-3.0)
  - SPEC-MINK-LLM-ROUTING-V2-AMEND-001
  - SPEC-MINK-AUTH-CREDENTIAL-001
  - SPEC-MINK-MEMORY-QMD-001
trust_metrics:
  requirements_total: 22
  acceptance_total: 24
  milestones: 3
---

# SPEC-MINK-CLI-TUI-003-AMEND-001 — CLI = TUI 피처 패리티 감사 + 용어 통일

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | stub (PR #232) | MoAI orchestrator |
| 0.2.0 | 2026-05-16 | 본격 EARS spec 승격. 추가 산출물 (research/plan/tasks/acceptance/progress) 은 후속 PR 이월 | MoAI orchestrator |

> 본 SPEC 은 AGPL-3.0-only 헌장 (ADR-002) 위에서 작성된다.

## 1. 개요

2026-05-16 Round 3 사용자 확정: *"mink 인자 없으면 TUI, 인자 있으면 CLI. 둘 다 같은 backend service layer 호출, 피처 패리티"*. 기존 CLI-TUI-003 (v0.2.0 implemented) 의 *CLI* 와 *TUI* 분리 표현을 *단일 mink 바이너리 + 피처 패리티* 로 통일.

### 1.1 Surface Assumptions

- A1: `internal/tui/*` 와 `cmd/mink/*` 는 이미 동일 service layer 호출 패턴에 가깝다 (CLI-TUI-001~003 implemented 결과)
- A2: 신규 backend 기능 추가 없음 (LLM-ROUTING + MEMORY + AUTH 의 인터페이스 wiring 만)
- A3: Bubble Tea 프레임워크 유지 (대안 ratatui-go 등 검토 안 함)
- A4: 코드 audit 으로 발견될 누락 명령은 5건 이하 예상

## 2. 스코프

### 2.1 IN

- 패리티 매트릭스 (TUI 메뉴 vs CLI 하위명령 1:1)
- 누락 보강: `mink memory {add|search|reindex|export|import|stats|prune}` (MEMORY-QMD-001) / `mink login` 통합 (LLM-ROUTING-V2-AMEND) / `mink config set/get/list` (AUTH-CREDENTIAL-001)
- 용어 codemod: 문서·README·help text 의 "CLI", "TUI" 분리 표현 → "mink" 단일 entry
- AGPL-3.0 헤더 신규 .go 파일 추가

### 2.2 OUT

- 신규 backend 기능
- TUI 디자인 변경 (Bubble Tea 유지)
- 신규 도움말 시스템 (기존 cobra help 유지)

## 3. EARS Requirements

### 3.1 Ubiquitous (6)

- **REQ-CTA-001 [P0]**: `mink` binary **shall** dispatch to TUI mode when invoked without args, CLI mode when args present
- **REQ-CTA-002 [P0]**: TUI mode and CLI mode **shall** call identical service layer functions (no duplicated business logic)
- **REQ-CTA-003 [P0]**: Every TUI menu action **shall** have a corresponding `mink <subcommand>` form
- **REQ-CTA-004 [P0]**: Every `mink <subcommand>` **shall** be reachable from TUI (no CLI-only operations)
- **REQ-CTA-005 [P0]**: Documentation (README, help text, MEMORY) **shall** refer to "mink" as single entry, not "MINK CLI" or "MINK TUI" separately
- **REQ-CTA-006 [P1]**: All new .go files **shall** carry `// SPDX-License-Identifier: AGPL-3.0-only` header (ADR-002 정합)

### 3.2 Event-Driven (6)

- **REQ-CTA-007 [P0]**: When `mink memory {add|search|reindex|export|import|stats|prune}` is invoked, the service **shall** delegate to MEMORY-QMD-001 service layer
- **REQ-CTA-008 [P0]**: When `mink login {anthropic|deepseek|openai|codex|zai}` is invoked, the service **shall** delegate to LLM-ROUTING-V2-AMEND-001 auth handler
- **REQ-CTA-009 [P0]**: When `mink config set auth.store {keyring|file|keyring,file}` is invoked, the service **shall** delegate to AUTH-CREDENTIAL-001 store selector
- **REQ-CTA-010 [P1]**: When TUI menu "Memory" is selected, the underlying call **shall** be identical to `mink memory` CLI path
- **REQ-CTA-011 [P1]**: When user pipes stdin to `mink ask`, the service **shall** read prompt from stdin (CLI automation 호환)
- **REQ-CTA-012 [P2]**: When `mink --help` is invoked, output **shall** include every TUI-reachable command without omission

### 3.3 State-Driven (3)

- **REQ-CTA-013 [P1]**: While running in CLI mode (args present), the service **shall not** open Bubble Tea TUI loop (즉 비대화 자동화 호환)
- **REQ-CTA-014 [P1]**: While running in TUI mode (no args), the service **shall** maintain interactive state until user explicit `q` / Ctrl+C
- **REQ-CTA-015 [P2]**: While `MINK_NO_COLOR=1` env, both modes **shall** disable ANSI color codes

### 3.4 Unwanted (4)

- **REQ-CTA-016 [P0]**: The service **shall not** duplicate business logic between `internal/tui/*` and `cmd/mink/*` (DRY 원칙)
- **REQ-CTA-017 [P0]**: The service **shall not** expose internal-only commands via CLI without explicit `--internal` flag
- **REQ-CTA-018 [P1]**: The service **shall not** mix Korean stdout messages with English errors in same execution path
- **REQ-CTA-019 [P1]**: Documentation **shall not** describe MINK as having "separate CLI tool" and "separate TUI tool"

### 3.5 Optional (3)

- **REQ-CTA-020 [P2, OPT]**: Where `MINK_TUI_THEME=dark|light` env is set, TUI **shall** apply theme accordingly
- **REQ-CTA-021 [P2, OPT]**: Where shell completion is requested (`mink completion bash|zsh|fish`), generate completion script covering all CLI subcommands
- **REQ-CTA-022 [P2, OPT]**: Where `mink --json` flag is set, output **shall** be JSON-formatted for machine consumption

## 4. 마일스톤 (요약)

- M1: 코드 audit + 패리티 매트릭스 작성 (manual)
- M2: 누락 명령 추가 (mink memory / mink login / mink config) + 용어 codemod
- M3: shell completion + JSON 출력 + theme env (Optional, post-launch)

## 5. 의존

- LLM-ROUTING-V2-AMEND-001 (`mink login`)
- MEMORY-QMD-001 (`mink memory`)
- AUTH-CREDENTIAL-001 (`mink config set auth.store`)

## 6. 본격 plan 이월

- research.md: Bubble Tea / cobra / Charm 생태계 패턴 + 기존 CLI-TUI-001~003 implementation 분석
- plan.md: 3 마일스톤 + Go 패키지 분할 + audit 메트릭
- tasks.md: 12~15 task
- acceptance.md: 24 AC (REQ ↔ AC 1:N traceable, audit B2 학습)
- progress.md: 0% / 3 마일스톤 ⏸️
- plan-auditor pass

## 7. TRUST 5

| 차원 | 적용 |
|---|---|
| Tested | 24 AC + unit/integration 기반 (M1 audit 결과 → M2 implementation) |
| Readable | 단일 entry "mink", 피처 패리티 |
| Unified | DRY (REQ-CTA-016) |
| Secured | --internal flag (REQ-CTA-017), AGPL 헤더 (REQ-CTA-006) |
| Trackable | 22 REQ + 24 AC traceable |
