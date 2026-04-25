package core

import (
	"context"
	"runtime/debug"
	"time"

	"go.uber.org/zap"
)

// DrainConsumer는 graceful shutdown 시 CleanupHook 이전에 fan-out 호출되는
// registry-style consumer를 나타낸다.
// TOOLS-001 Registry.Drain(), 후속 SPEC의 SessionRegistry/SubagentSpawner 등이 등록 대상.
// (SPEC-GOOSE-CORE-001 REQ-CORE-014, AC-CORE-011)
type DrainConsumer struct {
	// Name은 로그 식별자다.
	Name string
	// Fn은 실제 drain 작업이다. ctx가 취소되면 조기 종료해야 한다.
	Fn func(ctx context.Context) error
	// Timeout은 이 consumer에 허용되는 최대 실행 시간이다. 기본값 10s.
	Timeout time.Duration
}

// DrainCoordinator는 DrainConsumer 목록을 관리하고 순서대로 실행한다.
// ShutdownManager의 RunAllHooks 이전에 실행되어 in-flight 작업을 마감한다.
type DrainCoordinator struct {
	consumers []DrainConsumer
	logger    *zap.Logger
}

// NewDrainCoordinator는 새 DrainCoordinator를 반환한다.
func NewDrainCoordinator(logger *zap.Logger) *DrainCoordinator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DrainCoordinator{logger: logger}
}

// RegisterDrainConsumer는 drain consumer를 등록한다. 등록 순서대로 실행된다.
func (dc *DrainCoordinator) RegisterDrainConsumer(c DrainConsumer) {
	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}
	dc.consumers = append(dc.consumers, c)
}

// RunAllDrainConsumers는 등록된 모든 drain consumer를 순서대로 실행한다.
// - consumer가 panic하면 ERROR 로그 + stack trace를 기록하고 나머지를 계속 실행한다.
// - consumer가 에러를 반환하면 WARN 로그만 기록하고 진행한다.
// - parentCtx가 만료되면 남은 consumer를 건너뛰고 WARN 로그를 남긴다.
// (SPEC-GOOSE-CORE-001 REQ-CORE-014)
//
// @MX:ANCHOR: [AUTO] shutdown 경로 fan-in (TOOLS-001 Registry.Drain 등 다수 consumer 수렴)
// @MX:REASON: SIGTERM → StateDraining → RunAllDrainConsumers → RunAllHooks 순으로 수렴
// @MX:SPEC: SPEC-GOOSE-CORE-001 REQ-CORE-014
func (dc *DrainCoordinator) RunAllDrainConsumers(parentCtx context.Context) {
	for _, c := range dc.consumers {
		// parentCtx가 이미 만료됐으면 남은 consumer를 건너뛴다.
		select {
		case <-parentCtx.Done():
			dc.logger.Warn("drain timeout: skipping remaining consumers",
				zap.String("skipped_consumer", c.Name),
				zap.Error(parentCtx.Err()),
			)
			return
		default:
		}

		dc.runOne(parentCtx, c)
	}
}

// runOne은 단일 DrainConsumer를 panic-safe하게 실행한다.
func (dc *DrainCoordinator) runOne(parentCtx context.Context, c DrainConsumer) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			dc.logger.Error("drain consumer panicked",
				zap.String("consumer", c.Name),
				zap.Any("panic", r),
				zap.ByteString("stack", stack),
			)
		}
	}()

	ctx, cancel := context.WithTimeout(parentCtx, c.Timeout)
	defer cancel()

	if err := c.Fn(ctx); err != nil {
		dc.logger.Warn("drain consumer returned error",
			zap.String("consumer", c.Name),
			zap.Error(err),
		)
	} else {
		dc.logger.Info("drain consumer completed", zap.String("consumer", c.Name))
	}
}
