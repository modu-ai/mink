package ratelimit_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// ─── RED #1: Bucket 유도 속성 ───────────────────────────────────────────────

// TestBucket_UsagePct_ZeroLimit은 Limit==0일 때 UsagePct가 0.0을 반환하는지 검증한다.
// REQ-RL-002, §6.7 RED #1.
func TestBucket_UsagePct_ZeroLimit(t *testing.T) {
	t.Parallel()
	b := ratelimit.RateLimitBucket{}
	assert.Equal(t, 0.0, b.UsagePct(), "Limit==0일 때 UsagePct는 0.0이어야 한다")
}

// TestBucket_Used_NeverNegative는 Remaining > Limit이어도 Used()가 음수가 되지 않는지 검증한다.
// REQ-RL-002.
func TestBucket_Used_NeverNegative(t *testing.T) {
	t.Parallel()
	b := ratelimit.RateLimitBucket{Limit: 10, Remaining: 20}
	assert.GreaterOrEqual(t, b.Used(), 0, "Used()는 항상 >= 0이어야 한다")
}

// TestBucket_UsagePct_Normal은 정상 계산을 검증한다.
func TestBucket_UsagePct_Normal(t *testing.T) {
	t.Parallel()
	b := ratelimit.RateLimitBucket{Limit: 1000, Remaining: 200}
	assert.InDelta(t, 80.0, b.UsagePct(), 0.001)
}

// TestBucket_RemainingSecondsNow_Stale은 stale 버킷이 0.0을 반환하는지 검증한다.
// REQ-RL-007a.
func TestBucket_RemainingSecondsNow_Stale(t *testing.T) {
	t.Parallel()
	now := time.Now()
	b := ratelimit.RateLimitBucket{
		Limit:        1000,
		Remaining:    200,
		ResetSeconds: 60,
		CapturedAt:   now.Add(-120 * time.Second), // 120초 전 캡처, 60초 후 리셋 → stale
	}
	assert.Equal(t, 0.0, b.RemainingSecondsNow(now))
}

// TestBucket_IsStale_NotStale은 아직 리셋 시간이 지나지 않은 버킷이 stale이 아닌지 검증한다.
func TestBucket_IsStale_NotStale(t *testing.T) {
	t.Parallel()
	now := time.Now()
	b := ratelimit.RateLimitBucket{
		Limit:        1000,
		Remaining:    200,
		ResetSeconds: 60,
		CapturedAt:   now.Add(-30 * time.Second), // 30초 전 캡처, 60초 후 리셋 → 아직 30초 남음
	}
	assert.False(t, b.IsStale(now))
	assert.InDelta(t, 30.0, b.RemainingSecondsNow(now), 0.1)
}

// ─── RED #2: OpenAI parser happy path (AC-RL-001) ────────────────────────────

// TestOpenAIParser_HappyPath는 OpenAI 헤더로부터 4-bucket을 올바르게 파싱하는지 검증한다.
// AC-RL-001.
func TestOpenAIParser_HappyPath(t *testing.T) {
	t.Parallel()
	p := ratelimit.NewOpenAIParser("openai")
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "800",
		"x-ratelimit-reset-requests":     "60s",
		"x-ratelimit-limit-tokens":       "200000",
		"x-ratelimit-remaining-tokens":   "150000",
		"x-ratelimit-reset-tokens":       "30s",
	}

	state, debugMsgs := p.Parse(headers, now)
	assert.Empty(t, debugMsgs)
	assert.Equal(t, "openai", state.Provider)
	assert.Equal(t, 1000, state.RequestsMin.Limit)
	assert.Equal(t, 800, state.RequestsMin.Remaining)
	assert.InDelta(t, 60.0, state.RequestsMin.ResetSeconds, 0.001)
	assert.InDelta(t, 20.0, state.RequestsMin.UsagePct(), 0.001)
	assert.Equal(t, 200000, state.TokensMin.Limit)
	assert.Equal(t, 150000, state.TokensMin.Remaining)
}

