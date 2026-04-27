---
id: SPEC-GOOSE-TRANSPORT-001
version: 0.1.2
status: completed
completed: 2026-04-27
created_at: 2026-04-21
updated_at: 2026-04-27
author: manager-spec
priority: P0
issue_number: null
phase: 0
size: 중(M)
lifecycle: spec-anchored
labels: [phase-0, grpc, proto, transport, server, priority/p0-critical]
---

# SPEC-GOOSE-TRANSPORT-001 — gRPC 서버/proto 스키마 기본 계약

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (ROADMAP Phase 0 row 03 + tech ADR-002) | manager-spec |
| 0.1.1 | 2026-04-25 | 감사 리포트(mass-20260425/TRANSPORT-001-audit.md) 결함 수정: (a) §1 scope-clarity 문장 추가(D8), (b) §3.2 streaming BRIDGE-001 위임 명시(D8), (c) REQ-TR-001 vs REQ-TR-012 모순 일관화(D5), (d) REQ-TR-012/013 [Unwanted] 라벨 If/then 구조 수정(D6), (e) 고아 REQ 6건(REQ-TR-002/003/007/011/013/014)에 AC-TR-009~014 신설(D3), (f) AC-TR-002에 REQ-TR-015 추가(D4). REQ 번호 재배치 없음. | manager-spec |
| 0.1.2 | 2026-04-26 | 구현 직전 sanity check: 실제 go.mod module path가 `github.com/modu-ai/goose`임이 확인되어, REQ-TR-003 / §6.2 proto 스키마 / AC-TR-010 세 곳의 Go 패키지 경로 레퍼런스를 (구) `github.com/gooseagent/goose/...` → (신) `github.com/modu-ai/goose/...`로 정정. proto package 이름(`goose.v1`) 및 그 외 SPEC 의미는 변경 없음. AC 번호 재배치 없음. | claude(orchestrator) |

---

## 1. 개요 (Overview)

`goosed` 데몬이 외부 클라이언트(CLI, Desktop, Web)와 대화할 **gRPC 서버의 기본 계약과 proto 스키마 최소 집합**을 정의한다. CORE-001이 확보한 HTTP `/healthz`를 그대로 두되, 본 SPEC은 별도 포트에서 **gRPC listener**를 추가 바인딩한다.

수락 조건 통과 시점에서 grpcurl 또는 generated Go client가:

- `goose.v1.DaemonService/Ping` 호출 → `PingResponse{version, uptime_ms, state}` 회신
- `goose.v1.DaemonService/GetInfo` 호출 → 빌드 메타(commit, goVersion, buildTime) 회신
- `goose.v1.DaemonService/Shutdown` 호출 (with auth token) → daemon이 graceful shutdown 개시

본 SPEC은 **Agent/LLM/Tool RPC는 포함하지 않는다** (후속 SPEC). Daemon 수준 메타데이터와 lifecycle RPC만.

### 1.1 Scope Clarity (v0.1.1 추가)

본 SPEC의 이름은 "TRANSPORT-001"이지만, 실제 계약 범위는 **`goosed` daemon의 meta-RPC unary 3종(`Ping` / `GetInfo` / `Shutdown`)에 한정**된다. Transport 계층의 다른 측면은 각 후속 SPEC에서 정의된다:

- **Streaming transport (WebSocket/SSE/HTTP 포함)** → `SPEC-GOOSE-BRIDGE-001` (localhost Web UI bridge).
- **QUERY-001 `<-chan SDKMessage` stream 변환 계약** → BRIDGE-001에서 처리 (본 SPEC은 unary-only).
- **Agent / LLM / Tool RPC 정의** → 각각 `SPEC-GOOSE-AGENT-001`, `SPEC-GOOSE-LLM-001`, `SPEC-GOOSE-TOOL-001`.
- **TLS / mTLS / 고급 인증** → Phase 5+ 후속 SPEC.

본 SPEC을 "transport 계층 전체 authoritative 문서"로 해석해서는 안 된다. 본 SPEC이 다루는 것은 daemon meta-RPC unary 계약의 기본 바닥판이다.

---

## 2. 배경 (Background)

### 2.1 왜 지금

