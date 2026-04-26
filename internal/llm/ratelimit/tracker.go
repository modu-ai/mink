// Package ratelimit는 LLM provider 속도 제한 상태를 추적한다.
// SPEC-GOOSE-RATELIMIT-001 v0.2.0
package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	defaultThresholdPct = 80.0
	defaultWarnCooldown = 30 * time.Second
	minThresholdPct     = 50.0
	maxThresholdPct     = 100.0
)

// TrackerOptions는 Tracker 생성 옵션이다.
// REQ-RL-004/005: ThresholdPct, WarnCooldown은 기본값을 가지되 하드코딩 금지.
type TrackerOptions struct {
	// Parsers는 provider 이름 → Parser 매핑이다.
	Parsers map[string]Parser
	// Observers는 rate-limit 이벤트를 수신하는 관찰자 목록이다.
	// nil이거나 비어 있으면 observer 호출 없이 logger WARN만 발화한다(REQ-RL-011b).
	Observers []Observer
	// ThresholdPct는 경고 임계치(백분율, 기본 80.0).
	// [50.0, 100.0] 범위를 벗어나면 New()가 ErrInvalidThreshold를 반환한다(REQ-RL-013).
	ThresholdPct float64
	// WarnCooldown은 동일 provider×bucket 조합에 대한 경고 억제 기간(기본 30s)(REQ-RL-005).
	WarnCooldown time.Duration
	// Logger는 zap 로거이다. nil이면 nop 로거를 사용한다.
	Logger *zap.Logger
}

// Tracker는 provider별 rate-limit 상태를 스레드 안전하게 관리한다.
// REQ-RL-003: Parse, State, Display 동시 호출 안전.
//
// @MX:ANCHOR: [AUTO] ratelimit 패키지의 중심 진입점 — Parse/State/Display 모두 이 타입 경유
// @MX:REASON: SPEC-GOOSE-RATELIMIT-001 §6.2; ADAPTER-001/goosed 등 fan_in >= 3 예상
// @MX:SPEC: SPEC-GOOSE-RATELIMIT-001
type Tracker struct {
	opts   TrackerOptions
	mu     sync.RWMutex
	states map[string]*RateLimitState
	// lastWarn은 provider → bucketType → 마지막 경고 시각이다.
	// @MX:WARN: [AUTO] lastWarn 맵 접근은 반드시 mu.Lock() 보유 상태에서만 수행
	// @MX:REASON: Parse 내 threshold 평가와 lastWarn 갱신은 원자적이어야 함; RLock으로는 불충분
	lastWarn map[string]map[string]time.Time
}

