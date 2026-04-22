---
id: SPEC-GOOSE-BRIDGE-001
version: 0.1.0
status: Planned
created: 2026-04-21
updated: 2026-04-21
author: manager-spec
priority: P0
issue_number: null
phase: 6
size: 대(L)
lifecycle: spec-anchored
---

# SPEC-GOOSE-BRIDGE-001 — PC↔Mobile 원격 세션 Bridge 프로토콜

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | ROADMAP v4.0 Phase 6 "Cross-Platform Clients" 신규 SPEC. Claude Code `src/bridge/`(33 파일) 패턴을 Go로 직접 포팅하여 `goosed`(PC) ↔ `goose-mobile`(Mobile) 간 원격 세션 프로토콜을 확립한다. | manager-spec |

---

## 1. 개요 (Overview)

Bridge는 PC(`goosed` daemon)와 Mobile(`goose-mobile` React Native)을 연결하는 **원격 세션 제어 프로토콜**이다. 사용자 확정 컨셉(2026-04-22)에 따라 PC가 메인, Mobile은 원격 클라이언트(지시·제어)이다. Mobile은 Bridge를 통해:

1. 언제 어디서나 PC의 GOOSE 세션에 접속
2. 채팅 메시지 송수신(양방향 스트리밍)
3. 아침 브리핑·이벤트 푸시 알림 수신
4. 권한 요청(tool 실행, 파일 접근 등)을 모바일에서 승인
5. 첨부 파일 업·다운로드

본 SPEC은 Claude Code `src/bridge/`의 33개 파일이 정립한 **검증된 패턴**을 Go로 직접 포팅한다. 전송은 WebSocket(양방향) + SSE(PC→Mobile 일방향 fallback), 보안은 JWT + Trusted Device, 신뢰성은 flushGate + capacityWake + session polling으로 보장한다.

Bridge 자체는 **중계 서버가 아니다**. 중계는 RELAY-001(E2EE, Noise Protocol)이 담당하며, Bridge는 그 위에서 동작하는 **세션·메시지·인증 프로토콜**이다.

---

## 2. 배경 (Background)

### 2.1 왜 Claude Code bridge/ 패턴을 직접 포팅하는가

Claude Code의 `src/bridge/` (33 파일, ~5000 LOC)은 이미 **실사용 검증된 원격 세션 인프라**다:

- 모바일 앱에서 Mac의 Claude Code 세션 제어
- 불안정한 모바일 네트워크 환경에서의 재연결·큐잉
- 모바일 절전 모드 대응(capacityWake)
- 폴링 기반 업데이트 (pollConfig)
- JWT 인증 + Trusted Device
- Flush gate를 통한 backpressure
- Session ID 호환성(세션 복원)

이 패턴을 바닥부터 다시 설계하는 것은 낭비이며, **직접 포팅이 전략적 우위**다.

### 2.2 33 파일 포팅 매핑

| # | Claude Code | GOOSE Go 포팅 |
|---|------------|---------------|
| 1 | `bridgeApi.ts` | `internal/bridge/api.go` (public 진입점) |
| 2 | `bridgeConfig.ts` | `internal/bridge/config.go` |
| 3 | `bridgeDebug.ts` | `internal/bridge/debug.go` |
| 4 | `bridgeEnabled.ts` | `internal/bridge/enabled.go` (feature flag) |
| 5 | `bridgeMain.ts` | `internal/bridge/main.go` (lifecycle) |
| 6 | `bridgeMessaging.ts` | `internal/bridge/messaging.go` |
| 7 | `bridgePermissionCallbacks.ts` | `internal/bridge/permission.go` |
| 8 | `bridgePointer.ts` | `internal/bridge/pointer.go` (session pointer) |
| 9 | `bridgeStatusUtil.ts` | `internal/bridge/status.go` |
| 10 | `bridgeUI.ts` | (Desktop `src/bridge/` TS — 별도) |
| 11 | `capacityWake.ts` | `internal/bridge/capacity_wake.go` |
| 12 | `codeSessionApi.ts` | `internal/bridge/session_api.go` |
| 13 | `createSession.ts` | `internal/bridge/create_session.go` |
| 14 | `debugUtils.ts` | `internal/bridge/debug_utils.go` |
| 15 | `envLessBridgeConfig.ts` | `internal/bridge/envless_config.go` |
| 16 | `flushGate.ts` | `internal/bridge/flush_gate.go` |
| 17 | `inboundAttachments.ts` | `internal/bridge/inbound/attachments.go` |
| 18 | `inboundMessages.ts` | `internal/bridge/inbound/messages.go` |
| 19 | `initReplBridge.ts` | `internal/bridge/init.go` |
| 20 | `jwtUtils.ts` | `internal/bridge/auth/jwt.go` |
| 21 | `pollConfig.ts` | `internal/bridge/poll/config.go` |
| 22 | `pollConfigDefaults.ts` | `internal/bridge/poll/defaults.go` |
| 23 | `remoteBridgeCore.ts` | `internal/bridge/remote_core.go` |
| 24 | `replBridge.ts` | `internal/bridge/repl.go` |
| 25 | `replBridgeHandle.ts` | `internal/bridge/repl_handle.go` |
| 26 | `replBridgeTransport.ts` | `internal/bridge/repl_transport.go` |
| 27 | `sessionIdCompat.ts` | `internal/bridge/session_id_compat.go` |
| 28 | `sessionRunner.ts` | `internal/bridge/session_runner.go` |
| 29 | `trustedDevice.ts` | `internal/bridge/auth/trusted_device.go` |
| 30 | `types.ts` | `internal/bridge/types.go` |
| 31 | `workSecret.ts` | `internal/bridge/auth/work_secret.go` |
| (32-33) | (minor utils) | 인접 파일로 통합 |

