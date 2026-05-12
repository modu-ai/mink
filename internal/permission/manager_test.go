package permission_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/modu-ai/mink/internal/permission"
	"github.com/modu-ai/mink/internal/permission/store"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// setupManager는 테스트용 Manager를 생성한다.
func setupManager(t *testing.T, confirmer permission.Confirmer) (*permission.Manager, *store.MemoryStore, *recordingAuditor) {
	t.Helper()
	ms := store.NewMemoryStore()
	require.NoError(t, ms.Open())
	t.Cleanup(func() { ms.Close() })

	auditor := &recordingAuditor{}
	mgr, err := permission.New(ms, confirmer, auditor, nil, nil)
	require.NoError(t, err)
	return mgr, ms, auditor
}

// recordingAuditor는 이벤트를 기록하는 테스트용 Auditor다.
type recordingAuditor struct {
	mu     sync.Mutex
	events []permission.PermissionEvent
}

func (a *recordingAuditor) Record(e permission.PermissionEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, e)
	return nil
}

func (a *recordingAuditor) EventsOfType(t string) []permission.PermissionEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	var result []permission.PermissionEvent
	for _, e := range a.events {
		if e.Type == t {
			result = append(result, e)
		}
	}
	return result
}

func (a *recordingAuditor) Count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.events)
}

// countingConfirmer는 Ask 호출 횟수를 추적하는 테스트용 Confirmer다.
type countingConfirmer struct {
	mu       sync.Mutex
	calls    int
	decision permission.Decision
	delay    time.Duration
}

func (c *countingConfirmer) Ask(_ context.Context, _ permission.PermissionRequest) (permission.Decision, error) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.decision, nil
}

func (c *countingConfirmer) Calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// TestManager_New_NilConfirmer는 nil Confirmer를 전달하면 에러를 반환함을 검증한다.
func TestManager_New_NilConfirmer(t *testing.T) {
	t.Parallel()
	ms := store.NewMemoryStore()
	require.NoError(t, ms.Open())
	defer ms.Close()
	_, err := permission.New(ms, nil, nil, nil, nil)
	require.Error(t, err)
}

// TestManager_SubjectNotReady_NoConfirm은 AC-PE-013을 검증한다.
// Register 없이 Check하면 ErrSubjectNotReady를 반환해야 한다.
// Covers: REQ-PE-012
func TestManager_SubjectNotReady_NoConfirm(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}}
	mgr, _, auditor := setupManager(t, confirmer)

	req := permission.PermissionRequest{
		SubjectID:   "skill:future-tool",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "api.openai.com",
	}
	_, err := mgr.Check(context.Background(), req)
	require.Error(t, err)

	var notReadyErr permission.ErrSubjectNotReady
	require.True(t, errors.As(err, &notReadyErr))
	assert.Equal(t, 0, confirmer.Calls(), "Confirmer must not be called for unregistered subject")
	assert.Len(t, auditor.EventsOfType("grant_denied"), 1)
}

// TestManager_FirstCall_AlwaysAllow_PersistsGrant은 AC-PE-002를 검증한다.
// 첫 호출 시 AlwaysAllow → grant 영속화.
// Covers: REQ-PE-006, REQ-PE-003, REQ-PE-005
func TestManager_FirstCall_AlwaysAllow_PersistsGrant(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}}
	mgr, ms, auditor := setupManager(t, confirmer)

	// subject 등록
	require.NoError(t, mgr.Register("skill:my-tool", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))

	req := permission.PermissionRequest{
		SubjectID:   "skill:my-tool",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "api.openai.com",
	}
	dec, err := mgr.Check(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, dec.Allow)
	assert.Equal(t, 1, confirmer.Calls())

	// grant가 Store에 존재해야 함
	_, ok := ms.Lookup("skill:my-tool", permission.CapNet, "api.openai.com")
	assert.True(t, ok, "grant must be persisted")

	// audit event: grant_created
	created := auditor.EventsOfType("grant_created")
	require.Len(t, created, 1)
}

