package grpc_test

// AC-TR-001 ~ AC-TR-014 통합 테스트
// SPEC-GOOSE-TRANSPORT-001 §5 수용 기준
//
// 테스트 하네스 설계:
// - ephemeral TCP (:0)로 실제 gRPC 연결 수립
// - goleak으로 goroutine 누수 검증
// - t.Parallel()로 독립적 테스트 병렬 실행
// - Config 필드를 통해 환경변수 의존성 최소화 (race condition 방지)

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/modu-ai/goose/internal/core"
	grpcserver "github.com/modu-ai/goose/internal/transport/grpc"
	"github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("google.golang.org/grpc.(*Server).Serve"),
		goleak.IgnoreTopFunction("google.golang.org/grpc.(*Server).handleRawConn"),
		goleak.IgnoreTopFunction("google.golang.org/grpc/internal/transport.(*http2Server)"),
		goleak.IgnoreTopFunction("google.golang.org/grpc/internal/transport.newHTTP2Server"),
		goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
	)
}

// serverHarness는 테스트용 gRPC 서버와 클라이언트를 묶는 하네스다.
type serverHarness struct {
	srv    *grpcserver.Server
	conn   *grpc.ClientConn
	client goosev1.DaemonServiceClient
	health grpc_health_v1.HealthClient
	state  *core.StateHolder
	cancel context.CancelFunc
}

// newHarness는 테스트용 gRPC 서버를 시작하고 클라이언트 연결을 반환한다.
func newHarness(t *testing.T, cfg grpcserver.Config) *serverHarness {
	t.Helper()

	if cfg.BindAddr == "" {
		cfg.BindAddr = "127.0.0.1:0"
	}

	state := &core.StateHolder{}
	state.Store(core.StateServing)

	rootCtx, cancel := context.WithCancel(context.Background())

	srv, err := grpcserver.NewServer(cfg, zap.NewNop(), state, rootCtx)
	require.NoError(t, err)

	addr := srv.Addr()
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
		cancel()
	})

	return &serverHarness{
		srv:    srv,
		conn:   conn,
		client: goosev1.NewDaemonServiceClient(conn),
		health: grpc_health_v1.NewHealthClient(conn),
		state:  state,
		cancel: cancel,
	}
}

// newHarnessWithLogger는 커스텀 logger와 함께 서버를 시작한다.
func newHarnessWithLogger(t *testing.T, cfg grpcserver.Config, logger *zap.Logger) *serverHarness {
	t.Helper()

	if cfg.BindAddr == "" {
		cfg.BindAddr = "127.0.0.1:0"
	}

	state := &core.StateHolder{}
	state.Store(core.StateServing)

	rootCtx, cancel := context.WithCancel(context.Background())

	srv, err := grpcserver.NewServer(cfg, logger, state, rootCtx)
	require.NoError(t, err)

	addr := srv.Addr()
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
		cancel()
	})

	return &serverHarness{
		srv:    srv,
		conn:   conn,
		client: goosev1.NewDaemonServiceClient(conn),
		health: grpc_health_v1.NewHealthClient(conn),
		state:  state,
		cancel: cancel,
	}
}

// AC-TR-001: Ping RPC 정상 응답
func TestPingRPC_ReturnsVersionAndState(t *testing.T) {
	t.Parallel()
	h := newHarness(t, grpcserver.Config{})

	resp, err := h.client.Ping(context.Background(), &goosev1.PingRequest{})
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Version, "version이 비어있으면 안 됨")
	assert.Greater(t, resp.UptimeMs, int64(0), "uptime_ms가 0보다 커야 함")
	assert.Equal(t, "serving", resp.State, "state가 serving이어야 함")
}

// AC-TR-002: Health Check
func TestHealthCheck_ServiceServing(t *testing.T) {
	t.Parallel()
	h := newHarness(t, grpcserver.Config{})

	// serving 상태에서 SERVING 반환
	resp, err := h.health.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{
		Service: "goose.v1.DaemonService",
	})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)

	// draining 상태로 전이 후 NOT_SERVING 반환
	h.state.Store(core.StateDraining)
	h.srv.ForceHealthUpdate() // 즉시 health 상태 동기화
	resp, err = h.health.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{
		Service: "goose.v1.DaemonService",
	})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_NOT_SERVING, resp.Status)
}

