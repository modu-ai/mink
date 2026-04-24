// Package ratelimit는 LLM provider 속도 제한 상태를 추적한다.
// SPEC-GOOSE-ADAPTER-001 M0 T-004
// RATELIMIT-001 구현 시 Parse 로직이 채워진다.
package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// State는 특정 provider의 속도 제한 상태를 나타낸다.
type State struct {
	// Provider는 provider 이름이다.
	Provider string
	// UpdatedAt은 마지막으로 상태가 갱신된 시각이다.
	UpdatedAt time.Time
}

// Tracker는 provider별 속도 제한 상태를 스레드 안전하게 관리한다.
// RATELIMIT-001에서 Parse 로직이 구현될 예정이다.
// 현재는 noop stub이다.
type Tracker struct {
	mu     sync.Mutex
	states map[string]State
}

// NewTracker는 빈 Tracker를 생성한다.
func NewTracker() *Tracker {
	return &Tracker{
		states: make(map[string]State),
	}
}

// Parse는 HTTP 응답 헤더로부터 속도 제한 상태를 파싱한다.
// 현재 noop stub이다. RATELIMIT-001에서 구현된다.
func (t *Tracker) Parse(provider string, _ http.Header, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.states[provider] = State{
		Provider:  provider,
		UpdatedAt: now,
	}
}
