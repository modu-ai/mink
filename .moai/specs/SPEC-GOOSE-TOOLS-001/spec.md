---
id: SPEC-GOOSE-TOOLS-001
version: 0.1.2
status: implemented
created_at: 2026-04-21
updated_at: 2026-04-25
author: manager-spec
priority: P0
issue_number: null
phase: 3
size: 중(M)
lifecycle: spec-anchored
labels: [phase-3, tools, mcp, permission, security]
---

# SPEC-GOOSE-TOOLS-001 — Tool Registry 및 ToolSearch (Deferred Loading, Inventory)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (Phase 3 신규, claude-primitives §3.4 + Hermes model_tools.py 패턴 + QUERY-001 소비자 계약) | manager-spec |
| 0.1.1 | 2026-04-25 | plan-auditor 결함 수정: labels 채움(D1), AC-TOOLS-010~018 추가(D2), REQ-TOOLS-007 ↔ §6.4 일관화(D3, Option A), REQ-TOOLS-016 negative-path AC(D6, AC-TOOLS-015), REQ-TOOLS-001 behavioral 재표현(D5), REQ-TOOLS-021 MCP duplicate 추가(D7), REQ-TOOLS-022 sequential dispatch 추가(D8) | manager-spec |
| 0.1.2 | 2026-04-25 | Status sync — `internal/tools/` 패키지 구현 완료를 frontmatter에 반영. PR #10 (commit 18ee5fb): `registry.go` (Registry + Resolve/Register/Adopt/Drain/IsDraining), `executor.go` (Run + draining state check), `inventory.go`, `errors.go` (ErrRegistryDraining 등), `mcp_adapter.go`, `budget.go`, `scope.go`, `tool.go`, `builtin/` (built-in 6종 stub). status: planned → implemented. SPEC 본문 변경 없음. | manager-spec |

---

## 1. 개요 (Overview)

AI.GOOSE의 **Tool 실행 인프라 계층**을 정의한다. `SPEC-GOOSE-QUERY-001` §3.2(OUT)에서 인터페이스만 선언한 `tools.Registry` 및 `tools.Executor`를 본 SPEC이 완성하며, Claude Code의 deferred-loading `ToolSearch` 메커니즘(`.moai/project/research/claude-primitives.md` §3.4)과 Hermes `cli.py` / `model_tools.py`의 **auto-registry inventory 패턴**을 Go로 포팅한다.

본 SPEC 수락 시점에서:

- `internal/tools/` 패키지가 `Tool` 인터페이스, `Registry`, `Executor`, `Inventory`, `Search`를 제공하고,
- 내장 tool 최소 세트(**FileRead / FileWrite / FileEdit / Glob / Grep / Bash**)가 `init()` 기반 자동 등록되며,
- MCP-001의 connection을 통해 발견된 `mcp__{server}__{tool}` tool들이 **deferred loading**(매니페스트는 첫 호출 또는 명시적 `Search.Activate()` 시점에 fetch)되고,
- `Registry.Resolve(name)`이 이름 충돌(built-in vs MCP, MCP vs MCP)을 결정론적으로 해결하며,
- `CanUseTool` gate(QUERY-001 REQ-QUERY-006)가 호출되기 **전**에 `PermissionMatcher.Preapproved(name, input)` 검사로 settings.json allowlist를 소비하고,
- `Executor.Run(ctx, req)`이 순차 실행(Phase 3은 병렬 미포함, QUERY-001 OUT과 정렬) + tool_result 스트리밍 없는 bytes 반환을 제공한다.

본 SPEC은 **QueryEngine이 호출하는 소비자 측 계약**과 **내장 tool의 행동 계약**을 규정한다. Tool 실제 구현(fs 호출, pty, go-git 등)은 본 SPEC 내부 구현 세부이며, MCP tool의 transport/OAuth는 MCP-001이 담당한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **MVP Milestone 1 블로커**: ROADMAP §7 "MVP Milestone 1 — 동작하는 에이전트"의 필수 경로는 `TOOLS-001 → CLI-001`. 본 SPEC이 완료되어야 `goose ask "list files in current dir"` → `Glob tool 실행` → 결과 반환이 end-to-end로 동작.
- **QUERY-001 의존성 해소**: QUERY-001은 `tools.Registry`, `tools.Executor`, `ToolPermissionContext`를 **인터페이스 호출**만 한다. 본 SPEC이 구현체를 제공하지 않으면 QUERY-001의 AC-QUERY-002(1 tool call) / AC-QUERY-003(permission deny) / AC-QUERY-009(tool result budget) 테스트가 통합 레벨에서 실행 불가.
- **MCP-001 ↔ TOOLS-001 경계**: MCP-001은 transport/OAuth/connection 매니저만 담당. "MCP에서 받은 tool을 어디에 올리고, 이름 충돌을 어떻게 해결하고, 모델에게 어떤 매니페스트로 노출하는가"는 본 SPEC의 책임.

### 2.2 상속 자산 (패턴 계승)

- **Claude Code TypeScript** (`./claude-code-source-map/`): `tools/` 디렉토리, `ToolSearchTool`, `mcp__{server}__{tool}` prefix 규칙, `fetchToolsForClient()` memoize. 언어 상이로 직접 포트 아님.
  - 근거 문서: `.moai/project/research/claude-primitives.md` §3.4(Deferred Loading), §2.4(Model Invocation 제약), §9(재사용 80%).
- **Hermes Agent Python** (`./hermes-agent-main/`): `model_tools.py` — agent 시작 시 tool 목록을 inventory로 덤프하여 시스템 프롬프트에 주입, 호출 시 lazy dispatch. 본 SPEC은 **inventory 자동 등록 패턴**만 계승.
- **기존 SPEC-GOOSE-AGENT-001 (v1.0, DEPRECATED)**: 일부 tool 시그니처 아이디어만 참고.

### 2.3 범위 경계 (한 줄)

