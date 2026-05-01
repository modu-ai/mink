// Package expvar — test-only helpers for white-box inspection of internal
// handle types. Files with the _test.go suffix are compiled only when running
// `go test`, so the symbols below are NOT visible in production builds and do
// NOT pollute the public API surface of the expvar package.
//
// External test packages (package expvar_test) reference these helpers via the
// metricsexpvar import alias, e.g. metricsexpvar.CounterValue(c).
//
// SPEC-GOOSE-OBS-METRICS-001 — evaluator-active feedback W2: keep test helpers
// out of production source files.
package expvar

import "github.com/modu-ai/goose/internal/observability/metrics"

// CounterValue returns the current float64 value of a Counter handle.
// Panics if c is not an *expvarCounter (intentional — test helper, not for
// production callers).
func CounterValue(c metrics.Counter) float64 {
	return c.(*expvarCounter).value()
}

// GaugeValue returns the current float64 value of a Gauge handle.
// Panics if g is not an *expvarGauge (intentional — test helper).
func GaugeValue(g metrics.Gauge) float64 {
	return g.(*expvarGauge).value()
}

// HistogramCounts returns a snapshot of per-bucket counts for a Histogram
// handle. Panics if h is not an *expvarHistogram (intentional — test helper).
func HistogramCounts(h metrics.Histogram) []int64 {
	return h.(*expvarHistogram).countsSnapshot()
}

// ObserveHistogram calls Observe on h via the interface. Used by concurrent
// tests so that goroutines exercising the same handle do not need to perform
// type assertions inline.
func ObserveHistogram(h metrics.Histogram, value float64) {
	h.Observe(value)
}
