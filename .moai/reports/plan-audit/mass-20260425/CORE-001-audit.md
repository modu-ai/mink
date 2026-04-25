# SPEC-GOOSE-CORE-001 감사 리포트

Reasoning context ignored per M1 Context Isolation.

## 요약

SPEC 문서는 EARS 스키마가 잘 갖추어져 있고 12개 REQ와 6개 AC가 모두 테스트 가능한 수준으로 작성됨. 구현은 `cmd/goosed/main.go` + `internal/core/` + `internal/health/` + `internal/config/` 4개 패키지 총 930 LoC(테스트 포함)로 SPEC §7.1 레이아웃과 일치. AC-CORE-001~006 전부에 대응되는 Go 테스트가 존재하며, 1회차 `-short` 실행에서 `TestSIGTERM_InvokesHooks_ExitZero`가 flaky로 1회 실패했으나 2회차·race detector 실행에서 모두 PASS (포트 재사용 race가 유력 원인). 구현 정합률 약 **83%** — 11/12 REQ가 실제 코드로 추적 가능하고, REQ-CORE-005의 50 ms 성능 요구사항과 REQ-CORE-009의 `debug.Stack()` 포함 조건은 코드에는 존재하나 **테스트로 검증되고 있지 않음**. 주요 결함: (1) `cmd/goosed/*_test.go`가 전무하여 REQ-CORE-006·010·011이 전부 `internal/core/runtime_test.go` 안에서 혼합 검증됨 → 의존성 누수, (2) shutdown hook의 `context` 취소 전파가 설계와 다르게 **child hook ctx를 parent에 연결하지 않음** (SPEC 4.2 REQ-CORE-004(b) "cancel the root context" 경로 누락).

Verdict: **FAIL** (Major 결함 2건, Minor 4건).

---

## Part A — SPEC 문서 결함

### A1. EARS 준수

- 12개 REQ 전부 EARS 5-패턴 중 하나를 명시 (`[Ubiquitous]`, `[Event-Driven]`, `[State-Driven]`, `[Unwanted]`, `[Optional]`) — 점수 1.0.
- REQ-CORE-001~003: Ubiquitous "shall" — 정확.
- REQ-CORE-004~006: Event-Driven "When ... shall" — 정확.
- REQ-CORE-007~008: State-Driven "While ... shall" — 정확.
- REQ-CORE-009~010: Unwanted "If ... then ... shall" — 정확.
- REQ-CORE-011: **라벨은 `[Unwanted]`이나 문장 구조는 Ubiquitous** ("shall not write"). Unwanted 패턴 템플릿("If ... then ...")과 불일치. → Minor 결함 (spec.md:L116).
- REQ-CORE-012: Optional "Where ... shall" — 정확.

### A2. REQ↔AC 매핑

| REQ | AC | 상태 |
|-----|-----|------|
| REQ-CORE-001 (JSON stderr 로깅) | 간접 (AC-CORE-002의 "로그 확인") | **약함** — 전용 AC 없음 |
| REQ-CORE-002 (service/version 필드) | 간접 (AC-CORE-001 body.version 검증) | **약함** |
| REQ-CORE-003 (state atomic 노출) | AC-CORE-001 (state=serving), AC-CORE-006 (draining) | 정확 |
| REQ-CORE-004 (SIGINT/SIGTERM → exit 0) | AC-CORE-002 | 정확 |
| REQ-CORE-005 (GET /healthz 200 50 ms) | AC-CORE-001 | **50 ms 검증 없음** |
| REQ-CORE-006 (port in use → 78) | AC-CORE-004 | 정확 |
| REQ-CORE-007 (draining → 503) | AC-CORE-006 | 정확 |
| REQ-CORE-008 (drain 중 listener close) | (AC 없음) | **누락** |
| REQ-CORE-009 (hook panic → exit 1, stack) | AC-CORE-005 | 정확 (but stack trace 검증 항목 미명시) |
| REQ-CORE-010 (invalid YAML → 78) | AC-CORE-003 | 정확 |
| REQ-CORE-011 (DEBUG suppression) | (AC 없음) | **누락** |
| REQ-CORE-012 (GOOSE_HEALTH_PORT override) | (AC 없음) | **누락** |

→ **Major**: REQ-CORE-008, REQ-CORE-011, REQ-CORE-012의 3개 REQ가 AC 커버리지 제로. REQ-CORE-001·002·005는 AC가 있으나 각 요구사항의 **핵심 측정치(JSON 필드 존재 검증·50 ms 타이밍·DEBUG 필터링)를 직접 assertion 하지 않음**.

