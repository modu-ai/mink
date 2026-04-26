package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoader_LoadPlugin_Minimal는 AC-PL-001을 통합 검증한다.
// 최소 manifest로 플러그인이 정상 로드되어야 한다.
func TestLoader_LoadPlugin_Minimal(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{"name":"mini","version":"1.0.0"}`)

	cfg := &PluginsYAML{
		Plugins: map[string]PluginConfig{
			"mini": {Enabled: true},
		},
	}
	loader := NewLoader(nil, cfg)
	inst, err := loader.LoadPlugin(SourceProject, dir)
	require.NoError(t, err)
	assert.Equal(t, PluginID("mini"), inst.ID)
	assert.Equal(t, "1.0.0", inst.Manifest.Version)
	assert.Equal(t, SourceProject, inst.Source)
	assert.Equal(t, dir, inst.BaseDir)
}

// TestLoader_LoadPlugin_ReservedName은 예약 이름 거부를 검증한다 (AC-PL-003).
func TestLoader_LoadPlugin_ReservedName(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{"name":"_evil","version":"1.0.0"}`)

	loader := NewLoader(nil, nil)
	_, err := loader.LoadPlugin(SourceProject, dir)
	var e ErrReservedPluginName
	require.ErrorAs(t, err, &e)
}

// TestLoader_LoadPlugin_UnknownHookEvent은 미지원 hook 이벤트 거부를 검증한다 (AC-PL-004).
func TestLoader_LoadPlugin_UnknownHookEvent(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{
		"name":"my-plugin",
		"version":"1.0.0",
		"hooks":{"FrobnicateStart":[{"hooks":[{"command":"echo hi"}]}]}
	}`)

	loader := NewLoader(nil, nil)
	_, err := loader.LoadPlugin(SourceProject, dir)
	var e ErrUnknownHookEvent
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "FrobnicateStart", e.Event)
}

// TestLoader_LoadPlugin_CredentialsInURI는 자격증명 URI 거부를 검증한다 (AC-PL-010).
func TestLoader_LoadPlugin_CredentialsInURI(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{
		"name":"my-plugin",
		"version":"1.0.0",
		"mcpServers":[{"name":"srv","uri":"https://user:secret@host.com/mcp"}]
	}`)

	loader := NewLoader(nil, nil)
	_, err := loader.LoadPlugin(SourceProject, dir)
	var e ErrCredentialsInURI
	require.ErrorAs(t, err, &e)
}

// TestLoader_LoadPlugin_WithSkillsAndAgents는 AC-PL-002를 부분 검증한다.
// skills와 agents 디렉토리가 있는 플러그인의 primitive 로드를 검증한다.
func TestLoader_LoadPlugin_WithSkillsAndAgents(t *testing.T) {
	dir := t.TempDir()

	// skills 디렉토리 및 SKILL.md 생성
	skillDir := filepath.Join(dir, "skills", "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), `---
name: my-skill
description: test skill
---
# My Skill`)

	// agents 디렉토리 및 agent.md 생성 (subagent 이름은 [a-zA-Z0-9_]만 허용)
	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
	writeFile(t, filepath.Join(agentDir, "myagent.md"), `---
agent_type: myagent
name: myagent
---
# My Agent`)

	writeManifest(t, dir, `{
		"name":"full-plugin",
		"version":"1.0.0",
		"skills":[{"id":"my-skill","path":"skills/my-skill"}],
		"agents":[{"id":"myagent","path":"agents/myagent.md"}]
	}`)

	loader := NewLoader(nil, nil)
	inst, err := loader.LoadPlugin(SourceProject, dir)
	require.NoError(t, err)
	assert.Equal(t, PluginID("full-plugin"), inst.ID)
	assert.NotEmpty(t, inst.LoadedSkills)
	assert.NotEmpty(t, inst.LoadedAgents)
}

// TestLoader_LoadPlugin_RollbackOnPartialFailure는 AC-PL-012를 검증한다.
// 두 번째 skill 파일이 손상되면 첫 번째 skill도 롤백되어야 한다.
func TestLoader_LoadPlugin_RollbackOnPartialFailure(t *testing.T) {
	dir := t.TempDir()

	// skill 1: 정상
	skill1Dir := filepath.Join(dir, "skills", "skill1")
	require.NoError(t, os.MkdirAll(skill1Dir, 0755))
	writeFile(t, filepath.Join(skill1Dir, "SKILL.md"), `---
name: skill1
description: first skill
---
# Skill 1`)

	// skill 2: 손상된 YAML frontmatter
	skill2Dir := filepath.Join(dir, "skills", "skill2")
	require.NoError(t, os.MkdirAll(skill2Dir, 0755))
	writeFile(t, filepath.Join(skill2Dir, "SKILL.md"), `---
: invalid: yaml: content
bad:::
---
# Skill 2`)

	writeManifest(t, dir, `{
		"name":"bad-plugin",
		"version":"1.0.0",
		"skills":[
			{"id":"skill1","path":"skills/skill1"},
			{"id":"skill2","path":"skills/skill2"}
		]
	}`)

	loader := NewLoader(nil, nil)
	_, err := loader.LoadPlugin(SourceProject, dir)
	// 두 번째 skill 로드 실패로 전체 로드가 실패해야 한다
	assert.Error(t, err)
}

// TestLoader_LoadPlugin_DisabledPlugin은 enabled=false인 플러그인은 로드 시 스킵되어야 함을 검증한다.
func TestLoader_LoadPlugin_DisabledPlugin(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{"name":"disabled","version":"1.0.0"}`)

	cfg := &PluginsYAML{
		Plugins: map[string]PluginConfig{
			"disabled": {Enabled: false},
		},
	}
	loader := NewLoader(nil, cfg)
	// enabled=false 이면 로드 자체를 스킵
	inst, err := loader.LoadPluginIfEnabled(SourceProject, dir)
	require.NoError(t, err)
	assert.Nil(t, inst, "disabled plugin should not be loaded")
}

// --- 헬퍼 함수 ---

// writeManifest는 테스트 디렉토리에 manifest.json을 쓴다.
func writeManifest(t *testing.T, dir, content string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "manifest.json"), content)
}

// writeFile은 지정된 경로에 파일을 쓴다.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}
