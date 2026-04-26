// Package ratelimit는 LLM provider 속도 제한 상태를 추적한다.
// SPEC-GOOSE-RATELIMIT-001 v0.2.0
package ratelimit

import "time"

// RateLimitBucket은 단일 rate-limit 버킷(RPM/TPM/RPH/TPH 중 하나)의 상태를 나타낸다.
// 모든 유도 속성은 값 수신자 메서드로 제공된다.
type RateLimitBucket struct {
	Limit        int
	Remaining    int
	ResetSeconds float64
	CapturedAt   time.Time
}

// Used는 사용한 요청/토큰 수를 반환한다. 음수가 되지 않도록 보장한다.
// REQ-RL-002
func (b RateLimitBucket) Used() int {
	u := b.Limit - b.Remaining
	if u < 0 {
		return 0
	}
	return u
}

// UsagePct는 사용률을 백분율(0.0~100.0)로 반환한다.
// Limit이 0이면 0.0을 반환한다.
// REQ-RL-002
func (b RateLimitBucket) UsagePct() float64 {
	if b.Limit == 0 {
		return 0.0
	}
	return float64(b.Used()) / float64(b.Limit) * 100.0
}

// RemainingSecondsNow는 now 기준 남은 리셋 시간(초)을 반환한다.
// 버킷이 stale이면 0.0을 반환한다.
// REQ-RL-007a
func (b RateLimitBucket) RemainingSecondsNow(now time.Time) float64 {
	if b.IsStale(now) {
		return 0.0
	}
	elapsed := now.Sub(b.CapturedAt).Seconds()
	remaining := b.ResetSeconds - elapsed
	if remaining < 0 {
		return 0.0
	}
	return remaining
}

// IsStale은 버킷이 리셋 시간이 지나 오래된 상태인지 반환한다.
// REQ-RL-007a
func (b RateLimitBucket) IsStale(now time.Time) bool {
	if b.CapturedAt.IsZero() {
		return false
	}
	resetAt := b.CapturedAt.Add(time.Duration(b.ResetSeconds * float64(time.Second)))
	return now.After(resetAt)
}

// RateLimitState는 특정 provider의 4-bucket 속도 제한 상태 스냅샷이다.
// REQ-RL-001: State() 반환값은 copy이며 변경 불가 계약.
type RateLimitState struct {
	Provider     string
	RequestsMin  RateLimitBucket
	RequestsHour RateLimitBucket
	TokensMin    RateLimitBucket
	TokensHour   RateLimitBucket
	CapturedAt   time.Time
}

// IsEmpty는 아직 Parse가 한 번도 호출되지 않은 상태인지 반환한다.
// REQ-RL-008
func (s RateLimitState) IsEmpty() bool {
	return s.CapturedAt.IsZero()
}
