# SPEC-GOOSE-CLI-TUI-002 — Implementation Plan

> **Phase 진행 순서**: Area 1 (teatest harness) → Area 3 (streaming UX) → Area 2 (permission UI) → Area 4 (session UX). 이유: teatest 가 다른 3 영역의 시각 회귀를 잡아주므로 첫 phase 에 깔아야 함. Streaming UX 가 Area 2 의 permission modal 렌더에 의존하는 statusbar 를 먼저 정리하기 때문에 Area 3 우선.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — 4-phase decomposition (P1 teatest, P2 streaming UX, P3 permission UI, P4 session UX), mx_plan, reference implementations, plan_complete signal block | manager-spec |

---

## 0. Approach Summary

Brownfield refactor (`internal/cli/tui/{model,view,update,client}.go` MODIFY) + greenfield 신규 패키지 4종 (permission/, snapshots/, sessionmenu/, editor/) + proto 추가 1종 (AgentService.ResolvePermission). TDD per phase, RED → GREEN → REFACTOR.

phase 별 file ownership:
- **P1** owner: `tui/snapshots/`, `tui/testdata/snapshots/`, MODIFY `tui/model.go`, `tui/view.go` (minor)
- **P2** owner: `tui/editor/`, MODIFY `tui/{model,view,update}.go`
- **P3** owner: `tui/permission/`, `proto/goose/v1/agent.proto`, MODIFY `tui/{model,view,update,client}.go`
- **P4** owner: `tui/sessionmenu/`, MODIFY `tui/{model,update}.go` (slash command provider 등록 또는 TUI-local handler)

phase 간 worktree 분리 권장 (parallel 진행 가능 영역: P1 + P2 동시; P3 는 P1/P2 머지 후; P4 는 P3 후).

---

## 1. Phase Decomposition (4 phases, 17 tasks 총)

### Phase 1 — bubbletea teatest 하네스 + visual snapshot (Area 1)

**Goal**: TUI 회귀 자동 검출 인프라 구축. 다른 phase 의 시각 변경을 byte-level 로 보호.

**Files**:
- [NEW] `internal/cli/tui/snapshots/helper.go` (~80 LoC)
- [NEW] `internal/cli/tui/snapshots/helper_test.go` (~60 LoC)
- [NEW] `internal/cli/tui/testdata/snapshots/chat_repl_initial_render.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/slash_help_local.golden`
- [NEW] `internal/cli/tui/snapshot_initial_render_test.go` (~120 LoC)
- [NEW] `internal/cli/tui/snapshot_slash_help_test.go` (~80 LoC)
- [MODIFY] `go.mod` (teatest dependency 추가)

**Tasks (5)**:
| # | Task | RED test | Owner |
|---|------|---------|------|
| P1-T1 | `snapshots.helper.go` 구현 (SetupAsciiTermenv, FixedClock, RequireSnapshot with -update flag) | TestSnapshot_Helper_RequireSnapshot_Determinism | implementer |
| P1-T2 | teatest dependency 추가 + go.mod tidy | (compile) | implementer |
| P1-T3 | `chat_repl_initial_render.golden` snapshot 작성 + RED test | TestSnapshot_ChatREPL_InitialRender (AC-CLITUI-002) | tester |
| P1-T4 | `slash_help_local.golden` snapshot 작성 + RED test (회귀 보호 — CLI-001 AC-CLI-008 보강) | TestSlashHelp_LocalNoNetwork_Snapshot (AC-CLITUI-017) | tester |
| P1-T5 | macOS + linux CI matrix 확인, snapshot byte-equal | TestSnapshot_Helper_Determinism_AcrossOSes (AC-CLITUI-001) | tester |

**Acceptance**: AC-CLITUI-001, AC-CLITUI-002, AC-CLITUI-017 PASS. tui 패키지 cover 72.5% → 75%+ 유지.

**Phase exit gate**: snapshot helper 가 production code 로 빌드되고, 2 개 golden 이 CI 양쪽 OS에서 일치. PR 머지.

---

### Phase 2 — Streaming UX + statusbar 고도화 (Area 3)

**Goal**: Streaming 진행도 시각화 + multi-line input + markdown/code 렌더 + cost estimate.

