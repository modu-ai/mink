# internal/command

**Slash Command System 패키지** — 내장 command 및 사용자 정의 slash command 파서 및 디스패처

## 개요

본 패키지는 AI.GOOSE의 **Slash Command 파서 및 디스패처**를 구현합니다. Claude Code `commands/` 디렉토리의 slash command 시스템을 Go로 포팅하여, 사용자가 입력한 `/help`, `/clear`, `/model <alias>`, `/compact [target]`, `/exit` 등 내장 command를 LLM 호출 없이 local로 처리합니다.

또한 `/moai`, `/agency`, `/custom` 등 사용자 정의 slash command를 YAML frontmatter 기반으로 `~/.goose/commands/` + `.goose/commands/` 에서 로드하며, `$ARGUMENTS`, `$N` (positional) 치환을 수행합니다.

## 핵심 기능

### Command 판정 및 디스패치

```go
func (p *Parser) Parse(input string) (*Command, error) {
    if !strings.HasPrefix(input, "/") {
        return nil, ErrNotACommand // LLM 전달
    }

    cmd, err := p.registry.Lookup(input)
    if err != nil {
        return nil, err
    }

    return cmd, nil
}

func (d *Dispatcher) Execute(ctx context.Context, cmd *Command) (*Result, error) {
    // local command 실행
    return cmd.Handler(ctx, cmd.Args)
}
```

### 내장 Command (Built-in Commands)

| Command | 기능 | 구현 |
|---------|------|------|
| `/help` | 도움말 출력 | `builtin/help.go` |
| `/clear` | 대화 기록 삭제 | `builtin/clear.go` |
| `/model <alias>` | 모델 전환 | `builtin/model.go` |
| `/compact [target]` | 컨텍스트 압축 | `builtin/compact.go` |
| `/exit` | 종료 | `builtin/exit.go` |

### 사용자 정의 Command (Custom Commands)

**YAML frontmatter 기반** 정의:

```yaml
---
name: /moai
description: MoAI workflow command
allowed_tools: Skill
---

# /moai command

Execute MoAI workflow with arguments.
```

로딩 경로:
1. `~/.goose/commands/*.md` (사용자 전역)
2. `.goose/commands/*.md` (프로젝트 로컬)

### 변수 치환

Command 본문에서 특수 변수 치환:

| 변수 | 설명 | 예시 |
|------|------|------|
| `$ARGUMENTS` | 전체 인수 문자열 | `/moai run SPEC-XXX` → "run SPEC-XXX" |
| `$0`, `$1`, `$2`... | positional 인수 | `/model $0` → `/model gpt-4o`에서 `$0`="gpt-4o" |
| `$N` | 인수 개수 | `/echo $1 $2`에서 `$N`=2 |

```go
func expandArguments(body string, args []string) string {
    result := body
    result = strings.ReplaceAll(result, "$ARGUMENTS", strings.Join(args, " "))
    result = strings.ReplaceAll(result, "$N", strconv.Itoa(len(args)))

    for i, arg := range args {
        result = strings.ReplaceAll(result, "$"+strconv.Itoa(i), arg)
    }

    return result
}
```

## 핵심 구성 요소

### Command 구조체

```go
type Command struct {
    Name        string           // command 이름 (예: "/help")
    Description string           // 설명
    Handler     HandlerFunc      // 실행 핸들러
    Category    CommandCategory  // builtin | custom | skill
    Frontmatter Frontmatter      // YAML frontmatter (custom command)
    Enabled     bool             // 활성화 여부
}

type HandlerFunc func(ctx context.Context, args []string) (*Result, error)
```

### CommandRegistry

```go
type CommandRegistry struct {
    mu      sync.RWMutex
    commands map[string]*Command  // name -> Command
}

func (r *CommandRegistry) Register(cmd *Command) error
func (r *CommandRegistry) Lookup(input string) (*Command, error)
func (r *CommandRegistry) List() []*Command
```

### Parser

```go
type Parser struct {
    registry *CommandRegistry
}

func (p *Parser) Parse(input string) (*Command, error) {
    // 1. slash prefix 확인
    // 2. command name 추출
    // 3. 인수 파싱
    // 4. registry lookup
}
```

### Dispatcher

```go
type Dispatcher struct {
    parser    *Parser
    registry  *CommandRegistry
    output    chan<- message.StreamEvent  // 응답 전달 채널
}

func (d *Dispatcher) Dispatch(ctx context.Context, input string) error {
    cmd, err := d.parser.Parse(input)
    if err != nil {
        return err
    }

    result, err := d.Execute(ctx, cmd)
    if err != nil {
        return err
    }

    // 결과를 StreamEvent로 변환하여 전달
    d.SendResult(result)
    return nil
}
```

