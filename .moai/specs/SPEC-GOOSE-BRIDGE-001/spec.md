---
id: SPEC-GOOSE-BRIDGE-001
version: 0.2.1
status: completed
created_at: 2026-04-21
updated_at: 2026-05-04
author: manager-spec
priority: P0
issue_number: null
phase: 6
size: 중(M)
lifecycle: spec-anchored
labels: [bridge, transport, web-ui, localhost, websocket, sse, phase-6]
---

# SPEC-GOOSE-BRIDGE-001 — Daemon ↔ Web UI Local Bridge

## HISTORY

| 버전  | 날짜       | 변경 사유                                                                                                                                                                                                                                                                                                                                                                                  | 담당          |
| ----- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| 0.1.0 | 2026-04-21 | ROADMAP v4.0 Phase 6 "Cross-Platform Clients" 신규 SPEC. PC↔Mobile 원격 세션 Bridge로 착수.                                                                                                                                                                                                                                                                                              | manager-spec  |
| 0.2.0 | 2026-04-25 | **Major scope 재편**. SPEC-GOOSE-ARCH-REDESIGN-v0.2 및 mass-20260425 감사 결과(Score 0.28, D1 catastrophic scope/body inconsistency) 반영. 본문 전면 개정: PC↔Mobile 원격 세션·APNs/FCM·Trusted Device·ed25519 pairing 제거. **goosed daemon ↔ localhost Web UI bridge** (WebSocket/SSE over HTTP, loopback-only 바인딩)로 scope 축소. Mobile-only REQ는 `[DEPRECATED v0.2]` 주석으로 표시하고 동일 번호는 재사용하지 않음. 원격 Mobile bridge는 향후 BRIDGE-002 별도 SPEC. 감사 결함 D1~D16 대응. | manager-spec  |
| 0.2.1 | 2026-05-04 | **종결 메타 갱신**. M0~M5 + follow-up 6 milestone 모두 main 머지 완료 (#82 #84 #85 #87 #88 #89 #90 #91). 16 AC 전부 implemented + race-clean + `internal/bridge` coverage ≥ 80% (M5 follow-up 84.2%). `status: planned → completed`, `updated_at` 갱신. Out-of-scope 로 분리된 cookie-hash 기반 logical session 매핑은 향후 amendment v0.3 또는 신규 SPEC 후보 (handoff prompt §"WebSocket connID = sid + randSuffix" 함정).                                                                                                                                                                  | manager-docs  |

---

## 1. 개요 (Overview)

Bridge는 단일 머신 내부에서 `goosed` daemon과 **로컬 브라우저의 Web UI**를 연결하는 **localhost 전용 통신 bridge**이다. 비개발자가 CLI 없이 MINK를 사용하기 위한 Web UI(별도 Phase 6 산출물, Skill: MOAI-WEBUI-*)가 WebSocket 또는 SSE로 daemon에 접속할 때, 그 통신 계약과 구현을 규정한다.

핵심 특성:

1. **Loopback-only**: daemon이 `127.0.0.1:<PORT>` 또는 `[::1]:<PORT>`에만 바인딩한다. 외부망 노출 금지.
2. **HTTP upgrade 기반**: 같은 포트에서 WebSocket (양방향) + SSE (서버→브라우저 일방향) + HTTP POST (브라우저→서버) 를 제공한다.
3. **Same-origin 브라우저 클라이언트 전용**: 인증은 local session cookie + CSRF double-submit token. 외부 JWT·refresh token·Trusted Device registry는 본 SPEC에서 다루지 않음(BRIDGE-002 후보).
4. **TRANSPORT-001 과의 경계**: TRANSPORT-001은 gRPC를 통한 native client(Desktop/CLI) ↔ daemon 통신. BRIDGE-001은 HTTP/WebSocket/SSE를 통한 **브라우저** ↔ daemon 통신. 두 SPEC은 같은 daemon 위에 공존하며 서로 다른 listener (gRPC vs HTTP) 를 사용한다.
5. **단일 사용자 가정**: 같은 OS 사용자 계정 내부의 브라우저 탭들만 접근한다. 멀티 테넌시·외부 auth·federated identity는 out of scope.

Bridge 자체는 세션 매니저·인증 게이트·메시지 포워더 역할이며, QueryEngine(별도 SPEC) 에 메시지를 전달한다.

---

## 2. 배경 (Background)

### 2.1 왜 localhost Web UI bridge가 필요한가

CLI 경험이 없는 사용자가 MINK를 쓰려면 브라우저 UI가 필요하다. Web UI는 정적 번들(HTML/JS/CSS)이지만, LLM 응답은 스트리밍이므로 **양방향 영구 연결**이 필요하다. 해당 연결 요구를 충족하는 가장 단순한 경로:

- **WebSocket (기본)**: 양방향 frame, 낮은 오버헤드.
- **SSE (fallback)**: WebSocket이 차단된 환경(기업 보안 소프트웨어, 일부 proxy)에서 서버→브라우저 스트림 유지 + 요청은 HTTP POST 로 분리.

### 2.2 v0.1.0 과의 차이점

v0.1.0 은 Claude Code `src/bridge/` (33 파일) 을 Go로 포팅해 PC↔Mobile 원격 세션 프로토콜을 정의했다. 감사 결과(Score 0.28, D1) Amendment 와 본문이 두 제품을 동시에 기술하여 구현 불가능 상태였다.

v0.2.0 은 **Web UI bridge만** 남긴다. Mobile·APNs/FCM·Trusted Device·ed25519 pairing·refresh token·workSecret 은 본 SPEC에서 제거했고, 필요 시 **BRIDGE-002 (Remote Mobile Bridge)** 로 별도 SPEC 을 세운다.

### 2.3 Claude Code `src/bridge/` 레퍼런스 축소

v0.1.0 의 33 파일 포팅 매핑(§2.2) 은 Mobile 원격 세션이 목적이므로 **Web UI scope 에는 직접 적용되지 않는다**. Web UI bridge 구현은 Go 표준 라이브러리(`net/http`) + `github.com/coder/websocket` (또는 동급) 만으로 충분하며, Claude Code bridge 는 BRIDGE-002 가 생길 때 재참조한다.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/bridge/` Go 패키지: HTTP 서버(loopback bind), WebSocket 핸들러, SSE 핸들러, inbound HTTP POST 핸들러.
2. **Loopback-only 바인딩 검증**: 시작 시 bind 주소가 `127.0.0.1` 또는 `::1` 인지 강제 검사.
3. **Same-origin 인증**:
   - `Origin` / `Host` 헤더가 loopback 인지 검증.
   - Local session cookie (HttpOnly, SameSite=Strict, Secure=false for loopback, Path=/).
   - CSRF double-submit token (POST/WebSocket open 시 header 또는 첫 frame 으로 제시).
4. **세션 생명주기**: `open → active → idle → closed`. 세션은 브라우저 탭 1:1.
5. **전송**:
   - WebSocket (upgrade path `/bridge/ws`) — 기본.
   - SSE (GET `/bridge/stream`) + HTTP POST (`/bridge/inbound`) — fallback.
   - 두 path 는 같은 listener 에서 제공한다.
6. **메시지 계층**:
   - Inbound (브라우저→daemon): 사용자 프롬프트, 파일 첨부(base64 또는 multipart), 제어 메시지(ping, abort).
   - Outbound (daemon→브라우저): LLM 스트리밍 청크, 상태 이벤트, 오류 이벤트.
7. **Backpressure (flush-gate)**: 브라우저 측 수신 지연 시 송신 속도 조절. WebSocket 레벨은 `writeDeadline` + write queue depth 임계값으로 구현.
8. **재연결**: 브라우저가 네트워크 복귀 또는 새 탭으로 session resume 쿠키 제시 시 세션 버퍼 replay. 재연결 정책은 지수 백오프 1s → 30s 캡.
9. **Close code / error taxonomy**: WebSocket close code 표(§6.1) 에 따라 정상/오류 종료. SSE 는 SSE event `event: error` + close.
10. **관찰성**: 세션 수, 메시지 in/out 속도, flush-gate stall 횟수, reconnect 횟수를 OpenTelemetry metric 으로 노출.

### 3.2 OUT OF SCOPE

- Mobile client, APNs/FCM, push notification.
- 외부 네트워크 릴레이 (RELAY-001).
- Trusted Device registry, ed25519 pairing, QR code pairing, refresh token, 30-day JWT.
- Federated identity (OIDC/SAML).
- 멀티 사용자 / 멀티 테넌시 (단일 OS 사용자 가정).
- End-to-end encryption (loopback 전용이므로 TLS 도 불필요. OS-level isolation 에 의존).
- PC ↔ PC 원격 세션.
- Web UI 렌더링·컴포넌트·라우팅 (MOAI-WEBUI-* 별도).
- Browser DevTools extension integration.

원격 Mobile bridge 가 필요해지면 **BRIDGE-002** 로 신규 SPEC 을 작성한다.

### 3.3 TRANSPORT-001 과의 경계

| 항목                | SPEC-GOOSE-TRANSPORT-001                  | SPEC-GOOSE-BRIDGE-001 (본 SPEC)              |
| ------------------- | ----------------------------------------- | -------------------------------------------- |
| 클라이언트 유형     | Native (mink CLI, goose-desktop)         | Browser (Web UI)                             |
| 프로토콜            | gRPC over localhost                       | HTTP/WebSocket/SSE over loopback             |
| 포트 / listener     | 별도 (gRPC listener)                      | 별도 (HTTP listener)                         |
| 인증                | Unix socket perm / process user match     | Local session cookie + CSRF double-submit    |
| Serialization       | Protobuf                                  | JSON                                         |
| 바인딩 제약         | localhost (Unix socket 또는 127.0.0.1)    | **127.0.0.1 or ::1 전용**, 외부 bind reject  |

두 SPEC 은 동일한 daemon 프로세스 내에 공존한다. 서로의 listener 를 공유하지 않으며, 인증 체계도 독립적이다.

---

## 4. 의존성 (Dependencies)

- **상위 의존**:
  - SPEC-GOOSE-TRANSPORT-001: 같은 daemon 안의 별도 통신 계층. Bridge 는 TRANSPORT 의 내부 API(QueryEngine invocation) 를 호출한다.
  - SPEC-GOOSE-CORE-001: daemon lifecycle.
- **동일 Phase 6 (변경됨)**:
  - MOAI-WEBUI-* (가칭): Web UI 정적 번들. Bridge 의 클라이언트.
- **제거된 의존** (v0.1.0 → v0.2.0):
  - SPEC-GOOSE-RELAY-001 (외부망 릴레이): 제거.
  - SPEC-GOOSE-MOBILE-001 (Mobile client): 제거.
  - SPEC-GOOSE-DESKTOP-001 (페어링 UI): Web UI 는 페어링이 불필요(loopback).
  - SPEC-GOOSE-GATEWAY-001: 무관.
- **라이브러리**:
  - `net/http` (표준)
  - `github.com/coder/websocket` v1.8+ (또는 동급 WebSocket 라이브러리)
  - `go.opentelemetry.io/otel` (메트릭)
  - `crypto/rand` (CSRF token, session ID 생성)

---

## 5. 요구사항 (EARS Requirements)

### 5.1 Ubiquitous

- **REQ-BR-001**: Bridge **shall** maintain a registry of active Web UI sessions keyed by session ID, each record containing opened-at timestamp, last-activity timestamp, transport mode (websocket or sse), and the associated CSRF token hash.
  *(v0.1.0 의 Trusted Devices registry 는 Web UI sessions registry 로 재정의됨. Device ID, public key, display name, last-seen 필드는 제거.)*
- **REQ-BR-002**: Bridge **shall** issue a local session cookie (HttpOnly, SameSite=Strict, Path=/) with a 24-hour lifetime and invalidate the cookie on explicit logout or session close.
  *(v0.1.0 의 24h JWT access + 30d refresh 는 로컬 쿠키 단일 체계로 단순화. Refresh token 개념 제거.)*
- **REQ-BR-003**: Bridge **shall** expose WebSocket (path `/bridge/ws`) and SSE (path `/bridge/stream`) listeners on the same HTTP listener, dispatching by request path and `Upgrade` header.
- **REQ-BR-004**: Bridge **shall** expose session metrics (active sessions, inbound messages per second, outbound bytes per second, flush-gate stalls, reconnect attempts) via OpenTelemetry.

### 5.2 Event-Driven

- **REQ-BR-005**: **When** the Bridge HTTP server starts, Bridge **shall** verify that its configured bind address resolves to a loopback address (`127.0.0.1`, `::1`, or `localhost`) and **shall** refuse to start with a `non_loopback_bind_rejected` error if any other address is configured.
  *(v0.1.0 페어링 토큰 검증은 loopback bind 검증으로 재정의.)*
- **REQ-BR-006**: **When** an inbound message arrives over WebSocket or HTTP POST, Bridge **shall** validate the local session cookie, verify the CSRF token, check `Origin`/`Host` is loopback, and forward the decoded payload to the QueryEngine session runner.
- **REQ-BR-007**: **When** the QueryEngine emits an outbound chunk for a session, Bridge **shall** stream the chunk to the associated Web UI client over the session's current transport (WebSocket frame or SSE event) subject to flush-gate permission.
- **REQ-BR-008**: **When** a tool execution requires user permission and the originating session is a Web UI session, Bridge **shall** forward the permission request as an outbound `permission_request` event and block tool execution until a `permission_response` inbound message arrives or a 60-second timeout elapses (default-deny on timeout).

### 5.3 State-Driven

- **REQ-BR-009**: **While** a Web UI session is temporarily disconnected (tab backgrounded, brief network blip) and its session cookie is still within the 24-hour validity window, Bridge **shall** buffer outbound messages up to 4MB or 500 messages (whichever limit is reached first) and replay them in original order upon reconnection.
- **REQ-BR-010**: **While** the flush-gate indicates that the WebSocket write queue exceeds `flush_gate.high_watermark` (default: 256 KB or 64 queued frames), Bridge **shall** stop emitting further outbound chunks for that session until the queue drains below `flush_gate.low_watermark` (default: 64 KB or 16 queued frames); the drain transition is observed by the Go write loop, not by an explicit client-side ack.
- **REQ-BR-011**: **While** WebSocket upgrade is rejected by the browser or intermediary, Bridge **shall** serve the same session over SSE for outbound messages and HTTP POST (`/bridge/inbound`) for inbound messages; in SSE fallback mode the client polls for any missed events via the `Last-Event-ID` header on reconnection (no client-side polling interval is required — SSE is server-push).

### 5.4 Optional

- **REQ-BR-012 [DEPRECATED v0.2]**: ~~**Where** APNs/FCM push credentials are configured, Bridge **shall** deliver a capacity-wake push...~~
  *Removed: push notifications are out of scope for localhost Web UI. This REQ is obsolete; the slot number is retained for stable cross-reference and will not be reused. Candidate for BRIDGE-002 Remote Mobile Bridge.*
- **REQ-BR-013 [DEPRECATED v0.2]**: ~~**Where** the user has enabled device-level approval, Bridge **shall** route every permission request to Mobile...~~
  *Removed: Web UI is the only permission surface in v0.2.0. Permission routing is already covered by REQ-BR-008. The slot number is retained and will not be reused.*

### 5.5 Unwanted Behavior

- **REQ-BR-014**: **If** a local session cookie is expired, absent, or fails HMAC verification, **then** Bridge **shall not** accept any message on that connection and **shall** close the connection with WebSocket close code `4401` (`unauthenticated`) or HTTP status `401` for SSE/POST endpoints. (See §6.1 close code table.)
- **REQ-BR-015**: **If** an inbound message exceeds 10 MB (configurable via `bridge.max_inbound_bytes`), **then** Bridge **shall not** forward it to the session runner and **shall** respond with an inbound error `{"error": "size_exceeded", "limit_bytes": 10485760}` and close the connection with WebSocket close code `4413` (`payload_too_large`) or HTTP status `413`.
- **REQ-BR-016 [DEPRECATED v0.2]**: ~~**If** a Trusted Device is revoked while a session is active...~~
  *Removed: Trusted Device registry is not part of v0.2.0 scope. Session revocation is now REQ-BR-016 re-defined below.*
- **REQ-BR-016 (v0.2.0)**: **If** the user explicitly logs out or deletes the local session cookie, **then** Bridge **shall** terminate all Web UI sessions associated with that cookie within 2 seconds using WebSocket close code `4403` (`session_revoked`) and require a fresh session cookie for any subsequent connection.

### 5.6 Composite (Complex, v0.2.0)

- **REQ-BR-017**: **While** a Web UI tab is in the reconnecting state after a transient disconnect, **when** the client presents a valid session cookie within the 24-hour validity window, Bridge **shall** restore the session reference, replay buffered outbound messages that were queued during the disconnect, and emit a `session.resumed` event to the QueryEngine. *(Composite pattern "While-When-Shall" retained; drop into two atomic EARS rules during implementation if required. The 24h window bound comes from REQ-BR-002 cookie lifetime.)*

### 5.7 Reconnect / Retry Policy (v0.2.0 new)

The reconnect policy is promoted from v0.1.0 prose bullet (Section 3.1 item 11) to an explicit normative requirement.

- **REQ-BR-018 (v0.2.0)**: **While** a client is in a reconnecting state, the client **shall** apply exponential backoff starting at 1 second, doubling each attempt, capped at 30 seconds, with a jitter of ±20%. Bridge **shall** accept up to **10 consecutive failed reconnection attempts** per session cookie before invalidating the cookie; further attempts require a fresh session cookie and return WebSocket close code `4401`.

---

## 6. 기술 세부 (Technical Details)

### 6.1 WebSocket Close Code 표

다음 close code 를 normative 로 규정한다. RFC 6455 §7.4.2 application-specific range (4000–4999) 를 사용한다.

| Code | 이름                    | 트리거                                                                                | 클라이언트 대응             |
| ---- | ----------------------- | ------------------------------------------------------------------------------------- | ---------------------------- |
| 1000 | `normal_closure`        | 정상 종료 (브라우저 탭 닫힘, 서버 graceful shutdown)                                  | 재연결 금지                  |
| 1001 | `going_away`            | daemon shutdown 또는 SIGTERM                                                          | 백오프 후 재연결             |
| 1009 | `message_too_big`       | WebSocket 프레임 자체가 protocol-level 제한 초과                                      | 메시지 수정 후 재연결        |
| 1011 | `internal_error`        | daemon 내부 panic, unhandled error                                                    | 백오프 후 재연결             |
| 4401 | `unauthenticated`       | 쿠키 만료/부재/HMAC 실패 (REQ-BR-014)                                                 | 로그인 페이지로 이동         |
| 4403 | `session_revoked`       | 사용자 로그아웃, 쿠키 강제 삭제 (REQ-BR-016 v0.2.0)                                   | 재연결 금지, 로그인 요구     |
| 4408 | `session_timeout`       | 쿠키는 유효하나 24h 이상 idle                                                         | 로그인 갱신 후 재연결        |
| 4413 | `payload_too_large`     | inbound 메시지 10MB 초과 (REQ-BR-015)                                                 | 메시지 축소 후 재연결        |
| 4429 | `rate_limited`          | 연결 시도가 구성된 rate limit 초과 (세션당 분당 60회 기본)                            | 지수 백오프 후 재연결        |
| 4500 | `bridge_unavailable`    | Bridge 가 starting / shutting_down 상태                                               | 백오프 후 재연결             |

SSE 엔드포인트는 WebSocket close code 대신 `event: error` + `data: {"code": 4401, "reason": "unauthenticated"}` 를 emit 하고 HTTP status (401/403/413 등) 로 종료한다.

### 6.2 Retry / Reconnect 정책 (REQ-BR-018 뒷받침)

```
attempt 1  → 1.0s ± 0.2s
attempt 2  → 2.0s ± 0.4s
attempt 3  → 4.0s ± 0.8s
attempt 4  → 8.0s ± 1.6s
attempt 5  → 16.0s ± 3.2s
attempt 6  → 30.0s ± 6.0s (cap)
attempt 7  → 30.0s ± 6.0s
...
attempt 10 → 30.0s ± 6.0s
attempt 11+ → reject (close 4401, require fresh cookie)
```

- Jitter 는 full-jitter 방식(±20% 균일 분포).
- Close code `1000` (정상 종료) 를 받은 경우 reconnect 금지 (탭이 닫힌 것).
- Close code `4401` / `4403` 를 받은 경우 reconnect 금지 (인증 문제, 새 쿠키 필요).
- 다른 close code 는 위 스케줄에 따라 재시도.

### 6.3 핵심 Go 타입 시그니처 (v0.2.0)

```go
// internal/bridge/types.go
// goosed daemon ↔ localhost Web UI bridge. v0.2.0 scope.

type Bridge interface {
    Start(ctx context.Context) error // HTTP listener 시작 (WS + SSE + POST)
    Stop(ctx context.Context) error  // graceful shutdown
    Sessions() []WebUISession        // 활성 세션 스냅샷
    Metrics() Metrics                // OTel 메트릭
}

// WebUISession 은 브라우저 탭 하나에 대응한다.
type WebUISession struct {
    ID            string    // session UUID
    CookieHash    []byte    // HMAC(local session cookie) 저장 (원본 쿠키는 로그 금지)
    CSRFHash      []byte    // HMAC(CSRF token)
    Transport     Transport // websocket | sse
    OpenedAt      time.Time
    LastActivity  time.Time
    State         SessionState
}

type Transport string

const (
    TransportWebSocket Transport = "websocket"
    TransportSSE       Transport = "sse"
)

type SessionState string

const (
    SessionStateOpen         SessionState = "open"
    SessionStateActive       SessionState = "active"
    SessionStateIdle         SessionState = "idle"
    SessionStateReconnecting SessionState = "reconnecting"
    SessionStateClosed       SessionState = "closed"
)

// BindAddress 는 loopback 검증을 거친 주소.
type BindAddress struct {
    Host string // "127.0.0.1" | "::1" | "localhost"
    Port int    // 1–65535
}

// InboundMessage: 브라우저 → daemon
type InboundMessage struct {
    SessionID  string
    Type       InboundType
    Payload    []byte
    ReceivedAt time.Time
}

type InboundType string

const (
    InboundChat               InboundType = "chat"
    InboundAttachment         InboundType = "attachment"
    InboundPermissionResponse InboundType = "permission_response"
    InboundControl            InboundType = "control" // ping, abort
)

// OutboundMessage: daemon → 브라우저
type OutboundMessage struct {
    SessionID string
    Type      OutboundType
    Payload   []byte
    Sequence  uint64 // replay 순서 보장
}

type OutboundType string

const (
    OutboundChunk             OutboundType = "chunk"
    OutboundNotification      OutboundType = "notification"
    OutboundPermissionRequest OutboundType = "permission_request"
    OutboundStatus            OutboundType = "status"
    OutboundError             OutboundType = "error"
)

// FlushGate 는 브라우저 측 write queue backpressure 신호.
type FlushGate interface {
    // HighWatermark 초과 시 true; 이후 Wait() 호출자는 LowWatermark 이하로 드레인될 때까지 block.
    Stalled(sessionID string) bool
    Wait(ctx context.Context, sessionID string) error
    ObserveWrite(sessionID string, bytes int)
    ObserveDrain(sessionID string, bytes int)
}

// CloseCode 는 §6.1 표의 수치 code.
type CloseCode uint16

const (
    CloseNormal            CloseCode = 1000
    CloseGoingAway         CloseCode = 1001
    CloseMessageTooBig     CloseCode = 1009
    CloseInternalError     CloseCode = 1011
    CloseUnauthenticated   CloseCode = 4401
    CloseSessionRevoked    CloseCode = 4403
    CloseSessionTimeout    CloseCode = 4408
    ClosePayloadTooLarge   CloseCode = 4413
    CloseRateLimited       CloseCode = 4429
    CloseBridgeUnavailable CloseCode = 4500
)
```

### 6.4 Same-origin 및 CSRF 검증

1. **Bind 검증** (REQ-BR-005): 시작 시 bind host 가 `{"127.0.0.1", "::1", "localhost"}` 에 포함되지 않으면 `ErrNonLoopbackBind` 로 실패.
2. **Origin/Host 검증**: 모든 HTTP/WebSocket upgrade 요청에서 `Host` 와 `Origin` 이 loopback 호스트인지 확인. 불일치 시 `403`.
3. **CSRF double-submit**: 쿠키 설정 시 CSRF 토큰을 cookie (`csrf_token`, HttpOnly=false, SameSite=Strict) + 응답 body 양쪽에 전달. 이후 POST / WebSocket open frame 에 `X-CSRF-Token` 헤더 또는 첫 frame 필드로 제시. 서버는 양측 값이 일치하는지 상수시간 비교.
4. **로그에 원본 쿠키/CSRF 값 기록 금지**: metric/log 에는 HMAC 해시만.

---

## 7. 수락 기준 (Acceptance Criteria)

포맷 주석: 본 섹션의 시나리오는 Given/When/Then 으로 표기하되, 각 AC 는 §5 의 EARS REQ 에 1:1 로 매핑된다. Given/When/Then 은 **test scenario** 포맷이며, normative requirement 는 §5 에 있다. REQ→AC traceability 는 §8 표 참조.

### 7.1 AC-BR-001 — Web UI 세션 레지스트리

**Given** Bridge 가 시작된 상태이고 활성 세션이 0 **When** 브라우저가 `/bridge/ws` 로 WebSocket upgrade 에 성공 **Then** `Bridge.Sessions()` 반환 배열에 1 개의 `WebUISession` 이 포함되며, `OpenedAt` 은 현재 시각 ±1s, `CookieHash` 는 쿠키 HMAC, `Transport` 는 `websocket`, `State` 는 `open`. *Covers REQ-BR-001.*

### 7.2 AC-BR-002 — 로컬 세션 쿠키 발급 및 만료

**Given** 쿠키가 없는 브라우저 **When** `/bridge/login` 으로 POST (stub auth 또는 local handshake) **Then** 응답 `Set-Cookie: goose_session=...; HttpOnly; SameSite=Strict; Path=/; Max-Age=86400`, CSRF 토큰이 응답 body 에 포함. 24h 경과 후 같은 쿠키로 연결 시 WebSocket close code `4408` (`session_timeout`). *Covers REQ-BR-002.*

### 7.3 AC-BR-003 — 동일 listener 에서 WebSocket + SSE

**Given** Bridge 가 `127.0.0.1:8091` 에서 실행 중 **When** 같은 포트로 `/bridge/ws` (`Upgrade: websocket`) 와 `/bridge/stream` (`Accept: text/event-stream`) 요청을 동시에 보냄 **Then** WebSocket upgrade 성공 + SSE 스트림 open 모두 200 계열 응답. 두 세션은 서로 독립이며, `Bridge.Sessions()` 에 별개 레코드로 등장. *Covers REQ-BR-003.*

### 7.4 AC-BR-004 — OpenTelemetry 메트릭

**Given** Bridge 실행 중 **When** 세션 3 건 open, inbound 100 개, outbound 200 개 처리 **Then** OTel exporter 에 `bridge.sessions.active=3`, `bridge.messages.inbound.total=100`, `bridge.messages.outbound.total=200`, `bridge.flush_gate.stalls` (counter) 메트릭 노출. 측정은 같은 호스트, 단일 프로세스 조건. *Covers REQ-BR-004.*

### 7.5 AC-BR-005 — Loopback-only 바인딩 검증

**Given** 설정 파일 `bridge.bind = "0.0.0.0:8091"` **When** Bridge 시작 **Then** `Start()` 가 `ErrNonLoopbackBind` 로 실패하고 프로세스 자체는 시작되지 않음. `bridge.bind = "127.0.0.1:8091"` 또는 `"[::1]:8091"` 또는 `"localhost:8091"` 은 성공. *Covers REQ-BR-005.*

### 7.6 AC-BR-006 — 인바운드 메시지 검증 파이프라인

**Given** 활성 세션, 유효 쿠키 + CSRF 토큰 **When** 브라우저가 WebSocket frame 으로 `{"type":"chat","payload":"hi"}` 전송 **Then** Bridge 는 쿠키 HMAC 검증 통과, CSRF 토큰 상수시간 비교 통과, `Origin`/`Host` 가 loopback 확인 후 QueryEngine session runner 에 `InboundMessage{Type: InboundChat}` 전달. 같은 프로세스 측정 시 검증부터 QueryEngine 진입까지 p95 ≤ 20ms, p99 ≤ 50ms (host: 4-core x86_64, 8GB RAM). *Covers REQ-BR-006.*

### 7.7 AC-BR-007 — 아웃바운드 스트리밍

**Given** 활성 세션, QueryEngine 이 300ms 간격으로 청크 10 개 emit **When** Bridge 가 청크를 수신 **Then** 각 청크는 동일 프로세스 기준 p95 ≤ 15ms 내에 WebSocket frame 으로 브라우저에 도달. 순서는 emit 순서와 동일. flush-gate 가 stall 되지 않은 조건. *Covers REQ-BR-007.*

### 7.8 AC-BR-008 — Permission request 라운드트립

**Given** Web UI 세션이 활성, tool 실행이 permission 요구 **When** Bridge 가 `OutboundPermissionRequest` emit **Then** 브라우저가 60s 내 `InboundPermissionResponse{granted:true}` 회신 → tool 실행 진행. 60s 타임아웃 시 기본 거부 + `OutboundStatus{code:"permission_timeout"}`. *Covers REQ-BR-008.*

### 7.9 AC-BR-009 — 오프라인 버퍼링 + replay

**Given** 활성 세션에서 브라우저 탭 backgrounded (WebSocket close 1001 수신) **When** 그 동안 daemon 이 outbound 5 청크 생성 **Then** 24h 이내 동일 쿠키로 재연결 시 5 청크가 emit 순서로 도달. 누락·중복 없음. 버퍼는 4MB 또는 500 메시지 초과 시 oldest-drop. *Covers REQ-BR-009.*

### 7.10 AC-BR-010 — Flush-gate backpressure

**Given** 브라우저 탭 네트워크 stall, daemon write queue 가 256 KB 초과 **When** QueryEngine 이 outbound 청크를 추가 emit **Then** FlushGate.Stalled() 가 true 반환, Bridge 는 이후 emit 을 `FlushGate.Wait()` 로 block. 큐가 64 KB 이하로 드레인되면 `FlushGate.Stalled()` false → emit 재개. `bridge.flush_gate.stalls` 카운터 증가. *Covers REQ-BR-010.*

### 7.11 AC-BR-011 — SSE fallback

**Given** WebSocket upgrade 가 실패 (또는 명시적 fallback 선택) **When** 클라이언트가 `/bridge/stream` 으로 SSE open, inbound 는 `POST /bridge/inbound` 로 전송 **Then** 모든 메시지 계층 기능(chat, permission, abort) 이 동일하게 동작. 재연결 시 `Last-Event-ID` 헤더로 마지막 수신 sequence 전달 → Bridge 는 그 이후 버퍼된 청크만 replay. *Covers REQ-BR-011.*

### 7.12 AC-BR-012 — 만료/변조 쿠키 거부

**Given** 만료된 세션 쿠키 또는 HMAC 불일치 쿠키 **When** `/bridge/ws` 업그레이드 또는 `/bridge/stream` GET 시도 **Then** WebSocket 은 close code `4401`, SSE 는 HTTP `401`. 응답 body 에는 `{"error":"unauthenticated"}`. *Covers REQ-BR-014.*

### 7.13 AC-BR-013 — 메시지 크기 제한

**Given** 정상 세션 **When** 브라우저가 11 MB payload 의 inbound 메시지 전송 **Then** Bridge 는 수신 즉시 `{"error":"size_exceeded","limit_bytes":10485760}` 에러 반환하고 WebSocket close code `4413` 또는 HTTP `413`. QueryEngine 에 전달되지 않음. 10 MB 이하는 통과. *Covers REQ-BR-015.*

### 7.14 AC-BR-014 — 사용자 로그아웃 세션 종료

**Given** 같은 쿠키로 동시 탭 3 개 활성 **When** `POST /bridge/logout` 호출 **Then** 2s 이내 3 개 세션 모두 WebSocket close code `4403` (`session_revoked`) 로 종료. 이후 같은 쿠키로 재연결 시도 시 `4401`. 새 쿠키 발급 후에만 재접속 가능. *Covers REQ-BR-016 (v0.2.0).*

### 7.15 AC-BR-015 (v0.2.0) — 세션 resume

**Given** 24h 이내 쿠키 유효, 세션이 `reconnecting` 상태로 진입 **When** 브라우저가 같은 쿠키로 `/bridge/ws` 재접속 + `X-Last-Sequence` 헤더로 마지막 수신 sequence 전달 **Then** Bridge 는 해당 sequence 이후의 버퍼된 outbound 를 emit 순서대로 replay, QueryEngine 에 `session.resumed` 이벤트 발행. *Covers REQ-BR-017.*

### 7.16 AC-BR-016 (v0.2.0) — Reconnect 정책

**Given** 브라우저가 close code `1001` 수신 후 reconnect 모드 **When** 연속 실패 10 회 (각 시도 간 §6.2 스케줄 준수) **Then** 11 번째 시도는 서버가 `4401` 로 거부, 브라우저는 새 쿠키 발급을 요구. 1–10 회 동안 지수 백오프 ±20% jitter 준수는 클라이언트 단위 테스트에서 검증 (서버는 시도 간격을 강제하지 않으나 rate-limit 으로 초당 한도 초과 시 `4429`). *Covers REQ-BR-018.*

---

## 8. REQ → AC Traceability

| REQ                       | AC                                     | 비고                                         |
| ------------------------- | -------------------------------------- | -------------------------------------------- |
| REQ-BR-001                | AC-BR-001                              | Web UI session registry                      |
| REQ-BR-002                | AC-BR-002                              | Cookie lifecycle (24h, logout invalidation)  |
| REQ-BR-003                | AC-BR-003                              | 동일 listener 에서 WS+SSE                    |
| REQ-BR-004                | AC-BR-004                              | OTel metrics                                 |
| REQ-BR-005                | AC-BR-005                              | Loopback bind verification                   |
| REQ-BR-006                | AC-BR-006                              | Inbound validation pipeline                  |
| REQ-BR-007                | AC-BR-007                              | Outbound streaming                           |
| REQ-BR-008                | AC-BR-008                              | Permission roundtrip                         |
| REQ-BR-009                | AC-BR-009                              | Offline buffering + replay                   |
| REQ-BR-010                | AC-BR-010                              | Flush-gate backpressure                      |
| REQ-BR-011                | AC-BR-011                              | SSE fallback                                 |
| REQ-BR-012 [DEPRECATED]   | —                                      | Out of scope v0.2.0                          |
| REQ-BR-013 [DEPRECATED]   | —                                      | Out of scope v0.2.0                          |
| REQ-BR-014                | AC-BR-012                              | Auth failure                                 |
| REQ-BR-015                | AC-BR-013                              | Payload size limit                           |
| REQ-BR-016 [DEPRECATED]   | —                                      | Original Trusted-Device variant              |
| REQ-BR-016 (v0.2.0)       | AC-BR-014                              | Logout / session revocation                  |
| REQ-BR-017                | AC-BR-015 (v0.2.0)                     | Session resume                               |
| REQ-BR-018 (v0.2.0)       | AC-BR-016 (v0.2.0)                     | Reconnect policy                             |

---

## 9. TDD 전략

- **RED**: `internal/bridge/*_test.go` 에서 (a) 세션 생명주기 open→active→closed, (b) loopback bind 검증 실패 케이스, (c) 쿠키/CSRF 검증, (d) flush-gate stall, (e) SSE fallback, (f) close code 매트릭스 를 먼저 실패 테스트로 작성.
- **GREEN**: 최소 구현. HTTP mux + coder/websocket + SSE handler. 인메모리 session registry. FlushGate 는 chan 기반 간단 구현.
- **REFACTOR**: Transport 어댑터 패턴(`Transport` interface), FlushGate 를 watermark 기반 재구성, close code 테이블 constant 로 추출.
- **통합 테스트**: `httptest.Server` + 실제 WebSocket 클라이언트(동일 프로세스) 로 replay, backpressure, reconnect 시나리오 확인.
- **Fuzz**: inbound JSON 파서, CSRF 검증, 세션 쿠키 파서 퍼징.
- **커버리지**: 85%+ (Go `internal/bridge/`).

---

## 10. 제외 항목 (Exclusions, v0.2.0 확장)

v0.2.0 에서 본 SPEC 이 **다루지 않는** 것들. 이 목록은 Web UI bridge 맥락에서의 scope 제약을 명시한다.

- **원격 Mobile / 외부망 노출**: 127.0.0.1 외 bind 금지 (REQ-BR-005). 외부 릴레이, NAT traversal, 모바일 discovery, APNs/FCM 는 본 SPEC 에서 제외. 필요 시 BRIDGE-002 Remote Mobile Bridge 로 분리.
- **Mobile client, React Native, 모바일 UI**: MOBILE-001 과 무관하며 v0.2.0 에서 MOBILE-001 자체가 제거됨.
- **Trusted Device registry, ed25519 pairing, QR 페어링, refresh token, workSecret HKDF**: 전부 제거 (REQ-BR-001, REQ-BR-002 재정의).
- **E2EE / Noise Protocol**: loopback 이므로 적용하지 않음. RELAY-001 의존 제거.
- **Federated identity (OIDC/SAML/SSO)**: 단일 OS 사용자 가정. 기업 SSO 는 별도 SPEC.
- **멀티 사용자 / 멀티 테넌시**: 같은 OS 계정의 브라우저 탭만 가정.
- **CSRF 이외의 브라우저 공격 대응 (XSS/clickjacking)**: Web UI 번들 측 책임 (MOAI-WEBUI-*).
- **gRPC**: TRANSPORT-001 전담.
- **PC↔PC / Desktop↔Desktop 원격 세션**: 본 SPEC 및 BRIDGE-002 모두 다루지 않음.
- **Messenger bot (Slack/Telegram)**: GATEWAY-001 별도.
- **세션 간 메시지 순서 보장**: 단일 세션 내부 sequence 순서만 보장 (REQ-BR-017).
- **TLS / 인증서**: loopback 전용이므로 불필요. 외부 노출은 §3.1 item 2 에서 금지.
- **Web UI 렌더링/라우팅/상태관리**: MOAI-WEBUI-* 전담.
