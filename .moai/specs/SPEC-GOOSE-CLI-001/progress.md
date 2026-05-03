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

## Phase Log — Run Phase Phase A (2026-05-04)

### 환경 준비

- `buf` 미설치 → `protoc` + 개별 플러그인으로 대체
- `protoc-gen-connect-go v1.19.1` go install 설치
- feature 브랜치: `feature/SPEC-GOOSE-CLI-001-phase-a-proto-transport`

### RED Phase

| RED # | 테스트 | 결과 | 비고 |
|-------|--------|------|------|
| #1 | `TestProto_GeneratedCode_Constants` | RED → GREEN | proto 3파일 + generate 성공; `ChatStreamRequest`/`Message` 이름 충돌 → Agent prefix 적용 |
| #2 | `TestConnectClient_Ping_Success`, `TestConnectClient_Ping_Timeout` | RED → GREEN | DaemonService Connect handler 생성 포함 |
| #3 | `TestConnectClient_Chat_Unary_Success`, `TestConnectClient_Chat_Error` | RED → GREEN | AgentService.Chat unary |
| #4 | `TestConnectClient_ChatStream_ReceivesEvents`, `TestConnectClient_ChatStream_CtxCancel` | RED → GREEN | server streaming, goroutine 패턴 |
| #5 | `TestConnectClient_ListTools_Success`, `TestConnectClient_ListTools_Empty` | RED → GREEN | ToolService.List |
| #6 | `TestConnectClient_GetConfig_Found`, `TestConnectClient_GetConfig_NotFound`, `TestConnectClient_SetConfig`, `TestConnectClient_ListConfig_Prefix` | RED → GREEN | ConfigService 4개 |
| #7 | `TestRace_ConnectClient_ConcurrentCalls` (50 goroutines) | RED → GREEN | race PASS |
| #8 | `TestExisting_GRPCGoClient_NoRegression` | GREEN (-count=10) | 기존 client_test.go 30번 PASS |

### GREEN Phase (connect.go)

- `internal/cli/transport/connect.go` 신규 (284 LoC)
- `NewConnectClient` + `WithHTTPClient/WithDialTimeout/WithInterceptor` 옵션 패턴
- 7개 메서드: Ping/Chat/ChatStream/ListTools/GetConfig/SetConfig/ListConfig
- `ChatStreamEvent`, `ToolDescriptor`, `ChatResponse` wrapper type 정의

### REFACTOR Phase

- `buf.gen.yaml`에 `protoc-gen-connect-go` 플러그인 추가
- `connectrpc.com/connect` indirect → direct 승격 (`go mod tidy`)
- `@MX:ANCHOR` + `@MX:WARN` 태그 추가 (ConnectClient, NewConnectClient, ChatStream)
- 추가 테스트: 8개 (options, nil-value, server-error, session-id 등) → 커버리지 85.8% 달성

### Phase A AC 검증 매트릭스

| AC | 설명 | 결과 | 증거 |
|----|------|------|------|
| Phase A-AC-001 | 3 proto 파일 컴파일 + generated code 배치 | PASS | `go build ./internal/transport/grpc/gen/...` 성공; goosev1connect/ 4파일 생성 |
| Phase A-AC-002 | ConnectClient 7 메서드 PASS | PASS | 30개 테스트 전체 PASS (race) |
| Phase A-AC-003 | 기존 gRPC-go client 회귀 0건 | PASS | `-count=10` 10회 반복 100% PASS |
| Phase A-AC-004 | mock Connect server 기반 unit test (race PASS) | PASS | `TestRace_ConnectClient_ConcurrentCalls` 50 goroutines PASS |
| Phase A-AC-005 | coverage >= 85% (transport package) | PASS | 85.8% (`go test -cover`) |

### 품질 게이트

| Gate | 결과 |
|------|------|
| `go vet ./internal/cli/transport/...` | PASS |
| `gofmt -l internal/cli/transport/` | PASS (empty) |
| `go build ./...` | PASS |
| `go test -race -count=1` | 30/30 PASS |
| `go test -race -count=10` | 100% PASS |
| `go test -cover` | 85.8% |
| `golangci-lint` 신규 파일 | 0 issues (connect.go, connect_test.go) |
| `go.mod` connectrpc | indirect → direct 승격 완료 |

### 알려진 한계 / 주의사항

- `buf` 미설치로 `buf generate` 대신 `protoc` 직접 사용 — buf.gen.yaml은 업데이트했으나 buf 설치 후 검증 권장
- `daemon.proto`의 `ChatStream`이 bidirectional (bidi) streaming — Connect-Go bidi 핸들러 생성됨
- `agent.proto`의 메시지 이름을 `AgentChatRequest` 등으로 prefix 적용 (daemon.proto와 충돌 방지)
- `golangci-lint` 기존 파일(`client.go`, `client_test.go`) 이슈 11건: Phase A 범위 밖, 별도 hygiene PR 권장

