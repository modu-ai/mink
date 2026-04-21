---
id: SPEC-GOOSE-CLI-001
version: 0.2.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 3
size: 중(M)
lifecycle: spec-anchored
---

# SPEC-GOOSE-CLI-001 — goose CLI (cobra + Connect-gRPC + bubbletea TUI)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | v1.0 초안 (Phase 0, cobra 단일 구조, Ink v6 연기). **DEPRECATED** — DEPRECATED.md 참조 | manager-spec |
| 0.2.0 | 2026-04-21 | v2.0 재작성. Phase 0 → Phase 3. Connect-gRPC + bubbletea TUI + Slash command 통합. ROADMAP v2.0 §4 Phase 3 row 18, DEPRECATED.md 재작성 지시 반영 | manager-spec |

---

## 1. 개요 (Overview)

GOOSE-AGENT의 **사용자 대면 CLI**를 정의한다. v0.1.0은 Phase 0에서 최소 cobra subcommand만 제공했으나, ROADMAP v2.0(2026-04-21)이 본 SPEC을 Phase 3으로 재배치하며 아래 3가지 축으로 확장한다:

1. **Transport**: gRPC-go `grpc.DialContext` → **Connect-gRPC (`connectrpc/connect-go`)** 교체. HTTP/2 plaintext 또는 HTTP/1.1 기반 유연 호출, gRPC-Web 기본 지원, deprecated `WithBlock` 회피.
2. **Interactive Mode (TUI)**: `bubbletea` + `lipgloss` 기반 REPL 모드. 대화형 session, streaming 응답 렌더, keybindings (Ctrl-C 취소, Ctrl-D 종료), 상단 상태바(모델/세션/tokens), 하단 입력 편집.
3. **Slash Command 통합**: SPEC-GOOSE-COMMAND-001의 Dispatcher를 클라이언트 측에서 **프리-디스패치**하여 `/clear`, `/exit` 같은 local command는 네트워크 왕복 없이 처리. prompt expansion은 daemon 전달.

본 SPEC 수락 시점에서:

- `goose` (사용자 CLI) + `goosed` (데몬) 2개 실행파일.
- `goose` (인자 없음 또는 `goose chat`) → TUI REPL 진입.
- `goose ask "msg"` / `goose ask --stdin < file` → non-interactive, stdout에 응답 스트리밍.
- `goose session list/load/save/rm` → 세션 파일 관리 (`~/.goose/sessions/<name>.jsonl`).
- `goose config get/set/list` → 설정 조작 (CONFIG-001 RPC 위임).
- `goose tool list` → `Registry.ListNames()` 출력 (TOOLS-001 소비).
- `goose plugin list/install/remove` → PLUGIN-001 미구현 시 stub "not yet available".
- Keybindings: `Ctrl-C`=abort current turn, `Ctrl-D`=EOF/exit, `Ctrl-L`=clear screen, `Ctrl-R`=resume last session.

v0.1.0에서 유지되는 행동(version/ping/ask unary, exit code 0/1/2/69/78, `--format json`)은 그대로 보존.

---

## 2. 배경 (Background)

### 2.1 v0.1.0 → v0.2.0 재작성 이유 (DEPRECATED.md 정렬)

DEPRECATED.md 발췌:

> v1.0 SPEC-GOOSE-CLI-001은 cobra 단일 구조였으나, v2.0에서:
> - Connect-gRPC 클라이언트 통합 (TRANSPORT-001 소비자)
> - Slash Command System 연계 (COMMAND-001)
> - TUI 고도화 (Ink-like 패턴)
> - Phase 0 → Phase 3 재배치

**3가지 동인**:

1. **Phase 3 의존성 정렬**: MVP Milestone 1이 `TOOLS-001 → CLI-001`를 요구. CLI-001이 Phase 0에 있으면 `goose tool list` 같은 핵심 UX 명령이 누락.
2. **TRANSPORT-001 소비자 명확화**: TRANSPORT-001이 `grpc-go v1.66+` 서버를 제공. 클라이언트는 별개 선택지. Connect-gRPC는 동일 proto를 재사용하며 HTTP/1.1/gRPC-Web도 지원 → 향후 web client 확장 용이.
3. **TUI 필요성**: Hermes, Aider, Claude Code 모두 TUI 제공. CLI 전용 unary `ask`만으로는 경쟁력 부족. bubbletea는 Go 생태계에서 Ink 동급 (charmbracelet/crush, gh CLI extension 등 검증).

### 2.2 상속 자산

- **v0.1.0 SPEC-GOOSE-CLI-001 `spec.md` / `research.md`** (DEPRECATED, 본 repo 유지). exit code 표, cobra 명령 구조, `goosed/main.go` 골격은 **원문 계승**. Transport 부분만 Connect-gRPC로 교체.
- **Claude Code TypeScript** (`entrypoints/cli.tsx`): Ink v6 기반. **개념만 참고** (REPL 렌더 구조, keybinding 세트).
- **charmbracelet/crush** (`.claude/rules/moai/core/lsp-client.md` §1): powernap 결정 근거에 언급된 23k star 레포. bubbletea/lipgloss의 실전 사용 예로 참고.

### 2.3 범위 경계 (한 줄)

- **IN**: `cmd/goose/`, `cmd/goosed/`, `internal/cli/` (tui/commands/session/config), Connect-gRPC 클라이언트, bubbletea TUI, slash command 프리디스패치, session 파일, keybindings.
- **OUT**: Auth(`goose login`), plugin 실제 설치(PLUGIN-001), web/mobile/desktop 클라이언트, Windows 정식 지원, 자동 update checker.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. **2개 실행파일**:
   - `cmd/goosed/main.go`: 데몬 진입 (CORE-001 bootstrap + TRANSPORT-001 gRPC wire-up + Registry initialization). v0.1.0과 거의 동일, ~40 LoC.
   - `cmd/goose/main.go`: 사용자 CLI. cobra root + subcommand 트리, ~15 LoC (로직은 `internal/cli/`).

