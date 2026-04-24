---
id: SPEC-GOOSE-RELAY-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-21
author: manager-spec
priority: P1
issue_number: null
phase: 6
size: 대(L)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-RELAY-001 — E2EE 중계 서비스 (Noise Protocol)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | ROADMAP v4.0 Phase 6 신규 SPEC. BRIDGE-001이 요구하는 암호화 전송 계층. Noise Protocol(WireGuard 패턴) + ChaCha20-Poly1305. Go는 인터페이스만, 구현은 Rust `goose-crypto`/`goose-relay` 크레이트 위임(tech.md §6.1 Mullvad GotaTun 원형). | manager-spec |

---

## 1. 개요 (Overview)

Relay는 PC가 NAT/방화벽 뒤에 있을 때 Mobile이 PC에 도달하기 위한 **E2EE 중계 서비스**다. PC와 Mobile 사이에 Relay 서버가 놓이지만, Relay는 **암호화된 패킷만 중계**하며 평문 payload에 접근하지 **불가능**하다.

암호화 프리미티브:
- **Noise Protocol** (패턴 `XK`): 응답자(PC)의 정적 공개키를 사전 공유한 상태에서 핸드셰이크
- **X25519** 키 교환
- **ChaCha20-Poly1305** AEAD
- **Forward secrecy** (ephemeral key per session)
- **Post-quantum preshared key** (옵션, Noise PSK mode)

Relay 서버는 두 가지 배포 모드를 지원한다:
1. **공식 cloud relay**: `wss://relay.gooseagent.org` (GOOSE가 운영)
2. **Self-hosted**: 사용자가 자체 서버 운영 (Docker, systemd)

Go는 인터페이스와 Relay 서버의 Go 부분(세션 테이블, 연결 관리, 메트릭)만 담당한다. 실제 암호 프리미티브(handshake, encrypt/decrypt, key derivation)는 **Rust crates**(`goose-crypto`, `goose-relay`)에 위임한다.

---

## 2. 배경 (Background)

### 2.1 Relay 없이는 왜 동작하지 않는가

- 일반 사용자 PC는 공인 IP가 없고 NAT 뒤에 있음(유선/와이파이 라우터).
- 방화벽이 WebSocket inbound 차단.
- UPnP/NAT-PMP hole punching은 불안정.
- DDNS + port forwarding은 일반 사용자 설정 불가.

**해법**: PC가 outbound로 Relay에 연결 유지 → Mobile도 outbound로 Relay 연결 → Relay가 매칭 후 암호화된 패킷만 중계.

### 2.2 왜 Noise Protocol인가

- WireGuard(Mullvad, Cloudflare, AWS 사용)가 검증한 동일 패턴.
- 핸드셰이크 최소(1-RTT), AEAD 내장.
- Go → Rust 마이그레이션 사례(Mullvad GotaTun 2026)가 **메모리 안전성 + 비밀 누출 방지**에서 우위를 증명.
- TLS 1.3 대비 오버헤드 ~40% 낮음.
- 공식 `snow` Rust crate가 성숙.

### 2.3 왜 Rust에 위임하는가 (tech.md §6.1)

- 암호 primitive는 메모리 안전이 보안 크리티컬(Go GC는 secret zeroing 불확실).
- Rust `zeroize` crate로 키 메모리 소거 보장.
- Mullvad, Cloudflare, AWS 모두 Rust로 암호 레이어 이전 중.
- `snow`(Noise), `chacha20poly1305`, `x25519-dalek`, `ring` 생태계 성숙.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. **Go 인터페이스**: `internal/relay/` — `Relay`, `RelayEndpoint`, `NoiseTunnel`, `CryptoProvider` 등.
2. **Rust 구현**: `crates/goose-crypto/` (primitives) + `crates/goose-relay/` (session matching / server-side).
3. **Noise Protocol 핸드셰이크**: 패턴 `Noise_XK_25519_ChaChaPoly_BLAKE2s`.
4. **세션 매칭**: Relay 서버가 PC와 Mobile을 `relay_session_id`로 매칭, 각자 outbound WebSocket으로 연결.
5. **Forward secrecy**: 세션별 ephemeral key, 핸드셰이크 완료 후 rekey 가능(1GB 또는 1h마다).
6. **PSK mode**: 옵션, post-quantum 대비 pre-shared key 추가.
7. **Cloud Relay 기본값**: `wss://relay.gooseagent.org`, 인증 없음(암호화가 인증 겸함).
8. **Self-hosted**: Docker image `ghcr.io/gooseagent/goose-relay`, config는 TOML/YAML.
9. **Rate limiting**: 세션당 1Gbps cap, 전체 서버 10Gbps cap.
10. **관찰성**: Relay 서버 OTel 메트릭(세션 수, 처리량, 핸드셰이크 실패율). Relay는 payload를 볼 수 **없으므로** 메트릭은 메타데이터만.
11. **Go ↔ Rust FFI**: 기본 gRPC(tonic ↔ grpc-go), 핫패스는 CGO(goose-crypto static lib link).

