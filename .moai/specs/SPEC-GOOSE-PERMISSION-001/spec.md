---
id: SPEC-GOOSE-PERMISSION-001
version: 0.1.0
status: planned
created_at: 2026-04-24
updated_at: 2026-04-24
author: architecture-redesign-v0.2
priority: P0
issue_number: null
phase: 2
milestone: M2
size: 중(M)
lifecycle: spec-first
labels: []
---

# SPEC-GOOSE-PERMISSION-001 — Declared Permission + First-Call Confirm

> v0.2 신규. Skill/MCP frontmatter의 `requires:` 선언 기반 권한 시스템.
> Parent: SPEC-GOOSE-ARCH-REDESIGN-v0.2 Tier 5

## Goal

Claude Code 철학을 계승한 선언 기반 권한 시스템: Skill/MCP가 요구 권한을 frontmatter에 선언하고, 첫 호출 시 사용자 확인을 받아 이후 grant를 재사용한다.

## Scope

- Skill/MCP frontmatter `requires:` 스키마 (net, fs_read, fs_write, exec)
- First-call confirm flow (AskUserQuestion)
- Grant 영속화 (`permissions` 테이블)
- Revoke 명령 (`goose permission revoke <subject>`)

## Requirements (EARS)

### REQ-PERMISSION-001
**WHEN** Skill 또는 MCP가 처음 호출될 때, the system **SHALL** frontmatter `requires:` 선언을 읽고 AskUserQuestion으로 [항상 허용 / 이번만 / 거절] 중 선택을 요청한다.

### REQ-PERMISSION-002
**WHEN** 사용자가 "항상 허용"을 선택하면, the system **SHALL** `permissions` 테이블에 grant를 기록하고 이후 호출 시 재확인 없이 진행한다.

### REQ-PERMISSION-003
**WHEN** Skill이 선언하지 않은 권한을 요청할 때, the system **SHALL** 실행을 차단하고 명시적 선언을 요구한다.

### REQ-PERMISSION-004
**WHEN** `goose permission revoke <subject>` 가 실행되면, the system **SHALL** 해당 grant를 제거하고 다음 호출 시 재확인한다.

### REQ-PERMISSION-005
**WHEN** 선언된 권한이 blocked_always 목록과 겹칠 때, the system **SHALL** grant 요청 자체를 거부한다 (user override 불가).

## Acceptance Criteria

- AC-PERMISSION-01: frontmatter `requires:` 스키마 검증
- AC-PERMISSION-02: first-call confirm UX (CLI + Web UI 양쪽)
- AC-PERMISSION-03: grant 영속화/revoke 동작
- AC-PERMISSION-04: blocked_always override 불가
- AC-PERMISSION-05: audit.log에 모든 grant/revoke 기록

## Dependencies

- FS-ACCESS-001 (path matrix)
- AUDIT-001 (audit log)
- SKILLS
- MCP

## References

- `.moai/design/goose-runtime-architecture-v0.2.md` §5 Tier 5
