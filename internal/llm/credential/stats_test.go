// Package credential_test는 PoolStats 관측성 테스트를 포함한다.
// OI-07: PoolStats{Total, Available, Exhausted, RefreshesTotal, RotationsTotal}
package credential_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/credential"
)

// TestPoolStats_InitialState는 초기 풀 상태가 PoolStats에 올바르게 반영되는지 검증한다.
func TestPoolStats_InitialState(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
		{ID: "cred-2", Provider: "anthropic", KeyringID: "kr-2", Status: credential.CredOK},
		{ID: "cred-3", Provider: "openai", KeyringID: "kr-3", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	stats := pool.Stats()
	if stats.Total != 3 {
		t.Errorf("Total: got %d, want 3", stats.Total)
	}
	if stats.Available != 3 {
		t.Errorf("Available: got %d, want 3", stats.Available)
	}
	if stats.Exhausted != 0 {
		t.Errorf("Exhausted: got %d, want 0", stats.Exhausted)
	}
	if stats.SelectsTotal != 0 {
		t.Errorf("SelectsTotal: got %d, want 0", stats.SelectsTotal)
	}
}

// TestPoolStats_AfterSelect는 Select 후 PoolStats가 업데이트되는지 검증한다.
func TestPoolStats_AfterSelect(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
		{ID: "cred-2", Provider: "anthropic", KeyringID: "kr-2", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}
	defer pool.Release(c) //nolint:errcheck

	stats := pool.Stats()
	if stats.SelectsTotal != 1 {
		t.Errorf("SelectsTotal: got %d, want 1", stats.SelectsTotal)
	}
	if stats.Available != 1 {
		t.Errorf("Select 후 Available: got %d, want 1", stats.Available)
	}
}

// TestPoolStats_AfterExhaust는 MarkExhaustedAndRotate 후 PoolStats가 업데이트되는지 검증한다.
func TestPoolStats_AfterExhaust(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-a", Provider: "anthropic", KeyringID: "kr-a", Status: credential.CredOK},
		{ID: "cred-b", Provider: "anthropic", KeyringID: "kr-b", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}

	next, err := pool.MarkExhaustedAndRotate(context.Background(), c.ID, 429, 60*time.Second)
	if err != nil {
		t.Fatalf("MarkExhaustedAndRotate 실패: %v", err)
	}
	defer pool.Release(next) //nolint:errcheck

	stats := pool.Stats()
	if stats.Exhausted != 1 {
		t.Errorf("Exhausted: got %d, want 1", stats.Exhausted)
	}
	if stats.RotationsTotal != 1 {
		t.Errorf("RotationsTotal: got %d, want 1", stats.RotationsTotal)
	}
}

// TestPoolStats_AfterRefresh는 Refresher 성공 후 RefreshesTotal이 증가하는지 검증한다.
func TestPoolStats_AfterRefresh(t *testing.T) {
	t.Parallel()

	now := time.Now()
	creds := []*credential.PooledCredential{
		{
			ID:        "oauth-expiring",
			Provider:  "anthropic",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			ExpiresAt: now.Add(2 * time.Minute), // RefreshMargin 내
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

	stats := pool.Stats()
	if stats.RefreshesTotal != 1 {
		t.Errorf("RefreshesTotal: got %d, want 1", stats.RefreshesTotal)
	}
}

// TestPoolStats_ExhaustedCount는 만료된 크레덴셜이 Exhausted로 계산되는지 검증한다.
func TestPoolStats_ExhaustedCount(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
		{ID: "cred-2", Provider: "anthropic", KeyringID: "kr-2", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}

	if err := pool.MarkExhausted(c, 1*time.Hour); err != nil {
		t.Fatalf("MarkExhausted 실패: %v", err)
	}

	stats := pool.Stats()
	if stats.Exhausted != 1 {
		t.Errorf("Exhausted: got %d, want 1", stats.Exhausted)
	}
	if stats.Available != 1 {
		t.Errorf("Available: got %d, want 1", stats.Available)
	}
	if stats.Total != 2 {
		t.Errorf("Total: got %d, want 2", stats.Total)
	}

	// ErrExhausted 사용 확인 (import 확인)
	_ = errors.Is(credential.ErrExhausted, credential.ErrExhausted)
}
