package core

import (
	"context"
	"runtime/debug"
	"time"

	"go.uber.org/zap"
)

// CleanupHook은 graceful shutdown 시 실행할 단일 정리 작업을 나타낸다.
// (SPEC-GOOSE-CORE-001 §7.2)
type CleanupHook struct {
	// Name은 로그 식별자다.
	Name string
	// Fn은 실제 정리 작업이다. ctx가 취소되면 조기 종료해야 한다.
	Fn func(ctx context.Context) error
	// Timeout은 이 hook에 허용되는 최대 실행 시간이다. 기본값 10s.
	Timeout time.Duration
}

// ShutdownManager는 cleanup hook 목록을 관리하고 순서대로 실행한다.
// @MX:ANCHOR: [AUTO] RunAllHooks는 shutdown 경로의 핵심 팬인 지점
// @MX:REASON: SIGTERM 핸들러, panic recovery, exit code 결정이 모두 여기 수렴
type ShutdownManager struct {
	hooks  []CleanupHook
	logger *zap.Logger
}

// NewShutdownManager는 새 ShutdownManager를 반환한다.
func NewShutdownManager(logger *zap.Logger) *ShutdownManager {
	return &ShutdownManager{logger: logger}
}

// RegisterHook은 hook을 등록한다. 등록 순서대로 실행된다.
func (m *ShutdownManager) RegisterHook(h CleanupHook) {
	if h.Timeout == 0 {
		h.Timeout = 10 * time.Second
	}
	m.hooks = append(m.hooks, h)
}

// RunAllHooks는 등록된 모든 hook을 순서대로 실행한다.
// hook이 panic하면 stack trace를 ERROR로 기록하고 나머지 hook을 계속 실행한다.
// panic이 하나라도 발생하면 true를 반환한다.
// parentCtx가 만료되면 남은 hook을 즉시 건너뛰어 30 s 전체 timeout을 보장한다.
// (SPEC-GOOSE-CORE-001 REQ-CORE-009, REQ-CORE-004(c), AC-CORE-005)
//
// @MX:WARN: [AUTO] panic recovery 내부에서 로깅 후 계속 진행
// @MX:REASON: hook 중 하나가 panic해도 나머지 cleanup은 반드시 실행되어야 한다
func (m *ShutdownManager) RunAllHooks(parentCtx context.Context) (panicOccurred bool) {
	for _, h := range m.hooks {
		// parentCtx가 이미 만료됐으면 남은 hook을 건너뛴다.
		// 이를 통해 전체 shutdown이 30 s 이내에 완료됨을 보장한다. (REQ-CORE-004(c))
		select {
		case <-parentCtx.Done():
			m.logger.Warn("shutdown timeout: skipping remaining hooks",
				zap.String("skipped_hook", h.Name),
				zap.Error(parentCtx.Err()),
			)
			return panicOccurred
		default:
		}

		func(hook CleanupHook) {
			defer func() {
				if r := recover(); r != nil {
					panicOccurred = true
					stack := debug.Stack()
					m.logger.Error("cleanup hook panicked",
						zap.String("hook", hook.Name),
						zap.Any("panic", r),
						zap.ByteString("stack", stack),
					)
				}
			}()

			ctx, cancel := context.WithTimeout(parentCtx, hook.Timeout)
			defer cancel()

			if err := hook.Fn(ctx); err != nil {
				m.logger.Error("cleanup hook returned error",
					zap.String("hook", hook.Name),
					zap.Error(err),
				)
			} else {
				m.logger.Info("cleanup hook completed", zap.String("hook", hook.Name))
			}
		}(h)
	}
	return panicOccurred
}
