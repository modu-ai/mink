# SPEC-GENIE-COMMAND-001 — Research & Inheritance Analysis

> **목적**: Slash Command System 신규 작성을 위한 자산 조사 및 설계 결정. 내장 command 7종 선정, $ARGUMENTS/$N 치환 구문 확정, QUERY-001 hook 주입 지점 정의.
> **작성일**: 2026-04-21

---

## 1. 레포 상태

- `internal/command/` 부재. **신규 작성**.
- 본 레포 `.claude/commands/` 에 MoAI가 생성한 slash command 여러 개 존재 (`.claude/commands/moai.md`, `agency.md` 등) — 이 포맷이 사실상 source of truth 역할.

```
$ ls /Users/goos/MoAI/AgentOS/.claude/commands/ (예상)
agency.md  moai.md  etc.
```

이 파일들은 Claude Code `commands/` 표준을 따르며, 본 SPEC의 custom command loader가 동일 스키마를 지원해야 GENIE에서 재활용 가능하다.

---

## 2. 참조 자산별 분석

### 2.1 Claude Code — commands 시스템 (원문 인용)

`.moai/project/research/claude-primitives.md` §2.3:

```
### 2.3 Trigger 메커니즘 (4종)

1. **Inline Skill** (기본): processPromptSlashCommand로 전개, !command + $ARG 치환
```

`.moai/project/research/claude-primitives.md` §6 (Plugin 스키마):

```json
"commands": [{"name": "...", "description": "..."}]
```

**본 SPEC 계승**:
- `processPromptSlashCommand`는 **prompt expansion**이다 — slash command가 LLM prompt를 교체/확장. 본 SPEC의 `Result.PromptExpansion`이 동일 개념.
- `$ARGUMENTS` 치환은 Claude Code 사용자 친화성 유지 (동일 문법).
- Plugin manifest의 `commands` 필드 인식은 PLUGIN-001로 위임, 본 SPEC은 Provider 인터페이스만.

### 2.2 Claude Code — frontmatter 스키마 (§2.1 인용)

```yaml
---
name: custom-name                # 선택, 기본 디렉토리명
description: |                   # 선택, markdown
when-to-use: |                   # 선택, 모델 노출
argument-hint: "--flag value"
arguments: [arg1, arg2]
model: opus[1m]                  # 선택, "inherit" default
...
allowed-tools: [bash:readonly, read]
disable-model-invocation: false
user-invocable: true             # 모델 발견 가능
---
```

**본 SPEC에서의 subset 결정**:

| Frontmatter field | 본 SPEC에서 | 이유 |
|-------------------|-----------|------|
| `name` | ✅ 소비 | 필수 |
| `description` | ✅ 소비 | /help 출력에 필수 |
| `argument-hint` | ✅ 소비 | /help 풍부화 (REQ-CMD-019) |
| `allowed-tools` | ⚠️ 보존만 | SKILLS-001/TOOLS-001이 enforcement (본 SPEC은 metadata에 저장) |
| `when-to-use` | ❌ 무시 | SKILLS-001이 담당 (model invocation) |
| `arguments` | ❌ 무시 | Phase 3은 positional 우선, 명시적 spec은 후속 |
| `model` | ❌ 무시 | `/model` command가 별도, frontmatter의 model override는 SKILLS-001 |
| `effort`, `context`, `agent` | ❌ 무시 | SKILLS-001 영역 |
| `disable-model-invocation` | ❌ 무시 | SKILLS-001 모델 노출 제어 |
| `user-invocable` | ✅ 암묵적 true | slash command 파일은 본질적으로 user-invocable |
| `paths:` | ❌ 무시 | Conditional skill은 SKILLS-001 |
| `mutates` | ✅ 소비 | REQ-CMD-011 (plan mode 차단) |

### 2.3 본 레포 `.claude/commands/moai.md` (실존 참조)

CLAUDE.md 시스템 프롬프트에서 관찰:

```
### Unified Skill: /moai
Definition: Single entry point for all MoAI development workflows.
Subcommands: plan, run, sync, project, ...
```

실제 `.claude/commands/moai.md`는 아래와 같은 구조(CLAUDE.md §3 커맨드 레퍼런스와 정렬):

