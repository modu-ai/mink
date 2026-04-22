# SPEC-GOOSE-CLI-001 v0.2.0 — Research & Inheritance Analysis

> **목적**: v0.1.0에서 v0.2.0으로의 재작성 과정에서 발생하는 기술 선택(Connect-Go vs grpc-go 클라이언트, bubbletea vs tview, 키바인딩 세트) 근거 정리.
> **작성일**: 2026-04-21
> **참조**: DEPRECATED.md + ROADMAP v2.0 §4 Phase 3

---

## 1. 레포 상태

- `cmd/`, `internal/cli/` 부재. 본 SPEC은 **v0.1.0 설계를 대부분 계승 + 3개 축 확장**.
- v0.1.0 `spec.md`는 cobra + grpc-go 기반 단일 구조. DEPRECATED.md가 재작성 방향 지시.

---

## 2. 참조 자산별 분석

### 2.1 v0.1.0 계승 자산 (원문 유지)

v0.1.0 SPEC §3.1에서 **유지되는 항목**:

```
✅ 유지: cmd/goosed/main.go, cmd/goose/main.go 구조
✅ 유지: cobra root + subcommand tree
✅ 유지: exit code 0/1/2/69/78 매핑
✅ 유지: --format json, --config, --daemon-addr, --log-level flags
✅ 유지: goose version (ldflags + GetInfo)
✅ 유지: goose ping
✅ 유지: GOOSE_NO_COLOR, GOOSE_SHUTDOWN_TOKEN env
```

v0.1.0 §6.4 `goosed/main.go` 40 LoC 골격은 거의 그대로. TRANSPORT-001 변경 없음.

### 2.2 v0.2.0 신규 확장 3축

**(A) Connect-gRPC 교체** (서버는 그대로, 클라이언트만):

DEPRECATED.md 명시: "Connect-gRPC 클라이언트 통합 (TRANSPORT-001 소비자)".

v0.1.0 `research.md` §4.3:
```
WithBlock은 deprecated 상태(Go grpc 권장 방식 변경) — 대안: ...
본 SPEC은 단순성 위해 WithBlock 유지하되, deprecation 경고는 수용
```

v0.2.0은 **해당 이슈를 본격 해소**: Connect-Go는 `grpc.DialContext`/`WithBlock` 패턴을 대체하는 modern HTTP-based 클라이언트 라이브러리.

**(B) bubbletea TUI** (신규 계층):

DEPRECATED.md 명시: "TUI 고도화 (Ink-like 패턴)".

v0.1.0 §3.2 OUT OF SCOPE: "Ink v6 TUI, Tauri desktop, React Native ... 전부 Phase 5+".

v0.2.0 변경: **TypeScript 기반 Ink v6는 여전히 OUT**. 하지만 **Go 생태계의 bubbletea**는 Phase 3 IN.

이는 모순이 아님 — structure.md §374의 `packages/goose-cli/` (Ink v6 TypeScript)는 별도 패키지로, 본 SPEC의 `cmd/goose` (Go bubbletea)와 **공존**. bubbletea는 Go 단일 바이너리의 TUI를 제공.

**(C) Slash Command 통합**:

DEPRECATED.md 명시: "Slash Command System 연계 (COMMAND-001)".

v0.1.0은 slash command 개념 없이 subcommand만 처리. v0.2.0은 **TUI 입력에 들어오는 `/xxx`** 를 COMMAND-001 `Dispatcher`로 프리-디스패치.

---

### 2.3 Claude Code `entrypoints/cli.tsx` 패턴 참고

v0.1.0 research §2.1에서 인용:

```
- React + Ink v6 기반. 컴포넌트 트리로 CLI UI 렌더.
- `init.ts`(13KB)가 initialization 담당, `cli.tsx`가 rendering.
```

**v0.2.0 참고 포인트** (Go bubbletea로 번역):

| Claude Code (TS/Ink) | GOOSE v0.2.0 (Go/bubbletea) |
|---------------------|------------------------------|
| React 컴포넌트 트리 | tea.Model + View() lipgloss |
| useState / useReducer | Model struct + Update() |
| useStream / useEffect | tea.Cmd goroutine + tea.Msg |
| Ink `<Box>` layout | lipgloss `JoinVertical/Horizontal` |
| Ink `<Text>` | lipgloss style + string |
| Ink input (`@inkjs/ui`) | bubbles/textarea |
| Ink scroll | bubbles/viewport |

**개념 호환**: 이벤트-기반 render loop + model 중심. bubbletea는 Elm Architecture 기반이라 Ink보다 더 함수적.

