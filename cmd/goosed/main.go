// goosed는 GOOSE-AGENT의 핵심 데몬 프로세스다.
// SPEC-GOOSE-CORE-001 — 부트스트랩 및 Graceful Shutdown
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
	// 1. 설정 로드
	cfg, err := config.Load()
	if err != nil {
		// 로거 초기화 전이므로 stderr에 직접 출력
		fallbackLog("ERROR", "config parse error", err.Error())
		return core.ExitConfig
	}

	logLevel := os.Getenv("GOOSE_LOG_LEVEL")
	if logLevel == "" {
		logLevel = cfg.LogLevel
	}

	// 2. 로거 초기화
	logger, err := core.NewLogger(logLevel, "goosed", version)
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

	// 5. 헬스서버 기동
	healthSrv := health.New(rt.State, version, logger)
	if err := healthSrv.ListenAndServe(cfg.HealthPort); err != nil {
		logger.Error("health-port in use",
			zap.Int("port", cfg.HealthPort),
			zap.Error(err),
		)
		return core.ExitConfig
	}

	// 6. serving 상태 전환
	rt.State.Store(core.StateServing)
	// version 필드는 NewLogger에서 이미 With()로 주입되어 있으므로 중복 부여하지 않는다.
	logger.Info("goosed started", zap.Int("health_port", cfg.HealthPort))

	// 7. 시그널 대기 (REQ-CORE-004)
	// rootCtx는 SIGINT/SIGTERM 수신 시 cancel된다.
	// 이 시점 이후 rt.RootCtx.Done()을 구독하는 모든 하위 컴포넌트에 취소가 전파된다.
	<-rootCtx.Done()
	// 시그널 처리 완료 후 stop()을 호출하여 추가 시그널이 기본 동작(즉시 종료)으로 처리되도록 한다.
	stop()

	// 8. draining 상태 전환
	rt.State.Store(core.StateDraining)
	logger.Info("received shutdown signal, draining")

	// 9. 헬스서버 종료 (새 연결 차단, REQ-CORE-008)
	shutdownTimeout := time.Duration(cfg.ShutdownTimeout) * time.Second
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("health server shutdown error", zap.Error(err))
	}

	// 9.5 DrainConsumer fan-out (REQ-CORE-014)
	// CleanupHook 체인 이전에 외부 등록 drain consumer를 호출하여
	// TOOLS-001 Registry.Drain() 등 in-flight 작업을 마감한다.
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
