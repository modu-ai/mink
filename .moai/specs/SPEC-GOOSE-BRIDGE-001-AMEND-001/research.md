# Research — SPEC-GOOSE-BRIDGE-001-AMEND-001

> Cookie-hash logical session mapping amendment 의 설계 공간 분석. spec.md §5 의 결정을 뒷받침하는 증거 정리.

## 1. 문제 도메인

부모 v0.2.1 의 `internal/bridge/` 는 outbound replay buffer 가 connID 단위로 keying 된다. 새 WebSocket upgrade 또는 SSE reconnect 가 매번 새 `connID = sid + "-" + randSuffix()` 를 발급하므로, 이전 connID 의 buffer 는 보존되지만 도달 불가능한 상태가 된다.

코드 인용 (`internal/bridge/`):

- `ws.go:109` — `connID := sid + "-" + randSuffix()` (매 upgrade)
- `sse.go:88` — 동일 패턴 (매 SSE reconnect)
- `outbound.go:107~118` — `seq := d.nextSequence(sessionID); ... d.buffer.Append(msg)` (sessionID = connID)
- `buffer.go:84` — `q := b.queues[msg.SessionID]` (connID-keyed map)
- `resume.go:66~69` — `r.buffer.Replay(sessionID, lastSeq)` (connID 로 조회)
- `ws.go:149~155` — `for _, msg := range h.cfg.resumer.Resume(connID, r.Header)` — replay 호출 사이트, 결과는 항상 빈 슬라이스

결과: 부모 v0.2.1 의 AC-BR-009 (offline buffering + replay) 와 AC-BR-015 (session resume) 의 통합 테스트는 sessionID 가 일치하는 단순 시나리오만 검증하므로 결손이 잡히지 않았다.

## 2. 대안 설계 비교

| 옵션                                                          | LogicalID 정의                                | Buffer 키            | Multi-tab 의미              | Cross-transport replay | 복잡도 |
| ------------------------------------------------------------- | --------------------------------------------- | -------------------- | --------------------------- | ---------------------- | ------ |
| **A. 권장 — HMAC(cookieHash + transport)** (본 amendment)     | cookieHash + transport 길이-prefix concat HMAC | LogicalID            | 같은 cookie+transport 공유  | 불가 (의도된 격리)     | 중     |
| B. cookieHash only (transport 무시)                           | HMAC(cookieHash)                              | LogicalID            | 모든 cookie 공유            | 가능                   | 중     |
| C. 외부 인덱스 + connID 키 유지                               | 별도 map[cookieHash][]connID                  | connID               | 동일 cookie 의 모든 connID 검색해서 evict / replay | 가능 | 고     |
| D. broadcast dispatch + connID 키 유지                        | (LogicalID 도입하지 않음)                     | connID               | dispatcher 가 모든 active sender 에 emit | 가능 | 매우 고 |

### 2.1 옵션 A (채택) — Why

- **단일 책임 원칙**: buffer 는 LogicalID 단위로 일관되게 keying. dispatcher / resumer / buffer 의 mental model 이 동일.
- **부모 시그니처 보존**: `dispatcher.SendOutbound(sessionID, ...)`, `resumer.Resume(connID, ...)` 그대로. 외부 caller 영향 0건.
- **transport 격리의 안전성**: WS 의 frame 단위 framing 과 SSE 의 `id:` 라인 framing 은 sequence 의미가 다르다. WS 는 binary/text frame, SSE 는 `id:` + `event:` + `data:` 라인. cross-transport replay 는 framing/parsing 으로 인한 corner case 가 많다 (예: SSE 의 `Last-Event-ID` 가 마지막 receive 한 id 인지, 마지막 process 한 id 인지 EventSource 스펙이 모호한 면이 있음). transport 별 분리 default 가 더 안전.
- **Sequence 단조성**: dispatcher 의 `sequences map` 의 키도 LogicalID 로 일치시키면 `(LogicalID, Sequence)` 가 invariant. multi-tab race 가 자연스럽게 안전.

### 2.2 옵션 B (거부) — Why not

