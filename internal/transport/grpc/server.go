// Package grpc는 goosed 데몬의 gRPC 서버를 제공한다.
// SPEC-GOOSE-TRANSPORT-001 — gRPC 서버/proto 스키마 기본 계약
package grpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/modu-ai/mink/internal/core"
	"github.com/modu-ai/mink/internal/envalias"
	"github.com/modu-ai/mink/internal/transport/grpc/gen/minkv1"
)

// Config는 gRPC 서버 설정이다.
type Config struct {
	// BindAddr는 gRPC 서버가 바인딩할 주소다 (기본값: "127.0.0.1:9005").
	// REQ-TR-001: 기본값은 loopback.
	BindAddr string

	// ShutdownToken은 Shutdown RPC 인증 토큰이다.
	// 빈 문자열이면 환경변수 MINK_SHUTDOWN_TOKEN (legacy: GOOSE_SHUTDOWN_TOKEN)에서 읽는다.
	// REQ-TR-011: 최종적으로 빈 문자열이면 Shutdown RPC는 Unimplemented를 반환한다.
	// 테스트에서 환경변수 격리가 필요하면 ShutdownTokenOverride=true + ShutdownToken="" 조합 사용.
	ShutdownToken string

	// ShutdownTokenOverride가 true이면 ShutdownToken 값을 그대로 사용하고
	// 환경변수 MINK_SHUTDOWN_TOKEN (legacy: GOOSE_SHUTDOWN_TOKEN)을 완전히 무시한다.
	// 테스트 환경에서 환경변수 오염을 방지하기 위한 필드.
	ShutdownTokenOverride bool

	// MaxRecvMsgBytes는 gRPC 최대 수신 메시지 크기다.
	// 0이면 환경변수 MINK_GRPC_MAX_RECV_MSG_BYTES (legacy: GOOSE_GRPC_MAX_RECV_MSG_BYTES)에서 읽고, 그것도 없으면 4MiB.
	// REQ-TR-014
	MaxRecvMsgBytes int

	// EnableReflection은 gRPC reflection 서비스 활성화 여부다.
	// false이면 환경변수 MINK_GRPC_REFLECTION=true (legacy: GOOSE_GRPC_REFLECTION) 시 활성화.
	// REQ-TR-009
	EnableReflection bool

	// RegisterPanicTestService is a test-only flag that registers the internal
	// PanicTestService BEFORE Serve is started. This must be set via Config
	// because gRPC fatals on RegisterService-after-Serve.
	// AC-TR-006: integration test only.
	RegisterPanicTestService bool
}

// Server는 goosed gRPC 서버의 래퍼 타입이다.
// @MX:ANCHOR: [AUTO] gRPC 서버의 핵심 진입점 — NewServer, Serve, Stop 팬인 ≥ 3
// @MX:REASON: daemon_service, interceptors, health check가 모두 이 서버를 경유함
type Server struct {
	grpcSrv          *grpc.Server
	lis              net.Listener
	logger           *zap.Logger
	state            *core.StateHolder
	startTime        time.Time
	healthSrv        *health.Server
	interceptorNames []string // interceptor 체인 이름 (AC-TR-013 검증용)
	cancel           context.CancelFunc
	// gracefulStopBlocker는 테스트에서 GracefulStop을 블로킹하기 위한 채널이다.
	// nil이면 블로킹 없음. non-nil이면 close될 때까지 GracefulStop을 대기시킨다.
	gracefulStopBlocker <-chan struct{}
}

// NewServer는 gRPC 서버를 초기화하고 listener를 바인딩한다.
// rootCtx가 cancel되면 GracefulStop을 시작한다.
//
// @MX:ANCHOR: [AUTO] 모든 transport 컴포넌트의 초기화 지점
// @MX:REASON: daemon_service, interceptors, health, reflection 등 모든 등록이 여기서 발생
func NewServer(cfg Config, logger *zap.Logger, state *core.StateHolder, rootCtx context.Context) (*Server, error) {
	// rootCtx cancel 시 GracefulStop 트리거를 위한 내부 cancel 획득
	// NewServerWithCancel에서 cancel을 주입하는 경우 외에는 context cancel을 추적하지 않음
	return newServerInternal(cfg, logger, state, rootCtx, nil)
}

