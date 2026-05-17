---
id: SPEC-GOOSE-PERMISSION-001
version: 0.2.0
status: completed
completed: 2026-04-27
created_at: 2026-04-24
updated_at: 2026-04-27
author: manager-spec
priority: P0
issue_number: null
phase: 2
milestone: M2
size: 중(M)
lifecycle: spec-anchored
labels: [permission, security, declarative, first-call-confirm, phase-2, primitive/permission]
---

# SPEC-GOOSE-PERMISSION-001 — Declared Permission + First-Call Confirm (Skill/MCP Manifest 기반)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-24 | 초안 작성 (SPEC-GOOSE-ARCH-REDESIGN-v0.2 Tier 5 기반) — 5 REQ + 5 AC 스켈레톤 | architecture-redesign-v0.2 |
| 0.2.0 | 2026-04-26 | 본격 plan 보강 — Tier 5 합의 반영, 18+ EARS REQ + 10+ AC + 5 Phase 구현 계획 + Public API surface + Test Plan + Dependency map 신설. 기존 v0.1.0 5개 REQ는 모두 v0.2.0 분류 체계로 재흡수(REQ-PE-005/006/008/009/013/017에 매핑); 본문/식별자만 변경되고 의미는 보존. v0.1.0 AC-PERMISSION-01..05는 AC-PE-001/002/004/005/008로 재배치. labels 채움, lifecycle을 spec-first → spec-anchored로 격상. | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT의 **선언 기반 권한 시스템(Declared Permission)**을 정의한다. AI.MINK가 외부 자원(network, filesystem read/write, exec)에 접근하려면 해당 능력을 제공하는 `Skill` / `MCP server` / `Agent` 정의의 frontmatter에 `requires:` 필드로 권한을 **사전 선언**해야 하며, 사용자는 첫 호출 시점에 허용/거절을 선택하고 그 결정은 영속 grant store에 기록되어 이후 재사용된다.

본 SPEC이 통과한 시점에서 `internal/permission` 패키지는:

- `requires:` 스키마(net / fs_read / fs_write / exec 4 카테고리, 각 카테고리별 scope 토큰)를 frontmatter에서 파싱하고 검증하며,
- `Confirmer` 인터페이스를 통해 첫 호출 시점에 `[항상 허용 / 이번만 / 거절]` 3-way 선택을 외부(orchestrator, AskUserQuestion 경로)에 위임하고,
- `Store` 인터페이스(JSON 파일 + 향후 SQLite 백엔드 가능)를 통해 grant를 `(subject_id, capability)` 키로 영속화하며,
- `Auditor`를 통해 모든 grant/revoke/denied 이벤트를 SPEC-GOOSE-AUDIT-001의 append-only audit.log로 dispatch하고,
- `mink permission list/revoke <subject>` CLI 표면을 제공하며,
- Sub-agent가 부모의 grant를 명시 플래그(`inherit_grants: true`) 시에만 상속하도록 SUBAGENT-001과 통합한다.

본 SPEC은 **OS 수준 sandbox 강제 실행을 포함하지 않는다** (그것은 SECURITY-SANDBOX-001). 본 SPEC은 "선언 → 첫호출 확인 → grant 영속화 → 재사용/취소"의 **policy layer**만 책임진다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- ARCH-REDESIGN-v0.2 §5 5-tier defense-in-depth 모델의 **Tier 5 (가장 바깥쪽 policy 레이어)**에 해당. Tier 1~4(Storage Partition / FS Access Matrix / OS Sandbox / Zero-Knowledge Proxy)가 **기술적 강제 메커니즘**이라면 Tier 5는 **사용자 동의 메커니즘**으로, 대부분의 사용자가 첫 접점으로 만나게 된다.
- SKILLS-001(M2 row 11)·MCP-001(M2 row 12)·SUBAGENT-001(M2 row 13)이 모두 frontmatter `requires:` 필드 자리만 비워두고 본 SPEC 완료를 기다리는 상태. M2 critical path의 join point.
- Claude Code 동등 기능 비교: Claude Code는 `tools/`, `mcpServers/`, plugin manifest의 `permissions:` 필드와 첫 호출 시 권한 prompt(키보드 인터랙션) + `~/.claude/permissions/` grant 영속화로 동일 패턴을 운용한다. AI.MINK는 이를 (a) 4-카테고리 명시 분류, (b) AskUserQuestion 기반 3-way 선택, (c) grant scope inheritance 명시화로 **이디엄에 맞춰 재해석**한다.

### 2.2 상속 자산 (패턴만 계승)