**Files**:
- [NEW] `internal/cli/tui/editor/model.go` (~120 LoC)
- [NEW] `internal/cli/tui/editor/update.go` (~100 LoC)
- [NEW] `internal/cli/tui/editor/editor_test.go` (~180 LoC)
- [MODIFY] `internal/cli/tui/model.go` (streamingState struct 확장, editorMode 필드)
- [MODIFY] `internal/cli/tui/view.go` (renderStatusBar 확장: spinner + throughput + elapsed + abort hint + cost; markdown render via glamour for assistant)
- [MODIFY] `internal/cli/tui/update.go` (KeyCtrlN/CtrlJ 디스패치, StreamProgressMsg tick 핸들러)
- [NEW] `internal/cli/tui/testdata/snapshots/streaming_in_progress.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/streaming_aborted.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/editor_multiline.golden`

**Tasks (5)**:
| # | Task | RED test | Owner |
|---|------|---------|------|
| P2-T1 | `editor/model.go` + `editor/update.go` (single/multi mode, textarea/textinput wrap) | TestEditor_CtrlN_TogglesMode (AC-CLITUI-009) | implementer |
| P2-T2 | Multi-line behavior: Ctrl-J = newline, Enter = send | TestEditor_MultiLine_CtrlJ_NewlineEnter_Send (AC-CLITUI-010) | implementer |
| P2-T3 | `model.go` streamingState 확장 + `view.go` statusbar throughput/spinner/elapsed/abort hint | TestStatusbar_Streaming_Throughput (AC-CLITUI-007), `streaming_in_progress.golden` | implementer + tester |
| P2-T4 | glamour 도입 + assistant message markdown rendering | TestRender_MarkdownCodeBlock_GlamourEscapes (AC-CLITUI-011) | implementer |
| P2-T5 | Cost estimate (옵션, REQ-CLITUI-014) + `streaming_aborted.golden` 회귀 보호 | TestStatusbar_CostEstimate_FromUsage (AC-CLITUI-016), AC-CLITUI-008 snapshot | implementer + tester |

**Acceptance**: AC-CLITUI-007 ~ AC-CLITUI-011, AC-CLITUI-016 PASS. editor/ 패키지 cover 85%+. tui 패키지 modify 영향 cover ≥ 75%.

**Phase exit gate**: 3 새 snapshot 갱신, throughput tick 4 Hz 보장 (race 검증), glamour markdown 렌더 회귀 0건.

---

### Phase 3 — Tool call permission modal UI (Area 2)

**Goal**: QUERY-001 `permission_request` SDKMessage 를 진짜 modal로 표시 + persist + ResolvePermission RPC.

**Files**:
- [NEW] `internal/cli/tui/permission/model.go` (~150 LoC)
- [NEW] `internal/cli/tui/permission/view.go` (~100 LoC)
- [NEW] `internal/cli/tui/permission/update.go` (~120 LoC)
- [NEW] `internal/cli/tui/permission/store.go` (~120 LoC, atomic R/W + flock)
- [NEW] `internal/cli/tui/permission/*_test.go` (~300 LoC)
- [MODIFY] `proto/goose/v1/agent.proto` (ResolvePermission RPC 추가)
- [NEW] daemon 측 RPC handler skeleton (`internal/transport/grpc/agent_service.go` 또는 동등 — daemon 측 entry point에 위임 wrapper만; 실제 engine.ResolvePermission 호출은 QUERY-001 deps)
- [MODIFY] `internal/cli/tui/client.go` (permission_request decode → PermissionRequestMsg, ResolvePermission RPC wrapper)
- [MODIFY] `internal/cli/tui/model.go` (permissionState 필드)
- [MODIFY] `internal/cli/tui/view.go` (modal overlay 렌더 분기)
- [MODIFY] `internal/cli/tui/update.go` (permission mode 분기, KeyEscape 의미 분기 — §6.7 표 적용)
- [NEW] `internal/cli/tui/testdata/snapshots/permission_modal_open.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/permission_modal_persisted.golden`

**Tasks (5)**:
| # | Task | RED test | Owner |
|---|------|---------|------|
| P3-T1 | `permission/store.go` (atomic + flock + schema version) | TestPermissionStore_AtomicWrite_FlockSafe | implementer |
| P3-T2 | proto `ResolvePermission` RPC + buf generate + daemon stub | (compile + integration) | implementer |
| P3-T3 | `permission/{model,view,update}.go` modal sub-model | TestPermission_Modal_OpensOnRequest (AC-CLITUI-003), `permission_modal_open.golden` | implementer + tester |
| P3-T4 | client.go permission_request decode + ResolvePermission wrapper, model.go permissionState 통합, KeyEscape 분기 | TestPermission_AllowAlways_PersistsToDisk (AC-CLITUI-004), TestPermission_AllowOnce_DoesNotPersist (AC-CLITUI-005) | implementer |
| P3-T5 | streaming pause/resume during modal, persisted-tool fast-path | TestPermission_StreamPaused_WhileModalOpen (AC-CLITUI-006), `permission_modal_persisted.golden` | implementer + tester |

