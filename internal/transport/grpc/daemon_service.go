package grpc

import (
	"context"
	"runtime"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/modu-ai/mink/internal/core"
	"github.com/modu-ai/mink/internal/transport/grpc/gen/minkv1"
)

// 빌드 메타데이터 변수 (ldflags로 주입 가능)
var (
	// BuildVersion은 애플리케이션 버전이다.
	BuildVersion = "dev"
	// BuildGitCommit은 git commit hash다.
	BuildGitCommit = "unknown"
	// BuildGoVersion은 Go 버전이다.
	BuildGoVersion = runtime.Version()
	// BuildTime은 빌드 시각 (ISO-8601)이다.
	BuildTime = "unknown"
)

// daemonService는 DaemonServiceServer 인터페이스를 구현한다.
// @MX:ANCHOR: [AUTO] DaemonService RPC 핸들러 집합
// @MX:REASON: Ping/GetInfo/Shutdown 세 핸들러가 공통 state/cancel 의존
type daemonService struct {
	minkv1.UnimplementedDaemonServiceServer
	startTime     time.Time
	state         *core.StateHolder
	shutdownToken string
	rootCancel    context.CancelFunc
	logger        *zap.Logger
}

func newDaemonService(
	startTime time.Time,
	state *core.StateHolder,
	shutdownToken string,
	rootCancel context.CancelFunc,
	logger *zap.Logger,
) *daemonService {
	return &daemonService{
		startTime:     startTime,
		state:         state,
		shutdownToken: shutdownToken,
		rootCancel:    rootCancel,
		logger:        logger,
	}
}

// Ping은 데몬 상태를 반환한다.
// REQ-TR-005: version, uptime_ms, state 반환
// draining 중에도 Ping은 응답한다 (REQ-TR-008 예외).
func (s *daemonService) Ping(_ context.Context, _ *minkv1.PingRequest) (*minkv1.PingResponse, error) {
	uptime := time.Since(s.startTime).Milliseconds()
	return &minkv1.PingResponse{
		Version:  BuildVersion,
		UptimeMs: uptime,
		State:    s.state.Load().String(),
	}, nil
}

// GetInfo는 빌드 메타데이터를 반환한다.
// REQ-TR-008: draining 중 Unavailable 반환 (Ping 제외 모든 RPC)
func (s *daemonService) GetInfo(ctx context.Context, _ *minkv1.GetInfoRequest) (*minkv1.GetInfoResponse, error) {
	if err := checkDrainingState(s.state); err != nil {
		return nil, err
	}
	return &minkv1.GetInfoResponse{
		Version:   BuildVersion,
		GitCommit: BuildGitCommit,
		GoVersion: BuildGoVersion,
		BuildTime: BuildTime,
		Os:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}, nil
}

// Shutdown은 인증된 요청에 한해 daemon graceful shutdown을 트리거한다.
// REQ-TR-006: valid auth_token → accepted=true + rootCancel 호출
// REQ-TR-010: token mismatch → Unauthenticated
// REQ-TR-011: token unset → Unimplemented
// REQ-TR-008: draining 중 Unavailable (Shutdown은 draining 예외 대상이 아님)
func (s *daemonService) Shutdown(ctx context.Context, req *minkv1.ShutdownRequest) (*minkv1.ShutdownResponse, error) {
	if err := checkDrainingState(s.state); err != nil {
		return nil, err
	}

	if err := validateShutdownToken(ctx, s.shutdownToken); err != nil {
		return nil, err
	}

	s.logger.Info("shutdown requested via gRPC",
		zap.String("reason", req.GetReason()),
	)

	// 응답 후 비동기로 cancel 호출 (100ms 이내 — REQ-TR-006)
	go func() {
		time.Sleep(10 * time.Millisecond) // flush 대기
		s.rootCancel()
	}()

	return &minkv1.ShutdownResponse{
		Accepted: true,
		Message:  "shutdown initiated",
	}, nil
}

// checkDrainingState는 draining 상태인 경우 Unavailable 에러를 반환한다.
// REQ-TR-008: draining 중 새 RPC 호출(Ping 제외) → codes.Unavailable
func checkDrainingState(state *core.StateHolder) error {
	if state.Load() == core.StateDraining {
		return status.Error(codes.Unavailable, "daemon draining")
	}
	return nil
}