- **Claude Code 권한 모델**(https://docs.claude.com/en/docs/claude-code/iam): `tools/Bash/permissions.ts`, `permissions/` storage, first-call confirm UX. 직접 포팅 없음 — manifest schema와 grant key 정책만 차용.
- **MoAI-ADK `.claude/skills/*/SKILL.md`**: 본 레포의 26개 skill 정의 파일은 `requires:` 필드를 아직 사용하지 않으나, 본 SPEC의 v0.2.0+ 마이그레이션 가이드라인이 후속 작업으로 적용 가능.
- **OWASP IAM 권고**: Default Deny, Least Privilege, Defense in Depth — 본 SPEC의 모든 정책 결정에 가이드라인으로 반영.
- **Hermes Agent**: 권한 시스템 없음. 본 SPEC은 Hermes 자산 재사용 없음.

### 2.3 범위 경계

- **IN**: `requires:` frontmatter 스키마(4 카테고리 + scope 표기법), `RequiresParser`, `Grant` 타입, `Store` 인터페이스 + 파일 백엔드(`~/.goose/permissions/grants.json` + project-local override), `Confirmer` 인터페이스(orchestrator가 AskUserQuestion으로 구현), `Auditor` 인터페이스(AUDIT-001 forwarding), CLI 표면(`mink permission list/revoke/show`), Sub-agent inheritance flag, blocked_always 항목과의 교차 차단(security.yaml 연계), expiry/TTL 옵션.
- **OUT**: OS-level sandbox 강제(SECURITY-SANDBOX-001), filesystem path resolution + glob 매칭(FS-ACCESS-001), audit log 자체 구현(AUDIT-001), credential value 접근 제어(CREDENTIAL-PROXY-001), rate limiting / quota, network policy(예: 특정 도메인 차단), MFA / 생체 인증, 권한 위임(delegation) 모델, plugin marketplace에서의 manifest 공급망 검증(PLUGIN-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/permission/` 패키지: `schema`(파서) / `store`(영속) / `confirmer`(인터페이스) / `auditor`(인터페이스) / `cli`(`mink permission` 서브커맨드).
2. `requires:` 스키마 정의: 4-카테고리(`net` / `fs_read` / `fs_write` / `exec`)와 각 카테고리별 scope 토큰. 예:
   ```yaml
   requires:
     net: ["api.openai.com", "*.anthropic.com"]   # host 또는 host glob
     fs_read: ["./.goose/**", "~/.cache/goose/**"] # path glob (FS-ACCESS-001 표기와 동일)
     fs_write: ["./.goose/memory/**"]
     exec: ["git", "go"]                           # binary 이름 화이트리스트
   ```
3. `RequiresParser.Parse(raw map[string]any) (Manifest, []error)` — frontmatter의 `requires` 키를 받아 `Manifest{NetHosts, FSReadPaths, FSWritePaths, ExecBinaries}`로 변환. 알 수 없는 카테고리는 `ErrUnknownCapability` 누적, 카테고리 내 스칼라 값(non-array)은 `ErrInvalidScopeShape`.
4. `Grant` 타입: `{ID, SubjectID, SubjectType("skill"|"mcp"|"agent"|"plugin"), Capability, Scope, GrantedAt, GrantedBy, ExpiresAt, Revoked}`. `Capability`는 4-카테고리 enum, `Scope`은 매칭된 manifest scope 토큰 1건(예: `"api.openai.com"`).
5. `Store` 인터페이스 + 기본 파일 백엔드:
   - `Lookup(subjectID, capability, scope) (Grant, bool)` — O(log n) 또는 인메모리 인덱스.
   - `Save(grant Grant) error` — atomic write(temp + rename), file mode 0600.
   - `Revoke(subjectID string) (revoked int, err error)` — 해당 subject의 모든 grant를 soft-delete(Revoked=true), append-only 의미 보존.
   - `List(filter Filter) ([]Grant, error)` — CLI consumer용.
   - 파일 위치: 기본 `~/.goose/permissions/grants.json`(global), project override 파일 `./.goose/permissions/grants.local.json`이 존재하면 우선.
6. `Confirmer` 인터페이스 — 첫 호출 시 사용자 동의 수집:
   ```go
   type Confirmer interface {
       Ask(ctx context.Context, req PermissionRequest) (Decision, error)
   }
   type Decision struct {
       Choice    DecisionChoice // AlwaysAllow | OnceOnly | Deny
       ExpiresAt *time.Time     // AlwaysAllow + TTL 시 사용
   }
   ```
   본 SPEC은 인터페이스만 정의 — 실 구현은 orchestrator(MoAI 측의 AskUserQuestion-backed adapter, daemon 측의 IPC 채널)가 별도로 제공.
7. `Auditor` 인터페이스 — 모든 결정 이벤트를 AUDIT-001 채널로 dispatch:
   ```go
   type Auditor interface {
       Record(event PermissionEvent) error  // event.Type ∈ {grant_created, grant_reused, grant_denied, grant_revoked}
   }
   ```
8. `Manager` (top-level facade) — `Check(ctx, req PermissionRequest) (Decision, error)`:
   - (a) blocked_always 교차 차단 검사(security.yaml의 `blocked_always` 목록 참조; 매치 시 `ErrBlockedByPolicy`),
   - (b) declared 검사(요청한 capability/scope이 manifest `requires:`에 선언되었는가; 미선언 시 `ErrUndeclaredCapability`),
   - (c) Store에서 기존 grant lookup,
   - (d) 미존재 시 Confirmer 호출,
   - (e) 결과 Store 저장 + Auditor.Record.
9. CLI 표면(`internal/cli/permission_cmd.go`):
   - `mink permission list [--subject <id>] [--capability net|fs_read|fs_write|exec]`
   - `mink permission show <subject_id>`
   - `mink permission revoke <subject_id>` — 해당 subject의 모든 grant revoke + 후속 호출 시 재확인 강제.
   - `mink permission gc` — expired grant 정리.
10. Sub-agent inheritance: SUBAGENT-001의 `AgentDefinition`에 `inherit_grants: bool` 추가(본 SPEC에서 명세, 실 구현은 SUBAGENT-001 v0.4 amendment에서). 기본값 `false`(부모 grant 미상속). 명시적 `true` 시 sub-agent가 부모의 `subjectID`를 `parent_subject_id`로 받아 lookup 시 fallback chain 형성.
11. Expiry 정책: `Grant.ExpiresAt`이 non-nil이고 `time.Now() > ExpiresAt`이면 `Lookup`은 `(grant, false)` 반환(expired 취급), `permission gc`가 정리.
12. `requires:` 충돌 해결(merge): 동일 subject 정의가 2회 로드되면(예: skill upgrade) `Manifest`는 union 합집합으로 합쳐지되 신규 capability/scope는 신규 grant 요구로 처리. 축소(이전 manifest에 있던 scope이 사라짐)는 grant invalidation 트리거 — 다음 호출에서 재확인.

### 3.2 OUT OF SCOPE (명시적 제외)

- **OS-level sandbox 강제(seatbelt / Landlock+seccomp / AppContainer)**: SECURITY-SANDBOX-001. 본 SPEC은 policy decision만 — 실제 syscall 차단은 별도 layer.
- **fs_read/fs_write path glob 해석/blocked_always 매칭 엔진**: FS-ACCESS-001. 본 SPEC은 scope 토큰을 string으로 보유하고 매칭은 FS-ACCESS-001의 `Matcher`에 위임.
- **audit.log 본체 구현 (rotation / append-only enforcement)**: AUDIT-001. 본 SPEC은 `Auditor.Record(event)` 호출까지만.
- **rate limiting / quota / per-second call limit**: 후속 SPEC.
- **network policy enforcement(특정 IP 차단, mTLS 강제)**: 후속 SPEC.
- **MFA / 생체 인증 / 디바이스 바인딩**: 후속 SPEC.
- **권한 위임(delegation, A에게 받은 grant를 B가 사용)** 모델: 본 SPEC은 명시 inheritance flag 외 위임 없음. 후속 SPEC.
- **plugin marketplace 공급망 검증(서명 / SLSA)**: PLUGIN-001.
- **AskUserQuestion UI 자체**: 본 SPEC은 `Confirmer` 인터페이스만 정의. CLI / TUI / Web UI별 구현은 각 channel SPEC.

---

## 4. EARS 요구사항 (Requirements)

> §4의 REQ-PE-NNN은 §6.1 Public API surface(Go interface 시그니처)를 정본으로 참조한다. 본 SPEC의 카테고리 그룹화 정책: §4.1 Ubiquitous → §4.2 Event-Driven → §4.3 State-Driven → §4.4 Unwanted → §4.5 Optional 순서로 배치한다. 식별자는 단조 증가(001..018) 중복·결번 없음.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-PE-001 [Ubiquitous]** — Every `Skill` / `MCP server` / `Agent` / `Plugin` definition that requests access to network, filesystem read, filesystem write, or process exec **shall** declare those capabilities in its frontmatter `requires:` field with at least one scope token per used category; manifests that exercise an undeclared capability at runtime **shall** cause `Manager.Check` to return `ErrUndeclaredCapability` and **shall not** trigger user confirmation. (Default-Deny — undeclared = unconfirmable.)

**REQ-PE-002 [Ubiquitous]** — The `requires:` schema **shall** recognize exactly 4 capability categories: `net` (host strings or host globs), `fs_read` (path globs), `fs_write` (path globs), and `exec` (binary basenames); unknown categories **shall** cause `RequiresParser.Parse` to append `ErrUnknownCapability` to its error slice without aborting the parse, consistent with partial-success semantics in SKILLS-001 REQ-SK-005.

**REQ-PE-003 [Ubiquitous]** — Every `Grant` persisted via `Store.Save` **shall** carry a unique `ID` (UUIDv4 string), the granting `SubjectID`, `SubjectType ∈ {skill, mcp, agent, plugin}`, the resolved `Capability`, the matched `Scope` token (single value, not the full manifest list), `GrantedAt` (RFC3339), and an optional `ExpiresAt` (nil = never expires until explicit revoke).

**REQ-PE-004 [Ubiquitous]** — The `Store` file backend **shall** persist `~/.goose/permissions/grants.json` (global) and **shall** prefer `./.goose/permissions/grants.local.json` (project-local) when present; both files **shall** be created with file mode `0600` and parent directory mode `0700`. The process **shall** refuse to read a grant file whose mode exceeds `0600` and **shall** log a security warning, mirroring MCP-001 REQ-MCP-003 file-mode policy.

**REQ-PE-005 [Ubiquitous]** — All `Manager.Check` invocations **shall** dispatch a `PermissionEvent` to `Auditor.Record` regardless of outcome; events **shall** include at minimum `{type, subject_id, capability, scope, decision, ts}`. Failure of the auditor **shall not** mask the policy decision (decision is returned even if audit fails) but **shall** be surfaced as a non-fatal log warning. (v0.1.0 의 REQ-PERMISSION 5 audit 의무를 시스템 불변으로 격상.)

### 4.2 Event-Driven (이벤트 기반)

**REQ-PE-006 [Event-Driven]** — **When** `Manager.Check(ctx, req)` is invoked for the first time on a `(SubjectID, Capability, Scope)` triple — i.e., `Store.Lookup` returns `(_, false)` — the manager **shall** invoke `Confirmer.Ask(ctx, req)` and **shall** emit one of three terminal outcomes based on the returned `Decision.Choice`: (a) `AlwaysAllow` → create a Grant via `Store.Save`, return `Allow`; (b) `OnceOnly` → return `Allow` without persisting (single-use); (c) `Deny` → return `Deny` without persisting. The Confirmer **shall** be invoked exactly once per first-call gate; concurrent first-call attempts on the same triple **shall** be serialized via per-triple mutex (REQ-PE-016). (v0.1.0 REQ-PERMISSION-001 흡수.)

**REQ-PE-007 [Event-Driven]** — **When** `Manager.Check` finds an existing non-revoked, non-expired grant in `Store` matching `(SubjectID, Capability, Scope)`, the manager **shall** return `Allow` immediately (no Confirmer invocation), **shall** record an event of type `grant_reused`, and **shall** complete within 5 ms p95 (excluding audit dispatch). (v0.1.0 REQ-PERMISSION-002 흡수.)

**REQ-PE-008 [Event-Driven]** — **When** `mink permission revoke <subject_id>` is invoked, the CLI **shall** call `Store.Revoke(subjectID)` which (a) marks all matching grants `Revoked=true` with `RevokedAt = now`, (b) atomic-writes the store, (c) returns the revoked count, and (d) emits one `grant_revoked` audit event per affected grant. The next `Manager.Check` for that subject **shall** trigger `Confirmer.Ask` again (re-confirmation enforced). (v0.1.0 REQ-PERMISSION-004 흡수.)

**REQ-PE-009 [Event-Driven]** — **When** a manifest declares a scope token whose normalized form matches an entry in the security policy's `blocked_always` list (loaded from `./.goose/config/security.yaml` per FS-ACCESS-001), `Manager.Check` **shall** return `ErrBlockedByPolicy` and **shall** record an audit event of type `grant_denied` with `reason: "blocked_always"`; the Confirmer **shall not** be invoked, and no grant **shall** be created — even if the user attempted to override. The blocked_always list is FROZEN per ARCH-REDESIGN-v0.2 REQ-ARCH-005. (v0.1.0 REQ-PERMISSION-005 흡수.)

**REQ-PE-010 [Event-Driven]** — **When** `RequiresParser.Parse` encounters a manifest in which a category value is a non-array scalar (e.g., `net: "api.openai.com"` instead of `net: ["api.openai.com"]`), the parser **shall** append `ErrInvalidScopeShape{category, value}` to its error slice, **shall not** silently coerce the scalar to a single-element array, and **shall** return a Manifest with that category set to nil. (Strict shape — prevents accidental wildcard-by-omission.)

**REQ-PE-011 [Event-Driven]** — **When** a `Subagent` is spawned with `def.InheritGrants == true`, the manager invocations on behalf of that sub-agent **shall** apply a fallback lookup chain: first try `(childSubjectID, capability, scope)`, on miss try `(parentSubjectID, capability, scope)`. If a parent grant matches, the manager **shall** treat it as `Allow` and **shall** record `grant_reused` with `inherited_from: parentSubjectID` in the audit event. Default `InheritGrants == false`. (Cross-reference SUBAGENT-001 §3.1.)

### 4.3 State-Driven (상태 기반)

**REQ-PE-012 [State-Driven]** — **While** a manifest is mid-load (post-parse, pre-registry-publish), `Manager.Check` against that subject **shall** return `ErrSubjectNotReady`; a subject becomes available only after the loader (SKILLS-001 / MCP-001 / SUBAGENT-001) calls `Manager.Register(subjectID, manifest)`.

**REQ-PE-013 [State-Driven]** — **While** an existing grant's `ExpiresAt` is non-nil and `time.Now().After(ExpiresAt)` is true, `Store.Lookup` **shall** return `(grant, false)` (treat as miss), **shall not** silently update the grant, and **shall** allow the next `Manager.Check` to re-trigger the Confirmer flow. Expired grants are cleaned up on `mink permission gc` invocation (not eagerly).

**REQ-PE-014 [State-Driven]** — **While** the store file is being mutated (Save/Revoke), readers via `Lookup` **shall** see either the prior state or the new state — never a partial state. Implementation **shall** use atomic temp-file write + rename and protect in-memory index with `sync.RWMutex`; cross-process writers (multi-`goosed` instances) are out of scope (single-process assumption matching CREDPOOL-001 §3.2).

### 4.4 Unwanted Behavior (방지)

**REQ-PE-015 [Unwanted]** — `Manager.Check` **shall not** return `Allow` for a capability/scope combination not present in the subject's registered `Manifest`, even if a matching grant exists in the store from a prior version of the manifest. Manifest contraction (a previously-declared scope token disappears in the new manifest) **shall** invalidate corresponding grants on the next `Register` call, requiring re-confirmation — preventing privilege escalation via grant retention across manifest edits.

**REQ-PE-016 [Unwanted]** — Concurrent `Manager.Check` calls for the same `(SubjectID, Capability, Scope)` triple while the Confirmer is in flight **shall not** invoke the Confirmer more than once. Implementation: per-triple `sync.Mutex` keyed on `hash(subjectID + capability + scope)`. The second waiter **shall** observe the first waiter's persisted decision (Allow → returns Allow; Deny → returns Deny once-only).

**REQ-PE-017 [Unwanted]** — The `Store` file backend **shall not** silently accept a grants.json file whose JSON schema version (`schema_version` field) does not match the current code version. Mismatches **shall** cause `Store.Open` to return `ErrIncompatibleStoreVersion` with the file's version embedded; migration is operator-driven via `mink permission migrate --from <ver>` (out of scope for this SPEC; placeholder error only).

**REQ-PE-018 [Unwanted]** — `RequiresParser.Parse` **shall not** allow nested `requires:` nesting (e.g., `requires.requires.net: [...]`); flat 4-category structure is the only accepted shape. Nested structures **shall** cause `ErrInvalidScopeShape` with `nested: true` annotation.

### 4.5 Optional (선택적)

**REQ-PE-019 [Optional]** — **Where** the user passes `--ttl <duration>` to a confirmation prompt (orchestrator-side enrichment of `Decision`), the resulting Grant **shall** carry `ExpiresAt = now + duration`; default (no TTL) is `nil` (never expires). Consumers (CLI / TUI) decide how to surface the TTL knob.

**REQ-PE-020 [Optional]** — **Where** `subject_type == "plugin"` and the plugin's manifest is loaded from a marketplace (PLUGIN-001 future scope), the manager **may** require an additional integrity check (signature / hash) before invoking the Confirmer. Default behavior: integrity check is a no-op stub returning `ok` until PLUGIN-001 wires its verification routine. This REQ documents the extension point only.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then. `*_test.go`로 변환 가능한 수준. 마지막 줄에 `Covers: REQ-PE-XXX, ...` 메타라인을 포함하여 RTM 자동 추적을 보장한다.

**AC-PE-001 — `requires:` 4-카테고리 스키마 파싱 + 알려지지 않은 카테고리 + 잘못된 형태 누적 에러 (v0.1.0 AC-PERMISSION-01 재배치)**
- **Given** YAML frontmatter 3종 fixture: (a) 정상 4-카테고리 + 미지 카테고리 1종, (b) 스칼라 형태(`net: "api.openai.com"`), (c) 중첩 형태(`requires: {requires: {net: [...]}}`)
  ```yaml
  # fixture (a)
  requires:
    net: ["api.openai.com"]
    fs_read: ["./.goose/**"]
    exec: ["git"]
    misc: ["something"]
  ```
- **When** 각 fixture에 대해 `RequiresParser.Parse(raw)` 호출
- **Then** (a) 반환 `Manifest.NetHosts == ["api.openai.com"]`, `FSReadPaths == ["./.goose/**"]`, `ExecBinaries == ["git"]`, `FSWritePaths == nil`. Error slice에 `ErrUnknownCapability{key:"misc"}` 1건 포함. (b) Error slice에 `ErrInvalidScopeShape{category:"net", value:"api.openai.com"}` 포함, `Manifest.NetHosts == nil` (스칼라 silent coercion 금지). (c) Error slice에 `ErrInvalidScopeShape{nested:true}` 포함, `Manifest`의 모든 필드 nil
- **Covers**: REQ-PE-002, REQ-PE-010, REQ-PE-018

**AC-PE-002 — 첫 호출 시 Confirmer 호출 → AlwaysAllow → grant 영속화 (v0.1.0 AC-PERMISSION-02 재배치)**
- **Given** 신규 subject `skill:my-tool` (manifest declared `net: ["api.openai.com"]`), `Store`는 비어 있음, `Confirmer` stub이 `Decision{Choice: AlwaysAllow}` 반환
- **When** `Manager.Check(ctx, req{SubjectID:"skill:my-tool", Capability:CapNet, Scope:"api.openai.com"})` 호출
- **Then** (a) `Confirmer.Ask` 정확히 1회 호출, (b) `Store.Lookup` 후속 호출에서 grant 발견됨(`Revoked == false`, `GrantedAt ≈ now`), (c) 반환 `Decision{Allow:true}`, (d) `Auditor.Record`가 `type: grant_created` 이벤트 1건 받음
- **Covers**: REQ-PE-006, REQ-PE-003, REQ-PE-005

**AC-PE-003 — 두 번째 호출은 Confirmer 없이 reuse**
- **Given** AC-PE-002 종료 직후 상태 (grant 존재)
- **When** 동일 `(SubjectID, Capability, Scope)`로 `Manager.Check` 재호출
- **Then** (a) `Confirmer.Ask` 호출 0회, (b) 반환 `Allow`, (c) `Auditor.Record`가 `type: grant_reused` 이벤트 1건 받음, (d) p95 latency 5 ms 이내(테스트 100회 반복 측정)
- **Covers**: REQ-PE-007

**AC-PE-004 — 미선언 capability 호출 차단 (v0.1.0 AC-PERMISSION-03 재배치/재정의)**
- **Given** subject `skill:my-tool`의 manifest에는 `net: ["api.openai.com"]`만 선언, `Store`에 grant 존재 여부 무관
- **When** `Manager.Check(ctx, req{Capability:CapFSWrite, Scope:"./.goose/data/**"})` 호출 (선언되지 않은 capability)
- **Then** 반환 `(_, ErrUndeclaredCapability)`, `Confirmer.Ask` 호출 0회, `Auditor.Record`가 `type: grant_denied, reason:"undeclared"` 1건 받음
- **Covers**: REQ-PE-001, REQ-PE-005

**AC-PE-005 — Revoke 후 재확인 강제 (v0.1.0 AC-PERMISSION-04 재배치)**
- **Given** AC-PE-002 후 grant 존재 상태에서 `mink permission revoke skill:my-tool` CLI 실행
- **When** (a) `Store.Revoke("skill:my-tool")` 반환값 확인, (b) 동일 subject에 대해 `Manager.Check` 재호출
- **Then** (a) revoked 카운트 == 1, `Auditor.Record`가 `type: grant_revoked` 1건 받음, (b) 후속 `Manager.Check`에서 `Confirmer.Ask`가 다시 호출됨(이전 grant는 `Revoked==true`로 lookup 미스), (c) `Store.List({IncludeRevoked:true})`에는 해당 grant가 `Revoked:true`로 남아있음(append-only 의미 보존)
- **Covers**: REQ-PE-008

**AC-PE-006 — blocked_always override 불가 (v0.1.0 AC-PERMISSION-05 재배치/강화)**
- **Given** `~/.goose/config/security.yaml`의 `blocked_always: ["~/.ssh/**"]`, manifest가 `fs_read: ["~/.ssh/**"]`을 포함
- **When** subject 등록 후 `Manager.Check(ctx, req{Capability:CapFSRead, Scope:"~/.ssh/id_rsa"})` 호출 (사용자가 강제 시도)
- **Then** 반환 `(_, ErrBlockedByPolicy)`, `Confirmer.Ask` 호출 0회 (사용자 동의로도 우회 불가), `Auditor.Record`가 `type: grant_denied, reason:"blocked_always"` 1건 받음. `Store`에 grant 0건 추가
- **Covers**: REQ-PE-009

**AC-PE-007 — Grant 파일 mode 0644 거부**
- **Given** `~/.goose/permissions/grants.json`이 chmod 0644로 생성됨, `Store.Open` 호출
- **When** `Store.Open()` 실행
- **Then** `ErrStoreFilePermissions` 반환, 메모리 인덱스 미생성, zap WARN 로그(`level=WARN`, `msg="permission grant file mode exceeds 0600"`, `path` + `mode` 필드 포함) 1건. 후속 `Manager.Check`는 `ErrStoreNotReady` 반환
- **Covers**: REQ-PE-004

**AC-PE-008 — 동시 첫 호출 직렬화 (Confirmer 단일 호출 보장, v0.1.0 AC-PERMISSION-02 강화)**
- **Given** 신규 subject `skill:race-tool`, 동일 `(Capability:CapNet, Scope:"api.openai.com")` triple에 대해 10개 goroutine이 동시에 `Manager.Check` 호출, `Confirmer` stub이 100ms 지연 후 `AlwaysAllow` 반환
- **When** 10개 goroutine 동시 시작
- **Then** (a) `Confirmer.Ask` 호출 정확히 1회, (b) 10개 goroutine 모두 `Allow` 반환, (c) `Store`에 grant 1건만 존재, (d) `Auditor.Record`가 `grant_created` 1건 + `grant_reused` 9건 받음 (직렬화된 두 번째 이후는 reuse)
- **Covers**: REQ-PE-016, REQ-PE-006

**AC-PE-009 — Manifest contraction 시 grant 무효화**
- **Given** subject `skill:foo`가 manifest A `net: ["a.com", "b.com"]`로 등록되고 두 scope 모두 grant 영속화. 이후 manifest B(`net: ["a.com"]`만 선언)로 `Manager.Register("skill:foo", B)` 재등록
- **When** `Manager.Check(ctx, req{SubjectID:"skill:foo", Capability:CapNet, Scope:"b.com"})` 호출
- **Then** (a) 반환 `(_, ErrUndeclaredCapability)` (manifest contraction으로 b.com이 declared scope에서 제거됨), `Confirmer.Ask` 호출 0회, (b) 동일 호출을 `a.com`으로 하면 기존 grant 재사용으로 `Allow` 반환
- **Covers**: REQ-PE-015

**AC-PE-010 — Sub-agent inheritance 동작**
- **Given** parent agent `agent:planner`가 `net: ["api.openai.com"]` grant 보유, child agent `agent:planner-child`가 `def.InheritGrants == true`로 spawn
- **When** child의 manager invocation `Manager.Check(ctx, req{SubjectID:"agent:planner-child", Capability:CapNet, Scope:"api.openai.com", ParentSubjectID:"agent:planner"})` 호출
- **Then** (a) child grant lookup 미스 → parent fallback hit, (b) 반환 `Allow`, `Confirmer.Ask` 호출 0회, (c) `Auditor.Record`가 `type: grant_reused, inherited_from: "agent:planner"` 1건 받음. 별도 케이스로 `def.InheritGrants == false`이면 child에 대해 `Confirmer.Ask`가 새로 호출됨
- **Covers**: REQ-PE-011

**AC-PE-011 — Expired grant는 lookup 미스 + gc로 정리**
- **Given** subject `skill:bar` grant가 `ExpiresAt = now - 1h`(이미 만료) 상태로 store에 영속
- **When** (a) `Manager.Check` 호출, (b) 별도로 `mink permission gc` CLI 실행
- **Then** (a) `Confirmer.Ask` 새로 호출됨(expired는 lookup 미스 취급), (b) `gc` 실행 후 `Store.List({IncludeRevoked:true})`에서 해당 만료 grant가 제거됨(또는 `Revoked:true`로 표시됨, 구현 선택), 반환된 `pruned` 카운트 ≥ 1
- **Covers**: REQ-PE-013

**AC-PE-012 — Atomic write race-free**
- **Given** `Store`의 file backend, 10개 goroutine이 각기 다른 subject에 대해 `Store.Save` 100회씩 동시 호출 (총 1000 write)
- **When** 모든 goroutine 종료 후 `~/.goose/permissions/grants.json`을 raw 재파싱
- **Then** (a) JSON 파싱 성공(torn line / 손상 0), (b) 1000건 중 마지막 write의 grants가 모두 파일에 존재(또는 인덱스에 일관됨), (c) `go test -race` 통과, (d) 파일 권한 0600 유지
- **Covers**: REQ-PE-014, REQ-PE-004

**AC-PE-013 — Manifest 미등록 subject에 대한 Check은 NotReady**
- **Given** `Manager.Register("skill:future-tool", ...)` 미호출 상태
- **When** `Manager.Check(ctx, req{SubjectID:"skill:future-tool", ...})` 호출
- **Then** 반환 `(_, ErrSubjectNotReady)`, `Confirmer.Ask` / `Store.Lookup` 호출 0회
- **Covers**: REQ-PE-012

**AC-PE-014 — `mink permission list` / `show` CLI surface**
- **Given** Store에 3개 subject(`skill:a`, `mcp:b`, `agent:c`)의 grants 5건 존재
- **When** (a) `mink permission list` 실행, (b) `mink permission list --capability net` 실행, (c) `mink permission show skill:a` 실행
- **Then** (a) 5건 모두 출력, (b) `net` capability grant만 필터링 출력, (c) `skill:a`의 grants와 manifest에 선언된 capabilities 요약 출력. 모두 exit code 0, stdout JSON 또는 사람-친화 표 옵션(`--format json|table`) 지원
- **Covers**: REQ-PE-008 (CLI 표면 검증; revoke과 동일 SubjectID 인덱스 사용)

**AC-PE-015 — Schema version mismatch 거부**
- **Given** `~/.goose/permissions/grants.json`이 `{"schema_version": 99, "grants": []}` 내용으로 존재 (현재 코드 schema_version=1)
- **When** `Store.Open()` 호출
- **Then** 반환 `ErrIncompatibleStoreVersion`이며 에러 메시지에 file의 `schema_version: 99`와 코드의 `expected: 1` 두 값 모두 포함, 메모리 인덱스 미생성, zap WARN 로그 1건. 후속 `Manager.Check`는 `ErrStoreNotReady` 반환. 마이그레이션 placeholder는 별도 검증 (`mink permission migrate --from 99` 호출 시 `not implemented` 에러 반환 — OI-02 본문 참조)
- **Covers**: REQ-PE-017

**AC-PE-016 — TTL 옵션 적용**
- **Given** 신규 subject `skill:ttl-tool`, Confirmer stub이 `Decision{Choice: AlwaysAllow, ExpiresAt: now + 1h}` 반환
- **When** `Manager.Check(ctx, req)` 호출 → grant 영속화 → 1시간 + 1초 후 동일 호출 재시도
- **Then** (a) 첫 호출 후 grant의 `ExpiresAt ≈ now + 1h`로 영속됨, (b) 1시간 + 1초 경과 후 호출에서 `Confirmer.Ask`가 다시 호출됨(`Store.Lookup`이 expired로 미스 처리, REQ-PE-013 참조), (c) 별도 케이스: Confirmer가 `ExpiresAt: nil` 반환 시 grant는 영구 (expired 처리 없음)
- **Covers**: REQ-PE-019

**AC-PE-017 — Plugin integrity check extension point**
- **Given** subject `plugin:future-pkg` (SubjectType=Plugin)에 대해 `Manager.SetIntegrityChecker(checker)` 호출, checker stub이 (a) `ok` 반환 / (b) `ErrIntegrityCheckFailed` 반환 두 케이스
- **When** 각각 `Manager.Check(ctx, req{SubjectType: SubjectPlugin, ...})` 호출
- **Then** (a) checker가 `ok` 반환 시 정상 흐름(declared → lookup → confirm) 진행, (b) checker가 실패 반환 시 `ErrIntegrityCheckFailed` 반환되어 Confirmer 호출 0회, audit `type: grant_denied, reason:"integrity_check_failed"` 1건. checker가 nil(default no-op)이면 모든 plugin 호출이 정상 흐름 진행 (v0.2.0 default behavior)
- **Covers**: REQ-PE-020

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
internal/permission/
├── manifest.go                # Manifest, Capability, Scope 타입 + RequiresParser
├── manifest_test.go
├── grant.go                   # Grant, DecisionChoice, PermissionRequest, PermissionEvent
├── store/
│   ├── store.go               # Store interface
│   ├── file.go                # 파일 backend (atomic JSON write, 0600)
│   ├── file_test.go           # 동시성 + permissions race
│   └── memory.go              # 테스트용 in-memory backend
├── confirmer.go               # Confirmer interface (실 구현은 외부)
├── auditor.go                 # Auditor interface (AUDIT-001 forward)
├── manager.go                 # Manager facade — Check / Register / Revoke
├── manager_test.go
└── errors.go                  # ErrUnknownCapability / ErrUndeclaredCapability / ErrBlockedByPolicy / ...

internal/cli/
└── permission_cmd.go          # mink permission [list|show|revoke|gc|migrate]
```

### 6.2 Public API Surface (Go 시그니처)

```go
// internal/permission/manifest.go

type Capability int

const (
    CapNet     Capability = iota  // network host access
    CapFSRead                     // filesystem read
    CapFSWrite                    // filesystem write
    CapExec                       // process exec
)

func (c Capability) String() string  // "net" | "fs_read" | "fs_write" | "exec"

// Manifest는 frontmatter `requires:` 의 파싱 결과.
type Manifest struct {
    NetHosts      []string  // host literal or glob (e.g., "*.anthropic.com")
    FSReadPaths   []string  // path glob (FS-ACCESS-001 표기 호환)
    FSWritePaths  []string
    ExecBinaries  []string  // basename 화이트리스트
}

// RequiresParser는 frontmatter raw map → Manifest 변환.
// 알 수 없는 카테고리는 errs에 누적, 4 카테고리는 가능한 만큼 채움.
type RequiresParser struct{}

func (p *RequiresParser) Parse(raw map[string]any) (Manifest, []error)
```

```go
// internal/permission/grant.go

type DecisionChoice int

const (
    DecisionAlwaysAllow DecisionChoice = iota
    DecisionOnceOnly
    DecisionDeny
)

type SubjectType string

const (
    SubjectSkill  SubjectType = "skill"
    SubjectMCP    SubjectType = "mcp"
    SubjectAgent  SubjectType = "agent"
    SubjectPlugin SubjectType = "plugin"
)

type PermissionRequest struct {
    SubjectID       string       // "skill:my-tool" or "mcp:github" or "agent:planner"
    SubjectType     SubjectType
    ParentSubjectID string       // sub-agent inheritance용 (REQ-PE-011)
    Capability      Capability
    Scope           string       // 매칭된 manifest scope 토큰 1건
    RequestedAt     time.Time
}

type Grant struct {
    ID          string       // UUIDv4
    SubjectID   string
    SubjectType SubjectType
    Capability  Capability
    Scope       string
    GrantedAt   time.Time
    GrantedBy   string       // "user:goos" — Confirmer 측이 채움
    ExpiresAt   *time.Time   // nil = never
    Revoked     bool
    RevokedAt   *time.Time
}

type Decision struct {
    Allow     bool
    Choice    DecisionChoice  // Confirmer가 채운 값 그대로 회수 (감사용)
    ExpiresAt *time.Time
    Reason    string          // Deny 시 사람-친화 사유
}

type PermissionEvent struct {
    Type       string         // "grant_created" | "grant_reused" | "grant_denied" | "grant_revoked"
    SubjectID  string
    Capability Capability
    Scope      string
    Reason     string         // 거부 사유
    InheritedFrom string       // parent SubjectID (있을 때만)
    Timestamp  time.Time
}
```

```go
// internal/permission/store/store.go

type Filter struct {
    SubjectID      string       // exact match (empty = any)
    Capability     *Capability  // optional
    IncludeRevoked bool
    IncludeExpired bool
}

type Store interface {
    Open() error
    Lookup(subjectID string, cap Capability, scope string) (Grant, bool)
    Save(g Grant) error
    Revoke(subjectID string) (revoked int, err error)
    List(filter Filter) ([]Grant, error)
    GC(now time.Time) (pruned int, err error)
    Close() error
}
```

```go
// internal/permission/confirmer.go

type Confirmer interface {
    // Ask는 사용자 동의를 수집한다.
    // 본 SPEC은 인터페이스만 — 실 구현은 orchestrator(MoAI / daemon)가 제공.
    Ask(ctx context.Context, req PermissionRequest) (Decision, error)
}
```

```go
// internal/permission/auditor.go

type Auditor interface {
    Record(event PermissionEvent) error
}

// NoopAuditor는 테스트 / SPEC-AUDIT-001 미배선 환경용 폴백.
type NoopAuditor struct{}
func (NoopAuditor) Record(PermissionEvent) error { return nil }
```

```go
// internal/permission/manager.go

type Manager struct {
    store     Store
    confirmer Confirmer
    auditor   Auditor
    blocked   BlockedAlwaysMatcher  // FS-ACCESS-001 consumer (security.yaml)
    registry  map[string]Manifest    // SubjectID → Manifest
    mu        sync.RWMutex
    triplemu  *triplemap             // per-(subj,cap,scope) mutex (REQ-PE-016)
}

func New(store Store, confirmer Confirmer, auditor Auditor, blocked BlockedAlwaysMatcher) *Manager

// Register는 loader(SKILLS-001 / MCP-001 / SUBAGENT-001)가 호출.
// manifest contraction(REQ-PE-015) 시 grant 무효화 트리거.
func (m *Manager) Register(subjectID string, manifest Manifest) error

// Check은 모든 capability 진입점.
// @MX:ANCHOR — fan_in: SKILLS-001 / MCP-001 / SUBAGENT-001 / PLUGIN-001 모두가 호출.
func (m *Manager) Check(ctx context.Context, req PermissionRequest) (Decision, error)

// 운영자 API
func (m *Manager) Revoke(subjectID string) (revoked int, err error)
func (m *Manager) List(filter Filter) ([]Grant, error)
```

### 6.3 결정 흐름 의사코드

```
Check(ctx, req):
  // 1. Subject 등록 여부
  manifest, ok = registry[req.SubjectID]
  if !ok:
    audit(grant_denied, "not_ready"); return ErrSubjectNotReady

  // 2. blocked_always 교차 차단 (FROZEN policy, REQ-PE-009)
  if blocked.Matches(req.Capability, req.Scope):
    audit(grant_denied, "blocked_always"); return ErrBlockedByPolicy

  // 3. declared 검사 (REQ-PE-001 / REQ-PE-015)
  if !manifest.Declares(req.Capability, req.Scope):
    audit(grant_denied, "undeclared"); return ErrUndeclaredCapability

  // 4. per-triple lock (REQ-PE-016)
  unlock := triplemu.Lock(req.SubjectID, req.Capability, req.Scope)
  defer unlock()

  // 5. Store lookup (REQ-PE-007 + REQ-PE-013)
  grant, hit := store.Lookup(req.SubjectID, req.Capability, req.Scope)
  if !hit and req.ParentSubjectID != "" and inheritGrants(req):
    grant, hit = store.Lookup(req.ParentSubjectID, req.Capability, req.Scope)
    if hit: audit(grant_reused, inherited_from=parent); return Allow

  if hit:
    audit(grant_reused); return Allow

  // 6. First-call confirm (REQ-PE-006)
  decision, err = confirmer.Ask(ctx, req)
  if err: return Deny{reason:err}
  switch decision.Choice:
    case AlwaysAllow:
      g = newGrant(req, decision.ExpiresAt)
      store.Save(g); audit(grant_created); return Allow
    case OnceOnly:
      audit(grant_created, "once_only"); return Allow  // not persisted
    case Deny:
      audit(grant_denied, "user_denied"); return Deny
```

### 6.4 영속 파일 스키마 (`grants.json`)

```json
{
  "schema_version": 1,
  "grants": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "subject_id": "skill:my-tool",
      "subject_type": "skill",
      "capability": "net",
      "scope": "api.openai.com",
      "granted_at": "2026-04-26T10:30:00Z",
      "granted_by": "user:goos",
      "expires_at": null,
      "revoked": false,
      "revoked_at": null
    }
  ]
}
```

- 파일 권한: 0600, 부모 디렉토리 0700.
- atomic write: temp file `grants.json.tmp.{pid}` → fsync → rename.
- `schema_version: 1` 미스매치 시 `ErrIncompatibleStoreVersion` (REQ-PE-017).

### 6.5 5-Phase 구현 계획

| Phase | 범위 | 주요 산출물 | 의존 |
|-------|-----|----------|------|
| Phase 1 — Schema + Parser | `manifest.go` + `RequiresParser` + 4-카테고리 enum | RED #1, RED #10 통과 | Go 1.22+, gopkg.in/yaml.v3 (SKILLS-001 공유) |
| Phase 2 — Grant Store | `store/file.go` (atomic write) + `store/memory.go` (테스트) | RED #5, RED #7, RED #12 통과 | Phase 1 |
| Phase 3 — First-Call Confirm | `confirmer.go` + `manager.go` `Check` 핵심 + per-triple lock | RED #2, RED #3, RED #4, RED #8 통과 | Phase 1, 2 |
| Phase 4 — Audit + CLI | `auditor.go` + `permission_cmd.go` (list/show/revoke/gc) | RED #5, RED #11, RED #14 통과 | Phase 3, AUDIT-001 (interface stub OK) |
| Phase 5 — Inheritance + Manifest Contraction | sub-agent fallback chain + `Register` invalidation | RED #9, RED #10 통과 | Phase 3, SUBAGENT-001 (interface coordination) |

각 Phase는 별도 commit + 독립 검증. plan-auditor로 phase 간 누적 통합 확인.

### 6.6 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1** — `TestRequiresParser_FourCategories_PartialSuccess` (AC-PE-001)
2. **RED #2** — `TestManager_FirstCall_AlwaysAllow_PersistsGrant` (AC-PE-002)
3. **RED #3** — `TestManager_SecondCall_ReusesGrant_NoConfirmer` (AC-PE-003)
4. **RED #4** — `TestManager_UndeclaredCapability_Blocked` (AC-PE-004)
5. **RED #5** — `TestCLI_RevokeSubject_TriggersReConfirm` (AC-PE-005)
6. **RED #6** — `TestManager_BlockedAlways_NoOverride` (AC-PE-006)
7. **RED #7** — `TestStore_FilePermissions_0644Rejected` (AC-PE-007)
8. **RED #8** — `TestManager_ConcurrentFirstCall_SerializedSingleConfirm` (AC-PE-008)
9. **RED #9** — `TestManager_ManifestContraction_InvalidatesGrant` (AC-PE-009)
10. **RED #10** — `TestManager_SubagentInheritance_FallbackToParent` (AC-PE-010)
11. **RED #11** — `TestStore_ExpiredGrant_LookupMiss_AndGCPrunes` (AC-PE-011)
12. **RED #12** — `TestStore_AtomicWrite_RaceFree` (AC-PE-012)
13. **RED #13** — `TestManager_SubjectNotReady_NoConfirm` (AC-PE-013)
14. **RED #14** — `TestCLI_ListShow_FormatJSONAndTable` (AC-PE-014)
15. **GREEN** — Manager facade + Store file backend + CLI + audit dispatch.
16. **REFACTOR** — per-triple mutex 추출, Confirmer/Auditor 인터페이스 경계 정리, manifest contraction 검증을 별도 헬퍼로 분리.

### 6.7 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 14 AC → 14 Test, 동시성/race detector 필수, fixture 4-category 변종 + Confirmer stub 3가지(Allow/OnceOnly/Deny), coverage ≥ 85% |
| **R**eadable | 패키지 단일 책임(manifest/store/confirmer/auditor/manager 각 격리), 4-카테고리 enum + Capability.String() unified |
| **U**nified | `go fmt` + `golangci-lint`, atomic write 패턴 CREDPOOL-001 + MCP-001과 통일, file mode 0600 정책 통일 |
| **S**ecured | Default-Deny(undeclared = deny), blocked_always FROZEN(REQ-PE-009), file mode 0600 enforcement(REQ-PE-004), manifest contraction 시 grant invalidation(REQ-PE-015), per-triple race-free lock(REQ-PE-016) |
| **T**rackable | 모든 결정에 audit event(REQ-PE-005), `subject_id + capability + scope` 기반 zap 구조화 로그, `Auditor.Record` 실패도 로그 보존 |

---

## 7. Test Plan (보강)

본 SPEC의 검증은 4개 레벨로 구성된다:

### 7.1 Unit Tests
- `manifest_test.go`: 4-카테고리 + 알 수 없는 카테고리 + 스칼라 거부 + 중첩 거부 (REQ-PE-002, REQ-PE-010, REQ-PE-018)
- `store/file_test.go`: atomic write + concurrent Save + permission 0600 + schema_version mismatch (REQ-PE-004, REQ-PE-014, REQ-PE-017)
- `manager_test.go`: 4-단계 흐름(Subject not ready → blocked → undeclared → declared+lookup), per-triple lock, expired grant lookup miss (REQ-PE-006/007/012/013/016)

### 7.2 Integration Tests
- Manager + Store + Confirmer stub end-to-end: AC-PE-002 → AC-PE-003 → AC-PE-005 시나리오 연속 실행
- Manager + blocked_always 매처(FS-ACCESS-001 stub): AC-PE-006
- Manager + sub-agent inheritance: AC-PE-010 (parent subject 등록 → child fallback 검증)

### 7.3 Security Cases
- Manifest tampering: 로딩 후 `manifest_test.go`에서 manifest 객체를 외부 코드가 직접 mutate 시도 → registry 사본이 보호되는지 검증
- Revoked grant attempt: AC-PE-005 후속, revoked 후 동일 호출이 confirm 다시 트리거하는지
- Scope confusion: `net: "*.anthropic.com"`이 `evil-anthropic.com`을 매치하지 않는지(glob 매처 정확성, 단 본 SPEC은 매처를 외부 위임이므로 stub matcher 행위 검증만)
- File mode escalation: 외부 프로세스가 grants.json mode를 0644로 변경 → 다음 `Store.Open`이 거부 (AC-PE-007)

### 7.4 Characterization Tests (Claude Code parity)
- Claude Code 권한 prompt에서 알려진 케이스 5건(예: `Bash` 도구 첫 호출, `WebFetch` 첫 호출, MCP server 첫 연결, plugin 첫 로드, sub-agent fork)를 본 SPEC의 SubjectID 컨벤션(`tool:Bash`, `mcp:context7` 등)으로 매핑한 테이블이 `manager_test.go::TestParity_ClaudeCodePermissionStrings`에 포함되어, 향후 두 시스템의 권한 키 충돌 / 의미 분기 발생 시 회귀 감지.

### 7.5 검증 체크리스트
- [ ] `go test -race ./internal/permission/...` 통과
- [ ] coverage ≥ 85%
- [ ] `golangci-lint run --enable gosec ./internal/permission/...` 0 issue
- [ ] `goleak.VerifyNone(t)` 모든 테스트에서 pass (per-triple mutex map이 leak 안 함)
- [ ] CLI `mink permission --help` 출력에 4 서브커맨드 모두 존재 (smoke test)

---

## 8. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `~/.goose/` + `./.goose/` 경로 의미 + permission 디렉토리 위치 결정 |
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger, context 루트, graceful shutdown |
| 선행 SPEC | SPEC-GOOSE-SKILLS-001 | frontmatter `requires:` 필드 자리 + allowlist 추가 + Manager.Register 호출 통합 |
| 선행 SPEC | SPEC-GOOSE-MCP-001 | MCP server config의 `requires:` 통합 + `ConnectToServer` 진입점에서 `Manager.Check` 호출 |
| 선행 SPEC | SPEC-GOOSE-SUBAGENT-001 | `AgentDefinition.InheritGrants` 필드 추가 + 부모/자식 SubjectID 전파 (REQ-PE-011) |
| 후속 SPEC | SPEC-GOOSE-AUDIT-001 | `Auditor.Record(event)` 의 실 구현(append-only audit.log) |
| 후속 SPEC | SPEC-GOOSE-FS-ACCESS-001 | `BlockedAlwaysMatcher` + path glob 매처 실 구현 (본 SPEC은 인터페이스만) |
| 후속 SPEC | SPEC-GOOSE-SECURITY-SANDBOX-001 | OS 수준 syscall 차단 — 본 SPEC의 policy 결정을 강제 시행 |
| 후속 SPEC | SPEC-GOOSE-PLUGIN-001 | `requires:` 스키마 재사용 + integrity check 훅 (REQ-PE-020) |
| 외부 | Go 1.22+ | generics, context, sync.RWMutex |
| 외부 | `github.com/google/uuid` v1.6+ | Grant.ID 생성 (UUIDv4) |
| 외부 | `gopkg.in/yaml.v3` v3.0+ | manifest frontmatter 파서 (SKILLS-001 공유) |
| 외부 | `go.uber.org/zap` v1.27+ | 구조화 로깅 |
| 외부 (선택) | `github.com/zalando/go-keyring` v0.2+ | `Store` 백엔드를 OS keyring으로 교체할 경우(현 SPEC은 JSON 파일만; CREDENTIAL-PROXY-001과의 일관성을 위해 후속 옵션) |
| 외부 | `github.com/stretchr/testify` v1.9+ | 테스트 |

**라이브러리 결정 의도**:
- `golang.org/x/oauth2`: 미사용 (본 SPEC은 OAuth와 무관, manifest 기반 declared permission만).
- SQLite(`mattn/go-sqlite3`): 후보였으나 v0.2.0에서는 JSON 파일이 충분(grant 수 << 1000 예상). v1.0+ scale-up 시 재검토.
- `github.com/spf13/cobra`: 본 SPEC은 인터페이스만 — CLI 라이브러리 선택은 SPEC-GOOSE-CLI-001 결정에 위임.

---

## 9. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Grant store 손상(부분 write / disk full) | 낮 | 고 | atomic write(temp + fsync + rename), 0600 권한, schema_version 검증, write 실패 시 메모리 인덱스 보존 후 다음 mutation에서 재시도 |
| R2 | First-call confirm UX 방해(매번 prompt 폭증) | 중 | 중 | `AlwaysAllow` 기본 옵션 + grant scope 단위 영속화로 동일 scope 반복 호출 0회 prompt; sub-agent 인원이 늘어도 부모 grant 상속(`InheritGrants`) 옵션으로 prompt 폭증 차단; `--ttl` 옵션(REQ-PE-019)으로 단기 grant 가능 |
| R3 | Sub-agent grant leakage(부모 권한이 의도치 않게 자식에 흘러감) | 중 | 고 | `InheritGrants` 기본값 `false`(opt-in), 명시 플래그 시에만 fallback chain 활성화, audit event에 `inherited_from` 명시로 감사 가능, 매니페스트 contraction 시 자동 invalidation(REQ-PE-015) |
| R4 | `requires:` 스키마 breaking change (v0.2 → v0.3+) | 중 | 중 | Manifest에 `schema_version` 필드, 미스매치 시 명시 에러(REQ-PE-017), 마이그레이션 CLI(`mink permission migrate --from <ver>`) placeholder; 4-카테고리는 FROZEN(추가는 minor bump, 제거는 major bump) |
| R5 | per-triple mutex map이 메모리 누수(map 크기 무한 증가) | 중 | 중 | mutex GC 정책: triple key가 store에 grant로 영속되면 mutex 제거 가능; 또는 LRU(최대 1024 active triple), 만료된 triple은 zero-value mutex로 회수 |
| R6 | blocked_always 목록이 manifest scope 토큰 표기와 불일치(예: `~/.ssh/**` vs `$HOME/.ssh/**`) | 중 | 고 | normalization 단계 — 모든 path-like scope를 `os.UserHomeDir()` + `filepath.Clean`으로 정규화 후 매칭, FS-ACCESS-001 `Matcher`가 동일 정규화 수행 보장(인터페이스 계약), AC-PE-006에서 `~/.ssh` 경로 변형 4종 fixture로 검증 |
| R7 | Confirmer 미배선(orchestrator 누락) 시 모든 첫 호출이 영원히 block | 낮 | 고 | 기본 `DefaultDenyConfirmer` 폴백 — Confirmer가 nil이면 자동으로 모든 첫 호출 Deny 반환; `Manager.New` 생성 시 nil 검사로 명시 fail-fast(`ErrConfirmerRequired`); 통합 테스트 시 stub Confirmer 강제 |
| R8 | 다중 `goosed` 인스턴스가 동일 `grants.json`에 동시 write | 낮 | 중 | 단일 프로세스 가정(CREDPOOL-001과 동일), 다중 프로세스 환경의 cross-process 락은 후속 SPEC; 현 SPEC은 file rename atomicity로 최선 노력만 |

---

## 10. Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **OS-level sandbox 강제(seatbelt / Landlock+seccomp / AppContainer)을 구현하지 않는다**. SECURITY-SANDBOX-001.
- 본 SPEC은 **filesystem path glob 매칭/blocked_always 매처의 실 구현을 포함하지 않는다**. `BlockedAlwaysMatcher` 인터페이스만; 실 구현은 FS-ACCESS-001.
- 본 SPEC은 **audit.log 파일의 본체 구현(rotation / append-only attribute / chattr +a 적용)을 포함하지 않는다**. `Auditor.Record` 인터페이스만; AUDIT-001 담당.
- 본 SPEC은 **rate limiting / quota / per-second 호출 한도를 구현하지 않는다**. 후속 SPEC.
- 본 SPEC은 **network policy enforcement(특정 IP/도메인 차단, mTLS 강제)을 구현하지 않는다**. 후속 SPEC.
- 본 SPEC은 **MFA / 생체 인증 / 디바이스 바인딩을 포함하지 않는다**. 후속 SPEC.
- 본 SPEC은 **권한 위임(delegation; A의 grant를 B가 사용) 모델을 정의하지 않는다**. 명시 sub-agent inheritance(`InheritGrants: true`)만; 그 외는 후속 SPEC.
- 본 SPEC은 **plugin marketplace 공급망 검증(서명 / SLSA)을 포함하지 않는다**. PLUGIN-001 + 본 SPEC의 REQ-PE-020 extension point.
- 본 SPEC은 **AskUserQuestion UI / CLI prompt UX의 실 구현을 포함하지 않는다**. `Confirmer` 인터페이스만; CLI/TUI/Web UI별 구현은 각 channel SPEC 또는 orchestrator 측 작업.
- 본 SPEC은 **audit log retention policy(보존 기간 / 자동 삭제)를 정의하지 않는다**. AUDIT-001.
- 본 SPEC은 **다중 `goosed` 프로세스 간 grant store sync를 보장하지 않는다**. 단일 프로세스 가정.
- 본 SPEC은 **권한 그룹화 / 정책 templating(예: "이 5개 host는 한 번에 grant")을 구현하지 않는다**. v0.3.0 amendment 후보.
- 본 SPEC은 **grant migration tool(v0.1 schema → v0.2 schema 일괄 변환)의 실 구현을 포함하지 않는다**. `mink permission migrate` 서브커맨드는 v0.2에서 placeholder; 실 구현은 schema breaking change 발생 시.

---

## 11. Open Items (Run phase / 후속 SPEC 이관)

> 본 SPEC의 Plan 단계에 포함하지 않고 Run phase 또는 후속 SPEC으로 이관되는 항목.

### 11.1 본 SPEC Run phase로 이관 (구현 미착수)

| ID | 항목 | 관련 REQ/AC | 이관 사유 |
|----|-----|-----------|---------|
| OI-01 | per-triple mutex map의 LRU GC 정책 구체화 | REQ-PE-016, R5 | Plan에는 알고리즘 윤곽만; 구체 LRU 사이즈 / eviction 트리거는 GREEN 후 측정 데이터 기반 결정 |
| OI-02 | `mink permission migrate --from <ver>` 실 구현 | REQ-PE-017 | v0.2.0에는 placeholder error만; schema breaking change 발생 시 별도 구현 |
| OI-03 | Confirmer의 default `DefaultDenyConfirmer` polyfill 구현 | R7 | Manager.New 시점 fallback; 인터페이스 contract만 명시 |
| OI-04 | grant scope 토큰 normalization 헬퍼 (`~/`, `$HOME` 등) | R6 | FS-ACCESS-001과 공유 — 본 SPEC에서 인터페이스 계약 합의, 실 구현은 FS-ACCESS-001 Run phase |

### 11.2 후속 SPEC으로 위임

| ID | 항목 | 이관 대상 |
|----|-----|---------|
| OI-05 | OS keyring 백엔드 통합 (`zalando/go-keyring`) | CREDENTIAL-PROXY-001과 일관성 검토 후 결정 |
| OI-06 | audit.log 본체 (rotation, chattr +a) | AUDIT-001 |
| OI-07 | blocked_always glob 매처 + path normalization 엔진 | FS-ACCESS-001 |
| OI-08 | OS-level sandbox 강제 (seatbelt / Landlock+seccomp) | SECURITY-SANDBOX-001 |
| OI-09 | plugin manifest integrity check (서명 / SLSA) | PLUGIN-001 |
| OI-10 | 권한 그룹화 / 정책 templating | 후속 SPEC v0.3.0 amendment 후보 |
| OI-11 | rate limiting / quota | 후속 SPEC |
| OI-12 | MFA / 생체 인증 | 후속 SPEC |

---

## 12. 참고 (References)

### 12.1 프로젝트 문서
- `.moai/design/goose-runtime-architecture-v0.2.md` §5 Tier 5 (Declared Permission)
- `.moai/specs/SPEC-GOOSE-ARCH-REDESIGN-v0.2/spec.md` (parent meta-SPEC)
- `.moai/specs/SPEC-GOOSE-SKILLS-001/spec.md` — frontmatter allowlist + 4-trigger 모델
- `.moai/specs/SPEC-GOOSE-MCP-001/spec.md` — MCP server config + first-connect 흐름
- `.moai/specs/SPEC-GOOSE-SUBAGENT-001/spec.md` — sub-agent isolation + identity
- `.moai/specs/SPEC-GOOSE-AUDIT-001/spec.md` — append-only audit.log
- `.moai/specs/SPEC-GOOSE-FS-ACCESS-001/spec.md` — filesystem access matrix
- `.moai/specs/SPEC-GOOSE-CREDPOOL-001/spec.md` — atomic write 패턴 + file mode 0600 정책 참고
- CLAUDE.md §8 User Interaction Architecture — AskUserQuestion 사용 규약(orchestrator 측 Confirmer 구현 가이드)
- CLAUDE.md §1 HARD Rules — Default-Deny 원칙 근거

### 12.2 외부 참조
- Claude Code Permission Docs: https://docs.claude.com/en/docs/claude-code/iam
- OWASP IAM Top 10: https://owasp.org/www-project-top-ten/
- Go `sync.RWMutex` semantics: https://pkg.go.dev/sync
- `google/uuid`: https://github.com/google/uuid
- `zalando/go-keyring`: https://github.com/zalando/go-keyring (선택적 백엔드)

### 12.3 부속 문서
- 본 SPEC의 background는 본문 §2에 임베드 (별도 research.md 미생성). 추가 deep-dive 필요 시 후속 amendment에서 `./research.md` 추가.

---

## Implementation Notes (sync 정합화 2026-04-27)

- **Status Transition**: planned → implemented
- **Package**: `internal/permission/` (13 파일) + `internal/permission/store/` (file/memory 2 store 구현)
- **Core**: `manager.go`(first-call confirm `inflight` map for 동시성 제어), `manifest.go`, `grant.go`, `confirmer.go`, `errors.go`
- **Store**: `store.go` + `store/file.go`(영속 grant store), `store/memory.go`(테스트 in-memory)
- **Verified REQs (spot-check)**: declared permission(`requires:` manifest 필드 파싱), first-call-confirm flow + inflight 중복 차단, grant store 영속화
- **참고**: `internal/permissions/`(복수형, 별도 디렉토리)는 Claude SDK PermissionMode 추상화 — 본 SPEC 범위 외
- **Test Coverage** (6 파일): `manager_test.go`(메인), `confirmer_test.go`, `errors_test.go`, `manifest_test.go`, `store/file_test.go`, `store/memory_test.go`
- **Lifecycle**: spec-anchored Level 2, milestone M2

---

**End of SPEC-GOOSE-PERMISSION-001**