### 2.4 charmbracelet/crush 참고 (검증 사례)

`.claude/rules/moai/core/lsp-client.md` §1:

> charmbracelet/crush (23k+ GitHub stars as of 2026-04-12).
> crush spawns and manages real language-server subprocesses in production

crush는 bubbletea 기반 TUI 채팅 인터페이스 + 멀티 LLM + LSP 통합. **GOOSE의 목표와 거의 동형**. 구현 참고 우선순위 높음.

bubbletea + lipgloss + bubbles 조합이 crush에서 production-proven. 기술 선택 리스크 낮음.

---

## 3. Connect-Go vs grpc-go Client 비교

### 3.1 비교 표

| 축 | grpc-go (v0.1.0) | Connect-Go (v0.2.0) |
|----|-----------------|---------------------|
| **API 모던성** | 2015년 설계, `DialContext`/`WithBlock` deprecated 경고 | 2022+ 설계, HTTP 클라이언트 기반 |
| **HTTP/2 지원** | ✅ (필수) | ✅ (h2c 또는 TLS) |
| **HTTP/1.1 지원** | ❌ | ✅ (gRPC-Web 또는 Connect 프로토콜) |
| **gRPC-Web 지원** | 별도 proxy 필요 | ✅ 기본 |
| **생성 코드** | `protoc-gen-go-grpc` | `protoc-gen-connect-go` |
| **proto 호환** | 동일 `.proto` 파일 | 동일 `.proto` 파일 |
| **서버 호환** | grpc-go | grpc-go, Connect-Go, any HTTP/2 |
| **라이브러리 크기** | 큼 (~30MB go.sum) | 중간 (~10MB) |
| **유지보수** | Google | Buf (전문 protobuf 회사) |
| **라이선스** | Apache 2.0 | Apache 2.0 |
| **Streaming 지원** | ✅ | ✅ |
| **Interceptor API** | ServerOption/DialOption 복잡 | functional options 단순 |
| **context 전파** | ✅ | ✅ |
| **Deadline propagation** | gRPC header | HTTP header + gRPC header |

### 3.2 선택 이유

**Connect-Go 채택**:

1. **Deprecation 회피**: `grpc.DialContext` + `grpc.WithBlock`은 grpc-go 공식 문서에서 deprecated 표시. 신규 방식 `grpc.NewClient` + `conn.Connect()`는 async 연결로 liveness 검증 패턴 재설계 필요. Connect-Go는 기본적으로 HTTP 클라이언트 위에 올라가므로 `NewClient` 스타일과 자연스럽게 정렬.

2. **미래 web client 준비**: structure.md §374 `packages/goose-web/` (Next.js) 가 gRPC-Web 또는 Connect 프로토콜로 동일 daemon 호출 가능. 프로토콜 옵션만 변경.

3. **HTTP 기반 디버깅 용이성**: curl/httpie로 직접 Connect-Go 서비스 테스트 가능 (JSON 모드 활성화 시). gRPC는 grpcurl 필수.

4. **서버 호환**: Connect-Go 클라이언트 → grpc-go 서버는 HTTP/2 레벨에서 완전 호환. 서버 변경 없음(TRANSPORT-001 그대로 유지).

**거절된 대안**:
- `grpc-go` `NewClient` 전환: 동일하게 HTTP/1.1 미지원. gRPC-Web 필요 시 별도 grpc-web proxy (envoy) 필요.
- `twirp`: 비슷한 HTTP/JSON-RPC 스타일이나 community fork, Buf 제품군 대비 생태계 작음.

### 3.3 코드 예시 비교

**v0.1.0 (grpc-go)**:
```go
conn, err := grpc.DialContext(ctx, addr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithBlock(),                              // deprecated
    grpc.WithTimeout(3*time.Second),                // deprecated
)
client := goosev1.NewDaemonServiceClient(conn)
resp, err := client.Ping(ctx, &goosev1.PingRequest{})
```

**v0.2.0 (Connect-Go)**:
```go
httpClient := &http.Client{
    Transport: &http2.Transport{
        AllowHTTP: true,
        DialTLS: (h2c dialer)...,
    },
}
client := goosev1connect.NewDaemonServiceClient(httpClient, "http://"+addr, connect.WithGRPC())
resp, err := client.Ping(ctx, connect.NewRequest(&goosev1.PingRequest{}))
// resp.Msg → PingResponse
```

차이: `connect.NewRequest`/`resp.Msg` wrapper, HTTP 클라이언트 명시적 구성 가능 (timeout, proxy 등).

---

## 4. TUI 프레임워크 선택: bubbletea vs tview

### 4.1 비교

