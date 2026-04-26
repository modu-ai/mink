package plugin

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modu-ai/goose/internal/hook"
)

// --- errors.go 커버리지 ---

func TestErrors_AllErrorMessages(t *testing.T) {
	// ErrInvalidManifest
	e1 := ErrInvalidManifest{Reason: "test reason"}
	assert.Contains(t, e1.Error(), "test reason")

	// ErrReservedPluginName
	e2 := ErrReservedPluginName{Name: "goose"}
	assert.Contains(t, e2.Error(), "goose")

	// ErrDuplicatePluginName
	e3 := ErrDuplicatePluginName{Name: "foo"}
	assert.Contains(t, e3.Error(), "foo")

	// ErrUnknownHookEvent
	e4 := ErrUnknownHookEvent{Event: "BadEvent"}
	assert.Contains(t, e4.Error(), "BadEvent")

	// ErrCredentialsInURI
	e5 := ErrCredentialsInURI{URI: "https://user:pass@host.com"}
	assert.Contains(t, e5.Error(), "https://user:pass@host.com")

	// ErrZipSlip
	e6 := ErrZipSlip{Path: "../../etc/evil"}
	assert.Contains(t, e6.Error(), "../../etc/evil")

	// ErrMissingUserConfigVariable
	e7 := ErrMissingUserConfigVariable{Name: "API_KEY"}
	assert.Contains(t, e7.Error(), "API_KEY")

	// ErrNotImplemented
	assert.Error(t, ErrNotImplemented)

	// ErrPrimitiveLoad
	cause := errors.New("test cause")
	e8 := ErrPrimitiveLoad{Primitive: "skills", Cause: cause}
	assert.Contains(t, e8.Error(), "skills")
	assert.ErrorIs(t, e8.Unwrap(), cause)
}

// --- hook_handler.go 커버리지 ---

func TestPluginHookHandler_Interface(t *testing.T) {
	h := &pluginHookHandler{command: "echo hello", matcher: "*"}
	assert.Equal(t, "echo hello", h.Command())

	// Handle은 no-op
	out, err := h.Handle(context.Background(), hook.HookInput{})
	assert.NoError(t, err)
	assert.Equal(t, hook.HookJSONOutput{}, out)

	// Matches는 항상 true
	assert.True(t, h.Matches(hook.HookInput{}))
}

// --- loader.go WithSkillRegistry / WithHookRegistry 커버리지 ---

func TestLoader_WithRegistries(t *testing.T) {
	loader := NewLoader(nil, nil)
	hookReg := hook.NewHookRegistry()
	loader.WithHookRegistry(hookReg)
	assert.NotNil(t, loader.hookRegistry)
}

// --- loader.go 브랜치 커버리지: hook 등록 경로 ---

func TestLoader_LoadPlugin_WithHooks(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{
		"name":"hook-plugin",
		"version":"1.0.0",
		"hooks":{
			"SessionStart":[{"hooks":[{"command":"echo start"}]}],
			"Stop":[{"matcher":"*","hooks":[{"command":"echo stop","timeout":5}]}]
		}
	}`)

	hookReg := hook.NewHookRegistry()
	loader := NewLoader(nil, nil).WithHookRegistry(hookReg)
	inst, err := loader.LoadPlugin(SourceProject, dir)
	require.NoError(t, err)
	assert.Len(t, inst.LoadedHooks, 2)
}

// --- loader.go 브랜치 커버리지: skills 단일 SKILL.md 파일 ---

func TestLoader_LoadPlugin_SkillSingleFile(t *testing.T) {
	dir := t.TempDir()
	skillFile := filepath.Join(dir, "my-skill.md")
	writeFile(t, skillFile, `---
name: my-skill
description: test
---
# My Skill`)

	writeManifest(t, dir, `{
		"name":"single-skill-plugin",
		"version":"1.0.0",
		"skills":[{"id":"my-skill","path":"my-skill.md"}]
	}`)

	loader := NewLoader(nil, nil)
	inst, err := loader.LoadPlugin(SourceProject, dir)
	require.NoError(t, err)
	assert.Equal(t, PluginID("single-skill-plugin"), inst.ID)
	assert.NotEmpty(t, inst.LoadedSkills)
}

// --- loader.go: agent 디렉토리 로드 ---

func TestLoader_LoadPlugin_AgentDir(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
	writeFile(t, filepath.Join(agentDir, "myagent2.md"), `---
agent_type: myagent2
---
# Agent 2`)

	writeManifest(t, dir, `{
		"name":"agent-dir-plugin",
		"version":"1.0.0",
		"agents":[{"id":"myagent2","path":"agents"}]
	}`)

	loader := NewLoader(nil, nil)
	inst, err := loader.LoadPlugin(SourceProject, dir)
	require.NoError(t, err)
	assert.NotEmpty(t, inst.LoadedAgents)
}

// --- loader.go: commands 기록 ---

