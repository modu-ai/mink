// Package expvar_test verifies the expvar-backed metrics.Sink implementation.
// SPEC-GOOSE-OBS-METRICS-001 T-004/T-005/T-006.
package expvar_test

import (
	"fmt"
	"testing"

	"github.com/modu-ai/goose/internal/observability/metrics"
	metricsexpvar "github.com/modu-ai/goose/internal/observability/metrics/expvar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newTestSink creates a fresh expvar sink backed by a zap NopLogger.
// Tests that need to observe warn logs use newObservedSink instead.
func newTestSink() metrics.Sink {
	return metricsexpvar.New(zap.NewNop())
}

// newObservedSink returns a sink + observed log core for asserting warn emissions.
func newObservedSink() (metrics.Sink, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	return metricsexpvar.New(logger), logs
}

// ─── T-004: Counter ──────────────────────────────────────────────────────────

// TestExpvarSink_Counter_Increments verifies Inc and Add behaviour (AC-006).
// Each sink instance has its own private registry, so concurrent tests can
// use the same metric names without colliding.
func TestExpvarSink_Counter_Increments(t *testing.T) {
	t.Parallel()
	s := newTestSink()

	c := s.Counter("test.counter", nil)

	for range 100 {
		c.Inc()
	}

	// Read back via a second handle — same (name, labels) → same underlying counter.
	c2 := s.Counter("test.counter", nil)
	val := metricsexpvar.CounterValue(c2)
	assert.InDelta(t, 100.0, val, 0.001, "100 × Inc() should yield 100")

	c.Add(2.5)
	val = metricsexpvar.CounterValue(c)
	assert.InDelta(t, 102.5, val, 0.001, "Add(2.5) after 100 × Inc() should yield 102.5")
}

// TestExpvarSink_Labels_NameMangling verifies that different label combinations
// map to different underlying counters, and the same combination accumulates
// into the same counter (AC-009, REQ-004).
func TestExpvarSink_Labels_NameMangling(t *testing.T) {
	t.Parallel()
	s := newTestSink()

	c1 := s.Counter("cmdctx.method.calls", metrics.Labels{"method": "OnClear"})
	c2 := s.Counter("cmdctx.method.calls", metrics.Labels{"method": "OnCompact"})

	c1.Inc()
	c2.Add(3)

	v1 := metricsexpvar.CounterValue(c1)
	v2 := metricsexpvar.CounterValue(c2)

	assert.InDelta(t, 1.0, v1, 0.001, "OnClear counter should be 1")
	assert.InDelta(t, 3.0, v2, 0.001, "OnCompact counter should be 3")

	// Same (name, labels) accumulates into the same counter.
	c1b := s.Counter("cmdctx.method.calls", metrics.Labels{"method": "OnClear"})
	c1b.Inc()
	v1b := metricsexpvar.CounterValue(c1b)
	assert.InDelta(t, 2.0, v1b, 0.001, "second OnClear handle should see accumulated value")
}

// TestExpvarSink_CardinalityCap_DropsAndWarns verifies the cardinality cap:
//   - first 100 unique label combinations register successfully,
//   - 101st returns a noop handle (silent drop),
//   - exactly one warn log is emitted (AC-010, REQ-009, REQ-015).
func TestExpvarSink_CardinalityCap_DropsAndWarns(t *testing.T) {
	t.Parallel()
	s, logs := newObservedSink()

	const cap = 100
	counters := make([]metrics.Counter, cap)
	for i := range cap {
		counters[i] = s.Counter("cap.test", metrics.Labels{"id": fmt.Sprintf("v%d", i)})
	}

	// Each of the 100 counters should be functional (value starts at 0).
	for i, c := range counters {
		c.Inc()
		val := metricsexpvar.CounterValue(c)
		require.InDeltaf(t, 1.0, val, 0.001, "counter[%d] should increment", i)
	}

	// 101st unique combination → should drop + warn exactly once.
	overflow := s.Counter("cap.test", metrics.Labels{"id": "overflow"})
	overflow.Inc() // must not panic

	// The overflow counter must not interfere with the legitimate counters.
	val0 := metricsexpvar.CounterValue(counters[0])
	assert.InDelta(t, 1.0, val0, 0.001, "overflow drop must not corrupt counter[0]")

	// Exactly one warn log.
	warnLogs := logs.FilterMessage("metrics cardinality cap exceeded")
	assert.Equal(t, 1, warnLogs.Len(),
		"expected exactly 1 cardinality warn log, got %d", warnLogs.Len())

	// A second overflow call must NOT emit a second warn (warn-once, sync.Once).
	overflow2 := s.Counter("cap.test", metrics.Labels{"id": "overflow2"})
	overflow2.Inc()
	warnLogs2 := logs.FilterMessage("metrics cardinality cap exceeded")
	assert.Equal(t, 1, warnLogs2.Len(), "warn-once: second overflow must not re-emit")
}

// ─── T-005: Gauge ─────────────────────────────────────────────────────────────

// TestExpvarSink_Gauge_SetAndAdd verifies Gauge Set/Add semantics (AC-008).
func TestExpvarSink_Gauge_SetAndAdd(t *testing.T) {
	t.Parallel()
	s := newTestSink()

	g := s.Gauge("test.gauge", nil)
	g.Set(42.0)
	assert.InDelta(t, 42.0, metricsexpvar.GaugeValue(g), 0.001, "Set(42)")

	g.Add(8.0)
	assert.InDelta(t, 50.0, metricsexpvar.GaugeValue(g), 0.001, "Add(8) after Set(42)")

	g.Add(-30.0)
	assert.InDelta(t, 20.0, metricsexpvar.GaugeValue(g), 0.001, "Add(-30) after 50")
}

// ─── T-006: Histogram ─────────────────────────────────────────────────────────

// TestExpvarSink_Histogram_BucketsObserve verifies histogram bucket counting (AC-007).
// Expected bucket layout for [1, 10, 100]:
//
//	bucket[0]: count of values <= 1   (Observe(0.5) → 1)
//	bucket[1]: count of values <= 10  (Observe(5)   → 1)
//	bucket[2]: count of values <= 100 (Observe(50)  → 1)
//	bucket[3]: +Inf overflow bucket   (Observe(500) → 1)
func TestExpvarSink_Histogram_BucketsObserve(t *testing.T) {
	t.Parallel()
	s := newTestSink()

	h := s.Histogram("test.hist", nil, []float64{1, 10, 100})
	h.Observe(0.5)  // → bucket[0] (<=1)
	h.Observe(5)    // → bucket[1] (<=10)
	h.Observe(50)   // → bucket[2] (<=100)
	h.Observe(500)  // → bucket[3] (+Inf)

	counts := metricsexpvar.HistogramCounts(h)
	require.Len(t, counts, 4, "3 explicit buckets + 1 +Inf = 4 slots")

	// Each value lands in exactly one bucket (non-cumulative per-bucket count).
	expected := []int64{1, 1, 1, 1}
	for i, want := range expected {
		assert.Equal(t, want, counts[i], "bucket[%d] count mismatch", i)
	}
}

// TestExpvarSink_Histogram_NilBucketsUseDefault verifies fallback to
// defaultBuckets when buckets==nil (AC-007, REQ-007).
func TestExpvarSink_Histogram_NilBucketsUseDefault(t *testing.T) {
	t.Parallel()
	s := newTestSink()

	h := s.Histogram("test.hist.default", nil, nil)
	counts := metricsexpvar.HistogramCounts(h)
	// defaultBuckets=[0.1, 1, 10, 100, 1000] + +Inf = 6 slots
	assert.Equal(t, 6, len(counts),
		"nil buckets should fall back to 5 default + 1 +Inf = 6 slots")
}

// TestExpvarSink_Histogram_EmptyBucketsUseDefault verifies fallback on
// empty slice (AC-007, REQ-007).
func TestExpvarSink_Histogram_EmptyBucketsUseDefault(t *testing.T) {
	t.Parallel()
	s := newTestSink()

	h := s.Histogram("test.hist.empty", nil, []float64{})
	counts := metricsexpvar.HistogramCounts(h)
	assert.Equal(t, 6, len(counts),
		"empty buckets should fall back to 5 default + 1 +Inf = 6 slots")
}

// TestExpvarSink_Histogram_BadInputFallsBack verifies that non-ascending or
// invalid bucket input falls back to defaults without panicking (HARD #6, #7).
func TestExpvarSink_Histogram_BadInputFallsBack(t *testing.T) {
	t.Parallel()
	s := newTestSink()

	badCases := []struct {
		name    string
		buckets []float64
	}{
		{"descending", []float64{100, 10, 1}},
		{"duplicates", []float64{1, 1, 10}},
	}

	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := s.Histogram("test.hist.bad."+tc.name, nil, tc.buckets)
			counts := metricsexpvar.HistogramCounts(h)
			assert.Equal(t, 6, len(counts),
				"bad buckets %v should fall back to defaults (6 slots)", tc.buckets)
		})
	}
}

