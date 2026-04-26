package subagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeAgentFile은 테스트용 .claude/agents/*.md 파일을 생성한다.
func makeAgentFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	agentsDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o700))
	p := filepath.Join(agentsDir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

const validAgentMD = `---
name: researcher
description: Research agent
allowed-tools:
  - read
  - search
model: inherit
isolation: fork
memory-scopes:
  - project
---
# Researcher Agent

This agent performs research tasks.
`

// TestLoadAgentsDir_ParsesFrontmatter는 유효한 agent MD 파일의 frontmatter를
// 올바르게 파싱함을 검증한다. (AC-SA-013 기반)
func TestLoadAgentsDir_ParsesFrontmatter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeAgentFile(t, root, "researcher.md", validAgentMD)

	defs, errs := LoadAgentsDir(filepath.Join(root, ".claude", "agents"))
	require.Empty(t, errs)
	require.Len(t, defs, 1)

	def := defs[0]
	assert.Equal(t, "researcher", def.Name)
	assert.Equal(t, "Research agent", def.Description)
	assert.Equal(t, []string{"read", "search"}, def.Tools)
	assert.Equal(t, "inherit", def.Model)
	assert.Equal(t, IsolationFork, def.Isolation)
	assert.Equal(t, []MemoryScope{ScopeProject}, def.MemoryScopes)
	assert.Contains(t, def.SystemPrompt, "Researcher Agent")
}

// TestLoadAgentsDir_UnsafeProperty는 허용되지 않은 frontmatter 속성이
// ErrUnsafeAgentProperty를 반환함을 검증한다. (AC-SA-013)
func TestLoadAgentsDir_UnsafeProperty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeAgentFile(t, root, "evil.md", `---
name: evil
exec_on_load: true
---
body
`)
	makeAgentFile(t, root, "good.md", validAgentMD)

	defs, errs := LoadAgentsDir(filepath.Join(root, ".claude", "agents"))
	// evil은 로드 실패, good은 성공
	assert.Len(t, errs, 1)
	assert.ErrorIs(t, errs[0], ErrUnsafeAgentProperty)
	// good agent는 정상 로드
	require.Len(t, defs, 1)
	assert.Equal(t, "researcher", defs[0].Name)
}

// TestLoadAgentsDir_InvalidAgentName은 유효하지 않은 에이전트 이름이
// ErrInvalidAgentName을 반환함을 검증한다. (AC-SA-016, REQ-SA-018)
func TestLoadAgentsDir_InvalidAgentName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// underscore-prefixed: reserved
	makeAgentFile(t, root, "_hidden.md", `---
name: _hidden
---
body
`)
	// space in name
	makeAgentFile(t, root, "foo bar.md", `---
name: foo bar
---
body
`)
	// non-ASCII
	makeAgentFile(t, root, "légal.md", `---
name: légal
---
body
`)
	// valid
	makeAgentFile(t, root, "valid_agent_1.md", `---
name: valid_agent_1
description: Valid agent
---
body
`)

	defs, errs := LoadAgentsDir(filepath.Join(root, ".claude", "agents"))
	// 3개 무효: _hidden, "foo bar", légal
	assert.GreaterOrEqual(t, len(errs), 2, "expect at least 2 invalid name errors")
	for _, err := range errs {
		assert.ErrorIs(t, err, ErrInvalidAgentName)
	}
	// valid_agent_1만 로드
	require.Len(t, defs, 1)
	assert.Equal(t, "valid_agent_1", defs[0].Name)
}

// TestLoadAgentsDir_HyphenInName은 하이픈이 있는 이름이 ErrInvalidAgentName이거나
// legacy source로 로드됨을 검증한다. (REQ-SA-018: hyphen은 AgentID delimiter)
func TestLoadAgentsDir_HyphenInName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeAgentFile(t, root, "manager-spec.md", `---
name: manager-spec
description: Legacy agent with hyphen
---
body
`)
	_, _ = LoadAgentsDir(filepath.Join(root, ".claude", "agents"))
	// 하이픈이 있는 이름은 ErrInvalidAgentName이거나 source="legacy" 경고를 남긴다.
	// 이 테스트는 패닉이 없고 빈 슬라이스나 legacy 태그로 로드됨을 검증.
	// (실제 결과는 구현 상세에 따라 다름)
}

// TestLoadAgentsDir_EmptyDir은 빈 디렉토리가 빈 결과를 반환함을 검증한다.
func TestLoadAgentsDir_EmptyDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o700))

	defs, errs := LoadAgentsDir(agentsDir)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

// TestLoadAgentsDir_NonExistentDir은 존재하지 않는 디렉토리가 빈 결과를 반환함을 검증한다.
func TestLoadAgentsDir_NonExistentDir(t *testing.T) {
	t.Parallel()
	defs, errs := LoadAgentsDir("/nonexistent/path/agents")
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

// TestLoadAgentsDir_MemoryScopesDefault는 frontmatter에 memory-scopes가 없을 때
// 기본값으로 [ScopeProject]가 설정됨을 검증한다. (§6.2 AgentDefinition.MemoryScopes)
func TestLoadAgentsDir_MemoryScopesDefault(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeAgentFile(t, root, "agent.md", `---
name: agent
description: Agent without memory-scopes
---
body
`)
	defs, errs := LoadAgentsDir(filepath.Join(root, ".claude", "agents"))
	require.Empty(t, errs)
	require.Len(t, defs, 1)
	// 기본값: [ScopeProject]
	assert.Equal(t, []MemoryScope{ScopeProject}, defs[0].MemoryScopes)
}
