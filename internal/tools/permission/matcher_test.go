package permission_test

import (
	"encoding/json"
	"testing"

	"github.com/modu-ai/mink/internal/tools/permission"
	"github.com/stretchr/testify/assert"
)

func TestGlobMatcher_ToolNameOnly(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"Bash"}}

	approved, reason := m.Preapproved("Bash", json.RawMessage(`{"command":"ls"}`), cfg)
	assert.True(t, approved)
	assert.Contains(t, reason, "allowlist")
}

func TestGlobMatcher_ToolName_NoMatch(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"FileRead"}}

	approved, _ := m.Preapproved("Bash", json.RawMessage(`{"command":"ls"}`), cfg)
	assert.False(t, approved)
}

func TestGlobMatcher_WildcardToolName(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"mcp__github__*"}}

	approved, _ := m.Preapproved("mcp__github__create_issue", json.RawMessage(`{}`), cfg)
	assert.True(t, approved)

	approved2, _ := m.Preapproved("mcp__gitlab__create_issue", json.RawMessage(`{}`), cfg)
	assert.False(t, approved2)
}

func TestGlobMatcher_BashCommandPattern(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"Bash(git status)"}}

	// 정확 일치
	approved, _ := m.Preapproved("Bash", json.RawMessage(`{"command":"git status"}`), cfg)
	assert.True(t, approved)

	// 불일치
	notApproved, _ := m.Preapproved("Bash", json.RawMessage(`{"command":"rm -rf /"}`), cfg)
	assert.False(t, notApproved)
}

func TestGlobMatcher_BashCommandWildcard(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"Bash(git *)"}}

	tests := []struct {
		cmd      string
		expected bool
	}{
		{"git status", true},
		{"git commit -m 'msg'", true},
		{"git log", true},
		{"rm -rf /", false},
		{"git", false}, // "git *"은 "git " 접두 필요
	}

	for _, tt := range tests {
		input, _ := json.Marshal(map[string]any{"command": tt.cmd})
		approved, _ := m.Preapproved("Bash", input, cfg)
		assert.Equal(t, tt.expected, approved, "command: %q", tt.cmd)
	}
}

func TestGlobMatcher_FileReadPathPattern(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"FileRead(/tmp/**)"}}

	approved, _ := m.Preapproved("FileRead", json.RawMessage(`{"path":"/tmp/test.txt"}`), cfg)
	assert.True(t, approved)

	notApproved, _ := m.Preapproved("FileRead", json.RawMessage(`{"path":"/etc/passwd"}`), cfg)
	assert.False(t, notApproved)
}

func TestGlobMatcher_EmptyAllow(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{}

	approved, _ := m.Preapproved("Bash", json.RawMessage(`{"command":"ls"}`), cfg)
	assert.False(t, approved, "allow 목록이 비어 있으면 항상 false")
}

// TestGlobMatcher_MCPToolPrimaryField — MCP tool: 첫 string 필드 추출
func TestGlobMatcher_MCPToolPrimaryField(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"mcp__github__create_issue(bug-*)"}}

	// MCP tool의 primary field는 첫 string 값
	input, _ := json.Marshal(map[string]any{"title": "bug-report"})
	approved, _ := m.Preapproved("mcp__github__create_issue", input, cfg)
	assert.True(t, approved)
}

// TestGlobMatcher_PatternMissingCloseParen — 닫는 괄호 없는 패턴
func TestGlobMatcher_PatternMissingCloseParen(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"Bash(git status"}}

	// 닫는 괄호 없으면 false
	input, _ := json.Marshal(map[string]any{"command": "git status"})
	approved, _ := m.Preapproved("Bash", input, cfg)
	assert.False(t, approved)
}

// TestGlobMatcher_GrepPathPattern — Grep tool의 path 추출
func TestGlobMatcher_GrepPathPattern(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"Grep(/tmp/**)"}}

	input, _ := json.Marshal(map[string]any{"pattern": "hello", "path": "/tmp/search.txt"})
	approved, _ := m.Preapproved("Grep", input, cfg)
	assert.True(t, approved)

	input2, _ := json.Marshal(map[string]any{"pattern": "hello", "path": "/etc/passwd"})
	approved2, _ := m.Preapproved("Grep", input2, cfg)
	assert.False(t, approved2)
}

// TestGlobMatcher_EmptyInput — input이 비어있는 경우
func TestGlobMatcher_EmptyInput(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"Bash(git *)"}}

	// input이 nil이면 primary field 없음 → false
	approved, _ := m.Preapproved("Bash", json.RawMessage(`{}`), cfg)
	assert.False(t, approved)
}

// TestGlobMatcher_FileEditPattern — FileEdit tool의 path 추출
func TestGlobMatcher_FileEditPattern(t *testing.T) {
	m := &permission.GlobMatcher{}
	cfg := permission.Config{Allow: []string{"FileEdit(/workspace/**)"}}

	input, _ := json.Marshal(map[string]any{"path": "/workspace/src/main.go", "old_string": "x", "new_string": "y"})
	approved, _ := m.Preapproved("FileEdit", input, cfg)
	assert.True(t, approved)
}
