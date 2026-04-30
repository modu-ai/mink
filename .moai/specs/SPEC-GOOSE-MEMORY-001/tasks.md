---
spec: SPEC-GOOSE-MEMORY-001
version: 0.1.0
created_at: 2026-04-29
development_mode: tdd
harness: standard
total_tasks: 10
total_acs: 23
coverage_verified: true
---

# Tasks — SPEC-GOOSE-MEMORY-001

## Plan Summary

Build a Pluggable Memory Provider system for AI.GOOSE's self-evolution pipeline (Layer 4). The system defines a `MemoryProvider` interface (4 required + 9 optional methods), a `MemoryManager` for registration/dispatch coordination with 50ms dispatch budget and failure isolation, a `BuiltinProvider` using SQLite FTS5 (already in go.mod as `modernc.org/sqlite v1.50.0`) for full-text search with FIFO eviction, and a plugin adapter/registry for at most one external provider. TDD cycles follow a bottom-up dependency chain: types/errors first, then interface + BaseProvider, then Manager registration/dispatch, then BuiltinProvider SQLite + file layers, then plugin adapter. All 23 ACs are decomposed into 10 atomic tasks with clear RED-GREEN-REFACTOR cycles.

## Requirements Count

- **REQs**: 21 (REQ-MEMORY-001 through REQ-MEMORY-021)
- **ACs**: 23 (AC-MEMORY-001 through AC-MEMORY-023)
- **EARS Patterns**: 5 Ubiquitous + 6 Event-Driven + 2 State-Driven + 5 Unwanted + 3 Optional

## Key Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| SQLite driver | `modernc.org/sqlite` v1.50.0 | Already in go.mod; pure Go (no CGO); FTS5 included by default |
| FTS5 tokenizer | `porter unicode61` | Stemming + Unicode support for multilingual content |
| Logger | `go.uber.org/zap` | CORE-001 standard; tests use `zap.NewNop()` |
| Test isolation | `t.TempDir()` for SQLite | Each test gets its own DB file; no cross-test contamination |
| Error pattern | Sentinel errors with `errors.Is()` | Follows `internal/config/errors.go` pattern |
| Config pattern | YAML struct tags + defaults | Follows `internal/config/config.go` pattern |
| Concurrency | `sync.RWMutex` for manager, `sync.Mutex` per-provider | Follows `internal/tools/registry.go` pattern |

---

## Task List

### Task 01: Foundation Types + Sentinel Errors

**ID**: MEMORY-T01
**Priority**: P0 (blocker — all subsequent tasks depend on this)
**Dependencies**: none
**AC Coverage**: foundational (required by all ACs)

**Description**: Define all shared types (`SessionContext`, `RecallResult`, `RecallItem`, `ToolSchema`, `ToolContext`, `Message`) and sentinel errors (`ErrBuiltinRequired`, `ErrOnlyOnePluginAllowed`, `ErrNameCollision`, `ErrToolNameCollision`, `ErrInvalidProviderName`, `ErrUserMdReadOnly`, `ErrProviderNotInit`, `ErrUnknownPlugin`).

**Planned Files**:
- `internal/memory/errors.go` — Sentinel errors
- `internal/memory/types.go` — Shared types

**RED Tests**:
- `TestSentinelErrors_ProperlyWrapped` — errors.Is chains work correctly
- `TestSessionContext_Fields` — type construction and field access
- `TestToolSchema_JSONMarshal` — JSON serialization of ToolSchema
- `TestRecallResult_Empty` — zero-value behavior
- `TestProviderNameRegex_ValidInvalid` — name pattern validation

**GREEN Strategy**: Define all types as structs with documented fields. Errors as `fmt.Errorf` sentinel vars. Name regex as compiled `regexp.Regexp`.

**REFACTOR Notes**: Extract `isValidProviderName()` helper if repeated.

---

### Task 02: MemoryProvider Interface + BaseProvider

**ID**: MEMORY-T02
**Priority**: P0
**Dependencies**: Task 01
**AC Coverage**: foundational (interface contract for all provider ACs)

