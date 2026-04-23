---
id: SPEC-GOOSE-QMD-001
version: 0.1.0
status: Planned (skeleton)
created: 2026-04-24
updated: 2026-04-24
author: architecture-redesign-v0.2
priority: P0
issue_number: null
phase: 1
milestone: M1
size: 대(L)
lifecycle: spec-first
---

# SPEC-GOOSE-QMD-001 — QMD Embedded Hybrid Memory Search

> v0.2 신규. 상세 설계는 M1 진입 시 `manager-spec` 으로 완전 작성.
> Parent: SPEC-GOOSE-ARCH-REDESIGN-v0.2

## Goal

Local-first hybrid markdown 검색(BM25 + vector + LLM rerank)을 GOOSE 런타임에 임베드한다. `qntx-labs/qmd` (Rust) 를 CGO staticlib로 링크하여 단일 바이너리를 유지한다.

## Scope

- Episodic memory search (`./.goose/memory/`)
- Identity context search (`./.goose/context/*.md`)
- Skills semantic discovery (`./.goose/skills/**/*.md`)
- Task/Ritual result lookup
- MCP stdio server 노출

## Requirements (EARS)

### REQ-QMD-001
**WHEN** `goose qmd reindex` 가 호출되거나 파일 변경 이벤트가 감지되면, the system **SHALL** 대상 디렉토리(memory/context/skills/tasks/rituals)를 재인덱스한다.

### REQ-QMD-002
**WHEN** Agent Core가 context retrieval을 수행할 때, the system **SHALL** QMD hybrid search API를 통해 top-k 결과를 반환한다.

### REQ-QMD-003
**WHEN** QMD 바이너리가 빌드될 때, the system **SHALL** CGO staticlib 링크 후 goosed 단일 바이너리에 포함된다 (별도 런타임 의존성 없음).

### REQ-QMD-004
**WHEN** 임베더/리랭커 모델이 필요할 때, the system **SHALL** `./.goose/data/models/` 에 GGUF 모델을 자동 다운로드한다 (첫 실행 시).

### REQ-QMD-005
**WHEN** 외부 MCP 클라이언트가 쿼리할 때, the system **SHALL** stdio 기반 MCP 서버로 응답한다.

## Acceptance Criteria

- AC-QMD-01: `go build` 후 단일 바이너리에 QMD 기능 포함 (외부 실행 파일 없음)
- AC-QMD-02: 10,000+ 문서 인덱싱 후 쿼리 latency <10ms (p50)
- AC-QMD-03: BM25 + vector + rerank 3-stage pipeline 동작
- AC-QMD-04: MCP stdio 인터페이스로 외부 agent에서 쿼리 가능
- AC-QMD-05: 파일 변경 시 자동 증분 재인덱스

## Dependencies

- CORE-001 (runtime)
- qntx-labs/qmd Rust crate (upstream)

## References

- `.moai/design/goose-runtime-architecture-v0.2.md` §8 QMD Memory Search Integration
- https://github.com/qntx-labs/qmd
- https://github.com/tobi/qmd
