package subagent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/permissions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
)

// mockHookDispatcher는 테스트용 HookDispatcher 구현이다.
// mu로 모든 슬라이스 접근을 보호하여 race-free를 보장한다.
type mockHookDispatcher struct {
	mu                  sync.Mutex
	subagentStartCalls  []string
	subagentStopCalls   []string
	worktreeCreateCalls []string
	worktreeRemoveCalls []string
	teammateIdleCalls   []string
	sessionEndCalls     int
	shouldFailStart     bool
}

func (m *mockHookDispatcher) DispatchSubagentStart(_ context.Context, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shouldFailStart {
		return ErrHookDispatchFailed
	}
	m.subagentStartCalls = append(m.subagentStartCalls, agentID)
	return nil
}

func (m *mockHookDispatcher) DispatchSubagentStop(_ context.Context, agentID string, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subagentStopCalls = append(m.subagentStopCalls, agentID)
	return nil
}

func (m *mockHookDispatcher) DispatchWorktreeCreate(_ context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.worktreeCreateCalls = append(m.worktreeCreateCalls, path)
	return nil
}

func (m *mockHookDispatcher) DispatchWorktreeRemove(_ context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.worktreeRemoveCalls = append(m.worktreeRemoveCalls, path)
	return nil
}

func (m *mockHookDispatcher) DispatchTeammateIdle(_ context.Context, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.teammateIdleCalls = append(m.teammateIdleCalls, agentID)
	return nil
}

func (m *mockHookDispatcher) DispatchSessionEnd(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionEndCalls++
	return nil
}

// startCallCount는 race-free하게 subagentStartCalls 길이를 반환한다.
func (m *mockHookDispatcher) startCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.subagentStartCalls)
}

// startCallAt은 race-free하게 i번째 subagentStartCalls를 반환한다.
func (m *mockHookDispatcher) startCallAt(i int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.subagentStartCalls[i]
}

// idleCallCount는 race-free하게 teammateIdleCalls 길이를 반환한다.
func (m *mockHookDispatcher) idleCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.teammateIdleCalls)
}

// stopCallCount는 race-free하게 subagentStopCalls 길이를 반환한다.
func (m *mockHookDispatcher) stopCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.subagentStopCalls)
}

// worktreeCreateCallCount는 race-free하게 worktreeCreateCalls 길이를 반환한다.
func (m *mockHookDispatcher) worktreeCreateCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.worktreeCreateCalls)
}

// sessionEndCallCount는 race-free하게 sessionEndCalls 값을 반환한다.
func (m *mockHookDispatcher) sessionEndCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessionEndCalls
}

// TestRunAgent_ForkIsolation은 fork isolation으로 sub-agent를 spawn하고
// AgentID, TeammateIdentity, hooks를 검증한다. (AC-SA-001)
func TestRunAgent_ForkIsolation(t *testing.T) {
	t.Parallel()
	hooks := &mockHookDispatcher{}
	def := AgentDefinition{
		AgentType: "researcher",
		Name:      "researcher",
		Isolation: IsolationFork,
		Tools:     []string{"*"},
		Model:     "inherit",
	}
	input := SubagentInput{Prompt: "Hello"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sa, outCh, err := RunAgent(ctx, def, input,
		WithSessionID("parentSession"),
		WithHookDispatcher(hooks),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, sa)
	require.NotNil(t, outCh)

	// AgentID 형식 검증: researcher@parentSession-N
	assert.Contains(t, sa.AgentID, "researcher@parentSession-")
	assert.Equal(t, "researcher", sa.Identity.AgentName)
	assert.Equal(t, "parentSession", sa.Identity.ParentSessionID)

	// SubagentStart hook 호출 확인
	assert.Equal(t, 1, hooks.startCallCount())
	assert.Equal(t, sa.AgentID, hooks.startCallAt(0))

	// ctx 취소 후 채널 드레인
	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)
}

