// Package permissions는 tool 실행 권한 게이트 타입과 인터페이스를 정의한다.
// SPEC-GOOSE-QUERY-001 S0 T0.2
package permissions

// ToolPermissionContext는 CanUseTool.Check 호출 시 전달하는 컨텍스트 정보이다.
// REQ-QUERY-006 permission gate 호출 시그니처에 포함된다.
type ToolPermissionContext struct {
	// ToolUseID는 LLM 응답의 tool_use 블록 ID이다.
	ToolUseID string
	// ToolName은 호출 요청된 도구 이름이다.
	ToolName string
	// Input은 도구 입력 파라미터이다.
	Input map[string]any
	// Turn은 현재 loop turn 번호이다.
	Turn int
}
