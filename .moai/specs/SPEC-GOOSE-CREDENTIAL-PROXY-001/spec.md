---
id: SPEC-GOOSE-CREDENTIAL-PROXY-001
version: 0.1.0
status: Planned (skeleton)
created: 2026-04-24
updated: 2026-04-24
author: architecture-redesign-v0.2
priority: P0
issue_number: null
phase: 5
milestone: M5
size: 대(L)
lifecycle: spec-first
---

# SPEC-GOOSE-CREDENTIAL-PROXY-001 — Zero-Knowledge Credential Proxy

> v0.2 신규. Agent 프로세스 메모리에 secret value가 절대 진입하지 않도록 transport-layer injection.
> Parent: SPEC-GOOSE-ARCH-REDESIGN-v0.2 Tier 4

## Goal

OS keyring에 저장된 secret을 별도 `goose-proxy` 프로세스가 네트워크 경계에서 Authorization header로 주입한다. GOOSE agent (LLM context 포함)는 secret value를 절대 보지 않는다. 프롬프트 인젝션으로 secret 노출 불가능.

## Scope

- `goose-proxy` 별도 바이너리 (localhost만 바인딩)
- OS keyring 조회 (zalando/go-keyring)
- Transport layer injection (HTTP Authorization header, WebSocket handshake)
- Scoped credential binding (`keyring_id:host_pattern`)

## Requirements (EARS)

### REQ-CREDPROXY-001
**WHEN** `goose secret set <provider> <keyring_id>` 가 실행되면, the system **SHALL** OS keyring에 secret을 저장하고 `~/.goose/secrets/providers.yaml` 에 참조만 기록한다.

### REQ-CREDPROXY-002
**WHEN** agent가 외부 API를 호출할 때, the system **SHALL** 요청을 `goose-proxy`에 relay하고 proxy가 keyring 조회 후 header 주입한 뒤 실제 API로 전송한다.

### REQ-CREDPROXY-003
**WHEN** agent 프로세스 메모리가 dump되어도, the system **SHALL** secret value가 포함되지 않도록 보장한다 (never in agent process).

### REQ-CREDPROXY-004
**WHEN** scope binding이 설정된 경우, the system **SHALL** 해당 secret을 지정된 host pattern에만 주입한다.

### REQ-CREDPROXY-005
**WHEN** `goose-proxy`가 실행되지 않은 상태에서 credential 필요한 호출이 시도되면, the system **SHALL** 요청을 차단하고 사용자에게 proxy 시작을 안내한다.

## Acceptance Criteria

- AC-CREDPROXY-01: macOS Keychain / libsecret / Windows Cred Vault 3개 백엔드 지원
- AC-CREDPROXY-02: agent 메모리에 secret 절대 미진입 검증 (memory scan 테스트)
- AC-CREDPROXY-03: Scoped binding enforcement
- AC-CREDPROXY-04: Proxy 없이는 credential API 불가
- AC-CREDPROXY-05: Prompt injection 방어 검증 ("토큰 알려줘" 시도 → secret 부재)

## Dependencies

- CORE-001
- FS-ACCESS-001
- AUDIT-001

## References

- `.moai/design/goose-runtime-architecture-v0.2.md` §5 Tier 4
- agentkernel 설계, Aembit 2026