### 2.3 왜 별도 SPEC인가

Bridge는 TRANSPORT-001(gRPC, PC 내부 통신)과 **다른 계층**이다:

- TRANSPORT-001: `goosed` ↔ localhost 클라이언트(Desktop/CLI) — 같은 PC, 저지연, 신뢰 네트워크.
- BRIDGE-001: `goosed` ↔ 원격 Mobile — 불안정 모바일망, 방화벽 NAT, 절전 모드, 재연결 필요.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/bridge/` 패키지 구조(위 §2.2 매핑 기반).
2. **세션 생명주기**: createSession → authenticate → active → idle → resumed / expired.
3. **인증**:
   - JWT 액세스 토큰(24h 만료) + refresh 토큰(30d)
   - Trusted Device 등록/해지/목록(PC가 신뢰된 Mobile 목록 관리)
   - workSecret: 세션별 파생 비밀(HKDF)
4. **전송**: WebSocket(양방향 기본) + SSE(fallback, PC→Mobile 일방향).
5. **메시지 계층**:
   - Inbound(Mobile→PC): 채팅, 첨부, 권한 응답
   - Outbound(PC→Mobile): 스트리밍 청크, 알림, 권한 요청
6. **Backpressure**: flushGate로 Mobile 큐 초과 시 송신 정지, drain 완료 시 재개.
7. **Capacity wake**: Mobile이 background일 때 APNs/FCM push로 연결 회복 트리거.
8. **Polling fallback**: WebSocket 불가 시 pollConfig 기반 polling(기본 5s, 활성 시 1s).
9. **Session ID 호환**: 세션 재연결 시 sessionId 체크. 호환 모드 지원(claude → goose migration 시나리오는 out).
10. **Permission callback**: Mobile에서 tool 실행 승인(예: "파일 쓰기 허가?"), Desktop 대신 Mobile이 gate 역할 가능.
11. **재연결**: 지수 백오프 1s → 30s 캡, 영구 실패 시 reconnect failure 이벤트.
12. **관찰성**: 세션 메트릭(지연, 드롭, 재연결 횟수) OpenTelemetry export.

### 3.2 OUT OF SCOPE

- **E2EE 암호화**: RELAY-001이 담당. Bridge는 평문 메시지를 다루지 않음(RELAY-001이 제공하는 secure transport 위에서 동작).
- **중계 서버(Relay server) 구현**: RELAY-001.
- **Mobile UI**: MOBILE-001.
- **Desktop 페어링 UI**: DESKTOP-001.
- **QR 코드 형식 스펙**: MOBILE-001에서 확정(Bridge는 `{sessionId, workSecret, relayEndpoint}` 3-tuple만 소비).
- **Push notification provider 통합(APNs/FCM)**: MOBILE-001과 Bridge의 webhook 계약만 여기서 정의, 실제 APNs/FCM 호출은 Mobile이 등록한 gateway를 통과.

---

## 4. 의존성 (Dependencies)

- **상위 의존**: SPEC-GOOSE-TRANSPORT-001(gRPC 기반 daemon 내부 통신), SPEC-GOOSE-CORE-001(lifecycle).
- **동일 Phase 6**: SPEC-GOOSE-RELAY-001(transport 암호화 계층), SPEC-GOOSE-MOBILE-001(클라이언트), SPEC-GOOSE-DESKTOP-001(pairing UI).
- **향후 통합**: SPEC-GOOSE-GATEWAY-001(messenger bot은 Bridge가 아닌 별도 gateway).
- **라이브러리**:
  - `github.com/gorilla/websocket` v1.5+
  - `github.com/golang-jwt/jwt/v5` v5.2+
  - `golang.org/x/crypto/hkdf` (workSecret 파생)
  - `go.opentelemetry.io/otel` (메트릭)

---

## 5. 요구사항 (EARS Requirements)

### 5.1 Ubiquitous

- **REQ-BR-001**: Bridge **shall** maintain a registry of Trusted Devices with device ID, public key, display name, and last-seen timestamp.
- **REQ-BR-002**: Bridge **shall** issue a 24-hour JWT access token and a 30-day refresh token per authenticated session.
- **REQ-BR-003**: Bridge **shall** run WebSocket and SSE transport listeners concurrently on the same port using protocol negotiation.
- **REQ-BR-004**: Bridge **shall** expose session metrics (connected sessions, inbound rate, outbound rate, flush-gate stalls, reconnect attempts) via OpenTelemetry.

### 5.2 Event-Driven

- **REQ-BR-005**: **When** a Mobile client presents a valid pairing token, Bridge **shall** register the device in Trusted Devices, derive a workSecret via HKDF, and return access + refresh tokens.
- **REQ-BR-006**: **When** an inbound message arrives from Mobile, Bridge **shall** validate JWT, verify session binding, and forward the message to the QueryEngine session runner.
- **REQ-BR-007**: **When** the QueryEngine emits an outbound chunk, Bridge **shall** stream the chunk to the associated Mobile client subject to flush-gate permission.
- **REQ-BR-008**: **When** a tool execution requires permission and the originating session is bound to a Mobile client, Bridge **shall** forward the permission request to Mobile and block until a response arrives or a 60-second timeout elapses.

### 5.3 State-Driven

- **REQ-BR-009**: **While** the Mobile client is disconnected but its session is within the 24-hour JWT validity window, Bridge **shall** buffer outbound messages up to 4MB or 500 messages (whichever first) and replay them on reconnect in original order.
- **REQ-BR-010**: **While** the flush-gate reports a full outbound queue on the Mobile side, Bridge **shall** stop emitting further chunks until a drain ack is received.
- **REQ-BR-011**: **While** WebSocket is unavailable, Bridge **shall** serve the same session over SSE for outbound + HTTP POST for inbound at 1-second poll intervals.

### 5.4 Optional

- **REQ-BR-012**: **Where** APNs/FCM push credentials are configured, Bridge **shall** deliver a capacity-wake push when a session has pending outbound messages and the Mobile client is in background.
- **REQ-BR-013**: **Where** the user has enabled device-level approval, Bridge **shall** route every permission request to Mobile even when the session originated from Desktop.

### 5.5 Unwanted Behavior

- **REQ-BR-014**: **If** a JWT token is expired, revoked, or signature-invalid, **then** Bridge **shall not** accept any message on that token and **shall** close the connection with code 4401.
- **REQ-BR-015**: **If** an inbound message exceeds 10MB, **then** Bridge **shall not** forward it to the session runner and **shall** respond with a size-exceeded error.
- **REQ-BR-016**: **If** a Trusted Device is revoked while a session is active, **then** Bridge **shall** terminate all active sessions for that device within 5 seconds and require re-pairing.

### 5.6 Complex

- **REQ-BR-017**: **While** the Mobile client is reconnecting after a background wake, **when** the client presents the session resume token, Bridge **shall** restore the session state, replay buffered outbound messages, and emit a `session.resumed` event to the QueryEngine — provided the resume occurs within 24 hours of the original pairing.

---

## 6. 핵심 Go 타입 시그니처

```go
// internal/bridge/types.go
// Bridge 프로토콜의 핵심 타입. Claude Code src/bridge/types.ts 포팅.

