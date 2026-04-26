// wire_test_helpers_test.go는 integration_test.go에서 사용하는 테스트 전용 헬퍼를 정의한다.
// SPEC-GOOSE-DAEMON-WIRE-001
package main

import (
	"context"
	"time"

	"github.com/modu-ai/goose/internal/config"
	"github.com/modu-ai/goose/internal/core"
	"github.com/modu-ai/goose/internal/health"
	"github.com/modu-ai/goose/internal/hook"
	"github.com/modu-ai/goose/internal/skill"
	"github.com/modu-ai/goose/internal/tools"
	"go.uber.org/zap"
)

// wireCapture는 runWithHooks가 테스트에 노출하는 wire-up 결과다.
type wireCapture struct {
	rt            *core.Runtime
	hookRegistry  *hook.HookRegistry
	toolsRegistry *tools.Registry
	skillRegistry *skill.SkillRegistry
}

// runWithHooks는 테스트 가능한 run() 변형이다.
// gooseHome을 GOOSE_HOME으로 사용하여 13-step wire-up을 실행한다.
// readyCh는 StateServing 도달 시 신호를 보낸다.
// cancelCh close 시 rootCtx가 cancel되어 shutdown이 시작된다.
// captureFn을 통해 wire-up된 레지스트리를 테스트에 노출한다.
func runWithHooks(gooseHome string, readyCh chan<- struct{}, cancelCh <-chan struct{}, captureFn func(*wireCapture)) int {
	// 1. 설정 로드
	cfg, err := config.Load(config.LoadOptions{GooseHome: gooseHome})
	if err != nil {
		return core.ExitConfig
	}

	// 2. 로거 (테스트에서는 nop 사용)
	logger := zap.NewNop()

	// 3. Root context — cancelCh를 통해 테스트가 제어
	rootCtx, cancelFn := context.WithCancel(context.Background())
	// cancelCh close 시 rootCtx cancel
	go func() {
		<-cancelCh
		cancelFn()
	}()
	defer cancelFn()

	// 4. Runtime 초기화
	rt := core.NewRuntime(logger, rootCtx)
	rt.State.Store(core.StateBootstrap)

	// 5~7. Registries wire-up (wireRegistries 재사용)
	hookRegistry, toolsRegistry, skillRegistry := wireRegistries(cfg.SkillsRoot, logger)

	// captureFn 호출 — 레지스트리 노출
	if captureFn != nil {
		captureFn(&wireCapture{
			rt:            rt,
			hookRegistry:  hookRegistry,
			toolsRegistry: toolsRegistry,
			skillRegistry: skillRegistry,
		})
	}

	// 8~10. Consumer wire-up (wireConsumers 재사용)
	if err := wireConsumers(rt, hookRegistry, toolsRegistry, skillRegistry, logger); err != nil {
		return core.ExitConfig
	}

	// InteractiveHandler placeholder
	wireInteractiveHandler(rt, hookRegistry, nil, hook.WithExplicitNoOp())

	// 11. 헬스서버 기동 (포트 0 = OS가 자동 할당)
	healthSrv := health.New(rt.State, "test", logger)
	if err := healthSrv.ListenAndServe(cfg.Transport.HealthPort); err != nil {
		return core.ExitConfig
	}

	// 12. serving 상태 전환
	rt.State.Store(core.StateServing)

	// readyCh 신호 — 테스트에게 StateServing 도달을 알림
	select {
	case readyCh <- struct{}{}:
	default:
	}

	// 13. 신호 대기
	<-rootCtx.Done()

	// shutdown
	rt.State.Store(core.StateDraining)

	const shutdownTimeout = 10 * time.Second
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		_ = err // 테스트에서는 무시
	}

	rt.Drain.RunAllDrainConsumers(shutdownCtx)
	rt.Shutdown.RunAllHooks(shutdownCtx)

	rt.State.Store(core.StateStopped)
	return core.ExitOK
}

// runWithNilConsumerPath는 nil consumer wire-up 시 exit code를 반환한다.
// AC-WIRE-006 전용: nil consumer → ErrInvalidConsumer → ExitConfig 경로를 검증한다.
func runWithNilConsumerPath(gooseHome string) int {
	_, err := config.Load(config.LoadOptions{GooseHome: gooseHome})
	if err != nil {
		return core.ExitConfig
	}

	logger := zap.NewNop()
	hookRegistry := hook.NewHookRegistry()

	// nil consumer를 직접 등록 시도 → ErrInvalidConsumer
	if err := hookRegistry.SetSkillsFileChangedConsumer(nil); err != nil {
		logger.Error("wire-up failed: nil skills consumer", zap.Error(err))
		return core.ExitConfig
	}

	// 여기 도달하면 fail-fast가 작동하지 않은 것
	return core.ExitOK
}