### 3.2 OUT OF SCOPE

- **Bridge 프로토콜**: BRIDGE-001. Relay는 payload 내용에 무관심.
- **키 분배 인프라(PKI/CA)**: pairing 시 QR 코드로 서버 공개키 직접 전달. 1차는 자체 서명, 향후 X.509 통합 검토.
- **Onion routing / Tor 연동**: v2+.
- **Multi-hop relay(체인)**: 1차는 단일 hop.
- **P2P direct(hole punching)**: v2+. 1차는 항상 relay 경유.
- **HTTP fallback**: Relay는 WebSocket 전용.
- **Accounting / billing**: 무료 공식 relay. 1GB/일 soft cap은 metric only (차단은 v2+).

---

## 4. 의존성 (Dependencies)

- **상위 의존**: SPEC-GOOSE-BRIDGE-001(Bridge가 Relay 인터페이스 소비).
- **독립 의존 없음**: RELAY-001은 BRIDGE-001과 동시 개발 가능하지만, BRIDGE-001이 `Transport` 인터페이스를 정의해야 끼워 맞춤.
- **Rust 크레이트**:
  - `snow` 0.9+ (Noise Protocol)
  - `chacha20poly1305` 0.10+
  - `x25519-dalek` 2.x
  - `blake2` 0.10+
  - `zeroize` 1.8+ (secret 소거)
  - `ring` 0.17+ (RNG)
- **Go 라이브러리**:
  - `github.com/gorilla/websocket` (Relay 서버)
  - `google.golang.org/grpc` (Go ↔ Rust FFI 기본 경로)

---

## 5. 요구사항 (EARS Requirements)

### 5.1 Ubiquitous

- **REQ-RL-001**: The Relay service **shall** implement the Noise Protocol pattern `Noise_XK_25519_ChaChaPoly_BLAKE2s` using a reviewed Rust cryptography stack.
- **REQ-RL-002**: The Relay service **shall not** retain, log, or inspect payload plaintext.
- **REQ-RL-003**: Every tunnel session **shall** use unique ephemeral keys derived per handshake.
- **REQ-RL-004**: The Relay service **shall** zero out all session keys from memory within 10ms after session close using `zeroize` or equivalent guarantees.

### 5.2 Event-Driven

- **REQ-RL-005**: **When** a PC client connects with a valid `relay_session_id`, Relay **shall** place the connection in a waiting pool until the matching Mobile client connects or the 5-minute pairing window expires.
- **REQ-RL-006**: **When** both endpoints of a session are connected, Relay **shall** initiate opaque packet forwarding without parsing payload.
- **REQ-RL-007**: **When** either endpoint disconnects, Relay **shall** close the peer connection within 1 second and release session resources.

### 5.3 State-Driven

- **REQ-RL-008**: **While** a tunnel session is active and throughput exceeds 1GB cumulative or 1 hour duration, the Go-side client **shall** trigger a rekey by initiating a new Noise handshake transparently.
- **REQ-RL-009**: **While** the Relay service is under full capacity (configurable, default 10000 concurrent sessions), new session requests **shall** be rejected with HTTP 503 and a retry-after header.

### 5.4 Optional

- **REQ-RL-010**: **Where** a pre-shared key is configured, the Noise handshake **shall** incorporate the PSK via the `psk2` Noise modifier for post-quantum resistance.
- **REQ-RL-011**: **Where** the user provides a self-hosted Relay URL, the client **shall** skip the official cloud relay and connect directly to the configured endpoint.

### 5.5 Unwanted Behavior

- **REQ-RL-012**: **If** a handshake fails signature validation, **then** Relay **shall** close the connection with WebSocket code 4403 and record the failure in metrics without logging any payload bytes.
- **REQ-RL-013**: **If** a session exceeds the per-session 1Gbps rate cap, **then** Relay **shall** throttle the session using token-bucket without terminating it.
- **REQ-RL-014**: **If** the Rust crypto library reports a decryption failure on the client side, **then** the Go client **shall** terminate the tunnel immediately without retry and surface a hard-fail error to Bridge.

### 5.6 Complex

- **REQ-RL-015**: **While** a tunnel is active, **when** a rekey is triggered, Relay **shall** forward new handshake packets without interrupting the existing data flow — the client performs atomic switch to the new keys only after the new handshake completes and is acknowledged.