| 축 | bubbletea | tview |
|----|-----------|-------|
| **아키텍처** | Elm (Model/Update/View) | widget 트리 (tview.Box 계열) |
| **함수형 순수성** | 높음 (immutable Model) | 낮음 (mutable 필드) |
| **Ecosystem** | bubbles, lipgloss 풍부 | 내장 위젯 |
| **사용처** | crush, gh extension, charm/glow | k9s |
| **학습 곡선** | 중 (Elm 생소) | 낮 |
| **테스트 지원** | teatest 공식 | 제한적 |
| **채팅 UX 적합도** | ✅ (streaming 자연스러움) | 중 (Form/Table 중심) |
| **스타일링** | lipgloss (CSS-like) | ANSI 색상 직접 |
| **GitHub stars** | 25k+ | 10k+ |

### 4.2 선택: bubbletea

이유:
- crush (23k star) 실전 사용 검증.
- Elm 아키텍처가 streaming message 누적에 자연스러움 (tea.Cmd로 비동기 완벽 분리).
- lipgloss 통합으로 styling 복잡성 저감.
- teatest로 CI 통합 가능 (terminal 스크립팅).

bubbles 위젯 세트:
- `textarea`: multiline input.
- `viewport`: scrollable message pane.
- `spinner`: streaming indicator.
- `key`: keybinding 선언.

---

## 5. Keybindings 세트 결정

### 5.1 Competitor 관찰

| 행동 | Claude Code | aider | gh | crush | **GOOSE v0.2.0** |
|------|------------|-------|-----|-------|------------------|
| Exit | Ctrl-D | Ctrl-D | Ctrl-C | Ctrl-D | **Ctrl-D** |
| Abort turn | Ctrl-C | Ctrl-C | Ctrl-C | Ctrl-C | **Ctrl-C** |
| Clear screen | Ctrl-L | /clear | - | Ctrl-L | **Ctrl-L** |
| Resume last | (command) | (command) | - | - | **Ctrl-R** |
| Submit | Enter | Enter | - | Enter | **Enter** |
| Newline in input | Shift+Enter | Alt+Enter | - | Alt+Enter | **Alt+Enter** |
| History prev | Up (empty line) | Up | - | - | **Up (empty)** |

### 5.2 Emacs vs Vim preset

`REQ-CLI-024`: `cli.keymap = "emacs"` (기본) 또는 `"vim"`. 

**emacs 기본** (Phase 3):
- `Ctrl-A/E`: 줄 시작/끝
- `Ctrl-K`: kill-to-eol
- `Ctrl-W`: kill-word
- `Alt-F/B`: 단어 이동

**vim preset** (후속):
- `Esc`: 노멀 모드
- `i`: 인서트
- `dd`, `yy`, `p` 등

Phase 3은 emacs preset만 구현. vim은 후속 PR.

---

## 6. Session 파일 포맷 결정

### 6.1 대안 비교

| 포맷 | 장점 | 단점 |
|------|------|------|
| JSON (single object) | 구조화 | 대용량 시 파싱 느림, append 불가 |
| JSONL (line-delimited) | 스트리밍, append 용이 | 각 라인 인코딩 필요 |
| YAML | 사람이 읽기 쉬움 | 복잡 파싱 |
| SQLite | 쿼리 가능 | 바이너리, git-friendly 아님 |
| Parquet/Avro | 압축 | 라이브러리 무거움 |

### 6.2 선택: JSONL

이유:
- Append-only 자연스러움 (streaming 시 라인 단위 flush 가능).
- git diff 친화적 (라인별 변경 관찰).
- 표준 `encoding/json` 사용.
- Hermes/Claude Code agent-memory와 동형.

### 6.3 경로 + 네이밍

`~/.goose/sessions/<name>.jsonl`

- `<name>`: `[a-zA-Z0-9_-]{1,64}` (REQ-CLI-020 경로 탈출 방지).
- 특수 파일: `.last.jsonl` (Ctrl-R 대상, auto-save on exit).
- migration: Phase 3은 version 필드 없음. 향후 `{"version":2,...}` 추가 시 `goose session migrate` 명령.

---

## 7. `goose chat` vs default subcommand

### 7.1 UX 결정

- `goose` (인자 없음) → `goose chat`로 alias.
- `goose chat` → TUI.
- `goose ask <msg>` → non-interactive.

이유:
- Claude Code, aider 모두 기본 진입 TUI (인자 없이 실행 시).
- 사용자 학습비용 최소.
- `goose` alone = 가장 빈번한 용도(대화)에 최단 입력 할당.

### 7.2 Non-TTY fallback