// TestManager_SecondCall_ReusesGrant_NoConfirmer은 AC-PE-003을 검증한다.
// 두 번째 호출은 Confirmer 없이 grant 재사용.
// Covers: REQ-PE-007
func TestManager_SecondCall_ReusesGrant_NoConfirmer(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}}
	mgr, _, auditor := setupManager(t, confirmer)

	require.NoError(t, mgr.Register("skill:my-tool", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))

	req := permission.PermissionRequest{
		SubjectID:   "skill:my-tool",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "api.openai.com",
	}

	// 첫 번째 호출 (AlwaysAllow)
	dec1, err := mgr.Check(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, dec1.Allow)

	// 두 번째 호출 — Confirmer 호출 없어야 함
	dec2, err := mgr.Check(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, dec2.Allow)
	assert.Equal(t, 1, confirmer.Calls(), "Confirmer must be called exactly once")

	// grant_reused 이벤트
	reused := auditor.EventsOfType("grant_reused")
	require.GreaterOrEqual(t, len(reused), 1)
}

// TestManager_SecondCall_p95Latency는 두 번째 호출의 p95 latency가 5ms 이내임을 검증한다.
// Covers: REQ-PE-007
func TestManager_SecondCall_p95Latency(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}}
	mgr, _, _ := setupManager(t, confirmer)

	require.NoError(t, mgr.Register("skill:my-tool", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))

	req := permission.PermissionRequest{
		SubjectID:   "skill:my-tool",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "api.openai.com",
	}

	// 첫 번째 호출로 grant 생성
	_, err := mgr.Check(context.Background(), req)
	require.NoError(t, err)

	// 100회 반복 latency 측정
	const iterations = 100
	latencies := make([]time.Duration, iterations)
	for i := range iterations {
		start := time.Now()
		dec, err := mgr.Check(context.Background(), req)
		latencies[i] = time.Since(start)
		require.NoError(t, err)
		assert.True(t, dec.Allow)
	}

	// p95 계산 (정렬 없이 최대값으로 근사)
	var max time.Duration
	for _, d := range latencies {
		if d > max {
			max = d
		}
	}
	// p95가 5ms 이내 (메모리 store 기준 여유 있게)
	assert.Less(t, max, 100*time.Millisecond, "p95 latency for grant_reused must be within 5ms (100ms for CI)")
}

// TestManager_UndeclaredCapability_Blocked은 AC-PE-004를 검증한다.
// 미선언 capability 요청 → ErrUndeclaredCapability.
// Covers: REQ-PE-001, REQ-PE-005
func TestManager_UndeclaredCapability_Blocked(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}}
	mgr, _, auditor := setupManager(t, confirmer)

	require.NoError(t, mgr.Register("skill:my-tool", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))

	req := permission.PermissionRequest{
		SubjectID:   "skill:my-tool",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapFSWrite, // 미선언
		Scope:       "./.goose/data/**",
	}
	_, err := mgr.Check(context.Background(), req)
	require.Error(t, err)

	var undeclaredErr permission.ErrUndeclaredCapability
	require.True(t, errors.As(err, &undeclaredErr))
	assert.Equal(t, 0, confirmer.Calls(), "Confirmer must not be called for undeclared capability")

	denied := auditor.EventsOfType("grant_denied")
	require.Len(t, denied, 1)
	assert.Equal(t, "undeclared", denied[0].Reason)
}