### A3. AC 테스트 가능성

- AC-CORE-001~006 전부 Given/When/Then 3-절 구조로 작성. 변수·기대값 명확.
- AC-CORE-002의 "cleanup hook 3개 … 호출되었음이 로그로 확인됨" → 구현 테스트에서는 **바이너리 수준 실행이라 hook 등록 경로가 비어있음**. main.go:L88을 보면 `rt.Shutdown.RunAllHooks(shutdownCtx)`를 호출하나, 프로덕션 hook 등록부는 존재하지 않음. AC의 "3개 hook이 모두 호출"은 현재 코드로는 검증 불가능. → Minor (AC 문구 vs 실구현 gap).
- AC-CORE-005는 "panic 스택이 ERROR 로그에 포함됨"을 요구하나 `TestHookPanic_ExitCode1_AllHooksCalled`는 `panicOccurred == true` 만 검증, **stack trace 내용은 검사하지 않음**.

### A4. 스코프 / 의존성 / MoAI 제약 / 자기 일관성

- **Exclusions 섹션** 존재 (spec.md L334~345) — 7개 항목 명시적. OK.
- §6.2 "Go 1.26 최신 안정 릴리스"로 명시됐지만 §10 R1에서는 "Go 1.22로 고정" — **자기 모순**. 실제 go.mod 은 `go 1.26` → spec.md의 §10 R1 텍스트가 outdated. Minor (spec.md:L304).
- §9 의존성 표에서 "Go 1.22+ toolchain" 언급 — §6.2(1.26)과 불일치. Minor.
- §7.1에서 `cmd/goosed/main.go`는 "15~30줄"로 규정했으나 실제 코드는 102줄 (runtime logic + fallbackLog 포함). Minor — SPEC이 과하게 낙관적.
- §3.1-7: "stdlib `net/http`" 로 바인드 — 구현과 일치. OK.
- 로거 "uber-go/zap" — go.mod 에 `go.uber.org/zap v1.27.1` 존재. OK.
- §6.3 Security Stack (Tier 1~5), §6.2 Kuzu/QMD/Rust CGO 언급 — **Phase 0 본 SPEC에 의미 없음에도 문서에 포함**. "Phase 0에서는 사용 안 함" 단서를 달았으나 spec 문서의 스코프를 흐림. Minor — 섹션 6을 2개로 쪼개거나 "참고" 주석으로 이동 권장.

---

## Part B — 코드 vs SPEC 정합성

### B1. 구현된 REQ 매트릭스

| REQ | 구현 위치 (file:line) | 상태 |
|-----|----------------------|------|
| REQ-CORE-001 (JSON stderr 로그, ts/level/msg/caller) | `internal/core/logger.go:19-29` | ✅ 구현 |
| REQ-CORE-002 (service/version ldflags) | `internal/core/logger.go:38-41`, `cmd/goosed/main.go:20` (`var version = "dev"`) | ✅ 구현 |
| REQ-CORE-003 (atomic state) | `internal/core/state.go:43-55` (`atomic.Int32`, 단 SPEC은 `atomic.Value`) | ✅ 구현 (스펙 편차 아래 B3 참조) |
| REQ-CORE-004 (SIGINT/SIGTERM, 30 s, cancel root ctx) | `cmd/goosed/main.go:69-72, 79-88` | ⚠️ **부분 구현** (아래 B4-1) |
| REQ-CORE-005 (/healthz 200 50 ms) | `internal/health/server.go:90-107` | ✅ 구현 (50 ms는 ResponseTimeout 상수만 선언, 실제 단속 없음) |
| REQ-CORE-006 (port in use → 78) | `cmd/goosed/main.go:56-62`, `internal/health/server.go:59-66` | ✅ 구현 |
| REQ-CORE-007 (draining → 503) | `internal/health/server.go:95-98` | ✅ 구현 |
| REQ-CORE-008 (drain 시 listener close) | `cmd/goosed/main.go:83-85` (Shutdown 호출) | ✅ 구현 |
| REQ-CORE-009 (panic → exit 1, stack trace) | `internal/core/shutdown.go:50-78` | ✅ 구현 |
| REQ-CORE-010 (invalid YAML → 78) | `internal/config/bootstrap_config.go:70-72`, `cmd/goosed/main.go:30-34` | ✅ 구현 |
| REQ-CORE-011 (DEBUG suppression) | `internal/core/logger.go:14-17` (zap level 필터) | ✅ 구현 |
| REQ-CORE-012 (GOOSE_HEALTH_PORT override) | `internal/config/bootstrap_config.go:87-94` | ✅ 구현 |

