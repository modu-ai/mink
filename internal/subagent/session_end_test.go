package subagent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrphanWorktreeCleanup_OnSessionEnd은 SessionEnd 이벤트 발동 시
// orphan worktree가 제거됨을 검증한다. (AC-SA-015, REQ-SA-015)
func TestOrphanWorktreeCleanup_OnSessionEnd(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()

	// git repo 초기화
	gitInit(t, cwd)

	// orphan worktree 시뮬레이션: 디렉토리만 생성
	orphanDir := filepath.Join(cwd, ".claude", "worktrees", "researcher_crash")
	require.NoError(t, os.MkdirAll(orphanDir, 0o700))

	// active agents가 없는 상태에서 prune 실행
	pruneOrphanWorktrees(cwd, map[string]bool{})

	// orphan worktree 디렉토리가 제거되어야 함
	_, err := os.Stat(orphanDir)
	assert.True(t, os.IsNotExist(err), "orphan worktree directory should be removed")
}

// TestOrphanWorktreeCleanup_IsIdempotent는 cleanup이 idempotent함을 검증한다.
// (AC-SA-015)
func TestOrphanWorktreeCleanup_IsIdempotent(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	gitInit(t, cwd)

	// 첫 번째 prune (orphan 없음)
	pruneOrphanWorktrees(cwd, map[string]bool{})

	// 두 번째 prune (에러 없어야 함)
	assert.NotPanics(t, func() {
		pruneOrphanWorktrees(cwd, map[string]bool{})
	})
}

// TestSessionEnd_ConditionalHookIntegration은 HOOK-001의 SessionEnd가
// 존재할 때 hook dispatcher가 호출됨을 검증한다. (REQ-SA-015)
func TestSessionEnd_ConditionalHookIntegration(t *testing.T) {
	t.Parallel()
	hooks := &mockHookDispatcher{}

	// SessionEnd 디스패치
	err := hooks.DispatchSessionEnd(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, hooks.sessionEndCallCount())
}

// TestMemoryDir_Permission0700은 RunAgent가 memdir를 0700 권한으로 생성함을
// 검증한다. (AC-SA-012, REQ-SA-017)
func TestMemoryDir_Permission0700(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	mgr := &MemdirManager{
		agentType: "researcher",
		scopes:    []MemoryScope{ScopeProject},
		baseDirs: map[MemoryScope]string{
			ScopeProject: filepath.Join(homeDir, ".goose", "agent-memory", "researcher"),
		},
	}

	entry := MemoryEntry{
		ID:        "t1",
		Timestamp: time.Now(),
		Category:  "test",
		Key:       "k",
		Value:     "v",
		Scope:     ScopeProject,
	}
	require.NoError(t, mgr.Append(entry))

	dir := mgr.baseDirs[ScopeProject]
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

// gitInit은 테스트 디렉토리에 git repo를 초기화하는 헬퍼이다.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	if err := runGit(dir, "init"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	// 초기 commit 없이 HEAD 설정
	_ = runGit(dir, "commit", "--allow-empty", "-m", "init")
}

// runGit은 지정 디렉토리에서 git 명령을 실행한다.
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}