2. **`internal/cli/` 구조**:
   - `rootcmd.go`: cobra root + 전역 flag (`--config`, `--daemon-addr`, `--format`, `--log-level`, `--no-color`).
   - `commands/`: 각 subcommand 파일 (ask, chat, session, config, tool, plugin, version, ping, daemon).
   - `tui/`: bubbletea TUI (model.go, view.go, update.go, statusbar.go, input.go, messages.go).
   - `transport/`: Connect-gRPC client 래퍼 + retries + stream decoder.
   - `session/`: session 파일 I/O (jsonl), metadata, resume.
   - `config/`: CONFIG-001 RPC wrapper (get/set/list).
   - `output/`: plain/json formatter (v0.1.0과 동형).
   - `errors/`: exit code 매핑.
   - `keybindings/`: TUI keymap.

3. **Subcommand 트리**:
   ```
   goose
   ├── (no subcommand)           # → 'chat' 진입
   ├── chat                      # TUI REPL (interactive)
   ├── ask <message>             # unary streaming (non-interactive)
   │   └── --stdin               # read message from stdin
   ├── ping
   ├── version
   ├── session
   │   ├── list
   │   ├── load <name>           # resume → TUI
   │   ├── save <name>           # save current (only from TUI)
   │   └── rm <name>
   ├── config
   │   ├── get <key>
   │   ├── set <key> <value>
   │   └── list
   ├── tool
   │   └── list                  # Registry.ListNames
   ├── plugin
   │   ├── list                  # PLUGIN-001 미구현 시 "no plugins available"
   │   ├── install <source>      # stub: "not yet available"
   │   └── remove <name>         # stub
   └── daemon
       ├── status
       └── shutdown              # GOOSE_SHUTDOWN_TOKEN 필요
   ```

4. **Connect-gRPC 전환**:
   - `connectrpc/connect-go` v1.16+ 채택. proto는 TRANSPORT-001 그대로 소비 (`buf generate` 시 `connectrpc/connect-go` 추가 target).
   - 서버는 TRANSPORT-001 (`grpc-go`) 유지, 클라이언트만 Connect-Go.
   - **Compatibility**: Connect-Go 클라이언트는 grpc-go 서버와 호환 (HTTP/2). 추가 server-side 변경 없음 (TRANSPORT-001 호환 유지).
   - Stream client: `AgentService/ChatStream` (신규, 본 SPEC 범위) — bidi server streaming, QUERY-001 `SDKMessage` 스트림을 proto Stream으로 노출.

5. **bubbletea TUI**:
   - Model 구조: `{session, messages, input, statusbar, streaming_msg_index, keymap, viewport}`.
   - View: lipgloss 기반 3-panel layout (상단 상태바 / 중앙 messages viewport / 하단 input).
   - Update: key handling → slash command prefilter → `AgentService.ChatStream` 호출 → stream 결과를 tea.Msg로 변환 → model.messages 갱신 → redraw.
   - Streaming: SDKMessage stream_event delta를 현재 assistant message에 append (partial render).
   - Keybindings: keymap struct + `--keymap` flag로 override 가능 (vim/emacs preset 준비, 실제 저장은 후속).

6. **Session 파일 관리**:
   - 경로: `~/.goose/sessions/<name>.jsonl` (JSON Lines).
   - 각 라인: `{"role": "user|assistant|tool", "content": [...], "ts": <unix_ms>}`.
   - `session save <name>`: TUI 내에서만 실행 가능한 metacommand (CLI subcommand는 에러 반환, "use /save <name> in chat").
   - `session load <name>`: jsonl을 읽어 `initialMessages`로 TUI 시작 시 전달. QUERY-001 `QueryEngineConfig.InitialMessages` 소비.
   - `session rm <name>`: 파일 삭제 confirmation (`--yes` flag로 bypass).

7. **Slash command 프리-디스패치**:
   - `chat` TUI 입력 라인이 `/`로 시작하면 `command.Dispatcher.ProcessUserInput(line)` 호출.
   - Result가 `ProcessLocal` 또는 `ProcessExit`이면 네트워크 호출 없이 처리.
   - `ProcessProceed`이면 expanded prompt를 daemon에 전송.
   - Local echo: local command 결과는 `[system]` 스타일로 messages에 삽입.

8. **Keybindings (기본)**:
   - `Ctrl-C`: abort current turn (gRPC stream cancel → daemon side queryLoop abort, QUERY-001 AC-QUERY-008).
   - `Ctrl-D`: EOF. 입력 비어 있으면 exit. 그렇지 않으면 submit.
   - `Ctrl-L`: clear screen (messages 삭제 X).
   - `Ctrl-R`: resume last session (`~/.goose/sessions/.last.jsonl`).
   - `Esc`: cancel slash command suggestion popover (미구현 slot).
   - `Up/Down` (입력 비어 있을 때): 이전 user message 순환.

9. **Exit code 계약 (v0.1.0 계승)**:
   - `0` 정상, `1` 일반 에러, `2` usage error, `69` (EX_UNAVAILABLE) daemon 접속 불가, `78` (EX_CONFIG) config 오류.

10. **`--format json` non-interactive 지원**: TUI 모드는 json 미지원(stderr에 usage error). `ask`, `ping`, `tool list`, `session list`, `config get/list` 는 json 출력 지원.

### 3.2 OUT OF SCOPE (명시적 제외)

- **인증 명령**(`goose login`, OAuth 흐름): 후속 SPEC.
- **Plugin 실제 설치/업데이트**: 본 SPEC은 stub. 실제 구현은 PLUGIN-001.
- **Desktop/Mobile/Web 클라이언트** (goose-desktop Tauri, goose-mobile RN, goose-web Next.js): 별도 로드맵 (ROADMAP §10 OUT).
- **Windows 정식 지원**: Phase 3 OUT. darwin/linux 우선. bubbletea는 Windows도 지원하나 시그널 처리 차이로 본 SPEC은 darwin/linux 단언.
- **Shell completion 스크립트** (bash/zsh/fish): cobra 기능으로 간단히 생성 가능하나 Phase 3 OUT.
- **자동 update checker** (`goose update`): 후속.
- **Prompt history persistence across sessions**: Phase 3은 in-memory only (Ctrl-Up/Down). 영속화는 후속.
- **Multi-session simultaneous TUI**: Phase 3은 TUI당 1 세션.
- **Inline image/video 렌더**: 터미널 제약. 후속 (kitty protocol).
- **Agent switching UI** (`/agent <name>`): SUBAGENT-001 착수 후.
- **Tool call 사용자 확인 UI**: QUERY-001이 `permission_request` SDKMessage yield, CLI는 이를 TUI 모달로 표시하나 **본 SPEC은 최소 form** (한 줄 y/n). 완전한 인터랙티브 permission UI는 HOOK-001.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-CLI-001 [Ubiquitous]** — Both `goose` and `goosed` binaries **shall** expose `--version` or `version` subcommand that prints `goose version <semver> (commit <short>, built <iso>)` to stdout and exits 0. (v0.1.0 REQ-CLI-001 계승).