**Description**: Define the `MemoryProvider` interface with 13 methods (4 required: `Name`, `IsAvailable`, `Initialize`, `GetToolSchemas`; 9 optional with no-op defaults). Provide `BaseProvider` struct that embeds into concrete providers for default implementations.

**Planned Files**:
- `internal/memory/provider.go` — Interface + BaseProvider

**RED Tests**:
- `TestMemoryProvider_Interface_SatisfiedByMock` — compile-time interface check
- `TestBaseProvider_SystemPromptBlock_ReturnsEmpty` — no-op default
- `TestBaseProvider_Prefetch_ReturnsEmptyResult` — no-op default
- `TestBaseProvider_AllOptionalMethods_NoPanic` — all optional methods callable without panic
- `TestBaseProvider_QueuePrefetch_NoBlock` — returns immediately

**GREEN Strategy**: Define interface with Go doc comments per SPEC section 6.2. BaseProvider implements all optional methods as no-ops. Required methods remain unimplemented (forces concrete types to implement them).

**REFACTOR Notes**: Consider whether optional methods should be on a separate `OptionalProvider` interface for type assertion, or kept unified per SPEC.

---

### Task 03: MemoryConfig

**ID**: MEMORY-T03
**Priority**: P1
**Dependencies**: Task 01
**AC Coverage**: AC-016 (Builtin-only flow when plugin empty), config-driven behavior

**Description**: Define `MemoryConfig` struct with YAML tags for `builtin.db_path`, `builtin.max_rows`, `plugin.name`, `plugin.config`. Integrate with the existing `internal/config/config.go` pattern. Provide sensible defaults (db_path: `~/.goose/memory/memory.db`, max_rows: 10000).

**Planned Files**:
- `internal/memory/config.go` — MemoryConfig, BuiltinConfig, PluginConfig

**RED Tests**:
- `TestMemoryConfig_Defaults` — zero-value produces correct defaults
- `TestMemoryConfig_YAMLDeserialization` — YAML round-trip
- `TestMemoryConfig_EmptyPluginName` — no plugin is valid
- `TestMemoryConfig_BuiltinDefaults` — db_path and max_rows defaults
- `TestBuiltinConfig_DefaultMaxRows` — 10000 default

**GREEN Strategy**: Struct with YAML tags, `ApplyDefaults()` method, `Validate()` method. Follow `internal/config/config.go` immutable pattern.

**REFACTOR Notes**: May integrate into parent `Config` struct later via a `Memory MemoryConfig` field. For now, standalone.

---

### Task 04: MemoryManager Registration

**ID**: MEMORY-T04
**Priority**: P0 (core logic)
**Dependencies**: Task 02, Task 03
**AC Coverage**: AC-001 (Builtin required), AC-002 (max 1 plugin), AC-003 (name collision), AC-004 (tool name collision), AC-016 (Builtin-only flow)

**Description**: Implement `MemoryManager` struct with `New()`, `RegisterBuiltin()`, `RegisterPlugin()`. Enforce: Builtin always first, at most 1 plugin, name uniqueness (case-insensitive), tool name uniqueness across providers. Build `toolIndex` map at registration time.

**Planned Files**:
- `internal/memory/manager.go` — MemoryManager struct, constructor, registration methods
- `internal/memory/manager_test.go` — Registration tests
- `internal/memory/export_test.go` — Test-only accessors

**RED Tests**:
- `TestManager_InitializeWithoutBuiltin_ReturnsErrBuiltinRequired` — AC-001
- `TestManager_SecondPlugin_ReturnsErrOnlyOnePluginAllowed` — AC-002
- `TestManager_NameCollision_CaseInsensitive` — AC-003
- `TestManager_ToolNameCollision_AtRegistration` — AC-004
- `TestManager_BuiltinOnlyFlow_NoError` — AC-016
- `TestManager_RegisterBuiltin_InvalidName_ReturnsErr` — name regex
- `TestManager_RegisterPlugin_BeforeBuiltin_ReturnsErrBuiltinRequired`

