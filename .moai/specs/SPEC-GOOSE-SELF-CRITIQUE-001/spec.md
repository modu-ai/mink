---
id: SPEC-GOOSE-SELF-CRITIQUE-001
version: 0.1.0
status: implemented
created_at: 2026-04-24
updated_at: 2026-04-28
author: architecture-redesign-v0.2
priority: P0
issue_number: null
phase: 3
milestone: M3
size: 중(M)
lifecycle: spec-first
labels: []
---

# SPEC-GOOSE-SELF-CRITIQUE-001 — Task Self-Critique (Reflect Phase)

> v0.2 신규. Plan-Run-Reflect-Sync 루프의 Reflect phase 구현.
> Parent: SPEC-GOOSE-ARCH-REDESIGN-v0.2
>
> **네이밍 주의**: 기존 SPEC-GOOSE-REFLECT-001 은 "5단계 승격 파이프라인(Observation → Graduated)" 으로
> 학습 엔진의 knowledge graduation 개념이다. 본 SPEC은 **Task 단위 self-critique** 로 별개 개념.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|------|------|-----------|------|
| 0.1.0 | 2026-04-24 | SPEC 초안 작성 | manager-spec |
| 0.1.0 | 2026-04-27 | HISTORY 섹션 추가 (감사) | GOOS행님 |

## Goal

모든 Task 종료 후 LLM 자체 critique (gap / inconsistency / unsupported claim)를 실행하여 품질을 보장하고, score 미달 시 Plan 단계로 re-plan 한다 (최대 2회).

## Scope

- Post-Task self-critique (매 Task)
- Daily reflection (집계)
- Re-plan trigger (score < 0.7)

## Requirements (EARS)

### REQ-SELF-CRITIQUE-001
**WHEN** Run phase가 완료될 때, the system **SHALL** LLM critique (3 dimension: gap, inconsistency, unsupported claim)를 실행한다.

### REQ-SELF-CRITIQUE-002
**WHEN** reflection score가 0.7 미만일 때, the system **SHALL** Plan으로 돌아가 re-plan한다. Re-plan 횟수는 최대 2회.

### REQ-SELF-CRITIQUE-003
**WHEN** reflection이 완료되면, the system **SHALL** `./.goose/tasks/{task-id}/reflection.md` 및 `tasks.score` 컬럼에 결과를 기록한다.

### REQ-SELF-CRITIQUE-004
**WHEN** 일일 지정 시간에 도달하면, the system **SHALL** 오늘 task 전체 집계 reflection을 생성하고 primary channel로 전달한다.

## Acceptance Criteria

- AC-SELF-CRITIQUE-01: 모든 Task에 reflection.md 생성
- AC-SELF-CRITIQUE-02: score < 0.7 시 re-plan 자동 트리거 (최대 2회)
- AC-SELF-CRITIQUE-03: daily reflection 스케줄 가능
- AC-SELF-CRITIQUE-04: Task latency 증가 +20~40% 내 허용

## Dependencies

- Plan-Run-Sync engine (M3)
- CORE-001 (runtime)

## References

- `.moai/design/goose-runtime-architecture-v0.2.md` §3 Reflect Phase