```yaml
---
description: MoAI unified workflow
argument-hint: "[subcommand] $ARGUMENTS"
allowed-tools: Skill
---
Use Skill("moai") with arguments: [subcommand] $ARGUMENTS
```

**본 SPEC 계승**:
- `[subcommand]`, `$ARGUMENTS` 토큰 동시 사용 — `$ARGUMENTS`는 **전체** 인자, `$1`은 첫 positional. 호환성 중요.
- Thin command pattern (coding-standards.md 참조): 커맨드 body는 최소, 실제 로직은 Skill에 위임. 본 SPEC의 Skill Provider 경로와 정렬.

### 2.4 Hermes 참조

Hermes Python은 slash command 개념 없음. CLI는 단일 REPL 루프로 대화. 본 SPEC은 **Hermes에서 참조할 자산 없음**.

### 2.5 MoAI-ADK-Go 참조

외부 레포 미러 없음. tech.md/structure.md가 `cobra`를 CLI 프레임워크로 가정하나, cobra는 **OS-level** CLI 명령(`genie version`, `genie ask`)에 사용. 본 SPEC의 slash command는 **대화 내 in-band command**로 별도 계층.

---

## 3. 내장 Command 7종 선정 근거

### 3.1 선정 기준

- **LLM 호출 불필요**: local-resolvable (/help, /status, /version 등).
- **세션 제어**: /clear, /compact, /model — QUERY-001 상태에 영향.
- **터미네이션**: /exit.

### 3.2 다른 CLI tool 비교

| Tool | `/help` | `/clear` | `/exit` | `/model` | `/compact` | `/status` | 기타 |
|------|--------|--------|--------|---------|----------|---------|-----|
| Claude Code | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | /memory, /resume, /debug 등 |
| Hermes | ❌ (argparse CLI) | ❌ | Ctrl-D | ❌ | ❌ | ❌ | - |
| Aider | ✅ (/help) | ✅ (/clear) | ✅ (/exit) | ✅ (/model) | ❌ | ❌ | /add, /run 등 |
| Continue.dev | ✅ | ✅ | - | - | - | - | - |

**Phase 3 선정 (7종)**: `/help`, `/clear`, `/exit`, `/model`, `/compact`, `/status`, `/version`. Claude Code 상위집합의 **최소 공통 코어**. `/memory`, `/resume`, `/debug`는 후속 SPEC (MEMORY-001, CLI-001, DEBUG-001).

### 3.3 alias 테이블

- `/quit` → `/exit` (vim 계열 습관)
- `/?` → `/help` (vi/emacs 관례)
- `/h` → `/help` (최소 타이핑)

Registry 내부 alias map으로 구현. precedence 계산 전 normalize.

---

## 4. 치환 엔진 설계

### 4.1 문법 결정

| 토큰 | 의미 | 소스 |
|------|------|------|
| `$ARGUMENTS` | 전체 raw args | Claude Code |
| `$1` ~ `$9` | positional 1-indexed | Claude Code |
| `$CWD` | `cfg.Cwd` | 본 SPEC 신규 |
| `$GENIE_HOME` | env `GENIE_HOME` | 본 SPEC 신규 |
| `$$` | literal `$` (escape) | 본 SPEC 신규 |

**거절**: `${VAR}` (shell style) — 혼동 유발. `$VAR` (env) — 보안 (임의 env 누수). 명시적 화이트리스트만.

### 4.2 Positional 파싱

Shell-like: quote 처리 + escape.

```
/greet "Alice Bob" Charlie \Delta
```

→ Positional: `["Alice Bob", "Charlie", "Delta"]` (Delta는 backslash escape로 그대로)

`$1="Alice Bob"` (공백 포함 문자열). 이 분리 로직이 `parser.SplitArgs`.

### 4.3 Non-recursion

REQ-CMD-013: 단일 패스. 이유:
- 사용자가 `/echo $ARGUMENTS` 입력 → body `"Echo: $ARGUMENTS"` → `$ARGUMENTS`가 `"$ARGUMENTS"` 리터럴로 치환 → 결과 `"Echo: $ARGUMENTS"`.
- 만약 재귀 전개한다면 `"Echo: Echo: ..."` 무한 loop 위험.
- 단일 패스는 구현 단순 + 보안.

