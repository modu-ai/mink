# internal/subagent

**Sub-agent Management 패키지** — 격리된 에이전트 생애주기 관리

## 개요

본 패키지는 MINK의 **Sub-agent 시스템**을 구현합니다. Claude Code의 agent spawning 모델을 Go로 포팅하여, 메인 에이전트가 독립적인 하위 에이전트를 생성·실행·관리할 수 있게 합니다. 각 sub-agent는 자체 system prompt, context window, tool 권한을 가진 격리된 실행 환경에서 동작합니다.

## 핵심 구성 요소

### Identity

Sub-agent 식별 및 메타데이터:

```go
type Identity struct {
    ID          string            // agent 고유 식별자
    Name        string            // 표시 이름
    AgentType   string            // agent type (e.g., "expert-backend")
    Model       string            // 사용 모델
    SystemPrompt string           // agent별 system prompt
    Tools       []string          // 허용된 tool 목록
    Metadata    map[string]string // 추가 메타데이터
}
```

### Loader

에이전트 정의 로딩 (`.claude/agents/*.md`):

```go
type Loader struct {
    dirs []string // agent definition directories
}

func (l *Loader) Load(name string) (*Definition, error)
func (l *Loader) LoadAll() ([]*Definition, error)
```

### Runner

Sub-agent 실행 관리:

```go
type Runner struct {
    identity  *Identity
    memory    *Memory
    tools     *tool.Registry
}

func (r *Runner) Run(ctx context.Context, prompt string) (<-chan Event, error)
func (r *Runner) Resume(ctx context.Context, agentID string, prompt string) (<-chan Event, error)
```

### Memory

Sub-agent 컨텍스트 관리:

```go
type Memory struct {
    messages []message.Message
    budget   int // max context tokens
}

func (m *Memory) Append(msg message.Message) error
func (m *Memory) Compact() error
func (m *Memory) Snapshot() []message.Message
```

### Permission

Sub-agent 권한 격리:

```go
type Permission struct {
    allowedTools   map[string]bool
    allowedPaths   []string
    maxIterations  int
    mode           PermissionMode // plan, acceptEdits, bypassPermissions
}

func (p *Permission) Check(tool string, path string) error
```

### Resume

Sub-agent 재개 (이전 실행 상태 복원):

```go
func Resume(ctx context.Context, agentID string, prompt string) (<-chan Event, error)
```

## 생애주기

```
1. Load      → .claude/agents/ 에서 정의 로딩
2. Spawn     → Identity 생성, 격리 환경 초기화
3. Run       → prompt 전달, LLM 호출, tool 실행
4. Complete  → 결과 반환, 메모리 정리
5. (Resume)  → 이전 실행 상태에서 재개
```

## 파일 구조

```
internal/subagent/
├── identity.go          # Identity 구조체
├── loader.go            # Agent 정의 로더
├── run.go               # Sub-agent 실행기
├── memory.go            # 컨텍스트 메모리
├── permission.go        # 권한 격리
├── resume.go            # 실행 재개
└── *_test.go            # 테스트 (coverage 테스트 포함)
```

## 관련 SPEC

- **SPEC-GOOSE-SUBAGENT-001**: 본 패키지의 주요 SPEC
- **SPEC-GOOSE-SKILLS-001**: Forked skill 기반 agent prompt 구성
- **SPEC-GOOSE-PERMISSION-001**: 권한 모델

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-SUBAGENT-001
