---
id: SPEC-GOOSE-ARCH-REDESIGN-v0.2
version: 0.1.0
status: implemented
created_at: 2026-04-24
updated_at: 2026-04-27
author: GOOS행님
priority: P0
issue_number: null
phase: 0
size: 대(L)
lifecycle: spec-as-source
labels: [phase-0, area/runtime, area/core, type/refactor, priority/p0-critical]
---


- Status: Proposed (2026-04-24)
- Type: Meta-SPEC (아키텍처 재편 계획)
- Reference: `.moai/design/goose-runtime-architecture-v0.2.md`
- Supersedes: v6.2 3-Tier × 5-Channel 아키텍처 일부 (58 SPEC 중 약 19개 영향)

---

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|------|------|-----------|------|
| 0.1.0 | 2026-04-24 | Meta-SPEC 초안 작성 | GOOS행님 |
| 0.1.0 | 2026-04-27 | YAML frontmatter 및 HISTORY 추가 (감사) | GOOS행님 |

## Goal

GOOSE 런타임 아키텍처를 Hermes v0.10 · Claude Code · moai-adk · PAI · Macaron · 2026 agent sandboxing 베스트 프랙티스 연구 결과에 정합시킨다. `./.goose/` (workspace) + `~/.goose/` (secrets only) 2원 구조를 도입하고 defense-in-depth 5-tier 보안 모델을 확립한다.

## Scope

- **In-scope**: SPEC 삭제/재편/신규 목록 확정, milestone 재정렬, 의존 그래프 업데이트
- **Out-of-scope**: 실제 SPEC 본문 재작성 (각 SPEC 개별 작업), 코드 구현

---

## Requirements (EARS)

### REQ-ARCH-001 (Storage Partition)
**WHEN** GOOSE가 실행될 때, the system **SHALL** credential/security 자산은 `~/.goose/` 에, 프로젝트 워크스페이스(persona/memory/skills/tasks/data/...)는 `./.goose/` 에 저장한다.

### REQ-ARCH-002 (Upward Traversal)
**WHEN** `goose` CLI가 호출될 때, the system **SHALL** 현재 디렉토리부터 상위로 가장 가까운 `./.goose/` 를 탐색하고, 발견되지 않으면 `goose init` 안내를 표시한다.

### REQ-ARCH-003 (5-Tier Defense-in-Depth)
**WHEN** GOOSE가 파일시스템·네트워크·외부 도구에 접근할 때, the system **SHALL** 5-tier 방어 체계(Storage Partition / FS Access Matrix / OS Sandbox / Zero-Knowledge Proxy / Declared Permission) 모든 계층을 통과해야 한다.

### REQ-ARCH-004 (Zero-Knowledge Credential)
**WHEN** GOOSE agent가 외부 API를 호출할 때, the system **SHALL** secret value를 agent 프로세스 메모리에 로드하지 않고, 별도 `goose-proxy` 프로세스가 transport layer에서 Authorization header를 주입한다.

### REQ-ARCH-005 (Blocked Paths)
**WHEN** 파일시스템 접근 시도가 발생할 때, the system **SHALL** `/etc`, `/var`, `/usr`, `/bin`, `/sbin`, `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.env*`, `~/.netrc`, `/proc`, `/sys`, `/dev` 에 대한 읽기/쓰기를 모두 차단한다. 사용자는 이 차단 목록을 override할 수 없다.

### REQ-ARCH-006 (4-Layer Primitive)
**WHEN** 사용자가 GOOSE와 상호작용할 때, the system **SHALL** UI Layer (Conversation/Task/Ritual/Reflection) · Agent Core (Intent→Plan→Run→Reflect→Sync) · Identity (11 PAI files + persona) · Memory (Working/Episodic/Semantic/Procedural/Affective/Identity) 4-Layer 모델을 따른다.

### REQ-ARCH-007 (Plan-Run-Sync Scope)
**WHEN** Task가 수신될 때, the system **SHALL** Plan→Run→Reflect→Sync 전체 phase를 실행한다. Conversation은 Plan phase를 bypass한다. Ritual은 cron → Task 자동 생성 후 Task 경로와 동일하게 처리한다.

### REQ-ARCH-008 (QMD Memory Integration)
**WHEN** Agent Core가 context retrieval을 수행할 때, the system **SHALL** QMD hybrid search (BM25 + vector + LLM rerank)를 사용하여 markdown 자산(memory, context, skills, tasks, rituals)에서 의미 검색을 수행한다. QMD는 M1부터 편입된다.

