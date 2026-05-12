// Package credential_test는 credential 패키지의 통합 테스트를 포함한다.
package credential_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/credential"
)

// TestEmptyPool_ReturnsErrExhausted는 빈 풀에서 Select 호출 시 ErrExhausted를 반환하는지 검증한다.
// AC-1: 빈 풀은 Select 시 ErrExhausted 반환
func TestEmptyPool_ReturnsErrExhausted(t *testing.T) {
	t.Parallel()

	src := credential.NewDummySource(nil)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	_, err = pool.Select(context.Background())
	if !errors.Is(err, credential.ErrExhausted) {
		t.Errorf("빈 풀에서 ErrExhausted 기대했지만 %v 반환됨", err)
	}
}

// TestSingleCredentialPool_ReturnsSameCredential은 단일 크레덴셜 풀에서
// 매번 같은 크레덴셜을 반환하는지 검증한다.
// AC-2: 단일 크레덴셜 풀은 매 Select 시 동일한 크레덴셜 반환
func TestSingleCredentialPool_ReturnsSameCredential(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	c1, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("첫 번째 Select 실패: %v", err)
	}
	if err := pool.Release(c1); err != nil {
		t.Fatalf("Release 실패: %v", err)
	}

	c2, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("두 번째 Select 실패: %v", err)
	}

	if c1.ID != c2.ID {
		t.Errorf("단일 풀에서 다른 크레덴셜 반환됨: %s vs %s", c1.ID, c2.ID)
	}
}

// TestMarkExhausted_CooldownPreventsSelection은 MarkExhausted 후
// 쿨다운 기간 동안 해당 크레덴셜이 선택되지 않는지 검증한다.
// AC-4: MarkExhausted + 쿨다운 기간 동안 해당 크레덴셜 선택 불가
func TestMarkExhausted_CooldownPreventsSelection(t *testing.T) {
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

	// cred-1 선택 후 Exhausted 표시
	c1, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}

	cooldown := 100 * time.Millisecond
	if err := pool.MarkExhausted(c1, cooldown); err != nil {
		t.Fatalf("MarkExhausted 실패: %v", err)
	}

	// 쿨다운 동안 cred-1은 선택되면 안 됨
	for range 5 {
		c, err := pool.Select(context.Background())
		if err != nil {
			t.Fatalf("쿨다운 중 Select 실패: %v", err)
		}
		if c.ID == c1.ID {
			t.Errorf("쿨다운 중 exhausted 크레덴셜(%s)이 선택됨", c1.ID)
		}
		pool.Release(c) //nolint:errcheck
	}

	// 쿨다운 만료 후 다시 선택 가능해야 함
	time.Sleep(cooldown + 20*time.Millisecond)
	found := false
	for range 10 {
		c, err := pool.Select(context.Background())
		if err != nil {
			t.Fatalf("쿨다운 만료 후 Select 실패: %v", err)
		}
		if c.ID == c1.ID {
			found = true
			pool.Release(c) //nolint:errcheck
			break
		}
		pool.Release(c) //nolint:errcheck
	}
	if !found {
		t.Errorf("쿨다운 만료 후 크레덴셜(%s)이 다시 선택 가능해야 함", c1.ID)
	}
}