**GREEN Strategy**: MemoryManager holds `providers []MemoryProvider`, `toolIndex map[string]int`, `cfg MemoryConfig`, `logger *zap.Logger`. Registration methods lock `mu sync.RWMutex`. Validate name, check collision, append to slice.

**REFACTOR Notes**: Extract `validateRegistration()` helper. Consider `dispatcher` sub-struct for lifecycle hook routing.

---

### Task 05: MemoryManager Dispatch — Lifecycle Hooks

**ID**: MEMORY-T05
**Priority**: P0 (core logic)
**Dependencies**: Task 04
**AC Coverage**: AC-006 (dispatch order), AC-007 (panic isolation), AC-008 (IsAvailable skip), AC-011 (50ms budget), AC-018 (OnPreCompress aggregation), AC-019 (init error suppression + retry), AC-022 (empty string no-wrap)

**Description**: Implement lifecycle hook dispatch methods: `Initialize()`, `OnTurnStart()`, `OnSessionEnd()`, `OnPreCompress()`. Key behaviors: forward order for Initialize, reverse (LIFO) for OnSessionEnd, 50ms total budget with 40ms per-provider timeout for OnTurnStart, panic recovery, IsAvailable check, init-failure tracking per session, OnPreCompress aggregation with conditional wrapping.

**Planned Files**:
- `internal/memory/manager.go` — dispatch methods (continued)
- `internal/memory/dispatcher.go` — extracted dispatch logic
- `internal/memory/manager_test.go` — dispatch tests (continued)

**RED Tests**:
- `TestManager_DispatchOrder_InitializeForward_SessionEndReverse` — AC-006
- `TestManager_ProviderPanicIsolated` — AC-007
- `TestManager_IsAvailableFalse_SkipsProvider` — AC-008
- `TestManager_OnTurnStart_DispatchBudget50ms` — AC-011
- `TestManager_OnPreCompress_Aggregation` — AC-018
- `TestManager_InitErrorSuppressesUntilNextSession` — AC-019
- `TestManager_OnPreCompress_EmptyStringNoWrap` — AC-022

**GREEN Strategy**: `dispatcher` struct with `dispatchSequential()`, `dispatchReverse()`, `dispatchWithTimeout()`. Per-session init state tracking in `map[string]map[string]bool` (sessionID -> providerName -> initialized). Panic recovery via `defer recover()` in goroutine wrapper.

**REFACTOR Notes**: Extract dispatcher into its own file for clarity. Use functional options for timeout configuration.

**Risk Area**: 50ms budget test is timing-sensitive. Use mock providers with controlled sleep durations. Consider `testify/assert.Eventually` or channel-based synchronization for flakiness prevention.

---

### Task 06: MemoryManager Tool Routing + Aggregation

**ID**: MEMORY-T06
**Priority**: P1
**Dependencies**: Task 05
**AC Coverage**: AC-009 (SystemPromptBlock aggregation), AC-010 (tool routing), AC-012 (QueuePrefetch async), AC-020 (messages slice copy)

**Description**: Implement `SystemPromptBlock()`, `HandleToolCall()`, `QueuePrefetch()`, `GetAllToolSchemas()`, `Prefetch()`. SystemPromptBlock concatenates provider outputs with blank line separator. HandleToolCall routes via toolIndex. QueuePrefetch spawns goroutine. Messages slice must be deep-copied before passing to providers.

**Planned Files**:
- `internal/memory/manager.go` — remaining methods (continued)
- `internal/memory/manager_test.go` — routing tests (continued)

**RED Tests**:
- `TestManager_SystemPromptBlock_Aggregation` — AC-009
- `TestManager_HandleToolCall_RoutesToCorrectProvider` — AC-010
- `TestManager_QueuePrefetch_IsAsync` — AC-012
- `TestManager_MessagesNotRetainedAfterHook` — AC-020

**GREEN Strategy**: SystemPromptBlock iterates providers in order, concatenates with "\n\n". HandleToolCall looks up toolIndex, calls provider. QueuePrefetch uses `go func()` with recover. Messages copied via `append([]Message(nil), messages...)`.

