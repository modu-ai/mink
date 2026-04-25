package permission

import (
	"encoding/json"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Matcher는 tool 사전 승인 인터페이스이다.
// REQ-TOOLS-018
type Matcher interface {
	// Preapproved는 tool 호출이 allowlist에 의해 사전 승인되는지 확인한다.
	// approved=true이면 CanUseTool.Check 호출을 bypass한다.
	Preapproved(toolName string, input json.RawMessage, cfg Config) (approved bool, reason string)
}

// GlobMatcher는 doublestar 기반 패턴 매처이다.
// 구문: "<ToolName>(<arg-pattern>)" 또는 "<ToolName>" (인자 unchecked)
type GlobMatcher struct{}

// Preapproved는 allowlist 패턴을 순서대로 검사하고 첫 일치 시 승인한다.
// REQ-TOOLS-018
func (g *GlobMatcher) Preapproved(toolName string, input json.RawMessage, cfg Config) (bool, string) {
	for _, pattern := range cfg.Allow {
		if matchPattern(pattern, toolName, input) {
			return true, "allowlist: " + pattern
		}
	}
	return false, ""
}

// matchPattern은 단일 패턴을 (toolName, input) 쌍에 대해 검사한다.
func matchPattern(pattern, toolName string, input json.RawMessage) bool {
	// "ToolName(arg-pattern)" 형식 파싱
	argIdx := strings.Index(pattern, "(")
	if argIdx < 0 {
		// 인자 없는 패턴: tool 이름만 glob 매칭
		matched, err := doublestar.Match(pattern, toolName)
		return err == nil && matched
	}

	// tool 이름 부분 검사
	namePattern := pattern[:argIdx]
	matched, err := doublestar.Match(namePattern, toolName)
	if err != nil || !matched {
		return false
	}

	// 인자 패턴 추출
	if !strings.HasSuffix(pattern, ")") {
		return false
	}
	argPattern := pattern[argIdx+1 : len(pattern)-1]

	// primary field 추출
	primaryVal := extractPrimaryField(toolName, input)
	if primaryVal == "" {
		// schema 힌트 없으면 false 반환 (안전 기본값)
		return false
	}

	matched, err = doublestar.Match(argPattern, primaryVal)
	return err == nil && matched
}

// extractPrimaryField는 tool 이름 기반으로 input JSON에서 primary field 값을 추출한다.
// REQ-TOOLS-018: Bash는 command, FileRead/FileWrite/FileEdit/Glob/Grep은 path,
// mcp tool은 첫 string 필드.
func extractPrimaryField(toolName string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}

	var m map[string]any
	if err := json.Unmarshal(input, &m); err != nil {
		return ""
	}

	// tool 이름별 primary field 선택
	var primaryKey string
	switch toolName {
	case "Bash":
		primaryKey = "command"
	case "FileRead", "FileWrite", "FileEdit", "Glob":
		primaryKey = "path"
	case "Grep":
		primaryKey = "path"
	default:
		// MCP tool: 첫 string 필드
		for _, v := range m {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	if v, ok := m[primaryKey]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