// TestOpenAIParser_CaseInsensitiveHeaders는 헤더 키 대소문자 무관 파싱을 검증한다.
func TestOpenAIParser_CaseInsensitiveHeaders(t *testing.T) {
	t.Parallel()
	p := ratelimit.NewOpenAIParser("openai")
	now := time.Now()
	headers := map[string]string{
		"X-RateLimit-Limit-Requests":     "500",
		"X-RateLimit-Remaining-Requests": "400",
		"X-RateLimit-Reset-Requests":     "30s",
	}
	state, _ := p.Parse(headers, now)
	assert.Equal(t, 500, state.RequestsMin.Limit)
}

// ─── RED #3: Tracker 임계치 이벤트 (AC-RL-002) ────────────────────────────────

// spyObserver는 테스트에서 Event를 기록하는 관찰자이다.
type spyObserver struct {
	mu     sync.Mutex
	events []ratelimit.Event
}

func (s *spyObserver) OnRateLimitEvent(e ratelimit.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
}

func (s *spyObserver) Events() []ratelimit.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]ratelimit.Event, len(s.events))
	copy(cp, s.events)
	return cp
}

// TestTracker_ThresholdEventEmittedOnce는 85% 사용률에서 Event가 1회 발화하는지 검증한다.
// AC-RL-002.
func TestTracker_ThresholdEventEmittedOnce(t *testing.T) {
	t.Parallel()
	spy := &spyObserver{}
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		Observers:    []ratelimit.Observer{spy},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	now := time.Now()
	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "150", // 사용률 85%
		"x-ratelimit-reset-requests":     "60s",
	}
	err = tr.Parse("openai", headers, now)
	require.NoError(t, err)

	events := spy.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "openai", events[0].Provider)
	assert.Equal(t, ratelimit.BucketRequestsMin, events[0].BucketType)
	assert.InDelta(t, 85.0, events[0].UsagePct, 0.001)
}

// TestTracker_NoEventBelowThreshold는 임계치 미만 사용률에서 Event가 발화하지 않는지 검증한다.
func TestTracker_NoEventBelowThreshold(t *testing.T) {
	t.Parallel()
	spy := &spyObserver{}
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		Observers:    []ratelimit.Observer{spy},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "250", // 사용률 75% → 80% 미만
		"x-ratelimit-reset-requests":     "60s",
	}
	require.NoError(t, tr.Parse("openai", headers, time.Now()))
	assert.Empty(t, spy.Events())
}

// ─── RED #4: 쿨다운 억제 (AC-RL-003) ────────────────────────────────────────

// TestTracker_CooldownSuppressesDuplicate_ThenFiresAfterWindow는 쿨다운 내 중복 억제와
// 쿨다운 경과 후 재발화를 검증한다.
// AC-RL-003.
func TestTracker_CooldownSuppressesDuplicate_ThenFiresAfterWindow(t *testing.T) {
	t.Parallel()
	spy := &spyObserver{}
	logger := zaptest.NewLogger(t)
	cooldown := 30 * time.Second
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		Observers:    []ratelimit.Observer{spy},
		ThresholdPct: 80.0,
		WarnCooldown: cooldown,
		Logger:       logger,
	})
	require.NoError(t, err)

	base := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "100", // 90%
		"x-ratelimit-reset-requests":     "60s",
	}

	// 첫 번째 Parse → Event 발화
	require.NoError(t, tr.Parse("openai", headers, base))
	assert.Len(t, spy.Events(), 1)

	// 10초 후 → 쿨다운 내 → 억제
	require.NoError(t, tr.Parse("openai", headers, base.Add(10*time.Second)))
	assert.Len(t, spy.Events(), 1, "쿨다운 내에서는 중복 Event가 억제되어야 한다")

	// 35초 후 → 쿨다운 경과 → 재발화
	require.NoError(t, tr.Parse("openai", headers, base.Add(35*time.Second)))
	assert.Len(t, spy.Events(), 2, "쿨다운 경과 후 Event가 재발화해야 한다")
}