`goose` (stdin not TTY) → stderr에 "interactive mode requires TTY, use 'goose ask' for pipe mode" + exit 2. 또는 자동으로 `ask --stdin` 대체. Phase 3 결정: **명시적 에러** (사용자 의도 불명확).

---

## 8. `goose plugin` stub 처리

PLUGIN-001은 Phase 2 완료 후 의존 SPEC. 본 SPEC 작성 시점에 PLUGIN-001 미착수 가능.

**stub 전략**:
```
goose plugin list   → stdout: "no plugins available (PLUGIN-001 pending)"
goose plugin install <source> → stderr: "goose: plugin system not yet available", exit 1
goose plugin remove <name>    → 동일
```

PLUGIN-001 완료 시 이 부분만 교체. 본 SPEC은 subcommand 노출 + cobra 등록 + stub 메시지로 향후 확장 포인트만 준비.

---

## 9. 테스트 전략

### 9.1 Unit 테스트

**output/formatter** — JSON/JSONL 출력 검증:
- `TestOutput_Plain_String`
- `TestOutput_JSON_WellFormed`
- `TestOutput_JSONL_Streaming` (AC-CLI-015)

**errors/mapping** — REQ-CLI-005:
- `TestExitCode_Mapping` (table-driven 5 케이스)

**transport/client** — Connect-Go wiring:
- `TestDial_Success` (httptest fake server)
- `TestDial_Unreachable_69` (AC-CLI-004)
- `TestDial_Timeout_69`

**session/file**:
- `TestValidateName_RejectsTraversal` (AC-CLI-010 prerequisite, REQ-CLI-020)
- `TestSave_AtomicRename`
- `TestLoad_Roundtrip`

### 9.2 TUI 테스트 (teatest)

```go
// tui_test.go
func TestTUI_HelpLocal_NoNetwork(t *testing.T) {
    mockDispatcher := fakeDispatcher{
        onProcess: func(input string) command.ProcessedInput {
            if input == "/help" {
                return command.ProcessedInput{Kind: command.ProcessLocal, Messages: ...}
            }
            return command.ProcessedInput{Kind: command.ProcessProceed, Prompt: input}
        },
    }
    mockClient := fakeClient{streamCount: 0} // 스트림 호출 추적
    
    model := tui.NewModel(mockClient, mockDispatcher)
    tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))
    tm.Type("/help\n")
    tm.WaitFor(func(out []byte) bool { return bytes.Contains(out, []byte("version")) })
    tm.Send(tea.KeyMsg{Type: tea.KeyCtrlD})
    tm.Wait()
    
    require.Equal(t, 0, mockClient.streamCount) // 네트워크 호출 0
}
```

### 9.3 Integration (subprocess)

v0.1.0 §6.2 방식 계승:

```go
// tests/integration/cli_e2e_test.go (build tag: integration)
func TestE2E_AskWithMockLLM(t *testing.T) {
    daemon := startDaemonSubprocess(t, withMockLLM("Hi!"))
    defer daemon.Shutdown()
    
    out, code := runGooseCLI(t, "ask", "Hello")
    assert.Equal(t, 0, code)
    assert.Equal(t, "Hi!\n", out)
}
```

### 9.4 커버리지 목표

| 패키지 | 목표 |
|--------|------|
| `internal/cli/output` | 95%+ |
| `internal/cli/errors` | 100% |
| `internal/cli/transport` | 90%+ |
| `internal/cli/session` | 95%+ |
| `internal/cli/commands` | 85%+ |
| `internal/cli/tui` | 75%+ (view rendering은 snapshot test로 대체) |
| `cmd/goose/main.go` | 제외 |
| `cmd/goosed/main.go` | 제외 |

---

## 10. 구현 LoC 예상

| 영역 | LoC 예상 |
|------|---------|
| `cmd/goose`, `cmd/goosed` | 60 |
| `internal/cli/rootcmd.go` + errors/output | 300 |
| `internal/cli/commands/` (9 subcommands) | 800 |
| `internal/cli/transport/` (Connect-Go wrapper) | 300 |
| `internal/cli/session/` | 200 |
| `internal/cli/tui/` (model/update/view/stream/statusbar/...) | 1,200 |
| `internal/cli/config/` (RPC wrapper) | 150 |
| proto 확장 (`agent.proto` + `tool.proto` + `config.proto`) | 100 |
| **소계 (production)** | **~3,100** |
| 테스트 | ~3,500 |
| **총계** | **~6,600 LoC** |

