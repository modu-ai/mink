// Package context는 QueryEngine의 context window 관리와 compaction 전략을 구현한다.
package context

// ToolPermissionContext는 tool 실행 권한 컨텍스트이다.
type ToolPermissionContext struct {
	// ToolName은 실행할 도구 이름이다.
	ToolName string
	// Input은 도구 입력 파라미터이다.
	Input map[string]any
}

// ToolUseContext는 iteration 마다 새로 생성되는 mutable 구조체이다.
// SPEC-GOOSE-CONTEXT-001 §6.2 tool_use.go
type ToolUseContext struct {
	// TurnIndex는 현재 iteration turn 번호이다.
	TurnIndex int
	// InvocationIDs는 이번 iteration에서 호출된 tool invocation ID 목록이다.
	InvocationIDs []string
	// ReadFiles는 이번 iteration에서 읽은 파일 경로 목록이다.
	ReadFiles []string
	// WrittenFiles는 이번 iteration에서 작성한 파일 경로 목록이다.
	WrittenFiles []string
	// PermissionCtx는 tool 실행 권한 컨텍스트이다.
	PermissionCtx ToolPermissionContext
}
