package naming_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/tools/naming"
	"github.com/stretchr/testify/assert"
)

func TestMCPToolName(t *testing.T) {
	assert.Equal(t, "mcp__github__create_issue", naming.MCPToolName("github", "create_issue"))
	assert.Equal(t, "mcp__foo__bar", naming.MCPToolName("foo", "bar"))
}

func TestParseMCPToolName(t *testing.T) {
	serverID, toolName, ok := naming.ParseMCPToolName("mcp__github__create_issue")
	assert.True(t, ok)
	assert.Equal(t, "github", serverID)
	assert.Equal(t, "create_issue", toolName)

	_, _, ok2 := naming.ParseMCPToolName("Bash")
	assert.False(t, ok2)

	_, _, ok3 := naming.ParseMCPToolName("mcp__onlyserver")
	assert.False(t, ok3)
}

func TestIsValidServerID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"github", true},
		{"foo-bar", true},
		{"foo_bar", true},
		{"foo123", true},
		{"UPPERCASE", false},
		{"", false},
		{"a", true},
		{string(make([]byte, 65)), false}, // 65자 초과
	}
	for _, tt := range tests {
		assert.Equal(t, tt.valid, naming.IsValidServerID(tt.id), "serverID: %q", tt.id)
	}
}

func TestIsReservedName(t *testing.T) {
	reserved := []string{"FileRead", "FileWrite", "FileEdit", "Glob", "Grep", "Bash"}
	for _, name := range reserved {
		assert.True(t, naming.IsReservedName(name), "should be reserved: %s", name)
	}
	assert.False(t, naming.IsReservedName("create_issue"))
	assert.False(t, naming.IsReservedName("MyTool"))
}

func TestHasDoubleUnderscore(t *testing.T) {
	assert.True(t, naming.HasDoubleUnderscore("bad__name"))
	assert.False(t, naming.HasDoubleUnderscore("good_name"))
	assert.False(t, naming.HasDoubleUnderscore("create_issue"))
}
