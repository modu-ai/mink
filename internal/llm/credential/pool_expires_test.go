// Package credential_test는 ExpiresAt 필터링 관련 테스트를 포함한다.
package credential_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/credential"
)

// TestAvailable_FiltersExpiredOAuthTokens는 만료된 OAuth 토큰이 선택 후보에서 제외되는지 검증한다.
// REQ-CREDPOOL-001 (b): OAuth 토큰은 ExpiresAt > now 조건을 만족해야 선택 가능.
func TestAvailable_FiltersExpiredOAuthTokens(t *testing.T) {
	t.Parallel()

	now := time.Now()
	creds := []*credential.PooledCredential{
		// 만료된 OAuth 토큰
		{
			ID:        "oauth-expired",
			Provider:  "anthropic",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			ExpiresAt: now.Add(-1 * time.Hour), // 1시간 전 만료
		},
		// 유효한 OAuth 토큰
		{
			ID:        "oauth-valid-1",
			Provider:  "anthropic",
			KeyringID: "kr-2",
			Status:    credential.CredOK,
			ExpiresAt: now.Add(1 * time.Hour), // 1시간 후 만료
		},
		// 유효한 OAuth 토큰
		{
			ID:        "oauth-valid-2",
			Provider:  "anthropic",
			KeyringID: "kr-3",
			Status:    credential.CredOK,
			ExpiresAt: now.Add(2 * time.Hour), // 2시간 후 만료
		},
	}

	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	// 선택된 크레덴셜 ID 수집 (총 크레덴셜 수 번 선택)
	selected := make(map[string]int)
	for range 10 {
		c, err := pool.Select(context.Background())
		if err != nil {
			if errors.Is(err, credential.ErrExhausted) {
				break
			}
			t.Fatalf("Select 실패: %v", err)
		}
		selected[c.ID]++
		pool.Release(c) //nolint:errcheck
	}

	// 만료된 토큰은 절대 선택되면 안 됨
	if _, ok := selected["oauth-expired"]; ok {
		t.Errorf("만료된 OAuth 토큰(oauth-expired)이 선택됨 — REQ-CREDPOOL-001 (b) 위반")
	}

	// 유효한 토큰은 반드시 선택되어야 함
	if _, ok := selected["oauth-valid-1"]; !ok {
		t.Errorf("유효한 토큰(oauth-valid-1)이 선택되지 않음")
	}
	if _, ok := selected["oauth-valid-2"]; !ok {
		t.Errorf("유효한 토큰(oauth-valid-2)이 선택되지 않음")
	}
}

// TestAvailable_ZeroExpiresAt_IncludedAsNonExpiring은 ExpiresAt가 zero value인
// 크레덴셜(API key 등)이 영구 유효로 처리되어 항상 선택 후보에 포함되는지 검증한다.
// REQ-CREDPOOL-001 (b): ExpiresAt.IsZero() == 영구 유효 (API key).
func TestAvailable_ZeroExpiresAt_IncludedAsNonExpiring(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		// ExpiresAt zero value: API key (영구 유효)
		{
			ID:        "api-key-permanent",
			Provider:  "openai",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			// ExpiresAt 미설정 (zero value)
		},
	}

	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("ExpiresAt zero 크레덴셜 Select 실패: %v", err)
	}
	if c.ID != "api-key-permanent" {
		t.Errorf("예상한 크레덴셜이 아님: got %s, want api-key-permanent", c.ID)
	}

	// Size()의 availableCount()도 동일하게 처리해야 함
	pool.Release(c) //nolint:errcheck
	_, avail := pool.Size()
	if avail != 1 {
		t.Errorf("ExpiresAt zero 크레덴셜은 available로 계산되어야 함: got %d, want 1", avail)
	}
}

// TestAvailable_ExpiresAtInFuture_Included는 만료 시각이 미래인 OAuth 토큰이
// 정상적으로 선택 후보에 포함되는지 검증한다.
func TestAvailable_ExpiresAtInFuture_Included(t *testing.T) {
	t.Parallel()

	futureExpiry := time.Now().Add(24 * time.Hour)
	creds := []*credential.PooledCredential{
		{
			ID:        "oauth-future",
			Provider:  "anthropic",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			ExpiresAt: futureExpiry,
		},
	}

	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("미래 만료 토큰 Select 실패: %v", err)
	}
	if c.ID != "oauth-future" {
		t.Errorf("예상한 크레덴셜이 아님: got %s, want oauth-future", c.ID)
	}

	// Size()의 availableCount()에서도 포함 확인
	pool.Release(c) //nolint:errcheck
	_, avail := pool.Size()
	if avail != 1 {
		t.Errorf("미래 만료 토큰은 available로 계산되어야 함: got %d, want 1", avail)
	}
}

// TestAvailable_ExpiredTokenExcludedFromSize는 만료된 토큰이 Size()의
// availableCount()에서도 제외되는지 검증한다.
func TestAvailable_ExpiredTokenExcludedFromSize(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{
			ID:        "oauth-expired",
			Provider:  "anthropic",
			KeyringID: "kr-1",
			Status:    credential.CredOK,
			ExpiresAt: time.Now().Add(-1 * time.Minute), // 만료됨
		},
		{
			ID:        "api-key",
			Provider:  "anthropic",
			KeyringID: "kr-2",
			Status:    credential.CredOK,
			// ExpiresAt zero: 영구 유효
		},
	}

	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	total, avail := pool.Size()
	if total != 2 {
		t.Errorf("total=%d, 2 기대", total)
	}
	// 만료된 토큰은 available에서 제외되어야 함
	if avail != 1 {
		t.Errorf("만료 토큰 제외 후 available=%d, 1 기대 — REQ-CREDPOOL-001 (b) 위반", avail)
	}
}