**REQ-CLI-002 [Ubiquitous]** — All error messages written to stderr **shall** be prefixed with `goose:` (for `goose`) or `goosed:` (for `goosed`). (v0.1.0 계승).

**REQ-CLI-003 [Ubiquitous]** — Structured output (`--format json`) **shall** emit a single JSON object per invocation with fields `{ok: bool, data?: any, error?: {code, message}}` for non-streaming subcommands; streaming subcommands (`ask`, `chat`) **shall** emit one JSON object per SDKMessage when `--format json` is active in non-interactive mode (line-delimited / JSONL).

**REQ-CLI-004 [Ubiquitous]** — The `goose` client **shall** use Connect-gRPC (`connectrpc/connect-go`) as its transport library; the daemon's grpc-go server **shall** remain the server-side implementation (cross-library compatibility over HTTP/2).

**REQ-CLI-005 [Ubiquitous]** — Exit codes **shall** be: `0` success, `1` generic error, `2` usage error (cobra), `69` (EX_UNAVAILABLE) daemon unreachable, `78` (EX_CONFIG) config error. (v0.1.0 §3.1 계승).

### 4.2 Event-Driven (이벤트 기반)

**REQ-CLI-006 [Event-Driven]** — **When** `goose` is invoked with no subcommand (or `goose chat`), the CLI **shall** enter bubbletea TUI REPL, subscribing to `AgentService/ChatStream` and rendering streamed `SDKMessage` deltas progressively in the messages viewport.

**REQ-CLI-007 [Event-Driven]** — **When** `goose ask "<message>"` is invoked, the CLI **shall** open a `ChatStream`, send the message as the first stream request, and write the concatenated assistant text to stdout followed by a single newline; the stream **shall** be closed within 30 seconds default (override via `--timeout`).

**REQ-CLI-008 [Event-Driven]** — **When** the daemon connection cannot be established within 3 seconds, the CLI **shall** print `goose: daemon unreachable at <addr>` to stderr and exit with code 69. (v0.1.0 REQ-CLI-006 계승).

**REQ-CLI-009 [Event-Driven]** — **When** the user presses `Ctrl-C` during an active TUI turn, the TUI **shall** cancel the ongoing `ChatStream` via `context.CancelFunc`, causing the daemon-side `queryLoop` to abort (QUERY-001 REQ-QUERY-010); the TUI **shall** then print `[aborted]` in the messages viewport and return to the input prompt within 500ms.

**REQ-CLI-010 [Event-Driven]** — **When** the user types a line beginning with `/` in the TUI, the client **shall** invoke `command.Dispatcher.ProcessUserInput(line, sctx)` **before** sending to the daemon; if the result is `ProcessLocal` or `ProcessExit`, no network call is made; if `ProcessProceed`, the expanded prompt is sent to the daemon.

**REQ-CLI-011 [Event-Driven]** — **When** `goose session load <name>` is invoked, the CLI **shall** read `~/.goose/sessions/<name>.jsonl`, parse each line as an SDKMessage/Message, pass them as `ChatStream` initial messages, and enter TUI mode with viewport populated.

**REQ-CLI-012 [Event-Driven]** — **When** the user invokes `/save <name>` inside TUI, the client **shall** write the current messages array to `~/.goose/sessions/<name>.jsonl` (atomic via tmp+rename), display `[saved: <name>]`, and continue.

### 4.3 State-Driven (상태 기반)

**REQ-CLI-013 [State-Driven]** — **While** the daemon reports state `draining` via the Ping response, `goose ask` and `goose chat` **shall** return exit code 69 with message `daemon is draining`. (v0.1.0 REQ-CLI-008 계승).

**REQ-CLI-014 [State-Driven]** — **While** `GOOSE_NO_COLOR=1` is set or stdout is not a TTY, the CLI **shall** disable ANSI color codes in all output (including TUI — fallback to minimal rendering). (v0.1.0 REQ-CLI-009 계승).

**REQ-CLI-015 [State-Driven]** — **While** the TUI is rendering a streaming assistant message (partial delta accumulating), `Ctrl-C` **shall** cancel the stream but preserve the partial message in the viewport marked `[partial]`.

### 4.4 Unwanted Behavior (방지)

**REQ-CLI-016 [Unwanted]** — **If** the user supplies no arguments to `goose ask` and no `--stdin` flag, **then** cobra **shall** print usage and exit with code 2. (v0.1.0 REQ-CLI-010 계승).

**REQ-CLI-017 [Unwanted]** — **If** config file parsing fails (CONFIG-001 returns `ErrSyntax`), `goosed` **shall** exit with code 78. (v0.1.0 REQ-CLI-011 계승).

**REQ-CLI-018 [Unwanted]** — The TUI **shall not** accept input lines exceeding 16 KiB by default; longer input **shall** cause a truncation warning `[input truncated to 16 KiB]` and submission of the truncated value (override via config `cli.max_input_bytes`).

**REQ-CLI-019 [Unwanted]** — The CLI **shall not** print daemon stack traces to stderr under normal operation; Go runtime errors from the daemon's `error` SDKMessage **shall** be formatted as `goose: daemon error: <message>` without stack content.

**REQ-CLI-020 [Unwanted]** — The TUI **shall not** save session files outside `~/.goose/sessions/`; `/save ../foo` attempts **shall** return error `[invalid session name]` without writing.

