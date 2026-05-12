// Package context_test — SPEC-GOOSE-CONTEXT-001 UserContext 테스트.
// AC-CTX-002: GetUserContext walks CLAUDE.md up to root
// AC-CTX-010: Context invalidation
package context_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goosecontext "github.com/modu-ai/mink/internal/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetUserContext_WalksUpAndConcatenates는 AC-CTX-002를 검증한다.
// Given: /tmp/test/a/b/c에 cwd, /tmp/test/a/CLAUDE.md와 /tmp/test/a/b/CLAUDE.md 2개 파일 생성
// When: GetUserContext(ctx, "/tmp/test/a/b/c", nil)
// Then: 두 파일 내용이 문서 순서대로 포함, currentDate가 time.Now().UTC() 근사(±1초)
func TestGetUserContext_WalksUpAndConcatenates(t *testing.T) {
	// session-level memoization 테스트이므로 병렬 실행 불가
	goosecontext.InvalidateUserContext()
	t.Cleanup(goosecontext.InvalidateUserContext)

	// 임시 디렉터리 구조 생성
	tmpRoot := t.TempDir()
	dirA := filepath.Join(tmpRoot, "a")
	dirAB := filepath.Join(tmpRoot, "a", "b")
	dirABC := filepath.Join(tmpRoot, "a", "b", "c")

	require.NoError(t, os.MkdirAll(dirABC, 0o755))

	contentA := "# CLAUDE.md from a\nThis is the project root context."
	contentAB := "# CLAUDE.md from a/b\nThis is the subdirectory context."

	require.NoError(t, os.WriteFile(filepath.Join(dirA, "CLAUDE.md"), []byte(contentA), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dirAB, "CLAUDE.md"), []byte(contentAB), 0o644))

	ctx := context.Background()
	before := time.Now().UTC()

	result, err := goosecontext.GetUserContext(ctx, dirABC, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// contentA와 contentAB 둘 다 포함 (상위가 먼저)
	assert.Contains(t, result.ClaudeMd, contentA, "상위 CLAUDE.md 내용이 포함되어야 함")
	assert.Contains(t, result.ClaudeMd, contentAB, "하위 CLAUDE.md 내용이 포함되어야 함")

	// 순서 확인: 상위 디렉터리(a)가 하위(a/b)보다 앞에 나와야 함
	posA := strings.Index(result.ClaudeMd, contentA)
	posAB := strings.Index(result.ClaudeMd, contentAB)
	assert.Less(t, posA, posAB, "상위 CLAUDE.md가 하위보다 앞에 위치해야 함")

	// currentDate 검증: ±1초 내
	parsedDate, err := time.Parse("2006-01-02T15:04:05Z", result.CurrentDate)
	require.NoError(t, err, "currentDate 포맷이 올바르지 않음")
	assert.WithinDuration(t, before, parsedDate, 2*time.Second, "currentDate가 now()로부터 1초 이내여야 함")

	// 2회차 호출: 파일 IO 없이 캐시 반환 (동일 포인터)
	result2, err := goosecontext.GetUserContext(ctx, dirABC, nil)
	require.NoError(t, err)
	assert.Same(t, result, result2, "두 번째 호출은 캐시된 포인터를 반환해야 함")
}

// TestGetUserContext_AddDirs는 addDirs 경로에서도 CLAUDE.md를 수집함을 검증한다.
func TestGetUserContext_AddDirs(t *testing.T) {
	goosecontext.InvalidateUserContext()
	t.Cleanup(goosecontext.InvalidateUserContext)

	tmpRoot := t.TempDir()
	dirCwd := filepath.Join(tmpRoot, "project")
	dirExtra := filepath.Join(tmpRoot, "extra")

	require.NoError(t, os.MkdirAll(dirCwd, 0o755))
	require.NoError(t, os.MkdirAll(dirExtra, 0o755))

	contentExtra := "# Extra CLAUDE.md"
	require.NoError(t, os.WriteFile(filepath.Join(dirExtra, "CLAUDE.md"), []byte(contentExtra), 0o644))

	ctx := context.Background()
	result, err := goosecontext.GetUserContext(ctx, dirCwd, []string{dirExtra})
	require.NoError(t, err)

	assert.Contains(t, result.ClaudeMd, contentExtra, "addDirs의 CLAUDE.md가 포함되어야 함")
}

// TestInvalidateUserContext_ForcesRecompute는 AC-CTX-010을 검증한다.
// Given: GetUserContext 1회 호출 완료 (캐시됨)
// When: InvalidateUserContext() 호출 후 GetUserContext 재호출
// Then: 새 UserContext 반환, 이후 호출은 다시 캐시됨
func TestInvalidateUserContext_ForcesRecompute(t *testing.T) {
	goosecontext.InvalidateUserContext()
	t.Cleanup(goosecontext.InvalidateUserContext)

	tmpRoot := t.TempDir()
	dirCwd := filepath.Join(tmpRoot, "project")
	require.NoError(t, os.MkdirAll(dirCwd, 0o755))

	content1 := "# First CLAUDE.md"
	require.NoError(t, os.WriteFile(filepath.Join(tmpRoot, "CLAUDE.md"), []byte(content1), 0o644))

	ctx := context.Background()

	// 1회 호출 (캐시됨)
	first, err := goosecontext.GetUserContext(ctx, dirCwd, nil)
	require.NoError(t, err)

	// 파일 내용 변경
	content2 := "# Updated CLAUDE.md"
	require.NoError(t, os.WriteFile(filepath.Join(tmpRoot, "CLAUDE.md"), []byte(content2), 0o644))

	// 무효화
	goosecontext.InvalidateUserContext()

	// 재호출: 새 내용 반영
	second, err := goosecontext.GetUserContext(ctx, dirCwd, nil)
	require.NoError(t, err)

	assert.NotSame(t, first, second, "무효화 후 새 포인터가 반환되어야 함")
	assert.Contains(t, second.ClaudeMd, content2, "무효화 후 새 내용이 반영되어야 함")

	// 3회차 호출: 다시 캐시됨
	third, err := goosecontext.GetUserContext(ctx, dirCwd, nil)
	require.NoError(t, err)
	assert.Same(t, second, third, "3회차 호출은 2회차 캐시를 반환해야 함")
}
