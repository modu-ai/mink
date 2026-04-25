// Package naming은 tool 이름 충돌 해결 규약을 제공한다.
// REQ-TOOLS-003, REQ-TOOLS-004
package naming

import (
	"regexp"
	"strings"
)

// BuiltinNames는 예약된 built-in tool 이름 집합이다.
// REQ-TOOLS-003: 이 이름은 MCP tool이 클레임할 수 없다.
var BuiltinNames = map[string]struct{}{
	"FileRead":  {},
	"FileWrite": {},
	"FileEdit":  {},
	"Glob":      {},
	"Grep":      {},
	"Bash":      {},
}

// serverIDPattern은 유효한 serverID 패턴이다.
// REQ-TOOLS-004: [a-z0-9_-]{1,64}
var serverIDPattern = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// MCPPrefix는 MCP tool 이름 접두사이다.
const MCPPrefix = "mcp__"

// MCPToolName은 serverID와 toolName으로 MCP tool canonical 이름을 생성한다.
// REQ-TOOLS-004
func MCPToolName(serverID, toolName string) string {
	return MCPPrefix + serverID + "__" + toolName
}

// ParseMCPToolName은 canonical MCP tool 이름을 분해한다.
// 반환: (serverID, toolName, ok)
func ParseMCPToolName(name string) (serverID, toolName string, ok bool) {
	if !strings.HasPrefix(name, MCPPrefix) {
		return "", "", false
	}
	rest := name[len(MCPPrefix):]
	idx := strings.Index(rest, "__")
	if idx < 0 {
		return "", "", false
	}
	return rest[:idx], rest[idx+2:], true
}

// IsValidServerID는 serverID가 규칙에 맞는지 확인한다.
// REQ-TOOLS-004: [a-z0-9_-]{1,64}
func IsValidServerID(serverID string) bool {
	return serverIDPattern.MatchString(serverID)
}

// IsReservedName은 이름이 built-in 예약어인지 확인한다.
// REQ-TOOLS-003
func IsReservedName(name string) bool {
	_, ok := BuiltinNames[name]
	return ok
}

// HasDoubleUnderscore는 tool 이름에 __ 가 포함되는지 확인한다.
// REQ-TOOLS-017: MCP tool 이름에 __ 포함 시 거부
func HasDoubleUnderscore(toolName string) bool {
	return strings.Contains(toolName, "__")
}