**REQ-CLI-021 [Unwanted]** — The CLI **shall not** auto-dial the daemon for local slash commands (`/help`, `/clear`, `/exit`, `/version` processed by local Dispatcher); network calls are restricted to prompt-expansion results.

### 4.5 Optional (선택적)

**REQ-CLI-022 [Optional]** — **Where** `--daemon-addr` is omitted, the CLI **shall** use the configured `transport.grpc_port` from CONFIG-001 with `127.0.0.1` host. (v0.1.0 REQ-CLI-014 계승).

**REQ-CLI-023 [Optional]** — **Where** `GOOSE_SHUTDOWN_TOKEN` is set and the user invokes `goose daemon shutdown`, the CLI **shall** call `DaemonService/Shutdown` with that token as `auth_token` metadata. (v0.1.0 REQ-CLI-015 계승).

**REQ-CLI-024 [Optional]** — **Where** `cli.keymap` in CONFIG-001 is set to `"emacs"` or `"vim"`, the TUI **shall** apply the corresponding preset keybindings; other values default to `"emacs"`.

**REQ-CLI-025 [Optional]** — **Where** `cli.tui.theme` in CONFIG-001 specifies a lipgloss theme (`"default"`, `"dark"`, `"light"`, `"nord"`), the TUI **shall** apply it.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-CLI-001 — `goose version` 동작 (v0.1.0 AC-CLI-001 재수행)**
- **Given** 빌드 시 `-ldflags "-X main.version=0.2.0 -X main.commit=abc1234"`
- **When** `goose version`
- **Then** stdout에 `goose version 0.2.0 (commit abc1234, built <iso>)`, exit 0

**AC-CLI-002 — `goosed` 부트스트랩 + Connect-gRPC listen**
- **Given** `t.TempDir()`으로 `GOOSE_HOME` 설정
- **When** `goosed` 실행 후 500ms 대기
- **Then** HTTP `/healthz` 200 응답 + gRPC `DaemonService/Ping` (Connect-Go client로 호출) state="serving"

**AC-CLI-003 — `goose ping` (Connect-Go client)**
- **Given** `goosed` serving
- **When** `goose ping`
- **Then** stdout에 `pong (version=..., state=serving, uptime=...)`, exit 0. Connect-Go client가 grpc-go 서버와 정상 통신 입증

**AC-CLI-004 — Daemon unreachable 시 exit 69**
- **Given** `goosed` 미실행
- **When** `goose ask "hi"`
- **Then** stderr에 `goose: daemon unreachable at 127.0.0.1:17891` + exit 69

**AC-CLI-005 — `goose ask` streaming E2E**
- **Given** `goosed` serving + stub LLM이 `"Hi!"` 응답
- **When** `goose ask "Hello"`
- **Then** stdout에 `Hi!\n`, exit 0. 테스트는 `AgentService/ChatStream` 스트리밍 경로 사용 확인

**AC-CLI-006 — `goose ask --stdin`**
- **Given** `goosed` serving, stdin pipe에 `"Hello"`
- **When** `echo "Hello" | goose ask --stdin`
- **Then** stdout에 응답 스트리밍 표시, exit 0

**AC-CLI-007 — TUI REPL 기본 진입**
- **Given** TTY 환경 (gomega-expect 기반 pty test), `goosed` serving
- **When** `goose` (subcommand 없음) 또는 `goose chat`
- **Then** bubbletea TUI 모델이 초기화되고, 상태바에 `model=..., session=new, turns=0` 표시, 입력 프롬프트 렌더됨

**AC-CLI-008 — TUI에서 `/help` 로컬 처리**
- **Given** TUI session 활성
- **When** 사용자가 `/help` 입력 + Enter
- **Then** 메시지 viewport에 내장 command 목록(최소 7종)이 `[system]` 스타일로 표시됨. **gRPC 호출 없음**(테스트에서 stream open 카운트 0 확인)

**AC-CLI-009 — TUI `Ctrl-C` 취소**
- **Given** TUI에서 `ask "long answer"` 실행 중, stub LLM이 chunk 간 200ms 대기
- **When** 사용자가 Ctrl-C 입력
- **Then** stream cancel + viewport에 `[aborted]` 표시 + 입력 프롬프트 재활성화. 500ms 이내 완료 (REQ-CLI-009)

**AC-CLI-010 — `goose session save/load` 왕복**
- **Given** TUI session에 user + assistant 1쌍 메시지 존재
- **When** `/save test01` 실행 후 TUI 종료. 이후 `goose session load test01`
- **Then** 새 TUI 시작 시 viewport에 이전 2개 메시지 복원, `~/.goose/sessions/test01.jsonl` 파일 2줄

**AC-CLI-011 — `goose tool list`**
- **Given** `goosed` serving, TOOLS-001 Registry에 내장 6종 + MCP 1종(mock) 등록
- **When** `goose tool list`
- **Then** stdout에 정렬된 리스트 (`Bash, FileEdit, FileRead, FileWrite, Glob, Grep, mcp__foo__bar`), exit 0

**AC-CLI-012 — `goose config get/set/list`**
- **Given** `goosed` serving, CONFIG-001 RPC 지원
- **When** `goose config set log.level debug` → `goose config get log.level`
- **Then** 두 번째 명령 stdout이 `debug`, exit 0

**AC-CLI-013 — `goose plugin install` stub**
- **Given** PLUGIN-001 미구현 상태
- **When** `goose plugin install ./some-plugin`
- **Then** stderr에 `goose: plugin system not yet available (SPEC-GOOSE-PLUGIN-001 pending)`, exit 1

**AC-CLI-014 — Usage error exit 2**
- **Given** 실행 환경
- **When** `goose ask` (인자 없음, `--stdin` 없음)
- **Then** stderr에 cobra usage + exit 2

**AC-CLI-015 — `--format json` for streaming**
- **Given** `goosed` serving
- **When** `goose ask "hi" --format json`
- **Then** stdout에 JSONL: 각 SDKMessage가 한 줄씩 (`{"type":"user_ack",...}`, `{"type":"stream_event","delta":"H"}`, ..., `{"type":"terminal","success":true}`), exit 0

