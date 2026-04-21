# SPEC-GENIE-TRANSPORT-001 — Research & Inheritance Analysis

> **목적**: gRPC 서버와 proto 스키마 기본 계약 구현을 위한 자산 조사.
> **작성일**: 2026-04-21

---

## 1. 레포 상태 재확인

- `proto/`, `internal/transport/`, `internal/transport/grpc/` 부재 (`ls` 확인).
- `cmd/genied/main.go` 부재(CORE-001 GREEN 이후 생김).
- `buf.yaml`, `buf.gen.yaml` 없음.

Go proto/gRPC 자산 0. **신규 작성**.

---

## 2. 참조 자산별 분석

### 2.1 Claude Code TypeScript Bridge (`./claude-code-source-map/bridge/`)

Claude Code `bridge/` 디렉토리 33개 파일 탐색:

- `bridgeMain.ts`, `bridgeMessaging.ts`, `bridgeConfig.ts` 등.
- **Transport는 ndjson over stdin/stdout + WebSocket**으로, gRPC 아님.
- Proto 스키마 없음. JSON 메시지로 직접 송수신.

**계승 대상**: 없음. GENIE는 proto 퍼스트.

### 2.2 Hermes Agent (`./hermes-agent-main/`)

- Python asyncio 서버가 `uvicorn`으로 HTTP REST 제공(cli.py 관찰 기반).
- gRPC 또는 proto 경험 없음.

**계승 대상**: 없음.

### 2.3 MoAI-ADK-Go (외부 레포, 미러 없음)

- `structure.md` §1은 `proto/` 디렉토리를 언급하나 본 레포에 없음.
- tech ADR-002는 "모든 언어 경계는 gRPC"를 원칙으로 선언.
- 본 SPEC은 MoAI-ADK-Go의 proto 샘플 없이 독자 스키마로 시작. 미래에 외부 레포가 공개되면 `proto/moai/`를 그대로 받아 오는 정도의 계승이 가능.

---

## 3. gRPC 생태계 선택

### 3.1 grpc-go vs connect-go

| 축 | `grpc-go` | `connect-go` |
|----|----------|-------------|
| proto 호환 | ✅ 표준 | ✅ |
| HTTP/2 only | 기본 gRPC, H2C 지원 | HTTP/1.1 + HTTP/2 모두 |
| gRPC-Web | 별도 envoy/wrap 필요 | native |
| 생태계 | 성숙 (health, reflection 공식) | 신진 (Buf 주도) |
| MoAI 호환 | 모든 MoAI 샘플이 grpc-go 가정(ADR-002) | 추가 정당화 필요 |

**선택**: `grpc-go`. 이유:
- tech.md §3.3이 `google.golang.org/grpc` 명시.
- CLI-001 골격은 Phase 0에 Go CLI이므로 gRPC-Web 불필요(브라우저 없음).
- `grpc.health.v1`, `grpc.reflection.v1alpha` 공식 구현 바로 사용.

TypeScript 클라이언트(Phase 5+ genie-web)는 Connect-ES를 사용해 **동일 proto**를 재활용 가능. `connect-go`는 Phase 5+에서 재평가.

### 3.2 Buf vs protoc

| 축 | `protoc` | `buf` |
|----|---------|------|
| 설치 | 별도 binary + plugin 경로 관리 | 단일 binary |
| lint | 없음(별도 툴) | `buf lint` 내장 |
| breaking change 감지 | 없음 | `buf breaking` 내장 |
| 구성 | Makefile/bash | `buf.yaml` 선언 |
| MoAI 친화 | 범용 | 현대적 관행 |

**선택**: `buf`. 이유:
- `buf lint` 규칙이 RPC naming과 service 패키지 구조를 강제 → 후속 SPEC들이 일관된 proto 스타일 유지.
- `buf breaking` CI gate로 R1(리스크: 파괴적 변경) 자동 방지.
- `buf.gen.yaml`로 Go 코드 생성 플러그인(`protoc-gen-go`, `protoc-gen-go-grpc`) 선언 관리.

---

## 4. Go 이디엄

### 4.1 Listener 획득 후 Serve

```
lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Bind, cfg.GRPCPort))
if err != nil { return ErrPortInUse{Port: cfg.GRPCPort} }  // CORE-001 REQ-CORE-006과 동형
go srv.Serve(lis)
```

- listener를 핸드오프 받은 뒤 `Serve`를 별도 goroutine에서 실행.
- `Serve`는 `GracefulStop` 시 리턴.

### 4.2 Interceptor 체인

```
opts := []grpc.ServerOption{
    grpc.ChainUnaryInterceptor(
        RecoveryInterceptor(logger),
        LoggingInterceptor(logger),
    ),
}
```

- `grpc.ChainUnaryInterceptor`가 outer → inner 순서로 호출. Recovery를 제일 바깥에 두어 모든 handler panic을 포획.
- Stream interceptor는 본 SPEC 범위 밖(streaming RPC 없음).

### 4.3 State accessor

CORE-001의 `atomic.Value` state를 daemon service가 직접 참조해야 함. 순환 import 방지를 위해 `internal/core` → `internal/transport/grpc`로 **interface 주입**:

```go
type DaemonStateReader interface {
    State() string
    StartTime() time.Time
    Version() string
}
```

`transport/grpc/daemon_service.go`는 `DaemonStateReader`에만 의존. CORE-001이 구현체 제공.

