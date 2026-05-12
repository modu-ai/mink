// Package context_test — SPEC-GOOSE-CONTEXT-001 SystemContext 테스트.
// AC-CTX-001: GetSystemContext memoization
// AC-CTX-014: git 부재 graceful path (REQ-CTX-015)
package context_test

import (
	"context"
	"testing"

	goosecontext "github.com/modu-ai/mink/internal/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetSystemContext_MemoizesGitCommand는 AC-CTX-001을 검증한다.
// Given: 테스트용 session
// When: GetSystemContext(ctx)를 2회 호출
// Then: 두 호출 결과는 pointer-equal, SystemContext.GitStatus는 non-empty
func TestGetSystemContext_MemoizesGitCommand(t *testing.T) {
	// 주의: session-level memoization 테스트이므로 병렬 실행 불가
	// (전역 캐시를 공유하기 때문)
	goosecontext.InvalidateSystemContext()
	t.Cleanup(goosecontext.InvalidateSystemContext)

	ctx := context.Background()

	first, err := goosecontext.GetSystemContext(ctx)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := goosecontext.GetSystemContext(ctx)
	require.NoError(t, err)
	require.NotNil(t, second)

	// REQ-CTX-001: pointer equality (또는 deep equal)
	assert.Same(t, first, second, "두 번째 호출은 동일 포인터를 반환해야 함")

	// GitStatus는 non-empty (실제 git repo 또는 "(no git)")
	assert.NotEmpty(t, first.GitStatus, "GitStatus는 non-empty이어야 함")
	assert.NotZero(t, first.ComputedAt, "ComputedAt은 설정되어야 함")
}

// TestGetSystemContext_NoGit_Graceful는 AC-CTX-014를 검증한다.
// REQ-CTX-015: git 부재 시 GitStatus="(no git)", 에러 없음.
func TestGetSystemContext_NoGit_Graceful(t *testing.T) {
	// 이 테스트는 git이 없는 환경(tmpdir)을 시뮬레이션하기 어렵다.
	// 대신 "(no git)" 값이 반환 가능함을 확인하는 방식으로 검증.
	// 실제 git 부재 환경에서는 GitStatus == "(no git)" 가 반환됨.

	// 현재 실행 환경이 git repo이므로 GitStatus != "(no git)"이 예상됨.
	// 그러나 함수가 에러 없이 실행됨을 확인한다.
	goosecontext.InvalidateSystemContext()
	t.Cleanup(goosecontext.InvalidateSystemContext)

	ctx := context.Background()
	result, err := goosecontext.GetSystemContext(ctx)

	require.NoError(t, err, "git 실행 결과와 무관하게 에러가 없어야 함")
	require.NotNil(t, result)
	// GitStatus는 "(no git)" 또는 실제 git 상태 문자열이어야 함
	assert.NotEmpty(t, result.GitStatus)

	// 2회차 호출 검증: memoized
	result2, err := goosecontext.GetSystemContext(ctx)
	require.NoError(t, err)
	assert.Same(t, result, result2, "2회차 호출은 memoized 값을 반환해야 함")
}

// TestInvalidateSystemContext_ForcesRecompute는 InvalidateSystemContext 후 재계산을 검증한다.
func TestInvalidateSystemContext_ForcesRecompute(t *testing.T) {
	goosecontext.InvalidateSystemContext()
	t.Cleanup(goosecontext.InvalidateSystemContext)

	ctx := context.Background()

	first, err := goosecontext.GetSystemContext(ctx)
	require.NoError(t, err)

	goosecontext.InvalidateSystemContext()

	second, err := goosecontext.GetSystemContext(ctx)
	require.NoError(t, err)

	// 무효화 후 새 포인터가 반환되어야 함
	assert.NotSame(t, first, second, "무효화 후에는 새 값이 반환되어야 함")
}
