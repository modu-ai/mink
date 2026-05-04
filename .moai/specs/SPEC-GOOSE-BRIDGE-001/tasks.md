---
spec: SPEC-GOOSE-BRIDGE-001
version: 0.2.0
methodology: TDD (RED-GREEN-REFACTOR)
harness: standard
created_at: 2026-05-04
---

# Task Decomposition — SPEC-GOOSE-BRIDGE-001

> Daemon ↔ Web UI Local Bridge (v0.2.0). 본 문서는 spec.md §9 TDD 전략과 §7 AC 16 개 + §3 IN scope 10 항목을 milestone 단위 atomic task 로 분해한다. WEBUI-001 v0.2.1 amendment §11 의 결정 사항 (shared listener + path 분리, 명시 POST `/bridge/login`) 을 implementation 가이드로 사용한다.

## Acceptance Criteria → Task Mapping

| AC | REQ | Milestone-Task | 상태 |
|----|-----|----------------|------|
| AC-BR-001 | REQ-BR-001 | M0-T1 ~ M0-T4 (types + registry) | TODO |
| AC-BR-002 | REQ-BR-002 | M1-T1 ~ M1-T3 (cookie lifecycle) | TODO |
| AC-BR-003 | REQ-BR-003 | M2-T1 (single listener mux) | TODO |
| AC-BR-004 | REQ-BR-004 | M5-T2 (OTel metrics) | TODO |
| AC-BR-005 | REQ-BR-005 | M0-T5 (loopback bind verify) | TODO |
| AC-BR-006 | REQ-BR-006 | M2-T3 ~ M2-T5 (inbound validation) | TODO |
| AC-BR-007 | REQ-BR-007 | M3-T1 ~ M3-T3 (outbound streaming) | TODO |
| AC-BR-008 | REQ-BR-008 | M3-T4 (permission roundtrip) | TODO |
| AC-BR-009 | REQ-BR-009 | M4-T1 ~ M4-T3 (offline buffer + replay) | TODO |
| AC-BR-010 | REQ-BR-010 | M4-T4 ~ M4-T5 (flush-gate watermark) | TODO |
| AC-BR-011 | REQ-BR-011 | M2-T6 (SSE fallback) | TODO |
| AC-BR-012 | REQ-BR-014 | M1-T4 (auth failure paths) | TODO |
| AC-BR-013 | REQ-BR-015 | M2-T7 (10MB inbound limit) | TODO |
| AC-BR-014 | REQ-BR-016 (v0.2.0) | M1-T5 (logout + revocation) | TODO |
| AC-BR-015 | REQ-BR-017 | M4-T6 (session resume) | TODO |
| AC-BR-016 | REQ-BR-018 (v0.2.0) | M5-T1 (reconnect rate-limit) | TODO |

총 16 AC × 5 Milestone × 23+ atomic task.

## Milestones

### M0 — Foundation (types + listener scaffold + loopback verify)

목표: spec.md §6.3 의 Go 타입 시그니처 그대로 구현. HTTP listener bind 검증. 세션 registry skeleton (CRUD 만, 인증/wire 없음).

**Files**:
- `internal/bridge/types.go` (신규, ~150 LoC) — `Bridge` interface, `WebUISession`, `Transport`, `SessionState`, `BindAddress`, `InboundMessage`, `OutboundMessage`, `FlushGate`, `CloseCode` 상수.
- `internal/bridge/server.go` (신규, ~120 LoC) — `Bridge` interface 구현체 `bridgeServer`. `Start(ctx)`, `Stop(ctx)`, `Sessions()`, `Metrics()` 의 skeleton. HTTP listener bind 만 (handler mount 는 M2).
- `internal/bridge/bind.go` (신규, ~60 LoC) — `verifyLoopbackBind(addr string) error`. `127.0.0.1`, `::1`, `localhost` 만 허용.
- `internal/bridge/registry.go` (신규, ~100 LoC) — in-memory session registry. `Add`, `Get`, `Remove`, `Snapshot`. mutex 보호.
- `internal/bridge/types_test.go`, `bind_test.go`, `registry_test.go` (RED 진입 테스트).

**Tasks (atomic)**:

- M0-T1: `Transport` enum + `SessionState` enum + `CloseCode` 상수 정의 (spec.md §6.3 그대로).
- M0-T2: `WebUISession` struct + `InboundMessage` / `OutboundMessage` struct.
- M0-T3: `Bridge` interface + `bridgeServer` skeleton (Start/Stop no-op).
- M0-T4: `Registry` 구조체 + `Add/Get/Remove/Snapshot` + `Registry_test` (concurrent-safe assertions).
- M0-T5: `verifyLoopbackBind` + `verifyLoopbackBind_test` (REQ-BR-005, AC-BR-005). `0.0.0.0:8091` → `ErrNonLoopbackBind`. `127.0.0.1:8091` / `::1:8091` / `localhost:8091` → nil.

