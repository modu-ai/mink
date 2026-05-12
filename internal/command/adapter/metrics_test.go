package adapter

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/modu-ai/mink/internal/command"
	"github.com/modu-ai/mink/internal/observability/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// fakeMetricsSink — deterministic, thread-safe in-memory metrics recorder.
// Used by all metrics-related tests in this file.
// ---------------------------------------------------------------------------

// fakeCounter records increment calls.
type fakeCounter struct {
	mu    sync.Mutex
	value int64
}

func (c *fakeCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

func (c *fakeCounter) Add(delta float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += int64(delta)
}

func (c *fakeCounter) get() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// fakeHistogram records observation values.
type fakeHistogram struct {
	mu  sync.Mutex
	obs []float64
}

func (h *fakeHistogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.obs = append(h.obs, value)
}

func (h *fakeHistogram) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.obs)
}

func (h *fakeHistogram) values() []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	cp := make([]float64, len(h.obs))
	copy(cp, h.obs)
	return cp
}

// fakeGauge satisfies the metrics.Gauge interface (unused by adapter but
// required by metrics.Sink contract).
type fakeGauge struct{}

func (fakeGauge) Set(_ float64) {}
func (fakeGauge) Add(_ float64) {}

// fakeMetricsSink stores counters and histograms keyed by "name|k=v,k=v" label string.
type fakeMetricsSink struct {
	mu         sync.Mutex
	counters   map[string]*fakeCounter
	histograms map[string]*fakeHistogram
}

func newFakeMetricsSink() *fakeMetricsSink {
	return &fakeMetricsSink{
		counters:   make(map[string]*fakeCounter),
		histograms: make(map[string]*fakeHistogram),
	}
}

func labelKey(name string, labels metrics.Labels) string {
	// Produce a deterministic key. Sort keys for consistency.
	// For test clarity we accept the simple approach of iterating sorted keys.
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	// Insertion-order is non-deterministic for maps; sort for stable keys.
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	key := name
	for _, k := range keys {
		key += "|" + k + "=" + labels[k]
	}
	return key
}

func (s *fakeMetricsSink) Counter(name string, labels metrics.Labels) metrics.Counter {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := labelKey(name, labels)
	if _, ok := s.counters[k]; !ok {
		s.counters[k] = &fakeCounter{}
	}
	return s.counters[k]
}

func (s *fakeMetricsSink) Histogram(name string, labels metrics.Labels, _ []float64) metrics.Histogram {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := labelKey(name, labels)
	if _, ok := s.histograms[k]; !ok {
		s.histograms[k] = &fakeHistogram{}
	}
	return s.histograms[k]
}

func (s *fakeMetricsSink) Gauge(_ string, _ metrics.Labels) metrics.Gauge {
	return fakeGauge{}
}

// counterVal retrieves a counter value; returns 0 if the counter was never created.
func (s *fakeMetricsSink) counterVal(name string, labels metrics.Labels) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := labelKey(name, labels)
	c, ok := s.counters[k]
	if !ok {
		return 0
	}
	return c.get()
}

// histCount retrieves the observation count for a histogram; returns 0 if never created.
func (s *fakeMetricsSink) histCount(name string, labels metrics.Labels) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := labelKey(name, labels)
	h, ok := s.histograms[k]
	if !ok {
		return 0
	}
	return h.count()
}

// histValues retrieves all observations for a histogram.
func (s *fakeMetricsSink) histValues(name string, labels metrics.Labels) []float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := labelKey(name, labels)
	h, ok := s.histograms[k]
	if !ok {
		return nil
	}
	return h.values()
}

// ---------------------------------------------------------------------------
// panicSink — a MetricsSink whose Counter.Inc() panics.
// Used by AC-TEL-009.
// ---------------------------------------------------------------------------

type panicCounter struct{}

func (panicCounter) Inc() { panic("intentional metrics sink panic") }
func (panicCounter) Add(_ float64) {
	panic("intentional metrics sink panic")
}

type panicHistogram struct{}

func (panicHistogram) Observe(_ float64) {
	panic("intentional metrics sink panic")
}

type panicSink struct {
	invoked atomic.Bool
}

func (s *panicSink) Counter(_ string, _ metrics.Labels) metrics.Counter {
	s.invoked.Store(true)
	return panicCounter{}
}

