---
spec: SPEC-GOOSE-TRAJECTORY-001
version: 0.1.1
methodology: TDD (RED-GREEN-REFACTOR)
harness: standard
created_at: 2026-04-30
---

# Task Decomposition — SPEC-GOOSE-TRAJECTORY-001

## Acceptance Criteria → Task Mapping

| AC | REQ | Task | Status |
|----|-----|------|--------|
| AC-TRAJECTORY-001 | REQ-002, REQ-005 | M0: types.go ShareGPT schema + M2: collector OnTerminal success flush | TODO |
| AC-TRAJECTORY-002 | REQ-006 | M2: collector OnTerminal failure routing | TODO |
| AC-TRAJECTORY-003 | REQ-004 | M1: redact email rule | TODO |
| AC-TRAJECTORY-004 | REQ-004 | M1: 6 builtin redact rules | TODO |
| AC-TRAJECTORY-005 | REQ-007 | M3: writer size-based rotation | TODO |
| AC-TRAJECTORY-006 | REQ-008 | M3: writer date rollover | TODO |
| AC-TRAJECTORY-007 | REQ-011 | M2: collector disabled noop | TODO |
| AC-TRAJECTORY-008 | REQ-009 | M4: retention sweep | TODO |
| AC-TRAJECTORY-009 | REQ-010 | M3: writer error isolation | TODO |
| AC-TRAJECTORY-010 | REQ-016 | M1: redact system role skip | TODO |
| AC-TRAJECTORY-011 | REQ-012 | M2: collector buffer spill | TODO |
| AC-TRAJECTORY-012 | REQ-015 | M3: concurrent write no-interleave | TODO |
| AC-TRAJECTORY-013 | REQ-003 | M3: writer file permission 0600/0700 | TODO |
| AC-TRAJECTORY-014 | REQ-013 | M2: collector OnTurn latency <1ms | TODO |
| AC-TRAJECTORY-015 | REQ-014 | M1: redact panic isolation | TODO |

## Milestones

### M0: Data Model (types.go)
- Define `Role` enum (system/human/gpt/tool)
- Define `TrajectoryEntry` struct (ShareGPT schema)
- Define `Trajectory` struct with all metadata fields
- Define `TrajectoryMetadata` struct
- Define `TelemetryConfig` struct

**Tasks (atomic)**:
- M0-T1: Role enum + constants
- M0-T2: TrajectoryEntry (From/Value + JSON tags)
- M0-T3: Trajectory struct (Conversations/Timestamp/Model/Completed/SessionID/Metadata)
- M0-T4: TrajectoryMetadata struct (Tags/FailureReason/Partial/TurnCount/DurationMs/Tokens)
- M0-T5: TelemetryConfig struct (Enabled/RetentionDays/RedactRules/MaxFileBytes/InMemoryTurnCap)

### M1: Redact Subsystem (redact/)
- `redactor.go`: Rule struct + Chain type + Apply method with panic recovery
- `rules.go`: 6 builtin rules (email/openai_key/bearer_jwt/credit_card/kr_phone/home_path)
- `rules_test.go`: RED tests for each rule + panic isolation + system role skip

**Tasks (atomic)**:
- M1-T1: Rule struct + Chain Apply (no-op implementation)
- M1-T2: Panic recovery in Apply + `<REDACT_FAILED>` substitution
- M1-T3: AppliesToSystem gate (system role skip)
- M1-T4: email builtin rule
- M1-T5: openai_key builtin rule
- M1-T6: bearer_jwt builtin rule
- M1-T7: credit_card builtin rule (Luhn check)
- M1-T8: kr_phone builtin rule
- M1-T9: home_path builtin rule

### M2: Collector (collector.go)
- Internal channel + worker goroutine
- OnTurn: send to channel (non-blocking, <1ms)
- OnTerminal: route to success/failed, flush buffer
- Disabled noop mode
- Buffer spill at InMemoryTurnCap

**Tasks (atomic)**:
- M2-T1: Collector struct + New() + worker goroutine
- M2-T2: OnTurn implementation (channel send)
- M2-T3: OnTerminal success flush
- M2-T4: OnTerminal failure flush (failure_reason)
- M2-T5: Disabled noop (enabled=false)
- M2-T6: Buffer spill (InMemoryTurnCap exceeded, partial=true)
- M2-T7: Close() graceful shutdown

### M3: Writer (writer.go + rotation.go)
- Append-only JSON-L writer
- File permissions 0600 (file) + 0700 (dir)
- Size-based rotation
- Date-based rotation (clockwork mock clock)
- Concurrent write safety (mutex)
- Error isolation (non-blocking)

**Tasks (atomic)**:
- M3-T1: Writer struct + WriteTrajectory (basic append)
- M3-T2: File permission enforcement (0600/0700)
- M3-T3: Size-based rotation (N-suffix)
- M3-T4: Date-based rotation (clockwork)
- M3-T5: Write error isolation (log + no propagate)
- M3-T6: Concurrent write safety (mutex serialize)

### M4: Retention (retention.go)
- Sweep(). Delete files older than retention_days (UTC)
- Skip open files

**Tasks (atomic)**:
- M4-T1: Retention struct + Sweep() implementation
- M4-T2: UTC date comparison + 90-day default

## Task Execution Order

```
M0 (types) → M1 (redact) → M2 (collector) → M3 (writer) → M4 (retention)
```

Dependencies:
- M1 depends on M0 (TrajectoryEntry)
- M2 depends on M0 (Trajectory, TelemetryConfig) + M1 (Chain) + M3 (Writer)
- M3 depends on M0 (Trajectory)
- M4 depends on M3 (Writer)

## TDD Entry Order (per spec.md §6.7)

1. M1-T4: `TestRedactRule_Email_ReplacesCanonicalForm` (AC-003)
2. M1-T4..T9: `TestRedactRule_SixBuiltinsAllFire` (AC-004)
3. M1-T3: `TestRedactChain_SystemRoleSkippedByDefault` (AC-010)
4. M2+M3: `TestCollector_OnTerminalSuccess_WritesToSuccessDir` (AC-001)
5. M2+M3: `TestCollector_OnTerminalFailure_WritesToFailedDir` (AC-002)
6. M3-T3: `TestWriter_RotatesOnMaxBytes` (AC-005)
7. M3-T4: `TestWriter_RolloverOnDateChange` (AC-006)
8. M2-T5: `TestCollector_DisabledIsNoop` (AC-007)
9. M4-T1: `TestRetention_SweepOldFiles` (AC-008)
10. M3-T5: `TestWriter_WritePermissionDeniedDoesNotBlock` (AC-009)
11. M2-T6: `TestCollector_SpillOnBufferCap` (AC-011)
12. M3-T6: `TestConcurrentSessions_NoInterleaving` (AC-012)
13. M3-T2: `TestWriter_FilePermissions` (AC-013)
14. M2-T2: `TestCollector_OnTurnLatencyUnder1ms` (AC-014)
15. M1-T2: `TestRedactChain_PanicIsolation` (AC-015)

## Total Atomic Tasks: 23
