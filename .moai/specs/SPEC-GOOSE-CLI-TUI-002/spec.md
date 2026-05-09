---
id: SPEC-GOOSE-CLI-TUI-002
version: "0.1.1"
status: completed
created_at: 2026-05-05
updated_at: 2026-05-10
author: manager-spec
priority: P1
labels: [tui, cli, bubbletea, permission, ux]
issue_number: null
---

# SPEC-GOOSE-CLI-TUI-002 — goose CLI TUI 보강 (teatest harness + permission UI + streaming UX + session UX)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — SPEC-GOOSE-CLI-001 v0.2.0 (completed, PR #67/#69/#70/#71/#72 + multi-turn 후속) 위에 4가지 보강 영역 추가: ① bubbletea teatest 하네스 + visual snapshot, ② tool call permission modal UI (HOOK-001 미루지 않고), ③ streaming UX + statusbar 고도화 (spinner/throughput/multi-line editor/markdown), ④ in-TUI session save/load + recent-sessions menu + edit/regenerate. CLI-001 §3.2 OUT 의 "완전한 인터랙티브 permission UI" 항목 흡수. | manager-spec |
| 0.1.1 | 2026-05-10 | sync — bulk implemented→completed 일괄 갱신 시점 기록. P1~P4-T1 구현 PR #107~#111 머지 + sync(#117) 머지로 13 AC GREEN 완수. P4-T2/T3 는 SPEC-GOOSE-CLI-TUI-003 으로 분리되어 별도 SPEC 으로 implemented (#113~#116 + sync). 본 entry 에서 frontmatter status `implemented` → `completed` 전환. spec 본문/요구사항/AC 변경 없음 — 메타 갱신 only. | manager-docs |

---

## 1. 개요 (Overview)

본 SPEC은 **SPEC-GOOSE-CLI-001 v0.2.0** (completed) 의 bubbletea TUI를 4개 영역으로 보강하는 brownfield refactor + greenfield 추가를 정의한다. CLI-001은 REPL 진입, slash dispatch 통합, ChatStream 스트리밍, 단일 세션 jsonl 저장까지 완료했고, 본 SPEC은 사용자 체감 품질을 결정하는 "마무리 UX" 4축을 한 묶음으로 다룬다:

1. **Area 1 — bubbletea teatest 하네스 + visual snapshot**: TUI 회귀 자동 검출.
2. **Area 2 — Tool call permission modal UI**: QUERY-001 `permission_request` SDKMessage 를 진짜 modal로 표시 (현재 CLI-001 §3.2 OUT 으로 한 줄 y/n 만 — 본 SPEC 이 정식 처리).
3. **Area 3 — Streaming UX + statusbar 고도화**: spinner + token throughput + cost + multi-line editor + markdown/code 강조.
4. **Area 4 — Multi-turn polish + session UX**: in-TUI `/save`/`/load`, Ctrl-R recent menu, Ctrl-Up edit/regenerate.

본 SPEC 수락 시점에서:

- `internal/cli/tui/testdata/snapshots/*.golden` 8+ 파일이 회귀 보호.
- 사용자가 destructive tool 호출 시 modal 로 Allow once / Deny once / Allow always / Deny always 선택 가능, "always" 선택은 `~/.goose/permissions.json` 영속.
- statusbar 가 spinner + tokens/sec + elapsed + (옵션) cost 를 표시.
- `/save <name>`, `/load <name>` 가 TUI 안에서 동작, `Ctrl-R` 로 recent 10개 세션 목록 overlay, `Ctrl-Up` 으로 마지막 user message 편집 후 재전송.
- Streaming 중 permission modal 이 뜨면 stream/입력은 일시정지, 결정 후 자동 재개.

CLI-001 v0.2.0의 모든 행동(exit code, slash dispatch, multi-turn replay)은 byte-identical 로 보존한다.

## 2. 배경 (Background)

### 2.1 CLI-001 v0.2.0 완료 상태 + 미해결 UX 격차

CLI-001 progress.md (lines 600-669) 기준:
- Phase A/B/C/D 모두 머지: PR #67 (proto+ConnectClient), #69 (cobra wiring), #70+#71 (TUI factory+characterization), #72 (Dispatcher integration tests)
- Multi-turn replay 후속 PR (transport `WithInitialMessages` + `SplitMessagesAtLastUser`) 완료
- `internal/cli/tui/` 8 source files, ~800 LoC, 72.5% cover

**미해결 격차** (CLI-001 progress.md §"다음 후속 (별도 SPEC 권장)" + §3.2 OUT 마지막 항목):

| 격차 | 원인 | 본 SPEC 해소 |
|-----|------|-----------|
| TUI visual 회귀 검출 불가 | teatest harness 미도입 | Area 1 |
| Tool 권한 prompt가 "한 줄 y/n" 수준 | CLI-001 §3.2 OUT — HOOK-001 으로 미룸 | Area 2 (HOOK-001 의존 깨고 직접 구현) |
| Streaming 진행도가 statusbar `[Streaming...]` 단일 문자열 | 우선순위 낮춤 | Area 3 |
| Single-line input + literal 출력 (markdown 무시) | textinput 단일 컴포넌트 | Area 3 |
| In-TUI `/save`, `/load`, recent menu 부재 | CLI-001 §3.2 OUT 의 "Prompt history persistence" 부분만 OUT, 나머지는 Phase D 잔여 | Area 4 |
| Edit/regenerate 부재 | 미정의 | Area 4 (선택적, scope creep 시 별도 SPEC 분리) |

### 2.2 Brownfield + Greenfield 혼재 구조

- **[EXISTING]**: `dispatch.go`, `slash.go`, transport/adapter (변경 없음, 회귀 보호 대상)
- **[MODIFY]**: `model.go`, `view.go`, `update.go`, `client.go` (state struct 확장, 새 디스패치 분기 추가)
- **[NEW]**: `permission/`, `snapshots/`, `sessionmenu/`, `editor/` (4개 신규 패키지 — 격리된 sub-model)

`research.md` §1.2~§1.4 참조: 변경 영향 분석 + KeyEscape 충돌 해결 전략.

### 2.3 범위 경계 (한 줄)