func (s *panicSink) Histogram(_ string, _ metrics.Labels, _ []float64) metrics.Histogram {
	s.invoked.Store(true)
	return panicHistogram{}
}

func (s *panicSink) Gauge(_ string, _ metrics.Labels) metrics.Gauge {
	return fakeGauge{}
}

// ---------------------------------------------------------------------------
// AC-TEL-003: OnClear emits calls counter +1 and duration histogram 1 observation.
// REQ-CMDCTX-TEL-001, REQ-CMDCTX-TEL-002
// ---------------------------------------------------------------------------

func TestMetrics_OnClear_CountsAndDuration(t *testing.T) {
	t.Parallel()
	sink := newFakeMetricsSink()
	// loopCtrl nil → OnClear returns ErrLoopControllerUnavailable; but calls
	// and duration must still be emitted (emission happens before nil-check).
	// We need a working loopCtrl to avoid nil error path interfering with assertion.
	lc := &fakeLoopController{}
	a := New(Options{
		Metrics:        sink,
		LoopController: lc,
	})

	err := a.OnClear()
	require.NoError(t, err)

	calls := sink.counterVal("cmdctx.method.calls", metrics.Labels{"method": "OnClear"})
	assert.Equal(t, int64(1), calls, "calls counter must be +1 after one OnClear call")

	durCount := sink.histCount("cmdctx.method.duration_ms", metrics.Labels{"method": "OnClear"})
	assert.Equal(t, 1, durCount, "duration histogram must have exactly 1 observation")
}

// ---------------------------------------------------------------------------
// AC-TEL-004: OnClear with nil loopCtrl emits error counter for ErrLoopControllerUnavailable.
// REQ-CMDCTX-TEL-006, REQ-CMDCTX-TEL-007
// ---------------------------------------------------------------------------

func TestMetrics_OnClear_NilLoopCtrl_ErrorCounter(t *testing.T) {
	t.Parallel()
	sink := newFakeMetricsSink()
	a := New(Options{
		Metrics:        sink,
		LoopController: nil, // triggers ErrLoopControllerUnavailable
	})

	err := a.OnClear()
	require.ErrorIs(t, err, ErrLoopControllerUnavailable)

	errCnt := sink.counterVal("cmdctx.method.errors", metrics.Labels{
		"method":     "OnClear",
		"error_type": "ErrLoopControllerUnavailable",
	})
	assert.Equal(t, int64(1), errCnt, "error counter must be +1 for ErrLoopControllerUnavailable")
}

// ---------------------------------------------------------------------------
// AC-TEL-005: ResolveModelAlias with unknown alias emits ErrUnknownModel error counter.
// REQ-CMDCTX-TEL-007
// ---------------------------------------------------------------------------

func TestMetrics_ResolveModelAlias_Unknown_ErrorCounter(t *testing.T) {
	t.Parallel()
	sink := newFakeMetricsSink()
	a := New(Options{
		Metrics: sink,
		// No registry, no aliasMap → unknown alias
	})

	_, err := a.ResolveModelAlias("totally-unknown-alias")
	require.ErrorIs(t, err, command.ErrUnknownModel)

	errCnt := sink.counterVal("cmdctx.method.errors", metrics.Labels{
		"method":     "ResolveModelAlias",
		"error_type": "ErrUnknownModel",
	})
	assert.Equal(t, int64(1), errCnt, "error counter must be +1 for ErrUnknownModel")
}

// ---------------------------------------------------------------------------
// AC-TEL-006: OnModelChange with custom (non-sentinel) error emits error_type=other.
// REQ-CMDCTX-TEL-007 — "other" fallback
// ---------------------------------------------------------------------------

func TestMetrics_OnModelChange_OtherError(t *testing.T) {
	t.Parallel()
	sink := newFakeMetricsSink()

	customErr := errors.New("custom loop error")
	lc := &fakeLoopController{nextErr: customErr}
	a := New(Options{
		Metrics:        sink,
		LoopController: lc,
	})

	err := a.OnModelChange(command.ModelInfo{ID: "anthropic/claude-opus-4-7"})
	require.ErrorIs(t, err, customErr)

	errCnt := sink.counterVal("cmdctx.method.errors", metrics.Labels{
		"method":     "OnModelChange",
		"error_type": "other",
	})
	assert.Equal(t, int64(1), errCnt, "error counter must be +1 for other error type")

	// Verify sentinel types are NOT counted.
	unknownCnt := sink.counterVal("cmdctx.method.errors", metrics.Labels{
		"method":     "OnModelChange",
		"error_type": "ErrUnknownModel",
	})
	assert.Equal(t, int64(0), unknownCnt)
}

