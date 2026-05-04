# 진입점 (Entry Points) — CLI & Daemon 부트스트랩

cli/goose (클라이언트) 및 cmd/goosed (gRPC 데몬) 시작 흐름.

---

## CLI 진입점: cmd/goose

### 초기화 단계 (1-3단계)

```
main()  [cmd/goose/main.go]
  │
  ├─ Step 1: Parse CLI flags
  │   └─ cobra.Command (args parsing)
  │
  ├─ Step 2: Load config
  │   └─ config.Loader → .moai/config, ~/.goose/config
  │
  └─ Step 3: Create CLI client
      └─ cli.NewClient(cfg) → grpc.ClientConn
```

**파일**: cmd/goose/main.go, cmd/goose/version.go, cmd/goose/help.go

**진입 지점**:
```go
func main() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

---

### 실행 흐름 (4-7단계)

```
Step 4: Connect to daemon
  goose → localhost:5050 (gRPC)
    └─ Wait for daemon start (retry 3x)

Step 5: Submit message
  cli.Execute(prompt)
    └─ client.SubmitMessage(ctx, prompt)
        └─ gRPC call to goosed.SubmitMessage

Step 6: Stream response
  for msg := range <-chan SDKMessage {
    fmt.Println(msg.Content)
  }

Step 7: Exit
  os.Exit(0)  [on completion or error]
