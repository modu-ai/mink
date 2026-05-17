---
id: SPEC-MINK-CLI-TUI-003-AMEND-001
version: 0.1.0
status: draft
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI orchestrator
priority: medium
phase: 3
size: 소(S)
lifecycle: spec-first
labels: [cli, tui, parity, audit, sprint-3, amendment]
amends: [SPEC-GOOSE-CLI-TUI-003]
related:
  - ADR-002 (AGPL-3.0 전환)
  - SPEC-MINK-LLM-ROUTING-V2-AMEND-001
  - SPEC-MINK-MEMORY-QMD-001
---

# SPEC-MINK-CLI-TUI-003-AMEND-001 — CLI = TUI 피처 패리티 감사 + 용어 통일

> **STUB / DRAFT (2026-05-16)**: 본 SPEC 은 사용자 결정 진입을 표시하는 *amendment stub*. 본격 코드 audit + 문서 codemod 는 후속 PR 에서 expert-refactoring spawn 으로 진행.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | amendment stub — Round 3 사용자 결정 (CLI/TUI 통합 해석 확정) | MoAI orchestrator |

## 1. 개요

2026-05-16 사용자 확정 (Round 3 AskUserQuestion): *"mink 인자 없으면 TUI, 인자 있으면 CLI. 둘 다 같은 backend service layer 호출, 피처 패리티"*.

기존 CLI-TUI-003 (v0.2.0 implemented) 은 *CLI* 와 *TUI* 를 별개 entry 로 다루는 표현이 산재. 본 amendment 는 *단일 mink 바이너리 + 피처 패리티* 로 용어·코드·문서 통일.

## 2. 스코프

### 2.1 IN (후속 PR plan-auditor pass)

- **코드 audit**: `internal/tui/*` 와 `cmd/mink/*` 가 동일 service layer 호출하는지 검증
- **피처 패리티 매트릭스**: TUI 메뉴 vs CLI 하위명령 1:1 매핑 + 누락 보강
- **용어 통일**: 문서·README·help text 의 "CLI", "TUI" 분리 표현 → "mink" 단일 entry 로
- **mink 메모리 subcommand 추가**: MEMORY-QMD-001 정합 (`mink memory {add|search|reindex|export|import|stats|prune}`)
- **mink login 추가**: LLM-ROUTING-V2-AMEND 정합 (5 provider OAuth/key paste 통합)

### 2.2 OUT

- 새 backend 기능 (기존 service layer 그대로)
- TUI 신규 디자인 (기존 Bubble Tea 유지)

## 3. 의존

- LLM-ROUTING-V2-AMEND-001 (mink login 통합)
- MEMORY-QMD-001 (mink memory 명령)
- AUTH-CREDENTIAL-001 (credential 명령)

## 4. 본격 plan 이월 (후속 PR)

- expert-refactoring spawn 으로 코드 audit + 문서 codemod
- 5 산출물 (research/plan/tasks/acceptance/progress)
- plan-auditor pass