// New는 TrackerOptions를 검증하고 Tracker를 생성한다.
// ThresholdPct가 [50.0, 100.0] 범위를 벗어나면 ErrInvalidThreshold를 반환한다(REQ-RL-013).
func New(opts TrackerOptions) (*Tracker, error) {
	if opts.ThresholdPct == 0 {
		opts.ThresholdPct = defaultThresholdPct
	}
	if opts.WarnCooldown == 0 {
		opts.WarnCooldown = defaultWarnCooldown
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	if opts.Parsers == nil {
		opts.Parsers = make(map[string]Parser)
	}

	// REQ-RL-013: 범위 검증
	if opts.ThresholdPct < minThresholdPct || opts.ThresholdPct > maxThresholdPct {
		return nil, ErrInvalidThreshold{Value: opts.ThresholdPct}
	}

	return &Tracker{
		opts:     opts,
		states:   make(map[string]*RateLimitState),
		lastWarn: make(map[string]map[string]time.Time),
	}, nil
}

// Parse는 provider의 HTTP 응답 헤더를 파싱하여 4-bucket 상태를 갱신한다.
// REQ-RL-004: (a) parser 조회, (b) 4-bucket 파싱, (c) 원자적 상태 교체, (d) 임계치 평가, (e) Event 발화.
// REQ-RL-009: 헤더 맵을 변경하지 않는다(비파괴적 읽기).
// REQ-RL-011a: nil headers → DEBUG 로그, state 불변, panic 없음.
// REQ-RL-011c: zero-value now → DEBUG 로그, state 불변, panic 없음.
func (t *Tracker) Parse(provider string, headers map[string]string, now time.Time) error {
	// REQ-RL-011c: zero-time now 방어
	if now.IsZero() {
		t.opts.Logger.Debug("ratelimit: Parse called with zero-value now, skipping",
			zap.String("provider", provider),
		)
		return nil
	}

	// REQ-RL-011a: nil headers 방어
	if headers == nil {
		t.opts.Logger.Debug("ratelimit: Parse called with nil headers, skipping",
			zap.String("provider", provider),
		)
		return nil
	}

	// REQ-RL-010: parser 조회
	t.mu.RLock()
	parser, ok := t.opts.Parsers[provider]
	t.mu.RUnlock()
	if !ok {
		return ErrParserNotRegistered{Provider: provider}
	}

	// 파싱은 락 밖에서 수행 (REQ-RL-009: 헤더 비파괴 읽기)
	state, debugMsgs := parser.Parse(headers, now)

	// DEBUG 로그 출력 (REQ-RL-006)
	for _, msg := range debugMsgs {
		t.opts.Logger.Debug("ratelimit: header parse warning",
			zap.String("provider", provider),
			zap.String("detail", msg),
		)
	}

	// 락 획득 후 상태 원자적 교체 및 임계치 평가
	t.mu.Lock()
	defer t.mu.Unlock()

	t.states[provider] = &state
	t.evaluateThresholds(provider, &state, now)

	return nil
}

// evaluateThresholds는 각 버킷의 사용률을 확인하고 임계치 초과 시 Event를 발화한다.
// 반드시 t.mu.Lock() 보유 상태에서 호출해야 한다.
// REQ-RL-004(d)(e), REQ-RL-005: cooldown 적용.
// @MX:WARN: [AUTO] mu.Lock() 보유 상태에서 Observer.OnRateLimitEvent 호출 — observer가 내부적으로 Tracker 메서드를 재호출하면 deadlock 위험
// @MX:REASON: Lock 보유 중 observer 콜백 호출은 잠재적 deadlock을 유발; observer 계약에서 Tracker 재진입 금지 필요
func (t *Tracker) evaluateThresholds(provider string, state *RateLimitState, now time.Time) {
	buckets := []struct {
		name   string
		bucket RateLimitBucket
	}{
		{BucketRequestsMin, state.RequestsMin},
		{BucketRequestsHour, state.RequestsHour},
		{BucketTokensMin, state.TokensMin},
		{BucketTokensHour, state.TokensHour},
	}

	for _, b := range buckets {
		pct := b.bucket.UsagePct()
		if pct < t.opts.ThresholdPct {
			continue
		}

		// 쿨다운 확인 (REQ-RL-005)
		if _, ok := t.lastWarn[provider]; !ok {
			t.lastWarn[provider] = make(map[string]time.Time)
		}
		lastWarnAt := t.lastWarn[provider][b.name]
		if !lastWarnAt.IsZero() && now.Sub(lastWarnAt) < t.opts.WarnCooldown {
			continue
		}

		// 쿨다운 갱신
		t.lastWarn[provider][b.name] = now

		resetIn := time.Duration(b.bucket.RemainingSecondsNow(now) * float64(time.Second))
		event := Event{
			Provider:   provider,
			BucketType: b.name,
			UsagePct:   pct,
			ResetIn:    resetIn,
			At:         now,
		}

		// zap WARN 로그 (REQ-RL-004)
		t.opts.Logger.Warn("rate_limit_threshold_exceeded",
			zap.String("provider", provider),
			zap.String("bucket", b.name),
			zap.Float64("usage_pct", pct),
			zap.Duration("reset_in", resetIn),
		)

		// Observer 호출 (REQ-RL-011b: nil/empty이면 skip, REQ-RL-012: 등록 순서 보장)
		for _, obs := range t.opts.Observers {
			obs.OnRateLimitEvent(event)
		}
	}
}

// State는 provider의 현재 rate-limit 상태 스냅샷을 반환한다.
// REQ-RL-001: copy 반환(read-only 계약).
// REQ-RL-008: 미파싱 provider는 IsEmpty()==true인 zero-value 반환.
func (t *Tracker) State(provider string) RateLimitState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.states[provider]
	if !ok {
		return RateLimitState{}
	}
	// copy 반환
	return *s
}

// Display는 provider의 rate-limit 상태를 human-readable 문자열로 반환한다.
// REQ-RL-007b: stale 버킷에 [STALE] 마커 포함.
// AC-RL-008: IsEmpty이면 "no rate limit information yet" 반환.
func (t *Tracker) Display(provider string) string {
	state := t.State(provider)
	now := time.Now()
	return FormatDisplay(state, now)
}

// RegisterParser는 parser를 Tracker에 등록한다.
// 동일 provider 이름으로 재등록하면 덮어쓴다.
func (t *Tracker) RegisterParser(p Parser) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.opts.Parsers[p.Provider()] = p
}

// ParseHTTPHeader는 http.Header를 map[string]string으로 변환하여 Parse를 호출한다.
// 멀티값 헤더의 경우 첫 번째 값만 사용한다.
// 기존 provider 코드(ADAPTER-001 등)와의 하위 호환성을 위한 어댑터이다.
func (t *Tracker) ParseHTTPHeader(provider string, h http.Header, now time.Time) error {
	if h == nil {
		return t.Parse(provider, nil, now)
	}
	headers := make(map[string]string, len(h))
	for k, vs := range h {
		if len(vs) > 0 {
			headers[k] = vs[0]
		}
	}
	return t.Parse(provider, headers, now)
}

// NewTracker는 기본 옵션으로 Tracker를 생성하는 하위 호환 생성자이다.
// ADAPTER-001 등 기존 provider 코드에서 parser 없이 noop tracker가 필요한 경우 사용한다.
// 실질적인 헤더 파싱은 RegisterParser 후 ParseHTTPHeader로 수행한다.
func NewTracker() *Tracker {
	t, _ := New(TrackerOptions{
		ThresholdPct: defaultThresholdPct,
		WarnCooldown: defaultWarnCooldown,
	})
	return t
}
