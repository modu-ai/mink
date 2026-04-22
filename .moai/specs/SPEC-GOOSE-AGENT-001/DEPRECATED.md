# DEPRECATED

> **본 SPEC은 ROADMAP v2.0 재설계(2026-04-21)로 폐기됨.**

## 폐기 이유

v1.0 SPEC-GOOSE-AGENT-001은 단순 "Agent Runtime 최소 생애주기 + Persona"로 정의되었으나, v2.0에서 Claude Code agentic core + 4 primitive 패턴을 참고해 **QueryEngine 기반 async streaming loop + sub-agent isolation** 구조로 재설계.

## 대체 SPEC (v2.0)

- **SPEC-GOOSE-QUERY-001** (Phase 0) — QueryEngine + queryLoop (async streaming, state machine, 1 per conversation)
- **SPEC-GOOSE-SUBAGENT-001** (Phase 2) — Sub-agent Runtime (fork/worktree/background isolation, 3 memory scope)
- **SPEC-GOOSE-CONTEXT-001** (Phase 0) — Context Window 관리 + compaction

추가 연계:
- **SPEC-GOOSE-HOOK-001** (Phase 2) — Lifecycle Hook System (24 events + permission)
- **SPEC-GOOSE-TOOLS-001** (Phase 3) — Tool Registry + ToolSearch

## 참조

- `.moai/specs/ROADMAP.md` (v2.0)
- `.moai/project/research/claude-core.md` — QueryEngine 분석
- `.moai/project/research/claude-primitives.md` §4 — Sub-agent isolation

## 이전 내용 복구

본 디렉토리의 spec.md / research.md는 보존. git history 참조.
