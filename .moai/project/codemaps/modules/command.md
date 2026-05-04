# command 패키지 — Slash Command Dispatcher

**위치**: internal/command/  
**파일**: 17개 (dispatcher, registry, parser, builtin/, adapter/, custom/)  
**상태**: ✅ Active (SPEC-GOOSE-COMMAND-001)

---

## 목적

사용자 입력 처리 (slash 커맨드 vs 일반 텍스트). Command 레지스트리, 확장 시스템 (custom, builtin, adapter).

---

## 공개 API

### Dispatcher
```go
type Dispatcher struct {
    registry *Registry
    cfg      Config
}

// @MX:ANCHOR [AUTO] Core dispatch entry point
// @MX:REASON: Fan-in ≥3 (QUERY-001, test, integration)
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-004, REQ-CMD-005
func (d *Dispatcher) ProcessUserInput(
    ctx context.Context,
    input string,
    sctx SlashCommandContext,
) (ProcessedInput, error)

type ProcessedInput struct {
    Kind ProcessedKind  // ProcessProceed | ProcessLocal | ProcessExit | ProcessAbort
    Prompt string        // For ProcessProceed
    Messages []SDKMessage // For ProcessLocal
    ExitCode int          // For ProcessExit
}
```

### Registry
```go
type Registry struct {
    builtin   map[string]Command
    custom    map[string]Command
    adapter   map[string]Command  // Platform adapters
}

func (r *Registry) Resolve(name string) (Command, bool)
// 1. Check builtin (/help, /clear, /exit, /memory)
// 2. Check custom (from CustomCommandRoots)
// 3. Check adapter (platform-specific)
```

### Command Interface
```go
type Command interface {
    Name() string
    Help() string
    
    // REQ-CMD-011: Plan-mode check
    CanExecute(ctx SlashCommandContext) bool
    
    Execute(ctx context.Context, args []string) (Result, error)
}

type Result struct {
    Kind     ProcessedKind
    Output   string
    ExitCode int
}
```

---

## 처리 파이프라인 (REQ-CMD-004)

```
Step 1: Parse input
  ├─ Try regex: ^\s*/(\w+)\s*(.*)$
  ├─ If match: name, rawArgs
  └─ If no match: return ProcessProceed (prompt unchanged)

Step 2: Resolve command
  ├─ registry.Resolve(name)
  ├─ If found: command
  └─ If not found: return ProcessLocal (unknown command message)

Step 3: Plan-mode check (REQ-CMD-011)
  ├─ If mutating command (e.g., /clear, /exit) in plan mode
  │   └─ return ProcessLocal ("cannot mutate in plan mode")
  └─ Else: proceed

Step 4: Execute command
  ├─ command.Execute(ctx, args)
  ├─ Catch timeout (5s default)
  └─ Return result

Step 5: Prompt expansion size check (REQ-CMD-014)
  ├─ If ProcessProceed + expanded prompt > MaxExpandedPromptBytes
  │   └─ return ProcessLocal (size error)
  └─ Else: return as-is
```

---

## Builtin Commands

| Name | Kind | Effect |
|------|------|--------|
| `/help` | Meta | Show help |
| `/clear` | Mutating | Clear message history |
| `/memory list` | Read-only | List memories |
| `/memory save` | Mutating | Save memory |
| `/memory delete` | Mutating | Delete memory |
| `/exit` | Mutating | Exit CLI |

**구현**: internal/command/builtin/

---

## Custom Commands

### Discovery
```go
// Scan CustomCommandRoots (from Config)
// Look for *.md files with frontmatter:
// ---
// name: my-command
// help: "Description"
// execution: inline | subprocess
// ---
```

### Execution
```
Type: inline
  ├─ Parse content as prompt template
  ├─ Expand variables (${ARG0}, ${ARG1}, ...)
  └─ Return as prompt

Type: subprocess
  ├─ Execute shell script
  ├─ Capture stdout
  └─ Return as output
```

---

## Adapters (Platform-Specific)

### Location: internal/command/adapter/

```go
type Adapter interface {
    Name() string
    AvailableOn() []Platform  // macos, linux, windows
    Execute(ctx context.Context, args []string) (Result, error)
}

// Example: /browser-open (Tauri + desktop API)
// Example: /screenshot (Desktop automation)
// Example: /send-email (Native platform)
```

---

## Parser (internal/command/parser/)

```go
func Parse(input string) (name string, args string, ok bool)
// Regex: ^\s*/(\w+)(?:\s+(.*))?$
// Returns: (name, rawArgs, isCommand)
```

---

## @MX 주석

### @MX:ANCHOR
```go
// @MX:ANCHOR [AUTO] Primary entry point for all user input
// @MX:REASON: Called by QUERY-001 submitMessage on every line
// @MX:ANCHOR [AUTO] Core dispatch loop
// @MX:REASON: Fan-in ≥3 - all slash command flows through here
```

---

## SPEC 참조

| REQ | 내용 |
|-----|------|
| REQ-CMD-004 | Parse → Resolve → PlanMode → Execute → SizeCheck |
| REQ-CMD-005 | ProcessUserInput latency |
| REQ-CMD-011 | Plan-mode permission check |
| REQ-CMD-014 | PromptExpansion size limit (64 KiB default) |
| REQ-CMD-017 | CustomCommandRoots discovery |

---

**Version**: command v0.1.0  
**Generated**: 2026-05-04  
**LOC**: ~340  
**Builtin commands**: 6  
**@MX:ANCHOR Candidates**: 2 (Dispatcher, Registry.Resolve)
