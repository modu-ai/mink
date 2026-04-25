package core_test

// session_test.go — AC-CORE-010: WorkspaceRoot resolver 단위 테스트
// SPEC-GOOSE-CORE-001 REQ-CORE-013

import (
	"sync"
	"testing"

	"github.com/modu-ai/goose/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionRegistry_RegisterAndResolve는 두 세션 매핑 후 정확히 반환되는지 검증한다.
func TestSessionRegistry_RegisterAndResolve(t *testing.T) {
	t.Parallel()

	// Arrange
	reg := core.NewSessionRegistry()
	require.NotNil(t, reg)

	// Act
	reg.Register("sess-A", "/tmp/work-a")
	reg.Register("sess-B", "/tmp/work-b")

	// Assert
	assert.Equal(t, "/tmp/work-a", reg.WorkspaceRoot("sess-A"))
	assert.Equal(t, "/tmp/work-b", reg.WorkspaceRoot("sess-B"))
}

// TestSessionRegistry_UnknownSessionReturnsEmpty는 미등록 sessionID가 빈 문자열을 반환하는지 검증한다.
func TestSessionRegistry_UnknownSessionReturnsEmpty(t *testing.T) {
	t.Parallel()

	reg := core.NewSessionRegistry()

	// 등록하지 않은 세션 조회 → 빈 문자열
	assert.Equal(t, "", reg.WorkspaceRoot("sess-unknown"))
}

// TestSessionRegistry_Unregister는 Unregister 후 빈 문자열이 반환되는지 검증한다.
func TestSessionRegistry_Unregister(t *testing.T) {
	t.Parallel()

	reg := core.NewSessionRegistry()
	reg.Register("sess-A", "/tmp/work-a")

	// 해제 전 확인
	assert.Equal(t, "/tmp/work-a", reg.WorkspaceRoot("sess-A"))

	// 해제 후 빈 문자열
	reg.Unregister("sess-A")
	assert.Equal(t, "", reg.WorkspaceRoot("sess-A"))
}

// TestWorkspaceRoot_PackageHelper_NilSafe는 default registry 미설정 상태에서
// 패키지 헬퍼 호출 시 panic 없이 빈 문자열을 반환하는지 검증한다.
func TestWorkspaceRoot_PackageHelper_NilSafe(t *testing.T) {
	t.Parallel()

	// default registry가 초기화되지 않은 상태에서도 panic이 없어야 한다.
	// NewRuntime 호출 없이 바로 패키지 헬퍼를 호출한다.
	result := core.WorkspaceRoot("sess-any")
	assert.Equal(t, "", result)
}

// TestWorkspaceRoot_ConcurrentAccess는 AC-CORE-010 동시성 조건을 검증한다.
// 100개의 goroutine이 동시에 WorkspaceRoot를 호출해도 data race가 없고
// 결과가 항상 정확해야 한다.
func TestWorkspaceRoot_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	reg := core.NewSessionRegistry()
	reg.Register("sess-A", "/tmp/work-a")
	reg.Register("sess-B", "/tmp/work-b")

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			switch idx % 3 {
			case 0:
				got := reg.WorkspaceRoot("sess-A")
				assert.Equal(t, "/tmp/work-a", got)
			case 1:
				got := reg.WorkspaceRoot("sess-B")
				assert.Equal(t, "/tmp/work-b", got)
			case 2:
				got := reg.WorkspaceRoot("sess-unknown")
				assert.Equal(t, "", got)
			}
		}(i)
	}

	wg.Wait()
}
