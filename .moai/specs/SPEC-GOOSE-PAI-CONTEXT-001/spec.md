---
id: SPEC-GOOSE-PAI-CONTEXT-001
version: 0.1.0
status: Planned (skeleton)
created: 2026-04-24
updated: 2026-04-24
author: architecture-redesign-v0.2
priority: P0
issue_number: null
phase: 7
milestone: M7
size: 중(M)
lifecycle: spec-first
---

# SPEC-GOOSE-PAI-CONTEXT-001 — PAI Identity Context Files

> v0.2 신규. Daniel Miessler PAI 패턴 계승 11개 identity 파일 관리.
> Parent: SPEC-GOOSE-ARCH-REDESIGN-v0.2
>
> **네이밍 주의**: 기존 CONTEXT-001 은 "Context Window 관리 및 Compaction 전략" (런타임 context window 제어)
> 으로 별개 주제. 기존 IDENTITY-001 은 "Identity Graph (POLE+O, Kuzu)" (Phase 8 관계 그래프)로 또 다른 개념.

## Goal

`./.goose/context/` 디렉토리에 11개 PAI identity 파일을 관리하고, Agent Core가 Plan phase에서 적절히 로드·참조하도록 한다.

## Scope

**11 identity files**: mission, goals, projects, beliefs, models, strategies, narratives, learned, challenges, ideas, growth

## Requirements (EARS)

### REQ-PAI-CONTEXT-001
**WHEN** `goose init` 이 실행될 때, the system **SHALL** `./.goose/context/` 에 11개 템플릿 파일을 생성한다.

### REQ-PAI-CONTEXT-002
**WHEN** Agent Core가 Plan phase에서 context retrieval을 수행할 때, the system **SHALL** 관련 identity 파일을 relevance score 기반으로 선택적으로 로드한다.

### REQ-PAI-CONTEXT-003
**WHEN** Sync phase에서 새 정보가 학습될 때, the system **SHALL** `learned.md` 또는 적절한 context 파일에 append한다.

### REQ-PAI-CONTEXT-004
**WHEN** 사용자가 context 파일을 직접 편집할 때, the system **SHALL** 변경을 감지하고 QMD 인덱스를 재빌드한다.

### REQ-PAI-CONTEXT-005
**WHEN** growth stage가 변경될 때, the system **SHALL** `growth.md` 에 이력을 append하고 voice.md 참조를 갱신한다.

## Acceptance Criteria

- AC-PAI-CONTEXT-01: 11개 템플릿 파일 자동 생성
- AC-PAI-CONTEXT-02: Relevance-based selective loading 동작
- AC-PAI-CONTEXT-03: Auto-update on Sync phase
- AC-PAI-CONTEXT-04: 파일 편집 감지 → QMD reindex
- AC-PAI-CONTEXT-05: growth.md history 완전성

## Dependencies

- QMD-001 (context 파일 semantic 검색)
- CORE-001 (runtime)

## References

- `.moai/design/goose-runtime-architecture-v0.2.md` §1.2 `./.goose/context/`
- PAI (Daniel Miessler) — https://github.com/danielmiessler/Personal_AI_Infrastructure
