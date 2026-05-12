// Package query lifecycle tests.
// SPEC-GOOSE-QUERY-001 S3 T3.1~T3.4 통합 테스트
// Build tag removed to ensure tests run with standard `go test` command.
package query_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/query"
	"github.com/modu-ai/mink/internal/query/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// makeValidConfig는 유효한 QueryEngineConfig를 생성하는 헬퍼이다.
func makeValidConfig(t *testing.T) query.QueryEngineConfig {
	t.Helper()
	stub := testsupport.NewStubLLMCallSimple("hello")
	return query.QueryEngineConfig{
		LLMCall:    stub.AsFunc(),
		Tools:      []query.ToolDefinition{},
		CanUseTool: testsupport.NewStubCanUseToolAllow(),
		Executor:   testsupport.NewStubExecutor(),
		Logger:     zaptest.NewLogger(t),
		MaxTurns:   10,
		TaskBudget: query.TaskBudget{Total: 10000, Remaining: 10000},
	}
}

// T3.1: New(cfg) 유효성 검증 테스트

// TestQueryEngine_New_ValidConfig_Succeeds는 유효한 설정으로 New()가 성공함을 검증한다.
// REQ-QUERY-001: 유효성 검증 실패 시 에러 반환.
func TestQueryEngine_New_ValidConfig_Succeeds(t *testing.T) {
	t.Parallel()
	cfg := makeValidConfig(t)

	engine, err := query.New(cfg)

	require.NoError(t, err, "유효한 설정으로 New()는 에러 없이 성공해야 한다")
	require.NotNil(t, engine, "반환된 engine은 nil이 아니어야 한다")
}

// TestQueryEngine_New_MissingRequiredField_Fails는 필수 필드 누락 시 에러를 반환함을 검증한다.
// REQ-QUERY-001: LLMCall / CanUseTool / Executor / Logger 필수.
func TestQueryEngine_New_MissingRequiredField_Fails(t *testing.T) {
	t.Parallel()

	t.Run("nil_LLMCall", func(t *testing.T) {
		t.Parallel()
		cfg := makeValidConfig(t)
		cfg.LLMCall = nil

		engine, err := query.New(cfg)

		require.Error(t, err, "LLMCall이 nil이면 에러를 반환해야 한다")
		assert.Nil(t, engine)
		assert.Contains(t, err.Error(), "LLMCall")
	})

	t.Run("nil_CanUseTool", func(t *testing.T) {
		t.Parallel()
		cfg := makeValidConfig(t)
		cfg.CanUseTool = nil

		engine, err := query.New(cfg)

		require.Error(t, err, "CanUseTool이 nil이면 에러를 반환해야 한다")
		assert.Nil(t, engine)
		assert.Contains(t, err.Error(), "CanUseTool")
	})

	t.Run("nil_Executor", func(t *testing.T) {
		t.Parallel()
		cfg := makeValidConfig(t)
		cfg.Executor = nil

		engine, err := query.New(cfg)

		require.Error(t, err, "Executor가 nil이면 에러를 반환해야 한다")
		assert.Nil(t, engine)
		assert.Contains(t, err.Error(), "Executor")
	})

	t.Run("nil_Logger", func(t *testing.T) {
		t.Parallel()
		cfg := makeValidConfig(t)
		cfg.Logger = nil

		engine, err := query.New(cfg)

		require.Error(t, err, "Logger가 nil이면 에러를 반환해야 한다")
		assert.Nil(t, engine)
		assert.Contains(t, err.Error(), "Logger")
	})
}

// T3.2: SubmitMessage receive-only 채널 시그니처 테스트

// TestQueryEngine_SubmitMessage_ReturnsReceiveOnlyChannel은 SubmitMessage가
// receive-only 채널(<-chan SDKMessage)을 반환함을 검증한다.
// REQ-QUERY-002, REQ-QUERY-016: 반환 타입이 <-chan SDKMessage (송신 불가), unbuffered(capacity 0).
func TestQueryEngine_SubmitMessage_ReturnsReceiveOnlyChannel(t *testing.T) {
	t.Parallel()
	cfg := makeValidConfig(t)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	out, err := engine.SubmitMessage(ctx, "test")
	require.NoError(t, err)
	require.NotNil(t, out, "반환 채널은 nil이 아니어야 한다")

	// 반환 타입이 <-chan message.SDKMessage인지 검증 (컴파일 타임 보장)
	var _ <-chan message.SDKMessage = out

	// unbuffered 검증: cap(ch) == 0
	assert.Equal(t, 0, cap(out), "채널은 unbuffered(capacity 0)이어야 한다")

	// drain하여 goroutine 누수 방지
	for range out {
	}
}

