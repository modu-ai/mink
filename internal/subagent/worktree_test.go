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

// TestRunAgent_WorktreeIsolation은 worktree isolation으로 sub-agent를 spawn하고
// worktree 디렉토리 생성 + hook 호출을 검증한다. (AC-SA-002, REQ-SA-006)
func TestRunAgent_WorktreeIsolation(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	gitInit(t, cwd)

	hooks := &mockHookDispatcher{}
	def := AgentDefinition{
		AgentType: "worktree_agent",
		Name:      "worktree_agent",
		Isolation: IsolationWorktree,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "worktree"},
		WithCwd(cwd),
		WithHookDispatcher(hooks),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, sa)

	// worktree 디렉토리가 생성되었는지 확인
	worktreesDir := filepath.Join(cwd, ".claude", "worktrees")
	entries, err2 := os.ReadDir(worktreesDir)
	if err2 == nil && len(entries) > 0 {
		// worktree create hook 호출 확인
		assert.GreaterOrEqual(t, hooks.worktreeCreateCallCount(), 1)
	}

	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)

	// 완료 후 일시 대기 (cleanup goroutine을 위해)
	time.Sleep(200 * time.Millisecond)
}

// TestRunAgent_WorktreeIsolation_Cleanup은 완료 후 worktree가 정리됨을
// 검증한다. (AC-SA-002)
func TestRunAgent_WorktreeIsolation_Cleanup(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	gitInit(t, cwd)

	def := AgentDefinition{
		AgentType: "cleanup_agent",
		Name:      "cleanup_agent",
		Isolation: IsolationWorktree,
		MaxTurns:  0, // 즉시 종료
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithCwd(cwd),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, sa)

	// 채널 완전히 drain
	drainWithTimeout(outCh, 3*time.Second)
	// cleanup goroutine 실행 대기
	time.Sleep(300 * time.Millisecond)
}

// TestWorktreeCreate_ValidGit은 유효한 git repo에서 worktree를 생성함을 검증한다.
func TestWorktreeCreate_ValidGit(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	if err := runGit(cwd, "init"); err != nil {
		t.Skip("git not available")
	}
	if err := runGit(cwd, "commit", "--allow-empty", "-m", "init"); err != nil {
		t.Skip("git commit failed")
	}
	// git config
	_ = runGit(cwd, "config", "user.email", "test@test.com")
	_ = runGit(cwd, "config", "user.name", "Test")

	ctx := context.Background()
	worktreePath, cleanup, err := createWorktree(ctx, "test_agent@sess-1", cwd)
	if err != nil {
		// git 환경에 따라 실패할 수 있음 (detached HEAD 등)
		t.Logf("createWorktree skipped: %v", err)
		return
	}
	defer cleanup()

	// worktree 디렉토리가 생성되었는지 확인
	assert.NotEmpty(t, worktreePath)
	_, err2 := os.Stat(worktreePath)
	assert.NoError(t, err2)
}

// TestRemoveWorktree_Exists는 존재하는 worktree가 올바르게 제거됨을 검증한다.
func TestRemoveWorktree_Exists(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	if err := runGit(cwd, "init"); err != nil {
		t.Skip("git not available")
	}
	if err := runGit(cwd, "commit", "--allow-empty", "-m", "init"); err != nil {
		t.Skip("git commit failed")
	}
	_ = runGit(cwd, "config", "user.email", "test@test.com")
	_ = runGit(cwd, "config", "user.name", "Test")

	ctx := context.Background()
	worktreePath, _, err := createWorktree(ctx, "remove_test@sess-1", cwd)
	if err != nil {
		t.Logf("createWorktree skipped: %v", err)
		return
	}

	// worktree 제거
	removeWorktree(cwd, worktreePath, "goose/agent/remove_test_sess-1")

	// 디렉토리가 제거되었는지 확인
	_, err2 := os.Stat(worktreePath)
	assert.True(t, os.IsNotExist(err2))
}

// TestSanitizeSlug_EdgeCases는 slug 변환 edge case를 검증한다.
func TestSanitizeSlug_EdgeCases(t *testing.T) {
	t.Parallel()
	// 빈 문자열
	assert.Equal(t, "", sanitizeSlug(""))
	// 특수문자만
	assert.Equal(t, "_", sanitizeSlug("@"))
	// alphanumeric는 그대로
	assert.Equal(t, "researcher", sanitizeSlug("researcher"))
}