// Bridge는 PC daemon이 관리하는 원격 세션 관리자이다.
type Bridge interface {
    Start(ctx context.Context) error       // listener 시작 (WS + SSE)
    Stop(ctx context.Context) error        // graceful shutdown
    Sessions() []BridgeSession             // 활성 세션 스냅샷
    Metrics() Metrics                      // OTel 메트릭 export
}

// BridgeSession은 단일 Mobile 클라이언트와의 원격 세션을 나타낸다.
type BridgeSession struct {
    ID          string         // session UUID
    DeviceID    string         // Trusted Device 참조
    WorkSecret  []byte         // HKDF 파생 세션 비밀 (32B)
    CreatedAt   time.Time
    LastSeen    time.Time
    State       SessionState   // pending / active / idle / expired
}

type SessionState string

const (
    SessionStatePending SessionState = "pending"
    SessionStateActive  SessionState = "active"
    SessionStateIdle    SessionState = "idle"
    SessionStateExpired SessionState = "expired"
)

// JWTToken은 Bridge 인증 토큰이다.
type JWTToken struct {
    Raw       string
    DeviceID  string
    SessionID string
    ExpiresAt time.Time
    Kind      TokenKind // access | refresh
}

type TokenKind string

const (
    TokenKindAccess  TokenKind = "access"
    TokenKindRefresh TokenKind = "refresh"
)