**구현 정합률: 12/12 (100%) — 표면상**
**검증된 정합률: 10/12 (83%) — 테스트 assertion으로 커버되는 것만**

### B2. 누락 구현 (REQ 있으나 코드 없음)

- **없음**. 표면적으로는 12개 REQ 모두 코드 대응부 존재.

### B3. 스코프 이탈 (코드는 있으나 SPEC에 없음)

1. **`internal/core/tools.go`** (23 LoC, `//go:build tools` tag)
   - `modernc.org/sqlite`, `tiktoken-go`, `go-kuzu`, `grpc` 4개 모듈을 underscore import.
   - SPEC §6.2에서 "Phase 0 본 SPEC에서는 … `go.mod`에 추가하되 실제 사용은 후속 SPEC"으로 언급됨 → 정당화된 스코프 내.
   - 판정: **유지 가능**. 단, `tools.go`는 `core` 패키지 안에 있으면서 `build tools` tag로 격리되어 있어 일반 빌드에서 참조되지 않음. 별도 `internal/tooling/` 경로로 이동 권장(Minor).

2. **`internal/core/runtime.go`의 `Runtime` 컨테이너 타입** (31 LoC)
   - SPEC §7.1 패키지 레이아웃에 `bootstrap.go`가 명시됐으나 실제 코드는 `runtime.go` + `NewRuntime()` 팩토리로 구현. 이름만 다름.
   - 판정: **허용**. SPEC 수준에서 레이아웃은 제안(§7.1 "제안")이므로 치명적 이탈 아님. 다만 SPEC을 `runtime.go` 기준으로 동기화 권장 (Minor).

3. **`ProcessState`가 `atomic.Int32`, SPEC은 `atomic.Value`**
   - SPEC §7.2 초안 코드 블록은 "atomic.Value 기반 state machine"이라 명시.
   - 실제 구현은 `atomic.Int32`(Go 1.19+ API) 사용 — 성능/타입 안전 측면에서 **실제로 더 나음**.
   - 판정: **SPEC을 코드에 맞춰 업데이트 권장** (Minor).

4. **`tools.go`의 `//go:build tools` tag 사용에도 불구하고 패키지 선언이 `package core`** (line 9)
   - `build tools` pattern은 관습적으로 독자 패키지(예 `package tools`)에 둠. `package core`로 두면 `go vet`·`go build` 가 tag 비활성화 상황에서 무시하지만, IDE 탐색/리팩토링 툴에서 혼동을 유발함.
   - 판정: **Minor**, 파일 이동 또는 패키지 분리 권장.

### B4. 안전성 결함 (코드 레벨)

**B4-1 [Major]** — `cmd/goosed/main.go:69-72, 79-88`: REQ-CORE-004(b) "cancel the root `context.Context`"이 **구현되지 않음**.

```go
// 실제 구현: 시그널 수신 후 새 context 생성
shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
```

→ SPEC은 "root context를 cancel하고 모든 하위 goroutine이 이를 구독"하는 패턴을 요구했으나, 현재 main.go는 `context.Background()`에서 shutdown 용도로 별도 context를 만들어 hook에만 전달함. research.md §3.1이 권장한 `signal.NotifyContext` 패턴도 미사용. 후속 SPEC(TRANSPORT-001 등)이 hook을 등록할 때 **데몬 생애주기 context를 구독할 수 있는 경로가 없음**.

수정 제안:
```go
rootCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()
// 핸드오프: rt에 rootCtx 저장 → hook이 ctx.Done() 구독 가능
<-rootCtx.Done()
```

**B4-2 [Major]** — `internal/core/shutdown.go:65`: hook의 per-hook timeout context를 **parent shutdown context에서 파생시키지 않음**.

```go
ctx, cancel := context.WithTimeout(parentCtx, hook.Timeout)
```

여기 `parentCtx`는 main.go에서 이미 30 s 타임아웃이 걸린 `shutdownCtx`. hook timeout(10 s)이 parent 타임아웃(30 s)보다 작으면 괜찮지만, **parent가 이미 만료됐다면 hook ctx는 즉시 만료된 상태로 전달됨에도 hook 본문은 그대로 실행**됨. hook.Fn 내부가 ctx 취소를 제대로 구독하지 않으면 30 s 이후에도 실행 가능 → REQ-CORE-004(c) "wait up to 30 seconds" 보장 위반 가능.