ROADMAP §9 Phase 3 Go LoC 예산 2,000 대비 초과. 이유: TUI 계층이 예상보다 큼. 조정:
- TUI 복잡도를 Phase 3 MVP에서 축소 (theme은 default만, keymap은 emacs만).
- 또는 ROADMAP §9 재평가 — Phase 3 실제 규모를 3,000~4,000 LoC로 확정.

**권장**: 본 SPEC은 기능 완성도 우선, ROADMAP 숫자는 실 구현 후 조정.

---

## 11. 외부 의존성 합계

| 모듈 | 버전 | 용도 | 변경 |
|------|------|------|-----|
| `spf13/cobra` | v1.8+ | CLI | v0.1.0 유지 |
| `connectrpc/connect-go` | v1.16+ | 클라이언트 | v0.2.0 신규 |
| `golang.org/x/net/http2` | 최신 | h2c | v0.2.0 신규 |
| `charmbracelet/bubbletea` | v1.2+ | TUI | v0.2.0 신규 |
| `charmbracelet/lipgloss` | v1.1+ | styling | v0.2.0 신규 |
| `charmbracelet/bubbles` | v0.20+ | 위젯 | v0.2.0 신규 |
| `charmbracelet/x/exp/teatest` | 최신 | TUI 테스트 | v0.2.0 신규 |
| `golang.org/x/term` | 최신 | TTY | v0.1.0 유지 |
| `go.uber.org/zap` | v1.27+ | 로그 | 기존 |

v0.2.0 신규 5개 추가. 총 go.sum 크기 ~30MB 증가 예상. 빌드 시간 영향 관찰 필요.

---

## 12. 오픈 이슈

1. **Connect-Go client + grpc-go server compatibility corner cases**: trailer metadata, deadline propagation 미세 차이. AC-CLI-003가 기본 검증, 통합 테스트에서 더 많은 edge case 커버 필요.
2. **`/agent <name>` 명령**: SUBAGENT-001 이후 TUI에 추가. 현재 `/model`만.
3. **TUI에서 tool permission dialog**: QUERY-001 `permission_request` SDKMessage 수신 시 modal 표시. 본 SPEC은 단순 y/n prompt만. 고급 UI(선택지 표시, 세부 permission override)는 HOOK-001.
4. **대형 paste 처리**: REQ-CLI-018 16 KiB 제한. 사용자 피드백 관찰 후 조정.
5. **Session lock**: 동일 세션 파일을 두 개의 `goose` 인스턴스가 동시 로드 시 충돌. 현재는 filesystem lock 없음. 향후 `.lock` 파일 + 프로세스 ID.
6. **`goose session save`를 CLI에서 직접 실행 불가 이유 명확화**: stdin으로 messages array 받지 않음. 이유는 "TUI에서만 현재 대화 상태를 안다". 향후 `goose session import <jsonl>` 추가 검토.
7. **Ctrl-C 시 daemon-side abort 지연 측정**: 500ms SLO가 실제 만족되는지 integration test 추가.
8. **Windows 지원 로드맵**: bubbletea 자체는 Windows 지원. 시그널 처리(`syscall.SIGINT`), 경로 분리자만 조정. 향후 Phase 5+ 검토.

---

## 13. 결론

- **이식 자산**: v0.1.0의 60% (cobra 구조, exit code, version/ping, goosed bootstrap). 40%는 신규(Connect-Go, bubbletea TUI, slash command, session, tool/config/plugin subcommands).
- **참조 자산**: claude-primitives §5 / claude-core §1-3 / crush 구현 / v0.1.0 spec.md.
- **기술 스택**: cobra + connect-go + bubbletea + lipgloss.
- **구현 규모 예상**: ~3,100 production LoC + ~3,500 테스트 = ~6,600 LoC. ROADMAP §9 조정 대상.
- **주요 리스크**: Connect-Go↔grpc-go 호환(R1), bubbletea 터미널 호환(R2), TUI goroutine 누수(R6). 모두 AC-CLI/integration 테스트로 검증.

GREEN 완료 시점에서:
- `goose` (no arg) → TUI REPL.
- `goose ask "msg"` → streaming response.
- `goose session save test01` (TUI 내 `/save test01`) → `~/.goose/sessions/test01.jsonl` 생성.
- `goose session load test01` → 복원 후 TUI.
- `goose tool list`, `goose config get/set/list` 동작.
- `/help`, `/clear`, `/exit`, `/model`, `/compact`, `/status`, `/version` (COMMAND-001 연동).

Phase 3 MVP(`goose ask "hello"` + TUI chat + 6 slash command)가 CLI-001 + TOOLS-001 + COMMAND-001 3개 SPEC 모두 GREEN 시점에 성립.

---

**End of research.md v0.2.0**
