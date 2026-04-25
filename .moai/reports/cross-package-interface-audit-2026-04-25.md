---
id: REPORT-CROSS-PKG-IFACE-AUDIT-2026-04-25
type: audit
created_at: 2026-04-25
author: manager-spec
related_specs:
  - SPEC-GOOSE-TOOLS-001
  - SPEC-GOOSE-HOOK-001
  - SPEC-GOOSE-SKILLS-001
  - SPEC-GOOSE-CORE-001
  - SPEC-GOOSE-CLI-001
  - SPEC-GOOSE-SUBAGENT-001
status: open
---

# Cross-Package Interface Stub Audit (TOOLS-001 / HOOK-001 → consumer SPECs)

## 배경

PR #10 (TOOLS-001) 및 PR #11 (HOOK-001) 머지 시 **cross-package interface stub**이 다수 도입되었다. 이들 stub은 본 두 SPEC이 명시적으로 다른 consumer SPEC에 위임한 책임이며, 후속 SPEC 구현 시점에 실제 구현체가 등록되어야 정상 동작한다.

본 감사는 **각 consumer SPEC(SKILLS-001 / CORE-001 / CLI-001 / SUBAGENT-001)이 자신이 책임지는 인터페이스 계약을 SPEC 문서에 명시 참조하는가**를 확인하여 implementation 시점의 누락 위험을 사전 차단한다.

## 감사 범위

### Source SPEC (계약을 정의하는 측)

- **SPEC-GOOSE-TOOLS-001** — Tool Registry / Executor / 6 built-in tools
- **SPEC-GOOSE-HOOK-001** — Hook Registry / 14 hook events / Permission dispatcher

### Consumer SPEC (계약을 구현해야 하는 측)

- SPEC-GOOSE-SKILLS-001
- SPEC-GOOSE-CORE-001
- SPEC-GOOSE-CLI-001
- SPEC-GOOSE-SUBAGENT-001

## 식별된 Stub Interface

| Stub ID | 정의 SPEC | 정의 위치 | 인터페이스 시그니처 | 책임 SPEC |
|---------|----------|---------|------------------|---------|
| S-1 | HOOK-001 | REQ-HK-008, §6.2 `SetSkillsFileChangedConsumer(consumer FileChangedConsumer)` | `FileChangedConsumer(paths []string) []string` | SKILLS-001 |
| S-2 | HOOK-001 | REQ-HK-021(b), §6.11 | `WorkspaceRoot(sessionID string) string` | CORE-001 |
| S-3 | HOOK-001 | REQ-HK-009 (e) `InteractiveHandler` 라우팅 | `InteractiveHandler` (TUI permission prompt) | CLI-001 |
| S-4 | HOOK-001 | REQ-HK-009 (e) `CoordinatorHandler` / `SwarmWorkerHandler` 라우팅 | `CoordinatorHandler` / `SwarmWorkerHandler` (permission bubbling) | SUBAGENT-001 |
| S-5 | HOOK-001 | §3.1 OUT (Subagent lifecycle), event 상수 정의만 | `TaskCreated` / `TaskCompleted` dispatch 본체 | SUBAGENT-001 |
| S-6 | TOOLS-001 | §3.2 OUT, Exclusions | `goose tool list` CLI 서브커맨드 (Registry.ListNames + Inventory.ForModel 호출) | CLI-001 |
| S-7 | TOOLS-001 | §3.2 OUT, Exclusions | Plan Mode read-only guard (`permissionMode: plan` Executor 진입 전 필터) | SUBAGENT-001 |
| S-8 | TOOLS-001 | §3.2 OUT, Exclusions | Agent tool (sub-agent spawn) | SUBAGENT-001 |
| S-9 | TOOLS-001 | §3.2 OUT, Exclusions | Skill tool (Skill 발견 / 실행) | SKILLS-001 |
| S-10 | TOOLS-001 | REQ-TOOLS-011 | `Registry.Drain()` 호출 (graceful shutdown 순서) | CORE-001 |