### 4.4 Shutdown RPC 트리거 방식

- RPC handler가 direct로 `cancelFunc()` 호출하면 응답이 client 도달 전 connection drop 가능.
- **패턴**: 응답을 반환한 후, `time.AfterFunc(50*time.Millisecond, cancelFunc)`로 지연 cancel. 100ms 이내 보장(REQ-TR-006).

### 4.5 `buf generate` output 디렉토리

생성물 위치 논쟁:
- 옵션 A: `internal/transport/grpc/gen/geniev1/`
- 옵션 B: `pkg/proto/geniev1/` (public)

**선택 A**. Phase 0은 internal 전용. 외부 client SDK 공개는 Phase 5+.

---

## 5. 외부 의존성 합계

| 모듈 | 용도 | 비고 |
|------|------|-----|
| `google.golang.org/grpc` v1.66+ | 서버 | tech.md §3.3 |
| `google.golang.org/protobuf` v1.34+ | runtime | grpc-go 의존 |
| `google.golang.org/grpc/health` | health check 서비스 | stdlib 경로 |
| `google.golang.org/grpc/reflection` | reflection (dev) | stdlib 경로 |
| `github.com/bufbuild/buf` | codegen/lint (개발 도구만, runtime 의존 없음) | go:build tool 또는 독립 binary |
| `github.com/stretchr/testify` | 테스트 | 기존 |
| 본 레포 `go.uber.org/zap` | logging interceptor | CORE-001 공유 |

**의도적 미사용**:
- `grpc-gateway` (HTTP JSON 변환) — 범위 밖.
- `connect-go` — Phase 0 대체 정당화 부족.
- `grpc-middleware/recovery` / `grpc-middleware/logging` — 간단한 interceptor라 자체 구현으로 충분. 외부 의존 최소화.

---

## 6. 테스트 전략

### 6.1 Unit (예상 10~15)

- `TestPingHandler_FillsVersionAndState` — handler 단위.
- `TestLoggingInterceptor_LogsMethodAndStatus` — interceptor 격리.
- `TestRecoveryInterceptor_ConvertsPanicToInternal` — panic 유발 handler로 검증.
- `TestShutdownAuth_MissingToken_Unauth`.
- `TestShutdownAuth_WrongToken_Unauth`.
- `TestShutdownAuth_CorrectToken_OK`.
- `TestReflectionOff_ListFails`.
- `TestReflectionOn_ListIncludesDaemonService`.

### 6.2 Integration

- `TestGRPCServer_BootstrapsAndPing` — actual listener + client round trip.
- `TestGRPCServer_ShutdownViaRPC_ProcessExitZero` — subprocess mode (go test invokes compiled binary).
- `TestGRPCServer_PortInUse_Exit78` — CORE-001과 동일 경로 검증.
- `TestGRPCServer_GracefulStopUnder10s` — 등록된 hook 체인 실행 시간 측정.
- `TestHealthCheck_ServiceStatusSERVING`.

### 6.3 Contract (proto)

- `TestProto_BackwardCompatibility` — `buf breaking --against 'previous.bin'`의 CI wrapper.
- 생성 파일 diff 검사: `go generate ./... && git diff --exit-code proto/ internal/transport/grpc/gen/`.

### 6.4 Race

`go test -race`. gRPC server 동시성이 높으므로 필수.

### 6.5 커버리지 목표

- `internal/transport/grpc/`: 85%+ (handler 본체).
- Interceptor: 95%+.
- Generated 파일 `gen/` 제외.

---

## 7. 오픈 이슈

1. **Reflection 최종 정책**: 프로덕션 기본값 off는 확정. 그러나 "Debug 빌드 시 자동 on"을 빌드 태그로 제공할지(`//go:build debug`) — 본 SPEC은 환경변수 단일 제어로 단순화.
2. **Metadata 전달 표준**: `auth_token` 헤더명 소문자 규칙(gRPC metadata는 lowercase)—테스트와 client 모두 준수 확인.
3. **Proto 패키지 고정 규칙**: `genie.v1`으로 확정. 향후 breaking 필요 시 `genie.v2` 신규 디렉토리. 본 SPEC은 v2 준비 로직 없음.
4. **생성 파일 commit 정책**: commit(오프라인 빌드 허용)로 결정. `.gitattributes`에 `linguist-generated=true` 부여하여 GitHub diff noise 제거.
5. **`stop()` vs `GracefulStop()` 분기**: 기본 `GracefulStop` + 10s 초과 시 `Stop`. 이 로직은 CORE-001의 cleanup hook timeout(10s)과 정렬.

---

## 8. 결론

- **이식 자산**: 없음. proto 스키마 전부 신규.
- **참조 자산**: 없음 (Claude Code는 JSON, Hermes는 REST).
- **기술 스택 결정**: `grpc-go` + `buf` + 자체 interceptor 2종(logging, recovery).
- **구현 규모 예상**: 600~1,000 LoC (proto 생성물 제외, 테스트 포함 1,200~1,700 LoC).
- **주요 리스크**: proto 파괴적 변경(R1). CI `buf breaking` gate로 차단.

GREEN 단계 완료 시점에서 CLI-001은 generated `geniev1.DaemonServiceClient`를 소비할 수 있고, LLM/AGENT 후속 SPEC은 `proto/genie/v1/`에 서비스를 추가하는 표준 경로를 확보한다.

---

**End of research.md**
