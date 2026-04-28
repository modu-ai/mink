## Task Decomposition
SPEC: SPEC-GOOSE-AUDIT-001

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | Event Schema — AuditEvent struct, EventType enum, JSON marshaling | REQ-AUDIT-001 | - | internal/audit/event.go, internal/audit/event_test.go | done |
| T-002 | Append-Only Log Writer — O_APPEND file writer with concurrency safety | REQ-AUDIT-001, AC-01, AC-02 | T-001 | internal/audit/writer.go, internal/audit/writer_test.go | done |
| T-003 | Log Rotation — 100MB rotation + gzip compression | REQ-AUDIT-002, AC-03 | T-002 | internal/audit/rotation.go, internal/audit/rotation_test.go | done |
| T-004 | Dual-Location Writer — global + project-local simultaneous write | REQ-AUDIT-003 | T-002, T-003 | internal/audit/dual.go, internal/audit/dual_test.go | done |
| T-005 | Permission Auditor Adapter — permission.Auditor interface impl | REQ-AUDIT-001 | T-001, T-004 | internal/audit/adapter_permission.go, internal/audit/adapter_permission_test.go | done |
| T-006 | Query Engine — filter by time range, type, read .gz files | REQ-AUDIT-004, AC-04 | T-001 | internal/audit/query.go, internal/audit/query_test.go | done |
| T-007 | CLI Command — goose audit query [--since] [--until] [--type] | REQ-AUDIT-004 | T-006 | internal/cli/commands/audit.go, internal/cli/commands/audit_test.go | done |
| T-008 | Wire-up & Config — goosed bootstrap, AuditConfig, wiring | All REQs | T-004, T-005 | internal/config/config.go, internal/config/defaults.go | done |