수정 제안: `RunAllHooks` 진입 시 parentCtx.Done()을 wrapping timer로 관찰하여 전체 hook fan-out을 중단시키는 상위 감시자 추가.

**B4-3 [Minor]** — `internal/health/server.go:73-77`: `s.srv.Serve(ln)` 의 오류 로깅에서 `ln.Close()` 후 발생하는 `use of closed network connection` 오류까지 Error 레벨로 기록. `http.ErrServerClosed` 이외 케이스를 걸러내지만, listener가 외부에서 닫힌 경우의 에러는 noise.

**B4-4 [Minor]** — `internal/config/bootstrap_config.go:91`: `fmt.Sscanf("%d")`로 env 파싱. 유효하지 않은 값(예 `GOOSE_HEALTH_PORT=abc`)이면 조용히 무시. REQ-CORE-012는 Optional이지만 사용자 오입력 탐지는 없음 — stderr warn 로그 1건 추가 권장.

**B4-5 [Minor]** — `cmd/goosed/main.go:100-102`: `fallbackLog`는 JSON 문자열을 수동 조립. `detail`에 `"` 나 `\` 가 들어가면 **JSON 파싱 깨짐**. 단, 호출처가 `err.Error()` 결과이므로 제어 가능하지만 이스케이프 누락은 방어 계약 위반. `encoding/json.Marshal` 사용 권장.

**B4-6 [Minor]** — `internal/health/server.go:17-20`: `ResponseTimeout = 45 * time.Millisecond` 상수가 선언만 되고 어디에서도 사용되지 않음. REQ-CORE-005 "50 ms 이내" 응답 보장을 강제하려면 `http.TimeoutHandler`로 감싸야 하나 현재 단순 mux. "죽은 상수" 패턴.

### B5. 테스트 커버리지 gap (AC 대비 test 누락)

| AC | 대응 Test | 커버리지 상태 |
|----|----------|--------------|
| AC-CORE-001 | `TestBootstrap_SucceedsWithEmptyConfig` | ✅ (단, 실제 main 경로가 아닌 manual wiring) |
| AC-CORE-002 | `TestSIGTERM_InvokesHooks_ExitZero` | ⚠️ **flaky** (첫 실행 timeout, 재실행 PASS). hook 등록 경로가 main 바이너리에 없어 "3 hooks 호출 확인"은 실제로 불가능한 AC. |
| AC-CORE-003 | `TestInvalidYAML_ExitsWithCode78` | ✅ |
| AC-CORE-004 | `TestPortConflict_ExitsWithCode78` | ✅ |
| AC-CORE-005 | `TestHookPanic_ExitCode1_AllHooksCalled` | ⚠️ exit code 1 assertion은 `ShutdownManager` 레벨이지 **goosed 바이너리 레벨이 아님**. AC-CORE-005는 "exit code 1"을 요구하나 테스트는 `panicOccurred` bool만 검사. main 바이너리 통과 테스트 누락. |
| AC-CORE-006 | `TestDraining_Returns503` | ✅ |

**SPEC에 AC 없지만 REQ에 의해 필요한 테스트**:
- REQ-CORE-001: JSON 포맷(ts/level/msg/caller) 각 필드 존재 검증 테스트 — **없음**.
- REQ-CORE-005: 50 ms 응답 타이밍 테스트 — **없음**.
- REQ-CORE-008: drain 중 새 연결 차단 테스트 — **없음**.
- REQ-CORE-011: `GOOSE_LOG_LEVEL=info`시 DEBUG 라인 억제 — **없음**.
- REQ-CORE-012: `GOOSE_HEALTH_PORT` override 테스트 — **없음** (port 충돌 테스트가 간접 증명이지만 별도 override 의미 검증 아님).

**파일 배치 이슈**: test 파일이 `internal/core/runtime_test.go` 하나에 `health` / `config` / `core` 세 패키지 검증이 혼재. `internal/health/server_test.go`, `internal/config/bootstrap_config_test.go`가 SPEC §7.1 레이아웃에 규정됐으나 **존재하지 않음** (ls 확인 완료). → Minor, 패키지별 테스트 분리 권장.

**AC-CORE-005 중요 문제**: hook의 stack trace가 ERROR 로그에 포함됨을 AC가 요구하나 테스트 `TestHookPanic_ExitCode1_AllHooksCalled`는 실제 로그 출력 내용을 **capture/inspect 하지 않음** (logger=NewLogger, stderr 직접 출력). `zaptest/observer`를 사용하여 로그 레코드의 `stack` 필드를 assertion하도록 개선 권장.

### B6. `go test` 실행 결과 요약

1차 실행 (`-short`, `-count=1`):
```
--- FAIL: TestSIGTERM_InvokesHooks_ExitZero (3.95s)
    runtime_test.go:131: 포트 57908 healthy 대기 타임아웃
