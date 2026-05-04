# core 패키지 — Daemon Core (Runtime, Session, Drain)

**위치**: internal/core/  
**파일**: 14개 (runtime.go, session.go, drain.go, shutdown.go, logger.go)  
**상태**: ✅ Active (SPEC-GOOSE-CORE-001)

---

## 목적

데몬 생명주기: Runtime (부트스트랩) → Session (연결 상태) → Drain (graceful shutdown).

---

## 공개 API

### Runtime
```go
type Runtime struct {
    llmRouter   llm.Router
    memory      memory.MemoryProvider
    permStore   permission.Store
    logger      *zap.Logger
    sessionsMu  sync.RWMutex
    sessions    map[string]*Session
}

func (r *Runtime) Start(ctx context.Context) error
    // 1. Initialize services
    // 2. Start gRPC server
    // 3. Start drain worker

func (r *Runtime) Stop(ctx context.Context) error
    // 1. Stop accepting new connections
    // 2. Drain active sessions (20s timeout)
    // 3. Close services
```

### Session
```go
type Session struct {
    ID         string
    LogicalID  string
    inboundQ   chan InboundMessage
    outboundQ  chan OutboundMessage
    state      *query.QueryEngine  // 1:1 correspondence
    drainMu    sync.Mutex
}

func (s *Session) SubmitMessage(ctx context.Context, prompt string) (<-chan SDKMessage, error)
    // Route to QueryEngine

func (s *Session) Drain(ctx context.Context) error
    // Flush buffers, save state, close goroutines
```

### Drain (Queue Management)
```go
type Drain struct {
    mu sync.Mutex
    queues map[string]*SessionQueue  // sessionID → queue
}

func (d *Drain) StartWorker(ctx context.Context)
    // Background job: Check for stale sessions (>7d)
    // Drain queue, cleanup resources
    // Run every 1 minute
```

---

## Bootstrap 13-Step

```
1. Parse flags
2. Load config
3. Setup logger
4. Connect to LLM providers
5. Initialize memory (SQLite + Qdrant)
6. Initialize permission store
7. Create gRPC server
8. Register services
9. Listen on port 5050
10. Start drain worker
11. Log "Ready"
12. Block on signal (SIGTERM/SIGINT)
13. Graceful shutdown (up to 30s)
```

---

## Session Lifecycle

```
Connection → Create Session → Register → Message loop → Drain → Cleanup

Each Session:
  ├─ ID: UUID (unique)
  ├─ LogicalID: HMAC(cookieHash, transport)
  ├─ QueryEngine: 1:1 correspondence
  ├─ TTL: 7 days (configurable)
  └─ Drain: Save state on disconnect
```

---

## Graceful Shutdown Flow

```
SIGTERM → Stop listeners → Drain sessions (20s timeout) → Cleanup → Exit
  │                            │                            │
  ├─ No new connections        ├─ Flush outbound buffers   ├─ memory.Close()
  ├─ New SubmitMessage fails   ├─ Flush audit logs         ├─ permission.Save()
  └─ Signal waiting clients    └─ Save checkpoints         └─ Exit 0
```

---

## SPEC 참考

| SPEC | 状态 |
|------|------|
| SPEC-GOOSE-CORE-001 | ✅ Runtime lifecycle |
| SPEC-SESSION-001 | ✅ Session management |

---

**Version**: core v0.1.0  
**LOC**: ~140  
**Generated**: 2026-05-04
