// Package expvar provides the default stdlib-only metrics.Sink implementation
// backed by Go's expvar package. SPEC-GOOSE-OBS-METRICS-001.
//
// All metrics are exposed via stdlib expvar's /debug/vars endpoint when a
// net/http server is running in the same process.
package expvar

import (
	stdexpvar "expvar"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/modu-ai/mink/internal/observability/metrics"
	"go.uber.org/zap"
)

// defaultLabelCap is the maximum number of unique (name, labels) combinations
// per metric name. Overflow is silently dropped + warn-once logged (REQ-OBS-METRICS-009).
const defaultLabelCap = 100

// defaultBuckets is used when the caller passes nil or empty buckets (REQ-OBS-METRICS-007).
// Values are millisecond-scale upper bounds (0.1ms, 1ms, 10ms, 100ms, 1s).
var defaultBuckets = []float64{0.1, 1, 10, 100, 1000}

// New returns an expvar-backed metrics.Sink with the default cardinality cap (100).
// All metrics are exposed via stdlib expvar's /debug/vars endpoint.
//
// @MX:NOTE: [AUTO] stdlib expvar registers its /debug/vars HTTP handler in
//
//	net/http.DefaultServeMux at package init time when net/http is
//	imported. CLI-only programs without an HTTP server still accumulate
//	counters in-memory.
//
// @MX:REASON: Operator awareness: importing this package in a daemon with
//
//	net/http will expose /debug/vars automatically without further config.
//
// @MX:SPEC: SPEC-GOOSE-OBS-METRICS-001 REQ-OBS-METRICS-018
func New(logger *zap.Logger) metrics.Sink {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &expvarSink{
		logger:   logger,
		labelCap: defaultLabelCap,
		counters: make(map[string]*expvarCounter),
		gauges:   make(map[string]*expvarGauge),
		histos:   make(map[string]*expvarHistogram),
		capCount: make(map[string]*atomic.Int64),
		warnOnce: make(map[string]*sync.Once),
	}
}

// expvarSink is the internal implementation of metrics.Sink.
type expvarSink struct {
	logger   *zap.Logger
	labelCap int

	mu       sync.Mutex
	counters map[string]*expvarCounter
	gauges   map[string]*expvarGauge
	histos   map[string]*expvarHistogram
	capCount map[string]*atomic.Int64 // per metric-name series count
	warnOnce map[string]*sync.Once    // per metric-name warn-once gate
}

// getOrInitCap returns the atomic counter and warn-once gate for the given
// metric name, initializing them on first access. Must be called with mu held.
func (s *expvarSink) getOrInitCap(name string) (*atomic.Int64, *sync.Once) {
	cnt, ok := s.capCount[name]
	if !ok {
		cnt = &atomic.Int64{}
		s.capCount[name] = cnt
	}
	once, ok := s.warnOnce[name]
	if !ok {
		once = &sync.Once{}
		s.warnOnce[name] = once
	}
	return cnt, once
}

// capExceeded checks whether the cardinality cap is exceeded for a new series.
// If exceeded, emits a single warn log and returns true.
// Must be called with mu held (the atomic increment/read is inside the lock).
func (s *expvarSink) capExceeded(name string) bool {
	cnt, once := s.getOrInitCap(name)
	if cnt.Load() >= int64(s.labelCap) {
		// @MX:WARN: [AUTO] silent drop on cardinality overflow — the only signal
		// @MX:REASON: Silent drop is operator-surprising; warn-once log is the
		//             only observable signal that overflow occurred. Without this
		//             log, metric loss is invisible.
		// @MX:SPEC: SPEC-GOOSE-OBS-METRICS-001 REQ-OBS-METRICS-009
		once.Do(func() {
			s.logger.Warn("metrics cardinality cap exceeded",
				zap.String("metric", name),
				zap.Int("cap", s.labelCap))
		})
		return true
	}
	cnt.Add(1)
	return false
}

// ─── Counter ─────────────────────────────────────────────────────────────────

// Counter returns or creates an expvar-backed counter for (name, labels).
func (s *expvarSink) Counter(name string, labels metrics.Labels) metrics.Counter {
	key := seriesKey(name, labels)

	s.mu.Lock()
	defer s.mu.Unlock()

	if c, ok := s.counters[key]; ok {
		return c
	}
	if s.capExceeded(name) {
		return noopCounterHandle{}
	}

	// Use expvar.Float for float64 arithmetic (Add(2.5) compatibility — AC-006).
	// getOrCreateFloat handles the case where the key is already registered in
	// the global expvar registry (e.g. from a previous process registration).
	ev := getOrCreateFloat(key)
	c := &expvarCounter{ev: ev}
	s.counters[key] = c
	return c
}

// expvarCounter wraps expvar.Float with Counter semantics.
type expvarCounter struct {
	ev *stdexpvar.Float
}

func (c *expvarCounter) Inc()           { c.ev.Add(1) }
func (c *expvarCounter) Add(d float64)  { c.ev.Add(d) }
func (c *expvarCounter) value() float64 { return c.ev.Value() }

// ─── Gauge ────────────────────────────────────────────────────────────────────

// Gauge returns or creates an expvar-backed gauge for (name, labels).
func (s *expvarSink) Gauge(name string, labels metrics.Labels) metrics.Gauge {
	key := seriesKey(name, labels)

	s.mu.Lock()
	defer s.mu.Unlock()

	if g, ok := s.gauges[key]; ok {
		return g
	}
	if s.capExceeded(name) {
		return noopGaugeHandle{}
	}

	ev := getOrCreateFloat(key)
	g := &expvarGauge{ev: ev}
	s.gauges[key] = g
	return g
}