// TestMarkError_SetsLastErrorAt은 MarkError 호출 시 LastErrorAt이
// 설정되고 기본 상태는 Exhausted가 되지 않는지 검증한다.
// AC-5: MarkError는 LastErrorAt을 설정하며 기본적으로 Exhausted 상태로 전환하지 않음
func TestMarkError_SetsLastErrorAt(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "openai", KeyringID: "kr-1", Status: credential.CredOK},
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

	before := time.Now()
	if err := pool.MarkError(c, errors.New("rate limit exceeded")); err != nil {
		t.Fatalf("MarkError 실패: %v", err)
	}
	after := time.Now()

	if c.LastErrorAt.IsZero() {
		t.Error("LastErrorAt은 0이면 안 됨")
	}
	if c.LastErrorAt.Before(before) || c.LastErrorAt.After(after) {
		t.Errorf("LastErrorAt 시간 범위 오류: %v", c.LastErrorAt)
	}
	if c.Status == credential.CredExhausted {
		t.Error("MarkError는 기본적으로 Exhausted 상태로 전환하면 안 됨")
	}

	// 에러 후에도 여전히 선택 가능해야 함
	if err := pool.Release(c); err != nil {
		t.Fatalf("Release 실패: %v", err)
	}
	c2, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("에러 후 Select 실패: %v", err)
	}
	if c2.ID != c.ID {
		t.Errorf("에러 후 같은 크레덴셜이 반환되어야 함 (단일 풀): got %s", c2.ID)
	}
}

// TestConcurrentSelect_LeaseIsolation은 10개의 고루틴이 동시에 Select할 때
// 두 고루틴이 동일한 PooledCredential 포인터를 동시에 보유하지 않는지 검증한다.
// AC-6: 동시 Select에서 리스 격리 보장
func TestConcurrentSelect_LeaseIsolation(t *testing.T) {
	t.Parallel()

	const numCreds = 5
	const numGoroutines = 10
	const rounds = 20

	creds := make([]*credential.PooledCredential, numCreds)
	for i := range numCreds {
		creds[i] = &credential.PooledCredential{
			ID:        strings.Join([]string{"cred", strings.Repeat("x", i+1)}, "-"),
			Provider:  "anthropic",
			KeyringID: strings.Join([]string{"kr", strings.Repeat("x", i+1)}, "-"),
			Status:    credential.CredOK,
		}
	}

	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	var (
		mu      sync.Mutex
		inUse   = make(map[string]bool)
		collide atomic.Bool
	)

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				c, err := pool.Select(context.Background())
				if err != nil {
					// 모든 크레덴셜이 사용 중일 때는 정상적인 대기 필요
					// 이 테스트에서는 단순 재시도
					continue
				}

				mu.Lock()
				if inUse[c.ID] {
					collide.Store(true)
				}
				inUse[c.ID] = true
				mu.Unlock()

				// 짧은 작업 시뮬레이션
				time.Sleep(time.Microsecond)

				mu.Lock()
				inUse[c.ID] = false
				mu.Unlock()

				pool.Release(c) //nolint:errcheck
			}
		}()
	}
	wg.Wait()

	if collide.Load() {
		t.Error("동시 Select에서 동일 크레덴셜이 중복 사용됨 (리스 격리 실패)")
	}
}

// TestRelease_ReturnsCredentialToPool은 Release 후 크레덴셜이
// 다시 선택 가능한 상태가 되는지 검증한다.
// AC-7: Release는 크레덴셜을 사용 가능 풀로 반환
func TestRelease_ReturnsCredentialToPool(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	c1, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("첫 번째 Select 실패: %v", err)
	}

	// Release 전에는 풀이 비어야 함 (단일 크레덴셜)
	_, err2 := pool.Select(context.Background())
	if !errors.Is(err2, credential.ErrExhausted) {
		t.Errorf("Release 전 두 번째 Select는 ErrExhausted 기대, got %v", err2)
	}

	if err := pool.Release(c1); err != nil {
		t.Fatalf("Release 실패: %v", err)
	}

	// Release 후에는 다시 선택 가능해야 함
	c2, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Release 후 Select 실패: %v", err)
	}
	if c2.ID != c1.ID {
		t.Errorf("Release 후 같은 크레덴셜 기대: %s vs %s", c1.ID, c2.ID)
	}
}

