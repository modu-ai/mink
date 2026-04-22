# DEPRECATED (재작성 예정)

> **본 SPEC은 ROADMAP v2.0에서 Phase 3으로 재배치되며 재작성 예정(2026-04-21).**

## 재작성 이유

v1.0 SPEC-GOOSE-CLI-001은 cobra 단일 구조였으나, v2.0에서:
- Connect-gRPC 클라이언트 통합 (TRANSPORT-001 소비자)
- Slash Command System 연계 (COMMAND-001)
- TUI 고도화 (Ink-like 패턴)
- Phase 0 → Phase 3 재배치

## v2.0 SPEC

- **SPEC-GOOSE-CLI-001** (Phase 3, 재작성) — goose CLI (cobra + Connect-gRPC + TUI)
- **SPEC-GOOSE-COMMAND-001** (Phase 3) — Slash Command System

의존성:
- TRANSPORT-001, COMMAND-001

## 참조

- `.moai/specs/ROADMAP.md` (v2.0) §4 Phase 3
- 기존 spec.md의 proto 스키마 결정은 v2.0 재작성 시 재검토

## 상태

본 디렉토리의 기존 spec.md / research.md는 Phase 3 재작성 시점까지 보존. v2.0 실행 시점에 overwrite 예정.
