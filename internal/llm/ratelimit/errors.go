package ratelimit

import "fmt"

// ErrParserNotRegistered는 요청된 provider에 대한 Parser가 등록되지 않았을 때 반환된다.
// REQ-RL-010
type ErrParserNotRegistered struct {
	Provider string
}

func (e ErrParserNotRegistered) Error() string {
	return fmt.Sprintf("ratelimit: parser not registered for provider %q", e.Provider)
}

// ErrInvalidThreshold는 ThresholdPct가 [50.0, 100.0] 범위를 벗어날 때 반환된다.
// REQ-RL-013
type ErrInvalidThreshold struct {
	Value float64
}

func (e ErrInvalidThreshold) Error() string {
	return fmt.Sprintf("ratelimit: ThresholdPct %.2f out of valid range [50.0, 100.0]", e.Value)
}