// TrustedDevice는 PC가 신뢰한 Mobile 디바이스 등록 정보.
type TrustedDevice struct {
    ID          string
    PublicKey   []byte         // ed25519
    DisplayName string         // "Galaxy S25 of Goos"
    PairedAt    time.Time
    LastSeenAt  time.Time
    Revoked     bool
}

// InboundMessage: Mobile → PC
type InboundMessage struct {
    SessionID   string
    Type        InboundType    // chat | attachment | permission_response | control
    Payload     []byte         // 이미 RELAY-001 경유 복호화된 평문
    ReceivedAt  time.Time
}

type InboundType string

const (
    InboundChat               InboundType = "chat"
    InboundAttachment         InboundType = "attachment"
    InboundPermissionResponse InboundType = "permission_response"
    InboundControl            InboundType = "control"
)

// OutboundMessage: PC → Mobile
type OutboundMessage struct {
    SessionID   string
    Type        OutboundType   // chunk | notification | permission_request | status
    Payload     []byte
    Sequence    uint64         // 순서 보장 (재연결 후 replay용)
}

type OutboundType string

const (
    OutboundChunk             OutboundType = "chunk"
    OutboundNotification      OutboundType = "notification"
    OutboundPermissionRequest OutboundType = "permission_request"
    OutboundStatus            OutboundType = "status"
)

// PollConfig는 SSE/polling fallback 동작 설정.
type PollConfig struct {
    IdleIntervalMs     int   // 기본 5000
    ActiveIntervalMs   int   // 기본 1000
    MaxBatchSize       int   // 기본 32 메시지
    WakeTriggerLatency int   // 기본 1500ms
}