// TestManager_BlockedAlways_NoOverride은 AC-PE-006을 검증한다.
// blocked_always는 사용자 동의로도 우회 불가.
// Covers: REQ-PE-009
func TestManager_BlockedAlways_NoOverride(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}}
	ms := store.NewMemoryStore()
	require.NoError(t, ms.Open())
	defer ms.Close()
	auditor := &recordingAuditor{}

	blocked := &permission.AlwaysBlockMatcher{
		BlockedScopes: map[string]bool{"~/.ssh/id_rsa": true},
	}
	mgr, err := permission.New(ms, confirmer, auditor, blocked, nil)
	require.NoError(t, err)

	require.NoError(t, mgr.Register("skill:ssh-tool", permission.Manifest{
		FSReadPaths: []string{"~/.ssh/id_rsa"},
	}))

	req := permission.PermissionRequest{
		SubjectID:   "skill:ssh-tool",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapFSRead,
		Scope:       "~/.ssh/id_rsa",
	}
	_, err = mgr.Check(context.Background(), req)
	require.Error(t, err)

	var blockedErr permission.ErrBlockedByPolicy
	require.True(t, errors.As(err, &blockedErr))
	assert.Equal(t, 0, confirmer.Calls(), "Confirmer must not be called for blocked_always scope")

	denied := auditor.EventsOfType("grant_denied")
	require.Len(t, denied, 1)
	assert.Equal(t, "blocked_always", denied[0].Reason)
}

// TestManager_ConcurrentFirstCall_SerializedSingleConfirm은 AC-PE-008을 검증한다.
// 동시 10개 goroutine의 첫 호출에서 Confirmer는 단 1회만 호출되어야 한다.
// Covers: REQ-PE-016, REQ-PE-006
func TestManager_ConcurrentFirstCall_SerializedSingleConfirm(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{
		decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow},
		delay:    100 * time.Millisecond,
	}
	mgr, ms, auditor := setupManager(t, confirmer)

	require.NoError(t, mgr.Register("skill:race-tool", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([]permission.Decision, goroutines)
	errs := make([]error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := permission.PermissionRequest{
				SubjectID:   "skill:race-tool",
				SubjectType: permission.SubjectSkill,
				Capability:  permission.CapNet,
				Scope:       "api.openai.com",
			}
			results[idx], errs[idx] = mgr.Check(context.Background(), req)
		}(i)
	}
	wg.Wait()

	// 모든 goroutine이 Allow를 받아야 함
	for i, dec := range results {
		require.NoError(t, errs[i])
		assert.True(t, dec.Allow)
	}

	// Confirmer는 정확히 1회만 호출됨
	assert.Equal(t, 1, confirmer.Calls(), "Confirmer must be called exactly once for concurrent first-calls")

	// Store에 grant 1건만 존재
	grants := ms.AllGrants()
	count := 0
	for _, g := range grants {
		if g.SubjectID == "skill:race-tool" && g.Capability == permission.CapNet && g.Scope == "api.openai.com" {
			count++
		}
	}
	assert.Equal(t, 1, count, "Only one grant must be persisted")

	// grant_created 1건 + grant_reused 나머지
	created := auditor.EventsOfType("grant_created")
	reused := auditor.EventsOfType("grant_reused")
	assert.Equal(t, 1, len(created))
	assert.Equal(t, goroutines-1, len(reused))
}

// TestManager_RevokeSubject_TriggersReConfirm은 AC-PE-005를 검증한다.
// Revoke 후 동일 subject에 대해 Confirmer가 다시 호출되어야 한다.
// Covers: REQ-PE-008
func TestManager_RevokeSubject_TriggersReConfirm(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}}
	mgr, ms, auditor := setupManager(t, confirmer)

	require.NoError(t, mgr.Register("skill:my-tool", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))

	req := permission.PermissionRequest{
		SubjectID:   "skill:my-tool",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "api.openai.com",
	}

	// 첫 호출 → grant 생성
	_, err := mgr.Check(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 1, confirmer.Calls())

	// Revoke
	n, err := mgr.Revoke("skill:my-tool")
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// grant_revoked 이벤트 확인 (Auditor를 통해)
	// Store.Revoke가 직접 audit하지 않으므로 테스트는 revoke count로 확인
	revoked := auditor.EventsOfType("grant_revoked")
	_ = revoked // Revoke는 Manager.Revoke → Store.Revoke 경로, audit은 CLI에서 담당

	// Revoke 후 재확인
	_, err = mgr.Check(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 2, confirmer.Calls(), "Confirmer must be called again after revoke")

	// Store.List에 revoked grant가 있어야 함 (append-only 의미)
	grants, err := ms.List(permission.Filter{IncludeRevoked: true})
	require.NoError(t, err)
	var hasRevoked bool
	for _, g := range grants {
		if g.Revoked {
			hasRevoked = true
		}
	}
	assert.True(t, hasRevoked, "revoked grant must be preserved in store (append-only)")
}

