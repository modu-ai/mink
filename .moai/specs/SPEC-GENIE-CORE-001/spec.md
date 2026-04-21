---
id: SPEC-GENIE-CORE-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 0
size: 소(S)
lifecycle: spec-anchored
---

# SPEC-GENIE-CORE-001 — genied 데몬 부트스트랩 및 Graceful Shutdown

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (프로젝트 문서 9종 기반) | manager-spec |

---

## 1. 개요 (Overview)

GENIE-AGENT의 모든 후속 기능이 붙어야 할 **Go 데몬 프로세스 `genied`의 최소 부트스트랩 경로**를 정의한다. 본 SPEC은 제품 기능을 포함하지 않는다. 오로지 "데몬이 결정론적으로 뜨고, 신호를 받으면 결정론적으로 내려간다"는 **플랫폼 기반 계약**만 규정한다.

수락 조건을 통과한 시점에서 `genied`는:

- stdin/stdout/stderr에 구조화 로그를 뿜고,
- `SIGINT`/`SIGTERM` 수신 시 예약된 cleanup hook을 순서대로 실행한 뒤 0으로 종료하며,
- 설정 파일 누락·포트 충돌 등의 블로킹 실패 시 정의된 비-0 exit code로 실패한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- `.moai/project/structure.md` §3은 데몬 아키텍처를 `cmd/genied/main.go`와 `internal/core/*` 로 명시하나, 레포에 Go 소스가 아직 존재하지 않음 (`ls /Users/goos/MoAI/AgentOS/{cmd,internal,go.mod}` 결과 없음).
- 후속 SPEC(`TRANSPORT-001`, `LLM-001`, `AGENT-001` 등) 모두가 "genied가 떠 있다"는 전제 위에 세워져 있다. 본 SPEC이 통과하기 전에 어떠한 학습 엔진 SPEC도 시작할 수 없다.
- `.moai/project/tech.md` §1.2는 Polyglot 하이브리드(Go 70% + Rust 20% + TS 10%)를 공식화하였다. Rust/TS 경계는 Phase 5+로 유예되며 **Phase 0은 Go 단독**으로 시작한다.

### 2.2 상속 자산 (직접 포트 대상 없음, 패턴만 계승)

- **MoAI-ADK-Go (38,700 LoC, 외부 레포)**: `cmd/*/main.go` + `internal/core/*` + graceful shutdown 패턴 — 소스가 본 레포에 아직 미러되지 않았으므로 설계만 참고 (research.md 참조).
- **Claude Code TypeScript (`./claude-code-source-map/`)**: 프로세스 관리 원칙 참고만 (직접 포트 아님 — 언어 상이).
- **Hermes Agent Python (`./hermes-agent-main/`)**: Python 프로세스 패턴이므로 Go 재작성 필요. 계승 대상 아님.

### 2.3 범위 경계