// ─── RED #5: Anthropic ISO 8601 정규화 (AC-RL-004) ───────────────────────────

// TestAnthropicParser_ISO8601ResetNormalized_DeterministicNow는 Anthropic ISO 8601 reset을
// 결정적 now 기준으로 변환하는지 검증한다.
// AC-RL-004.
func TestAnthropicParser_ISO8601ResetNormalized_DeterministicNow(t *testing.T) {
	t.Parallel()
	p := ratelimit.NewAnthropicParser()
	// AC-RL-004: now = 2026-04-21T11:59:34Z, reset = 2026-04-21T12:00:00Z → 26초
	now := time.Date(2026, 4, 21, 11, 59, 34, 0, time.UTC)
	headers := map[string]string{
		"anthropic-ratelimit-requests-limit":     "500",
		"anthropic-ratelimit-requests-remaining": "400",
		"anthropic-ratelimit-requests-reset":     "2026-04-21T12:00:00Z",
		"anthropic-ratelimit-tokens-limit":       "2000000",
		"anthropic-ratelimit-tokens-remaining":   "1500000",
		"anthropic-ratelimit-tokens-reset":       "2026-04-21T12:01:00Z",
	}

	state, debugMsgs := p.Parse(headers, now)
	assert.Empty(t, debugMsgs)
	assert.InDelta(t, 26.0, state.RequestsMin.ResetSeconds, 0.001, "RFC3339 reset 변환 오차가 ±0.001 이내여야 한다")
	assert.Equal(t, 500, state.RequestsMin.Limit)
	assert.Equal(t, 400, state.RequestsMin.Remaining)
}

// ─── RED #6: Malformed 헤더 graceful (AC-RL-005) ─────────────────────────────

// TestParser_MalformedHeader_ZeroValue는 잘못된 limit 값이 zero-value로 처리되고
// 다른 필드는 정상 파싱되는지 검증한다.
// AC-RL-005.
func TestParser_MalformedHeader_ZeroValue(t *testing.T) {
	t.Parallel()
	p := ratelimit.NewOpenAIParser("openai")
	now := time.Now()
	headers := map[string]string{
		"x-ratelimit-limit-requests":     "abc", // 잘못된 int
		"x-ratelimit-remaining-requests": "800",
		"x-ratelimit-reset-requests":     "60s",
		"x-ratelimit-limit-tokens":       "200000",
		"x-ratelimit-remaining-tokens":   "150000",
		"x-ratelimit-reset-tokens":       "30s",
	}

	state, debugMsgs := p.Parse(headers, now)
	// DEBUG 메시지가 1건 이상 있어야 함
	assert.NotEmpty(t, debugMsgs, "malformed 헤더에 대한 DEBUG 메시지가 있어야 한다")
	// limit이 잘못된 경우 해당 버킷은 zero-value
	assert.Equal(t, 0, state.RequestsMin.Limit, "malformed limit은 0이어야 한다")
	// tokens 버킷은 정상 파싱
	assert.Equal(t, 200000, state.TokensMin.Limit, "다른 버킷은 정상 파싱되어야 한다")
}

// ─── RED #7: Stale 버킷 표시 (AC-RL-006) ─────────────────────────────────────

// TestBucket_StaleDetection_DisplayContainsSTALEMarker는 stale 버킷이 [STALE] 마커를
// 포함하고 RemainingSecondsNow가 0.0을 반환하는지 검증한다.
// AC-RL-006, REQ-RL-007a/007b.
func TestBucket_StaleDetection_DisplayContainsSTALEMarker(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	// 120초 전에 60초 리셋으로 파싱 → stale
	pastNow := time.Now().Add(-120 * time.Second)
	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "200",
		"x-ratelimit-reset-requests":     "60s",
	}
	require.NoError(t, tr.Parse("openai", headers, pastNow))

	// AC-RL-006: Stale 검증
	state := tr.State("openai")
	now := time.Now()
	assert.True(t, state.RequestsMin.IsStale(now), "버킷이 stale이어야 한다")
	assert.Equal(t, 0.0, state.RequestsMin.RemainingSecondsNow(now), "stale이면 RemainingSecondsNow==0.0")
	assert.InDelta(t, 80.0, state.RequestsMin.UsagePct(), 0.001, "stale이어도 UsagePct는 유지")

	// Display에 [STALE] 포함 검증
	display := ratelimit.FormatDisplay(state, now)
	assert.Contains(t, display, "[STALE]", "Display에 [STALE] 마커가 포함되어야 한다")
}