- **IN**: `internal/cli/tui/{model,view,update,client}.go` 보강, `internal/cli/tui/{permission,snapshots,sessionmenu,editor}/` 신규, `internal/cli/tui/testdata/snapshots/*.golden`, `~/.goose/permissions.json` store, proto `AgentService/ResolvePermission` unary RPC (research.md §3.3 옵션 A).
- **OUT**: theme customization, session encryption at rest, telemetry, Windows-specific signal handling, web-based TUI, agent switching UI (`/agent`), MCP server CLI 직접 기동, plugin install 실제 구현.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE — 4 augmentation areas

#### Area 1 — bubbletea teatest 하네스 + visual snapshot

1. **신규 의존**: `github.com/charmbracelet/x/exp/teatest` (또는 동등 stable 버전).
2. **`internal/cli/tui/snapshots/helper.go`** 신규 — golden file 비교 helper:
   - `SetupAsciiTermenv(t *testing.T)` — `lipgloss.SetColorProfile(termenv.Ascii)` + cleanup
   - `FixedClock(t time.Time) func() time.Time` — spinner/elapsed 결정성
   - `RequireSnapshot(t *testing.T, name string, output []byte)` — `-update` flag 지원
3. **Snapshot 파일** (`internal/cli/tui/testdata/snapshots/`): 최소 8개:
   - `chat_repl_initial_render.golden`
   - `streaming_in_progress.golden`
   - `streaming_aborted.golden`
   - `permission_modal_open.golden`
   - `permission_modal_persisted.golden`
   - `slash_help_local.golden`
   - `session_menu_open.golden`
   - `editor_multiline.golden`
4. **`go test -update ./internal/cli/tui/...`** 명령으로 일괄 갱신 가능.
5. terminfo 비의존 (TERM=dumb 환경에서도 동작).

#### Area 2 — Tool call permission modal UI

1. **신규 패키지** `internal/cli/tui/permission/`:
   - `model.go` — `PermissionModel` sub-model, `PermissionRequest{ToolUseID, ToolName, Input}`, `PermissionChoice{Behavior (Allow|Deny), Persist (None|Once|Always)}`
   - `view.go` — lipgloss modal box (centered overlay), 4 options 라벨
   - `update.go` — Tab/Shift-Tab navigation, Enter 확정, Esc = Deny once, 단축키 (a/d for once, A/D for always)
   - `store.go` — `~/.goose/permissions.json` atomic R/W, schema `{"version": 1, "tools": {"<name>": "allow"|"deny"}}`
2. **modal 활성 시**:
   - statusbar 우측에 `[awaiting permission]` 배지
   - main input은 disabled (KeyEnter는 modal에 위임)
   - 진행 중 streaming은 일시정지 (KeyEscape 처리는 modal mode 분기로 cancel-stream과 충돌 회피)
3. **proto 확장**: `AgentService/ResolvePermission(ResolvePermissionRequest{tool_use_id, behavior})` unary RPC 추가 (research.md §3.3 옵션 A).
4. **client.go**: `ChatStream` adapter가 `permission_request` SDKMessage 를 감지하면 `PermissionRequestMsg` (tea.Msg)로 변환하여 model 에 전달.
5. **persist 정책**:
   - "Allow once" / "Deny once" → 메모리만 (process 종료 시 사라짐)
   - "Allow always (this tool)" / "Deny always (this tool)" → `~/.goose/permissions.json` 에 즉시 atomic write
   - 다음 세션 시작 시 store 로드 → 같은 tool_name 이면 modal 표시 없이 즉시 응답
6. **flag 옵션**: `--ephemeral-permissions` (또는 `cli.permissions.ephemeral=true`) 시 always 선택도 process 한정 적용 (REQ-CLITUI-013 의 "once 선택 시 disk 미저장" 보강).

#### Area 3 — Streaming UX + statusbar 고도화

1. **Spinner**: `bubbles/spinner` 도입. statusbar 좌측에 `streaming` 시 회전 표시.
2. **Token throughput**:
   - Stream chunk 수신 시 tokens 카운트 (간이: rune count; 정확도는 후속), 매 250ms 마다 throughput tick `StreamProgressMsg{tokensIn, tokensOut, elapsed, throughput}` 발생
   - Statusbar 표시: `↓ 142 tok | ↑ 387 tok | 23 t/s | 4.2s`
3. **Cost estimate (옵션, REQ-CLITUI-014)**:
   - LLM provider response가 `usage{input_tokens, output_tokens}` 포함 시
   - `cli.pricing.<model>.input_per_million` / `cli.pricing.<model>.output_per_million` (config) 으로 cost 계산
   - 데이터 부재 시 graceful no-op (no crash, no fallback warning)
   - statusbar 우측 하단 `~$0.0042` 표시
4. **Multi-line editor**:
   - 신규 패키지 `internal/cli/tui/editor/` — `EditorModel` (single/multi mode), `bubbles/textarea` wrap
   - `Ctrl-N` → 모드 토글 (single ↔ multi)
   - multi 모드에서: `Enter` = send, `Ctrl-J` = 행 추가
   - single 모드에서: `Enter` = send (legacy), `Ctrl-J` = no-op (warning silent)
5. **Markdown / code highlighting**:
   - `glamour.NewTermRenderer(WithAutoStyle())` 도입 — assistant message rendering 시 apply
   - Code fence (` ```go ... ``` `)은 chroma syntax highlight (glamour 내장)
   - User/system message 는 plain text 유지 (역할 표시는 lipgloss)
6. **Abort hint**:
   - streaming 중 statusbar 우측에 `Ctrl-C: abort` 작은 텍스트 항상 표시
   - 완료 시 사라짐

#### Area 4 — Multi-turn polish + session UX

1. **`/save <name>`** in-TUI:
   - dispatcher provider 또는 TUI-local handler 로 등록 (research.md §10 참조 — provider 권장)
   - 현재 messages → `~/.goose/sessions/<name>.jsonl` atomic write (CLI-001 session/file.go 재사용)
   - 표시: `[saved: <name>]` system message
   - 빈 name (`/save`) → inline mini-input 으로 이름 입력 → 검증 후 저장
2. **`/load <name>`** in-TUI:
   - 현재 unsaved 메시지 존재 시 confirm modal: `Save current session? [y/n/cancel]`
   - 로드 후: `connectClientFactory` 가 `WithInitialMessages` 옵션 (CLI-001 multi-turn 후속) 으로 새 ChatStream 시작
   - 표시: `[loaded: <name>, N messages restored]`