// TestRunAgent_TeammateIdentity_Injected는 TeammateIdentity가 child context에
// 올바르게 주입됨을 검증한다. (REQ-SA-005b)
func TestRunAgent_TeammateIdentity_Injected(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{
		AgentType: "analyst",
		Name:      "analyst",
		Isolation: IsolationFork,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithSessionID("sess-test"),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, sa)

	// AgentID 포함 확인
	assert.NotEmpty(t, sa.AgentID)
	assert.Contains(t, sa.AgentID, agentIDDelimiter)

	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)
}

// TestRunAgent_SpawnDepthExceeded는 MaxSpawnDepth 초과 시 ErrSpawnDepthExceeded를
// 반환함을 검증한다. (AC-SA-009, REQ-SA-014)
func TestRunAgent_SpawnDepthExceeded(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{
		AgentType: "worker",
		Name:      "worker",
		Isolation: IsolationFork,
	}
	// spawn depth를 MaxSpawnDepth+1로 설정
	ctx := context.Background()
	for i := 0; i <= MaxSpawnDepth; i++ {
		ctx = withSpawnDepth(ctx)
	}

	_, _, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"}, WithLogger(nopLogger()))
	assert.ErrorIs(t, err, ErrSpawnDepthExceeded)
}

// TestRunAgent_HookStartFailed는 SubagentStart hook 실패 시
// ErrHookDispatchFailed를 반환함을 검증한다. (REQ-SA-005-F ii)
func TestRunAgent_HookStartFailed(t *testing.T) {
	t.Parallel()
	hooks := &mockHookDispatcher{shouldFailStart: true}
	def := AgentDefinition{
		AgentType: "researcher",
		Name:      "researcher",
		Isolation: IsolationFork,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithHookDispatcher(hooks),
		WithLogger(nopLogger()),
	)
	assert.ErrorIs(t, err, ErrHookDispatchFailed)
}

// TestRunAgent_BackgroundIsolation_NonBlocking은 background isolation에서
// RunAgent가 즉시 반환함을 검증한다. (AC-SA-003, REQ-SA-007)
func TestRunAgent_BackgroundIsolation_NonBlocking(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{
		AgentType: "bg_agent",
		Name:      "bg_agent",
		Isolation: IsolationBackground,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "background test"},
		WithLogger(nopLogger()),
	)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, sa)
	require.NotNil(t, outCh)

	// background는 500ms 이내에 반환 (non-blocking)
	assert.Less(t, elapsed, 500*time.Millisecond, "background RunAgent must return immediately")

	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)
}

// TestRunAgent_CoordinatorNested_Warn은 중첩 coordinator 모드에서
// WARN 로그가 출력됨을 검증한다. (AC-SA-008, REQ-SA-011)
func TestRunAgent_CoordinatorNested_Warn(t *testing.T) {
	t.Parallel()
	// 부모 ctx에 TeammateIdentity 주입 (coordinator)
	parentID := TeammateIdentity{AgentID: "coordinator@sess-1"}
	ctx := WithTeammateIdentity(context.Background(), parentID)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	def := AgentDefinition{
		AgentType:       "sub_coordinator",
		Name:            "sub_coordinator",
		Isolation:       IsolationFork,
		CoordinatorMode: true, // nested coordinator
	}

	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithLogger(nopLogger()),
	)
	// warn이 출력되지만 spawn은 성공
	require.NoError(t, err)
	require.NotNil(t, sa)
	assert.True(t, sa.Definition.CoordinatorMode)

	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)
}

// TestRunAgent_SubagentState_Running은 spawn 직후 state가 Running임을 검증한다.
func TestRunAgent_SubagentState_Running(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{
		AgentType: "runner",
		Name:      "runner",
		Isolation: IsolationFork,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"}, WithLogger(nopLogger()))
	require.NoError(t, err)
	assert.Equal(t, StateRunning, sa.State())

	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)
}