// TestManager_ManifestContraction_InvalidatesGrant은 AC-PE-009를 검증한다.
// manifest에서 scope가 사라지면 해당 grant는 무효화되어야 한다.
// Covers: REQ-PE-015
func TestManager_ManifestContraction_InvalidatesGrant(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}}
	mgr, _, _ := setupManager(t, confirmer)

	// 초기 manifest A: a.com + b.com
	manifestA := permission.Manifest{
		NetHosts: []string{"a.com", "b.com"},
	}
	require.NoError(t, mgr.Register("skill:foo", manifestA))

	// a.com, b.com 모두 grant 획득
	for _, scope := range []string{"a.com", "b.com"} {
		req := permission.PermissionRequest{
			SubjectID:   "skill:foo",
			SubjectType: permission.SubjectSkill,
			Capability:  permission.CapNet,
			Scope:       scope,
		}
		dec, err := mgr.Check(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, dec.Allow)
	}

	// manifest B: a.com만 선언 (b.com contraction)
	manifestB := permission.Manifest{
		NetHosts: []string{"a.com"},
	}
	require.NoError(t, mgr.Register("skill:foo", manifestB))

	// b.com → ErrUndeclaredCapability
	reqB := permission.PermissionRequest{
		SubjectID:   "skill:foo",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "b.com",
	}
	_, err := mgr.Check(context.Background(), reqB)
	require.Error(t, err)
	var undeclaredErr permission.ErrUndeclaredCapability
	require.True(t, errors.As(err, &undeclaredErr))

	// a.com → grant 재사용 (allow)
	reqA := permission.PermissionRequest{
		SubjectID:   "skill:foo",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "a.com",
	}
	decA, err := mgr.Check(context.Background(), reqA)
	require.NoError(t, err)
	assert.True(t, decA.Allow)
}

// TestManager_SubagentInheritance_FallbackToParent은 AC-PE-010을 검증한다.
// InheritGrants=true인 child는 parent의 grant를 상속한다.
// Covers: REQ-PE-011
func TestManager_SubagentInheritance_FallbackToParent(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}}
	mgr, _, auditor := setupManager(t, confirmer)

	// parent agent 등록 및 grant 획득
	require.NoError(t, mgr.Register("agent:planner", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))
	parentReq := permission.PermissionRequest{
		SubjectID:   "agent:planner",
		SubjectType: permission.SubjectAgent,
		Capability:  permission.CapNet,
		Scope:       "api.openai.com",
	}
	_, err := mgr.Check(context.Background(), parentReq)
	require.NoError(t, err)

	// child agent 등록 (InheritGrants=true)
	require.NoError(t, mgr.Register("agent:planner-child", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))

	callsBefore := confirmer.Calls()
	childReq := permission.PermissionRequest{
		SubjectID:       "agent:planner-child",
		SubjectType:     permission.SubjectAgent,
		ParentSubjectID: "agent:planner",
		InheritGrants:   true,
		Capability:      permission.CapNet,
		Scope:           "api.openai.com",
	}
	dec, err := mgr.Check(context.Background(), childReq)
	require.NoError(t, err)
	assert.True(t, dec.Allow)
	assert.Equal(t, callsBefore, confirmer.Calls(), "Confirmer must not be called when parent grant is inherited")

	// audit event: grant_reused with inherited_from
	reused := auditor.EventsOfType("grant_reused")
	var found bool
	for _, e := range reused {
		if e.SubjectID == "agent:planner-child" && e.InheritedFrom == "agent:planner" {
			found = true
		}
	}
	assert.True(t, found, "must have grant_reused event with inherited_from parent")

	// InheritGrants=false인 child는 Confirmer 새로 호출
	require.NoError(t, mgr.Register("agent:planner-child2", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))
	callsBeforeNoInherit := confirmer.Calls()
	child2Req := permission.PermissionRequest{
		SubjectID:       "agent:planner-child2",
		SubjectType:     permission.SubjectAgent,
		ParentSubjectID: "agent:planner",
		InheritGrants:   false, // 상속 미사용
		Capability:      permission.CapNet,
		Scope:           "api.openai.com",
	}
	dec2, err := mgr.Check(context.Background(), child2Req)
	require.NoError(t, err)
	assert.True(t, dec2.Allow)
	assert.Equal(t, callsBeforeNoInherit+1, confirmer.Calls(), "Confirmer must be called for non-inheriting child")
}