- **장점**: 같은 cookie 가 transport 를 바꾸어 reconnect 해도 replay 가 동작. (예: WS 가 차단되어 SSE 로 fallback 하는 시나리오에 메시지가 자동 전달.)
- **단점 1**: WS frame 과 SSE event 의 sequence 는 서로 다른 sender 가 발급하더라도, 같은 LogicalID 에 합쳐지면 두 transport 의 sequence 카운터가 충돌 가능. dispatcher 가 cross-transport sequence 단조성을 보장해야 하는데 sender 가 transport 별이므로 통일 어렵다.
- **단점 2**: SSE `Last-Event-ID` 와 WS `X-Last-Sequence` 가 같은 sequence 공간을 공유한다는 보장이 클라이언트 측에서 깨질 수 있다. 브라우저의 EventSource 는 `Last-Event-ID` 를 자동으로 헤더에 채우는데, 같은 cookie 가 WS 로 다시 접속할 때 WS client 가 그 값을 알 방법이 없다. 클라이언트 협력이 더 복잡.
- **단점 3**: transport fallback 시 명시적 fresh session 을 받는 패턴이 (a) framing 일관성, (b) re-auth 보장, (c) 디버깅 가능성 면에서 더 안전.
- **결정**: 거부. cross-transport replay 가 정말 필요해지면 별도 SPEC 에서 transport-agnostic LogicalID 와 함께 sequence 단조성 보장 스킴을 도입.

### 2.3 옵션 C (거부) — Why not

- **장점**: buffer 의 키가 connID 그대로 — 변경 범위가 가장 좁아 보임.
- **단점 1**: cookieHash → []connID 인덱스가 별도 mutex 영역으로 등장 → registry 의 동시성 책임이 늘어남.
- **단점 2**: replay 시점에 dispatcher / resumer 가 cookieHash 인덱스를 조회 → 여러 connID 의 buffer 를 sequence 순으로 merge 해야 함 → buffer 자체가 cross-connID merge 알고리즘을 가져야 함 → LogicalID 도입과 동일한 비용에 도달.
- **단점 3**: evict 정책 (4MB / 500 msg / 24h TTL) 이 cross-connID 로 일관되지 않음 — connID 별 4MB 가 서로 분리되어 효과적 buffer 가 cookieHash 기준 N×4MB 가 됨.
- **결정**: 거부.

### 2.4 옵션 D (거부) — Why not

- **장점**: multi-tab live broadcast UX 보장. 사용자가 두 탭에서 동일한 응답을 동시에 보게 됨.
- **단점 1**: 부모 dispatcher.SendOutbound(sessionID, ...) 의 sessionID 인자가 broadcast 에서 의미를 잃음 → 시그니처 변경 필요.
- **단점 2**: flush-gate 가 sessionID 단위인데 broadcast 시 어느 게이트를 따르는지 모호 → flush-gate 도 LogicalID 단위로 옮겨야 함 → backpressure 설계가 깨짐.
- **단점 3**: broadcast 자체가 사용자 UX 요구 사항인지 미확인 (spec.md §5.4 Open Question 1). 명확한 요구가 있을 때까지 도입하지 않는 편이 안전.
- **결정**: 거부. 별도 SPEC 후보.

## 3. LogicalID 산출 방식 — 보안 / 결정성 검토

### 3.1 함수 정의 (최종, spec.md §3.1 + REQ-BR-AMEND-001 와 동기화)

```
DeriveLogicalID(cookieHash []byte, transport Transport) string
  := base64.RawURLEncoding(
       HMAC_SHA256(secret = bridge_cookie_hmac_secret,
                   data = "bridge-logical-id-v1\x00"
                       || uvarint(len(cookieHash))
                       || cookieHash
                       || transport_string)
     )[:32]
```

세 부분으로 분리해 해석:
1. `"bridge-logical-id-v1\x00"` — fixed domain-separator (NIST SP 800-108 키 분리, 21 bytes).
2. `uvarint(len(cookieHash)) || cookieHash` — 가변 길이 prefix-encoded cookie hash.
3. `transport_string` — `"websocket"` 또는 `"sse"` ASCII.

### 3.2 보안 속성 (Design Note — D5/OQ2 통합)

이 절은 spec.md §3.1 + §5.4 의 design 결정의 근거를 정리한다. 더 이상 open question 이 아니며, M1-T1 의 implementation contract 로 고정된다.

**보안 목표 1 — Pre-image 저항**: HMAC-SHA256 이므로 LogicalID 만 알아도 cookieHash 복원 불가. SHA-256 의 256-bit pre-image 저항 그대로.