- ROADMAP §4 Phase 0 row 03은 `TRANSPORT-001`을 CORE-001 의존 후속으로 명시. tech ADR-002가 근거("gRPC로 모든 언어 경계 통신").
- CLI-001(goose CLI)은 gRPC로 `goosed`에 접속. LLM-001 역시 daemon 내부에서 동작하므로 전송을 직접 쓰지 않지만, 향후 MCP 서버 노출 또는 외부 세션 시 gRPC 경유 필요.
- proto 스키마가 없으면 CLI와 daemon이 ad-hoc JSON으로 붙게 되어 tech.md §3.3(proto 표준)의 원칙 위배.

### 2.2 범위 경계

- **IN**: `proto/goose/v1/daemon.proto`의 최소 `DaemonService`, `grpc-go` 서버 초기화, reflection 지원(개발자 편의), interceptor 체인(logging + recovery), insecure TCP(localhost 전제), graceful stop 시 CORE-001 shutdown hook과 연동.
- **OUT**: Agent/LLM/Tool RPC 정의, TLS/mTLS, 인증(Token 헤더 기본 스텁만), gRPC-Web, gRPC gateway(HTTP JSON), streaming RPC 구체 구현(Shutdown 외), client SDK publish.

### 2.3 CORE-001과의 관계

- CORE-001이 기동한 동일 `context.Context` 트리에 gRPC server가 등록된 cleanup hook으로 붙는다.
- gRPC listener는 **별도 포트** (기본 `:17891`, ENV `GOOSE_GRPC_PORT`).
- CORE-001의 HTTP health server는 그대로 유지. gRPC server 내부에서도 health checking protocol(`grpc.health.v1`)을 구현하여 gRPC 클라이언트가 `Check("goose.v1.DaemonService")`로 상태 조회 가능.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `proto/goose/v1/daemon.proto` 파일 생성 (package `goose.v1`).
2. 3개 RPC: `Ping`, `GetInfo`, `Shutdown`.
3. `internal/transport/grpc/server.go`에서 `*grpc.Server` 초기화, listener 바인드, 등록된 cleanup hook을 통해 `GracefulStop` 호출.
4. Interceptor 2종: `LoggingInterceptor`(zap 기반 access log), `RecoveryInterceptor`(panic → `codes.Internal` + stack log).
5. `grpc.health.v1.Health` 서비스 등록: `Check` + `Watch` 기본 구현.
6. `grpc.reflection.v1alpha` reflection 서비스 등록 (DEBUG/개발 빌드에서만; `GOOSE_GRPC_REFLECTION=true` env로 제어).
7. Shutdown RPC 인증: 초기 단순 static token(ENV `GOOSE_SHUTDOWN_TOKEN`). 없으면 Shutdown RPC 비활성.
8. `buf.yaml` + `buf.gen.yaml`로 proto → Go 코드 생성 파이프라인(`buf generate`). 생성물은 `internal/transport/grpc/gen/`.
9. 포트 충돌 시 CORE-001과 동일한 exit code 78 (EX_CONFIG) 계약.

### 3.2 OUT OF SCOPE

- TLS/mTLS 구성 (후속 SPEC, 아마 Phase 5+).
- 실제 `AgentService`, `LLMService`, `ToolService` RPC 정의 — 각각 AGENT-001, LLM-001, TOOL-001.
- gRPC-Web / Connect 변환 레이어 (CLI-001이 필요 시 별도 SPEC).
- **Streaming transport는 본 SPEC의 범위가 아니며 `SPEC-GOOSE-BRIDGE-001`(localhost Web UI bridge)이 담당한다. 여기에는 server-streaming / client-streaming / bidi-streaming gRPC, WebSocket, SSE, HTTP long-polling, 그리고 QUERY-001의 `<-chan SDKMessage` stream 변환 계약이 모두 포함된다. 본 SPEC은 unary RPC(Ping / GetInfo / Shutdown) 3종만 정의한다.** (streaming is handled by BRIDGE-001)
- client library 퍼블리싱(`pkg/`).
- Observability: OpenTelemetry tracing interceptor — Phase 5+.
- Rate limiting / quota interceptor.
- Cross-origin / CORS (gRPC는 HTTP/2이므로 부적용, gRPC-Web일 때만 이슈).
- Windows named pipe / Unix socket 변형 — TCP 전용.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous

**REQ-TR-001 [Ubiquitous]** — The gRPC server **shall** bind to `127.0.0.1` by default and only allow loopback connections unless `GOOSE_GRPC_BIND` is explicitly set to a non-loopback interface.

