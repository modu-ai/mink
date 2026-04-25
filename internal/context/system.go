// Package context는 QueryEngine의 context window 관리와 compaction 전략을 구현한다.
package context

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SystemContext는 git 상태 등 시스템 수준 context 정보이다.
// SPEC-GOOSE-CONTEXT-001 §6.2 system.go
type SystemContext struct {
	// GitStatus는 git 상태 문자열이다 (최대 4KB). git 부재 시 "(no git)".
	GitStatus string
	// CacheBreaker는 build version 또는 session id이다.
	CacheBreaker string
	// ComputedAt은 처음 계산된 시각이다.
	ComputedAt time.Time
}

const (
	// gitStatusMaxBytes는 GitStatus 최대 크기이다 (4KB).
	gitStatusMaxBytes = 4096
	// gitTimeoutSeconds는 git 명령 타임아웃(초)이다.
	gitTimeoutSeconds = 2
)

// sessionSystemContext는 session-local memoized SystemContext이다.
var (
	sessionSystemCtxMu    sync.Mutex
	sessionSystemCtxOnce  sync.Once
	sessionSystemCtxValue atomic.Pointer[SystemContext]
)

// GetSystemContext는 session 내에서 memoized SystemContext를 반환한다.
// REQ-CTX-001: 동일 session 내 반복 호출은 동일 포인터를 반환한다.
// REQ-CTX-005: 최초 호출 시 git 명령을 실행하고 결과를 캐시한다.
//
// @MX:ANCHOR: [AUTO] session 단위 SystemContext 단일 진입점
// @MX:REASON: SPEC-GOOSE-CONTEXT-001 REQ-CTX-001 - memoization 불변식 보장, fan_in >= 3
func GetSystemContext(ctx context.Context) (*SystemContext, error) {
	// 이미 캐시된 값이 있으면 반환
	if cached := sessionSystemCtxValue.Load(); cached != nil {
		return cached, nil
	}

	sessionSystemCtxMu.Lock()
	defer sessionSystemCtxMu.Unlock()

	// double-check 후 캐시된 값이 있으면 반환
	if cached := sessionSystemCtxValue.Load(); cached != nil {
		return cached, nil
	}

	sysCtx := computeSystemContext(ctx)
	sessionSystemCtxValue.Store(sysCtx)
	return sysCtx, nil
}

// InvalidateSystemContext는 session-local SystemContext 캐시를 무효화한다.
// REQ-CTX-001: 다음 GetSystemContext 호출 시 git 명령을 재실행한다.
func InvalidateSystemContext() {
	sessionSystemCtxMu.Lock()
	defer sessionSystemCtxMu.Unlock()
	sessionSystemCtxValue.Store(nil)
	sessionSystemCtxOnce = sync.Once{} // reset for future use
}

// computeSystemContext는 git 명령을 실행해 SystemContext를 계산한다.
// REQ-CTX-015: git 부재 또는 타임아웃 시 GitStatus="(no git)"로 graceful 처리.
func computeSystemContext(ctx context.Context) *SystemContext {
	gitStatus := runGitCommands(ctx)
	return &SystemContext{
		GitStatus:  gitStatus,
		ComputedAt: time.Now().UTC(),
	}
}

// runGitCommands는 git 명령을 실행하고 결과를 반환한다.
// 타임아웃 또는 에러 시 "(no git)" 반환.
func runGitCommands(ctx context.Context) string {
	timeoutCtx, cancel := context.WithTimeout(ctx, gitTimeoutSeconds*time.Second)
	defer cancel()

	var parts []string

	// git branch --show-current
	if branch := runGitCmd(timeoutCtx, "git", "branch", "--show-current"); branch != "" {
		parts = append(parts, "branch: "+branch)
	}

	// git status --porcelain
	if status := runGitCmd(timeoutCtx, "git", "status", "--porcelain"); status != "" {
		parts = append(parts, "status:\n"+status)
	}

	// git log -1 --format=%h %s
	if log := runGitCmd(timeoutCtx, "git", "log", "-1", "--format=%h %s"); log != "" {
		parts = append(parts, "last commit: "+log)
	}

	if len(parts) == 0 {
		return "(no git)"
	}

	result := strings.Join(parts, "\n")
	// 4KB truncation
	if len(result) > gitStatusMaxBytes {
		result = result[:gitStatusMaxBytes]
	}
	return result
}

// runGitCmd는 단일 git 명령을 실행하고 stdout을 반환한다.
// 에러 시 빈 문자열 반환.
func runGitCmd(ctx context.Context, name string, args ...string) string {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}