**보안 목표 2 — Collision 저항**: SHA-256 의 collision 저항 기대. 다만 MAC truncation (32 chars base64url ≈ 192 bits) 로 인해 birthday bound 가 96 bits 로 떨어지므로, cookie 공간이 2^96 미만임을 가정 (현실적으로는 2^128 cookie space, 안전 margin 충분).

**보안 목표 3 — 도메인 분리 (D5 fix, NIST SP 800-108 키 분리)**:
- 문제: `auth.go:51 HMACSecret` 는 cookie 서명/검증 (`a.sign(payload)` at auth.go:172) 에 이미 사용 중. 동일 secret 으로 LogicalID 까지 파생하면 한 secret 이 두 cryptographic primitive (cookie 서명 vs identifier 파생) 에 사용된다.
- 위험 시나리오: LogicalID 가 텔레메트리 (OTel attribute, 디버그 로그, 에러 메시지) 로 의도치 않게 노출될 경우, attacker 는 `HMAC(secret, cookieHash || transport)` 의 output 을 관찰할 수 있는 oracle 을 획득. 이는 secret 의 cookie HMAC 분석에 미세하게 도움.
- Mitigation: `"bridge-logical-id-v1\x00"` prefix 를 input 에 포함시켜 cookie 서명 input 공간 (`payload = sessionID || expiry`, auth.go:102~104) 과 절대 겹치지 않도록 분리. cookie 서명 input 은 24-byte fixed length 이고 LogicalID input 은 `"bridge-logical-id-v1\x00"` 로 시작하므로 두 input 공간은 byte-wise disjoint.
- 비용: 단일 prefix 추가로 21 bytes 의 input overhead. RFC 4107 / NIST SP 800-108 의 키 도출 모범 사례 준수.
- 대안 검토:
  - HKDF-Expand: 더 정통적이지만 코드 복잡도 증가 (subkey 생성 + 캐싱). 단일 prefix 가 동등한 보안을 제공하므로 거부.
  - 완전 별도 secret 도입: lifecycle 관리 표면 증가 (rotation, KMS 통합). 본 amendment scope 초과.

**보안 목표 4 — Length-prefix attack 방어 (구 OQ2 결정 근거)**:
- 문제: `cookieHash || transport` 의 단순 concat 형태는 cookieHash 가 가변 길이가 될 경우 ambiguous parsing 가능. 예: cookieHash = `[0xAA, 0xBB]` + transport = `"sse"` 와 cookieHash = `[0xAA, 0xBB, 0x73, 0x73]` + transport = `"e"` 가 같은 byte sequence 를 produce.
- 현재 cookieHash 는 SHA-256 출력 32 byte 고정 (auth.go:148) 이므로 이 attack 은 실제로는 적용 불가. 그러나 향후 cookieHash 가 다른 hash (e.g., SHA-512 64 byte, BLAKE2 가변) 로 변경될 경우 안전하지 않다.
- Mitigation: `uvarint(len(cookieHash))` length prefix 를 input 에 포함. cookieHash 가 임의 길이 byte 시퀀스라도 prefix-free 인코딩이 보장되어 다른 (hash, transport) 쌍이 동일 input 을 produce 할 수 없음.
- 결정 (구 OQ2): 명시적 length prefix 채택. spec.md §3.1 + REQ-BR-AMEND-001 에 반영. M1-T1 의 implementation contract.

**보안 목표 5 — Transport 분리**: `transport_string` 은 `"websocket"` 또는 `"sse"` ASCII (9 / 3 bytes). 두 값이 prefix 관계가 아니므로 ambiguous parsing 불가. 향후 새 transport 추가 시 동일한 prefix-disjoint 명명 규칙 유지 필요 (예: `"grpc"`, `"webtransport"`).

### 3.3 결정성 검증 (M1-T1 unit test)

- 같은 input 100 회 반복 → 100 개 동일 출력.
- transport 단일 byte 변경 (e.g., "websocket" → "Websocket") → 다른 출력.
- cookieHash 단일 bit flip → 다른 출력.
- Domain prefix 변경 (`"bridge-logical-id-v1\x00"` → `"bridge-logical-id-v2\x00"`) → 다른 출력 — D5 fix 의 unit test 검증 (D5 도메인 분리가 input 에 실제로 포함되는지 확인).

