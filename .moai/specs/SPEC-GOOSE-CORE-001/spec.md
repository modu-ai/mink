---
id: SPEC-GOOSE-CORE-001
version: 1.1.0
status: implemented
created_at: 2026-04-21
updated_at: 2026-04-25
author: manager-spec
priority: P0
issue_number: null
phase: 0
size: 소(S)
lifecycle: spec-anchored
labels: [phase-0, area/core, area/runtime, area/health, type/feature]
---

# SPEC-GOOSE-CORE-001 — goosed 데몬 부트스트랩 및 Graceful Shutdown

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (프로젝트 문서 9종 기반) | manager-spec |
| 0.1.1 | 2026-04-24 | Phase C1 코드 수정 반영: `signal.NotifyContext` 도입(REQ-CORE-004 b절 강화), `RunAllHooks` parentCtx 만료 감시 추가(REQ-CORE-004 c절 보강). 신규 테스트 `TestGoosedMain_SIGTERM_CancelsRootContext`, `TestRunAllHooks_ParentCtxCanceled_StopsIteration` 추가 (commit `79d92ff`). | manager-spec |
| 1.0.0 | 2026-04-25 | iter 1·2 감사 결함 18건 정합화: REQ-CORE-008/011/012 AC 신설(AC-CORE-007/008/009), REQ-CORE-011 EARS 패턴 정정([Unwanted] If…then…), REQ-CORE-003 `atomic.Int32` 표기 정렬, Go 1.26 버전 단일화, §11 번호링 `11.x` 정정, §6.3 Phase 0 OUT OF SCOPE 마킹, §7.1 미생성 테스트 파일 마킹, AC-CORE-001 50ms 단속 강화, AC-CORE-005 stack trace 검증 메커니즘 명시. status: planned→implemented (Phase C1 코드 반영). | manager-spec |
| 1.1.0 (amend) | 2026-04-25 | Cross-package interface contract 추가 (cross-pkg audit `REPORT-CROSS-PKG-IFACE-AUDIT-2026-04-25` 반영) — (1) REQ-CORE-013 `[Pending Implementation v1.1]` 신설: HOOK-001 REQ-HK-021(b)이 의존하는 `WorkspaceRoot(sessionID string) string` resolver. (2) REQ-CORE-014 `[Pending Implementation v1.1]` 신설: TOOLS-001 REQ-TOOLS-011 `Registry.Drain()`을 graceful shutdown sequence에 등록. (3) AC-CORE-010/011 신설. (4) §3.1 IN SCOPE에 (9)/(10) 항목 추가. (5) §7.2 핵심 타입에 `WorkspaceRootResolver`/`DrainConsumer` 시그니처 추가. (6) §9 의존성 표에 HOOK-001/TOOLS-001 후속 SPEC 추가. (7) §12 Open Items 섹션 신설(OI-CORE-1/2 등재). status: implemented→amended (v1.0.0 표면은 구현됨, v1.1.0 신규 REQ는 후속 implementation 필요). | manager-spec |
| 1.1.0 (impl) | 2026-04-25 | OI-CORE-1/2 (REQ-CORE-013/014) implementation 완료 (PR #16, commit 0a71e8e) — (1) `internal/core/session.go` 신규 (SessionRegistry interface + sync.RWMutex 구현체 + 패키지 레벨 WorkspaceRoot 헬퍼), (2) `internal/core/drain.go` 신규 (DrainCoordinator + DrainConsumer + RunAllDrainConsumers panic-safe), (3) `internal/core/runtime.go` Sessions/Drain 필드 추가 + default registry wire-up, (4) `cmd/goosed/main.go` 단계 9.5에 RunAllDrainConsumers 통합. 모든 `[Pending Implementation v1.1]` 마커 제거, AC-CORE-010/011 GREEN, §12 OI-CORE-1/2 CLOSED. status: amended→implemented. 신규 테스트 10건 PASS (race detector green), 기존 21건 회귀 0건. | manager-tdd |

---

## 1. 개요 (Overview)

GOOSE-AGENT의 모든 후속 기능이 붙어야 할 **Go 데몬 프로세스 `goosed`의 최소 부트스트랩 경로**를 정의한다. 본 SPEC은 제품 기능을 포함하지 않는다. 오로지 "데몬이 결정론적으로 뜨고, 신호를 받으면 결정론적으로 내려간다"는 **플랫폼 기반 계약**만 규정한다.

수락 조건을 통과한 시점에서 `goosed`는:

- stdin/stdout/stderr에 구조화 로그를 뿜고,
- `SIGINT`/`SIGTERM` 수신 시 예약된 cleanup hook을 순서대로 실행한 뒤 0으로 종료하며,
- 설정 파일 누락·포트 충돌 등의 블로킹 실패 시 정의된 비-0 exit code로 실패한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- `.moai/project/structure.md` §3은 데몬 아키텍처를 `cmd/goosed/main.go`와 `internal/core/*` 로 명시하나, 레포에 Go 소스가 아직 존재하지 않음 (`ls /Users/goos/MoAI/AgentOS/{cmd,internal,go.mod}` 결과 없음).
- 후속 SPEC(`TRANSPORT-001`, `LLM-001`, `AGENT-001` 등) 모두가 "goosed가 떠 있다"는 전제 위에 세워져 있다. 본 SPEC이 통과하기 전에 어떠한 학습 엔진 SPEC도 시작할 수 없다.
- `.moai/project/tech.md` §1.2는 Polyglot 하이브리드(Go 70% + Rust 20% + TS 10%)를 공식화하였다. Rust/TS 경계는 Phase 5+로 유예되며 **Phase 0은 Go 단독**으로 시작한다.

### 2.2 상속 자산 (직접 포트 대상 없음, 패턴만 계승)

- **MoAI-ADK-Go (38,700 LoC, 외부 레포)**: `cmd/*/main.go` + `internal/core/*` + graceful shutdown 패턴 — 소스가 본 레포에 아직 미러되지 않았으므로 설계만 참고 (research.md 참조).
- **Claude Code TypeScript (`./claude-code-source-map/`)**: 프로세스 관리 원칙 참고만 (직접 포트 아님 — 언어 상이).
- **Hermes Agent Python (`./hermes-agent-main/`)**: Python 프로세스 패턴이므로 Go 재작성 필요. 계승 대상 아님.

### 2.3 범위 경계

- **IN**: 단일 Go 바이너리 `goosed`, 설정 파일 파싱 최소치(경로 1개), 로거 초기화, SIGINT/SIGTERM 핸들링, exit code 계약, 헬스체크 endpoint(HTTP 하나).
- **OUT**: gRPC 서버(→ TRANSPORT-001), LLM 라우팅(→ LLM-001), 에이전트 런타임(→ AGENT-001), 구성 계층화(→ CONFIG-001), CLI 클라이언트(→ CLI-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. Go 1.26+ 단일 바이너리 `goosed`가 `cmd/goosed/main.go`에 존재한다.
2. 프로세스 생애주기: `init → bootstrap → serve → shutdown`의 4-단계 상태 머신.
3. 환경 변수 `GOOSE_HOME` (기본값: `~/.goose`)에서 설정 파일 `~/.goose/config.yaml`을 읽는다. 파일 부재 시 템플릿 기본값으로 fallback.
4. 구조화 로거(`uber-go/zap`) 초기화, JSON 포맷 + stderr 출력.
5. `context.Context` 기반 shutdown 전파 (context cancellation을 모든 하위 goroutine이 구독).
6. `SIGINT` 또는 `SIGTERM` 수신 시 30초 이내 cleanup + exit 0.
7. 최소 HTTP 헬스체크 서버: `GET /healthz` → `200 OK` + JSON `{"status":"ok","version":"..."}`. 기본 포트 `:17890`.
8. Exit code 계약 (아래 §7).
9. `WorkspaceRoot(sessionID string) string` cross-package resolver 노출 — HOOK-001(`REQ-HK-021(b)`)이 shell hook 격리에 사용 (v1.1.0 신규, **구현 완료 PR #16**).
10. `Registry.Drain()` 등 외부 등록형 drain consumer를 graceful shutdown sequence에 fan-out 처리 — TOOLS-001(`REQ-TOOLS-011`)이 의존 (v1.1.0 신규, **구현 완료 PR #16**).

### 3.2 OUT OF SCOPE (명시적 제외)

- gRPC 서비스 등록 (SPEC-GOOSE-TRANSPORT-001).
- 다중 프로바이더 설정 로더 (SPEC-GOOSE-CONFIG-001 — 본 SPEC은 단일 파일만 로드).
- Agent/Tool/Memory 초기화 (각자 SPEC).
- TLS/인증 (mTLS는 전송 SPEC에서).
- 로그 회전(logrotate), 파일 로거 (stderr only).
- `GOOSE_HOME` 외 복수 설정 탐색 경로 (예: `/etc/goose/`).
- Windows Service / launchd / systemd 유닛 파일 생성.
- 백그라운드 데몬화(`daemon()` fork/setsid) — 포어그라운드 전용. systemd 등 상위 supervisor가 관리한다고 가정.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-CORE-001 [Ubiquitous]** — The `goosed` process **shall** write all log lines to stderr in line-delimited JSON with mandatory fields `{ts, level, msg, caller}`.

**REQ-CORE-002 [Ubiquitous]** — The `goosed` process **shall** identify itself with the fields `service="goosed"` and `version="<semver>"` injected at build time via `-ldflags "-X main.version=..."`.

**REQ-CORE-003 [Ubiquitous]** — The `goosed` process **shall** expose its process state (`init|bootstrap|serving|draining|stopped`) through an internal `atomic.Int32` readable by health probes (typed counter for type-safe state machine; replaces earlier `atomic.Value` draft).

### 4.2 Event-Driven (이벤트 기반)

**REQ-CORE-004 [Event-Driven]** — **When** the process receives `SIGINT` or `SIGTERM`, the `goosed` process **shall** (a) transition state to `draining`, (b) cancel the root `context.Context`, (c) wait up to 30 seconds for registered cleanup hooks to return, and (d) exit with code `0` on success.

**REQ-CORE-005 [Event-Driven]** — **When** an HTTP `GET /healthz` request arrives, the health server **shall** respond with `200 OK` and JSON body `{"status":"ok","state":"<current-state>","version":"<semver>"}` within 50ms.

**REQ-CORE-006 [Event-Driven]** — **When** the configured health-port is already in use at bootstrap, the `goosed` process **shall** log a single ERROR line containing the port number and exit with code `78` (EX_CONFIG).

### 4.3 State-Driven (상태 기반)

**REQ-CORE-007 [State-Driven]** — **While** state is `draining`, the health endpoint **shall** respond with `503 Service Unavailable` and JSON body `{"status":"draining"}`.

**REQ-CORE-008 [State-Driven]** — **While** cleanup hooks are executing, no new HTTP requests **shall** be accepted on the health-port (listener closed before hook fan-out).

### 4.4 Unwanted Behavior (방지)

**REQ-CORE-009 [Unwanted]** — **If** any cleanup hook panics, **then** the `goosed` process **shall** log the panic with full stack trace, continue executing remaining hooks, and exit with code `1` instead of `0`.

**REQ-CORE-010 [Unwanted]** — **If** `GOOSE_HOME/config.yaml` exists but fails YAML parsing, **then** the `goosed` process **shall** exit with code `78` (EX_CONFIG) without starting the health server.

**REQ-CORE-011 [Unwanted]** — **If** `GOOSE_LOG_LEVEL` is set to `info` or higher, **then** the `goosed` process **shall not** write any log line at level `DEBUG` or below.

### 4.5 Optional (선택적)

**REQ-CORE-012 [Optional]** — **Where** environment variable `GOOSE_HEALTH_PORT` is defined, the health server **shall** bind to that port instead of the default `17890`.

### 4.6 Cross-Package Contracts (v1.1.0 신규 — `[Pending Implementation v1.1]`)

> 본 섹션의 두 REQ는 v1.1.0 amendment에서 추가되었으며, 모두 후속 implementation 작업으로 분리됨. v1.0.0 GREEN 판정에는 영향 없음. cross-pkg audit `REPORT-CROSS-PKG-IFACE-AUDIT-2026-04-25` D-CORE-IF-1/2 대응.

**REQ-CORE-013 [Event-Driven]** — **When** an external consumer (typically the HOOK-001 dispatcher fulfilling `REQ-HK-021(b)`) invokes `core.WorkspaceRoot(sessionID string) string`, the runtime **shall** return the absolute project workspace root path mapped to that session, or an empty string if no mapping exists for the given `sessionID`. The resolver **shall** be safe for concurrent invocation from multiple goroutines and **shall not** block on I/O for cached entries. The session-to-workspace mapping is registered via `core.SessionRegistry.Register(sessionID, workspaceRoot)` during session start; mappings persist for the session lifetime. (v1.1.0: 구현 완료 — `internal/core/session.go` `SessionRegistry` interface + `sync.RWMutex` 동시성 안전 구현체 + 패키지 레벨 `WorkspaceRoot` 헬퍼. PR #16 / commit 0a71e8e. §12 OI-CORE-1 CLOSED.)

**REQ-CORE-014 [Event-Driven]** — **When** the process state transitions to `draining` (REQ-CORE-004 (a)), the runtime **shall** invoke all registered `DrainConsumer` callbacks (registered via `Runtime.Drain.RegisterDrainConsumer(c DrainConsumer)`) before fanning out the existing `CleanupHook` chain. `DrainConsumer` invocations **shall** run sequentially in registration order with a per-consumer timeout of 10s; consumer errors **shall** be logged at WARN level but **shall not** abort the drain sequence. This contract supports TOOLS-001 `Registry.Drain()` (REQ-TOOLS-011) and similar registry-style consumers that must reject new work before in-flight operations complete. (v1.1.0: 구현 완료 — `internal/core/drain.go` `DrainCoordinator` + `RunAllDrainConsumers` (panic-safe, parentCtx 만료 watch), `cmd/goosed/main.go` 단계 9.5에 wire-up. PR #16 / commit 0a71e8e. §12 OI-CORE-2 CLOSED.)

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then. `*_test.go`로 변환 가능한 수준.

**AC-CORE-001 — 정상 부트스트랩 및 헬스체크 (REQ-CORE-001/002/003/005)**
- **Given** `GOOSE_HOME`이 `t.TempDir()`로 설정되고 `config.yaml`이 비어있는 기본값 상태
- **When** `goosed`를 실행하고 200ms 대기
- **Then** 상태는 `serving`이고, `curl localhost:17890/healthz`가 200 + `{"status":"ok","state":"serving","version":"<semver>"}`를 반환하며, **응답 latency가 50ms 이내**임이 측정되어야 한다(REQ-CORE-005 — `time.Since(start) < 50*time.Millisecond` 단속).

**AC-CORE-002 — SIGTERM 수신 시 graceful shutdown**
- **Given** `goosed`가 `serving` 상태로 동작 중
- **When** 테스트가 `SIGTERM`을 해당 PID로 송신
- **Then** 프로세스는 3초 이내 exit 0으로 종료하고, 등록된 cleanup hook 3개가 모두 호출되었음이 로그로 확인됨

**AC-CORE-003 — 잘못된 YAML 설정 파일 거부**
- **Given** `GOOSE_HOME/config.yaml`에 비-YAML 텍스트 (`"::: not yaml :::"`) 기록
- **When** `goosed` 실행
- **Then** exit code 78, stderr에 `config parse error` ERROR 레벨 로그 1건, 헬스서버는 기동하지 않음

**AC-CORE-004 — 포트 충돌 시 실패**
- **Given** 다른 listener가 이미 `:17890`에 바인딩되어 있음
- **When** `goosed` 실행
- **Then** exit code 78, stderr에 `health-port in use: 17890` 메시지

**AC-CORE-005 — cleanup hook panic 격리 (REQ-CORE-009)**
- **Given** 3개 cleanup hook이 등록되어 있고 그중 2번째가 `panic("boom")`을 발생
- **When** `SIGTERM` 송신
- **Then** exit code 1, 3개 hook이 모두 호출됨(패닉 이후에도 나머지 실행), `zaptest/observer`로 캡처한 ERROR 로그 레코드의 `stack` 필드가 `goroutine ` 패턴(또는 `runtime.gopanic`)을 포함함을 assertion으로 검증.

**AC-CORE-006 — 드레이닝 중 503 응답 (REQ-CORE-007)**
- **Given** `goosed` state=`draining` (cleanup 진행 중 강제 고정)
- **When** `GET /healthz`
- **Then** 503 + body `{"status":"draining"}`

**AC-CORE-007 — 드레이닝 시 listener 종료 (REQ-CORE-008)**
- **Given** `goosed`가 `serving` 상태로 동작 중이고 health listener는 `:17890`에 바인딩되어 있음
- **When** `SIGTERM`을 송신하여 process가 `draining`으로 전이된 후, 새로운 TCP 연결 `net.Dial("tcp", "127.0.0.1:17890")`을 시도
- **Then** 연결 시도가 `connection refused` 또는 동등한 OS 오류로 실패해야 한다(listener는 hook fan-out 이전에 close됨). 실패는 `draining` 상태 관측 시점으로부터 100ms 이내에 발생해야 한다.

**AC-CORE-008 — DEBUG 로그 억제 (REQ-CORE-011)**
- **Given** 환경변수 `GOOSE_LOG_LEVEL=info`로 `goosed` 시작
- **When** 내부 코드가 `logger.Debug(...)`를 1회 이상 호출
- **Then** stderr에서 캡처한 JSON 로그 라인 중 `"level":"debug"` 필드를 가진 라인이 **0건**이어야 한다.

**AC-CORE-009 — 헬스 포트 override (REQ-CORE-012)**
- **Given** 환경변수 `GOOSE_HEALTH_PORT=18999` 설정 후 `goosed` 시작
- **When** `curl http://127.0.0.1:18999/healthz` 200ms 이내 호출
- **Then** 200 + `{"status":"ok","state":"serving",...}` 반환. 동시에 기본 포트 `:17890`에는 어떤 listener도 바인딩되어 있지 않아야 한다(`net.Dial("tcp", "127.0.0.1:17890")` → `connection refused`).

**AC-CORE-010 — WorkspaceRoot resolver (REQ-CORE-013)**
- **Given** `goosed`가 `serving` 상태이고, 두 세션 `sess-A`/`sess-B`에 대해 각각 `core.SessionRegistry.Register("sess-A", "/tmp/work-a")`, `Register("sess-B", "/tmp/work-b")` 호출됨
- **When** 100개의 goroutine이 동시에 `core.WorkspaceRoot("sess-A")` / `core.WorkspaceRoot("sess-B")` / `core.WorkspaceRoot("sess-unknown")`을 임의 순서로 호출
- **Then** `sess-A`는 항상 `/tmp/work-a`, `sess-B`는 항상 `/tmp/work-b`, `sess-unknown`은 항상 `""`을 반환하며, 100개 goroutine 모두 race detector(`-race`) 환경에서 data race 없음. 단일 호출 latency는 메모리 캐시 hit 기준 1ms 이내.
- **v1.1.0 상태**: GREEN. `internal/core/session.go` 구현 + `TestWorkspaceRoot_ConcurrentAccess`(100 goroutine race detection) PASS. PR #16 / commit 0a71e8e. §12 OI-CORE-1 CLOSED.

**AC-CORE-011 — DrainConsumer fan-out (REQ-CORE-014)**
- **Given** 3개의 `DrainConsumer`가 `core.RegisterDrainConsumer`로 등록됨 — 첫 번째는 정상 반환, 두 번째는 `errors.New("drain failed")` 반환, 세 번째는 정상 반환. 추가로 1개의 `CleanupHook`이 `core.RegisterHook`으로 등록됨
- **When** `SIGTERM` 송신
- **Then** (1) 3개 `DrainConsumer`가 등록 순서대로 모두 호출되고(두 번째 에러는 WARN 로그만 기록, 후속 consumer 진행), (2) 모든 `DrainConsumer` 완료 후 `CleanupHook`이 호출되며, (3) exit code 0으로 종료. 호출 순서는 zap observer로 캡처한 로그 시퀀스로 검증.
- **v1.1.0 상태**: GREEN. `internal/core/drain.go` 구현 + 5건 단위 테스트(RegisterAndFanOut/ErrorIsolation/PanicIsolation/PerConsumerTimeout/ParentCtxExpired) PASS. `cmd/goosed/main.go` 단계 9.5에 통합. PR #16 / commit 0a71e8e. §12 OI-CORE-2 CLOSED.

---

## 6. 기술 스택 (확정, v0.2 재편 반영)

> M0 구현 시 사용하는 의존성 목록. 후속 SPEC 작성자는 이 항목을 기준으로 삼을 것.
> 본 섹션은 SPEC-GOOSE-ARCH-REDESIGN-v0.2 확정본과 정합된다 (`.moai/design/goose-runtime-architecture-v0.2.md`).

### 6.1 모듈 & 바이너리

| 항목 | 값 |
|-----|------|
| **Module path** | `github.com/modu-ai/goose` |
| **Binaries** | `goosed` (daemon), `goose` (user CLI), `goose-proxy` (zero-knowledge credential proxy) |
| **Storage 2원화** | `~/.goose/` (credentials only) + `./.goose/` (project workspace) |

### 6.2 핵심 의존성

| 구분 | 패키지 / 버전 | 근거 |
|-----|-------------|------|
| **Go 런타임** | `go 1.26` | 최신 안정 릴리스 |
| **SQLite 드라이버** | `modernc.org/sqlite` (CGO-free) | 순수 Go, 크로스컴파일 용이; 단일 `goose.db` WAL 모드 |
| **DB Migration** | `golang-migrate` embedded | up/down SQL 파일 `go:embed` 번들 |
| **토크나이저** | `github.com/pkoukk/tiktoken-go` | tiktoken 호환; LLM 토큰 계산 (Phase 2+) |
| **그래프 DB** | `github.com/kuzudb/go-kuzu` (임베디드) | Phase 8 Semantic memory에서 활성화 |
| **LLM 스트림** | `google.golang.org/grpc` (gRPC streaming) | TRANSPORT-001 설계와 일관 |
| **Hybrid 검색 (신규)** | `qntx-labs/qmd` (Rust, CGO staticlib) | M1 편입; BM25 + vector + LLM rerank; markdown 의미 검색 |
| **Rust CGO** | CGO-embedded staticlib | QMD · (후속) LoRA · crypto; 단일 바이너리 유지 |
| **OS Keyring** | `github.com/zalando/go-keyring` | macOS Keychain / libsecret / Windows Cred Vault 추상 |

### 6.3 보안 스택 (참고용 — Phase 0 **OUT OF SCOPE**)

> **Phase 0 적용 불가**: 본 SPEC은 보안 계층을 구현하지 않는다. 아래 표는 후속 SPEC(M5 Safety) 작성자를 위한 사전 합의 자료이며, Phase 0 코드/테스트 평가 기준에서 제외된다.

| 계층 | 도구 / 기법 | 도입 SPEC |
|-----|-----------|---------|
| Tier 1 Storage partition | 파일시스템 분리 (`~/.goose/secrets/` vs `./.goose/**`) | M5 Safety |
| Tier 2 FS access matrix | `security.yaml` allowlist/denylist + blocked_always 목록 | M5 Safety |
| Tier 3 OS sandbox | macOS Seatbelt, Linux Landlock+Seccomp, Windows AppContainer | M5 Safety |
| Tier 4 Zero-knowledge proxy | OS keyring + `goose-proxy` transport injection | M5 Safety |
| Tier 5 Declared permission | Skill/MCP frontmatter `requires:` + first-call confirm | M5 Safety |

> **Phase 0 (본 SPEC)** 에서는 `modernc.org/sqlite`, gRPC, tiktoken-go를 `go.mod`에 추가하되,
> 실제 사용은 후속 SPEC에서 진행한다. Kuzu는 Phase 8, QMD는 M1, Rust CGO staticlib 바인딩은 M5/M8로 유예.

---

## 7. 기술적 접근 (Technical Approach)

### 7.1 제안 패키지 레이아웃

```
/ (repo root)
├── go.mod                          # module github.com/modu-ai/goose (Go 1.26+)
├── cmd/goosed/main.go              # 진입점: ≤120 LoC (root ctx + signal.NotifyContext + exit code 분기 포함)
├── internal/core/
│   ├── runtime.go                  # NewRuntime() factory, RootCtx 노출 (실제 구현은 bootstrap.go 대신 runtime.go)
│   ├── shutdown.go                 # RegisterHook / RunAllHooks / 30s timeout / parentCtx 만료 감시
│   ├── state.go                    # atomic.Int32 기반 state machine
│   ├── logger.go                   # zap JSON 로거 (REQ-CORE-001/002/011)
│   ├── exitcodes.go                # const ExitOK, ExitConfig=78, ExitHookPanic=1
│   └── runtime_test.go             # 통합 테스트 (현 시점 health/config 테스트 포함; 후속 분리 예정)
├── internal/health/
│   └── server.go                   # http.Server + /healthz (※ server_test.go는 차기 sprint에서 추가)
└── internal/config/
    └── bootstrap_config.go         # 최소 loader (본 SPEC 전용, CONFIG-001이 확장; ※ bootstrap_config_test.go는 차기 sprint에서 추가)
```

> **레이아웃 노트**: SPEC 초안은 `bootstrap.go` / 패키지별 `_test.go` 파일을 가정했으나, 실제 구현은 `runtime.go` 팩토리 + 통합 `internal/core/runtime_test.go` 단일 파일로 진행됨. `internal/health/server_test.go`와 `internal/config/bootstrap_config_test.go`는 본 SPEC 범위에서 미생성 상태이며, 차기 리팩터 sprint(테스트 분리 작업)에서 추가한다.

### 7.2 핵심 타입 (초안)

```go
// internal/core/state.go
type ProcessState int

const (
    StateInit ProcessState = iota
    StateBootstrap
    StateServing
    StateDraining
    StateStopped
)

// internal/core/shutdown.go
type CleanupHook struct {
    Name    string
    Fn      func(ctx context.Context) error
    Timeout time.Duration // per-hook; default 10s
}

// === v1.1.0 신규 (Pending Implementation) ===

// internal/core/session.go (REQ-CORE-013, AC-CORE-010)
//
// SessionRegistry는 sessionID → workspace root 매핑을 관리한다.
// HOOK-001 dispatcher가 shell hook subprocess의 working directory를 결정할 때
// WorkspaceRoot(sessionID) 형태로 호출한다.
type SessionRegistry interface {
    Register(sessionID, workspaceRoot string)
    Unregister(sessionID string)
    WorkspaceRoot(sessionID string) string // empty string if unmapped
}

// 패키지 레벨 헬퍼 (HOOK-001이 직접 호출하는 표면)
func WorkspaceRoot(sessionID string) string

// internal/core/drain.go (REQ-CORE-014, AC-CORE-011)
//
// DrainConsumer는 graceful shutdown 시 CleanupHook 이전에 fan-out 호출되는
// registry-style consumer를 지원한다. TOOLS-001 Registry.Drain(), 후속 SPEC의
// SessionRegistry/SubagentSpawner 등이 등록 대상.
type DrainConsumer struct {
    Name    string
    Fn      func(ctx context.Context) error
    Timeout time.Duration // per-consumer; default 10s
}

func RegisterDrainConsumer(c DrainConsumer)
// 내부 호출 — REQ-CORE-014: 등록 순서대로 sequential 실행, 에러는 WARN 로그
func runDrainConsumers(ctx context.Context) error
```

### 7.3 의존성 (최소)

| 라이브러리 | 용도 | 근거 |
|----------|------|-----|
| `go.uber.org/zap` v1.27+ | 구조화 JSON 로거 | tech.md §3.1 |
| `gopkg.in/yaml.v3` | 설정 파일 파싱 | 표준 YAML 선택지 |
| stdlib `net/http` | 헬스서버 (외부 프레임워크 없음) | 최소주의 원칙 |
| stdlib `os/signal`, `context`, `sync/atomic` | 시그널 + 상태 | 표준 라이브러리 |

gin, viper, cobra 등은 본 SPEC에서 **의도적으로 미사용**. CONFIG-001(viper)과 CLI-001(cobra)에서 도입.

### 7.4 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1**: `TestBootstrap_SucceedsWithEmptyConfig` — AC-CORE-001 → 구현 없음 → 실패.
2. **RED #2**: `TestHealthz_ReturnsServingJSON` — AC-CORE-001 hand-off → 실패.
3. **RED #3**: `TestSIGTERM_InvokesHooks_ExitZero` — AC-CORE-002 → 실패.
4. ... (AC별 테스트 모두 RED 확인 후)
5. **GREEN**: `internal/core/*`, `internal/health/*` 최소 구현.
6. **REFACTOR**: state machine을 `atomic.Int32`로 일관화, logger를 모든 hook에 DI.

#### 7.4.1 Phase C1 보강 테스트 (commit `79d92ff`, 2026-04-24)

iter 1 감사에서 도출된 Major 결함 B4-1·B4-2(root context 미전파, RunAllHooks parentCtx 만료 감시 부재)를 해결하기 위해 다음 두 테스트가 추가되었다. 본 테스트는 REQ-CORE-004 b·c 절의 계약 강화를 검증한다.

- **`TestGoosedMain_SIGTERM_CancelsRootContext`** (`internal/core/runtime_test.go`) — `signal.NotifyContext` 기반 root ctx 취소 경로 검증. `SIGTERM` 송신 시 `Runtime.RootCtx.Done()`이 닫히고 hook이 해당 ctx를 구독할 수 있음을 assertion (REQ-CORE-004 b절).
- **`TestRunAllHooks_ParentCtxCanceled_StopsIteration`** (`internal/core/runtime_test.go`) — `RunAllHooks` 진입 후 parentCtx가 만료되면 남은 hook 실행을 중단하고 warn 로그를 남기는 경로 검증 (REQ-CORE-004 c절 30s timeout 보장).

두 테스트 모두 `go test -race -count=5` 240s 환경에서 PASS 확인됨. 본 SPEC의 구현 정합률은 Phase C1 이후 12/12 REQ + 9/9 AC 커버리지로 정렬된다.

### 7.5 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, 시그널·포트·패닉 전부 integration test |
| **R**eadable | main.go ≤120 LoC (root ctx + signal.NotifyContext + exit code 분기 포함; 초기 30줄 목표는 root ctx 보강 후 완화), package별 단일 책임 |
| **U**nified | `go fmt` + `golangci-lint` (errcheck, govet, staticcheck) |
| **S**ecured | 헬스 endpoint 인증 없음은 의도적 (localhost only, 기본 포트 바인드 `127.0.0.1`) |
| **T**rackable | `service="goosed"`, SPEC-ID commit trailer 적용 |

---

## 8. Exit Code 계약 (Contract)

| 코드 | 의미 | 트리거 |
|-----|------|-------|
| `0` | 정상 종료 | SIGINT/SIGTERM 후 모든 hook 성공 |
| `1` | hook 실행 중 panic 발생 | REQ-CORE-009 |
| `78` (EX_CONFIG) | 설정 오류 | 파싱 실패, 포트 충돌 (REQ-CORE-006, REQ-CORE-010) |
| `64` (EX_USAGE) | (미래) 잘못된 CLI 플래그 | 본 SPEC 사용 안 함. 예약 |

---

## 9. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | (없음) | 본 SPEC이 Phase 0의 최초 SPEC |
| 후속 SPEC | SPEC-GOOSE-CONFIG-001 | 계층형 설정으로 확장 |
| 후속 SPEC | SPEC-GOOSE-TRANSPORT-001 | 동일 process 내 gRPC 등록 |
| 후속 SPEC | SPEC-GOOSE-HOOK-001 | `core.WorkspaceRoot(sessionID string) string` consumer (REQ-HK-021(b)) — v1.1.0 신규 의존 |
| 후속 SPEC | SPEC-GOOSE-TOOLS-001 | `core.RegisterDrainConsumer(...)` consumer (REQ-TOOLS-011 `Registry.Drain()`) — v1.1.0 신규 의존 |
| 외부 | Go 1.26+ toolchain | `go build`, `go test` (go.mod = `go 1.26`) |
| 외부 | `go.uber.org/zap` | 구조화 로깅 |

---

## 10. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Go 버전 합의 실패 (tech.md 1.26+ vs 과거 안정 1.22 잔존) | 낮 | 낮 | 본 SPEC에서 **Go 1.26**으로 단일화 (go.mod `go 1.26`). tech.md/structure.md/spec.md §6.2 모두 일관. 1.26 미만 toolchain은 빌드 거부. |
| R2 | `SIGTERM` 처리가 OS별로 상이 (Windows) | 중 | 낮 | 초기 타깃은 darwin/linux만. Windows 지원은 OUT OF SCOPE |
| R3 | 30초 graceful 타임아웃 부족 (LoRA 훈련 중 종료 등) | 낮 | 중 | 본 SPEC은 학습 hook 없음. 학습 SPEC들이 자체 cancellable 패턴 보장 |
| R4 | panic recovery가 stack trace를 분실 | 낮 | 중 | `debug.Stack()` 호출 보장, 테스트로 검증 (AC-CORE-005) |
| R5 | 기본 포트 17890이 사용 중인 환경에서 기본값 충돌 | 중 | 낮 | `GOOSE_HEALTH_PORT` env override 제공 (REQ-CORE-012) |

---

## 11. 참고 (References)

### 11.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/structure.md` §1 (디렉토리), §3 (3-tier 계층), §9 (MoAI-ADK 상속)
- `.moai/project/tech.md` §1.2 (3-언어 매핑), §3.1 (Go 런타임 스택), §11.1 (개발환경)
- `.moai/project/product.md` §3.1 (4-Layer 아키텍처 상 "goosed" 등장)
- `.moai/project/migration.md` §6.4 (v4.0 신규 작업 항목 중 "코어 런타임" 필요)

### 11.2 외부 참조

- **MoAI-ADK-Go** (외부 레포): `cmd/*/main.go` + `internal/core/bootstrap.go` graceful shutdown 패턴
- **go.uber.org/zap** v1.27: https://pkg.go.dev/go.uber.org/zap
- **sysexits.h** exit code 관례: FreeBSD `/usr/include/sysexits.h`

### 11.3 부속 문서

- `./research.md` — MoAI-ADK-Go 상속 가능 영역, Hermes/Claude Code 분석 결과
- `../ROADMAP.md` — 전체 Phase 계획
- `.moai/reports/cross-package-interface-audit-2026-04-25.md` — cross-pkg interface stub audit (v1.1.0 amendment 근거)

---

## 12. Open Items (v1.1.0 amendment — 모두 CLOSED, traceability 보존을 위해 표 유지)

> 본 섹션은 v1.1.0 amendment에서 신설됨. cross-pkg audit `REPORT-CROSS-PKG-IFACE-AUDIT-2026-04-25` D-CORE-IF-1/2 대응. v1.1.0 amendment 시점에 `[Pending Implementation v1.1]` 마킹으로 등재되었던 OI-CORE-1/2는 PR #16 (commit `0a71e8e`)에서 일괄 CLOSED 처리되었다. 행은 traceability 목적으로 유지된다(삭제 금지).

| OI ID | 대응 REQ | 대응 AC | 결함 | 구현 범위 | 우선순위 | 목표 버전 |
|-------|---------|--------|------|---------|---------|---------|
| OI-CORE-1 | REQ-CORE-013 | AC-CORE-010 | D-CORE-IF-1 (Major). HOOK-001 REQ-HK-021(b)이 의존하는 `WorkspaceRoot(sessionID string) string` resolver가 v1.0.0 시점에 미정의. HOOK-001 단독 구현 시 fail-closed로 동작(session-resolution error 반환)하나, 정상 동작을 위해 CORE-001 export 필요. | `internal/core/session.go` 신규 (`SessionRegistry` interface + 동시성 안전 map 기반 구현체) + 패키지 레벨 `WorkspaceRoot(sessionID)` 헬퍼 + `runtime.go`에서 `SessionRegistry`를 `Runtime`에 wire-up. 단위 테스트: 동시 100 goroutine race detection (AC-CORE-010). | High (HOOK-001 정상 동작 차단) | **CLOSED in v1.1.0 (PR #16, commit `0a71e8e`)** |
| OI-CORE-2 | REQ-CORE-014 | AC-CORE-011 | D-CORE-IF-2 (Minor). TOOLS-001 REQ-TOOLS-011 `Registry.Drain()`을 호출할 graceful shutdown 단계가 미명시. 현 `shutdown.go`는 `RegisterHook`/`RunAllHooks`만 노출. | `internal/core/drain.go` 신규 (`DrainConsumer` 타입 + `RegisterDrainConsumer` API + `runDrainConsumers` 내부 fan-out) + `runtime.go` SIGTERM 경로에서 (1) state→draining, (2) `runDrainConsumers`, (3) `RunAllHooks` 순서로 호출하도록 수정. 단위 테스트: 3 consumer 순서/에러 격리 (AC-CORE-011). | Medium (TOOLS-001 in-flight 보호) | **CLOSED in v1.1.0 (PR #16, commit `0a71e8e`)** |

### 처리 원칙 (v1.1.0 CLOSE 시점 기준)

- OI-CORE-1/2는 v1.0.0 수락 기준(`AC-CORE-001~009`)에 영향 없음을 유지하면서, v1.1.0 amendment의 신규 AC-CORE-010/011을 추가로 GREEN 처리하였다.
- TDD RED→GREEN→REFACTOR 사이클로 구현됨 (PR #16). 신규 테스트 10건 모두 race detector 환경에서 PASS, 기존 21건 회귀 0건.
- 본 §12 표는 traceability 목적으로 유지 (삭제 금지). 향후 외부 참조 시 OI ID로 조회 가능.

### 감사 참조

- `.moai/reports/cross-package-interface-audit-2026-04-25.md` D-CORE-IF-1, D-CORE-IF-2

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **gRPC 서버를 기동하지 않는다**. HTTP /healthz만. gRPC는 TRANSPORT-001.
- 본 SPEC은 **LLM 호출 없음**. 데몬이 뜨는 것 자체가 목표. LLM-001이 이어받음.
- 본 SPEC은 **자기진화 엔진을 로드하지 않는다**. REFLECT-001 이전에는 어떠한 학습 관련 hook도 등록하지 않음.
- 본 SPEC은 **TLS / 인증을 구현하지 않는다**. 헬스 endpoint는 127.0.0.1 바인드.
- 본 SPEC은 **백그라운드 데몬화(`daemon()`)를 수행하지 않는다**. systemd 등 상위 supervisor가 담당.
- 본 SPEC은 **Windows를 타깃하지 않는다**. darwin + linux amd64/arm64만.
- 본 SPEC은 **Tauri / Mobile / Web 클라이언트를 포함하지 않는다**. CLI는 CLI-001.

---

**End of SPEC-GOOSE-CORE-001**