// ─── RED #8: 병렬 Parse 경쟁 조건 (AC-RL-007) ───────────────────────────────

// TestTracker_ConcurrentParse_RaceDetectorPasses_WithInvariants는 100개 goroutine의
// 동시 Parse가 race detector를 통과하고 invariant를 유지하는지 검증한다.
// AC-RL-007.
func TestTracker_ConcurrentParse_RaceDetectorPasses_WithInvariants(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		ThresholdPct: 80.0,
		WarnCooldown: 1 * time.Millisecond,
		Logger:       logger,
	})
	require.NoError(t, err)

	now := time.Now()
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(remaining int) {
			defer wg.Done()
			headers := map[string]string{
				"x-ratelimit-limit-requests":     "1000",
				"x-ratelimit-remaining-requests": fmt.Sprint(remaining),
				"x-ratelimit-reset-requests":     "60s",
			}
			_ = tr.Parse("openai", headers, now)
		}(i)
	}
	wg.Wait()

	// AC-RL-007 invariants
	state := tr.State("openai")
	// (2) Limit은 항상 1000
	assert.Equal(t, 1000, state.RequestsMin.Limit, "Limit은 1000이어야 한다")
	// (3) Remaining은 0..99 범위 내
	assert.GreaterOrEqual(t, state.RequestsMin.Remaining, 0)
	assert.LessOrEqual(t, state.RequestsMin.Remaining, 99)
}

// ─── RED #9: 미파싱 provider의 IsEmpty (AC-RL-008) ───────────────────────────

// TestTracker_IsEmptyOnUnseenProvider는 아직 Parse가 호출되지 않은 provider에 대해
// IsEmpty()==true를 반환하는지 검증한다.
// AC-RL-008, REQ-RL-008.
func TestTracker_IsEmptyOnUnseenProvider(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	state := tr.State("openai")
	assert.True(t, state.IsEmpty(), "Parse 전에는 IsEmpty()==true")
	assert.Equal(t, 0, state.RequestsMin.Limit)
	assert.Equal(t, 0, state.TokensMin.Limit)

	display := ratelimit.FormatDisplay(state, time.Now())
	assert.Contains(t, display, "no rate limit information yet")
}

// ─── RED #10: 미등록 Parser ErrParserNotRegistered (AC-RL-009) ───────────────

// TestTracker_ErrParserNotRegistered_NoStateMutation은 미등록 provider에 대한 Parse가
// ErrParserNotRegistered를 반환하고 상태를 변경하지 않는지 검증한다.
// AC-RL-009, REQ-RL-010.
func TestTracker_ErrParserNotRegistered_NoStateMutation(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	// 사전 anthropic state 스냅샷 (empty)
	beforeState := tr.State("anthropic")
	assert.True(t, beforeState.IsEmpty())

	headers := map[string]string{"anthropic-ratelimit-requests-limit": "500"}
	parseErr := tr.Parse("anthropic", headers, time.Now())
	require.Error(t, parseErr)

	// ErrParserNotRegistered 타입 검증
	var notRegErr ratelimit.ErrParserNotRegistered
	require.ErrorAs(t, parseErr, &notRegErr)
	assert.Equal(t, "anthropic", notRegErr.Provider)

	// 상태 불변 확인
	afterState := tr.State("anthropic")
	assert.True(t, afterState.IsEmpty(), "에러 후 state가 변경되지 않아야 한다")

	// OpenAI 상태도 영향 없음
	openaiState := tr.State("openai")
	assert.True(t, openaiState.IsEmpty())
}

