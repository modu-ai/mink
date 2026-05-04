# bridge 패키지 — Claude Code Bridge (ndjson + BRIDGE-001/AMEND-001)

**위치**: internal/bridge/  
**파일**: 43개 (.go + _test.go)  
**최근 변경**: SPEC-GOOSE-BRIDGE-001-AMEND-001 (M1~M4) PRs #93~#98  
**상태**: ✅ Completed (v0.1.1)

---

## 목적

Claude Code와 AI.GOOSE 간 이벤트 기반 통신 (WebSocket/SSE + ndjson). 세션 재연결 시 메시지 재전송 (LogicalID 기반, 24시간 TTL).

---

## 공개 API

### WebSocket Handler
```go
type WebSocketHandler struct {
    // WS upgrade + auth + session binding
}

func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)
// 웹소켓 업그레이드, 세션 인증, inbound/outbound 루프 시작
```

### Buffer (Reconnect Replay)
```go
type outboundBuffer struct {
    mu sync.Mutex
    queues map[string][]bufferEntry  // key: LogicalID (AMEND-001)
}

func (b *outboundBuffer) Append(logicalID string, msg OutboundMessage)
// Append message to queue (FIFO, 24h TTL)

func (b *outboundBuffer) Replay(logicalID string, lastSeq int64) []OutboundMessage
// Return all messages with Sequence > lastSeq in order
// @MX:ANCHOR — Sequence gap 불가능 invariant
```

### LogicalID Derivation (BRIDGE-001 AMEND-001)
```go
type LogicalID string

func DeriveLogicalID(cookieHash string, transport string) LogicalID
// HMAC-SHA256(cookieHash, "bridge" || transport)
// Invariant: same (cookieHash, transport) = same LogicalID
```

### Session Resumption
```go
type ResumerLogicalID struct {
    registry map[string]map[string]LogicalID  // transport → logicalID
}

func (r *ResumerLogicalID) Lookup(cookieHash string, transport string) LogicalID
// Return stored LogicalID if exists, else nil
```

---

## 내부 상태 머신

### Inbound (Client → Server)

```
WebSocket frame arrives (ndjson)
  │
  ├─ Parse JSON
  │   └─ Unmarshal to InboundMessage
  │
  ├─ Dispatch by Type
  │   ├─ "submit_message"
  │   │   └─ Send to query.SubmitMessage
  │   │
  │   ├─ "resolve_permission"
  │   │   └─ Send to query.ResolvePermission
  │   │
  │   └─ "keepalive"
  │       └─ Reset inactivity timer
  │
  └─ Send response (if any)
```

### Outbound (Server → Client)

```
Server creates OutboundMessage
  │
  ├─ Allocate Sequence (per LogicalID, AMEND-001)
  │   └─ outboundDispatcher.nextSequence(logicalID)
  │
  ├─ Append to buffer
  │   └─ outboundBuffer.Append(logicalID, msg)
  │
  ├─ Send to all connIDs with same LogicalID
  │   ├─ Try conn1 → success ✓
  │   ├─ Try conn2 → offline (buffer holds it)
  │   └─ Try conn3 → exists
  │
  └─ Encode ndjson + send frame
```

---

## BRIDGE-001 + AMEND-001 변경사항

### M1: DeriveLogicalID + WebUISession.LogicalID (PR #94)

**이전**: 연결마다 새 sessionID (임시)  
**현재**: CookieHash + Transport 조합 → LogicalID (영구)

```go
// WebUISession now has LogicalID field
type WebUISession struct {
    ID         string
    LogicalID  string  // NEW: HMAC-SHA256(cookieHash, transport)
    CookieHash string
    Transport  string
    CreatedAt  time.Time
}

// Derive during auth
func authHandler.Authenticate(...) {
    logID := DeriveLogicalID(cookies["GOOSE_COOKIE"], "websocket")
    // ... create session with logID
}
```

**이유**: 다탭/재연결 시 동일 사용자 세션 추적

### M2: Registry.LogicalID lookup + ws/sse LogicalID fill (PR #95)

**추가**: LogicalID Registry (in-memory)

```go
// Global registry (session-scoped)
type registryLogicalID struct {
    mu      sync.RWMutex
    entries map[string]LogicalID  // sessionID → logicalID
}

func (r *registryLogicalID) Store(sessionID, logID string)
func (r *registryLogicalID) Lookup(sessionID string) (string, bool)

// Fill LogicalID on inbound
func dispatchInbound(ctx context.Context, msg InboundMessage) error {
    logID, found := registry.Lookup(msg.SessionID)
    if found {
        msg.LogicalID = logID
    }
    // ... continue dispatch
}
```

**이유**: 무상태 dispatch에서 LogicalID 재구성 가능

### M3: dispatcher buffer rekey (LogicalID) + logout eager-drop (PR #96)

**변경**: Buffer key = LogicalID (이전: connID)

```go
// Before (connID-keyed):
outboundBuffer.queues map[connID][]OutboundMessage
// Problem: 재연결 시 새 connID → 버퍼 손실

// After (LogicalID-keyed, AMEND-001):
outboundBuffer.queues map[LogicalID][]OutboundMessage
// Benefit: 동일 LogicalID의 모든 connID가 같은 버퍼 공유
```

**Logout eager-drop**:
```go
// On logout, immediately drop buffer for that LogicalID
func logoutHandler(logicalID string) {
    buffer.Drop(logicalID)  // Don't wait for TTL
}
```

