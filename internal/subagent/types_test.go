package subagent

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsolationMode_Constants는 3종 isolation 모드 상수를 검증한다.
func TestIsolationMode_Constants(t *testing.T) {
	assert.Equal(t, IsolationMode("fork"), IsolationFork)
	assert.Equal(t, IsolationMode("worktree"), IsolationWorktree)
	assert.Equal(t, IsolationMode("background"), IsolationBackground)
}

// TestMemoryScope_Constants는 3종 memory scope 상수를 검증한다.
func TestMemoryScope_Constants(t *testing.T) {
	assert.Equal(t, MemoryScope("user"), ScopeUser)
	assert.Equal(t, MemoryScope("project"), ScopeProject)
	assert.Equal(t, MemoryScope("local"), ScopeLocal)
}

// TestDefaultBackgroundIdleThreshold_Named는 DefaultBackgroundIdleThreshold가
// 명명 상수로 정의되어 있음을 검증한다. (REQ-SA-007)
func TestDefaultBackgroundIdleThreshold_Named(t *testing.T) {
	assert.Equal(t, 5*time.Second, DefaultBackgroundIdleThreshold, "DefaultBackgroundIdleThreshold must be 5s")
}

// TestGoroutineShutdownGrace_Named는 GoroutineShutdownGrace가 100ms임을 검증한다.
// REQ-SA-023
func TestGoroutineShutdownGrace_Named(t *testing.T) {
	assert.Equal(t, 100*time.Millisecond, GoroutineShutdownGrace)
}

// TestMaxSpawnDepth_Named는 MaxSpawnDepth가 5임을 검증한다. (REQ-SA-014)
func TestMaxSpawnDepth_Named(t *testing.T) {
	assert.Equal(t, 5, MaxSpawnDepth)
}

// TestAgentIDDelimiter는 agentID delimiter가 '@'임을 검증한다. (REQ-SA-018)
func TestAgentIDDelimiter(t *testing.T) {
	assert.Equal(t, "@", agentIDDelimiter)
}

// TestWithTeammateIdentity_RoundTrip은 context에 TeammateIdentity를 주입하고
// 추출하는 round-trip을 검증한다.
func TestWithTeammateIdentity_RoundTrip(t *testing.T) {
	t.Parallel()
	id := TeammateIdentity{
		AgentID:          "researcher@sess-1-1",
		AgentName:        "researcher",
		TeamName:         "analysis",
		PlanModeRequired: false,
		ParentSessionID:  "sess-1",
	}
	ctx := WithTeammateIdentity(context.Background(), id)
	got, ok := TeammateIdentityFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, id, got)
}

// TestTeammateIdentityFromContext_Missing는 identity가 없는 context에서
// 두 번째 반환값이 false임을 검증한다.
func TestTeammateIdentityFromContext_Missing(t *testing.T) {
	t.Parallel()
	_, ok := TeammateIdentityFromContext(context.Background())
	assert.False(t, ok)
}

// TestSpawnDepth_Context는 spawn depth가 context를 통해 올바르게 증가함을 검증한다.
// REQ-SA-014
func TestSpawnDepth_Context(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	assert.Equal(t, 0, spawnDepthFromContext(ctx))

	ctx = withSpawnDepth(ctx)
	assert.Equal(t, 1, spawnDepthFromContext(ctx))

	ctx = withSpawnDepth(ctx)
	assert.Equal(t, 2, spawnDepthFromContext(ctx))
}

// TestSubagent_State_Atomic은 Subagent.State()가 atomic하게 읽힘을 검증한다.
// REQ-SA-008: state mutation은 channel close happen-before.
func TestSubagent_State_Atomic(t *testing.T) {
	t.Parallel()
	s := &Subagent{}
	s.setState(StateRunning)
	assert.Equal(t, StateRunning, s.State())

	s.setState(StateCompleted)
	assert.Equal(t, StateCompleted, s.State())
}

// TestSubagent_State_ConcurrentReadWrite는 상태가 race-free로 읽고 쓰임을 검증한다.
func TestSubagent_State_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()
	s := &Subagent{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			s.setState(StateRunning)
			s.setState(StateCompleted)
		}
	}()
	for {
		select {
		case <-done:
			return
		default:
			_ = s.State()
		}
	}
}

// TestSubagent_State_InitialPending은 초기 상태가 StatePending(0)임을 검증한다.
func TestSubagent_State_InitialPending(t *testing.T) {
	t.Parallel()
	s := &Subagent{}
	assert.Equal(t, StatePending, s.State())
}

// TestAtomicSpawnIndex는 atomic.AddInt64이 동시 호출에서 유일한 인덱스를 생성함을 검증한다.
// REQ-SA-001
func TestAtomicSpawnIndex(t *testing.T) {
	t.Parallel()
	var counter int64
	seen := make(map[int64]bool)
	var mu sync.Mutex // 테스트 전용 map 보호

	const n = 100
	results := make(chan int64, n)
	for i := 0; i < n; i++ {
		go func() {
			idx := atomic.AddInt64(&counter, 1)
			results <- idx
		}()
	}
	for i := 0; i < n; i++ {
		idx := <-results
		mu.Lock()
		assert.False(t, seen[idx], "duplicate spawnIndex: %d", idx)
		seen[idx] = true
		mu.Unlock()
	}
}

// TestErrorSentinels는 에러 sentinel이 nil이 아님을 검증한다.
func TestErrorSentinels(t *testing.T) {
	t.Parallel()
	errs := []error{
		ErrUnsafeAgentProperty,
		ErrInvalidAgentName,
		ErrEngineInitFailed,
		ErrHookDispatchFailed,
		ErrSpawnAborted,
		ErrSpawnDepthExceeded,
		ErrAgentNotFound,
		ErrAgentNotInPlanMode,
		ErrMemdirLockUnsupported,
		ErrScopeNotEnabled,
		ErrTranscriptCorrupted,
		ErrUnknownModelAlias,
	}
	for _, err := range errs {
		assert.NotNil(t, err)
		assert.NotEmpty(t, err.Error())
	}
}