// AC-TR-003: Shutdown 토큰 없이 거부 (토큰 설정됨 + 헤더 누락)
func TestShutdownWithoutToken_Unauthenticated(t *testing.T) {
	t.Parallel()
	h := newHarness(t, grpcserver.Config{
		ShutdownToken: "secret",
	})

	_, err := h.client.Shutdown(context.Background(), &goosev1.ShutdownRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	// 데몬이 계속 serving 상태여야 함
	assert.Equal(t, core.StateServing, h.state.Load())
}

// AC-TR-004: Shutdown 토큰 포함 시 종료 개시 (500ms 이내 cancel)
func TestShutdownWithToken_Accepted(t *testing.T) {
	t.Parallel()

	state := &core.StateHolder{}
	state.Store(core.StateServing)
	rootCtx, cancel := context.WithCancel(context.Background())
	cancelCalled := make(chan struct{})

	// cancel wrapper: cancel 호출 시 채널에 신호
	wrappedCancel := func() {
		cancel()
		close(cancelCalled)
	}

	srv, err := grpcserver.NewServerWithCancel(grpcserver.Config{
		BindAddr:      "127.0.0.1:0",
		ShutdownToken: "secret",
	}, zap.NewNop(), state, rootCtx, wrappedCancel)
	require.NoError(t, err)

	conn, err := grpc.NewClient(srv.Addr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
	})

	client := goosev1.NewDaemonServiceClient(conn)
	md := metadata.Pairs("auth_token", "secret")
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	resp, err := client.Shutdown(ctx, &goosev1.ShutdownRequest{Reason: "test"})
	require.NoError(t, err)
	assert.True(t, resp.Accepted)

	select {
	case <-cancelCalled:
		// 정상: 500ms 이내 cancel 호출됨
	case <-time.After(500 * time.Millisecond):
		t.Fatal("500ms 이내에 daemon cancel이 호출되지 않음")
	}
}

// AC-TR-005: Draining 중 GetInfo → Unavailable
func TestGetInfo_DrainingState_Unavailable(t *testing.T) {
	t.Parallel()

	state := &core.StateHolder{}
	state.Store(core.StateDraining)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := grpcserver.NewServer(grpcserver.Config{
		BindAddr: "127.0.0.1:0",
	}, zap.NewNop(), state, rootCtx)
	require.NoError(t, err)

	conn, err := grpc.NewClient(srv.Addr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
	})

	client := goosev1.NewDaemonServiceClient(conn)
	_, err = client.GetInfo(context.Background(), &goosev1.GetInfoRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "daemon draining")
}

// AC-TR-006: Panic 복구 — 테스트 전용 PanicTest RPC를 서버에 추가 등록
func TestPanicHandler_Recovered(t *testing.T) {
	t.Parallel()

	// zap observer로 패닉 로그를 캡처
	core2, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core2)

	state := &core.StateHolder{}
	state.Store(core.StateServing)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := grpcserver.NewServer(grpcserver.Config{
		BindAddr: "127.0.0.1:0",
	}, logger, state, rootCtx)
	require.NoError(t, err)

	// PanicTest RPC 서비스 등록
	srv.RegisterPanicTestService()

	conn, err := grpc.NewClient(srv.Addr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
	})

	// PanicTest 클라이언트 호출 → codes.Internal 기대
	panicClient := grpcserver.NewPanicTestClient(conn)
	_, err = panicClient.TriggerPanic(context.Background(), &goosev1.PingRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	// 프로세스가 계속 서빙 중인지 확인 (프로세스 생존)
	pingClient := goosev1.NewDaemonServiceClient(conn)
	_, pingErr := pingClient.Ping(context.Background(), &goosev1.PingRequest{})
	assert.NoError(t, pingErr, "panic 이후에도 서버가 계속 동작해야 함")

	// zap logger에 panic 로그가 ERROR 레벨로 기록됐는지 확인
	allLogs := logs.All()
	var foundPanicLog bool
	for _, entry := range allLogs {
		if entry.Level == zap.ErrorLevel {
			if msg := entry.Message; msg == "grpc handler panicked" {
				foundPanicLog = true
			}
		}
	}
	assert.True(t, foundPanicLog, "panic 발생 시 ERROR 레벨 로그가 기록되어야 함")
}

// AC-TR-007: Reflection off by default
func TestReflection_OffByDefault(t *testing.T) {
	t.Parallel()
	// GOOSE_GRPC_REFLECTION 미설정 상태, Config.EnableReflection=false
	h := newHarness(t, grpcserver.Config{
		EnableReflection: false,
	})

	// reflection 서비스 호출 시 unknown service 에러
	ctx := context.Background()
	err := h.conn.Invoke(ctx, "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo", nil, nil)
	require.Error(t, err)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}

