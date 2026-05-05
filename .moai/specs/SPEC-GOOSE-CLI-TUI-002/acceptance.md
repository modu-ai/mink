# SPEC-GOOSE-CLI-TUI-002 — Acceptance Criteria (상세)

> **목적**: spec.md §5 의 17 AC 시나리오를 fuller Given/When/Then 형식으로 재구성. 구체 입력/출력, snapshot 명명 규약, 성능/coverage gate 명시.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — 17 AC 상세화 + snapshot 규약 + coverage gate + DoD | manager-spec |

---

## 0. 일반 규약

### 0.1 Snapshot 파일 명명 규약

- 경로: `internal/cli/tui/testdata/snapshots/<scenario_slug>.golden`
- slug 규칙: `<area>_<state>_<modifier>` (snake_case, lowercase)
- 예시:
  - `chat_repl_initial_render.golden`
  - `streaming_in_progress.golden`
  - `streaming_aborted.golden`
  - `permission_modal_open.golden`
  - `permission_modal_persisted.golden`
  - `slash_help_local.golden`
  - `session_menu_open.golden`
  - `editor_multiline.golden`
- 갱신 명령: `go test -update ./internal/cli/tui/...`
- Git diff 검토 의무: PR review 시 snapshot 변경분 시각 검증 (의도된 UI 변경인지 확인)

### 0.2 테스트 환경 표준화

모든 snapshot 테스트의 setup:

```go
func TestSnapshot_X(t *testing.T) {
    snapshots.SetupAsciiTermenv(t)            // lipgloss color profile = ascii
    clock := snapshots.FixedClock(             // 고정 시계
        time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
    )
    model := tui.NewModel(mockClient, "(unnamed)", false)
    model.Clock = clock                         // dependency injection
    
    tm := teatest.NewTestModel(t, model,
        teatest.WithInitialTermSize(80, 24),
    )
    // ... interactions
    tm.Quit()
    tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
    snapshots.RequireSnapshot(t, "scenario_name.golden", tm.FinalOutput(t))
}
```

### 0.3 성능 게이트

| 메트릭 | 임계값 | 측정 방법 |
|------|-------|---------|
| REPL initial render latency | < 100 ms | `time.Now()` before/after `tea.NewProgram(model).Start()` (관찰 가능 bound, strict 아님) |
| Throughput tick rate | ≥ 4 Hz (250 ms 간격) | tea.Tick 호출 빈도 + `StreamProgressMsg` count 측정 |
| Permission modal close → stream resume | < 200 ms | `t.Now()` modal close → 첫 buffered chunk viewport append |
| Snapshot test 1 회 실행 시간 | < 500 ms (each) | go test elapsed |

### 0.4 Coverage 게이트

| 패키지 | 최소 cover | 측정 |
|-------|----------|-----|
| `internal/cli/tui/permission/` | 85% | `go test -cover ./internal/cli/tui/permission/` |
| `internal/cli/tui/sessionmenu/` | 85% | `go test -cover ./internal/cli/tui/sessionmenu/` |
| `internal/cli/tui/editor/` | 85% | `go test -cover ./internal/cli/tui/editor/` |
| `internal/cli/tui/snapshots/` | 80% (helper code, not all paths testable) | `go test -cover ./internal/cli/tui/snapshots/` |
| `internal/cli/tui/` (수정 영향) | 75% (CLI-001 Phase D 종료 시 72.5%, 본 SPEC 후 ≥ 75%) | `go test -cover ./internal/cli/tui/` |

전체 race detection: `go test -race -count=10 ./internal/cli/tui/...` 100% PASS.

---

## 1. AC-CLITUI-001 — Snapshot harness 결정성

**Given**:
- `internal/cli/tui/snapshots/helper.go` 의 `SetupAsciiTermenv(t)` + `FixedClock(2026-05-05T12:00:00Z)` 적용된 `TestSnapshot_ChatREPL_InitialRender`
- 동일 `Model` 초기 상태: `messages=[]`, `sessionName="(unnamed)"`, `daemonAddr="127.0.0.1:17891"`, `noColor=false` (color 이지만 ascii profile 강제로 무효화)
- 환경: macOS (darwin/arm64) CI runner + Linux (linux/amd64) CI runner

**When**:
- 양쪽 OS 에서 `go test -run TestSnapshot_ChatREPL_InitialRender ./internal/cli/tui/...` 실행
- `tm.FinalOutput(t)` 바이트 캡처

