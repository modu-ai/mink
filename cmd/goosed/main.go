// goosed는 AI.GOOSE의 핵심 데몬 프로세스다.
// SPEC-GOOSE-CORE-001 — 부트스트랩 및 Graceful Shutdown
// SPEC-GOOSE-CONFIG-001 — 계층형 설정 로더 적용
// SPEC-GOOSE-DAEMON-WIRE-001 — 7개 cross-package SPEC wire-up
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modu-ai/goose/internal/config"
	"github.com/modu-ai/goose/internal/core"
	"github.com/modu-ai/goose/internal/health"
	"github.com/modu-ai/goose/internal/hook"
	"go.uber.org/zap"
)

// version은 빌드 시 ldflags로 주입된다.
// 예: go build -ldflags "-X main.version=0.1.0"
var version = "dev"

func main() {
	os.Exit(run())
}

// run은 데몬 생애주기를 실행하고 exit code를 반환한다.
// OS 시그널(SIGINT/SIGTERM)을 수신하여 context를 cancel하고 runWithContext에 위임한다.
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-002
func run() int {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return runWithContext(rootCtx)
}

// runWithContext는 13-step wire-up 생애주기를 실행하고 exit code를 반환한다.
// init → bootstrap → wire-up → serve → drain → shutdown (13-step)
// ctx가 cancel되면 shutdown 시퀀스가 시작된다.
//
// @MX:ANCHOR: [AUTO] goosed 13-step 생애주기 진입점 — 모든 wire-up 경로가 여기를 통과
// @MX:REASON: SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-002 — main, run, runWithHooks(test) 3곳에서 호출
func runWithContext(ctx context.Context) int {
	// 1. 설정 로드 (SPEC-GOOSE-CONFIG-001 계층형 로더)
	cfg, err := config.Load(config.LoadOptions{})
	if err != nil {
		// 로거 초기화 전이므로 stderr에 직접 출력
		fallbackLog("ERROR", "config parse error", err.Error())
		return core.ExitConfig
	}

	// 2. 로거 초기화 — cfg.Log.Level 사용 (SPEC-GOOSE-CONFIG-001 §6.2)
	logger, err := core.NewLogger(cfg.Log.Level, "goosed", version)
	if err != nil {
		fallbackLog("ERROR", "logger init failed", err.Error())
		return core.ExitConfig
	}
	defer logger.Sync() //nolint:errcheck

	// 3. Root context — 호출자가 제공한 ctx 사용 (REQ-CORE-004(b))
	rootCtx := ctx

	// 4. Runtime 초기화
	rt := core.NewRuntime(logger, rootCtx)
	rt.State.Store(core.StateBootstrap)

	// 5~7. Registries wire-up (hook, tools, skill)
	hookRegistry, toolsRegistry, skillRegistry := wireRegistries(cfg.SkillsRoot, logger)

	// 8~10. Consumer wire-up (WorkspaceRoot adapter, Drain, FileChanged)
	if err := wireConsumers(rt, hookRegistry, toolsRegistry, skillRegistry, logger); err != nil {
		return core.ExitConfig
	}

	// InteractiveHandler placeholder (REQ-WIRE-009)
	wireInteractiveHandler(rt, hookRegistry, nil, hook.WithExplicitNoOp())

	// 11. 헬스서버 기동 — cfg.Transport.HealthPort 사용 (SPEC-GOOSE-CONFIG-001 §6.2)
	healthSrv := health.New(rt.State, version, logger)
	if err := healthSrv.ListenAndServe(cfg.Transport.HealthPort); err != nil {
		logger.Error("health-port in use",
			zap.Int("port", cfg.Transport.HealthPort),
			zap.Error(err),
		)
		return core.ExitConfig
	}

	// 12. serving 상태 전환
	rt.State.Store(core.StateServing)
	logger.Info("goosed started", zap.Int("health_port", cfg.Transport.HealthPort))

	// 13. 시그널 대기 (REQ-CORE-004)
	<-rootCtx.Done()

	// shutdown: drain → cleanup → stop
	rt.State.Store(core.StateDraining)
	logger.Info("received shutdown signal, draining")

	const shutdownTimeout = 30 * time.Second
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("health server shutdown error", zap.Error(err))
	}

	rt.Drain.RunAllDrainConsumers(shutdownCtx)

	panicOccurred := rt.Shutdown.RunAllHooks(shutdownCtx)

	rt.State.Store(core.StateStopped)
	logger.Info("goosed stopped")

	if panicOccurred {
		return core.ExitHookPanic
	}
	return core.ExitOK
}

// fallbackLog는 로거 초기화 전 stderr에 최소 JSON 형식으로 출력한다.
func fallbackLog(level, msg, detail string) {
	_, _ = os.Stderr.WriteString(`{"level":"` + level + `","msg":"` + msg + `","detail":"` + detail + `"}` + "\n")
}