**REQ-TR-002 [Ubiquitous]** — All gRPC requests **shall** pass through the `LoggingInterceptor`, which records `{method, peer, status_code, duration_ms}` at INFO level for success and ERROR level for non-OK responses.

**REQ-TR-003 [Ubiquitous]** — The proto package name **shall** be `goose.v1` and the Go package path **shall** be `github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1`.

### 4.2 Event-Driven

**REQ-TR-004 [Event-Driven]** — **When** an RPC handler panics, the `RecoveryInterceptor` **shall** recover, log the panic with full stack trace, and return `status.Error(codes.Internal, "internal error")` to the client without leaking stack content over the wire.

**REQ-TR-005 [Event-Driven]** — **When** a client invokes `DaemonService/Ping`, the server **shall** respond with `PingResponse{version, uptime_ms, state}` where `state` matches the CORE-001 process state enum values (`init|bootstrap|serving|draining|stopped`).

**REQ-TR-006 [Event-Driven]** — **When** a client invokes `DaemonService/Shutdown` with a valid `auth_token` header matching `GOOSE_SHUTDOWN_TOKEN`, the server **shall** reply `ShutdownResponse{accepted: true}` and then trigger the CORE-001 root context cancellation within 100ms after response is flushed.

**REQ-TR-007 [Event-Driven]** — **When** the CORE-001 shutdown hook fires, the gRPC server **shall** call `GracefulStop()` and complete within 10s; if not completed, it **shall** call `Stop()` and log a WARN.

### 4.3 State-Driven

**REQ-TR-008 [State-Driven]** — **While** the process state is `draining`, new RPC invocations (except `Ping`) **shall** return `codes.Unavailable` with message `"daemon draining"`.

**REQ-TR-009 [State-Driven]** — **While** `GOOSE_GRPC_REFLECTION` is unset or `false`, the reflection service **shall not** be registered on the server.

### 4.4 Unwanted Behavior

**REQ-TR-010 [Unwanted]** — **If** `Shutdown` is invoked without a valid `auth_token` header, **then** the server **shall** return `codes.Unauthenticated` with message `"missing or invalid shutdown token"` and **shall not** initiate shutdown.

**REQ-TR-011 [Unwanted]** — **If** `GOOSE_SHUTDOWN_TOKEN` is empty or unset, **then** the server **shall** register `Shutdown` RPC as disabled (returning `codes.Unimplemented`) — never accept shutdown via RPC in that mode.

**REQ-TR-012 [Unwanted]** — **If** `GOOSE_GRPC_BIND` is unset or set to `127.0.0.1` and a connection attempt originates from a non-loopback peer, **then** the listener **shall** reject the connection before it reaches any RPC handler. **Note (v0.1.1)**: When `GOOSE_GRPC_BIND` is explicitly set to a non-loopback interface (e.g., `0.0.0.0`), this REQ does not apply — that opt-in case is governed by REQ-TR-001 and is the operator's responsibility. This resolves the prior contradiction between REQ-TR-001 (explicit non-loopback opt-in permitted) and REQ-TR-012 v0.1.0 (which read as an absolute prohibition).

**REQ-TR-013 [Unwanted]** — **If** the interceptor chain is constructed at server initialization time **without** `RecoveryInterceptor` attached as the outermost unary interceptor, **then** the build **shall** fail at compile time (or server startup **shall** abort with `codes.FailedPrecondition`). In other words, no RPC handler may be registered before `RecoveryInterceptor` is in place.

### 4.5 Optional

**REQ-TR-014 [Optional]** — **Where** `GOOSE_GRPC_MAX_RECV_MSG_BYTES` is set to a positive integer, the server **shall** apply that value as `grpc.MaxRecvMsgSize` instead of the default `4 MiB`.

### 4.6 Health Check Service (v0.1.1 추가)

**REQ-TR-015 [State-Driven]** — **While** the process state is `serving`, the server **shall** register the `grpc.health.v1.Health` service and return `Status=SERVING` when queried with `Check(HealthCheckRequest{Service: "goose.v1.DaemonService"})`. **While** the process state is `draining` or `stopped`, the same query **shall** return `Status=NOT_SERVING`. This REQ backs AC-TR-002 (health check contract), resolving the prior orphan-AC defect (audit D4).

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-TR-001 — Ping RPC 정상 응답**
- **Given** `goosed` state=`serving`, gRPC server bound to random port (test harness)
- **When** 테스트 client가 `Ping(context.TODO(), &PingRequest{})` 호출
- **Then** `resp.Version != ""`, `resp.UptimeMs > 0`, `resp.State == "serving"` 응답 수신 (에러 nil)

