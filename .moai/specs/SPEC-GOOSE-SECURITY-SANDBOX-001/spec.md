---
id: SPEC-GOOSE-SECURITY-SANDBOX-001
version: 0.1.0
status: planned
created_at: 2026-04-24
updated_at: 2026-04-24
author: architecture-redesign-v0.2
priority: P0
issue_number: null
phase: 5
milestone: M5
size: 대(L)
lifecycle: spec-first
labels: []
---

# SPEC-GOOSE-SECURITY-SANDBOX-001 — OS-Level Sandbox

> v0.2 신규. path-string denylist 우회 방지를 위한 kernel-layer 격리.
> Parent: SPEC-GOOSE-ARCH-REDESIGN-v0.2 Tier 3

## Goal

OS 커널 레벨 sandbox (macOS Seatbelt / Linux Landlock+Seccomp / Windows AppContainer)를 통해 agent의 FS·network·process 접근을 구조적으로 제한한다. Path-string 기반 denylist가 bypass되어도 kernel이 차단한다.

## Scope

- macOS Seatbelt profile (`sandbox-exec -p "..."`)
- Linux Landlock LSM + Seccomp-BPF filter
- Windows AppContainer manifest
- Fallback policy: sandbox 활성화 불가 시 실행 거부 (`fallback_behavior: refuse`)

## Requirements (EARS)

### REQ-SANDBOX-001
**WHEN** goosed가 시작될 때, the system **SHALL** 플랫폼별 sandbox를 활성화하고 `security.yaml` 정책을 kernel rule로 변환한다.

### REQ-SANDBOX-002
**WHEN** macOS에서 실행될 때, the system **SHALL** Seatbelt profile로 `~/.ssh`, `/etc`, `/var`, `/proc` 등 blocked_always 경로 접근을 커널 차원에서 차단한다.

### REQ-SANDBOX-003
**WHEN** Linux에서 실행될 때, the system **SHALL** Landlock LSM 루트셋 + Seccomp filter로 동일한 차단을 enforce한다.

### REQ-SANDBOX-004
**WHEN** sandbox 활성화가 실패할 때, the system **SHALL** `fallback_behavior: refuse` 기본값에 따라 실행을 거부한다.

### REQ-SANDBOX-005
**WHEN** 차단된 syscall이 시도될 때, the system **SHALL** audit.log에 이벤트를 기록하고 부모 프로세스(agent)에 결과를 리턴한다.

## Acceptance Criteria

- AC-SANDBOX-01: 3개 플랫폼에서 동일한 blocked_always enforcement
- AC-SANDBOX-02: Ona-류 `/proc/self/root/*` 우회 시도 차단
- AC-SANDBOX-03: sandbox 실패 시 refuse fallback
- AC-SANDBOX-04: 차단 이벤트 audit.log 기록
- AC-SANDBOX-05: non-root 실행 강제

## Dependencies

- FS-ACCESS-001 (policy 정의)
- AUDIT-001
- CORE-001

## References

- `.moai/design/goose-runtime-architecture-v0.2.md` §5 Tier 3
- Northflank / NVIDIA AI Red Team 2026 권고