// FlushGate는 Mobile 큐 초과 backpressure 신호.
type FlushGate interface {
    Acquire(ctx context.Context, sessionID string, bytes int) error
    Release(sessionID string, bytes int)
    Stalled(sessionID string) bool
}
```

---

## 7. 수락 기준 (Acceptance Criteria)

### 7.1 AC-BR-001 — 페어링 및 토큰 발급

**Given** 신규 Mobile이 유효 페어링 토큰 보유 **When** Bridge `/pair` 엔드포인트로 POST **Then** 200 응답에 access(24h) + refresh(30d) 토큰, Trusted Device 레지스트리에 device 등록 완료.

### 7.2 AC-BR-002 — 만료 JWT 거부

**Given** 만료된 JWT **When** 해당 토큰으로 WebSocket 연결 시도 **Then** 서버가 close code 4401 + 이유 `"token_expired"`로 종료한다.

### 7.3 AC-BR-003 — 양방향 스트리밍

**Given** 활성 세션 **When** Mobile이 채팅 메시지 전송 **Then** 500ms 이내 `goosed` QueryEngine 세션 runner가 수신. QueryEngine 응답 청크는 100ms 이내 Mobile로 스트리밍.

### 7.4 AC-BR-004 — 오프라인 버퍼링 + replay

**Given** 세션 활성 상태에서 Mobile 연결 끊김 **When** 끊긴 동안 PC가 outbound 5 청크 생성 **Then** 24h 이내 재연결 시 모든 청크가 원래 순서대로 도착. 누락·중복 없음.

### 7.5 AC-BR-005 — flushGate backpressure

**Given** Mobile이 queue full 신호 **When** PC에서 다음 청크 발행 시도 **Then** `flushGate.Acquire`가 block되고 drain ack 수신 후에만 release.

### 7.6 AC-BR-006 — SSE fallback

**Given** WebSocket 연결 실패(포트 차단) **When** 동일 세션으로 SSE 엔드포인트 접속 **Then** outbound는 SSE로, inbound는 HTTP POST로 동작하며 모든 기능 유지.

### 7.7 AC-BR-007 — capacityWake

**Given** Mobile이 background **When** PC가 outbound notification 생성 **Then** APNs/FCM push가 등록된 토큰으로 발송되고, Mobile이 포그라운드 복귀 ≤3s 후 WebSocket 재연결.

### 7.8 AC-BR-008 — 권한 콜백

**Given** 세션이 Mobile-bound **When** tool 실행이 permission을 요구 **Then** permission_request가 Mobile로 전달되고 60s 내 응답 없으면 기본 거부. 응답 도착 시 tool 실행이 그대로 진행.

### 7.9 AC-BR-009 — Trusted Device 폐기

**Given** PC에서 device A를 revoke **When** device A가 활성 세션 보유 **Then** 5s 이내 해당 세션이 close code 4403으로 종료, 이후 페어링 재등록만 허용.

### 7.10 AC-BR-010 — 세션 복원

**Given** 24h 이내 페어링된 세션이 idle **When** Mobile이 resume 토큰으로 재접속 **Then** 세션 상태 복원 + 버퍼 replay + QueryEngine `session.resumed` 이벤트 발행.

### 7.11 AC-BR-011 — 메시지 크기 제한

**Given** 정상 세션 **When** Mobile이 15MB 메시지 전송 **Then** Bridge가 수신 즉시 "size_exceeded" 에러 반환, QueryEngine에 전달되지 않음.

### 7.12 AC-BR-012 — Session ID 호환

**Given** 기존 sessionId 포맷과 동일 **When** Mobile이 구버전 포맷으로 연결 **Then** sessionIdCompat이 신버전 내부 포맷으로 변환하여 동일 세션 참조 유지.

### 7.13 AC-BR-013 — OpenTelemetry 메트릭

**Given** Bridge 실행 중 **When** 세션 3건, 메시지 100건 처리 **Then** OTel exporter에 `bridge.sessions.active=3`, `bridge.messages.inbound=100`, `bridge.flush_gate.stalls` 메트릭이 노출된다.

### 7.14 AC-BR-014 — 최대 동시 세션

**Given** Bridge **When** 1000개 Mobile이 동시 세션 수립 **Then** 모든 세션이 정상 active 상태, 메시지 지연 p99 ≤ 200ms.

---

## 8. TDD 전략

- **RED**: `internal/bridge/*_test.go`에서 세션 생명주기(create→active→expired), JWT 검증, flushGate backpressure 실패 테스트부터 작성.
- **GREEN**: 최소 구현은 WebSocket 단일 전송 + 인메모리 device registry. SSE/polling은 단계적 추가.
- **REFACTOR**: 전송별 어댑터 패턴(`Transport` interface로 WS/SSE/Poll 추상화). Bridge 코어는 전송 무관.
- **통합 테스트**: `httptest` + 실제 WebSocket 클라이언트로 flushGate backpressure, 세션 resume, capacityWake 시뮬레이션.
- **Fuzz**: JWT 파서, inbound 메시지 파서 퍼징 필수.
- 커버리지: 85%+ (Go).

---

## 9. 제외 항목 (Exclusions)

- **E2EE 암호화·Noise Protocol**: RELAY-001 전담. Bridge는 이미 복호화된 payload를 받는다는 가정.
- **Mobile UI 구현**: MOBILE-001.
- **Desktop 페어링 UI**: DESKTOP-001 AC-DK-*.
- **Messenger bot(Telegram/Slack 등)**: GATEWAY-001. Bridge는 퍼스트파티 Mobile/Desktop 전용.
- **Desktop 간 원격 세션(PC ↔ PC)**: 본 SPEC은 PC↔Mobile만 명시. PC↔PC는 향후 별도 SPEC.
- **Federated auth(OIDC/SAML)**: 1차 자체 JWT만. 기업 SSO는 v2+.
- **End-to-end message ordering across multiple sessions**: 단일 세션 내 순서 보장만.