**AC-TR-002 — Health Check (REQ-TR-015 커버)**
- **Given** gRPC server 기동(process state=`serving`) + `grpc.health.v1.Health` 서비스 등록
- **When** `Check(ctx, &HealthCheckRequest{Service: "goose.v1.DaemonService"})`
- **Then** `Status == SERVING`. 추가로 process state를 `draining`으로 강제 전이한 뒤 동일 호출 시 `Status == NOT_SERVING`이 반환되어 REQ-TR-015의 state-conditioned 계약이 관찰 가능함을 확인.

**AC-TR-003 — Shutdown 토큰 없이 거부**
- **Given** `GOOSE_SHUTDOWN_TOKEN=secret`, client metadata에 `auth_token` 헤더 미포함
- **When** `Shutdown(ctx, &ShutdownRequest{})`
- **Then** `status.Code(err) == codes.Unauthenticated`, daemon은 계속 serving 상태

**AC-TR-004 — Shutdown 토큰 포함 시 종료 개시**
- **Given** `GOOSE_SHUTDOWN_TOKEN=secret`, client metadata `auth_token=secret`
- **When** `Shutdown(ctx, &ShutdownRequest{})`
- **Then** 응답 `accepted=true` 수신 후 500ms 이내 daemon process exit 0

**AC-TR-005 — Draining 중 Unavailable**
- **Given** daemon state=`draining` (cleanup 진행 중 테스트 훅으로 고정)
- **When** `GetInfo(ctx, ...)`
- **Then** `status.Code(err) == codes.Unavailable`, message 포함 `"daemon draining"`

**AC-TR-006 — Panic 복구**
- **Given** 테스트용 RPC `PanicTest`가 등록되어 `panic("boom")`을 발생 (integration test only)
- **When** client가 `PanicTest` 호출
- **Then** `status.Code(err) == codes.Internal`, 프로세스는 crash 없이 계속 serving, stderr에 panic stack trace 로그

**AC-TR-007 — Reflection off by default**
- **Given** `GOOSE_GRPC_REFLECTION` 미설정
- **When** `grpcurl -plaintext localhost:17891 list`
- **Then** `unknown service grpc.reflection.v1alpha.ServerReflection` 에러

**AC-TR-008 — Non-loopback bind 거부 (REQ-TR-012 커버)**
- **Given** `GOOSE_GRPC_BIND` 미설정(기본값 `127.0.0.1`)인 상태로 daemon 기동 + CI 플랫폼(linux/amd64 단일)에서만 실행
- **When** 동일 호스트의 non-loopback 인터페이스(예: docker bridge IP) 주소로 gRPC `Ping` 연결 시도
- **Then** listener 수준에서 연결이 거부된다(해당 플랫폼에서 `ECONNREFUSED`). `GOOSE_GRPC_BIND=0.0.0.0`으로 명시적 opt-in한 별도 케이스는 동일 요청이 성공함을 대조로 확인.

**AC-TR-009 — LoggingInterceptor 필드 기록 (REQ-TR-002 커버, v0.1.1 추가)**
- **Given** zap logger가 test observer sink로 주입된 gRPC 서버 + 테스트 client
- **When** (a) `Ping` 정상 호출 1회, (b) `Shutdown` 토큰 누락으로 실패 호출 1회 수행
- **Then** 로그 observer에 정확히 2개의 entry가 기록되며, 각 entry는 `method`, `peer`, `status_code`, `duration_ms` 4개 필드를 모두 포함한다. (a) 케이스는 INFO 레벨 + `status_code="OK"`, (b) 케이스는 ERROR 레벨 + `status_code="Unauthenticated"`로 기록됨을 assertion.

**AC-TR-010 — proto 패키지 및 Go 패키지 경로 (REQ-TR-003 커버, v0.1.1 추가)**
- **Given** `buf generate` 결과로 생성된 `internal/transport/grpc/gen/goosev1/` 디렉토리
- **When** 생성된 `.pb.go` 파일을 static 검사
- **Then** (a) proto `package` 선언이 `goose.v1`이고, (b) `option go_package`가 `github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1;goosev1`이며, (c) Go 패키지 `import` 경로가 동일한 경로에서 resolve된다. 컴파일 단계(테스트: `go vet ./internal/transport/grpc/gen/...`)에서 경로 불일치가 없어야 한다.