### REQ-ARCH-009 (Module Path)
**WHEN** Go module을 초기화할 때, the system **SHALL** `github.com/modu-ai/goose` module path를 사용한다. 바이너리는 `goosed` (daemon) + `goose` (CLI) + `goose-proxy` (credential proxy) 3종이다.

### REQ-ARCH-010 (v0.1 Alpha Channels)
**WHEN** v0.1 Alpha가 릴리스될 때, the system **SHALL** CLI/TUI, Telegram, Web UI (localhost) 3개 채널을 지원한다. Email 채널은 v0.1 범위에서 제외한다.

---

## Deletions (5 SPECs)

다음 SPEC은 **완전 삭제**된다 (SPEC 디렉토리 제거, ROADMAP/IMPLEMENTATION-ORDER 업데이트).

| SPEC | 삭제 사유 |
|---|---|
| SPEC-GOOSE-MOBILE-001 | 네이티브 모바일 앱 제거 — Hermes 패턴과 충돌, self-hosted 철학에 집중 |
| SPEC-GOOSE-WIDGET-001 | Apple Live Activity 제거 — 플랫폼 네이티브 종속성 제거 |
| SPEC-GOOSE-SYNC-001 | Multi-device sync 제거 — self-hosted + project-local로 불필요 |
| SPEC-GOOSE-CLOUD-001 | Cloud Free Tier 제거 — 3-Tier 전제 폐기 |
| SPEC-GOOSE-DISCOVERY-001 | 3-Tier discovery 제거 — 로컬 실행만 |

## Modifications (5 SPECs)

다음 SPEC은 **축소/재편**된다. 기존 본문 일부 유지, 스코프 변경 필요.

| SPEC | 변경 내용 |
|---|---|
| SPEC-GOOSE-AUTH-001 | 이메일 가입/QR 페어링 제거 → token 기반 local auth만 유지 |
| SPEC-GOOSE-NOTIFY-001 | APNs/FCM 제거 → Telegram/Web UI messenger push로 대체 |
| SPEC-GOOSE-BRIDGE-001 | 모바일 브릿지 제거 → localhost Web UI bridge로 전환 |
| SPEC-GOOSE-ONBOARDING-001 | 이메일 가입 플로우 제거 → CLI + Web UI install wizard 기반 |
| SPEC-GOOSE-CORE-001 | 기술 스택 섹션에 `github.com/modu-ai/goose` module path, QMD 추가, storage 2원화 반영 |

## Additions (9 SPECs)

다음 SPEC은 **신규 생성** 필요. Milestone 배치 제안 포함.

| SPEC | Milestone | 내용 |
|---|---|---|
| SPEC-GOOSE-QMD-001 | M1 | QMD embedded (qntx-labs/qmd Rust, CGO staticlib). 인덱싱 정책, MCP 노출, reindex 트리거 |
| SPEC-GOOSE-WEBUI-001 | M6 | 비개발자용 localhost Web UI (설치/관리/대화 GUI) |
| SPEC-GOOSE-SELF-CRITIQUE-001 | M3 | Task self-critique loop + daily reflection (기존 REFLECT-001 = 5단계 승격 파이프라인과 구분) |
| SPEC-GOOSE-PAI-CONTEXT-001 | M7 | PAI 11 identity files 관리 (mission/goals/projects/beliefs/models/strategies/narratives/learned/challenges/ideas + growth). 기존 CONTEXT-001 (Context Window 관리) 및 IDENTITY-001 (Identity Graph Phase 8)과 구분 |
| SPEC-GOOSE-RITUAL-001 | M7 | Adaptive Ritual 엔진 (날씨/기분/요일/성장단계 반영) |
| SPEC-GOOSE-PERMISSION-001 | M2 | 선언 기반 permission (Skill/MCP frontmatter `requires`) + first-call confirm |
| SPEC-GOOSE-SECURITY-SANDBOX-001 | M5 | OS-level sandbox (Seatbelt/Landlock+Seccomp/AppContainer) |
| SPEC-GOOSE-CREDENTIAL-PROXY-001 | M5 | Zero-knowledge `goose-proxy` 프로세스 (OS keyring + transport injection) |
| SPEC-GOOSE-FS-ACCESS-001 | M5 | Filesystem access matrix + `security.yaml` 해석 엔진 |
| SPEC-GOOSE-AUDIT-001 | M5 | Append-only audit.log (write/permission/sandbox events) |

(위 10건 중 9건이 신규, SPEC-GOOSE-PERMISSION-001은 이미 기존에 경량 형태로 언급되었을 경우 재편으로 분류)

