# SPEC-GOOSE-CLI-TUI-002 — Research (코드베이스 + 외부 패턴 분석)

> **목적**: SPEC-GOOSE-CLI-001 v0.2.0 (completed) 위에 4가지 보강 영역(teatest harness, permission UI, streaming UX, session UX)을 얹기 전 — 기존 코드 패턴, 외부 라이브러리 API, 의존 SPEC 인터페이스, 참고 구현을 명시적으로 정렬한다.
>
> **범위**: `internal/cli/tui/`, `internal/cli/transport/`, `internal/query/`, `internal/permissions/`, 외부 charmbracelet 생태계.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — CLI-001 v0.2.0 완료 후 후속 보강 SPEC을 위한 코드베이스 + 외부 패턴 분석 | manager-spec |

---

## 1. 기존 `internal/cli/tui/` 코드베이스 분석

### 1.1 파일 목록 + LoC 분석 (2026-05-05 기준)

| 파일 | LoC | 책임 | DELTA 분류 |
|------|-----|------|-----------|
| `model.go` | 122 | `Model` struct (textinput + viewport + state flags), `NewModel`, `Init`, `Update` 디스패치 | [MODIFY] — `streaming` flag → `streamingState` struct로 확장, permission/sessionmenu/editor 모드 필드 추가 |
| `view.go` | 84 | `View` (lipgloss JoinVertical), `renderStatusBar`, `renderInputArea` | [MODIFY] — statusbar 렌더 토큰 throughput/cost/elapsed 필드 추가, modal overlay 분기 추가 |
| `update.go` | 298 | `handleKeyMsg` (KeyCtrlC/Esc/Enter/CtrlS/CtrlL), `handleStreamEvent`, `sendMessage`, `startStreaming`, `saveSession` (placeholder), `updateViewport` | [MODIFY] — KeyCtrlR/CtrlN/CtrlUp 핸들러 추가, permission modal mode 진입/탈출 핸들러, streaming 중 permission 일시정지 로직 |
| `dispatch.go` | 122 | `AppInterface`, `ProcessResult/Kind`, `DispatchInput`, `FormatLocalResult`, `DispatchSlashCmd` | [EXISTING] — 변경 없음 (이미 안정 인터페이스) |
| `slash.go` | (legacy) | `SlashCmd` parse + legacy `HandleSlashCmd` | [EXISTING] — 신규 `/save`, `/load` 슬래시 명령은 dispatcher 경로(processor) 추가로 처리 권장, slash.go 직접 변경은 최소화 |
| `client.go` | (transport bridging) | `connectClientFactory`, `ChatStream` adapter, `WithInitialMessages` 사용 | [MODIFY] — permission_response inbound 채널 추가 (현재 단방향 stream → bidi 비슷한 패턴 필요 검토; 또는 별도 RPC) |
| `*_test.go` | ~1k | tui_test, dispatch_test, slash_test, phase_c_test, connect_factory_test | [EXISTING] — 회귀 보호. 신규 테스트는 별도 `*_teatest_test.go` 파일에 추가 |

**합계**: 8 source + 5 test, 약 800 LoC source / 1k LoC test, tui 패키지 cover 72.5% (CLI-001 progress.md §"Phase D" 보고).

### 1.2 핵심 데이터 구조 (변경 영향 분석)

`Model` (model.go:41) 현재 필드:
- `client DaemonClient` — transport 클라이언트 (ChatStream + Close)
- `app AppInterface` — slash dispatcher 통합 (nil 허용)
- `sessionName string`, `messages []ChatMessage`
- `input textinput.Model`, `viewport viewport.Model`
- `width, height int`
- `streaming bool`, `quitting bool`, `confirmQuit bool`, `noColor bool`

**Brownfield 보강 시 추가될 필드** (Section 6 spec.md 참조):
- `streamingState` — `{active bool, msgIndex int, cancelFn context.CancelFunc, partial bool, tokensIn int, tokensOut int, startedAt time.Time}` (Area 3)
- `permissionState` — `{active bool, request *PermissionRequest, persistChoice PersistMode}` (Area 2)
- `sessionMenuState` — `{visible bool, items []SessionEntry, cursor int}` (Area 4)
- `editorMode` — `enum {SingleLine, MultiLine}` (Area 3)
- `editingMessageIndex int` — Ctrl-Up edit 모드의 대상 인덱스 (Area 4, -1=disabled)

