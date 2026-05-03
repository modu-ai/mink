---
id: SPEC-GOOSE-AUDIT-001
version: 0.1.0
status: completed
created_at: 2026-04-24
updated_at: 2026-05-04
completed: 2026-05-04
author: architecture-redesign-v0.2
priority: P0
issue_number: null
phase: 5
milestone: M5
size: 소(S)
lifecycle: spec-first
labels: []
---

# SPEC-GOOSE-AUDIT-001 — Append-Only Audit Log

> v0.2 신규. 보안 이벤트 (write, permission grant/revoke, sandbox block, credential access) 추적.
> Parent: SPEC-GOOSE-ARCH-REDESIGN-v0.2

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|------|------|-----------|------|
| 0.1.0 | 2026-04-24 | SPEC 초안 작성 | manager-spec |
| 0.1.0 | 2026-04-27 | HISTORY 섹션 추가 (감사) | GOOS행님 |

## Goal

Append-only 로그로 모든 보안 관련 이벤트를 기록하여 incident 분석과 forensic 추적을 가능하게 한다. `~/.goose/logs/audit.log` (전역) + `./.goose/logs/audit.local.log` (프로젝트).

## Scope

- Event schema (JSON lines)
- Append-only 보장 (`chattr +a` on Linux, SIP on macOS 권장)
- Structured event types
- Rotation policy (size/time-based)

## Event Types

- `fs.write`, `fs.read.denied`, `fs.blocked_always`
- `permission.grant`, `permission.revoke`, `permission.denied`
- `sandbox.blocked_syscall`
- `credential.accessed` (key reference 조회; value 아님)
- `task.plan_approved`, `task.plan_rejected`
- `goosed.start`, `goosed.stop`

## Requirements (EARS)

### REQ-AUDIT-001
**WHEN** 보안 관련 이벤트가 발생할 때, the system **SHALL** JSON line 형식으로 audit.log에 append한다 (never update or delete).

### REQ-AUDIT-002
**WHEN** audit.log 파일이 100MB를 초과할 때, the system **SHALL** 타임스탬프 접미사로 rotate한다 (기존 파일은 gzip 압축).

### REQ-AUDIT-003
**WHEN** 프로젝트 레벨 이벤트가 발생할 때, the system **SHALL** `./.goose/logs/audit.local.log`에도 복제하여 프로젝트 소유자가 검토 가능하게 한다.

### REQ-AUDIT-004
**WHEN** `goose audit query [--since=...] [--type=...]` 가 실행되면, the system **SHALL** 구조화된 검색 결과를 반환한다.

## Acceptance Criteria

- AC-AUDIT-01: JSON line format 모든 이벤트 기록
- AC-AUDIT-02: append-only 무결성 (tampering 시도 검출 가능)
- AC-AUDIT-03: rotation 무중단
- AC-AUDIT-04: CLI 쿼리 정확성

## Dependencies

- CORE-001
- FS-ACCESS-001
- PERMISSION-001
- SECURITY-SANDBOX-001
