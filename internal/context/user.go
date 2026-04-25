// Package context는 QueryEngine의 context window 관리와 compaction 전략을 구현한다.
package context

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// UserContext는 CLAUDE.md 내용 등 사용자 환경 context 정보이다.
// SPEC-GOOSE-CONTEXT-001 §6.2 user.go
type UserContext struct {
	// ClaudeMd는 모든 CLAUDE.md 파일의 concat 내용이다.
	ClaudeMd string
	// CurrentDate는 최초 계산 시각의 UTC ISO 8601 포맷이다.
	CurrentDate string
	// ComputedAt은 처음 계산된 시각이다.
	ComputedAt time.Time
}

// sessionUserContext는 session-local memoized UserContext이다.
var (
	sessionUserCtxMu    sync.Mutex
	sessionUserCtxValue atomic.Pointer[UserContext]
)

// GetUserContext는 session 내에서 memoized UserContext를 반환한다.
// REQ-CTX-001/002: 동일 session 내 반복 호출은 캐시된 값을 반환한다.
// REQ-CTX-006: 최초 호출 시 cwd → root walk + addDirs 검색으로 CLAUDE.md를 수집한다.
//
// @MX:ANCHOR: [AUTO] session 단위 UserContext 단일 진입점
// @MX:REASON: SPEC-GOOSE-CONTEXT-001 REQ-CTX-002 - currentDate memoization 불변식, fan_in >= 3
func GetUserContext(_ context.Context, cwd string, addDirs []string) (*UserContext, error) {
	// 이미 캐시된 값이 있으면 반환
	if cached := sessionUserCtxValue.Load(); cached != nil {
		return cached, nil
	}

	sessionUserCtxMu.Lock()
	defer sessionUserCtxMu.Unlock()

	// double-check 후 캐시된 값이 있으면 반환
	if cached := sessionUserCtxValue.Load(); cached != nil {
		return cached, nil
	}

	userCtx, err := computeUserContext(cwd, addDirs)
	if err != nil {
		return nil, err
	}
	sessionUserCtxValue.Store(userCtx)
	return userCtx, nil
}

// InvalidateUserContext는 session-local UserContext 캐시를 무효화한다.
// REQ-CTX-010: 다음 GetUserContext 호출 시 파일 IO를 재실행한다.
func InvalidateUserContext() {
	sessionUserCtxMu.Lock()
	defer sessionUserCtxMu.Unlock()
	sessionUserCtxValue.Store(nil)
}

// computeUserContext는 CLAUDE.md 파일을 수집해 UserContext를 계산한다.
// SPEC-GOOSE-CONTEXT-001 §6.5 Walk 알고리즘.
func computeUserContext(cwd string, addDirs []string) (*UserContext, error) {
	now := time.Now().UTC()
	currentDate := now.Format("2006-01-02T15:04:05Z")

	contents, err := collectClaudeMd(cwd, addDirs)
	if err != nil {
		return nil, err
	}

	return &UserContext{
		ClaudeMd:    contents,
		CurrentDate: currentDate,
		ComputedAt:  now,
	}, nil
}

// collectClaudeMd는 cwd에서 루트까지 walk-up하며 CLAUDE.md를 수집한다.
// SPEC-GOOSE-CONTEXT-001 §6.5
func collectClaudeMd(cwd string, addDirs []string) (string, error) {
	var results []string

	// cwd → root walk-up (prepend: 상위일수록 앞에)
	dir := filepath.Clean(cwd)
	for {
		claudeMdPath := filepath.Join(dir, "CLAUDE.md")
		if content, err := os.ReadFile(claudeMdPath); err == nil {
			results = append([]string{string(content)}, results...)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // root 도달
		}
		dir = parent
	}

	// addDirs 검색 (append)
	for _, addDir := range addDirs {
		claudeMdPath := filepath.Join(filepath.Clean(addDir), "CLAUDE.md")
		if content, err := os.ReadFile(claudeMdPath); err == nil {
			results = append(results, string(content))
		}
	}

	return strings.Join(results, "\n\n---\n\n"), nil
}