**REFACTOR Notes**: Messages copy should use a helper `copyMessages()`. QueuePrefetch completion tracking may need an observable side effect for testing.

**Risk Area**: AC-020 (messages copy verification) requires pointer comparison. Use `reflect.ValueOf(captured).Pointer()` to verify no retained reference. Test mutates original after call and verifies provider snapshot unchanged.

---

### Task 07: BuiltinProvider — SQLite FTS5 Core

**ID**: MEMORY-T07
**Priority**: P0
**Dependencies**: Task 02
**AC Coverage**: AC-005 (session isolation), AC-013 (FTS5 recall), AC-023 (FIFO eviction)

**Description**: Implement `BuiltinProvider` with SQLite FTS5 backend. Create facts table with FTS5 virtual table, sync triggers. Implement `Initialize()`, `Prefetch()`, `SyncTurn()`, `SaveFact()` (internal). Session isolation via `session_id` column. FIFO eviction when exceeding `max_rows`.

**Planned Files**:
- `internal/memory/builtin/builtin.go` — BuiltinProvider struct, required interface methods
- `internal/memory/builtin/sqlite.go` — Schema DDL, query functions, FTS5 integration
- `internal/memory/builtin/sqlite_test.go` — FTS5 and session isolation tests
- `internal/memory/builtin/export_test.go` — Test-only accessors

**RED Tests**:
- `TestBuiltin_FTS5_RecallByKeyword` — AC-013
- `TestBuiltin_SessionIsolation` — AC-005
- `TestBuiltin_FIFOEvictionOnMaxRows` — AC-023
- `TestBuiltin_Initialize_CreatesSchema` — schema creation
- `TestBuiltin_Prefetch_FuzzyMatch` — FTS5 porter stemming
- `TestBuiltin_SaveFact_UpsertByKey` — UNIQUE(session_id, key) constraint
- `TestBuiltin_DBLocked_StillOperational` — concurrent access

**GREEN Strategy**: Schema from SPEC section 6.3. DDL executed in `Initialize()`. Prefetch uses `facts_fts MATCH ?` with session_id filter. FIFO eviction: before INSERT, check row count; if >= max_rows, DELETE oldest. All writes in transaction with `defer tx.Rollback()`.

**REFACTOR Notes**: Extract SQL queries as package-level constants. Consider `sqlc` for type-safe queries in future iteration.

**Risk Area**: FTS5 query syntax requires escaping special characters (quotes, asterisks). Implement `sanitizeFTSQuery()` helper. Test with special characters in content.

---

### Task 08: BuiltinProvider — File Management

**ID**: MEMORY-T08
**Priority**: P1
**Dependencies**: Task 07
**AC Coverage**: AC-014 (USER.md read-only), AC-015 (MEMORY.md append)

**Description**: Implement `SystemPromptBlock()` (reads USER.md + recent MEMORY.md), `SyncTurn()` (appends to MEMORY.md), USER.md write protection. File permissions: MEMORY.md 0600, directory 0700. MEMORY.md format: `- [sessionID] content`.

**Planned Files**:
- `internal/memory/builtin/files.go` — MEMORY.md/USER.md read/write logic
- `internal/memory/builtin/files_test.go` — File management tests

**RED Tests**:
- `TestBuiltin_UserMdReadOnly_WritingReturnsError` — AC-014
- `TestBuiltin_UserMdReadOnly_ContentIncludedInPrompt` — AC-014
- `TestBuiltin_MemoryMd_AppendOnSyncTurn` — AC-015
- `TestBuiltin_SystemPromptBlock_Truncation` — 8KB limit for large files
- `TestBuiltin_MemoryMd_CreatesDirectoryIfMissing` — auto-create `~/.goose/memory/`

**GREEN Strategy**: `SystemPromptBlock()` reads USER.md if exists (returns content), reads MEMORY.md (last N KB), concatenates. `SyncTurn()` opens MEMORY.md with `os.O_APPEND|os.O_CREATE|os.O_WRONLY`, appends formatted line. Write to USER.md returns `ErrUserMdReadOnly`.

**REFACTOR Notes**: Consider extracting `fileProvider` sub-struct if file logic grows beyond 100 lines.

