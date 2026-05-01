// Package noop provides a zero-cost metrics.Sink fallback used when metrics
// are disabled (GOOSE_METRICS_ENABLED unset/false).
// SPEC-GOOSE-OBS-METRICS-001.
package noop

import "github.com/modu-ai/goose/internal/observability/metrics"

// New returns a no-op metrics.Sink. All methods return shared no-op handles.
// Use as the default fallback when GOOSE_METRICS_ENABLED is unset or false.
//
// @MX:NOTE: [AUTO] zero-cost fallback; all handle methods are empty stubs.
// @MX:REASON: Target <= 5ns/op per NFR-OBS-METRICS-003. No allocations on
//
//	the hot path.
//
// @MX:SPEC: SPEC-GOOSE-OBS-METRICS-001 REQ-OBS-METRICS-011
func New() metrics.Sink {
	return noopSink{}
}

// noopSink implements metrics.Sink with empty stubs.
type noopSink struct{}

func (noopSink) Counter(_ string, _ metrics.Labels) metrics.Counter {
	return noopCounter{}
}

func (noopSink) Histogram(_ string, _ metrics.Labels, _ []float64) metrics.Histogram {
	return noopHistogram{}
}

func (noopSink) Gauge(_ string, _ metrics.Labels) metrics.Gauge {
	return noopGauge{}
}

// noopCounter is an empty Counter handle.
type noopCounter struct{}

func (noopCounter) Inc()          {}
func (noopCounter) Add(_ float64) {}

// noopHistogram is an empty Histogram handle.
type noopHistogram struct{}

func (noopHistogram) Observe(_ float64) {}

// noopGauge is an empty Gauge handle.
type noopGauge struct{}

func (noopGauge) Set(_ float64) {}
func (noopGauge) Add(_ float64) {}
