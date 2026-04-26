// Package credential_test는 Pool.Reset(id) 운영자 수동 영구 고갈 해제 테스트를 포함한다.
// OI-08: Pool.Reset(id) 운영자 수동 영구 고갈 해제 API
package credential_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/credential"
)

// TestPool_Reset_ClearsExhaustedState는 Reset이 Exhausted 상태를 해제하는지 검증한다.
// OI-08: Pool.Reset(id)
func TestPool_Reset_ClearsExhaustedState(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	// cred-1 선택 후 1시간 쿨다운 exhausted 처리
	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}
	if err := pool.MarkExhausted(c, 1*time.Hour); err != nil {
		t.Fatalf("MarkExhausted 실패: %v", err)
	}

	// Exhausted 상태에서 Select 불가
	_, err = pool.Select(context.Background())
	if !errors.Is(err, credential.ErrExhausted) {
		t.Errorf("Exhausted 후 Select: got %v, want ErrExhausted", err)
	}

	// Reset으로 해제
	if err := pool.Reset("cred-1"); err != nil {
		t.Fatalf("Reset 실패: %v", err)
	}

	// Reset 후 다시 선택 가능
	c2, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Reset 후 Select 실패: %v", err)
	}
	if c2.ID != "cred-1" {
		t.Errorf("Reset 후 Select: got %q, want %q", c2.ID, "cred-1")
	}
}

// TestPool_Reset_PermanentExhaust는 영구 고갈 상태도 Reset으로 해제되는지 검증한다.
func TestPool_Reset_PermanentExhaust(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-only", Provider: "anthropic", KeyringID: "kr-only", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	// 402 → 영구 고갈
	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}
	_, _ = pool.MarkExhaustedAndRotate(context.Background(), c.ID, 402, 0)

	// 영구 고갈 확인
	_, err = pool.Select(context.Background())
	if !errors.Is(err, credential.ErrExhausted) {
		t.Errorf("402 영구 고갈 후 Select: got %v, want ErrExhausted", err)
	}

	// Reset
	if err := pool.Reset("cred-only"); err != nil {
		t.Fatalf("영구 고갈 Reset 실패: %v", err)
	}

	// 다시 선택 가능
	c2, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("영구 고갈 Reset 후 Select 실패: %v", err)
	}
	if c2.ID != "cred-only" {
		t.Errorf("Reset 후 Select: got %q, want %q", c2.ID, "cred-only")
	}
}

// TestPool_Reset_NotFound는 존재하지 않는 ID에 Reset 시 에러를 반환하는지 검증한다.
func TestPool_Reset_NotFound(t *testing.T) {
	t.Parallel()

	src := credential.NewDummySource(nil)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	err = pool.Reset("nonexistent")
	if err == nil {
		t.Error("존재하지 않는 ID Reset: 에러 기대")
	}
	if !errors.Is(err, credential.ErrNotFound) {
		t.Errorf("ErrNotFound 기대: got %v", err)
	}
}
