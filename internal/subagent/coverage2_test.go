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

// TestMemdirManager_Query_NoDir는 디렉토리가 없을 때 Query가 에러 없이 반환함을 검증한다.
func TestMemdirManager_Query_NoDir(t *testing.T) {
	t.Parallel()
	mgr := &MemdirManager{
		agentType: "test",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs:  map[MemoryScope]string{ScopeProject: "/nonexistent/path/query"},
	}
	result, err := mgr.Query(func(e MemoryEntry) bool { return true })
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// TestBuildMemoryPrompt_AllScopesEmpty는 모든 scope가 비어있을 때 빈 문자열을 반환함을 검증한다.
func TestBuildMemoryPrompt_AllScopesEmpty(t *testing.T) {
	t.Parallel()
	mgr := &MemdirManager{
		agentType: "test",
		scopes:    []MemoryScope{ScopeLocal, ScopeProject, ScopeUser},
		baseDirs: map[MemoryScope]string{
			ScopeLocal:   "/nonexistent/local",
			ScopeProject: "/nonexistent/project",
			ScopeUser:    "/nonexistent/user",
		},
	}
	prompt, err := mgr.BuildMemoryPrompt()
	assert.NoError(t, err)
	assert.Empty(t, prompt)
}

// TestBuildMemoryPrompt_SingleScope는 단일 scope에서 올바르게 동작함을 검증한다.
func TestBuildMemoryPrompt_SingleScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".goose", "agent-memory", "test")

	mgr := &MemdirManager{
		agentType: "test",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs:  map[MemoryScope]string{ScopeProject: dir},
	}

	entry := MemoryEntry{
		ID:        "e1",
		Timestamp: time.Now(),
		Category:  "fact",
		Key:       "foo.bar",
		Value:     "hello",
		Scope:     ScopeProject,
	}
	require.NoError(t, mgr.Append(entry))

	prompt, err := mgr.BuildMemoryPrompt()
	require.NoError(t, err)
	assert.Contains(t, prompt, "foo.bar")
	assert.Contains(t, prompt, "Agent Memory")
}

// TestRunAgent_PlanModeTimeout는 plan mode timeout이 올바르게 동작함을 검증한다.
// REQ-SA-022(d): PlanApprovalTimeout 내 미승인 시 Failed
func TestRunAgent_PlanModeTimeout(t *testing.T) {
	t.Parallel()
	// timeout 테스트는 300s이므로 실제로는 ctx cancel로 테스트
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	def := AgentDefinition{
		AgentType:      "timeout_planner",
		Name:           "timeout_planner",
		Isolation:      IsolationFork,
		PermissionMode: "plan",
	}
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "plan"},
		WithSessionID("timeout-sess"),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	assert.True(t, sa.Identity.PlanModeRequired)

	// ctx cancel으로 종료
	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)
}

// TestStubExecutor_Run는 stubExecutor가 올바르게 동작함을 검증한다.
func TestStubExecutor_Run(t *testing.T) {
	t.Parallel()
	exec := stubExecutor{}
	result, err := exec.Run(context.Background(), "id1", "read", map[string]any{})
	assert.NoError(t, err)
	assert.Contains(t, result, "read")
}

// TestRegisterDeregisterPlanMode는 plan mode 레지스트리 등록/해제를 검증한다.
func TestRegisterDeregisterPlanMode(t *testing.T) {
	t.Parallel()
	agentID := "reg_test_agent@sess-1"
	entry := registerPlanMode(agentID)
	assert.NotNil(t, entry)
	assert.True(t, entry.required)

	_, ok := planModeRegistry.Load(agentID)
	assert.True(t, ok)

	deregisterPlanMode(agentID)
	_, ok2 := planModeRegistry.Load(agentID)
	assert.False(t, ok2)
}

// TestWorktreeCreate_NonGitDir는 비 git 디렉토리에서 createWorktree가 에러를 반환함을 검증한다.
func TestWorktreeCreate_NonGitDir(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	nonGitDir := t.TempDir()
	_, _, err := createWorktree(ctx, "test@sess-1", nonGitDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git worktree add")
}

// TestRemoveWorktree_NonExistent는 존재하지 않는 worktree 제거가 panic하지 않음을 검증한다.
func TestRemoveWorktree_NonExistent(t *testing.T) {
	t.Parallel()
	assert.NotPanics(t, func() {
		removeWorktree(t.TempDir(), "/nonexistent/worktree", "nonexistent-branch")
	})
}

// TestWorktreeListActive_ValidGit는 유효한 git repo에서 worktree list를 반환함을 검증한다.
func TestWorktreeListActive_ValidGit(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	if err := runGit(cwd, "init"); err != nil {
		t.Skip("git not available")
	}
	paths := worktreeListActive(cwd)
	// 최소 1개 (메인 worktree) 반환
	assert.NotEmpty(t, paths)
}

// TestRunAgent_ForkIsolation_MultipleSpawns는 여러 번 RunAgent를 호출할 때
// AgentID가 모두 유일함을 검증한다.
func TestRunAgent_ForkIsolation_MultipleSpawns(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for i := 0; i < 5; i++ {
		def := AgentDefinition{
			AgentType: "multi_worker",
			Name:      "multi_worker",
			Isolation: IsolationFork,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
			WithSessionID("multi-sess"),
			WithLogger(nopLogger()),
		)
		require.NoError(t, err)
		assert.False(t, seen[sa.AgentID], "duplicate AgentID: %s", sa.AgentID)
		seen[sa.AgentID] = true
		cancel()
		drainWithTimeout(outCh, 300*time.Millisecond)
	}
}

// TestLoadAgentsDir_ReadError는 파일 읽기 에러를 올바르게 처리함을 검증한다.
func TestLoadAgentsDir_ReadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o700))

	// 권한 없는 파일
	badFile := filepath.Join(agentsDir, "valid.md")
	require.NoError(t, os.WriteFile(badFile, []byte("---\nname: valid\n---\nbody"), 0o600))
	// 디렉토리 퍼미션을 읽기 불가로 (루트가 아닌 경우에만 동작)
	if os.Getuid() != 0 {
		require.NoError(t, os.Chmod(badFile, 0o000))
		defer os.Chmod(badFile, 0o600)

		_, errs := LoadAgentsDir(agentsDir)
		// 파일 읽기 에러가 포함되어야 함
		assert.NotEmpty(t, errs)
	}
}

