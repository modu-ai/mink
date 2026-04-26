package ratelimit

import (
	"strconv"
	"strings"
	"time"
)

// Parser는 provider별 HTTP 응답 헤더를 RateLimitState로 변환하는 인터페이스이다.
// REQ-RL-004: Parse(provider, headers, now) 파이프라인에서 사용.
type Parser interface {
	// Provider는 이 파서가 처리하는 provider 이름을 반환한다.
	Provider() string
	// Parse는 headers에서 4-bucket RateLimitState를 추출한다.
	// 파싱 실패는 해당 버킷을 zero-value로 남기고 계속 진행(REQ-RL-006).
	Parse(headers map[string]string, now time.Time) (RateLimitState, []string)
}

// CaseInsensitiveGet은 HTTP 헤더 맵에서 대소문자 무관하게 값을 조회한다.
// 헤더 키는 HTTP 사양에 따라 대소문자 구분 없음.
func CaseInsensitiveGet(headers map[string]string, key string) (string, bool) {
	lowerKey := strings.ToLower(key)
	for k, v := range headers {
		if strings.ToLower(k) == lowerKey {
			return v, true
		}
	}
	return "", false
}

// parseIntOrZero는 문자열을 int로 변환한다.
// 실패 시 0과 에러 메시지를 반환한다(REQ-RL-006).
func parseIntOrZero(s string) (int, string) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, "malformed integer: " + s
	}
	return v, ""
}

// parseDurationSeconds는 Go duration 문자열("6s", "1m30s")을 초로 변환한다.
func parseDurationSeconds(s string) (float64, string) {
	d, err := time.ParseDuration(strings.TrimSpace(s))
	if err != nil {
		return 0, "malformed duration: " + s
	}
	return d.Seconds(), ""
}

// parseISO8601ResetSeconds는 ISO 8601 timestamp를 now 기준 남은 초로 변환한다.
// reset <= now이면 0.0을 반환한다.
func parseISO8601ResetSeconds(s string, now time.Time) (float64, string) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return 0, "malformed RFC3339 timestamp: " + s
	}
	secs := t.Sub(now).Seconds()
	if secs < 0 {
		secs = 0
	}
	return secs, ""
}
