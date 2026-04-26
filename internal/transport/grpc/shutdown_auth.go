package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// extractShutdownToken은 gRPC metadata에서 auth_token 헤더를 추출한다.
// REQ-TR-010: 헤더 미포함 시 empty string 반환
func extractShutdownToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("auth_token")
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// validateShutdownToken은 제공된 토큰이 expected와 일치하는지 확인한다.
// REQ-TR-010: 불일치 시 codes.Unauthenticated 반환
// REQ-TR-011: expected가 빈 문자열(미설정)이면 codes.Unimplemented 반환
//
// @MX:ANCHOR: [AUTO] Shutdown 인증 검증의 단일 진입점
// @MX:REASON: AC-TR-003, AC-TR-012 두 개의 코드 경로(Unauthenticated vs Unimplemented)가 여기서 분기
func validateShutdownToken(ctx context.Context, expectedToken string) error {
	// REQ-TR-011: 토큰 미설정 → Unimplemented
	if expectedToken == "" {
		return status.Error(codes.Unimplemented, "shutdown RPC is disabled")
	}

	// REQ-TR-010: 토큰 설정됨 + 헤더 누락 → Unauthenticated
	provided := extractShutdownToken(ctx)
	if provided == "" || provided != expectedToken {
		return status.Error(codes.Unauthenticated, "missing or invalid shutdown token")
	}

	return nil
}