## 4. dispatcher.SendOutbound 변경 패턴

부모 v0.2.1 의 `outboundDispatcher.SendOutbound(sessionID, t, payload)`:

```go
seq := d.nextSequence(sessionID)
msg := OutboundMessage{
    SessionID: sessionID,
    Sequence:  seq,
    ...
}
if d.buffer != nil {
    d.buffer.Append(msg)
}
sender := d.registry.Sender(sessionID)
if sender != nil { sender.SendOutbound(msg) }
```

본 amendment 후 (REQ-BR-AMEND-003):

```go
logicalID, ok := d.registry.LogicalID(sessionID)
if !ok {
    logicalID = sessionID  // fallback for tests / unmapped sessions
}
seq := d.nextSequence(logicalID)  // sequence map keyed by LogicalID
msg := OutboundMessage{
    SessionID: sessionID,  // wire envelope keeps connID for transport routing
    Sequence:  seq,
    ...
}
if d.buffer != nil {
    bufKeyMsg := msg
    bufKeyMsg.SessionID = logicalID  // buffer entry keyed by LogicalID
    d.buffer.Append(bufKeyMsg)
}
sender := d.registry.Sender(sessionID)
if sender != nil { sender.SendOutbound(msg) }  // emit to named connID only
```

핵심:
- `nextSequence` 의 키 = LogicalID → 같은 LogicalID 의 모든 connID 가 단조 sequence 공유.
- `buffer.Append` 의 키 = LogicalID → cross-connection replay 가능.
- `sender` 호출 시 인자 `msg.SessionID` 는 connID (wire envelope 에 connID 가 남는지 LogicalID 가 남는지 결정 — 본 amendment 는 connID 유지로 결정. wire 측은 transport 가 책임).

### 4.1 Wire envelope invariant (D3 fix — outboundEnvelope 안전성 근거)

위 swap 패턴 (`bufKeyMsg.SessionID = logicalID; d.buffer.Append(bufKeyMsg)`) 은 `OutboundMessage` 의 in-memory 사본 두 개가 서로 다른 SessionID 를 갖는 상태를 만든다. 이것이 wire 측에 어떤 영향도 주지 않는 근거는 `outbound.go:147~160` 의 `outboundEnvelope` 직렬화 형태에 있다:

```go
// outbound.go:147~150
type outboundEnvelope struct {
    Type     string          `json:"type"`
    Sequence uint64          `json:"sequence"`
    Payload  json.RawMessage `json:"payload,omitempty"`
}

// outbound.go:156~166
func encodeOutboundJSON(msg OutboundMessage) ([]byte, error) {
    env := outboundEnvelope{
        Type:     string(msg.Type),
        Sequence: msg.Sequence,
        Payload:  json.RawMessage(msg.Payload),
    }
    if len(msg.Payload) == 0 {
        env.Payload = nil
    }
    return json.Marshal(env)
}
```

`OutboundMessage.SessionID` 는 envelope 의 어느 필드에도 매핑되지 않는다. 따라서:
- buffer 에 저장된 `OutboundMessage{SessionID: logicalID, ...}` 와
- sender 에 전달된 `OutboundMessage{SessionID: connID, ...}` 는

`encodeOutboundJSON` 을 거치면 byte-equal JSON 을 produce 한다 (wire 표현 동일). 이 invariant 가 본 amendment 의 in-memory swap 안전성을 보장하며, spec.md §7.1 에 명시되었다.

**향후 깨질 수 있는 시나리오 (위험 알람)**: envelope 에 SessionID 필드를 추가하면 (예: 디버그 목적의 `"session_id": ...`), buffer 에서 replay 시 LogicalID 가 wire 로 누출. 이 경우 본 amendment 의 dispatcher 는 buffer 에서 꺼낸 message 의 SessionID 를 connID 로 다시 swap 한 뒤 sender 에 전달해야 한다. 향후 wire schema 변경 PR 은 본 §4.1 invariant 와 spec.md §7.1 을 함께 검토하도록 reviewer 안내.

대안 검토 — `OutboundMessage` struct 에 `LogicalID` 필드 별도 추가? — 거부. wire envelope (encodeOutboundJSON) 변경 → 클라이언트 호환성 영향. internal-only 변경으로 충분.