3. **`Ctrl-R` recent sessions menu**:
   - 신규 패키지 `internal/cli/tui/sessionmenu/`
   - `~/.goose/sessions/*.jsonl` mtime 기준 desc 정렬, 최대 10개
   - overlay: list + cursor, Arrow Up/Down, Enter = load, Esc = dismiss
   - 빈 목록 시 `[no recent sessions]` 표시 후 즉시 닫힘
4. **`Ctrl-Up` edit last user message** (선택적, scope creep 시 별도 SPEC):
   - 직전 user message 가 입력창에 로드 + 모드 = "edit" (커서가 선택된 메시지 인덱스 기억)
   - Enter → 기존 user msg + 다음 assistant msg 를 messages 슬라이스에서 제거 + 새 message 로 ChatStream 재시작
   - Esc → edit 모드 취소, 입력창 clear
5. **Note (scope risk)**: Recent sessions menu (3) + edit/regenerate (4) 가 구현 복잡도 폭발 시 4번 항목을 별도 후속 SPEC (예: SPEC-GOOSE-CLI-TUI-003) 으로 분리. plan.md §Risks 에 명시.

### 3.2 OUT OF SCOPE — 명시적 제외 (defer to future SPECs)

- **Theme customization** (`cli.tui.theme=nord` 등 자유 색상): CLI-001 REQ-CLI-025에 hint 만 있고 본 SPEC도 미구현. 별도 SPEC.
- **Session encryption at rest** (`~/.goose/sessions/*.jsonl` 암호화): 별도 보안 SPEC.
- **Telemetry collection** (사용 통계, error reporting): 별도 SPEC.
- **Windows-specific signal handling**: CLI-001 §3.2 OUT 그대로 유지. WSL2 권장.
- **Web-based TUI** (xterm.js 등 브라우저): ROADMAP §10 OUT.
- **Agent switching UI** (`/agent <name>`): SUBAGENT-001 의존, 별도 SPEC.
- **MCP server CLI 직접 기동**: MCP-001 daemon 내부 관리.
- **Slash command 자동완성 popover**: 별도 UX SPEC 후보.
- **Plugin install 실제 구현**: PLUGIN-001 의존.
- **OAuth login UI** (`goose login`): AUTH-001 의존.
- **Session 검색 / fuzzy filter**: Recent menu 는 단순 mtime 정렬만. 향후.
- **Inline image / video rendering** (kitty graphics protocol): CLI-001 §3.2 OUT 유지.
- **다중 세션 동시 TUI** (split pane): CLI-001 §3.2 OUT 유지.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-CLITUI-001 [Ubiquitous]** — The TUI test harness **shall** use `lipgloss.SetColorProfile(termenv.Ascii)` and a fixed clock during snapshot tests so that golden files are byte-identical across machines and terminfo configurations.

**REQ-CLITUI-002 [Ubiquitous]** — Permission decisions stored to `~/.goose/permissions.json` **shall** be persisted atomically (tmp file + rename), schema-versioned (`{"version": 1, ...}`), and re-loaded at every TUI startup before the first ChatStream begins.

**REQ-CLITUI-003 [Ubiquitous]** — The statusbar **shall** always display the current TUI mode/state in a single line: idle (`Session: <name> | Daemon: <addr> | Messages: <N>`), streaming (adds spinner + throughput + elapsed + abort hint), permission (adds `[awaiting permission]` badge), or session-menu (overlays without removing statusbar).

**REQ-CLITUI-004 [Ubiquitous]** — While a permission modal is active, the TUI **shall not** forward streaming chunks to the messages viewport; received `stream_event` payloads **shall** be buffered until the modal resolves, then flushed in arrival order.

**REQ-CLITUI-005 [Ubiquitous]** — All in-TUI help text, modal labels, slash command response strings, and statusbar prompts **shall** be rendered in the user's `conversation_language` setting (read from `.moai/config/sections/language.yaml`); identifier names (key labels like `Ctrl-R`, tool names, file paths) remain English.

### 4.2 Event-Driven (이벤트 기반)

**REQ-CLITUI-006 [Event-Driven]** — **When** a `permission_request` SDKMessage arrives on the ChatStream, the TUI **shall** convert it to a `PermissionRequestMsg` tea.Msg, set `permissionState.active=true`, render the modal, and pause input dispatch to the main editor until the user resolves.

**REQ-CLITUI-007 [Event-Driven]** — **When** the user presses `Ctrl-R` in the TUI (and no modal is active), the sessionmenu overlay **shall** open, populated with up to 10 entries from `~/.goose/sessions/*.jsonl` sorted by mtime descending; arrow keys move the cursor, Enter loads the selected session, Esc dismisses.

**REQ-CLITUI-008 [Event-Driven]** — **When** the user presses `Ctrl-N` in the editor, the editor mode **shall** toggle between single-line (`textinput`) and multi-line (`textarea`); the active component receives focus and prior buffer content is preserved across the toggle.

**REQ-CLITUI-009 [Event-Driven]** — **When** the user presses `Ctrl-Up` while the input is empty and at least one user message exists in history, the TUI **shall** load the most recent user message into the editor, mark `editingMessageIndex` with that message's slice index, and switch to `mode=edit` (visual indicator: input prompt changes to `(edit)>`); pressing Enter then removes the original user message and the immediately following assistant message from the slice and submits the edited text as a new ChatStream request.

**REQ-CLITUI-010 [Event-Driven]** — **When** the user submits `/save <name>` or `/load <name>` in the TUI, the TUI **shall** invoke the session save/load handler (CLI-001 `session/file.go`), display a confirmation system message in the messages viewport (`[saved: <name>]` or `[loaded: <name>, <N> messages]`), and on `/load` reset the active ChatStream by passing `WithInitialMessages` (CLI-001 multi-turn 후속) to the next chat invocation.

### 4.3 State-Driven (상태 기반)

**REQ-CLITUI-011 [State-Driven]** — **While** the TUI is in `streaming=true` state, the statusbar **shall** render a spinner frame, the latest token throughput tick (`↓ <N> tok | ↑ <M> tok | <T> t/s | <E>s`), and the abort hint `Ctrl-C: abort`; updates **shall** occur at a minimum rate of 4 Hz (every 250 ms) without blocking incoming stream chunks.

**REQ-CLITUI-012 [State-Driven]** — **While** `permissionState.active=true`, all key input from the user **shall** be routed exclusively to the permission modal's update handler; input destined for the main editor **shall** be queued in `editor.pendingBuffer` (max 4 KiB) and replayed in order to the editor after the modal resolves.

