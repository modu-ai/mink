---
id: SPEC-GOOSE-FS-ACCESS-001
version: 0.1.0
status: planned
created_at: 2026-04-24
updated_at: 2026-04-24
author: architecture-redesign-v0.2
priority: P0
issue_number: null
phase: 5
milestone: M5
size: 중(M)
lifecycle: spec-first
labels: []
---

# SPEC-GOOSE-FS-ACCESS-001 — Filesystem Access Matrix Engine

> v0.2 신규. `security.yaml` 해석 엔진 + path resolution (Tier 2).
> Parent: SPEC-GOOSE-ARCH-REDESIGN-v0.2 Tier 2

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|------|------|-----------|------|
| 0.1.0 | 2026-04-24 | SPEC 초안 작성 | manager-spec |
| 0.1.0 | 2026-04-27 | HISTORY 섹션 추가 (감사) | GOOS행님 |

## Goal

`./.goose/config/security.yaml` 의 write_paths / read_paths / blocked_always 선언을 파싱하여 agent의 모든 FS 접근 시도를 매칭하고 허용/거부/AskUser 흐름으로 분기한다.

## Scope

- security.yaml 스키마 정의 및 파서
- Glob 패턴 매칭 (`./.goose/**`, `~/.ssh/**` 등)
- 3-stage decision flow (blocked_always → write matrix → read matrix)
- AskUserQuestion fallback
- fs_access_log 기록

## Requirements (EARS)

### REQ-FSACCESS-001
**WHEN** FS write/create/delete 요청이 발생할 때, the system **SHALL** blocked_always 우선 검사 후 write_paths allowlist를 확인하고, 매칭 없으면 AskUserQuestion을 호출한다.

### REQ-FSACCESS-002
**WHEN** FS read 요청이 발생할 때, the system **SHALL** blocked_always 우선 검사 후 `./` 및 read_paths allowlist 확인, 매칭 없으면 AskUserQuestion을 호출한다.

### REQ-FSACCESS-003
**WHEN** blocked_always 목록의 경로에 접근 시도가 있을 때, the system **SHALL** 예외 없이 거부하고 audit.log에 기록한다 (user override 불가).

### REQ-FSACCESS-004
**WHEN** `security.yaml` 이 파일 변경으로 갱신되면, the system **SHALL** 런타임에 정책을 hot-reload한다.

### REQ-FSACCESS-005
**WHEN** 모든 FS 접근 결정이 내려질 때, the system **SHALL** `fs_access_log` 테이블에 (operation, path, allowed, reason, at)을 기록한다.

## Acceptance Criteria

- AC-FSACCESS-01: 3-stage decision flow 정확성 (blocked > write > read > ask)
- AC-FSACCESS-02: glob 패턴 매칭 정밀도 (e.g., `./drafts/**.md` vs `./drafts/sub/nested.md`)
- AC-FSACCESS-03: blocked_always override 절대 불가
- AC-FSACCESS-04: 핫 리로드 동작
- AC-FSACCESS-05: fs_access_log 완전성

## Dependencies

- PERMISSION-001 (AskUserQuestion flow)
- AUDIT-001 (logging)
- SECURITY-SANDBOX-001 (kernel-level enforcement backup)

## References

- `.moai/design/goose-runtime-architecture-v0.2.md` §5 Tier 2
