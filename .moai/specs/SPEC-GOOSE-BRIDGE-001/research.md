# SPEC-GOOSE-BRIDGE-001 — Research Notes

> Claude Code `src/bridge/` (33 파일) 완전 분석 + Go 포팅 전략.

---

## 1. Claude Code bridge/ 상세 분석

Claude Code의 bridge/는 "Mac에서 실행되는 Claude Code 세션을 모바일에서 원격 제어"라는 명확한 제품 요구를 해결한다. 33개 파일은 다음 6개 관심사로 분류:

### 1.1 진입점 / 설정 (6 파일)

| 파일 | 책임 |
|-----|-----|
| `bridgeApi.ts` | 외부 공개 API (start/stop/sessions) |
| `bridgeConfig.ts` | Bridge 설정 스키마 (port, paths, timeouts) |
| `bridgeEnabled.ts` | Feature flag, 환경별 토글 |
| `bridgeMain.ts` | main() 진입점, lifecycle 조립 |
| `envLessBridgeConfig.ts` | 환경변수 없이 파일/defaults로 구성하는 경로 |
| `initReplBridge.ts` | REPL bridge 초기화 (interactive 모드) |

### 1.2 세션 관리 (5 파일)

| 파일 | 책임 |
|-----|-----|
| `bridgePointer.ts` | 활성 세션 포인터 (한 번에 하나의 Mobile을 pinned) |
| `codeSessionApi.ts` | 세션의 공개 API (send/receive/abort) |
| `createSession.ts` | 세션 신규 생성 (sessionId 할당, workSecret 파생) |
| `sessionRunner.ts` | 세션 내부 이벤트 루프 (inbound → QueryEngine) |
| `sessionIdCompat.ts` | 구/신 sessionId 포맷 변환 |

### 1.3 전송 (6 파일)

| 파일 | 책임 |
|-----|-----|
| `replBridge.ts` | REPL 전송 추상화 |
| `replBridgeHandle.ts` | REPL 핸들 (현재 세션 참조) |
| `replBridgeTransport.ts` | 전송 구현 (WebSocket 등) |
| `remoteBridgeCore.ts` | 원격 전송 코어 (Mobile ↔ PC) |
| `pollConfig.ts` | 폴링 간격·배치 크기 설정 |
| `pollConfigDefaults.ts` | 기본 폴링 설정값 |

### 1.4 메시지 (3 파일)

| 파일 | 책임 |
|-----|-----|
| `bridgeMessaging.ts` | 메시지 프레이밍·시리얼라이즈 |
| `inboundMessages.ts` | Mobile → PC 메시지 파서·라우터 |
| `inboundAttachments.ts` | 첨부(이미지/파일) 업로드 핸들러 |

### 1.5 인증·보안 (3 파일)

| 파일 | 책임 |
|-----|-----|
| `jwtUtils.ts` | JWT 생성·검증·갱신 |
| `trustedDevice.ts` | Trusted Device 레지스트리 |
| `workSecret.ts` | 세션별 파생 비밀 (HKDF) |

### 1.6 신뢰성·관찰성 (5 파일)

| 파일 | 책임 |
|-----|-----|
| `flushGate.ts` | Outbound queue backpressure |
| `capacityWake.ts` | Mobile background wake (APNs/FCM trigger) |
| `bridgeStatusUtil.ts` | 상태 조회 (health, sessions count) |
| `bridgeUI.ts` | UI 상태 export (Desktop/Mobile 모두) |
| `bridgeDebug.ts` + `debugUtils.ts` | 디버그 이벤트·로그 |

### 1.7 공통 (1 파일)

- `types.ts` — 전체 bridge 타입 정의

---

## 2. Go 포팅 전략

### 2.1 왜 TS → Go 포팅이 타당한가

- Claude Code는 Node.js 환경, GOOSE는 Go daemon. 인터페이스 계약은 거의 1:1.
- TS의 async/await → Go의 goroutine + channel. 더 명확해짐.
- WebSocket 라이브러리(gorilla/websocket)가 Node의 ws와 동등 기능.
- JWT는 `github.com/golang-jwt/jwt/v5` 표준.

### 2.2 패키지 구조 결정