**Acceptance**: AC-CLITUI-003 ~ AC-CLITUI-006 PASS. permission/ 패키지 cover 85%+.

**Phase exit gate**: ResolvePermission RPC 가 client/server 양쪽에서 동작 (mock daemon E2E), `~/.goose/permissions.json` 영속, KeyEscape 분기 표 모든 상태 검증.

---

### Phase 4 — Multi-turn polish + session UX (Area 4)

**Goal**: in-TUI `/save`, `/load`, Ctrl-R recent menu, Ctrl-Up edit/regenerate.

**Files**:
- [NEW] `internal/cli/tui/sessionmenu/model.go` (~100 LoC)
- [NEW] `internal/cli/tui/sessionmenu/view.go` (~80 LoC)
- [NEW] `internal/cli/tui/sessionmenu/update.go` (~80 LoC)
- [NEW] `internal/cli/tui/sessionmenu/loader.go` (~80 LoC, mtime scan)
- [NEW] `internal/cli/tui/sessionmenu/*_test.go` (~200 LoC)
- [MODIFY] `internal/cli/tui/model.go` (sessionMenuState 필드, editingMessageIndex)
- [MODIFY] `internal/cli/tui/update.go` (KeyCtrlR / KeyCtrlUp 디스패치)
- [MODIFY] `internal/cli/tui/dispatch.go` 또는 `slash.go` (`/save`, `/load` provider 등록 또는 TUI-local handler — research.md §10 참조; provider 권장이지만 dispatcher 변경 부담 시 TUI-local 도 허용)
- [NEW] `internal/cli/tui/testdata/snapshots/session_menu_open.golden`

**Tasks (4)**:
| # | Task | RED test | Owner |
|---|------|---------|------|
| P4-T1 | `/save <name>` + `/load <name>` slash command provider (또는 TUI-local handler), 기존 session/file.go (CLI-001) 재사용 | TestSession_Save_WritesJsonl (AC-CLITUI-012), TestSession_Load_RestoresWithInitialMessages (AC-CLITUI-013) | implementer |
| P4-T2 | `sessionmenu/{loader,model,view,update}.go` Ctrl-R overlay | TestSessionMenu_CtrlR_OpensList (AC-CLITUI-014), `session_menu_open.golden` | implementer + tester |
| P4-T3 | Ctrl-Up edit mode 구현 (model.go editingMessageIndex, update.go 디스패치, view.go 입력 prompt 변경) | TestEdit_CtrlUp_ReplacesLastTurn (AC-CLITUI-015) | implementer |
| P4-T4 | streaming 중 Ctrl-Up no-op + warning, R10 완화 검증 | (보강 테스트) | tester |

**Acceptance**: AC-CLITUI-012 ~ AC-CLITUI-015 PASS. sessionmenu/ cover 85%+.

**Phase exit gate**: 모든 17 task 머지, 17 RED 테스트 GREEN 전환, 8 snapshot 안정. tui 전체 cover ≥ 80%.

**Scope creep guard**: P4-T2 + P4-T3 가 합산 LoC 250 초과 시 P4-T2/T3 를 별도 SPEC (SPEC-GOOSE-CLI-TUI-003) 으로 분리하고 본 SPEC 은 P4-T1 만 수용 (Recent menu + Edit/regenerate 는 후속). 분리 결정은 P4-T2 RED 시점에 LoC 추정으로 판단.

---

## 2. Phase Ordering Rationale

```
P1 (teatest harness) ────────────┐
                                  ├──► P3 (permission UI)  ──► P4 (session UX)
P2 (streaming UX + editor) ──────┘
```

- **P1 first**: snapshot harness 가 P2/P3/P4 의 시각 회귀를 잡음. P1 없이 진행 시 P2 statusbar 변경이 P3 modal 렌더와 충돌해도 검출 안 됨.
- **P1 + P2 parallel 가능**: 다른 파일/패키지 owner. P1=snapshots/+testdata/, P2=editor/+model.go(streamingState 부분만). model.go 충돌은 explicit merge 필요.
- **P3 after P1+P2**: P3 의 permission_modal_open.golden snapshot 이 P2 의 statusbar 변경 (`[awaiting permission]` 배지 위치) 에 의존. P3 의 KeyEscape 분기가 P2 의 streaming-cancel 분기와 충돌 — P2 머지 후 진행.
- **P4 last**: P4 의 sessionmenu overlay 가 P3 의 modal overlay 패턴 재사용. P3 머지 후 진행.

