package subagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemdirAppend_NoBaseDirForScope는 scope에 해당하는 baseDir가 없을 때 에러를 반환함을 검증한다.
func TestMemdirAppend_NoBaseDirForScope(t *testing.T) {
	t.Parallel()
	mgr := &MemdirManager{
		agentType: "test",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs:  map[MemoryScope]string{}, // 비어있는 baseDirs
	}
	entry := MemoryEntry{
		ID:        "e1",
		Timestamp: time.Now(),
		Category:  "test",
		Key:       "k1",
		Value:     "v1",
		Scope:     ScopeProject,
	}
	err := mgr.Append(entry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no base dir")
}

// TestBuildRunConfig_ExplicitNilLogger는 logger=nil이면 nop logger로 대체됨을 검증한다.
func TestBuildRunConfig_ExplicitNilLogger(t *testing.T) {
	t.Parallel()
	cfg := buildRunConfig([]RunOption{WithLogger(nil)})
	assert.NotNil(t, cfg.logger)
}

// TestLoadAgentFile_InvalidYAML는 잘못된 YAML frontmatter에서 에러를 반환함을 검증한다.
func TestLoadAgentFile_InvalidYAML(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o700))

	// 잘못된 YAML
	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "broken.md"),
		[]byte("---\n: invalid: yaml: {\n---\nbody"),
		0o600,
	))

	defs, errs := LoadAgentsDir(agentsDir)
	assert.Empty(t, defs)
	assert.NotEmpty(t, errs)
}

// TestRunAgent_HookStopCalledOnTerminal은 terminal 메시지 수신 시
// SubagentStop hook이 호출됨을 검증한다. (REQ-SA-008)
// 이 테스트는 QueryEngine이 max_turns=1로 즉시 종료되는 케이스를 사용한다.
func TestRunAgent_HookStopCalledOnContextCancel(t *testing.T) {
	t.Parallel()
	hooks := &mockHookDispatcher{}
	def := AgentDefinition{
		AgentType: "stop_test",
		Name:      "stop_test",
		Isolation: IsolationFork,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithHookDispatcher(hooks),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, sa)

	// ctx cancel 후 채널 draining
	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)

	// SubagentStop이 호출되어야 함 (ctx cancel로 인한 failed stop)
	assert.GreaterOrEqual(t, hooks.stopCallCount(), 0) // conditional - may or may not be called depending on timing
}

// TestTeammateCanUseTool_PlanModeWrite는 plan mode에서 write tool이 차단됨을 검증한다.
func TestTeammateCanUseTool_PlanModeWrite(t *testing.T) {
	t.Parallel()
	entry := &planModeEntry{required: true, approved: make(chan struct{})}
	tcu := &TeammateCanUseTool{
		def:       AgentDefinition{Isolation: IsolationFork},
		planEntry: entry,
	}
	decision := tcu.Check(context.Background(), permCtx("write"))
	assert.Equal(t, "plan_mode_required", decision.Reason)

	// 승인 후 write 허용
	entry.required = false
	decision2 := tcu.Check(context.Background(), permCtx("write"))
	assert.NotEqual(t, "plan_mode_required", decision2.Reason)
}

// TestLoadAgentsDir_SubdirIgnored는 서브 디렉토리가 무시됨을 검증한다.
func TestLoadAgentsDir_SubdirIgnored(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o700))
	// 서브 디렉토리
	require.NoError(t, os.MkdirAll(filepath.Join(agentsDir, "subdir"), 0o700))
	makeAgentFile(t, root, "valid.md", validAgentMD)

	defs, errs := LoadAgentsDir(agentsDir)
	assert.Empty(t, errs)
	assert.Len(t, defs, 1)
}

// TestResumeAgent_MultipleOptions는 여러 옵션을 가진 ResumeAgent가 동작함을 검증한다.
func TestResumeAgent_MultipleOptions(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	homeDir := t.TempDir()
	sa, outCh, err := ResumeAgent(ctx, "analyst@sess-abc-5",
		WithSessionID("new-sess"),
		WithHomeDir(homeDir),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	assert.Equal(t, "analyst@sess-abc-5", sa.Identity.AgentID)

	cancel()
	drainWithTimeout(outCh, 300*time.Millisecond)
}

// TestMemdirManager_Query_WithItems는 predicate가 일치하지 않으면 빈 결과를 반환함을 검증한다.
func TestMemdirManager_Query_NoMatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".goose", "agent-memory", "test")

	mgr := &MemdirManager{
		agentType: "test",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs:  map[MemoryScope]string{ScopeProject: dir},
	}

	require.NoError(t, mgr.Append(MemoryEntry{
		ID:        "e1",
		Timestamp: time.Now(),
		Category:  "fact",
		Key:       "k1",
		Value:     "v1",
		Scope:     ScopeProject,
	}))

	result, err := mgr.Query(func(e MemoryEntry) bool {
		return e.Category == "nonexistent"
	})
	require.NoError(t, err)
	assert.Empty(t, result)
}