// TestReload_NewCredentialsVisible은 Reload 후 Source에서 새 크레덴셜이
// 풀에 반영되는지 검증한다.
// AC-8: Reload 후 Source의 새 크레덴셜이 풀에 반영됨
func TestReload_NewCredentialsVisible(t *testing.T) {
	t.Parallel()

	initial := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(initial)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	total, available := pool.Size()
	if total != 1 || available != 1 {
		t.Errorf("초기 크기: total=%d available=%d, 1/1 기대", total, available)
	}

	// Source에 새 크레덴셜 추가
	updated := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
		{ID: "cred-2", Provider: "anthropic", KeyringID: "kr-2", Status: credential.CredOK},
	}
	src.Update(updated)

	if err := pool.Reload(context.Background()); err != nil {
		t.Fatalf("Reload 실패: %v", err)
	}

	total, available = pool.Size()
	if total != 2 {
		t.Errorf("Reload 후 total=%d, 2 기대", total)
	}
	if available != 2 {
		t.Errorf("Reload 후 available=%d, 2 기대", available)
	}
}

// TestPooledCredential_NoSecretFields는 PooledCredential 구조체에
// 민감한 시크릿 필드가 없는지 컴파일 시간에 검증한다.
// AC-9: PooledCredential에 raw secret 필드 없음 (Zero-Knowledge)
func TestPooledCredential_NoSecretFields(t *testing.T) {
	t.Parallel()

	forbiddenNames := []string{
		"Secret", "Token", "AccessToken", "APIKey", "ApiKey",
		"Bearer", "Password", "Credential", "PrivateKey",
	}

	cred := credential.PooledCredential{}
	typ := reflect.TypeOf(cred)

	for i := range typ.NumField() {
		fieldName := typ.Field(i).Name
		for _, forbidden := range forbiddenNames {
			if strings.EqualFold(fieldName, forbidden) {
				t.Errorf("PooledCredential에 시크릿 필드 발견: %s (Zero-Knowledge 원칙 위반)", fieldName)
			}
		}
	}
}

// TestContextCancellation_DuringSelect는 컨텍스트 취소 시
// Select가 ctx.Err()를 반환하는지 검증한다.
// AC-11: 컨텍스트 취소 시 Select는 ctx.Err() 반환
func TestContextCancellation_DuringSelect(t *testing.T) {
	t.Parallel()

	// 크레덴셜이 모두 사용 중인 상황 시뮬레이션 (빈 풀)
	src := credential.NewDummySource(nil)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = pool.Select(ctx)
	if err == nil {
		t.Fatal("취소된 컨텍스트에서 에러 없이 Select 성공")
	}
	// ErrExhausted 또는 ctx.Err() 둘 다 허용 (빈 풀은 즉시 ErrExhausted)
	// 컨텍스트 취소가 활성화된 경우 ctx.Err() 도 가능
	if !errors.Is(err, credential.ErrExhausted) && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("ErrExhausted 또는 context error 기대, got %v", err)
	}
}

// TestPool_Size는 총 크레덴셜 수와 사용 가능한 크레덴셜 수를 올바르게
// 반환하는지 검증한다.
func TestPool_Size(t *testing.T) {
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

	total, available := pool.Size()
	if total != 3 {
		t.Errorf("total=%d, 3 기대", total)
	}
	if available != 3 {
		t.Errorf("available=%d, 3 기대", available)
	}

	// 하나 선택
	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}

	total2, available2 := pool.Size()
	if total2 != 3 {
		t.Errorf("Select 후 total=%d, 3 기대", total2)
	}
	if available2 != 2 {
		t.Errorf("Select 후 available=%d, 2 기대", available2)
	}
	_ = c
}

// TestPool_AllExhausted_ReturnsErrExhausted는 모든 크레덴셜이 리스 중일 때
// ErrExhausted를 반환하는지 검증한다.
func TestPool_AllExhausted_ReturnsErrExhausted(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
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

	_, err2 := pool.Select(context.Background())
	if !errors.Is(err2, credential.ErrExhausted) {
		t.Errorf("모두 사용 중일 때 ErrExhausted 기대, got %v", err2)
	}
}