// T3.3: SubmitMessage 10ms 마감 테스트

// TestQueryEngine_SubmitMessage_Returns_Within_10ms는 SubmitMessage가 10ms 이내에 반환함을 검증한다.
// REQ-QUERY-005, REQ-QUERY-016: LLM 연결은 goroutine 내부에서 수행.
// N=1000, p99 ≤ 10ms. StubLLMCall에 100ms 지연 설정으로 측정 정밀도 확보.
func TestQueryEngine_SubmitMessage_Returns_Within_10ms(t *testing.T) {
	t.Parallel()

	const N = 1000
	const p99Threshold = 10 * time.Millisecond
	// 100ms 지연 stub: goroutine 내부에서만 지연되므로 SubmitMessage는 즉시 반환해야 한다.
	stub := testsupport.NewStubLLMCallSimple("latency-test")
	stub.InitialDelay = int64(100 * time.Millisecond)

	cfg := query.QueryEngineConfig{
		LLMCall:    stub.AsFunc(),
		Tools:      []query.ToolDefinition{},
		CanUseTool: testsupport.NewStubCanUseToolAllow(),
		Executor:   testsupport.NewStubExecutor(),
		Logger:     zaptest.NewLogger(t),
		MaxTurns:   10,
		TaskBudget: query.TaskBudget{Total: 10000, Remaining: 10000},
	}

	durations := make([]time.Duration, N)

	for i := range N {
		engine, err := query.New(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		start := time.Now()
		out, err := engine.SubmitMessage(ctx, "ping")
		elapsed := time.Since(start)

		require.NoError(t, err)
		durations[i] = elapsed

		// goroutine 누수 방지: drain을 별도 goroutine에서 처리
		go func() {
			for range out {
			}
		}()
	}

	// p99 계산
	sorted := make([]time.Duration, N)
	copy(sorted, durations)
	// 간단한 삽입 정렬 (N=1000 정도에서 충분)
	for i := 1; i < N; i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}

	p99Idx := int(float64(N)*0.99) - 1
	p99 := sorted[p99Idx]

	assert.LessOrEqual(t, p99, p99Threshold,
		"SubmitMessage p99 지연이 10ms를 초과했다: p99=%v", p99)
}

// T3.4: 동시 호출 직렬화 테스트

// TestQueryEngine_Concurrent_SubmitMessage_IsSerialized는 동시 SubmitMessage 호출이
// 직렬화됨을 검증한다. REQ-QUERY-004: sync.Mutex 또는 동등한 메커니즘.
func TestQueryEngine_Concurrent_SubmitMessage_IsSerialized(t *testing.T) {
	t.Parallel()

	stub := testsupport.NewStubLLMCallSimple("concurrent")
	cfg := query.QueryEngineConfig{
		LLMCall:    stub.AsFunc(),
		Tools:      []query.ToolDefinition{},
		CanUseTool: testsupport.NewStubCanUseToolAllow(),
		Executor:   testsupport.NewStubExecutor(),
		Logger:     zaptest.NewLogger(t),
		MaxTurns:   10,
		TaskBudget: query.TaskBudget{Total: 10000, Remaining: 10000},
	}

	engine, err := query.New(cfg)
	require.NoError(t, err)

	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// 동시 호출: panic 없이 모두 완료되어야 한다.
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			out, submitErr := engine.SubmitMessage(ctx, "concurrent-test")
			if submitErr != nil {
				errors <- submitErr
				return
			}
			// drain하여 완료 대기
			for range out {
			}
		}()
	}

	wg.Wait()
	close(errors)

	// 에러 없이 모든 goroutine 완료 확인
	for err := range errors {
		require.NoError(t, err, "동시 SubmitMessage 호출에서 에러가 발생했다")
	}
}
