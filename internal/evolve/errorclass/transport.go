// Package errorclass — transport 휴리스틱 (stage 5)
package errorclass

import (
	"context"
	"errors"
	"net"
	"strings"
)

// _serverDisconnectPatterns는 서버 강제 종료를 나타내는 메시지 패턴.
var _serverDisconnectPatterns = []string{
	"server disconnected",
	"peer closed connection",
	"connection reset by peer",
	"connection was closed",
	"network connection lost",
	"unexpected eof",
	"incomplete chunked read",
	"connection reset",
	"eof",
}

// matchTransport는 transport 레벨 오류를 분류한다 (stage 5).
//
// 처리 순서:
//  1. context.DeadlineExceeded → Timeout
//  2. net.Error.Timeout() → Timeout
//  3. 서버 disconnect + 큰 context → ContextOverflow (REQ-016)
//  4. 서버 disconnect → TransportError
func matchTransport(err error, meta ErrorMeta) (FailoverReason, bool) {
	// 1. context deadline
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout, true
	}

	// 2. net.Error timeout
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return Timeout, true
	}

	// 3 & 4. 서버 disconnect 휴리스틱
	msg := strings.ToLower(err.Error())
	isDisconnect := false
	for _, pat := range _serverDisconnectPatterns {
		if strings.Contains(msg, pat) {
			isDisconnect = true
			break
		}
	}

	if isDisconnect {
		// Context bloat 판단 (REQ-016):
		// meta.ApproxTokens > ContextLength*0.6 OR > 120_000 OR MessageCount > 200
		isContextBloat := (meta.ContextLength > 0 && meta.ApproxTokens > int(float64(meta.ContextLength)*0.6)) ||
			meta.ApproxTokens > 120_000 ||
			meta.MessageCount > 200

		if isContextBloat {
			return ContextOverflow, true
		}
		return TransportError, true
	}

	return Unknown, false
}
