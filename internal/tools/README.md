# internal/tools

**Tool Execution System 패키지** — LLM tool 호출의 등록, 실행, 권한 관리

## 개요

본 패키지는 MINK의 **Tool 시스템**을 구현합니다. LLM이 생성한 `tool_use` 요청을 수신하여 해당 tool을 실행하고 결과를 LLM에게 반환합니다. Built-in tool, MCP tool, 검색 tool을 통합 관리하며 실행 예산(budget) 및 권한 검증을 수행합니다.

## 핵심 구성 요소

### Tool Registry

Tool 등록 및 조회:

```go
type Registry struct {
    mu    sync.RWMutex
    tools map[string]*ToolDefinition
}

func (r *Registry) Register(def *ToolDefinition) error
func (r *Registry) Lookup(name string) (*ToolDefinition, error)
func (r *Registry) ListByScope(scope Scope) []*ToolDefinition
```

### Tool Executor

`tool_use` 요청 실행:

```go
type Executor struct {
    registry  *Registry
    budgets   *BudgetManager
    permStore permission.Store
}

func (e *Executor) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
    // 1. Registry에서 tool 조회
    // 2. 권한 검증
    // 3. 예산 확인
    // 4. 실행
    // 5. 결과 반환
}
```

### Budget Manager

Tool 호출 예산 관리:

```go
type Budget struct {
    MaxCalls     int           // 최대 호출 횟수
    MaxTokens    int           // 최대 토큰 사용량
    Timeout      time.Duration // 호출 타임아웃
    TotalUsed    int           // 사용된 호출 횟수
}
```

### MCP Adapter

MCP 서버의 tool을 내부 tool로 변환:

```go
type MCPAdapter struct {
    client *mcp.Client
}

func (a *MCPAdapter) ListTools(ctx context.Context) ([]*ToolDefinition, error)
func (a *MCPAdapter) Execute(ctx context.Context, call ToolCall) (*ToolResult, error)
```

## 서브패키지

| 패키지 | 설명 |
|--------|------|
| `builtin/` | 내장 tool 구현 (Bash, Read, Write, Edit, Grep, Glob) |
| `mcp/` | MCP tool adapter |
| `naming/` | Tool name resolution 및 alias |
| `permission/` | Tool 실행 권한 관리 |
| `search/` | 검색 tool (codebase search, web search) |

## 권한 모델

Tool 실행 전 권한 검증:

```go
type Scope int

const (
    ScopeRead    Scope = iota // 읽기 전용 (Read, Grep, Glob)
    ScopeWrite                // 쓰기 (Write, Edit)
    ScopeExecute              // 실행 (Bash)
    ScopeNetwork              // 네트워크 (WebSearch, WebFetch)
)
```

## 파일 구조

```
internal/tools/
├── tool.go              # ToolDefinition, ToolCall, ToolResult
├── registry.go          # Tool 레지스트리
├── executor.go          # Tool 실행기
├── budget.go            # 예산 관리
├── inventory.go         # Tool 인벤토리
├── scope.go             # 권한 스코프
├── errors.go            # 에러 정의
├── mcp_adapter.go       # MCP tool adapter
├── builtin/             # 내장 tool
├── mcp/                 # MCP 연동
├── naming/              # 이름 해석
├── permission/          # 권한 관리
└── search/              # 검색 tool
```

## 관련 SPEC

- **SPEC-GOOSE-TOOLS-001**: 본 패키지의 주요 SPEC
- **SPEC-GOOSE-MCP-001**: MCP tool adapter 연동
- **SPEC-GOOSE-PERMISSION-001**: Tool 실행 권한

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-TOOLS-001