---

### Task 09: BuiltinProvider — Tool Schemas + Integration

**ID**: MEMORY-T09
**Priority**: P1
**Dependencies**: Task 08
**AC Coverage**: AC-016 (Builtin-only flow), AC-017 (IsAvailable no I/O)

**Description**: Implement `GetToolSchemas()` returning `memory_recall` and `memory_save` tool definitions. Verify `IsAvailable()` performs no network/file I/O. Full integration test with MemoryManager using only BuiltinProvider.

**Planned Files**:
- `internal/memory/builtin/tools.go` — Tool schema definitions
- `internal/memory/builtin/integration_test.go` — Full flow test

**RED Tests**:
- `TestBuiltin_GetToolSchemas_ReturnsExpectedTools` — memory_recall + memory_save
- `TestBuiltin_IsAvailable_NoIO` — AC-017 (no network/file I/O)
- `TestIntegration_BuiltinOnly_FullLifecycle` — AC-016
- `TestBuiltin_HandleToolCall_Recall` — tool call dispatch
- `TestBuiltin_HandleToolCall_Save` — tool call dispatch

**GREEN Strategy**: Tool schemas as JSON-serializable structs matching OpenAI tool format. `IsAvailable()` checks `db != nil` only (no I/O). Integration test: create manager, register builtin, run full lifecycle (Initialize, OnTurnStart, Prefetch, HandleToolCall, OnSessionEnd).

**REFACTOR Notes**: Tool parameter schemas should be constants, not constructed at each call.

---

### Task 10: Plugin Adapter + Registry

**ID**: MEMORY-T10
**Priority**: P2 (optional extensibility)
**Dependencies**: Task 04
**AC Coverage**: AC-021 (factory registry lookup)

**Description**: Implement `PluginProvider` adapter interface in `internal/memory/plugin/`, factory registry (`map[string]func(config any) (MemoryProvider, error)`), and `RegisterFactory()` for external plugin registration. `ErrUnknownPlugin` for unregistered names.

**Planned Files**:
- `internal/memory/plugin/adapter.go` — PluginProvider adapter
- `internal/memory/plugin/registry.go` — Factory map + RegisterFactory
- `internal/memory/plugin/registry_test.go` — Registry tests
- `internal/memory/plugin/README.md` — External plugin authoring guide

**RED Tests**:
- `TestPlugin_FactoryRegistry_KnownPlugin_Succeeds` — AC-021 case A
- `TestPlugin_FactoryRegistry_UnknownPlugin_ReturnsErrUnknownPlugin` — AC-021 case B
- `TestPlugin_RegisterFactory_DuplicateName_Panics` — safety check
- `TestPlugin_Adapter_SatisfiesMemoryProvider` — compile-time check

**GREEN Strategy**: Global `registry map[string]FactoryFunc` with `sync.RWMutex`. `Lookup(name string) (MemoryProvider, error)` creates instance via factory. Plugin adapter wraps external provider interface into MemoryProvider.

**REFACTOR Notes**: Consider `sync.Once` for registry initialization. README.md should document the plugin authoring contract.

---

## Dependency Graph

```
T01 (Types+Errors)
 ├── T02 (Provider Interface)
 │    ├── T04 (Manager Registration)
 │    │    ├── T05 (Manager Dispatch)
 │    │    │    └── T06 (Manager Routing)
 │    │    └── T10 (Plugin Registry)
 │    └── T07 (Builtin SQLite)
 │         └── T08 (Builtin Files)
 │              └── T09 (Builtin Tools+Integration)
 └── T03 (MemoryConfig)
      └── T04 (Manager Registration)
```

**Parallelization Opportunity**: T02 and T03 can proceed in parallel (both depend only on T01). T07 can start once T02 is complete, independent of T04-T06. T10 depends only on T04.

**Critical Path**: T01 -> T02 -> T04 -> T05 -> T06 (5 sequential tasks)

---

## Risks

### R1: FTS5 Query Escaping (Severity: MEDIUM)