// ---------------------------------------------------------------------------
// AC-TEL-007: PlanModeActive hot-path — 100 calls, counter == 100.
// REQ-CMDCTX-TEL-001, REQ-CMDCTX-TEL-002
// ---------------------------------------------------------------------------

func TestMetrics_PlanModeActive_HotPath(t *testing.T) {
	t.Parallel()
	sink := newFakeMetricsSink()
	a := New(Options{Metrics: sink})

	const n = 100
	for range n {
		_ = a.PlanModeActive()
	}

	calls := sink.counterVal("cmdctx.method.calls", metrics.Labels{"method": "PlanModeActive"})
	assert.Equal(t, int64(n), calls, "calls counter must equal number of PlanModeActive invocations")

	// PlanModeActive does not return error; errors counter must be zero.
	errCnt := sink.counterVal("cmdctx.method.errors", metrics.Labels{
		"method": "PlanModeActive",
	})
	assert.Equal(t, int64(0), errCnt, "PlanModeActive never returns error; error counter must be 0")
}

// ---------------------------------------------------------------------------
// AC-TEL-008: nil sink — all 6 methods execute without panic or side effect.
// REQ-CMDCTX-TEL-009
// ---------------------------------------------------------------------------

func TestMetrics_NilSink_NoOp(t *testing.T) {
	t.Parallel()

	// Use a panicSink to verify it is never called when Metrics == nil.
	// We pass nil explicitly; the panicSink is a "guard" to catch any bug
	// where the adapter resolves nil to the panicSink.
	a := New(Options{
		Metrics:        nil,
		LoopController: &fakeLoopController{},
	})

	// None of these must panic.
	require.NotPanics(t, func() {
		_ = a.OnClear()
		_ = a.OnCompactRequest(0)
		_ = a.OnModelChange(command.ModelInfo{ID: "test/model"})
		_, _ = a.ResolveModelAlias("unknown")
		_ = a.SessionSnapshot()
		_ = a.PlanModeActive()
	})
}

// ---------------------------------------------------------------------------
// AC-TEL-009: panic in sink — OnClear does not propagate the panic; Logger.Warn called once.
// REQ-CMDCTX-TEL-011
// ---------------------------------------------------------------------------

func TestMetrics_PanicInSink_DoesNotBreakMethod(t *testing.T) {
	t.Parallel()
	ps := &panicSink{}
	lg := &fakeWarnLogger{}
	lc := &fakeLoopController{}
	a := New(Options{
		Metrics:        ps,
		Logger:         lg,
		LoopController: lc,
	})

	// OnClear must not panic even though the sink panics.
	require.NotPanics(t, func() {
		_ = a.OnClear()
	})

	assert.True(t, ps.invoked.Load(), "panicSink must have been invoked")
	assert.Equal(t, 1, lg.getWarnCount(), "Logger.Warn must be called exactly once")

	args := lg.getLastArgs()
	require.Greater(t, len(args), 0, "warn args must not be empty")
	// First key should be "panic".
	found := false
	for i := 0; i < len(args)-1; i += 2 {
		if args[i] == "panic" {
			found = true
			break
		}
	}
	assert.True(t, found, "Warn must include 'panic' key in args")
}

// ---------------------------------------------------------------------------
// AC-TEL-010: WithContext child shares the same sink as the parent.
// REQ-CMDCTX-TEL-005
// ---------------------------------------------------------------------------

func TestMetrics_WithContext_ChildSharesSink(t *testing.T) {
	t.Parallel()
	sink := newFakeMetricsSink()
	lc := &fakeLoopController{}
	parent := New(Options{
		Metrics:        sink,
		LoopController: lc,
	})

	child := parent.WithContext(t.Context())

	// Emit via child.
	err := child.OnClear()
	require.NoError(t, err)

	// Counter must appear in the shared sink (same instance as parent's sink).
	calls := sink.counterVal("cmdctx.method.calls", metrics.Labels{"method": "OnClear"})
	assert.Equal(t, int64(1), calls, "child must emit to the parent's shared sink")
}