## 5. resumer.Resume 변경 패턴

부모 v0.2.1:

```go
func (r *resumer) Resume(sessionID string, h http.Header) []OutboundMessage {
    lastSeq := parseLastSequence(h)
    return r.buffer.Replay(sessionID, lastSeq)
}
```

본 amendment 후 (REQ-BR-AMEND-004):

```go
func (r *resumer) Resume(sessionID string, h http.Header) []OutboundMessage {
    lastSeq := parseLastSequence(h)
    if r.registry != nil {
        if logicalID, ok := r.registry.LogicalID(sessionID); ok {
            return r.buffer.Replay(logicalID, lastSeq)
        }
    }
    return r.buffer.Replay(sessionID, lastSeq)  // fallback
}
```

resumer struct 에 `registry *Registry` 필드 추가 (`newResumer(buf, reg)` 시그니처 변경 — package-private 이므로 호환).

## 6. dispatcher.dropSequence 의 의미 변경 (transient disconnect vs logout 분리)

부모 v0.2.1:
```go
func (d *outboundDispatcher) dropSequence(sessionID string) {
    delete(d.sequences, sessionID)
    d.buffer.Drop(sessionID)
    d.gate.Drop(sessionID)
}
```

본 amendment 는 lifecycle event 종류에 따라 두 가지 정책을 분리한다:

### 6.1 Transient disconnect (lazy TTL — out-of-scope, spec §10 item 5)

시나리오: 네트워크 stall, 브라우저 tab background, 일시적 connection drop. 사용자는 곧 같은 cookie 로 재접속할 의도.

정책:
- `registry.Remove(connID)` 시점에 같은 LogicalID 의 다른 active connID 가 있으면 sequence counter 와 buffer 를 유지해야 함 (replay 보존).
- 마지막 connID 가 unregister 되더라도 LogicalID 의 sequence counter 와 buffer 는 lazy 로 24h TTL 에 위임.
- 이렇게 하면 multi-tab 의 한쪽 close 가 다른 쪽의 sequence/buffer 를 침해하지 않으며, 같은 cookie 로 재접속한 새 connID 가 이전 buffer 를 회수 가능.

이 결정은 M3-T3 의 implementation detail. spec.md §10 항목 5 에서 "리소스 누출 없음" 만 요구 — 24h TTL 이 마지막 안전망.

### 6.2 Logout / explicit invalidation (eager drop — REQ-BR-AMEND-007, AC-BR-AMEND-008)

시나리오: `auth.CloseSessionsByCookieHash(cookieHash)` (registry.go:161) 호출. 사용자 logout button, admin-driven session revocation, 또는 cookie expiry 이후 force-revoke.

정책 (in-scope, REQ-BR-AMEND-007):
- registry 가 closer 를 invoke 하기 **이전에** dispatcher 가 LogicalID 의 buffer 와 sequence counter 를 즉시 drop 한다.
- 24h TTL 에 위임하지 않는다 — 보안 이벤트는 의도적 invalidation 이므로 buffered outbound 가 살아 있으면 oracle 위험.
- 구현 hook 후보: `Registry.CloseSessionsByCookieHash` 가 dispatcher 에 logout notification 을 전달 (cleanest: registry 가 `dispatcher.dropLogicalBuffer(LogicalID)` 호출 후 closer invoke). 본 hook 의 정확한 위치는 task M3-T3 또는 새 task M3-T5 의 implementation detail로 다룬다.

### 6.3 두 정책의 invariant 차이

| 시나리오                         | sequence drop 시점          | buffer drop 시점          | 보안 risk     |
| -------------------------------- | --------------------------- | ------------------------- | ------------- |
| Transient disconnect             | lazy (24h TTL)              | lazy (24h TTL)            | low (cookie 재사용 가능) |
| Logout / CloseSessionsByCookieHash | eager (registry hook 시점) | eager (registry hook 시점) | high → mitigated by eager drop |

이 분리가 본 amendment 의 핵심 결정 중 하나이며, plan-auditor D4 의 응답이다.

## 7. 통합 테스트 시나리오 — 이미 식별된 케이스

부모 v0.2.1 의 `m4_integration_test.go` 와 `followup_integration_test.go` 는 다음을 검증한다 (spec.md §2.2 결손 분석):