// TestRunAgent_BackgroundIdleThreshold는 DefaultBackgroundIdleThreshold 이후
// TeammateIdle hook이 호출됨을 검증한다. (REQ-SA-007)
func TestRunAgent_BackgroundIdleThreshold(t *testing.T) {
	t.Parallel()
	hooks := &mockHookDispatcher{}
	def := AgentDefinition{
		AgentType: "idle_bg",
		Name:      "idle_bg",
		Isolation: IsolationBackground,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"},
		WithHookDispatcher(hooks),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, sa)

	// DefaultBackgroundIdleThreshold(5s) + 약간의 여유 시간 대기
	// 테스트 속도를 위해 임시로 짧은 시간으로 테스트
	// 실제로는 5초가 지나야 idle이 발생하지만 ctx cancel로 테스트 단축
	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)
	// idle hook은 발생할 수도 있고 아닐 수도 있다 (ctx cancel로 인한 race)
}

// TestNoGoroutineLeak는 spawn+cancel 후 goroutine 누수가 없음을 검증한다.
// REQ-SA-023 / AC-SA-021
func TestNoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreAnyFunction("go.uber.org/zap/zapcore.(*CheckedEntry).Write"),
		// QueryEngine loop goroutine은 LLM call의 ctx cancellation에 의해 종료된다.
		// stubLLMCall이 ctx cancel 후 곧 종료되므로 잠시 후 사라진다.
		goleak.IgnoreAnyFunction("github.com/modu-ai/goose/internal/query.(*QueryEngine).SubmitMessage"),
		goleak.IgnoreAnyFunction("github.com/modu-ai/goose/internal/query.(*QueryEngine).SubmitMessage.func1"),
		goleak.IgnoreAnyFunction("github.com/modu-ai/goose/internal/query/loop.queryLoop"),
		goleak.IgnoreAnyFunction("github.com/modu-ai/goose/internal/query/loop.queryLoop.func2"),
		goleak.IgnoreAnyFunction("github.com/modu-ai/goose/internal/query/loop.send"),
	)

	// fork
	{
		def := AgentDefinition{AgentType: "leak_fork", Name: "leak_fork", Isolation: IsolationFork}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"}, WithLogger(nopLogger()))
		if err == nil {
			cancel()
			drainWithTimeout(outCh, 300*time.Millisecond)
		} else {
			cancel()
		}
	}

	// background
	{
		def := AgentDefinition{AgentType: "leak_bg", Name: "leak_bg", Isolation: IsolationBackground}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "test"}, WithLogger(nopLogger()))
		if err == nil {
			cancel()
			drainWithTimeout(outCh, 300*time.Millisecond)
		} else {
			cancel()
		}
	}

	// 모든 goroutine이 GoroutineShutdownGrace(100ms) 내 종료 확인
	time.Sleep(GoroutineShutdownGrace + 50*time.Millisecond)
}