```

**스트리밍 보장**:
- 첫 청크 < 2초 (LLM 요청 + 수신)
- 각 청크 < 100ms (버퍼)
- 연결 실패 시 graceful degradation

**타임아웃**:
- 데몬 연결: 5초
- 전체 대화: 5분 (REQ-QUERY-005 deadline)

---

## Daemon 진입점: cmd/goosed

### 부트스트랩 (Step 1-3)

```
main()  [cmd/goosed/main.go]
  │
  ├─ Step 1: Setup signals
  │   └─ signal.Notify(SIGINT, SIGTERM)
  │
  ├─ Step 2: Load config
  │   └─ config.Loader → .moai/config/*.yaml
  │
  └─ Step 3: Initialize logger
      └─ zap.NewProduction() or zap.NewDevelopment()
```

### 데몬 초기화 (Step 4-6)

```
Step 4: Create core services
  ├─ LLM Router
  │   └─ Detect available providers (Ollama, OpenAI, Claude, etc)
  │
  ├─ Memory Provider
  │   └─ Connect to SQLite FTS + Qdrant
  │
  └─ Permission Store
      └─ Initialize SQLite permission DB

Step 5: Create query engine factory
  └─ query.NewQueryEngine(cfg) [not instantiated yet]

Step 6: Create transport layer
  ├─ gRPC server (port 5050)
  ├─ WebSocket upgrade handler
  └─ SSE handler
```

### 통신 계층 시작 (Step 7-9)

```
Step 7: Register gRPC services
  └─ agent.proto services to gRPC server

Step 8: Listen on port 5050
  └─ listener, _ := net.Listen("tcp", ":5050")
     server.Serve(listener)

Step 9: Start session drain background job
  └─ go core.StartSessionDrainWorker() [checks for stale sessions]
```

### Graceful Shutdown (Step 10-13)

```
Step 10: Wait for signal
  └─ <-sigChan (SIGINT or SIGTERM)

Step 11: Stop accepting new connections
  └─ server.GracefulStop() [5s timeout]

Step 12: Drain active sessions
  └─ session.Drain() [flush buffers, save state]

Step 13: Cleanup
  ├─ memory.Close() [checkpoint SQLite/Qdrant]
  ├─ audit.Flush() [flush audit logs]
  └─ os.Exit(0)
```

**파일**: cmd/goosed/main.go, cmd/goosed/server.go, cmd/goosed/signals.go

---

## Session 생명주기

### 새 연결 (1-3단계)

```
Client connects to goosed:5050 (gRPC)
  │
  ├─ Step 1: Authentication
  │   └─ bridge.AuthHandler.Authenticate(ctx, credentials)
  │       ├─ Check API key
  │       └─ Create WebUISession (with LogicalID)
  │
  ├─ Step 2: Create session state
  │   └─ session.New(sessionID)
  │       ├─ Initialize context budget
  │       ├─ Create logger (session-scoped)
  │       └─ Start inbound/outbound queues
  │
  └─ Step 3: Register session
      └─ core.RegisterSession(sessionID, session)
          ├─ Map sessionID → *session.Session
          └─ Start keep-alive (30s intervals)
```

### Message 처리 (4-6단계)

```
Step 4: Client sends SubmitMessage
  └─ transport.SubmitMessage(sessionID, prompt)
      └─ route to core.Session.SubmitMessage

Step 5: Session spawns query engine
  └─ query.SubmitMessage(ctx, prompt)
      ├─ Create QueryEngine (session-scoped)
      ├─ Spawn Agent goroutine (async)
      └─ Return <-chan SDKMessage immediately

Step 6: Stream responses
  └─ Agent streaming:
      ├─ Call LLM provider
      ├─ Emit chunks to <-chan SDKMessage
      └─ On completion: LogInteraction to learning
```

### 재연결 (BRIDGE-001 AMEND-001)

```
Step 7: Client reconnects with same CookieHash
  └─ bridge.ResumerLogicalID.Lookup(cookieHash, transport)
      ├─ Find previous LogicalID
      └─ Reuse queues (if within 24h TTL)

Step 8: Replay buffered messages
  └─ outboundBuffer.Replay(logicalID, lastSeq)
      ├─ Find all msgs where Sequence > lastSeq
      └─ Resend in order

Step 9: Resume new messages
  └─ Forward new messages sent while offline
```

### 세션 종료 (10-12단계)

```
Step 10: Client disconnects
  └─ session.Inbound closed (EOF)

Step 11: Stop session
  └─ session.Stop(ctx)
      ├─ Drain: flush outbound buffer to disk
      ├─ Save: checkpoint agent state
      └─ Cleanup: close all goroutines

Step 12: Unregister session
  └─ core.UnregisterSession(sessionID)
      └─ Allow GC of session + QueryEngine
```

**상태 보존**:
- 버퍼: 24시간 (REQ-BR-002)
- 세션 메타: 7일 (설정 가능)
- 학습 데이터: 무한 (SQLite + Qdrant)

---

## WebSocket 핸드셰이크

### 업그레이드 흐름 (1-4단계)

```
Client HTTP GET /ws with Upgrade headers
  │
  ├─ Step 1: Gorilla websocket upgrade
  │   └─ upgrader.Upgrade(w, r, nil)
  │       └─ Emit 101 Switching Protocols
  │
  ├─ Step 2: Authentication
  │   └─ bridge.AuthHandler (check headers/cookies)
  │       └─ Extract sessionID or create new
  │
  ├─ Step 3: Bind connection
  │   └─ bridge.BindHandler.Bind(ws, sessionID)
  │       ├─ Register conn with session
  │       ├─ Enable cross-conn replay (LogicalID)
  │       └─ Start inbound/outbound frame loops
  │
  └─ Step 4: Start message loops
      ├─ go runInboundFrameLoop(conn, sessionID)
      │   └─ Read ndjson from WS → Inbound queue
      └─ go runOutboundFrameLoop(conn, sessionID)
          └─ Write ndjson from Outbound queue → WS
```

### Frame Format (ndjson)

```
Inbound (client → server):
  {"type": "submit_message", "payload": {"prompt": "..."}}
  {"type": "resolve_permission", "payload": {"toolUseID": "...", "approved": true}}

Outbound (server → client):
  {"type": "message", "payload": {"role": "assistant", "content": [...]}}
  {"type": "permission_request", "payload": {"toolUseID": "...", "request": {...}}}
  {"type": "status", "payload": {"state": "streaming"}}
```

### 에러 처리 (5-6단계)

```
Step 5: Handle frame errors
  ├─ Network error (I/O)
  │   └─ Graceful close (save state)
  ├─ Protocol error (malformed ndjson)
  │   └─ Send error frame + keep-alive
  └─ Session error (sessionID not found)
      └─ Close conn with 4000 code

Step 6: Reconnection
  └─ (see BRIDGE-001 AMEND-001 above)
```

---

## SSE (Server-Sent Events) 흐름

### 단방향 스트리밍 (1-3단계)

```
Client HTTP GET /events?sessionID=...
  │
  ├─ Step 1: Authentication
  │   └─ Extract sessionID from query param
  │
  ├─ Step 2: Create SSE writer
  │   └─ w.Header().Set("Content-Type", "text/event-stream")
  │
  └─ Step 3: Subscribe to session's outbound queue
      └─ channel := session.OutboundQueue()
```

### Event Stream (4-5단계)

```
Step 4: Stream events
  for event := range channel {
    fmt.Fprintf(w, "data: %s\n\n", json.Marshal(event))
    flusher.Flush()
  }

Step 5: Graceful close
  └─ Client closes request → channel cleanup
```

**제약사항**:
- 단방향 (서버 → 클라이언트만)
- 새 요청으로 인바운드 (REST API로 분리)
- CORS 필요

---

## Shutdown 시나리오

### Graceful Shutdown (최대 30초)

```
SIGTERM/SIGINT
  │
  ├─ Step 1: Stop accepting new connections
  │   └─ listener.Close()
  │
  ├─ Step 2: Stop creating new sessions
  │   └─ core.StopAcceptingSessions()
  │
  ├─ Step 3: Drain active sessions (timeout 20s)
  │   for sessionID, sess := range activeSessions {
  │     go func() {
  │       ctx, cancel := context.WithTimeout(ctx, 20s)
  │       defer cancel()
  │       sess.Drain(ctx)
  │     }()
  │   }
  │
  ├─ Step 4: Checkpoint state
  │   ├─ memory.Checkpoint()
  │   ├─ audit.Flush()
  │   └─ permission.Save()
  │
  └─ Step 5: Exit
      └─ os.Exit(0)
```

### Force Shutdown (10초 경과 시)

```
If any session still active after 20s:
  ├─ Cancel context (interrupt LLM calls)
  ├─ Close all connections (no graceful close)
  ├─ Abandon pending buffers (lose message history)
  └─ Force exit (cleanup by kernel)
```

---

## 환경 변수 / 설정

### Critical

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `GOOSE_LISTEN` | `:5050` | gRPC 수신 주소 |
| `GOOSE_LLM_PROVIDER` | `ollama` | 기본 LLM (ollama/openai/claude) |
| `GOOSE_LOG_LEVEL` | `info` | 로그 레벨 (debug/info/warn/error) |

### Optional

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `GOOSE_MEMORY_DB` | `~/.goose/memory.db` | SQLite 경로 |
| `GOOSE_QDRANT_URL` | `http://localhost:6333` | Qdrant 벡터 DB |
| `GOOSE_PERMISSION_DB` | `~/.goose/permission.db` | Permission store |
| `GOOSE_SESSION_TTL` | `7d` | 세션 만료 |
| `GOOSE_BUFFER_TTL` | `24h` | 버퍼 TTL (BRIDGE-001) |

---

**Version**: Entry Points v0.1.0  
**Generated**: 2026-05-04  
**Bootstrap Steps**: 13 (daemon), 7 (CLI)  
**Key Invariants**: Session 1:1 with QueryEngine, LogicalID reuse (BRIDGE-AMEND-001), Graceful shutdown 30s
