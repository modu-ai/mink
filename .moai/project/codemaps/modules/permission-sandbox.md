# permission + sandbox 패키지 (통합)

**위치**: internal/permission/, internal/permissions/, internal/sandbox/  
**파일**: 14개 + 13개 (3-layer permission system)  
**상태**: ✅ Active (SPEC-GOOSE-PERMISSION-001)

---

## 목적

3-layer permission system: Declared (설정) → First-Call (사용자 확인) → Audit (로깅).

---

## 공개 API

### PermissionRequester
```go
type PermissionRequester interface {
    // @MX:ANCHOR [AUTO] Tool execution gate
    // @MX:REASON: Fan-in ≥3 (Agent, Bridge, Query)
    Request(ctx context.Context, sessionID string, payload []byte) (bool, error)
}

// Checks in order:
// 1. DeclaredPermission (pre-approved)
// 2. FirstCall cache (this session already approved?)
// 3. Ask user (interactive dialog)
// 4. Audit log approval
```

### PermissionStore
```go
type PermissionStore interface {
    // Declared permissions (pre-approved)
    IsDeclared(user, tool, resource string) bool
    
    // First-call cache
    HasApproved(sessionID, user, tool, resource string) bool
    MarkApproved(sessionID, user, tool, resource string) error
    
    // Audit log
    LogRequest(entry AuditEntry) error
}
```

### Sandbox
```go
type Sandbox interface {
    // Execute tool in isolated WASM
    Execute(ctx context.Context, tool Tool, args map[string]interface{}) (Output, error)
    // Limits: 5s timeout, 256MB memory, restricted file access
}
```

---

## 3-Layer Permission Flow

```
Layer 1: Declared Permission
  ├─ Check config: policy pre-approval
  └─ If approved: log + allow

Layer 2: First-Call Cache
  ├─ Check session cache: already approved?
  └─ If yes: allow

Layer 3: Interactive Ask
  ├─ Send PermissionRequest to user
  ├─ User responds
  ├─ Cache decision
  ├─ Audit log
  └─ Return result
```

---

## @MX:ANCHOR

```go
// @MX:ANCHOR [AUTO] Tool permission gate
// @MX:REASON: Fan-in ≥3 (Agent, Bridge, Query)
// @MX:ANCHOR [AUTO] Sandbox entry
// @MX:REASON: Security boundary - untrusted code
```

---

**Version**: permission + sandbox v0.1.0  
**Generated**: 2026-05-04  
**LOC**: ~400