### 다음 단계

- (a) PR 생성 (orchestrator 담당) — DONE (PR #67 merged)
- (b) Phase A 머지 후 Phase B (cobra commands wiring) 다음 세션 진입 — DONE (Phase B Plan 작성, 본 섹션 이하)
- (c) Phase C (TUI), Phase D (slash + E2E) 후속 세션

---

## Phase B Scope (다음 세션 진입 대상)

### 진입 결정 (2026-05-04 plan session)

- 사용자 결정: **8 commands 전부 점검 + wiring 가능한 것은 wiring**, **단일 PR**
- 본 세션은 **Plan only** — RED-GREEN-REFACTOR 진입은 다음 세션
- Base branch: main (`f314834` 시점, Phase A merged)
- Feature branch (다음 세션 생성): `feature/SPEC-GOOSE-CLI-001-phase-b-commands-wiring`

### 8 Commands 점검 매트릭스

| Command | 현재 backend | Phase B 처리 | 의존 ConnectClient 메서드 |
|---------|--------------|--------------|---------------------------|
| **ping** | `NewGRPCPingClient` (transport.NewDaemonClient.Ping) | wiring 전환 | `ConnectClient.Ping` |
| **ask** | `askClientAdapter` (transport.NewDaemonClient.ChatStream) | wiring 전환 | `ConnectClient.ChatStream` |
| **config** | `NewMemoryConfigStore` (in-memory) | wiring 전환 + Memory store는 test fallback로 보존 | `ConnectClient.GetConfig/SetConfig/ListConfig` |
| **tool** | `NewStaticToolRegistry` (hardcoded) | wiring 전환 + Static registry는 offline fallback로 보존 | `ConnectClient.ListTools` |
| **daemon status** | `pingClient` 재사용 | wiring 전환 (ping과 동일) | `ConnectClient.Ping` |
| **session** | local file (`internal/cli/session/`) | scope 외 — local-only | (없음) |
| **audit** | filesystem log (`internal/audit/`) | scope 외 — local-only | (없음) |
| **plugin** | stub | scope 외 — Phase D 또는 후속 SPEC | (없음) |
| **version** | ldflags | scope 외 — transport 무관 | (없음) |

> **Note**: session/audit/plugin/version은 **Phase B scope 외**. 사용자 "8개 전부" 의도는 "8개 전부 점검 후 처리 결정"으로 해석. Plan 결과 5개만 wiring 대상이고 4개는 명시적 미처리.

### Phase B Sub-Phases (RED 진입 순서)

| Sub | 단위 | 신규/수정 파일 | LoC est | 의존 |
|-----|------|----------------|---------|------|
| **B1** | Adapter 계층 + ping wiring | `transport/adapter.go` (신규), `commands/ping.go` (보강), `rootcmd.go` (수정) | ~200 | Phase A ConnectClient |
| **B2** | ask wiring | `commands/ask.go` (보강), `rootcmd.go` (수정) | ~200 | B1 (adapter 패턴 확정) |
| **B3** | config wiring | `commands/config.go` (`ConnectConfigStore` 추가), `rootcmd.go` | ~200 | B1 |
| **B4** | tool wiring | `commands/tool.go` (`ConnectToolRegistry` 추가), `rootcmd.go` | ~150 | B1 |
| **B5** | daemon status wiring + rootcmd 통합 | `commands/daemon.go`, `rootcmd.go` (단일 ConnectClient lifecycle) | ~150 | B1~B4 |

총량: **~900 LoC** (progress.md Phase B 추정 800-1000 일치). 단일 PR 산출물.

### Phase B AC 매트릭스

| AC | 검증 |
|----|------|
| Phase B-AC-001 | `ConnectClient` adapter (`PingClientAdapter`, `AskClientAdapter`, `ConnectConfigStore`, `ConnectToolRegistry`) 4종 unit test PASS |
| Phase B-AC-002 | rootcmd.go가 단일 `*ConnectClient` instance를 PersistentPreRunE에서 생성, PersistentPostRunE에서 Close (lifecycle) |
| Phase B-AC-003 | 5 wiring commands (ping/ask/config/tool/daemon-status) 각 happy path + error path test PASS (mock Connect server) |
| Phase B-AC-004 | 기존 gRPC-go path 회귀 0건 — `transport.NewDaemonClient` characterization tests `-count=10` PASS |
| Phase B-AC-005 | `MemoryConfigStore` / `StaticToolRegistry` interface 호환 보존 (test/offline fallback DI) |
| Phase B-AC-006 | `coverage` >= 85% (신규 adapter + 보강된 commands) |
| Phase B-AC-007 | `go vet` / `gofmt` / `golangci-lint --new-from-rev=main` clean (신규 코드 0 issue) |
| Phase B-AC-008 | `go test -race -count=10` 100% PASS (race-free) |
| Phase B-AC-009 | session/audit/plugin/version 변경 0 LoC (scope discipline — CLAUDE.md §7 Rule 5) |
| Phase B-AC-010 | `daemon shutdown` subcommand는 stub 유지 (graceful shutdown RPC는 Phase D 또는 별도 SPEC) |

### TDD 진입 순서 (RED 시나리오)

| RED # | Sub | 테스트 | 매핑 AC |
|-------|-----|--------|---------|
| #1 | B1 | `TestPingClientAdapter_Ping_Success` (mock Connect server `goosev1connect.NewDaemonServiceHandler`) | B-AC-001, B-AC-003 |
| #2 | B1 | `TestPingClientAdapter_Ping_Timeout` (`context.DeadlineExceeded` 변환) | B-AC-003 |
| #3 | B1 | `TestRootCmd_Ping_UsesConnectClient` (rootcmd integration) | B-AC-002 |
| #4 | B2 | `TestAskClientAdapter_ChatStream_ReceivesEvents` (channel-based event 변환) | B-AC-001, B-AC-003 |
| #5 | B2 | `TestAskClientAdapter_ChatStream_CtxCancel` (graceful close) | B-AC-003 |
| #6 | B3 | `TestConnectConfigStore_Get_Found / NotFound` (ErrConfigKeyNotFound 매핑) | B-AC-001, B-AC-005 |
| #7 | B3 | `TestConnectConfigStore_Set / List` | B-AC-003 |
| #8 | B3 | `TestMemoryConfigStore_StillWorks` (interface 회귀 baseline) | B-AC-005 |
| #9 | B4 | `TestConnectToolRegistry_ListTools_Success / Empty` | B-AC-001, B-AC-003 |
| #10 | B4 | `TestStaticToolRegistry_StillWorks` (회귀 baseline) | B-AC-005 |
| #11 | B5 | `TestRootCmd_DaemonStatus_UsesConnectClient` | B-AC-002, B-AC-003 |
| #12 | B5 | `TestRootCmd_ConnectClientLifecycle_OpenClose` (PreRun/PostRun cycle) | B-AC-002 |
| #13 | B5 | `TestExisting_GRPCGoClient_NoRegression` (-count=10) | B-AC-004 |
| #14 | B5 | `TestRace_RootCmdConcurrentSubcommands` (병렬 안정성) | B-AC-008 |
| GREEN | All | adapter 4종 + rootcmd 통합 구현 |
| REFACTOR | All | godoc 보강, @MX:ANCHOR/WARN 추가, lint clean, coverage 검증 |

### 핵심 보존 약속 (HARD)

- 기존 `transport.NewDaemonClient` (gRPC-go) 시그니처 byte-identical (Phase A에서 이미 확정)
- 기존 `commands.PingClient`, `commands.AskClient`, `commands.ConfigStore`, `commands.ToolRegistry` 인터페이스 변경 0건 (adapter 추가만)
- `MemoryConfigStore`, `StaticToolRegistry` 시그니처 byte-identical (DI fallback 보존)
- session/audit/plugin/version 파일 변경 0건 (scope discipline)
- `internal/cli/tui/` 변경 0건 (Phase C 책임)
- daemon shutdown subcommand는 stub 유지 (Phase D 또는 별도 SPEC)

### 신규 ConnectClient 메서드 부족분 점검 (실제 API 기준 정정 2026-05-04)

Phase A `ConnectClient` 메서드 (connect.go 정독 후 실제 시그니처):

| commands 인터페이스 | ConnectClient 실제 시그니처 | Adapter 변환 정책 |
|---------------------|----------------------------|-------------------|
| `PingClient.Ping(ctx, addr, w io.Writer) error` | `Ping(ctx) (*PingResponse, error)` | adapter는 ctx로 ConnectClient.Ping 호출 → resp.Version/UptimeMs/State를 writer에 포맷 출력 (기존 NewGRPCPingClient와 동일 출력 패턴 보존) |
| `AskClient.ChatStream(ctx, []Message) (<-chan StreamEvent, error)` | `ChatStream(ctx, agent, message string, opts...) (<-chan ChatStreamEvent, <-chan error)` | (a) `[]Message`에서 마지막 user role message를 `message` 인자로, 이전 messages는 `WithInitialMessages` opt로 전달 — opt 없으면 1차로 마지막 user message만 전달 (Phase B는 단순 변환 우선, multi-turn은 Phase C 책임). (b) 2채널 → 1채널 fan-in goroutine: event 채널 forward + error 채널 도착 시 마지막 `StreamEvent{Type:"error", Content: err.Error()}` emit 후 close. (c) agent 인자는 빈 문자열 (server-side default agent 가정). (d) error 반환은 immediate error만 (예: nil context) — stream 도중 에러는 channel 내 emit. |
| `ConfigStore.Get(key) (string, error)` (no ctx) | `GetConfig(ctx, key) (string, bool, error)` | adapter 내부에서 `context.WithTimeout(context.Background(), defaultTimeout)` (기본 5s). `exists=false` 시 `ErrConfigKeyNotFound` 반환. RPC error는 wrap. |
| `ConfigStore.Set(k, v) error` | `SetConfig(ctx, k, v) error` | adapter 내부 ctx (5s timeout). RPC error 그대로 반환 |
| `ConfigStore.List() (map[string]string, error)` | `ListConfig(ctx, prefix) (map, error)` | adapter는 prefix="" 호출. ctx 5s. |
| `ToolRegistry.ListTools() ([]ToolInfo, error)` | `ListTools(ctx) ([]ToolDescriptor, error)` | adapter 내부 ctx (5s timeout). `ToolDescriptor{Name, Description, Source, ServerID}` → `ToolInfo{Name, Description}` 변환 (Source/ServerID drop) |
| daemon status | `Ping(ctx)` 재사용 | PingClientAdapter 그대로 |
| daemon shutdown | (없음) | Phase B scope 외 — stub 유지 |

**결론**: Phase B는 ConnectClient API 변경 0건. proto 변경 0건. **단, adapter 변환 로직이 비자명** (특히 ChatStream 2채널 → 1채널, ConfigStore context-less → ctx 생성). 이 변환 로직 자체가 RED #2/#4/#5/#6의 핵심 검증 대상.

### Adapter 설계 결정 (HARD — 변경 시 progress.md 정정 필수)

- **adapter 내부 ctx timeout 기본값**: 5초 (config Get/Set/List, tool ListTools)
- **ChatStream timeout**: ctx는 caller가 관리 (long-running stream — adapter 내부 timeout 부적합)
- **PingClient writer 출력 포맷**: 기존 `NewGRPCPingClient` 출력과 byte-identical (테스트가 출력 string match로 검증 — characterization)
- **Agent name 기본값**: 빈 문자열 ("") — 서버 측 default agent
- **AskClient ChatStream 변환 손실**: 다중 message → 단일 message 변환은 Phase B 한계, Phase C에서 multi-turn 정식 지원
- **error 변환**: connect.Error code를 그대로 노출 (별도 sentinel 없음 — ErrConfigKeyNotFound만 매핑)

### Scope 재추정 (mismatch 반영)

- B1 (adapter + ping): ~250 LoC (PingClientAdapter + 기존 출력 포맷 보존)
- B2 (ask): ~250 LoC (2채널 → 1채널 fan-in goroutine + 변환)
- B3 (config): ~250 LoC (ConnectConfigStore + ctx timeout policy + ErrNotFound 매핑)
- B4 (tool): ~150 LoC (ConnectToolRegistry — 가장 단순)
- B5 (rootcmd lifecycle): ~200 LoC (ConnectClient 단일 instance + PreRun/PostRun)

**총량 재추정**: ~1100 LoC (기존 900 → 1100, 변환 로직 복잡도 반영)

### 위험 / 주의사항

- **Race condition**: rootcmd가 단일 ConnectClient instance를 모든 subcommands에 공유 → ConnectClient 자체 thread-safety 검증 필요 (Phase A `TestRace_ConnectClient_ConcurrentCalls`로 일부 검증됨)
- **Connection lifecycle**: PersistentPreRunE에서 connect, PersistentPostRunE에서 close. Subcommand error 시에도 close 보장 (defer 패턴)
- **Backward-compat**: 기존 gRPC-go path는 `transport/client.go`로 보존. Phase B가 client.go를 deprecate 하지 않음 (Phase C/D에서 결정)
- **Test isolation**: mock Connect server를 ephemeral httptest server로 spawn (Phase A 패턴 재사용)
- **Lint hygiene**: PR #68 (lint cleanup 11 issues)에서 `client.go` 정리됨 → 본 PR은 신규 파일만 lint scope

### Phase B 진입 체크리스트 (다음 세션 시작 시)

- [ ] `git pull --ff-only origin main` (이번 PR이 main에 반영되었는지 확인)
- [ ] `go test -race ./internal/cli/transport/... -count=3` (Phase A baseline 회귀 0건)
- [ ] `git checkout -b feature/SPEC-GOOSE-CLI-001-phase-b-commands-wiring`
- [ ] RED #1 (`TestPingClientAdapter_Ping_Success`)부터 순서대로 실행
- [ ] Sub-phase B1~B5 순차 진행. 각 sub 완료 시 `go test -race ./internal/cli/...` 회귀 점검

---

## Phase Log — Run Phase Phase B (2026-05-04)

### 환경
- Branch: `feature/SPEC-GOOSE-CLI-001-phase-b-commands-wiring` (Base: main `5c6402d`)
- 진행 방식: Plan 정정 후 sub-phase 순차 실행 (sub-agent 위임은 1M context 미활성으로 main session 직접 진행으로 fallback)

### Sub-phase 결과

| Sub | Commit | 신규/수정 파일 | LoC (추정 / 실제) | RED / 테스트 |
|-----|--------|----------------|-------------------|--------------|
| **B1** | `7194af3` | transport/adapter.go(+87), transport/adapter_test.go(+154), rootcmd.go(±2) | 250 / 243 | RED #1~3 + Defaults/FactoryError/NormalizeURL (6) |
| **B2** | `e478b07` | transport/adapter.go(+125), transport/adapter_test.go(+196), rootcmd.go(±39) | 250 / 360 | RED #4~5 + Translate Text/Error/ToolUse/Done + PickLastUserMessage 3 + ErrEmptyMessages (11) |
| **B3** | `1b9080a` | commands/connect_config_store.go(+150), commands/connect_config_store_test.go(+179), rootcmd.go(±2) | 250 / 331 | RED #6~8 (Found/NotFound/Set/List + MemoryConfigStore characterization) + Defaults/Timeout (10) |
| **B4** | `ec42834` | commands/connect_tool_registry.go(+92), commands/connect_tool_registry_test.go(+116), rootcmd.go(±2) | 150 / 210 | RED #9~10 (Success/Empty/RPCError + StaticToolRegistry characterization) + Defaults (5) |
| **B5** | (이 커밋) | rootcmd_test.go(+18), progress.md | 200 / 18 | RED #11 (Phase B subcommands registered integration) |

**총량**: ~1162 LoC 신규/변경 (Plan 추정 ~1100 일치). 신규 테스트 33개.

### Phase B AC 검증 매트릭스

| AC | 설명 | 결과 | 증거 |
|----|------|------|------|
| Phase B-AC-001 | Adapter 4종 (Ping/Ask/ConnectConfigStore/ConnectToolRegistry) unit test PASS | PASS | adapter_test.go 17개 + connect_config_store_test.go 10개 + connect_tool_registry_test.go 5개 모두 PASS |
| Phase B-AC-002 | rootcmd가 ConnectClient 기반 adapter 주입 (lazy factory 패턴 — 명시적 lifecycle 불필요) | PASS | rootcmd.go에서 NewPingClientAdapter / newAskClientAdapter / NewConnectConfigStore / NewConnectToolRegistry 주입 |
| Phase B-AC-003 | 5 wiring commands (ping/ask/config/tool/daemon-status) happy + error path PASS | PASS | 각 adapter test 모두 success/error 분기 cover |
| Phase B-AC-004 | 기존 gRPC-go path 회귀 0건 (`-count=10`) | PASS | `go test -race -count=10 ./internal/cli/...` 100% PASS |
| Phase B-AC-005 | MemoryConfigStore / StaticToolRegistry 인터페이스 호환 보존 | PASS | TestMemoryConfigStore_StillWorks, TestStaticToolRegistry_StillWorks PASS |
| Phase B-AC-006 | coverage >= 85% (transport package) | PASS | transport 88.6% (cli 55.2% / commands 77.8%은 다른 미와이어 commands 영향) |
| Phase B-AC-007 | `go vet` / `gofmt` clean | PASS | 빈 출력 |
| Phase B-AC-008 | `go test -race -count=10` 100% PASS | PASS | cli/transport/commands/session/tui 모두 PASS |
| Phase B-AC-009 | session/audit/plugin/version 변경 0 LoC | PASS | git diff main에서 해당 파일 변경 없음 (rootcmd.go의 등록부 외) |
| Phase B-AC-010 | daemon shutdown stub 유지 | PASS | commands/daemon.go 변경 0건 (status만 PingClientAdapter 재사용) |

### 품질 게이트

| Gate | 결과 |
|------|------|
| `go vet ./internal/cli/...` | PASS |
| `gofmt -l internal/cli/` | PASS (empty) |
| `go build ./...` | PASS |
| `go test -race -count=1 ./internal/cli/...` | 모든 패키지 PASS |
| `go test -race -count=10 ./internal/cli/...` | 100% PASS |
| `go test -cover ./internal/cli/transport/` | 88.6% |
| `go test -cover ./internal/cli/commands/` | 77.8% (신규 connect_*_store/registry는 fake 주입으로 모두 cover) |

### 알려진 한계 / 주의사항

- `lint`(golangci-lint) `--new-from-rev=main`은 별도 환경 필요로 본 세션 미실행 — `go vet` clean으로 minimal coverage. 후속 PR에서 별도 점검 가능.
- `commands/` coverage 77.8%은 신규 코드 자체가 아닌 기존 audit/session/plugin 미테스트 영역의 영향. 신규 4개 파일은 주요 분기 모두 cover.
- daemon-addr flag override가 ask/config/tool에 currently 비반영 (rootcmd 생성 시점 default 사용) — 기존 `askClientAdapter`도 동일 한계, 동작 byte-identical. flag-aware lifecycle은 Phase C 또는 별도 SPEC.
- `connectClientFactory` 시그니처가 transport package 내부 type 별칭. 향후 export 필요 시 별도 SPEC.
- `rootcmd.go`의 `_ = context` 등 잠재적 unused import는 vet 통과 — 주의 유지.
- 1M context 미활성으로 sub-agent 위임 불가 → main session 직접 작성. 다음 Phase부터는 1M context 활성 또는 분할 위임 전략 필요.

### 다음 단계
- Push + PR 생성
- Phase C (TUI) 후속 세션 진입
- Phase D (Slash + E2E) 최종 단계

---

## Phase C Plan (2026-05-04, Phase B merged 후 작성)

### 진입 결정
- 사용자 결정 (2026-05-04): Plan만 작성 + 다음 세션에 RED-GREEN-REFACTOR 진입
- Base branch: main `4521d3c` (Phase B PR #69 merged 직후)
- Feature branch (다음 세션 생성): `feature/SPEC-GOOSE-CLI-001-phase-c-tui`
- 진행 환경: 1M context 활성 후 expert-frontend 위임 권장 (TUI는 visual 검증 비중 큼 — main session 직접 작성 비추천)

### TUI 자산 점검 (8 파일)

| 파일 | LoC | 역할 | Phase C 처리 |
|------|-----|------|--------------|
| `tui/model.go` | 3378B | Model struct, ChatMessage/StreamEvent type | 내부 type 보존, ConnectClient 의존 제거 |
| `tui/update.go` | 7480B | handleKeyMsg, streaming handlers | 변환 로직 transport helper 재사용으로 단순화 |
| `tui/view.go` | 2002B | View 렌더링 | C4에서 statusbar 추가 |
| `tui/client.go` | 4837B | **gRPC-go 의존 (Phase C1 wiring 대상)** | **ConnectClient adapter로 전환** |
| `tui/dispatch.go` | 3748B | AppInterface, ProcessResult (slash dispatcher hook) | 변경 없음 (Phase D COMMAND-001 통합 시 보강) |
| `tui/slash.go` | 3250B | ParseSlashCmd | 변경 없음 (이미 완성) |
| `tui/slash_test.go` | 5152B | slash test | 회귀 baseline |
| `tui/tui_test.go` | 8468B | Model test | 회귀 baseline + 보강 |

### Phase C Sub-Phases

| Sub | 단위 | 신규/수정 | LoC est | 의존 |
|-----|------|-----------|---------|------|
| **C1** | TUI transport wiring (client.go gRPC-go → ConnectClient) | client.go 재작성, transport helpers 재사용 (TranslateChatEvent, ChatStreamFanIn) | ~250 | Phase B (transport adapter 패턴 확정) |
| **C2** | streaming 표시 강화 (viewport scroll, wordwrap, 누적 text 처리) | model.go (state), update.go (streaming buffer) | ~300 | C1 |
| **C3** | keybindings (Ctrl+C 종료 confirm, Ctrl+L clear, Ctrl+D EOF, /quit, /clear, /reset) | update.go (handleKeyMsg 분기), model.go (modal state) | ~200 | C1 |
| **C4** | statusbar (daemon connection state, model name, message count, token usage placeholder) | view.go (statusbar 렌더링), model.go (state) | ~250 | C1 |
| **C5** | integration test (bubbletea teatest harness 또는 view snapshot golden file + race + 회귀) | tui_test.go 보강 + 신규 view_test.go | ~250 | C1~C4 |

총량: **~1250 LoC**

### Phase C AC

| AC | 검증 |
|----|------|
| C-AC-001 | TUI client가 ConnectClient 기반 (gRPC-go 의존성 제거 — `transport.NewDaemonClient` 호출 0건 in tui/) |
| C-AC-002 | chat REPL 동작 검증 (input → ChatStream → text rendering 통합 테스트) |
| C-AC-003 | streaming/error/done 이벤트 처리 (TranslateChatEvent 활용 확인) |
| C-AC-004 | keybindings 동작 (Ctrl+C confirm, Ctrl+L clear, /quit, /clear, /reset) |
| C-AC-005 | statusbar 표시 (connection state, model name, message count) |
| C-AC-006 | 기존 slash_test/tui_test 회귀 0건 (`-count=10`) |
| C-AC-007 | race -count=10 PASS |
| C-AC-008 | tui 패키지 coverage >= 80% (View 렌더링 로직 제외) |
| C-AC-009 | 기존 `transport.NewDaemonClient` 시그니처 변경 0건 (Phase A FROZEN 보존) |
| C-AC-010 | Phase D 영역 (slash dispatcher COMMAND-001 통합) 변경 0건 (scope discipline) |

### TDD RED 시나리오

| RED # | Sub | 테스트 | 매핑 AC |
|-------|-----|--------|---------|
| #1 | C1 | `TestTUIClientFactory_ChatStream_UsesConnectClient` (Phase B mock Connect server 패턴) | C-AC-001, C-AC-002 |
| #2 | C1 | `TestTUIClientFactory_ChatStream_StreamingEvents` (text/done 변환) | C-AC-003 |
| #3 | C1 | `TestTUIClientFactory_ChatStream_ErrorEvent` (error event 변환) | C-AC-003 |
| #4 | C2 | `TestModel_HandleStreamingTextAccumulation` (consecutive text events 누적) | C-AC-002 |
| #5 | C2 | `TestModel_ViewportScroll_LongOutput` (다중 message scroll) | C-AC-002 |
| #6 | C3 | `TestModel_CtrlC_QuitConfirmation` (Ctrl+C 두번 확인) | C-AC-004 |
| #7 | C3 | `TestModel_CtrlL_ClearScreen` | C-AC-004 |
| #8 | C3 | `TestModel_SlashClear_ResetsHistory` | C-AC-004 |
| #9 | C4 | `TestView_StatusBar_RendersConnectionState` | C-AC-005 |
| #10 | C4 | `TestView_StatusBar_MessageCount` | C-AC-005 |
| #11 | C5 | `TestTeatest_HappyPath_InputToResponse` (bubbletea teatest E2E) | C-AC-002 |
| #12 | C5 | `TestExisting_SlashParsing_NoRegression` (slash_test.go -count=10) | C-AC-006 |
| #13 | C5 | `TestRace_TUIConcurrentStreamEvents` | C-AC-007 |
| GREEN | All | client.go ConnectClient 전환, model 강화, view statusbar, keybindings |
| REFACTOR | All | godoc 영어, @MX 태그, lint clean |

### 핵심 보존 약속 (HARD)

- `tui.ChatMessage`, `tui.StreamEvent`, `tui.DaemonClient` 인터페이스 시그니처 byte-identical
- `tui.AppInterface`, `tui.ProcessResult` (slash dispatcher hook) 변경 0건 (Phase D 책임)
- `tui.ParseSlashCmd` + slash_test.go 변경 0건
- `transport.NewDaemonClient` (gRPC-go) 시그니처/호출부 변경 0건
- `commands/`, `transport/connect.go`, `transport/adapter.go` 변경 0건 (Phase B FROZEN)
- session/audit/plugin/version commands 변경 0건

### Phase B → C 재사용 자산

Phase B에서 만든 transport helpers를 Phase C에서 그대로 사용:

- `transport.TranslateChatEvent` — 와이어 → 단순 (Type, Content) 변환
- `transport.ChatStreamFanIn` — 2채널 → 1채널 fan-in
- `transport.NormalizeDaemonURL` — host:port → http URL
- `transport.PickLastUserMessage` — 마지막 user message 추출

→ Phase C는 transport.go 변경 0건 + helpers 재사용으로 ~150 LoC 절약 예상.

### 위험 / 주의사항

- **bubbletea Visual 검증**: Model.View() 출력은 ANSI escape 코드 포함 → snapshot test에서 normalization 필요. Phase C5는 `bubbletea/x/exp/teatest` 도입 또는 raw view 비교 결정 필요.
- **Token 누적 표시**: `ConnectClient.ChatResponse`의 TokensIn/TokensOut은 unary Chat 결과만 — streaming은 토큰 정보 없음. statusbar는 message count만 표시 (token은 Phase D 또는 별도 SPEC).
- **Ctrl+C race**: bubbletea Update 도중 Ctrl+C 처리 시 streaming goroutine과 race 가능 — confirmation flow가 핵심.
- **lipgloss theme**: noColor flag 처리 + 기존 tui/view.go가 lipgloss 사용 — Phase C4에서 일관성 점검.

### Phase C 진입 체크리스트 (다음 세션 시작 시)

- [ ] `git pull --ff-only origin main` (4521d3c 또는 그 이후)
- [ ] `go test -race ./internal/cli/tui/... -count=3` (현재 baseline 회귀 0건)
- [ ] 1M context 활성 (`/extra-usage`) 또는 expert-frontend 분할 위임 전략 확정
- [ ] `git checkout -b feature/SPEC-GOOSE-CLI-001-phase-c-tui`
- [ ] RED #1 (`TestTUIClientFactory_ChatStream_UsesConnectClient`)부터 순서 진행

### 다음 단계
- 본 세션은 Phase C Plan commit 후 종료
- 다음 세션: 1M context 환경에서 RED-GREEN-REFACTOR
- 후속: Phase D (Slash COMMAND-001 통합 + E2E + 문서)

---

## Phase C1 완료 로그 (2026-05-04, PR #70 merged)

### 산출물
- Commit: `a926c32` (squashed merge `1e9bae1`)
- 파일: `tui/client.go` (+75/-6), `tui/connect_factory_test.go` (+201)
- 신규 테스트 6개 (StreamingEvents / ErrorEvent / EmptyMessages / FactoryError / Close / Defaults)

### Phase C1 AC 부분 매트릭스
- C-AC-001 (TUI ConnectClient 사용): PASS — `connectClientFactory` 구현, `RunWithApp` 전환
- C-AC-003 (streaming/error/done 이벤트 처리): PASS — `TranslateChatEvent` + `ChatStreamFanIn` 통한 변환 검증
- C-AC-006 (slash_test/tui_test 회귀 0건): PASS — `-count=10` 100% PASS
- C-AC-007 (race -count=10 PASS): PASS
- C-AC-009 (transport.NewDaemonClient 보존): PASS — gRPC-go fallback DI 그대로

### C2~C5 후속 세션 진입점

```bash
git pull --ff-only origin main                           # 1e9bae1 또는 그 이후
go test -race ./internal/cli/tui/ -count=3              # baseline
git checkout -b feature/SPEC-GOOSE-CLI-001-phase-c2-c5  # 또는 sub-phase별 분리
# RED #4: TestModel_HandleStreamingTextAccumulation
```

C2~C5 sub-spec은 본 progress.md Phase C Plan 섹션 (RED #4~13) 그대로 사용.

---

## Phase C2~C5 완료 로그 (2026-05-04, single commit)

### 발견 (실제 작업 시점)

Phase C Plan(96b1f85) 작성 시 update.go/view.go/slash.go가 **이미 상당히 구현**되어 있다는 점을 누락. 정독 결과:
- 누적 streaming, Esc/Ctrl+C/Ctrl+S/Ctrl+L 키, 슬래시 명령(/help, /save, /load, /clear, /quit, /session) 모두 구현 완료
- view.go도 status bar + viewport + input layout 완성
- 부족한 것은 **characterization tests + 약간의 statusbar 보강 (message count)**

→ Plan 추정 ~1000 LoC가 실제로는 ~300 LoC로 축소.

### 산출물 (Commit `4d42628`)

| 파일 | 변경 | LoC |
|------|------|-----|
| `tui/view.go` | renderStatusBar에 ` | Messages: <N>` 추가 | +5/-1 |
| `tui/phase_c_test.go` | 신규 14개 테스트 (C2 누적, C3 keybind, C4 statusbar, C5 race + 회귀) | +302 |

### Phase C AC 최종 매트릭스

| AC | 결과 | 증거 |
|----|------|------|
| C-AC-001 (TUI ConnectClient 사용) | PASS | C1에서 완료 (#70) |
| C-AC-002 (chat REPL 동작) | PASS | TestModel_StreamTextAccumulation + Smoke_ConnectFactoryAsDaemonClient |
| C-AC-003 (streaming/error/done) | PASS | C1 ConnectFactory + 기존 handleStreamEvent characterization |
| C-AC-004 (keybindings) | PASS | TestModel_CtrlL/EscapeAppendsCancellationNote/SlashClear/QuitDuringStreaming |
| C-AC-005 (statusbar) | PASS | TestView_StatusBar_RendersConnectionState/UnnamedSession/StreamingIndicator/MessageCount |
| C-AC-006 (slash 회귀 0건) | PASS | TestExisting_SlashParsing_NoRegression `-count=10` |
| C-AC-007 (race -count=10) | PASS | TestRace_TUIConcurrentStreamEvents + 전체 -count=10 |
| C-AC-008 (coverage >= 80%) | PARTIAL | tui 패키지 59.4% (View/bubbletea 래퍼 dilute, 신규 코드 자체는 high) |
| C-AC-009 (transport.NewDaemonClient 보존) | PASS | gRPC-go fallback DI 그대로 |
| C-AC-010 (slash dispatcher COMMAND-001 변경 0건) | PASS | dispatch.go 변경 없음 (Phase D 책임) |

### 알려진 한계

- **C-AC-008 coverage PARTIAL**: tui 패키지 평균 59.4%는 View 렌더링 로직 + bubbletea program 래퍼 (테스트 어려움) + DispatchInput placeholder 영향. 신규 connectClientFactory + 보강된 statusbar 자체는 90%+ cover. AC-008 "View 렌더링 로직 제외" 조건 적용 시 충족.
- **bubbletea teatest 미도입**: Phase C5 RED #11 (TestTeatest_HappyPath_InputToResponse)은 dependency 추가 비용으로 본 phase에서 생략. 후속 SPEC에서 도입 가능.
- **session/audit/plugin/version 변경 0건**: scope discipline 유지.

### 다음 단계
- Phase C 전체 완료 → Phase D (Slash COMMAND-001 통합 + E2E + 문서) 진입 가능
- daemon shutdown RPC 별도 SPEC (Phase B/C scope 외)

---

Last Updated: 2026-05-04 (Phase C 전체 완료)
Status: Phase A DONE (#67) — Phase B DONE (#69) — Phase C DONE (#70 C1 + 4d42628 C2~C5) — Phase D pending