// NewServerWithCancel은 rootCancel 함수를 명시적으로 주입하는 생성자다.
// Shutdown RPC가 rootCancel을 호출하여 daemon을 종료한다.
// AC-TR-004 테스트에서 cancel 호출 감지를 위해 사용.
func NewServerWithCancel(cfg Config, logger *zap.Logger, state *core.StateHolder, rootCtx context.Context, rootCancel context.CancelFunc) (*Server, error) {
	return newServerInternal(cfg, logger, state, rootCtx, rootCancel)
}

// newServerInternal은 공통 초기화 로직이다.
//
// @MX:WARN: [AUTO] interceptor 체인 순서가 잘못되면 패닉이 아닌 서버 abort
// @MX:REASON: AC-TR-013 — RecoveryInterceptor는 반드시 outermost(index 0)이어야 함
func newServerInternal(cfg Config, logger *zap.Logger, state *core.StateHolder, rootCtx context.Context, rootCancel context.CancelFunc) (*Server, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	bindAddr := cfg.BindAddr
	if bindAddr == "" {
		bindAddr = "127.0.0.1:9005"
	}

	// listener 바인딩 (REQ-TR-001: loopback 기본)
	lis, err := net.Listen("tcp", bindAddr)
	if err != nil {
		// REQ-TR-007 / spec §6.5: listener 실패 시 exit 78
		logger.Error("grpc listener bind 실패 — exit 78",
			zap.String("addr", bindAddr),
			zap.Error(err),
		)
		os.Exit(78)
		return nil, fmt.Errorf("grpc listener bind 실패: %w", err)
	}

	maxRecvBytes := resolveMaxRecvBytes(cfg.MaxRecvMsgBytes)
	shutdownToken := resolveShutdownToken(cfg.ShutdownToken, cfg.ShutdownTokenOverride)

	// rootCancel이 nil이면 rootCtx에서 파생 cancel 생성
	// Shutdown RPC가 cancel을 호출하여 daemon을 종료한다.
	cancel := rootCancel
	if cancel == nil {
		_, cancel = context.WithCancel(rootCtx)
	}

	s := &Server{
		logger:    logger,
		state:     state,
		startTime: time.Now(),
		cancel:    cancel,
	}

	// Interceptor 체인 구성 (AC-TR-013: Recovery가 outermost)
	// interceptorNames는 chain 순서 검증용 (index 0 = outermost)
	s.interceptorNames = []string{"recovery", "logging"}

	recoveryOpts := []grpcmiddleware.Option{
		grpcmiddleware.WithRecoveryHandlerContext(func(ctx context.Context, p any) error {
			return recoverPanic(ctx, p, logger)
		}),
	}

	chainedInterceptor := grpc.ChainUnaryInterceptor(
		grpcmiddleware.UnaryServerInterceptor(recoveryOpts...), // index 0: Recovery (outermost)
		newLoggingInterceptor(logger),                          // index 1: Logging
	)

	grpcOpts := []grpc.ServerOption{
		chainedInterceptor,
		grpc.MaxRecvMsgSize(maxRecvBytes),
	}

	s.grpcSrv = grpc.NewServer(grpcOpts...)
	s.lis = lis

	// DaemonService 등록
	svc := newDaemonService(s.startTime, state, shutdownToken, cancel, logger)
	minkv1.RegisterDaemonServiceServer(s.grpcSrv, svc)

	// Health service 등록 (REQ-TR-015)
	s.healthSrv = health.NewServer()
	s.updateHealthState()
	grpc_health_v1.RegisterHealthServer(s.grpcSrv, s.healthSrv)

	// Reflection (REQ-TR-009: 기본 off, Config.EnableReflection 또는 환경변수 true 시 on)
	// SPEC-MINK-ENV-MIGRATE-001: MINK_GRPC_REFLECTION (legacy: GOOSE_GRPC_REFLECTION)
	reflectionVal, _, _ := envalias.DefaultGet("GRPC_REFLECTION")
	if cfg.EnableReflection || reflectionVal == "true" {
		reflection.Register(s.grpcSrv)
	}

	// PanicTestService registration (test-only, AC-TR-006).
	// Must register before Serve — gRPC fatals on RegisterService-after-Serve.
	if cfg.RegisterPanicTestService {
		registerPanicTestService(s.grpcSrv)
	}

	// Serve 시작
	go func() {
		if err := s.grpcSrv.Serve(lis); err != nil {
			logger.Warn("grpc server Serve 종료", zap.Error(err))
		}
	}()

	// state 변경 모니터링 (health 상태 동기화)
	go s.watchState(rootCtx)

	return s, nil
}