```
internal/bridge/
├── api.go              # Claude bridgeApi.ts
├── main.go             # bridgeMain.ts
├── config.go           # bridgeConfig.ts + envLessBridgeConfig.ts
├── enabled.go          # bridgeEnabled.ts
├── debug.go            # bridgeDebug.ts + debugUtils.ts
├── messaging.go        # bridgeMessaging.ts
├── permission.go       # bridgePermissionCallbacks.ts
├── pointer.go          # bridgePointer.ts
├── status.go           # bridgeStatusUtil.ts
├── capacity_wake.go    # capacityWake.ts
├── session_api.go      # codeSessionApi.ts
├── create_session.go   # createSession.ts
├── session_runner.go   # sessionRunner.ts
├── session_id_compat.go # sessionIdCompat.ts
├── repl.go             # replBridge.ts
├── repl_handle.go      # replBridgeHandle.ts
├── repl_transport.go   # replBridgeTransport.ts
├── remote_core.go      # remoteBridgeCore.ts
├── types.go            # types.ts
├── init.go             # initReplBridge.ts
├── auth/
│   ├── jwt.go          # jwtUtils.ts
│   ├── trusted_device.go # trustedDevice.ts
│   └── work_secret.go  # workSecret.ts
├── inbound/
│   ├── messages.go     # inboundMessages.ts
│   └── attachments.go  # inboundAttachments.ts
├── poll/
│   ├── config.go       # pollConfig.ts
│   └── defaults.go     # pollConfigDefaults.ts
└── flush_gate.go       # flushGate.ts
```

### 2.3 TS idiom → Go idiom 변환

| TS 패턴 | Go 대응 |
|--------|--------|
| `async/await` | goroutine + `chan` |
| Promise 체인 | `errgroup.Group` |
| EventEmitter | `chan Event` + fan-out |
| class with private fields | struct + unexported fields |
| Zod 스키마 | struct + 수동 validator (또는 `go-playground/validator`) |
| RxJS operators | goroutine + select |

---

## 3. WebSocket vs SSE vs Polling 선택 기준

| 전송 | 장점 | 단점 | GOOSE 사용 |
|-----|------|------|----------|
| **WebSocket** | 양방향, 저지연, 효율적 | 방화벽 차단 가능, 모바일 절전 비친화 | 기본 |
| **SSE** | HTTP 위, 프록시 친화, 단방향 | PC→Mobile만, inbound는 HTTP POST 별도 | WS 실패 시 fallback |
| **Polling** | 가장 robust, 모든 환경 동작 | 지연 큼, 네트워크 낭비 | SSE도 실패 시 final fallback |

전송 네고시에이션 순서:
1. WebSocket (`/bridge/ws`) 시도 → 실패 시
2. SSE (`/bridge/sse` + POST `/bridge/inbound`) 시도 → 실패 시
3. Polling (`GET /bridge/poll` + POST `/bridge/inbound`)

---

## 4. JWT + Trusted Device 설계

### 4.1 토큰 구조

```json
{
  "iss": "goose-bridge",
  "sub": "device:abc123",
  "sid": "session:uuid",
  "iat": 1745000000,
  "exp": 1745086400,
  "kind": "access"
}
```

서명: HMAC-SHA256 with 32B workSecret (디바이스별 파생).

### 4.2 Trusted Device 레지스트리

SQLite 테이블:

```sql
CREATE TABLE trusted_devices (
  id          TEXT PRIMARY KEY,       -- UUID
  public_key  BLOB NOT NULL,          -- ed25519 32B
  display_name TEXT NOT NULL,
  paired_at   INTEGER NOT NULL,       -- unix seconds
  last_seen_at INTEGER NOT NULL,
  revoked     INTEGER DEFAULT 0
);

CREATE INDEX idx_last_seen ON trusted_devices(last_seen_at);
```

### 4.3 페어링 플로우

1. Desktop이 `createPairingToken(deviceDisplayName)` 호출 → 1회용 페어링 토큰(5분 유효) 생성.
2. Desktop이 QR 코드에 `{pairingToken, relayEndpoint, serverPublicKey}` 인코딩.
3. Mobile이 QR 스캔 → ed25519 키 쌍 생성 → `POST /bridge/pair { pairingToken, publicKey, displayName }`.
4. Bridge 검증 성공 시 Trusted Device 등록 + access/refresh JWT 응답.
5. 이후 Mobile은 access JWT로 WebSocket 연결.

---

## 5. flushGate 백프레셔 메커니즘

### 5.1 필요성

Mobile의 네트워크가 느리거나 화면이 꺼져 수신 지연 시, PC가 outbound 청크를 계속 생성하면 서버 메모리가 폭증한다.

### 5.2 설계

```go
type flushGate struct {
    mu         sync.Mutex
    perSession map[string]*gate
    perSessionLimit int64 // 기본 4MB
}

type gate struct {
    inFlight int64
    cond     *sync.Cond
}

func (g *flushGate) Acquire(ctx context.Context, sid string, bytes int) error {
    // inFlight + bytes > limit 이면 cond.Wait
}

func (g *flushGate) Release(sid string, bytes int) {
    // inFlight -= bytes, cond.Signal
}
```