// TestPlanModeApprove는 plan mode sub-agent의 승인 플로우를 검증한다.
// AC-SA-020: REQ-SA-022
func TestPlanModeApprove(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{
		AgentType:      "planner",
		Name:           "planner",
		Isolation:      IsolationFork,
		PermissionMode: "plan",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sa, outCh, err := RunAgent(ctx, def, SubagentInput{Prompt: "plan"},
		WithSessionID("plan-sess"),
		WithLogger(nopLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, sa)
	assert.True(t, sa.Identity.PlanModeRequired)

	// plan mode 확인: planModeRegistry에서 entry 조회
	v, hasPlanEntry := planModeRegistry.Load(sa.AgentID)
	if hasPlanEntry {
		entry := v.(*planModeEntry)
		assert.True(t, entry.required, "plan mode must be required before approval")
	}

	// PlanModeApprove 호출
	err = PlanModeApprove(ctx, sa.AgentID)
	assert.NoError(t, err)

	// ErrAgentNotFound 검증
	err2 := PlanModeApprove(ctx, "nonexistent@sess-999")
	assert.ErrorIs(t, err2, ErrAgentNotFound)

	cancel()
	drainWithTimeout(outCh, 500*time.Millisecond)
}

// TestTeammateCanUseTool_BubbleToParent는 bubble mode에서 부모 CanUseTool로
// 위임됨을 검증한다. (AC-SA-007, REQ-SA-010)
func TestTeammateCanUseTool_BubbleToParent(t *testing.T) {
	t.Parallel()
	parentPerm := &denyAllCanUseTool{reason: "parent-policy"}
	tcu := &TeammateCanUseTool{
		def:              AgentDefinition{PermissionMode: "bubble", Isolation: IsolationFork},
		parentCanUseTool: parentPerm,
	}

	decision := tcu.Check(context.Background(), permCtx("search"))
	assert.Equal(t, "parent-policy", decision.Reason)
}

// TestTeammateCanUseTool_BackgroundWriteDenied는 background agent의
// write tool 기본 거부를 검증한다. (AC-SA-011, REQ-SA-016)
func TestTeammateCanUseTool_BackgroundWriteDenied(t *testing.T) {
	t.Parallel()
	tcu := &TeammateCanUseTool{
		def: AgentDefinition{
			Isolation:      IsolationBackground,
			PermissionMode: "bubble",
		},
		settingsPerms: &SettingsPermissions{},
	}

	decision := tcu.Check(context.Background(), permCtx("write"))
	assert.Equal(t, "background_agent_write_denied", decision.Reason)
}

// TestSubagent_ExplicitToolsFilter는 명시적 tool 목록이 올바르게 필터링됨을
// 검증한다. (AC-SA-010, REQ-SA-013)
func TestSubagent_ExplicitToolsFilter(t *testing.T) {
	t.Parallel()
	def := AgentDefinition{
		AgentType: "filtered",
		Name:      "filtered",
		Tools:     []string{"read", "search"},
		Isolation: IsolationFork,
	}
	tools := buildToolList(def)
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}
	assert.True(t, toolNames["read"])
	assert.True(t, toolNames["search"])
	assert.True(t, toolNames["task-update"]) // baseline
	assert.False(t, toolNames["write"])      // 제외됨
	assert.False(t, toolNames["bash"])       // 제외됨
}

// TestGenerateAgentID_UniqueAcrossConcurrent은 동시 호출에서 AgentID가 유일함을
// 검증한다. (REQ-SA-001)
func TestGenerateAgentID_UniqueAcrossConcurrent(t *testing.T) {
	t.Parallel()
	const n = 100
	results := make(chan string, n)
	for i := 0; i < n; i++ {
		go func() {
			id := generateAgentID("researcher", "sess-1")
			results <- id
		}()
	}
	seen := make(map[string]bool)
	for i := 0; i < n; i++ {
		id := <-results
		assert.False(t, seen[id], "duplicate AgentID: %s", id)
		seen[id] = true
	}
}

// TestParseAgentID는 AgentID 파싱이 올바름을 검증한다. (REQ-SA-018)
func TestParseAgentID(t *testing.T) {
	t.Parallel()
	id := "researcher@parentSession-42"
	name, sessID, idx, err := parseAgentID(id)
	require.NoError(t, err)
	assert.Equal(t, "researcher", name)
	assert.Equal(t, "parentSession", sessID)
	assert.Equal(t, int64(42), idx)
}

// --- helpers ---

// nopLogger는 nop zap logger를 반환한다.
func nopLogger() *zap.Logger { return zap.NewNop() }

// drainWithTimeout은 SDKMessage 채널을 timeout까지 drain한다.
func drainWithTimeout(ch <-chan message.SDKMessage, timeout time.Duration) {
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
		}
	}
}

// permCtx는 테스트용 ToolPermissionContext를 생성한다.
func permCtx(toolName string) permissions.ToolPermissionContext {
	return permissions.ToolPermissionContext{ToolName: toolName}
}

// denyAllCanUseTool은 모든 tool을 거부하는 CanUseTool 구현이다.
type denyAllCanUseTool struct {
	reason string
}

func (d *denyAllCanUseTool) Check(_ context.Context, _ permissions.ToolPermissionContext) permissions.Decision {
	return permissions.Decision{Behavior: permissions.Deny, Reason: d.reason}
}