// expvarGauge wraps expvar.Float with Gauge semantics.
type expvarGauge struct {
	ev *stdexpvar.Float
}

func (g *expvarGauge) Set(v float64)  { g.ev.Set(v) }
func (g *expvarGauge) Add(d float64)  { g.ev.Add(d) }
func (g *expvarGauge) value() float64 { return g.ev.Value() }

// ─── Histogram ────────────────────────────────────────────────────────────────

// Histogram returns or creates an expvar-backed histogram for (name, labels, buckets).
func (s *expvarSink) Histogram(name string, labels metrics.Labels, buckets []float64) metrics.Histogram {
	key := seriesKey(name, labels)

	s.mu.Lock()
	defer s.mu.Unlock()

	if h, ok := s.histos[key]; ok {
		return h
	}
	if s.capExceeded(name) {
		return noopHistogramHandle{}
	}

	bounds := normalizeBuckets(buckets)
	// len(bounds) explicit buckets + 1 +Inf overflow slot.
	counts := make([]atomic.Int64, len(bounds)+1)

	h := &expvarHistogram{
		bounds: bounds,
		counts: counts,
	}
	// Expose per-bucket counts as a JSON object under /debug/vars.
	// Guard against duplicate publish (e.g. test re-runs in the same process).
	if stdexpvar.Get(key) == nil {
		stdexpvar.Publish(key, stdexpvar.Func(h.expvarFunc))
	}
	s.histos[key] = h
	return h
}

// normalizeBuckets validates and returns the bucket upper bounds to use.
// Falls back to defaultBuckets when the input is nil, empty, non-ascending,
// or contains duplicates or non-finite values (HARD constraints #6 #7).
func normalizeBuckets(input []float64) []float64 {
	if len(input) == 0 {
		cp := make([]float64, len(defaultBuckets))
		copy(cp, defaultBuckets)
		return cp
	}
	// Validate: strictly ascending, finite values (no NaN or +/-Inf).
	for i, v := range input {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return append([]float64(nil), defaultBuckets...)
		}
		if i > 0 && v <= input[i-1] {
			return append([]float64(nil), defaultBuckets...)
		}
	}
	cp := make([]float64, len(input))
	copy(cp, input)
	return cp
}

// expvarHistogram implements metrics.Histogram using atomic per-bucket counters
// and binary search placement. The last slot is the +Inf overflow bucket.
type expvarHistogram struct {
	bounds []float64      // upper bounds; len(counts) == len(bounds)+1
	counts []atomic.Int64 // parallel counters; counts[len(bounds)] is +Inf
}

// Observe records value into the appropriate bucket via binary search (O(log n)).
func (h *expvarHistogram) Observe(value float64) {
	// sort.SearchFloat64s returns the first index i where bounds[i] >= value.
	i := sort.SearchFloat64s(h.bounds, value)
	if i >= len(h.bounds) {
		// Value exceeds all explicit bounds → +Inf bucket.
		h.counts[len(h.bounds)].Add(1)
	} else {
		h.counts[i].Add(1)
	}
}

// counts returns a snapshot of per-bucket observation counts.
func (h *expvarHistogram) countsSnapshot() []int64 {
	out := make([]int64, len(h.counts))
	for i := range h.counts {
		out[i] = h.counts[i].Load()
	}
	return out
}

// expvarFunc returns the histogram as a map[string]int64 for expvar JSON output.
func (h *expvarHistogram) expvarFunc() interface{} {
	m := make(map[string]int64, len(h.counts))
	for i, bound := range h.bounds {
		key := fmt.Sprintf("le_%g", bound)
		m[key] = h.counts[i].Load()
	}
	m["le_+Inf"] = h.counts[len(h.bounds)].Load()
	return m
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// getOrCreateFloat returns the existing expvar.Float for key if one is already
// registered in the global registry, or creates and registers a new one.
// This prevents the panic that stdexpvar.NewFloat triggers on duplicate keys.
//
// If a non-Float expvar.Var is already registered under key (e.g. an *expvar.Int
// from an unrelated subsystem), this function returns a detached *expvar.Float
// that is NOT published to the global registry. The returned Float still
// accumulates correctly for in-process readers via the handle, but its value
// will not be visible under /debug/vars at that key. This is preferable to a
// panic per REQ-OBS-METRICS-015 (no panic regardless of caller input).
func getOrCreateFloat(key string) *stdexpvar.Float {
	if existing := stdexpvar.Get(key); existing != nil {
		if f, ok := existing.(*stdexpvar.Float); ok {
			return f
		}
		// Conflict: a non-Float Var is already registered under this key.
		// Return a detached Float to avoid panicking on duplicate publish.
		return new(stdexpvar.Float)
	}
	return stdexpvar.NewFloat(key)
}

// ─── noop handles (returned on cardinality cap overflow) ─────────────────────

type noopCounterHandle struct{}

func (noopCounterHandle) Inc()          {}
func (noopCounterHandle) Add(_ float64) {}

type noopGaugeHandle struct{}

func (noopGaugeHandle) Set(_ float64) {}
func (noopGaugeHandle) Add(_ float64) {}

type noopHistogramHandle struct{}

func (noopHistogramHandle) Observe(_ float64) {}
