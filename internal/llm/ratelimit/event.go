package ratelimit

import "time"

// BucketType은 4개 버킷 식별자이다.
const (
	BucketRequestsMin  = "requests_min"
	BucketRequestsHour = "requests_hour"
	BucketTokensMin    = "tokens_min"
	BucketTokensHour   = "tokens_hour"
)

// Event는 rate-limit 임계치 초과 이벤트이다.
// REQ-RL-004: ThresholdPct 이상 bucket에 대해 발화.
type Event struct {
	Provider   string
	BucketType string // BucketRequestsMin 등
	UsagePct   float64
	ResetIn    time.Duration
	At         time.Time
}

// Observer는 rate-limit 이벤트를 수신하는 관찰자 인터페이스이다.
// REQ-RL-012: Observers는 등록 순서대로 호출된다.
type Observer interface {
	OnRateLimitEvent(e Event)
}