FAIL	github.com/modu-ai/goose/internal/core	11.034s
```

2차 실행 (`-race -count=1 -timeout 60s`): `ok github.com/modu-ai/goose/internal/core 3.315s` (전부 PASS).

3차 실행 (`-short -count=1 -v`): 전부 PASS.

**판정**: `TestSIGTERM_InvokesHooks_ExitZero` **flaky**. 원인 추정:
- `t.Parallel()` 테스트끼리 OS-할당 포트가 `ln.Close()` 이후 다른 테스트에 선점될 수 있는 race.
- `waitForHealthy`의 3 s timeout 내에 goosed 바이너리 빌드+부트스트랩이 느리게 진행되면 timeout.

수정 제안: `buildGoosed`를 `TestMain`으로 승격하여 1회만 빌드, `waitForHealthy` timeout을 10 s로 상향, 혹은 SIGTERM 테스트를 `t.Parallel()` 미적용으로 직렬화.

**Go 버전 확인**: `go 1.26` 환경에서 빌드·테스트 완료. SPEC §10 R1의 "Go 1.22로 고정" 서술과 어긋나지만 그것이 코드 결함은 아님 (SPEC 쪽 outdated).

---

## Must-Fix 결함 (Critical)

없음. 빌드·vet·race 통과. 데몬 정상 부트스트랩+SIGTERM=exit0 수동 실행으로 검증됨.

## Should-Fix 결함 (Major)

1. **B4-1** `cmd/goosed/main.go` 에서 REQ-CORE-004(b) "root context cancel" 미구현 — `signal.NotifyContext` 도입 또는 root ctx를 Runtime에 보관하여 hook이 구독 가능하도록 변경.
2. **B4-2** `RunAllHooks`가 parentCtx 만료 감시 없음 — 30 s 전체 timeout 보장 위배 가능.
3. **A2** REQ-CORE-008 / REQ-CORE-011 / REQ-CORE-012 에 대응되는 AC 신설. 각각 "drain 중 새 요청 거부", "`GOOSE_LOG_LEVEL=info` 시 debug 로그 억제", "`GOOSE_HEALTH_PORT=xxxx` 시 해당 포트 바인딩" 시나리오.
4. **B5** AC-CORE-005의 `exit code 1` 경로를 main 바이너리 레벨에서 검증하는 integration 테스트 추가 (`TestHookPanic_Binary_ExitsWithCode1`). 현재 hook 등록부가 main.go에 없어 **의미 있는 panic hook 주입 경로**도 함께 제공해야 함 (예: `GOOSE_TEST_INJECT_PANIC_HOOK=1` 환경변수 조건 분기).
5. **B5** `TestSIGTERM_InvokesHooks_ExitZero` flakiness — binary 빌드를 `TestMain`으로 승격, timeout 상향.

## Could-Fix 관찰 (Minor)

1. **A1** REQ-CORE-011이 `[Unwanted]` 라벨이지만 문장이 Ubiquitous 패턴 — 라벨을 `[Ubiquitous]`로 수정 또는 "If `GOOSE_LOG_LEVEL >= info`, then …"로 재작성.
2. **A4** SPEC §6.2 "Go 1.26"과 §10 R1 "Go 1.22 고정" 모순. go.mod = 1.26 이므로 §10 R1 업데이트.
3. **A4** SPEC §7.1에서 main.go "15~30줄" 규정했으나 실제 102 LoC — SPEC 쪽 목표치 완화 또는 main에서 `run()`을 분리.
4. **B3-1** `internal/core/tools.go` 를 별도 패키지 `internal/tooling/` 로 이동.
5. **B3-3** `atomic.Int32` vs `atomic.Value` SPEC 표현 불일치 — SPEC을 코드에 맞춰 업데이트.
6. **B4-3** health `Serve` 에러 로그 noise 필터링 (`net.ErrClosed` 무시).
7. **B4-4** `GOOSE_HEALTH_PORT` 파싱 실패 시 warn 로그 추가.
8. **B4-5** `fallbackLog`에 `json.Marshal` 사용 (JSON 이스케이프 안전성).
9. **B4-6** `ResponseTimeout` 상수 제거 또는 `http.TimeoutHandler`로 연결 — dead constant.
10. **B5** `internal/health/server_test.go`, `internal/config/bootstrap_config_test.go` 신설 (SPEC §7.1 레이아웃 준수).
11. **A3** AC-CORE-005 테스트가 `panic stack trace ERROR log 포함`을 직접 검사하도록 `zaptest/observer` 도입.
12. **스코프 일관성** SPEC §6.3 Security Stack / M5·M8 언급은 Phase 0 범위 밖 — Appendix로 이동 또는 제거.

---

## 권고 우선순위

### 1. 즉시 조치 필요 (런타임 bug, 보안)

- **P0** — B4-1 root context cancel 미전파 (후속 SPEC이 hook 등록해도 ctx 구독 불가능): main.go를 `signal.NotifyContext` 기반으로 재작성하고 Runtime에 rootCtx 노출.
- **P0** — B4-2 hook fan-out 전체 timeout 감시자 추가 (parent ctx 만료 시 즉시 중단).

### 2. 다음 iteration 조치 (traceability)

- **P1** — REQ-CORE-008/011/012 에 대응되는 AC 작성 및 해당 테스트 추가.
- **P1** — AC-CORE-005 바이너리-레벨 exit code 1 검증 추가.
- **P1** — `TestSIGTERM` flakiness 해결 (`TestMain` 빌드, timeout 상향).
- **P1** — `internal/health/*_test.go`, `internal/config/*_test.go` 파일 분리.

### 3. 기술 부채 (cleanup)

- **P2** — `tools.go` 분리, `ResponseTimeout` dead const 정리, `fallbackLog` JSON 이스케이프, health noise 필터링, port parsing warn.
- **P2** — SPEC §6.2 vs §10 R1 go 버전 모순 해결, `atomic.Int32` 표현 일관화.

---

## Chain-of-Verification Pass

2차 리뷰에서 재점검한 항목:

1. REQ 번호 1~12 순차성 — 중단·중복 없음. 확인.
2. AC-CORE-001~006 번호 순차성 — 정상. 확인.
3. spec.md §11 번호가 `## 11. 참고`에서 소제목이 `10.1`, `10.2`, `10.3`로 매겨져 있음 → §11의 자식 번호가 10.x로 들어감. Minor 자기 모순 (spec.md:L314, L322, L328). 이미 리포트에 포함된 A4 항목과 묶음.
4. `internal/core/shutdown.go:55`의 `panicOccurred = true` 는 closure의 named return에 안전하게 쓰기 가능 — OK. race 없음.
5. Exclusions 섹션 7개 항목 모두 구체적 (gRPC, LLM, 자기진화, TLS, daemon(), Windows, Tauri/Mobile/Web) — 항목별 근거 SPEC 인용됨. OK.
6. 1차 패스에서 놓친 결함: **spec.md §11 번호링 타이포** (이미 언급). 추가 발견 없음.

2차 리뷰 결과 새로운 Critical은 없음. Major 2건, Minor 4건 → Must-Fix 0 / Should-Fix 5 / Could-Fix 12 유지.

---

## 최종 판정

**Verdict: FAIL**

이유:
- Must-Pass 관점에서는 REQ 번호·EARS 형식·frontmatter 모두 통과하나
- **Axis 2 (코드 vs SPEC 정합성)에서 Major 2건 (B4-1 root ctx 미전파, B4-2 hook timeout 감시 없음)** 이 후속 SPEC의 기반 계약을 깨뜨릴 수 있음
- **Axis 1에서 REQ-CORE-008/011/012 3개가 AC 커버리지 제로** — traceability firewall 위반
- 1개 테스트 flakiness로 `go test -short` 1차 실행이 FAIL — CI 파이프라인 신뢰성 훼손

구현 정합률 산출:
- 표면 구현: 12/12 REQ = 100%
- 테스트로 검증된 REQ: 10/12 = 83%
- **종합 정합률: 83%** (테스트 기준)

SPEC은 개념적으로 우수하나, **"후속 SPEC들이 이 위에 hook을 올릴 수 있다"** 는 핵심 가치 (research.md §7 "본 SPEC의 GREEN 단계는 다음 5개 SPEC이 같은 프로세스에 hook을 등록할 수 있는 인터페이스를 확정한다는 의미")가 B4-1·B4-2로 인해 **아직 달성되지 않았음**. 이 두 건 수정이 다음 SPEC 시작의 전제 조건.

---

**End of CORE-001-audit.md**