- **IN**: Tool 인터페이스, Registry/Executor, 내장 6 tool, MCP tool adoption, 이름 충돌 해결, deferred loading via ToolSearch, permission pre-approval matcher.
- **OUT**: MCP transport (→MCP-001), 실제 Permission UI (→HOOK-001), 병렬 tool 실행, sandbox 강화(→SAFETY-001), tool 생성(→SUBAGENT-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/tools/` 루트: 핵심 타입과 Registry/Executor.
   - `Tool` interface (`Name()`, `Schema()`, `Scope()`, `Call(ctx, input) (ToolResult, error)`).
   - `Registry` (R/W mutex로 보호되는 name→Tool 매핑 + metadata).
   - `Executor` (Registry + Preflight + Call dispatch).
   - `Inventory` (모델에게 노출할 tool 매니페스트 JSON).
   - `ToolResult` (`{Content []byte, IsError bool, Metadata map[string]any}`).
   - `ToolPermissionContext` (QUERY-001과 공유되는 타입. 실제 타입은 본 SPEC에서 소유, QUERY-001이 import).

2. `internal/tools/search/` — **ToolSearch (Deferred Loading)**.
   - `Search.List(ctx, filter) []ToolDescriptor` — 활성/비활성 tool 모두 열거.
   - `Search.Activate(ctx, name) error` — deferred tool(MCP)의 매니페스트 late-bind.
   - `Search.InvalidateCache(serverID string)` — MCP 재연결 시 캐시 무효화.

3. `internal/tools/builtin/` — 내장 tool 6종.
   - `file/`:
     - `FileRead` (`{path, offset?, limit?}` → file bytes, UTF-8 검증).
     - `FileWrite` (`{path, content}` → write 결과, atomic via tmp + rename).
     - `FileEdit` (`{path, old_string, new_string, replace_all?}` → before/after diff).
     - `Glob` (`{pattern, cwd?}` → matched paths list).
     - `Grep` (`{pattern, path, flags?}` → matches, ripgrep wrapper 또는 pure Go regex).
   - `terminal/`:
     - `Bash` (`{command, timeout_ms?, working_dir?}` → stdout/stderr/exit_code).

4. `internal/tools/mcp_adapter.go` — MCP-001이 제공하는 `mcp.Connection`을 **Tool 형태로 래핑**.
   - `NewMCPToolAdapter(conn mcp.Connection, name string, manifest mcp.ToolManifest) Tool`.
   - `Call()` 내부에서 `conn.ToolsCall(name, input)`으로 위임.
   - Manifest fetch는 deferred (첫 `Call` 또는 `Search.Activate`).

5. `internal/tools/permission/` — Permission Matcher (사전 승인 계층).
   - `PermissionMatcher` interface: `Preapproved(name string, input json.RawMessage, cfg PermissionsConfig) (approved bool, reason string)`.
   - settings.json `permissions.allow` 패턴 매칭 (glob style: `Bash(git *)`, `FileRead(/etc/*)`, `mcp__github__*`).
   - `CanUseTool` gate 호출 **전** pre-check: matched → `Allow` 즉시 반환. QUERY-001 REQ-QUERY-006과 배치 순서 정렬.

6. `internal/tools/naming/` — 이름 충돌 해결.
   - Built-in tool은 예약어(`FileRead`, `Bash` 등) — 등록 시도 시 `ErrReservedName`.
   - MCP tool prefix: `mcp__{serverID}__{toolName}`.
   - 동일 MCP server 내 중복: 에러 + 로그 (MCP 스펙 위반).
   - 두 MCP server가 동일 toolName 보유: prefix로 이미 구분되므로 충돌 없음.

7. `internal/tools/budget.go` — Tool result size 체크.
   - `ApplyResultBudget(result ToolResult, cap int64) (ToolResult, TruncationMeta)` — QUERY-001 REQ-QUERY-007 보조. 실제 치환 로직 구현은 본 SPEC, 호출 위치는 QUERY-001.

8. 자동 등록 (Inventory):
   - 각 builtin tool 파일에서 `init()` 시 `builtin.Register(NewFileRead())` 호출.
   - 런타임 시작 시 `tools.NewRegistry(WithBuiltins(), WithMCPConnections(mcpMgr))` 로 일괄 populate.

### 3.2 OUT OF SCOPE (명시적 제외)

- **MCP transport**(stdio/WS/SSE) / OAuth / connection lifecycle: SPEC-GOOSE-MCP-001.
- **Permission UI / 사용자 프롬프트** ("Ask" branch 실제 대화창): QUERY-001은 `permission_request` SDKMessage를 yield만, 실제 UI는 CLI-001(TUI) / HOOK-001(SessionStart hook).
- **병렬 tool 실행**: Phase 3 OUT. 한 응답 내 여러 `tool_use` 블록은 순차. 향후 TOOLS-002 확장.
- **Sandbox 강화**: 본 SPEC의 `Bash`는 현재 프로세스 권한으로 실행. chroot/ seccomp/ capabilities 제거는 SAFETY-001.
- **Plan Mode (read-only guard)**: SUBAGENT-001의 `permissionMode: plan` 구현.
- **Tool 생성 / plugin tool**: PLUGIN-001.
- **Web 도구(WebFetch/WebSearch/Scrape)**: 본 SPEC 최소 세트에서 제외. 향후 TOOLS-002.
- **Agent tool (spawn sub-agent)**: SUBAGENT-001.
- **Memory 도구 (Recall/Save)**: MEMORY-001.
- **Skill tool (Skill 발견/실행)**: SKILLS-001.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-TOOLS-001 [Ubiquitous]** — The `tools.Registry` **shall** expose a read-only `Resolve(name string) (Tool, bool)` that is safe for concurrent callers without requiring external locking. The synchronization primitive selection is deferred to §6.2 implementation guidance.

**REQ-TOOLS-002 [Ubiquitous]** — Every registered `Tool` **shall** declare a non-empty `Schema()` returning a valid JSON Schema (draft 2020-12) describing its `input` object; registration of a tool whose `Schema()` fails validation **shall** return `ErrInvalidSchema` at registration time.

**REQ-TOOLS-003 [Ubiquitous]** — The `Registry` **shall** treat tool names as case-sensitive; the canonical forms for built-in tools are `FileRead`, `FileWrite`, `FileEdit`, `Glob`, `Grep`, `Bash` and **shall not** be redefined by MCP servers (any MCP tool claiming an unprefixed built-in name **shall** be rejected on adoption).

**REQ-TOOLS-004 [Ubiquitous]** — MCP tools **shall** be registered with the canonical name `mcp__{serverID}__{toolName}`; the `serverID` component **shall** be sanitized to `[a-z0-9_-]{1,64}` at MCP connection setup (MCP-001 contract) and rejected otherwise at adoption time in this SPEC.

**REQ-TOOLS-005 [Ubiquitous]** — The `Inventory.ForModel(ctx, filter)` method **shall** return a deterministic, sorted list of tool descriptors (alphabetical by canonical name) so that identical context produces byte-identical system prompts for prompt caching (PROMPT-CACHE-001 prerequisite).

### 4.2 Event-Driven (이벤트 기반)

**REQ-TOOLS-006 [Event-Driven]** — **When** `Executor.Run(ctx, req)` is invoked, the executor **shall** (a) call `Registry.Resolve(req.ToolName)` to locate the tool, (b) validate `req.Input` against the tool's JSON Schema, (c) invoke `PermissionMatcher.Preapproved(...)`, (d) if not pre-approved, invoke the `CanUseTool` gate supplied by QUERY-001 via `ToolPermissionContext`, and (e) on `Allow` dispatch `Tool.Call(ctx, input)`. On any step failure, a synthetic `ToolResult{IsError: true, Content: <reason>}` **shall** be returned without panicking.

**REQ-TOOLS-007 [Event-Driven]** — **When** a tool is referenced by name but not yet activated (MCP-backed with deferred manifest), `Registry.Resolve(name)` **shall** either (a) return a stub Tool whose first `Call` triggers a manifest fetch via the shared Search cache (keyed by `(serverID, toolName)`) and then re-dispatches to the resolved tool, or (b) return `(nil, false)` if `eagerResolveMCP` policy is enabled via config; the default policy is (a). The stub's fetch path **shall** use the same cache that `Search.Activate(ctx, name)` populates, ensuring cache coherency when `Search.InvalidateCache(serverID)` fires.

**REQ-TOOLS-008 [Event-Driven]** — **When** `Search.Activate(ctx, name)` is invoked for an MCP-backed tool, the adapter **shall** call `mcp.Connection.FetchToolManifest(toolName)` (MCP-001 API), cache the result in-memory keyed by `(serverID, toolName)`, and complete within 5 seconds or return `ErrMCPTimeout`.

**REQ-TOOLS-009 [Event-Driven]** — **When** an MCP server connection is removed (MCP-001 emits `ConnectionClosed` event), the `Registry` **shall** unregister all `mcp__{serverID}__*` tools and invalidate their cached manifests within 1 second.

**REQ-TOOLS-010 [Event-Driven]** — **When** `Bash.Call(ctx, input)` exceeds `input.timeout_ms` (default 120,000ms, max 600,000ms per `.claude/rules/moai/core/agent-common-protocol.md`), the tool **shall** SIGTERM the subprocess, collect partial stdout/stderr, wait up to 2s for graceful exit, then SIGKILL if still running, and return `ToolResult{IsError: true, Content: "timeout: <duration>", Metadata: {stdout_partial, stderr_partial, exit_code: -1}}`.

**REQ-TOOLS-022 [Event-Driven]** — **When** a single LLM response yields N `tool_use` blocks with N > 1, the `Executor` **shall** dispatch them sequentially in their original array order and **shall not** start block N+1 until block N has produced a `ToolResult`; parallel dispatch is explicitly deferred to TOOLS-002 and **shall** not be enabled in this SPEC.

### 4.3 State-Driven (상태 기반)

**REQ-TOOLS-011 [State-Driven]** — **While** `Registry` is in `Draining` state (set by `Registry.Drain()` during CORE-001 shutdown), new `Executor.Run` calls **shall** return `ToolResult{IsError: true, Content: "registry draining"}` without invoking any tool; in-flight `Call`s **shall** be allowed to complete up to the per-tool cancellation deadline.

**REQ-TOOLS-012 [State-Driven]** — **While** the QueryEngineConfig declares `CoordinatorMode == true` (QUERY-001 REQ-QUERY-012), the `Inventory.ForModel(ctx, filter)` **shall** exclude tools whose `Scope() == ScopeLeaderOnly`, producing the same filtered manifest that QUERY-001 will pass to the LLM.

### 4.4 Unwanted Behavior (방지)

**REQ-TOOLS-013 [Unwanted]** — **If** a built-in tool registration attempts to shadow another built-in (same canonical name), **then** the second registration **shall** panic at `init()` time; this is a compile/startup-time contract to guarantee the six built-in names are unique.

**REQ-TOOLS-014 [Unwanted]** — The `Executor` **shall not** invoke a tool whose input fails JSON Schema validation; instead it **shall** return `ToolResult{IsError: true, Content: "schema_validation_failed: <detail>"}` and log at WARN level.

**REQ-TOOLS-015 [Unwanted]** — The `FileWrite` and `FileEdit` tools **shall not** write outside `QueryEngineConfig.Cwd` by default; attempts to write to paths outside cwd **shall** return an error result unless the path is explicitly allowlisted via `PermissionsConfig.additional_directories`.

**REQ-TOOLS-016 [Unwanted]** — The `Bash` tool **shall not** inherit environment variables matching secret-name heuristics (`*_TOKEN`, `*_KEY`, `*_SECRET`, `GOOSE_SHUTDOWN_TOKEN`) into subprocess env unless `input.inherit_secrets == true` is explicitly set AND the invocation is pre-approved via `PermissionMatcher`.

**REQ-TOOLS-017 [Unwanted]** — An MCP tool manifest that claims `tool.name` containing `__` (double underscore) **shall** be rejected at adoption to prevent prefix collision ambiguity; the adapter logs ERROR and skips the tool.

**REQ-TOOLS-021 [Unwanted]** — **If** `AdoptMCPServer` processes a manifest containing a `(serverID, toolName)` pair already adopted in the `Registry`, **then** the adapter **shall** return `ErrDuplicateName`, log ERROR with the conflicting pair, and **shall not** replace or mutate the existing registration.

### 4.5 Optional (선택적)

**REQ-TOOLS-018 [Optional]** — **Where** `PermissionsConfig.allow` contains a pattern matching `(toolName, input)` (glob form, e.g., `Bash(git status)`, `FileRead(/tmp/**)`, `mcp__github__create_issue`), the `PermissionMatcher.Preapproved` **shall** return `(true, "allowlist: <pattern>")` and bypass `CanUseTool` invocation.

**REQ-TOOLS-019 [Optional]** — **Where** `ToolsConfig.strict_schema == true`, tool authors **shall** declare `additionalProperties: false` in their `Schema()`; the registry rejects registrations lacking this flag.

**REQ-TOOLS-020 [Optional]** — **Where** `ToolsConfig.log_invocations == true`, every `Executor.Run` **shall** emit a structured zap log entry `{tool, outcome, duration_ms, input_size, output_size}` at INFO level (outcome ∈ `allow|deny|preapproved|error`).

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then. `internal/tools/*_test.go` 및 `tests/integration/tools/` 하위로 변환 가능.

**AC-TOOLS-001 — Built-in 6종 자동 등록 + 이름 중복 방지**
- **Given** 프로세스 bootstrap 시점 (CORE-001 완료, tools 패키지 import된 상태)
- **When** `registry := tools.NewRegistry(tools.WithBuiltins())` 호출 후 `registry.ListNames()`
- **Then** 정렬된 결과가 정확히 `["Bash", "FileEdit", "FileRead", "FileWrite", "Glob", "Grep"]`이며, 동일 이름의 두 번째 등록 시도는 panic 또는 `ErrDuplicateName` 반환

**AC-TOOLS-002 — MCP tool adoption + prefix**
- **Given** mock `mcp.Connection`(serverID=`github`)이 `{tool.name: "create_issue", ...}` 매니페스트를 노출
- **When** `registry.AdoptMCPServer(conn)` 호출
- **Then** `registry.Resolve("mcp__github__create_issue")`가 `(Tool, true)`를 반환하고, `registry.Resolve("create_issue")`는 `(nil, false)`를 반환

**AC-TOOLS-003 — Deferred loading via Search.Activate**
- **Given** MCP server가 adopt되었으나 manifest fetch가 아직 수행되지 않음 (mock `FetchToolManifest`가 호출 카운트 0)
- **When** `registry.Resolve("mcp__foo__bar")` 호출
- **Then** stub Tool이 반환되고, `stubTool.Call(ctx, input)` 실행 시 `FetchToolManifest`가 정확히 1회 호출되며, 이후 동일 tool 재호출 시 fetch 카운트는 증가하지 않음

**AC-TOOLS-004 — Permission pre-approval 경로**
- **Given** `PermissionsConfig.allow = ["Bash(git status)"]`, `Executor`가 `PermissionMatcher` 주입됨
- **When** `Executor.Run(ctx, {ToolName:"Bash", Input:{"command":"git status"}})` 호출. `CanUseTool`는 stub으로 `Deny`를 반환하도록 설정
- **Then** `Tool.Call`이 실행되고 tool 결과가 반환됨 (pre-approval이 `CanUseTool` Deny를 **bypass**했음을 증명). `CanUseTool.Check` 호출 카운트는 0

**AC-TOOLS-005 — CanUseTool Deny 경로**
- **Given** `PermissionsConfig.allow = []` (비어 있음), `CanUseTool` stub이 `Behavior=Deny, Reason="destructive"` 반환
- **When** `Executor.Run(ctx, {ToolName:"Bash", Input:{"command":"rm -rf /"}})`
- **Then** `ToolResult{IsError:true, Content:"denied: destructive"}`이 반환되고, `Bash.Call`은 **호출되지 않음**

**AC-TOOLS-006 — Schema validation 실패**
- **Given** `FileRead`의 Schema가 `path: string (required)` 요구
- **When** `Executor.Run(ctx, {ToolName:"FileRead", Input:{"wrong_field":1}})`
- **Then** `ToolResult{IsError:true, Content: starts-with "schema_validation_failed"}`이 반환되고 `FileRead.Call`은 호출되지 않음

**AC-TOOLS-007 — Tool not found 처리**
- **Given** registry에 `NonExistent` tool 미등록
- **When** `Executor.Run(ctx, {ToolName:"NonExistent", Input:{}})`
- **Then** `ToolResult{IsError:true, Content:"tool_not_found: NonExistent"}`이 반환. QUERY-001 REQ-QUERY-017과 정렬 — conversation은 terminate되지 않음 (Terminal 발행 없음)

**AC-TOOLS-008 — Bash timeout 처리**
- **Given** `Bash` tool, `input = {"command":"sleep 60", "timeout_ms": 200}`
- **When** `Executor.Run`
- **Then** 500ms 이내 반환, `ToolResult.IsError=true`, `Content` 포함 `"timeout"`, `Metadata.exit_code == -1`, 자식 프로세스는 `ps` 검증 시 존재하지 않음

**AC-TOOLS-009 — Cwd 바깥 쓰기 거부**
- **Given** `QueryEngineConfig.Cwd = "/tmp/project"`, `PermissionsConfig.additional_directories = []`
- **When** `Executor.Run(ctx, {ToolName:"FileWrite", Input:{"path":"/etc/passwd", "content":"..."}})`
- **Then** `ToolResult{IsError:true, Content: contains "outside cwd"}`이 반환되고 `/etc/passwd`는 수정되지 않음

**AC-TOOLS-010 — Inventory 결정론적 정렬 (REQ-TOOLS-005)**
- **Given** 동일한 tool 집합이 순서만 다르게 두 개의 Registry 인스턴스에 등록되어 있음 (built-in 6종 + MCP `mcp__foo__bar`, `mcp__foo__baz` adopted)
- **When** 각 Registry에 대해 `Inventory.ForModel(ctx, filter{})`를 호출하여 결과를 바이트로 직렬화
- **Then** 두 결과는 바이트 단위로 정확히 동일하며, descriptor 배열은 canonical name 알파벳 오름차순으로 정렬됨 (`Bash`, `FileEdit`, ..., `mcp__foo__bar`, `mcp__foo__baz`)

**AC-TOOLS-011 — Search.Activate 5초 타임아웃 (REQ-TOOLS-008)**
- **Given** `mcp.Connection.FetchToolManifest`가 6초 동안 블로킹하도록 mock 설정됨, MCP tool `mcp__slow__op`가 adopt된 상태
- **When** `search.Activate(ctx, "mcp__slow__op")`를 호출
- **Then** 호출은 5초 이내에 반환하며 `ErrMCPTimeout`을 반환하고, `Search.cache`에 `(slow, op)` 항목은 저장되지 않음

**AC-TOOLS-012 — ConnectionClosed 이벤트 처리 (REQ-TOOLS-009)**
- **Given** MCP server `foo`가 adopt되어 `mcp__foo__a`, `mcp__foo__b` 두 tool이 등록됨, 해당 server 매니페스트가 `Search.cache`에 존재
- **When** MCP-001이 `ConnectionClosed{ServerID: "foo"}` 이벤트를 발행
- **Then** 1초 이내에 `registry.Resolve("mcp__foo__a")`와 `registry.Resolve("mcp__foo__b")`가 모두 `(nil, false)`를 반환하며, `Search.cache`에서 `foo/*` 항목이 제거됨

**AC-TOOLS-013 — Draining 상태 진입 (REQ-TOOLS-011)**
- **Given** Registry가 정상 동작 중이고 in-flight `Tool.Call` 없음
- **When** `registry.Drain()` 호출 후 `executor.Run(ctx, {ToolName:"FileRead", Input:{"path":"/tmp/x"}})`
- **Then** `ToolResult{IsError:true, Content:"registry draining"}`이 반환되고 `FileRead.Call`은 호출되지 않음

**AC-TOOLS-014 — CoordinatorMode Inventory 필터 (REQ-TOOLS-012)**
- **Given** Registry에 built-in 6종(모두 `ScopeShared`) + `ScopeLeaderOnly` tool `TeamSpawn`이 등록됨, `QueryEngineConfig.CoordinatorMode = true`
- **When** `Inventory.ForModel(ctx, filter{CoordinatorMode:true})` 호출
- **Then** 반환된 descriptor 배열에 `TeamSpawn`은 포함되지 않고, built-in 6종만 포함됨

**AC-TOOLS-015 — Bash secret env 필터링 negative + positive path (REQ-TOOLS-016)**
- **Given 1** 프로세스 env에 `GITHUB_TOKEN=xyz`, `MY_API_KEY=abc`, `PATH=/usr/bin` 설정
- **When 1** `Executor.Run(ctx, {ToolName:"Bash", Input:{"command":"env"}})` 호출 (`inherit_secrets` 미설정, pre-approval 없음)
- **Then 1** `ToolResult.Content` stdout에 `GITHUB_TOKEN` 및 `MY_API_KEY`가 포함되지 않고 `PATH`는 포함됨
- **Given 2** 동일 env, `PermissionsConfig.allow = []` (pre-approval 없음)
- **When 2** `Executor.Run(ctx, {ToolName:"Bash", Input:{"command":"env", "inherit_secrets":true}})`, `CanUseTool` stub이 `Allow` 반환
- **Then 2** secret 필터링은 여전히 유효 — `GITHUB_TOKEN`과 `MY_API_KEY`는 stdout에 포함되지 않음 (`inherit_secrets:true` 단독으로는 bypass 불가)
- **Given 3** 동일 env, `PermissionsConfig.allow = ["Bash(env)"]` (pre-approval 일치)
- **When 3** `Executor.Run(ctx, {ToolName:"Bash", Input:{"command":"env", "inherit_secrets":true}})`
- **Then 3** `GITHUB_TOKEN=xyz`와 `MY_API_KEY=abc`가 stdout에 포함됨 (pre-approval + inherit_secrets 양쪽 만족 시에만 통과)

**AC-TOOLS-016 — MCP 이름에 `__` 포함 시 거부 (REQ-TOOLS-017)**
- **Given** mock MCP connection이 manifest `{tool.name: "bad__name", ...}`를 노출
- **When** `registry.AdoptMCPServer(conn)` 호출
- **Then** `bad__name` tool은 등록되지 않으며(`registry.Resolve("mcp__srv__bad__name")` → `(nil, false)`), ERROR 레벨 로그가 emit되고, 동일 manifest 내 다른 유효 tool은 영향 없이 등록됨

**AC-TOOLS-017 — strict_schema additionalProperties 강제 (REQ-TOOLS-019)**
- **Given** `ToolsConfig.strict_schema = true`, tool `BadTool`의 `Schema()`는 `additionalProperties` 필드를 선언하지 않음
- **When** `registry.Register(NewBadTool(), SourceBuiltin)` 호출
- **Then** 등록이 실패하고 `ErrInvalidSchema` (또는 동등한 strict-schema 위반 에러)가 반환되며, `registry.Resolve("BadTool")`은 `(nil, false)`

**AC-TOOLS-018 — log_invocations 구조화 로그 (REQ-TOOLS-020)**
- **Given** `ToolsConfig.log_invocations = true`, in-memory zap observer logger 주입
- **When** `executor.Run(ctx, {ToolName:"FileRead", Input:{"path":"/tmp/x"}})` 호출하여 성공
- **Then** INFO 레벨 로그 엔트리 1건이 emit되고, 필드에 `tool="FileRead"`, `outcome="allow"`, `duration_ms` 숫자, `input_size`/`output_size` 바이트 값이 포함됨; 실패 경로에서는 `outcome="error"` 또는 `outcome="deny"`로 기록됨

**AC-TOOLS-019 — MCP adoption duplicate 거부 (REQ-TOOLS-021)**
- **Given** mock MCP connection이 serverID=`github`, manifest tool `create_issue`를 노출하여 1회 adopt 완료된 상태 (`registry.Resolve("mcp__github__create_issue")`가 `(Tool, true)`)
- **When** 동일 `(github, create_issue)` pair를 포함하는 manifest로 `registry.AdoptMCPServer(conn)` 재호출
- **Then** 함수는 `ErrDuplicateName`을 반환하고, ERROR 레벨 로그에 `serverID=github` 및 `toolName=create_issue`가 기록되며, 기존 registration은 교체/변경되지 않음 (동일 Tool 인스턴스 pointer 유지)

**AC-TOOLS-020 — Sequential tool_use dispatch (REQ-TOOLS-022)**
- **Given** 한 LLM 응답이 3개의 `tool_use` 블록을 순서대로 포함: `[{id:"a", name:"Glob"}, {id:"b", name:"FileRead"}, {id:"c", name:"Bash"}]`, 각 tool은 호출 완료까지 200ms 소요
- **When** QueryEngine이 해당 응답을 받아 `Executor.Run`을 디스패치
- **Then** tool 호출 순서는 정확히 `a → b → c`이며, 블록 `b`의 시작 시각은 블록 `a`의 완료 이후(단조 증가 time window 검증), 블록 `c` 시작 시각은 `b` 완료 이후여야 함; 동시 실행된 호출 수는 어느 순간에도 1을 넘지 않음 (tool.callCount 동시성 카운터로 측정)

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/tools/
├── tool.go                    # Tool interface, ToolResult, ToolDescriptor
├── registry.go                # Registry struct + Resolve/Register/Adopt/Drain
├── executor.go                # Executor.Run (orchestration)
├── inventory.go               # Inventory.ForModel (sorted manifest)
├── scope.go                   # ScopeShared | ScopeLeaderOnly | ScopeWorkerShareable
├── budget.go                  # ApplyResultBudget
├── errors.go                  # ErrInvalidSchema, ErrDuplicateName, ErrReservedName, ErrMCPTimeout
├── mcp_adapter.go             # NewMCPToolAdapter + deferred fetch
├── naming.go                  # prefix 규약, sanitize
├── registry_test.go
├── executor_test.go
│
├── search/
│   ├── search.go              # Search.List / Activate / InvalidateCache
│   └── search_test.go
│
├── permission/
│   ├── matcher.go             # PermissionMatcher interface + GlobMatcher impl
│   ├── config.go              # PermissionsConfig
│   └── matcher_test.go
│
└── builtin/
    ├── builtin.go             # init()에서 Register
    ├── file/
    │   ├── read.go            # FileRead
    │   ├── write.go           # FileWrite (atomic rename)
    │   ├── edit.go            # FileEdit (diff-preserving)
    │   ├── glob.go            # Glob (doublestar 라이브러리)
    │   ├── grep.go            # Grep (regexp 표준 패키지)
    │   └── *_test.go
    └── terminal/
        ├── bash.go            # Bash (os/exec, pty 없음)
        └── bash_test.go
```

### 6.2 핵심 타입 (Go 시그니처)

```go
// internal/tools/tool.go

// Tool은 모델이 호출 가능한 단일 기능 단위.
// Name은 Registry의 키, Schema는 모델에게 노출되는 입력 계약,
// Scope는 coordinator/worker 가시성 제어 (QUERY-001 REQ-QUERY-012 연동),
// Call은 실제 실행 로직.
type Tool interface {
    Name() string
    Schema() json.RawMessage // JSON Schema draft 2020-12
    Scope() Scope
    Call(ctx context.Context, input json.RawMessage) (ToolResult, error)
}

type ToolResult struct {
    Content  []byte            // UTF-8 text 또는 base64 데이터
    IsError  bool
    Metadata map[string]any    // 옵션: exit_code, bytes_read 등
}

type ToolDescriptor struct {
    Name        string
    Description string
    Schema      json.RawMessage
    Scope       Scope
    Source      Source // SourceBuiltin | SourceMCP | SourcePlugin
    ServerID    string // MCP일 때만
}

type Scope int
const (
    ScopeShared         Scope = iota // 기본 — leader+worker 모두
    ScopeLeaderOnly                   // coordinator 모드에서 hidden
    ScopeWorkerShareable              // coordinator 모드에서 exposed
)


// internal/tools/registry.go

type Registry struct {
    mu       sync.RWMutex
    tools    map[string]Tool
    meta     map[string]ToolDescriptor
    mcpSub   mcp.ConnectionSubscriber // MCP-001 제공
    draining atomic.Bool
}

type Option func(*Registry)

func WithBuiltins() Option                     // file + terminal 6종
func WithMCPConnections(mgr mcp.Manager) Option // MCP adopt
func WithPermissionMatcher(m permission.Matcher) Option

func NewRegistry(opts ...Option) *Registry

func (r *Registry) Register(t Tool, src Source) error    // REQ-TOOLS-003/013
func (r *Registry) AdoptMCPServer(conn mcp.Connection) error // REQ-TOOLS-004/009
func (r *Registry) Resolve(name string) (Tool, bool)     // REQ-TOOLS-001/007
func (r *Registry) ListNames() []string                  // 정렬 (REQ-TOOLS-005)
func (r *Registry) Drain()                               // REQ-TOOLS-011


// internal/tools/executor.go

type Executor struct {
    registry   *Registry
    matcher    permission.Matcher
    canUseTool permissions.CanUseTool // QUERY-001 주입
    logger     *zap.Logger
}

type ExecRequest struct {
    ToolName        string
    Input           json.RawMessage
    ToolUseID       string                         // LLM 생성 uuid
    PermissionCtx   permissions.ToolPermissionContext
}

// Run은 orchestration 메서드 — schema → preapproval → canUseTool → call.
// REQ-TOOLS-006 참조.
func (e *Executor) Run(ctx context.Context, req ExecRequest) ToolResult


// internal/tools/search/search.go

type Search struct {
    registry *tools.Registry
    cache    sync.Map // key: "serverID/toolName" -> mcp.ToolManifest
}

func (s *Search) List(ctx context.Context, filter Filter) []tools.ToolDescriptor
func (s *Search) Activate(ctx context.Context, name string) error // REQ-TOOLS-007/008
func (s *Search) InvalidateCache(serverID string)                 // REQ-TOOLS-009


// internal/tools/permission/matcher.go

type Matcher interface {
    Preapproved(toolName string, input json.RawMessage, cfg Config) (approved bool, reason string)
}

type Config struct {
    Allow []string // ["Bash(git *)", "FileRead(/tmp/**)", "mcp__github__*"]
    Deny  []string
    AdditionalDirectories []string // REQ-TOOLS-015 연동
}

// GlobMatcher는 doublestar 기반 패턴 매처.
// 구문: "<ToolName>(<arg-pattern>)" 또는 "<ToolName>" (인자 unchecked).
type GlobMatcher struct{}

func (g *GlobMatcher) Preapproved(name string, input json.RawMessage, cfg Config) (bool, string)
```

### 6.3 내장 Tool 6종 행동 계약

| Tool | Input | Output content | 핵심 제약 |
|------|-------|----------------|-----------|
| `FileRead` | `{path string, offset?, limit?}` | 파일 내용(UTF-8 또는 base64 fallback) | Cwd bound (REQ-TOOLS-015), offset/limit은 lines 단위 |
| `FileWrite` | `{path string, content string}` | `{bytes_written}` | Atomic (tmp + rename), Cwd bound, pre-existing 보존 X(overwrite) |
| `FileEdit` | `{path, old_string, new_string, replace_all?}` | `{replacements}` | old_string 정확 일치 필수, 미존재 시 error |
| `Glob` | `{pattern, cwd?}` | `[]path` | doublestar (`**`) 지원, cwd default Registry Cwd |
| `Grep` | `{pattern, path, flags?: {i,n,C}}` | `[]match{file, line, text}` | Go `regexp` 표준, ripgrep 의존성 없음 |
| `Bash` | `{command, timeout_ms?, working_dir?, inherit_secrets?}` | `{stdout, stderr, exit_code}` | Secret filtering (REQ-TOOLS-016), timeout kill tree |

### 6.4 MCP adapter 전략 (Deferred Loading)

```go
// mcp_adapter.go 핵심 로직

type mcpStubTool struct {
    registry *Registry
    serverID string
    toolName string
    fetcher  func(ctx context.Context) (mcp.ToolManifest, error)
    realOnce sync.Once
    realTool atomic.Pointer[mcpRealTool]
    realErr  atomic.Pointer[error]
}

func (m *mcpStubTool) Call(ctx context.Context, input json.RawMessage) (ToolResult, error) {
    // 첫 호출 시 manifest fetch → real tool swap
    m.realOnce.Do(func() {
        manifest, err := m.fetcher(ctx) // MCP-001 API
        if err != nil {
            m.realErr.Store(&err)
            return
        }
        real := &mcpRealTool{conn: ..., manifest: manifest}
        m.realTool.Store(real)
    })
    if errPtr := m.realErr.Load(); errPtr != nil {
        return ToolResult{IsError: true, Content: []byte("mcp_activation_failed: " + (*errPtr).Error())}, nil
    }
    return m.realTool.Load().Call(ctx, input)
}
```

MCP 매니페스트 캐시는 `Search.cache`에 공유 저장하여, 동일 tool이 여러 경로로 resolve되어도 fetch는 1회.

### 6.5 Permission Matcher 구문

```
"Bash"                    → 모든 Bash 호출 사전승인
"Bash(git status)"        → command=="git status"인 Bash만
"Bash(git *)"             → command이 "git " 접두인 Bash만 (doublestar glob)
"FileRead(/tmp/**)"       → path가 /tmp/ 하위인 FileRead만
"mcp__github__*"          → github MCP server의 모든 tool
"mcp__github__create_*"   → github MCP server의 create_ 접두 tool
```

arg-pattern 해석: tool schema가 `primary field`(Bash는 `command`, FileRead는 `path`, MCP tool은 첫 string 필드)를 선언하고, 그 필드 값을 pattern 매칭. schema 힌트가 없으면 `<ToolName>(...)` 패턴은 `false` 반환(안전 기본).

### 6.6 TDD 진입 순서 (RED → GREEN → REFACTOR)

quality.yaml이 `development_mode: tdd`이므로 모든 구현은 RED-first:

1. **RED #1**: `TestRegistry_RegisterBuiltins_HasSixCanonicalNames` — AC-TOOLS-001.
2. **RED #2**: `TestRegistry_RegisterDuplicate_Panics` — REQ-TOOLS-013.
3. **RED #3**: `TestRegistry_AdoptMCP_AppliesPrefix` — AC-TOOLS-002.
4. **RED #4**: `TestRegistry_ResolveMCPStub_LazyFetch` — AC-TOOLS-003.
5. **RED #5**: `TestExecutor_Run_SchemaValidationFails` — AC-TOOLS-006.
6. **RED #6**: `TestExecutor_Run_ToolNotFound` — AC-TOOLS-007.
7. **RED #7**: `TestExecutor_Run_PreapprovalBypassesCanUseTool` — AC-TOOLS-004.
8. **RED #8**: `TestExecutor_Run_CanUseToolDeny` — AC-TOOLS-005.
9. **RED #9**: `TestBash_TimeoutKillsSubprocess` — AC-TOOLS-008.
10. **RED #10**: `TestFileWrite_OutsideCwd_Denied` — AC-TOOLS-009.
11. **RED #11**: `TestInventory_ForModel_CoordinatorFilters` — REQ-TOOLS-012.
12. **RED #12**: `TestMCPConnectionClosed_UnregistersTools` — REQ-TOOLS-009.
13. **GREEN**: 최소 구현 — registry map + resolve + adopt, executor orchestration, matcher glob, bash os/exec.
14. **REFACTOR**: mcp_adapter를 `sync.Once` + `atomic.Pointer`로 정리, schema validator를 `github.com/santhosh-tekuri/jsonschema` 래퍼로 추출.

### 6.7 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **Tested** | 90%+ 커버리지 목표 (core는 95%+), integration test에 실제 bash 실행 + 실제 fs write, `go test -race` 필수 |
| **Readable** | Tool interface는 4 메서드만, builtin은 파일당 100 LoC 이하, 명명 규약 일관(`Name()` = 캐노니컬) |
| **Unified** | `golangci-lint` (errcheck, govet, staticcheck, ineffassign, gosec), JSON Schema로 입력 통일 |
| **Secured** | Secret env 자동 필터링(REQ-TOOLS-016), Cwd 경계(REQ-TOOLS-015), schema validation 강제, timeout kill tree |
| **Trackable** | `Executor.Run` 구조화 로그(REQ-TOOLS-020), 모든 tool 호출에 `tool_use_id` 관통, ToolResult.Metadata에 outcome 기록 |

### 6.8 의존성 결정 (라이브러리)

| 라이브러리 | 버전 | 용도 | 근거 |
|----------|------|-----|-----|
| `github.com/bmatcuk/doublestar/v4` | v4.6+ | Glob pattern (permission + tool) | `**` 지원, 의존성 없음 |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6+ | JSON Schema draft 2020-12 validation | draft 2020-12 완전 지원, 저의존 |
| `go.uber.org/zap` | v1.27+ | 구조화 로그 | CORE-001 계승 |
| `github.com/stretchr/testify` | v1.9+ | 테스트 | 기존 계승 |
| 표준 `os/exec`, `os/signal`, `regexp`, `encoding/json` | - | builtin 구현 | 외부 의존 최소화 |

**의도적 미사용**:
- `github.com/BurntSushi/ripgrep-go` (존재하지 않음) — Grep은 Go `regexp`로 충분, ripgrep 바이너리 외부 실행은 의존성 증가 → Phase 3 거절.
- `github.com/creack/pty` (Bash의 pty 래핑) — Phase 3은 pipe-based only, pty는 CLI-001/TUI에서 필요 시 채택.

### 6.9 QUERY-001과의 인터페이스 조정

QUERY-001 §6.2의 `QueryEngineConfig.Tools tools.Registry`와 `internal/tools.Executor`는 **본 SPEC에서 정의하는 타입**을 가리킨다. `permissions.CanUseTool`은 `internal/permissions/` 패키지(QUERY-001 정의)에 있으며, 본 SPEC의 `Executor`가 이를 주입받아 호출만 한다. 

실행 순서 (AC-TOOLS-004/005/007과 정렬):

```
QUERY-001 queryLoop:
  tool_use 블록 파싱
    → TOOLS-001 Executor.Run(ctx, req)
         ├── Registry.Resolve              (REQ-TOOLS-001)
         ├── JSON Schema validate          (REQ-TOOLS-014)
         ├── PermissionMatcher.Preapproved (REQ-TOOLS-018)  ← 새 계층
         ├── CanUseTool.Check              (QUERY-001 REQ-QUERY-006)
         └── Tool.Call
    → ToolResult
  ToolResult → QUERY-001 ApplyResultBudget (QUERY-001 REQ-QUERY-007, 본 SPEC의 budget.go 호출)
  tool_result block → messages 추가
```

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger, context root, graceful shutdown |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `ToolsConfig`, `PermissionsConfig` 로딩 |
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | `permissions.CanUseTool`, `ToolPermissionContext` 타입 소유 (본 SPEC이 구현을 **호출**) |
| 선행 SPEC | SPEC-GOOSE-MCP-001 | `mcp.Connection`, `mcp.Manager`, `FetchToolManifest`, `ConnectionClosed` 이벤트 |
| 후속 SPEC | SPEC-GOOSE-CLI-001 | `goose tool list` 서브커맨드가 `Registry.ListNames` + `Inventory.ForModel` 호출 |
| 후속 SPEC | SPEC-GOOSE-COMMAND-001 | slash command가 tool을 참조 (`/bash`, `/read` 등 custom command) |
| 후속 SPEC | SPEC-GOOSE-SUBAGENT-001 | teammate별 tool visibility 제어 (`useExactTools`, `tools: ["*"]`) |
| 후속 SPEC | SPEC-GOOSE-HOOK-001 | PreToolUse/PostToolUse hook이 `Executor.Run` 감싸는 middleware로 진입 |
| 외부 | `doublestar/v4` v4.6+ | glob |
| 외부 | `jsonschema/v6` | schema validation |
| 외부 | Go 1.22+ | generics, sync/atomic.Pointer |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | `sync.Once` 기반 MCP lazy fetch에서 fetch 실패 시 재시도 불가 (Once는 1회) | 고 | 중 | realErr pointer 저장 + `time.Duration` backoff 재시도는 `Search.InvalidateCache(serverID)`로 명시적 reset. MCP-001 reconnect 이벤트에서 invalidate 호출 |
| R2 | Permission glob 구문이 사용자가 기대와 다른 매칭 (ex `Bash(git *)`가 `git` 단독을 포함하는지) | 중 | 중 | doublestar 표준 의미론 따름. 문서화 + 실 테스트 20+ 케이스. 실패 시 경고 로그 |
| R3 | Bash secret env 필터링이 우연히 사용자 환경변수 삭제 | 중 | 고 | 필터는 **시크릿 heuristic 패턴(*_TOKEN/KEY/SECRET)만** 제거. 일반 env(PATH, HOME, LANG 등)는 inherit. `inherit_secrets: true` 옵션 + pre-approval 필수 |
| R4 | MCP 이름 충돌(`mcp__a__tool` vs `mcp__a__tool` 중복) | 낮 | 중 | Adoption 시점 ErrDuplicate 반환, 로그. MCP-001이 동일 serverID 중복 연결 금지 |
| R5 | Cwd 경계 검사가 symlink를 통한 우회 허용 | 중 | 고 | `filepath.EvalSymlinks` → `filepath.Clean` → `strings.HasPrefix(absPath, cwdAbs + sep)` 형태로 정규화 후 검사 |
| R6 | JSON Schema validator 성능 (매 호출마다 compile) | 중 | 낮 | Registry 등록 시점에 `schema.Compile(schemaBytes)` 1회 수행, compiled schema를 Tool과 함께 캐시 |
| R7 | 병렬 tool 실행 미지원으로 LLM이 동시 `tool_use` 블록 여러 개 반환 시 비효율 | 낮 | 낮 | Phase 3은 순차 고정. Phase 4+에서 TOOLS-002로 확장 검토 |
| R8 | 내장 tool의 OS 차이 (Bash on Windows 없음) | 중 | 중 | Phase 3은 darwin/linux 전용 (CORE-001과 정렬). Windows는 향후 `sh -c` → `cmd.exe /C` 매퍼 추가 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/claude-primitives.md` §3.1(이중 역할), §3.2(Transport), §3.4(Deferred Loading / ToolSearch), §2.4(Model Invocation 제약 — SAFE_SKILL_PROPERTIES 패턴 계승), §9(재사용 80%)
- `.moai/project/research/claude-core.md` §1(queryLoop runTools 호출 순서), §2(Task 타입 — tool은 task가 아님 명확화), §6 설계원칙 5(coordinator 모드 tool visibility)
- `.moai/project/structure.md` §172-211 (`internal/tools/` 기대 레이아웃), §842 (Tool auto-registry)
- `.moai/specs/ROADMAP.md` §4 Phase 3 row 16, §7 MVP Milestone 1
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` §3.2 OUT(tools.Executor 위임), REQ-QUERY-006/007/012/017, AC-QUERY-002/003/009/010
- `.moai/specs/SPEC-GOOSE-MCP-001/spec.md` (동시 작성 중 — `mcp.Connection`, `mcp.Manager`, `FetchToolManifest` API)

### 9.2 외부 참조

- JSON Schema draft 2020-12: https://json-schema.org/draft/2020-12/schema
- doublestar glob: https://github.com/bmatcuk/doublestar
- Claude Code ToolSearchTool (패턴만): `./claude-code-source-map/tools/` (존재 시)
- Hermes `model_tools.py` (패턴만): `./hermes-agent-main/hermes/*.py`

### 9.3 부속 문서

- `./research.md` — claude-primitives §3.4 deferred loading 구현 전략 상세, permission matcher 구문 결정 근거, builtin 6종 선정 이유
- `../ROADMAP.md` — Phase 3 의존 그래프
- `../SPEC-GOOSE-COMMAND-001/spec.md` — Slash command가 tool을 소비하는 경로
- `../SPEC-GOOSE-CLI-001/spec.md` — CLI에서 `goose tool list`로 Inventory 노출

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **MCP transport / OAuth / reconnection을 구현하지 않는다**. `mcp.Connection`, `mcp.Manager` 인터페이스 호출만. MCP-001 구현 책임.
- 본 SPEC은 **Permission UI를 구현하지 않는다**. `PermissionMatcher.Preapproved`만 제공하며 `Ask` branch의 사용자 대화창은 CLI-001(TUI) / HOOK-001(useCanUseTool 사용자 프롬프트) 책임.
- 본 SPEC은 **병렬 tool 실행을 지원하지 않는다**. 한 응답 내 tool_use 블록 여러 개는 QUERY-001이 순차 dispatch.
- 본 SPEC은 **sandbox 격리(chroot/seccomp/capabilities drop)를 포함하지 않는다**. Bash는 현재 프로세스 권한으로 실행. SAFETY-001에서 강화.
- 본 SPEC은 **Plan Mode read-only guard를 구현하지 않는다**. SUBAGENT-001이 `permissionMode: plan` 도입 시 Executor 진입 전 필터.
- 본 SPEC은 **Tool 생성 / plugin 확장을 포함하지 않는다**. PLUGIN-001.
- 본 SPEC은 **Web tool (WebFetch/WebSearch/Scrape)을 포함하지 않는다**. 향후 TOOLS-002.
- 본 SPEC은 **Agent / Memory / Skill / Vision / Code tool을 포함하지 않는다**. 각각 SUBAGENT-001, MEMORY-001, SKILLS-001, 별도 SPEC.
- 본 SPEC은 **PTY 지원을 포함하지 않는다** (pipe-based stdout/stderr only). 향후 CLI-001 TUI에서 필요 시.
- 본 SPEC은 **Windows 지원을 보장하지 않는다**. darwin/linux 우선 (CORE-001 정렬).
- 본 SPEC은 **Tool 결과 스트리밍을 지원하지 않는다**. `ToolResult.Content []byte` 전량 반환. 스트리밍 tool 결과는 미래 확장.

---

**End of SPEC-GOOSE-TOOLS-001**