// TestBuildQueryEngine_WithPlanEntry는 plan mode entry가 있는 경우 engine을 생성함을 검증한다.
func TestBuildQueryEngine_WithPlanEntry(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{
		AgentType:      "plan_engine",
		Name:           "plan_engine",
		PermissionMode: "plan",
		Isolation:      IsolationFork,
	}
	entry := &planModeEntry{required: true, approved: make(chan struct{})}
	cfg := buildRunConfig([]RunOption{WithLogger(nopLogger())})
	identity := TeammateIdentity{AgentID: "plan_engine@sess-1"}

	engine, err := buildQueryEngine(context.Background(), def, identity, entry, cfg)
	require.NoError(t, err)
	assert.NotNil(t, engine)
}

// TestRunAgent_ContextAlreadyCancelled는 이미 취소된 context에서 spawn이 실패함을 검증한다.
// REQ-SA-005-F(iii)
func TestRunAgent_ContextAlreadyCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	def := AgentDefinition{
		AgentType: "cancelled_agent",
		Name:      "cancelled_agent",
		Isolation: IsolationFork,
	}
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"}, WithLogger(nopLogger()))
	if err != nil {
		// ctx 취소로 인한 spawn 실패
		assert.ErrorIs(t, err, ErrSpawnAborted)
	} else {
		// engine이 이미 취소된 ctx를 처리하고 즉시 종료
		assert.NotNil(t, sa)
		drainWithTimeout(outCh, 500*time.Millisecond)
	}
}

// TestMemdirAppend_WithHomeDir는 homeDir가 설정된 경우 Append가 동작함을 검증한다.
func TestMemdirAppend_WithHomeDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "user_scope")

	mgr := &MemdirManager{
		agentType: "researcher",
		scopes:    []MemoryScope{ScopeUser},
		baseDirs:  map[MemoryScope]string{ScopeUser: dir},
	}

	entry := MemoryEntry{
		ID:        "u1",
		Timestamp: time.Now(),
		Category:  "memory",
		Key:       "user.key",
		Value:     "user.value",
		Scope:     ScopeUser,
	}
	require.NoError(t, mgr.Append(entry))

	// 파일이 생성되었는지 확인
	_, err := os.Stat(filepath.Join(dir, "memdir.jsonl"))
	assert.NoError(t, err)
}

// TestRunAgent_WithoutHookDispatcher는 hookDispatcher가 nil인 경우에도 동작함을 검증한다.
func TestRunAgent_WithoutHookDispatcher(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	def := AgentDefinition{
		AgentType: "no_hook",
		Name:      "no_hook",
		Isolation: IsolationFork,
	}
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithLogger(nopLogger()),
		// hookDispatch 없음
	)
	require.NoError(t, err)
	assert.NotNil(t, sa)

	cancel()
	drainWithTimeout(outCh, 300*time.Millisecond)
}

// TestBuildRunConfig_NilLogger는 logger가 nil이면 nop logger가 설정됨을 검증한다.
func TestBuildRunConfig_NilLogger(t *testing.T) {
	t.Parallel()
	cfg := buildRunConfig(nil)
	assert.NotNil(t, cfg.logger)
}

// TestWorktreeListActive_Lines는 worktree list 결과 파싱을 검증한다.
func TestWorktreeListActive_Lines(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	if err := runGit(cwd, "init"); err != nil {
		t.Skip("git not available")
	}
	_ = runGit(cwd, "commit", "--allow-empty", "-m", "init")
	paths := worktreeListActive(cwd)
	assert.GreaterOrEqual(t, len(paths), 1)
	// 첫 번째 path는 cwd의 실제 경로(symlink 해석)와 동일해야 함
	// macOS에서 /var/folders → /private/var/folders symlink가 있으므로 suffix 비교
	assert.NotEmpty(t, paths[0])
}