---

## 6. 핵심 Go 인터페이스 시그니처

```go
// internal/relay/types.go
// Relay 클라이언트/서버 공통 타입. 실제 암호 연산은 CryptoProvider(Rust) 위임.

// Relay는 PC/Mobile 양쪽에서 사용되는 E2EE 터널 client 인터페이스.
type Relay interface {
    // Dial은 Relay 서버에 접속하여 상대 endpoint와 매칭하고 Noise handshake 완료까지 수행.
    Dial(ctx context.Context, opts DialOptions) (NoiseTunnel, error)

    // Listen은 상대 endpoint의 접속을 기다린다 (PC 측 사용).
    Listen(ctx context.Context, opts ListenOptions) (NoiseTunnel, error)

    // Close는 Relay 클라이언트 자원을 해제.
    Close() error
}

// RelayEndpoint는 Relay 서버 URL 및 인증 메타데이터.
type RelayEndpoint struct {
    URL              string   // 예: wss://relay.gooseagent.org
    ServerPublicKey  []byte   // 32B ed25519 or X25519, 자체 서명 검증용
    PreSharedKey     []byte   // 옵션 PSK (32B), nil 가능
}

// DialOptions: Relay 서버에 연결할 때 필요한 정보.
type DialOptions struct {
    Endpoint        RelayEndpoint
    SessionID       string         // Bridge가 발급한 relay_session_id
    LocalPrivateKey []byte         // 클라이언트 개인키 (X25519 32B)
    PeerPublicKey   []byte         // 상대 공개키 (X25519 32B), pairing 시 교환
    Timeout         time.Duration  // 기본 5분 (pairing 윈도우)
}

type ListenOptions = DialOptions

// NoiseTunnel은 Noise 핸드셰이크 완료 후 양방향 암호화 스트림.
// io.ReadWriteCloser 패턴. Read/Write는 자동 암/복호화.
type NoiseTunnel interface {
    io.Reader
    io.Writer
    io.Closer

    // Rekey는 명시적 재협상 트리거 (REQ-RL-008).
    Rekey(ctx context.Context) error

    // Stats는 터널 통계 (바이트 카운트, 경과 시간).
    Stats() TunnelStats
}

type TunnelStats struct {
    BytesSent     uint64
    BytesReceived uint64
    EstablishedAt time.Time
    HandshakeMs   uint32
    RekeyCount    uint32
}

// CryptoProvider는 실제 암호 연산(Rust 위임) 인터페이스.
// Go 코드는 이 인터페이스만 의존하고, 구현체는 gRPC 또는 CGO로 Rust binding.
type CryptoProvider interface {
    // GenerateKeypair는 X25519 키 쌍 생성.
    GenerateKeypair(ctx context.Context) (pub, priv []byte, err error)

    // InitiateHandshake는 XK 패턴 initiator (Mobile 측).
    InitiateHandshake(ctx context.Context, priv, peerPub, psk []byte) (HandshakeHandle, error)

    // RespondHandshake는 XK 패턴 responder (PC 측).
    RespondHandshake(ctx context.Context, priv, psk []byte) (HandshakeHandle, error)

    // Encrypt은 완료된 핸드셰이크 핸들로 평문을 암호화.
    Encrypt(handle HandshakeHandle, plaintext []byte) ([]byte, error)

    // Decrypt는 ciphertext 복호화. 검증 실패 시 error.
    Decrypt(handle HandshakeHandle, ciphertext []byte) ([]byte, error)

    // Zeroize는 핸들 내 모든 비밀 소거.
    Zeroize(handle HandshakeHandle) error
}

type HandshakeHandle uint64 // Rust 쪽 핸들 ID (CGO 또는 gRPC)

// RelayServer는 self-hosted/official relay 서버 공통 인터페이스.
// (internal/relay/server/ 하위에 구현체)
type RelayServer interface {
    Start(ctx context.Context, listenAddr string) error
    Stop(ctx context.Context) error
    ActiveSessions() int
    Metrics() ServerMetrics
}

type ServerMetrics struct {
    SessionsActive    int
    SessionsAccepted  uint64
    SessionsRejected  uint64
    HandshakesSucceeded uint64
    HandshakesFailed  uint64
    BytesRelayed      uint64
}
```

---

## 7. 수락 기준 (Acceptance Criteria)

### 7.1 AC-RL-001 — Noise 핸드셰이크 성공

**Given** PC와 Mobile이 동일 `relay_session_id` 보유 **When** 양쪽이 Relay에 접속 **Then** XK 핸드셰이크가 1-RTT 내 완료되고 양방향 터널이 established 상태.

