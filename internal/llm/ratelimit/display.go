package ratelimit

import (
	"fmt"
	"strings"
	"time"
)

// formatBucket은 단일 버킷을 human-readable 문자열로 변환한다.
// stale 버킷에는 "[STALE]" 마커를 추가한다(REQ-RL-007b).
func formatBucket(name string, b RateLimitBucket, now time.Time) string {
	stale := ""
	if b.IsStale(now) {
		stale = " [STALE]"
	}

	limit := b.Limit
	remaining := b.Remaining
	pct := b.UsagePct()
	resetSecs := b.RemainingSecondsNow(now)

	// 큰 숫자는 K 단위로 표시
	if limit >= 1000 || remaining >= 1000 {
		return fmt.Sprintf("%s: %dK/%dK (%.0f%%), reset in %.0fs%s",
			name,
			remaining/1000,
			limit/1000,
			pct,
			resetSecs,
			stale,
		)
	}
	return fmt.Sprintf("%s: %d/%d (%.0f%%), reset in %.0fs%s",
		name,
		remaining,
		limit,
		pct,
		resetSecs,
		stale,
	)
}

// FormatDisplay는 RateLimitState를 human-readable 문자열로 변환한다.
// §3.1 Display 예시: "requests_min: 120/1000 (12%), tokens_min: 50K/200K (25%), reset in 34s"
// AC-RL-008: IsEmpty이면 "no rate limit information yet" 반환.
func FormatDisplay(state RateLimitState, now time.Time) string {
	if state.IsEmpty() {
		return "no rate limit information yet"
	}

	parts := []string{
		formatBucket("requests_min", state.RequestsMin, now),
		formatBucket("requests_hour", state.RequestsHour, now),
		formatBucket("tokens_min", state.TokensMin, now),
		formatBucket("tokens_hour", state.TokensHour, now),
	}
	return strings.Join(parts, ", ")
}