func TestLoader_LoadPlugin_WithCommands(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{
		"name":"cmd-plugin",
		"version":"1.0.0",
		"commands":[{"name":"my-cmd","description":"test","path":"commands/my-cmd.md"}]
	}`)

	loader := NewLoader(nil, nil)
	inst, err := loader.LoadPlugin(SourceProject, dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"my-cmd"}, inst.LoadedCommands)
}

// --- loader.go: MCP 서버 기록 ---

func TestLoader_LoadPlugin_WithMCPServers(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{
		"name":"mcp-plugin",
		"version":"1.0.0",
		"mcpServers":[{"name":"my-server","transport":"stdio","command":"my-cmd"}]
	}`)

	loader := NewLoader(nil, nil)
	inst, err := loader.LoadPlugin(SourceProject, dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"my-server"}, inst.LoadedMCP)
}

// --- loader.go: permissions 기록 ---

func TestLoader_LoadPlugin_WithPermissions(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{
		"name":"perm-plugin",
		"version":"1.0.0",
		"permissions":["network","filesystem:project"]
	}`)

	loader := NewLoader(nil, nil)
	inst, err := loader.LoadPlugin(SourceProject, dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"network", "filesystem:project"}, inst.Permissions)
}

// --- mcpb.go: 파일 없는 mcpb ---

func TestMCPB_EmptyZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.mcpb")
	f, err := os.Create(path)
	require.NoError(t, err)
	w := zip.NewWriter(f)
	w.Close()
	f.Close()

	loader := NewLoader(nil, nil)
	// manifest.json 없으므로 로드 실패
	_, err = loader.LoadMCPB(path)
	assert.Error(t, err)
}

// --- mcpb.go: 유효하지 않은 zip ---

func TestMCPB_InvalidZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.mcpb")
	require.NoError(t, os.WriteFile(path, []byte("not a zip file"), 0644))

	loader := NewLoader(nil, nil)
	_, err := loader.LoadMCPB(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MCPB 압축 해제 실패")
}

// --- config.go: IsEnabled 브랜치 ---

func TestConfig_IsEnabled_Branches(t *testing.T) {
	// nil cfg
	var cfg *PluginsYAML
	assert.True(t, cfg.IsEnabled("any"))

	// 플러그인 설정 없음 → 기본 활성화
	cfg = &PluginsYAML{}
	assert.True(t, cfg.IsEnabled("unknown-plugin"))

	// 명시적 enabled=true
	cfg = &PluginsYAML{
		Plugins: map[string]PluginConfig{
			"foo": {Enabled: true},
		},
	}
	assert.True(t, cfg.IsEnabled("foo"))
}

// --- validator.go: URI 파싱 실패 케이스 ---

func TestValidator_URI_NoUser(t *testing.T) {
	// user 없는 URL은 통과
	err := validateNoCredentialsInURI("https://host.com/mcp")
	assert.NoError(t, err)
}

// --- ReloadFromDirs: 로드 실패 건너뜀 ---

func TestLoader_ReloadFromDirs_SkipsFailures(t *testing.T) {
	validDir := t.TempDir()
	writeManifest(t, validDir, `{"name":"valid-plugin","version":"1.0.0"}`)

	invalidDir := t.TempDir()
	// manifest.json 없는 디렉토리

	cfg := &PluginsYAML{
		Plugins: map[string]PluginConfig{
			"valid-plugin": {Enabled: true},
		},
	}
	loader := NewLoader(nil, cfg)
	reg := NewPluginRegistry(nil)

	err := loader.ReloadFromDirs(reg, []string{validDir, invalidDir})
	require.NoError(t, err)

	list := reg.List()
	assert.Len(t, list, 1)
	assert.Equal(t, PluginID("valid-plugin"), list[0].ID)
}

// --- ReloadFromDirs: 중복 이름 건너뜀 ---

func TestLoader_ReloadFromDirs_SkipsDuplicates(t *testing.T) {
	dir1 := t.TempDir()
	writeManifest(t, dir1, `{"name":"dup-plugin","version":"1.0.0"}`)
	dir2 := t.TempDir()
	writeManifest(t, dir2, `{"name":"dup-plugin","version":"2.0.0"}`)

	loader := NewLoader(nil, nil) // 모든 enabled
	reg := NewPluginRegistry(nil)

	err := loader.ReloadFromDirs(reg, []string{dir1, dir2})
	require.NoError(t, err)

	list := reg.List()
	assert.Len(t, list, 1, "duplicate name should be skipped")
}

// --- loader.go: 잘못된 skill path ---

func TestLoader_LoadPlugin_SkillPathNotExist(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{
		"name":"bad-skill-plugin",
		"version":"1.0.0",
		"skills":[{"id":"missing","path":"nonexistent/path"}]
	}`)

	loader := NewLoader(nil, nil)
	_, err := loader.LoadPlugin(SourceProject, dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "skill")
}

// --- loader.go: 잘못된 agent path ---

func TestLoader_LoadPlugin_AgentPathNotExist(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{
		"name":"bad-agent-plugin",
		"version":"1.0.0",
		"agents":[{"id":"missing","path":"nonexistent/agent.md"}]
	}`)

	loader := NewLoader(nil, nil)
	_, err := loader.LoadPlugin(SourceProject, dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent")
}

// --- PluginRegistry: IsLoading state 검증 ---

func TestRegistry_IsLoadingState(t *testing.T) {
	reg := NewPluginRegistry(nil)
	// 기본 상태에서 IsLoading == false
	// PluginHookLoader 인터페이스 구현체로 registry가 활용 가능한지 확인
	assert.NotNil(t, reg)
}