**호환성 원칙**: 기존 필드/메서드는 byte-identical로 유지. 새 필드는 `omitempty` 패턴이 아니라 zero-value가 "비활성"을 의미하도록 설계 (Go 관용).

### 1.3 기존 Update 디스패치 흐름

```
tea.Msg
  ├── tea.KeyMsg ── handleKeyMsg
  │     ├── confirmQuit==true ─ Ctrl-C → quit
  │     ├── KeyCtrlC ─ streaming ? confirmQuit=true : quit
  │     ├── KeyEscape ─ streaming ? cancel + [Response cancelled] msg
  │     ├── KeyEnter ─ slash dispatch → ProcessLocal/Proceed/Exit/Abort
  │     │              ↓ ProcessProceed → sendMessage → startStreaming
  │     ├── KeyCtrlS ─ saveSession (placeholder)
  │     └── KeyCtrlL ─ viewport.GotoTop
  ├── tea.WindowSizeMsg ── handleWindowSize
  └── StreamEventMsg ── handleStreamEvent (text/error/done)
```

**Brownfield 보강 디스패치 추가**:
- `KeyCtrlR` → sessionMenu open (Area 4) — 단, streaming 중에는 무시
- `KeyCtrlN` → editor mode toggle (Area 3) — 단, streaming/permission 중에는 무시
- `KeyCtrlUp` → 마지막 user message edit 모드 진입 (Area 4)
- `KeyCtrlJ` (multi-line 모드) → 입력 내 \n 삽입 (Area 3)
- 새 `tea.Msg` 타입: `PermissionRequestMsg`, `SessionMenuLoadedMsg`, `StreamProgressMsg` (token throughput tick)

### 1.4 `KeyEscape` 의 현재 의미 vs Area 2 충돌

현재 update.go:37 `KeyEscape` = "streaming cancel". Area 2 permission modal에서는 통상 Esc = "deny once". → **충돌 회피 결정**: permission modal active 시 KeyEscape는 modal 한정으로 "deny once" 로 매핑, 일반(streaming only) 상태에서는 기존 cancel 동작 유지. 2-mode 디스패치는 update.go에서 `m.permissionState.active` 분기로 처리 (이는 permissionState 필드 추가의 이유).

## 2. bubbletea + teatest API 분석

### 2.1 bubbletea API surface 점검

현재 의존 (CLI-001 spec.md §6.9):
- `github.com/charmbracelet/bubbletea` v1.2+ (Init/Update/View, tea.Msg, tea.Cmd, tea.Quit)
- `github.com/charmbracelet/lipgloss` v1.1+ (스타일링, JoinVertical/Horizontal)
- `github.com/charmbracelet/bubbles` v0.20+ (textinput, viewport)

**보강 SPEC에서 추가 필요**:
- `github.com/charmbracelet/bubbles/spinner` — Area 3 streaming spinner
- `github.com/charmbracelet/bubbles/list` — Area 4 sessionmenu (또는 자체 구현)
- `github.com/charmbracelet/bubbles/textarea` — Area 3 multi-line editor (현재 textinput은 single-line 전용)
- `github.com/charmbracelet/glamour` — Area 3 markdown/code rendering (lipgloss 위 layered)

**의존성 결정**:
- `textarea` 도입은 `textinput`을 완전 교체할지 보조로 둘지 — Single↔Multi 토글 시 mode-별 다른 컴포넌트를 들고 있다가 `Focus()` 하는 방식이 깔끔. (textinput과 textarea는 둘 다 bubbles에 있으므로 새 외부 의존 없음.)
- `glamour` 신규 도입. 약 2-3MB 추가 binary size 증가 — 수용 가능.

### 2.2 teatest 하네스 API + golden-file snapshot 전략

**참고 패키지**: `github.com/charmbracelet/x/exp/teatest` (실험적이지만 stable production usage in `gh` CLI extension framework, Crush 등).

**핵심 API**:
- `teatest.NewTestModel(t, model, opts...)` — 모델 생성. opts에 `WithInitialTermSize(w, h)` 권장.
- `tm.Send(tea.KeyMsg{...})` — keystroke 주입
- `tm.Type("hello")` — 문자열 타이핑
- `tm.WaitFinished(t, teatest.WithFinalTimeout(5s))` — Quit 대기
- `tm.FinalOutput(t)` — 최종 viewport 바이트 스트림 캡처 (snapshot 비교 대상)
- `tm.GetProgram()` — bubbletea Program 인스턴스 (advanced)