**TDD entry order**:
1. M0-T5 first (단순, 검증 로직만, 즉시 실패 케이스 작성 가능).
2. M0-T4 (Registry CRUD).
3. M0-T1 ~ M0-T3 (skeleton, dependent on M0-T4).

**Exit criteria**:
- `go build ./internal/bridge/...` clean.
- `go test -race ./internal/bridge/...` PASS.
- `go vet ./internal/bridge/...` clean.
- `verifyLoopbackBind` 테스트 5+ 케이스 (3 valid + 2 invalid).
- coverage ≥ 80% on M0 files.

### M1 — Authentication (cookie + CSRF + auth failure)

목표: same-origin 인증 파이프라인. 로컬 세션 쿠키, CSRF double-submit, 만료/변조 거부.

**Files**:
- `internal/bridge/auth.go` (신규, ~180 LoC) — `IssueSessionCookie`, `VerifySessionCookie`, `IssueCSRFToken`, `VerifyCSRFToken`. HMAC-SHA256 기반.
- `internal/bridge/login.go` (신규, ~120 LoC) — `POST /bridge/login` 핸들러 (WEBUI v0.2.1 OI-A 결정 명시 POST), `POST /bridge/logout` 핸들러.
- `internal/bridge/auth_test.go`, `login_test.go`.

**Tasks**:

- M1-T1: cookie HMAC issue + verify + 24h 만료 (REQ-BR-002, AC-BR-002).
- M1-T2: CSRF token issue + double-submit verify (상수시간 비교).
- M1-T3: `POST /bridge/login` 핸들러 — `intent: first_install|resume` body, 응답 `Set-Cookie` + `csrf_token` (WEBUI OI-A).
- M1-T4: 만료/변조 쿠키 거부 — WebSocket close 4401, SSE/POST 401 (REQ-BR-014, AC-BR-012).
- M1-T5: `POST /bridge/logout` — 같은 쿠키의 모든 세션 2s 내 close 4403 (REQ-BR-016 v0.2.0, AC-BR-014).

**Exit criteria**:
- 16+ test cases (cookie issue/verify/expire, CSRF match/mismatch, login 200, logout 즉시 close).
- `go test -race` PASS.
- `coverage(auth.go + login.go) ≥ 85%`.

### M2 — Single listener mux (WebSocket + SSE + POST)

목표: 동일 HTTP listener 위에 path-based dispatching. WebSocket upgrade, SSE stream, HTTP POST inbound. WEBUI v0.2.1 OI-B 결정에 따라 `/webui/*` 정적 mount 진입점도 같은 mux 에 마련 (정적 핸들러 자체는 WEBUI-001 run 에서 추가).

**Files**:
- `internal/bridge/mux.go` (신규, ~150 LoC) — `BuildMux` 함수, path 라우팅 표.
- `internal/bridge/ws.go` (신규, ~250 LoC) — WebSocket upgrade + frame loop. `coder/websocket` v1.8+ 사용.
- `internal/bridge/sse.go` (신규, ~180 LoC) — SSE handler + `event:` writer.
- `internal/bridge/inbound.go` (신규, ~120 LoC) — `POST /bridge/inbound` (SSE fallback 의 inbound).
- `internal/bridge/mux_test.go`, `ws_test.go`, `sse_test.go`, `inbound_test.go`.

**Tasks**:

- M2-T1: `BuildMux` — `/bridge/ws`, `/bridge/stream`, `/bridge/inbound`, `/bridge/login`, `/bridge/logout` 라우팅 (REQ-BR-003, AC-BR-003).
- M2-T2: WebSocket upgrade — `coder/websocket.Accept`, frame read loop, close code propagation.
- M2-T3: 인바운드 검증 파이프라인 — cookie + CSRF + Origin/Host 검증, p95 ≤ 20ms 측정 (REQ-BR-006, AC-BR-006).
- M2-T4: 인바운드 message 타입 dispatch (`chat | attachment | permission_response | control`) → QueryEngine adapter stub.
- M2-T5: `Origin`/`Host` 검증 helper (loopback 만 허용).
- M2-T6: SSE handler + `text/event-stream` MIME + `Last-Event-ID` 처리 (REQ-BR-011, AC-BR-011).
- M2-T7: 10MB 인바운드 한계 — close 4413 / HTTP 413 (REQ-BR-015, AC-BR-013).

**Exit criteria**:
- httptest 기반 통합 테스트 (실제 WebSocket 클라이언트 dial → upgrade → frame send/recv).
- p95 ≤ 20ms / p99 ≤ 50ms 측정 테스트 (4-core x86_64 host 가정, 단일 프로세스).
- coverage ≥ 80%.