// ─── RED #11: nil 입력 방어 (AC-RL-010) ─────────────────────────────────────

// TestTracker_NilInputs_DoNotPanic은 nil headers, nil observers, zero-time now에서
// panic이 발생하지 않는지 검증한다.
// AC-RL-010, REQ-RL-011a/011b/011c.
func TestTracker_NilInputs_DoNotPanic(t *testing.T) {
	t.Parallel()

	t.Run("nil_headers", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
		tr, err := ratelimit.New(ratelimit.TrackerOptions{
			Parsers: map[string]ratelimit.Parser{
				"openai": ratelimit.NewOpenAIParser("openai"),
			},
			ThresholdPct: 80.0,
			WarnCooldown: 60 * time.Second,
			Logger:       logger,
		})
		require.NoError(t, err)

		// AC-RL-010 Case A
		require.NotPanics(t, func() {
			_ = tr.Parse("openai", nil, time.Now())
		})
		assert.True(t, tr.State("openai").IsEmpty(), "nil headers 후 state는 empty여야 한다")
	})

	t.Run("nil_observers", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		// AC-RL-010 Case B: Observers == nil
		tr, err := ratelimit.New(ratelimit.TrackerOptions{
			Parsers: map[string]ratelimit.Parser{
				"openai": ratelimit.NewOpenAIParser("openai"),
			},
			Observers:    nil,
			ThresholdPct: 80.0,
			WarnCooldown: 60 * time.Second,
			Logger:       logger,
		})
		require.NoError(t, err)

		headers := map[string]string{
			"x-ratelimit-limit-requests":     "1000",
			"x-ratelimit-remaining-requests": "100", // 90%
			"x-ratelimit-reset-requests":     "60s",
		}
		require.NotPanics(t, func() {
			_ = tr.Parse("openai", headers, time.Now())
		})
	})

	t.Run("zero_time_now", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
		tr, err := ratelimit.New(ratelimit.TrackerOptions{
			Parsers: map[string]ratelimit.Parser{
				"openai": ratelimit.NewOpenAIParser("openai"),
			},
			ThresholdPct: 80.0,
			WarnCooldown: 60 * time.Second,
			Logger:       logger,
		})
		require.NoError(t, err)

		headers := map[string]string{
			"x-ratelimit-limit-requests":     "1000",
			"x-ratelimit-remaining-requests": "800",
			"x-ratelimit-reset-requests":     "60s",
		}
		// AC-RL-010 Case C
		require.NotPanics(t, func() {
			_ = tr.Parse("openai", headers, time.Time{})
		})
		assert.True(t, tr.State("openai").IsEmpty(), "zero-time 후 state는 empty여야 한다")
	})
}

// ─── RED #12: Observer 순서 보존 및 에러 격리 (AC-RL-011) ────────────────────

// orderRecordingObserver는 호출 순서와 에러 반환을 테스트하는 관찰자이다.
type orderRecordingObserver struct {
	mu    sync.Mutex
	calls []string
	id    string
}

func (o *orderRecordingObserver) OnRateLimitEvent(_ ratelimit.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls = append(o.calls, o.id)
}

func (o *orderRecordingObserver) CallIDs() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	cp := make([]string, len(o.calls))
	copy(cp, o.calls)
	return cp
}

// callOrderTracker는 전역 호출 순서를 기록한다.
type callOrderTracker struct {
	mu    sync.Mutex
	order []string
}

func (c *callOrderTracker) observe(id string) ratelimit.Observer {
	return &callOrderObserver{tracker: c, id: id}
}

type callOrderObserver struct {
	tracker *callOrderTracker
	id      string
}

func (o *callOrderObserver) OnRateLimitEvent(_ ratelimit.Event) {
	o.tracker.mu.Lock()
	defer o.tracker.mu.Unlock()
	o.tracker.order = append(o.tracker.order, o.id)
}

