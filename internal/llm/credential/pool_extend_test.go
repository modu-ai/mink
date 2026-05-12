// T-007 CREDPOOL 확장 테스트: MarkExhaustedAndRotate + AcquireLease
package credential_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/credential"
)

// TestPool_MarkExhaustedAndRotate_Success는 MarkExhaustedAndRotate가
// 현재 크레덴셜을 exhausted로 표시하고 다음 크레덴셜을 반환하는지 검증한다.
// AC-ADAPTER-008 coverage
func TestPool_MarkExhaustedAndRotate_Success(t *testing.T) {
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

	// cred-a 선택
	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}
	if c.ID != "cred-a" {
		t.Fatalf("첫 번째 Select: got %q, want %q", c.ID, "cred-a")
	}

	// cred-a를 exhausted 처리하고 다음 크레덴셜 획득
	next, err := pool.MarkExhaustedAndRotate(context.Background(), c.ID, 429, 120*time.Second)
	if err != nil {
		t.Fatalf("MarkExhaustedAndRotate 실패: %v", err)
	}
	if next == nil {
		t.Fatal("MarkExhaustedAndRotate: nil 반환 — cred-b가 있어야 함")
	}
	if next.ID != "cred-b" {
		t.Errorf("rotated credential: got %q, want %q", next.ID, "cred-b")
	}
}

// TestPool_MarkExhaustedAndRotate_LastCred는 마지막 크레덴셜 exhausted 시
// ErrExhausted를 반환하는지 검증한다.
func TestPool_MarkExhaustedAndRotate_LastCred(t *testing.T) {
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

	_, err = pool.MarkExhaustedAndRotate(context.Background(), c.ID, 429, 60*time.Second)
	if !errors.Is(err, credential.ErrExhausted) {
		t.Errorf("단일 크레덴셜 exhausted: got %v, want ErrExhausted", err)
	}
}

// TestPool_MarkExhaustedAndRotate_NotFound는 존재하지 않는 ID 시 에러를 검증한다.
func TestPool_MarkExhaustedAndRotate_NotFound(t *testing.T) {
	t.Parallel()

	src := credential.NewDummySource(nil)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	_, err = pool.MarkExhaustedAndRotate(context.Background(), "nonexistent", 429, 60*time.Second)
	if err == nil {
		t.Error("존재하지 않는 ID에서 에러 없이 성공")
	}
}

// TestPool_AcquireLease_IsolatesCredential은 AcquireLease가
// 크레덴셜을 리스 상태로 만들고 Release가 해제하는지 검증한다.
func TestPool_AcquireLease_IsolatesCredential(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	// Select로 크레덴셜 획득
	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}

	// AcquireLease로 리스 획득
	lease := pool.AcquireLease(c.ID)
	if lease == nil {
		t.Fatal("AcquireLease: nil 반환")
	}

	// 리스 중에는 같은 크레덴셜을 다시 선택할 수 없어야 함
	_, err2 := pool.Select(context.Background())
	if !errors.Is(err2, credential.ErrExhausted) {
		t.Errorf("리스 중 재선택: got %v, want ErrExhausted", err2)
	}

	// Release 후 다시 선택 가능
	lease.Release()
	c2, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Release 후 Select 실패: %v", err)
	}
	if c2.ID != c.ID {
		t.Errorf("Release 후 크레덴셜: got %q, want %q", c2.ID, c.ID)
	}
}

// TestPool_AcquireLease_Concurrent는 AcquireLease/Release의 동시성을 검증한다.
func TestPool_AcquireLease_Concurrent(t *testing.T) {
	t.Parallel()

	creds := make([]*credential.PooledCredential, 3)
	for i := range 3 {
		creds[i] = &credential.PooledCredential{
			ID:       "cred-" + string(rune('a'+i)),
			Provider: "anthropic",
			Status:   credential.CredOK,
		}
	}

	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				c, err := pool.Select(context.Background())
				if err != nil {
					continue
				}
				lease := pool.AcquireLease(c.ID)
				time.Sleep(time.Microsecond)
				if lease != nil {
					lease.Release()
				} else {
					pool.Release(c) //nolint:errcheck
				}
			}
		}()
	}
	wg.Wait()
}