// TestExpvarSink_Histogram_OverflowBucket verifies that values exceeding the
// largest explicit bucket are counted in the +Inf slot (R1 mitigation).
func TestExpvarSink_Histogram_OverflowBucket(t *testing.T) {
	t.Parallel()
	s := newTestSink()

	h := s.Histogram("test.hist.overflow", nil, []float64{1, 10, 100})
	h.Observe(999) // lands in +Inf (bucket[3])

	counts := metricsexpvar.HistogramCounts(h)
	require.Len(t, counts, 4)
	assert.Equal(t, int64(1), counts[3], "+Inf bucket should have count 1")
	assert.Equal(t, int64(0), counts[0], "bucket[0] should be empty")
}

// TestExpvarSink_NilLogger_DefaultsToNop verifies that New(nil) does not panic
// and produces a functional sink (branch coverage for nil logger guard).
func TestExpvarSink_NilLogger_DefaultsToNop(t *testing.T) {
	t.Parallel()
	s := metricsexpvar.New(nil) // nil logger should be replaced by zap.NewNop()
	c := s.Counter("nil.logger.counter", nil)
	c.Inc()
	assert.InDelta(t, 1.0, metricsexpvar.CounterValue(c), 0.001)
}

// TestExpvarSink_GaugeCardinalityCap_Drops verifies that gauge cap overflow
// returns a noop handle that does not panic (coverage for Gauge cap branch).
func TestExpvarSink_GaugeCardinalityCap_Drops(t *testing.T) {
	t.Parallel()
	s, _ := newObservedSink()

	for i := range 100 {
		s.Gauge("gauge.cap.test", metrics.Labels{"id": fmt.Sprintf("v%d", i)})
	}

	// 101st should return noop gauge; operations must not panic.
	g := s.Gauge("gauge.cap.test", metrics.Labels{"id": "overflow"})
	g.Set(99.0)
	g.Add(-1.0)
}