### 4.4 Unwanted Behavior (방지)

**REQ-CLITUI-013 [Unwanted]** — The TUI **shall not** render assistant message content that contains raw ANSI escape sequences from user input or LLM output without first passing through `glamour.RenderBytes` (which escape-encodes raw control sequences); additionally, the TUI **shall not** persist a permission decision to `~/.goose/permissions.json` when the user selected `Allow once` or `Deny once` (only `Allow always` and `Deny always` write to disk).

### 4.5 Optional (선택적)

**REQ-CLITUI-014 [Optional]** — **Where** the LLM provider returns a `usage{input_tokens, output_tokens}` payload AND `cli.pricing.<model_name>` config keys are present, the TUI **shall** display a cumulative cost estimate `~$<X.XXXX>` in the statusbar bottom-right; absence of either condition **shall** result in graceful no-op (no error, no log noise).

---

## 5. 수용 기준 (Acceptance Criteria)

각 REQ 마다 최소 1개 이상의 Given/When/Then. 총 18 시나리오. teatest snapshot은 4가지 시각 상태 이상을 cover.

**AC-CLITUI-001 — Snapshot harness 결정성 (REQ-CLITUI-001)** — ✅ [IMPLEMENTED: PR #107]
- **Given** `tui/snapshots/helper.go` 의 `SetupAsciiTermenv(t)` + `FixedClock(2026-05-05T12:00:00Z)` 적용된 테스트
- **When** 동일 테스트를 macOS 와 linux CI 양쪽에서 실행
- **Then** `chat_repl_initial_render.golden` 바이트가 100% 일치 (no ANSI escape, no terminfo-dependent bytes)

**AC-CLITUI-002 — `chat_repl_initial_render.golden` 회귀 보호 (REQ-CLITUI-001)** — ✅ [IMPLEMENTED: PR #107]
- **Given** 초기 `Model` (no messages, sessionName="(unnamed)", daemonAddr="127.0.0.1:17891"), WindowSize 80x24, ascii termenv
- **When** `tm := teatest.NewTestModel(t, model); tm.Quit(); out := tm.FinalOutput(t)`
- **Then** out 이 `testdata/snapshots/chat_repl_initial_render.golden` 와 byte-equal. 표시 내용 검증: statusbar 1 line + viewport (empty) + input prompt `> `

**AC-CLITUI-003 — Permission modal opens on permission_request (REQ-CLITUI-006)** — ✅ [IMPLEMENTED: PR #109]
- **Given** TUI 활성, mock client 가 `ChatStream` 응답 stream 에 `permission_request{tool_use_id:"t1", tool_name:"Bash", input:{"command":"rm -rf /tmp/x"}}` 페이로드 주입
- **When** stream 이 도착하여 model 이 메시지 처리
- **Then** `permissionState.active==true`, `PermissionModel.Request.ToolName=="Bash"`, snapshot `permission_modal_open.golden` 일치 (modal box 렌더 검증)

**AC-CLITUI-004 — Allow always persists to disk (REQ-CLITUI-002, REQ-CLITUI-013)** — ✅ [IMPLEMENTED: PR #109]
- **Given** permission modal open for tool_name="Bash", `~/.goose/permissions.json` 미존재 (tmpdir HOME)
- **When** 사용자가 Tab 으로 "Allow always (this tool)" 선택 → Enter
- **Then** `~/.goose/permissions.json` 파일 생성, 내용 = `{"version":1,"tools":{"Bash":"allow"}}`, modal 닫힘, `client.ResolvePermission("t1", Allow)` RPC 호출됨, snapshot `permission_modal_persisted.golden` 일치 (다음 Bash 호출에서 modal 미표시)

**AC-CLITUI-005 — Allow once does NOT persist (REQ-CLITUI-013)** — ✅ [IMPLEMENTED: PR #109]
- **Given** permission modal open for tool_name="FileWrite", `~/.goose/permissions.json` 미존재
- **When** 사용자가 Enter (기본 "Allow once" 선택)
- **Then** `~/.goose/permissions.json` 파일이 여전히 미존재, modal 닫힘, `client.ResolvePermission("t1", Allow)` 호출됨, in-memory `permissionState.activeTools` 에 "FileWrite" 미기록 (다음 호출에서 modal 다시 표시)

**AC-CLITUI-006 — Streaming pauses while modal open (REQ-CLITUI-004, REQ-CLITUI-012)** — ✅ [IMPLEMENTED: PR #109]
- **Given** TUI streaming 활성, mock client 가 5개 chunk 를 250ms 간격으로 yield, 3번째 chunk 직전에 `permission_request` 주입
- **When** modal 이 열린 후 1초 대기, 사용자가 Enter (Allow once)
- **Then** modal open 동안 viewport 메시지 길이 변화 없음, modal close 후 4번째/5번째 chunk 가 viewport 에 순서대로 추가됨, 누락 없음

**AC-CLITUI-007 — Statusbar token throughput 표시 (REQ-CLITUI-011)** — ✅ [IMPLEMENTED: PR #108]
- **Given** TUI streaming 활성, mock 이 매 100ms 마다 10 token chunk yield (총 5 chunk = 500ms = 50 tokens)
- **When** 1 second 대기 후 statusbar 캡처
- **Then** statusbar 에 `streaming` spinner frame, `↑ 50 tok`, `100 t/s` 근사값 (±10%), `~1.0s` elapsed, `Ctrl-C: abort` hint 모두 포함. snapshot `streaming_in_progress.golden` 일치 (elapsed 와 throughput 은 fixed clock + deterministic mock 으로 결정성)

**AC-CLITUI-008 — Streaming aborted snapshot (REQ-CLITUI-011, CLI-001 REQ-CLI-009 회귀)** — ✅ [IMPLEMENTED: PR #108]
- **Given** TUI streaming 활성
- **When** 사용자가 Ctrl-C (1회, confirmQuit 모드 진입)
- **Then** snapshot `streaming_aborted.golden` 일치. 후속 Ctrl-C 는 quit, 후속 다른 키는 cancel-confirm — 본 SPEC 은 CLI-001 동작 보존만 검증

**AC-CLITUI-009 — Multi-line editor toggle (REQ-CLITUI-008)** — ✅ [IMPLEMENTED: PR #108]
- **Given** TUI 활성, single-line mode (textinput), 입력 내용 = "hello\n"
- **When** 사용자가 Ctrl-N
- **Then** mode=multi (textarea), 기존 "hello\n" 버퍼 보존, focus 가 textarea 로 이동, snapshot `editor_multiline.golden` 일치

**AC-CLITUI-010 — Multi-line Ctrl-J inserts newline, Enter sends (REQ-CLITUI-008)** — ✅ [IMPLEMENTED: PR #108]
- **Given** TUI multi-line mode, 입력 = "line1"
- **When** 사용자가 Ctrl-J → "line2" 타이핑 → Enter
- **Then** ChatStream 에 송신된 user message content = "line1\nline2", input cleared, mode 는 multi 유지

**AC-CLITUI-011 — Markdown code rendering (REQ-CLITUI-013)** — ✅ [IMPLEMENTED: PR #108]
- **Given** assistant message content = `"Here is code:\n\n` + ` ``` `+`go\nfunc main() {}\n`+` ``` `+`\n"`
- **When** message 가 viewport 에 추가됨
- **Then** glamour 가 `func main() {}` 를 chroma 로 색상 강조 (테스트는 ascii termenv 이므로 색상 대신 box border 또는 indent 검증), raw markdown ` ``` ` 마커는 viewport 출력에 없음 (glamour 가 제거)

**AC-CLITUI-012 — `/save <name>` writes jsonl (REQ-CLITUI-010)** — ✅ [IMPLEMENTED: PR #110]
- **Given** TUI 활성, 1 user + 1 assistant 메시지, tmpdir HOME
- **When** 사용자가 `/save test01` 입력 후 Enter
- **Then** `~/.goose/sessions/test01.jsonl` 파일 생성 (atomic), 2 줄 (user/assistant), system message `[saved: test01]` viewport 표시

**AC-CLITUI-013 — `/load <name>` restores session (REQ-CLITUI-010)** — ✅ [IMPLEMENTED: PR #110]
- **Given** `~/.goose/sessions/test01.jsonl` 에 2 메시지 존재, TUI 활성 (현재 0 메시지)
- **When** 사용자가 `/load test01` 입력 후 Enter
- **Then** viewport 에 2 메시지 복원, system message `[loaded: test01, 2 messages]` 표시, 다음 ChatStream 호출이 `WithInitialMessages` 로 2 메시지 포함

**AC-CLITUI-014 — Ctrl-R recent menu (REQ-CLITUI-007)** — ⏸️ [DEFERRED: CLI-TUI-003]
- **Given** `~/.goose/sessions/` 에 3 개 jsonl (mtime 다름), TUI 활성
- **When** 사용자가 Ctrl-R
- **Then** sessionmenu overlay 열림, 3 entry mtime desc 정렬, 첫 번째 cursor 강조, snapshot `session_menu_open.golden` 일치. Esc → overlay 닫힘 (no side effect)

**AC-CLITUI-015 — Ctrl-Up edit last user message (REQ-CLITUI-009)** — ⏸️ [DEFERRED: CLI-TUI-003]
- **Given** TUI 활성, messages = `[user:"hello", assistant:"hi"]`, input 비어있음
- **When** 사용자가 Ctrl-Up → input 에 "hello world" 로 변경 → Enter
- **Then** messages 슬라이스에서 기존 `user:"hello"` + `assistant:"hi"` 제거, `user:"hello world"` 추가, ChatStream 호출 (재전송), 새 assistant 응답 도착 시 append. `editingMessageIndex` 는 -1 로 reset

**AC-CLITUI-016 — Cost estimate (REQ-CLITUI-014)** — ✅ [IMPLEMENTED: PR #108]
- **Given** TUI streaming 활성, config `cli.pricing.claude-3-5-sonnet.input_per_million=3.0`, `output_per_million=15.0`, mock provider 가 stream 종료 시 `usage{input_tokens:1000, output_tokens:500}` 포함
- **When** stream 종료 후 statusbar 캡처
- **Then** statusbar 우측에 `~$0.0105` 표시 (1000 × 3.0/1e6 + 500 × 15.0/1e6 = 0.003 + 0.0075 = 0.0105), graceful no-op 검증: pricing 키 부재 시 cost 부분 미표시 (no error)

**AC-CLITUI-017 — Slash help local snapshot 회귀 (CLI-001 AC-CLI-008 보강)** — ✅ [IMPLEMENTED: PR #107]
- **Given** TUI 활성
- **When** 사용자가 `/help` 입력 후 Enter
- **Then** snapshot `slash_help_local.golden` 일치, 네트워크 호출 0회 (mock client.ChatStream invocation count == 0 — CLI-001 REQ-CLI-021 회귀 보호)

**AC-CLITUI-018 — In-TUI text language conformance (REQ-CLITUI-005)** — ⏸️ [DEFERRED: CLI-TUI-003]
- **Given** `.moai/config/sections/language.yaml` 의 `language.conversation_language=ko`, TUI 시작 직후
- **When** 다음 4개 표면을 캡처: (a) statusbar idle 상태 prompt, (b) `/help` 응답 system message, (c) permission modal label/button 텍스트, (d) `Ctrl-R` session menu 헤더
- **Then** 각 표면의 자연어 부분(키 라벨 `Ctrl-R`/`Tab`/`Enter`, 도구명 `Bash`/`FileWrite`, 파일 경로는 제외)에 ko 로컬라이즈된 사전 정의 substring 1개 이상 포함 (예: `세션:`, `대화 명령어`, `이 도구 호출을 허용하시겠습니까?`, `최근 세션`)
- **Sub-test (en)**: `language.conversation_language=en` 으로 재기동 시 동일 4 표면이 영어 substring(`Session:`, `Conversation commands`, `Allow this tool call?`, `Recent sessions`)으로 렌더 — i18n catalog 가 키 기반으로 양방향 동작함을 증명
- **Snapshot 정책**: 본 AC 의 4 표면은 `*_ko.golden` / `*_en.golden` 2종 golden 으로 격리 (REQ-CLITUI-001 결정성 보존)

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃 (research.md §8)

```
internal/cli/tui/
├── model.go                [MODIFY]   # streamingState/permissionState/sessionMenuState/editorMode 필드 추가
├── view.go                 [MODIFY]   # statusbar 토큰 throughput, modal overlay 분기
├── update.go               [MODIFY]   # KeyCtrlR/CtrlN/CtrlUp/CtrlJ 디스패치 추가
├── client.go               [MODIFY]   # permission_request 디코딩, ResolvePermission RPC 호출
├── dispatch.go             [EXISTING]
├── slash.go                [EXISTING]
├── permission/             [NEW]
│   ├── model.go view.go update.go store.go
│   └── *_test.go
├── snapshots/              [NEW]
│   ├── helper.go (production code)
│   └── helper_test.go
├── sessionmenu/            [NEW]
│   ├── model.go view.go update.go loader.go
│   └── *_test.go
├── editor/                 [NEW]
│   ├── model.go update.go
│   └── *_test.go
└── testdata/snapshots/     [NEW]
    └── *.golden (8+ files)
```

`Reference: internal/cli/tui/model.go:41` — `Model` struct 확장 진입점.
`Reference: internal/cli/tui/update.go:14` — `handleKeyMsg` 디스패치 진입점.
`Reference: internal/cli/tui/view.go:46` — `renderStatusBar` 토큰 throughput 추가 위치.
`Reference: internal/cli/transport/adapter.go:1` — `WithInitialMessages` (CLI-001 multi-turn 후속) 패턴 — `/load` 에서 재사용.

### 6.2 DELTA 마커 적용

| 파일 | 분류 | 변경 사유 |
|------|------|---------|
| `model.go` | [MODIFY] | streamingState/permissionState/sessionMenuState/editorMode 필드 추가; 기존 필드 보존 |
| `view.go` | [MODIFY] | renderStatusBar 확장, modal overlay 렌더 분기 |
| `update.go` | [MODIFY] | KeyCtrlR/CtrlN/CtrlUp/CtrlJ 핸들러 추가, permission mode 분기 |
| `client.go` | [MODIFY] | permission_request decode, ResolvePermission RPC wrapper |
| `dispatch.go` | [EXISTING] | 변경 없음 — `AppInterface.ProcessInput` 인터페이스 안정 |
| `slash.go` | [EXISTING] | 변경 없음 — legacy 경로 보존 |
| `tui_test.go`, `dispatch_test.go`, `phase_c_test.go`, `connect_factory_test.go` | [EXISTING] | 회귀 보호 — 0건 변경 |
| `permission/`, `snapshots/`, `sessionmenu/`, `editor/` | [NEW] | greenfield 신규 패키지 |

### 6.3 TDD 진입 순서 (RED → GREEN → REFACTOR)

Phase 진행은 plan.md §1 phase decomposition 참조. RED 테스트 우선순위:

1. **RED #1**: `TestSnapshot_ChatREPL_InitialRender` (Area 1) — AC-CLITUI-002
2. **RED #2**: `TestSnapshot_Helper_Determinism_AcrossOSes` (Area 1) — AC-CLITUI-001
3. **RED #3**: `TestStatusbar_Streaming_Throughput` (Area 3) — AC-CLITUI-007
4. **RED #4**: `TestEditor_CtrlN_TogglesMode` (Area 3) — AC-CLITUI-009
5. **RED #5**: `TestEditor_MultiLine_CtrlJ_NewlineEnter_Send` (Area 3) — AC-CLITUI-010
6. **RED #6**: `TestRender_MarkdownCodeBlock_GlamourEscapes` (Area 3) — AC-CLITUI-011
7. **RED #7**: `TestPermission_Modal_OpensOnRequest` (Area 2) — AC-CLITUI-003
8. **RED #8**: `TestPermission_AllowAlways_PersistsToDisk` (Area 2) — AC-CLITUI-004
9. **RED #9**: `TestPermission_AllowOnce_DoesNotPersist` (Area 2) — AC-CLITUI-005
10. **RED #10**: `TestPermission_StreamPaused_WhileModalOpen` (Area 2) — AC-CLITUI-006
11. **RED #11**: `TestSession_Save_WritesJsonl` (Area 4) — AC-CLITUI-012
12. **RED #12**: `TestSession_Load_RestoresWithInitialMessages` (Area 4) — AC-CLITUI-013
13. **RED #13**: `TestSessionMenu_CtrlR_OpensList` (Area 4) — AC-CLITUI-014
14. **RED #14**: `TestEdit_CtrlUp_ReplacesLastTurn` (Area 4) — AC-CLITUI-015
15. **RED #15**: `TestStatusbar_CostEstimate_FromUsage` (Area 3) — AC-CLITUI-016
16. **RED #16**: `TestSlashHelp_LocalNoNetwork_Snapshot` (regression) — AC-CLITUI-017
17. **GREEN**: 위 16 테스트를 phase 순서대로 (Area 1 → 3 → 2 → 4) 통과
18. **REFACTOR**: streamingState/permissionState 의 mode-machine 을 explicit FSM 으로 추출, sub-model interface 통일

### 6.4 TRUST 5 매핑

| 차원 | 본 SPEC 의 달성 방법 |
|-----|-----------------|
| **Tested** | 신규 패키지 (permission/sessionmenu/editor) 85%+, 수정 패키지 75%+, teatest snapshot 8+, race 검증 |
| **Readable** | sub-model 패키지 격리 (각 ~150-300 LoC), `Model` struct 의 new 필드는 의미 단위로 grouping |
| **Unified** | golangci-lint 유지, lipgloss style 공용 helper, snapshot golden 갱신 절차 표준화 (`-update` flag) |
| **Secured** | `~/.goose/permissions.json` atomic write + path validation (CLI-001 ValidateName 재사용), glamour 가 ANSI escape sanitize, permission 영속 동의 의무 (Allow once 는 disk 저장 금지) |
| **Trackable** | 새 RPC `ResolvePermission` 은 audit log 가능, statusbar throughput tick 은 stream-event-driven (관측 가능) |

### 6.5 의존성 결정 (라이브러리)

| 라이브러리 | 버전 | 용도 | 신규/계승 |
|----------|------|-----|---------|
| `github.com/charmbracelet/x/exp/teatest` | 최신 stable | TUI snapshot 하네스 | **신규** |
| `github.com/charmbracelet/glamour` | v0.7+ | markdown/code 렌더 | **신규** |
| `github.com/charmbracelet/bubbles/spinner` | v0.20+ | streaming spinner | **신규** (bubbles 자체는 CLI-001 계승) |
| `github.com/charmbracelet/bubbles/textarea` | v0.20+ | multi-line editor | **신규** |
| `github.com/charmbracelet/bubbletea` | v1.2+ | TUI framework | 계승 |
| `github.com/charmbracelet/lipgloss` | v1.1+ | 스타일링 | 계승 |
| `github.com/muesli/termenv` | (lipgloss 의존) | color profile 제어 | 계승 (직접 사용은 snapshots/helper.go) |

**의도적 미사용**:
- `github.com/charmbracelet/bubbles/list` — sessionmenu 는 자체 구현 (10 entry 단순 cursor, list 의존 과도)
- `github.com/charmbracelet/huh` — form 컴포넌트 너무 무거움. permission modal 은 자체 구현

### 6.6 proto 변경 (research.md §3.3 옵션 A)

```proto
// proto/goose/v1/agent.proto (CLI-001 v0.2.0 기반에 추가)

service AgentService {
  rpc Chat(ChatRequest) returns (ChatResponse);
  rpc ChatStream(ChatStreamRequest) returns (stream ChatStreamEvent);
  rpc ResolvePermission(ResolvePermissionRequest) returns (ResolvePermissionResponse);  // NEW
}

message ResolvePermissionRequest {
  string tool_use_id = 1;
  string behavior = 2;     // "allow" | "deny"
}
message ResolvePermissionResponse {
  bool ok = 1;
  string error = 2;        // empty if ok
}
```

Daemon 측 구현은 본 SPEC 범위 밖 (QUERY-001 `engine.ResolvePermission` wrapper). 본 SPEC 은 client-side 만 — 서버 stub 은 별도 SPEC 또는 본 SPEC 의 task 분해 시 daemon RPC handler 추가 (research.md §10 의존성 표 참조).

### 6.7 KeyEscape 의미 분기 표 (research.md §1.4)

| 상태 | KeyEscape 의미 |
|------|------------|
| permission modal active | "Deny once" (modal 닫힘 + ResolvePermission(Deny)) |
| sessionmenu overlay active | overlay 닫힘 (no side effect) |
| edit mode active | edit 모드 취소, input clear |
| streaming + no modal | (CLI-001 동작) streaming cancel + `[Response cancelled]` |
| 그 외 (idle) | no-op |

업데이트 우선순위: modal > overlay > edit > streaming > idle.

### 6.8 Persistence Layout

```
~/.goose/
├── sessions/                  # CLI-001 계승
│   ├── <name>.jsonl
│   └── .last.jsonl
└── permissions.json           # NEW (Area 2)
    {
      "version": 1,
      "tools": {
        "Bash": "allow",
        "FileEdit": "deny"
      }
    }
```

검증 (REQ-CLITUI-002):
- atomic write (tmp + rename)
- schema version 매 startup 검증, mismatch 시 rename 백업 + warn 로그
- 빈 파일 / 손상 파일 시 zero state 로 시작 (graceful)

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC (FROZEN) | SPEC-GOOSE-CLI-001 v0.2.0 (completed) | TUI base, transport, slash dispatch, session jsonl. **본 SPEC 은 CLI-001 의 모든 행동을 byte-identical 보존** |
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | `permission_request` SDKMessage payload, `ResolvePermission(toolUseID, behavior)` 시맨틱 (research.md §3) |
| 선행 SPEC | SPEC-GOOSE-COMMAND-001 | Slash dispatcher provider 패턴 (`/save`, `/load` 신규 provider 등록) |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `~/.goose/` 경로 + 신규 `cli.pricing.<model>.*` 키 |
| 선행 SPEC | SPEC-GOOSE-TRANSPORT-001 | proto `AgentService` — `ResolvePermission` RPC 추가 (서버 stub 은 본 SPEC 외 또는 task 분해로 흡수) |
| 후속 SPEC (deferred) | SPEC-GOOSE-HOOK-001 | 본 SPEC 이 흡수한 permission UI 의 고급 정책 (per-input pattern matching, regex deny 등) |
| 후속 SPEC (deferred) | SPEC-GOOSE-CLI-TUI-003 (proposed) | scope creep 시 Area 4 의 edit/regenerate + recent menu 분리 (plan.md §Risks) |
| 외부 | `connectrpc.com/connect-go` v1.16+ | client transport (CLI-001 계승) |
| 외부 | `charmbracelet/bubbletea` v1.2+ | TUI (계승) |
| 외부 | `charmbracelet/lipgloss` v1.1+ | 스타일 (계승) |
| 외부 | `charmbracelet/bubbles` v0.20+ | spinner/textarea/viewport (textinput/viewport 계승, spinner/textarea 신규) |
| 외부 | `charmbracelet/glamour` v0.7+ | markdown/code 렌더 (신규) |
| 외부 | `charmbracelet/x/exp/teatest` 최신 | TUI snapshot (신규) |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Snapshot 결정성 깨짐 (terminfo, OS 차이) | 중 | 고 | REQ-CLITUI-001 의 ascii termenv + fixed clock 강제, CI에서 macOS+linux 양쪽 검증, snapshot 갱신은 PR review 시 시각 검토 의무 |
| R2 | `~/.goose/permissions.json` 동시 접근 (동일 호스트에서 다중 TUI 인스턴스) | 낮 | 중 | atomic rename + flock(`syscall.Flock`) helper. 충돌 시 retry max 3회, 실패 시 modal 에 warning |
| R3 | Multi-line editor state machine 복잡도 (edit mode + multi mode + permission mode 동시) | 고 | 중 | KeyEscape 의미 분기 표 (§6.7) 명문화, mode 우선순위 explicit FSM, refactor phase 에서 추출 |
| R4 | Scope creep — Area 4 의 Ctrl-R recent menu + Ctrl-Up edit 가 두 SPEC 분량 | 고 | 중 | plan.md §1 phase 4 task 진행 중 LoC 가 250 초과 시 별도 SPEC (SPEC-GOOSE-CLI-TUI-003) 분리. plan.md 명시 |
| R5 | `ResolvePermission` 서버 stub 부재 — daemon 미구현 시 client 만 작성하면 수용 테스트 불가 | 중 | 고 | task 분해에서 daemon RPC handler 도 포함 (proto 추가 + handler skeleton). QUERY-001 engine.ResolvePermission 위임만 |
| R6 | glamour 의 LoC 증가 (binary size + 의존 트리) | 낮 | 낮 | 약 2-3MB 증가 — 수용 가능. 후속 SPEC 에서 가능하면 chroma 직접 사용으로 다이어트 (본 SPEC 외) |
| R7 | Permission modal 이 streaming 일시정지로 사용자가 지연 체감 | 중 | 낮 | statusbar `[awaiting permission]` 명확 표시 + modal centered overlay 로 직관적, default 옵션 = "Allow once" (Enter 1회로 즉시 진행) |
| R8 | Session menu 가 `~/.goose/sessions/` 에 무효 파일 (corrupted jsonl) 포함 시 crash | 중 | 중 | loader 에서 첫 줄만 검증 (lazy parse), 실패 시 entry 건너뛰고 warn 로그. crash 안 됨 |
| R9 | Cost estimate 가 잘못된 pricing 으로 오해 유발 | 낮 | 낮 | "best-effort" 라벨 + `~$` prefix 명시, config 부재 시 미표시 (graceful no-op REQ-CLITUI-014) |
| R10 | Edit/regenerate 가 messages 슬라이스 인덱스 race (streaming 중 Ctrl-Up 시 어떤 메시지가 last user 인지 모호) | 중 | 중 | streaming 활성 시 Ctrl-Up no-op (statusbar warning), 또는 streaming 종료 후만 활성. plan.md task 명시 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/specs/SPEC-GOOSE-CLI-001/spec.md` lines 168-180 (§3.2 OUT — 본 SPEC 이 흡수하는 항목들)
- `.moai/specs/SPEC-GOOSE-CLI-001/spec.md` line 815 ("완전한 인터랙티브 permission UI 는 HOOK-001" — 본 SPEC 이 정정)
- `.moai/specs/SPEC-GOOSE-CLI-001/progress.md` lines 600-669 (Phase A~D 완료 보고 + multi-turn 후속)
- `.moai/specs/SPEC-GOOSE-CLI-001/progress.md` lines 627-633 (다음 후속 별도 SPEC 권장 — 본 SPEC 이 4번째 항목 흡수)
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` REQ-QUERY-006, REQ-QUERY-013 (permission_request schema, suspend/resume)
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` §6 `engine.ResolvePermission` 시그니처
- `.moai/specs/SPEC-GOOSE-COMMAND-001/spec.md` (slash dispatcher provider 등록)
- `.moai/project/tech.md` (의존성 정책)
- `.moai/specs/ROADMAP.md` Phase 3 잔존 보강

### 9.2 외부 참조

- bubbletea: https://github.com/charmbracelet/bubbletea
- bubbles (textarea/spinner/textinput/viewport): https://github.com/charmbracelet/bubbles
- lipgloss: https://github.com/charmbracelet/lipgloss
- glamour: https://github.com/charmbracelet/glamour
- teatest: https://github.com/charmbracelet/x/tree/main/exp/teatest
- crush (참고 구현, 23k stars): charmbracelet/crush — multi-pane chat TUI
- gh CLI extension framework: cli/cli pkg/cmd/extension/browse — modal 패턴 참고
- termenv: https://github.com/muesli/termenv

### 9.3 부속 문서

- `./research.md` — 코드베이스 + 외부 패턴 + permission RPC 옵션 분석
- `./plan.md` — 4-phase 분해 + mx_plan + reference implementations
- `./acceptance.md` — Given/When/Then 상세 + snapshot 명명 규약 + coverage gate
- `./spec-compact.md` — REQ/AC + Files + Exclusions only
- `./progress.md` — initial skeleton

---

## Exclusions (What NOT to Build)

> **필수 섹션**: 본 SPEC 범위 누수 방지. CLI-001 §3.2 OUT 의 일부 (permission UI) 는 본 SPEC 이 흡수했고, 나머지는 그대로 OUT 유지.

- 본 SPEC 은 **theme customization** 을 구현하지 않는다 (자유 색상, font, ANSI 256 → truecolor 전환 등). CLI-001 REQ-CLI-025 의 4가지 preset 은 별도 SPEC.
- 본 SPEC 은 **session 파일 암호화** 를 구현하지 않는다 (`~/.goose/sessions/*.jsonl` plain text 유지). 별도 보안 SPEC.
- 본 SPEC 은 **telemetry collection** 을 구현하지 않는다. 사용 통계, error reporting, opt-in survey 모두 제외.
- 본 SPEC 은 **Windows 정식 지원을 보장하지 않는다**. CLI-001 §3.2 OUT 그대로 유지 (darwin/linux 우선, WSL2 권장).
- 본 SPEC 은 **web-based TUI** (xterm.js, ttyd 등) 를 구현하지 않는다.
- 본 SPEC 은 **agent switching UI** (`/agent <name>`) 를 구현하지 않는다. SUBAGENT-001 의존.
- 본 SPEC 은 **MCP server CLI 직접 기동** 을 구현하지 않는다. MCP-001 daemon 내부 관리.
- 본 SPEC 은 **slash command 자동완성 popover** 를 구현하지 않는다. 별도 UX SPEC 후보.
- 본 SPEC 은 **plugin install 실제 구현** 을 포함하지 않는다 (PLUGIN-001 의존). CLI-001 stub 유지.
- 본 SPEC 은 **OAuth login UI** (`goose login`) 를 구현하지 않는다 (AUTH-001 의존).
- 본 SPEC 은 **session 검색 / fuzzy filter** 를 구현하지 않는다. recent menu 는 단순 mtime desc 정렬만.
- 본 SPEC 은 **inline image / video rendering** (kitty graphics protocol) 을 구현하지 않는다. CLI-001 §3.2 OUT 유지.
- 본 SPEC 은 **다중 세션 동시 TUI** (split pane, multi-tab) 를 구현하지 않는다. CLI-001 §3.2 OUT 유지.
- 본 SPEC 은 **HOOK-001 전체** 를 구현하지 않는다. 본 SPEC 이 흡수한 것은 **단일 tool 단위 Allow/Deny 모달 + persist** 까지. regex 기반 deny pattern, per-input matching, custom hook script 는 HOOK-001 의 진짜 범위로 남김.
- 본 SPEC 은 **고급 cost analytics** (per-session cost summary, monthly aggregate, alerts) 을 구현하지 않는다. statusbar 의 cumulative `~$X.XXXX` 만.
- 본 SPEC 은 **edit/regenerate 의 다중 turn rewind** (Ctrl-Up 두 번 → 두 turn 전 user msg) 를 구현하지 않는다. 직전 1 user msg 만 (R10 완화 + 복잡도 제어).

---

**End of SPEC-GOOSE-CLI-TUI-002 v0.1.0**