**Golden file snapshot 전략**:

1. 결정성 (determinism) 확보:
   - Test setup에서 `lipgloss.SetColorProfile(termenv.Ascii)` 호출 → ANSI escape 무효화, plain text만 출력
   - 또는 `lipgloss.SetHasDarkBackground(true)` 명시 + color profile 고정
   - 시간 의존(spinner frame, elapsed seconds) 컴포넌트는 테스트에서 시계 mock → `WithStartTime(fixed)` 패턴

2. 파일 경로:
   - `internal/cli/tui/testdata/snapshots/<scenario>.golden` (테이블 케이스명 슬러그)
   - 예: `chat_repl_initial_render.golden`, `streaming_in_progress.golden`, `permission_modal_open.golden`

3. 비교 도구:
   - `teatest.RequireEqualOutput(t, tm.FinalOutput(t))` (option) — 자동 update via `-update` flag
   - 또는 stdlib `bytes.Equal` + `os.WriteFile`로 수동 update flag 처리

4. Update 정책:
   - `go test -update ./internal/cli/tui/...` 로 일괄 갱신 (의도적 UI 변경 시)
   - PR review 시 .golden diff가 변경 의도와 일치하는지 검증 (시각적 review)

5. terminfo 비의존:
   - `TERM=dumb` 환경 변수 설정으로 termenv가 색상/cursor 제어 비활성
   - Lipgloss는 termenv 의존 → `lipgloss.SetColorProfile(termenv.Ascii)` (테스트용 helper 작성)

**Reference**: `github.com/charmbracelet/bubbletea/examples` 의 `progress`, `chat` 데모는 테스트 파일이 거의 없어 제한적. `github.com/charmbracelet/crush` 일부 컴포넌트 테스트가 더 좋은 참고. `gh` CLI extension framework도 teatest 채택 사례.

## 3. SPEC-GOOSE-QUERY-001 `permission_request` SDKMessage 스키마

### 3.1 SDKMessage 타입 enum (QUERY-001 §6.2)

QUERY-001 spec.md에서 추출:

```go
type SDKMessageType string

const (
    SDKMsgUserAck            SDKMessageType = "user_ack"
    SDKMsgStreamEvent        SDKMessageType = "stream_event"
    SDKMsgMessage            SDKMessageType = "message"
    SDKMsgToolUseSummary     SDKMessageType = "tool_use_summary"
    SDKMsgPermissionRequest  SDKMessageType = "permission_request"  // Ask 분기
    SDKMsgPermissionCheck    SDKMessageType = "permission_check"    // Allow/Deny 분기
    SDKMsgCompactBoundary    SDKMessageType = "compact_boundary"
    SDKMsgError              SDKMessageType = "error"
    SDKMsgTerminal           SDKMessageType = "terminal"
)
```

### 3.2 `permission_request` payload 구조 (REQ-QUERY-006)

QUERY-001 REQ-QUERY-006 발췌:

> `Ask` → yield a `permission_request{tool_use_id, tool_name, input}` `SDKMessage` and suspend the loop until a resolution arrives on the engine's permission inbox (REQ-QUERY-013).

→ payload JSON 스키마 (proto/agent.proto의 `ChatStreamEvent.payload_json`에 직렬화):

```json
{
  "tool_use_id": "toolu_abc123",
  "tool_name": "Bash",
  "input": { "command": "rm -rf /tmp/foo", "...": "..." }
}
```

### 3.3 `PermissionDecision` 응답 경로 (REQ-QUERY-013)

QUERY-001 §6 발췌:

```go
// internal/query/engine.go
func (e *QueryEngine) ResolvePermission(
    toolUseID string,
    behavior permissions.Behavior,  // Allow | Deny
) error
```

**문제**: 현재 `internal/cli/transport/connect.go`의 `Chat`/`ChatStream`은 server-streaming (단방향). client → daemon으로 permission decision을 보낼 통로가 없음.

