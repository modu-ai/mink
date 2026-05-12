# GOOSE CLI 사용자 가이드

`goose` CLI는 GOOSE 데몬(`goosed`)과 통신해 LLM 채팅, 세션 관리, 설정/도구 조회, 감사 로그 검색을 수행하는 단일 진입 바이너리입니다. 본 가이드는 사용자가 빌드부터 일상 사용까지 따라갈 수 있도록 한국어로 정리되어 있습니다.

> 본 문서는 `SPEC-GOOSE-CLI-001` 후속 산출물입니다. 코드 기준일: main `06f5ea6` (Phase A~D + multi-turn replay 후속까지 머지된 상태).

## 목차

1. [시작하기](getting-started.md)
   - 사전 요구사항, 빌드, daemon 기동, 첫 `goose ask`
2. [명령 레퍼런스](commands.md)
   - 8개 최상위 명령 (`ask` / `ping` / `session` / `config` / `tool` / `audit` / `daemon` / `plugin` / `version`) 과 전역 플래그
3. [TUI 채팅 모드](tui.md)
   - 인자 없는 `goose` 실행, slash 명령, multi-turn replay 동작, 키 바인딩
4. [트러블슈팅](troubleshooting.md)
   - daemon unreachable, 종료 코드, 자주 발생하는 문제

## 빠른 진입

```bash
# 1. 빌드
go build ./cmd/goosed
go build ./cmd/goose

# 2. daemon 실행 (별도 터미널)
./goosed

# 3. 한 줄 채팅
./goose ask "안녕"

# 4. TUI 채팅
./goose
```

자세한 절차는 [시작하기](getting-started.md)에서 단계별로 설명합니다.

## 아키텍처 한눈에 보기

```
┌──────────────┐   Connect-protocol HTTP/2    ┌─────────────┐
│ goose (CLI)  │◀────────────────────────────▶│ goosed       │
│ cobra + TUI  │   (.goose/v1/*.proto)        │ (daemon)     │
└──────────────┘                              └─────────────┘
```

- CLI는 `internal/cli/transport`의 `ConnectClient`(`charmbracelet/x` 기반 HTTP/2 + protobuf-over-HTTP)로 daemon과 통신합니다.
- 모든 명령은 동일한 daemon 주소(`--daemon-addr`, 기본 `127.0.0.1:9005`)를 사용합니다.
- daemon이 켜져 있지 않으면 모든 daemon-의존 명령(`ask`, `ping`, `session load`, `config`, `tool`, `daemon status`)은 종료 코드 `69` 와 함께 `goose: daemon unreachable at <addr>` 메시지를 출력합니다.

## 관련 문서

- [`README.md`](../../README.md) — MINK 전체 개요
- [`SPEC-GOOSE-CLI-001`](../../.moai/specs/SPEC-GOOSE-CLI-001/spec.md) — CLI 명세 (EARS 형식)
- [`.moai/specs/SPEC-GOOSE-CLI-001/progress.md`](../../.moai/specs/SPEC-GOOSE-CLI-001/progress.md) — Phase A~D + 후속 작업 이력