// TestExpvarSink_HistogramCardinalityCap_Drops verifies that histogram cap
// overflow returns a noop handle (coverage for Histogram cap branch).
func TestExpvarSink_HistogramCardinalityCap_Drops(t *testing.T) {
	t.Parallel()
	s, _ := newObservedSink()

	for i := range 100 {
		s.Histogram("hist.cap.test", metrics.Labels{"id": fmt.Sprintf("v%d", i)}, nil)
	}

	// 101st should return noop histogram; operations must not panic.
	h := s.Histogram("hist.cap.test", metrics.Labels{"id": "overflow"}, nil)
	h.Observe(42.0)
}

// TestExpvarSink_GetOrCreateFloat_ReuseExisting exercises the getOrCreateFloat
// path where a key is already registered in the global expvar registry.
// We achieve this by creating a counter, then creating a new sink and requesting
// the same key — the second New call should get the existing expvar.Float.
func TestExpvarSink_GetOrCreateFloat_ReuseExisting(t *testing.T) {
	t.Parallel()

	// Use a unique name to avoid collisions with parallel tests.
	const name = "reuse.existing.float.counter"

	s1 := metricsexpvar.New(zap.NewNop())
	c1 := s1.Counter(name, nil)
	c1.Inc() // registers the expvar.Float globally

	// Create a new sink (fresh internal map) but same global expvar registry.
	s2 := metricsexpvar.New(zap.NewNop())
	c2 := s2.Counter(name, nil)
	// c2 should reuse the existing expvar.Float; its value starts where c1 left it.
	val := metricsexpvar.CounterValue(c2)
	assert.InDelta(t, 1.0, val, 0.001, "second sink should see the same expvar float")
}