**AC-CLI-016 — Input truncation**
- **Given** `cli.max_input_bytes = 100`, TUI
- **When** 사용자가 200 bytes 입력 후 Enter
- **Then** stderr-equivalent TUI status line에 `[input truncated to 100 bytes]`, 서버에 100 bytes만 전송됨

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
cmd/
├── goose/main.go                # ~15 LoC
└── goosed/main.go               # ~40 LoC (v0.1.0 유지 + Registry init)

internal/cli/
├── rootcmd.go                   # cobra root + persistent flags
├── errors.go                    # exit code mapping
├── output/
│   ├── formatter.go             # plain vs JSON/JSONL
│   └── color.go                 # TTY detection + NO_COLOR
├── commands/
│   ├── ask.go                   # unary / stdin
│   ├── chat.go                  # → tui.Run
│   ├── ping.go
│   ├── version.go
│   ├── session.go               # list/load/save/rm
│   ├── config.go                # get/set/list
│   ├── tool.go                  # list
│   ├── plugin.go                # stub
│   └── daemon.go                # status/shutdown
├── tui/
│   ├── model.go                 # Model struct
│   ├── update.go                # tea.Msg dispatch
│   ├── view.go                  # lipgloss render
│   ├── statusbar.go
│   ├── input.go                 # viewport + editor
│   ├── messages.go              # message list render
│   ├── stream.go                # ChatStream goroutine → tea.Msg
│   ├── keybindings.go           # keymap (emacs|vim)
│   ├── theme.go                 # lipgloss styles
│   └── tui_test.go              # teatest harness
├── transport/
│   ├── client.go                # Connect-Go client factory
│   ├── dial.go                  # dial with timeout + health check
│   └── stream.go                # AgentService.ChatStream wrapper
├── session/
│   ├── file.go                  # jsonl read/write atomic
│   ├── path.go                  # name validation + path resolution
│   └── file_test.go
└── config/
    └── rpc.go                   # CONFIG-001 RPC wrapper

proto/goose/v1/
├── daemon.proto                 # TRANSPORT-001 유지
├── agent.proto                  # v0.1.0 추가: AgentService.Chat (unary) + ChatStream (server streaming)
├── tool.proto                   # NEW: ToolService.List
└── config.proto                 # NEW: ConfigService.Get/Set/List
```

### 6.2 proto 확장 (본 SPEC 범위)

```proto
// proto/goose/v1/agent.proto

service AgentService {
  rpc Chat(ChatRequest) returns (ChatResponse);            // unary (v0.1.0 계승)
  rpc ChatStream(ChatStreamRequest) returns (stream ChatStreamEvent);  // NEW
}

message ChatStreamRequest {
  string agent = 1;        // optional, default "default"
  string message = 2;
  repeated Message initial_messages = 3;  // for session resume
  string session_id = 4;   // optional
}

message ChatStreamEvent {
  string type = 1;         // "user_ack"|"stream_event"|"message"|"tool_use_summary"|...
  bytes payload_json = 2;  // type별 payload (SDKMessage 직렬화)
}

message Message {
  string role = 1;         // "user"|"assistant"|"tool"
  repeated ContentBlock content = 2;
  int64 ts_ms = 3;
}

message ContentBlock {
  string kind = 1;         // "text"|"tool_use"|"tool_result"|"image"
  bytes data_json = 2;
}
```

```proto
// proto/goose/v1/tool.proto

service ToolService {
  rpc List(ListRequest) returns (ListResponse);
}

message ListRequest {}
message ListResponse {
  repeated ToolDescriptor tools = 1;
}
message ToolDescriptor {
  string name = 1;
  string description = 2;
  string source = 3;       // "builtin"|"mcp"|"plugin"
  string server_id = 4;    // MCP일 때
}
```

```proto
// proto/goose/v1/config.proto

service ConfigService {
  rpc Get(ConfigKey) returns (ConfigValue);
  rpc Set(ConfigEntry) returns (SetResponse);
  rpc List(ListConfigRequest) returns (ListConfigResponse);
}
message ConfigKey { string key = 1; }
message ConfigValue { string key = 1; string value = 2; bool exists = 3; }
message ConfigEntry { string key = 1; string value = 2; }
message SetResponse { bool ok = 1; string message = 2; }
message ListConfigRequest { string prefix = 1; }
message ListConfigResponse { repeated ConfigEntry entries = 1; }
```

### 6.3 Connect-gRPC 클라이언트 전환

```go
// internal/cli/transport/client.go

import (
    "connectrpc.com/connect"
    goosev1 "github.com/gooseagent/goose/internal/transport/grpc/gen/goosev1"
    "github.com/gooseagent/goose/internal/transport/grpc/gen/goosev1/goosev1connect"
)

type Client struct {
    daemon  goosev1connect.DaemonServiceClient
    agent   goosev1connect.AgentServiceClient
    tool    goosev1connect.ToolServiceClient
    config  goosev1connect.ConfigServiceClient
    baseURL string
}

// Dial은 Connect-Go 클라이언트 팩토리.
// HTTP/2 over h2c (plaintext HTTP/2, 루프백 전제).
func Dial(ctx context.Context, addr string, timeout time.Duration) (*Client, error) {
    baseURL := "http://" + addr
    httpClient := &http.Client{
        Transport: &http2.Transport{
            AllowHTTP: true,
            DialTLS: func(network, addr string, _ *tls.Config) (net.Conn, error) {
                return net.DialTimeout(network, addr, timeout)
            },
        },
        Timeout: 0, // streaming: no overall timeout
    }
    c := &Client{
        baseURL: baseURL,
        daemon:  goosev1connect.NewDaemonServiceClient(httpClient, baseURL, connect.WithGRPC()),
        agent:   goosev1connect.NewAgentServiceClient(httpClient, baseURL, connect.WithGRPC()),
        tool:    goosev1connect.NewToolServiceClient(httpClient, baseURL, connect.WithGRPC()),
        config:  goosev1connect.NewConfigServiceClient(httpClient, baseURL, connect.WithGRPC()),
    }
    // Liveness check
    pingCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    _, err := c.daemon.Ping(pingCtx, connect.NewRequest(&goosev1.PingRequest{}))
    if err != nil { return nil, err }
    return c, nil
}
```

**기존 grpc-go client와의 차이**:
- `WithBlock` 불필요 → deprecation 회피.
- Liveness 검증을 Ping RPC로 명시적 수행 (REQ-CLI-008의 3s timeout 충족).
- gRPC-Web 지원: 추가 flag 없이도 `connect.WithProtoJSON()` 추가 시 HTTP/1.1 JSON 모드로 전환 가능 (미래 web client).

### 6.4 bubbletea TUI 핵심 구조

```go
// internal/cli/tui/model.go

