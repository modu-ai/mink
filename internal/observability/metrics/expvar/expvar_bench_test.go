// Package expvar_test — benchmark tests for expvar adapter.
// SPEC-GOOSE-OBS-METRICS-001 T-009.
package expvar_test

import (
	"testing"

	metricsexpvar "github.com/modu-ai/mink/internal/observability/metrics/expvar"
	"go.uber.org/zap"
)

// BenchmarkExpvarSink_CounterInc measures expvar.Counter.Inc() on an
// uncontended counter. Target: <= 100ns/op (NFR-OBS-METRICS-004).
//
// Handle caching is recommended for hot paths: obtain the Counter handle once
// and reuse it across calls to avoid map + lock overhead on each factory call.
func BenchmarkExpvarSink_CounterInc(b *testing.B) {
	s := metricsexpvar.New(zap.NewNop())
	c := s.Counter("bench.expvar.counter", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		c.Inc()
	}
}

// BenchmarkExpvarSink_HistogramObserve measures expvar.Histogram.Observe()
// with 5-bucket default configuration. Target: <= 200ns/op (NFR-OBS-METRICS-005).
func BenchmarkExpvarSink_HistogramObserve(b *testing.B) {
	s := metricsexpvar.New(zap.NewNop())
	// Default 5 buckets [0.1, 1, 10, 100, 1000] + +Inf = 6 slots.
	h := s.Histogram("bench.expvar.hist", nil, nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		h.Observe(5.0)
	}
}

// BenchmarkExpvarSink_GaugeSet measures expvar.Gauge.Set() overhead.
func BenchmarkExpvarSink_GaugeSet(b *testing.B) {
	s := metricsexpvar.New(zap.NewNop())
	g := s.Gauge("bench.expvar.gauge", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		g.Set(42.0)
	}
}
