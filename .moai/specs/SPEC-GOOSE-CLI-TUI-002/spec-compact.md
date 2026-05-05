# SPEC-GOOSE-CLI-TUI-002 — Compact (auto-derived)

> Auto-derived 압축본. spec.md §4 (REQ) + §5 (AC) + plan.md §1 (Files) + spec.md Exclusions 만 verbatim 포함. Overview/Background/Refs/History 제외.

---

## REQ-CLITUI 요구사항 (verbatim)

### 4.1 Ubiquitous

**REQ-CLITUI-001 [Ubiquitous]** — The TUI test harness **shall** use `lipgloss.SetColorProfile(termenv.Ascii)` and a fixed clock during snapshot tests so that golden files are byte-identical across machines and terminfo configurations.

**REQ-CLITUI-002 [Ubiquitous]** — Permission decisions stored to `~/.goose/permissions.json` **shall** be persisted atomically (tmp file + rename), schema-versioned (`{"version": 1, ...}`), and re-loaded at every TUI startup before the first ChatStream begins.

**REQ-CLITUI-003 [Ubiquitous]** — The statusbar **shall** always display the current TUI mode/state in a single line: idle (`Session: <name> | Daemon: <addr> | Messages: <N>`), streaming (adds spinner + throughput + elapsed + abort hint), permission (adds `[awaiting permission]` badge), or session-menu (overlays without removing statusbar).

**REQ-CLITUI-004 [Ubiquitous]** — While a permission modal is active, the TUI **shall not** forward streaming chunks to the messages viewport; received `stream_event` payloads **shall** be buffered until the modal resolves, then flushed in arrival order.

**REQ-CLITUI-005 [Ubiquitous]** — All in-TUI help text, modal labels, slash command response strings, and statusbar prompts **shall** be rendered in the user's `conversation_language` setting (read from `.moai/config/sections/language.yaml`); identifier names (key labels like `Ctrl-R`, tool names, file paths) remain English.

### 4.2 Event-Driven

**REQ-CLITUI-006 [Event-Driven]** — **When** a `permission_request` SDKMessage arrives on the ChatStream, the TUI **shall** convert it to a `PermissionRequestMsg` tea.Msg, set `permissionState.active=true`, render the modal, and pause input dispatch to the main editor until the user resolves.

**REQ-CLITUI-007 [Event-Driven]** — **When** the user presses `Ctrl-R` in the TUI (and no modal is active), the sessionmenu overlay **shall** open, populated with up to 10 entries from `~/.goose/sessions/*.jsonl` sorted by mtime descending; arrow keys move the cursor, Enter loads the selected session, Esc dismisses.

**REQ-CLITUI-008 [Event-Driven]** — **When** the user presses `Ctrl-N` in the editor, the editor mode **shall** toggle between single-line (`textinput`) and multi-line (`textarea`); the active component receives focus and prior buffer content is preserved across the toggle.

**REQ-CLITUI-009 [Event-Driven]** — **When** the user presses `Ctrl-Up` while the input is empty and at least one user message exists in history, the TUI **shall** load the most recent user message into the editor, mark `editingMessageIndex` with that message's slice index, and switch to `mode=edit` (visual indicator: input prompt changes to `(edit)>`); pressing Enter then removes the original user message and the immediately following assistant message from the slice and submits the edited text as a new ChatStream request.

**REQ-CLITUI-010 [Event-Driven]** — **When** the user submits `/save <name>` or `/load <name>` in the TUI, the TUI **shall** invoke the session save/load handler (CLI-001 `session/file.go`), display a confirmation system message in the messages viewport (`[saved: <name>]` or `[loaded: <name>, <N> messages]`), and on `/load` reset the active ChatStream by passing `WithInitialMessages` (CLI-001 multi-turn 후속) to the next chat invocation.

### 4.3 State-Driven

**REQ-CLITUI-011 [State-Driven]** — **While** the TUI is in `streaming=true` state, the statusbar **shall** render a spinner frame, the latest token throughput tick (`↓ <N> tok | ↑ <M> tok | <T> t/s | <E>s`), and the abort hint `Ctrl-C: abort`; updates **shall** occur at a minimum rate of 4 Hz (every 250 ms) without blocking incoming stream chunks.

**REQ-CLITUI-012 [State-Driven]** — **While** `permissionState.active=true`, all key input from the user **shall** be routed exclusively to the permission modal's update handler; input destined for the main editor **shall** be queued in `editor.pendingBuffer` (max 4 KiB) and replayed in order to the editor after the modal resolves.

### 4.4 Unwanted

**REQ-CLITUI-013 [Unwanted]** — The TUI **shall not** render assistant message content that contains raw ANSI escape sequences from user input or LLM output without first passing through `glamour.RenderBytes` (which escape-encodes raw control sequences); additionally, the TUI **shall not** persist a permission decision to `~/.goose/permissions.json` when the user selected `Allow once` or `Deny once` (only `Allow always` and `Deny always` write to disk).

### 4.5 Optional