**이유**: 메모리 효율 + 보안 (로그아웃 후 재사용 방지)

### M4: resumer LogicalID lookup + multi-tab integration (PR #97)

**추가**: ResumerLogicalID (재연결 전용)

```go
// On new connection
func bindHandler.Bind(connID, logicalID string, transport string) {
    // 1. Look up previous LogicalID
    prevLogID, found := resumer.Lookup(logicalID, transport)
    
    if found {
        // 2. Reuse buffer queue
        msgs := buffer.Replay(prevLogID, lastSeq)
        // 3. Send replayed messages
        for _, msg := range msgs {
            conn.Send(msg)
        }
    }
}

// Multi-tab: all tabs with same LogicalID get same replay
```

**Invariant**: Sequence gap 없음 (monotonic per LogicalID)

```go
// Sequence allocation
type outboundDispatcher struct {
    seqCounters map[LogicalID]int64  // per-LogicalID sequence
}

func (d *outboundDispatcher) nextSequence(logicalID string) int64 {
    d.mu.Lock()
    defer d.mu.Unlock()
    seq := d.seqCounters[logicalID]
    d.seqCounters[logicalID]++
    return seq
}
```

---

## 핵심 타입

### OutboundMessage
```go
type OutboundMessage struct {
    Type     string      // "message", "permission_request", "status"
    Sequence int64       // Monotonic per LogicalID
    Payload  json.RawMessage
    SentAt   time.Time
}
```

### InboundMessage
```go
type InboundMessage struct {
    Type       string
    SessionID  string  // From cookie/header
    LogicalID  string  // Filled by registry or derived
    Payload    json.RawMessage
    ReceivedAt time.Time
}
```

### bufferEntry (Internal)
```go
type bufferEntry struct {
    msg     OutboundMessage
    addedAt time.Time
    bytes   int
}
```

---

## 버퍼 정책 (REQ-BR-009)

### 제한
- **MaxBufferBytes**: 4 MiB (전체)
- **MaxBufferMessages**: 500 (per LogicalID)
- **BufferTTL**: 24시간

### 제거 (Eviction)
```
Evict when:
  (total_bytes >= 4 MiB) OR (queued_count >= 500)

Drop oldest until both constraints hold.

TTL check: Lazy (on every Append/Replay).
```

---

## @MX:ANCHOR 함수

| # | 함수 | 이유 |
|----|------|------|
| 1 | `outboundBuffer.Replay()` | 재연결 replay 핵심 (fan-in 2+) |
| 2 | `DeriveLogicalID()` | BRIDGE-001 AMEND-001 (모든 인증에서 호출) |
| 3 | `WebSocketHandler.runFrameLoop()` | 메시지 루프 (goroutine entry) |
| 4 | `dispatchInbound()` | 인바운드 라우팅 (fan-in 3+) |

---

## 동시성 안전성

### Mutex Guards

| 구조 | Mutex | 역할 |
|-----|-------|------|
| outboundBuffer.queues | outboundBuffer.mu | 버퍼 맵 + 카운트 |
| registryLogicalID.entries | registryLogicalID.mu | 로직ID 등록 |
| outboundDispatcher.seqCounters | outboundDispatcher.mu | Sequence 할당 |

### Race Hazards

1. **bufferEntry.addedAt** (lazy TTL check)
   - 안전: 각 entry의 time.Time은 불변
   - ❌ @MX:WARN if accessed without mu

2. **Multiple connIDs same LogicalID**
   - 안전: 모두 동일 LogicalID 키 사용
   - 다른 connID의 메시지도 buffer에서 find 가능
   - ⚠️ 주의: conn1이 ack한 메시지를 conn2도 받을 수 있음 (중복 가능)

---

## 테스트 커버리지

| 파일 | 테스트 |
|------|--------|
| buffer.go | buffer_test.go, buffer_logical_test.go |
| logical_id.go | logical_id_test.go |
| auth.go | auth_test.go |
| bind.go | bind_test.go |
| outbound.go | outbound_test.go, cross_conn_replay_test.go |

**최근 추가** (AMEND-001):
- `cross_conn_replay_test.go` — multi-connID, same LogicalID 재연결 (PR #97)
- `followup_integration_test.go` — 후속 메시지 + 재연결 (PR #97)

---

## 최근 SPEC 참조

| SPEC | Stage | Status |
|------|-------|--------|
| SPEC-GOOSE-BRIDGE-001 | M1-M4 | ✅ Merged (#93~#98) |
| SPEC-GOOSE-BRIDGE-001-AMEND-001 | M1-M4 | ✅ Completed |

**REQ 매핑**:
- REQ-BR-002: 24h TTL (buffer + cookie 일관성)
- REQ-BR-009: Buffer limits (4 MiB, 500 msg)
- REQ-BR-AMEND-003: LogicalID rekey (M3)
- REQ-BR-AMEND-006: Sequence monotonic (M4)

---

## 향후 작업 (Potential)

1. **Compression**: ndjson 크기 최적화 (gzip frame)
2. **Ack protocol**: 명시적 ack (중복 제거)
3. **Encryption**: End-to-end (TLS 추가)
4. **Priority queue**: 중요 메시지 우선

---

**Version**: bridge v0.1.1  
**AMEND-001 Status**: Completed (v0.1.1)  
**Generated**: 2026-05-04  
**Files**: 43 (internal/bridge/*.go)  
**LOC**: ~800  
**Quality**: TRUST 5 (Tested, Readable, Unified, Secured, Trackable)