// TestTracker_ObserverOrder_ErrorIsolation은 3개 observer가 등록 순서대로 호출되고
// obs2가 있어도 obs3까지 호출되는지 검증한다.
// AC-RL-011, REQ-RL-012.
func TestTracker_ObserverOrder_ErrorIsolation(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tracker := &callOrderTracker{}
	obs1 := tracker.observe("obs1")
	obs2 := tracker.observe("obs2")
	obs3 := tracker.observe("obs3")

	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		Observers:    []ratelimit.Observer{obs1, obs2, obs3},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "100", // 90%
		"x-ratelimit-reset-requests":     "60s",
	}
	require.NoError(t, tr.Parse("openai", headers, time.Now()))

	// 호출 순서 검증
	order := tracker.order
	require.GreaterOrEqual(t, len(order), 3, "3개 observer가 모두 호출되어야 한다")
	// requests_min 이벤트에 대해 obs1, obs2, obs3 순서
	assert.Equal(t, "obs1", order[0])
	assert.Equal(t, "obs2", order[1])
	assert.Equal(t, "obs3", order[2])
}

// ─── RED #13: ThresholdPct 경계 검증 (AC-RL-012) ────────────────────────────

// TestTracker_ThresholdPctBounds_ValidationAtNew는 ThresholdPct 범위 검증을 검증한다.
// AC-RL-012, REQ-RL-013.
func TestTracker_ThresholdPctBounds_ValidationAtNew(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	validCases := []float64{50.0, 75.0, 80.0, 100.0}
	for _, tc := range validCases {
		tc := tc
		t.Run(fmt.Sprintf("valid_%.1f", tc), func(t *testing.T) {
			t.Parallel()
			tr, err := ratelimit.New(ratelimit.TrackerOptions{
				Parsers:      map[string]ratelimit.Parser{"openai": ratelimit.NewOpenAIParser("openai")},
				ThresholdPct: tc,
				WarnCooldown: 60 * time.Second,
				Logger:       logger,
			})
			require.NoError(t, err)
			require.NotNil(t, tr)
		})
	}

	// 0.0은 미설정(zero value) → 기본값 80.0으로 처리; 명시적 잘못된 값만 검증
	invalidCases := []float64{49.9, 100.1, 200.0}
	for _, tc := range invalidCases {
		tc := tc
		t.Run(fmt.Sprintf("invalid_%.1f", tc), func(t *testing.T) {
			t.Parallel()
			tr, err := ratelimit.New(ratelimit.TrackerOptions{
				Parsers:      map[string]ratelimit.Parser{"openai": ratelimit.NewOpenAIParser("openai")},
				ThresholdPct: tc,
				WarnCooldown: 60 * time.Second,
				Logger:       logger,
			})
			require.Error(t, err, "ThresholdPct %.1f는 에러를 반환해야 한다", tc)
			assert.Nil(t, tr, "부분 생성 금지")
			var invalidErr ratelimit.ErrInvalidThreshold
			assert.ErrorAs(t, err, &invalidErr)
		})
	}
}

// ─── 추가: OpenRouter parser 검증 ────────────────────────────────────────────

// TestOpenRouterParser_SameAsOpenAI는 OpenRouter parser가 OpenAI와 동일 포맷을 처리하는지 검증한다.
func TestOpenRouterParser_SameAsOpenAI(t *testing.T) {
	t.Parallel()
	p := ratelimit.NewOpenRouterParser()
	assert.Equal(t, "openrouter", p.Provider())
	now := time.Now()
	headers := map[string]string{
		"x-ratelimit-limit-requests":     "500",
		"x-ratelimit-remaining-requests": "400",
		"x-ratelimit-reset-requests":     "30s",
	}
	state, debugMsgs := p.Parse(headers, now)
	assert.Empty(t, debugMsgs)
	assert.Equal(t, 500, state.RequestsMin.Limit)
}

// ─── 추가: Tracker State copy 계약 검증 ─────────────────────────────────────

