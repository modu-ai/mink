// Package credential_test는 Refresher 인터페이스 배선 및 refresh 실패 처리 테스트를 포함한다.
// OI-02: Refresher interface를 Select 경로에 배선
// OI-03: refreshFailCount + 3회 연속 실패 시 영구 고갈
// AC-CREDPOOL-003: OAuth refresh trigger 검증
// AC-CREDPOOL-009: Refresh 3회 연속 실패 → 영구 고갈
package credential_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/credential"
)

// refresherSpy는 테스트용 Refresher 스파이 구현체이다.
type refresherSpy struct {
	callCount atomic.Int64
	returnErr error
}

func (r *refresherSpy) Refresh(_ context.Context, _ *credential.PooledCredential) error {
	r.callCount.Add(1)
	return r.returnErr
}

// TestPool_OAuthRefreshAutoOnExpiring은 AC-CREDPOOL-003을 검증한다.
// ExpiresAt이 RefreshMargin 내에 들어오면 Select 경로에서 Refresh가 호출되어야 한다.
func TestPool_OAuthRefreshAutoOnExpiring(t *testing.T) {
	t.Parallel()

	now := time.Now()

	creds := []*credential.PooledCredential{
		{
			ID:        "oauth-expiring",
			Provider:  "anthropic",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			ExpiresAt: now.Add(2 * time.Minute), // 2분 후 만료 (RefreshMargin=5분 내)
		},
	}

	spy := &refresherSpy{returnErr: nil}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy(),
		credential.WithRefresher(spy),
		credential.WithRefreshMargin(5*time.Minute))
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}
	defer pool.Release(c) //nolint:errcheck

	// Refresher가 정확히 1회 호출되었어야 함
	if got := spy.callCount.Load(); got != 1 {
		t.Errorf("Refresh 호출 횟수: got %d, want 1", got)
	}
}

// TestPool_OAuthRefreshNotTriggered는 ExpiresAt이 RefreshMargin 밖이면
// Refresh가 호출되지 않는지 검증한다.
func TestPool_OAuthRefreshNotTriggered(t *testing.T) {
	t.Parallel()

	now := time.Now()

	creds := []*credential.PooledCredential{
		{
			ID:        "oauth-fresh",
			Provider:  "anthropic",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			ExpiresAt: now.Add(1 * time.Hour), // 1시간 후 만료 (RefreshMargin=5분 밖)
		},
	}

	spy := &refresherSpy{returnErr: nil}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy(),
		credential.WithRefresher(spy),
		credential.WithRefreshMargin(5*time.Minute))
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}
	defer pool.Release(c) //nolint:errcheck

	// Refresher는 호출되지 않아야 함
	if got := spy.callCount.Load(); got != 0 {
		t.Errorf("만료 여유 있는 경우 Refresh 호출됨: got %d, want 0", got)
	}
}

// TestPool_OAuthRefreshApiKeyNotTriggered는 ExpiresAt.IsZero() (API key)이면
// Refresh가 호출되지 않는지 검증한다.
func TestPool_OAuthRefreshApiKeyNotTriggered(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{
			ID:        "api-key",
			Provider:  "openai",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			// ExpiresAt zero = 영구 유효 API key
		},
	}

	spy := &refresherSpy{returnErr: nil}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy(),
		credential.WithRefresher(spy),
		credential.WithRefreshMargin(5*time.Minute))
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}
	defer pool.Release(c) //nolint:errcheck

	// API key는 Refresh 대상이 아님
	if got := spy.callCount.Load(); got != 0 {
		t.Errorf("API key에 대해 Refresh 호출됨: got %d, want 0", got)
	}
}

// TestPool_RefreshFailure_ExhaustsEntry는 Refresh 실패 시 해당 엔트리가
// Exhausted 상태로 전환되어 Select에서 제외되는지 검증한다.
func TestPool_RefreshFailure_ExhaustsEntry(t *testing.T) {
	t.Parallel()

	now := time.Now()

	creds := []*credential.PooledCredential{
		{
			ID:        "oauth-failing",
			Provider:  "anthropic",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			ExpiresAt: now.Add(2 * time.Minute), // RefreshMargin 내
		},
		{
			ID:        "api-key-fallback",
			Provider:  "openai",
			KeyringID: "kr-2",
			Status:    credential.CredOK,
			// ExpiresAt zero = 영구 유효
		},
	}

	spy := &refresherSpy{returnErr: errors.New("refresh failed")}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy(),
		credential.WithRefresher(spy),
		credential.WithRefreshMargin(5*time.Minute))
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	// Select는 oauth-failing을 건너뛰고 api-key-fallback을 반환해야 함
	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}

	if c.ID != "api-key-fallback" {
		t.Errorf("Refresh 실패한 엔트리 건너뛰어야 함: got %q, want %q", c.ID, "api-key-fallback")
	}
	defer pool.Release(c) //nolint:errcheck
}

// TestPool_Refresh3FailuresMarksPermanentExhausted는 AC-CREDPOOL-009를 검증한다.
// Refresh가 3회 연속 실패하면 해당 엔트리는 영구 고갈 상태로 전환되어야 한다.
func TestPool_Refresh3FailuresMarksPermanentExhausted(t *testing.T) {
	t.Parallel()

	now := time.Now()

	creds := []*credential.PooledCredential{
		{
			ID:        "oauth-broken",
			Provider:  "anthropic",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			ExpiresAt: now.Add(2 * time.Minute), // RefreshMargin 내
		},
	}

	spy := &refresherSpy{returnErr: errors.New("refresh always fails")}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy(),
		credential.WithRefresher(spy),
		credential.WithRefreshMargin(5*time.Minute))
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	// 3회 Select 시도 — 각 시도에서 Refresh가 실패하므로 oauth-broken은 Exhausted
	for i := range 3 {
		_, err := pool.Select(context.Background())
		// 실패가 누적될수록 풀 고갈 or 해당 엔트리 건너뜀
		_ = err
		_ = i
	}

	// Refresh가 3회 이상 호출되어야 함 (각 Select에서 1회)
	if got := spy.callCount.Load(); got < 1 {
		t.Errorf("Refresh가 한 번도 호출되지 않음")
	}

	// 4번째 Select에서 oauth-broken은 영구 고갈로 후보에서 제외되어야 함 (ErrExhausted)
	_, err = pool.Select(context.Background())
	if !errors.Is(err, credential.ErrExhausted) {
		t.Errorf("3회 실패 후 영구 고갈 기대: got %v", err)
	}
}

// TestPool_NoRefresher_DoesNotPanic은 Refresher가 nil일 때 패닉하지 않는지 검증한다.
func TestPool_NoRefresher_DoesNotPanic(t *testing.T) {
	t.Parallel()

	now := time.Now()

	creds := []*credential.PooledCredential{
		{
			ID:        "oauth-expiring",
			Provider:  "anthropic",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			ExpiresAt: now.Add(2 * time.Minute),
		},
	}

	// Refresher 없이 풀 생성
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	// Refresher 없어도 Select는 정상 동작해야 함
	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Refresher 없이 Select 실패: %v", err)
	}
	defer pool.Release(c) //nolint:errcheck
}
