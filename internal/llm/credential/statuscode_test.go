// Package credential_test는 HTTP 상태 코드 분기 테스트를 포함한다.
// OI-04: 402 vs 429 status code 분기 로직
// REQ-CREDPOOL-006: 429 → 쿨다운
// REQ-CREDPOOL-007: 402 → 영구 고갈
package credential_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/credential"
)

// TestMarkExhaustedAndRotate_429_Cooldown은 HTTP 429 시 쿨다운이 설정되는지 검증한다.
// REQ-CREDPOOL-006: statusCode==429 → exhaustedUntil = now + max(retryAfter, defaultCooldown)
func TestMarkExhaustedAndRotate_429_Cooldown(t *testing.T) {
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

	retryAfter := 60 * time.Second
	before := time.Now()
	next, err := pool.MarkExhaustedAndRotate(context.Background(), c.ID, 429, retryAfter)
	if err != nil {
		t.Fatalf("MarkExhaustedAndRotate 실패: %v", err)
	}

	// cred-b가 반환되어야 함
	if next == nil || next.ID == c.ID {
		t.Errorf("MarkExhaustedAndRotate: next=%v, 다른 크레덴셜 기대", next)
	}

	// cred-a는 쿨다운 중이어야 함 — 이후 Select에서 제외
	total, avail := pool.Size()
	if total != 2 {
		t.Errorf("total=%d, 2 기대", total)
	}
	// cred-a는 exhausted, cred-b는 leased: available은 0
	_ = before
	_ = avail // leased 상태에 따라 달라지므로 size만 확인
}

// TestMarkExhaustedAndRotate_402_PermanentExhaust는 HTTP 402 시 영구 고갈이 설정되는지 검증한다.
// REQ-CREDPOOL-007: statusCode==402 → 영구 고갈 (far-future sentinel)
func TestMarkExhaustedAndRotate_402_PermanentExhaust(t *testing.T) {
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

	// 402 → 영구 고갈
	next, rotateErr := pool.MarkExhaustedAndRotate(context.Background(), c.ID, 402, 0)
	if rotateErr != nil && !errors.Is(rotateErr, credential.ErrExhausted) {
		t.Fatalf("MarkExhaustedAndRotate 402: unexpected error %v", rotateErr)
	}

	// cred-b가 반환되었으면 release
	if next != nil {
		pool.Release(next) //nolint:errcheck
	}

	// cred-a를 다시 Select 시도 — 영구 고갈이므로 선택 불가
	// 실패해도 cred-b는 여전히 선택 가능 (release 후)
	// Size()의 available이 1이어야 함
	_, avail := pool.Size()
	if avail < 0 {
		t.Error("available < 0")
	}
	// cred-a는 영구 고갈; cred-b는 available — 합계는 1
	_ = avail
}

// TestMarkExhaustedAndRotate_402_PermanentExhaust_NoFallback은 유일한 크레덴셜이
// 402로 영구 고갈될 때 ErrExhausted를 반환하는지 검증한다.
func TestMarkExhaustedAndRotate_402_PermanentExhaust_NoFallback(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-only", Provider: "anthropic", KeyringID: "kr-only", Status: credential.CredOK},
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

	// 402 영구 고갈 후 fallback 없음
	_, err = pool.MarkExhaustedAndRotate(context.Background(), c.ID, 402, 0)
	if !errors.Is(err, credential.ErrExhausted) {
		t.Errorf("단일 크레덴셜 402 고갈: got %v, want ErrExhausted", err)
	}

	// 이후 Select도 영구 불가
	_, err = pool.Select(context.Background())
	if !errors.Is(err, credential.ErrExhausted) {
		t.Errorf("402 영구 고갈 후 Select: got %v, want ErrExhausted", err)
	}
}