// TestTracker_State_ReturnsCopy는 State()가 copy를 반환하는지 검증한다.
// REQ-RL-001.
func TestTracker_State_ReturnsCopy(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "800",
		"x-ratelimit-reset-requests":     "60s",
	}
	require.NoError(t, tr.Parse("openai", headers, time.Now()))

	state1 := tr.State("openai")
	state1.RequestsMin.Limit = 9999 // 변경

	state2 := tr.State("openai")
	assert.Equal(t, 1000, state2.RequestsMin.Limit, "State()는 독립적인 copy를 반환해야 한다")
}

// ─── 추가: Provider() 메서드 검증 ──────────────────────────────────────────────

// TestParsers_Provider는 각 parser의 Provider() 반환값을 검증한다.
func TestParsers_Provider(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "openai", ratelimit.NewOpenAIParser("openai").Provider())
	assert.Equal(t, "groq", ratelimit.NewOpenAIParser("groq").Provider())
	assert.Equal(t, "anthropic", ratelimit.NewAnthropicParser().Provider())
	assert.Equal(t, "openrouter", ratelimit.NewOpenRouterParser().Provider())
}

// ─── 추가: Tracker.Display() 검증 ────────────────────────────────────────────

// TestTracker_Display_AfterParse는 Display()가 파싱 후 의미있는 문자열을 반환하는지 검증한다.
func TestTracker_Display_AfterParse(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "800",
		"x-ratelimit-reset-requests":     "60s",
	}
	require.NoError(t, tr.Parse("openai", headers, time.Now()))

	display := tr.Display("openai")
	assert.Contains(t, display, "requests_min")
}

// TestTracker_Display_Empty는 미파싱 provider에 대해 Display()가 의미있는 문자열을 반환하는지 검증한다.
func TestTracker_Display_Empty(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	display := tr.Display("openai")
	assert.Contains(t, display, "no rate limit information yet")
}

// ─── 추가: RegisterParser 검증 ───────────────────────────────────────────────

// TestTracker_RegisterParser는 Parse 후 RegisterParser로 추가된 parser가 동작하는지 검증한다.
func TestTracker_RegisterParser(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	// 초기에는 openai parser 없음
	parseErr := tr.Parse("openai", map[string]string{}, time.Now())
	require.Error(t, parseErr)

	// RegisterParser로 추가
	tr.RegisterParser(ratelimit.NewOpenAIParser("openai"))

	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "800",
		"x-ratelimit-reset-requests":     "60s",
	}
	require.NoError(t, tr.Parse("openai", headers, time.Now()))
	state := tr.State("openai")
	assert.False(t, state.IsEmpty())
}

// ─── 추가: parser 에러 분기 커버리지 ──────────────────────────────────────────

// TestParser_MalformedDuration은 잘못된 duration 포맷을 DEBUG 메시지로 처리하는지 검증한다.
func TestParser_MalformedDuration(t *testing.T) {
	t.Parallel()
	p := ratelimit.NewOpenAIParser("openai")
	now := time.Now()
	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "800",
		"x-ratelimit-reset-requests":     "notaduration", // 잘못된 duration
	}
	state, debugMsgs := p.Parse(headers, now)
	assert.NotEmpty(t, debugMsgs)
	assert.Equal(t, 1000, state.RequestsMin.Limit, "limit은 파싱 성공")
	assert.Equal(t, 0.0, state.RequestsMin.ResetSeconds, "reset은 0으로 fallback")
}

// TestAnthropicParser_MalformedReset은 잘못된 ISO 8601 reset을 DEBUG 메시지로 처리하는지 검증한다.
func TestAnthropicParser_MalformedReset(t *testing.T) {
	t.Parallel()
	p := ratelimit.NewAnthropicParser()
	now := time.Now()
	headers := map[string]string{
		"anthropic-ratelimit-requests-limit":     "500",
		"anthropic-ratelimit-requests-remaining": "400",
		"anthropic-ratelimit-requests-reset":     "not-a-date",
	}
	_, debugMsgs := p.Parse(headers, now)
	assert.NotEmpty(t, debugMsgs)
}