## Consumer SPEC별 계약 참조 현황

### ✅ SKILLS-001 (PASS — 모든 계약 명시)

| Stub | 참조 위치 | 비고 |
|------|---------|------|
| S-1 | spec.md L109 (FileChangedConsumer 정의), L145 (REQ-SK-007), L490 (`func (r *SkillRegistry) FileChangedConsumer`), L500 (TriggerConditional → HOOK-001 매핑), L573 (후속 SPEC 표) | EARS REQ-SK-007 + AC-SK-004 + Go API 표면 모두 정합 |
| S-9 | spec.md L62 (OUT: Skill SDK), L73 (hooks parsing only), L189-191 (parser shall not execute) | Skill tool 파서/레지스트리 책임 분리 명확 |

### ✅ CLI-001 (PASS — 모든 계약 명시)

| Stub | 참조 위치 | 비고 |
|------|---------|------|
| S-3 | spec.md L180 (permission UI 최소 form), L737 (후속 SPEC: HOOK-001 permission 통합), L811 (Exclusions: 고급 UI는 HOOK-001) | y/n 최소 form만 책임, 고급 UI는 HOOK-001로 위임 명시 |
| S-6 | spec.md L42, L64, L166, L300-302 (AC-CLI-011 `goose tool list`), L733 (선행 SPEC: TOOLS-001), L776, L793 | TOOLS-001 Registry consumer 역할 명시 |

### ✅ SUBAGENT-001 (PASS — 모든 계약 명시)

| Stub | 참조 위치 | 비고 |
|------|---------|------|
| S-4 | spec.md L44, L54, L91, L93, L495 (§6.7 Permission Bubbling), L515, L540 | `SwarmWorkerHandler` consumer 구현 명시 + REQ-SA-010 |
| S-5 | spec.md L65 (OUT: workflow.yaml team orchestrator → CLI-001 또는 별도 SPEC) | Subagent lifecycle은 SUBAGENT-001 책임. workflow.yaml team orchestration은 별도 SPEC 위임 |
| S-7 | spec.md L65 (PermissionMode override: `tools`/`model`/`maxTurns`/`effort`/`permissionMode`) | Plan Mode read-only guard는 PermissionMode 시스템에 흡수 |
| S-8 | spec.md 전체 (Agent tool ≡ RunAgent + AgentDefinition) | SPEC 본체가 Agent tool 구현 |

### ❌ CORE-001 (FAIL — 2건 누락)

| Stub | SPEC 내 참조 | 비고 |
|------|------------|------|
| S-2 | **없음** | HOOK-001 REQ-HK-021(b)이 `WorkspaceRoot(sessionID string) string` resolver를 CORE-001에 위임함을 명시했으나, CORE-001 spec.md에는 `WorkspaceRoot` / `sessionID` 키워드 0건. EARS REQ 누락. |
| S-10 | **부분** | CORE-001 §7.1에 `shutdown.go: RegisterHook / RunAllHooks / 30s timeout` 인프라는 정의되어 있으나, **TOOLS-001 `Registry.Drain()` 호출 의무**가 명시되지 않음. AC-CORE-002 (SIGTERM graceful shutdown)에 Registry drain 단계 추가 필요. |

## 결함 요약

