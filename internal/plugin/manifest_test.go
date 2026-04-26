package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginManifest_Parse_Minimal는 AC-PL-001을 검증한다.
// 최소 필드(name, version)만 있는 manifest를 파싱하면 오류 없이 PluginManifest를 반환해야 한다.
func TestPluginManifest_Parse_Minimal(t *testing.T) {
	raw := `{"name":"mini","version":"1.0.0"}`
	var m PluginManifest
	err := json.Unmarshal([]byte(raw), &m)
	require.NoError(t, err)
	assert.Equal(t, "mini", m.Name)
	assert.Equal(t, "1.0.0", m.Version)
	assert.Empty(t, m.Skills)
	assert.Empty(t, m.Agents)
	assert.Empty(t, m.MCPServers)
	assert.Empty(t, m.Commands)
	assert.Empty(t, m.Hooks)
}

// TestPluginManifest_Parse_Full은 4-primitive를 모두 포함한 manifest 파싱을 검증한다.
func TestPluginManifest_Parse_Full(t *testing.T) {
	raw := `{
		"name": "full-plugin",
		"version": "2.1.3",
		"description": "테스트 플러그인",
		"skills": [{"id": "my-skill", "path": "skills/my-skill/SKILL.md"}],
		"agents": [{"id": "my-agent", "path": "agents/my-agent.md"}],
		"mcpServers": [{"name": "my-mcp", "transport": "stdio", "command": "my-cmd"}],
		"commands": [{"name": "my-cmd", "path": "commands/my-cmd.md"}],
		"hooks": {
			"SessionStart": [{"hooks": [{"command": "echo hello"}]}]
		},
		"permissions": ["network"]
	}`
	var m PluginManifest
	err := json.Unmarshal([]byte(raw), &m)
	require.NoError(t, err)
	assert.Equal(t, "full-plugin", m.Name)
	assert.Equal(t, "2.1.3", m.Version)
	assert.Len(t, m.Skills, 1)
	assert.Len(t, m.Agents, 1)
	assert.Len(t, m.MCPServers, 1)
	assert.Len(t, m.Commands, 1)
	assert.Contains(t, m.Hooks, "SessionStart")
	assert.Equal(t, []string{"network"}, m.Permissions)
}

// TestPluginManifest_ParseFile_ReadsManifestFromDisk는 파일 기반 manifest 읽기를 검증한다.
func TestPluginManifest_ParseFile_ReadsManifestFromDisk(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{"name":"disk-test","version":"0.1.0"}`)

	m, err := ParseManifestFile(dir)
	require.NoError(t, err)
	assert.Equal(t, "disk-test", m.Name)
	assert.Equal(t, "0.1.0", m.Version)
}

// TestPluginManifest_ParseFile_MissingFile은 manifest.json 없을 때 오류 반환을 검증한다.
func TestPluginManifest_ParseFile_MissingFile(t *testing.T) {
	_, err := ParseManifestFile(t.TempDir())
	assert.Error(t, err)
}

// TestPluginManifest_ParseFile_InvalidJSON은 JSON 파싱 실패를 검증한다.
func TestPluginManifest_ParseFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{invalid json}`)
	_, err := ParseManifestFile(dir)
	assert.Error(t, err)
}