**옵션 분석**:
- **옵션 A**: 별도 unary RPC `AgentService/ResolvePermission(tool_use_id, behavior)` 추가 (proto 변경 필요). 가장 간단.
- **옵션 B**: `ChatStream`을 bidirectional streaming으로 승격 (proto의 `rpc ChatStream(stream Req) returns (stream Resp)`). 큰 변경.
- **옵션 C**: 이미 정의된 `ChatStreamRequest` (initial_messages 보유)에 `permission_responses repeated` 필드를 추가하고 client측 stream으로 send. proto 호환 유지.

→ **권장 = 옵션 A** (Spec.md §6 기술적 접근에 명시). 이유: streaming 중 unary 호출이 가능하며 (Connect-Go는 동일 client에서 병행 호출 지원), proto 변경 최소(새 RPC만 추가), bidi 복잡도 회피.

### 3.4 SDKMessage → TUI 표시 연결고리

현재 `tui/update.go:handleStreamEvent`는 `StreamEventMsg.Event.Type == "text"|"error"|"done"`만 인식. permission 분기는 처리 안 됨.

**보강 시 추가**:
- `client.go`의 `ChatStream` adapter가 `ChatStreamEvent.type == "permission_request"`를 감지하면 별도 `PermissionRequestMsg` (tea.Msg)로 변환하여 model에 전달
- Model.Update에서 `PermissionRequestMsg` 수신 시 `permissionState.active = true`, modal 렌더 트리거
- 사용자 선택 → `client.ResolvePermission(toolUseID, behavior)` unary 호출 → success 시 `permissionState.active = false`, streaming 자동 재개 (daemon side queryLoop가 inbox로 unblock됨)

## 4. lipgloss / glamour 렌더링 파이프라인

### 4.1 현재 (CLI-001 view.go)

- 단순 lipgloss style: foreground color (86=Green, 228=Yellow, 241=Gray) per role
- `lipgloss.JoinVertical(Left, statusBar, viewport.View(), inputArea)`
- `viewport.SetContent(content string)` — content는 plain text + ANSI escape
- 코드 블록 강조 없음, markdown 무시 (literal 출력)

### 4.2 제안 (Area 3 후)

**파이프라인**:
```
ChatMessage.Content (raw markdown)
  └─→ glamour.RenderBytes(md, "dark"|"light"|"ascii")
       └─→ ANSI-styled bytes
            └─→ lipgloss padding/border wrap
                 └─→ viewport.SetContent(...)
```

**glamour 사양**:
- `glamour.NewTermRenderer(opts ...glamour.TermRendererOption)` — opts: `WithStyles(dark)`, `WithWordWrap(80)`, `WithAutoStyle()` (자동 dark/light 감지)
- 코드 블록의 language hint를 chroma syntax highlighter로 처리 (glamour 내장)
- 출력은 ANSI escape 포함 string

**테스트 결정성** (Area 1 snapshot과 호환):
- glamour는 termenv 의존 — 위 §2.2의 ascii color profile 강제로 ANSI 제거
- glamour `ascii` 스타일 직접 지정도 가능 → snapshot 안정

### 4.3 인라인 `code` 처리

- glamour는 inline `code`를 옅은 박스로 렌더 (chroma 미사용, lipgloss only)
- 사용자 입력 대비 보안: glamour는 markdown만 처리, raw ANSI escape는 escape 처리됨 (XSS 유사 안전) → REQ-CLITUI-013 충족 가능

## 5. 참고 TUI 구현 (외부)

### 5.1 charmbracelet/crush

- 23k stars, charm팀 공식 LSP-aware AI chat TUI
- 참고 가치:
  - Multi-pane layout (sidebar + main + input) — Area 4 sessionmenu overlay 패턴
  - Streaming spinner + token count statusbar
  - Slash command 자동완성 popover (본 SPEC OUT, 향후 SPEC 후보)
- 참고 파일 (외부 레포): `internal/tui/components/` — bubbletea pattern 매뉴얼 수준

### 5.2 gh CLI extension framework (cli/cli `pkg/cmd/extension/browse`)

- bubbletea + lipgloss
- Modal pattern 참고: `internal/cmd/...` 의 `confirm.go` 같은 단일 키 입력 대기 모달
- Area 2 permission modal 디자인의 Go 관용 patterns

### 5.3 aider (Python, 참고만)

- Aider chat의 `/save`, `/clear`, `/diff` 등 슬래시 명령 UX 참고
- 권한 prompt: `apply changes? [y/n/a/d]` (always/deny-always 옵션 영감)

### 5.4 Claude Code (TS, Ink 기반)