### 4.4 `$$` 이스케이프

`$$ARGUMENTS` → `$ARGUMENTS` literal (no substitution). 이유: 드물지만 사용자가 `$ARGUMENTS` 자체를 문자열로 전달하고 싶을 때 필요.

---

## 5. Registry Precedence 결정 근거

### 5.1 4단계 precedence (REQ-CMD-002)

```
builtin > custom (project) > custom (user) > skill-provided
```

**이유**:
- **builtin 최상위**: `/help`, `/exit` 같은 핵심 명령이 사용자/스킬에 의해 깨지면 UX 붕괴. Claude Code와 동일 원칙.
- **project > user**: 현재 프로젝트의 `.genie/commands/`가 `~/.genie/commands/` 보다 우선. 프로젝트별 워크플로우 커스터마이징.
- **skill 최하**: SKILLS-001의 user-invocable skill은 명시적으로 다른 command를 override하지 않는다. 필요 시 다른 이름 사용.

### 5.2 Shadow 경고

precedence에 의해 무시된 엔트리는 WARN 로그:

```
WARN command_shadowed name=help shadowed_source=custom-user canonical=builtin
```

사용자가 의도치 않게 override 시도 시 조기 발견.

### 5.3 reload 원자성

`Registry.Reload(ctx)`는 새 snapshot 빌드 후 atomic swap:

```go
newBuiltin := ...    // unchanged
newProject := loadCustom(projectRoot)
newUser := loadCustom(userRoot)

r.mu.Lock()
r.project = newProject
r.user = newUser
r.mu.Unlock()
```

In-flight `Resolve` 호출은 RLock — 이전 snapshot 또는 새 snapshot 중 일관된 하나를 본다.

---

## 6. QUERY-001 훅 주입 전략

### 6.1 현재 QUERY-001 계약 (§3.2 OUT)

> Slash command 파싱: `processUserInput(prompt)`는 본 SPEC에서 noop (원문 그대로 반환). COMMAND-001이 확장.

### 6.2 본 SPEC 확장 후 QUERY-001 `submitMessage` 흐름

```
func (e *QueryEngine) SubmitMessage(ctx, prompt, opts...) (<-chan SDKMessage, error):
    out := make(chan SDKMessage)
    
    processed, err := e.dispatcher.ProcessUserInput(ctx, prompt, e.sctx)
    if err != nil { return nil, err }
    
    switch processed.Kind {
    case command.ProcessProceed:
        // 기존 경로
        state.Messages = append(state.Messages, userMsg(processed.Prompt))
        go e.queryLoop(ctx, state, out)
        return out, nil
    
    case command.ProcessLocal:
        go func() {
            defer close(out)
            for _, m := range processed.Messages { out <- m }
            out <- SDKMessage{Type: SDKMsgTerminal, Payload: Terminal{Success: true}}
        }()
        return out, nil
    
    case command.ProcessExit:
        go func() {
            defer close(out)
            out <- SDKMessage{Type: SDKMsgTerminal, Payload: Terminal{
                Success: true, Error: "exit", ExitCode: processed.ExitCode,
            }}
        }()
        return out, nil
    }
```

**QUERY-001 다음 버전(0.2.0)** 에서 `e.dispatcher`, `e.sctx` 필드 추가 및 REQ-QUERY-005 재기술 필요. 본 SPEC의 §6.4는 이 변경을 기대.

### 6.3 10ms 반환 시간 (QUERY-001 REQ-QUERY-016)

`ProcessUserInput`은:
- Parser.Parse: 마이크로초 (문자열 스캔)
- Registry.Resolve: 마이크로초 (map lookup)
- Command.Execute: builtin은 모두 mem-resident, custom은 이미 로드된 template 치환 (마이크로초)

10ms 여유 내 수행. filesystem 스캔은 Registry 초기화 시점에만 발생 — 초기화는 daemon bootstrap의 일부.

---

## 7. Go 이디엄

### 7.1 Command interface 간결성

```go
type Command interface {
    Name() string
    Metadata() Metadata
    Execute(ctx context.Context, args Args) (Result, error)
}
```

3 메서드. Skill 기반 복잡성은 Metadata에 담고, 실행은 단일 Execute. Dispatcher가 pre/post 처리.