**AC-TR-011 — GracefulStop 10s 준수 및 fallback (REQ-TR-007 커버, v0.1.1 추가)**
- **Given** 테스트용 cleanup hook 2종 등록: (a) 200ms 내 완료되는 정상 hook, (b) 30s sleep하는 stuck hook (명시적 가짜 timeout 조건)
- **When** (a) 정상 hook 시나리오: CORE-001 shutdown hook 발사 → `GracefulStop()` 경로 검증. (b) stuck hook 시나리오: shutdown hook 발사 → 10s 경과 후 `Stop()` fallback 경로 검증.
- **Then** (a)는 wall clock 기준 10s 이내에 `GracefulStop` 리턴, WARN 로그 없음. (b)는 10s ± 500ms 시점에 `Stop()`이 호출되고 zap logger에 `"grpc server stop fallback after graceful timeout"` 류의 WARN 레벨 entry가 1개 기록.

**AC-TR-012 — Shutdown 토큰 미설정 시 Unimplemented (REQ-TR-011 커버, v0.1.1 추가)**
- **Given** `GOOSE_SHUTDOWN_TOKEN` 환경변수가 빈 문자열 또는 unset 상태로 daemon 기동
- **When** 클라이언트가 `Shutdown(ctx, &ShutdownRequest{})`를 임의 metadata로 호출
- **Then** `status.Code(err) == codes.Unimplemented`가 반환되고, daemon은 종료 동작 없이 계속 `serving` 상태를 유지한다. AC-TR-003(토큰 설정됨 + 헤더 누락 → `Unauthenticated`)과는 의도적으로 다른 코드 경로임을 확인.

**AC-TR-013 — RecoveryInterceptor 체인 배치 검증 (REQ-TR-013 커버, v0.1.1 추가)**
- **Given** `NewServer(cfg, logger, stateAccessor)`가 반환하는 `*grpc.Server` 초기화 설정
- **When** 테이블 드리븐 테스트가 서버 초기화 시 등록된 `grpc.ServerOption` 슬라이스(특히 unary interceptor 체인)를 reflection/인터페이스 검사로 확인
- **Then** `RecoveryInterceptor`가 chain의 outermost(인덱스 0)에 위치하며, 그 뒤로 `LoggingInterceptor` 순서로 등록되어 있음. 인터셉터가 누락되거나 순서가 뒤바뀌면 테스트 실패. 또는 서버 초기화가 `codes.FailedPrecondition`으로 abort함을 대체 assertion으로 허용.

**AC-TR-014 — MaxRecvMsgSize 환경변수 override (REQ-TR-014 커버, v0.1.1 추가)**
- **Given** `GOOSE_GRPC_MAX_RECV_MSG_BYTES=1024` 환경변수 설정 상태로 daemon 기동 (기본값 `4 MiB`를 1024 byte로 override)
- **When** 클라이언트가 2048 byte 페이로드(예: `GetInfoRequest`를 패딩으로 확장한 테스트 variant)로 RPC 호출
- **Then** `status.Code(err) == codes.ResourceExhausted` 반환, 메시지에 "received message larger than max" 문구 포함. 같은 테스트를 `GOOSE_GRPC_MAX_RECV_MSG_BYTES` 미설정 상태에서 수행하면 성공 응답이 반환되어 default 4MiB 동작과의 분기 확인.

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 패키지 레이아웃

```
proto/goose/v1/
├── daemon.proto                # service DaemonService { Ping, GetInfo, Shutdown }
└── common.proto                # common messages (Empty, Status, etc. — 필요 시)

internal/transport/grpc/
├── server.go                   # NewServer(cfg, logger, stateAccessor) *grpc.Server
├── interceptors.go             # Logging, Recovery
├── daemon_service.go           # implements DaemonServiceServer
├── shutdown_auth.go            # token extraction from metadata
├── gen/goosev1/                # buf-generated *.pb.go, *_grpc.pb.go (do not edit)
└── *_test.go

buf.yaml                        # buf 모듈 설정
buf.gen.yaml                    # protoc-gen-go + protoc-gen-go-grpc
```

### 6.2 proto 스키마 (초안)

