# SPEC-GOOSE-RELAY-001 — Research Notes

> Noise Protocol 채택 근거, Mullvad GotaTun 패턴 분석, Go ↔ Rust FFI 설계, BRIDGE-001과의 경계 명확화.

---

## 1. Noise Protocol 선택 근거

### 1.1 Noise vs TLS 1.3 비교

| 항목 | Noise Protocol (XK) | TLS 1.3 |
|-----|--------------------|---------| 
| 핸드셰이크 왕복 | 1-RTT | 1-RTT (0-RTT 옵션) |
| 핸드셰이크 크기 | ~96 bytes | ~1500 bytes (cert chain) |
| 인증 방식 | 정적 공개키 사전 공유 | X.509 체인 + CA |
| CPU 오버헤드 | 낮음 (AEAD만) | 중간 (체인 검증 + AEAD) |
| 구현 복잡도 | 낮음 (단일 RFC) | 높음 (다중 확장) |
| 메타데이터 누출 | 서버 공개키 식별 불가 (XK) | SNI 노출 |
| 라이브러리 성숙도 | `snow` (Rust), `noise-java` 등 | OpenSSL, rustls |

GOOSE는 **소규모 고정 endpoint** (PC ↔ Mobile 1:1)라 CA 기반 X.509가 오버킬이다. Noise가 최적.

### 1.2 왜 XK 패턴인가

Noise 표준 패턴들 중:

- **NN**: 양쪽 익명 — 인증 없음, MITM 취약
- **XX**: 양쪽 interactive exchange — 3-RTT
- **XK**: responder 정적 키만 사전 공유 — 1-RTT, 익명 initiator 지원
- **IK**: 양쪽 정적 키 사전 공유 — 1-RTT, 가장 엄격

GOOSE 시나리오:
- PC = responder, 자체 정적 X25519 키 보유 + QR로 Mobile에 전달
- Mobile = initiator, 페어링 시 PC 공개키 수령 → XK에 부합
- Mobile 정적 키는 pairing 시 전송(Trusted Device 등록) → 이후 세션에서는 Mobile ephemeral만 사용해도 무방

**결론**: XK 채택.

### 1.3 Suite: `Noise_XK_25519_ChaChaPoly_BLAKE2s`

- **KEM**: X25519 (curve25519)
- **Cipher**: ChaCha20-Poly1305 (AEAD)
- **Hash**: BLAKE2s (32 bytes)

이 suite는 WireGuard와 동일하며 `snow` 기본 제공.

---

## 2. Mullvad GotaTun Go→Rust 마이그레이션 교훈

Mullvad VPN은 2026년 초 WireGuard 엔진을 Go에서 Rust(GotaTun)로 이전. 이유:

| 문제 (Go) | 해결 (Rust) |
|---------|-----------|
| GC가 예측 불가 지연 유발 (p99 spike) | Rust는 ownership, GC 없음 |
| Go runtime이 secret memory 보유 후 불확실한 zero 시점 | `zeroize` crate + Drop impl로 결정적 소거 |
| Goroutine 스케줄러가 crypto hot path latency 증가 | 네이티브 스레드, 더 예측 가능 |
| Mobile 배터리 수명: Go 이진의 주기적 GC sweep | Rust는 steady-state 전력 낮음 |

**배터리 수명 15-20% 향상 보고**(techradar 2026).

GOOSE도 Mobile을 지원하므로 동일 교훈 적용 필수. E2EE 핵심 코어는 Rust로.

---

## 3. Go ↔ Rust FFI 설계

### 3.1 두 가지 경로

#### (A) gRPC (기본)

```
Go Relay client --grpc--> goose-crypto sidecar (Rust, localhost)
```

- 장점: 독립 프로세스, 개별 빌드/배포, 디버그 쉬움
- 단점: 핸드셰이크당 ~1-2ms 오버헤드

#### (B) CGO + static link (핫 패스)

```
Go Relay client --CGO--> libgoose_crypto.a (Rust, 같은 프로세스)
```

- 장점: 지연 ~0, 동일 주소공간
- 단점: cross-compile 복잡, cbindgen 필요

### 3.2 결정

- **기본**: gRPC(A) — 개발 단계 & 대부분의 세션 생애주기 작업(핸드셰이크, close)에는 충분
- **핫 패스**: CGO(B) — 터널 established 이후 Encrypt/Decrypt는 매 메시지 호출이라 CGO 필수