type Model struct {
    client         *transport.Client
    dispatcher     *command.Dispatcher
    session        *session.State       // in-memory messages
    input          textarea.Model       // bubbles/textarea
    viewport       viewport.Model       // bubbles/viewport
    statusbar      statusBar
    streaming      streamingState
    keymap         keyMap
    theme          theme
    width, height  int
}

type streamingState struct {
    active      bool
    msgIndex    int          // session.Messages[idx] being streamed
    cancelFn    context.CancelFunc
    partial     bool
}

// Init, Update, View는 tea.Model interface 구현.
func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKey(msg)
    case streamChunkMsg:
        return m.appendStreamChunk(msg)
    case streamEndMsg:
        return m.finalizeStream(msg)
    case errMsg:
        return m.handleErr(msg)
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        return m, nil
    }
    return m, nil
}

func (m Model) View() string {
    return lipgloss.JoinVertical(lipgloss.Left,
        m.statusbar.Render(m.width),
        m.viewport.View(),
        m.input.View(),
    )
}
```

### 6.5 Slash command 프리-디스패치

```go
// internal/cli/tui/update.go 일부

func (m Model) handleSubmit() (Model, tea.Cmd) {
    line := m.input.Value()
    if line == "" { return m, nil }
    
    // 프리-디스패치
    processed, err := m.dispatcher.ProcessUserInput(context.TODO(), line, m.sctx())
    if err != nil { return m.handleErr(errMsg{err}) }
    
    switch processed.Kind {
    case command.ProcessLocal:
        // Local 응답을 messages에 삽입 (네트워크 호출 없음)
        m.session.AppendSystem(processed.Messages)
        m.input.Reset()
        return m, nil
    case command.ProcessExit:
        return m, tea.Quit
    case command.ProcessProceed:
        // daemon 전달
        return m.startStream(processed.Prompt)
    case command.ProcessAbort:
        m.input.Reset()
        return m, nil
    }
    return m, nil
}

// sctx는 TUI가 Dispatcher에 주입하는 SlashCommandContext 구현.
func (m Model) sctx() command.SlashCommandContext {
    return &tuiSlashContext{
        onClear:          m.clearSession,
        onModelChange:    m.setModel,
        onCompactRequest: m.requestCompact,
        resolveModel:     m.client.ResolveModelAlias,
        sessionSnapshot:  m.buildSnapshot,
    }
}
```

### 6.6 Session 파일 (jsonl, atomic)

```go
// internal/cli/session/file.go

type Entry struct {
    Role    string        `json:"role"`
    Content []ContentBlock `json:"content"`
    TsMs    int64         `json:"ts_ms"`
}

func Save(path string, entries []Entry) error {
    // atomic write: tmp + rename
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o755); err != nil { return err }
    tmp, err := os.CreateTemp(dir, ".goose-session-*.tmp")
    if err != nil { return err }
    defer os.Remove(tmp.Name())
    
    enc := json.NewEncoder(tmp)
    for _, e := range entries {
        if err := enc.Encode(e); err != nil { return err }
    }
    if err := tmp.Close(); err != nil { return err }
    return os.Rename(tmp.Name(), path)
}

func Load(path string) ([]Entry, error) {
    f, err := os.Open(path)
    if err != nil { return nil, err }
    defer f.Close()
    var entries []Entry
    dec := json.NewDecoder(f)
    for dec.More() {
        var e Entry
        if err := dec.Decode(&e); err != nil { return nil, err }
        entries = append(entries, e)
    }
    return entries, nil
}