### 7.2 AC-RL-002 — 평문 접근 불가 (Relay 관찰자 테스트)

**Given** Relay가 실행 중 **When** 패킷 캡처(tcpdump)로 관찰 **Then** 모든 payload가 ChaCha20-Poly1305 ciphertext. decrypt 시도 시 Auth Tag 검증 실패.

### 7.3 AC-RL-003 — 세션 close 후 키 zeroize

**Given** 터널이 established **When** 클라이언트가 Close 호출 **Then** 10ms 이내 HandshakeHandle 내 모든 key 영역이 0으로 덮여지며, 이후 Encrypt 호출은 `ErrHandleClosed` 반환.

### 7.4 AC-RL-004 — 변조 탐지

**Given** 양쪽 터널 established **When** 중간에 ciphertext 1비트 변조 **Then** Decrypt가 `AEAD verification failed` 에러 반환, Bridge가 터널 즉시 종료(REQ-RL-014).

### 7.5 AC-RL-005 — 5분 페어링 윈도우 만료

**Given** PC만 Relay에 접속, Mobile 미접속 **When** 5분 경과 **Then** Relay가 PC 연결 close, 이후 동일 session_id 재사용 불가.

### 7.6 AC-RL-006 — Rekey 투명 전환

**Given** 활성 터널, 1GB 데이터 송신 도달 **When** Rekey 트리거 **Then** 새 핸드셰이크가 기존 스트림 유지 상태로 진행, 완료 후 클라이언트가 새 키로 atomic 전환. 애플리케이션 읽기/쓰기 중단 없음.

### 7.7 AC-RL-007 — PSK 모드

**Given** PSK 32B 설정 **When** 동일 PSK 양쪽 **Then** 핸드셰이크 성공. PSK 불일치 **Then** 핸드셰이크 실패 에러.

### 7.8 AC-RL-008 — Self-hosted 전환

**Given** config에 `relay.url = wss://my-relay.local` **When** Dial 호출 **Then** 공식 cloud relay에 접속하지 않고 지정 URL에 직접 연결.

### 7.9 AC-RL-009 — Rate limit

**Given** 단일 세션 2Gbps 송신 시도 **When** Relay가 트래픽 측정 **Then** 1Gbps로 throttle되지만 세션은 유지. 양측 애플리케이션은 throughput 저하만 관찰.

### 7.10 AC-RL-010 — Fuzz 저항

**Given** `cargo fuzz` 10분 실행 **When** 무작위 패킷을 Rust Noise decoder에 주입 **Then** 프로세스 crash 없음, 모든 입력이 정상적으로 reject 또는 valid parse.

---

## 8. TDD 전략

- **RED (Rust)**:
  - `cargo test` — Noise handshake roundtrip, encrypt/decrypt, AEAD tamper detection.
  - `cargo fuzz` — decoder fuzz.
- **RED (Go)**:
  - `relay_test.go` — mock CryptoProvider로 Dial/Listen 생명주기.
  - `server_test.go` — 세션 매칭, 타임아웃, rate limit.
- **GREEN**:
  - Rust: `snow` crate wrapping, `zeroize` for Drop impl.
  - Go: Relay 서버는 gorilla/websocket으로 단순 byte forwarding.
- **REFACTOR**:
  - Rust: unsafe 0 유지 (tech.md §12.1), clippy strict.
  - Go: CryptoProvider 백엔드 교체 가능(gRPC vs CGO) abstraction.
- **통합**:
  - `integration/e2e_tunnel_test.go` — 실제 Relay server + 2 clients + full handshake + 100MB 전송.
- **벤치마크**:
  - `cargo bench` — handshake 지연, encrypt throughput.
  - Go: relay 서버 10000 동시 세션 부하 테스트.
- 커버리지: Rust 80%+, Go 85%+.

---

## 9. 제외 항목 (Exclusions)

- **Tor/onion routing 통합**: v2+.
- **Multi-hop relay chain**: v2+.
- **P2P hole punching(NAT traversal)**: v2+. 1차는 항상 relay 경유.
- **HTTP fallback**: WebSocket만.
- **mTLS 인증**: Noise가 인증·기밀성 모두 제공하므로 mTLS 중복 안 함.
- **CA 인프라(PKI)**: 자체 서명 공개키를 pairing 시 직접 교환. 웹 인증서 필요 없음.
- **Billing/quota enforcement**: 1차 metric only.
- **DDoS 방어(L7 rate limit 외)**: Cloudflare 계층이 담당.
- **Relay 서버 고가용성/federation**: 1차 단일 region. 이후 multi-region은 별도 SPEC.