- single-tab buffer + same-connID replay → PASS (현재).
- cross-connection replay (다른 connID 로 재접속) → 검증 안 됨 (결손).
- multi-tab → 검증 안 됨 (결손).

본 amendment 가 추가하는 통합 테스트:

- AC-BR-AMEND-003: 같은 cookie+WS 로 새 connID 재접속 → 이전 connID 의 buffer 회수.
- AC-BR-AMEND-004: partial Last-Event-ID/X-Last-Sequence 로 부분 replay.
- AC-BR-AMEND-005: 두 active connID 의 buffer 공유, sender 분리 emit, 세 번째 reconnect 가 두 outbound 모두 회수.
- AC-BR-AMEND-006: race 조건에서 sequence 단조성.

## 8. 부모 spec 과의 회귀 보호

부모 v0.2.1 의 16 AC 가 본 amendment 후에도 동일하게 통과해야 한다 (HARD). 위험 요소:

- **AC-BR-009 (offline buffering + replay)**: 같은 connID 시나리오만 검증 — LogicalID 도입 후에도 같은 connID 의 LogicalID 가 동일하므로 영향 0.
- **AC-BR-015 (session resume)**: 같은 sessionID 검증 — registry.LogicalID lookup hit 후 buffer.Replay(LogicalID, lastSeq) 가 같은 결과 반환 (단일 connID 시나리오에서 LogicalID 의 buffer 가 connID 의 buffer 와 동일).
- **AC-BR-010 (flush-gate)**: gate 는 sender 단위 (transport) 이므로 LogicalID 도입과 무관. 부모 그대로.
- **AC-BR-018 (reconnect rate-limit)**: cookie 단위 rate-limit, LogicalID 와 무관. 영향 0.

각 AC 의 통합 테스트를 그대로 보존 + 신규 통합 테스트 (multi-tab + cross-conn) 를 추가하는 패턴.

## 9. 외부 의존성 영향

- 신규 외부 의존성: 0건. `crypto/hmac`, `crypto/sha256`, `encoding/base64`, `encoding/binary` (uvarint) 모두 표준.
- 기존 의존성 변경: 없음.
- HMAC secret: 부모 v0.2.1 의 cookie HMAC 비밀키를 재사용. `auth.go` 의 secret 참조 패턴 그대로.

## 10. Open Questions (사용자 검토 / 향후 결정)

1. **Multi-tab live broadcast 가 정말 필요한가?** — spec.md §5.4 OQ 1. 본 amendment 는 거부 입장. 요구 사항이 있으면 별도 SPEC.

### 10.1 v0.1.1 audit Iteration 1 에서 해소된 questions (참고용)

다음 항목은 plan-auditor 1차 audit 의 권고에 따라 더 이상 open 이 아니다:

- ~~**dispatcher.dropSequence 의 LogicalID GC 정책 (eager vs lazy)**~~ — D4 와 통합 해소. transient disconnect = lazy (research §6.1, spec §10 item 5), logout = eager (REQ-BR-AMEND-007, research §6.2). M3-T3 또는 M3-T5 implementation detail.
- ~~**LogicalID input concatenation 형식 (구 OQ2)**~~ — `uvarint(len(cookieHash)) || cookieHash || transport` 로 결정. spec.md §3.1, research §3.1, §3.2. M1-T1 contract.
- ~~**logical-id 비밀키 분리?**~~ — D5 fix 로 해소. cookie HMAC secret 재사용 + 도메인 분리 prefix `"bridge-logical-id-v1\x00"` (NIST SP 800-108 키 분리). 별도 secret / KMS / rotation 은 spec.md §10 item 4 에 deferred.

## 11. 참고

- 부모 SPEC: `.moai/specs/SPEC-GOOSE-BRIDGE-001/spec.md` (v0.2.1)
- 부모 progress: `.moai/specs/SPEC-GOOSE-BRIDGE-001/progress.md` Out-of-scope 절
- amendment 패턴 reference: `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001/`
- 관련 코드: `internal/bridge/{buffer.go, resume.go, outbound.go, ws.go, sse.go, registry.go, types.go}`
- handoff 메모리: `~/.claude/projects/-Users-goos-MoAI-AI-Goose/memory/handoff_session_2026-05-04_bridge_m0_m3.md` (BRIDGE M0~M3 hand-off)
