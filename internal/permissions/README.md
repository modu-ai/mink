# Package permissions (Deprecated)

**Status:** Deprecated - Use `internal/permission` instead

## Migration Guide

This package (`internal/permissions` with an 's') is **deprecated** and should not be used for new code. All permission-related functionality has been moved to the singular `internal/permission` package.

### What to Use Instead

| Old (Deprecated) | New (Use This) |
|------------------|----------------|
| `internal/permissions` | `internal/permission` |
| `permissions.PermissionBehavior` | `permission.DecisionChoice` |
| `permissions.Decision` | `permission.Decision` |
| `permissions.CanUseTool` | `permission.CanUseTool` (interface) |
| `permissions.ToolPermissionContext` | `permission.PermissionRequest` |

### Why the Change?

The `internal/permission` package (singular) provides a complete, production-ready implementation of the Declared Permission system with:

- **Full feature coverage**: Grant management, store abstraction, confirmer/auditor interfaces
- **Test coverage**: 89.8% vs 0% in this package
- **Active maintenance**: All new features go to `internal/permission`
- **Better architecture**: Cleaner separation of concerns with store, manager, and grant types

### Timeline

- **Deprecated:** 2026-04-28
- **Removal:** v0.2.0 (after 2 minor versions grace period)

### Migration Example

**Before (deprecated):**
```go
import "github.com/modu-ai/goose/internal/permissions"

func checkPermission(ctx context.Context, tpc permissions.ToolPermissionContext) permissions.Decision {
    // ...
}
```

**After (use this):**
```go
import "github.com/modu-ai/goose/internal/permission"

func checkPermission(ctx context.Context, req permission.PermissionRequest) permission.Decision {
    // ...
}
```

## Package Overview (Legacy)

This package defined basic permission behavior types and interfaces for tool execution gating. It has been superseded by the full-featured `internal/permission` package.

### Exports (Deprecated)

- `PermissionBehavior`: Allow/Deny/Ask constants
- `Decision`: Permission decision with behavior and reason
- `CanUseTool`: Interface for permission checks
- `ToolPermissionContext`: Tool execution context

### No Test Coverage

This package has **0% test coverage** with no test files. The replacement `internal/permission` package has comprehensive tests in `manager_test.go`, `errors_test.go`, and other test files.

---

**Last Updated:** 2026-04-28
**Deprecation Notice:** Use `internal/permission` for all new code