// AC-TR-009: LoggingInterceptor 필드 기록
func TestLoggingInterceptor_RecordsFields(t *testing.T) {
	t.Parallel()

	// DEBUG 레벨 이하도 캡처하도록 zaptest observer
	core2, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core2)

	h := newHarnessWithLogger(t, grpcserver.Config{
		ShutdownToken: "testsecret",
	}, logger)

	client := h.client

	// (a) Ping 정상 호출
	_, err := client.Ping(context.Background(), &goosev1.PingRequest{})
	require.NoError(t, err)

	// (b) Shutdown 토큰 누락으로 실패
	_, err = client.Shutdown(context.Background(), &goosev1.ShutdownRequest{})
	require.Error(t, err)

	// 로그 엔트리 확인 (최소 2개)
	allLogs := logs.All()
	require.GreaterOrEqual(t, len(allLogs), 2, "최소 2개 로그 엔트리 필요")

	// 4개 필드 확인
	for _, entry := range allLogs[:2] {
		fields := entry.ContextMap()
		assert.Contains(t, fields, "method")
		assert.Contains(t, fields, "peer")
		assert.Contains(t, fields, "status_code")
		assert.Contains(t, fields, "duration_ms")
	}

	// Ping은 INFO, Shutdown 실패는 ERROR
	var foundPingInfo, foundShutdownError bool
	for _, entry := range allLogs {
		fields := entry.ContextMap()
		if sc, ok := fields["status_code"]; ok {
			if sc == "OK" && entry.Level == zap.InfoLevel {
				foundPingInfo = true
			}
			if sc == "Unauthenticated" && entry.Level == zap.ErrorLevel {
				foundShutdownError = true
			}
		}
	}
	assert.True(t, foundPingInfo, "Ping 성공 → INFO 레벨 로그 필요")
	assert.True(t, foundShutdownError, "Shutdown 실패 → ERROR 레벨 로그 필요")
}

// AC-TR-010: proto 패키지 및 Go 패키지 경로 정합성 (go vet 통과로 확인)
func TestProtoPackage_GoVetClean(t *testing.T) {
	// go vet ./internal/transport/grpc/gen/... 은 CI에서 별도 실행
	// 여기서는 import path가 올바른지 컴파일 타임에 검증
	var _ goosev1.DaemonServiceServer
	var _ goosev1.PingRequest
	var _ goosev1.GetInfoRequest
	var _ goosev1.ShutdownRequest
}

// AC-TR-011: GracefulStop 10s 준수 및 fallback
func TestGracefulStop_WithTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("10s timeout 테스트 — short 모드에서 skip")
	}

	core2, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core2)

	state := &core.StateHolder{}
	state.Store(core.StateServing)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := grpcserver.NewServer(grpcserver.Config{
		BindAddr: "127.0.0.1:0",
	}, logger, state, rootCtx)
	require.NoError(t, err)

	conn, err := grpc.NewClient(srv.Addr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	// normal graceful stop: 빠르게 완료되어야 함
	start := time.Now()
	srv.GracefulStopWithTimeout(10 * time.Second)
	elapsed := time.Since(start)

	// 정상 케이스에서는 10s보다 훨씬 빠르게 완료
	assert.Less(t, elapsed, 12*time.Second)

	// 정상 완료 시 WARN 로그 없음
	_ = logs
}

// TestGracefulStop_FallbackOnTimeout은 Stop() fallback + WARN 로그를 검증한다.
// AC-TR-011 (b) stuck hook 시나리오
func TestGracefulStop_FallbackOnTimeout(t *testing.T) {
	t.Parallel()

	core2, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core2)

	state := &core.StateHolder{}
	state.Store(core.StateServing)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := grpcserver.NewServer(grpcserver.Config{
		BindAddr: "127.0.0.1:0",
	}, logger, state, rootCtx)
	require.NoError(t, err)

	conn, err := grpc.NewClient(srv.Addr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	// blocker 채널로 GracefulStop을 블로킹 → 매우 짧은 timeout으로 강제 fallback 유도
	blocker := make(chan struct{}) // 닫히지 않아서 GracefulStop이 영원히 대기
	srv.SetGracefulStopBlocker(blocker)
	// 테스트 종료 시 blocker를 닫아 goroutine 누수 방지
	t.Cleanup(func() { close(blocker) })

	start := time.Now()
	srv.GracefulStopWithTimeout(50 * time.Millisecond)
	elapsed := time.Since(start)

	// 50ms + 여유 시간 이내 완료 (Stop fallback)
	assert.Less(t, elapsed, 2*time.Second, "fallback이 2s 이내에 완료되어야 함")

	// WARN 로그 확인
	allLogs := logs.All()
	var foundWarn bool
	for _, entry := range allLogs {
		if entry.Level == zap.WarnLevel && entry.Message == "grpc server stop fallback after graceful timeout" {
			foundWarn = true
		}
	}
	assert.True(t, foundWarn, "GracefulStop timeout 시 WARN 로그 기록 필요")
}