**Problem**: FTS5 MATCH syntax treats `*`, `"`, `AND`, `OR`, `NOT` as operators. User content containing these characters will cause query errors or unexpected behavior.

**Mitigation**: Implement `sanitizeFTSQuery()` that wraps search terms in double quotes and escapes internal quotes. Test with content containing special characters in Task 07.

### R2: 50ms Dispatch Budget Test Flakiness (Severity: MEDIUM)

**Problem**: Timing-dependent tests (`OnTurnStart` budget) may fail on slow CI runners or under load.

**Mitigation**: Use generous margin (test allows up to 60ms for budget check). Mock provider uses channel-based synchronization instead of `time.Sleep` for deterministic timing. Consider `-race` flag compatibility.

### R3: Messages Slice Copy Verification (Severity: LOW-MEDIUM)

**Problem**: AC-020 requires proving no retained reference to `messages[]` after hook returns. This requires internal state inspection.

**Mitigation**: Use `export_test.go` to expose internal state for testing. `reflect.DeepEqual` for content comparison, `reflect.ValueOf().Pointer()` for reference comparison. Test mutates original after call to prove isolation.

---

## REQ-to-Task Traceability Matrix

| REQ | AC | Task |
|-----|-----|------|
| REQ-MEMORY-001 | AC-001 | T04 |
| REQ-MEMORY-002 | AC-002 | T04 |
| REQ-MEMORY-003 | AC-003 | T04 |
| REQ-MEMORY-004 | AC-017 | T09 |
| REQ-MEMORY-005 | AC-005 | T07 |
| REQ-MEMORY-006 | AC-006 | T05 |
| REQ-MEMORY-007 | AC-007 | T05 |
| REQ-MEMORY-008 | AC-006 | T05 |
| REQ-MEMORY-009 | AC-010 | T06 |
| REQ-MEMORY-010 | AC-009 | T06 |
| REQ-MEMORY-011 | AC-018 | T05 |
| REQ-MEMORY-012 | AC-008 | T05 |
| REQ-MEMORY-013 | AC-019 | T05 |
| REQ-MEMORY-014 | AC-011 | T05 |
| REQ-MEMORY-015 | AC-004 | T04 |
| REQ-MEMORY-016 | AC-014 | T08 |
| REQ-MEMORY-017 | AC-020 | T06 |
| REQ-MEMORY-018 | AC-012 | T06 |
| REQ-MEMORY-019 | AC-021 | T10 |
| REQ-MEMORY-020 | AC-022 | T05 |
| REQ-MEMORY-021 | AC-023 | T07 |

---

## TRUST 5 Compliance Plan

| Dimension | How Achieved |
|-----------|--------------|
| **T**ested | 85%+ coverage via TDD; all 23 ACs have RED tests first; `-race` flag on all tests |
| **R**eadable | English godoc on all exported symbols; `@MX:NOTE` annotations on complex dispatch logic; BaseProvider embed pattern for clarity |
| **U**nified | `golangci-lint` pass; consistent naming (`internal/memory/`, `internal/memory/builtin/`, `internal/memory/plugin/`); uniform error pattern |
| **S**ecured | File permissions 0600/0700; USER.md read-only enforcement; session_id isolation; panic guard on all dispatch; no credential logging |
| **T**rackable | Zap structured logging on all dispatch events `{provider, hook, duration}`; tool routing audit log; conventional commits with SPEC/REQ/AC trailers |

---

## Approximate Size Estimate

| Package | Estimated LoC | Files |
|---------|--------------|-------|
| `internal/memory/` | ~400 | 6 files (types, errors, provider, config, manager, dispatcher) |
| `internal/memory/` tests | ~600 | 2 files (manager_test, export_test) |
| `internal/memory/builtin/` | ~350 | 4 files (builtin, sqlite, files, tools) |
| `internal/memory/builtin/` tests | ~450 | 3 files (sqlite_test, files_test, integration_test) |
| `internal/memory/plugin/` | ~100 | 2 files (adapter, registry) |
| `internal/memory/plugin/` tests | ~100 | 1 file (registry_test) |
| **Total** | **~2000** | **18 files** |
