package grpc

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// newLoggingInterceptor는 모든 RPC 호출에 대해 4개 필드를 기록하는 인터셉터를 반환한다.
// REQ-TR-002: method, peer, status_code, duration_ms 필드 기록
// 성공(OK) → INFO, 실패(non-OK) → ERROR
//
// @MX:ANCHOR: [AUTO] 모든 RPC 호출의 access log 기록 지점
// @MX:REASON: Logging interceptor는 chain index 1 위치 — Recovery 다음
func newLoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		// peer 주소 추출
		peerAddr := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			peerAddr = p.Addr.String()
		}

		resp, err := handler(ctx, req)

		elapsed := time.Since(start)
		code := status.Code(err)
		durationMs := float64(elapsed.Milliseconds())

		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.String("peer", peerAddr),
			zap.String("status_code", code.String()),
			zap.Float64("duration_ms", durationMs),
		}

		if code == codes.OK {
			logger.Info("grpc request", fields...)
		} else {
			logger.Error("grpc request failed", fields...)
		}

		return resp, err
	}
}

// recoverPanic은 panic을 복구하고 codes.Internal 에러를 반환한다.
// REQ-TR-004: panic 복구 + stack trace를 stderr에 기록
// 스택은 wire로 절대 흘리지 않음 (보안).
//
// @MX:WARN: [AUTO] panic recovery — 스택을 wire에 노출 금지
// @MX:REASON: 스택 trace에 내부 경로, 비밀 등이 포함될 수 있어 외부 노출 금지
func recoverPanic(_ context.Context, p any, logger *zap.Logger) error {
	stack := debug.Stack()
	// stderr에만 기록 (wire에는 흘리지 않음)
	fmt.Fprintf(os.Stderr, "grpc handler panic: %v\n%s\n", p, stack)
	logger.Error("grpc handler panicked",
		zap.Any("panic", p),
		zap.ByteString("stack", stack),
	)
	return status.Errorf(codes.Internal, "internal error")
}
