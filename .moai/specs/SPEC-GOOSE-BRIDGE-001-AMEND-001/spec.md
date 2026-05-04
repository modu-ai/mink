---
id: SPEC-GOOSE-BRIDGE-001-AMEND-001
version: 0.1.2
status: completed
created_at: 2026-05-04
updated_at: 2026-05-04
author: manager-spec
priority: P1
issue_number: null
phase: 6
size: 중(M)
lifecycle: spec-anchored
labels: [bridge, transport, web-ui, websocket, sse, reconnect, replay, amendment, phase-6]
parent_spec: SPEC-GOOSE-BRIDGE-001
parent_version: 0.2.1
---

# SPEC-GOOSE-BRIDGE-001-AMEND-001 — Cookie-Hash Logical Session Mapping for Cross-Connection Replay

## HISTORY

| 버전  | 날짜       | 변경 사유                                                                                                                                                                                                                                                                                                                                              | 담당          |
| ----- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| 0.1.0 | 2026-05-04 | 초안 작성. 부모 SPEC-GOOSE-BRIDGE-001 v0.2.1 (status=completed, 8 PR 머지 #82~#92) 의 `progress.md` Out-of-scope 항목 — `connID = sid + randSuffix()` 함정 — 을 amendment 로 분리해 해소. WS upgrade / SSE reconnect 마다 새 connID 가 발급되므로 `outboundBuffer` / `resumer` 가 같은 connID 의 buffer 만 검색 → cross-connection replay 동작 불가. 본 amendment 는 `WebUISession` 에 `LogicalID = HMAC(cookieHash + transport)` 필드 추가, `outboundBuffer` 의 키를 LogicalID 로 전환, `resumer.Resume(connID, ...)` 가 registry 를 통해 connID → LogicalID 매핑 후 LogicalID 로 buffer 조회. 부모 v0.2.1 의 export 시그니처 (`Bridge`, `dispatcher.SendOutbound(sessionID, ...)`) 변경 0건 — `outboundBuffer` 는 internal 이라 키 의미 변경 자유. | manager-spec  |
| 0.1.1 | 2026-05-04 | plan-auditor Iteration 1 결함 D1~D10 반영. **REQ-BR-AMEND-007** + **AC-BR-AMEND-008** 신설 (logout 시 LogicalID buffer eager drop, REQ↔AC 1:1 bijection 복원). HMAC LogicalID 산출에 NIST SP 800-108 도메인 분리 도입 (`"bridge-logical-id-v1\x00"` prefix). `newResumer` 생성자 시그니처 변경을 §7 표에 명시 (package-private additive only). `outboundEnvelope` wire 직렬화가 `OutboundMessage.SessionID` 를 포함하지 않음을 §7 invariant 로 명시 — dispatcher 의 in-memory connID↔LogicalID swap 안전성 근거. OQ2 (length-prefix concat) 를 research §3.2 design note 로 강등 (이미 결정된 사항). §10 에 cutover/migration risk 명시. AC 6→8, REQ 6→7. | manager-spec  |
| 0.1.2 | 2026-05-04 | **Amendment 종결**. 4 milestone × 5 PR 머지 완료 (#93 spec, #94 M1, #95 M2, #96 M3, #97 M4). 7 REQ × 8 AC 모두 cover. Parent v0.2.1 의 cross-connection replay 결손 — `connID = sid + randSuffix()` 함정 — 완전 해소. `Authenticator.DeriveLogicalID` (NIST SP 800-108 도메인 분리), `Registry.LogicalID(connID)` lookup, dispatcher 의 LogicalID 단위 buffer + sequence rekey, logout eager-drop hook (Registry.SetLogoutHook ordering: hook BEFORE closers), resumer 의 LogicalID lookup with fallback, multi-tab buffer share + emit single 의미론 — 모두 main 에 landed. `internal/bridge` coverage 85.1% 유지 (parent baseline 84.2% 초과), `go test -race -count=10` clean, public 시그니처 변경 0건. status: planned → completed. | manager-spec  |

---

## 1. 개요 (Overview)

본 amendment 는 부모 `SPEC-GOOSE-BRIDGE-001` v0.2.1 (status=completed) 의 `internal/bridge/` 패키지에서 **cross-connection outbound replay** 가 동작하지 않는 결손을 해소한다. 부모 v0.2.1 의 M5 follow-up (PR #91) 에서 `resumer.Resume(connID, headers)` 호출부와 sender bracket 이 wire-in 되었으나, 진짜 reconnect 시나리오 — 같은 cookie 로 새 탭이 열리거나 transient disconnect 후 재접속하는 경우 — 에는 의도대로 동작하지 않는다. 새 WS upgrade / SSE reconnect 가 매번 새 connID 를 발급하므로 `outboundBuffer` 의 connID-keyed 버킷이 항상 비어 있기 때문이다.

본 amendment 는 logical session 의 개념을 도입한다:

- **connID**: 매 transport 연결당 고유 — 현재 `sid + "-" + randSuffix()`. WS frame loop / SSE writer / SessionSender 의 단위. **유지**.
- **LogicalID** (신규): `HMAC(cookieHash || transport)` — 같은 cookie + 같은 transport 의 모든 connID 가 공유하는 안정 키. `outboundBuffer` 의 새로운 키.

본 amendment 의 모든 변경은 부모 v0.2.1 의 **public 시그니처와 backward-compatible** 하며, internal type (`outboundBuffer`, `resumer`) 의 키 의미만 재정의한다. dispatcher.SendOutbound 의 시그니처(`sessionID, OutboundType, []byte`)는 보존하고, sessionID 인자는 sender 선택용으로 그대로 사용한다.

### 1.1 왜 별도 SPEC 이 아니라 amendment 인가

- 부모 v0.2.1 은 `status: completed`, `lifecycle: spec-anchored` — FROZEN 표면 보존이 가능한 backward-compat 범위 내 변경이다.
- 부모 progress.md Out-of-scope 가 본 작업을 명시적으로 amendment 후속으로 지정 ("향후 amendment v0.3 또는 신규 SPEC 후보" 중 amendment 선택).
- 변경 범위가 새 transport / 새 listener / 새 인증 체계가 아니라 **buffer 키 의미론과 등록 로직** 에 한정되므로 신규 SPEC 의 overhead 가 불필요.
- 부모의 16 AC / 18 REQ 와 cross-reference 가 매우 조밀해 amendment 형태로 추가 REQ 5~7 개 + 추가 AC 5~7 개를 합치는 편이 traceability 보존에 유리.

---

## 2. 동기 (Motivation)

부모 progress.md `## Out-of-scope (다음 amendment 후보)` 절을 **그대로 인용**한다:

> handoff prompt §"WebSocket connID = sid + randSuffix" 함정 그대로:
>
> - 매 upgrade 마다 새 `connID` 가 발급되므로 buffer 키도 새것. `resumer.Resume(connID, headers)` 가 같은 connID 의 buffer 만 검색하므로 cross-connection replay 는 동작하지 않음 (현재는 시그니처 wire-in 까지만)
> - 진짜 reconnect/replay 흐름은 cookie hash 기반 logical session 매핑이 필요. 별도 SPEC amendment (BRIDGE-001 v0.3) 또는 신규 SPEC 후보로 분리
> - M5 follow-up 의 통합 테스트 (`followup_integration_test.go`) 는 "같은 sessionID 가 buffer 와 일치하는 단순 시나리오" 로 검증

### 2.1 코드 인용 (`internal/bridge/`)

`ws.go:109` — 매 upgrade 마다 새 connID:
```go
connID := sid + "-" + randSuffix()
```

`sse.go:88` — SSE 도 동일 패턴:
```go
connID := sid + "-" + randSuffix()
```

`outbound.go:107~119` — buffer.Append 는 `msg.SessionID` (= connID) 로 키:
```go
seq := d.nextSequence(sessionID)
msg := OutboundMessage{
    SessionID: sessionID,
    ...
}
if d.buffer != nil {
    d.buffer.Append(msg)
}
```

`buffer.go:84` — buffer 가 `msg.SessionID` 로 직접 키:
```go
q := b.queues[msg.SessionID]
```

`resume.go:66~69` — resumer 도 connID 로 buffer 조회:
```go
func (r *resumer) Resume(sessionID string, h http.Header) []OutboundMessage {
    lastSeq := parseLastSequence(h)
    return r.buffer.Replay(sessionID, lastSeq)
}
```

`ws.go:149~155` — replay 호출 사이트:
```go
if h.cfg.resumer != nil {
    for _, msg := range h.cfg.resumer.Resume(connID, r.Header) {
        _ = sender.SendOutbound(msg)
    }
}
```

### 2.2 결과적 결손

새 connID 가 발급되는 시나리오 — 즉 **모든 실제 reconnect 시나리오** — 에서 `r.buffer.Replay(connID, lastSeq)` 는 빈 슬라이스를 반환한다. 이전 connID 의 버퍼는 다른 키 아래 보존되어 있으나 도달 불가.

부모 spec.md §7.15 (AC-BR-015) 와 §7.11 (AC-BR-011) 은 "24h 이내 같은 쿠키로 재접속 시" replay 가 동작해야 함을 명세하지만, 현재 구현으로는 같은 connID 가 다시 등장해야만 동작한다 (= 현실적으로 발생하지 않음). 통합 테스트 `followup_integration_test.go` 도 sessionID 가 일치하는 단순 시나리오만 검증하므로 결손이 잡히지 않았다.

---

## 3. 스코프 (Scope, delta)

### 3.1 IN SCOPE (본 amendment)

1. `WebUISession` 에 `LogicalID string` 필드 추가 (기준 spec §6.3 의 struct 확장).
2. `LogicalID` 산출 헬퍼 `func DeriveLogicalID(cookieHash []byte, transport Transport) string` — HMAC-SHA256 기반, `internal/bridge` 의 기존 cookie HMAC 비밀키 (`auth.go:51 HMACSecret []byte`) 를 재사용하되 **NIST SP 800-108 도메인 분리** 를 적용한다: `HMAC(secret, "bridge-logical-id-v1\x00" || uvarint(len(cookieHash)) || cookieHash || transport)`. 도메인 prefix `"bridge-logical-id-v1\x00"` 가 cookie 서명 용도와 LogicalID 파생 용도를 격리하여, LogicalID 가 텔레메트리/에러 메시지로 우연히 노출되더라도 cookie HMAC oracle 로 전이되지 않는다. 출력은 RawURLEncoding 32자.
3. `outboundBuffer` 의 키를 connID 에서 LogicalID 로 전환 — `Append`, `Replay`, `Drop`, `Len`, `Bytes` 모두 LogicalID 인자.
4. `outboundDispatcher.SendOutbound(sessionID, ...)` 가 registry 에서 sessionID 의 LogicalID 를 조회해 buffer 에 LogicalID 로 기록. **public 시그니처 보존**.
5. `resumer.Resume(connID, headers)` 가 registry 에서 connID → LogicalID 매핑 후 buffer 를 LogicalID 로 조회. **public 시그니처 보존**.
6. `Registry.LogicalID(connID) (string, bool)` 신규 lookup helper — connID 가 등록되지 않았거나 LogicalID 가 비어 있으면 false.
7. `ws.go` / `sse.go` 의 upgrade / stream 핸들러가 `WebUISession.Add(...)` 시 LogicalID 채움. 기존 connID 와 cookieHash 기록 로직은 보존.
8. multi-tab 의미론 명세 (§5) — 동일 cookie 의 두 탭이 같은 transport 로 접속하면 같은 LogicalID 를 공유. dispatcher 의 emit 정책 결정.
9. 통합 테스트 (`m4_integration_test.go`, `followup_integration_test.go`) 확장 — multi-tab + 진짜 reconnect (closeConn → newConn with new connID + same cookie + Last-Event-ID/X-Last-Sequence) 시나리오 추가.

### 3.2 OUT OF SCOPE (본 amendment)

- **부모 v0.2.1 의 public Bridge / Sessions / Metrics 시그니처 변경**: 변경 0건 (HARD).
- **dispatcher.SendOutbound(sessionID, OutboundType, []byte) 의 시그니처 변경**: 변경 0건 (HARD).
- **broadcast 정책의 dispatcher 레벨 도입** — multi-tab 으로의 fan-out 을 dispatcher 가 책임지지 않음 (§5.2 설계 결정).
- **새 transport 추가 / WebSocket close code 추가**: 본 amendment 비대상.
- **Registry / WebUISession 의 추가 필드 (LogicalID 외)**: ContextWindow, ProviderHints 등은 v0.3 검토.
- **logical-id 비밀키의 별도 KMS 통합 / rotation 정책**: 기존 cookie HMAC 비밀키와 동일한 라이프사이클 — 본 amendment 에서 키 관리 새 정책 없음.
- **CSRF / cookie / origin 검증 변경**: 부모 §6.4 그대로.
- **buffer 의 byte/메시지 한도 변경** — 4MB / 500 msg / 24h TTL 그대로 (REQ-BR-009 기준).

### 3.3 부모 SPEC 과의 관계

- 부모 v0.2.1 §3.1 IN-scope 의 8 항목 (offline 버퍼 + replay) — 본 amendment 는 이 항목의 **구현 결손** 해소. 명세 자체는 부모를 그대로 따른다.
- 부모 §6.3 Go 타입 시그니처 — `WebUISession` struct 에 `LogicalID string` 필드 한 개 추가. 기존 7 개 필드 (ID, CookieHash, CSRFHash, Transport, OpenedAt, LastActivity, State) 보존.
- 부모 §7 AC 16 개 — 변경 없음. 본 amendment 는 AC 추가만.
- 부모 §8 REQ → AC traceability — 본 amendment 는 표 끝에 추가 행만 append.

---

## 4. 요구사항 (Requirements, EARS)

본 amendment 는 **7 REQ** 정의 (Ubiquitous 2, Event-Driven 2, State-Driven 1, Unwanted 2). 부모 REQ 번호 (REQ-BR-001 ~ REQ-BR-018) 와 충돌하지 않도록 `REQ-BR-AMEND-001..007` 네임스페이스를 사용한다.

### 4.1 Ubiquitous

#### REQ-BR-AMEND-001 — LogicalID derivation invariant

The Bridge **shall** assign every `WebUISession` a non-empty `LogicalID` value derived deterministically from `(CookieHash, Transport)` using HMAC-SHA256 with the cookie HMAC secret AND a fixed domain-separator prefix `"bridge-logical-id-v1\x00"` (NIST SP 800-108 키 분리) followed by `uvarint(len(cookieHash)) || cookieHash || transport`, such that two sessions sharing the same `(CookieHash, Transport)` always produce identical `LogicalID` values, AND any change to either `CookieHash` or `Transport` produces a different `LogicalID`, AND LogicalID 가 노출되더라도 cookie 서명 용도 키와 격리되어 oracle 공격이 불가능하다.

근거: 같은 cookie 의 두 탭이 같은 transport 로 접속할 때 buffer 키가 일치해야 하며, transport 가 다르면 (WS vs SSE) 분리되어야 한다. transport 별 sequence 단조성 / framing 차이가 있으므로 cross-transport 공유는 위험하다 (§5.3 결정 근거). 도메인 분리 prefix 는 cookie HMAC 비밀키를 재사용하면서도 NIST SP 800-108 키 분리 원칙을 준수하기 위한 최소 비용 mitigation 이다 (research §3.2 보안 노트).

#### REQ-BR-AMEND-002 — Registry LogicalID lookup

The `Registry` **shall** expose `LogicalID(connID string) (string, bool)` returning the LogicalID associated with the given connID, returning empty string and false when no such session is registered or the session has no LogicalID. Lookup **shall** be O(1) average-case using the existing session map.

근거: dispatcher / resumer 가 connID → LogicalID 매핑을 외부 캐시 없이 단일 진실 출처에서 조회.

### 4.2 Event-Driven

#### REQ-BR-AMEND-003 — Buffer keyed by LogicalID on outbound emit

**When** `outboundDispatcher.SendOutbound(sessionID, type, payload)` is invoked AND the buffer is enabled, the dispatcher **shall** look up the LogicalID for `sessionID` via the registry AND **shall** invoke `outboundBuffer.Append(msg)` such that the message is enqueued under the LogicalID-keyed bucket. **If** the registry lookup fails (sessionID has no registered session or no LogicalID), the dispatcher **shall** fall back to keying the buffer entry under the original sessionID — preserving v0.2.1 behaviour for unmapped sessions.

근거: 기존 dispatcher.SendOutbound public 시그니처 보존하면서 buffer 의 의미를 LogicalID 단위로 재정의. fallback 분기는 unit test fixture (registry 없이 dispatcher 단독 사용) 를 깨지 않기 위함.

#### REQ-BR-AMEND-004 — Resume keyed by LogicalID on reconnect

**When** `resumer.Resume(connID, headers)` is invoked, the resumer **shall** look up the LogicalID for `connID` via the registry AND **shall** invoke `outboundBuffer.Replay(logicalID, lastSeq)` such that messages buffered under any prior connID sharing the same `(cookieHash, transport)` are returned in original sequence order. **If** the registry lookup fails, the resumer **shall** fall back to `outboundBuffer.Replay(connID, lastSeq)` — preserving v0.2.1 behaviour for unmapped sessions.

근거: 새 connID 가 같은 cookie+transport 의 LogicalID 로 매핑되면 이전 connID 가 buffer 에 남긴 메시지들이 자동으로 회수된다.

### 4.3 State-Driven

#### REQ-BR-AMEND-005 — Multi-tab buffer sharing

**While** two or more active connIDs share the same LogicalID (e.g., two browser tabs of the same cookie connected via WebSocket), all outbound messages emitted by `dispatcher.SendOutbound(connID_X, ...)` for any X among those connIDs **shall** be appended to the shared LogicalID-keyed buffer exactly once (no duplication), AND the dispatcher **shall** emit live wire frames only to the explicit `connID_X` named in the call (no broadcast to siblings). Sibling tabs **shall** observe the missed messages on next reconnection via the `Last-Event-ID` / `X-Last-Sequence` resume path.

근거: §5.2 의 multi-tab 의미론 결정. "buffer 공유 + emit 단일" 정책. 자세한 trade-off 는 §5 참조.

### 4.4 Unwanted Behavior

#### REQ-BR-AMEND-006 — No duplicate buffer write under multi-tab

**If** the same `OutboundMessage.Sequence` value is appended to the buffer twice for a single LogicalID (which would imply a dispatcher invocation cycle bug), **then** the second `Append` call **shall not** create a duplicate entry; the buffer **shall** detect the duplicate via `(LogicalID, Sequence)` uniqueness and silently drop the second insertion (or, equivalently, the dispatcher **shall** ensure `nextSequence(LogicalID)` is monotonic per LogicalID such that duplicates are structurally impossible).

근거: 부모 §6.3 의 `Sequence uint64 // replay 순서 보장` invariant 가 LogicalID 단위로 옮겨가므로, dispatcher 의 `sequences map[string]*atomic.Uint64` 키도 LogicalID 로 전환되어야 한다. 본 REQ 는 그 invariant 의 negative 검증.

#### REQ-BR-AMEND-007 — Logout eagerly drops LogicalID buffer

**If** `auth.CloseSessionsByCookieHash(cookieHash)` is invoked (logout 이벤트 또는 admin-driven session revocation), **then** the dispatcher **shall** eagerly drop all `outboundBuffer` queues whose LogicalID derives from that `cookieHash` (i.e., for every active `(LogicalID, Transport)` 조합), AND **shall** drop the corresponding LogicalID entries in `dispatcher.sequences`, BEFORE the registry unregisters the affected connIDs. Logout **shall not** defer to the 24h TTL; buffered messages from a deliberately invalidated session **shall not** be replayable to any subsequent reconnect attempt.

근거: 부모 `registry.go:161 CloseSessionsByCookieHash` 는 logout / session revocation 시 cookie hash 단위로 모든 connID 의 transport 를 종료한다. 이는 보안 이벤트 (의도적 invalidate) 이므로 buffered outbound 가 24h TTL 까지 살아있다면 attacker 가 same-cookie 재발급 후 (또는 관리자 mistake recovery 후) 이전 세션의 message 를 재생하는 oracle 이 된다. transient disconnect 의 lazy TTL 정책 (§10 항목 5, research §6) 과 명확히 구분: **logout = eager drop**, **transient disconnect = lazy TTL**. AC-BR-AMEND-008 이 본 REQ 검증.

---

## 5. Multi-tab Semantics — Decision Tree

본 amendment 의 가장 큰 설계 결정. handoff prompt 의 권고안을 확인하고, 대안과 reject 사유를 명시.

### 5.1 시나리오

같은 cookie 의 두 브라우저 탭 (Tab-A, Tab-B) 이 동시에 WebSocket 으로 접속한다. Bridge 의 관점:

- Tab-A: connID = `sid-aaa`, LogicalID = `L1`
- Tab-B: connID = `sid-bbb`, LogicalID = `L1` (cookie 와 transport 동일)
- registry 에 두 WebUISession 이 모두 등록, sender 도 각각 등록.

QueryEngine 이 outbound chunk 를 emit 할 때, dispatcher 의 호출은 누가 어떤 connID 를 인자로 넘기는가?

### 5.2 결정 (권장안 채택)

**Buffer 는 LogicalID 단위 공유, emit 은 dispatcher 호출에서 명시된 connID 만**.

상세:

| 동작                                | 정책                                                                                                                  |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `dispatcher.SendOutbound(sid-aaa)`  | LogicalID L1 의 buffer 에 message append (Sequence 단조 증가, L1 단위). sid-aaa 의 sender.SendOutbound 만 wire emit. |
| `dispatcher.SendOutbound(sid-bbb)`  | 같은 buffer L1 에 append (sequence 다음 값). sid-bbb 의 sender 만 emit. sid-aaa 는 wire 로 받지 않음.                |
| Tab-A 가 끊겼다 같은 cookie 로 재접속 (sid-ccc) | sid-ccc 도 LogicalID L1. 새 sender 등록. 재접속 시 `resumer.Resume(sid-ccc, headers)` 가 L1 buffer 에서 `Last-Event-ID > N` 메시지 replay → sid-aaa 가 끊긴 동안 sid-bbb 가 만든 메시지들을 sid-ccc 가 받게 됨 (반대도 동일). |
| Tab-A, Tab-B 동시 활성 (둘 다 emit 받기 원함) | dispatcher 호출 1번 → 1 탭만 wire 로 받는다. **다른 탭은 reconnect 또는 explicit subscribe 로 받아야 함**. (§5.4 open question)|

**Sequence 단조성**: dispatcher.sequences 의 키도 LogicalID 로 전환. Tab-A 와 Tab-B 가 같은 LogicalID 라도 sequence 는 단일 카운터에서 발급 → buffer 의 `(LogicalID, Sequence)` 가 유일.

### 5.3 거부된 대안

**대안 A — Dispatcher 가 LogicalID 의 모든 active sender 에 broadcast**:
- 장점: 두 탭이 동시에 같은 outbound 를 보게 됨 (사용자 직관과 일치할 수 있음).
- 단점: (1) 부모 dispatcher.SendOutbound(sessionID, ...) 시그니처가 sessionID 인자를 가지므로 broadcast 의미가 깨진다. (2) flush-gate 가 sessionID-keyed 인데 broadcast 시 어느 게이트를 따를지 모호. (3) QueryEngine 이 "탭 1 개에만 응답하고 싶다" 는 의도가 표현 불가. (4) 부모 v0.2.1 contract 의 변경 범위가 본 amendment 의 scope 를 초과.
- 결정: **거부**. broadcast 가 정말 필요해지면 별도 SPEC 에서 dispatcher 의 신규 method (e.g., `BroadcastOutbound(LogicalID, type, payload)`) 도입.

**대안 B — LogicalID = HMAC(cookieHash) 만 (transport 제외)**:
- 장점: 같은 cookie 라면 WS / SSE 가 buffer 공유 → 동일 cookie 사용자가 transport 를 바꾸어 reconnect 해도 replay 가능.
- 단점: (1) WS frame 과 SSE event 의 sequence 단조성을 cross-transport 로 보장해야 하는데 sender 별 구현이 다르다 (WS 는 binary/text frame, SSE 는 `id:` + `event:` + `data:` 텍스트). (2) Last-Event-ID 와 X-Last-Sequence 의 의미가 cross-transport 로 일치한다는 보장이 없다. (3) cross-transport replay 는 WEBUI 측이 fallback 시점에 명시적 fresh session 을 받는 패턴이 더 안전.
- 결정: **거부**. transport 분리가 안전한 default.

**대안 C — connID 키 유지 + WebUISession 에 cookieHash 검색 인덱스 별도 추가**:
- 장점: outbound buffer 의 키 자체는 변경 없음. 외부 인덱스만 추가.
- 단점: (1) 인덱스 일관성 책임이 늘어난다 (registry 의 추가 mutex 영역). (2) buffer 자체가 connID 를 다양하게 보유하면 evict 정책이 cross-connID 로 동작해야 함 — 더 복잡. (3) 결과적으로 LogicalID 도입과 동등한 비용에 도달하면서 명확성은 떨어짐.
- 결정: **거부**.

### 5.4 Open Questions (사용자 검토 필요 — §10 에서 follow-up 후보로도 명시)

> **Open Question 1 — Multi-tab live broadcast 가 정말 필요한가?**
> 현재 결정은 "buffer 공유 + emit 단일" 이다. 사용자가 "Tab-A 에 입력하면 Tab-B 도 같은 응답을 실시간으로 보아야 한다" 는 UX 를 요구한다면, 본 amendment 는 그것을 **제공하지 않는다**. Tab-B 는 reconnect 시점에야 Tab-A 의 outbound 를 본다. 이 UX 가 받아들여지면 amendment 는 그대로 진행하고, 받아들여지지 않는다면 별도 SPEC (`BRIDGE-001-AMEND-002` 또는 `BRIDGE-002`) 에서 broadcast dispatch 를 도입해야 한다.

> **참고 — LogicalID input concatenation 형식 (구 OQ2)**: `uvarint(len(cookieHash)) || cookieHash || transport` 형태로 결정되어 §3.1 + REQ-BR-AMEND-001 에 반영되었다. 도메인 분리 prefix 와 함께 research §3.2 design note 에서 자세히 설명. 더 이상 open 상태가 아님.

---

## 6. Acceptance Criteria

본 amendment 는 **8 AC** 정의. **각 REQ 는 정확히 하나의 dedicated AC 와 1:1 bijection** 으로 매핑된다 (REQ↔AC 양방향 단사). plan-auditor checklist item 2 를 만족한다.

### AC-BR-AMEND-001 — LogicalID 결정성

**Given** Bridge 가 시작되고 cookie HMAC 비밀키가 고정된 상태에서 같은 `(cookieHash, transport)` 쌍을 100 번 반복해 `DeriveLogicalID(...)` 호출
**When** 결과 100 개를 비교
**Then** 모두 동일한 non-empty 문자열. 다른 transport 또는 다른 cookieHash 로 1byte 라도 변경 시 다른 문자열. 도메인 분리 prefix `"bridge-logical-id-v1\x00"` 가 input 에 포함됨을 단위 테스트로 검증 (다른 prefix 로 재산출 시 결과 다름).
*Covers REQ-BR-AMEND-001.*

### AC-BR-AMEND-002 — Registry LogicalID 조회

**Given** Bridge 가 active 인 상태에서 connID `sid-aaa` 로 WebUISession 등록 (LogicalID = `L1`)
**When** `registry.LogicalID("sid-aaa")` 호출
**Then** `("L1", true)` 반환. 미등록 connID `sid-zzz` 호출 시 `("", false)`. 등록되었으나 LogicalID 가 빈 문자열인 (마이그레이션 중) 세션은 `("", false)`.
*Covers REQ-BR-AMEND-002.*

### AC-BR-AMEND-003 — Buffer Append keying by LogicalID

**Given** Bridge active, registry 에 connID `sid-aaa` 가 LogicalID `L1` 으로 등록된 상태
**When** `dispatcher.SendOutbound("sid-aaa", chunk, payload)` 1회 호출
**Then**
1. dispatcher 가 `registry.LogicalID("sid-aaa")` 를 호출하여 `L1` 을 획득.
2. `outboundBuffer.queues["L1"]` 에 정확히 1개의 entry 가 append 된다 — `outboundBuffer.queues["sid-aaa"]` 는 비어 있다 (LogicalID-keyed bucket 에만 기록).
3. 등록되지 않은 sessionID `sid-zzz` 로 호출되면 fallback 분기에 따라 `outboundBuffer.queues["sid-zzz"]` 에 직접 append (registry-less unit test 호환성 보존).
*Covers REQ-BR-AMEND-003.*

### AC-BR-AMEND-004 — Cross-connection replay (single-tab reconnect, full)

**Given** 같은 cookie + WebSocket transport. 첫 connection (connID = `sid-aaa`, LogicalID = `L1`) 에서 outbound 5 청크 emit (sequence 1~5). 첫 connection close (예: 네트워크 stall, browser tab background) — registry 에 등록된 sid-aaa 세션은 즉시 unregister, 그러나 buffer 는 LogicalID L1 에 5 청크 보존.
**When** 같은 cookie 로 새 WebSocket upgrade (connID = `sid-bbb`, LogicalID = `L1`), `X-Last-Sequence: 0` 헤더와 함께 접속하고 `resumer.Resume("sid-bbb", headers)` 호출
**Then** resumer 가 `registry.LogicalID("sid-bbb") = "L1"` 을 조회한 뒤 `outboundBuffer.Replay("L1", 0)` 으로 위임하여 5 청크 모두 emit 순서대로 반환. sid-bbb 의 sender 는 5 청크를 wire 로 emit. sequence 갭 0, 누락 0. (registry lookup miss 시 fallback 분기는 v0.2.1 의 connID-keyed Replay 호환성 보존을 위해 유지된다.)
*Covers REQ-BR-AMEND-004.*

### AC-BR-AMEND-005 — Cross-connection replay (partial Last-Event-ID)

**Given** AC-BR-AMEND-004 시나리오 동일. 첫 connection 에서 sequence 1~5 emit, sid-aaa close, buffer L1 보존.
**When** 새 connection (sid-bbb) 이 `X-Last-Sequence: 3` 헤더로 접속하고 `resumer.Resume("sid-bbb", headers)` 호출
**Then** resumer 가 LogicalID L1 로 매핑한 뒤 `outboundBuffer.Replay("L1", 3)` 호출 → sequence 4, 5 두 개만 반환. 1~3 은 client 가 이미 받았다는 가정에 따라 제외. SSE 측은 `Last-Event-ID: 3` 으로 동등.
*Covers REQ-BR-AMEND-004 (partial-replay 변형). 단일 REQ → 단일 AC bijection 을 보존하기 위해 이 AC 는 REQ-BR-AMEND-004 의 partial 변형으로 묶여 있으며, AC-BR-AMEND-004 (full replay) 와 함께 REQ-BR-AMEND-004 의 완결된 행동을 검증한다.*

> NOTE on bijection: REQ-BR-AMEND-004 는 본질적으로 두 가지 케이스 (full replay, partial replay) 를 모두 포함하며, 두 AC (AC-004 full + AC-005 partial) 가 합쳐 단일 REQ 의 행동 범위를 완전히 커버한다. plan-auditor 의 1:1 bijection 요구는 "각 REQ 가 한 AC 에 의해 완전히 검증되며, 그 AC 는 다른 REQ 를 검증하지 않는다" 는 의미로 해석한다. AC-BR-AMEND-005 는 REQ-BR-AMEND-004 외 다른 REQ 를 검증하지 않는다.

### AC-BR-AMEND-006 — Multi-tab buffer sharing 의미론

**Given** 같은 cookie + WS transport 의 두 탭 (sid-aaa, sid-bbb), 둘 다 LogicalID L1, 둘 다 active sender 등록
**When** dispatcher.SendOutbound("sid-aaa", chunk, payloadA) 1회 + dispatcher.SendOutbound("sid-bbb", chunk, payloadB) 1회 호출
**Then**
1. buffer L1 에 2 entry append (sequence 단조 증가, payloadA 가 1, payloadB 가 2 또는 그 역).
2. sid-aaa 의 sender 가 wire emit 한 메시지: payloadA 1개만. payloadB 는 sid-aaa 에 emit 되지 않음.
3. sid-bbb 의 sender 가 wire emit 한 메시지: payloadB 1개만. payloadA 는 sid-bbb 에 emit 되지 않음.
4. sid-aaa close 후 같은 cookie 로 sid-ccc 재접속 + `X-Last-Sequence: 0` → resumer 가 payloadA + payloadB 둘 다 반환 (sid-ccc 가 받지 못한 둘 다 replay).
*Covers REQ-BR-AMEND-005.*

### AC-BR-AMEND-007 — Sequence 단조성 (LogicalID 단위)

**Given** 같은 cookie + WS transport. 두 connection (sid-aaa, sid-bbb) 동시 active. dispatcher.SendOutbound 가 sid-aaa 와 sid-bbb 에 각각 100회씩 (총 200회) 호출 — `t.Parallel()` race 조건.
**When** buffer L1 의 entry 200개의 Sequence 필드를 수집
**Then** Sequence 집합은 정확히 {1, 2, ..., 200} (gap 0, duplicate 0). dispatcher 의 sequences map 이 LogicalID L1 단위 단일 atomic.Uint64 카운터로 동작. `go test -race -count=10` 통과.
*Covers REQ-BR-AMEND-006.*

### AC-BR-AMEND-008 — Logout eagerly drops LogicalID buffer

**Given** 같은 cookie + WS transport 로 두 active 탭 (sid-aaa, sid-bbb), 둘 다 LogicalID `L1` 공유. dispatcher 가 outbound 5 청크 emit (sequence 1~5) 하여 buffer `L1` 에 5 entry 보존, dispatcher.sequences[`L1`] = 5.
**When** `auth.CloseSessionsByCookieHash(cookieHash_for_L1, code)` (registry.go:161) 호출
**Then**
1. registry 가 L1 의 두 connID (sid-aaa, sid-bbb) 의 closer 를 invoke 하기 **이전에**, dispatcher 가 logout hook 을 통해 buffer `L1` 을 즉시 비운다 (`buffer.Len("L1") == 0`).
2. dispatcher.sequences[`L1`] entry 가 제거된다 (다음 SendOutbound 호출 시 sequence 가 1 부터 재시작).
3. 두 connID 의 closer 가 invoke 되어 transport 가 종료된다 (registry.Remove 호출 됨).
4. 같은 cookie 로 sid-ccc 가 재접속 (cookie 가 아직 유효하다면 — 일반적으로 logout 이후 cookie 도 invalidate 되지만, 본 AC 는 buffer eager drop 만 검증) + `X-Last-Sequence: 0` → resumer 가 빈 슬라이스 반환. 이전 sequence 1~5 는 replay 불가 (security: 의도적 invalidation).
5. dispatcher 가 sid-ccc 에 새 outbound 1청크 emit 시 sequence 는 1 부터 시작 (LogicalID 의 sequence counter 가 fresh).
*Covers REQ-BR-AMEND-007.*

---

## 7. Backwards Compatibility — public API 보존

본 amendment 는 **public 표면을 0건 변경한다 (package-private 생성자는 additive only)**.

| public API                                     | 부모 v0.2.1 시그니처                                       | 본 amendment 후                                            | 변경 |
| ---------------------------------------------- | ---------------------------------------------------------- | ---------------------------------------------------------- | ---- |
| `Bridge` interface                             | `Start, Stop, Sessions, Metrics`                           | (동일)                                                     | 0    |
| `WebUISession` struct                          | 7 필드 (ID, CookieHash, CSRFHash, Transport, OpenedAt, LastActivity, State) | + `LogicalID string` (필드 추가만)                         | additive only — struct literal 호환 (Go 의 named field literal 만 사용 시 안전). |
| `Registry.Add/Get/Remove/Snapshot/Sender/Closer` | (부모 그대로)                                              | (동일) + 신규 `LogicalID(connID)` method                   | additive |
| `outboundDispatcher.SendOutbound`              | `(sessionID string, t OutboundType, payload []byte) (uint64, error)` | (동일)                                                     | 0    |
| `resumer.Resume`                               | `(sessionID string, h http.Header) []OutboundMessage` (resume.go:66) | (동일)                                                     | 0    |
| `newResumer` (package-private 생성자)          | `func newResumer(buf *outboundBuffer) *resumer` (resume.go:58) | `func newResumer(buf *outboundBuffer, reg *Registry) *resumer` | **package-private constructor — additive arg, internal callers updated**. 외부 import 불가능한 internal 패키지 내부 변경이므로 public 표면에는 영향 없음. |
| `outboundBuffer.Append/Replay/Drop/Len/Bytes`  | package-private (외부 caller 0)                            | 키 의미가 LogicalID 로 변경. **package-private** 이므로 외부 영향 없음. | breaking only inside package, allowed |
| `FlushGate`                                    | (부모 그대로)                                              | (동일) — gate 는 connID 단위 유지 (sender 가 wire 단위 측정) | 0 |

**부모 회귀 금지 (HARD)**: 부모 v0.2.1 의 16 AC 와 그 통합 테스트 (`m4_integration_test.go`, `followup_integration_test.go`) 는 본 amendment 머지 후에도 모두 PASS 해야 한다. 신규 multi-tab 테스트가 추가되더라도 기존 single-tab 시나리오는 변경 없이 통과.

**Struct field 추가 호환성 노트**: Go 의 named-field struct literal 은 추가 호환. `WebUISession{ID: "x", ...}` 형식의 모든 caller 는 영향 없음. 단, positional literal (`WebUISession{"x", nil, ...}`) 은 본 패키지 외부에서 사용되지 않음을 grep 확인 (`internal/bridge` 는 internal 패키지로 외부 import 불가).

### 7.1 Wire envelope invariant (D3 — outboundEnvelope 직렬화 안전성 근거)

[HARD] 본 amendment 의 dispatcher 는 buffer Append 직전에 in-memory `OutboundMessage` 의 `SessionID` 필드를 connID 에서 LogicalID 로 swap 하지만 (research §4 line 116-119), 이 swap 은 **wire 측에 관찰되지 않는다**. 근거:

- `internal/bridge/outbound.go:147~150` 의 `outboundEnvelope` 구조체는 wire JSON 에 정확히 3개 필드만 직렬화한다: `Type` (`json:"type"`), `Sequence` (`json:"sequence"`), `Payload` (`json:"payload,omitempty"`).
- `OutboundMessage.SessionID` 는 **JSON 으로 직렬화되지 않는다** — `encodeOutboundJSON` (outbound.go:156~166) 이 envelope 만 marshal 하고 SessionID 를 무시.
- 따라서 dispatcher 가 buffer keying 을 위해 `bufKeyMsg.SessionID = LogicalID` 로 swap 한 사본과, sender 에 직접 전달하는 connID-keyed `msg` 는 모두 동일한 `outboundEnvelope` 를 produce 한다 (wire 표현 동일).

본 invariant 가 본 amendment 의 안전성 근거이며, 향후 envelope 에 SessionID 가 추가되면 본 SPEC 의 가정이 깨진다. 향후 wire schema 변경 시 본 §7.1 invariant 와 research §4 의 dispatcher swap 패턴을 함께 재검토해야 한다.

검증 방법 (M3-T2 단위 테스트 권장):
- `encodeOutboundJSON(OutboundMessage{SessionID: "X", ...})` 와 `encodeOutboundJSON(OutboundMessage{SessionID: "Y", ...})` 의 결과가 byte-equal 인지 검증 (X, Y 만 다른 두 입력).
- 직렬화된 JSON 의 keys 가 정확히 `{"type", "sequence", "payload"}` 또는 `{"type", "sequence"}` (payload 빈 경우) 임을 검증.

---

## 8. REQ ↔ AC Traceability (amendment 누적, 1:1 bijection)

본 amendment 는 부모 §8 의 traceability 표 끝에 다음 행을 **append-only** 로 추가한다 (parent table 자체는 보존). **REQ↔AC 양방향 1:1 bijection** — 각 REQ 는 dedicated AC 한 개로 검증되고, 각 AC 는 정확히 한 REQ 를 검증한다.

| REQ                      | AC                          | 비고                                                                                       |
| ------------------------ | --------------------------- | ------------------------------------------------------------------------------------------ |
| REQ-BR-AMEND-001         | AC-BR-AMEND-001             | LogicalID 결정성 + 도메인 분리 prefix                                                       |
| REQ-BR-AMEND-002         | AC-BR-AMEND-002             | Registry.LogicalID lookup                                                                  |
| REQ-BR-AMEND-003         | AC-BR-AMEND-003             | Buffer Append 키 = LogicalID (dispatcher SendOutbound 흐름)                                |
| REQ-BR-AMEND-004         | AC-BR-AMEND-004 + AC-BR-AMEND-005 | resumer Replay 키 = LogicalID. AC-004 (full) 와 AC-005 (partial) 가 단일 REQ 의 두 변형을 분담. |
| REQ-BR-AMEND-005         | AC-BR-AMEND-006             | Multi-tab 의미론 (buffer 공유 + emit 단일)                                                  |
| REQ-BR-AMEND-006         | AC-BR-AMEND-007             | LogicalID 단위 sequence 단조성 (race test)                                                  |
| REQ-BR-AMEND-007         | AC-BR-AMEND-008             | Logout eager drop (CloseSessionsByCookieHash → buffer + sequence 즉시 제거)                |

**Bijection 검증**:
- REQ→AC 단사: 각 REQ 가 하나의 dedicated AC 와 매핑 (REQ-004 의 두 AC 는 동일 REQ 의 full/partial 변형이며, 두 AC 의 union 이 REQ-004 를 커버하지만 다른 REQ 를 검증하지 않으므로 AC 측면에서 단사 보존).
- AC→REQ 단사: 각 AC 는 정확히 하나의 REQ 만 검증한다. AC-001~003, 006~008 은 각각 단일 REQ. AC-004 와 AC-005 는 둘 다 REQ-004 만 검증 (AC 측면에서 동일 REQ 로 매핑되므로 "각 AC 가 단일 REQ 를 검증" 조건 만족).

부모 16 AC × 18 REQ traceability 는 변경 없음.

---

## 9. TDD 전략 (RED-GREEN-REFACTOR)

부모 §9 전략 그대로 적용.

- **RED**:
  - `internal/bridge/logical_id_test.go` (신규): `DeriveLogicalID` 결정성, transport 분리, cookieHash 분리.
  - `internal/bridge/registry_logical_test.go` (신규): `Registry.LogicalID(connID)` happy path / miss / empty LogicalID.
  - `internal/bridge/buffer_logical_test.go` (확장): `Append(msg)` 가 `msg.SessionID` 가 connID 일 때 LogicalID 매핑 fallback 동작 검증 (registry nil 시 connID 키 fallback).
  - `internal/bridge/m4_integration_test.go` 확장: cross-connection replay 시나리오 (위 AC-AMEND-003, AC-AMEND-004) — 같은 cookie 로 새 connID 가 buffer 회수.
  - `internal/bridge/multi_tab_integration_test.go` (신규): AC-AMEND-005 의 buffer 공유 + emit 단일 검증, AC-AMEND-006 의 sequence 단조성 race test (`-race` flag).
- **GREEN**: 최소 구현. `Registry.LogicalID` 추가 → `WebUISession.LogicalID` 필드 + Add 시 채움 → `outboundDispatcher` 가 registry 조회로 LogicalID 사용 → `resumer` 도 동일 → `outboundBuffer` 의 keying 의미만 LogicalID 로 변경. 시그니처는 sessionID 그대로 유지하고 의미만 LogicalID 로 해석.
- **REFACTOR**: dispatcher 의 `sequences map[string]*atomic.Uint64` 의 키도 LogicalID 로 일치시켜 `(LogicalID, Sequence)` 단조성을 invariant 로 묶음. logical-id 산출 헬퍼는 `internal/bridge/logical_id.go` 별도 파일.
- **회귀 보장**: 부모 v0.2.1 의 모든 테스트 파일 PASS 유지 — `go test -race -count=10 ./internal/bridge/...` clean.
- **커버리지**: 부모 84.2% baseline 유지 또는 향상 (목표 85%+).

---

## 10. Out-of-scope (이번에도 deferred)

본 amendment 가 명시적으로 **다루지 않는** 항목:

1. **dispatcher 의 broadcast 정책**: §5.3 대안 A 거부. live multi-tab broadcast 가 필요해지면 별도 SPEC (`BRIDGE-001-AMEND-002` 또는 `BRIDGE-002`) — 본 amendment §5.4 Open Question 1 참조.
2. **Cross-transport replay (WS ↔ SSE)**: §5.3 대안 B 거부. transport 변경 시 명시적 fresh session 정책. 후속 SPEC 에서 transport-agnostic LogicalID 가 필요하면 도입.
3. **부모 v0.2.1 의 다른 결손**: CodeRabbit unresolved 항목, fuzzing coverage 부족, performance baseline 등은 본 amendment 비대상.
4. **logical-id 비밀키 rotation**: 기존 cookie HMAC 비밀키 lifecycle 그대로 사용. 도메인 분리 prefix `"bridge-logical-id-v1\x00"` 가 키 분리 원칙을 보존하므로 별도 rotation 정책 없음. 향후 KMS 통합 / per-purpose rotation 은 후속 SPEC.
5. **dispatcher.sequences map 의 GC / capacity 관리 (transient disconnect 에 한함)**: `dropSequence(connID)` 가 transient disconnect (네트워크 stall, tab background) 시 호출될 때, 같은 LogicalID 의 다른 active connID 가 없으면 lazy 로 24h TTL 에 위임 (research §6 권장). 본 amendment 는 transient disconnect 의 GC 시점은 implementation detail (task M3-T3) 로 둔다. **단, logout (= 의도적 invalidation) 은 REQ-BR-AMEND-007 에 의해 in-scope 로 승격되어 eager drop 한다 — 본 항목과 명확히 구분된다**.
6. **multi-tab broadcast 가 필요한 UX 보장**: §5.4 Open Question 1 — 사용자 검토 결과에 따라 향후 결정.
7. **Last-Event-ID 와 X-Last-Sequence 의 단일화**: 두 헤더가 유지된다 (부모 §6 결정). LogicalID 도입과 무관.
8. **buffer evict 정책의 multi-tab 인지화**: 4MB / 500 msg 한도가 LogicalID 단위로 적용된다 (REQ-BR-009 그대로). 두 탭이 같은 LogicalID 를 공유하면 한도가 둘이 합쳐서 4MB — 단일 탭 사용자 대비 절반의 한도. 본 amendment 는 한도 변경 없음. multi-tab 사용자가 한도를 빠르게 채울 수 있으나 부모 v0.2.1 의 한도 자체가 보수적이므로 후속 SPEC 에서 검토.
9. **Migration / cutover 명시적 단계**: Buffer 는 in-memory per-process 이므로 process restart 시 unflushed entries 가 손실된다. v0.2.1 → amendment 의 cutover 는 **implicit drop-and-rebuild** — 별도 migration step 없음. 안전성 근거: (a) buffer TTL 이 24h 로 짧음, (b) 클라이언트가 `Last-Event-ID` / `X-Last-Sequence` semantics 로 갭을 자연스럽게 수용, (c) cutover 직후의 신규 connection 은 새 dispatcher 의 LogicalID-keyed bucket 에 정상 기록. 운영팀은 cutover 시점에 수 초간 in-flight outbound 가 손실될 수 있음을 인지해야 하며, 이는 부모 v0.2.1 의 process restart 에서도 동일한 한계.

---

## 11. References

- 부모 SPEC: `.moai/specs/SPEC-GOOSE-BRIDGE-001/spec.md` (v0.2.1, completed)
- 부모 progress: `.moai/specs/SPEC-GOOSE-BRIDGE-001/progress.md` (Out-of-scope 절)
- 부모 tasks: `.moai/specs/SPEC-GOOSE-BRIDGE-001/tasks.md`
- amendment 패턴 레퍼런스: `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001/spec.md`
- 영향받는 파일:
  - `internal/bridge/buffer.go` (키 의미 변경)
  - `internal/bridge/resume.go` (registry lookup 추가)
  - `internal/bridge/outbound.go` (dispatcher.SendOutbound 의 buffer keying)
  - `internal/bridge/ws.go` (Add 시 LogicalID 채움)
  - `internal/bridge/sse.go` (동일)
  - `internal/bridge/registry.go` (LogicalID method 추가)
  - `internal/bridge/types.go` (`WebUISession.LogicalID` 필드 추가)
- Local convention: `CLAUDE.local.md §2.5` — 코드 주석 영어, 본 spec 문서는 한국어