**Then**:
- 양쪽 OS 의 캡처 바이트가 100% 일치 (`bytes.Equal == true`)
- ANSI escape sequence 부재 (`bytes.Contains(out, []byte{0x1b}) == false`)
- terminfo dependent bytes 부재 (cursor save/restore `\x1b[s`/`\x1b[u` 등)

**REQ**: REQ-CLITUI-001

---

## 2. AC-CLITUI-002 — chat_repl_initial_render snapshot 회귀 보호

**Given**:
- 초기 `Model` (Section 0.2 setup), WindowSize 80x24

**When**:
- `tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))`
- 즉시 Quit 없이 `tm.Output()` 캡처 (또는 `tm.Quit() + tm.FinalOutput(t)`)

**Then**:
- 캡처 바이트가 `testdata/snapshots/chat_repl_initial_render.golden` 와 byte-equal
- 표시 검증 (시각): 1 line statusbar (`Session: (unnamed) | Daemon: 127.0.0.1:17891 | Messages: 0`), empty viewport, input prompt `> ` (cursor)
- 80 column 폭, 24 row 높이 정확히 차지

**REQ**: REQ-CLITUI-001

---

## 3. AC-CLITUI-003 — Permission modal opens on permission_request

**Given**:
- TUI 활성, Section 0.2 setup
- mock client: `ChatStream` 응답 stream 에 다음 페이로드 주입
  ```json
  {"type": "permission_request", "payload_json": "{\"tool_use_id\":\"t1\",\"tool_name\":\"Bash\",\"input\":{\"command\":\"rm -rf /tmp/x\"}}"}
  ```

**When**:
- 사용자가 임의 user message 입력 → Enter
- mock 이 stream 에 위 페이로드 yield (stream open 후 100 ms 내)
- `tm.WaitFor(...)` 로 PermissionRequestMsg 처리 완료 대기

**Then**:
- `model.permissionState.active == true`
- `model.permissionState.request.ToolName == "Bash"`
- `model.permissionState.request.Input["command"] == "rm -rf /tmp/x"`
- `tm.Output()` snapshot 이 `testdata/snapshots/permission_modal_open.golden` 와 일치
- modal box 시각: 중앙 정렬, 4 옵션 라벨 ("Allow once" highlighted), tool 설명 표시

**REQ**: REQ-CLITUI-006

---

## 4. AC-CLITUI-004 — Allow always persists to disk

**Given**:
- AC-CLITUI-003 의 modal open 상태
- `t.TempDir()` 기반 HOME → `~/.goose/permissions.json` 미존재

**When**:
- 사용자가 Tab 2 회 (option index 0 → 2 = "Allow always (this tool)") → Enter

**Then**:
- `~/.goose/permissions.json` 파일 생성 (atomic write tmp+rename)
- 파일 내용 = `{"version":1,"tools":{"Bash":"allow"}}` (JSON, 2-space indent 또는 compact 둘 다 허용)
- modal 닫힘 (`permissionState.active == false`)
- `client.ResolvePermission("t1", Allow)` RPC 호출됨 (mock 클라이언트 호출 카운트 1)
- 후속 stream 이 다음 Bash tool_use 도착 시: modal 표시 안 됨, 즉시 `ResolvePermission` 호출
- snapshot `permission_modal_persisted.golden` 시각 검증 (modal 미표시, statusbar 정상)

**REQ**: REQ-CLITUI-002, REQ-CLITUI-013

---

## 5. AC-CLITUI-005 — Allow once does NOT persist

**Given**:
- mock client 가 `permission_request{tool_name:"FileWrite"}` 주입
- `~/.goose/permissions.json` 미존재

**When**:
- modal open → 사용자가 Enter (기본 option = "Allow once")

**Then**:
- `~/.goose/permissions.json` 여전히 미존재 (디스크 zero write)
- modal 닫힘
- `client.ResolvePermission("t1", Allow)` 호출됨
- in-memory `permissionState.activeTools` 에 "FileWrite" 미기록
- 후속 stream 이 새 FileWrite tool_use 도착 시 modal 다시 표시됨

**REQ**: REQ-CLITUI-013

---

## 6. AC-CLITUI-006 — Streaming pauses while modal open

**Given**:
- TUI streaming 활성 (mock 이 5 chunk 를 250 ms 간격으로 yield)
- 3 번째 chunk (index 2) 직전에 `permission_request` 주입

**When**:
- modal 열린 후 1 초 대기 (chunk 4, 5 가 도착했을 시간)
- 사용자가 Enter (Allow once)

