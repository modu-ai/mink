---
id: SPEC-GOOSE-COMMAND-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-04-21
author: manager-spec
priority: P1
issue_number: null
phase: 3
size: 소(S)
lifecycle: spec-anchored
labels: []
---

# SPEC-GOOSE-COMMAND-001 — Slash Command System (내장 + Custom, Skill 연계)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (Phase 3 신규, Claude Code `commands/` 패턴 + QUERY-001 `processUserInput` 훅 포인트) | manager-spec |

---

## 1. 개요 (Overview)

AI.GOOSE의 **Slash Command 파서 및 디스패처**를 정의한다. `SPEC-GOOSE-QUERY-001` §3.2는 `processUserInput(prompt)`를 "본 SPEC에서 noop (원문 그대로 반환). COMMAND-001이 확장"으로 위임한다. 본 SPEC이 그 훅 포인트를 채우며, Claude Code `commands/` 디렉토리의 slash command 시스템을 Go로 포팅한다.

본 SPEC 수락 시점에서:

- 사용자가 입력한 `/help`, `/clear`, `/model <alias>`, `/compact [target]`, `/exit` 등 **내장 command 최소 세트**가 LLM 호출 없이 local로 처리되고,
- `/moai`, `/agency`, `/custom` 등 **사용자 정의 slash command**가 YAML frontmatter 기반으로 `~/.goose/commands/` + `.goose/commands/` 에서 로드되며,
- `$ARGUMENTS`, `$N` (positional) 치환이 수행되고,
- Skill-backed command(SKILLS-001 연계)가 동일 디스패처를 통해 실행되며,
- QUERY-001의 `submitMessage(prompt)` 진입 전에 prompt가 slash command인지 판정하여 (a) local command는 `SDKMessage` 스트림으로 즉시 응답, (b) prompt expansion command는 원문을 교체한 뒤 QUERY-001로 전달.

본 SPEC은 **Command 타입 시스템, parser, registry, dispatcher**를 규정한다. Skill 로딩 실체(SKILLS-001), Tool 호출(TOOLS-001)은 인터페이스 경계만 정의.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- **UX 필수**: 사용자가 `/help`, `/clear`, `/exit` 없이 CLI를 쓰는 것은 현실적 불가. CLI-001(Phase 3)이 착수하려면 본 SPEC이 먼저 (또는 병행) 완성되어야 함.
- **QUERY-001 훅 포인트 해소**: QUERY-001 `processUserInput`의 passthrough stub을 제거하려면 command 판정 로직이 필요.
- **Skill 확장 경로**: SKILLS-001이 정의할 "user-invocable skill"은 slash command로 노출된다. 본 SPEC이 공통 디스패치 경로를 제공해야 SKILLS-001 작업이 동일 인프라를 재사용.
- **MoAI/Agency 명령 호환성**: ROADMAP의 목표는 Claude Code `.claude/commands/*.md`와 동형의 slash command를 GOOSE가 실행하는 것. `/moai run SPEC-XXX` 같은 기존 MoAI 워크플로우를 그대로 재활용하려면 동일 frontmatter 스키마를 지원해야 함.

### 2.2 상속 자산 (패턴 계승)

- **Claude Code TypeScript** (`./claude-code-source-map/`): `commands/` 디렉토리 구조, `processPromptSlashCommand`, YAML frontmatter (description/argument-hint/allowed-tools), `$ARGUMENTS` 치환. 언어 상이로 직접 포트 아님.
  - 근거 문서: `.moai/project/research/claude-primitives.md` §2.3 "Inline Skill" 처리 (`processPromptSlashCommand`로 전개, `!command + $ARG` 치환) — skill과 command가 같은 파이프라인 공유.
- **기존 MoAI 프로젝트** (`.claude/commands/moai.md`, `.claude/commands/agency.md` 등): 본 레포에 실존. frontmatter 예시의 source of truth.

### 2.3 범위 경계 (한 줄)

