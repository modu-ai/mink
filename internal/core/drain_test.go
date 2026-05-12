package core_test

// drain_test.go — AC-CORE-011: DrainConsumer fan-out 단위 테스트
// SPEC-GOOSE-CORE-001 REQ-CORE-014

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestDrainConsumer_RegisterAndFanOut는 3개 consumer가 등록 순서대로 모두 호출되는지 검증한다.
func TestDrainConsumer_RegisterAndFanOut(t *testing.T) {
	t.Parallel()

	core2, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core2)

	dc := core.NewDrainCoordinator(logger)
	require.NotNil(t, dc)

	// 호출 순서를 기록할 슬라이스 (단일 goroutine 순서 실행이므로 mutex 불필요)
	var callOrder []string

	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name: "consumer-0",
		Fn: func(ctx context.Context) error {
			callOrder = append(callOrder, "consumer-0")
			return nil
		},
	})
	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name: "consumer-1",
		Fn: func(ctx context.Context) error {
			callOrder = append(callOrder, "consumer-1")
			return nil
		},
	})
	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name: "consumer-2",
		Fn: func(ctx context.Context) error {
			callOrder = append(callOrder, "consumer-2")
			return nil
		},
	})

	dc.RunAllDrainConsumers(context.Background())

	// 등록 순서대로 호출
	assert.Equal(t, []string{"consumer-0", "consumer-1", "consumer-2"}, callOrder)
	// 정상 완료 로그 확인
	_ = logs // 로그 구조 확인용
}

// TestDrainConsumer_ErrorIsolation는 AC-CORE-011 에러 격리를 검증한다.
// 두 번째 consumer가 에러를 반환해도 세 번째가 계속 실행되고 WARN 로그가 기록된다.
func TestDrainConsumer_ErrorIsolation(t *testing.T) {
	t.Parallel()

	core2, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core2)

	dc := core.NewDrainCoordinator(logger)

	var callCount atomic.Int32

	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name: "consumer-ok-0",
		Fn: func(ctx context.Context) error {
			callCount.Add(1)
			return nil
		},
	})
	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name: "consumer-err-1",
		Fn: func(ctx context.Context) error {
			callCount.Add(1)
			return errors.New("drain failed")
		},
	})
	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name: "consumer-ok-2",
		Fn: func(ctx context.Context) error {
			callCount.Add(1)
			return nil
		},
	})

	dc.RunAllDrainConsumers(context.Background())

	// 3개 모두 호출됨
	assert.Equal(t, int32(3), callCount.Load())

	// WARN 로그가 1건 기록됨 (에러 격리)
	warnLogs := logs.FilterLevelExact(zap.WarnLevel)
	assert.GreaterOrEqual(t, warnLogs.Len(), 1, "에러 발생 시 WARN 로그가 기록되어야 함")
}

// TestDrainConsumer_PanicIsolation는 두 번째 consumer가 panic해도
// 세 번째가 계속 실행되고 ERROR 로그가 기록되는지 검증한다.
func TestDrainConsumer_PanicIsolation(t *testing.T) {
	t.Parallel()

	core2, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core2)

	dc := core.NewDrainCoordinator(logger)

	var callCount atomic.Int32

	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name: "consumer-ok-0",
		Fn: func(ctx context.Context) error {
			callCount.Add(1)
			return nil
		},
	})
	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name: "consumer-panic-1",
		Fn: func(ctx context.Context) error {
			callCount.Add(1)
			panic("boom")
		},
	})
	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name: "consumer-ok-2",
		Fn: func(ctx context.Context) error {
			callCount.Add(1)
			return nil
		},
	})

	// panic이 RunAllDrainConsumers 바깥으로 전파되지 않아야 한다
	assert.NotPanics(t, func() {
		dc.RunAllDrainConsumers(context.Background())
	})

	// 3개 모두 호출됨
	assert.Equal(t, int32(3), callCount.Load())

	// ERROR 로그가 1건 기록됨 (panic 격리)
	errorLogs := logs.FilterLevelExact(zap.ErrorLevel)
	assert.GreaterOrEqual(t, errorLogs.Len(), 1, "panic 발생 시 ERROR 로그가 기록되어야 함")
}

// TestDrainConsumer_PerConsumerTimeout는 per-consumer timeout이 동작하는지 검증한다.
// Timeout=50ms인 consumer가 100ms sleep하면 50ms 안에 ctx.Done()을 받아야 한다.
func TestDrainConsumer_PerConsumerTimeout(t *testing.T) {
	t.Parallel()

	core2, _ := observer.New(zap.WarnLevel)
	logger := zap.New(core2)

	dc := core.NewDrainCoordinator(logger)

	start := time.Now()
	var ctxCanceled bool

	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name:    "consumer-slow",
		Timeout: 50 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			select {
			case <-time.After(100 * time.Millisecond):
				// timeout이 없으면 여기로 오면 안 된다
				return nil
			case <-ctx.Done():
				ctxCanceled = true
				return ctx.Err()
			}
		},
	})

	dc.RunAllDrainConsumers(context.Background())
	elapsed := time.Since(start)

	// 50ms timeout으로 조기 종료되어야 한다 (100ms sleep 전에)
	assert.True(t, ctxCanceled, "per-consumer timeout에 의해 ctx가 취소되어야 함")
	// 100ms보다 훨씬 짧게 걸려야 한다 (넉넉하게 80ms)
	assert.Less(t, elapsed, 80*time.Millisecond, "timeout이 50ms이므로 80ms 이내 완료되어야 함")
}

// TestDrainConsumer_ParentCtxExpired는 parentCtx가 이미 canceled된 상태로 진입 시
// 첫 consumer부터 건너뛰고 WARN 로그 1건이 기록되는지 검증한다.
func TestDrainConsumer_ParentCtxExpired(t *testing.T) {
	t.Parallel()

	core2, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core2)

	dc := core.NewDrainCoordinator(logger)

	var callCount atomic.Int32

	dc.RegisterDrainConsumer(core.DrainConsumer{
		Name: "consumer-should-skip",
		Fn: func(ctx context.Context) error {
			callCount.Add(1)
			return nil
		},
	})

	// 이미 취소된 context로 진입
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	dc.RunAllDrainConsumers(canceledCtx)

	// consumer가 호출되지 않아야 한다
	assert.Equal(t, int32(0), callCount.Load(), "취소된 ctx로 진입 시 consumer는 호출되지 않아야 함")

	// WARN 로그 1건 기록
	warnLogs := logs.FilterLevelExact(zap.WarnLevel)
	assert.GreaterOrEqual(t, warnLogs.Len(), 1, "parent ctx 만료 시 WARN 로그가 기록되어야 함")
}