---

## 3. Risks (실행 단계 리스크)

spec.md §8 의 R1~R10 외 plan 단계 추가 리스크:

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| PR-1 | P1 의 teatest 의존이 macOS CI 에서 동작하지만 linux CI 에서 stdin TTY emulation 차이로 fail | 중 | 고 | teatest 의 `WithoutFinalize()` 옵션 + `tea.WithInput(strings.NewReader(""))` 명시. P1 시작 시 양쪽 OS smoke test 우선 |
| PR-2 | P2 + P3 동시 진행 시 model.go 머지 충돌 (streamingState 추가 vs permissionState 추가가 같은 struct) | 고 | 중 | model.go 의 신규 필드는 명확한 grouping comment 로 구분, P2 는 `// streaming` block, P3 는 `// permission` block. 다른 phase 가 자기 block 만 수정. PR review 시 명시적 검증 |
| PR-3 | P3 의 daemon 측 ResolvePermission handler 가 QUERY-001 engine 미구현 시 nil pointer | 중 | 고 | handler 는 QUERY-001 ResolvePermission 인터페이스가 있는지 startup 검증, 없으면 명확한 error 반환. CLI-TUI-002 task 분해 시 daemon stub 만 추가 (실제 engine 위임은 QUERY-001 의존 명시) |
| PR-4 | P4-T2 + P4-T3 LoC 폭발 → 별도 SPEC 분리 결정이 늦어 phase 4 PR 비대화 | 고 | 중 | P4-T2 RED 작성 직후 LoC 추정 (model+view+update+loader+test). 250 LoC 초과 시 즉시 별도 SPEC 분리. plan.md 명시 |
| PR-5 | 8 golden file 의 -update flag 갱신이 PR 마다 노이즈 | 중 | 낮 | `-update` 는 명시적 인텐트 (테스트 cmd flag), git diff 시각 review 의무. PR description 에 갱신된 snapshot 와 사유 명시 의무 |
| PR-6 | glamour markdown rendering 이 기존 plain message format 과 호환성 깨짐 (existing user/system 메시지 영향) | 중 | 중 | glamour 는 assistant role 한정 적용, user/system 은 plain (역할 표시는 lipgloss 색상 보존). 명시적 분기 in updateViewport |

---

## 4. mx_plan (Phase 3.5 of plan workflow)

`internal/cli/tui/` 의 functions 중 `@MX` annotation 이 implementation phase 에서 추가/업데이트될 대상.

### `@MX:NOTE` 추가 후보 (intent 전달)

| 파일:함수 | 사유 |
|----------|------|
| `tui/snapshots/helper.go:SetupAsciiTermenv` | (NEW) "Snapshot determinism — terminfo 비의존 강제. REQ-CLITUI-001" |
| `tui/snapshots/helper.go:RequireSnapshot` | (NEW) "Golden file 비교 + `-update` flag 처리" |
| `tui/permission/store.go:Save` | (NEW) "Atomic write + flock — REQ-CLITUI-002" |
| `tui/permission/store.go:Load` | (NEW) "Schema version 검증, mismatch 시 백업 + warn" |
| `tui/editor/model.go:Toggle` | (NEW) "Single ↔ Multi 토글 시 buffer 보존" |
| `tui/sessionmenu/loader.go:ScanRecent` | (NEW) "mtime desc 정렬, max 10. 손상 jsonl skip + warn" |
| `tui/view.go:renderStatusBar` (수정) | "Streaming throughput + cost + abort hint + permission badge 분기" |

### `@MX:ANCHOR` 추가/업데이트 후보 (invariant contract, fan_in ≥ 3)

| 파일:함수 | 사유 |
|----------|------|
| `tui/model.go:Model` (struct, 기존 ANCHOR 유지) | "주요 sub-state struct 그룹화 — streamingState, permissionState, sessionMenuState, editorMode. 호출자: View/Update/handleKeyMsg 등" |
| `tui/permission/model.go:PermissionModel` | (NEW) "Modal 의 핵심 contract — Update/View/Resolve 진입점, 호출자: tui/update.go (modal mode 분기), client.go (PermissionRequestMsg 변환)" |
| `tui/sessionmenu/model.go:SessionMenuModel` | (NEW) "Overlay sub-model contract" |
| `tui/editor/model.go:EditorModel` | (NEW) "Editor mode + buffer contract, 호출자: tui/update.go (KeyEnter/KeyCtrlN/KeyCtrlJ), tui/view.go (renderInputArea)" |
| `tui/client.go:ChatStream adapter` (수정) | "permission_request decode 진입점, 호출자: tui/update.go startStreaming, sessionmenu/loader" |