| 결함 ID | 심각도 | 대상 SPEC | 내용 | 권고 조치 |
|---------|------|---------|------|---------|
| D-CORE-IF-1 | Major | SPEC-GOOSE-CORE-001 | S-2 누락. HOOK-001이 의존하는 `WorkspaceRoot(sessionID string) string` resolver가 CORE-001에 미정의. HOOK-001 구현 완료 시점에 fail-closed로 동작(REQ-HK-021(b) "session-resolution error 반환")하므로 immediate runtime failure는 없으나, 정상 동작을 위해 CORE-001이 resolver를 export 해야 함. | CORE-001 SPEC amendment(v0.2 또는 후속 minor): (1) §6.2 또는 §7.2 핵심 타입에 `WorkspaceRoot(sessionID string) string` 시그니처 추가, (2) 신규 EARS REQ-CORE-XXX 추가 ("**When** `WorkspaceRoot(sessionID)` is invoked, the resolver **shall** return the session's project root path or empty string if unresolved"), (3) SessionRegistry 또는 ContextRoot 컴포넌트에 매핑. |
| D-CORE-IF-2 | Minor | SPEC-GOOSE-CORE-001 | S-10 부분 명시. graceful shutdown 인프라(`RunAllHooks`)는 있으나 **TOOLS-001 `Registry.Drain()` 호출**이 shutdown sequence에 명시되지 않음. | CORE-001 SPEC amendment: AC-CORE-002 (SIGTERM graceful) 단계에 "Registry.Drain() 호출 → in-flight tool call 완료 대기 → context cancel" 순서 추가, 또는 §7.2 shutdown.go 의사코드에 Drain hook 등록 명시. |

## 권고 조치 (Action Plan)

### 즉시 (이번 minor release)

- 본 감사 보고서 머지 (변경 없음, 추적용 문서)
- CORE-001 maintainer에게 D-CORE-IF-1 / D-CORE-IF-2 알림

### 후속 SPEC 작업

| Priority | 작업 | 예상 SPEC |
|---------|------|---------|
| P1-high | CORE-001 v0.2 amendment (S-2 + S-10 명시) | SPEC-GOOSE-CORE-001 amendment 또는 SPEC-GOOSE-CORE-002 |
| P2-medium | SKILLS-001 implementation (S-1, S-9 구현체) | SPEC-GOOSE-SKILLS-001 run phase |
| P2-medium | CLI-001 implementation (S-3, S-6 구현체) | SPEC-GOOSE-CLI-001 run phase |
| P2-medium | SUBAGENT-001 implementation (S-4, S-5, S-7, S-8 구현체) | SPEC-GOOSE-SUBAGENT-001 run phase |

### Implementation 진입 전 체크포인트

각 consumer SPEC의 `/moai run` 진입 시점에 본 감사 표를 cross-check 하여 누락된 인터페이스 등록을 RED 단계 첫 테스트로 추가할 것을 권고:

- SKILLS-001 RED #1: `hookRegistry.SetSkillsFileChangedConsumer(skillsRegistry.FileChangedConsumer)` 호출 시 nil 등록 거부 + dispatch 통합 테스트
- CORE-001 RED #1: `core.WorkspaceRoot("test-session")` 시그니처 + 기본 fallback 동작
- CORE-001 RED #2: `tools.Registry.Drain()`이 shutdown hook chain에 등록되어 SIGTERM 시 호출되는지 검증
- CLI-001 RED #1: `goose tool list`가 `tools.Registry.ListNames()`를 정확히 호출하고 표시
- CLI-001 RED #2: `permission_request` SDKMessage 수신 시 `InteractiveHandler` 인터페이스로 노출
- SUBAGENT-001 RED #1: `permissionMode: bubble` 인 sub-agent의 `CanUseTool`이 `hooks.SwarmWorkerHandler` 경유
- SUBAGENT-001 RED #2: `RunAgent` Worktree isolation에서 `WorktreeCreate`/`WorktreeRemove` dispatch 정상

## 참조

- SPEC-GOOSE-TOOLS-001 §3.2 (Out of Scope) + REQ-TOOLS-011
- SPEC-GOOSE-HOOK-001 §3.2 (Out of Scope) + REQ-HK-008, REQ-HK-009, REQ-HK-021
- SPEC-GOOSE-SKILLS-001 §6.1, REQ-SK-007, AC-SK-004
- SPEC-GOOSE-CORE-001 §7.2, AC-CORE-002 (현재 누락분 표시)
- SPEC-GOOSE-CLI-001 AC-CLI-011, §11 후속 SPEC 표
- SPEC-GOOSE-SUBAGENT-001 §6.7 Permission Bubbling, REQ-SA-005~SA-010

---

Version: 1.0.0
Last Updated: 2026-04-25