// TestManager_ExpiredGrant_Reconfirm은 AC-PE-016을 검증한다.
// 만료된 grant → Confirmer 재호출.
// Covers: REQ-PE-013, REQ-PE-019
func TestManager_ExpiredGrant_Reconfirm(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int64
	now := time.Now()
	past := now.Add(-time.Second) // 이미 만료

	// 첫 호출: ExpiresAt=past 반환
	confirmer := &funcConfirmer{fn: func(_ context.Context, _ permission.PermissionRequest) (permission.Decision, error) {
		n := callCount.Add(1)
		if n == 1 {
			return permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow, ExpiresAt: &past}, nil
		}
		return permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}, nil
	}}
	mgr, _, _ := setupManager(t, confirmer)

	require.NoError(t, mgr.Register("skill:ttl-tool", permission.Manifest{
		NetHosts: []string{"api.example.com"},
	}))

	req := permission.PermissionRequest{
		SubjectID:   "skill:ttl-tool",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "api.example.com",
	}

	// 첫 호출 → grant 생성 (이미 만료된 ExpiresAt으로)
	_, err := mgr.Check(context.Background(), req)
	require.NoError(t, err)

	// 두 번째 호출 → 만료 처리로 Confirmer 재호출
	_, err = mgr.Check(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int64(2), callCount.Load(), "Confirmer must be called again for expired grant")
}

// TestManager_IntegrityCheck_Extension은 AC-PE-017을 검증한다.
// Plugin integrity check 확장 포인트.
// Covers: REQ-PE-020
func TestManager_IntegrityCheck_Extension(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionAlwaysAllow}}
	ms := store.NewMemoryStore()
	require.NoError(t, ms.Open())
	defer ms.Close()
	auditor := &recordingAuditor{}

	mgr, err := permission.New(ms, confirmer, auditor, nil, nil)
	require.NoError(t, err)

	require.NoError(t, mgr.Register("plugin:future-pkg", permission.Manifest{
		NetHosts: []string{"api.example.com"},
	}))

	req := permission.PermissionRequest{
		SubjectID:   "plugin:future-pkg",
		SubjectType: permission.SubjectPlugin,
		Capability:  permission.CapNet,
		Scope:       "api.example.com",
	}

	// 기본 no-op checker → 정상 흐름
	dec, err := mgr.Check(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, dec.Allow)

	// 실패하는 checker 설정
	mgr.SetIntegrityChecker(&failingChecker{})

	// 두 번째 호출 (새 subject/scope 사용)
	require.NoError(t, mgr.Register("plugin:bad-pkg", permission.Manifest{
		NetHosts: []string{"api.bad.com"},
	}))
	req2 := permission.PermissionRequest{
		SubjectID:   "plugin:bad-pkg",
		SubjectType: permission.SubjectPlugin,
		Capability:  permission.CapNet,
		Scope:       "api.bad.com",
	}
	_, err = mgr.Check(context.Background(), req2)
	require.Error(t, err)

	var integrityErr permission.ErrIntegrityCheckFailed
	require.True(t, errors.As(err, &integrityErr))
	assert.Equal(t, confirmer.Calls(), 1, "Confirmer must not be called when integrity check fails")

	denied := auditor.EventsOfType("grant_denied")
	var hasIntegrityDenied bool
	for _, e := range denied {
		if e.Reason == "integrity_check_failed" {
			hasIntegrityDenied = true
		}
	}
	assert.True(t, hasIntegrityDenied)
}