### M3 — Outbound streaming + permission roundtrip

목표: daemon → 브라우저 outbound 청크 emit. permission_request/response 라운드트립.

**Files**:
- `internal/bridge/outbound.go` (신규, ~200 LoC) — outbound chunk dispatcher, sequence 번호 부여, transport 별 emit.
- `internal/bridge/permission.go` (신규, ~130 LoC) — `OutboundPermissionRequest` 발행, 60s timeout, default-deny.
- `internal/bridge/outbound_test.go`, `permission_test.go`.

**Tasks**:

- M3-T1: Outbound chunk dispatcher — QueryEngine 의 chunk channel 을 받아 transport 에 emit.
- M3-T2: Sequence 번호 (uint64) 단조 증가, replay 정렬용.
- M3-T3: p95 ≤ 15ms latency 테스트 (REQ-BR-007, AC-BR-007).
- M3-T4: `OutboundPermissionRequest` + 60s timeout + default-deny + `OutboundStatus{permission_timeout}` (REQ-BR-008, AC-BR-008).

**Exit criteria**:
- chunk 순서 보장 테스트 (random delay injection 후에도 emit 순서 == 수신 순서).
- 60s timeout 테스트 (timer mock).
- coverage ≥ 80%.

### M4 — Offline buffering + flush-gate + reconnect/replay

목표: 세션이 reconnecting 상태일 때 outbound 버퍼링, watermark 기반 flush-gate, resume 시 replay.

**Files**:
- `internal/bridge/buffer.go` (신규, ~180 LoC) — per-session ring buffer (4MB or 500 messages, oldest-drop).
- `internal/bridge/flushgate.go` (신규, ~130 LoC) — `FlushGate` 인터페이스 구현. high-watermark 256KB / low-watermark 64KB.
- `internal/bridge/resume.go` (신규, ~100 LoC) — `X-Last-Sequence` / `Last-Event-ID` 처리, replay loop.
- `internal/bridge/buffer_test.go`, `flushgate_test.go`, `resume_test.go`.

**Tasks**:

- M4-T1: ring buffer 구현 (4MB or 500 msg, oldest-drop) (REQ-BR-009, AC-BR-009).
- M4-T2: 24h 윈도우 내 같은 cookie 로 reconnect 시 replay.
- M4-T3: 버퍼 누락·중복 검증 테스트 (sequence 갭 0).
- M4-T4: `FlushGate.Stalled / Wait / ObserveWrite / ObserveDrain` 구현 (REQ-BR-010, AC-BR-010).
- M4-T5: 256KB high-watermark → emit 차단, 64KB low → resume.
- M4-T6: WebSocket `X-Last-Sequence` + SSE `Last-Event-ID` resume (REQ-BR-017, AC-BR-015).

**Exit criteria**:
- 통합 테스트: write storm → flush-gate stall → drain 시나리오.
- 통합 테스트: tab background → reconnect → replay 5 청크.
- coverage ≥ 80%.

### M5 — Reconnect rate-limit + OTel metrics

목표: 클라이언트 reconnect 정책 서버측 enforcement, OpenTelemetry 메트릭 노출.

**Files**:
- `internal/bridge/ratelimit.go` (신규, ~140 LoC) — 10 연속 실패 후 4401, 분당 60회 한도 → 4429.
- `internal/bridge/metrics.go` (신규, ~120 LoC) — OTel Meter + Counter/Gauge 등록.
- `internal/bridge/ratelimit_test.go`, `metrics_test.go`.

**Tasks**:

- M5-T1: reconnect rate-limit (10 연속 실패 → 4401, 분당 60회 → 4429) (REQ-BR-018, AC-BR-016).
- M5-T2: OTel 메트릭 — `bridge.sessions.active`, `bridge.messages.inbound.total`, `bridge.messages.outbound.total`, `bridge.flush_gate.stalls`, `bridge.reconnect.attempts` (REQ-BR-004, AC-BR-004).

**Exit criteria**:
- OTel exporter (memory) 에 메트릭 dump 후 assertion.
- coverage ≥ 80%.
- 전체 `internal/bridge/` coverage ≥ 85%.

## TDD Entry Order (전체)