---

## Milestone 재정렬

| M# | 이름 | 핵심 SPEC |
|---|---|---|
| M0 | Foundation | CORE-001 (완료) |
| M1 | Multi-LLM + QMD | CREDPOOL, ROUTER, ADAPTER, QMD-001, PROVIDER-FALLBACK |
| M2 | 4 Primitives | SKILLS, MCP, HOOK, SUBAGENT, PERMISSION-001 |
| M3 | Core Workflow | COMMAND, CLI, TUI, Plan-Run-Sync engine, SELF-CRITIQUE-001 |
| M4 | Self-Evolution | TRAJECTORY, COMPRESSOR, INSIGHTS, auto-skill 제안 |
| M5 | Safety (대폭 확장) | SAFETY-001, ROLLBACK-001, SECURITY-SANDBOX-001, CREDENTIAL-PROXY-001, FS-ACCESS-001, AUDIT-001 |
| M6 | Channels | TELEGRAM-001, WEBUI-001 (Email 제거) |
| M7 | Daily Companion v1.0 | RITUAL-001, BRIEFING, JOURNAL, PAI-CONTEXT-001, GROWTH |
| M8 | Deep Personalization | IDENTITY, VECTOR, LORA, Kuzu, Affective |
| M9 | Ecosystem v2.0 | plugin marketplace, additional channels |

SPEC 총량: 58 → 약 48 (삭제 5 + 재편 5 + 신규 9 = 순증 +4).

---

## Acceptance Criteria

### AC-ARCH-01 — 문서 저장
`.moai/design/goose-runtime-architecture-v0.2.md` 파일이 존재하고 본 SPEC과 일관된다.

### AC-ARCH-02 — 의존성 그래프 업데이트
`.moai/specs/IMPLEMENTATION-ORDER.md` 와 `.moai/specs/ROADMAP.md` 가 본 재편에 따라 업데이트된다. (별도 작업)

### AC-ARCH-03 — 5건 SPEC 삭제 반영
MOBILE-001, WIDGET-001, SYNC-001, CLOUD-001, DISCOVERY-001 SPEC 디렉토리가 제거되거나 DEPRECATED 마킹된다.

### AC-ARCH-04 — 9건 신규 SPEC 스켈레톤 생성
Additions 목록의 9개 SPEC이 스켈레톤 상태로 `.moai/specs/` 에 생성된다.

### AC-ARCH-05 — CORE-001 SPEC 업데이트
SPEC-GOOSE-CORE-001 "기술 스택" 섹션에 storage 2원화, QMD, `github.com/modu-ai/goose`, `goose-proxy` 가 반영된다.

### AC-ARCH-06 — go.mod module path 변경
`go.mod` 의 module이 `github.com/moai/goose` → `github.com/modu-ai/goose` 로 변경되고 모든 import가 업데이트된다. 기존 테스트 전부 통과.

---

## Follow-up Tasks

이 meta-SPEC이 승인되면 다음 실행 단계:

1. **Task: SPEC 삭제** (5건) — git rm + ROADMAP 업데이트
2. **Task: SPEC 재편** (5건) — 기존 본문 스코프 재작성
3. **Task: SPEC 신규 생성** (9건) — EARS 요구사항 작성
4. **Task: go.mod module path 변경** + import 일괄 치환
5. **Task: SPEC-GOOSE-CORE-001 업데이트**
6. **Task: ROADMAP + IMPLEMENTATION-ORDER 재작성**

각 Task는 별도 커밋 + 독립 검증.

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| 대규모 SPEC 재편 중 일관성 깨짐 | 각 SPEC 변경을 단일 커밋으로 격리, `plan-auditor` 검증 |
| module path 변경으로 기존 커밋 코드 깨짐 | 별도 commit + 전체 테스트 스위트 통과 확인 |
| QMD Rust crate 빌드 복잡도 | CGO staticlib 빌드 검증 SPEC-GOOSE-QMD-001에 포함 |
| OS sandbox 구현 크로스플랫폼 비용 | macOS Seatbelt 우선, Linux/Windows는 후속 |

---

## References

- Architecture Document: `.moai/design/goose-runtime-architecture-v0.2.md`
- 5-round Socratic interview transcripts (session log)
- arXiv 2602.10479 · 2510.25445 (agentic AI surveys)
- NousResearch/hermes-agent v0.10 docs
- tobi/qmd + qntx-labs/qmd
- Claude Code Permission Docs
- NVIDIA AI Red Team / Northflank / agentkernel (sandboxing)
