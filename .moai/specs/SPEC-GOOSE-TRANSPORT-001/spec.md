---
id: SPEC-GOOSE-TRANSPORT-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 0
size: 중(M)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-TRANSPORT-001 — gRPC 서버/proto 스키마 기본 계약

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (ROADMAP Phase 0 row 03 + tech ADR-002) | manager-spec |

---

## 1. 개요 (Overview)

`goosed` 데몬이 외부 클라이언트(CLI, Desktop, Web)와 대화할 **gRPC 서버의 기본 계약과 proto 스키마 최소 집합**을 정의한다. CORE-001이 확보한 HTTP `/healthz`를 그대로 두되, 본 SPEC은 별도 포트에서 **gRPC listener**를 추가 바인딩한다.

수락 조건 통과 시점에서 grpcurl 또는 generated Go client가:

- `goose.v1.DaemonService/Ping` 호출 → `PingResponse{version, uptime_ms, state}` 회신
- `goose.v1.DaemonService/GetInfo` 호출 → 빌드 메타(commit, goVersion, buildTime) 회신
- `goose.v1.DaemonService/Shutdown` 호출 (with auth token) → daemon이 graceful shutdown 개시

본 SPEC은 **Agent/LLM/Tool RPC는 포함하지 않는다** (후속 SPEC). Daemon 수준 메타데이터와 lifecycle RPC만.

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
- Streaming RPC 예시(Shutdown은 일회성 unary).
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

**REQ-TR-003 [Ubiquitous]** — The proto package name **shall** be `goose.v1` and the Go package path **shall** be `github.com/gooseagent/goose/internal/transport/grpc/gen/goosev1`.

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

**REQ-TR-012 [Unwanted]** — The server **shall not** accept connections over plaintext HTTP/2 from non-loopback addresses; any connection attempt from a non-loopback peer while `GOOSE_GRPC_BIND=127.0.0.1` **shall** be rejected by the listener.

**REQ-TR-013 [Unwanted]** — The server **shall not** register any RPC handler before the `RecoveryInterceptor` is attached to the interceptor chain (compile-time wiring).

### 4.5 Optional

**REQ-TR-014 [Optional]** — **Where** `GOOSE_GRPC_MAX_RECV_MSG_BYTES` is set to a positive integer, the server **shall** apply that value as `grpc.MaxRecvMsgSize` instead of the default `4 MiB`.

---

## 5. 수용 기준 (Acceptance Criteria)

**AC-TR-001 — Ping RPC 정상 응답**
- **Given** `goosed` state=`serving`, gRPC server bound to random port (test harness)
- **When** 테스트 client가 `Ping(context.TODO(), &PingRequest{})` 호출
- **Then** `resp.Version != ""`, `resp.UptimeMs > 0`, `resp.State == "serving"` 응답 수신 (에러 nil)

**AC-TR-002 — Health Check**
- **Given** gRPC server 기동 + `grpc.health.v1.Health` 서비스 등록
- **When** `Check(ctx, &HealthCheckRequest{Service: "goose.v1.DaemonService"})`
- **Then** `Status == SERVING`

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

**AC-TR-008 — Non-loopback bind 거부**
- **Given** `GOOSE_GRPC_BIND=0.0.0.0`은 유효(명시적 opt-in), 그러나 기본값 `127.0.0.1`인 상태
- **When** 원격(또는 `localhost` 외) IP로 연결 시도
- **Then** OS 레벨 connection refused 또는 timeout (본 AC는 기본 바인드가 루프백 전용임을 listener 수준에서 보장)

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
option go_package = "github.com/gooseagent/goose/internal/transport/grpc/gen/goosev1;goosev1";

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

**End of SPEC-GOOSE-TRANSPORT-001**