### `@MX:WARN` 추가 후보 (danger zone, requires @MX:REASON)

| 파일:함수 | 사유 |
|----------|------|
| `tui/permission/store.go:saveAtomic` | (NEW) "Concurrent flock — race with multi-instance TUI. @MX:REASON: REQ-CLITUI-002 atomic + flock retry max 3" |
| `tui/update.go:handleKeyMsg` (수정) | "Mode FSM 분기 (modal > overlay > edit > streaming > idle). @MX:REASON: cyclomatic complexity ≥ 15 우려, refactor phase 에서 explicit FSM 추출 검토" |
| `tui/view.go:renderStatusBar` (수정) | "Throughput tick 250 ms — concurrent stream chunk processing 과 race. @MX:REASON: tea.Tick + tea.Cmd 패턴, lock 미사용 (model 단일 소유 원칙)" |

### `@MX:TODO` 추가 (RED 단계 placeholder)

각 RED 테스트 작성 시 대응 production code (modify 대상) 위치에 `@MX:TODO: SPEC-GOOSE-CLI-TUI-002 — implement <area>` 임시 마커. GREEN phase 에서 제거.

### MX policy 준수

- 본 SPEC 은 `code_comments: ko` (`.moai/config/sections/language.yaml`) → @MX 본문은 한국어
- 새 ANCHOR 4개 (PermissionModel, SessionMenuModel, EditorModel, ChatStream adapter modify) 는 fan_in ≥ 3 검증 후 등록
- WARN 3개 모두 @MX:REASON 필수

---

## 5. Reference Implementations (Reference: file:line)

본 SPEC 구현 시 참고할 패턴:

1. **Reference: `internal/cli/tui/model.go:41`** — `Model` struct 확장 진입점. 신규 필드는 기존 필드 뒤에 grouping comment (`// streaming`, `// permission`, `// session menu`, `// editor`) 와 함께 추가
2. **Reference: `internal/cli/tui/dispatch.go:48`** — `DispatchInput` 패턴은 변경 없이 재사용. P4 의 `/save`, `/load` provider 는 dispatcher provider 등록 패턴 (COMMAND-001) 으로 추가
3. **Reference: `internal/cli/transport/adapter.go:1`** — `WithInitialMessages` (CLI-001 multi-turn 후속) 의 적용 패턴. P4-T1 의 `/load` 가 동일 옵션 재사용
4. **Reference: `internal/cli/tui/update.go:14`** — `handleKeyMsg` 의 switch 패턴. 신규 KeyCtrlR/CtrlN/CtrlUp/CtrlJ 는 동일 switch 에 case 추가 (modal/overlay/edit mode 분기 우선)
5. **Reference: `internal/cli/tui/view.go:46`** — `renderStatusBar` 의 lipgloss style 패턴. P2 의 throughput 추가는 동일 스타일 helper 재사용
6. **Reference: `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md:REQ-QUERY-006`** — permission_request payload 구조 명세. client.go decode 시 정확한 JSON 키 사용 (`tool_use_id`, `tool_name`, `input`)
7. **Reference: `internal/cli/transport/connect.go`** (CLI-001 Phase A PR #67) — Connect-Go RPC client factory 패턴. P3 의 `ResolvePermission` unary RPC wrapper 가 동일 패턴 재사용
8. **Reference: External `github.com/charmbracelet/x/exp/teatest` README** — `NewTestModel`, `Send`, `WaitFinished`, `FinalOutput` 사용 예
9. **Reference: External `github.com/charmbracelet/glamour` README** — `NewTermRenderer(WithAutoStyle())` 옵션. ascii termenv 강제 시 `WithStyles(glamour.AsciiStyleConfig)` 명시
10. **Reference: External `github.com/charmbracelet/crush/internal/tui/components/chat/` (참고만)** — multi-pane chat + modal pattern. 라이선스 확인 후 패턴 차용 (코드 직접 복사 금지)

---

## 6. Plan Completion Signal Block

(이 블록은 plan-auditor PASS 후 채워짐 — 현재는 placeholder)

```yaml
plan_complete_at: <ISO-8601>  # plan-auditor PASS 시점에 기록
plan_status: audit-ready       # 또는 audit-pending / audit-failed
```

---

**End of plan.md**