- **IN**: 단일 Go 바이너리 `genied`, 설정 파일 파싱 최소치(경로 1개), 로거 초기화, SIGINT/SIGTERM 핸들링, exit code 계약, 헬스체크 endpoint(HTTP 하나).
- **OUT**: gRPC 서버(→ TRANSPORT-001), LLM 라우팅(→ LLM-001), 에이전트 런타임(→ AGENT-001), 구성 계층화(→ CONFIG-001), CLI 클라이언트(→ CLI-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. Go 1.22+ 단일 바이너리 `genied`가 `cmd/genied/main.go`에 존재한다.
2. 프로세스 생애주기: `init → bootstrap → serve → shutdown`의 4-단계 상태 머신.
3. 환경 변수 `GENIE_HOME` (기본값: `~/.genie`)에서 설정 파일 `~/.genie/config.yaml`을 읽는다. 파일 부재 시 템플릿 기본값으로 fallback.
4. 구조화 로거(`uber-go/zap`) 초기화, JSON 포맷 + stderr 출력.
5. `context.Context` 기반 shutdown 전파 (context cancellation을 모든 하위 goroutine이 구독).
6. `SIGINT` 또는 `SIGTERM` 수신 시 30초 이내 cleanup + exit 0.
7. 최소 HTTP 헬스체크 서버: `GET /healthz` → `200 OK` + JSON `{"status":"ok","version":"..."}`. 기본 포트 `:17890`.
8. Exit code 계약 (아래 §7).

### 3.2 OUT OF SCOPE (명시적 제외)

- gRPC 서비스 등록 (SPEC-GENIE-TRANSPORT-001).
- 다중 프로바이더 설정 로더 (SPEC-GENIE-CONFIG-001 — 본 SPEC은 단일 파일만 로드).
- Agent/Tool/Memory 초기화 (각자 SPEC).
- TLS/인증 (mTLS는 전송 SPEC에서).
- 로그 회전(logrotate), 파일 로거 (stderr only).
- `GENIE_HOME` 외 복수 설정 탐색 경로 (예: `/etc/genie/`).
- Windows Service / launchd / systemd 유닛 파일 생성.
- 백그라운드 데몬화(`daemon()` fork/setsid) — 포어그라운드 전용. systemd 등 상위 supervisor가 관리한다고 가정.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-CORE-001 [Ubiquitous]** — The `genied` process **shall** write all log lines to stderr in line-delimited JSON with mandatory fields `{ts, level, msg, caller}`.

**REQ-CORE-002 [Ubiquitous]** — The `genied` process **shall** identify itself with the fields `service="genied"` and `version="<semver>"` injected at build time via `-ldflags "-X main.version=..."`.

**REQ-CORE-003 [Ubiquitous]** — The `genied` process **shall** expose its process state (`init|bootstrap|serving|draining|stopped`) through an internal `atomic.Value` readable by health probes.

### 4.2 Event-Driven (이벤트 기반)

**REQ-CORE-004 [Event-Driven]** — **When** the process receives `SIGINT` or `SIGTERM`, the `genied` process **shall** (a) transition state to `draining`, (b) cancel the root `context.Context`, (c) wait up to 30 seconds for registered cleanup hooks to return, and (d) exit with code `0` on success.

**REQ-CORE-005 [Event-Driven]** — **When** an HTTP `GET /healthz` request arrives, the health server **shall** respond with `200 OK` and JSON body `{"status":"ok","state":"<current-state>","version":"<semver>"}` within 50ms.

**REQ-CORE-006 [Event-Driven]** — **When** the configured health-port is already in use at bootstrap, the `genied` process **shall** log a single ERROR line containing the port number and exit with code `78` (EX_CONFIG).

### 4.3 State-Driven (상태 기반)

**REQ-CORE-007 [State-Driven]** — **While** state is `draining`, the health endpoint **shall** respond with `503 Service Unavailable` and JSON body `{"status":"draining"}`.

**REQ-CORE-008 [State-Driven]** — **While** cleanup hooks are executing, no new HTTP requests **shall** be accepted on the health-port (listener closed before hook fan-out).

### 4.4 Unwanted Behavior (방지)

**REQ-CORE-009 [Unwanted]** — **If** any cleanup hook panics, **then** the `genied` process **shall** log the panic with full stack trace, continue executing remaining hooks, and exit with code `1` instead of `0`.

**REQ-CORE-010 [Unwanted]** — **If** `GENIE_HOME/config.yaml` exists but fails YAML parsing, **then** the `genied` process **shall** exit with code `78` (EX_CONFIG) without starting the health server.

**REQ-CORE-011 [Unwanted]** — The `genied` process **shall not** write any log line at level `DEBUG` or below when `GENIE_LOG_LEVEL` is set to `info` or higher.

### 4.5 Optional (선택적)

**REQ-CORE-012 [Optional]** — **Where** environment variable `GENIE_HEALTH_PORT` is defined, the health server **shall** bind to that port instead of the default `17890`.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then. `*_test.go`로 변환 가능한 수준.

**AC-CORE-001 — 정상 부트스트랩 및 헬스체크**
- **Given** `GENIE_HOME`이 `t.TempDir()`로 설정되고 `config.yaml`이 비어있는 기본값 상태
- **When** `genied`를 실행하고 200ms 대기
- **Then** 상태는 `serving`이고, `curl localhost:17890/healthz`가 200 + `{"status":"ok","state":"serving",...}`를 반환

**AC-CORE-002 — SIGTERM 수신 시 graceful shutdown**
- **Given** `genied`가 `serving` 상태로 동작 중
- **When** 테스트가 `SIGTERM`을 해당 PID로 송신
- **Then** 프로세스는 3초 이내 exit 0으로 종료하고, 등록된 cleanup hook 3개가 모두 호출되었음이 로그로 확인됨

**AC-CORE-003 — 잘못된 YAML 설정 파일 거부**
- **Given** `GENIE_HOME/config.yaml`에 비-YAML 텍스트 (`"::: not yaml :::"`) 기록
- **When** `genied` 실행
- **Then** exit code 78, stderr에 `config parse error` ERROR 레벨 로그 1건, 헬스서버는 기동하지 않음

**AC-CORE-004 — 포트 충돌 시 실패**
- **Given** 다른 listener가 이미 `:17890`에 바인딩되어 있음
- **When** `genied` 실행
- **Then** exit code 78, stderr에 `health-port in use: 17890` 메시지

**AC-CORE-005 — cleanup hook panic 격리**
- **Given** 3개 cleanup hook이 등록되어 있고 그중 2번째가 `panic("boom")`을 발생
- **When** `SIGTERM` 송신
- **Then** exit code 1, 3개 hook이 모두 호출됨(패닉 이후에도 나머지 실행), 패닉 스택이 ERROR 로그에 포함됨

**AC-CORE-006 — 드레이닝 중 503 응답**
- **Given** `genied` state=`draining` (cleanup 진행 중 강제 고정)
- **When** `GET /healthz`
- **Then** 503 + body `{"status":"draining"}`

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
/ (repo root)
├── go.mod                          # module github.com/genieagent/genie (Go 1.22+)
├── cmd/genied/main.go              # 진입점: 15~30줄, argparse, build-time version
├── internal/core/
│   ├── bootstrap.go                # bootstrap() → context, config, logger
│   ├── shutdown.go                 # RegisterHook / RunAllHooks / 30s timeout
│   ├── state.go                    # atomic.Value 기반 state machine
│   └── exitcodes.go                # const ExitOK, ExitConfig=78, ExitHookPanic=1
├── internal/health/
│   ├── server.go                   # http.Server + /healthz
│   └── server_test.go              # integration test
└── internal/config/
    ├── bootstrap_config.go         # 최소 loader (본 SPEC 전용, CONFIG-001이 확장)
    └── bootstrap_config_test.go
```

### 6.2 핵심 타입 (초안)

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
```

### 6.3 의존성 (최소)

| 라이브러리 | 용도 | 근거 |
|----------|------|-----|
| `go.uber.org/zap` v1.27+ | 구조화 JSON 로거 | tech.md §3.1 |
| `gopkg.in/yaml.v3` | 설정 파일 파싱 | 표준 YAML 선택지 |
| stdlib `net/http` | 헬스서버 (외부 프레임워크 없음) | 최소주의 원칙 |
| stdlib `os/signal`, `context`, `sync/atomic` | 시그널 + 상태 | 표준 라이브러리 |

gin, viper, cobra 등은 본 SPEC에서 **의도적으로 미사용**. CONFIG-001(viper)과 CLI-001(cobra)에서 도입.

### 6.4 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1**: `TestBootstrap_SucceedsWithEmptyConfig` — AC-CORE-001 → 구현 없음 → 실패.
2. **RED #2**: `TestHealthz_ReturnsServingJSON` — AC-CORE-001 hand-off → 실패.
3. **RED #3**: `TestSIGTERM_InvokesHooks_ExitZero` — AC-CORE-002 → 실패.
4. ... (AC별 테스트 모두 RED 확인 후)
5. **GREEN**: `internal/core/*`, `internal/health/*` 최소 구현.
6. **REFACTOR**: state machine을 `sync/atomic.Value`로 일관화, logger를 모든 hook에 DI.

### 6.5 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, 시그널·포트·패닉 전부 integration test |
| **R**eadable | main.go 30줄 미만, package별 단일 책임 |
| **U**nified | `go fmt` + `golangci-lint` (errcheck, govet, staticcheck) |
| **S**ecured | 헬스 endpoint 인증 없음은 의도적 (localhost only, 기본 포트 바인드 `127.0.0.1`) |
| **T**rackable | `service="genied"`, SPEC-ID commit trailer 적용 |

---

## 7. Exit Code 계약 (Contract)

| 코드 | 의미 | 트리거 |
|-----|------|-------|
| `0` | 정상 종료 | SIGINT/SIGTERM 후 모든 hook 성공 |
| `1` | hook 실행 중 panic 발생 | REQ-CORE-009 |
| `78` (EX_CONFIG) | 설정 오류 | 파싱 실패, 포트 충돌 (REQ-CORE-006, REQ-CORE-010) |
| `64` (EX_USAGE) | (미래) 잘못된 CLI 플래그 | 본 SPEC 사용 안 함. 예약 |

---

## 8. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | (없음) | 본 SPEC이 Phase 0의 최초 SPEC |
| 후속 SPEC | SPEC-GENIE-CONFIG-001 | 계층형 설정으로 확장 |
| 후속 SPEC | SPEC-GENIE-TRANSPORT-001 | 동일 process 내 gRPC 등록 |
| 외부 | Go 1.22+ toolchain | `go build`, `go test` |
| 외부 | `go.uber.org/zap` | 구조화 로깅 |

---

## 9. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Go 버전 합의 실패 (tech.md 1.26+ vs 현실 1.22 최신 안정) | 중 | 중 | 본 SPEC에서 Go 1.22로 고정. tech.md는 로드맵 버전이므로 후속 SPEC이 필요 시 업그레이드 |
| R2 | `SIGTERM` 처리가 OS별로 상이 (Windows) | 중 | 낮 | 초기 타깃은 darwin/linux만. Windows 지원은 OUT OF SCOPE |
| R3 | 30초 graceful 타임아웃 부족 (LoRA 훈련 중 종료 등) | 낮 | 중 | 본 SPEC은 학습 hook 없음. 학습 SPEC들이 자체 cancellable 패턴 보장 |
| R4 | panic recovery가 stack trace를 분실 | 낮 | 중 | `debug.Stack()` 호출 보장, 테스트로 검증 (AC-CORE-005) |
| R5 | 기본 포트 17890이 사용 중인 환경에서 기본값 충돌 | 중 | 낮 | `GENIE_HEALTH_PORT` env override 제공 (REQ-CORE-012) |

---

## 10. 참고 (References)

### 10.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/structure.md` §1 (디렉토리), §3 (3-tier 계층), §9 (MoAI-ADK 상속)
- `.moai/project/tech.md` §1.2 (3-언어 매핑), §3.1 (Go 런타임 스택), §11.1 (개발환경)
- `.moai/project/product.md` §3.1 (4-Layer 아키텍처 상 "genied" 등장)
- `.moai/project/migration.md` §6.4 (v4.0 신규 작업 항목 중 "코어 런타임" 필요)

### 10.2 외부 참조

- **MoAI-ADK-Go** (외부 레포): `cmd/*/main.go` + `internal/core/bootstrap.go` graceful shutdown 패턴
- **go.uber.org/zap** v1.27: https://pkg.go.dev/go.uber.org/zap
- **sysexits.h** exit code 관례: FreeBSD `/usr/include/sysexits.h`

### 10.3 부속 문서

- `./research.md` — MoAI-ADK-Go 상속 가능 영역, Hermes/Claude Code 분석 결과
- `../ROADMAP.md` — 전체 Phase 계획

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

**End of SPEC-GENIE-CORE-001**