**REQ-CLITUI-014 [Optional]** — **Where** the LLM provider returns a `usage{input_tokens, output_tokens}` payload AND `cli.pricing.<model_name>` config keys are present, the TUI **shall** display a cumulative cost estimate `~$<X.XXXX>` in the statusbar bottom-right; absence of either condition **shall** result in graceful no-op (no error, no log noise).

---

## AC-CLITUI 수용 기준 (verbatim)

**AC-CLITUI-001** — Snapshot harness 결정성 (REQ-CLITUI-001): Given fixed termenv ascii + fixed clock, when 동일 테스트를 macOS + linux CI 양쪽 실행, then `chat_repl_initial_render.golden` 바이트가 100% 일치.

**AC-CLITUI-002** — `chat_repl_initial_render.golden` 회귀 보호 (REQ-CLITUI-001): Given 초기 Model + 80x24 + ascii termenv, when teatest NewTestModel + Quit + FinalOutput, then golden byte-equal + statusbar/empty-viewport/`> ` prompt 검증.

**AC-CLITUI-003** — Permission modal opens on permission_request (REQ-CLITUI-006): Given mock client 가 permission_request payload 주입, when stream 도착, then permissionState.active=true + ToolName="Bash" + snapshot `permission_modal_open.golden` 일치.

**AC-CLITUI-004** — Allow always persists to disk (REQ-CLITUI-002, REQ-CLITUI-013): Given modal open Bash + tmpdir HOME + permissions.json 미존재, when "Allow always" + Enter, then permissions.json 생성 (`{"version":1,"tools":{"Bash":"allow"}}`) + ResolvePermission RPC 호출 + 다음 Bash 호출 modal 미표시.

**AC-CLITUI-005** — Allow once does NOT persist (REQ-CLITUI-013): Given modal open FileWrite + permissions.json 미존재, when 기본 Enter (Allow once), then permissions.json 미존재 유지 + ResolvePermission 호출 + in-memory 미기록 (다음 호출 modal 재표시).

**AC-CLITUI-006** — Streaming pauses while modal open (REQ-CLITUI-004, REQ-CLITUI-012): Given 5 chunk stream + 3번째 직전 permission_request 주입, when modal 1초 + Allow once, then modal open 동안 viewport 변화 없음 + close 후 4/5 chunk 도착 순서 추가 + 누락 0건.

**AC-CLITUI-007** — Statusbar token throughput 표시 (REQ-CLITUI-011): Given streaming + 100ms 마다 10-token chunk, when 1초 후 캡처, then spinner + `↑ 50 tok` + `~100 t/s` (±10%) + `1.0s` elapsed + `Ctrl-C: abort` 모두 포함, snapshot `streaming_in_progress.golden` 일치.

**AC-CLITUI-008** — Streaming aborted snapshot (REQ-CLITUI-011, CLI-001 REQ-CLI-009 회귀): Given streaming, when Ctrl-C 1회, then confirmQuit=true + snapshot `streaming_aborted.golden` 일치 + 후속 Ctrl-C=quit, 다른 키=cancel-confirm 보존.

**AC-CLITUI-009** — Multi-line editor toggle (REQ-CLITUI-008): Given single-line + 입력 "hello", when Ctrl-N, then mode=multi + buffer 보존 + textarea focus + snapshot `editor_multiline.golden` 일치.

**AC-CLITUI-010** — Multi-line Ctrl-J/Enter (REQ-CLITUI-008): Given multi-line + "line1", when Ctrl-J + "line2" + Enter, then ChatStream 송신 content="line1\nline2" + input cleared + mode=multi 유지.