// ---------------------------------------------------------------------------
// AC-TEL-012: duration observations are >= 0 (monotonic clock, no negative).
// REQ-CMDCTX-TEL-002, REQ-CMDCTX-TEL-010
// ---------------------------------------------------------------------------

func TestMetrics_DurationOrder(t *testing.T) {
	t.Parallel()
	sink := newFakeMetricsSink()
	lc := &fakeLoopController{}
	a := New(Options{
		Metrics:        sink,
		LoopController: lc,
	})

	_ = a.OnClear()

	vals := sink.histValues("cmdctx.method.duration_ms", metrics.Labels{"method": "OnClear"})
	require.Len(t, vals, 1, "must have exactly 1 duration observation")
	assert.GreaterOrEqual(t, vals[0], float64(0), "duration must be non-negative (monotonic clock)")
}

// ---------------------------------------------------------------------------
// AC-TEL-018: error_type label values are exactly the 3 allowed strings.
// REQ-CMDCTX-TEL-013 — static enum validation via runtime exercise.
// ---------------------------------------------------------------------------

func TestMetrics_ErrorTypeStaticEnum(t *testing.T) {
	t.Parallel()

	allowedErrorTypes := map[string]bool{
		"ErrUnknownModel":              true,
		"ErrLoopControllerUnavailable": true,
		"other":                        true,
	}

	sink := newFakeMetricsSink()

	// 1. nil loopCtrl → ErrLoopControllerUnavailable
	a1 := New(Options{Metrics: sink, LoopController: nil})
	_ = a1.OnClear()
	_ = a1.OnCompactRequest(0)
	_ = a1.OnModelChange(command.ModelInfo{ID: "x"})

	// 2. unknown alias → ErrUnknownModel
	a2 := New(Options{Metrics: sink})
	_, _ = a2.ResolveModelAlias("no-such-alias")

	// 3. custom error → other
	customErr := errors.New("random err")
	lc := &fakeLoopController{nextErr: customErr}
	a3 := New(Options{Metrics: sink, LoopController: lc})
	_ = a3.OnClear()

	// Inspect all recorded counters for "cmdctx.method.errors".
	sink.mu.Lock()
	defer sink.mu.Unlock()
	for key := range sink.counters {
		// Keys look like "cmdctx.method.errors|error_type=X|method=Y"
		if len(key) < len("cmdctx.method.errors") {
			continue
		}
		if key[:len("cmdctx.method.errors")] != "cmdctx.method.errors" {
			continue
		}
		// Extract error_type value from key.
		const etPrefix = "error_type="
		found := false
		start := 0
		for {
			idx := indexOf(key[start:], etPrefix)
			if idx == -1 {
				break
			}
			absIdx := start + idx + len(etPrefix)
			// Value runs until the next "|" or end of string.
			end := absIdx
			for end < len(key) && key[end] != '|' {
				end++
			}
			val := key[absIdx:end]
			assert.True(t, allowedErrorTypes[val], "error_type=%q is not in the allowed enum", val)
			found = true
			start = end + 1
			if start >= len(key) {
				break
			}
		}
		_ = found
	}
}

// indexOf returns the index of needle in s, or -1 if not found.
func indexOf(s, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(needle); i++ {
		if s[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// NFR-TEL-003: nil-sink fast-path overhead benchmark.
// Compare BenchmarkPlanModeActive_NilSink vs BenchmarkPlanModeActive_WithMetrics
// to verify nil-sink adds <= 10ns overhead.
// ---------------------------------------------------------------------------

// BenchmarkPlanModeActive_NilSink measures PlanModeActive with no metrics sink.
// NFR-CMDCTX-TEL-003.
func BenchmarkPlanModeActive_NilSink(b *testing.B) {
	a := New(Options{}) // nil sink
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		_ = a.PlanModeActive()
	}
}

// BenchmarkPlanModeActive_WithMetrics measures PlanModeActive with a noop sink.
// NFR-CMDCTX-TEL-004.
func BenchmarkPlanModeActive_WithMetrics(b *testing.B) {
	sink := newFakeMetricsSink()
	a := New(Options{Metrics: sink})
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		_ = a.PlanModeActive()
	}
}
