# audit 패키지 — Audit Logging (AUDIT-001)

**위치**: internal/audit/  
**파일**: 8개  
**상태**: ✅ Active (SPEC-GOOSE-AUDIT-001)

---

## 목적

모든 활동 감시: 에이전트 실행, 도구 사용, 권한 결정, 메모리 접근.

---

## 공개 API

### AuditLog
```go
type Logger struct {
    db *sql.DB  // SQLite audit.db
}

func (l *Logger) LogAgentExecution(agent string, prompt string, result string) error
func (l *Logger) LogToolExecution(tool string, args map[string]interface{}, output string) error
func (l *Logger) LogPermissionRequest(user, tool, resource string, approved bool) error
func (l *Logger) LogMemoryAccess(user string, query string, results int) error
```

### Query
```go
func (l *Logger) Query(filter AuditFilter) ([]AuditEntry, error)
    // Filter by: timestamp, user, agent, tool, approved
    // Return ordered entries
```

---

## Schema

```sql
CREATE TABLE audit (
    id INTEGER PRIMARY KEY,
    timestamp DATETIME,
    event_type TEXT,  -- agent_execution, tool_exec, permission_req, memory_access
    user_id TEXT,
    agent_name TEXT,
    tool_name TEXT,
    details TEXT,
    result TEXT
);
```

---

## Use Cases

```
1. Compliance: Export audit logs (SOC 2)
2. Debugging: "What happened to user X?"
3. Security: "Did anyone access private files?"
4. Learning: "Which tools are most used?"
```

---

**Version**: audit v0.1.0  
**Generated**: 2026-05-04  
**LOC**: ~200