- **IN**: Command 타입/parser/registry/dispatcher, 내장 6~8종, custom command loader, argument 치환, Skill-backed command 실행 경로, QUERY-001 처리 훅.
- **OUT**: Skill 본체 로딩(→SKILLS-001), Tool 실행(→TOOLS-001), Plugin command(→PLUGIN-001), Interactive command UI(→CLI-001).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/command/` 루트 패키지.
   - `Command` interface (`Name()`, `Metadata()`, `Execute(ctx, args) (Result, error)`).
   - `Registry` (name → Command 매핑, 탐색 우선순위 규칙).
   - `Parser` (slash 라인 파싱, positional vs remainder 분리).
   - `Dispatcher` (Registry + Parser + context wiring).
   - `CommandArgs` (parsed arguments struct).
   - `Result` discriminated: `LocalReply{text}` | `PromptExpansion{new_prompt}` | `Exit{code}` | `Abort`.

2. `internal/command/builtin/` — 내장 command.
   - `/help [command]` — 전체 또는 특정 command 설명 (Registry walk).
   - `/clear` — 세션 messages 초기화 시그널 → `Result.PromptExpansion{new_prompt: "", clear: true}` 형태로 `SlashCommandContext.OnClear()` 콜백 호출.
   - `/exit` 또는 `/quit` — `Result.Exit{code: 0}`.
   - `/model <alias>` — 모델 교체 지시(실제 교체는 QUERY-001 다음 submitMessage 전에 적용).
   - `/compact [target_tokens]` — CONTEXT-001 compaction 강제 실행 요청.
   - `/status` — 현재 세션 상태(turn count, task budget remaining, model, cwd) local 출력.
   - `/version` — goose 버전 정보 local 출력.

3. `internal/command/parser/` — Slash command parsing.
   - `Parse(line string) (name string, rawArgs string, ok bool)` — 첫 토큰이 `/`로 시작하면 slash. quote/escape 처리는 rawArgs 그대로 반환 후 Command가 자체 해석(단순화).
   - `SplitArgs(rawArgs string, spec ArgSpec) (CommandArgs, error)` — positional vs `--flag` vs remainder.

4. `internal/command/custom/` — Custom command loader.
   - `~/.goose/commands/*.md` (user scope), `.goose/commands/*.md` (project scope) 스캔.
   - YAML frontmatter (claude-primitives §2.1 스킬 frontmatter 부분집합) 파싱:
     ```yaml
     ---
     name: moai
     description: MoAI workflow entry
     argument-hint: "plan|run|sync SPEC-ID"
     allowed-tools: ["*"]
     ---
     ```
   - 본문은 **prompt template**. `$ARGUMENTS`, `$1`, `$2` 치환 후 `Result.PromptExpansion{new_prompt: expanded}`.

5. `internal/command/substitute/` — 치환 엔진.
   - `$ARGUMENTS` → rawArgs 전체.
   - `$1`, `$2`, ... `$N` → positional args.
   - `$CWD`, `$GOOSE_HOME` → context values.
   - 이스케이프: `$$` → literal `$`.

6. Skill-backed command 경로 (SKILLS-001 연계).
   - SKILLS-001이 `user-invocable: true`로 선언된 skill에 대해 `SkillRegistry.AsCommand(skillID) command.Command`를 제공.
   - 본 SPEC의 `Registry`는 `RegisterProvider(func() []Command)` 훅으로 외부 provider(SKILLS-001) 등록 가능.
   - 이름 충돌: 내장 > custom > skill 순 precedence.

7. QUERY-001 통합 훅.
   - `func ProcessUserInput(ctx context.Context, input string, sctx SlashCommandContext) (ProcessedInput, error)` 공개 API.
   - `ProcessedInput` 결과:
     - `ProceedWithPrompt{prompt string}` — 일반 메시지 또는 확장된 prompt.
     - `LocalMessage{messages []SDKMessage}` — local command 응답, QUERY-001은 API 호출 없이 스트림에 직접 yield.
     - `Exit{code int}` — CLI-001이 process exit.
     - `Aborted` — 사용자 취소.

### 3.2 OUT OF SCOPE (명시적 제외)

- **Skill 본체 로딩 / L0-L3 progressive disclosure / conditional skill / remote skill**: SPEC-GOOSE-SKILLS-001.
- **Tool 직접 호출 파이프라인**: TOOLS-001. command가 내부적으로 tool을 호출할 수는 있으나, 본 SPEC은 경로 제공만.
- **Plugin-provided command**: PLUGIN-001. 본 SPEC의 `RegisterProvider`가 plugin 제공자를 수용할 준비만.
- **CLI readline / history / autocomplete**: CLI-001 TUI 책임.
- **Multi-line / heredoc input**: 향후 확장. Phase 3은 single-line slash command.
- **Command help auto-generation to Markdown file**: 문서 자동 생성은 후속.
- **`bash:readonly` 같은 allowed-tools 세분화 enforcement**: SKILLS-001과 겹치므로 SKILLS-001에서 통합 처리.
- **MCP prompt (resources/prompts/list)** 을 slash command로 변환: SKILLS-001 경로.

---

## 4. EARS 요구사항 (Requirements)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-CMD-001 [Ubiquitous]** — The `command.Parser.Parse` function **shall** treat an input line as a slash command if and only if its first non-whitespace character is `/` and the character immediately following `/` is an ASCII letter; otherwise it **shall** return `ok=false` and the line is treated as a plain LLM prompt.

**REQ-CMD-002 [Ubiquitous]** — The `command.Registry` **shall** enforce a name resolution precedence of `builtin > custom (project) > custom (user) > skill-provided`; on name collision, the higher-precedence entry wins and a WARN log records the shadowed source.

**REQ-CMD-003 [Ubiquitous]** — Command names **shall** be case-insensitive during lookup but the canonical form stored in the registry **shall** be lowercased; registration of a command whose name contains characters outside `[a-z0-9_-]` after lowercasing **shall** be rejected with `ErrInvalidCommandName`.

**REQ-CMD-004 [Ubiquitous]** — The `ProcessUserInput` API **shall** be invoked exactly once per user input line before QUERY-001 `SubmitMessage`; QUERY-001 **shall not** invoke it again for tool-generated user messages or compaction-generated messages.

### 4.2 Event-Driven (이벤트 기반)

**REQ-CMD-005 [Event-Driven]** — **When** a user submits a line beginning with `/<name>`, the Dispatcher **shall** (a) parse name and rawArgs, (b) resolve the command via `Registry.Resolve(lowerName)`, (c) if found, call `Command.Execute(ctx, args)` and return the `Result` to QUERY-001, (d) if not found, return `LocalReply{text: "unknown command: /<name>. Type /help to list."}` and log at INFO level.

**REQ-CMD-006 [Event-Driven]** — **When** a custom command's body contains `$ARGUMENTS`, the substitution engine **shall** replace it with the entire rawArgs string (trimmed of leading/trailing whitespace); when it contains `$1`, `$2`, ..., `$9`, each **shall** be replaced with the corresponding positional token (whitespace-separated); unreferenced positionals **shall** appear in the remainder accessible via `$ARGUMENTS`.

**REQ-CMD-007 [Event-Driven]** — **When** `/clear` is executed, the Dispatcher **shall** invoke `SlashCommandContext.OnClear()` (callback wired by QUERY-001) and return `Result.LocalReply{text: "conversation cleared"}`; QUERY-001 **shall** reset `State.Messages` to empty in response to the callback.

**REQ-CMD-008 [Event-Driven]** — **When** `/model <alias>` is executed with a valid alias resolvable by ROUTER-001 (via `SlashCommandContext.ResolveModelAlias(alias) (*ModelInfo, error)`), the Dispatcher **shall** return `Result.LocalReply{text: "model set to <resolved>"}` after calling `SlashCommandContext.OnModelChange(info)`; invalid alias **shall** return `LocalReply{text: "unknown model: <alias>"}`.

**REQ-CMD-009 [Event-Driven]** — **When** a custom command is loaded from a `.md` file with malformed YAML frontmatter, the loader **shall** skip the file, log ERROR with file path and YAML error, and continue loading remaining files (loader **shall not** abort).

**REQ-CMD-010 [Event-Driven]** — **When** `Registry.Reload()` is invoked (e.g., on filesystem watch event or explicit `/reload`), the registry **shall** atomically swap the custom command set; in-flight `Execute` calls **shall** complete against the pre-swap snapshot.

### 4.3 State-Driven (상태 기반)

**REQ-CMD-011 [State-Driven]** — **While** `SlashCommandContext.PlanModeActive == true` (future SUBAGENT-001 integration), commands declared with `mutates: true` in their Metadata **shall** return `LocalReply{text: "command '<name>' disabled in plan mode"}` without executing.

**REQ-CMD-012 [State-Driven]** — **While** `Registry.Loading == true` (custom command reload in progress), `Resolve` **shall** serve from the previous snapshot; callers **shall not** observe an empty registry at any time.

### 4.4 Unwanted Behavior (방지)

**REQ-CMD-013 [Unwanted]** — The Dispatcher **shall not** expand `$ARGUMENTS` recursively; expansion is single-pass — a custom command body that produces another `$ARGUMENTS` after expansion **shall** leave the inner literal untouched.

**REQ-CMD-014 [Unwanted]** — **If** a custom command body produces an expanded prompt exceeding `CommandConfig.MaxExpandedPromptBytes` (default 64 KiB), **then** the Dispatcher **shall** return `LocalReply{text: "expanded prompt exceeds size limit (<bytes>)"}` and **shall not** forward to QUERY-001.

**REQ-CMD-015 [Unwanted]** — The Parser **shall not** execute any shell or perform any IO during parsing; parsing is a pure string operation.

**REQ-CMD-016 [Unwanted]** — Custom command loader **shall not** follow symlinks to parent directories of the configured command roots (symlink escape prevention).

### 4.5 Optional (선택적)

**REQ-CMD-017 [Optional]** — **Where** `CommandConfig.CustomCommandRoots` contains additional paths beyond `~/.goose/commands/` and `.goose/commands/`, the loader **shall** scan each additional root with the same precedence rules (later-declared roots have higher precedence).

**REQ-CMD-018 [Optional]** — **Where** a Skill provider is registered via `Registry.RegisterProvider(skillAsCommandProvider)`, the Dispatcher **shall** consult the provider for commands not found in built-in or custom sets, enabling SKILLS-001 user-invocable skills to surface as slash commands.

**REQ-CMD-019 [Optional]** — **Where** a command's Metadata declares `argument-hint`, `/help <name>` **shall** include the hint in its output.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then. `internal/command/*_test.go` 로 변환 가능.

**AC-CMD-001 — `/help` 출력**
- **Given** 내장 command 7종만 등록된 Registry, custom 없음
- **When** `ProcessUserInput(ctx, "/help", sctx)` 호출
- **Then** 결과 타입 `LocalReply`, `text` 내에 `"help"`, `"clear"`, `"exit"`, `"model"`, `"compact"`, `"status"`, `"version"` 모두 포함, QUERY-001에 prompt는 전달되지 않음

**AC-CMD-002 — 일반 메시지는 passthrough**
- **Given** Registry 초기 상태
- **When** `ProcessUserInput(ctx, "hello world", sctx)`
- **Then** `ProceedWithPrompt{prompt: "hello world"}` 반환, 어떤 Command도 실행되지 않음

**AC-CMD-003 — Unknown command**
- **Given** Registry 초기 상태
- **When** `ProcessUserInput(ctx, "/nonexistent", sctx)`
- **Then** `LocalReply` 반환, `text` 내 `"unknown command: /nonexistent"`, INFO 레벨 로그 생성, exit code 아님

**AC-CMD-004 — Custom command 로드 + $ARGUMENTS 치환**
- **Given** 테스트 fixture `t.TempDir()/commands/greet.md`:
  ```
  ---
  name: greet
  description: greet user
  argument-hint: "<name>"
  ---
  Hello $ARGUMENTS, welcome to GOOSE.
  ```
- **When** Registry를 `WithCustomRoots(tmpDir)`로 구성 후 `ProcessUserInput(ctx, "/greet Alice", sctx)`
- **Then** `ProceedWithPrompt{prompt: "Hello Alice, welcome to GOOSE."}` 반환

**AC-CMD-005 — $N positional 치환**
- **Given** custom command body: `"First: $1, Second: $2, All: $ARGUMENTS"`
- **When** `ProcessUserInput(ctx, "/pos foo bar baz", sctx)`
- **Then** expanded prompt가 `"First: foo, Second: bar, All: foo bar baz"`

**AC-CMD-006 — `/clear` 콜백**
- **Given** mock `SlashCommandContext`의 `OnClear`가 호출 카운트 0
- **When** `ProcessUserInput(ctx, "/clear", sctx)`
- **Then** `OnClear` 정확히 1회 호출됨, Result는 `LocalReply` with text "conversation cleared"

**AC-CMD-007 — `/exit` 종료 신호**
- **Given** Registry 초기 상태
- **When** `ProcessUserInput(ctx, "/exit", sctx)`
- **Then** `Exit{code: 0}` 반환

**AC-CMD-008 — `/model <alias>` 유효**
- **Given** mock `sctx.ResolveModelAlias("gpt-4o")`가 `&ModelInfo{ID: "gpt-4o-2024-08-06"}` 반환
- **When** `ProcessUserInput(ctx, "/model gpt-4o", sctx)`
- **Then** `sctx.OnModelChange` 정확히 1회 호출됨 (받은 info가 resolved ID), `LocalReply.text` 포함 `"gpt-4o-2024-08-06"`

**AC-CMD-009 — `/model <alias>` 무효**
- **Given** `sctx.ResolveModelAlias`가 `nil, ErrUnknownModel` 반환
- **When** `ProcessUserInput(ctx, "/model xxx", sctx)`
- **Then** `LocalReply.text` 포함 `"unknown model: xxx"`, `OnModelChange` 호출되지 않음

**AC-CMD-010 — Malformed frontmatter skip**
- **Given** `t.TempDir()/commands/` 에 두 파일: `good.md`(정상) + `bad.md`(frontmatter YAML 파싱 실패)
- **When** Registry를 `WithCustomRoots(tmpDir)`로 빌드
- **Then** `bad` 이름으로 Resolve 실패, `good`은 성공, ERROR 로그에 `bad.md` 경로 포함

**AC-CMD-011 — Precedence: builtin > custom**
- **Given** custom command `name: help` (`.goose/commands/help.md`)
- **When** Registry 빌드 후 `Resolve("help")`
- **Then** 내장 `/help` Command가 반환되고 custom은 shadowed, WARN 로그 생성

**AC-CMD-012 — Recursive $ARGUMENTS 방지**
- **Given** custom command body 내 사용자 입력을 통해 다시 `$ARGUMENTS`가 문자열로 나타나는 경우: 사용자가 `/echo $ARGUMENTS` 입력, body는 `"Echo: $ARGUMENTS"`
- **When** `ProcessUserInput(ctx, "/echo $ARGUMENTS", sctx)`
- **Then** expanded prompt가 `"Echo: $ARGUMENTS"` (literal preserved, 재전개 없음)

**AC-CMD-013 — 크기 제한 초과 시 거부**
- **Given** `CommandConfig.MaxExpandedPromptBytes = 100`, custom command body가 expansion 후 200 bytes
- **When** `ProcessUserInput` with 해당 command
- **Then** `LocalReply.text` 포함 `"expanded prompt exceeds size limit"`, QUERY-001에 prompt 전달 없음

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/command/
├── command.go            # Command interface, Metadata, Result, Args
├── registry.go           # Registry struct + Register/Resolve/Reload/RegisterProvider
├── dispatcher.go         # Dispatcher.ProcessUserInput
├── context.go            # SlashCommandContext (QUERY-001 wiring)
├── errors.go             # ErrInvalidCommandName, ErrFrontmatterInvalid, ErrPromptTooLarge
├── command_test.go
│
├── parser/
│   ├── parser.go         # Parse(line), SplitArgs(raw, spec)
│   └── parser_test.go
│
├── substitute/
│   ├── substitute.go     # Expand($ARGUMENTS / $N / $CWD / $$)
│   └── substitute_test.go
│
├── custom/
│   ├── loader.go         # Walk roots, parse .md frontmatter
│   ├── frontmatter.go    # YAML parse + validation
│   ├── markdown.go       # body extraction
│   └── loader_test.go
│
└── builtin/
    ├── builtin.go        # Register()에서 전 7종 등록
    ├── help.go
    ├── clear.go
    ├── exit.go
    ├── model.go
    ├── compact.go
    ├── status.go
    ├── version.go
    └── *_test.go
```

### 6.2 핵심 타입 (Go 시그니처)

```go
// internal/command/command.go

// Command는 slash 명령의 추상 단위.
// builtin은 Go 함수 본문, custom은 .md 본문 template,
// skill-backed는 SKILLS-001이 wrap하여 제공.
type Command interface {
    Name() string               // 정규화 전 원 이름 (표시용)
    Metadata() Metadata
    Execute(ctx context.Context, args Args) (Result, error)
}

type Metadata struct {
    Description  string
    ArgumentHint string   // "plan|run|sync SPEC-ID"
    AllowedTools []string // future: SKILLS-001 연계
    Mutates      bool     // REQ-CMD-011 — plan mode에서 차단
    Source       Source   // SourceBuiltin|SourceCustomProject|SourceCustomUser|SourceSkill
    FilePath     string   // custom/skill일 때 원본 경로
}

type Args struct {
    RawArgs     string   // trim된 전체 인자 문자열
    Positional  []string // 공백 split (quotes 고려)
    Flags       map[string]string // --key=value 파싱 결과
    OriginalLine string   // 재전송 또는 로그용
}

// Result는 discriminated union.
type Result struct {
    Kind   ResultKind
    Text   string              // LocalReply
    Prompt string              // PromptExpansion
    Exit   int                 // Exit
    Meta   map[string]any      // 추가 이벤트(OnClear already called 등)
}

type ResultKind int
const (
    ResultLocalReply ResultKind = iota
    ResultPromptExpansion
    ResultExit
    ResultAbort
)


// internal/command/context.go

// SlashCommandContext는 QUERY-001이 Dispatcher에 주입하는 런타임 hook.
// Dispatcher는 이 hook을 통해 상위 상태에 쓰기 없이 이벤트만 통지.
type SlashCommandContext interface {
    // OnClear는 /clear command가 messages 초기화를 요청.
    OnClear() error
    
    // OnModelChange는 /model <alias> 성공 시 알림.
    OnModelChange(info ModelInfo) error
    
    // OnCompactRequest는 /compact 강제 compaction 요청.
    OnCompactRequest(target int) error
    
    // ResolveModelAlias는 alias → 실 모델 정보 변환 (ROUTER-001 노출).
    ResolveModelAlias(alias string) (*ModelInfo, error)
    
    // SessionSnapshot은 /status 출력용 현재 세션 상태 조회.
    SessionSnapshot() SessionSnapshot
    
    // PlanModeActive는 plan mode 여부 (SUBAGENT-001 noop if absent).
    PlanModeActive() bool
}


// internal/command/dispatcher.go

type Dispatcher struct {
    registry *Registry
    parser   *parser.Parser
    cfg      Config
    logger   *zap.Logger
}

type Config struct {
    MaxExpandedPromptBytes int64 // default 64*1024
    CustomCommandRoots     []string
}

type ProcessedInput struct {
    Kind     ProcessedKind
    Prompt   string           // ProceedWithPrompt
    Messages []SDKMessage     // LocalMessage — QUERY-001이 직접 yield
    ExitCode int              // Exit
}

type ProcessedKind int
const (
    ProcessProceed ProcessedKind = iota
    ProcessLocal
    ProcessExit
    ProcessAbort
)

// ProcessUserInput: QUERY-001 submitMessage 진입 전에 호출.
// REQ-CMD-004 참조.
func (d *Dispatcher) ProcessUserInput(
    ctx context.Context,
    input string,
    sctx SlashCommandContext,
) (ProcessedInput, error)


// internal/command/registry.go

type Registry struct {
    mu       sync.RWMutex
    builtin  map[string]Command // lowercased name
    project  map[string]Command
    user     map[string]Command
    provider []Provider          // SKILLS-001 연계
}

type Provider interface {
    Commands() []Command
}

func NewRegistry(opts ...Option) (*Registry, error)

func (r *Registry) Register(c Command, src Source) error
func (r *Registry) RegisterProvider(p Provider)
func (r *Registry) Resolve(name string) (Command, bool) // REQ-CMD-002 precedence
func (r *Registry) Reload(ctx context.Context) error    // REQ-CMD-010 atomic swap
func (r *Registry) List() []Metadata                    // /help 용


// internal/command/parser/parser.go

// Parse는 slash line 파싱. 첫 토큰이 "/<name>"이면 ok=true,
// 나머지는 rawArgs로 반환 (quote/escape는 SplitArgs에서 처리).
func Parse(line string) (name, rawArgs string, ok bool)

// SplitArgs는 rawArgs를 positional/flag로 분해.
// Shell-like quote 처리 (doublequote, singlequote, backslash escape).
func SplitArgs(rawArgs string) (Args, error)


// internal/command/substitute/substitute.go

type Context struct {
    Args Args
    Env  map[string]string // $CWD, $GOOSE_HOME 등
}

// Expand는 $ARGUMENTS / $1..$9 / $CWD / $$ → literal $ 치환.
// REQ-CMD-013: 단일 패스, 결과의 $... 은 재전개 없음.
func Expand(template string, ctx Context) (string, error)
```

### 6.3 Custom command frontmatter 스키마

```yaml
---
name: moai                         # required; 파일명 우선순위
description: MoAI workflow         # required
argument-hint: "plan|run SPEC-ID"  # optional
allowed-tools: ["FileRead", "FileWrite"]  # optional; SKILLS-001 통합
mutates: true                      # optional; plan mode 차단 여부
---
# Body (Markdown)
You are executing the MoAI $1 workflow for $2.

$ARGUMENTS
```

**필수 필드**: `name`, `description`. 나머지는 선택. 파싱 실패 시 REQ-CMD-009에 따라 skip.

Frontmatter YAML 파싱은 `gopkg.in/yaml.v3` 사용.

### 6.4 QUERY-001 통합 경로

QUERY-001 §3.2는 `processUserInput(prompt)`이 **본 SPEC 결과에 따라 분기**하는 계약을 가진다. 수정된 QUERY-001 동작:

```
submitMessage(prompt):
  processed, err := dispatcher.ProcessUserInput(ctx, prompt, e.sctx)
  switch processed.Kind {
    case ProcessProceed:
        // prompt 교체 후 기존 로직 계속
        append user Message(processed.Prompt)
        spawn queryLoop
    case ProcessLocal:
        // API call 없이 직접 yield
        for _, msg := range processed.Messages { out <- msg }
        out <- Terminal{success: true}
        close(out)
    case ProcessExit:
        out <- Terminal{success: true, error: "exit", exit_code: processed.ExitCode}
        close(out)
    case ProcessAbort:
        out <- Terminal{success: false, error: "aborted"}
        close(out)
  }
```

이 추가는 QUERY-001의 REQ-QUERY-005 (user_ack → stream_request_start) 흐름의 전단에 삽입된다. QUERY-001 `SubmitMessage`는 여전히 10ms 이내 반환(REQ-QUERY-016) — `ProcessUserInput`은 순수 함수 + mem lookup이라 빠름.

**주의**: `SlashCommandContext.OnClear/OnModelChange/OnCompactRequest`는 QUERY-001이 구현. 본 SPEC은 콜백 타입만 정의. QUERY-001 다음 버전(0.2.0)에서 이 hook을 추가한다 (QUERY-001 HISTORY에 기록).

### 6.5 내장 Command 최소 7종 명세

| Name | Description | Args | Result |
|------|-------------|------|--------|
| `/help` | 전체/특정 command 설명 | `[name]` | LocalReply(Registry.List 정렬 후 출력) |
| `/clear` | conversation 초기화 | - | LocalReply + `sctx.OnClear()` 호출 |
| `/exit` | process 종료 | - | Exit{0} |
| `/model <alias>` | 모델 교체 | 1 positional | LocalReply + `sctx.OnModelChange` |
| `/compact [target]` | compaction 강제 | 0-1 positional | LocalReply + `sctx.OnCompactRequest(target)` |
| `/status` | 세션 스냅샷 | - | LocalReply(SessionSnapshot format) |
| `/version` | 버전 | - | LocalReply |

별칭: `/quit` → `/exit`, `/?` → `/help` (Registry가 alias 테이블 유지).

### 6.6 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1**: `TestParser_DetectsSlash` — `/foo bar` → `("foo", "bar", true)`, `hello` → `("", "", false)`. REQ-CMD-001.
2. **RED #2**: `TestParser_RejectsNonLetterAfterSlash` — `//`, `/1`, `/ foo` → `ok=false`.
3. **RED #3**: `TestRegistry_Precedence_BuiltinWinsCustom` — AC-CMD-011.
4. **RED #4**: `TestDispatcher_Unknown_ReturnsLocalReply` — AC-CMD-003.
5. **RED #5**: `TestDispatcher_PlainPrompt_Proceeds` — AC-CMD-002.
6. **RED #6**: `TestBuiltinHelp_ListsAllCommands` — AC-CMD-001.
7. **RED #7**: `TestBuiltinClear_InvokesOnClear` — AC-CMD-006.
8. **RED #8**: `TestBuiltinExit_ReturnsExit0` — AC-CMD-007.
9. **RED #9**: `TestBuiltinModel_Valid_InvokesOnModelChange` — AC-CMD-008.
10. **RED #10**: `TestBuiltinModel_Invalid_NoOnModelChange` — AC-CMD-009.
11. **RED #11**: `TestSubstitute_Arguments` — AC-CMD-004.
12. **RED #12**: `TestSubstitute_Positional` — AC-CMD-005.
13. **RED #13**: `TestSubstitute_NoRecursion` — AC-CMD-012.
14. **RED #14**: `TestCustomLoader_MalformedSkipped` — AC-CMD-010.
15. **RED #15**: `TestDispatcher_MaxSizeExceeded` — AC-CMD-013.
16. **GREEN**: 최소 구현 — parser split, registry map + precedence, dispatcher wiring.
17. **REFACTOR**: substitute engine을 AST 기반으로 정리, custom loader를 goroutine pool로 병렬 스캔, Registry swap을 atomic.Value.

### 6.7 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **Tested** | 90%+ 커버리지, substitute는 100% (핵심 security 경계), `go test -race` 필수, table-driven |
| **Readable** | Command interface 3 메서드, Dispatcher는 ProcessUserInput 하나만 공개, builtin 파일당 50 LoC 이하 |
| **Unified** | `golangci-lint`, yaml.v3 통일, Command.Name()은 원문·canonical 분리 규약 |
| **Secured** | Parser IO 금지 (REQ-CMD-015), symlink escape 방지 (REQ-CMD-016), 크기 제한 (REQ-CMD-014), 재귀 치환 금지 (REQ-CMD-013) |
| **Trackable** | Unknown command INFO 로그, malformed frontmatter ERROR 로그 with file path, precedence shadowing WARN |

### 6.8 의존성 결정 (라이브러리)

| 라이브러리 | 버전 | 용도 | 근거 |
|----------|------|-----|-----|
| `gopkg.in/yaml.v3` | v3.0.1+ | Frontmatter 파싱 | 표준 Go YAML, 많은 프로젝트에서 검증 |
| `go.uber.org/zap` | v1.27+ | 구조화 로그 | CORE-001 계승 |
| 표준 `unicode`, `strings`, `regexp` | - | Parser | 외부 의존성 불필요 |
| `github.com/stretchr/testify` | v1.9+ | 테스트 | 기존 계승 |

**의도적 미사용**:
- `spf13/pflag` — Command.Args는 command-specific, 일괄 flag parser 불필요.
- `text/template` — $ARGUMENTS/$N은 단순 치환이므로 overkill. Custom substitute engine이 100 LoC 이하로 충분.

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger, context root |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `CommandConfig` (custom roots, max size) |
| 선행 SPEC | SPEC-GOOSE-QUERY-001 | `submitMessage` 훅 포인트, `SlashCommandContext` 구현측 |
| 후속 SPEC | SPEC-GOOSE-SKILLS-001 | `user-invocable` skill을 command로 노출 (Provider 인터페이스 구현) |
| 후속 SPEC | SPEC-GOOSE-CLI-001 | CLI readline / history / `goose ask "/help"` 경로 |
| 후속 SPEC | SPEC-GOOSE-ROUTER-001 | `ResolveModelAlias` 구현 (/model 명령) |
| 후속 SPEC | SPEC-GOOSE-CONTEXT-001 | `OnCompactRequest` 처리 (/compact 명령) |
| 후속 SPEC | SPEC-GOOSE-SUBAGENT-001 | `PlanModeActive` 힌트 (REQ-CMD-011) |
| 외부 | `gopkg.in/yaml.v3` | frontmatter |
| 외부 | Go 1.22+ | generics, unicode/utf8 |

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | `/` 로 시작하지만 의도는 경로 (`/etc/passwd를 읽어줘`) | 중 | 낮 | REQ-CMD-001: 첫 두 문자가 `/<letter>`일 때만. 경로는 일반적으로 `/etc`처럼 바로 letter 시작이지만 실사용 시 프롬프트 맥락 문자가 앞설 가능성 높음. 모호 시 LocalReply로 사용자에게 안내 |
| R2 | `$1` 치환 시 shell escape 미처리 (나중에 Bash tool로 전달) | 중 | 고 | 본 SPEC은 LLM prompt 치환만 수행. shell escape는 Bash tool(TOOLS-001 REQ-TOOLS-016)이 주입 지점에서 별도 처리. 문서화 필요 |
| R3 | Custom command 이름이 내장을 shadow하려 할 때 사용자 기대 어긋남 | 중 | 낮 | REQ-CMD-002 precedence + WARN 로그. `/help` 사용 시 shadowed 내역 표시 |
| R4 | 동시 `/reload` 호출 race | 낮 | 중 | REQ-CMD-012: 로딩 중 이전 snapshot 유지. `sync.Once`/`atomic.Pointer[map]` 활용 |
| R5 | Custom command 파일 수천 개 → 로드 지연 | 낮 | 중 | Phase 3은 순차 로드. 벤치마크 5ms/file 기준 500 file = 2.5s 수락. 성능 문제 시 goroutine pool 확장 |
| R6 | `$ARGUMENTS` 치환 결과가 LLM prompt injection vector | 고 | 고 | 본 SPEC은 **prompt expansion만** 담당. injection 방어는 LLM 호출 계층(ADAPTER-001/ROUTER-001)의 책임. 문서화하여 `/exec $ARGUMENTS` 같은 위험 패턴 경고 |
| R7 | SKILLS-001 Provider 등록 순서 의존성 | 중 | 중 | Provider는 lazy evaluation (Resolve 시점에 `provider.Commands()` 호출). precedence는 고정(builtin > custom > skill) |
| R8 | QUERY-001이 아직 SlashCommandContext를 구현하지 않음 | 고 | 고 | 본 SPEC은 인터페이스만. QUERY-001 다음 버전(0.2.0)에서 구현. 본 SPEC GREEN에서 mock SlashCommandContext로 테스트 가능 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/claude-primitives.md` §2.1 YAML frontmatter (공통 스키마 참조), §2.3 Inline Skill (processPromptSlashCommand + `!command + $ARG` 치환), §9 재사용 평가(80% 패턴 재사용)
- `.moai/project/research/claude-core.md` §3 인터페이스 (processUserInput 훅 위치), §6 설계원칙 6(continue sites explicit — command 처리는 continue site 밖)
- `.moai/project/structure.md` §455 `commands/` 디렉토리 구조
- `.moai/specs/ROADMAP.md` §4 Phase 3 row 17
- `.moai/specs/SPEC-GOOSE-QUERY-001/spec.md` §3.2 OUT (processUserInput passthrough), §5 AC-QUERY-001 (user_ack 흐름)
- `.moai/specs/SPEC-GOOSE-SKILLS-001/spec.md` (동시 작성 중 — `user-invocable` skill 노출 경로)

### 9.2 외부 참조

- Claude Code `commands/` 구조 (패턴만): https://docs.anthropic.com/en/docs/claude-code/
- `.claude/commands/moai.md`, `.claude/commands/agency.md` (본 레포 내 예시): 실존 frontmatter 참고
- gopkg.in/yaml.v3: https://pkg.go.dev/gopkg.in/yaml.v3

### 9.3 부속 문서

- `./research.md` — Claude Code command 패턴 상세, $ARGUMENTS 치환 구문 결정 근거, QUERY-001 훅 주입 시나리오
- `../ROADMAP.md` Phase 3 의존 그래프
- `../SPEC-GOOSE-TOOLS-001/spec.md` — Permission matcher 구문 (glob style 공유 디자인 결정)
- `../SPEC-GOOSE-CLI-001/spec.md` — CLI readline이 command line을 submitMessage로 전달

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **Skill 본체 로딩 / progressive disclosure / conditional skill / remote skill을 구현하지 않는다**. SPEC-GOOSE-SKILLS-001.
- 본 SPEC은 **Tool 실행 파이프라인을 구현하지 않는다**. Command가 내부적으로 tool을 호출할 수는 있으나 Dispatcher 레벨 지원만 제공. 실 실행은 TOOLS-001.
- 본 SPEC은 **Plugin-provided command를 패키징하지 않는다**. PLUGIN-001이 manifest.json에서 command를 로드하여 `Registry.RegisterProvider`로 주입.
- 본 SPEC은 **CLI readline / history / autocomplete를 구현하지 않는다**. CLI-001 TUI (bubbletea) 책임.
- 본 SPEC은 **Multi-line / heredoc input을 지원하지 않는다**. Phase 3은 single-line.
- 본 SPEC은 **`allowed-tools` enforcement를 수행하지 않는다**. frontmatter 파싱 및 metadata 보존만. 실제 tool 제한은 SKILLS-001/TOOLS-001 Permission matcher.
- 본 SPEC은 **MCP prompt (`prompts/list`)를 slash command로 변환하지 않는다**. SKILLS-001이 MCP prompt → skill 변환 후 user-invocable skill로 노출.
- 본 SPEC은 **Interactive confirmation / modal UI를 제공하지 않는다**. Command는 `LocalReply` 텍스트만 반환; UI는 CLI-001.
- 본 SPEC은 **Command help auto-generation to Markdown을 수행하지 않는다**. `/help` 출력만 제공.
- 본 SPEC은 **Recursive command expansion을 지원하지 않는다** (REQ-CMD-013). Custom command body 내 `$ARGUMENTS` 재전개 금지.

---

**End of SPEC-GOOSE-COMMAND-001**