## QUERY-001 연계

`QUERY-001`의 `processUserInput` 훅 포인트에서 command 판정:

```go
func (qe *QueryEngine) processUserInput(prompt string) (string, error) {
    // 1. command 판정
    cmd, err := qe.commandParser.Parse(prompt)
    if err != nil {
        if errors.Is(err, command.ErrNotACommand) {
            // LLM 전달
            return prompt, nil
        }
        return "", err
    }

    // 2. local command 실행
    result, err := qe.commandDispatcher.Execute(ctx, cmd)
    if err != nil {
        return "", err
    }

    // 3-a. prompt expansion command인 경우 원문 교체
    if result.Type == command.ResultTypePromptExpansion {
        return result.ExpandedPrompt, nil // QUERY-001로 전달
    }

    // 3-b. local 응답인 경우 SDK Message로 즉시 응답
    qe.SendSDKMessage(result.Message)
    return "", nil // LLM 호출 스킵
}
```

## Command 유형

### Local Response Command

LLM 호출 없이 즉시 응답:

```go
func handleHelp(ctx context.Context, args []string) (*command.Result, error) {
    return &command.Result{
        Type:    command.ResultTypeLocal,
        Message: "Available commands:\n- /help\n- /clear\n- /exit",
    }, nil
}
```

### Prompt Expansion Command

원문을 교체한 후 QUERY-001로 전달:

```go
func handleModel(ctx context.Context, args []string) (*command.Result, error) {
    if len(args) < 1 {
        return nil, ErrMissingArgument
    }

    alias := args[0]
    model := resolveModelAlias(alias)

    return &command.Result{
        Type:           command.ResultTypePromptExpansion,
        ExpandedPrompt: fmt.Sprintf("(System: Use model %s for all responses)", model),
    }, nil
}
```

## Skill-Backed Command

`SKILLS-001` 연계: skill을 command로 노출

```go
func (r *CommandRegistry) LoadSkillCommands(skillRegistry *skill.Registry) error {
    skills := skillRegistry.List()
    for _, sk := range skills {
        if sk.Frontmatter.Command != "" {
            cmd := &command.Command{
                Name:     sk.Frontmatter.Command,
                Category: command.CategorySkill,
                Handler:  skillCommandHandler(sk),
            }
            r.Register(cmd)
        }
    }
    return nil
}
```

## 테스트

### 단위 테스트

```bash
go test ./internal/command/...
```

현재 전체 커버리지: **91.2%**
- substitute: 100%
- custom: 90.3%
- parser: 96.4%

### 통합 테스트

```go
func TestCommandEndToEnd(t *testing.T) {
    // 1. parser 생성
    parser := command.NewParser()

    // 2. builtin command 등록
    registry := command.NewRegistry()
    registry.RegisterBuiltinCommands()

    // 3. /help command 파싱
    cmd, err := parser.Parse("/help")
    assert.NoError(t, err)
    assert.Equal(t, "/help", cmd.Name)

    // 4. 실행
    result, err := cmd.Handler(ctx, []string{})
    assert.NoError(t, err)
    assert.Equal(t, command.ResultTypeLocal, result.Type)
}
```

## 파일 구조

```
internal/command/
├── parser.go              # Command 파서
├── registry.go            # Command 레지스트리
├── dispatcher.go          # Command 디스패처
├── substitute/            # 변수 치환 (100% coverage)
│   └── substituter.go
├── builtin/               # 내장 command 구현
│   ├── help.go
│   ├── clear.go
│   ├── model.go
│   ├── compact.go
│   └── exit.go
├── custom/                # 사용자 정의 command 로더 (90.3% coverage)
│   ├── loader.go
│   └── parser.go
├── parser.go              # 전체 parser (96.4% coverage)
└── *_test.go              # 각 서브패키지 테스트
```

## 상호 의존성

본 패키지는 다음 SPEC와 통합됩니다:

- **QUERY-001**: `processUserInput` 훅 포인트 구현
- **SKILLS-001**: Skill-backed command 지원
- **TOOLS-001**: Tool 호출 command (예: `/tool search`)

## 관련 SPEC

- **SPEC-GOOSE-COMMAND-001**: 본 패키지의 주요 SPEC (P1, Phase 3)
- **SPEC-GOOSE-QUERY-001**: `processUserInput` 훅 포인트 정의
- **SPEC-GOOSE-SKILLS-001**: Skill-backed command 연계

---

Version: 0.1.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-COMMAND-001