Config: `relay.crypto_transport = "grpc" | "cgo"` (기본 grpc, 성능 중시는 cgo).

### 3.3 cbindgen 스켈레톤

```rust
// crates/goose-crypto/src/lib.rs
#[no_mangle]
pub extern "C" fn goose_crypto_init_handshake(
    priv_key: *const u8, priv_len: usize,
    peer_pub: *const u8, peer_pub_len: usize,
    psk: *const u8, psk_len: usize,
    out_handle: *mut u64,
) -> i32 { /* ... */ }

#[no_mangle]
pub extern "C" fn goose_crypto_encrypt(
    handle: u64,
    plaintext: *const u8, pt_len: usize,
    out_buf: *mut u8, out_cap: usize,
    out_len: *mut usize,
) -> i32 { /* ... */ }
```

cbindgen으로 `goose_crypto.h` 생성 → Go `#cgo` 지시어로 링크.

---

## 4. BRIDGE-001과의 경계 명확화

| 책임 | BRIDGE-001 | RELAY-001 |
|-----|-----------|-----------|
| 세션 프로토콜 (JWT, message type) | ✅ | ❌ |
| 권한 콜백 | ✅ | ❌ |
| 전송 추상화 (WS/SSE/Polling) | ✅ | ❌ (WebSocket 내부 사용만) |
| 암호화 (encrypt/decrypt) | ❌ | ✅ |
| 핸드셰이크 | ❌ | ✅ |
| 중계 서버 운영 | ❌ | ✅ |
| NAT 우회 | ❌ | ✅ (모든 트래픽 relay 경유) |

Bridge는 "Relay의 평문 스트림" 위에서 동작한다. 즉 Bridge 코드는 암호화 코드를 전혀 보지 않으며, 모든 byte가 이미 복호화된 상태로 도착한다.

---

## 5. 공식 Cloud Relay 운영 고려

### 5.1 스펙

- **URL**: `wss://relay.gooseagent.org`
- **Region**: 1차 us-east-1, ap-northeast-2 (Korea) 추가
- **Capacity**: 10000 concurrent sessions per instance
- **SLA**: best-effort (free tier), 99.5% target

### 5.2 프라이버시 약속

- 서버는 **평문 접근 불가** (Noise가 보장)
- 메타데이터 최소 저장: session_id, connection timestamp, byte counts
- IP 주소는 로그에 기록하지 않음 (Cloudflare 앞단에서 obfuscate)

### 5.3 Self-hosted

사용자가 자체 Relay를 운영하고 싶은 경우:

```yaml
# relay-config.yaml
relay:
  listen: "0.0.0.0:443"
  tls:
    cert: /etc/letsencrypt/live/my-relay/fullchain.pem
    key: /etc/letsencrypt/live/my-relay/privkey.pem
  max_sessions: 1000
  metrics:
    enabled: true
    port: 9090
```

Docker:
```bash
docker run -d -p 443:443 \
  -v $PWD/config.yaml:/etc/goose-relay/config.yaml \
  ghcr.io/gooseagent/goose-relay:v1
```

---

## 6. Noise XK 핸드셰이크 상세

### 6.1 메시지 시퀀스

```
Initiator (Mobile)           Responder (PC)
  e, es                  ->
                         <-  e, ee
  s, se                  ->
  <establishment complete, both directions>
```

- `e`: ephemeral public key
- `s`: static public key
- `es`/`ee`/`se`: DH exchanges derive chaining key

1-RTT 완료. Handshake 패킷 크기 ~96 bytes.

### 6.2 PSK 모드

`Noise_XK_psk2_25519_ChaChaPoly_BLAKE2s` 변형:

```
  e, es                  ->
                         <-  e, ee, psk
  s, se                  ->
```

PSK는 1회성 pre-shared key 32B. post-quantum 내성 강화 (PQ 알고리즘이 뚫려도 PSK 없이는 해독 불가).

pairing 시 QR 코드에 PSK 포함 옵션.

---

## 7. 메모리 안전 보증

### 7.1 Rust 쪽

```rust
use zeroize::Zeroize;

#[derive(Zeroize)]
#[zeroize(drop)]
struct SessionKeys {
    send_key: [u8; 32],
    recv_key: [u8; 32],
    chaining_key: [u8; 32],
}

impl Drop for SessionKeys {
    fn drop(&mut self) {
        self.zeroize();  // 명시적 0-fill
    }
}
```