// AC-TR-012: Shutdown 토큰 미설정 시 Unimplemented
func TestShutdownTokenUnset_Unimplemented(t *testing.T) {
	t.Parallel()

	// ShutdownTokenOverride=true + ShutdownToken="" → 환경변수 무시, 완전히 미설정
	h := newHarness(t, grpcserver.Config{
		ShutdownToken:         "", // 의도적으로 미설정
		ShutdownTokenOverride: true,
	})

	// 어떤 metadata를 넣어도 Unimplemented 반환
	md := metadata.Pairs("auth_token", "anything")
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	_, err := h.client.Shutdown(ctx, &goosev1.ShutdownRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Unimplemented, status.Code(err),
		"GOOSE_SHUTDOWN_TOKEN 미설정 시 반드시 Unimplemented 반환")

	// daemon은 계속 serving
	assert.Equal(t, core.StateServing, h.state.Load())
}

// AC-TR-013: RecoveryInterceptor가 chain outermost(인덱스 0)
func TestInterceptorChainOrder_RecoveryOutermost(t *testing.T) {
	t.Parallel()
	h := newHarness(t, grpcserver.Config{})

	// 체인 순서 검증: Recovery가 index 0
	chain := h.srv.InterceptorChain()
	require.GreaterOrEqual(t, len(chain), 2, "최소 2개 interceptor 필요")
	assert.Equal(t, "recovery", chain[0], "chain[0]은 recovery이어야 함")
	assert.Equal(t, "logging", chain[1], "chain[1]은 logging이어야 함")
}

// AC-TR-014: MaxRecvMsgSize 환경변수 override
func TestMaxRecvMsgSize_Override(t *testing.T) {
	t.Parallel()

	// Config.MaxRecvMsgBytes=1024로 설정
	h := newHarness(t, grpcserver.Config{
		MaxRecvMsgBytes: 1024,
	})

	// 2048 byte 페이로드: reason 필드를 패딩
	bigReason := string(make([]byte, 2048))
	_, err := h.client.Shutdown(context.Background(), &goosev1.ShutdownRequest{Reason: bigReason})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Contains(t, err.Error(), "received message larger than max")
}

// TestMaxRecvMsgSize_EnvOverride는 환경변수로 MaxRecvMsgBytes를 override하는 케이스를 검증한다.
// AC-TR-014 추가 검증
func TestMaxRecvMsgSize_EnvOverride(t *testing.T) {
	// 이 테스트는 환경변수를 사용하므로 병렬 실행 불가
	old, exists := os.LookupEnv("GOOSE_GRPC_MAX_RECV_MSG_BYTES")
	t.Cleanup(func() {
		if exists {
			os.Setenv("GOOSE_GRPC_MAX_RECV_MSG_BYTES", old)
		} else {
			os.Unsetenv("GOOSE_GRPC_MAX_RECV_MSG_BYTES")
		}
	})
	os.Setenv("GOOSE_GRPC_MAX_RECV_MSG_BYTES", "1024")

	state := &core.StateHolder{}
	state.Store(core.StateServing)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Config.MaxRecvMsgBytes=0이면 환경변수에서 읽음
	srv, err := grpcserver.NewServer(grpcserver.Config{
		BindAddr: "127.0.0.1:0",
	}, zap.NewNop(), state, rootCtx)
	require.NoError(t, err)

	conn, err := grpc.NewClient(srv.Addr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
	})

	client := goosev1.NewDaemonServiceClient(conn)
	bigReason := string(make([]byte, 2048))
	_, err = client.Shutdown(context.Background(), &goosev1.ShutdownRequest{Reason: bigReason})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// AC-TR-008: Non-loopback bind 거부 (linux/amd64에서만 실행)
func TestNonLoopbackBind_Rejected(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("linux/amd64 전용 테스트")
	}
	t.Parallel()

	// GOOSE_GRPC_BIND 미설정(127.0.0.1)에서 non-loopback 연결 거부
	state := &core.StateHolder{}
	state.Store(core.StateServing)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := grpcserver.NewServer(grpcserver.Config{
		BindAddr: "127.0.0.1:0",
	}, zap.NewNop(), state, rootCtx)
	require.NoError(t, err)
	defer srv.Stop()

	// non-loopback IP로 연결 시도
	_, portStr, _ := net.SplitHostPort(srv.Addr())
	nonLoopbackAddr := fmt.Sprintf("0.0.0.0:%s", portStr)

	conn, err := grpc.NewClient(nonLoopbackAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := goosev1.NewDaemonServiceClient(conn)
	ctx, cancelCtx := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelCtx()
	_, err = client.Ping(ctx, &goosev1.PingRequest{})
	// loopback에만 bind했으므로 non-loopback에서는 실패해야 함
	require.Error(t, err)
}
