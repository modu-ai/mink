// Package permission은 tool 실행 사전 승인 계층을 제공한다.
// REQ-TOOLS-018: settings.json permissions.allow 패턴 매칭
package permission

// Config는 permission 설정이다.
type Config struct {
	// Allow는 사전 승인 패턴 목록이다.
	// 구문: "<ToolName>(<arg-pattern>)" 또는 "<ToolName>"
	// 예: "Bash(git *)", "FileRead(/tmp/**)", "mcp__github__*"
	Allow []string
	// Deny는 명시적 거부 패턴 목록이다.
	Deny []string
	// AdditionalDirectories는 cwd 외부에서 쓰기가 허용되는 경로 목록이다.
	// REQ-TOOLS-015 연동
	AdditionalDirectories []string
}