1. M0-T5: `verifyLoopbackBind` (REQ-BR-005, AC-BR-005) — 가장 단순한 RED 진입.
2. M0-T4: Registry CRUD (REQ-BR-001 부분, AC-BR-001 준비).
3. M0-T1~T3: types + bridgeServer skeleton.
4. M1-T1~T2: HMAC cookie + CSRF.
5. M1-T3: `POST /bridge/login` (WEBUI OI-A).
6. M1-T4~T5: auth failure + logout.
7. M2-T1: BuildMux + path 라우팅.
8. M2-T5: Origin/Host 검증.
9. M2-T2~T3: WebSocket upgrade + 인바운드 검증.
10. M2-T7: 10MB 한계.
11. M2-T6: SSE handler + Last-Event-ID.
12. M2-T4: 인바운드 dispatch.
13. M3-T1~T3: outbound dispatcher + sequence + latency.
14. M3-T4: permission roundtrip.
15. M4-T1~T3: ring buffer + replay.
16. M4-T4~T5: flush-gate.
17. M4-T6: resume.
18. M5-T1: reconnect rate-limit.
19. M5-T2: OTel metrics.

## Concurrent Implementation Strategy

- M0 / M1 / M2 의 일부는 병렬 가능 (다른 파일, 다른 책임). M3 / M4 / M5 는 순차 (하위 milestone 의 빌드/테스트 통과 전제).
- 단일 PR 단위 권장: M0 단독, M1 단독, M2-T1~T7 묶음, M3 묶음, M4-T1~T3 / M4-T4~T6 분리, M5 묶음.
- 총 6~7 PR 예상.

## External Dependencies (go.mod 추가 후보)

| Package | 용도 | 도입 milestone |
|---------|------|----------------|
| `github.com/coder/websocket` v1.8+ | WebSocket upgrade + frame I/O | M2 |
| `go.opentelemetry.io/otel` v1.26+ | 메트릭 (이미 프로젝트 의존성일 수 있음) | M5 |
| `github.com/jonboulle/clockwork` v0.4+ | mock clock (cookie 만료 / 60s timeout / rate-limit) | M1, M3, M5 |

`crypto/rand`, `net/http`, `time`, `sync`, `errors` 는 표준 라이브러리만 사용.

## Out of Scope (본 SPEC 내에서 다루지 않음)

- Web UI 정적 번들 (WEBUI-001 run 의 책임)
- 인스톨 wizard 관리 API (`/webui/*`) 핸들러 (WEBUI-001 run)
- channel-aware Confirmer 라우팅 (HOOK-001 amendment 후속)
- Mobile remote bridge (BRIDGE-002 별도 SPEC)
- TLS / 인증서 / 외부 노출 (loopback 전용)

## TRUST 5 Gate Plan

- **T**ested: 각 milestone 별 coverage ≥ 80% (M0~M5 평균 85%+).
- **R**eadable: 명확한 핸들러 명 + Go doc + spec.md cross-reference.
- **U**nified: gofmt + golangci-lint clean.
- **S**ecured: HMAC cookie + CSRF double-submit + Origin/Host 검증 + 10MB 한계 + 4429 rate-limit.
- **T**rackable: 각 PR 의 conventional commit + SPEC reference + `@MX:ANCHOR` (Bridge interface, BuildMux, Registry, FlushGate).

## @MX Tag Plan

| 위치 | tag | 이유 |
|------|-----|------|
| `Bridge` interface | `@MX:ANCHOR` | API surface, fan_in ≥ 3 (server, mux, tests) |
| `BuildMux` | `@MX:ANCHOR` | 모든 path 라우팅 단일 진입점 |
| `verifyLoopbackBind` | `@MX:ANCHOR` + `@MX:REASON` | 보안 invariant (외부 bind 차단) |
| `FlushGate` interface | `@MX:ANCHOR` | backpressure contract |
| `IssueSessionCookie` / `VerifyCSRFToken` | `@MX:WARN` + `@MX:REASON` | crypto 코드, HMAC misuse 위험 |
| WebSocket frame loop goroutine | `@MX:WARN` + `@MX:REASON` | goroutine + context cancel 라이프사이클 |

## 진입 조건 (다음 세션)

1. `git fetch && git log -3 main` — 본 PR `chore/bridge-001-tasks-decomposition` 머지 확인.
2. `git checkout -b feature/SPEC-GOOSE-BRIDGE-001-run-m0`.
3. `/moai run SPEC-GOOSE-BRIDGE-001` 또는 `manager-tdd` 직접 위임 (M0 milestone 한정 scope guard).
4. 진입 prompt: "Implement M0 only (types.go + bind.go + registry.go + server.go skeleton). RED-GREEN-REFACTOR per spec.md §9. Stop at end of M0; M1+ 은 후속 PR."
5. 1M context 활성 또는 신선 세션 권장 (M0 만 ~30K agent token, M0~M2 한 번에는 ~150K+).

## Total

- **Atomic tasks**: 23+
- **Files**: 18 production + 11 test
- **LoC 예상**: production ~1500 / test ~2000
- **PR 예상**: 6~7
- **Milestone 수**: 5 (M0~M5)
