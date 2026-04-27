---
id: SPEC-GOOSE-WEBUI-001
version: 0.1.0
status: planned
created_at: 2026-04-24
updated_at: 2026-04-24
author: architecture-redesign-v0.2
priority: P0
issue_number: null
phase: 6
milestone: M6
size: 대(L)
lifecycle: spec-first
labels: []
---

# SPEC-GOOSE-WEBUI-001 — Localhost Web UI (비개발자 대응)

> v0.2 신규. 비개발자 사용자가 CLI 없이 GOOSE 설치/설정/사용 가능한 localhost 기반 웹 GUI.
> Parent: SPEC-GOOSE-ARCH-REDESIGN-v0.2

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|------|------|-----------|------|
| 0.1.0 | 2026-04-24 | SPEC 초안 작성 | manager-spec |
| 0.1.0 | 2026-04-27 | HISTORY 섹션 추가 (감사) | GOOS행님 |

## Goal

CLI 학습 없이도 GOOSE를 설치하고 운영할 수 있는 localhost 기반 Web UI를 제공한다. 외부 네트워크에 바인딩하지 않으며, 단일 머신 내부 접근만 허용한다.

## Scope

- Install wizard (provider key 입력, persona 생성, messenger 활성화)
- Chat interface (SSE 스트림)
- Task/Ritual 조회 및 승인 flow
- Settings 관리 (security.yaml · providers.yaml · channels.yaml)
- Log/Audit 뷰어

## Requirements (EARS)

### REQ-WEBUI-001
**WHEN** `goose web` 커맨드가 실행되면, the system **SHALL** localhost:8787(기본)에 Web UI를 바인딩하고 기본 브라우저를 열어준다.

### REQ-WEBUI-002
**WHEN** 외부 네트워크 인터페이스에서 접근 시도하면, the system **SHALL** 연결을 거부한다 (localhost/127.0.0.1만 허용).

### REQ-WEBUI-003
**WHEN** Web UI가 goosed 데몬과 통신할 때, the system **SHALL** SSE(Server-Sent Events)로 스트리밍 응답을 전달한다.

### REQ-WEBUI-004
**WHEN** 첫 접속 시 auth token이 없으면, the system **SHALL** install wizard를 표시한다.

### REQ-WEBUI-005
**WHEN** Plan 단계에서 AskUserQuestion 유발 task가 발생하면, the system **SHALL** Web UI에서 승인/거절/수정 UI를 제공한다.

## Acceptance Criteria

- AC-WEBUI-01: 비개발자 사용자가 5분 내 설치 및 첫 대화 가능
- AC-WEBUI-02: localhost 외부 접근 100% 차단
- AC-WEBUI-03: SSE 스트림 latency <100ms 첫 토큰 도달
- AC-WEBUI-04: 반응형 (모바일 브라우저에서도 사용 가능)
- AC-WEBUI-05: Settings 변경 즉시 goosed에 반영

## Dependencies

- BRIDGE-001 (daemon ↔ Web UI 로컬 bridge)
- PERMISSION-001 (token auth)
- CREDENTIAL-PROXY-001 (secret 입력 시 keyring 저장)

## References

- `.moai/design/goose-runtime-architecture-v0.2.md` §6 메신저 채널