`#[zeroize(drop)]`가 Drop impl 자동 생성. Rust는 panic 시에도 Drop 실행(safe unwind).

### 7.2 Go 쪽 제약

Go는 secret을 보유해서는 안 된다. `CryptoProvider` 인터페이스는 handle(u64)만 주고받으며, 실제 key material은 Rust 주소공간에만 존재.

만약 Go가 PSK를 읽어야 한다면, 즉시 Rust에 전달 후 Go 쪽 slice를 `runtime.KeepAlive + clear` 호출.

---

## 8. TDD 전략 상세

### 8.1 Rust (cargo test)

1. `handshake_xk_roundtrip`: initiator/responder 양측 핸드셰이크 성공 & 동일 transport key 도출
2. `handshake_wrong_peer_pub`: responder가 다른 정적 키 보유 시 실패
3. `encrypt_decrypt_roundtrip`: 평문 `p` → ciphertext `c` → 평문 `p` 동일
4. `tamper_detection`: ciphertext 1 bit 변조 시 decrypt 실패
5. `rekey_before_after`: rekey 후 예전 키로 decrypt 불가
6. `zeroize_on_drop`: Drop 후 unsafe pointer peek로 0 확인 (테스트 전용)
7. `psk_mismatch`: PSK 다르면 handshake fail

### 8.2 Rust (cargo fuzz)

- `fuzz_noise_decode`: 무작위 byte를 Noise handshake decoder에 주입
- `fuzz_ciphertext`: 무작위 byte를 Decrypt에 주입

### 8.3 Go (go test)

1. `relay_server_matching`: 두 클라이언트가 동일 session_id로 접속 → pair
2. `relay_server_timeout`: 5분 내 상대 없음 → close
3. `relay_server_rate_limit`: session 하나가 대용량 전송 → throttle
4. `relay_client_dial_listen`: mock CryptoProvider로 Dial + Listen 시나리오
5. `relay_client_rekey`: 1GB 초과 시 자동 Rekey 호출 확인

### 8.4 통합 테스트

- `integration/e2e_full_tunnel_test.go`:
  - 실제 goose-relay 서버 docker 실행
  - PC 역할 Go 테스트, Mobile 역할 Go 테스트
  - 100MB 파일 전송 → SHA-256 일치 확인
  - 마지막 1bit 변조 → 복호화 실패 확인

### 8.5 벤치마크

- Rust `criterion`:
  - handshake 시간 (p50/p99)
  - encrypt throughput (MB/s) — 목표 > 2 Gbps (tech.md §7)
  - decrypt throughput
- Go:
  - 10000 동시 세션 relay 서버 부하
  - 세션당 지연 p99

---

## 9. 오픈 이슈

1. **Noise 구현체**: `snow` 0.9 vs `rust-noise` 0.1. snow가 더 성숙, 채택.
2. **X25519 constant-time 구현 확인**: `x25519-dalek` 2.x는 constant-time 보장, OK.
3. **Relay 서버 언어**: Go로 결정 (세션 매칭은 비암호화 영역, Go가 적합). 암호는 Client만 Rust.
4. **PSK 분배 UX**: QR에 넣으면 QR 크기 증가. 1차는 PSK 옵션만, 기본 off.
5. **Rekey 트리거 기준**: 1GB/1h 외에 "시간 기준" 만료 (60분) 추가 여부. snow는 nonce exhaustion 자동 방지.
6. **Go ↔ Rust 기본 경로**: 성능 vs 독립성. 1차 gRPC로 시작하되 core loop(encrypt/decrypt)만 CGO 옵션.
7. **Relay 서버 로드밸런싱**: 단일 인스턴스 한계. L7 consistent hash(session_id 기반)로 scale-out 가능 but v2+.

---

## 10. 참조

- Noise Protocol Framework: <http://noiseprotocol.org/noise.html> (rev 34)
- RFC 8439 (ChaCha20-Poly1305)
- RFC 7748 (X25519, X448)
- `snow` crate: <https://crates.io/crates/snow>
- WireGuard Whitepaper: <https://www.wireguard.com/papers/wireguard.pdf>
- Mullvad GotaTun announcement (2026): <https://mullvad.net/blog/gotatun>
- tech.md §6.1 "E2EE (Rust goose-crypto)"
- tech.md §2.2 "Go ↔ Rust 통신"