**Then**:
- modal open 동안 `model.messages[len-1].Content` 길이 변화 없음 (chunk 4, 5 미반영)
- modal close 직후 chunk 4, 5 가 viewport 에 도착 순서대로 추가됨
- 누락 chunk 0건 (전체 5 chunk 모두 반영)
- 전체 stream 종료 후 `model.messages[last].Content` = chunk 1+2 (modal open 전) + chunk 4+5 (modal close 후) (chunk 3 는 permission_request 자체이므로 viewport 에 없음)

**REQ**: REQ-CLITUI-004, REQ-CLITUI-012

---

## 7. AC-CLITUI-007 — Statusbar token throughput 표시

**Given**:
- TUI streaming 활성
- mock 이 매 100 ms 마다 10-token chunk yield (총 5 chunk = 500 ms = 50 tokens)
- `FixedClock` 으로 elapsed 결정성 보장

**When**:
- 1 초 대기 후 `tm.Output()` 캡처

**Then**:
- statusbar 에 모두 포함:
  - spinner frame (4 frame 회전 중 1)
  - `↑ 50 tok` (output token count)
  - throughput `~100 t/s` (±10% 허용 — 50 tokens / 0.5 s)
  - elapsed `1.0s`
  - `Ctrl-C: abort` hint
- snapshot `streaming_in_progress.golden` byte-equal

**REQ**: REQ-CLITUI-011

---

## 8. AC-CLITUI-008 — Streaming aborted snapshot 회귀

**Given**:
- TUI streaming 활성

**When**:
- 사용자가 Ctrl-C 1 회 (CLI-001 confirmQuit 모드 진입)

**Then**:
- `model.confirmQuit == true`, `model.streaming == true` (stream 자체는 아직 continue, 사용자 확인 대기)
- snapshot `streaming_aborted.golden` byte-equal (statusbar `[Press Ctrl-C again to quit]` 또는 동등 문구)
- 후속 Ctrl-C → quit (CLI-001 동작 보존)
- 후속 다른 키 → confirmQuit cancel (CLI-001 동작 보존)

**REQ**: REQ-CLITUI-011 + CLI-001 REQ-CLI-009 (회귀 보호)

---

## 9. AC-CLITUI-009 — Multi-line editor toggle

**Given**:
- TUI 활성, `editor.mode == single` (default), 입력 = "hello"

**When**:
- 사용자가 Ctrl-N

**Then**:
- `editor.mode == multi`
- 기존 "hello" 버퍼 보존 (multi-line textarea 의 첫 줄에 표시)
- focus 가 textarea 로 이동 (`textarea.Focused() == true`)
- snapshot `editor_multiline.golden` byte-equal (입력 영역이 multi-line frame 으로 표시)

**REQ**: REQ-CLITUI-008

---

## 10. AC-CLITUI-010 — Multi-line Ctrl-J inserts newline, Enter sends

**Given**:
- TUI multi-line mode, 입력 = "line1"

**When**:
- 사용자가 Ctrl-J → "line2" 타이핑 → Enter

**Then**:
- ChatStream 에 송신된 user message content == "line1\nline2" (개행 문자 포함)
- input 영역 cleared (`textarea.Value() == ""`)
- mode 는 multi 유지 (Enter 가 모드 변경 안 함)
- mock client 의 `ChatStream` 1 회 호출, messages 마지막 entry = `{role:"user", content:"line1\nline2"}`

**REQ**: REQ-CLITUI-008

---

## 11. AC-CLITUI-011 — Markdown code rendering

**Given**:
- assistant message content 주입 (mock stream): 
  ```
  Here is code:

  ```go
  func main() {}
  ```
  ```
  (정확히는 `\`\`\`go\nfunc main() {}\n\`\`\``)

**When**:
- message 가 `model.messages` 에 append 되고 viewport 갱신

