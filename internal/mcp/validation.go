package mcp

import "strings"

// validateToolName은 tool 이름의 예약어 포함 여부를 검사한다.
// REQ-MCP-016: '/', ':', '__' 포함 시 ErrReservedToolName 반환
func validateToolName(name string) error {
	if strings.Contains(name, "/") ||
		strings.Contains(name, ":") ||
		strings.Contains(name, "__") {
		return ErrReservedToolName
	}
	return nil
}

// isProtocolVersionSupported는 프로토콜 버전이 지원되는지 확인한다.
// REQ-MCP-018
func isProtocolVersionSupported(version string) bool {
	for _, v := range SupportedProtocolVersions {
		if v == version {
			return true
		}
	}
	return false
}

// namespacedToolName은 tool 이름에 서버 네임스페이스를 적용한다.
// REQ-MCP-001: "mcp__{serverName}__{toolName}"
func namespacedToolName(serverName, toolName string) string {
	return "mcp__" + serverName + "__" + toolName
}
