// Package credential_test는 리스 동시성 테스트를 포함한다.
package credential_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/credential"
)

// TestLease_AcquireRelease는 리스 취득과 반환이 올바르게 작동하는지 검증한다.
func TestLease_AcquireRelease(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "cred-1", Provider: "anthropic", KeyringID: "kr-1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	// 리스 취득
	c, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}
	if c == nil {
		t.Fatal("Select가 nil 반환")
	}

	// 리스 반환
	if err := pool.Release(c); err != nil {
		t.Fatalf("Release 실패: %v", err)
	}

	// 반환 후 재사용 가능 확인
	c2, err := pool.Select(context.Background())
	if err != nil {
		t.Fatalf("Release 후 Select 실패: %v", err)
	}
	if c2.ID != c.ID {
		t.Errorf("반환 후 다른 크레덴셜: %s vs %s", c.ID, c2.ID)
	}
}

// TestLease_ContextCancelledBeforeAcquire는 컨텍스트가 이미 취소된 경우
// Select가 즉시 에러를 반환하는지 검증한다.
func TestLease_ContextCancelledBeforeAcquire(t *testing.T) {
	t.Parallel()

	src := credential.NewDummySource(nil)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	_, err = pool.Select(ctx)
	if err == nil {
		t.Fatal("취소된 컨텍스트에서 에러 없이 Select 성공")
	}
}

// TestLease_ConcurrentAcquireRelease는 고루틴들이 동시에 취득/반환할 때
// 데이터 레이스가 발생하지 않는지 검증한다 (-race 플래그로 검출).
func TestLease_ConcurrentAcquireRelease(t *testing.T) {
	t.Parallel()

	const numCreds = 3
	const goroutines = 20
	const ops = 50

	creds := make([]*credential.PooledCredential, numCreds)
	for i := range numCreds {
		creds[i] = &credential.PooledCredential{
			ID:       "cr" + string(rune('1'+i)),
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
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range ops {
				c, err := pool.Select(context.Background())
				if err != nil {
					// 모든 리스가 사용 중이면 재시도
					if errors.Is(err, credential.ErrExhausted) {
						continue
					}
					return
				}
				// 짧은 작업 후 반환
				time.Sleep(time.Microsecond)
				pool.Release(c) //nolint:errcheck
			}
		}()
	}
	wg.Wait()
}

// TestLease_SoftLease_ContextTimeout은 컨텍스트 타임아웃이 리스에
// 올바르게 전파되는지 검증한다.
func TestLease_SoftLease_ContextTimeout(t *testing.T) {
	t.Parallel()

	// 크레덴셜이 없는 풀 (항상 ErrExhausted)
	src := credential.NewDummySource(nil)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	if err != nil {
		t.Fatalf("풀 생성 실패: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = pool.Select(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("타임아웃 컨텍스트에서 에러 없이 Select 성공")
	}
	// 타임아웃이 발생하거나 즉시 ErrExhausted 반환 (빈 풀)
	if elapsed > 100*time.Millisecond {
		t.Errorf("Select가 너무 오래 걸림: %v", elapsed)
	}
}

// TestLease_DoubleFreeProtection은 같은 크레덴셜을 두 번 Release할 때
// 에러가 발생하는지 검증한다.
func TestLease_DoubleFreeProtection(t *testing.T) {
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

	if err := pool.Release(c); err != nil {
		t.Fatalf("첫 번째 Release 실패: %v", err)
	}

	// 두 번째 Release는 에러를 반환해야 함
	err2 := pool.Release(c)
	if err2 == nil {
		t.Error("이중 Release가 에러 없이 성공 (double-free 보호 실패)")
	}
}
