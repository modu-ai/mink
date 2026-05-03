# SPEC-GOOSE-CLI-001 Progress

- **Started**: 2026-04-21 (v0.2.0 재작성, plan phase)
- **Status**: 🟡 PARTIAL — 부분 구현 (기존 cobra + gRPC-go client + bubbletea TUI 골격 존재) — Phase A 진입 단계
- **Mode**: TDD (`quality.development_mode: tdd`, brownfield) — 기존 code 영역은 characterization tests, 신규 영역은 RED-GREEN-REFACTOR
- **Harness**: standard (file_count >10, multi-domain proto/transport/cobra/tui, 비-보안/비-결제)
- **Scale-Based Mode**: Standard
- **Language**: Go (`moai-lang-go`)
- **Greenfield 여부**: brownfield refactor + greenfield (proto 신규)
- **Branch base**: main (PR #66 머지 후, `ed113a3`)
- **Phase**: 3 (Core Workflow)
- **Priority**: P0 (Critical Path — MVP Milestone 1)

## 4-Phase 분할 (~3-4k LoC 총량 → 단계적 PR)

| Phase | 범위 | 신규/수정 | LoC est | 의존성 |
|-------|------|-----------|---------|--------|
| **A** | proto 확장 (agent/tool/config) + Connect-gRPC transport client | 신규 proto 3 + transport refactor | ~800-1200 | TRANSPORT-001 (FROZEN), connectrpc.com/connect v1.19+ |
| **B** | cobra rootcmd 보강 + non-interactive commands (ask/session/config/tool/ping) | 기존 commands/ 보강 | ~800-1000 | Phase A transport |
| **C** | bubbletea TUI (chat REPL, streaming, keybindings, statusbar) | 기존 tui/ 고도화 | ~1000-1500 | Phase A transport stream |
| **D** | Slash command pre-dispatch (COMMAND-001 통합) + E2E + 문서 | 통합 | ~500 | Phase B + C, COMMAND-001 |

각 phase 완료 시 별도 PR. CI green + admin merge → 다음 phase main 기반 진입.

## 의존 / 후속 SPEC 상태

| SPEC | Status | 본 SPEC 과의 관계 |
|------|--------|-------------------|
| SPEC-GOOSE-TRANSPORT-001 | implemented (FROZEN) | `daemon.proto` 소비 + 신규 proto (agent/tool/config) 추가 |
| SPEC-GOOSE-COMMAND-001 | implemented (FROZEN) | Phase D에서 Dispatcher 클라이언트 측 프리디스패치 |
| SPEC-GOOSE-TOOLS-001 | implemented (FROZEN) | `goose tool list` 백엔드 (ToolService.List) |
| SPEC-GOOSE-CONFIG-001 | implemented (FROZEN) | `goose config get/set/list` 백엔드 (ConfigService) |
| SPEC-GOOSE-INSIGHTS-001 | implemented (PR #66 merged) | `/goose insights` (Phase D 또는 후속 wiring) |

## 기존 자산 (PARTIAL — 회고 정리)

- **cmd/goose/main.go**, **cmd/goosed/main.go** — 골격 존재
- **internal/cli/app.go** (7629B), **rootcmd.go** (5717B), **errors.go** (450B) — cobra root 골격
- **internal/cli/commands/** — 다수 sub-command 부분 구현
- **internal/cli/transport/client.go** (4856B) — gRPC-go 기반 (Connect-Go 전환 대상)
- **internal/cli/transport/client_test.go** (8445B) — bufconn mock 기반 client 테스트 (회귀 baseline)
- **internal/cli/tui/** — bubbletea 골격 10개 파일
- **internal/cli/session/** — 세션 파일 골격
- **proto/goose/v1/daemon.proto** (3410B) — TRANSPORT-001 산출물

## Phase A Scope (본 세션)

### 신규 proto 파일 (3건)

1. `proto/goose/v1/agent.proto` (NEW) — `AgentService.Chat` (unary) + `ChatStream` (server streaming) + Message/ContentBlock messages
2. `proto/goose/v1/tool.proto` (NEW) — `ToolService.List` + ToolDescriptor
3. `proto/goose/v1/config.proto` (NEW) — `ConfigService.Get/Set/List` + ConfigEntry

### buf generate 갱신

- `buf.yaml` / `buf.gen.yaml` — Connect-Go target 추가 (이미 indirect dependency 존재 확인: `connectrpc.com/connect v1.19.1`)
- `internal/transport/grpc/gen/goosev1/` — generated code 추가 (agent/tool/config 3개 service)

### transport client 전환

- 기존 `internal/cli/transport/client.go` (gRPC-go DaemonService.Ping/ChatStream)는 **PRESERVE** — characterization test 보존
- 신규 `internal/cli/transport/client_connect.go` (또는 `connect.go`) — Connect-Go 기반 신규 client
  - `NewConnectClient(daemonAddr string, opts ...ConnectOption) (*ConnectClient, error)`
  - `Ping(ctx) (*PingResponse, error)`
  - `Chat(ctx, msg) (*ChatResponse, error)` (unary)
  - `ChatStream(ctx, msg) (<-chan ChatStreamEvent, error)` (server streaming)
  - `ListTools(ctx) ([]ToolDescriptor, error)`
  - `GetConfig(ctx, key) (string, error)` / `SetConfig(ctx, k, v) error` / `ListConfig(ctx, prefix) (map[string]string, error)`
- 기존 `Message` / `StreamEvent` struct 시그니처 보존 (backward-compat — Phase B/C가 점진적 migration)
- mock Connect server 기반 client unit tests (~300 LoC)

### Phase A 완료 조건

- 3 proto 파일 + generated code 컴파일 성공
- ConnectClient 6개 메서드 unit test PASS
- 기존 gRPC-go client 회귀 0건 (`go test ./internal/cli/transport/... -count=10`)
- coverage >= 85% (신규 코드)
- `go vet` / `gofmt` / `golangci-lint` clean
- 신규 외부 의존성: `connectrpc.com/connect`를 indirect → direct로 승격 (이미 v1.19.1 존재)

### Phase A AC (잠정 — spec.md AC-CLI-* 매핑 후속)

| AC | 검증 |
|----|------|
| Phase A-AC-001 | 3 proto 파일 컴파일 + generated code 배치 |
| Phase A-AC-002 | ConnectClient 6 메서드 (Ping/Chat/ChatStream/ListTools/GetConfig/SetConfig/ListConfig) PASS |
| Phase A-AC-003 | 기존 gRPC-go client 회귀 0건 (-count=10) |
| Phase A-AC-004 | mock Connect server 기반 unit test (race PASS) |
| Phase A-AC-005 | coverage >= 85% (transport package) |

## 핵심 보존 약속 (HARD)

- TRANSPORT-001 daemon.proto 변경 0건 (신규 proto만 추가)
- 기존 internal/cli/transport/client.go 시그니처 byte-identical (Message, StreamEvent type)
- 기존 internal/cli/{app,rootcmd,errors}.go 변경 0건 (Phase A는 transport-only)
- Phase A는 cobra commands 직접 wiring 없음 (Phase B 책임)

## TDD 진입 순서 (Phase A)

| RED # | 작업 | Phase A AC |
|------|------|--------|
| #1 | proto 파일 작성 + buf generate 검증 (compile-only test) | A-001 |
| #2 | TestConnectClient_Ping (mock Connect server) | A-002 |
| #3 | TestConnectClient_Chat_Unary | A-002 |
| #4 | TestConnectClient_ChatStream_Streaming | A-002 |
| #5 | TestConnectClient_ListTools | A-002 |
| #6 | TestConnectClient_GetConfig / SetConfig / ListConfig | A-002 |
| #7 | TestRace_ConnectClient_ConcurrentCalls | A-004 |
| #8 | TestExisting_GRPCGoClient_NoRegression (-count=10) | A-003 |
| GREEN | ConnectClient 구현 + helpers |
| REFACTOR | factory 함수 정리, godoc 보강, lint clean |

## 다음 단계

- (a) Phase A run (manager-tdd, isolation worktree) → PR 생성
- (b) Phase A 머지 후 Phase B (cobra commands wiring) 다음 세션 진입
- (c) Phase C (TUI), Phase D (slash + E2E) 후속 세션

---

Last Updated: 2026-05-04 (Phase A 진입 자격 확보, ~800-1200 LoC scope 명세)
Status Source: 사용자 결정 (Phase A만 본 세션 진행)