```proto
syntax = "proto3";
package goose.v1;
option go_package = "github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1;goosev1";

service DaemonService {
  rpc Ping(PingRequest) returns (PingResponse);
  rpc GetInfo(GetInfoRequest) returns (GetInfoResponse);
  rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);
}

message PingRequest {}
message PingResponse {
  string version   = 1;
  int64  uptime_ms = 2;
  string state     = 3;  // matches CORE-001 ProcessState.String()
}

message GetInfoRequest {}
message GetInfoResponse {
  string version     = 1;
  string git_commit  = 2;
  string go_version  = 3;
  string build_time  = 4;  // ISO-8601
  string os          = 5;  // runtime.GOOS
  string arch        = 6;  // runtime.GOARCH
}

message ShutdownRequest {
  string reason = 1;  // optional audit trail
}
message ShutdownResponse {
  bool   accepted = 1;
  string message  = 2;
}
```

### 6.3 의존성

| 라이브러리 | 버전 | 용도 |
|----------|------|-----|
| `google.golang.org/grpc` | v1.66+ | gRPC 서버 |
| `google.golang.org/protobuf` | v1.34+ | protobuf runtime |
| `github.com/bufbuild/buf` (개발 도구) | 최신 | `buf generate`, `buf lint`, `buf breaking` |
| `google.golang.org/grpc/health` | (stdlib 경로) | `grpc.health.v1` 구현 |
| `google.golang.org/grpc/reflection` | (stdlib 경로) | reflection (dev only) |

### 6.4 생성 파이프라인

개발자 워크플로우:

```
$ buf lint          # lint proto
$ buf breaking --against '.git#branch=main'
$ buf generate      # writes internal/transport/grpc/gen/goosev1/
```

CI에서는 `buf lint` + `buf breaking` + `go test`. 생성된 `gen/` 디렉토리는 `.gitignore`에 넣지 않고 commit(오프라인 빌드 허용).

### 6.5 CORE-001 연동

```
bootstrap():
  core.RegisterHook("grpc-server", func(ctx) error {
      return grpcServer.GracefulStop() // wrapper with timeout
  })
  go grpcServer.Serve(lis)
```

- listener 획득 실패(포트 충돌) 시 CORE-001의 REQ-CORE-006과 동일 경로로 exit 78.
- `GracefulStop`에 10s timeout 래퍼를 두어 hook timeout(CORE-001 §6.2의 per-hook 기본 10s)과 정확히 일치.

### 6.6 Interceptor 체인 순서

```
[incoming RPC]
  -> RecoveryInterceptor (outer: panic 차단)
    -> LoggingInterceptor
      -> auth check (Shutdown만, handler 내부)
        -> handler
```

Recovery가 outer에 있어야 logging이 panic으로 누락되지 않고 logging 이후의 어떤 panic도 잡힌다.

### 6.7 TDD 진입

1. **RED**: `TestPingRPC_ReturnsVersionAndState` → AC-TR-001 실패.
2. **RED**: `TestHealthCheck_ServiceServing` → AC-TR-002 실패.
3. **RED**: `TestShutdownWithoutToken_Unauth` → AC-TR-003 실패.
4. **RED**: `TestShutdownWithToken_ExitsZero` → AC-TR-004 실패.
5. **RED**: `TestPanicHandler_Recovered` → AC-TR-006 실패.
6. **GREEN**: `server.go`, `interceptors.go`, `daemon_service.go` 최소 구현.
7. **REFACTOR**: interceptor ordering 명문화, shutdown auth를 별도 함수로 분리.

### 6.8 TRUST 5 매핑

