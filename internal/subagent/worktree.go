package subagent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// slugRE는 AgentID를 worktree path-safe slug로 변환하는 정규식이다.
var slugRE = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// sanitizeSlug는 agentID를 worktree path-safe slug로 변환한다.
func sanitizeSlug(agentID string) string {
	return slugRE.ReplaceAllString(agentID, "_")
}

// createWorktree는 git worktree를 생성하고 cleanup 함수를 반환한다.
// REQ-SA-006: git worktree add ./.claude/worktrees/{agent-slug} -b {branch}
//
// @MX:WARN: [AUTO] git worktree add는 OS 외부 프로세스 실행
// @MX:REASON: REQ-SA-006 — git binary 의존성. git 없으면 실패. R1: fallback to fork 권장
func createWorktree(ctx context.Context, agentID, cwd string) (worktreePath string, cleanup func(), err error) {
	slug := sanitizeSlug(agentID)
	worktreePath = filepath.Join(cwd, ".claude", "worktrees", slug)
	branch := fmt.Sprintf("goose/agent/%s", slug)

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branch, worktreePath)
	cmd.Dir = cwd
	if out, err2 := cmd.CombinedOutput(); err2 != nil {
		return "", nil, fmt.Errorf("createWorktree: git worktree add: %w (output: %s)", err2, out)
	}

	cleanup = func() {
		removeWorktree(cwd, worktreePath, branch)
	}
	return worktreePath, cleanup, nil
}

// removeWorktree는 worktree와 branch를 제거한다. idempotent.
func removeWorktree(cwd, worktreePath, branch string) {
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = cwd
	_ = cmd.Run()

	cmd2 := exec.Command("git", "branch", "-D", branch)
	cmd2.Dir = cwd
	_ = cmd2.Run()

	_ = os.RemoveAll(worktreePath)
}

// pruneOrphanWorktrees는 현재 세션의 active agent에 없는 orphan worktree를 제거한다.
// REQ-SA-015: startup-time idempotent scan.
func pruneOrphanWorktrees(cwd string, activeAgentIDs map[string]bool) {
	worktreesDir := filepath.Join(cwd, ".claude", "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		// active agent인지 확인
		isActive := false
		for agentID := range activeAgentIDs {
			if sanitizeSlug(agentID) == slug {
				isActive = true
				break
			}
		}
		if !isActive {
			worktreePath := filepath.Join(worktreesDir, slug)
			// git worktree prune 먼저
			cmd := exec.Command("git", "worktree", "prune")
			cmd.Dir = cwd
			_ = cmd.Run()
			// 디렉토리 제거
			_ = os.RemoveAll(worktreePath)
			logWarn("pruneOrphanWorktrees: removed orphan worktree",
				zap.String("path", worktreePath),
			)
		}
	}
}

// worktreeListActive는 git worktree list에서 활성 worktree 경로를 반환한다.
func worktreeListActive(cwd string) []string {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			paths = append(paths, strings.TrimSpace(path))
		}
	}
	return paths
}