**AC-CLITUI-011** — Markdown code rendering (REQ-CLITUI-013): Given assistant content with ` ```go ... ``` ` 코드블록, when viewport 갱신, then raw ` ``` ` 마커 부재 + glamour ascii style (indent/border) 적용 + inline `code` 도 동일.

**AC-CLITUI-012** — `/save <name>` writes jsonl (REQ-CLITUI-010): Given 1 user + 1 assistant + tmpdir HOME, when `/save test01` + Enter, then `~/.goose/sessions/test01.jsonl` atomic 생성 + 2 줄 jsonl + system msg `[saved: test01]`.

**AC-CLITUI-013** — `/load <name>` restores (REQ-CLITUI-010): Given test01.jsonl 2 메시지 + 현재 0, when `/load test01` + Enter, then messages 길이 2 복원 + viewport 표시 + `[loaded: test01, 2 messages]` + 다음 ChatStream `WithInitialMessages` 포함.

**AC-CLITUI-014** — Ctrl-R recent menu (REQ-CLITUI-007): Given 3 jsonl 다른 mtime, when Ctrl-R, then overlay 열림 + mtime desc 정렬 + 첫 entry highlight + snapshot `session_menu_open.golden` 일치 + Esc 닫힘 (no side effect) + Down + Enter = `/load <name>` 동등.

**AC-CLITUI-015** — Ctrl-Up edit last (REQ-CLITUI-009): Given messages = [user "hello", assistant "hi"], input 비어있음, when Ctrl-Up + 수정 "hello world" + Enter, then 기존 user/assistant 제거 + 새 user "hello world" + ChatStream 호출 + editingMessageIndex=-1 reset.

**AC-CLITUI-016** — Cost estimate (REQ-CLITUI-014): Given streaming + pricing config + usage{1000, 500}, when stream 종료, then statusbar `~$0.0105` 표시 (1000×3.0/1e6 + 500×15.0/1e6); 별도 sub-test: pricing 없으면 cost 부분 미표시 (no error).

**AC-CLITUI-017** — Slash help local snapshot 회귀 (CLI-001 AC-CLI-008 보강): Given TUI, when `/help` + Enter, then snapshot `slash_help_local.golden` 일치 + 네트워크 호출 0회 (mock ChatStream invocation count 0) + viewport 에 7+ command 목록 표시.

**AC-CLITUI-018** — In-TUI text language conformance (REQ-CLITUI-005): Given `language.conversation_language=ko`, when 4 표면 (statusbar idle / `/help` 응답 / permission modal / session menu) 캡처, then 각 표면에 ko substring (`세션:`, `대화 명령어`, `이 도구 호출을 허용하시겠습니까?`, `최근 세션`) 모두 포함; sub-test `=en` 시 영어 equivalent (`Session:`, `Conversation commands`, `Allow this tool call?`, `Recent sessions`) 렌더; snapshot 은 `*_ko.golden`/`*_en.golden` 8개로 분리.

---

## Files (plan.md §1 verbatim — 변경/신규 파일 일람)

### Phase 1 (Area 1 — teatest harness)

- [NEW] `internal/cli/tui/snapshots/helper.go` (~80 LoC)
- [NEW] `internal/cli/tui/snapshots/helper_test.go` (~60 LoC)
- [NEW] `internal/cli/tui/testdata/snapshots/chat_repl_initial_render.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/slash_help_local.golden`
- [NEW] `internal/cli/tui/snapshot_initial_render_test.go` (~120 LoC)
- [NEW] `internal/cli/tui/snapshot_slash_help_test.go` (~80 LoC)
- [MODIFY] `go.mod` (teatest dependency 추가)

### Phase 2 (Area 3 — streaming UX + editor)

- [NEW] `internal/cli/tui/editor/model.go` (~120 LoC)
- [NEW] `internal/cli/tui/editor/update.go` (~100 LoC)
- [NEW] `internal/cli/tui/editor/editor_test.go` (~180 LoC)
- [MODIFY] `internal/cli/tui/model.go` (streamingState struct 확장, editorMode 필드)
- [MODIFY] `internal/cli/tui/view.go` (renderStatusBar 확장, glamour 도입)
- [MODIFY] `internal/cli/tui/update.go` (KeyCtrlN/CtrlJ 디스패치, StreamProgressMsg tick)
- [NEW] `internal/cli/tui/testdata/snapshots/streaming_in_progress.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/streaming_aborted.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/editor_multiline.golden`

### Phase 3 (Area 2 — permission UI)

- [NEW] `internal/cli/tui/permission/model.go` (~150 LoC)
- [NEW] `internal/cli/tui/permission/view.go` (~100 LoC)
- [NEW] `internal/cli/tui/permission/update.go` (~120 LoC)
- [NEW] `internal/cli/tui/permission/store.go` (~120 LoC, atomic + flock)
- [NEW] `internal/cli/tui/permission/*_test.go` (~300 LoC)
- [MODIFY] `proto/goose/v1/agent.proto` (ResolvePermission RPC 추가)
- [NEW] daemon 측 RPC handler skeleton (`internal/transport/grpc/agent_service.go` 또는 동등)
- [MODIFY] `internal/cli/tui/client.go` (permission_request decode + ResolvePermission RPC wrapper)
- [MODIFY] `internal/cli/tui/model.go` (permissionState 필드)
- [MODIFY] `internal/cli/tui/view.go` (modal overlay 렌더 분기)
- [MODIFY] `internal/cli/tui/update.go` (permission mode 분기, KeyEscape 의미 분기)
- [NEW] `internal/cli/tui/testdata/snapshots/permission_modal_open.golden`
- [NEW] `internal/cli/tui/testdata/snapshots/permission_modal_persisted.golden`

### Phase 4 (Area 4 — session UX)

- [NEW] `internal/cli/tui/sessionmenu/model.go` (~100 LoC)
- [NEW] `internal/cli/tui/sessionmenu/view.go` (~80 LoC)
- [NEW] `internal/cli/tui/sessionmenu/update.go` (~80 LoC)
- [NEW] `internal/cli/tui/sessionmenu/loader.go` (~80 LoC, mtime scan)
- [NEW] `internal/cli/tui/sessionmenu/*_test.go` (~200 LoC)
- [MODIFY] `internal/cli/tui/model.go` (sessionMenuState 필드, editingMessageIndex)
- [MODIFY] `internal/cli/tui/update.go` (KeyCtrlR / KeyCtrlUp 디스패치)
- [MODIFY] `internal/cli/tui/dispatch.go` 또는 `slash.go` (`/save`, `/load` provider 또는 TUI-local)
- [NEW] `internal/cli/tui/testdata/snapshots/session_menu_open.golden`

---

## Exclusions (What NOT to Build) — verbatim from spec.md

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

**End of spec-compact.md**