| 차원 | 달성 |
|-----|-----|
| Tested | unit(interceptor 개별) + integration(실 listener + client) + concurrent Ping race test |
| Readable | proto 파일 주석 필수, 각 RPC에 대한 `// When ... shall ...` 형식 doc-comment |
| Unified | `buf lint` (naming, services) + `golangci-lint` |
| Secured | default bind 127.0.0.1, shutdown 토큰, reflection off-by-default, recovery 필수 |
| Trackable | LoggingInterceptor 표준 필드(method/status/duration), proto 버전 `v1` |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | **SPEC-GOOSE-CORE-001** | shutdown hook, state accessor, zap logger |
| 선행 SPEC | **SPEC-GOOSE-CONFIG-001** | `TransportConfig.GRPCPort` 소비 |
| 후속 SPEC | SPEC-GOOSE-LLM-001 | LLM RPC 추가 정의 |
| 후속 SPEC | SPEC-GOOSE-AGENT-001 | Agent RPC 추가 정의 |
| 후속 SPEC | SPEC-GOOSE-CLI-001 | generated Go client 소비 |
| 외부 | `grpc-go` v1.66+ | 서버 구현 |
| 외부 | `bufbuild/buf` | codegen 파이프라인 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | proto 변경이 파괴적(breaking)으로 발전 | 중 | 높 | `buf breaking` CI gate, `goose.v1` 패키지 불변, v2 필요 시 신규 디렉토리 |
| R2 | generated 코드를 레포에 commit할지 여부 논쟁 | 중 | 낮 | 본 SPEC은 commit 유지(오프라인 빌드). `make proto` 스크립트로 재생성 간편 |
| R3 | Shutdown RPC 남용(DoS) | 낮 | 중 | 토큰 필수 + rate limiter는 후속 SPEC |
| R4 | reflection on in production | 중 | 중 | 환경변수 opt-in, CI smoke test로 default off 확인 |
| R5 | `GracefulStop` 무한대기 (client hang) | 중 | 중 | 10s timeout 래퍼, 초과 시 `Stop()` fallback |
| R6 | `buf` toolchain 빌드 환경 의존 | 중 | 낮 | `tools/tools.go` blank import로 Go 도구체인 고정, `make bootstrap`으로 설치 자동화 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서

- `.moai/project/tech.md` §3.3 (protobuf, grpc-go), ADR-002 (gRPC 선택)
- `.moai/project/structure.md` §1 `proto/`, `internal/transport/`, §8 (gRPC 경계)
- `.moai/specs/SPEC-GOOSE-CORE-001/spec.md` §3.2 OUT OF SCOPE (gRPC는 본 SPEC)

### 9.2 외부 참조

- grpc-go interceptor 문서: https://pkg.go.dev/google.golang.org/grpc
- `grpc.health.v1` 스펙: https://github.com/grpc/grpc/blob/master/doc/health-checking.md
- Buf style guide: https://buf.build/docs/lint/rules/

### 9.3 부속 문서

- `./research.md`
- `../ROADMAP.md` §4 Phase 0 row 03

---

## Exclusions (What NOT to Build)

- 본 SPEC은 **TLS/mTLS를 구현하지 않는다**. 모든 연결은 H2C(plaintext HTTP/2) + 루프백.
- 본 SPEC은 **Agent/LLM/Tool RPC를 정의하지 않는다**. 각 후속 SPEC의 책임.
- 본 SPEC은 **gRPC-Web / Connect / HTTP/JSON gateway를 제공하지 않는다**.
- 본 SPEC은 **streaming RPC를 구현하지 않는다**(Shutdown 포함, 전부 unary).
- 본 SPEC은 **OpenTelemetry tracing / metric exporter를 포함하지 않는다**.
- 본 SPEC은 **client library를 `pkg/`에 공개하지 않는다**. 내부 `internal/` 전용.
- 본 SPEC은 **rate limiting / quota interceptor를 포함하지 않는다**.
- 본 SPEC은 **Unix socket / Windows named pipe 변형을 지원하지 않는다**. TCP-only.
- 본 SPEC은 **인증을 완전 구현하지 않는다**. Shutdown 토큰은 단순 static string 비교. 실제 인증은 후속 SPEC.

---

## Implementation Notes (sync 정합화 2026-04-27)

- **Status Transition**: planned → implemented
- **Package**: `internal/transport/grpc/` (7 파일) + `internal/transport/grpc/gen/goosev1/` (proto 생성물)
- **Core**: `server.go` (10KB, `grpc.NewServer` + health service `goose.v1.DaemonService`), `daemon_service.go`(unary 3종 RPC), `interceptors.go`, `shutdown_auth.go`, `panic_test_service.go`
- **Generated**: `gen/goosev1/daemon.pb.go` + `daemon_grpc.pb.go` — proto package `goose.v1`, module path `github.com/modu-ai/goose` (v0.1.2 정정과 일관)
- **Verified REQs (spot-check)**: REQ-TR-001/012 gRPC listener 일관, `goose.v1.DaemonService` 등록, modu-ai 패키지 path. BRIDGE-001 streaming 위임은 본 SPEC 범위 외
- **Test Coverage**: `server_test.go` (18KB) — health/serving status/shutdown auth 검증
- **Lifecycle**: spec-anchored Level 2 — Shutdown 인증은 단순 static string, 후속 SPEC에서 강화 예정

---

**End of SPEC-GOOSE-TRANSPORT-001**