// TestAnthropicParser_PastReset은 reset 시간이 now보다 과거인 경우 0.0을 반환하는지 검증한다.
func TestAnthropicParser_PastReset(t *testing.T) {
	t.Parallel()
	p := ratelimit.NewAnthropicParser()
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	headers := map[string]string{
		"anthropic-ratelimit-requests-limit":     "500",
		"anthropic-ratelimit-requests-remaining": "400",
		"anthropic-ratelimit-requests-reset":     "2026-04-21T11:00:00Z", // 과거
	}
	state, _ := p.Parse(headers, now)
	assert.Equal(t, 0.0, state.RequestsMin.ResetSeconds)
}

// TestTracker_New_DefaultValues는 ThresholdPct 미설정 시 기본값 80을 사용하는지 검증한다.
func TestTracker_New_DefaultValues(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		Logger: logger,
		// ThresholdPct, WarnCooldown 미설정 → 기본값 사용
	})
	require.NoError(t, err)
	require.NotNil(t, tr)
}

// TestNewTracker_CompatConstructor는 NewTracker()가 nil이 아닌 Tracker를 반환하는지 검증한다.
// 기존 provider 코드와의 하위 호환성 확인.
func TestNewTracker_CompatConstructor(t *testing.T) {
	t.Parallel()
	tr := ratelimit.NewTracker()
	require.NotNil(t, tr)
	// nop tracker이므로 parser 없이 Parse 시 ErrParserNotRegistered
	err := tr.Parse("openai", map[string]string{}, time.Now())
	var notRegErr ratelimit.ErrParserNotRegistered
	assert.ErrorAs(t, err, &notRegErr)
}

// TestTracker_ParseHTTPHeader는 http.Header를 올바르게 변환하여 파싱하는지 검증한다.
func TestTracker_ParseHTTPHeader(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	h := http.Header{}
	h.Set("X-Ratelimit-Limit-Requests", "1000")
	h.Set("X-Ratelimit-Remaining-Requests", "800")
	h.Set("X-Ratelimit-Reset-Requests", "60s")

	require.NoError(t, tr.ParseHTTPHeader("openai", h, time.Now()))
	state := tr.State("openai")
	assert.False(t, state.IsEmpty())
	assert.Equal(t, 1000, state.RequestsMin.Limit)
}

// TestTracker_ParseHTTPHeader_Nil은 nil http.Header를 안전하게 처리하는지 검증한다.
func TestTracker_ParseHTTPHeader_Nil(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		ThresholdPct: 80.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)
	require.NotPanics(t, func() {
		_ = tr.ParseHTTPHeader("openai", nil, time.Now())
	})
	assert.True(t, tr.State("openai").IsEmpty())
}

// ─── 추가: Tracker ThresholdPct 75% 커스텀 검증 (AC-RL-012 Case A) ──────────

// TestTracker_CustomThreshold75_FiresAt75Pct는 ThresholdPct=75.0 설정 시
// 75% 사용률에서 Event가 발화하는지 검증한다.
// AC-RL-012 Case A.
func TestTracker_CustomThreshold75_FiresAt75Pct(t *testing.T) {
	t.Parallel()
	spy := &spyObserver{}
	logger := zaptest.NewLogger(t)
	tr, err := ratelimit.New(ratelimit.TrackerOptions{
		Parsers: map[string]ratelimit.Parser{
			"openai": ratelimit.NewOpenAIParser("openai"),
		},
		Observers:    []ratelimit.Observer{spy},
		ThresholdPct: 75.0,
		WarnCooldown: 60 * time.Second,
		Logger:       logger,
	})
	require.NoError(t, err)

	headers := map[string]string{
		"x-ratelimit-limit-requests":     "1000",
		"x-ratelimit-remaining-requests": "250", // 75%
		"x-ratelimit-reset-requests":     "60s",
	}
	require.NoError(t, tr.Parse("openai", headers, time.Now()))
	assert.Len(t, spy.Events(), 1, "75% 사용률에서 ThresholdPct=75.0이면 Event가 발화해야 한다")
}