Mobile이 chunk를 소비하면 ack 메시지 전송 → server가 Release 호출.

---

## 6. capacityWake: Mobile 절전 대응

### 6.1 문제

iOS는 background WebSocket을 ~30초 내 유지하다 suspend. Android Doze mode도 비슷.

### 6.2 Claude Code 해법

- Mobile이 APNs(iOS)/FCM(Android) 토큰을 pairing 시 등록.
- PC에 outbound 메시지 pending이 있고, Mobile WebSocket이 disconnected 상태라면,
- PC가 silent push 발송 → Mobile OS가 앱 wake → 앱이 background에서 WebSocket 재연결 시도.

### 6.3 GOOSE 구현

```go
// internal/bridge/capacity_wake.go
type CapacityWake interface {
    RegisterToken(deviceID string, token string, platform Platform) error
    Trigger(deviceID string) error  // silent push 발송
}
```

실제 APNs/FCM 호출은 MOBILE-001이 제공하는 push provider 인터페이스 경유.

---

## 7. Session ID Compatibility

Claude Code는 sessionId 포맷 변경 이력이 있어 `sessionIdCompat.ts`가 변환 로직을 수행. GOOSE는 초기부터 v1 포맷(UUID v7)으로 시작하되, 향후 변경 대비 `session_id_compat.go`를 placeholder로 둔다(no-op transform).

---

## 8. Hermes gateway/ 와의 관계 명확화

Hermes gateway/는 **BRIDGE-001이 아니라 GATEWAY-001의 참조**이다:

- Hermes gateway/: messenger bot (Telegram, Discord 등) — 3rd party 플랫폼 통합
- Claude Code bridge/: per스트파티 모바일 원격 세션 — **BRIDGE-001의 참조**

둘은 목적이 다르다. BRIDGE-001은 Claude Code bridge/만 참조한다.

---

## 9. TDD 전략 상세

### 9.1 단위 테스트 순서

1. `auth/jwt_test.go`: 토큰 생성·검증·만료 엣지 케이스 (expired, wrong signature, wrong kind)
2. `auth/work_secret_test.go`: HKDF 결정적 출력 검증
3. `auth/trusted_device_test.go`: CRUD + revoke 반영
4. `flush_gate_test.go`: 정상 → full → drain → resume 시퀀스
5. `inbound/messages_test.go`: 크기 초과, 잘못된 type, 정상 경로
6. `session_runner_test.go`: lifecycle 상태 전이
7. `remote_core_test.go`: WebSocket 실제 연결(httptest)

### 9.2 통합 테스트

- `integration/pair_test.go`: full pairing → WS connect → send → receive → disconnect → resume
- `integration/fallback_test.go`: WS 차단 → SSE 자동 전환 → 동일 기능

### 9.3 Fuzzing

- `fuzz_jwt_test.go`: JWT 입력 퍼징 (panic 없음 확인)
- `fuzz_inbound_test.go`: inbound 메시지 파서 퍼징

### 9.4 벤치마크

- 1000 세션 동시 연결 시 메시지 지연 p50/p99
- flushGate 오버헤드

---

## 10. 오픈 이슈

1. **gorilla/websocket vs nhooyr.io/websocket**: 후자가 context 친화적이지만 gorilla가 더 성숙. 1차는 gorilla.
2. **SQLite vs in-memory Trusted Device**: 프로덕션은 SQLite, 테스트는 in-memory mock.
3. **Refresh token 회전(rotation)**: 매 refresh마다 신규 토큰 발급 + 이전 invalidate → 보안 ↑ but 구현 복잡. 1차는 단순 갱신.
4. **JWT HS256 vs EdDSA**: workSecret 공유 구조라 HS256으로 충분. 서버가 여러 인스턴스일 때는 EdDSA 재고.
5. **첨부 크기 제한 10MB vs 청크 분할**: 1차 10MB hard limit, 이후 multi-chunk 업로드 추가.
6. **세션 동시성**: 한 디바이스당 1 세션 vs N 세션. 1차는 Claude Code와 동일 단일 세션(pinned pointer).

---

## 11. 참조

- Claude Code `src/bridge/` 33 파일 (사용자 제공 분석)
- IETF RFC 7519 (JWT), RFC 5869 (HKDF), RFC 8032 (Ed25519)
- Apple APNs docs (silent push)
- Google FCM docs (data-only messages)
- gorilla/websocket: <https://github.com/gorilla/websocket>