**Then**:
- viewport 출력에 raw ` ``` ` 마커 부재 (`bytes.Contains(out, []byte("\`\`\`")) == false`)
- code block 영역에 syntax highlighting 표식 (ascii termenv 라 색상 대신: indent +2 spaces 또는 box border `│` 등 glamour ascii style 적용)
- inline `code` 도 동일 패턴 적용

**REQ**: REQ-CLITUI-013

---

## 12. AC-CLITUI-012 — `/save <name>` writes jsonl

**Given**:
- TUI 활성, `model.messages` 에 1 user + 1 assistant 메시지 (총 2 entry)
- tmpdir HOME

**When**:
- 사용자가 `/save test01` 입력 후 Enter

**Then**:
- `~/.goose/sessions/test01.jsonl` 파일 생성 (atomic via tmp + rename)
- 파일 내용 2 줄 (JSON Lines), 각각 user/assistant entry
- entry 스키마: `{"role":"user|assistant","content":[{"kind":"text","data_json":"..."}],"ts_ms":...}` (CLI-001 session/file.go 형식 그대로)
- system message `[saved: test01]` viewport 표시

**REQ**: REQ-CLITUI-010

---

## 13. AC-CLITUI-013 — `/load <name>` restores session

**Given**:
- `~/.goose/sessions/test01.jsonl` 에 2 메시지 존재 (AC-CLITUI-012 산출물)
- TUI 활성, 현재 0 메시지

**When**:
- 사용자가 `/load test01` 입력 후 Enter

**Then**:
- `model.messages` 길이 = 2 (복원됨)
- viewport 에 2 메시지 표시
- system message `[loaded: test01, 2 messages]` 표시
- 다음 ChatStream 호출 시 (사용자가 추가 입력 후 Enter): `WithInitialMessages` 옵션에 2 메시지 포함, mock daemon 이 InitialMessages 수신 검증

**REQ**: REQ-CLITUI-010

---

## 14. AC-CLITUI-014 — Ctrl-R recent menu

**Given**:
- `~/.goose/sessions/` 에 3 개 jsonl 파일:
  - `a.jsonl` (mtime: 2026-05-04)
  - `b.jsonl` (mtime: 2026-05-05 09:00)
  - `c.jsonl` (mtime: 2026-05-05 10:00)
- TUI 활성

**When**:
- 사용자가 Ctrl-R

**Then**:
- sessionmenu overlay 열림 (`model.sessionMenuState.visible == true`)
- entry 순서: `c, b, a` (mtime desc)
- 첫 entry (`c`) 에 cursor highlight
- snapshot `session_menu_open.golden` byte-equal
- 사용자가 Esc → overlay 닫힘 (no side effect, no message append)
- 사용자가 Down → cursor `b`, Enter → `/load b` 와 동등 효과 (AC-CLITUI-013 회로)

**REQ**: REQ-CLITUI-007

---

## 15. AC-CLITUI-015 — Ctrl-Up edit last user message

**Given**:
- TUI 활성, `model.messages = [{role:"user",content:"hello"}, {role:"assistant",content:"hi"}]`
- input 비어있음, streaming 비활성

**When**:
- 사용자가 Ctrl-Up
- input 에 "hello" 가 로드됨, prompt 가 `(edit)>` 로 변경
- 사용자가 입력을 "hello world" 로 수정 → Enter

**Then**:
- `model.messages` 에서 기존 user "hello" + assistant "hi" 제거
- `model.messages` 길이 = 1 (`{role:"user",content:"hello world"}`) → ChatStream 호출 직후 길이 2 (assistant 응답 stream 시작)
- mock client `ChatStream` 1 회 호출, 마지막 user message = "hello world"
- `model.editingMessageIndex == -1` (reset)
- prompt 가 `> ` 로 복귀

**REQ**: REQ-CLITUI-009

---

## 16. AC-CLITUI-016 — Cost estimate

**Given**:
- TUI streaming 활성
- config: `cli.pricing.claude-3-5-sonnet.input_per_million=3.0`, `cli.pricing.claude-3-5-sonnet.output_per_million=15.0`
- mock provider 가 stream 종료 시 `usage{input_tokens:1000, output_tokens:500}` 포함

**When**:
- stream 종료 후 `tm.Output()` 캡처

**Then**:
- statusbar 우측에 `~$0.0105` 표시
- 계산: 1000 × 3.0/1e6 + 500 × 15.0/1e6 = 0.003 + 0.0075 = 0.0105
- graceful no-op 보강 검증: pricing 키 부재 시 cost 부분 미표시 (no error log, no panic)
  - 별도 sub-test: `TestStatusbar_CostEstimate_NoConfig_Hides`

**REQ**: REQ-CLITUI-014

---

## 17. AC-CLITUI-017 — Slash help local snapshot 회귀

**Given**:
- TUI 활성

**When**:
- 사용자가 `/help` 입력 후 Enter

**Then**:
- snapshot `slash_help_local.golden` byte-equal
- mock client `ChatStream` 호출 카운트 0 (네트워크 호출 0회 — CLI-001 REQ-CLI-021 회귀 보호)
- viewport 에 system message (built-in command 목록 7+ 종) 표시

**REQ**: 회귀 보호 (CLI-001 AC-CLI-008 보강) + REQ-CLITUI-001 (snapshot 결정성)

---

## 18. AC-CLITUI-018 — In-TUI text language conformance

**Given**:
- `.moai/config/sections/language.yaml` 의 `language.conversation_language=ko`
- TUI 가 막 시작되어 idle 상태 (no streaming, no modal, no overlay)
- 사전 정의된 i18n catalog 가 ko/en 키 양쪽을 cover

**When**:
- 다음 4개 표면을 순서대로 캡처:
  - (a) statusbar idle 상태 prompt → `tm.Output()` 의 마지막 라인
  - (b) `/help` 슬래시 명령 응답 system message → viewport 내 마지막 system message 블록
  - (c) permission modal: mock client 가 `permission_request{tool_name:"Bash"}` 주입 후 modal 첫 라벨/버튼 캡처
  - (d) session menu: `Ctrl-R` 입력 후 overlay 의 헤더 + 빈-목록 placeholder 캡처

**Then**:
- 각 표면의 자연어 부분(키 라벨 `Ctrl-R`/`Tab`/`Enter`, 도구명 `Bash`/`FileWrite`, 파일 경로는 제외)에 ko 로컬라이즈된 사전 정의 substring 이 1개 이상 포함:
  - (a): `세션:`
  - (b): `대화 명령어`
  - (c): `이 도구 호출을 허용하시겠습니까?`
  - (d): `최근 세션`
- 4 표면 모두에서 ko substring 검증이 PASS 해야 본 AC 충족 (compound assertion, all-or-nothing)
- Sub-test `TestStatusbar_LangConformance_EN`: `language.conversation_language=en` 환경 변수 또는 임시 yaml 로 재기동 시 동일 4 표면이 영어 substring 으로 렌더:
  - (a): `Session:`
  - (b): `Conversation commands`
  - (c): `Allow this tool call?`
  - (d): `Recent sessions`

**Snapshot 정책**:
- 본 AC 는 ko/en 2 케이스이므로 snapshot 도 분리: `statusbar_idle_ko.golden` / `statusbar_idle_en.golden`, `slash_help_response_ko.golden` / `_en.golden`, `permission_modal_label_ko.golden` / `_en.golden`, `session_menu_header_ko.golden` / `_en.golden` (총 8 golden, REQ-CLITUI-001 결정성 보존)
- ko/en 외 다른 conversation_language 값은 본 SPEC scope 외 (i18n catalog 미보유 시 fallback to en — REQ-CLITUI-005 의 helper 정의)

**REQ**: REQ-CLITUI-005

---

## 18. Definition of Done (전체 SPEC 수용)

- [ ] 모든 17 AC 시나리오 GREEN (`go test -race -count=10 ./internal/cli/tui/...` 100% PASS)
- [ ] 8 snapshot golden 파일 안정 (macOS + Linux CI byte-equal)
- [ ] Coverage gate 모두 충족:
  - `internal/cli/tui/permission/` ≥ 85%
  - `internal/cli/tui/sessionmenu/` ≥ 85%
  - `internal/cli/tui/editor/` ≥ 85%
  - `internal/cli/tui/snapshots/` ≥ 80%
  - `internal/cli/tui/` ≥ 75%
- [ ] 성능 게이트 충족 (REPL render < 100ms, throughput tick ≥ 4Hz, modal close < 200ms)
- [ ] `go vet ./...` clean, `gofmt -l` 0 file, `golangci-lint run ./internal/cli/tui/...` clean
- [ ] proto `ResolvePermission` RPC 추가 + buf generate 결과물 commit
- [ ] daemon 측 RPC handler stub 동작 (mock E2E)
- [ ] CLI-001 v0.2.0 의 모든 기존 테스트 회귀 0건 (`internal/cli/tui/*_test.go` 기존 PASS 유지)
- [ ] @MX tag 추가/업데이트 (mx_plan §4 plan.md 의 후보 모두 반영, ANCHOR 4개 신규 fan_in 검증, WARN 3개 @MX:REASON 필수)
- [ ] spec.md / plan.md / acceptance.md / spec-compact.md / progress.md 5개 문서 모두 commit
- [ ] PR description 에 변경된 snapshot 와 사유 명시
- [ ] 본 SPEC scope creep guard (plan.md §1 P4-T2/T3 LoC 250 초과 시 별도 SPEC 분리 결정 commit)

---

**End of acceptance.md**
