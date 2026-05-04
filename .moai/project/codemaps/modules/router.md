# router 패키지 — Request Routing

**위치**: internal/router/ (or internal/core/router.go)  
**상태**: ✅ Active

---

## 목적

메시지 라우팅: gRPC request → internal service.

---

## 라우팅 테이블

```
/agent.proto/Agent/SubmitMessage → core.Session.SubmitMessage
/agent.proto/Agent/ResolvePermission → core.QueryEngine.ResolvePermission
/world.proto/World/RegisterService → core.RegisterService
```

---

## Implementation

```go
type Router struct {
    handlers map[string]Handler
}

func (r *Router) Route(method string, req interface{}) (interface{}, error)
```

---

**Version**: router v0.1.0  
**Generated**: 2026-05-04  
**LOC**: ~120