### 7.2 Result discriminated union

Go에는 discriminated union이 없어 `Kind + 다중 필드` 패턴 사용. QUERY-001 `SDKMessage` 와 동일 전략.

대안으로 interface:
```go
type Result interface { isResult() }
type LocalReply struct{ Text string; meta ... }
func (LocalReply) isResult() {}
```

하지만 type switch 필요 + zero-value 안전성 떨어짐. struct + enum 채택.

### 7.3 Custom loader 병렬 최적화 (REFACTOR 단계)

```go
paths := scanRoot(root) // []string
results := make(chan custom.Command, len(paths))
var wg sync.WaitGroup
for _, p := range paths {
    wg.Add(1)
    go func(path string) {
        defer wg.Done()
        cmd, err := loadFile(path)
        if err != nil { logger.Error(...); return }
        results <- cmd
    }(p)
}
go func() { wg.Wait(); close(results) }()
```

1000 파일 기준 goroutine 오버헤드 무시할 수준, I/O bound이므로 병렬 효율 높음.

### 7.4 Frontmatter 파싱

```go
func parseMarkdown(data []byte) (frontmatter, body []byte, err error) {
    if !bytes.HasPrefix(data, []byte("---\n")) {
        return nil, data, nil // frontmatter optional
    }
    end := bytes.Index(data[4:], []byte("\n---\n"))
    if end < 0 { return nil, nil, ErrFrontmatterUnterminated }
    return data[4 : 4+end], data[4+end+5:], nil
}

var fm struct {
    Name        string   `yaml:"name"`
    Description string   `yaml:"description"`
    Hint        string   `yaml:"argument-hint"`
    AllowedTools []string `yaml:"allowed-tools"`
    Mutates     bool     `yaml:"mutates"`
}
yaml.Unmarshal(fmBytes, &fm)
```

`yaml.v3`가 tag 기반 매핑 제공. strict mode(`KnownFields(true)`)로 unknown field 감지 + WARN.

---

## 8. 테스트 전략 (TDD RED-first)

### 8.1 Unit 테스트

**Parser** (`parser/parser_test.go`):
- `TestParse_NotSlash_ReturnsOkFalse` — `"hello"`, `"/"`, `"/1"`, `"/ foo"`
- `TestParse_BasicSlash` — `"/help"` → `("help", "", true)`
- `TestParse_WithArgs` — `"/foo bar baz"` → `("foo", "bar baz", true)`
- `TestParse_LeadingWhitespace` — `"  /foo"` → `("foo", "", true)`
- `TestSplitArgs_Quotes` — `'Alice "Bob Charlie"'` → `["Alice", "Bob Charlie"]`

**Substitute** (`substitute/substitute_test.go`):
- `TestExpand_Arguments` — AC-CMD-004
- `TestExpand_Positional` — AC-CMD-005
- `TestExpand_NoRecursion` — AC-CMD-012
- `TestExpand_DollarDollarEscape` — `"$$ARGS"` → `"$ARGS"`
- `TestExpand_Env` — `$CWD`, `$GENIE_HOME`
- `TestExpand_UndefinedPositional` — `$5` 없을 때 → `""` literal

**Registry** (`registry_test.go`):
- `TestRegistry_Register_Duplicate_ReturnsErr`
- `TestRegistry_Resolve_CaseInsensitive` — `/HELP`, `/help` 모두 같은 command
- `TestRegistry_Precedence` — AC-CMD-011
- `TestRegistry_Reload_Atomic`

**Dispatcher** (`dispatcher_test.go`):
- AC-CMD-001 ~ 013 전체를 dispatcher 통합 테스트로.

**Custom Loader** (`custom/loader_test.go`):
- `TestLoader_Walk_IgnoresNonMd`
- `TestLoader_MalformedYaml_Skipped` — AC-CMD-010
- `TestLoader_SymlinkEscape_Rejected` — REQ-CMD-016

**Builtin 7종** (`builtin/*_test.go`):
- 각 command의 Execute 결과 검증 + sctx 콜백 호출 카운트 확인.

### 8.2 Integration 테스트

