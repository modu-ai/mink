# transport + config 패키지 (통합)

**위치**: internal/transport/, internal/config/  
**파일**: 4개 (transport) + 12개 (config)  
**상태**: ✅ Active

---

## 목적

Transport: gRPC/WS/SSE 코덱. Config: YAML 로더 + 유효성 검증.

---

## Transport API

### Transport Interface
```go
type Transport interface {
    // gRPC, WebSocket, SSE 모두 구현
    Send(msg Message) error
    Receive() (<-chan Message, error)
    Close() error
}

// Implementations:
// - gRPC: Bidirectional streaming
// - WebSocket: Framed ndjson
// - SSE: Server-sent events (server→client only)
```

---

## Config API

### Loader
```go
type Loader struct {
    configPath string  // ~/.goose/config.yaml
}

func (l *Loader) Load() (Config, error)
    // 1. Read YAML
    // 2. Merge with defaults
    // 3. Validate all fields
    // 4. Return Config
```

### Config Schema
```yaml
llm:
  provider: ollama  # or openai, claude, google
  models: [gpt-4, claude-3]
  
memory:
  sqlite: ~/.goose/memory.db
  qdrant: http://localhost:6333
  
permission:
  declared:
    - tool: read
      approved: true
  
server:
  listen: :5050
  tls: false
```

---

## Validation

```
1. Required fields check
2. Type validation
3. Range checks (timeouts, limits)
4. Dependency checks (if A then B required)
5. File path existence
```

---

**Version**: transport + config v0.1.0  
**Generated**: 2026-05-04  
**LOC**: ~160