// watchState는 state 변경을 폴링하며 health 상태를 업데이트한다.
func (s *Server) watchState(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.updateHealthState()
		}
	}
}

// updateHealthState는 현재 ProcessState에 맞게 health 상태를 업데이트한다.
// REQ-TR-015: serving → SERVING, draining/stopped → NOT_SERVING
func (s *Server) updateHealthState() {
	if s.state.Load() == core.StateServing {
		s.healthSrv.SetServingStatus("mink.v1.DaemonService", grpc_health_v1.HealthCheckResponse_SERVING)
	} else {
		s.healthSrv.SetServingStatus("mink.v1.DaemonService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	}
}

// Addr는 서버가 바인딩된 주소를 반환한다.
func (s *Server) Addr() string {
	return s.lis.Addr().String()
}

// Stop은 서버를 즉시 종료한다.
func (s *Server) Stop() {
	s.grpcSrv.Stop()
}

// GracefulStopWithTimeout은 timeout 이내에 GracefulStop을 시도하고,
// 초과 시 Stop() fallback + WARN 로그를 남긴다. (REQ-TR-007, AC-TR-011)
//
// @MX:WARN: [AUTO] GracefulStop이 timeout 초과 시 Stop() 강제 종료
// @MX:REASON: hung 클라이언트가 있으면 daemon이 무한 대기할 수 있음
func (s *Server) GracefulStopWithTimeout(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		// gracefulStopBlocker가 설정된 경우(테스트용) blocker가 닫힐 때까지 대기
		if s.gracefulStopBlocker != nil {
			<-s.gracefulStopBlocker
		}
		s.grpcSrv.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		// 정상 완료
	case <-time.After(timeout):
		s.logger.Warn("grpc server stop fallback after graceful timeout",
			zap.Duration("timeout", timeout),
		)
		s.grpcSrv.Stop()
	}
}

// SetGracefulStopBlocker는 테스트에서 GracefulStop을 블로킹하기 위한 채널을 설정한다.
// 테스트 코드에서만 사용하며, 프로덕션 코드에서는 호출하지 않는다.
func (s *Server) SetGracefulStopBlocker(blocker <-chan struct{}) {
	s.gracefulStopBlocker = blocker
}

// InterceptorChain은 등록된 interceptor 이름 목록을 반환한다 (AC-TR-013 검증용).
func (s *Server) InterceptorChain() []string {
	return s.interceptorNames
}

// ForceHealthUpdate는 현재 ProcessState를 즉시 health 서비스에 반영한다.
// 테스트에서 state 전환 직후 health 상태를 즉시 확인할 때 사용한다.
func (s *Server) ForceHealthUpdate() {
	s.updateHealthState()
}

// resolveMaxRecvBytes는 gRPC 최대 수신 메시지 크기를 결정한다.
// configured > 0이면 그 값을 사용, 아니면 MINK_GRPC_MAX_RECV_MSG_BYTES (legacy: GOOSE_GRPC_MAX_RECV_MSG_BYTES),
// 그것도 없으면 기본값 4MiB를 반환한다. (REQ-TR-014)
// SPEC-MINK-ENV-MIGRATE-001: envalias.DefaultGet("GRPC_MAX_RECV_MSG_BYTES") 경유.
func resolveMaxRecvBytes(configured int) int {
	if configured > 0 {
		return configured
	}
	if v, _, ok := envalias.DefaultGet("GRPC_MAX_RECV_MSG_BYTES"); ok {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			return n
		}
	}
	return 4 * 1024 * 1024 // 4MiB
}

// resolveShutdownToken은 Shutdown RPC 인증 토큰을 결정한다.
// override=true이면 configured 값을 그대로 사용 (빈 문자열도 허용).
// override=false이면 configured가 빈 문자열일 때 MINK_SHUTDOWN_TOKEN (legacy: GOOSE_SHUTDOWN_TOKEN) 환경변수를 읽는다.
// (REQ-TR-011, ShutdownTokenOverride 테스트 격리 패턴)
// SPEC-MINK-ENV-MIGRATE-001: envalias.DefaultGet("SHUTDOWN_TOKEN") 경유.
func resolveShutdownToken(configured string, override bool) string {
	if override {
		return configured
	}
	if configured != "" {
		return configured
	}
	token, _, _ := envalias.DefaultGet("SHUTDOWN_TOKEN")
	return token
}