- `tests/integration/command/e2e_test.go`: 실 `QueryEngine` stub + Dispatcher 연결 → `submitMessage("/help")` → `SDKMessage` stream에 LocalReply 포함.
- `tests/integration/command/reload_test.go`: fsnotify 없이도 수동 `Reload()` 호출로 custom 추가/제거 반영 확인.

### 8.3 커버리지

- `internal/command/`: 95%+
- `internal/command/substitute/`: 100% (보안 경계)
- `internal/command/parser/`: 100% (보안 경계)
- `internal/command/builtin/`: 90%+

---

## 9. 외부 라이브러리 결정

| 라이브러리 | 용도 | 채택 | 근거 |
|----------|------|-----|-----|
| `gopkg.in/yaml.v3` | Frontmatter 파싱 | ✅ | 표준, tag 매핑, strict mode |
| 표준 `strings`, `unicode/utf8` | Parser/Substitute | ✅ | 외부 의존 불필요 |
| `go.uber.org/zap` | 로깅 | ✅ | CORE-001 계승 |
| `fsnotify/fsnotify` | custom reload 자동화 | ❌ | Phase 3 OUT. Manual `/reload`만 |
| `github.com/adrg/frontmatter` | frontmatter + body 파싱 | ❌ | 직접 구현 (~20 LoC) + yaml.v3로 충분 |
| `text/template` / `html/template` | 치환 | ❌ | 단순 `$ARGUMENTS`에 overkill, 보안 경계 축소 이점 |

---

## 10. 오픈 이슈

1. **Alias 우선순위**: `/quit` → `/exit`의 alias가 builtin 선등록일 때 custom `/quit` 가능한지. 현재 답: alias도 builtin precedence 유지. 사용자가 custom으로 `/quit` 덮어쓸 수 없음 → 문서화.
2. **`$GENIE_HOME` 노출**: env 변수 치환을 환영할지 보안 위험으로 볼지. 현재는 명시 화이트리스트만(`$CWD`, `$GENIE_HOME`). 일반 env는 거절.
3. **Dispatcher의 Context timeout**: Custom command가 body만 교체하는 순수 함수면 타임아웃 불필요. Skill-backed command가 외부 호출 시 별도 timeout은 SKILLS-001 구현 책임.
4. **Multi-step command** (`/moai plan` 후 사용자 입력 대기): 현재는 단일 `ProcessUserInput` 호출로 종료. 상태있는 command는 Skill-backed로 우회 — 본 SPEC의 단순성 유지.
5. **`/help` 출력 포맷**: plain text vs Markdown vs 테이블. CLI-001 `--format json` 대응도 고려. 현재: plain text, CLI-001이 후처리.
6. **Registry initialization timing**: `NewRegistry`가 custom 로딩까지 수행하면 bootstrap 시간 증가. 대안: lazy load (첫 Resolve 시점). 현재는 eager (구조적 단순성).

---

## 11. 결론

- **이식 자산**: 40% (Claude Code frontmatter + $ARGUMENTS 치환 + precedence 규칙). 60% 신규(Go interface, Registry provider 모델, SlashCommandContext hook).
- **참조 자산**: claude-primitives.md §2.1/§2.3 / 본 레포 `.claude/commands/*.md` 예시.
- **기술 스택**: `yaml.v3` + 표준 lib. 외부 의존 최소.
- **구현 규모 예상**: ~1,000 ~ 1,500 LoC (테스트 포함 ~2,500). ROADMAP §9 Phase 3 Go LoC 2,000 내 (TOOLS-001 + COMMAND-001 + CLI-001 합계).
- **주요 리스크**: QUERY-001 훅 미구현 (R8 — 본 SPEC과 QUERY-001 0.2.0 동기화 필요), prompt injection (R6 — ADAPTER 계층 책임 문서화).

GREEN 완료 시점에서:
- `genie` CLI에서 `/help`, `/clear`, `/exit`, `/model`, `/compact`, `/status`, `/version` 동작
- `.genie/commands/moai.md` 로드 → `/moai plan SPEC-XXX` → prompt expansion → QUERY-001로 전달 → LLM 호출
- QUERY-001 AC-QUERY-001 (user_ack 흐름) 이전에 slash command 판정 수행

Phase 3 MVP(`genie ask "hello"` → LLM 응답 + `/help` 로컬 응답)가 CLI-001과 함께 성립.

---

**End of research.md**