// ValidateName은 REQ-CLI-020 경로 탈출 방지.
func ValidateName(name string) error {
    if name == "" { return errors.New("empty name") }
    if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
        return errors.New("invalid session name")
    }
    return nil
}
```

### 6.7 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1**: `TestExitCode_Mapping` — 모든 exit code 매핑 (REQ-CLI-005).
2. **RED #2**: `TestTransport_Dial_UnreachableFails69` — AC-CLI-004.
3. **RED #3**: `TestCommand_Version_PrintsLdflags` — AC-CLI-001.
4. **RED #4**: `TestCommand_Ping_Connect` — AC-CLI-003 (Connect-Go client ↔ grpc-go server 호환성 입증).
5. **RED #5**: `TestCommand_Ask_UnaryStream` — AC-CLI-005.
6. **RED #6**: `TestCommand_Ask_Stdin` — AC-CLI-006.
7. **RED #7**: `TestCommand_Ask_NoArg_Usage2` — AC-CLI-014.
8. **RED #8**: `TestCommand_Ask_JsonFormat_JSONL` — AC-CLI-015.
9. **RED #9**: `TestTUI_HelpLocal_NoNetwork` — AC-CLI-008 (teatest harness).
10. **RED #10**: `TestTUI_CtrlC_CancelStream` — AC-CLI-009.
11. **RED #11**: `TestSession_SaveLoad_Roundtrip` — AC-CLI-010.
12. **RED #12**: `TestSession_ValidateName_RejectsTraversal` — REQ-CLI-020.
13. **RED #13**: `TestCommand_ToolList` — AC-CLI-011.
14. **RED #14**: `TestCommand_ConfigGetSet` — AC-CLI-012.
15. **RED #15**: `TestCommand_PluginInstall_Stub` — AC-CLI-013.
16. **RED #16**: `TestTUI_InputTruncation` — AC-CLI-016.
17. **GREEN**: 최소 구현 — rootcmd, transport/client, commands/{ask,ping,version}, tui/{model,update,view} 최소.
18. **REFACTOR**: keymap을 viper 구조로 추출, TUI streaming goroutine을 errgroup 기반 cleanup.

### 6.8 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **Tested** | 85%+ 커버리지 (main.go 제외), `teatest` 기반 TUI 스크립트 테스트, integration: `os/exec.Cmd` subprocess 조합 |
| **Readable** | cmd/main.go는 ~40/15 LoC, commands/ 파일당 <100 LoC, tui/ 책임별 파일 분리 |
| **Unified** | `buf lint`로 proto 일관성, `golangci-lint` (errcheck, govet, staticcheck), lipgloss theme struct 공용화 |
| **Secured** | Session 경로 검증(REQ-CLI-020), Shutdown 토큰(REQ-CLI-023), Input 크기 제한(REQ-CLI-018), insecure transport는 loopback 전제 |
| **Trackable** | JSONL 구조화 스트림(REQ-CLI-003), exit code 명시(REQ-CLI-005), session 파일은 git-friendly(line-delimited) |

### 6.9 의존성 결정 (라이브러리)

| 라이브러리 | 버전 | 용도 | 근거 |
|----------|------|-----|-----|
| `github.com/spf13/cobra` | v1.8+ | CLI 프레임워크 | v0.1.0 계승, tech.md §3.1 명시 |
| `connectrpc.com/connect-go` | v1.16+ | Connect-gRPC 클라이언트 | HTTP/2 유연성, WithBlock deprecation 회피 |
| `golang.org/x/net/http2` | 최신 | h2c (plaintext HTTP/2) | Connect-Go 내장 transport |
| `github.com/charmbracelet/bubbletea` | v1.2+ | TUI framework | Go 생태계 표준, crush/gh 사용 검증 |
| `github.com/charmbracelet/lipgloss` | v1.1+ | 스타일링 | bubbletea 공식 companion |
| `github.com/charmbracelet/bubbles` | v0.20+ | viewport/textarea/spinner | UI 위젯 |
| `golang.org/x/term` | 최신 | TTY 감지 | v0.1.0 계승 |
| `go.uber.org/zap` | v1.27+ | 로그 | CORE-001 계승 |
| `github.com/stretchr/testify` | v1.9+ | 테스트 | 기존 |
| `github.com/charmbracelet/x/exp/teatest` | 최신 | TUI script test | bubbletea 공식 테스트 하네스 |

**의도적 미사용**:
- `google.golang.org/grpc` (클라이언트) — Connect-Go로 대체.
- `github.com/urfave/cli` — cobra 중복.
- `github.com/fatih/color` — lipgloss가 커버.

### 6.10 v0.1.0에서 계승 항목 요약

| v0.1.0 요소 | v0.2.0 조치 |
|-----------|-----------|
| Exit code 0/1/2/69/78 | REQ-CLI-005로 그대로 계승 |
| `goose version` | REQ-CLI-001, AC-CLI-001 계승 |
| `goose ping` (unary) | 그대로 유지 (v0.1.0 AgentService/Ping) |
| `goose ask` (unary) | v0.1.0 unary 유지 + **`--stream` 옵션(기본 true)으로 ChatStream 사용** |
| cobra 구조 | 유지, subcommand 확장 |
| v0.1.0 `agent.proto` `AgentService/Chat` | 유지 + `ChatStream` 추가 |
| `goosed/main.go` ~40 LoC | 유지 |
| exit code 매핑 함수 | 그대로 이전 |
| `GOOSE_NO_COLOR` | 유지 (REQ-CLI-014) |
| `GOOSE_SHUTDOWN_TOKEN` | 유지 (REQ-CLI-023) |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | `goosed` bootstrap, exit code |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `config.Load`, CLI flag 매핑 |
| 선행 SPEC | SPEC-GOOSE-TRANSPORT-001 | proto 기본 스키마, grpc-go 서버 |
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | SDKMessage 스트림 소비 |
| 선행 SPEC | SPEC-GOOSE-COMMAND-001 | `Dispatcher.ProcessUserInput`, `SlashCommandContext` |
| 선행 SPEC | SPEC-GOOSE-TOOLS-001 | `goose tool list` 데이터 소스 |
| 후속 SPEC | SPEC-GOOSE-SKILLS-001 | Skill-backed command가 TUI에서 표시됨 (Provider 등록 경로) |
| 후속 SPEC | SPEC-GOOSE-MCP-001 | `goose tool list`가 MCP tool도 포함 |
| 후속 SPEC | SPEC-GOOSE-SUBAGENT-001 | `/agent <name>` 명령 (추후 추가) |
| 후속 SPEC | SPEC-GOOSE-HOOK-001 | permission 인터랙션 UI 통합 |
| 후속 SPEC | SPEC-GOOSE-PLUGIN-001 | `plugin install/remove` stub → 실제 구현 |
| 외부 | `connectrpc/connect-go` v1.16+ | 클라이언트 transport |
| 외부 | `charmbracelet/bubbletea` v1.2+ | TUI |
| 외부 | `charmbracelet/lipgloss` v1.1+ | 스타일 |
| 외부 | `spf13/cobra` v1.8+ | CLI |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Connect-Go client가 grpc-go 서버와 미묘한 호환성 문제 (metadata 인코딩 등) | 중 | 고 | `buf generate`로 양쪽 코드 모두 생성, AC-CLI-003 테스트가 Connect-Go ↔ grpc-go 호환성 명시적 검증 |
| R2 | bubbletea TUI의 terminal 제어 문자가 일부 터미널(iTerm/Alacritty 차이)에서 렌더 이슈 | 중 | 중 | 지원 터미널 명시 (iTerm2, Terminal.app, Alacritty, WezTerm, tmux). `--no-tui` fallback flag 제공 |
| R3 | `Ctrl-C`가 daemon-side queryLoop abort로 즉시 전파되지 않음 (네트워크 지연) | 중 | 중 | Connect-Go stream cancel이 HTTP/2 RST_STREAM으로 전파. daemon측 ctx.Done() 즉시 fire (QUERY-001 REQ-QUERY-010) 검증 |
| R4 | Session jsonl schema drift (v1 저장 → v2 로드) | 중 | 중 | Entry에 `version` 필드 없음 (Phase 3). 변경 시 `migrate` 명령 추가 (후속) |
| R5 | TUI 입력 중 대형 paste (100KB+) → lag | 고 | 낮 | REQ-CLI-018로 16 KiB 기본 제한. 사용자에게 truncation 표시 |
| R6 | bubbletea goroutine leak (stream goroutine이 model 종료 후에도 실행) | 중 | 중 | errgroup + context 기반 수명 관리. `teatest` 기반 goroutine 누수 테스트 |
| R7 | Windows 기본 터미널(`cmd.exe`)에서 bubbletea 렌더 깨짐 | 높 | 낮 | Phase 3은 darwin/linux 공식 지원. Windows는 WSL2 권장 문서화 |
| R8 | Plugin install stub이 UX 혼란 유발 | 낮 | 낮 | stderr에 명확한 "not yet available" 메시지 + 로드맵 참조 URL |
| R9 | Connect-Go의 `WithGRPC()` 프로토콜이 gRPC-Web과 혼용 시 혼란 | 낮 | 낮 | Phase 3은 gRPC 프로토콜만. 문서화. 향후 `--protocol grpc|grpc-web|connect` flag |
| R10 | `/save <name>`이 TUI에서만 가능한 제약이 사용자 직관에 반함 | 중 | 낮 | `goose session save <name>`은 명시적으로 "use /save in chat" 안내. 이유: stdin으로 현재 상태 주입 불가 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/claude-primitives.md` §5(Hook 시스템 — TUI permission 인터랙션 참조), §2(Skills — user-invocable skill 표시 경로)
- `.moai/project/research/claude-core.md` §1(SDKMessage 스트림), §3(QueryEngine 인터페이스)
- `.moai/project/structure.md` §53-54 `cmd/goose/`, §228 `internal/transport/`, §374 `goose-cli` Phase 5 TS 패키지 언급 (본 SPEC과 별개)
- `.moai/project/tech.md` §3.1(cobra), §3.3(proto), ADR-002(gRPC)
- `.moai/specs/ROADMAP.md` §4 Phase 3 row 18 "기존 CLI-001 재작성", §7 MVP Milestone 1, §8 (재작성 처리 방안)
- `.moai/specs/SPEC-GOOSE-CLI-001/DEPRECATED.md` (v0.1.0 재작성 지시)
- `.moai/specs/SPEC-GOOSE-TRANSPORT-001/spec.md` §6.2 proto 스키마, REQ-TR-005 Ping 응답
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` §5 SDKMessage 타입, REQ-QUERY-010 ctx cancel
- `.moai/specs/SPEC-GOOSE-COMMAND-001/spec.md` (동시 작성 — `ProcessUserInput`, `SlashCommandContext`)
- `.moai/specs/SPEC-GOOSE-TOOLS-001/spec.md` (동시 작성 — `goose tool list` 데이터 소스)

### 9.2 외부 참조

- Connect-gRPC Go: https://connectrpc.com/docs/go/
- charmbracelet/bubbletea: https://github.com/charmbracelet/bubbletea
- charmbracelet/lipgloss: https://github.com/charmbracelet/lipgloss
- charmbracelet/bubbles: https://github.com/charmbracelet/bubbles
- teatest (TUI testing): https://github.com/charmbracelet/x/tree/main/exp/teatest
- charmbracelet/crush (참고 구현): 23k stars, powernap LSP 통합 (`.claude/rules/moai/core/lsp-client.md`)

### 9.3 부속 문서

- `./research.md` — Connect-Go vs grpc-go 클라이언트 선택 근거, bubbletea vs tview 비교, 키바인딩 세트 결정 매트릭스
- `./DEPRECATED.md` — v0.1.0 재작성 이유 및 이력 (보존)
- `../ROADMAP.md` Phase 3 의존 그래프
- `../SPEC-GOOSE-COMMAND-001/spec.md` — slash command 시스템 (TUI에서 프리디스패치)
- `../SPEC-GOOSE-TOOLS-001/spec.md` — tool registry (CLI `goose tool list` 소스)

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지. v0.1.0 OUT OF SCOPE 대부분 유지 + v0.2.0 추가.

- 본 SPEC은 **인증 명령**(`goose login`, OAuth device flow)을 구현하지 않는다. 후속 SPEC.
- 본 SPEC은 **Plugin 실제 설치/업데이트를 구현하지 않는다**. stub stderr 메시지만. PLUGIN-001 의존.
- 본 SPEC은 **Desktop / Mobile / Web 클라이언트를 구현하지 않는다**. ROADMAP §10 OUT.
- 본 SPEC은 **Windows 정식 지원을 보장하지 않는다**. darwin/linux 우선. Windows 사용자에게 WSL2 안내.
- 본 SPEC은 **Shell completion 스크립트를 포함하지 않는다**. cobra 기능으로 간단히 추가 가능하나 Phase 3 OUT.
- 본 SPEC은 **자동 update checker를 포함하지 않는다** (`goose update`). 후속.
- 본 SPEC은 **Prompt history 영속화를 구현하지 않는다**. TUI in-memory Up/Down만 (Phase 3).
- 본 SPEC은 **다중 세션 동시 TUI를 지원하지 않는다**. TUI 인스턴스 당 1 세션.
- 본 SPEC은 **Inline image / video rendering을 지원하지 않는다**. 터미널 제약. 향후 kitty graphics protocol 검토.
- 본 SPEC은 **Agent switching UI를 구현하지 않는다** (`/agent <name>`). SUBAGENT-001 이후 추가.
- 본 SPEC은 **완전한 인터랙티브 permission UI를 제공하지 않는다**. `permission_request` SDKMessage는 단순 y/n 프롬프트로 처리. 고급 UI(HOOK-001).
- 본 SPEC은 **gRPC-Web 전용 클라이언트 모드를 제공하지 않는다**. Connect-Go의 gRPC 프로토콜만. 향후 `--protocol` flag로 확장.
- 본 SPEC은 **MCP server를 CLI에서 직접 기동하지 않는다**. MCP-001이 daemon 내부에서 관리. CLI는 `goose tool list`로 관찰만.
- 본 SPEC은 **Remote daemon 접속 전용 모드를 보장하지 않는다**. 기본 loopback. 원격 접속은 TLS/인증 도입 후 (후속 SPEC).

---

**End of SPEC-GOOSE-CLI-001 v0.2.0**