- CLI-001에서 이미 "개념만 참고"
- Area 2 permission UI: Ink의 `<Permission />` 컴포넌트 → bubbletea로 포팅 (별도 sub-model)

## 6. 기존 테스트 패턴 분석 (회귀 보호)

### 6.1 `tui_test.go` 패턴

CLI-001 progress.md §"Phase C2~C5" (PR #71)에서 추가된 characterization tests:
- `TestModel_Init_DefaultState` — 신규 모델의 기본 필드값
- `TestModel_HandleKey_CtrlC_Quits_NoStreaming`
- `TestModel_HandleKey_CtrlC_StreamingConfirmQuit`
- `TestModel_HandleStreamEvent_Text/Error/Done`
- statusbar message count 표시 검증

### 6.2 `dispatch_test.go` 패턴 (PR #72, Phase D)

- Mock `AppInterface` 주입
- `DispatchInput` / `FormatLocalResult` / `DispatchSlashCmd` 단위 테스트
- 13 신규 테스트, tui cover 59.4% → 72.5%

### 6.3 신규 테스트 추가 시 제약

- 신규 `*_teatest_test.go` 파일은 `//go:build teatest` 빌드 태그로 분리하지 않고 일반 test로 추가 (CI 단순화)
- 단, `teatest` 의존이 무겁다면 `go test -short` 시 skip 패턴 적용 검토
- 회귀 0건 정책 유지 (기존 테스트 byte-identical 결과)

## 7. 알려진 한계 (CLI-001 progress.md §"다음 후속 (별도 SPEC 권장)")

CLI-001 progress.md lines 627-633 직접 인용:

> 다음 후속 (별도 SPEC 권장)
> - 사용자 문서 (README / docs/cli/)
> - E2E with live daemon (docker-compose 또는 long-running binary)
> - daemon shutdown RPC (proto 추가 필요)
> - multi-turn chat replay (ConnectClient.WithInitialMessages 도입) ← **이미 multi-turn 후속 PR로 완료**
> - bubbletea teatest harness 도입 (TUI visual snapshot) ← **본 SPEC Area 1**

→ 본 SPEC은 위 4번째 (teatest harness)를 직접 흡수하고, 5번째(사용자 문서)는 sync phase에서 별도 처리, E2E와 daemon shutdown RPC는 본 SPEC 범위 밖 (spec.md §3.2 OUT).

## 8. 패키지 레이아웃 결정 (proposed)

```
internal/cli/tui/
├── model.go                [MODIFY]   # streamingState/permissionState/sessionMenuState/editorMode 추가
├── view.go                 [MODIFY]   # statusbar 토큰 throughput, modal overlay 분기
├── update.go               [MODIFY]   # KeyCtrlR/CtrlN/CtrlUp 추가, permission mode 분기
├── dispatch.go             [EXISTING] # 변경 없음
├── slash.go                [EXISTING] # 변경 없음 (legacy 유지)
├── client.go               [MODIFY]   # permission_request 디코딩, ResolvePermission RPC 호출
├── permission/             [NEW]      # 권한 modal sub-model
│   ├── model.go            #   modal state, options
│   ├── view.go             #   lipgloss modal box render
│   ├── update.go           #   key handling (Tab/Enter/Esc/y/n/a/d)
│   ├── store.go            #   ~/.goose/permissions.json read/write
│   └── *_test.go
├── snapshots/              [NEW]      # teatest 하네스 helper
│   ├── helper.go           #   asciiTermenv setup, fixedClock, snapshotEqual
│   └── helper_test.go
├── sessionmenu/            [NEW]      # Ctrl-R recent sessions overlay
│   ├── model.go            #   list state, cursor
│   ├── view.go             #   overlay render
│   ├── update.go           #   arrow keys, Enter, Esc
│   ├── loader.go           #   ~/.goose/sessions/*.jsonl mtime scan
│   └── *_test.go
├── editor/                 [NEW]      # multi-line editor + edit mode
│   ├── model.go            #   single/multi mode toggle, textarea wrap
│   ├── update.go           #   KeyCtrlN/KeyCtrlJ/KeyCtrlUp 핸들러
│   └── *_test.go
└── testdata/
    └── snapshots/          [NEW]      # *.golden 파일들
        ├── chat_repl_initial_render.golden
        ├── streaming_in_progress.golden
        ├── streaming_aborted.golden
        ├── permission_modal_open.golden
        ├── permission_modal_persisted.golden
        ├── slash_help_local.golden
        ├── session_menu_open.golden
        └── editor_multiline.golden
```

**의도**:
- 새 sub-model (permission/sessionmenu/editor)은 자체 model.go/view.go/update.go로 격리 → main `Model.Update`는 mode 분기 후 sub-model에 위임
- snapshots/ helper는 testdata/와 별도. helper.go는 production 코드 (테스트 의존 가능)
- `permissions.json` 저장은 `~/.goose/permissions.json` (CLI-001 session 저장 경로와 동일 디렉터리)

## 9. TDD 진입 순서 (제안)

Phase 진행 순서 = Area 1 (teatest) → Area 3 (streaming UX) → Area 2 (permission UI) → Area 4 (session UX). 이유: teatest harness가 다른 3개 영역의 회귀를 잡아주므로 첫 번째로 깔아야 함.

각 영역의 RED 테스트 시작점 (실제 task 분해는 plan.md):
1. Area 1: `TestSnapshot_ChatREPL_InitialRender` — 빈 모델 + WindowSize → snapshot
2. Area 3: `TestStatusbar_TokenThroughput_DisplaysSpinner`, `TestEditor_MultiLineToggle_CtrlN`
3. Area 2: `TestPermission_ModalOpens_OnRequestMsg`, `TestPermission_AllowAlways_PersistsToDisk`
4. Area 4: `TestSessionMenu_CtrlR_OpensList`, `TestEdit_LastUserMessage_CtrlUp`

## 10. 의존 SPEC 인터페이스 명세 (cross-check)

| SPEC | 인터페이스 | 본 SPEC 사용 |
|------|----------|-----------|
| SPEC-GOOSE-CLI-001 v0.2.0 | `Model`, `DaemonClient.ChatStream`, `AppInterface.ProcessInput`, exit code, session jsonl 형식 | brownfield 보강 — 모두 보존 |
| SPEC-GOOSE-QUERY-001 | `SDKMsgPermissionRequest` 페이로드, `ResolvePermission(id, behavior)` | client.go에서 디코딩 + RPC wrapper |
| SPEC-GOOSE-COMMAND-001 | `Dispatcher.ProcessUserInput`, `ProcessResult.Kind` | 변경 없음 (`/save`, `/load`는 Area 4에서 dispatcher provider 등록 또는 TUI-local handler) |
| SPEC-GOOSE-CONFIG-001 | `~/.goose/` 경로 규약 | `~/.goose/permissions.json` 신규 추가 시 검증 (deconflict) |
| SPEC-GOOSE-TRANSPORT-001 | proto `AgentService` | `ResolvePermission` unary RPC 추가 권장 (옵션 A 결정) |

## 11. Reference 인용 (`Reference: file:line` 형식)

- `Reference: internal/cli/tui/model.go:41` — `Model` struct 확장 진입점
- `Reference: internal/cli/tui/update.go:14` — `handleKeyMsg` 디스패치 추가 진입점
- `Reference: internal/cli/tui/view.go:46` — `renderStatusBar` 토큰 throughput 추가 위치
- `Reference: internal/cli/tui/dispatch.go:48` — `DispatchInput` 변경 없이 재사용
- `Reference: internal/cli/transport/adapter.go:1` — `WithInitialMessages` (CLI-001 multi-turn 후속) 패턴 — Area 4 `/load`에서 재사용
- `Reference: .moai/specs/SPEC-GOOSE-QUERY-001/spec.md:REQ-QUERY-006` — permission_request 구조
- `Reference: .moai/specs/SPEC-GOOSE-QUERY-001/spec.md:REQ-QUERY-013` — Ask 분기 suspend/resume
- `Reference: .moai/specs/SPEC-GOOSE-CLI-001/spec.md:§3.2 OUT 마지막 항목` — 본 SPEC이 흡수하는 "완전한 인터랙티브 permission UI" out-of-scope 라인
- 외부: `github.com/charmbracelet/x/exp/teatest` README — teatest 사용 예
- 외부: `github.com/charmbracelet/glamour` README — markdown 렌더 옵션
- 외부: `github.com/charmbracelet/crush/internal/tui/components/chat/` — multi-pane chat 패턴 (사용 라이선스 확인 필요)

---

**End of research.md**