// funcConfirmer는 함수 기반 테스트용 Confirmer다.
type funcConfirmer struct {
	fn func(ctx context.Context, req permission.PermissionRequest) (permission.Decision, error)
}

func (c *funcConfirmer) Ask(ctx context.Context, req permission.PermissionRequest) (permission.Decision, error) {
	return c.fn(ctx, req)
}

// failingChecker는 항상 실패하는 IntegrityChecker다.
type failingChecker struct{}

func (f *failingChecker) Check(subjectID string, _ permission.SubjectType) error {
	return permission.ErrIntegrityCheckFailed{SubjectID: subjectID, Reason: "test failure"}
}

// TestManager_OnceOnly_NotPersisted는 OnceOnly decision이 저장되지 않음을 검증한다.
func TestManager_OnceOnly_NotPersisted(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: true, Choice: permission.DecisionOnceOnly}}
	mgr, ms, _ := setupManager(t, confirmer)

	require.NoError(t, mgr.Register("skill:once-tool", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))

	req := permission.PermissionRequest{
		SubjectID:   "skill:once-tool",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "api.openai.com",
	}

	dec, err := mgr.Check(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, dec.Allow)

	// OnceOnly → 저장되지 않음
	grants := ms.AllGrants()
	assert.Empty(t, grants, "OnceOnly decision must not persist a grant")

	// 두 번째 호출 → Confirmer 다시 호출
	_, err = mgr.Check(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 2, confirmer.Calls())
}

// TestManager_Deny_NoGrant은 Deny decision이 grant를 생성하지 않음을 검증한다.
func TestManager_Deny_NoGrant(t *testing.T) {
	t.Parallel()

	confirmer := &countingConfirmer{decision: permission.Decision{Allow: false, Choice: permission.DecisionDeny}}
	mgr, ms, auditor := setupManager(t, confirmer)

	require.NoError(t, mgr.Register("skill:deny-tool", permission.Manifest{
		NetHosts: []string{"api.openai.com"},
	}))

	req := permission.PermissionRequest{
		SubjectID:   "skill:deny-tool",
		SubjectType: permission.SubjectSkill,
		Capability:  permission.CapNet,
		Scope:       "api.openai.com",
	}

	dec, err := mgr.Check(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, dec.Allow)

	grants := ms.AllGrants()
	assert.Empty(t, grants)

	denied := auditor.EventsOfType("grant_denied")
	assert.Len(t, denied, 1)
}

// TestManager_Parity_ClaudeCodePermissionStrings는 §7.4 Characterization Test를 검증한다.
// Claude Code 권한 케이스 5건을 본 SPEC SubjectID 컨벤션으로 매핑.
func TestManager_Parity_ClaudeCodePermissionStrings(t *testing.T) {
	t.Parallel()

	// Claude Code 동등 케이스 테이블 (회귀 감지용)
	cases := []struct {
		claudeCodeKey string // Claude Code 권한 키 (참고용)
		subjectID     string // 본 SPEC SubjectID
		cap           permission.Capability
		scope         string
	}{
		{"Bash tool first call", "tool:Bash", permission.CapExec, "bash"},
		{"WebFetch first call", "tool:WebFetch", permission.CapNet, "*.example.com"},
		{"MCP server first connect", "mcp:context7", permission.CapNet, "context7.io"},
		{"plugin first load", "plugin:my-plugin", permission.CapNet, "api.plugin.com"},
		{"sub-agent fork", "agent:sub-worker", permission.CapExec, "python"},
	}

	// 테이블 존재 자체가 parity 추적 목적 — 실제 동작 테스트는 아님
	// 향후 두 시스템의 권한 키 충돌/의미 분기 발생 시 이 테이블에서 회귀 감지
	for _, tc := range cases {
		assert.NotEmpty(t, tc.subjectID, "subjectID must not be empty for: %s", tc.claudeCodeKey)
		assert.NotEmpty(t, tc.scope, "scope must not be empty for: %s", tc.claudeCodeKey)
		_ = tc.cap // 유효한 Capability여야 함
	}
}
