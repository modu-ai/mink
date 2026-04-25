// goosed는 GOOSE-AGENT의 핵심 데몬 프로세스다.
// SPEC-GOOSE-CORE-001 — 부트스트랩 및 Graceful Shutdown
// SPEC-GOOSE-CONFIG-001 — 계층형 설정 로더 적용
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
	"go.uber.org/zap"
)

// version은 빌드 시 ldflags로 주입된다.
// 예: go build -ldflags "-X main.version=0.1.0"
var version = "dev"

func main() {
	os.Exit(run())
}

// run은 데몬 생애주기를 실행하고 exit code를 반환한다.
// init → bootstrap → serve → shutdown
func run() int {
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

	// 3. Root context 생성 — SIGINT/SIGTERM 수신 시 cancel됨 (REQ-CORE-004(b))
	// signal.NotifyContext를 사용하여 OS 시그널과 context cancellation을 연결한다.
	// 후속 SPEC의 hook은 rt.RootCtx를 구독하여 데몬 생애주기에 참여할 수 있다.
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 4. Runtime 초기화
	rt := core.NewRuntime(logger, rootCtx)
	rt.State.Store(core.StateBootstrap)

	// 5. 헬스서버 기동 — cfg.Transport.HealthPort 사용 (SPEC-GOOSE-CONFIG-001 §6.2)
	healthSrv := health.New(rt.State, version, logger)
	if err := healthSrv.ListenAndServe(cfg.Transport.HealthPort); err != nil {
		logger.Error("health-port in use",
			zap.Int("port", cfg.Transport.HealthPort),
			zap.Error(err),
		)
		return core.ExitConfig
	}

	// 6. serving 상태 전환
	rt.State.Store(core.StateServing)
	logger.Info("goosed started", zap.Int("health_port", cfg.Transport.HealthPort))

	// 7. 시그널 대기 (REQ-CORE-004)
	// rootCtx는 SIGINT/SIGTERM 수신 시 cancel된다.
	<-rootCtx.Done()
	stop()

	// 8. draining 상태 전환
	rt.State.Store(core.StateDraining)
	logger.Info("received shutdown signal, draining")

	// 9. 헬스서버 종료 (REQ-CORE-008)
	// CORE-001 REQ-CORE-004(c): 30초 고정 타임아웃 (SPEC-mandated 상수)
	const shutdownTimeout = 30 * time.Second
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("health server shutdown error", zap.Error(err))
	}

	// 9.5 DrainConsumer fan-out (REQ-CORE-014)
	rt.Drain.RunAllDrainConsumers(shutdownCtx)

	// 10. cleanup hook 실행 (REQ-CORE-004, REQ-CORE-009)
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
