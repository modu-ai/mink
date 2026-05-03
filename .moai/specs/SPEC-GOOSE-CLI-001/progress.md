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

### 신규 ConnectClient 메서드 부족분 점검

Phase A `ConnectClient` 7 메서드 vs Phase B 요구:

| Phase B 요구 | ConnectClient 메서드 | 상태 |
|--------------|----------------------|------|
| ping | `Ping(ctx)` | ✅ 존재 |
| ask | `ChatStream(ctx, []Message)` | ✅ 존재 |
| config get | `GetConfig(ctx, key)` | ✅ 존재 |
| config set | `SetConfig(ctx, k, v)` | ✅ 존재 |
| config list | `ListConfig(ctx, prefix)` | ✅ 존재 |
| tool list | `ListTools(ctx)` | ✅ 존재 |
| daemon status | `Ping(ctx)` (재사용) | ✅ 존재 |
| daemon shutdown | (없음) | ❌ Phase B scope 외 — stub 유지 |

**결론**: Phase B는 ConnectClient API 변경 0건. proto 변경 0건. 순수 wiring 작업.

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

Last Updated: 2026-05-04 (Phase B Plan 작성)
Status: Phase A DONE (PR #67) — Phase B PLAN READY — Phase C/D pending
